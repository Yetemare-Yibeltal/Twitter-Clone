// backend/internal/handler/notification_handler.go
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

// NotificationHandler handles all notification-related HTTP endpoints.
type NotificationHandler struct {
	notificationService service.NotificationService
	wsHub               *WebSocketHub
	log                 *logrus.Entry
}

// NewNotificationHandler creates a new notification handler.
func NewNotificationHandler(
	notificationService service.NotificationService,
	wsHub *WebSocketHub,
) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		wsHub:               wsHub,
		log:                 logger.WithField("handler", "notification"),
	}
}

// ======================================================================
// Get Notifications
// ======================================================================

// GetNotifications handles retrieving notifications for the authenticated user.
// @Summary Get notifications
// @Description Retrieves notifications for the authenticated user with pagination
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param unread_only query bool false "Only return unread notifications"
// @Param type query string false "Filter by notification type (like, retweet, follow, reply, mention, quote)"
// @Success 200 {object} dto.NotificationListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications [get]
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	unreadOnly, _ := strconv.ParseBool(r.URL.Query().Get("unread_only"))
	filterType := r.URL.Query().Get("type")

	var notifications []*entities.Notification
	var nextCursor string
	var total int64

	if unreadOnly {
		notifications, nextCursor, total, err = h.notificationService.GetUnreadByUserID(r.Context(), userID, cursor, limit)
	} else if filterType != "" {
		notifications, nextCursor, total, err = h.notificationService.GetByUserIDAndType(r.Context(), userID, filterType, cursor, limit)
	} else {
		notifications, nextCursor, total, err = h.notificationService.GetByUserID(r.Context(), userID, cursor, limit)
	}

	if err != nil {
		h.handleServiceError(w, err, "Failed to get notifications")
		return
	}

	// Convert to response DTOs
	responses := make([]*dto.NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		responses = append(responses, dto.ToNotificationResponse(n))
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        responses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
// Get Unread Count
// ======================================================================

// GetUnreadCount handles retrieving the number of unread notifications.
// @Summary Get unread notification count
// @Description Retrieves the count of unread notifications for the authenticated user
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UnreadCountResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/unread-count [get]
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	count, err := h.notificationService.GetUnreadCount(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get unread count")
		return
	}

	// Get breakdown by type for the user
	typeBreakdown, _ := h.notificationService.GetUnreadBreakdown(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"unread_count":   count,
		"type_breakdown": typeBreakdown,
	})
}

// ======================================================================
= Mark as Read
// ======================================================================

// MarkAsRead handles marking a notification as read.
// @Summary Mark notification as read
// @Description Marks a specific notification as read
// @Tags notifications
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/{id}/read [post]
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	notificationID := vars["id"]
	if notificationID == "" {
		h.sendError(w, http.StatusBadRequest, "Notification ID required", nil)
		return
	}

	if err := h.notificationService.MarkAsRead(r.Context(), userID, notificationID); err != nil {
		h.handleServiceError(w, err, "Failed to mark as read")
		return
	}

	// Get updated unread count
	unreadCount, _ := h.notificationService.GetUnreadCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "Notification marked as read",
		"unread_count": unreadCount,
	})
}

// ======================================================================
= Mark All as Read
// ======================================================================

// MarkAllAsRead handles marking all notifications as read.
// @Summary Mark all notifications as read
// @Description Marks all notifications for the authenticated user as read
// @Tags notifications
// @Security BearerAuth
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/read-all [post]
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.notificationService.MarkAllAsRead(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to mark all as read")
		return
	}

	// Clear WebSocket unread notification count for the user
	if h.wsHub != nil {
		h.wsHub.BroadcastToUser(userID, map[string]interface{}{
			"type":         "unread_cleared",
			"timestamp":    time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "All notifications marked as read",
	})
}

// ======================================================================
= Mark Multiple as Read
// ======================================================================

// MarkMultipleAsRead handles marking multiple notifications as read.
// @Summary Mark multiple notifications as read
// @Description Marks multiple notifications as read
// @Tags notifications
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.MarkReadRequest true "Notification IDs"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/read-multiple [post]
func (h *NotificationHandler) MarkMultipleAsRead(w http.ResponseWriter, r *http.Request) {
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

	if err := h.notificationService.MarkMultipleAsRead(r.Context(), userID, req.IDs); err != nil {
		h.handleServiceError(w, err, "Failed to mark notifications as read")
		return
	}

	// Get updated unread count
	unreadCount, _ := h.notificationService.GetUnreadCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "Notifications marked as read",
		"unread_count": unreadCount,
	})
}

// ======================================================================
= Delete Notification
// ======================================================================

// DeleteNotification handles deleting a notification.
// @Summary Delete notification
// @Description Deletes a specific notification
// @Tags notifications
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/{id} [delete]
func (h *NotificationHandler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	notificationID := vars["id"]
	if notificationID == "" {
		h.sendError(w, http.StatusBadRequest, "Notification ID required", nil)
		return
	}

	if err := h.notificationService.Delete(r.Context(), userID, notificationID); err != nil {
		h.handleServiceError(w, err, "Failed to delete notification")
		return
	}

	// Get updated unread count
	unreadCount, _ := h.notificationService.GetUnreadCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "Notification deleted",
		"unread_count": unreadCount,
	})
}

// ======================================================================
= Delete All Notifications
// ======================================================================

// DeleteAllNotifications handles deleting all notifications.
// @Summary Delete all notifications
// @Description Deletes all notifications for the authenticated user
// @Tags notifications
// @Security BearerAuth
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/delete-all [delete]
func (h *NotificationHandler) DeleteAllNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.notificationService.DeleteAll(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to delete all notifications")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "All notifications deleted",
	})
}

// ======================================================================
= Get Grouped Notifications
// ======================================================================

// GetGroupedNotifications handles retrieving grouped notifications.
// @Summary Get grouped notifications
// @Description Retrieves notifications grouped by type and reference
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.GroupedNotificationListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/grouped [get]
func (h *NotificationHandler) GetGroupedNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	groups, nextCursor, total, err := h.notificationService.GetGroupedNotifications(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get grouped notifications")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        groups,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Notification Stats (User)
// ======================================================================

// GetNotificationStats handles retrieving notification statistics for the user.
// @Summary Get notification stats
// @Description Retrieves notification statistics for the authenticated user
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UserNotificationStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/stats [get]
func (h *NotificationHandler) GetNotificationStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	stats, err := h.notificationService.GetUserNotificationStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get notification stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Get Notification Settings
// ======================================================================

// GetNotificationSettings handles retrieving notification settings.
// @Summary Get notification settings
// @Description Retrieves the user's notification settings
// @Tags notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.NotificationSettingsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/settings [get]
func (h *NotificationHandler) GetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	settings, err := h.notificationService.GetNotificationSettings(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get notification settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Update Notification Settings
// ======================================================================

// UpdateNotificationSettings handles updating notification settings.
// @Summary Update notification settings
// @Description Updates the user's notification settings
// @Tags notifications
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdateNotificationSettingsRequest true "Notification settings"
// @Success 200 {object} dto.NotificationSettingsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/notifications/settings [put]
func (h *NotificationHandler) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdateNotificationSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	settings, err := h.notificationService.UpdateNotificationSettings(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update notification settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Get Admin Notification Stats
// ======================================================================

// AdminGetNotificationStats handles retrieving global notification statistics.
// @Summary Admin get global notification stats
// @Description Retrieves global notification statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.GlobalNotificationStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/notifications/stats [get]
func (h *NotificationHandler) AdminGetNotificationStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	stats, err := h.notificationService.AdminGetNotificationStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get global notification stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Admin Broadcast Notification (WebSocket)
// ======================================================================

// AdminBroadcastNotification handles broadcasting a notification via WebSocket.
// @Summary Admin broadcast notification
// @Description Sends a notification to a user via WebSocket (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.BroadcastNotificationRequest true "Broadcast data"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/notifications/broadcast [post]
func (h *NotificationHandler) AdminBroadcastNotification(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	var req dto.BroadcastNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if req.UserID == "" || req.Message == "" {
		h.sendError(w, http.StatusBadRequest, "User ID and message are required", nil)
		return
	}

	// Create notification record
	notification := &entities.Notification{
		ID:         uuid.New().String(),
		UserID:     req.UserID,
		FromUserID: "system",
		Type:       "admin",
		Message:    req.Message,
		Data:       req.Data,
		Read:       false,
		CreatedAt:  time.Now(),
	}

	if err := h.notificationService.Create(r.Context(), notification); err != nil {
		h.handleServiceError(w, err, "Failed to create notification")
		return
	}

	// Send via WebSocket
	if h.wsHub != nil {
		h.wsHub.SendNotification(req.UserID, notification)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Notification broadcast sent",
	})
}

// ======================================================================
= Admin Get Notification Analytics
// ======================================================================

// AdminGetNotificationAnalytics handles retrieving notification analytics.
// @Summary Admin get notification analytics
// @Description Retrieves notification analytics for moderation insights (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.NotificationAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/notifications/analytics [get]
func (h *NotificationHandler) AdminGetNotificationAnalytics(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	analytics, err := h.notificationService.AdminGetNotificationAnalytics(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get notification analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, analytics)
}

// ======================================================================
= Admin Manage Notification Templates
// ======================================================================

// AdminListNotificationTemplates handles listing notification templates.
// @Summary Admin list notification templates
// @Description Lists all notification templates (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.NotificationTemplateListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/notifications/templates [get]
func (h *NotificationHandler) AdminListNotificationTemplates(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	templates, err := h.notificationService.AdminListNotificationTemplates(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to list notification templates")
		return
	}

	h.sendSuccess(w, http.StatusOK, templates)
}

// AdminUpdateNotificationTemplate handles updating a notification template.
// @Summary Admin update notification template
// @Description Updates a notification template (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Template ID"
// @Param request body dto.UpdateNotificationTemplateRequest true "Template details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/notifications/templates/{id} [put]
func (h *NotificationHandler) AdminUpdateNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	templateID := vars["id"]
	if templateID == "" {
		h.sendError(w, http.StatusBadRequest, "Template ID required", nil)
		return
	}

	var req dto.UpdateNotificationTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.notificationService.AdminUpdateNotificationTemplate(r.Context(), templateID, &req); err != nil {
		h.handleServiceError(w, err, "Failed to update notification template")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Notification template updated successfully",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *NotificationHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *NotificationHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *NotificationHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *NotificationHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrNotificationNotFound):
		h.sendError(w, http.StatusNotFound, "Notification not found", nil)
	case errors.Is(err, service.ErrNotificationAlreadyRead):
		h.sendError(w, http.StatusBadRequest, "Notification already read", nil)
	case errors.Is(err, service.ErrNotificationUserMismatch):
		h.sendError(w, http.StatusForbidden, "Notification does not belong to user", nil)
	case errors.Is(err, service.ErrInvalidNotificationType):
		h.sendError(w, http.StatusBadRequest, "Invalid notification type", nil)
	case errors.Is(err, service.ErrNotificationLimitExceeded):
		h.sendError(w, http.StatusBadRequest, "Notification limit exceeded", nil)
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

// HealthCheck returns the health status of the notification handler.
func (h *NotificationHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "notification_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}