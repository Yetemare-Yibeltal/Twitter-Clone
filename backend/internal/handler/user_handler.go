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
	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/pkg/logger"
)

// UserHandler handles all user-related HTTP endpoints.
type UserHandler struct {
	userService         service.UserService
	followService       service.FollowService
	tweetService        service.TweetService
	notificationService service.NotificationService
	searchService       service.SearchService
	userRepo            interfaces.UserRepository
	followRepo          interfaces.FollowRepository
	redisAdapter        adapter.RedisAdapter
	log                 *logrus.Entry
}

// NewUserHandler creates a new user handler.
func NewUserHandler(
	userService service.UserService,
	followService service.FollowService,
	tweetService service.TweetService,
	notificationService service.NotificationService,
	searchService service.SearchService,
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	redisAdapter adapter.RedisAdapter,
) *UserHandler {
	return &UserHandler{
		userService:         userService,
		followService:       followService,
		tweetService:        tweetService,
		notificationService: notificationService,
		searchService:       searchService,
		userRepo:            userRepo,
		followRepo:          followRepo,
		redisAdapter:        redisAdapter,
		log:                 logger.WithField("handler", "user"),
	}
}

// ======================================================================
// Profile Operations
// ======================================================================

// GetProfile handles retrieving a user's profile.
// @Summary Get user profile
// @Description Retrieves a user's profile by username or ID
// @Tags users
// @Produce json
// @Param identifier path string true "Username or User ID"
// @Success 200 {object} dto.UserProfileResponse
// @Failure 400 {object} dto.ErrorResponse
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

	// Check cache first
	cacheKey := fmt.Sprintf("user_profile:%s", identifier)
	if h.redisAdapter != nil {
		var cached dto.UserProfileResponse
		if err := h.redisAdapter.GetJSON(r.Context(), cacheKey, &cached); err == nil {
			h.sendSuccess(w, http.StatusOK, cached)
			return
		}
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	profile, err := h.userService.GetProfile(r.Context(), identifier, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get profile")
		return
	}

	// Cache for 1 minute
	if h.redisAdapter != nil {
		_ = h.redisAdapter.CacheSet(r.Context(), cacheKey, profile, 1*time.Minute)
	}

	h.sendSuccess(w, http.StatusOK, profile)
}

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

	// Invalidate cache
	if h.redisAdapter != nil {
		_ = h.redisAdapter.Delete(r.Context(), fmt.Sprintf("user_profile:%s", userID))
		_ = h.redisAdapter.Delete(r.Context(), fmt.Sprintf("user_profile:%s", profile.Username))
	}

	h.sendSuccess(w, http.StatusOK, profile)
}

// UploadAvatar handles uploading a user's avatar.
// @Summary Upload user avatar
// @Description Uploads a new avatar image for the authenticated user
// @Tags users
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param avatar formData file true "Avatar image file"
// @Success 200 {object} dto.UploadAvatarResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/avatar [post]
func (h *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Parse multipart form (max 5MB)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse form", nil)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Avatar file is required", nil)
		return
	}
	defer file.Close()

	// Validate file type
	contentType := header.Header.Get("Content-Type")
	allowedTypes := []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
	allowed := false
	for _, t := range allowedTypes {
		if t == contentType {
			allowed = true
			break
		}
	}
	if !allowed {
		h.sendError(w, http.StatusBadRequest, "Only JPEG, PNG, GIF, and WEBP images are allowed", nil)
		return
	}

	// Validate file size (max 5MB)
	if header.Size > 5<<20 {
		h.sendError(w, http.StatusBadRequest, "File size must be less than 5MB", nil)
		return
	}

	avatarURL, err := h.userService.UploadAvatar(r.Context(), userID, file, header.Filename)
	if err != nil {
		h.handleServiceError(w, err, "Failed to upload avatar")
		return
	}

	// Invalidate cache
	if h.redisAdapter != nil {
		_ = h.redisAdapter.Delete(r.Context(), fmt.Sprintf("user_profile:%s", userID))
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"avatar_url": avatarURL,
		"message":    "Avatar uploaded successfully",
	})
}

// ======================================================================
// Follow/Unfollow
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

	// Invalidate caches
	if h.redisAdapter != nil {
		_ = h.redisAdapter.Delete(r.Context(), fmt.Sprintf("user_profile:%s", targetID))
		_ = h.redisAdapter.Delete(r.Context(), fmt.Sprintf("user_profile:%s", userID))
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

	// Invalidate caches
	if h.redisAdapter != nil {
		_ = h.redisAdapter.Delete(r.Context(), fmt.Sprintf("user_profile:%s", targetID))
		_ = h.redisAdapter.Delete(r.Context(), fmt.Sprintf("user_profile:%s", userID))
	}

	h.sendSuccess(w, http.StatusOK, result)
}

// ======================================================================
= Followers / Following Lists
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
= User Tweets
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

// GetUserLikedTweets handles retrieving a user's liked tweets.
// @Summary Get user liked tweets
// @Description Retrieves tweets liked by a specific user
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.TweetListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/likes [get]
func (h *UserHandler) GetUserLikedTweets(w http.ResponseWriter, r *http.Request) {
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

	tweets, nextCursor, total, err := h.tweetService.GetUserLikedTweets(r.Context(), userID, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user liked tweets")
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
= User Stats
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

	// Check cache
	cacheKey := fmt.Sprintf("user_stats:%s", userID)
	if h.redisAdapter != nil {
		var cached dto.UserStatsResponse
		if err := h.redisAdapter.GetJSON(r.Context(), cacheKey, &cached); err == nil {
			h.sendSuccess(w, http.StatusOK, cached)
			return
		}
	}

	stats, err := h.userService.GetUserStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user stats")
		return
	}

	// Cache for 5 minutes
	if h.redisAdapter != nil {
		_ = h.redisAdapter.CacheSet(r.Context(), cacheKey, stats, 5*time.Minute)
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Search
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
= Suggestions
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

	// Check cache
	cacheKey := fmt.Sprintf("suggestions:%s:%d", userID, limit)
	if h.redisAdapter != nil {
		var cached []*dto.UserSuggestionResponse
		if err := h.redisAdapter.GetJSON(r.Context(), cacheKey, &cached); err == nil {
			h.sendSuccess(w, http.StatusOK, map[string]interface{}{
				"suggestions": cached,
				"limit":       limit,
			})
			return
		}
	}

	suggestions, err := h.followService.GetSuggestions(r.Context(), userID, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get suggestions")
		return
	}

	// Cache for 10 minutes
	if h.redisAdapter != nil {
		_ = h.redisAdapter.CacheSet(r.Context(), cacheKey, suggestions, 10*time.Minute)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"suggestions": suggestions,
		"limit":       limit,
	})
}

// ======================================================================
= Mutual Follows
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
= Follow Status
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
= Online Status
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

	// Check if user exists
	_, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			h.sendError(w, http.StatusNotFound, "User not found", nil)
			return
		}
		h.handleServiceError(w, err, "Failed to check user")
		return
	}

	// Check online status from Redis or WebSocket hub
	isOnline := false
	if h.redisAdapter != nil {
		key := fmt.Sprintf("user_online:%s", userID)
		val, err := h.redisAdapter.Get(r.Context(), key)
		if err == nil && val == "online" {
			isOnline = true
		}
	}

	lastSeen, err := h.userService.GetLastSeen(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get last seen")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"user_id":   userID,
		"is_online": isOnline,
		"last_seen": lastSeen.Format(time.RFC3339),
	})
}

// ======================================================================
= Block/Unblock
// ======================================================================

// Block handles blocking a user.
// @Summary Block a user
// @Description Blocks the specified user
// @Tags users
// @Security BearerAuth
// @Param id path string true "User ID to block"
// @Success 200 {object} dto.BlockResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/block [post]
func (h *UserHandler) Block(w http.ResponseWriter, r *http.Request) {
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
		h.sendError(w, http.StatusBadRequest, "Cannot block yourself", nil)
		return
	}

	err = h.followService.Block(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to block user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked": true,
		"user_id": targetID,
		"message": "User blocked successfully",
	})
}

// Unblock handles unblocking a user.
// @Summary Unblock a user
// @Description Unblocks the specified user
// @Tags users
// @Security BearerAuth
// @Param id path string true "User ID to unblock"
// @Success 200 {object} dto.BlockResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/unblock [post]
func (h *UserHandler) Unblock(w http.ResponseWriter, r *http.Request) {
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

	err = h.followService.Unblock(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to unblock user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked": false,
		"user_id": targetID,
		"message": "User unblocked successfully",
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
	case errors.Is(err, service.ErrAlreadyBlocked):
		h.sendError(w, http.StatusConflict, "User is already blocked", nil)
	case errors.Is(err, service.ErrNotBlocked):
		h.sendError(w, http.StatusBadRequest, "User is not blocked", nil)
	case errors.Is(err, service.ErrInvalidUsername):
		h.sendError(w, http.StatusBadRequest, "Invalid username", nil)
	case errors.Is(err, service.ErrInvalidEmail):
		h.sendError(w, http.StatusBadRequest, "Invalid email", nil)
	case errors.Is(err, service.ErrProfileUpdateFailed):
		h.sendError(w, http.StatusBadRequest, "Failed to update profile", nil)
	case errors.Is(err, context.Canceled):
		h.sendError(w, http.StatusRequestTimeout, "Request cancelled", nil)
	case errors.Is(err, context.DeadlineExceeded):
		h.sendError(w, http.StatusGatewayTimeout, "Request timed out", nil)
	default:
		h.log.WithError(err).Error(defaultMsg)
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}