// backend/internal/handler/follow_handler.go
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

// FollowHandler handles all follow-related HTTP endpoints.
type FollowHandler struct {
	followService       service.FollowService
	userService         service.UserService
	notificationService service.NotificationService
	log                 *logrus.Entry
}

// NewFollowHandler creates a new follow handler.
func NewFollowHandler(
	followService service.FollowService,
	userService service.UserService,
	notificationService service.NotificationService,
) *FollowHandler {
	return &FollowHandler{
		followService:       followService,
		userService:         userService,
		notificationService: notificationService,
		log:                 logger.WithField("handler", "follow"),
	}
}

// ======================================================================
// Follow/Unfollow
// ======================================================================

// Follow handles following a user.
// @Summary Follow a user
// @Description Follows a user
// @Tags follows
// @Security BearerAuth
// @Param id path string true "User ID to follow"
// @Success 200 {object} dto.FollowResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/{id} [post]
func (h *FollowHandler) Follow(w http.ResponseWriter, r *http.Request) {
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

	// Get updated counts
	followers, _ := h.followService.GetFollowCounts(r.Context(), targetID)
	following, _ := h.followService.GetFollowCounts(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"following":       result.Following,
		"followee_id":     targetID,
		"follower_id":     userID,
		"follower_count":  followers.Followers,
		"following_count": following.Following,
		"timestamp":       time.Now().Unix(),
	})
}

// Unfollow handles unfollowing a user.
// @Summary Unfollow a user
// @Description Unfollows a user
// @Tags follows
// @Security BearerAuth
// @Param id path string true "User ID to unfollow"
// @Success 200 {object} dto.FollowResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/{id} [delete]
func (h *FollowHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
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

	// Get updated counts
	followers, _ := h.followService.GetFollowCounts(r.Context(), targetID)
	following, _ := h.followService.GetFollowCounts(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"following":       result.Following,
		"followee_id":     targetID,
		"follower_id":     userID,
		"follower_count":  followers.Followers,
		"following_count": following.Following,
		"timestamp":       time.Now().Unix(),
	})
}

// ToggleFollow handles toggling follow status.
// @Summary Toggle follow
// @Description Toggles follow status on a user
// @Tags follows
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} dto.FollowResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/{id}/toggle [post]
func (h *FollowHandler) ToggleFollow(w http.ResponseWriter, r *http.Request) {
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

	// Check if already following
	isFollowing, err := h.followService.IsFollowing(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check follow status")
		return
	}

	var result *dto.FollowResponse
	if isFollowing {
		result, err = h.followService.Unfollow(r.Context(), userID, targetID)
	} else {
		result, err = h.followService.Follow(r.Context(), userID, targetID)
	}

	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle follow")
		return
	}

	// Get updated counts
	followers, _ := h.followService.GetFollowCounts(r.Context(), targetID)
	following, _ := h.followService.GetFollowCounts(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"following":       result.Following,
		"followee_id":     targetID,
		"follower_id":     userID,
		"follower_count":  followers.Followers,
		"following_count": following.Following,
		"timestamp":       time.Now().Unix(),
	})
}

// ======================================================================
= Check Follow Status
// ======================================================================

// CheckFollowStatus handles checking if a user is following another.
// @Summary Check follow status
// @Description Checks if the authenticated user is following another user
// @Tags follows
// @Security BearerAuth
// @Param id path string true "User ID to check"
// @Success 200 {object} dto.FollowStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/{id}/status [get]
func (h *FollowHandler) CheckFollowStatus(w http.ResponseWriter, r *http.Request) {
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

	// Get counts
	followers, _ := h.followService.GetFollowCounts(r.Context(), targetID)
	following, _ := h.followService.GetFollowCounts(r.Context(), targetID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"following":      isFollowing,
		"mutual":         isMutual,
		"user_id":        targetID,
		"follower_count": followers.Followers,
		"following_count": following.Following,
	})
}

// ======================================================================
= Get Followers
// ======================================================================

// GetFollowers handles retrieving a user's followers.
// @Summary Get user followers
// @Description Retrieves the list of users following a user
// @Tags follows
// @Produce json
// @Param id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FollowerListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/{id}/followers [get]
func (h *FollowHandler) GetFollowers(w http.ResponseWriter, r *http.Request) {
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
		"user_id":     userID,
	})
}

// ======================================================================
= Get Following
// ======================================================================

// GetFollowing handles retrieving users a user is following.
// @Summary Get users a user is following
// @Description Retrieves the list of users a user is following
// @Tags follows
// @Produce json
// @Param id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FollowerListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/{id}/following [get]
func (h *FollowHandler) GetFollowing(w http.ResponseWriter, r *http.Request) {
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
		"user_id":     userID,
	})
}

// ======================================================================
= Get Mutual Follows
// ======================================================================

// GetMutualFollows handles retrieving mutual follows between two users.
// @Summary Get mutual follows
// @Description Retrieves users that both users follow
// @Tags follows
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.FollowerListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/{id}/mutual [get]
func (h *FollowHandler) GetMutualFollows(w http.ResponseWriter, r *http.Request) {
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
		"user_id":     targetID,
	})
}

// ======================================================================
= Get Follow Counts
// ======================================================================

// GetFollowCounts handles retrieving follower and following counts.
// @Summary Get follow counts
// @Description Retrieves follower and following counts for a user
// @Tags follows
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.FollowCountsResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/{id}/counts [get]
func (h *FollowHandler) GetFollowCounts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	counts, err := h.followService.GetFollowCounts(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get follow counts")
		return
	}

	h.sendSuccess(w, http.StatusOK, counts)
}

// ======================================================================
= Get Follow Suggestions
// ======================================================================

// GetFollowSuggestions handles retrieving suggested users to follow.
// @Summary Get follow suggestions
// @Description Retrieves suggested users to follow for the authenticated user
// @Tags follows
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Items per page (default 10, max 50)"
// @Success 200 {object} dto.SuggestionListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/suggestions [get]
func (h *FollowHandler) GetFollowSuggestions(w http.ResponseWriter, r *http.Request) {
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
		h.handleServiceError(w, err, "Failed to get follow suggestions")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  suggestions,
		"limit": limit,
	})
}

// ======================================================================
= Get User Follow Stats
// ======================================================================

// GetUserFollowStats handles retrieving follow statistics for the user.
// @Summary Get user follow stats
// @Description Retrieves follow statistics for the authenticated user
// @Tags follows
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UserFollowStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/stats [get]
func (h *FollowHandler) GetUserFollowStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	stats, err := h.followService.GetUserFollowStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get follow stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Check If Follows (Public)
// ======================================================================

// CheckIfFollows handles checking if user A follows user B (public).
// @Summary Check if user follows
// @Description Checks if user A follows user B
// @Tags follows
// @Produce json
// @Param follower_id query string true "Follower user ID"
// @Param followee_id query string true "Followee user ID"
// @Success 200 {object} dto.FollowStatusPublicResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/follows/check [get]
func (h *FollowHandler) CheckIfFollows(w http.ResponseWriter, r *http.Request) {
	followerID := r.URL.Query().Get("follower_id")
	followeeID := r.URL.Query().Get("followee_id")

	if followerID == "" || followeeID == "" {
		h.sendError(w, http.StatusBadRequest, "follower_id and followee_id are required", nil)
		return
	}

	isFollowing, err := h.followService.IsFollowing(r.Context(), followerID, followeeID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check follow status")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"following":   isFollowing,
		"follower_id": followerID,
		"followee_id": followeeID,
	})
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListFollows handles admin listing of all follows.
// @Summary Admin list follows
// @Description Lists all follow relationships for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param follower_id query string false "Filter by follower ID"
// @Param followee_id query string false "Filter by followee ID"
// @Success 200 {object} dto.FollowListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/follows [get]
func (h *FollowHandler) AdminListFollows(w http.ResponseWriter, r *http.Request) {
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
	followerID := r.URL.Query().Get("follower_id")
	followeeID := r.URL.Query().Get("followee_id")

	follows, nextCursor, total, err := h.followService.AdminListFollows(r.Context(), cursor, limit, followerID, followeeID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list follows")
		return
	}

	// Build response
	followResponses := make([]*dto.FollowAdminResponse, 0, len(follows))
	for _, f := range follows {
		follower, _ := h.userService.GetUserByID(r.Context(), f.FollowerID)
		followee, _ := h.userService.GetUserByID(r.Context(), f.FolloweeID)
		followResponses = append(followResponses, &dto.FollowAdminResponse{
			ID:           f.ID,
			FollowerID:   f.FollowerID,
			FolloweeID:   f.FolloweeID,
			FollowerUsername: func() string {
				if follower != nil {
					return follower.Username
				}
				return ""
			}(),
			FolloweeUsername: func() string {
				if followee != nil {
					return followee.Username
				}
				return ""
			}(),
			CreatedAt: f.CreatedAt,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        followResponses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeleteFollow handles admin deletion of a follow relationship.
// @Summary Admin delete follow
// @Description Deletes a follow relationship (admin only)
// @Tags admin
// @Security BearerAuth
// @Param follower_id path string true "Follower user ID"
// @Param followee_id path string true "Followee user ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/follows/{follower_id}/{followee_id} [delete]
func (h *FollowHandler) AdminDeleteFollow(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	followerID := vars["follower_id"]
	followeeID := vars["followee_id"]

	if followerID == "" || followeeID == "" {
		h.sendError(w, http.StatusBadRequest, "follower_id and followee_id are required", nil)
		return
	}

	if err := h.followService.AdminDeleteFollow(r.Context(), followerID, followeeID); err != nil {
		h.handleServiceError(w, err, "Failed to delete follow")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Follow relationship deleted successfully",
	})
}

// AdminGetFollowStats handles retrieving global follow statistics.
// @Summary Admin get follow stats
// @Description Retrieves global follow statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.GlobalFollowStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/follows/stats [get]
func (h *FollowHandler) AdminGetFollowStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	stats, err := h.followService.AdminGetFollowStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get follow stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *FollowHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *FollowHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *FollowHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *FollowHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrAlreadyFollowing):
		h.sendError(w, http.StatusConflict, "Already following this user", nil)
	case errors.Is(err, service.ErrNotFollowing):
		h.sendError(w, http.StatusBadRequest, "Not following this user", nil)
	case errors.Is(err, service.ErrCannotFollowSelf):
		h.sendError(w, http.StatusBadRequest, "Cannot follow yourself", nil)
	case errors.Is(err, service.ErrUserSuspended):
		h.sendError(w, http.StatusForbidden, "User is suspended", nil)
	case errors.Is(err, service.ErrUserInactive):
		h.sendError(w, http.StatusForbidden, "User is inactive", nil)
	case errors.Is(err, service.ErrFollowNotFound):
		h.sendError(w, http.StatusNotFound, "Follow relationship not found", nil)
	case errors.Is(err, service.ErrInvalidFollowID):
		h.sendError(w, http.StatusBadRequest, "Invalid follow ID", nil)
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

// HealthCheck returns the health status of the follow handler.
func (h *FollowHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "follow_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}