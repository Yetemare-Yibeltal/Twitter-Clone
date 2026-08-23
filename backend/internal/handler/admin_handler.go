// backend/internal/handler/admin_handler.go
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

// AdminHandler handles all admin-related HTTP endpoints.
type AdminHandler struct {
	adminService        service.AdminService
	userService         service.UserService
	tweetService        service.TweetService
	followService       service.FollowService
	reportService       service.ReportService
	notificationService service.NotificationService
	log                 *logrus.Entry
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(
	adminService service.AdminService,
	userService service.UserService,
	tweetService service.TweetService,
	followService service.FollowService,
	reportService service.ReportService,
	notificationService service.NotificationService,
) *AdminHandler {
	return &AdminHandler{
		adminService:        adminService,
		userService:         userService,
		tweetService:        tweetService,
		followService:       followService,
		reportService:       reportService,
		notificationService: notificationService,
		log:                 logger.WithField("handler", "admin"),
	}
}

// ======================================================================
// Admin Middleware Helper
// ======================================================================

// checkAdmin checks if the authenticated user is an admin.
func (h *AdminHandler) checkAdmin(ctx context.Context) (bool, error) {
	role, err := middleware.GetUserRole(ctx)
	if err != nil {
		return false, err
	}
	return role == "admin", nil
}

// requireAdmin checks admin status and sends error if not.
func (h *AdminHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	isAdmin, err := h.checkAdmin(r.Context())
	if err != nil || !isAdmin {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return false
	}
	return true
}

// ======================================================================
// User Management
// ======================================================================

// ListUsers handles listing all users (admin only).
// @Summary List all users
// @Description Retrieves a list of all users with pagination and filtering (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, suspended, inactive)"
// @Param role query string false "Filter by role (user, moderator, admin)"
// @Param search query string false "Search by username or full name"
// @Success 200 {object} dto.AdminUserListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users [get]
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	status := r.URL.Query().Get("status")
	role := r.URL.Query().Get("role")
	search := r.URL.Query().Get("search")

	users, nextCursor, total, err := h.adminService.ListUsers(r.Context(), cursor, limit, status, role, search)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list users")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        users,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// GetUserDetails handles retrieving detailed user information.
// @Summary Get user details
// @Description Retrieves detailed information about a specific user (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} dto.AdminUserDetailResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users/{id} [get]
func (h *AdminHandler) GetUserDetails(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	details, err := h.adminService.GetUserDetails(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user details")
		return
	}

	h.sendSuccess(w, http.StatusOK, details)
}

// UpdateUserRole handles updating a user's role.
// @Summary Update user role
// @Description Updates a user's role (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body dto.UpdateUserRoleRequest true "Role update"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users/{id}/role [put]
func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	var req dto.UpdateUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.adminService.UpdateUserRole(r.Context(), userID, req.Role); err != nil {
		h.handleServiceError(w, err, "Failed to update user role")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User role updated successfully",
	})
}

// SuspendUser handles suspending a user.
// @Summary Suspend user
// @Description Suspends a user account (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param request body dto.SuspendUserRequest true "Suspension details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users/{id}/suspend [post]
func (h *AdminHandler) SuspendUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	var req dto.SuspendUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.adminService.SuspendUser(r.Context(), userID, req.Reason, req.Duration); err != nil {
		h.handleServiceError(w, err, "Failed to suspend user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User suspended successfully",
	})
}

// UnsuspendUser handles unsuspending a user.
// @Summary Unsuspend user
// @Description Unsuspends a user account (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users/{id}/unsuspend [post]
func (h *AdminHandler) UnsuspendUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if err := h.adminService.UnsuspendUser(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to unsuspend user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User unsuspended successfully",
	})
}

// DeleteUser handles deleting a user (admin).
// @Summary Delete user
// @Description Permanently deletes a user account (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users/{id}/delete [delete]
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if err := h.adminService.DeleteUser(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to delete user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User deleted successfully",
	})
}

// ======================================================================
// Tweet Moderation
// ======================================================================

// ListTweets handles listing all tweets for moderation.
// @Summary List all tweets
// @Description Retrieves a list of all tweets with pagination and filtering (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, deleted, reported)"
// @Param user_id query string false "Filter by user ID"
// @Param search query string false "Search by content"
// @Success 200 {object} dto.AdminTweetListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/tweets [get]
func (h *AdminHandler) ListTweets(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	status := r.URL.Query().Get("status")
	userID := r.URL.Query().Get("user_id")
	search := r.URL.Query().Get("search")

	tweets, nextCursor, total, err := h.adminService.ListTweets(r.Context(), cursor, limit, status, userID, search)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list tweets")
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

// DeleteTweet handles deleting a tweet (admin).
// @Summary Delete tweet
// @Description Deletes a tweet (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/tweets/{id}/delete [delete]
func (h *AdminHandler) DeleteTweet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	if err := h.adminService.DeleteTweet(r.Context(), tweetID); err != nil {
		h.handleServiceError(w, err, "Failed to delete tweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tweet deleted successfully",
	})
}

// HideTweet handles hiding a tweet from feeds.
// @Summary Hide tweet
// @Description Hides a tweet from all feeds (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Param duration query int false "Hide duration in hours (default 24)"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/tweets/{id}/hide [post]
func (h *AdminHandler) HideTweet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	duration, _ := strconv.Atoi(r.URL.Query().Get("duration"))
	if duration < 1 {
		duration = 24
	}

	if err := h.adminService.HideTweet(r.Context(), tweetID, duration); err != nil {
		h.handleServiceError(w, err, "Failed to hide tweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tweet hidden successfully",
	})
}

// UnhideTweet handles unhiding a tweet.
// @Summary Unhide tweet
// @Description Unhides a previously hidden tweet (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/tweets/{id}/unhide [post]
func (h *AdminHandler) UnhideTweet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	if err := h.adminService.UnhideTweet(r.Context(), tweetID); err != nil {
		h.handleServiceError(w, err, "Failed to unhide tweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tweet unhidden successfully",
	})
}

// PinTweet handles pinning a tweet.
// @Summary Pin tweet
// @Description Pins a tweet to the top of feeds (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/tweets/{id}/pin [post]
func (h *AdminHandler) PinTweet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	if err := h.adminService.PinTweet(r.Context(), tweetID); err != nil {
		h.handleServiceError(w, err, "Failed to pin tweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tweet pinned successfully",
	})
}

// UnpinTweet handles unpinning a tweet.
// @Summary Unpin tweet
// @Description Unpins a previously pinned tweet (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Tweet ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/tweets/{id}/unpin [post]
func (h *AdminHandler) UnpinTweet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	tweetID := vars["id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	if err := h.adminService.UnpinTweet(r.Context(), tweetID); err != nil {
		h.handleServiceError(w, err, "Failed to unpin tweet")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tweet unpinned successfully",
	})
}

// ======================================================================
// Report Management
// ======================================================================

// ListReports handles listing all reports.
// @Summary List all reports
// @Description Retrieves a list of all reports with pagination and filtering (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (pending, under_review, resolved, dismissed)"
// @Param severity query string false "Filter by severity (low, medium, high, critical)"
// @Param target_type query string false "Filter by target type (tweet, user, comment)"
// @Success 200 {object} dto.AdminReportListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports [get]
func (h *AdminHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	status := r.URL.Query().Get("status")
	severity := r.URL.Query().Get("severity")
	targetType := r.URL.Query().Get("target_type")

	reports, nextCursor, total, err := h.adminService.ListReports(r.Context(), cursor, limit, status, severity, targetType)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list reports")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        reports,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// GetReportDetails handles retrieving detailed report information.
// @Summary Get report details
// @Description Retrieves detailed information about a specific report (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Success 200 {object} dto.AdminReportDetailResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports/{id} [get]
func (h *AdminHandler) GetReportDetails(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	details, err := h.adminService.GetReportDetails(r.Context(), reportID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get report details")
		return
	}

	h.sendSuccess(w, http.StatusOK, details)
}

// AssignReport handles assigning a report to a reviewer.
// @Summary Assign report
// @Description Assigns a report to a reviewer (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.AssignReportRequest true "Assignment details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports/{id}/assign [post]
func (h *AdminHandler) AssignReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.AssignReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.adminService.AssignReport(r.Context(), reportID, req.ReviewerID); err != nil {
		h.handleServiceError(w, err, "Failed to assign report")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Report assigned successfully",
	})
}

// UpdateReportStatus handles updating a report's status.
// @Summary Update report status
// @Description Updates the status of a report (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.UpdateReportStatusRequest true "Status update"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports/{id}/status [put]
func (h *AdminHandler) UpdateReportStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.UpdateReportStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.adminService.UpdateReportStatus(r.Context(), reportID, req.Status, req.Notes); err != nil {
		h.handleServiceError(w, err, "Failed to update report status")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Report status updated successfully",
	})
}

// UpdateReportSeverity handles updating a report's severity.
// @Summary Update report severity
// @Description Updates the severity of a report (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.UpdateReportSeverityRequest true "Severity update"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports/{id}/severity [put]
func (h *AdminHandler) UpdateReportSeverity(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.UpdateReportSeverityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.adminService.UpdateReportSeverity(r.Context(), reportID, req.Severity); err != nil {
		h.handleServiceError(w, err, "Failed to update report severity")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Report severity updated successfully",
	})
}

// ResolveReport handles resolving a report.
// @Summary Resolve report
// @Description Marks a report as resolved (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.ResolveReportRequest true "Resolution details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports/{id}/resolve [post]
func (h *AdminHandler) ResolveReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.ResolveReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.adminService.ResolveReport(r.Context(), reportID, req.Action, req.Notes); err != nil {
		h.handleServiceError(w, err, "Failed to resolve report")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Report resolved successfully",
	})
}

// DismissReport handles dismissing a report.
// @Summary Dismiss report
// @Description Marks a report as dismissed (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.DismissReportRequest true "Dismissal details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports/{id}/dismiss [post]
func (h *AdminHandler) DismissReport(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.DismissReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.adminService.DismissReport(r.Context(), reportID, req.Notes); err != nil {
		h.handleServiceError(w, err, "Failed to dismiss report")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Report dismissed successfully",
	})
}

// ======================================================================
// System Stats
// ======================================================================

// GetSystemStats handles retrieving system statistics.
// @Summary Get system stats
// @Description Retrieves overall system statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SystemStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/stats [get]
func (h *AdminHandler) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	stats, err := h.adminService.GetSystemStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get system stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// GetDailyStats handles retrieving daily statistics.
// @Summary Get daily stats
// @Description Retrieves daily statistics for the past N days (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days (default 7, max 30)"
// @Success 200 {object} dto.DailyStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/stats/daily [get]
func (h *AdminHandler) GetDailyStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	stats, err := h.adminService.GetDailyStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get daily stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// GetEngagementStats handles retrieving engagement statistics.
// @Summary Get engagement stats
// @Description Retrieves platform engagement statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days (default 7, max 30)"
// @Success 200 {object} dto.EngagementStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/stats/engagement [get]
func (h *AdminHandler) GetEngagementStats(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	stats, err := h.adminService.GetEngagementStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get engagement stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// GetReportAnalytics handles retrieving report analytics.
// @Summary Get report analytics
// @Description Retrieves report analytics for moderation insights (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days (default 7, max 30)"
// @Success 200 {object} dto.ReportAnalyticsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/stats/reports [get]
func (h *AdminHandler) GetReportAnalytics(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	analytics, err := h.adminService.GetReportAnalytics(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get report analytics")
		return
	}

	h.sendSuccess(w, http.StatusOK, analytics)
}

// ======================================================================
// Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *AdminHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *AdminHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *AdminHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *AdminHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrTweetNotFound):
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
	case errors.Is(err, service.ErrReportNotFound):
		h.sendError(w, http.StatusNotFound, "Report not found", nil)
	case errors.Is(err, service.ErrUserAlreadySuspended):
		h.sendError(w, http.StatusBadRequest, "User is already suspended", nil)
	case errors.Is(err, service.ErrUserNotSuspended):
		h.sendError(w, http.StatusBadRequest, "User is not suspended", nil)
	case errors.Is(err, service.ErrCannotSuspendAdmin):
		h.sendError(w, http.StatusBadRequest, "Cannot suspend an admin user", nil)
	case errors.Is(err, service.ErrReportAlreadyResolved):
		h.sendError(w, http.StatusBadRequest, "Report is already resolved", nil)
	case errors.Is(err, service.ErrReportAlreadyDismissed):
		h.sendError(w, http.StatusBadRequest, "Report is already dismissed", nil)
	case errors.Is(err, service.ErrInvalidRole):
		h.sendError(w, http.StatusBadRequest, "Invalid role", nil)
	case errors.Is(err, service.ErrInvalidReportStatus):
		h.sendError(w, http.StatusBadRequest, "Invalid report status", nil)
	case errors.Is(err, service.ErrInvalidReportSeverity):
		h.sendError(w, http.StatusBadRequest, "Invalid report severity", nil)
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

// HealthCheck returns the health status of the admin handler.
func (h *AdminHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "admin_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}