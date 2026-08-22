// backend/internal/handler/websocket_handler.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/config"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/repository/interfaces"
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
	MsgTypePing          = "ping"
	MsgTypePong          = "pong"
	MsgTypeNewMessage    = "new_message"
	MsgTypeTyping        = "typing"
	MsgTypeStopTyping    = "stop_typing"
	MsgTypeReadReceipt   = "read_receipt"
	MsgTypeNotification  = "notification"
	MsgTypeNewTweet      = "new_tweet"
	MsgTypeLike          = "like"
	MsgTypeRetweet       = "retweet"
	MsgTypeFollow        = "follow"
	MsgTypeError         = "error"
	MsgTypeAck           = "ack"
	MsgTypeConversation  = "conversation"
	MsgTypeUserStatus    = "user_status"
	MsgTypeOnline        = "online"
	MsgTypeOffline       = "offline"
)

// ======================================================================
= WebSocket Hub
// ======================================================================

// Client represents a WebSocket connection.
type Client struct {
	ID         string
	UserID     string
	Conn       *websocket.Conn
	Hub        *Hub
	Send       chan []byte
	Rooms      map[string]bool
	CreatedAt  time.Time
	LastPing   time.Time
	mu         sync.RWMutex
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	// Registered clients
	Clients map[string]*Client // userID -> client (only one connection per user)
	
	// Register requests from clients
	Register chan *Client
	
	// Unregister requests from clients
	Unregister chan *Client
	
	// Broadcast messages to all clients
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
	
	// Wait group for goroutines
	wg sync.WaitGroup
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
= Hub Creation and Management
// ======================================================================

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		Clients:       make(map[string]*Client),
		Register:      make(chan *Client),
		Unregister:    make(chan *Client),
		Broadcast:     make(chan []byte, 256),
		UserBroadcast: make(chan UserMessage, 256),
		RoomBroadcast: make(chan RoomMessage, 256),
		log:           logger.WithField("component", "websocket_hub"),
		stop:          make(chan struct{}),
	}
}

// Run starts the hub main loop.
func (h *Hub) Run() {
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

// registerClient registers a new client.
func (h *Hub) registerClient(client *Client) {
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
	}
	
	h.Clients[client.UserID] = client
	h.log.WithFields(logrus.Fields{
		"user_id":   client.UserID,
		"client_id": client.ID,
		"total":     len(h.Clients),
	}).Info("Client registered")
}

// unregisterClient unregisters a client.
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if existing, exists := h.Clients[client.UserID]; exists && existing.ID == client.ID {
		delete(h.Clients, client.UserID)
		close(client.Send)
		h.log.WithFields(logrus.Fields{
			"user_id":   client.UserID,
			"client_id": client.ID,
			"total":     len(h.Clients),
		}).Info("Client unregistered")
	}
}

// broadcastMessage broadcasts a message to all clients.
func (h *Hub) broadcastMessage(message []byte) {
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
func (h *Hub) sendToUser(userID string, data []byte) {
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
func (h *Hub) sendToRoom(roomID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	for _, client := range h.Clients {
		if client.Rooms[roomID] {
			select {
			case client.Send <- data:
			default:
				h.log.WithFields(logrus.Fields{
					"user_id": client.UserID,
					"room_id": roomID,
				}).Warn("Client send channel full")
			}
		}
	}
}

// ======================================================================
= Client Cleanup
// ======================================================================

// cleanupInactiveClients removes clients with stale connections.
func (h *Hub) cleanupInactiveClients() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	now := time.Now()
	for userID, client := range h.Clients {
		if now.Sub(client.LastPing) > 2*PongWait {
			h.log.WithFields(logrus.Fields{
				"user_id":   userID,
				"client_id": client.ID,
				"last_ping": client.LastPing,
			}).Warn("Client inactive, removing")
			close(client.Send)
			delete(h.Clients, userID)
		}
	}
}

// ======================================================================
= Hub Shutdown
// ======================================================================

// shutdown gracefully shuts down the hub.
func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	for userID, client := range h.Clients {
		close(client.Send)
		delete(h.Clients, userID)
	}
	h.log.Info("Hub shutdown complete")
}

// Stop stops the hub.
func (h *Hub) Stop() {
	close(h.stop)
	h.wg.Wait()
}

// GetClientCount returns the number of connected clients.
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.Clients)
}

// GetOnlineUsers returns a list of online user IDs.
func (h *Hub) GetOnlineUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	users := make([]string, 0, len(h.Clients))
	for userID := range h.Clients {
		users = append(users, userID)
	}
	return users
}

// ======================================================================
= WebSocket Handler
// ======================================================================

// WebSocketHandler handles WebSocket connections.
type WebSocketHandler struct {
	hub          *Hub
	upgrader     websocket.Upgrader
	userRepo     interfaces.UserRepository
	redisAdapter adapter.RedisAdapter
	config       *config.Config
	log          *logrus.Entry
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(
	hub *Hub,
	userRepo interfaces.UserRepository,
	redisAdapter adapter.RedisAdapter,
	cfg *config.Config,
) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins in development, restrict in production
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
		},
		userRepo:     userRepo,
		redisAdapter: redisAdapter,
		config:       cfg,
		log:          logger.WithField("handler", "websocket"),
	}
}

// ======================================================================
= WebSocket Connection Handler
// ======================================================================

// ServeWS handles WebSocket upgrade requests.
func (h *WebSocketHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context (set by auth middleware)
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.log.WithError(err).Warn("Unauthorized WebSocket connection attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	// Verify user exists
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		h.log.WithError(err).WithField("user_id", userID).Warn("User not found")
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	
	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.WithError(err).Error("Failed to upgrade to WebSocket")
		return
	}
	
	// Create client
	client := &Client{
		ID:        uuid.New().String(),
		UserID:    userID,
		Conn:      conn,
		Hub:       h.hub,
		Send:      make(chan []byte, 256),
		Rooms:     make(map[string]bool),
		CreatedAt: time.Now(),
		LastPing:  time.Now(),
	}
	
	// Add user to their personal room
	client.Rooms["user:"+userID] = true
	
	// Register client with hub
	h.hub.Register <- client
	
	// Set read/write deadlines
	conn.SetReadDeadline(time.Now().Add(PongWait))
	conn.SetPongHandler(func(string) error {
		client.mu.Lock()
		client.LastPing = time.Now()
		client.mu.Unlock()
		conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})
	
	// Send online status to all clients
	h.broadcastUserStatus(userID, true)
	
	// Start goroutines
	go h.writePump(client)
	go h.readPump(client)
	
	h.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"username": user.Username,
		"client_id": client.ID,
	}).Info("WebSocket connection established")
}

// ======================================================================
= Read Pump
// ======================================================================

// readPump reads messages from the WebSocket connection.
func (h *WebSocketHandler) readPump(client *Client) {
	defer func() {
		h.hub.Unregister <- client
		client.Conn.Close()
		h.broadcastUserStatus(client.UserID, false)
	}()
	
	client.Conn.SetReadLimit(MaxMessageSize)
	
	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.log.WithError(err).WithField("user_id", client.UserID).Warn("WebSocket read error")
			}
			break
		}
		
		// Process message
		h.handleMessage(client, message)
	}
}

// ======================================================================
= Write Pump
// ======================================================================

// writePump writes messages to the WebSocket connection.
func (h *WebSocketHandler) writePump(client *Client) {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				// The hub closed the channel
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
			client.Conn.SetWriteDeadline(time.Now().Add(WriteWait))
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
func (h *WebSocketHandler) handleMessage(client *Client, data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(client, "Invalid JSON format")
		return
	}
	
	msgType, ok := msg["type"].(string)
	if !ok {
		h.sendError(client, "Message type is required")
		return
	}
	
	switch msgType {
	case MsgTypePing:
		h.handlePing(client)
		
	case MsgTypeTyping:
		h.handleTyping(client, msg)
		
	case MsgTypeStopTyping:
		h.handleStopTyping(client, msg)
		
	case MsgTypeReadReceipt:
		h.handleReadReceipt(client, msg)
		
	case MsgTypeNewMessage:
		h.handleNewMessage(client, msg)
		
	default:
		h.sendError(client, "Unknown message type: "+msgType)
	}
}

// ======================================================================
= Message Handlers
// ======================================================================

// handlePing handles ping messages.
func (h *WebSocketHandler) handlePing(client *Client) {
	client.mu.Lock()
	client.LastPing = time.Now()
	client.mu.Unlock()
	
	response := map[string]interface{}{
		"type": MsgTypePong,
		"timestamp": time.Now().Unix(),
	}
	h.sendToClient(client, response)
}

// handleTyping handles typing indicators.
func (h *WebSocketHandler) handleTyping(client *Client, msg map[string]interface{}) {
	receiverID, ok := msg["receiver_id"].(string)
	if !ok || receiverID == "" {
		return
	}
	
	response := map[string]interface{}{
		"type":       MsgTypeTyping,
		"sender_id":  client.UserID,
		"receiver_id": receiverID,
		"timestamp":  time.Now().Unix(),
	}
	data, _ := json.Marshal(response)
	h.hub.sendToUser(receiverID, data)
}

// handleStopTyping handles stop typing indicators.
func (h *WebSocketHandler) handleStopTyping(client *Client, msg map[string]interface{}) {
	receiverID, ok := msg["receiver_id"].(string)
	if !ok || receiverID == "" {
		return
	}
	
	response := map[string]interface{}{
		"type":       MsgTypeStopTyping,
		"sender_id":  client.UserID,
		"receiver_id": receiverID,
		"timestamp":  time.Now().Unix(),
	}
	data, _ := json.Marshal(response)
	h.hub.sendToUser(receiverID, data)
}

// handleReadReceipt handles read receipts.
func (h *WebSocketHandler) handleReadReceipt(client *Client, msg map[string]interface{}) {
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
	h.hub.sendToUser(senderID, data)
}

// handleNewMessage handles new message events.
func (h *WebSocketHandler) handleNewMessage(client *Client, msg map[string]interface{}) {
	// This is handled by the DM service
	// We just acknowledge receipt
	msgID, _ := msg["message_id"].(string)
	response := map[string]interface{}{
		"type":       MsgTypeAck,
		"message_id": msgID,
		"status":     "delivered",
		"timestamp":  time.Now().Unix(),
	}
	h.sendToClient(client, response)
}

// ======================================================================
= Send Helpers
// ======================================================================

// sendToClient sends a message to a specific client.
func (h *WebSocketHandler) sendToClient(client *Client, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		h.log.WithError(err).Error("Failed to marshal message")
		return
	}
	select {
	case client.Send <- bytes:
	default:
		h.log.WithField("user_id", client.UserID).Warn("Client send channel full")
	}
}

// sendError sends an error message to a client.
func (h *WebSocketHandler) sendError(client *Client, message string) {
	response := map[string]interface{}{
		"type":    MsgTypeError,
		"error":   message,
		"timestamp": time.Now().Unix(),
	}
	h.sendToClient(client, response)
}

// broadcastUserStatus broadcasts user online/offline status.
func (h *WebSocketHandler) broadcastUserStatus(userID string, online bool) {
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
	h.hub.Broadcast <- data
}

// ======================================================================
= Broadcast Methods (Public)
// ======================================================================

// BroadcastToUser broadcasts a message to a specific user.
func (h *WebSocketHandler) BroadcastToUser(userID string, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		h.log.WithError(err).Error("Failed to marshal broadcast message")
		return
	}
	h.hub.UserBroadcast <- UserMessage{
		UserID: userID,
		Data:   bytes,
	}
}

// BroadcastToRoom broadcasts a message to a room.
func (h *WebSocketHandler) BroadcastToRoom(roomID string, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		h.log.WithError(err).Error("Failed to marshal broadcast message")
		return
	}
	h.hub.RoomBroadcast <- RoomMessage{
		RoomID: roomID,
		Data:   bytes,
	}
}

// BroadcastToAll broadcasts a message to all connected clients.
func (h *WebSocketHandler) BroadcastToAll(data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		h.log.WithError(err).Error("Failed to marshal broadcast message")
		return
	}
	h.hub.Broadcast <- bytes
}

// ======================================================================
= Notification Helpers
// ======================================================================

// SendNotification sends a notification to a user.
func (h *WebSocketHandler) SendNotification(userID string, notification *entities.Notification) {
	data := map[string]interface{}{
		"type":         MsgTypeNotification,
		"notification": notification,
		"timestamp":    time.Now().Unix(),
	}
	h.BroadcastToUser(userID, data)
}

// SendNewTweet sends a new tweet notification.
func (h *WebSocketHandler) SendNewTweet(userID string, tweet *dto.TweetResponse) {
	data := map[string]interface{}{
		"type":    MsgTypeNewTweet,
		"tweet":   tweet,
		"timestamp": time.Now().Unix(),
	}
	h.BroadcastToUser(userID, data)
}

// SendLike sends a like notification.
func (h *WebSocketHandler) SendLike(userID string, tweetID string, liked bool) {
	data := map[string]interface{}{
		"type":     MsgTypeLike,
		"tweet_id": tweetID,
		"liked":    liked,
		"timestamp": time.Now().Unix(),
	}
	h.BroadcastToUser(userID, data)
}

// SendRetweet sends a retweet notification.
func (h *WebSocketHandler) SendRetweet(userID string, tweetID string, retweeted bool) {
	data := map[string]interface{}{
		"type":       MsgTypeRetweet,
		"tweet_id":   tweetID,
		"retweeted":  retweeted,
		"timestamp":  time.Now().Unix(),
	}
	h.BroadcastToUser(userID, data)
}

// SendFollow sends a follow notification.
func (h *WebSocketHandler) SendFollow(userID string, followerID string, followed bool) {
	data := map[string]interface{}{
		"type":       MsgTypeFollow,
		"follower_id": followerID,
		"followed":   followed,
		"timestamp":  time.Now().Unix(),
	}
	h.BroadcastToUser(userID, data)
}

// ======================================================================
= Status and Health
// ======================================================================

// GetOnlineUsers returns all online user IDs.
func (h *WebSocketHandler) GetOnlineUsers() []string {
	return h.hub.GetOnlineUsers()
}

// GetOnlineCount returns the number of online users.
func (h *WebSocketHandler) GetOnlineCount() int {
	return h.hub.GetClientCount()
}

// HealthCheck checks the health of the WebSocket handler.
func (h *WebSocketHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"component":    "websocket_handler",
		"status":       "ok",
		"online_users": h.GetOnlineCount(),
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ======================================================================
= Close and Cleanup
// ======================================================================

// Close closes the WebSocket handler and hub.
func (h *WebSocketHandler) Close() {
	h.hub.Stop()
	h.log.Info("WebSocket handler closed")
}