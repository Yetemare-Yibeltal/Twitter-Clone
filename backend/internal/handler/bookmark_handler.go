// backend/internal/handler/bookmark_handler.go
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

// BookmarkHandler handles all bookmark-related HTTP endpoints.
type BookmarkHandler struct {
	bookmarkService service.BookmarkService
	tweetService    service.TweetService
	log             *logrus.Entry
}

// NewBookmarkHandler creates a new bookmark handler.
func NewBookmarkHandler(
	bookmarkService service.BookmarkService,
	tweetService service.TweetService,
) *BookmarkHandler {
	return &BookmarkHandler{
		bookmarkService: bookmarkService,
		tweetService:    tweetService,
		log:             logger.WithField("handler", "bookmark"),
	}
}

// ======================================================================
// Bookmark/Unbookmark
// ======================================================================

// AddBookmark handles bookmarking a tweet.
// @Summary Bookmark a tweet
// @Description Adds a tweet to the user's bookmarks
// @Tags bookmarks
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 201 {object} dto.BookmarkResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks/{id} [post]
func (h *BookmarkHandler) AddBookmark(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.bookmarkService.AddBookmark(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to add bookmark")
		return
	}

	h.sendSuccess(w, http.StatusCreated, result)
}

// RemoveBookmark handles unbookmarking a tweet.
// @Summary Unbookmark a tweet
// @Description Removes a tweet from the user's bookmarks
// @Tags bookmarks
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.BookmarkResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks/{id} [delete]
func (h *BookmarkHandler) RemoveBookmark(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.bookmarkService.RemoveBookmark(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to remove bookmark")
		return
	}

	h.sendSuccess(w, http.StatusOK, result)
}

// ToggleBookmark handles toggling a bookmark on a tweet.
// @Summary Toggle bookmark
// @Description Toggles bookmark status on a tweet
// @Tags bookmarks
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.BookmarkResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks/{id}/toggle [post]
func (h *BookmarkHandler) ToggleBookmark(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.bookmarkService.ToggleBookmark(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle bookmark")
		return
	}

	h.sendSuccess(w, http.StatusOK, result)
}

// ======================================================================
= Check Bookmark Status
// ======================================================================

// CheckBookmarkStatus handles checking if a tweet is bookmarked.
// @Summary Check bookmark status
// @Description Checks if a tweet is bookmarked by the authenticated user
// @Tags bookmarks
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.BookmarkStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks/{id}/status [get]
func (h *BookmarkHandler) CheckBookmarkStatus(w http.ResponseWriter, r *http.Request) {
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

	isBookmarked, err := h.bookmarkService.IsBookmarked(r.Context(), userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check bookmark status")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"bookmarked": isBookmarked,
		"tweet_id":   tweetID,
	})
}

// ======================================================================
= Get Bookmarks
// ======================================================================

// GetBookmarks handles retrieving the user's bookmarked tweets.
// @Summary Get bookmarks
// @Description Retrieves all tweets bookmarked by the authenticated user
// @Tags bookmarks
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param include_tweet_details query bool false "Include full tweet details"
// @Success 200 {object} dto.BookmarkListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks [get]
func (h *BookmarkHandler) GetBookmarks(w http.ResponseWriter, r *http.Request) {
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
	includeTweetDetails, _ := strconv.ParseBool(r.URL.Query().Get("include_tweet_details"))

	bookmarks, nextCursor, total, err := h.bookmarkService.GetBookmarks(r.Context(), userID, cursor, limit, includeTweetDetails)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get bookmarks")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        bookmarks,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// GetBookmarkedTweetIDs handles retrieving only the bookmarked tweet IDs.
// @Summary Get bookmarked tweet IDs
// @Description Retrieves only the IDs of tweets bookmarked by the user
// @Tags bookmarks
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.BookmarkedIDsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks/ids [get]
func (h *BookmarkHandler) GetBookmarkedTweetIDs(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	ids, err := h.bookmarkService.GetBookmarkedTweetIDs(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get bookmarked tweet IDs")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"tweet_ids": ids,
		"count":     len(ids),
	})
}

// ======================================================================
= Get Bookmark Count
// ======================================================================

// GetBookmarkCount handles retrieving the bookmark count for a tweet.
// @Summary Get bookmark count
// @Description Retrieves the total number of bookmarks for a tweet
// @Tags bookmarks
// @Produce json
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.BookmarkCountResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks/count/{id} [get]
func (h *BookmarkHandler) GetBookmarkCount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	count, err := h.bookmarkService.GetBookmarkCount(r.Context(), tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get bookmark count")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"bookmark_count": count,
		"tweet_id":       tweetID,
	})
}

// ======================================================================
= Get Bookmarks by Tweet IDs (Bulk)
// ======================================================================

// GetBookmarksByTweetIDs handles retrieving bookmark statuses for multiple tweets.
// @Summary Get bookmark statuses for multiple tweets
// @Description Checks bookmark status for multiple tweet IDs (bulk)
// @Tags bookmarks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.BulkBookmarkStatusRequest true "Tweet IDs"
// @Success 200 {object} dto.BulkBookmarkStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/bookmarks/bulk-status [post]
func (h *BookmarkHandler) GetBookmarksByTweetIDs(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.BulkBookmarkStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	statuses, err := h.bookmarkService.GetBookmarksByTweetIDs(r.Context(), userID, req.TweetIDs)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get bookmark statuses")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"statuses": statuses,
	})
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListBookmarks handles admin listing of all bookmarks.
// @Summary Admin list bookmarks
// @Description Lists all bookmarks for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param user_id query string false "Filter by user ID"
// @Param tweet_id query string false "Filter by tweet ID"
// @Success 200 {object} dto.BookmarkListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/bookmarks [get]
func (h *BookmarkHandler) AdminListBookmarks(w http.ResponseWriter, r *http.Request) {
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

	bookmarks, nextCursor, total, err := h.bookmarkService.AdminListBookmarks(r.Context(), cursor, limit, userID, tweetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list bookmarks")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        bookmarks,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeleteBookmark handles admin deletion of a bookmark.
// @Summary Admin delete bookmark
// @Description Deletes a bookmark (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Bookmark ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/bookmarks/{id} [delete]
func (h *BookmarkHandler) AdminDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	bookmarkID := vars["id"]
	if bookmarkID == "" {
		h.sendError(w, http.StatusBadRequest, "Bookmark ID required", nil)
		return
	}

	if err := h.bookmarkService.AdminDeleteBookmark(r.Context(), bookmarkID); err != nil {
		h.handleServiceError(w, err, "Failed to delete bookmark")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Bookmark deleted successfully",
	})
}

// AdminGetBookmarkStats handles retrieving global bookmark statistics.
// @Summary Admin get bookmark stats
// @Description Retrieves global bookmark statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.GlobalBookmarkStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/bookmarks/stats [get]
func (h *BookmarkHandler) AdminGetBookmarkStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	stats, err := h.bookmarkService.AdminGetBookmarkStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get bookmark stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *BookmarkHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *BookmarkHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *BookmarkHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *BookmarkHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrBookmarkNotFound):
		h.sendError(w, http.StatusNotFound, "Bookmark not found", nil)
	case errors.Is(err, service.ErrAlreadyBookmarked):
		h.sendError(w, http.StatusConflict, "Tweet already bookmarked", nil)
	case errors.Is(err, service.ErrTweetNotFound):
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
	case errors.Is(err, service.ErrInvalidBookmarkID):
		h.sendError(w, http.StatusBadRequest, "Invalid bookmark ID", nil)
	case errors.Is(err, service.ErrBookmarkDisabled):
		h.sendError(w, http.StatusBadRequest, "Bookmarking is disabled for this tweet", nil)
	case errors.Is(err, service.ErrBookmarkNotFoundByUser):
		h.sendError(w, http.StatusNotFound, "Bookmark not found for this user", nil)
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

// HealthCheck returns the health status of the bookmark handler.
func (h *BookmarkHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "bookmark_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}