// backend/internal/handler/dm_handler.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/pkg/logger"
)

// DMHandler handles all direct message-related HTTP endpoints.
type DMHandler struct {
	dmService service.DMService
	wsHub     *WebSocketHub
	log       *logrus.Entry
}

// NewDMHandler creates a new DM handler.
func NewDMHandler(
	dmService service.DMService,
	wsHub *WebSocketHub,
) *DMHandler {
	return &DMHandler{
		dmService: dmService,
		wsHub:     wsHub,
		log:       logger.WithField("handler", "dm"),
	}
}

// ======================================================================
// Send Message
// ======================================================================

// SendMessage handles sending a direct message.
// @Summary Send direct message
// @Description Sends a direct message to another user
// @Tags direct_messages
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.SendMessageRequest true "Message details"
// @Success 201 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/send [post]
func (h *DMHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Inject user agent and IP
	req.UserAgent = r.UserAgent()
	req.IP = r.RemoteAddr

	msg, err := h.dmService.SendMessage(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to send message")
		return
	}

	h.sendSuccess(w, http.StatusCreated, msg)
}

// ======================================================================
= Get Conversation
// ======================================================================

// GetConversation handles retrieving messages between two users.
// @Summary Get conversation
// @Description Retrieves messages between the authenticated user and another user
// @Tags direct_messages
// @Security BearerAuth
// @Produce json
// @Param other_user_id path string true "Other user ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.MessageListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/conversation/{other_user_id} [get]
func (h *DMHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	otherUserID := vars["other_user_id"]
	if otherUserID == "" {
		h.sendError(w, http.StatusBadRequest, "Other user ID is required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	req := &dto.GetConversationRequest{
		OtherUserID: otherUserID,
		Cursor:      cursor,
		Limit:       limit,
	}

	messages, err := h.dmService.GetConversation(r.Context(), userID, req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get conversation")
		return
	}

	h.sendSuccess(w, http.StatusOK, messages)
}

// ======================================================================
= Get Conversations List
// ======================================================================

// GetConversations handles retrieving all conversations for a user.
// @Summary Get conversations list
// @Description Retrieves all conversations for the authenticated user
// @Tags direct_messages
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ConversationListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/conversations [get]
func (h *DMHandler) GetConversations(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	conversations, err := h.dmService.GetConversations(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get conversations")
		return
	}

	h.sendSuccess(w, http.StatusOK, conversations)
}

// ======================================================================
= Mark as Read
// ======================================================================

// MarkAsRead handles marking a message or conversation as read.
// @Summary Mark as read
// @Description Marks a specific message or all messages in a conversation as read
// @Tags direct_messages
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.MarkReadRequest true "Mark read details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/mark-read [post]
func (h *DMHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.MarkReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.dmService.MarkAsRead(r.Context(), userID, &req); err != nil {
		h.handleServiceError(w, err, "Failed to mark as read")
		return
	}

	// Get updated unread count
	unreadCount, _ := h.dmService.GetUnreadCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "Marked as read",
		"unread_count": unreadCount,
	})
}

// ======================================================================
= Mark Conversation as Read
// ======================================================================

// MarkConversationAsRead handles marking all messages in a conversation as read.
// @Summary Mark conversation as read
// @Description Marks all messages in a conversation as read
// @Tags direct_messages
// @Security BearerAuth
// @Param other_user_id path string true "Other user ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/conversation/{other_user_id}/read [post]
func (h *DMHandler) MarkConversationAsRead(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	otherUserID := vars["other_user_id"]
	if otherUserID == "" {
		h.sendError(w, http.StatusBadRequest, "Other user ID is required", nil)
		return
	}

	if err := h.dmService.MarkConversationAsRead(r.Context(), userID, otherUserID); err != nil {
		h.handleServiceError(w, err, "Failed to mark conversation as read")
		return
	}

	// Get updated unread count
	unreadCount, _ := h.dmService.GetUnreadCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "Conversation marked as read",
		"unread_count": unreadCount,
	})
}

// ======================================================================
= Mark All as Read
// ======================================================================

// MarkAllAsRead handles marking all messages as read.
// @Summary Mark all messages as read
// @Description Marks all messages for the authenticated user as read
// @Tags direct_messages
// @Security BearerAuth
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/mark-all-read [post]
func (h *DMHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.dmService.MarkAllAsRead(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to mark all as read")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "All messages marked as read",
	})
}

// ======================================================================
= Delete Message
// ======================================================================

// DeleteMessage handles deleting a message.
// @Summary Delete message
// @Description Deletes a specific message
// @Tags direct_messages
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.DeleteMessageRequest true "Delete details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/delete [post]
func (h *DMHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.DeleteMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.dmService.DeleteMessage(r.Context(), userID, &req); err != nil {
		h.handleServiceError(w, err, "Failed to delete message")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Message deleted",
	})
}

// ======================================================================
= Delete Conversation
// ======================================================================

// DeleteConversation handles deleting an entire conversation.
// @Summary Delete conversation
// @Description Deletes all messages in a conversation
// @Tags direct_messages
// @Security BearerAuth
// @Param other_user_id path string true "Other user ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/conversation/{other_user_id} [delete]
func (h *DMHandler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	otherUserID := vars["other_user_id"]
	if otherUserID == "" {
		h.sendError(w, http.StatusBadRequest, "Other user ID is required", nil)
		return
	}

	if err := h.dmService.DeleteConversation(r.Context(), userID, otherUserID); err != nil {
		h.handleServiceError(w, err, "Failed to delete conversation")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Conversation deleted",
	})
}

// ======================================================================
= Search Messages
// ======================================================================

// SearchMessages handles searching messages.
// @Summary Search messages
// @Description Searches through the user's direct messages
// @Tags direct_messages
// @Security BearerAuth
// @Produce json
// @Param q query string true "Search query"
// @Param with query string false "Filter by user ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.MessageListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/search [get]
func (h *DMHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		h.sendError(w, http.StatusBadRequest, "Search query is required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	withUser := r.URL.Query().Get("with")

	req := &dto.SearchMessagesRequest{
		Query:  query,
		With:   withUser,
		Cursor: cursor,
		Limit:  limit,
	}

	messages, err := h.dmService.SearchMessages(r.Context(), userID, req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search messages")
		return
	}

	h.sendSuccess(w, http.StatusOK, messages)
}

// ======================================================================
= Get Unread Count
// ======================================================================

// GetUnreadCount handles retrieving the total unread message count.
// @Summary Get unread message count
// @Description Retrieves the total number of unread messages for the authenticated user
// @Tags direct_messages
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UnreadCountResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/unread-count [get]
func (h *DMHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	count, err := h.dmService.GetUnreadCount(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get unread count")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"unread_count": count,
	})
}

// ======================================================================
= Get Unread Count From User
// ======================================================================

// GetUnreadFromUser handles retrieving unread messages from a specific user.
// @Summary Get unread messages from user
// @Description Retrieves the number of unread messages from a specific user
// @Tags direct_messages
// @Security BearerAuth
// @Param other_user_id path string true "Other user ID"
// @Success 200 {object} dto.UnreadCountResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/unread-from/{other_user_id} [get]
func (h *DMHandler) GetUnreadFromUser(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	otherUserID := vars["other_user_id"]
	if otherUserID == "" {
		h.sendError(w, http.StatusBadRequest, "Other user ID is required", nil)
		return
	}

	count, err := h.dmService.GetUnreadFromUser(r.Context(), userID, otherUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get unread count")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"user_id":       otherUserID,
		"unread_count":  count,
	})
}

// ======================================================================
= Get Message Stats
// ======================================================================

// GetMessageStats handles retrieving message statistics for the user.
// @Summary Get message stats
// @Description Retrieves message statistics for the authenticated user
// @Tags direct_messages
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.MessageStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/stats [get]
func (h *DMHandler) GetMessageStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	stats, err := h.dmService.GetMessageStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get message stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Send Typing Indicator
// ======================================================================

// SendTypingIndicator handles sending a typing indicator to another user.
// @Summary Send typing indicator
// @Description Sends a typing indicator to another user
// @Tags direct_messages
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.TypingIndicatorRequest true "Typing indicator details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/typing [post]
func (h *DMHandler) SendTypingIndicator(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.TypingIndicatorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if req.RecipientID == "" {
		h.sendError(w, http.StatusBadRequest, "Recipient ID is required", nil)
		return
	}

	// Send via WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToUser(req.RecipientID, map[string]interface{}{
			"type":        "typing",
			"sender_id":   userID,
			"is_typing":   req.IsTyping,
			"timestamp":   time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Typing indicator sent",
	})
}

// ======================================================================
= Get Online Status
// ======================================================================

// GetOnlineStatus handles checking if a user is online.
// @Summary Get online status
// @Description Checks if a user is currently online
// @Tags direct_messages
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Success 200 {object} dto.OnlineStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/dm/online/{user_id} [get]
func (h *DMHandler) GetOnlineStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetUserID := vars["user_id"]
	if targetUserID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID is required", nil)
		return
	}

	isOnline := false
	if h.wsHub != nil {
		onlineUsers := h.wsHub.GetOnlineUsers()
		for _, uid := range onlineUsers {
			if uid == targetUserID {
				isOnline = true
				break
			}
		}
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"user_id":   targetUserID,
		"is_online": isOnline,
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *DMHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *DMHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := dto.ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
		Details: details,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.WithError(err).Error("Failed to encode error response")
	}
}

// sendValidationError handles validation errors.
func (h *DMHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *DMHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrDMSenderNotFound):
		h.sendError(w, http.StatusNotFound, "Sender not found", nil)
	case errors.Is(err, service.ErrDMReceiverNotFound):
		h.sendError(w, http.StatusNotFound, "Receiver not found", nil)
	case errors.Is(err, service.ErrDMUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrDMMessageNotFound):
		h.sendError(w, http.StatusNotFound, "Message not found", nil)
	case errors.Is(err, service.ErrDMPermissionDenied):
		h.sendError(w, http.StatusForbidden, "Permission denied", nil)
	case errors.Is(err, service.ErrDMSelfMessage):
		h.sendError(w, http.StatusBadRequest, "Cannot send message to yourself", nil)
	case errors.Is(err, service.ErrDMInvalidContent):
		h.sendError(w, http.StatusBadRequest, "Invalid message content", nil)
	case errors.Is(err, service.ErrDMContentTooLong):
		h.sendError(w, http.StatusBadRequest, "Message content too long", nil)
	case errors.Is(err, service.ErrDMMediaTooMany):
		h.sendError(w, http.StatusBadRequest, "Too many media files", nil)
	case errors.Is(err, service.ErrDMInvalidMedia):
		h.sendError(w, http.StatusBadRequest, "Invalid media URL", nil)
	case errors.Is(err, service.ErrDMConversationEmpty):
		h.sendError(w, http.StatusNotFound, "Conversation is empty", nil)
	case errors.Is(err, service.ErrDMSearchQueryEmpty):
		h.sendError(w, http.StatusBadRequest, "Search query cannot be empty", nil)
	case errors.Is(err, context.Canceled):
		h.sendError(w, http.StatusRequestTimeout, "Request cancelled", nil)
	case errors.Is(err, context.DeadlineExceeded):
		h.sendError(w, http.StatusGatewayTimeout, "Request timed out", nil)
	default:
		h.log.WithError(err).Error(defaultMsg)
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck returns the health status of the DM handler.
func (h *DMHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "dm_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}