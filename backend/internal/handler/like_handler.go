// backend/internal/handler/like_handler.go
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

// LikeHandler handles all like-related HTTP endpoints.
type LikeHandler struct {
	likeService    service.LikeService
	tweetService   service.TweetService
	notificationService service.NotificationService
	log            *logrus.Entry
}

// NewLikeHandler creates a new like handler.
func NewLikeHandler(
	likeService service.LikeService,
	tweetService service.TweetService,
	notificationService service.NotificationService,
) *LikeHandler {
	return &LikeHandler{
		likeService:    likeService,
		tweetService:   tweetService,
		notificationService: notificationService,
		log:            logger.WithField("handler", "like"),
	}
}

// ======================================================================
// Toggle Like
// ======================================================================

// ToggleLike handles liking/unliking a tweet.
// @Summary Toggle like on tweet
// @Description Likes or unlikes a tweet (toggles)
// @Tags likes
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.LikeResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/{id}/toggle [post]
func (h *LikeHandler) ToggleLike(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.likeService.ToggleLike(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle like")
		return
	}

	// Get updated like count
	count, _ := h.likeService.GetLikeCount(r.Context(), tweetID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"liked":       result.Liked,
		"like_count":  count,
		"tweet_id":    tweetID,
		"timestamp":   time.Now().Unix(),
	})
}

// LikeTweet handles explicitly liking a tweet.
// @Summary Like a tweet
// @Description Likes a tweet
// @Tags likes
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 201 {object} dto.LikeResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/{id} [post]
func (h *LikeHandler) LikeTweet(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.likeService.LikeTweet(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to like tweet")
		return
	}

	// Get updated like count
	count, _ := h.likeService.GetLikeCount(r.Context(), tweetID)

	// Create notification for tweet owner (if not self-like)
	tweet, err := h.tweetService.GetTweetByID(r.Context(), tweetID)
	if err == nil && tweet.UserID != userID {
		// Notification creation handled by service
	}

	h.sendSuccess(w, http.StatusCreated, map[string]interface{}{
		"liked":       result.Liked,
		"like_count":  count,
		"tweet_id":    tweetID,
	})
}

// UnlikeTweet handles explicitly unliking a tweet.
// @Summary Unlike a tweet
// @Description Unlikes a tweet
// @Tags likes
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.LikeResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/{id} [delete]
func (h *LikeHandler) UnlikeTweet(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.likeService.UnlikeTweet(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to unlike tweet")
		return
	}

	// Get updated like count
	count, _ := h.likeService.GetLikeCount(r.Context(), tweetID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"liked":       result.Liked,
		"like_count":  count,
		"tweet_id":    tweetID,
	})
}

// ======================================================================
= Check Like Status
// ======================================================================

// CheckLikeStatus handles checking if a tweet is liked.
// @Summary Check like status
// @Description Checks if a tweet is liked by the authenticated user
// @Tags likes
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.LikeStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/{id}/status [get]
func (h *LikeHandler) CheckLikeStatus(w http.ResponseWriter, r *http.Request) {
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

	isLiked, err := h.likeService.IsLiked(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check like status")
		return
	}

	// Get like count
	count, _ := h.likeService.GetLikeCount(r.Context(), tweetID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"liked":       isLiked,
		"like_count":  count,
		"tweet_id":    tweetID,
	})
}

// ======================================================================
= Get Like Count
// ======================================================================

// GetLikeCount handles retrieving the like count for a tweet.
// @Summary Get like count
// @Description Retrieves the total number of likes for a tweet
// @Tags likes
// @Produce json
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.LikeCountResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/count/{id} [get]
func (h *LikeHandler) GetLikeCount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	count, err := h.likeService.GetLikeCount(r.Context(), tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get like count")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"like_count": count,
		"tweet_id":   tweetID,
	})
}

// GetLikesForTweets handles retrieving like counts for multiple tweets (bulk).
// @Summary Get like counts for multiple tweets
// @Description Retrieves like counts for a list of tweet IDs
// @Tags likes
// @Produce json
// @Param tweet_ids query string true "Comma-separated tweet IDs"
// @Success 200 {object} dto.BulkLikeCountResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/counts [get]
func (h *LikeHandler) GetLikesForTweets(w http.ResponseWriter, r *http.Request) {
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

	counts, err := h.likeService.GetLikesForTweets(r.Context(), tweetIDs)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get like counts")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"counts": counts,
	})
}

// ======================================================================
= Get Likers
// ======================================================================

// GetLikers handles retrieving users who liked a tweet.
// @Summary Get likers
// @Description Retrieves users who liked a tweet with pagination
// @Tags likes
// @Produce json
// @Param id path string true "Tweet ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.LikerListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/{id}/likers [get]
func (h *LikeHandler) GetLikers(w http.ResponseWriter, r *http.Request) {
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

	likers, nextCursor, total, err := h.likeService.GetLikers(r.Context(), tweetID, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get likers")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        likers,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
		"tweet_id":    tweetID,
	})
}

// ======================================================================
= Get Liked Tweets (User's likes)
// ======================================================================

// GetLikedTweets handles retrieving tweets liked by the authenticated user.
// @Summary Get liked tweets
// @Description Retrieves tweets liked by the authenticated user
// @Tags likes
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TweetListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/mine [get]
func (h *LikeHandler) GetLikedTweets(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.likeService.GetLikedTweets(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get liked tweets")
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

// GetLikedTweetIDs handles retrieving only the IDs of tweets liked by the user.
// @Summary Get liked tweet IDs
// @Description Retrieves only the IDs of tweets liked by the user
// @Tags likes
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.LikedIDsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/ids [get]
func (h *LikeHandler) GetLikedTweetIDs(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	ids, err := h.likeService.GetLikedTweetIDs(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get liked tweet IDs")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"tweet_ids": ids,
		"count":     len(ids),
	})
}

// ======================================================================
= Get Like Status for Multiple Tweets (Bulk)
// ======================================================================

// GetLikeStatuses handles retrieving like statuses for multiple tweets.
// @Summary Get like statuses for multiple tweets
// @Description Checks like status for a list of tweet IDs (bulk)
// @Tags likes
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.BulkLikeStatusRequest true "Tweet IDs"
// @Success 200 {object} dto.BulkLikeStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/bulk-status [post]
func (h *LikeHandler) GetLikeStatuses(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.BulkLikeStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	statuses, err := h.likeService.GetLikeStatuses(r.Context(), userID, req.TweetIDs)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get like statuses")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"statuses": statuses,
	})
}

// ======================================================================
= Get Like Stats (User)
// ======================================================================

// GetUserLikeStats handles retrieving like statistics for the user.
// @Summary Get user like stats
// @Description Retrieves like statistics for the authenticated user
// @Tags likes
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UserLikeStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/stats [get]
func (h *LikeHandler) GetUserLikeStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	stats, err := h.likeService.GetUserLikeStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get like stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Get Most Liked Tweets (Trending)
// ======================================================================

// GetMostLikedTweets handles retrieving the most liked tweets.
// @Summary Get most liked tweets
// @Description Retrieves the most popular tweets by like count
// @Tags likes
// @Produce json
// @Param limit query int false "Items per page (default 10, max 50)"
// @Param since query string false "Since timestamp"
// @Success 200 {object} dto.TweetListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/likes/most-liked [get]
func (h *LikeHandler) GetMostLikedTweets(w http.ResponseWriter, r *http.Request) {
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

	tweets, err := h.likeService.GetMostLikedTweets(r.Context(), limit, sinceTime, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get most liked tweets")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  tweets,
		"limit": limit,
		"since": sinceTime.Format(time.RFC3339),
	})
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListLikes handles admin listing of all likes.
// @Summary Admin list likes
// @Description Lists all likes for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param user_id query string false "Filter by user ID"
// @Param tweet_id query string false "Filter by tweet ID"
// @Success 200 {object} dto.LikeListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/likes [get]
func (h *LikeHandler) AdminListLikes(w http.ResponseWriter, r *http.Request) {
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

	likes, nextCursor, total, err := h.likeService.AdminListLikes(r.Context(), cursor, limit, userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list likes")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        likes,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeleteLike handles admin deletion of a like.
// @Summary Admin delete like
// @Description Deletes a like (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Like ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/likes/{id} [delete]
func (h *LikeHandler) AdminDeleteLike(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	likeID := vars["id"]
	if likeID == "" {
		h.sendError(w, http.StatusBadRequest, "Like ID required", nil)
		return
	}

	if err := h.likeService.AdminDeleteLike(r.Context(), likeID); err != nil {
		h.handleServiceError(w, err, "Failed to delete like")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Like deleted successfully",
	})
}

// AdminGetLikeStats handles retrieving global like statistics.
// @Summary Admin get like stats
// @Description Retrieves global like statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.GlobalLikeStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/likes/stats [get]
func (h *LikeHandler) AdminGetLikeStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	stats, err := h.likeService.AdminGetLikeStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get like stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *LikeHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *LikeHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *LikeHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *LikeHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrLikeNotFound):
		h.sendError(w, http.StatusNotFound, "Like not found", nil)
	case errors.Is(err, service.ErrAlreadyLiked):
		h.sendError(w, http.StatusConflict, "Tweet already liked", nil)
	case errors.Is(err, service.ErrTweetNotFound):
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
	case errors.Is(err, service.ErrInvalidLikeID):
		h.sendError(w, http.StatusBadRequest, "Invalid like ID", nil)
	case errors.Is(err, service.ErrLikeDisabled):
		h.sendError(w, http.StatusBadRequest, "Liking is disabled for this tweet", nil)
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

// HealthCheck returns the health status of the like handler.
func (h *LikeHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "like_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}