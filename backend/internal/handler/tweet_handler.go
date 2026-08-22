// backend/internal/handler/tweet_handler.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// TweetHandler handles all tweet-related HTTP endpoints.
type TweetHandler struct {
	tweetService service.TweetService
	likeService  service.LikeService
	retweetService service.RetweetService
	bookmarkService service.BookmarkService
	pollService  service.PollService
	searchService service.SearchService
	feedService  service.FeedService
	log          *logrus.Entry
}

// NewTweetHandler creates a new tweet handler.
func NewTweetHandler(
	tweetService service.TweetService,
	likeService service.LikeService,
	retweetService service.RetweetService,
	bookmarkService service.BookmarkService,
	pollService service.PollService,
	searchService service.SearchService,
	feedService service.FeedService,
) *TweetHandler {
	return &TweetHandler{
		tweetService:    tweetService,
		likeService:     likeService,
		retweetService:  retweetService,
		bookmarkService: bookmarkService,
		pollService:     pollService,
		searchService:   searchService,
		feedService:     feedService,
		log:             logger.WithField("handler", "tweet"),
	}
}

// ======================================================================
// Create Tweet
// ======================================================================

// CreateTweet handles tweet creation.
// @Summary Create a new tweet
// @Description Creates a new tweet with optional media and poll
// @Tags tweets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreateTweetRequest true "Tweet details"
// @Success 201 {object} dto.TweetResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 413 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets [post]
func (h *TweetHandler) CreateTweet(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.CreateTweetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Create tweet
	tweet, err := h.tweetService.CreateTweet(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to create tweet")
		return
	}

	h.sendSuccess(w, http.StatusCreated, tweet)
}

// ======================================================================
// Get Tweet
// ======================================================================

// GetTweet handles retrieving a single tweet.
// @Summary Get a tweet by ID
// @Description Retrieves a tweet with its replies and metadata
// @Tags tweets
// @Produce json
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.TweetDetailResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/{id} [get]
func (h *TweetHandler) GetTweet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	tweet, err := h.tweetService.GetTweet(r.Context(), tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get tweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, tweet)
}

// ======================================================================
// Update Tweet
// ======================================================================

// UpdateTweet handles updating a tweet.
// @Summary Update a tweet
// @Description Updates the content of a tweet (owner only)
// @Tags tweets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Tweet ID"
// @Param request body dto.UpdateTweetRequest true "Updated content"
// @Success 200 {object} dto.TweetResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/{id} [put]
func (h *TweetHandler) UpdateTweet(w http.ResponseWriter, r *http.Request) {
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

	var req dto.UpdateTweetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	tweet, err := h.tweetService.UpdateTweet(r.Context(), tweetID, userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update tweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, tweet)
}

// ======================================================================
// Delete Tweet
// ======================================================================

// DeleteTweet handles deleting a tweet.
// @Summary Delete a tweet
// @Description Deletes a tweet (owner or admin)
// @Tags tweets
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 204 "No Content"
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/{id} [delete]
func (h *TweetHandler) DeleteTweet(w http.ResponseWriter, r *http.Request) {
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

	if err := h.tweetService.DeleteTweet(r.Context(), tweetID, userID); err != nil {
		h.handleServiceError(w, err, "Failed to delete tweet")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ======================================================================
= Get Feed
// ======================================================================

// GetFeed handles retrieving the home feed.
// @Summary Get home feed
// @Description Retrieves tweets from followed users with pagination
// @Tags tweets
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FeedResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/feed [get]
func (h *TweetHandler) GetFeed(w http.ResponseWriter, r *http.Request) {
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

	feed, nextCursor, err := h.feedService.GetFeed(r.Context(), userID, cursor, limit)
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
= Get User Tweets
// ======================================================================

// GetUserTweets handles retrieving tweets by a specific user.
// @Summary Get user tweets
// @Description Retrieves tweets from a specific user
// @Tags tweets
// @Produce json
// @Param username path string true "Username"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FeedResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{username}/tweets [get]
func (h *TweetHandler) GetUserTweets(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		h.sendError(w, http.StatusBadRequest, "Username required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	includeReplies, _ := strconv.ParseBool(r.URL.Query().Get("include_replies"))

	tweets, nextCursor, err := h.tweetService.GetUserTweets(r.Context(), username, cursor, limit, includeReplies)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user tweets")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
	})
}

// ======================================================================
= Get Replies
// ======================================================================

// GetReplies handles retrieving replies to a tweet.
// @Summary Get tweet replies
// @Description Retrieves all replies to a tweet with pagination
// @Tags tweets
// @Produce json
// @Param id path string true "Tweet ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FeedResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/{id}/replies [get]
func (h *TweetHandler) GetReplies(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	replies, nextCursor, err := h.tweetService.GetReplies(r.Context(), tweetID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get replies")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        replies,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
	})
}

// ======================================================================
= Like/Unlike
// ======================================================================

// ToggleLike handles liking/unliking a tweet.
// @Summary Like or unlike a tweet
// @Description Toggles like status on a tweet
// @Tags interactions
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.LikeResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/{id}/like [post]
func (h *TweetHandler) ToggleLike(w http.ResponseWriter, r *http.Request) {
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

	liked, count, err := h.likeService.ToggleLike(r.Context(), tweetID, userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle like")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"liked":     liked,
		"like_count": count,
	})
}

// ======================================================================
= Retweet/Undo
// ======================================================================

// ToggleRetweet handles retweeting/unretweeting a tweet.
// @Summary Retweet or undo retweet
// @Description Toggles retweet status on a tweet
// @Tags interactions
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.RetweetResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/{id}/retweet [post]
func (h *TweetHandler) ToggleRetweet(w http.ResponseWriter, r *http.Request) {
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

	retweeted, count, err := h.retweetService.ToggleRetweet(r.Context(), tweetID, userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle retweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"retweeted":     retweeted,
		"retweet_count": count,
	})
}

// ======================================================================
= Bookmark/Unbookmark
// ======================================================================

// ToggleBookmark handles bookmarking/unbookmarking a tweet.
// @Summary Bookmark or unbookmark a tweet
// @Description Toggles bookmark status on a tweet
// @Tags interactions
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.BookmarkResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/{id}/bookmark [post]
func (h *TweetHandler) ToggleBookmark(w http.ResponseWriter, r *http.Request) {
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

	bookmarked, err := h.bookmarkService.ToggleBookmark(r.Context(), tweetID, userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle bookmark")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"bookmarked": bookmarked,
	})
}

// ======================================================================
= Get Bookmarks
// ======================================================================

// GetBookmarks handles retrieving bookmarked tweets.
// @Summary Get bookmarked tweets
// @Description Retrieves tweets bookmarked by the user
// @Tags interactions
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FeedResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks [get]
func (h *TweetHandler) GetBookmarks(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, err := h.bookmarkService.GetBookmarks(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get bookmarks")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
	})
}

// ======================================================================
= Quote Tweet
// ======================================================================

// QuoteTweet handles quoting a tweet.
// @Summary Quote a tweet
// @Description Creates a new tweet quoting an existing tweet
// @Tags tweets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Tweet ID to quote"
// @Param request body dto.QuoteTweetRequest true "Quote details"
// @Success 201 {object} dto.TweetResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/tweets/{id}/quote [post]
func (h *TweetHandler) QuoteTweet(w http.ResponseWriter, r *http.Request) {
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

	var req dto.QuoteTweetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	tweet, err := h.tweetService.QuoteTweet(r.Context(), tweetID, userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to quote tweet")
		return
	}

	h.sendSuccess(w, http.StatusCreated, tweet)
}

// ======================================================================
= Search Tweets
// ======================================================================

// SearchTweets handles searching tweets.
// @Summary Search tweets
// @Description Searches tweets by keyword with filters
// @Tags tweets
// @Produce json
// @Param q query string true "Search query"
// @Param from query string false "Username filter (from:user)"
// @Param to query string false "Username filter (to:user)"
// @Param since query string false "Date filter (since:YYYY-MM-DD)"
// @Param until query string false "Date filter (until:YYYY-MM-DD)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FeedResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/search/tweets [get]
func (h *TweetHandler) SearchTweets(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.sendError(w, http.StatusBadRequest, "Search query required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	// Parse filters
	filters := h.parseSearchFilters(r)
	filters.Query = query

	tweets, nextCursor, err := h.searchService.SearchTweets(r.Context(), &filters, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search tweets")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        tweets,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
	})
}

// ======================================================================
= Get Trending
// ======================================================================

// GetTrending handles getting trending topics.
// @Summary Get trending topics
// @Description Gets trending topics/hashtags
// @Tags tweets
// @Produce json
// @Param limit query int false "Number of trends (default 10, max 50)"
// @Success 200 {object} dto.TrendingResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/trending [get]
func (h *TweetHandler) GetTrending(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	trends, err := h.tweetService.GetTrending(r.Context(), limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"trends": trends,
		"limit":  limit,
	})
}

// ======================================================================
= Poll Operations
// ======================================================================

// VotePoll handles voting on a poll.
// @Summary Vote on poll
// @Description Votes on a poll option
// @Tags interactions
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Param option_id path string true "Option ID"
// @Success 200 {object} dto.PollResult
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/{id}/vote [post]
func (h *TweetHandler) VotePoll(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	pollID := vars["id"]
	if pollID == "" {
		h.sendError(w, http.StatusBadRequest, "Poll ID required", nil)
		return
	}

	var req dto.VotePollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	result, err := h.pollService.Vote(r.Context(), pollID, userID, req.OptionID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to vote")
		return
	}

	h.sendSuccess(w, http.StatusOK, result)
}

// ======================================================================
= Helper Methods
// ======================================================================

// parseSearchFilters parses search filters from query parameters.
func (h *TweetHandler) parseSearchFilters(r *http.Request) dto.SearchFilters {
	filters := dto.SearchFilters{}

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

// sendSuccess writes a success response.
func (h *TweetHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *TweetHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *TweetHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *TweetHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrTweetNotFound):
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
	case errors.Is(err, service.ErrTweetDeleted):
		h.sendError(w, http.StatusNotFound, "Tweet has been deleted", nil)
	case errors.Is(err, service.ErrUnauthorized):
		h.sendError(w, http.StatusForbidden, "Not authorized to perform this action", nil)
	case errors.Is(err, service.ErrAlreadyLiked):
		h.sendError(w, http.StatusConflict, "Already liked", nil)
	case errors.Is(err, service.ErrAlreadyRetweeted):
		h.sendError(w, http.StatusConflict, "Already retweeted", nil)
	case errors.Is(err, service.ErrAlreadyBookmarked):
		h.sendError(w, http.StatusConflict, "Already bookmarked", nil)
	case errors.Is(err, service.ErrPollExpired):
		h.sendError(w, http.StatusBadRequest, "Poll has expired", nil)
	case errors.Is(err, service.ErrPollAlreadyVoted):
		h.sendError(w, http.StatusConflict, "Already voted on this poll", nil)
	case errors.Is(err, service.ErrInvalidPollOption):
		h.sendError(w, http.StatusBadRequest, "Invalid poll option", nil)
	case errors.Is(err, context.Canceled):
		h.sendError(w, http.StatusRequestTimeout, "Request cancelled", nil)
	case errors.Is(err, context.DeadlineExceeded):
		h.sendError(w, http.StatusGatewayTimeout, "Request timed out", nil)
	default:
		h.log.WithError(err).Error(defaultMsg)
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}