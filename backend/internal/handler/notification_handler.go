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
	wsHub               interface{} // WebSocket hub for real-time notifications
	log                 *logrus.Entry
}

// NewNotificationHandler creates a new notification handler.
func NewNotificationHandler(
	notificationService service.NotificationService,
	wsHub interface{},
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
// @Param type query string false "Filter by notification type (like, retweet, follow, reply, mention)"
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
	notificationType := r.URL.Query().Get("type")

	var notifications []*entities.Notification
	var nextCursor string
	var total int64

	if unreadOnly {
		notifications, nextCursor, total, err = h.notificationService.GetUnreadByUserID(r.Context(), userID, cursor, limit)
	} else if notificationType != "" {
		notifications, nextCursor, total, err = h.notificationService.GetByUserIDAndType(r.Context(), userID, notificationType, cursor, limit)
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
		// Get from user info
		fromUser, _ := h.userService.GetUserByID(r.Context(), n.FromUserID)
		responses = append(responses, &dto.NotificationResponse{
			ID:           n.ID,
			Type:         n.Type,
			ReferenceID:  n.ReferenceID,
			Read:         n.Read,
			FromUserID:   n.FromUserID,
			FromUsername: func() string {
				if fromUser != nil {
					return fromUser.Username
				}
				return ""
			}(),
			FromAvatarURL: func() string {
				if fromUser != nil {
					return fromUser.AvatarURL
				}
				return ""
			}(),
			CreatedAt: n.CreatedAt,
		})
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
= Get Unread Count
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

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"unread_count": count,
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

	// Convert to response
	responses := make([]*dto.GroupedNotificationResponse, 0, len(groups))
	for _, group := range groups {
		fromUser, _ := h.userService.GetUserByID(r.Context(), group.FromUserID)
		responses = append(responses, &dto.GroupedNotificationResponse{
			Type:          group.Type,
			ReferenceID:   group.ReferenceID,
			Count:         group.Count,
			LatestAt:      group.LatestAt,
			FromUserID:    group.FromUserID,
			FromUsername: func() string {
				if fromUser != nil {
					return fromUser.Username
				}
				return ""
			}(),
			FromAvatarURL: func() string {
				if fromUser != nil {
					return fromUser.AvatarURL
				}
				return ""
			}(),
			IsRead: group.IsRead,
		})
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
= Get Notification Stats
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
= Admin Endpoints
// ======================================================================

// AdminListNotifications handles admin listing of all notifications.
// @Summary Admin list notifications
// @Description Lists all notifications for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param user_id query string false "Filter by user ID"
// @Param type query string false "Filter by notification type"
// @Param status query string false "Filter by status (read, unread)"
// @Success 200 {object} dto.NotificationAdminListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/notifications [get]
func (h *NotificationHandler) AdminListNotifications(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	userID := r.URL.Query().Get("user_id")
	notificationType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")

	notifications, nextCursor, total, err := h.notificationService.AdminListNotifications(r.Context(), cursor, limit, userID, notificationType, status)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list notifications")
		return
	}

	// Build response
	responses := make([]*dto.NotificationAdminResponse, 0, len(notifications))
	for _, n := range notifications {
		user, _ := h.userService.GetUserByID(r.Context(), n.UserID)
		fromUser, _ := h.userService.GetUserByID(r.Context(), n.FromUserID)
		responses = append(responses, &dto.NotificationAdminResponse{
			ID:           n.ID,
			Type:         n.Type,
			ReferenceID:  n.ReferenceID,
			Read:         n.Read,
			UserID:       n.UserID,
			FromUserID:   n.FromUserID,
			Username: func() string {
				if user != nil {
					return user.Username
				}
				return ""
			}(),
			FromUsername: func() string {
				if fromUser != nil {
					return fromUser.Username
				}
				return ""
			}(),
			CreatedAt: n.CreatedAt,
			ReadAt:    n.ReadAt,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        responses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeleteNotification handles admin deletion of a notification.
// @Summary Admin delete notification
// @Description Deletes a notification (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/notifications/{id} [delete]
func (h *NotificationHandler) AdminDeleteNotification(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	notificationID := vars["id"]
	if notificationID == "" {
		h.sendError(w, http.StatusBadRequest, "Notification ID required", nil)
		return
	}

	if err := h.notificationService.AdminDeleteNotification(r.Context(), notificationID); err != nil {
		h.handleServiceError(w, err, "Failed to delete notification")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Notification deleted successfully",
	})
}

// AdminGetNotificationStats handles retrieving global notification statistics.
// @Summary Admin get notification stats
// @Description Retrieves global notification statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
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

	stats, err := h.notificationService.AdminGetNotificationStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get notification stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Real-time Notification Broadcast (Admin)
// ======================================================================

// AdminBroadcastNotification handles broadcasting a notification to a user.
// @Summary Admin broadcast notification
// @Description Broadcasts a notification to a specific user in real-time (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.BroadcastNotificationRequest true "Broadcast details"
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

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Broadcast via WebSocket if hub is available
	if h.wsHub != nil {
		// This would call a method on the WebSocket hub
		// For now, we just log and return success
		h.log.WithFields(logrus.Fields{
			"user_id": req.UserID,
			"message": req.Message,
		}).Info("Admin broadcast notification")
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Notification broadcast sent",
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
	case errors.Is(err, service.ErrInvalidUserID):
		h.sendError(w, http.StatusBadRequest, "Invalid user ID", nil)
	case errors.Is(err, service.ErrNotificationSettingsNotFound):
		h.sendError(w, http.StatusNotFound, "Notification settings not found", nil)
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