// backend/internal/handler/feed_handler.go
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

// FeedHandler handles all feed-related HTTP endpoints.
type FeedHandler struct {
	feedService       service.FeedService
	tweetService      service.TweetService
	followService     service.FollowService
	notificationService service.NotificationService
	log               *logrus.Entry
}

// NewFeedHandler creates a new feed handler.
func NewFeedHandler(
	feedService service.FeedService,
	tweetService service.TweetService,
	followService service.FollowService,
	notificationService service.NotificationService,
) *FeedHandler {
	return &FeedHandler{
		feedService:       feedService,
		tweetService:      tweetService,
		followService:     followService,
		notificationService: notificationService,
		log:               logger.WithField("handler", "feed"),
	}
}

// ======================================================================
// Get Home Feed
// ======================================================================

// GetHomeFeed handles retrieving the home feed.
// @Summary Get home feed
// @Description Retrieves tweets from followed users with pagination
// @Tags feed
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param include_replies query bool false "Include replies in feed"
// @Param include_retweets query bool false "Include retweets in feed"
// @Success 200 {object} dto.FeedResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/home [get]
func (h *FeedHandler) GetHomeFeed(w http.ResponseWriter, r *http.Request) {
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

	feed, nextCursor, err := h.feedService.GetHomeFeed(r.Context(), userID, cursor, limit, includeReplies, includeRetweets)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get feed")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        feed,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
	})
}

// ======================================================================
// Get User Feed (Tweets from a specific user)
// ======================================================================

// GetUserFeed handles retrieving tweets from a specific user.
// @Summary Get user tweets
// @Description Retrieves tweets from a specific user
// @Tags feed
// @Produce json
// @Param username path string true "Username"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param include_replies query bool false "Include replies"
// @Success 200 {object} dto.FeedResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/user/{username} [get]
func (h *FeedHandler) GetUserFeed(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.feedService.GetUserFeed(r.Context(), username, cursor, limit, includeReplies, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user feed")
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
// Get Personalized Feed (For You)
// ======================================================================

// GetForYouFeed handles retrieving the "For You" personalized feed.
// @Summary Get personalized feed
// @Description Retrieves personalized tweet recommendations
// @Tags feed
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FeedResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/for-you [get]
func (h *FeedHandler) GetForYouFeed(w http.ResponseWriter, r *http.Request) {
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

	feed, nextCursor, err := h.feedService.GetForYouFeed(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get personalized feed")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        feed,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
	})
}

// ======================================================================
// Get Trending Feed
// ======================================================================

// GetTrendingFeed handles retrieving trending tweets.
// @Summary Get trending feed
// @Description Retrieves trending tweets
// @Tags feed
// @Produce json
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param since query string false "Since timestamp (ISO 8601)"
// @Success 200 {object} dto.FeedResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/trending [get]
func (h *FeedHandler) GetTrendingFeed(w http.ResponseWriter, r *http.Request) {
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

	tweets, err := h.feedService.GetTrendingFeed(r.Context(), limit, sinceTime, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending feed")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":    tweets,
		"limit":   limit,
		"since":   sinceTime.Format(time.RFC3339),
	})
}

// ======================================================================
// Get Feed Recommendations
// ======================================================================

// GetFeedRecommendations handles retrieving recommended tweets.
// @Summary Get feed recommendations
// @Description Retrieves recommended tweets based on user activity
// @Tags feed
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Items per page (default 10, max 50)"
// @Success 200 {object} dto.FeedResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/recommendations [get]
func (h *FeedHandler) GetFeedRecommendations(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	recommendations, err := h.feedService.GetFeedRecommendations(r.Context(), userID, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get recommendations")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  recommendations,
		"limit": limit,
	})
}

// ======================================================================
// Get Feed Preferences
// ======================================================================

// GetFeedPreferences handles retrieving a user's feed preferences.
// @Summary Get feed preferences
// @Description Retrieves the user's feed customization settings
// @Tags feed
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.FeedPreferencesResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/preferences [get]
func (h *FeedHandler) GetFeedPreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	preferences, err := h.feedService.GetFeedPreferences(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get feed preferences")
		return
	}

	h.sendSuccess(w, http.StatusOK, preferences)
}

// ======================================================================
// Update Feed Preferences
// ======================================================================

// UpdateFeedPreferences handles updating a user's feed preferences.
// @Summary Update feed preferences
// @Description Updates the user's feed customization settings
// @Tags feed
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdateFeedPreferencesRequest true "Feed preferences"
// @Success 200 {object} dto.FeedPreferencesResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/preferences [put]
func (h *FeedHandler) UpdateFeedPreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdateFeedPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	preferences, err := h.feedService.UpdateFeedPreferences(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update feed preferences")
		return
	}

	h.sendSuccess(w, http.StatusOK, preferences)
}

// ======================================================================
= Get Feed Metrics (Admin only)
// ======================================================================

// GetFeedMetrics handles retrieving feed performance metrics.
// @Summary Get feed metrics
// @Description Retrieves feed performance and engagement metrics (admin only)
// @Tags feed
// @Security BearerAuth
// @Produce json
// @Param days query int false "Days to analyze (default 7, max 30)"
// @Success 200 {object} dto.FeedMetricsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/feed/metrics [get]
func (h *FeedHandler) GetFeedMetrics(w http.ResponseWriter, r *http.Request) {
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

	metrics, err := h.feedService.GetFeedMetrics(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get feed metrics")
		return
	}

	h.sendSuccess(w, http.StatusOK, metrics)
}

// ======================================================================
// Get Feed Analytics (User)
// ======================================================================

// GetUserFeedStats handles retrieving a user's feed interaction stats.
// @Summary Get user feed stats
// @Description Retrieves feed interaction statistics for the authenticated user
// @Tags feed
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UserFeedStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/stats [get]
func (h *FeedHandler) GetUserFeedStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	stats, err := h.feedService.GetUserFeedStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get feed stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
// Dismiss Feed Item
// ======================================================================

// DismissFeedItem handles dismissing a tweet from the feed.
// @Summary Dismiss feed item
// @Description Dismisses a tweet from the user's feed (temporarily)
// @Tags feed
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/feed/dismiss/{id} [post]
func (h *FeedHandler) DismissFeedItem(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	if err := h.feedService.DismissFeedItem(r.Context(), userID, tweetID); err != nil {
		h.handleServiceError(w, err, "Failed to dismiss feed item")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Feed item dismissed",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *FeedHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *FeedHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *FeedHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *FeedHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrTweetNotFound):
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
	case errors.Is(err, service.ErrFeedEmpty):
		h.sendError(w, http.StatusNotFound, "No feed items available", nil)
	case errors.Is(err, service.ErrInvalidCursor):
		h.sendError(w, http.StatusBadRequest, "Invalid cursor", nil)
	case errors.Is(err, service.ErrFeedPreferencesNotFound):
		h.sendError(w, http.StatusNotFound, "Feed preferences not found", nil)
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

// HealthCheck returns the health status of the feed handler.
func (h *FeedHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "feed_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}