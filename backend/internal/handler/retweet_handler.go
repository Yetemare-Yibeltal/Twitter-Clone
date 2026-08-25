// backend/internal/handler/retweet_handler.go
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

// RetweetHandler handles all retweet-related HTTP endpoints.
type RetweetHandler struct {
	retweetService  service.RetweetService
	tweetService    service.TweetService
	notificationService service.NotificationService
	log             *logrus.Entry
}

// NewRetweetHandler creates a new retweet handler.
func NewRetweetHandler(
	retweetService service.RetweetService,
	tweetService service.TweetService,
	notificationService service.NotificationService,
) *RetweetHandler {
	return &RetweetHandler{
		retweetService:  retweetService,
		tweetService:    tweetService,
		notificationService: notificationService,
		log:             logger.WithField("handler", "retweet"),
	}
}

// ======================================================================
// Retweet/Unretweet
// ======================================================================

// RetweetTweet handles retweeting a tweet.
// @Summary Retweet a tweet
// @Description Retweets a tweet
// @Tags retweets
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 201 {object} dto.RetweetResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/{id} [post]
func (h *RetweetHandler) RetweetTweet(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.retweetService.RetweetTweet(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to retweet")
		return
	}

	// Get updated retweet count
	count, _ := h.retweetService.GetRetweetCount(r.Context(), tweetID)

	h.sendSuccess(w, http.StatusCreated, map[string]interface{}{
		"retweeted":     result.Retweeted,
		"retweet_id":    result.ID,
		"retweet_count": count,
		"tweet_id":      tweetID,
		"timestamp":     time.Now().Unix(),
	})
}

// UnretweetTweet handles unretweeting a tweet.
// @Summary Unretweet a tweet
// @Description Unretweets a tweet
// @Tags retweets
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.RetweetResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/{id} [delete]
func (h *RetweetHandler) UnretweetTweet(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.retweetService.UnretweetTweet(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to unretweet")
		return
	}

	// Get updated retweet count
	count, _ := h.retweetService.GetRetweetCount(r.Context(), tweetID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"retweeted":     result.Retweeted,
		"retweet_id":    result.ID,
		"retweet_count": count,
		"tweet_id":      tweetID,
		"timestamp":     time.Now().Unix(),
	})
}

// ToggleRetweet handles toggling retweet status on a tweet.
// @Summary Toggle retweet
// @Description Toggles retweet status on a tweet
// @Tags retweets
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.RetweetResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/{id}/toggle [post]
func (h *RetweetHandler) ToggleRetweet(w http.ResponseWriter, r *http.Request) {
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

	// Check if already retweeted
	isRetweeted, err := h.retweetService.IsRetweeted(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check retweet status")
		return
	}

	var result *dto.RetweetResponse
	if isRetweeted {
		result, err = h.retweetService.UnretweetTweet(r.Context(), userID, tweetID)
	} else {
		result, err = h.retweetService.RetweetTweet(r.Context(), userID, tweetID)
	}

	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle retweet")
		return
	}

	// Get updated retweet count
	count, _ := h.retweetService.GetRetweetCount(r.Context(), tweetID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"retweeted":     result.Retweeted,
		"retweet_id":    result.ID,
		"retweet_count": count,
		"tweet_id":      tweetID,
		"timestamp":     time.Now().Unix(),
	})
}

// ======================================================================
= Check Retweet Status
// ======================================================================

// CheckRetweetStatus handles checking if a tweet is retweeted.
// @Summary Check retweet status
// @Description Checks if a tweet is retweeted by the authenticated user
// @Tags retweets
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.RetweetStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/{id}/status [get]
func (h *RetweetHandler) CheckRetweetStatus(w http.ResponseWriter, r *http.Request) {
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

	isRetweeted, err := h.retweetService.IsRetweeted(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check retweet status")
		return
	}

	// Get retweet count
	count, _ := h.retweetService.GetRetweetCount(r.Context(), tweetID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"retweeted":     isRetweeted,
		"retweet_count": count,
		"tweet_id":      tweetID,
	})
}

// ======================================================================
= Get Retweet Count
// ======================================================================

// GetRetweetCount handles retrieving the retweet count for a tweet.
// @Summary Get retweet count
// @Description Retrieves the total number of retweets for a tweet
// @Tags retweets
// @Produce json
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.RetweetCountResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/count/{id} [get]
func (h *RetweetHandler) GetRetweetCount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	// Verify tweet exists
	_, err := h.tweetService.GetTweetByID(r.Context(), tweetID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
		return
	}

	count, err := h.retweetService.GetRetweetCount(r.Context(), tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweet count")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"retweet_count": count,
		"tweet_id":      tweetID,
	})
}

// ======================================================================
= Get Retweets for Tweets (Bulk)
// ======================================================================

// GetRetweetsForTweets handles retrieving retweet counts for multiple tweets.
// @Summary Get retweet counts for multiple tweets
// @Description Retrieves retweet counts for a list of tweet IDs
// @Tags retweets
// @Produce json
// @Param tweet_ids query string true "Comma-separated tweet IDs"
// @Success 200 {object} dto.BulkRetweetCountResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/counts [get]
func (h *RetweetHandler) GetRetweetsForTweets(w http.ResponseWriter, r *http.Request) {
	tweetIDsParam := r.URL.Query().Get("tweet_ids")
	if tweetIDsParam == "" {
		h.sendError(w, http.StatusBadRequest, "tweet_ids query parameter is required", nil)
		return
	}

	tweetIDs := strings.Split(tweetIDsParam, ",")
	if len(tweetIDs) == 0 {
		h.sendError(w, http.StatusBadRequest, "At least one tweet ID is required", nil)
		return
	}

	counts, err := h.retweetService.GetRetweetsForTweets(r.Context(), tweetIDs)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweet counts")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"counts": counts,
	})
}

// ======================================================================
= Get Retweeters
// ======================================================================

// GetRetweeters handles retrieving users who retweeted a tweet.
// @Summary Get retweeters
// @Description Retrieves users who retweeted a tweet with pagination
// @Tags retweets
// @Produce json
// @Param id path string true "Tweet ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.RetweeterListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/{id}/retweeters [get]
func (h *RetweetHandler) GetRetweeters(w http.ResponseWriter, r *http.Request) {
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

	currentUserID, _ := middleware.GetUserID(r.Context())

	retweeters, nextCursor, total, err := h.retweetService.GetRetweeters(r.Context(), tweetID, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweeters")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        retweeters,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
		"tweet_id":    tweetID,
	})
}

// ======================================================================
= Get Retweeted Tweets
// ======================================================================

// GetRetweetedTweets handles retrieving tweets retweeted by the authenticated user.
// @Summary Get retweeted tweets
// @Description Retrieves tweets retweeted by the authenticated user
// @Tags retweets
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TweetListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/mine [get]
func (h *RetweetHandler) GetRetweetedTweets(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.retweetService.GetRetweetedTweets(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweeted tweets")
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
= Get Retweeted Tweet IDs
// ======================================================================

// GetRetweetedTweetIDs handles retrieving only the IDs of tweets retweeted by the user.
// @Summary Get retweeted tweet IDs
// @Description Retrieves only the IDs of tweets retweeted by the user
// @Tags retweets
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.RetweetedIDsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/ids [get]
func (h *RetweetHandler) GetRetweetedTweetIDs(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	ids, err := h.retweetService.GetRetweetedTweetIDs(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweeted tweet IDs")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"tweet_ids": ids,
		"count":     len(ids),
	})
}

// ======================================================================
= Get Retweet Statuses (Bulk)
// ======================================================================

// GetRetweetStatuses handles retrieving retweet statuses for multiple tweets.
// @Summary Get retweet statuses for multiple tweets
// @Description Checks retweet status for a list of tweet IDs (bulk)
// @Tags retweets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.BulkRetweetStatusRequest true "Tweet IDs"
// @Success 200 {object} dto.BulkRetweetStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/bulk-status [post]
func (h *RetweetHandler) GetRetweetStatuses(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.BulkRetweetStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	statuses, err := h.retweetService.GetRetweetStatuses(r.Context(), userID, req.TweetIDs)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweet statuses")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"statuses": statuses,
	})
}

// ======================================================================
= Get User Retweet Stats
// ======================================================================

// GetUserRetweetStats handles retrieving retweet statistics for the user.
// @Summary Get user retweet stats
// @Description Retrieves retweet statistics for the authenticated user
// @Tags retweets
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UserRetweetStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/stats [get]
func (h *RetweetHandler) GetUserRetweetStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	stats, err := h.retweetService.GetUserRetweetStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweet stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Get Most Retweeted Tweets
// ======================================================================

// GetMostRetweetedTweets handles retrieving the most retweeted tweets.
// @Summary Get most retweeted tweets
// @Description Retrieves the most popular tweets by retweet count
// @Tags retweets
// @Produce json
// @Param limit query int false "Items per page (default 10, max 50)"
// @Param since query string false "Since timestamp"
// @Success 200 {object} dto.TweetListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/most-retweeted [get]
func (h *RetweetHandler) GetMostRetweetedTweets(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	since := r.URL.Query().Get("since")
	var sinceTime time.Time
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = t
		}
	}
	if sinceTime.IsZero() {
		sinceTime = time.Now().Add(-7 * 24 * time.Hour)
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	tweets, err := h.retweetService.GetMostRetweetedTweets(r.Context(), limit, sinceTime, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get most retweeted tweets")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  tweets,
		"limit": limit,
		"since": sinceTime.Format(time.RFC3339),
	})
}

// ======================================================================
= Get Retweet Chain
// ======================================================================

// GetRetweetChain handles retrieving the retweet chain for a tweet.
// @Summary Get retweet chain
// @Description Retrieves the retweet chain showing who retweeted from whom
// @Tags retweets
// @Produce json
// @Param id path string true "Tweet ID"
// @Param max_depth query int false "Maximum depth of chain (default 5, max 10)"
// @Success 200 {object} dto.RetweetChainResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/retweets/{id}/chain [get]
func (h *RetweetHandler) GetRetweetChain(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	maxDepth, err := strconv.Atoi(r.URL.Query().Get("max_depth"))
	if err != nil || maxDepth < 1 || maxDepth > 10 {
		maxDepth = 5
	}

	chain, err := h.retweetService.GetRetweetChain(r.Context(), tweetID, maxDepth)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweet chain")
		return
	}

	h.sendSuccess(w, http.StatusOK, chain)
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListRetweets handles admin listing of all retweets.
// @Summary Admin list retweets
// @Description Lists all retweets for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param user_id query string false "Filter by user ID"
// @Param tweet_id query string false "Filter by tweet ID"
// @Success 200 {object} dto.RetweetListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/retweets [get]
func (h *RetweetHandler) AdminListRetweets(w http.ResponseWriter, r *http.Request) {
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
	tweetID := r.URL.Query().Get("tweet_id")

	retweets, nextCursor, total, err := h.retweetService.AdminListRetweets(r.Context(), cursor, limit, userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list retweets")
		return
	}

	// Build response
	retweetResponses := make([]*dto.RetweetAdminResponse, 0, len(retweets))
	for _, r := range retweets {
		user, _ := h.userService.GetUserByID(r.Context(), r.UserID)
		tweet, _ := h.tweetService.GetTweetByID(r.Context(), r.TweetID)
		retweetResponses = append(retweetResponses, &dto.RetweetAdminResponse{
			ID:           r.ID,
			UserID:       r.UserID,
			TweetID:      r.TweetID,
			Username: func() string {
				if user != nil {
					return user.Username
				}
				return ""
			}(),
			TweetContent: func() string {
				if tweet != nil {
					return tweet.Content
				}
				return ""
			}(),
			CreatedAt: r.CreatedAt,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        retweetResponses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeleteRetweet handles admin deletion of a retweet.
// @Summary Admin delete retweet
// @Description Deletes a retweet (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Retweet ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/retweets/{id} [delete]
func (h *RetweetHandler) AdminDeleteRetweet(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	retweetID := vars["id"]
	if retweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Retweet ID required", nil)
		return
	}

	if err := h.retweetService.AdminDeleteRetweet(r.Context(), retweetID); err != nil {
		h.handleServiceError(w, err, "Failed to delete retweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Retweet deleted successfully",
	})
}

// AdminGetRetweetStats handles retrieving global retweet statistics.
// @Summary Admin get retweet stats
// @Description Retrieves global retweet statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.GlobalRetweetStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/retweets/stats [get]
func (h *RetweetHandler) AdminGetRetweetStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	stats, err := h.retweetService.AdminGetRetweetStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get retweet stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *RetweetHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *RetweetHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *RetweetHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *RetweetHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrRetweetNotFound):
		h.sendError(w, http.StatusNotFound, "Retweet not found", nil)
	case errors.Is(err, service.ErrAlreadyRetweeted):
		h.sendError(w, http.StatusConflict, "Tweet already retweeted", nil)
	case errors.Is(err, service.ErrTweetNotFound):
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
	case errors.Is(err, service.ErrCannotRetweetOwn):
		h.sendError(w, http.StatusForbidden, "Cannot retweet your own tweet", nil)
	case errors.Is(err, service.ErrInvalidRetweetID):
		h.sendError(w, http.StatusBadRequest, "Invalid retweet ID", nil)
	case errors.Is(err, service.ErrRetweetDisabled):
		h.sendError(w, http.StatusBadRequest, "Retweeting is disabled for this tweet", nil)
	case errors.Is(err, service.ErrInvalidUserID):
		h.sendError(w, http.StatusBadRequest, "Invalid user ID", nil)
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

// HealthCheck returns the health status of the retweet handler.
func (h *RetweetHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "retweet_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}