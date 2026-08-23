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
	notificationService service.NotificationService
	log          *logrus.Entry
}

// NewBlockHandler creates a new block handler.
func NewBlockHandler(
	blockService service.BlockService,
	userService service.UserService,
	notificationService service.NotificationService,
) *BlockHandler {
	return &BlockHandler{
		blockService:       blockService,
		userService:        userService,
		notificationService: notificationService,
		log:                logger.WithField("handler", "block"),
	}
}

// ======================================================================
// Block/Unblock User
// ======================================================================

// BlockUser handles blocking a user.
// @Summary Block a user
// @Description Blocks a user from interacting with the authenticated user
// @Tags blocks
// @Security BearerAuth
// @Param id path string true "User ID to block"
// @Success 200 {object} dto.BlockResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
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

	result, err := h.blockService.BlockUser(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to block user")
		return
	}

	// Get updated counts
	blockedCount, _ := h.blockService.GetBlockedCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":       result.Blocked,
		"blocked_user_id": targetID,
		"blocker_id":    userID,
		"blocked_count": blockedCount,
		"timestamp":     time.Now().Unix(),
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
		"blocked":        result.Blocked,
		"blocked_user_id": targetID,
		"blocker_id":     userID,
		"blocked_count":  blockedCount,
		"timestamp":      time.Now().Unix(),
	})
}

// ToggleBlock handles toggling block status on a user.
// @Summary Toggle block
// @Description Toggles block status on a user
// @Tags blocks
// @Security BearerAuth
// @Param id path string true "User ID"
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
		result, err = h.blockService.BlockUser(r.Context(), userID, targetID)
	}

	if err != nil {
		h.handleServiceError(w, err, "Failed to toggle block")
		return
	}

	// Get updated counts
	blockedCount, _ := h.blockService.GetBlockedCount(r.Context(), userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":        result.Blocked,
		"blocked_user_id": targetID,
		"blocker_id":     userID,
		"blocked_count":  blockedCount,
		"timestamp":      time.Now().Unix(),
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

	isBlocked, err := h.blockService.IsBlocked(r.Context(), userID, targetID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check block status")
		return
	}

	// Check if target has blocked the user
	isBlockedByTarget, _ := h.blockService.IsBlocked(r.Context(), targetID, userID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":          isBlocked,
		"blocked_by_target": isBlockedByTarget,
		"user_id":          targetID,
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

	blocks, nextCursor, total, err := h.blockService.GetBlockedUsers(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get blocked users")
		return
	}

	// Build response
	blockedUsers := make([]*dto.BlockedUserResponse, 0, len(blocks))
	for _, block := range blocks {
		user, err := h.userService.GetUserByID(r.Context(), block.BlockedUserID)
		if err != nil {
			continue
		}
		blockedUsers = append(blockedUsers, &dto.BlockedUserResponse{
			ID:         user.ID,
			Username:   user.Username,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			Bio:        user.Bio,
			IsVerified: user.IsVerified,
			BlockedAt:  block.CreatedAt,
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
= Get Block Count
// ======================================================================

// GetBlockCount handles retrieving the block count for a user.
// @Summary Get block count
// @Description Retrieves the number of users blocked by a user
// @Tags blocks
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} dto.BlockCountResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/blocks/count/{id} [get]
func (h *BlockHandler) GetBlockCount(w http.ResponseWriter, r *http.Request) {
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

	count, err := h.blockService.GetBlockedCount(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get block count")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"block_count": count,
		"user_id":     userID,
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

	isBlocked, err := h.blockService.IsBlocked(r.Context(), blockerID, blockedID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check block status")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"blocked":      isBlocked,
		"blocker_id":   blockerID,
		"blocked_id":   blockedID,
	})
}

// ======================================================================
= Get Blocks By User (Admin)
// ======================================================================

// AdminGetUserBlocks handles retrieving blocks by a user (admin only).
// @Summary Admin get user blocks
// @Description Retrieves all blocks made by a user (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param user_id path string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.BlockedListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/blocks/user/{user_id} [get]
func (h *BlockHandler) AdminGetUserBlocks(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	userID := vars["user_id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	// Verify user exists
	_, err = h.userService.GetUserByID(r.Context(), userID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "User not found", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	blocks, nextCursor, total, err := h.blockService.GetBlockedUsers(r.Context(), userID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user blocks")
		return
	}

	blockedUsers := make([]*dto.BlockedUserResponse, 0, len(blocks))
	for _, block := range blocks {
		user, err := h.userService.GetUserByID(r.Context(), block.BlockedUserID)
		if err != nil {
			continue
		}
		blockedUsers = append(blockedUsers, &dto.BlockedUserResponse{
			ID:         user.ID,
			Username:   user.Username,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			Bio:        user.Bio,
			IsVerified: user.IsVerified,
			BlockedAt:  block.CreatedAt,
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

// AdminDeleteBlock handles admin deletion of a block relationship.
// @Summary Admin delete block
// @Description Deletes a block relationship (admin only)
// @Tags admin
// @Security BearerAuth
// @Param blocker_id path string true "Blocker user ID"
// @Param blocked_id path string true "Blocked user ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/blocks/{blocker_id}/{blocked_id} [delete]
func (h *BlockHandler) AdminDeleteBlock(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	blockerID := vars["blocker_id"]
	blockedID := vars["blocked_id"]

	if blockerID == "" || blockedID == "" {
		h.sendError(w, http.StatusBadRequest, "blocker_id and blocked_id are required", nil)
		return
	}

	if err := h.blockService.AdminDeleteBlock(r.Context(), blockerID, blockedID); err != nil {
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

	stats, err := h.blockService.AdminGetBlockStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get block stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
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
// @Success 200 {object} dto.BlockedListResponse
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
			ID:         user.ID,
			Username:   user.Username,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			Bio:        user.Bio,
			IsVerified: user.IsVerified,
			BlockedAt:  block.CreatedAt,
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