// backend/internal/handler/trending_handler.go
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

// TrendingHandler handles all trending-related HTTP endpoints.
type TrendingHandler struct {
	trendingService service.TrendingService
	tweetService    service.TweetService
	log             *logrus.Entry
}

// NewTrendingHandler creates a new trending handler.
func NewTrendingHandler(
	trendingService service.TrendingService,
	tweetService service.TweetService,
) *TrendingHandler {
	return &TrendingHandler{
		trendingService: trendingService,
		tweetService:    tweetService,
		log:             logger.WithField("handler", "trending"),
	}
}

// ======================================================================
// Get Trending Topics
// ======================================================================

// GetTrendingTopics handles retrieving trending topics.
// @Summary Get trending topics
// @Description Retrieves trending topics, hashtags, and trends
// @Tags trending
// @Produce json
// @Param limit query int false "Number of trends (default 10, max 50)"
// @Param category query string false "Category (global, for-you, following, local)"
// @Param days query int false "Days to analyze (default 1, max 7)"
// @Param include_hashtags query bool false "Include hashtags in response"
// @Param include_topics query bool false "Include topics in response"
// @Param include_people query bool false "Include trending people in response"
// @Success 200 {object} dto.TrendingResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending [get]
func (h *TrendingHandler) GetTrendingTopics(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	category := r.URL.Query().Get("category")
	if category == "" {
		category = "global"
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 7 {
		days = 1
	}
	includeHashtags, _ := strconv.ParseBool(r.URL.Query().Get("include_hashtags"))
	includeTopics, _ := strconv.ParseBool(r.URL.Query().Get("include_topics"))
	includePeople, _ := strconv.ParseBool(r.URL.Query().Get("include_people"))

	// Default to include all if none specified
	if !includeHashtags && !includeTopics && !includePeople {
		includeHashtags = true
		includeTopics = true
		includePeople = true
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	response, err := h.trendingService.GetTrendingTopics(r.Context(), limit, category, days, includeHashtags, includeTopics, includePeople, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending topics")
		return
	}

	h.sendSuccess(w, http.StatusOK, response)
}

// ======================================================================
// Get Trending Hashtags
// ======================================================================

// GetTrendingHashtags handles retrieving trending hashtags.
// @Summary Get trending hashtags
// @Description Retrieves trending hashtags
// @Tags trending
// @Produce json
// @Param limit query int false "Number of hashtags (default 10, max 50)"
// @Param days query int false "Days to analyze (default 1, max 7)"
// @Param search query string false "Search hashtags"
// @Success 200 {object} dto.HashtagListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending/hashtags [get]
func (h *TrendingHandler) GetTrendingHashtags(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 7 {
		days = 1
	}
	search := r.URL.Query().Get("search")

	currentUserID, _ := middleware.GetUserID(r.Context())

	hashtags, err := h.trendingService.GetTrendingHashtags(r.Context(), limit, days, search, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending hashtags")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  hashtags,
		"limit": limit,
		"days":  days,
	})
}

// ======================================================================
= Get Trending Topics with Details
// ======================================================================

// GetTrendingTopicsDetailed handles retrieving trending topics with detailed analytics.
// @Summary Get trending topics detailed
// @Description Retrieves trending topics with engagement analytics
// @Tags trending
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Number of topics (default 10, max 50)"
// @Param days query int false "Days to analyze (default 1, max 7)"
// @Param sort_by query string false "Sort by (score, engagement, volume)"
// @Success 200 {object} dto.TrendingDetailedResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending/detailed [get]
func (h *TrendingHandler) GetTrendingTopicsDetailed(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 7 {
		days = 1
	}
	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "score"
	}

	topics, err := h.trendingService.GetTrendingTopicsDetailed(r.Context(), userID, limit, days, sortBy)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get detailed trending topics")
		return
	}

	h.sendSuccess(w, http.StatusOK, topics)
}

// ======================================================================
= Get Trending People
// ======================================================================

// GetTrendingPeople handles retrieving trending users.
// @Summary Get trending people
// @Description Retrieves trending users/creators
// @Tags trending
// @Produce json
// @Param limit query int false "Number of users (default 10, max 50)"
// @Param days query int false "Days to analyze (default 1, max 7)"
// @Param category query string false "Category (global, for-you, following)"
// @Success 200 {object} dto.TrendingPeopleResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending/people [get]
func (h *TrendingHandler) GetTrendingPeople(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 7 {
		days = 1
	}
	category := r.URL.Query().Get("category")
	if category == "" {
		category = "global"
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	people, err := h.trendingService.GetTrendingPeople(r.Context(), limit, days, category, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending people")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  people,
		"limit": limit,
		"days":  days,
	})
}

// ======================================================================
= Get Trend History
// ======================================================================

// GetTrendHistory handles retrieving historical trend data.
// @Summary Get trend history
// @Description Retrieves historical data for a specific trend
// @Tags trending
// @Produce json
// @Param trend query string true "Trend name or hashtag"
// @Param days query int false "Days to analyze (default 7, max 30)"
// @Param granularity query string false "Granularity (hourly, daily) default daily"
// @Success 200 {object} dto.TrendHistoryResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending/history [get]
func (h *TrendingHandler) GetTrendHistory(w http.ResponseWriter, r *http.Request) {
	trend := r.URL.Query().Get("trend")
	if trend == "" {
		h.sendError(w, http.StatusBadRequest, "Trend is required", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "daily"
	}

	history, err := h.trendingService.GetTrendHistory(r.Context(), trend, days, granularity)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trend history")
		return
	}

	h.sendSuccess(w, http.StatusOK, history)
}

// ======================================================================
= Get Trending Topics by Location
// ======================================================================

// GetTrendingByLocation handles retrieving trending topics by location.
// @Summary Get trending by location
// @Description Retrieves trending topics for a specific location
// @Tags trending
// @Produce json
// @Param location query string true "Location (city, country)"
// @Param limit query int false "Number of trends (default 10, max 50)"
// @Param days query int false "Days to analyze (default 1, max 7)"
// @Success 200 {object} dto.TrendingResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending/location [get]
func (h *TrendingHandler) GetTrendingByLocation(w http.ResponseWriter, r *http.Request) {
	location := r.URL.Query().Get("location")
	if location == "" {
		h.sendError(w, http.StatusBadRequest, "Location is required", nil)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 7 {
		days = 1
	}

	trends, err := h.trendingService.GetTrendingByLocation(r.Context(), location, limit, days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending by location")
		return
	}

	h.sendSuccess(w, http.StatusOK, trends)
}

// ======================================================================
= Get Trending Topics by Category
// ======================================================================

// GetTrendingByCategory handles retrieving trending topics by category.
// @Summary Get trending by category
// @Description Retrieves trending topics for a specific category
// @Tags trending
// @Produce json
// @Param category path string true "Category (sports, news, entertainment, tech, business, health, science)"
// @Param limit query int false "Number of trends (default 10, max 50)"
// @Param days query int false "Days to analyze (default 1, max 7)"
// @Success 200 {object} dto.TrendingResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending/category/{category} [get]
func (h *TrendingHandler) GetTrendingByCategory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	category := vars["category"]
	if category == "" {
		h.sendError(w, http.StatusBadRequest, "Category is required", nil)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 7 {
		days = 1
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	trends, err := h.trendingService.GetTrendingByCategory(r.Context(), category, limit, days, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending by category")
		return
	}

	h.sendSuccess(w, http.StatusOK, trends)
}

// ======================================================================
= Get Trending Statistics
// ======================================================================

// GetTrendingStats handles retrieving trending statistics.
// @Summary Get trending stats
// @Description Retrieves statistics about trending topics
// @Tags trending
// @Security BearerAuth
// @Produce json
// @Param days query int false "Days to analyze (default 7, max 30)"
// @Success 200 {object} dto.TrendingStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending/stats [get]
func (h *TrendingHandler) GetTrendingStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	stats, err := h.trendingService.GetTrendingStats(r.Context(), userID, days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListTrends handles admin listing of all trends.
// @Summary Admin list trends
// @Description Lists all trends for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, hidden, removed)"
// @Param category query string false "Filter by category"
// @Param search query string false "Search by name"
// @Success 200 {object} dto.TrendAdminListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/trends [get]
func (h *TrendingHandler) AdminListTrends(w http.ResponseWriter, r *http.Request) {
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
	status := r.URL.Query().Get("status")
	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	trends, nextCursor, total, err := h.trendingService.AdminListTrends(r.Context(), cursor, limit, status, category, search)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list trends")
		return
	}

	// Build admin response
	responses := make([]*dto.TrendAdminResponse, 0, len(trends))
	for _, t := range trends {
		responses = append(responses, &dto.TrendAdminResponse{
			ID:          t.ID,
			Name:        t.Name,
			Category:    t.Category,
			Status:      t.Status,
			Score:       t.Score,
			Volume:      t.Volume,
			Engagement:  t.Engagement,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
			ExpiresAt:   t.ExpiresAt,
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

// AdminHideTrend handles hiding a trend.
// @Summary Admin hide trend
// @Description Hides a trend from public view (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Trend ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/trends/{id}/hide [post]
func (h *TrendingHandler) AdminHideTrend(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	trendID := vars["id"]
	if trendID == "" {
		h.sendError(w, http.StatusBadRequest, "Trend ID required", nil)
		return
	}

	if err := h.trendingService.AdminHideTrend(r.Context(), trendID); err != nil {
		h.handleServiceError(w, err, "Failed to hide trend")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Trend hidden successfully",
	})
}

// AdminUnhideTrend handles unhiding a trend.
// @Summary Admin unhide trend
// @Description Unhides a previously hidden trend (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Trend ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/trends/{id}/unhide [post]
func (h *TrendingHandler) AdminUnhideTrend(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	trendID := vars["id"]
	if trendID == "" {
		h.sendError(w, http.StatusBadRequest, "Trend ID required", nil)
		return
	}

	if err := h.trendingService.AdminUnhideTrend(r.Context(), trendID); err != nil {
		h.handleServiceError(w, err, "Failed to unhide trend")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Trend unhidden successfully",
	})
}

// AdminDeleteTrend handles admin deletion of a trend.
// @Summary Admin delete trend
// @Description Deletes a trend (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Trend ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/trends/{id} [delete]
func (h *TrendingHandler) AdminDeleteTrend(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	trendID := vars["id"]
	if trendID == "" {
		h.sendError(w, http.StatusBadRequest, "Trend ID required", nil)
		return
	}

	if err := h.trendingService.AdminDeleteTrend(r.Context(), trendID); err != nil {
		h.handleServiceError(w, err, "Failed to delete trend")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Trend deleted successfully",
	})
}

// AdminGetTrendStats handles retrieving global trend statistics.
// @Summary Admin get trend stats
// @Description Retrieves global trend statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.TrendGlobalStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/trends/stats [get]
func (h *TrendingHandler) AdminGetTrendStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.trendingService.AdminGetTrendStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trend stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// AdminRefreshTrends handles refreshing trending topics.
// @Summary Admin refresh trends
// @Description Forces a refresh of trending topics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/trends/refresh [post]
func (h *TrendingHandler) AdminRefreshTrends(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	if err := h.trendingService.AdminRefreshTrends(r.Context()); err != nil {
		h.handleServiceError(w, err, "Failed to refresh trends")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Trends refreshed successfully",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *TrendingHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *TrendingHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *TrendingHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *TrendingHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrTrendNotFound):
		h.sendError(w, http.StatusNotFound, "Trend not found", nil)
	case errors.Is(err, service.ErrTrendAlreadyExists):
		h.sendError(w, http.StatusConflict, "Trend already exists", nil)
	case errors.Is(err, service.ErrTrendHidden):
		h.sendError(w, http.StatusBadRequest, "Trend is hidden", nil)
	case errors.Is(err, service.ErrTrendAlreadyHidden):
		h.sendError(w, http.StatusBadRequest, "Trend is already hidden", nil)
	case errors.Is(err, service.ErrTrendNotHidden):
		h.sendError(w, http.StatusBadRequest, "Trend is not hidden", nil)
	case errors.Is(err, service.ErrInvalidTrendCategory):
		h.sendError(w, http.StatusBadRequest, "Invalid trend category", nil)
	case errors.Is(err, service.ErrInvalidTrendStatus):
		h.sendError(w, http.StatusBadRequest, "Invalid trend status", nil)
	case errors.Is(err, service.ErrTrendCalculationFailed):
		h.sendError(w, http.StatusInternalServerError, "Failed to calculate trends", nil)
	case errors.Is(err, service.ErrTrendRefreshFailed):
		h.sendError(w, http.StatusInternalServerError, "Failed to refresh trends", nil)
	case errors.Is(err, service.ErrTrendNotFoundInHistory):
		h.sendError(w, http.StatusNotFound, "Trend not found in history", nil)
	case errors.Is(err, service.ErrInvalidLocation):
		h.sendError(w, http.StatusBadRequest, "Invalid location", nil)
	case errors.Is(err, service.ErrInvalidGranularity):
		h.sendError(w, http.StatusBadRequest, "Invalid granularity", nil)
	case errors.Is(err, service.ErrInvalidSortBy):
		h.sendError(w, http.StatusBadRequest, "Invalid sort by parameter", nil)
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

// HealthCheck returns the health status of the trending handler.
func (h *TrendingHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "trending_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}