// backend/internal/handler/user_handler.go
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

// UserHandler handles all user-related HTTP endpoints.
type UserHandler struct {
	userService    service.UserService
	followService  service.FollowService
	tweetService   service.TweetService
	notificationService service.NotificationService
	searchService  service.SearchService
	log            *logrus.Entry
}

// NewUserHandler creates a new user handler.
func NewUserHandler(
	userService service.UserService,
	followService service.FollowService,
	tweetService service.TweetService,
	notificationService service.NotificationService,
	searchService service.SearchService,
) *UserHandler {
	return &UserHandler{
		userService:    userService,
		followService:  followService,
		tweetService:   tweetService,
		notificationService: notificationService,
		searchService:  searchService,
		log:            logger.WithField("handler", "user"),
	}
}

// ======================================================================
// Get Profile
// ======================================================================

// GetProfile handles retrieving a user's profile.
// @Summary Get user profile
// @Description Retrieves a user's profile by username or ID
// @Tags users
// @Produce json
// @Param identifier path string true "Username or User ID"
// @Success 200 {object} dto.UserProfileResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{identifier} [get]
func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "User identifier required", nil)
		return
	}

	// Get current user ID if authenticated
	currentUserID, _ := middleware.GetUserID(r.Context())

	profile, err := h.userService.GetProfile(r.Context(), identifier, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get profile")
		return
	}

	h.sendSuccess(w, http.StatusOK, profile)
}

// ======================================================================
// Update Profile
// ======================================================================

// UpdateProfile handles updating a user's profile.
// @Summary Update user profile
// @Description Updates the authenticated user's profile
// @Tags users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdateProfileRequest true "Profile updates"
// @Success 200 {object} dto.UserProfileResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/profile [put]
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	profile, err := h.userService.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update profile")
		return
	}

	h.sendSuccess(w, http.StatusOK, profile)
}

// ======================================================================
= Follow/Unfollow
// ======================================================================

// Follow handles following a user.
// @Summary Follow a user
// @Description Follows the specified user
// @Tags users
// @Security BearerAuth
// @Param id path string true "User ID to follow"
// @Success 200 {object} dto.FollowResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/follow [post]
func (h *UserHandler) Follow(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	targetID := vars["id"]
	if targetID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if userID == targetID {
		h.sendError(w, http.StatusBadRequest, "Cannot follow yourself", nil)
		return
	}

	result, err := h.followService.Follow(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to follow user")
		return
	}

	h.sendSuccess(w, http.StatusOK, result)
}

// Unfollow handles unfollowing a user.
// @Summary Unfollow a user
// @Description Unfollows the specified user
// @Tags users
// @Security BearerAuth
// @Param id path string true "User ID to unfollow"
// @Success 200 {object} dto.FollowResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/unfollow [post]
func (h *UserHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	targetID := vars["id"]
	if targetID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if userID == targetID {
		h.sendError(w, http.StatusBadRequest, "Cannot unfollow yourself", nil)
		return
	}

	result, err := h.followService.Unfollow(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to unfollow user")
		return
	}

	h.sendSuccess(w, http.StatusOK, result)
}

// ======================================================================
= Get Followers / Following
// ======================================================================

// GetFollowers handles retrieving a user's followers.
// @Summary Get user followers
// @Description Retrieves the list of users following a user
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FollowerListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/followers [get]
func (h *UserHandler) GetFollowers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	followers, nextCursor, total, err := h.followService.GetFollowers(r.Context(), userID, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get followers")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        followers,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// GetFollowing handles retrieving users a user is following.
// @Summary Get users a user is following
// @Description Retrieves the list of users a user is following
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FollowerListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/following [get]
func (h *UserHandler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	following, nextCursor, total, err := h.followService.GetFollowing(r.Context(), userID, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get following")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        following,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get User Tweets
// ======================================================================

// GetUserTweets handles retrieving a user's tweets.
// @Summary Get user tweets
// @Description Retrieves tweets from a specific user
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param include_replies query bool false "Include replies"
// @Success 200 {object} dto.TweetListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/tweets [get]
func (h *UserHandler) GetUserTweets(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	includeReplies, _ := strconv.ParseBool(r.URL.Query().Get("include_replies"))

	currentUserID, _ := middleware.GetUserID(r.Context())

	tweets, nextCursor, total, err := h.tweetService.GetUserTweetsWithStats(r.Context(), userID, cursor, limit, includeReplies, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user tweets")
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
= Get User Stats
// ======================================================================

// GetUserStats handles retrieving a user's statistics.
// @Summary Get user statistics
// @Description Retrieves stats like tweet count, follower count, etc.
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.UserStatsResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/stats [get]
func (h *UserHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	stats, err := h.userService.GetUserStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Search Users
// ======================================================================

// SearchUsers handles searching for users.
// @Summary Search users
// @Description Searches for users by username or full name
// @Tags users
// @Produce json
// @Param q query string true "Search query"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.UserSearchResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/search [get]
func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
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

	currentUserID, _ := middleware.GetUserID(r.Context())

	results, nextCursor, total, err := h.searchService.SearchUsers(r.Context(), query, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to search users")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        results,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Suggestions
// ======================================================================

// GetSuggestions handles retrieving user suggestions.
// @Summary Get suggested users
// @Description Retrieves suggested users to follow
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Number of suggestions (default 10, max 50)"
// @Success 200 {object} dto.SuggestionsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/suggestions [get]
func (h *UserHandler) GetSuggestions(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	suggestions, err := h.followService.GetSuggestions(r.Context(), userID, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get suggestions")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"suggestions": suggestions,
		"limit":       limit,
	})
}

// ======================================================================
= Get Mutual Follows
// ======================================================================

// GetMutualFollows handles retrieving mutual follows.
// @Summary Get mutual follows
// @Description Retrieves users that both you and another user follow
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FollowerListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/mutual [get]
func (h *UserHandler) GetMutualFollows(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	targetID := vars["id"]
	if targetID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	mutual, nextCursor, total, err := h.followService.GetMutualFollows(r.Context(), userID, targetID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get mutual follows")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        mutual,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Check Follow Status
// ======================================================================

// CheckFollowStatus handles checking if a user follows another.
// @Summary Check follow status
// @Description Checks if the authenticated user follows another user
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID to check"
// @Success 200 {object} dto.FollowStatusResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/follow-status [get]
func (h *UserHandler) CheckFollowStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	targetID := vars["id"]
	if targetID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	isFollowing, isMutual, err := h.followService.CheckFollowStatus(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check follow status")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"following": isFollowing,
		"mutual":    isMutual,
		"user_id":   targetID,
	})
}

// ======================================================================
= Get Online Status
// ======================================================================

// GetOnlineStatus handles checking if a user is online.
// @Summary Check online status
// @Description Checks if a user is currently online
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.OnlineStatusResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/online [get]
func (h *UserHandler) GetOnlineStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	// This would need to check the WebSocket hub
	// For now, return a placeholder
	// In production, inject the WebSocket hub to check online status

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"user_id":   userID,
		"is_online": false,
		"last_seen": time.Now().UTC().Format(time.RFC3339),
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *UserHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *UserHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *UserHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *UserHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrUserSuspended):
		h.sendError(w, http.StatusForbidden, "User is suspended", nil)
	case errors.Is(err, service.ErrUserInactive):
		h.sendError(w, http.StatusForbidden, "User is inactive", nil)
	case errors.Is(err, service.ErrAlreadyFollowing):
		h.sendError(w, http.StatusConflict, "Already following this user", nil)
	case errors.Is(err, service.ErrNotFollowing):
		h.sendError(w, http.StatusBadRequest, "Not following this user", nil)
	case errors.Is(err, service.ErrCannotFollowSelf):
		h.sendError(w, http.StatusBadRequest, "Cannot follow yourself", nil)
	case errors.Is(err, context.Canceled):
		h.sendError(w, http.StatusRequestTimeout, "Request cancelled", nil)
	case errors.Is(err, context.DeadlineExceeded):
		h.sendError(w, http.StatusGatewayTimeout, "Request timed out", nil)
	default:
		h.log.WithError(err).Error(defaultMsg)
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
} 