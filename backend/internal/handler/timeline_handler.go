// backend/internal/handler/timeline_handler.go
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

// TimelineHandler handles all timeline-related HTTP endpoints.
type TimelineHandler struct {
	feedService      service.FeedService
	tweetService     service.TweetService
	followService    service.FollowService
	notificationService service.NotificationService
	log              *logrus.Entry
}

// NewTimelineHandler creates a new timeline handler.
func NewTimelineHandler(
	feedService service.FeedService,
	tweetService service.TweetService,
	followService service.FollowService,
	notificationService service.NotificationService,
) *TimelineHandler {
	return &TimelineHandler{
		feedService:      feedService,
		tweetService:     tweetService,
		followService:    followService,
		notificationService: notificationService,
		log:              logger.WithField("handler", "timeline"),
	}
}

// ======================================================================
// Get Home Timeline
// ======================================================================

// GetHomeTimeline handles retrieving the home timeline.
// @Summary Get home timeline
// @Description Retrieves tweets from followed users with pagination
// @Tags timeline
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param include_replies query bool false "Include replies in timeline"
// @Param include_retweets query bool false "Include retweets in timeline"
// @Success 200 {object} dto.TimelineResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/home [get]
func (h *TimelineHandler) GetHomeTimeline(w http.ResponseWriter, r *http.Request) {
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
	includeReplies, _ := strconv.ParseBool(r.URL.Query().Get("include_replies"))
	includeRetweets, _ := strconv.ParseBool(r.URL.Query().Get("include_retweets"))

	tweets, nextCursor, total, err := h.feedService.GetHomeTimeline(r.Context(), userID, cursor, limit, includeReplies, includeRetweets)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get home timeline")
		return
	}

	// Get notification count for the user
	unreadCount, _ := h.notificationService.GetUnreadCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":              tweets,
		"next_cursor":       nextCursor,
		"has_more":          nextCursor != "",
		"limit":             limit,
		"total":             total,
		"unread_notifications": unreadCount,
	})
}

// ======================================================================
// Get User Timeline
// ======================================================================

// GetUserTimeline handles retrieving a user's timeline.
// @Summary Get user timeline
// @Description Retrieves tweets from a specific user
// @Tags timeline
// @Produce json
// @Param username path string true "Username"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param include_replies query bool false "Include replies"
// @Success 200 {object} dto.TimelineResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/user/{username} [get]
func (h *TimelineHandler) GetUserTimeline(w http.ResponseWriter, r *http.Request) {
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
	includeReplies, _ := strconv.ParseBool(r.URL.Query().Get("include_replies"))

	currentUserID, _ := middleware.GetUserID(r.Context())

	tweets, nextCursor, total, err := h.feedService.GetUserTimeline(r.Context(), username, cursor, limit, includeReplies, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
		"username":    username,
	})
}

// ======================================================================
= Get Mentions Timeline
// ======================================================================

// GetMentionsTimeline handles retrieving tweets mentioning the user.
// @Summary Get mentions timeline
// @Description Retrieves tweets that mention the authenticated user
// @Tags timeline
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TimelineResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/mentions [get]
func (h *TimelineHandler) GetMentionsTimeline(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.feedService.GetMentionsTimeline(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get mentions timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Hashtag Timeline
// ======================================================================

// GetHashtagTimeline handles retrieving tweets by hashtag.
// @Summary Get hashtag timeline
// @Description Retrieves tweets containing a specific hashtag
// @Tags timeline
// @Produce json
// @Param hashtag path string true "Hashtag (without #)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TimelineResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/hashtag/{hashtag} [get]
func (h *TimelineHandler) GetHashtagTimeline(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hashtag := vars["hashtag"]
	if hashtag == "" {
		h.sendError(w, http.StatusBadRequest, "Hashtag is required", nil)
		return
	}
	// Remove # if present
	hashtag = strings.TrimPrefix(hashtag, "#")

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	tweets, nextCursor, total, err := h.feedService.GetHashtagTimeline(r.Context(), hashtag, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get hashtag timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
		"hashtag":     hashtag,
	})
}

// ======================================================================
= Get Trending Timeline
// ======================================================================

// GetTrendingTimeline handles retrieving trending tweets.
// @Summary Get trending timeline
// @Description Retrieves currently trending tweets
// @Tags timeline
// @Produce json
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param since query string false "Since timestamp (ISO 8601)"
// @Success 200 {object} dto.TimelineResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/trending [get]
func (h *TimelineHandler) GetTrendingTimeline(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	since := r.URL.Query().Get("since")
	var sinceTime time.Time
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = t
		}
	}
	if sinceTime.IsZero() {
		sinceTime = time.Now().Add(-24 * time.Hour)
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	tweets, total, err := h.feedService.GetTrendingTimeline(r.Context(), limit, sinceTime, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  tweets,
		"limit": limit,
		"since": sinceTime.Format(time.RFC3339),
		"total": total,
	})
}

// ======================================================================
= Get For You Timeline (Personalized)
// ======================================================================

// GetForYouTimeline handles retrieving personalized timeline.
// @Summary Get For You timeline
// @Description Retrieves personalized tweet recommendations
// @Tags timeline
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TimelineResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/for-you [get]
func (h *TimelineHandler) GetForYouTimeline(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.feedService.GetForYouTimeline(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get personalized timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Bookmarks Timeline (User's bookmarks)
// ======================================================================

// GetBookmarksTimeline handles retrieving the user's bookmarked tweets.
// @Summary Get bookmarks timeline
// @Description Retrieves tweets bookmarked by the authenticated user
// @Tags timeline
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TimelineResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/bookmarks [get]
func (h *TimelineHandler) GetBookmarksTimeline(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.feedService.GetBookmarksTimeline(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get bookmarks timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Likes Timeline (User's liked tweets)
// ======================================================================

// GetLikesTimeline handles retrieving the user's liked tweets.
// @Summary Get likes timeline
// @Description Retrieves tweets liked by the authenticated user
// @Tags timeline
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TimelineResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/likes [get]
func (h *TimelineHandler) GetLikesTimeline(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.feedService.GetLikesTimeline(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get likes timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Media Timeline
// ======================================================================

// GetMediaTimeline handles retrieving tweets with media.
// @Summary Get media timeline
// @Description Retrieves tweets containing media (images/videos)
// @Tags timeline
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TimelineResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/media [get]
func (h *TimelineHandler) GetMediaTimeline(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.feedService.GetMediaTimeline(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get media timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Mixed Timeline (Combined)
// ======================================================================

// GetMixedTimeline handles retrieving a mixed timeline of various sources.
// @Summary Get mixed timeline
// @Description Retrieves a mixed timeline combining home, trending, and recommendations
// @Tags timeline
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TimelineResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/mixed [get]
func (h *TimelineHandler) GetMixedTimeline(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.feedService.GetMixedTimeline(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get mixed timeline")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Timeline Preferences
// ======================================================================

// GetTimelinePreferences handles retrieving timeline preferences.
// @Summary Get timeline preferences
// @Description Retrieves the user's timeline customization settings
// @Tags timeline
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.TimelinePreferencesResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/preferences [get]
func (h *TimelineHandler) GetTimelinePreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	preferences, err := h.feedService.GetTimelinePreferences(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get timeline preferences")
		return
	}

	h.sendSuccess(w, http.StatusOK, preferences)
}

// ======================================================================
= Update Timeline Preferences
// ======================================================================

// UpdateTimelinePreferences handles updating timeline preferences.
// @Summary Update timeline preferences
// @Description Updates the user's timeline customization settings
// @Tags timeline
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdateTimelinePreferencesRequest true "Timeline preferences"
// @Success 200 {object} dto.TimelinePreferencesResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/timeline/preferences [put]
func (h *TimelineHandler) UpdateTimelinePreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdateTimelinePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	preferences, err := h.feedService.UpdateTimelinePreferences(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update timeline preferences")
		return
	}

	h.sendSuccess(w, http.StatusOK, preferences)
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminGetTimelineStats handles retrieving timeline analytics.
// @Summary Admin get timeline stats
// @Description Retrieves timeline performance and engagement metrics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Days to analyze (default 7, max 30)"
// @Success 200 {object} dto.TimelineStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/timeline/stats [get]
func (h *TimelineHandler) AdminGetTimelineStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.feedService.AdminGetTimelineStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get timeline stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// AdminClearCache handles clearing timeline cache.
// @Summary Admin clear timeline cache
// @Description Clears all timeline caches (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param user_id query string false "User ID (optional)"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/timeline/clear-cache [post]
func (h *TimelineHandler) AdminClearCache(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	userID := r.URL.Query().Get("user_id")

	if userID != "" {
		if err := h.feedService.ClearUserCache(r.Context(), userID); err != nil {
			h.handleServiceError(w, err, "Failed to clear user cache")
			return
		}
	} else {
		if err := h.feedService.ClearAllCache(r.Context()); err != nil {
			h.handleServiceError(w, err, "Failed to clear all cache")
			return
		}
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "Cache cleared successfully",
		"user_id":   userID,
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *TimelineHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *TimelineHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *TimelineHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *TimelineHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrFeedEmpty):
		h.sendError(w, http.StatusNotFound, "No timeline items available", nil)
	case errors.Is(err, service.ErrInvalidCursor):
		h.sendError(w, http.StatusBadRequest, "Invalid cursor", nil)
	case errors.Is(err, service.ErrTimelinePreferencesNotFound):
		h.sendError(w, http.StatusNotFound, "Timeline preferences not found", nil)
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

// HealthCheck returns the health status of the timeline handler.
func (h *TimelineHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "timeline_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}