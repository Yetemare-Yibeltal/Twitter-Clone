// backend/internal/handler/block_handler.go
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

// BlockHandler handles all block-related HTTP endpoints.
type BlockHandler struct {
	blockService service.BlockService
	userService  service.UserService
	log          *logrus.Entry
}

// NewBlockHandler creates a new block handler.
func NewBlockHandler(
	blockService service.BlockService,
	userService service.UserService,
) *BlockHandler {
	return &BlockHandler{
		blockService: blockService,
		userService:  userService,
		log:          logger.WithField("handler", "block"),
	}
}

// ======================================================================
// Block/Unblock
// ======================================================================

// BlockUser handles blocking a user.
// @Summary Block a user
// @Description Blocks a user from interacting with the authenticated user
// @Tags blocks
// @Security BearerAuth
// @Param id path string true "User ID to block"
// @Param duration query int false "Block duration in hours (default 0 = permanent)"
// @Param reason query string false "Reason for blocking"
// @Success 200 {object} dto.BlockResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks/{id} [post]
func (h *BlockHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
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

	// Parse optional parameters
	duration, _ := strconv.Atoi(r.URL.Query().Get("duration"))
	if duration < 0 {
		duration = 0
	}
	reason := r.URL.Query().Get("reason")
	if reason != "" {
		reason = strings.TrimSpace(reason)
	}

	result, err := h.blockService.BlockUser(r.Context(), userID, targetID, duration, reason)
	if err != nil {
		h.handleServiceError(w, err, "Failed to block user")
		return
	}

	// Get updated counts
	blockedCount, _ := h.blockService.GetBlockedCount(r.Context(), userID)

	// Determine if the block is permanent or temporary
	isPermanent := duration == 0
	expiresAt := ""
	if !isPermanent {
		expiresAt = time.Now().Add(time.Duration(duration) * time.Hour).Format(time.RFC3339)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":         result.Blocked,
		"blocked_user_id": targetID,
		"blocker_id":      userID,
		"blocked_count":   blockedCount,
		"reason":          result.Reason,
		"is_permanent":    isPermanent,
		"expires_at":      expiresAt,
		"timestamp":       time.Now().Unix(),
	})
}

// UnblockUser handles unblocking a user.
// @Summary Unblock a user
// @Description Unblocks a previously blocked user
// @Tags blocks
// @Security BearerAuth
// @Param id path string true "User ID to unblock"
// @Success 200 {object} dto.BlockResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks/{id} [delete]
func (h *BlockHandler) UnblockUser(w http.ResponseWriter, r *http.Request) {
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
		h.sendError(w, http.StatusBadRequest, "Cannot unblock yourself", nil)
		return
	}

	result, err := h.blockService.UnblockUser(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to unblock user")
		return
	}

	// Get updated counts
	blockedCount, _ := h.blockService.GetBlockedCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":         result.Blocked,
		"blocked_user_id": targetID,
		"blocker_id":      userID,
		"blocked_count":   blockedCount,
		"timestamp":       time.Now().Unix(),
	})
}

// ToggleBlock handles toggling block status on a user.
// @Summary Toggle block
// @Description Toggles block status on a user (block if not blocked, unblock if blocked)
// @Tags blocks
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param duration query int false "Block duration in hours (default 0 = permanent)"
// @Param reason query string false "Reason for blocking"
// @Success 200 {object} dto.BlockResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks/{id}/toggle [post]
func (h *BlockHandler) ToggleBlock(w http.ResponseWriter, r *http.Request) {
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

	// Check if already blocked
	isBlocked, err := h.blockService.IsBlocked(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check block status")
		return
	}

	var result *dto.BlockResponse
	if isBlocked {
		result, err = h.blockService.UnblockUser(r.Context(), userID, targetID)
	} else {
		duration, _ := strconv.Atoi(r.URL.Query().Get("duration"))
		if duration < 0 {
			duration = 0
		}
		reason := r.URL.Query().Get("reason")
		if reason != "" {
			reason = strings.TrimSpace(reason)
		}
		result, err = h.blockService.BlockUser(r.Context(), userID, targetID, duration, reason)
	}

	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle block")
		return
	}

	// Get updated counts
	blockedCount, _ := h.blockService.GetBlockedCount(r.Context(), userID)

	isPermanent := true
	expiresAt := ""
	if result.ExpiresAt != nil {
		isPermanent = false
		expiresAt = result.ExpiresAt.Format(time.RFC3339)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":         result.Blocked,
		"blocked_user_id": targetID,
		"blocker_id":      userID,
		"blocked_count":   blockedCount,
		"reason":          result.Reason,
		"is_permanent":    isPermanent,
		"expires_at":      expiresAt,
		"timestamp":       time.Now().Unix(),
	})
}

// ======================================================================
= Check Block Status
// ======================================================================

// CheckBlockStatus handles checking if a user is blocked.
// @Summary Check block status
// @Description Checks if the authenticated user has blocked another user
// @Tags blocks
// @Security BearerAuth
// @Param id path string true "User ID to check"
// @Success 200 {object} dto.BlockStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks/{id}/status [get]
func (h *BlockHandler) CheckBlockStatus(w http.ResponseWriter, r *http.Request) {
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

	blockInfo, err := h.blockService.GetBlockInfo(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check block status")
		return
	}

	// Check if target has blocked the user
	blockedByTarget, _ := h.blockService.IsBlocked(r.Context(), targetID, userID)
	blockInfoByTarget, _ := h.blockService.GetBlockInfo(r.Context(), targetID, userID)

	// Get counts
	blockedCount, _ := h.blockService.GetBlockedCount(r.Context(), userID)
	blockedByCount, _ := h.blockService.GetBlockedCount(r.Context(), targetID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":                 blockInfo.Blocked,
		"blocked_by_target":       blockedByTarget,
		"user_id":                 targetID,
		"reason":                  blockInfo.Reason,
		"is_permanent":            blockInfo.IsPermanent,
		"expires_at":              blockInfo.ExpiresAt,
		"blocked_count":           blockedCount,
		"blocked_by_target_count": blockedByCount,
	})
}

// ======================================================================
= Get Blocked Users
// ======================================================================

// GetBlockedUsers handles retrieving the user's blocked list.
// @Summary Get blocked users
// @Description Retrieves the list of users blocked by the authenticated user
// @Tags blocks
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param include_expired query bool false "Include expired blocks"
// @Param search query string false "Search by username or full name"
// @Success 200 {object} dto.BlockedListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks [get]
func (h *BlockHandler) GetBlockedUsers(w http.ResponseWriter, r *http.Request) {
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

	blocks, nextCursor, total, err := h.blockService.GetBlockedUsers(r.Context(), userID, cursor, limit, includeExpired, search)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get blocked users")
		return
	}

	// Build response with user details
	blockedUsers := make([]*dto.BlockedUserResponse, 0, len(blocks))
	for _, block := range blocks {
		user, err := h.userService.GetUserByID(r.Context(), block.BlockedUserID)
		if err != nil {
			continue
		}
		isActive := block.ExpiresAt == nil || block.ExpiresAt.After(time.Now())
		blockedUsers = append(blockedUsers, &dto.BlockedUserResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			Reason:      block.Reason,
			BlockedAt:   block.CreatedAt,
			ExpiresAt:   block.ExpiresAt,
			IsActive:    isActive,
			IsPermanent: block.ExpiresAt == nil,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        blockedUsers,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Block Counts
// ======================================================================

// GetBlockCounts handles retrieving block counts for a user.
// @Summary Get block counts
// @Description Retrieves the number of users blocked and blocking a user
// @Tags blocks
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.BlockCountsResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks/counts/{id} [get]
func (h *BlockHandler) GetBlockCounts(w http.ResponseWriter, r *http.Request) {
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

	blockedCount, err := h.blockService.GetBlockedCount(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get blocked count")
		return
	}

	blockingCount, err := h.blockService.GetBlockingCount(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get blocking count")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked_count":  blockedCount,
		"blocking_count": blockingCount,
		"user_id":        userID,
	})
}

// ======================================================================
= Check If Blocked (Public)
// ======================================================================

// CheckIfBlocked handles checking if user A has blocked user B (public).
// @Summary Check if blocked
// @Description Checks if user A has blocked user B
// @Tags blocks
// @Produce json
// @Param blocker_id query string true "Blocker user ID"
// @Param blocked_id query string true "Blocked user ID"
// @Success 200 {object} dto.BlockStatusPublicResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks/check [get]
func (h *BlockHandler) CheckIfBlocked(w http.ResponseWriter, r *http.Request) {
	blockerID := r.URL.Query().Get("blocker_id")
	blockedID := r.URL.Query().Get("blocked_id")

	if blockerID == "" || blockedID == "" {
		h.sendError(w, http.StatusBadRequest, "blocker_id and blocked_id are required", nil)
		return
	}

	blockInfo, err := h.blockService.GetBlockInfo(r.Context(), blockerID, blockedID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check block status")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":      blockInfo.Blocked,
		"blocker_id":   blockerID,
		"blocked_id":   blockedID,
		"reason":       blockInfo.Reason,
		"is_permanent": blockInfo.IsPermanent,
		"expires_at":   blockInfo.ExpiresAt,
	})
}

// ======================================================================
= Get Blocked By Users (Who blocked me)
// ======================================================================

// GetBlockedByUsers handles retrieving users who have blocked the authenticated user.
// @Summary Get users who blocked me
// @Description Retrieves users who have blocked the authenticated user
// @Tags blocks
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.BlockedByListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks/blocked-by [get]
func (h *BlockHandler) GetBlockedByUsers(w http.ResponseWriter, r *http.Request) {
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

	blocks, nextCursor, total, err := h.blockService.GetBlockedByUsers(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get blocked by users")
		return
	}

	blockedByUsers := make([]*dto.BlockedByResponse, 0, len(blocks))
	for _, block := range blocks {
		user, err := h.userService.GetUserByID(r.Context(), block.BlockerID)
		if err != nil {
			continue
		}
		blockedByUsers = append(blockedByUsers, &dto.BlockedByResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			BlockedAt:   block.CreatedAt,
			ExpiresAt:   block.ExpiresAt,
			Reason:      block.Reason,
			IsActive:    block.ExpiresAt == nil || block.ExpiresAt.After(time.Now()),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        blockedByUsers,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListBlocks handles admin listing of all block relationships.
// @Summary Admin list blocks
// @Description Lists all block relationships for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param blocker_id query string false "Filter by blocker ID"
// @Param blocked_id query string false "Filter by blocked ID"
// @Param active_only query bool false "Only show active blocks"
// @Success 200 {object} dto.BlockAdminListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/blocks [get]
func (h *BlockHandler) AdminListBlocks(w http.ResponseWriter, r *http.Request) {
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
	blockerID := r.URL.Query().Get("blocker_id")
	blockedID := r.URL.Query().Get("blocked_id")
	activeOnly, _ := strconv.ParseBool(r.URL.Query().Get("active_only"))

	blocks, nextCursor, total, err := h.blockService.AdminListBlocks(r.Context(), cursor, limit, blockerID, blockedID, activeOnly)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list blocks")
		return
	}

	// Build response
	blockResponses := make([]*dto.BlockAdminResponse, 0, len(blocks))
	for _, b := range blocks {
		blocker, _ := h.userService.GetUserByID(r.Context(), b.BlockerID)
		blocked, _ := h.userService.GetUserByID(r.Context(), b.BlockedUserID)
		isActive := b.ExpiresAt == nil || b.ExpiresAt.After(time.Now())
		blockResponses = append(blockResponses, &dto.BlockAdminResponse{
			ID:              b.ID,
			BlockerID:       b.BlockerID,
			BlockedUserID:   b.BlockedUserID,
			BlockerUsername: func() string {
				if blocker != nil {
					return blocker.Username
				}
				return ""
			}(),
			BlockedUsername: func() string {
				if blocked != nil {
					return blocked.Username
				}
				return ""
			}(),
			Reason:      b.Reason,
			CreatedAt:   b.CreatedAt,
			ExpiresAt:   b.ExpiresAt,
			IsActive:    isActive,
			IsPermanent: b.ExpiresAt == nil,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        blockResponses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeleteBlock handles admin deletion of a block relationship.
// @Summary Admin delete block
// @Description Deletes a block relationship (admin only)
// @Tags admin
// @Security BearerAuth
// @Param block_id path string true "Block ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/blocks/{block_id} [delete]
func (h *BlockHandler) AdminDeleteBlock(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	blockID := vars["block_id"]
	if blockID == "" {
		h.sendError(w, http.StatusBadRequest, "Block ID required", nil)
		return
	}

	if err := h.blockService.AdminDeleteBlock(r.Context(), blockID); err != nil {
		h.handleServiceError(w, err, "Failed to delete block")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Block relationship deleted successfully",
	})
}

// AdminGetBlockStats handles retrieving global block statistics.
// @Summary Admin get block stats
// @Description Retrieves global block statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.BlockStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/blocks/stats [get]
func (h *BlockHandler) AdminGetBlockStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.blockService.AdminGetBlockStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get block stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *BlockHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *BlockHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *BlockHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *BlockHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrAlreadyBlocked):
		h.sendError(w, http.StatusConflict, "User already blocked", nil)
	case errors.Is(err, service.ErrNotBlocked):
		h.sendError(w, http.StatusBadRequest, "User is not blocked", nil)
	case errors.Is(err, service.ErrCannotBlockSelf):
		h.sendError(w, http.StatusBadRequest, "Cannot block yourself", nil)
	case errors.Is(err, service.ErrBlockNotFound):
		h.sendError(w, http.StatusNotFound, "Block relationship not found", nil)
	case errors.Is(err, service.ErrInvalidBlockID):
		h.sendError(w, http.StatusBadRequest, "Invalid block ID", nil)
	case errors.Is(err, service.ErrBlockAlreadyExpired):
		h.sendError(w, http.StatusBadRequest, "Block has already expired", nil)
	case errors.Is(err, service.ErrBlockDurationInvalid):
		h.sendError(w, http.StatusBadRequest, "Invalid block duration", nil)
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

// HealthCheck returns the health status of the block handler.
func (h *BlockHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "block_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}