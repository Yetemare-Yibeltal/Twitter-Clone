// backend/internal/utils/websocket.go
package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// WebSocket settings
	DefaultPingInterval   = 30 * time.Second
	DefaultPongWait       = 60 * time.Second
	DefaultWriteWait      = 10 * time.Second
	DefaultMaxMessageSize = 512 * 1024 // 512KB

	// Message types
	MessageTypePing = "ping"
	MessageTypePong = "pong"
	MessageTypeText = "text"
	MessageTypeJSON = "json"
	MessageTypeBinary = "binary"
)

var (
	ErrWebSocketClosed = errors.New("websocket connection closed")
	ErrWriteTimeout    = errors.New("websocket write timeout")
	ErrReadTimeout     = errors.New("websocket read timeout")
	ErrInvalidMessage  = errors.New("invalid websocket message")
)

// ======================================================================
// WebSocket Upgrader
// ======================================================================

// UpgraderConfig holds configuration for the WebSocket upgrader.
type UpgraderConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(r *http.Request) bool
	Subprotocols    []string
}

// DefaultUpgraderConfig returns sensible defaults.
func DefaultUpgraderConfig() UpgraderConfig {
	return UpgraderConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
}

// NewUpgrader creates a new WebSocket upgrader with the given config.
func NewUpgrader(cfg UpgraderConfig) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		CheckOrigin:     cfg.CheckOrigin,
		Subprotocols:    cfg.Subprotocols,
	}
}

// NewDefaultUpgrader creates a new WebSocket upgrader with default config.
func NewDefaultUpgrader() websocket.Upgrader {
	return NewUpgrader(DefaultUpgraderConfig())
}

// ======================================================================
// WebSocket Client
// ======================================================================

// WebSocketClient represents a connected WebSocket client.
type WebSocketClient struct {
	ID            string
	Conn          *websocket.Conn
	Send          chan []byte
	Receive       chan []byte
	PingInterval  time.Duration
	PongWait      time.Duration
	WriteWait     time.Duration
	MaxMessageSize int64
	IsClosed      bool
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// ClientConfig holds configuration for a WebSocket client.
type ClientConfig struct {
	PingInterval   time.Duration
	PongWait       time.Duration
	WriteWait      time.Duration
	MaxMessageSize int64
	BufferSize     int
}

// DefaultClientConfig returns sensible defaults.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		PingInterval:   DefaultPingInterval,
		PongWait:       DefaultPongWait,
		WriteWait:      DefaultWriteWait,
		MaxMessageSize: DefaultMaxMessageSize,
		BufferSize:     256,
	}
}

// NewWebSocketClient creates a new WebSocket client.
func NewWebSocketClient(conn *websocket.Conn, cfg ClientConfig) *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketClient{
		ID:             GenerateUUID(),
		Conn:           conn,
		Send:           make(chan []byte, cfg.BufferSize),
		Receive:        make(chan []byte, cfg.BufferSize),
		PingInterval:   cfg.PingInterval,
		PongWait:       cfg.PongWait,
		WriteWait:      cfg.WriteWait,
		MaxMessageSize: cfg.MaxMessageSize,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start begins the client's read and write pumps.
func (c *WebSocketClient) Start() {
	c.wg.Add(2)
	go c.readPump()
	go c.writePump()
}

// Stop closes the client and stops all goroutines.
func (c *WebSocketClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.IsClosed {
		return
	}
	c.IsClosed = true
	c.cancel()
	c.Conn.Close()
	close(c.Send)
	close(c.Receive)
	c.wg.Wait()
}

// readPump reads messages from the WebSocket connection.
func (c *WebSocketClient) readPump() {
	defer c.wg.Done()
	defer c.Stop()

	c.Conn.SetReadLimit(c.MaxMessageSize)
	if err := c.Conn.SetReadDeadline(time.Now().Add(c.PongWait)); err != nil {
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(c.PongWait))
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			messageType, data, err := c.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					// Log or handle error
				}
				return
			}
			// Handle ping/pong internally
			if messageType == websocket.PingMessage {
				_ = c.Conn.WriteMessage(websocket.PongMessage, nil)
				continue
			}
			if messageType == websocket.PongMessage {
				continue
			}
			// Send to receive channel
			select {
			case c.Receive <- data:
			default:
				// Buffer full, drop message
			}
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *WebSocketClient) writePump() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message, ok := <-c.Send:
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.SetWriteDeadline(time.Now().Add(c.WriteWait)); err != nil {
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)
			// Write any queued messages
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte("\n"))
				_, _ = w.Write(<-c.Send)
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(c.WriteWait)); err != nil {
				return
			}
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendJSON sends a JSON message to the client.
func (c *WebSocketClient) SendJSON(data interface{}) error {
	if c.IsClosed {
		return ErrWebSocketClosed
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	select {
	case c.Send <- bytes:
		return nil
	case <-c.ctx.Done():
		return ErrWebSocketClosed
	default:
		return errors.New("send channel full")
	}
}

// ReceiveJSON receives and unmarshals a JSON message from the client.
func (c *WebSocketClient) ReceiveJSON(v interface{}) error {
	select {
	case data, ok := <-c.Receive:
		if !ok {
			return ErrWebSocketClosed
		}
		if err := json.Unmarshal(data, v); err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		return nil
	case <-c.ctx.Done():
		return ErrWebSocketClosed
	}
}

// SendText sends a text message to the client.
func (c *WebSocketClient) SendText(text string) error {
	if c.IsClosed {
		return ErrWebSocketClosed
	}
	select {
	case c.Send <- []byte(text):
		return nil
	case <-c.ctx.Done():
		return ErrWebSocketClosed
	default:
		return errors.New("send channel full")
	}
}

// ReceiveText receives a text message from the client.
func (c *WebSocketClient) ReceiveText() (string, error) {
	select {
	case data, ok := <-c.Receive:
		if !ok {
			return "", ErrWebSocketClosed
		}
		return string(data), nil
	case <-c.ctx.Done():
		return "", ErrWebSocketClosed
	}
}

// IsAlive checks if the client is still connected.
func (c *WebSocketClient) IsAlive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.IsClosed
}

// GetID returns the client ID.
func (c *WebSocketClient) GetID() string {
	return c.ID
}

// ======================================================================
= WebSocket Hub
// ======================================================================

// WebSocketHub manages multiple WebSocket clients.
type WebSocketHub struct {
	clients     map[string]*WebSocketClient
	mu          sync.RWMutex
	register    chan *WebSocketClient
	unregister  chan *WebSocketClient
	broadcast   chan []byte
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewWebSocketHub creates a new WebSocket hub.
func NewWebSocketHub() *WebSocketHub {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketHub{
		clients:    make(map[string]*WebSocketClient),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		broadcast:  make(chan []byte, 256),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins the hub's main loop.
func (h *WebSocketHub) Start() {
	h.wg.Add(1)
	go h.run()
}

// Stop stops the hub and all clients.
func (h *WebSocketHub) Stop() {
	h.cancel()
	h.wg.Wait()
}

// run is the main loop for the hub.
func (h *WebSocketHub) run() {
	defer h.wg.Done()
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				client.Stop()
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.Send <- message:
				default:
					// Skip if client is full
				}
			}
			h.mu.RUnlock()
		case <-h.ctx.Done():
			h.mu.Lock()
			for _, client := range h.clients {
				client.Stop()
			}
			h.clients = make(map[string]*WebSocketClient)
			h.mu.Unlock()
			return
		}
	}
}

// Register adds a client to the hub.
func (h *WebSocketHub) Register(client *WebSocketClient) {
	select {
	case h.register <- client:
	case <-h.ctx.Done():
	}
}

// Unregister removes a client from the hub.
func (h *WebSocketHub) Unregister(client *WebSocketClient) {
	select {
	case h.unregister <- client:
	case <-h.ctx.Done():
	}
}

// Broadcast sends a message to all connected clients.
func (h *WebSocketHub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	case <-h.ctx.Done():
	}
}

// BroadcastJSON sends a JSON message to all connected clients.
func (h *WebSocketHub) BroadcastJSON(data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	h.Broadcast(bytes)
	return nil
}

// BroadcastToClient sends a message to a specific client.
func (h *WebSocketHub) BroadcastToClient(clientID string, message []byte) error {
	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()
	if !ok {
		return errors.New("client not found")
	}
	select {
	case client.Send <- message:
		return nil
	case <-h.ctx.Done():
		return ErrWebSocketClosed
	default:
		return errors.New("client send channel full")
	}
}

// BroadcastToClientJSON sends a JSON message to a specific client.
func (h *WebSocketHub) BroadcastToClientJSON(clientID string, data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return h.BroadcastToClient(clientID, bytes)
}

// GetClient returns a client by ID.
func (h *WebSocketHub) GetClient(clientID string) (*WebSocketClient, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.clients[clientID]
	return client, ok
}

// GetClients returns all connected clients.
func (h *WebSocketHub) GetClients() []*WebSocketClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]*WebSocketClient, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	return clients
}

// GetClientCount returns the number of connected clients.
func (h *WebSocketHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ======================================================================
= WebSocket Middleware
// ======================================================================

// WebSocketAuthFunc is a function that authenticates a WebSocket request.
type WebSocketAuthFunc func(r *http.Request) (userID string, err error)

// WebSocketUpgradeHandler upgrades an HTTP request to a WebSocket connection.
func WebSocketUpgradeHandler(
	w http.ResponseWriter,
	r *http.Request,
	upgrader websocket.Upgrader,
	authFunc WebSocketAuthFunc,
	clientConfig ClientConfig,
) (*WebSocketClient, error) {
	// Authenticate
	if authFunc != nil {
		userID, err := authFunc(r)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		// Store user ID in context for later use
		ctx := context.WithValue(r.Context(), "user_id", userID)
		r = r.WithContext(ctx)
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade failed: %w", err)
	}

	// Create client
	client := NewWebSocketClient(conn, clientConfig)
	client.Start()
	return client, nil
}

// ======================================================================
= Utility Functions
// ======================================================================

// IsWebSocketRequest checks if a request is a WebSocket upgrade request.
func IsWebSocketRequest(r *http.Request) bool {
	return r.Header.Get("Upgrade") == "websocket" &&
		r.Header.Get("Connection") == "Upgrade"
}

// GetWebSocketUserID extracts the user ID from the WebSocket context.
func GetWebSocketUserID(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}

// CloseWebSocketClient safely closes a WebSocket client.
func CloseWebSocketClient(client *WebSocketClient) {
	if client != nil {
		client.Stop()
	}
}

// SafeWrite writes a message with error handling.
func SafeWrite(client *WebSocketClient, message []byte) error {
	if client == nil || client.IsClosed {
		return ErrWebSocketClosed
	}
	select {
	case client.Send <- message:
		return nil
	case <-client.ctx.Done():
		return ErrWebSocketClosed
	default:
		return errors.New("write blocked")
	}
}

// SafeWriteJSON writes a JSON message with error handling.
func SafeWriteJSON(client *WebSocketClient, data interface{}) error {
	if client == nil || client.IsClosed {
		return ErrWebSocketClosed
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal JSON failed: %w", err)
	}
	return SafeWrite(client, bytes)
}