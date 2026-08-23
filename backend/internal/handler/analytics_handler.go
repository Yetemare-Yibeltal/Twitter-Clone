// backend/internal/handler/analytics_handler.go
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

// AnalyticsHandler handles all analytics-related HTTP endpoints.
type AnalyticsHandler struct {
	analyticsService service.AnalyticsService
	userService      service.UserService
	tweetService     service.TweetService
	log              *logrus.Entry
}

// NewAnalyticsHandler creates a new analytics handler.
func NewAnalyticsHandler(
	analyticsService service.AnalyticsService,
	userService service.UserService,
	tweetService service.TweetService,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
		userService:      userService,
		tweetService:     tweetService,
		log:              logger.WithField("handler", "analytics"),
	}
}

// ======================================================================
// Get User Analytics (Self)
// ======================================================================

// GetUserAnalytics handles retrieving analytics for the authenticated user.
// @Summary Get user analytics
// @Description Retrieves analytics data for the authenticated user
// @Tags analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Param metrics query string false "Comma-separated metrics to include (tweets,likes,retweets,replies,followers)"
// @Success 200 {object} dto.UserAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/analytics/user [get]
func (h *AnalyticsHandler) GetUserAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}
	metricsParam := r.URL.Query().Get("metrics")
	metrics := strings.Split(metricsParam, ",")
	if len(metrics) == 0 || metricsParam == "" {
		metrics = []string{"tweets", "likes", "retweets", "replies", "followers"}
	}

	analytics, err := h.analyticsService.GetUserAnalytics(r.Context(), userID, days, metrics)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, analytics)
}

// ======================================================================
// Get Tweet Analytics
// ======================================================================

// GetTweetAnalytics handles retrieving analytics for a specific tweet.
// @Summary Get tweet analytics
// @Description Retrieves analytics data for a specific tweet
// @Tags analytics
// @Security BearerAuth
// @Produce json
// @Param id path string true "Tweet ID"
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.TweetAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/analytics/tweet/{id} [get]
func (h *AnalyticsHandler) GetTweetAnalytics(w http.ResponseWriter, r *http.Request) {
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

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	// Verify ownership
	tweet, err := h.tweetService.GetTweetByID(r.Context(), tweetID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
		return
	}
	if tweet.UserID != userID {
		// For now, allow users to see analytics of their own tweets only
		role, _ := middleware.GetUserRole(r.Context())
		if role != "admin" {
			h.sendError(w, http.StatusForbidden, "You can only view analytics of your own tweets", nil)
			return
		}
	}

	analytics, err := h.analyticsService.GetTweetAnalytics(r.Context(), tweetID, days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get tweet analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, analytics)
}

// ======================================================================
= Get User Analytics (Admin)
// ======================================================================

// AdminGetUserAnalytics handles retrieving analytics for any user (admin only).
// @Summary Admin get user analytics
// @Description Retrieves analytics data for any user (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param user_id path string true "User ID"
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Param metrics query string false "Comma-separated metrics"
// @Success 200 {object} dto.UserAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/analytics/user/{user_id} [get]
func (h *AnalyticsHandler) AdminGetUserAnalytics(w http.ResponseWriter, r *http.Request) {
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
	metricsParam := r.URL.Query().Get("metrics")
	metrics := strings.Split(metricsParam, ",")
	if len(metrics) == 0 || metricsParam == "" {
		metrics = []string{"tweets", "likes", "retweets", "replies", "followers"}
	}

	// Verify user exists
	_, err = h.userService.GetUserByID(r.Context(), targetUserID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "User not found", nil)
		return
	}

	analytics, err := h.analyticsService.GetUserAnalytics(r.Context(), targetUserID, days, metrics)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, analytics)
}

// ======================================================================
= Get Platform Analytics (Admin)
// ======================================================================

// AdminGetPlatformAnalytics handles retrieving platform-wide analytics.
// @Summary Admin get platform analytics
// @Description Retrieves platform-wide analytics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Param metrics query string false "Comma-separated metrics (users,tweets,engagements,growth)"
// @Success 200 {object} dto.PlatformAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/analytics/platform [get]
func (h *AnalyticsHandler) AdminGetPlatformAnalytics(w http.ResponseWriter, r *http.Request) {
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
	metricsParam := r.URL.Query().Get("metrics")
	metrics := strings.Split(metricsParam, ",")
	if len(metrics) == 0 || metricsParam == "" {
		metrics = []string{"users", "tweets", "engagements", "growth"}
	}

	analytics, err := h.analyticsService.GetPlatformAnalytics(r.Context(), days, metrics)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get platform analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, analytics)
}

// ======================================================================
= Get Engagement Analytics
// ======================================================================

// GetEngagementAnalytics handles retrieving engagement analytics for the user.
// @Summary Get engagement analytics
// @Description Retrieves engagement analytics (likes, retweets, replies, etc.) for the authenticated user
// @Tags analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.EngagementAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/analytics/engagement [get]
func (h *AnalyticsHandler) GetEngagementAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	analytics, err := h.analyticsService.GetEngagementAnalytics(r.Context(), userID, days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get engagement analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, analytics)
}

// ======================================================================
= Get Follower Growth Analytics
// ======================================================================

// GetFollowerGrowthAnalytics handles retrieving follower growth analytics.
// @Summary Get follower growth analytics
// @Description Retrieves follower growth data over time for the authenticated user
// @Tags analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.FollowerGrowthResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/analytics/followers [get]
func (h *AnalyticsHandler) GetFollowerGrowthAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	growth, err := h.analyticsService.GetFollowerGrowth(r.Context(), userID, days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get follower growth")
		return
	}

	h.sendSuccess(w, http.StatusOK, growth)
}

// ======================================================================
= Get Tweet Performance Analytics
// ======================================================================

// GetTweetPerformanceAnalytics handles retrieving tweet performance analytics.
// @Summary Get tweet performance analytics
// @Description Retrieves performance metrics for all tweets of the authenticated user
// @Tags analytics
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Param limit query int false "Number of top tweets to return (default 10, max 50)"
// @Success 200 {object} dto.TweetPerformanceResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/analytics/tweet-performance [get]
func (h *AnalyticsHandler) GetTweetPerformanceAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	performance, err := h.analyticsService.GetTweetPerformance(r.Context(), userID, days, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get tweet performance")
		return
	}

	h.sendSuccess(w, http.StatusOK, performance)
}

// ======================================================================
= Get Dashboard Analytics (Admin)
// ======================================================================

// AdminGetDashboardAnalytics handles retrieving dashboard analytics for admin panel.
// @Summary Admin get dashboard analytics
// @Description Retrieves dashboard analytics with summaries and charts (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.DashboardAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/analytics/dashboard [get]
func (h *AnalyticsHandler) AdminGetDashboardAnalytics(w http.ResponseWriter, r *http.Request) {
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

	dashboard, err := h.analyticsService.GetDashboardAnalytics(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get dashboard analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, dashboard)
}

// ======================================================================
= Get Real-time Analytics (Admin)
// ======================================================================

// AdminGetRealtimeAnalytics handles retrieving real-time analytics.
// @Summary Admin get real-time analytics
// @Description Retrieves real-time platform activity metrics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param window query string false "Time window (1h, 6h, 24h)"
// @Success 200 {object} dto.RealtimeAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/analytics/realtime [get]
func (h *AnalyticsHandler) AdminGetRealtimeAnalytics(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	window := r.URL.Query().Get("window")
	if window == "" {
		window = "24h"
	}

	realtime, err := h.analyticsService.GetRealtimeAnalytics(r.Context(), window)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get real-time analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, realtime)
}

// ======================================================================
= Get Top Users (Admin)
// ======================================================================

// AdminGetTopUsers handles retrieving top users by engagement.
// @Summary Admin get top users
// @Description Retrieves top users by engagement, followers, or activity (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param sort_by query string false "Sort by (followers, engagement, tweets)"
// @Param limit query int false "Number of users to return (default 10, max 50)"
// @Success 200 {object} dto.TopUsersResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/analytics/top-users [get]
func (h *AnalyticsHandler) AdminGetTopUsers(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "followers"
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	topUsers, err := h.analyticsService.GetTopUsers(r.Context(), sortBy, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get top users")
		return
	}

	h.sendSuccess(w, http.StatusOK, topUsers)
}

// ======================================================================
= Get Top Tweets (Admin)
// ======================================================================

// AdminGetTopTweets handles retrieving top tweets by engagement.
// @Summary Admin get top tweets
// @Description Retrieves top tweets by engagement (likes, retweets, replies) (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param sort_by query string false "Sort by (likes, retweets, replies)"
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Param limit query int false "Number of tweets to return (default 10, max 50)"
// @Success 200 {object} dto.TopTweetsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/analytics/top-tweets [get]
func (h *AnalyticsHandler) AdminGetTopTweets(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "likes"
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	topTweets, err := h.analyticsService.GetTopTweets(r.Context(), sortBy, days, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get top tweets")
		return
	}

	h.sendSuccess(w, http.StatusOK, topTweets)
}

// ======================================================================
= Get Export Analytics (Admin)
// ======================================================================

// AdminExportAnalytics handles exporting analytics data as CSV/JSON.
// @Summary Admin export analytics
// @Description Exports analytics data in CSV or JSON format (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce application/json, text/csv
// @Param format query string false "Export format (json, csv)"
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Param report_type query string false "Report type (users, tweets, engagements)"
// @Success 200 {file} file
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/analytics/export [get]
func (h *AnalyticsHandler) AdminExportAnalytics(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}
	reportType := r.URL.Query().Get("report_type")
	if reportType == "" {
		reportType = "tweets"
	}

	exportData, err := h.analyticsService.ExportAnalytics(r.Context(), reportType, days, format)
	if err != nil {
		h.handleServiceError(w, err, "Failed to export analytics")
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=analytics_export.csv")
		w.Write(exportData)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=analytics_export.json")
		w.Write(exportData)
	}
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *AnalyticsHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *AnalyticsHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *AnalyticsHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *AnalyticsHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrAnalyticsNotFound):
		h.sendError(w, http.StatusNotFound, "Analytics data not found", nil)
	case errors.Is(err, service.ErrInvalidMetric):
		h.sendError(w, http.StatusBadRequest, "Invalid metric requested", nil)
	case errors.Is(err, service.ErrInvalidReportType):
		h.sendError(w, http.StatusBadRequest, "Invalid report type", nil)
	case errors.Is(err, service.ErrExportFailed):
		h.sendError(w, http.StatusInternalServerError, "Failed to export data", nil)
	case errors.Is(err, service.ErrNoDataAvailable):
		h.sendError(w, http.StatusNotFound, "No data available for the requested period", nil)
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrTweetNotFound):
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
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

// HealthCheck returns the health status of the analytics handler.
func (h *AnalyticsHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "analytics_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}