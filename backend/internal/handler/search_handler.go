// backend/internal/handler/search_handler.go
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

// SearchHandler handles all search-related HTTP endpoints.
type SearchHandler struct {
	searchService service.SearchService
	log           *logrus.Entry
}

// NewSearchHandler creates a new search handler.
func NewSearchHandler(searchService service.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		log:           logger.WithField("handler", "search"),
	}
}

// ======================================================================
// Search Tweets
// ======================================================================

// SearchTweets handles searching for tweets.
// @Summary Search tweets
// @Description Searches for tweets matching the query with filters
// @Tags search
// @Produce json
// @Param q query string true "Search query (supports operators: from:, to:, since:, until:, filter:media, filter:replies, etc.)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param from query string false "Filter by username (from:user)"
// @Param to query string false "Filter by username (to:user)"
// @Param since query string false "Date filter (since:YYYY-MM-DD)"
// @Param until query string false "Date filter (until:YYYY-MM-DD)"
// @Param include_replies query bool false "Include replies in results"
// @Param include_retweets query bool false "Include retweets in results"
// @Param media_only query bool false "Only show tweets with media"
// @Success 200 {object} dto.TweetSearchResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/tweets [get]
func (h *SearchHandler) SearchTweets(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.sendError(w, http.StatusBadRequest, "Search query is required", nil)
		return
	}

	// Parse filters
	filters := h.parseSearchFilters(r)

	// Get current user ID for interaction status
	currentUserID, _ := middleware.GetUserID(r.Context())

	// Parse pagination
	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	// Call service
	tweets, nextCursor, total, err := h.searchService.SearchTweets(r.Context(), query, filters, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search tweets")
		return
	}

	// Mark interactions for current user if authenticated
	if currentUserID != "" {
		h.markTweetInteractions(r.Context(), tweets, currentUserID)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
		"query":       query,
	})
}

// ======================================================================
// Search Users
// ======================================================================

// SearchUsers handles searching for users.
// @Summary Search users
// @Description Searches for users by username or full name
// @Tags search
// @Security BearerAuth
// @Produce json
// @Param q query string true "Search query"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.UserSearchResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/users [get]
func (h *SearchHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
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

	currentUserID, _ := middleware.GetUserID(r.Context())

	users, nextCursor, total, err := h.searchService.SearchUsers(r.Context(), query, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search users")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        users,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
		"query":       query,
	})
}

// ======================================================================
// Search Hashtags
// ======================================================================

// SearchHashtags handles searching for hashtags.
// @Summary Search hashtags
// @Description Searches for hashtags matching the query
// @Tags search
// @Produce json
// @Param q query string true "Search query (without #)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.HashtagSearchResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/hashtags [get]
func (h *SearchHandler) SearchHashtags(w http.ResponseWriter, r *http.Request) {
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

	hashtags, nextCursor, total, err := h.searchService.SearchHashtags(r.Context(), query, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search hashtags")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        hashtags,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
		"query":       query,
	})
}

// ======================================================================
// Search All (Combined)
// ======================================================================

// SearchAll handles combined search across tweets, users, and hashtags.
// @Summary Combined search
// @Description Searches across tweets, users, and hashtags in one request
// @Tags search
// @Security BearerAuth
// @Produce json
// @Param q query string true "Search query"
// @Param limit query int false "Items per category (default 10, max 50)"
// @Success 200 {object} dto.CombinedSearchResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/all [get]
func (h *SearchHandler) SearchAll(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.sendError(w, http.StatusBadRequest, "Search query is required", nil)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	results, err := h.searchService.SearchAll(r.Context(), query, "", limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to perform combined search")
		return
	}

	h.sendSuccess(w, http.StatusOK, results)
}

// ======================================================================
= Get Search Suggestions
// ======================================================================

// GetSuggestions handles getting search suggestions.
// @Summary Get search suggestions
// @Description Returns autocomplete suggestions based on partial query
// @Tags search
// @Produce json
// @Param q query string true "Partial search query"
// @Param limit query int false "Number of suggestions (default 10, max 20)"
// @Success 200 {object} dto.SearchSuggestionsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/suggestions [get]
func (h *SearchHandler) GetSuggestions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.sendError(w, http.StatusBadRequest, "Query is required", nil)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 20 {
		limit = 10
	}

	suggestions, err := h.searchService.GetSearchSuggestions(r.Context(), query, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get suggestions")
		return
	}

	h.sendSuccess(w, http.StatusOK, suggestions)
}

// ======================================================================
= Get Trending Searches
// ======================================================================

// GetTrendingSearches handles getting trending search queries.
// @Summary Get trending searches
// @Description Returns the most popular search queries
// @Tags search
// @Produce json
// @Param limit query int false "Number of trending searches (default 10, max 50)"
// @Success 200 {object} dto.TrendingSearchesResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/trending [get]
func (h *SearchHandler) GetTrendingSearches(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	trending, err := h.searchService.GetTrendingSearches(r.Context(), limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending searches")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"trending": trending,
		"limit":    limit,
	})
}

// ======================================================================
= Get Search Stats (Admin only)
// ======================================================================

// GetSearchStats handles retrieving search analytics.
// @Summary Get search statistics
// @Description Returns search usage analytics (admin only)
// @Tags search
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SearchStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/search/stats [get]
func (h *SearchHandler) GetSearchStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	stats, err := h.searchService.GetSearchStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get search stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Record Search (optional endpoint for client-side logging)
// ======================================================================

// RecordSearch handles recording a user's search query.
// @Summary Record search
// @Description Records a search query for analytics (authenticated users)
// @Tags search
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.RecordSearchRequest true "Search record"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/record [post]
func (h *SearchHandler) RecordSearch(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.RecordSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if req.Query == "" {
		h.sendError(w, http.StatusBadRequest, "Query is required", nil)
		return
	}

	// Record search with default result count 0
	if err := h.searchService.RecordSearch(r.Context(), req.Query, userID, 0); err != nil {
		h.handleServiceError(w, err, "Failed to record search")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Search recorded",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// parseSearchFilters parses search filters from query parameters.
func (h *SearchHandler) parseSearchFilters(r *http.Request) *dto.SearchFilters {
	filters := &dto.SearchFilters{}

	// Query from URL param 'q' is handled separately

	// Parse from query params
	if from := r.URL.Query().Get("from"); from != "" {
		filters.FromUser = from
	}
	if to := r.URL.Query().Get("to"); to != "" {
		filters.ToUser = to
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse("2006-01-02", since); err == nil {
			filters.Since = t
		}
	}
	if until := r.URL.Query().Get("until"); until != "" {
		if t, err := time.Parse("2006-01-02", until); err == nil {
			filters.Until = t
		}
	}
	if includeReplies := r.URL.Query().Get("include_replies"); includeReplies != "" {
		filters.IncludeReplies, _ = strconv.ParseBool(includeReplies)
	}
	if includeRetweets := r.URL.Query().Get("include_retweets"); includeRetweets != "" {
		filters.IncludeRetweets, _ = strconv.ParseBool(includeRetweets)
	}
	if mediaOnly := r.URL.Query().Get("media_only"); mediaOnly != "" {
		filters.MediaOnly, _ = strconv.ParseBool(mediaOnly)
	}

	return filters
}

// markTweetInteractions marks like, retweet, bookmark status for current user.
func (h *SearchHandler) markTweetInteractions(ctx context.Context, tweets []*dto.TweetResponse, userID string) {
	// This is a convenience; the service may already include these based on current user.
	// If not, we could batch check interactions.
	// For now, assume the service already populated these fields.
	// If we need to populate them, we'd need to call like/retweet/bookmark services.
	// We'll keep this as a placeholder for completeness.
}

// ======================================================================
= Response Helpers
// ======================================================================

// sendSuccess writes a success response.
func (h *SearchHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *SearchHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *SearchHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *SearchHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrSearchQueryEmpty):
		h.sendError(w, http.StatusBadRequest, "Search query cannot be empty", nil)
	case errors.Is(err, service.ErrSearchQueryTooShort):
		h.sendError(w, http.StatusBadRequest, "Search query is too short", nil)
	case errors.Is(err, service.ErrSearchQueryTooLong):
		h.sendError(w, http.StatusBadRequest, "Search query is too long", nil)
	case errors.Is(err, service.ErrSearchInvalidFilter):
		h.sendError(w, http.StatusBadRequest, "Invalid search filter", nil)
	case errors.Is(err, service.ErrSearchNoResults):
		h.sendError(w, http.StatusNotFound, "No results found", nil)
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

// HealthCheck returns the health status of the search handler.
func (h *SearchHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "search_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}