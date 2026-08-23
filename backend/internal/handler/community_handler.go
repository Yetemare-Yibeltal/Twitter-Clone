// backend/internal/handler/community_handler.go
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

// CommunityHandler handles all community-related HTTP endpoints.
type CommunityHandler struct {
	communityService service.CommunityService
	tweetService     service.TweetService
	log              *logrus.Entry
}

// NewCommunityHandler creates a new community handler.
func NewCommunityHandler(
	communityService service.CommunityService,
	tweetService service.TweetService,
) *CommunityHandler {
	return &CommunityHandler{
		communityService: communityService,
		tweetService:     tweetService,
		log:              logger.WithField("handler", "community"),
	}
}

// ======================================================================
// Community CRUD
// ======================================================================

// CreateCommunity handles creating a new community.
// @Summary Create community
// @Description Creates a new community
// @Tags communities
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreateCommunityRequest true "Community details"
// @Success 201 {object} dto.CommunityResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities [post]
func (h *CommunityHandler) CreateCommunity(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.CreateCommunityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	community, err := h.communityService.CreateCommunity(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to create community")
		return
	}

	h.sendSuccess(w, http.StatusCreated, community)
}

// GetCommunity handles retrieving a community by ID or slug.
// @Summary Get community
// @Description Retrieves a community by ID or slug
// @Tags communities
// @Produce json
// @Param identifier path string true "Community ID or slug"
// @Success 200 {object} dto.CommunityDetailResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier} [get]
func (h *CommunityHandler) GetCommunity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	community, err := h.communityService.GetCommunity(r.Context(), identifier, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get community")
		return
	}

	h.sendSuccess(w, http.StatusOK, community)
}

// UpdateCommunity handles updating a community.
// @Summary Update community
// @Description Updates a community (admin/owner only)
// @Tags communities
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param identifier path string true "Community ID or slug"
// @Param request body dto.UpdateCommunityRequest true "Community updates"
// @Success 200 {object} dto.CommunityResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier} [put]
func (h *CommunityHandler) UpdateCommunity(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	var req dto.UpdateCommunityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	community, err := h.communityService.UpdateCommunity(r.Context(), identifier, userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update community")
		return
	}

	h.sendSuccess(w, http.StatusOK, community)
}

// DeleteCommunity handles deleting a community.
// @Summary Delete community
// @Description Deletes a community (admin/owner only)
// @Tags communities
// @Security BearerAuth
// @Param identifier path string true "Community ID or slug"
// @Success 204 "No Content"
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier} [delete]
func (h *CommunityHandler) DeleteCommunity(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	if err := h.communityService.DeleteCommunity(r.Context(), identifier, userID); err != nil {
		h.handleServiceError(w, err, "Failed to delete community")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ======================================================================
// List Communities
// ======================================================================

// ListCommunities handles listing communities with pagination and filters.
// @Summary List communities
// @Description Lists communities with pagination and filtering
// @Tags communities
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param search query string false "Search by name or description"
// @Param is_private query bool false "Filter by privacy"
// @Param sort_by query string false "Sort by (created_at, member_count, post_count)"
// @Param order query string false "Sort order (ASC, DESC)"
// @Success 200 {object} dto.CommunityListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities [get]
func (h *CommunityHandler) ListCommunities(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	search := r.URL.Query().Get("search")
	isPrivate := r.URL.Query().Get("is_private")
	sortBy := r.URL.Query().Get("sort_by")
	order := r.URL.Query().Get("order")

	currentUserID, _ := middleware.GetUserID(r.Context())

	communities, nextCursor, total, err := h.communityService.ListCommunities(
		r.Context(), cursor, limit, search, isPrivate, sortBy, order, currentUserID,
	)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list communities")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        communities,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// GetUserCommunities handles retrieving communities a user belongs to.
// @Summary Get user communities
// @Description Retrieves communities the authenticated user belongs to
// @Tags communities
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.CommunityListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/user [get]
func (h *CommunityHandler) GetUserCommunities(w http.ResponseWriter, r *http.Request) {
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

	communities, nextCursor, total, err := h.communityService.GetUserCommunities(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user communities")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        communities,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
// Membership Management
// ======================================================================

// JoinCommunity handles joining a community.
// @Summary Join community
// @Description Joins the authenticated user to a community
// @Tags communities
// @Security BearerAuth
// @Param identifier path string true "Community ID or slug"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/join [post]
func (h *CommunityHandler) JoinCommunity(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	if err := h.communityService.JoinCommunity(r.Context(), identifier, userID); err != nil {
		h.handleServiceError(w, err, "Failed to join community")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Joined community successfully",
	})
}

// LeaveCommunity handles leaving a community.
// @Summary Leave community
// @Description Leaves the authenticated user from a community
// @Tags communities
// @Security BearerAuth
// @Param identifier path string true "Community ID or slug"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/leave [post]
func (h *CommunityHandler) LeaveCommunity(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	if err := h.communityService.LeaveCommunity(r.Context(), identifier, userID); err != nil {
		h.handleServiceError(w, err, "Failed to leave community")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Left community successfully",
	})
}

// GetCommunityMembers handles retrieving community members.
// @Summary Get community members
// @Description Retrieves members of a community with pagination
// @Tags communities
// @Produce json
// @Param identifier path string true "Community ID or slug"
// @Param role query string false "Filter by role (owner, admin, moderator, member)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.MemberListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/members [get]
func (h *CommunityHandler) GetCommunityMembers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	role := r.URL.Query().Get("role")
	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	members, nextCursor, total, err := h.communityService.GetCommunityMembers(r.Context(), identifier, role, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get community members")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        members,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// UpdateMemberRole handles updating a member's role.
// @Summary Update member role
// @Description Updates a member's role in a community (admin/owner only)
// @Tags communities
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param identifier path string true "Community ID or slug"
// @Param user_id path string true "User ID"
// @Param request body dto.UpdateMemberRoleRequest true "Role update"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/members/{user_id}/role [put]
func (h *CommunityHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	adminUserID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}
	targetUserID := vars["user_id"]
	if targetUserID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	var req dto.UpdateMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.communityService.UpdateMemberRole(r.Context(), identifier, adminUserID, targetUserID, req.Role); err != nil {
		h.handleServiceError(w, err, "Failed to update member role")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Member role updated successfully",
	})
}

// RemoveMember handles removing a member from a community.
// @Summary Remove member
// @Description Removes a member from a community (admin/owner only)
// @Tags communities
// @Security BearerAuth
// @Param identifier path string true "Community ID or slug"
// @Param user_id path string true "User ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/members/{user_id} [delete]
func (h *CommunityHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	adminUserID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}
	targetUserID := vars["user_id"]
	if targetUserID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if err := h.communityService.RemoveMember(r.Context(), identifier, adminUserID, targetUserID); err != nil {
		h.handleServiceError(w, err, "Failed to remove member")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Member removed successfully",
	})
}

// ======================================================================
// Moderation - Bans
// ======================================================================

// BanUser handles banning a user from a community.
// @Summary Ban user
// @Description Bans a user from a community (admin/owner only)
// @Tags communities
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param identifier path string true "Community ID or slug"
// @Param user_id path string true "User ID"
// @Param request body dto.BanUserRequest true "Ban details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/ban/{user_id} [post]
func (h *CommunityHandler) BanUser(w http.ResponseWriter, r *http.Request) {
	adminUserID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}
	targetUserID := vars["user_id"]
	if targetUserID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	var req dto.BanUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.communityService.BanUser(r.Context(), identifier, adminUserID, targetUserID, req.Reason); err != nil {
		h.handleServiceError(w, err, "Failed to ban user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User banned successfully",
	})
}

// UnbanUser handles unbanning a user from a community.
// @Summary Unban user
// @Description Unbans a user from a community (admin/owner only)
// @Tags communities
// @Security BearerAuth
// @Param identifier path string true "Community ID or slug"
// @Param user_id path string true "User ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/unban/{user_id} [post]
func (h *CommunityHandler) UnbanUser(w http.ResponseWriter, r *http.Request) {
	adminUserID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}
	targetUserID := vars["user_id"]
	if targetUserID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if err := h.communityService.UnbanUser(r.Context(), identifier, adminUserID, targetUserID); err != nil {
		h.handleServiceError(w, err, "Failed to unban user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User unbanned successfully",
	})
}

// GetBannedUsers handles retrieving banned users.
// @Summary Get banned users
// @Description Retrieves banned users from a community (admin/owner only)
// @Tags communities
// @Security BearerAuth
// @Param identifier path string true "Community ID or slug"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.BannedUserListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/bans [get]
func (h *CommunityHandler) GetBannedUsers(w http.ResponseWriter, r *http.Request) {
	adminUserID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	bans, nextCursor, total, err := h.communityService.GetBannedUsers(r.Context(), identifier, adminUserID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get banned users")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        bans,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
// Community Posts
// ======================================================================

// AddPost handles adding a post to a community.
// @Summary Add post to community
// @Description Adds a tweet as a post to a community
// @Tags communities
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param identifier path string true "Community ID or slug"
// @Param request body dto.AddPostRequest true "Post details"
// @Success 201 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/posts [post]
func (h *CommunityHandler) AddPost(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	var req dto.AddPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.communityService.AddPost(r.Context(), identifier, userID, req.TweetID); err != nil {
		h.handleServiceError(w, err, "Failed to add post")
		return
	}

	h.sendSuccess(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Post added successfully",
	})
}

// RemovePost handles removing a post from a community.
// @Summary Remove post from community
// @Description Removes a post from a community (admin/moderator only)
// @Tags communities
// @Security BearerAuth
// @Param identifier path string true "Community ID or slug"
// @Param post_id path string true "Tweet ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/posts/{post_id} [delete]
func (h *CommunityHandler) RemovePost(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}
	postID := vars["post_id"]
	if postID == "" {
		h.sendError(w, http.StatusBadRequest, "Post ID required", nil)
		return
	}

	if err := h.communityService.RemovePost(r.Context(), identifier, userID, postID); err != nil {
		h.handleServiceError(w, err, "Failed to remove post")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Post removed successfully",
	})
}

// GetCommunityPosts handles retrieving posts from a community.
// @Summary Get community posts
// @Description Retrieves posts from a community with pagination
// @Tags communities
// @Produce json
// @Param identifier path string true "Community ID or slug"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.PostListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/posts [get]
func (h *CommunityHandler) GetCommunityPosts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	posts, nextCursor, total, err := h.communityService.GetCommunityPosts(r.Context(), identifier, cursor, limit, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get community posts")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        posts,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Community Stats
// ======================================================================

// GetCommunityStats handles retrieving community statistics.
// @Summary Get community stats
// @Description Retrieves statistics for a community
// @Tags communities
// @Produce json
// @Param identifier path string true "Community ID or slug"
// @Success 200 {object} dto.CommunityStatsResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/communities/{identifier}/stats [get]
func (h *CommunityHandler) GetCommunityStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	stats, err := h.communityService.GetCommunityStats(r.Context(), identifier)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get community stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListCommunities handles admin listing of all communities.
// @Summary Admin list communities
// @Description Lists all communities for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, deleted)"
// @Param search query string false "Search by name or description"
// @Success 200 {object} dto.CommunityListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/communities [get]
func (h *CommunityHandler) AdminListCommunities(w http.ResponseWriter, r *http.Request) {
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
	search := r.URL.Query().Get("search")

	communities, nextCursor, total, err := h.communityService.AdminListCommunities(r.Context(), cursor, limit, status, search)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list communities")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        communities,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeleteCommunity handles admin deletion of a community.
// @Summary Admin delete community
// @Description Deletes a community (admin only)
// @Tags admin
// @Security BearerAuth
// @Param identifier path string true "Community ID or slug"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/communities/{identifier} [delete]
func (h *CommunityHandler) AdminDeleteCommunity(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	identifier := vars["identifier"]
	if identifier == "" {
		h.sendError(w, http.StatusBadRequest, "Community identifier required", nil)
		return
	}

	if err := h.communityService.AdminDeleteCommunity(r.Context(), identifier); err != nil {
		h.handleServiceError(w, err, "Failed to delete community")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Community deleted successfully",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *CommunityHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *CommunityHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *CommunityHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *CommunityHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrCommunityNotFound):
		h.sendError(w, http.StatusNotFound, "Community not found", nil)
	case errors.Is(err, service.ErrCommunityDeleted):
		h.sendError(w, http.StatusNotFound, "Community has been deleted", nil)
	case errors.Is(err, service.ErrDuplicateSlug):
		h.sendError(w, http.StatusConflict, "Community slug already exists", nil)
	case errors.Is(err, service.ErrMemberNotFound):
		h.sendError(w, http.StatusNotFound, "Member not found", nil)
	case errors.Is(err, service.ErrMemberAlreadyExists):
		h.sendError(w, http.StatusConflict, "Already a member", nil)
	case errors.Is(err, service.ErrNotCommunityMember):
		h.sendError(w, http.StatusForbidden, "Not a member of this community", nil)
	case errors.Is(err, service.ErrNotCommunityAdmin):
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
	case errors.Is(err, service.ErrNotCommunityModerator):
		h.sendError(w, http.StatusForbidden, "Moderator access required", nil)
	case errors.Is(err, service.ErrCommunityPrivate):
		h.sendError(w, http.StatusForbidden, "Community is private", nil)
	case errors.Is(err, service.ErrUserAlreadyBanned):
		h.sendError(w, http.StatusConflict, "User is already banned", nil)
	case errors.Is(err, service.ErrBanNotFound):
		h.sendError(w, http.StatusNotFound, "Ban not found", nil)
	case errors.Is(err, service.ErrPostNotFound):
		h.sendError(w, http.StatusNotFound, "Post not found", nil)
	case errors.Is(err, service.ErrPostAlreadyExists):
		h.sendError(w, http.StatusConflict, "Post already exists in this community", nil)
	case errors.Is(err, service.ErrCannotRemoveOwner):
		h.sendError(w, http.StatusBadRequest, "Cannot remove the community owner", nil)
	case errors.Is(err, service.ErrCannotDemoteOwner):
		h.sendError(w, http.StatusBadRequest, "Cannot demote the community owner", nil)
	case errors.Is(err, service.ErrCommunityFull):
		h.sendError(w, http.StatusBadRequest, "Community has reached maximum members", nil)
	case errors.Is(err, service.ErrInvalidCommunityRole):
		h.sendError(w, http.StatusBadRequest, "Invalid community role", nil)
	case errors.Is(err, service.ErrCommunityNameRequired):
		h.sendError(w, http.StatusBadRequest, "Community name is required", nil)
	case errors.Is(err, service.ErrCommunityNameTooLong):
		h.sendError(w, http.StatusBadRequest, "Community name is too long", nil)
	case errors.Is(err, service.ErrCommunityDescriptionTooLong):
		h.sendError(w, http.StatusBadRequest, "Community description is too long", nil)
	case errors.Is(err, service.ErrCommunitySlugRequired):
		h.sendError(w, http.StatusBadRequest, "Community slug is required", nil)
	case errors.Is(err, service.ErrInvalidCommunitySlug):
		h.sendError(w, http.StatusBadRequest, "Invalid community slug format", nil)
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

// HealthCheck returns the health status of the community handler.
func (h *CommunityHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "community_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}