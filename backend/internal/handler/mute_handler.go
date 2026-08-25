// backend/internal/handler/mute_handler.go
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

// MuteHandler handles all mute-related HTTP endpoints.
type MuteHandler struct {
	muteService service.MuteService
	userService service.UserService
	log         *logrus.Entry
}

// NewMuteHandler creates a new mute handler.
func NewMuteHandler(
	muteService service.MuteService,
	userService service.UserService,
) *MuteHandler {
	return &MuteHandler{
		muteService: muteService,
		userService: userService,
		log:         logger.WithField("handler", "mute"),
	}
}

// ======================================================================
// Mute/Unmute
// ======================================================================

// MuteUser handles muting a user.
// @Summary Mute a user
// @Description Mutes a user to hide their content from the authenticated user's feed
// @Tags mutes
// @Security BearerAuth
// @Param id path string true "User ID to mute"
// @Param duration query int false "Mute duration in hours (default 24, 0 for permanent)"
// @Param reason query string false "Reason for muting"
// @Success 200 {object} dto.MuteResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes/{id} [post]
func (h *MuteHandler) MuteUser(w http.ResponseWriter, r *http.Request) {
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
		h.sendError(w, http.StatusBadRequest, "Cannot mute yourself", nil)
		return
	}

	// Parse optional parameters
	duration, _ := strconv.Atoi(r.URL.Query().Get("duration"))
	if duration < 0 {
		duration = 0
	}
	reason := r.URL.Query().Get("reason")
	if reason != "" {
		reason = strings.TrimSpace(reason)
	}

	result, err := h.muteService.MuteUser(r.Context(), userID, targetID, duration, reason)
	if err != nil {
		h.handleServiceError(w, err, "Failed to mute user")
		return
	}

	// Get updated counts
	mutedCount, _ := h.muteService.GetMutedCount(r.Context(), userID)

	// Determine if the mute is permanent or temporary
	isPermanent := duration == 0
	expiresAt := ""
	if !isPermanent {
		expiresAt = time.Now().Add(time.Duration(duration) * time.Hour).Format(time.RFC3339)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"muted":          result.Muted,
		"muted_user_id":  targetID,
		"muter_id":       userID,
		"muted_count":    mutedCount,
		"reason":         result.Reason,
		"is_permanent":   isPermanent,
		"expires_at":     expiresAt,
		"timestamp":      time.Now().Unix(),
	})
}

// UnmuteUser handles unmuting a user.
// @Summary Unmute a user
// @Description Unmutes a previously muted user
// @Tags mutes
// @Security BearerAuth
// @Param id path string true "User ID to unmute"
// @Success 200 {object} dto.MuteResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes/{id} [delete]
func (h *MuteHandler) UnmuteUser(w http.ResponseWriter, r *http.Request) {
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
		h.sendError(w, http.StatusBadRequest, "Cannot unmute yourself", nil)
		return
	}

	result, err := h.muteService.UnmuteUser(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to unmute user")
		return
	}

	// Get updated counts
	mutedCount, _ := h.muteService.GetMutedCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"muted":          result.Muted,
		"muted_user_id":  targetID,
		"muter_id":       userID,
		"muted_count":    mutedCount,
		"timestamp":      time.Now().Unix(),
	})
}

// ToggleMute handles toggling mute status on a user.
// @Summary Toggle mute
// @Description Toggles mute status on a user (mute if not muted, unmute if muted)
// @Tags mutes
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param duration query int false "Mute duration in hours (default 24, 0 for permanent)"
// @Param reason query string false "Reason for muting"
// @Success 200 {object} dto.MuteResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes/{id}/toggle [post]
func (h *MuteHandler) ToggleMute(w http.ResponseWriter, r *http.Request) {
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
		h.sendError(w, http.StatusBadRequest, "Cannot mute yourself", nil)
		return
	}

	// Check if already muted
	isMuted, err := h.muteService.IsMuted(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check mute status")
		return
	}

	var result *dto.MuteResponse
	if isMuted {
		result, err = h.muteService.UnmuteUser(r.Context(), userID, targetID)
	} else {
		duration, _ := strconv.Atoi(r.URL.Query().Get("duration"))
		if duration < 0 {
			duration = 0
		}
		reason := r.URL.Query().Get("reason")
		if reason != "" {
			reason = strings.TrimSpace(reason)
		}
		result, err = h.muteService.MuteUser(r.Context(), userID, targetID, duration, reason)
	}

	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle mute")
		return
	}

	// Get updated counts
	mutedCount, _ := h.muteService.GetMutedCount(r.Context(), userID)

	isPermanent := true
	expiresAt := ""
	if result.ExpiresAt != nil {
		isPermanent = false
		expiresAt = result.ExpiresAt.Format(time.RFC3339)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"muted":          result.Muted,
		"muted_user_id":  targetID,
		"muter_id":       userID,
		"muted_count":    mutedCount,
		"reason":         result.Reason,
		"is_permanent":   isPermanent,
		"expires_at":     expiresAt,
		"timestamp":      time.Now().Unix(),
	})
}

// ======================================================================
// Check Mute Status
// ======================================================================

// CheckMuteStatus handles checking if a user is muted.
// @Summary Check mute status
// @Description Checks if the authenticated user has muted another user
// @Tags mutes
// @Security BearerAuth
// @Param id path string true "User ID to check"
// @Success 200 {object} dto.MuteStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes/{id}/status [get]
func (h *MuteHandler) CheckMuteStatus(w http.ResponseWriter, r *http.Request) {
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

	muteInfo, err := h.muteService.GetMuteInfo(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check mute status")
		return
	}

	// Get muted count
	mutedCount, _ := h.muteService.GetMutedCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"muted":          muteInfo.Muted,
		"user_id":        targetID,
		"reason":         muteInfo.Reason,
		"is_permanent":   muteInfo.IsPermanent,
		"expires_at":     muteInfo.ExpiresAt,
		"muted_count":    mutedCount,
	})
}

// ======================================================================
= Get Muted Users
// ======================================================================

// GetMutedUsers handles retrieving the user's muted list.
// @Summary Get muted users
// @Description Retrieves the list of users muted by the authenticated user
// @Tags mutes
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param include_expired query bool false "Include expired mutes"
// @Param search query string false "Search by username or full name"
// @Param active_only query bool false "Only return active mutes"
// @Success 200 {object} dto.MutedListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes [get]
func (h *MuteHandler) GetMutedUsers(w http.ResponseWriter, r *http.Request) {
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
	includeExpired, _ := strconv.ParseBool(r.URL.Query().Get("include_expired"))
	search := r.URL.Query().Get("search")
	activeOnly, _ := strconv.ParseBool(r.URL.Query().Get("active_only"))

	mutes, nextCursor, total, err := h.muteService.GetMutedUsers(r.Context(), userID, cursor, limit, includeExpired, search, activeOnly)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get muted users")
		return
	}

	// Build response with user details
	mutedUsers := make([]*dto.MutedUserResponse, 0, len(mutes))
	for _, mute := range mutes {
		user, err := h.userService.GetUserByID(r.Context(), mute.MutedUserID)
		if err != nil {
			continue
		}
		isActive := mute.ExpiresAt == nil || mute.ExpiresAt.After(time.Now())
		mutedUsers = append(mutedUsers, &dto.MutedUserResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			Reason:      mute.Reason,
			MutedAt:     mute.CreatedAt,
			ExpiresAt:   mute.ExpiresAt,
			IsActive:    isActive,
			IsPermanent: mute.ExpiresAt == nil,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        mutedUsers,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Active Mutes
// ======================================================================

// GetActiveMutes handles retrieving currently active mutes.
// @Summary Get active mutes
// @Description Retrieves currently active mutes for the authenticated user
// @Tags mutes
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.MutedListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes/active [get]
func (h *MuteHandler) GetActiveMutes(w http.ResponseWriter, r *http.Request) {
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

	mutes, nextCursor, total, err := h.muteService.GetActiveMutes(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get active mutes")
		return
	}

	mutedUsers := make([]*dto.MutedUserResponse, 0, len(mutes))
	for _, mute := range mutes {
		user, err := h.userService.GetUserByID(r.Context(), mute.MutedUserID)
		if err != nil {
			continue
		}
		mutedUsers = append(mutedUsers, &dto.MutedUserResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			Reason:      mute.Reason,
			MutedAt:     mute.CreatedAt,
			ExpiresAt:   mute.ExpiresAt,
			IsActive:    true,
			IsPermanent: mute.ExpiresAt == nil,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        mutedUsers,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Expired Mutes
// ======================================================================

// GetExpiredMutes handles retrieving expired mutes.
// @Summary Get expired mutes
// @Description Retrieves expired mutes for the authenticated user
// @Tags mutes
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.MutedListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes/expired [get]
func (h *MuteHandler) GetExpiredMutes(w http.ResponseWriter, r *http.Request) {
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

	mutes, nextCursor, total, err := h.muteService.GetExpiredMutes(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get expired mutes")
		return
	}

	mutedUsers := make([]*dto.MutedUserResponse, 0, len(mutes))
	for _, mute := range mutes {
		user, err := h.userService.GetUserByID(r.Context(), mute.MutedUserID)
		if err != nil {
			continue
		}
		mutedUsers = append(mutedUsers, &dto.MutedUserResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			Reason:      mute.Reason,
			MutedAt:     mute.CreatedAt,
			ExpiresAt:   mute.ExpiresAt,
			IsActive:    false,
			IsPermanent: false,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        mutedUsers,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Mute Count
// ======================================================================

// GetMuteCount handles retrieving the mute count for a user.
// @Summary Get mute count
// @Description Retrieves the number of users muted by a user
// @Tags mutes
// @Produce json
// @Param id path string true "User ID"
// @Param active_only query bool false "Only count currently active mutes"
// @Success 200 {object} dto.MuteCountResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes/count/{id} [get]
func (h *MuteHandler) GetMuteCount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	// Verify user exists
	_, err := h.userService.GetUserByID(r.Context(), userID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "User not found", nil)
		return
	}

	activeOnly, _ := strconv.ParseBool(r.URL.Query().Get("active_only"))
	count, err := h.muteService.GetMutedCount(r.Context(), userID, activeOnly)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get mute count")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"mute_count": count,
		"user_id":    userID,
	})
}

// ======================================================================
= Check If Muted (Public)
// ======================================================================

// CheckIfMuted handles checking if user A has muted user B (public).
// @Summary Check if muted
// @Description Checks if user A has muted user B
// @Tags mutes
// @Produce json
// @Param muter_id query string true "Muter user ID"
// @Param muted_id query string true "Muted user ID"
// @Success 200 {object} dto.MuteStatusPublicResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/mutes/check [get]
func (h *MuteHandler) CheckIfMuted(w http.ResponseWriter, r *http.Request) {
	muterID := r.URL.Query().Get("muter_id")
	mutedID := r.URL.Query().Get("muted_id")

	if muterID == "" || mutedID == "" {
		h.sendError(w, http.StatusBadRequest, "muter_id and muted_id are required", nil)
		return
	}

	muteInfo, err := h.muteService.GetMuteInfo(r.Context(), muterID, mutedID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check mute status")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"muted":          muteInfo.Muted,
		"muter_id":       muterID,
		"muted_id":       mutedID,
		"reason":         muteInfo.Reason,
		"is_permanent":   muteInfo.IsPermanent,
		"expires_at":     muteInfo.ExpiresAt,
	})
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListMutes handles admin listing of all mute relationships.
// @Summary Admin list mutes
// @Description Lists all mute relationships for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param muter_id query string false "Filter by muter ID"
// @Param muted_id query string false "Filter by muted ID"
// @Param active_only query bool false "Only show active mutes"
// @Success 200 {object} dto.MuteAdminListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/mutes [get]
func (h *MuteHandler) AdminListMutes(w http.ResponseWriter, r *http.Request) {
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
	muterID := r.URL.Query().Get("muter_id")
	mutedID := r.URL.Query().Get("muted_id")
	activeOnly, _ := strconv.ParseBool(r.URL.Query().Get("active_only"))

	mutes, nextCursor, total, err := h.muteService.AdminListMutes(r.Context(), cursor, limit, muterID, mutedID, activeOnly)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list mutes")
		return
	}

	// Build response
	muteResponses := make([]*dto.MuteAdminResponse, 0, len(mutes))
	for _, m := range mutes {
		muter, _ := h.userService.GetUserByID(r.Context(), m.MuterID)
		muted, _ := h.userService.GetUserByID(r.Context(), m.MutedUserID)
		isActive := m.ExpiresAt == nil || m.ExpiresAt.After(time.Now())
		muteResponses = append(muteResponses, &dto.MuteAdminResponse{
			ID:             m.ID,
			MuterID:        m.MuterID,
			MutedUserID:    m.MutedUserID,
			MuterUsername: func() string {
				if muter != nil {
					return muter.Username
				}
				return ""
			}(),
			MutedUsername: func() string {
				if muted != nil {
					return muted.Username
				}
				return ""
			}(),
			Reason:      m.Reason,
			CreatedAt:   m.CreatedAt,
			ExpiresAt:   m.ExpiresAt,
			IsActive:    isActive,
			IsPermanent: m.ExpiresAt == nil,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        muteResponses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeleteMute handles admin deletion of a mute relationship.
// @Summary Admin delete mute
// @Description Deletes a mute relationship (admin only)
// @Tags admin
// @Security BearerAuth
// @Param mute_id path string true "Mute ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/mutes/{mute_id} [delete]
func (h *MuteHandler) AdminDeleteMute(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	muteID := vars["mute_id"]
	if muteID == "" {
		h.sendError(w, http.StatusBadRequest, "Mute ID required", nil)
		return
	}

	if err := h.muteService.AdminDeleteMute(r.Context(), muteID); err != nil {
		h.handleServiceError(w, err, "Failed to delete mute")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Mute relationship deleted successfully",
	})
}

// AdminGetMuteStats handles retrieving global mute statistics.
// @Summary Admin get mute stats
// @Description Retrieves global mute statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.MuteStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/mutes/stats [get]
func (h *MuteHandler) AdminGetMuteStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.muteService.AdminGetMuteStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get mute stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *MuteHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *MuteHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *MuteHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *MuteHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrAlreadyMuted):
		h.sendError(w, http.StatusConflict, "User already muted", nil)
	case errors.Is(err, service.ErrNotMuted):
		h.sendError(w, http.StatusBadRequest, "User is not muted", nil)
	case errors.Is(err, service.ErrCannotMuteSelf):
		h.sendError(w, http.StatusBadRequest, "Cannot mute yourself", nil)
	case errors.Is(err, service.ErrMuteNotFound):
		h.sendError(w, http.StatusNotFound, "Mute relationship not found", nil)
	case errors.Is(err, service.ErrInvalidMuteID):
		h.sendError(w, http.StatusBadRequest, "Invalid mute ID", nil)
	case errors.Is(err, service.ErrMuteExpired):
		h.sendError(w, http.StatusBadRequest, "Mute has expired", nil)
	case errors.Is(err, service.ErrMuteDurationInvalid):
		h.sendError(w, http.StatusBadRequest, "Invalid mute duration", nil)
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

// HealthCheck returns the health status of the mute handler.
func (h *MuteHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "mute_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}