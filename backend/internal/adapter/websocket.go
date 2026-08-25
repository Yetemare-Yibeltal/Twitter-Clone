// backend/internal/adapter/websocket.go
package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/config"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// WebSocket settings
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
	PingPeriod     = (PongWait * 9) / 10
	MaxMessageSize = 512 * 1024 // 512KB

	// Message types
	MsgTypePing         = "ping"
	MsgTypePong         = "pong"
	MsgTypeNewMessage   = "new_message"
	MsgTypeTyping       = "typing"
	MsgTypeStopTyping   = "stop_typing"
	MsgTypeReadReceipt  = "read_receipt"
	MsgTypeNotification = "notification"
	MsgTypeNewTweet     = "new_tweet"
	MsgTypeLike         = "like"
	MsgTypeRetweet      = "retweet"
	MsgTypeFollow       = "follow"
	MsgTypeError        = "error"
	MsgTypeAck          = "ack"
	MsgTypeOnline       = "online"
	MsgTypeOffline      = "offline"
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrWebSocketClosed     = errors.New("websocket connection closed")
	ErrWebSocketNotOpen    = errors.New("websocket connection not open")
	ErrWebSocketWriteFailed = errors.New("websocket write failed")
	ErrWebSocketReadFailed  = errors.New("websocket read failed")
	ErrWebSocketUpgradeFailed = errors.New("websocket upgrade failed")
	ErrWebSocketAuthFailed   = errors.New("websocket authentication failed")
)

// ======================================================================
= Client
// ======================================================================

// Client represents a WebSocket connection.
type Client struct {
	ID         string
	UserID     string
	Conn       *websocket.Conn
	Hub        *WebSocketHub
	Send       chan []byte
	Rooms      map[string]bool
	CreatedAt  time.Time
	LastPing   time.Time
	UserAgent  string
	IP         string
	mu         sync.RWMutex
}

// ======================================================================
= WebSocketHub
// ======================================================================

// WebSocketHub maintains the set of active clients and broadcasts messages.
type WebSocketHub struct {
	// Registered clients (userID -> client)
	Clients map[string]*Client
	// Clients by room (roomID -> userID -> true)
	Rooms map[string]map[string]bool
	// Register requests
	Register chan *Client
	// Unregister requests
	Unregister chan *Client
	// Broadcast to all clients
	Broadcast chan []byte
	// Broadcast to specific user
	UserBroadcast chan UserMessage
	// Broadcast to room
	RoomBroadcast chan RoomMessage
	// Mutex for safe access
	mu sync.RWMutex
	// Logger
	log *logrus.Entry
	// Stop channel
	stop chan struct{}
	// Wait group
	wg sync.WaitGroup
	// Configuration
	config *WebSocketConfig
}

// WebSocketConfig holds WebSocket configuration.
type WebSocketConfig struct {
	PingInterval    time.Duration
	WriteWait       time.Duration
	PongWait        time.Duration
	MaxMessageSize  int64
	EnableHeartbeat bool
}

// UserMessage represents a message to a specific user.
type UserMessage struct {
	UserID string
	Data   []byte
}

// RoomMessage represents a message to a room.
type RoomMessage struct {
	RoomID string
	Data   []byte
}

// ======================================================================
= Hub Creation
// ======================================================================

// NewWebSocketHub creates a new WebSocket hub.
func NewWebSocketHub(cfg *WebSocketConfig) *WebSocketHub {
	if cfg == nil {
		cfg = &WebSocketConfig{
			PingInterval:    PingPeriod,
			WriteWait:       WriteWait,
			PongWait:        PongWait,
			MaxMessageSize:  MaxMessageSize,
			EnableHeartbeat: true,
		}
	}
	return &WebSocketHub{
		Clients:       make(map[string]*Client),
		Rooms:         make(map[string]map[string]bool),
		Register:      make(chan *Client, 256),
		Unregister:    make(chan *Client, 256),
		Broadcast:     make(chan []byte, 256),
		UserBroadcast: make(chan UserMessage, 256),
		RoomBroadcast: make(chan RoomMessage, 256),
		log:           logger.WithField("component", "websocket_hub"),
		stop:          make(chan struct{}),
		config:        cfg,
	}
}

// ======================================================================
= Hub Run
// ======================================================================

// Run starts the hub main loop.
func (h *WebSocketHub) Run() {
	h.wg.Add(1)
	defer h.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case message := <-h.Broadcast:
			h.broadcastMessage(message)

		case userMsg := <-h.UserBroadcast:
			h.sendToUser(userMsg.UserID, userMsg.Data)

		case roomMsg := <-h.RoomBroadcast:
			h.sendToRoom(roomMsg.RoomID, roomMsg.Data)

		case <-ticker.C:
			h.cleanupInactiveClients()

		case <-h.stop:
			h.shutdown()
			return
		}
	}
}

// ======================================================================
= Client Registration
// ======================================================================

// registerClient registers a new client.
func (h *WebSocketHub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If client already exists, close old connection
	if oldClient, exists := h.Clients[client.UserID]; exists {
		h.log.WithFields(logrus.Fields{
			"user_id":   client.UserID,
			"client_id": oldClient.ID,
		}).Info("Replacing existing client connection")
		close(oldClient.Send)
		delete(h.Clients, client.UserID)
		// Remove from rooms
		for roomID, members := range h.Rooms {
			delete(members, client.UserID)
			if len(members) == 0 {
				delete(h.Rooms, roomID)
			}
		}
	}

	h.Clients[client.UserID] = client

	// Add to personal room
	client.Rooms["user:"+client.UserID] = true
	// Add to online room
	client.Rooms["online"] = true
	h.addToRoom("online", client.UserID)

	// Add to any other rooms the client joins
	for roomID := range client.Rooms {
		h.addToRoom(roomID, client.UserID)
	}

	h.log.WithFields(logrus.Fields{
		"user_id":   client.UserID,
		"client_id": client.ID,
		"total":     len(h.Clients),
	}).Info("Client registered")

	// Broadcast online status
	h.broadcastUserStatus(client.UserID, true)
}

// addToRoom adds a user to a room.
func (h *WebSocketHub) addToRoom(roomID, userID string) {
	if h.Rooms[roomID] == nil {
		h.Rooms[roomID] = make(map[string]bool)
	}
	h.Rooms[roomID][userID] = true
}

// removeFromRoom removes a user from a room.
func (h *WebSocketHub) removeFromRoom(roomID, userID string) {
	if members, ok := h.Rooms[roomID]; ok {
		delete(members, userID)
		if len(members) == 0 {
			delete(h.Rooms, roomID)
		}
	}
}

// ======================================================================
= Client Unregistration
// ======================================================================

// unregisterClient unregisters a client.
func (h *WebSocketHub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, exists := h.Clients[client.UserID]; exists && existing.ID == client.ID {
		delete(h.Clients, client.UserID)

		// Remove from all rooms
		for roomID := range client.Rooms {
			h.removeFromRoom(roomID, client.UserID)
		}
		// Ensure user room is removed
		h.removeFromRoom("user:"+client.UserID, client.UserID)
		h.removeFromRoom("online", client.UserID)

		close(client.Send)

		h.log.WithFields(logrus.Fields{
			"user_id":   client.UserID,
			"client_id": client.ID,
			"total":     len(h.Clients),
		}).Info("Client unregistered")

		// Broadcast offline status
		h.broadcastUserStatus(client.UserID, false)
	}
}

// ======================================================================
= Broadcast Methods
// ======================================================================

// broadcastMessage broadcasts a message to all clients.
func (h *WebSocketHub) broadcastMessage(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.Clients {
		select {
		case client.Send <- message:
		default:
			h.log.WithField("user_id", client.UserID).Warn("Client send channel full, skipping")
		}
	}
}

// sendToUser sends a message to a specific user.
func (h *WebSocketHub) sendToUser(userID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if client, exists := h.Clients[userID]; exists {
		select {
		case client.Send <- data:
		default:
			h.log.WithField("user_id", userID).Warn("Client send channel full")
		}
	}
}

// sendToRoom sends a message to all clients in a room.
func (h *WebSocketHub) sendToRoom(roomID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if members, ok := h.Rooms[roomID]; ok {
		for userID := range members {
			if client, exists := h.Clients[userID]; exists {
				select {
				case client.Send <- data:
				default:
					h.log.WithFields(logrus.Fields{
						"user_id": userID,
						"room_id": roomID,
					}).Warn("Client send channel full")
				}
			}
		}
	}
}

// broadcastUserStatus broadcasts user online/offline status.
func (h *WebSocketHub) broadcastUserStatus(userID string, online bool) {
	status := MsgTypeOffline
	if online {
		status = MsgTypeOnline
	}
	response := map[string]interface{}{
		"type":      MsgTypeUserStatus,
		"user_id":   userID,
		"status":    status,
		"timestamp": time.Now().Unix(),
	}
	data, _ := json.Marshal(response)
	h.Broadcast <- data
}

// ======================================================================
= Cleanup
// ======================================================================

// cleanupInactiveClients removes clients with stale connections.
func (h *WebSocketHub) cleanupInactiveClients() {
	if !h.config.EnableHeartbeat {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for userID, client := range h.Clients {
		if now.Sub(client.LastPing) > 2*h.config.PongWait {
			h.log.WithFields(logrus.Fields{
				"user_id":   userID,
				"client_id": client.ID,
				"last_ping": client.LastPing,
			}).Warn("Client inactive, removing")
			close(client.Send)
			delete(h.Clients, userID)
			// Remove from rooms
			for roomID, members := range h.Rooms {
				delete(members, userID)
				if len(members) == 0 {
					delete(h.Rooms, roomID)
				}
			}
			h.broadcastUserStatus(userID, false)
		}
	}
}

// ======================================================================
= Shutdown
// ======================================================================

// shutdown gracefully shuts down the hub.
func (h *WebSocketHub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for userID, client := range h.Clients {
		close(client.Send)
		delete(h.Clients, userID)
	}
	h.log.Info("Hub shutdown complete")
}

// Stop stops the hub.
func (h *WebSocketHub) Stop() {
	close(h.stop)
	h.wg.Wait()
}

// ======================================================================
= Hub Status
// ======================================================================

// GetClientCount returns the number of connected clients.
func (h *WebSocketHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.Clients)
}

// GetOnlineUsers returns a list of online user IDs.
func (h *WebSocketHub) GetOnlineUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := make([]string, 0, len(h.Clients))
	for userID := range h.Clients {
		users = append(users, userID)
	}
	return users
}

// IsUserOnline checks if a user is online.
func (h *WebSocketHub) IsUserOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.Clients[userID]
	return ok
}

// GetRoomUsers returns users in a room.
func (h *WebSocketHub) GetRoomUsers(roomID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if members, ok := h.Rooms[roomID]; ok {
		users := make([]string, 0, len(members))
		for userID := range members {
			users = append(users, userID)
		}
		return users
	}
	return []string{}
}

// GetStats returns hub statistics.
func (h *WebSocketHub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return map[string]interface{}{
		"total_clients":   len(h.Clients),
		"total_rooms":     len(h.Rooms),
		"online_users":    len(h.Clients),
		"broadcast_queue": len(h.Broadcast),
		"user_queue":      len(h.UserBroadcast),
		"room_queue":      len(h.RoomBroadcast),
	}
}

// ======================================================================
= WebSocket Adapter
// ======================================================================

// WebSocketAdapter wraps the hub with additional functionality.
type WebSocketAdapter struct {
	hub       *WebSocketHub
	upgrader  websocket.Upgrader
	config    *config.Config
	log       *logrus.Entry
	authFunc  func(ctx context.Context, token string) (string, error)
}

// NewWebSocketAdapter creates a new WebSocket adapter.
func NewWebSocketAdapter(hub *WebSocketHub, cfg *config.Config) *WebSocketAdapter {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if cfg.Environment == "production" {
				origin := r.Header.Get("Origin")
				for _, allowed := range cfg.AllowedOrigins {
					if origin == allowed {
						return true
					}
				}
				return false
			}
			return true
		},
	}
	return &WebSocketAdapter{
		hub:      hub,
		upgrader: upgrader,
		config:   cfg,
		log:      logger.WithField("component", "websocket_adapter"),
	}
}

// SetAuthFunc sets the authentication function.
func (a *WebSocketAdapter) SetAuthFunc(fn func(ctx context.Context, token string) (string, error)) {
	a.authFunc = fn
}

// ======================================================================
= Connection Handler
// ======================================================================

// ServeWS handles WebSocket upgrade requests.
func (a *WebSocketAdapter) ServeWS(w http.ResponseWriter, r *http.Request, userID string) error {
	// Upgrade to WebSocket
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		a.log.WithError(err).Error("Failed to upgrade to WebSocket")
		return ErrWebSocketUpgradeFailed
	}

	// Create client
	client := &Client{
		ID:         generateID(),
		UserID:     userID,
		Conn:       conn,
		Hub:        a.hub,
		Send:       make(chan []byte, 256),
		Rooms:      make(map[string]bool),
		CreatedAt:  time.Now(),
		LastPing:   time.Now(),
		UserAgent:  r.UserAgent(),
		IP:         r.RemoteAddr,
	}

	// Set read/write deadlines
	conn.SetReadLimit(a.hub.config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(a.hub.config.PongWait))
	conn.SetPongHandler(func(string) error {
		client.mu.Lock()
		client.LastPing = time.Now()
		client.mu.Unlock()
		conn.SetReadDeadline(time.Now().Add(a.hub.config.PongWait))
		return nil
	})

	// Register client
	a.hub.Register <- client

	// Start write pump
	go a.writePump(client)

	// Start read pump (blocking)
	a.readPump(client)

	return nil
}

// ======================================================================
= Read Pump
// ======================================================================

// readPump reads messages from the WebSocket connection.
func (a *WebSocketAdapter) readPump(client *Client) {
	defer func() {
		a.hub.Unregister <- client
		client.Conn.Close()
	}()

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				a.log.WithError(err).WithField("user_id", client.UserID).Warn("WebSocket read error")
			}
			break
		}

		// Process message
		a.handleMessage(client, message)
	}
}

// ======================================================================
= Write Pump
// ======================================================================

// writePump writes messages to the WebSocket connection.
func (a *WebSocketAdapter) writePump(client *Client) {
	ticker := time.NewTicker(a.hub.config.PingInterval)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(a.hub.config.WriteWait))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the same WebSocket message
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if !a.hub.config.EnableHeartbeat {
				continue
			}
			client.Conn.SetWriteDeadline(time.Now().Add(a.hub.config.WriteWait))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ======================================================================
= Message Handling
// ======================================================================

// handleMessage processes incoming WebSocket messages.
func (a *WebSocketAdapter) handleMessage(client *Client, data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		a.sendError(client, "Invalid JSON format")
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		a.sendError(client, "Message type is required")
		return
	}

	switch msgType {
	case MsgTypePing:
		a.handlePing(client, msg)
	case MsgTypeTyping:
		a.handleTyping(client, msg)
	case MsgTypeStopTyping:
		a.handleStopTyping(client, msg)
	case MsgTypeReadReceipt:
		a.handleReadReceipt(client, msg)
	default:
		// Forward unknown messages as-is (could be custom)
		a.log.WithField("type", msgType).Debug("Unknown message type")
	}
}

// ======================================================================
= Message Handlers
// ======================================================================

// handlePing handles ping messages.
func (a *WebSocketAdapter) handlePing(client *Client, msg map[string]interface{}) {
	client.mu.Lock()
	client.LastPing = time.Now()
	client.mu.Unlock()

	response := map[string]interface{}{
		"type":      MsgTypePong,
		"timestamp": time.Now().Unix(),
	}
	a.sendToClient(client, response)
}

// handleTyping handles typing indicators.
func (a *WebSocketAdapter) handleTyping(client *Client, msg map[string]interface{}) {
	receiverID, ok := msg["receiver_id"].(string)
	if !ok || receiverID == "" {
		return
	}
	response := map[string]interface{}{
		"type":        MsgTypeTyping,
		"sender_id":   client.UserID,
		"receiver_id": receiverID,
		"timestamp":   time.Now().Unix(),
	}
	data, _ := json.Marshal(response)
	a.hub.sendToUser(receiverID, data)
}

// handleStopTyping handles stop typing indicators.
func (a *WebSocketAdapter) handleStopTyping(client *Client, msg map[string]interface{}) {
	receiverID, ok := msg["receiver_id"].(string)
	if !ok || receiverID == "" {
		return
	}
	response := map[string]interface{}{
		"type":        MsgTypeStopTyping,
		"sender_id":   client.UserID,
		"receiver_id": receiverID,
		"timestamp":   time.Now().Unix(),
	}
	data, _ := json.Marshal(response)
	a.hub.sendToUser(receiverID, data)
}

// handleReadReceipt handles read receipts.
func (a *WebSocketAdapter) handleReadReceipt(client *Client, msg map[string]interface{}) {
	messageID, ok := msg["message_id"].(string)
	if !ok || messageID == "" {
		return
	}
	senderID, ok := msg["sender_id"].(string)
	if !ok || senderID == "" {
		return
	}
	response := map[string]interface{}{
		"type":       MsgTypeReadReceipt,
		"message_id": messageID,
		"reader_id":  client.UserID,
		"timestamp":  time.Now().Unix(),
	}
	data, _ := json.Marshal(response)
	a.hub.sendToUser(senderID, data)
}

// ======================================================================
= Send Helpers
// ======================================================================

// sendToClient sends a message to a specific client.
func (a *WebSocketAdapter) sendToClient(client *Client, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		a.log.WithError(err).Error("Failed to marshal message")
		return
	}
	select {
	case client.Send <- bytes:
	default:
		a.log.WithField("user_id", client.UserID).Warn("Client send channel full")
	}
}

// sendError sends an error message to a client.
func (a *WebSocketAdapter) sendError(client *Client, message string) {
	response := map[string]interface{}{
		"type":      MsgTypeError,
		"error":     message,
		"timestamp": time.Now().Unix(),
	}
	a.sendToClient(client, response)
}

// ======================================================================
= Broadcast Methods (Public)
// ======================================================================

// BroadcastToUser broadcasts a message to a specific user.
func (a *WebSocketAdapter) BroadcastToUser(userID string, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		a.log.WithError(err).Error("Failed to marshal broadcast message")
		return
	}
	a.hub.UserBroadcast <- UserMessage{
		UserID: userID,
		Data:   bytes,
	}
}

// BroadcastToRoom broadcasts a message to a room.
func (a *WebSocketAdapter) BroadcastToRoom(roomID string, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		a.log.WithError(err).Error("Failed to marshal broadcast message")
		return
	}
	a.hub.RoomBroadcast <- RoomMessage{
		RoomID: roomID,
		Data:   bytes,
	}
}

// BroadcastToAll broadcasts a message to all connected clients.
func (a *WebSocketAdapter) BroadcastToAll(data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		a.log.WithError(err).Error("Failed to marshal broadcast message")
		return
	}
	a.hub.Broadcast <- bytes
}

// ======================================================================
= Notification Helpers
// ======================================================================

// SendNotification sends a notification to a user.
func (a *WebSocketAdapter) SendNotification(userID string, notification interface{}) {
	data := map[string]interface{}{
		"type":      MsgTypeNotification,
		"notification": notification,
		"timestamp": time.Now().Unix(),
	}
	a.BroadcastToUser(userID, data)
}

// SendNewTweet sends a new tweet notification.
func (a *WebSocketAdapter) SendNewTweet(userID string, tweet interface{}) {
	data := map[string]interface{}{
		"type":      MsgTypeNewTweet,
		"tweet":     tweet,
		"timestamp": time.Now().Unix(),
	}
	a.BroadcastToUser(userID, data)
}

// SendLike sends a like notification.
func (a *WebSocketAdapter) SendLike(userID string, tweetID string, liked bool) {
	data := map[string]interface{}{
		"type":      MsgTypeLike,
		"tweet_id":  tweetID,
		"liked":     liked,
		"timestamp": time.Now().Unix(),
	}
	a.BroadcastToUser(userID, data)
}

// SendRetweet sends a retweet notification.
func (a *WebSocketAdapter) SendRetweet(userID string, tweetID string, retweeted bool) {
	data := map[string]interface{}{
		"type":       MsgTypeRetweet,
		"tweet_id":   tweetID,
		"retweeted":  retweeted,
		"timestamp":  time.Now().Unix(),
	}
	a.BroadcastToUser(userID, data)
}

// SendFollow sends a follow notification.
func (a *WebSocketAdapter) SendFollow(userID string, followerID string, followed bool) {
	data := map[string]interface{}{
		"type":       MsgTypeFollow,
		"follower_id": followerID,
		"followed":   followed,
		"timestamp":  time.Now().Unix(),
	}
	a.BroadcastToUser(userID, data)
}

// ======================================================================
= Status and Health
// ======================================================================

// GetOnlineUsers returns all online user IDs.
func (a *WebSocketAdapter) GetOnlineUsers() []string {
	return a.hub.GetOnlineUsers()
}

// GetOnlineCount returns the number of online users.
func (a *WebSocketAdapter) GetOnlineCount() int {
	return a.hub.GetClientCount()
}

// IsUserOnline checks if a user is online.
func (a *WebSocketAdapter) IsUserOnline(userID string) bool {
	return a.hub.IsUserOnline(userID)
}

// GetHub returns the underlying hub.
func (a *WebSocketAdapter) GetHub() *WebSocketHub {
	return a.hub
}

// HealthCheck checks the health of the WebSocket adapter.
func (a *WebSocketAdapter) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"component":    "websocket_adapter",
		"status":       "ok",
		"online_users": a.GetOnlineCount(),
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	stats := a.hub.GetStats()
	status["stats"] = stats

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ======================================================================
= Close and Cleanup
// ======================================================================

// Close closes the WebSocket adapter and hub.
func (a *WebSocketAdapter) Close() {
	a.hub.Stop()
	a.log.Info("WebSocket adapter closed")
}

// ======================================================================
= Helper Functions
// ======================================================================

// generateID generates a unique ID for a client.
func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%100000)
}