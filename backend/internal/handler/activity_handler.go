// backend/internal/handler/activity_handler.go
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

// ActivityHandler handles all activity-related HTTP endpoints.
type ActivityHandler struct {
	activityService service.ActivityService
	userService     service.UserService
	tweetService    service.TweetService
	followService   service.FollowService
	likeService     service.LikeService
	retweetService  service.RetweetService
	log             *logrus.Entry
}

// NewActivityHandler creates a new activity handler.
func NewActivityHandler(
	activityService service.ActivityService,
	userService service.UserService,
	tweetService service.TweetService,
	followService service.FollowService,
	likeService service.LikeService,
	retweetService service.RetweetService,
) *ActivityHandler {
	return &ActivityHandler{
		activityService: activityService,
		userService:     userService,
		tweetService:    tweetService,
		followService:   followService,
		likeService:     likeService,
		retweetService:  retweetService,
		log:             logger.WithField("handler", "activity"),
	}
}

// ======================================================================
// Get Activity Feed
// ======================================================================

// GetActivityFeed handles retrieving the user's activity feed.
// @Summary Get activity feed
// @Description Retrieves the activity feed for the authenticated user
// @Tags activity
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param activity_types query string false "Comma-separated activity types (tweet, like, retweet, follow, reply)"
// @Success 200 {object} dto.ActivityFeedResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/feed [get]
func (h *ActivityHandler) GetActivityFeed(w http.ResponseWriter, r *http.Request) {
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
	typesParam := r.URL.Query().Get("activity_types")
	var activityTypes []string
	if typesParam != "" {
		activityTypes = strings.Split(typesParam, ",")
	}

	activities, nextCursor, total, err := h.activityService.GetActivityFeed(r.Context(), userID, cursor, limit, activityTypes)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get activity feed")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        activities,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
// Get User Activity
// ======================================================================

// GetUserActivity handles retrieving activity for a specific user.
// @Summary Get user activity
// @Description Retrieves activity for a specific user (public)
// @Tags activity
// @Produce json
// @Param username path string true "Username"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.ActivityFeedResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/user/{username} [get]
func (h *ActivityHandler) GetUserActivity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		h.sendError(w, http.StatusBadRequest, "Username is required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	activities, nextCursor, total, err := h.activityService.GetUserActivity(r.Context(), username, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user activity")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        activities,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
		"username":    username,
	})
}

// ======================================================================
= Get Activity Stats
// ======================================================================

// GetActivityStats handles retrieving activity statistics for the user.
// @Summary Get activity stats
// @Description Retrieves activity statistics for the authenticated user
// @Tags activity
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.ActivityStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/stats [get]
func (h *ActivityHandler) GetActivityStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	stats, err := h.activityService.GetActivityStats(r.Context(), userID, days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get activity stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Get Activity Timeline
// ======================================================================

// GetActivityTimeline handles retrieving activity timeline with filters.
// @Summary Get activity timeline
// @Description Retrieves activity timeline with various filters
// @Tags activity
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param since query string false "Since timestamp"
// @Param until query string false "Until timestamp"
// @Param types query string false "Comma-separated activity types"
// @Success 200 {object} dto.ActivityTimelineResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/timeline [get]
func (h *ActivityHandler) GetActivityTimeline(w http.ResponseWriter, r *http.Request) {
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
	since := r.URL.Query().Get("since")
	until := r.URL.Query().Get("until")
	typesParam := r.URL.Query().Get("types")
	var activityTypes []string
	if typesParam != "" {
		activityTypes = strings.Split(typesParam, ",")
	}

	var sinceTime, untilTime time.Time
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = t
		}
	}
	if until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			untilTime = t
		}
	}

	timeline, err := h.activityService.GetActivityTimeline(r.Context(), userID, cursor, limit, sinceTime, untilTime, activityTypes)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get activity timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, timeline)
}

// ======================================================================
= Get Activity Heatmap
// ======================================================================

// GetActivityHeatmap handles retrieving activity heatmap data.
// @Summary Get activity heatmap
// @Description Retrieves activity heatmap data for the authenticated user
// @Tags activity
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 30, max 90)"
// @Success 200 {object} dto.ActivityHeatmapResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/heatmap [get]
func (h *ActivityHandler) GetActivityHeatmap(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 90 {
		days = 30
	}

	heatmap, err := h.activityService.GetActivityHeatmap(r.Context(), userID, days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get activity heatmap")
		return
	}

	h.sendSuccess(w, http.StatusOK, heatmap)
}

// ======================================================================
= Get Recent Activities
// ======================================================================

// GetRecentActivities handles retrieving the most recent activities.
// @Summary Get recent activities
// @Description Retrieves the most recent activities for the user
// @Tags activity
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Number of activities to return (default 10, max 50)"
// @Success 200 {object} dto.RecentActivitiesResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/recent [get]
func (h *ActivityHandler) GetRecentActivities(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	activities, err := h.activityService.GetRecentActivities(r.Context(), userID, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get recent activities")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  activities,
		"limit": limit,
	})
}

// ======================================================================
= Get Activity Types
// ======================================================================

// GetActivityTypes handles retrieving available activity types.
// @Summary Get activity types
// @Description Retrieves the available activity types for filtering
// @Tags activity
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ActivityTypesResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/types [get]
func (h *ActivityHandler) GetActivityTypes(w http.ResponseWriter, r *http.Request) {
	types := []map[string]string{
		{"value": "tweet", "label": "Tweets"},
		{"value": "like", "label": "Likes"},
		{"value": "retweet", "label": "Retweets"},
		{"value": "reply", "label": "Replies"},
		{"value": "follow", "label": "Follows"},
		{"value": "mention", "label": "Mentions"},
		{"value": "quote", "label": "Quotes"},
		{"value": "bookmark", "label": "Bookmarks"},
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"types": types,
		"count": len(types),
	})
}

// ======================================================================
= Get Activity Summary
// ======================================================================

// GetActivitySummary handles retrieving a summary of user activity.
// @Summary Get activity summary
// @Description Retrieves a summary of the user's activity
// @Tags activity
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ActivitySummaryResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/summary [get]
func (h *ActivityHandler) GetActivitySummary(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	summary, err := h.activityService.GetActivitySummary(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get activity summary")
		return
	}

	h.sendSuccess(w, http.StatusOK, summary)
}

// ======================================================================
= Record Activity (for client-side tracking)
// ======================================================================

// RecordActivity handles recording a user activity.
// @Summary Record activity
// @Description Records a user activity (for client-side tracking)
// @Tags activity
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.RecordActivityRequest true "Activity details"
// @Success 201 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/activity/record [post]
func (h *ActivityHandler) RecordActivity(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.RecordActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.activityService.RecordActivity(r.Context(), userID, req.ActivityType, req.ReferenceID, req.Metadata); err != nil {
		h.handleServiceError(w, err, "Failed to record activity")
		return
	}

	h.sendSuccess(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Activity recorded successfully",
	})
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminGetUserActivityStats handles retrieving activity stats for any user (admin).
// @Summary Admin get user activity stats
// @Description Retrieves activity statistics for any user (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param user_id path string true "User ID"
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.ActivityStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/activity/user-stats/{user_id} [get]
func (h *ActivityHandler) AdminGetUserActivityStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	targetUserID := vars["user_id"]
	if targetUserID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	// Verify user exists
	_, err = h.userService.GetUserByID(r.Context(), targetUserID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "User not found", nil)
		return
	}

	stats, err := h.activityService.GetActivityStats(r.Context(), targetUserID, days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get activity stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// AdminGetPlatformActivityStats handles retrieving platform-wide activity stats (admin).
// @Summary Admin get platform activity stats
// @Description Retrieves platform-wide activity statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.PlatformActivityStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/activity/platform-stats [get]
func (h *ActivityHandler) AdminGetPlatformActivityStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.activityService.GetPlatformActivityStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get platform activity stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// AdminDeleteActivity handles admin deletion of an activity record.
// @Summary Admin delete activity
// @Description Deletes an activity record (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Activity ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/activity/{id} [delete]
func (h *ActivityHandler) AdminDeleteActivity(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	activityID := vars["id"]
	if activityID == "" {
		h.sendError(w, http.StatusBadRequest, "Activity ID required", nil)
		return
	}

	if err := h.activityService.AdminDeleteActivity(r.Context(), activityID); err != nil {
		h.handleServiceError(w, err, "Failed to delete activity")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Activity deleted successfully",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *ActivityHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *ActivityHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *ActivityHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *ActivityHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrActivityNotFound):
		h.sendError(w, http.StatusNotFound, "Activity not found", nil)
	case errors.Is(err, service.ErrInvalidActivityType):
		h.sendError(w, http.StatusBadRequest, "Invalid activity type", nil)
	case errors.Is(err, service.ErrNoActivityData):
		h.sendError(w, http.StatusNotFound, "No activity data available", nil)
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

// HealthCheck returns the health status of the activity handler.
func (h *ActivityHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "activity_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}