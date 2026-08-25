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
	tweetService  service.TweetService
	userService   service.UserService
	log           *logrus.Entry
}

// NewSearchHandler creates a new search handler.
func NewSearchHandler(
	searchService service.SearchService,
	tweetService service.TweetService,
	userService service.UserService,
) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		tweetService:  tweetService,
		userService:   userService,
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
// @Param sort_by query string false "Sort by (relevance, latest, oldest, most_liked, most_retweeted)"
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

	// Parse sort
	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "relevance"
	}

	// Call service
	tweets, nextCursor, total, err := h.searchService.SearchTweets(r.Context(), query, filters, cursor, limit, sortBy)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search tweets")
		return
	}

	// Mark interactions for current user if authenticated
	if currentUserID != "" && len(tweets) > 0 {
		h.markTweetInteractions(r.Context(), tweets, currentUserID)
	}

	// Build response
	response := &dto.TweetSearchResponse{
		Data:        tweets,
		NextCursor:  nextCursor,
		HasMore:     nextCursor != "",
		Limit:       limit,
		Total:       total,
		Query:       query,
		SortBy:      sortBy,
		Filters:     filters,
	}

	h.sendSuccess(w, http.StatusOK, response)
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
// @Param sort_by query string false "Sort by (relevance, followers, tweets, joined_at)"
// @Param filter_following query bool false "Only show users the authenticated user follows"
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
	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "relevance"
	}
	filterFollowing, _ := strconv.ParseBool(r.URL.Query().Get("filter_following"))

	currentUserID, _ := middleware.GetUserID(r.Context())

	users, nextCursor, total, err := h.searchService.SearchUsers(r.Context(), query, cursor, limit, sortBy, filterFollowing, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search users")
		return
	}

	// Build response
	response := &dto.UserSearchResponse{
		Data:        users,
		NextCursor:  nextCursor,
		HasMore:     nextCursor != "",
		Limit:       limit,
		Total:       total,
		Query:       query,
	}

	h.sendSuccess(w, http.StatusOK, response)
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
// @Param sort_by query string false "Sort by (relevance, popularity, latest)"
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
	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "popularity"
	}

	hashtags, nextCursor, total, err := h.searchService.SearchHashtags(r.Context(), query, cursor, limit, sortBy)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search hashtags")
		return
	}

	// Build response
	response := &dto.HashtagSearchResponse{
		Data:        hashtags,
		NextCursor:  nextCursor,
		HasMore:     nextCursor != "",
		Limit:       limit,
		Total:       total,
		Query:       query,
	}

	h.sendSuccess(w, http.StatusOK, response)
}

// ======================================================================
= Search All (Combined)
// ======================================================================

// SearchAll handles combined search across tweets, users, and hashtags.
// @Summary Combined search
// @Description Searches across tweets, users, and hashtags in one request
// @Tags search
// @Security BearerAuth
// @Produce json
// @Param q query string true "Search query"
// @Param limit query int false "Items per category (default 10, max 50)"
// @Param include_tweets query bool false "Include tweets in results"
// @Param include_users query bool false "Include users in results"
// @Param include_hashtags query bool false "Include hashtags in results"
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
	includeTweets, _ := strconv.ParseBool(r.URL.Query().Get("include_tweets"))
	includeUsers, _ := strconv.ParseBool(r.URL.Query().Get("include_users"))
	includeHashtags, _ := strconv.ParseBool(r.URL.Query().Get("include_hashtags"))

	// Default to include all if none specified
	if !includeTweets && !includeUsers && !includeHashtags {
		includeTweets = true
		includeUsers = true
		includeHashtags = true
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	results, err := h.searchService.SearchAll(r.Context(), query, limit, includeTweets, includeUsers, includeHashtags, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to perform combined search")
		return
	}

	// Add query to response
	results.Query = query

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
// @Param include_hashtags query bool false "Include hashtag suggestions"
// @Param include_users query bool false "Include user suggestions"
// @Param include_trending query bool false "Include trending suggestions"
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
	includeHashtags, _ := strconv.ParseBool(r.URL.Query().Get("include_hashtags"))
	includeUsers, _ := strconv.ParseBool(r.URL.Query().Get("include_users"))
	includeTrending, _ := strconv.ParseBool(r.URL.Query().Get("include_trending"))

	// Default to all types
	if !includeHashtags && !includeUsers && !includeTrending {
		includeHashtags = true
		includeUsers = true
		includeTrending = true
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	suggestions, err := h.searchService.GetSearchSuggestions(r.Context(), query, limit, includeHashtags, includeUsers, includeTrending, currentUserID)
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
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.TrendingSearchesResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/trending [get]
func (h *SearchHandler) GetTrendingSearches(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	trending, err := h.searchService.GetTrendingSearches(r.Context(), limit, days, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending searches")
		return
	}

	response := &dto.TrendingSearchesResponse{
		Trending: trending,
		Limit:    limit,
		Days:     days,
	}

	h.sendSuccess(w, http.StatusOK, response)
}

// ======================================================================
= Record Search (for analytics)
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

	if err := h.searchService.RecordSearch(r.Context(), req.Query, userID, req.ResultCount); err != nil {
		h.handleServiceError(w, err, "Failed to record search")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Search recorded",
	})
}

// ======================================================================
= Admin Search Stats
// ======================================================================

// AdminGetSearchStats handles retrieving search analytics.
// @Summary Admin get search stats
// @Description Returns search usage analytics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Param granularity query string false "Granularity (hourly, daily, weekly) default daily"
// @Success 200 {object} dto.SearchStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/search/stats [get]
func (h *SearchHandler) AdminGetSearchStats(w http.ResponseWriter, r *http.Request) {
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
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "daily"
	}

	stats, err := h.searchService.GetSearchStats(r.Context(), days, granularity)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get search stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Admin Clear Search Cache
// ======================================================================

// AdminClearSearchCache handles clearing search cache.
// @Summary Admin clear search cache
// @Description Clears all search caches (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param type query string false "Cache type (tweets, users, hashtags, all) default all"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/search/clear-cache [post]
func (h *SearchHandler) AdminClearSearchCache(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	cacheType := r.URL.Query().Get("type")
	if cacheType == "" {
		cacheType = "all"
	}

	if err := h.searchService.ClearSearchCache(r.Context(), cacheType); err != nil {
		h.handleServiceError(w, err, "Failed to clear search cache")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Search cache cleared successfully",
		"type":    cacheType,
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// parseSearchFilters parses search filters from query parameters.
func (h *SearchHandler) parseSearchFilters(r *http.Request) *dto.SearchFilters {
	filters := &dto.SearchFilters{}

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
	// If we need to populate them, we'd need to call like/retweet/bookmark services.
	// The search service should already handle this when currentUserID is provided.
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
		h.sendError(w, http.StatusBadRequest, "Search query is too short (minimum 2 characters)", nil)
	case errors.Is(err, service.ErrSearchQueryTooLong):
		h.sendError(w, http.StatusBadRequest, "Search query is too long (maximum 200 characters)", nil)
	case errors.Is(err, service.ErrSearchInvalidFilter):
		h.sendError(w, http.StatusBadRequest, "Invalid search filter", nil)
	case errors.Is(err, service.ErrSearchNoResults):
		h.sendError(w, http.StatusNotFound, "No results found", nil)
	case errors.Is(err, service.ErrInvalidSortBy):
		h.sendError(w, http.StatusBadRequest, "Invalid sort by parameter", nil)
	case errors.Is(err, service.ErrInvalidGranularity):
		h.sendError(w, http.StatusBadRequest, "Invalid granularity parameter", nil)
	case errors.Is(err, service.ErrCacheClearFailed):
		h.sendError(w, http.StatusInternalServerError, "Failed to clear cache", nil)
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

// HealthCheck returns the health status of the search handler.
func (h *SearchHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "search_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}