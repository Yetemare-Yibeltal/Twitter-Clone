// backend/internal/handler/auth_handler.go
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

// AuthHandler handles all authentication-related HTTP endpoints.
type AuthHandler struct {
	authService service.AuthService
	userService service.UserService
	log         *logrus.Entry
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(
	authService service.AuthService,
	userService service.UserService,
) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		userService: userService,
		log:         logger.WithField("handler", "auth"),
	}
}

// ======================================================================
// Registration
// ======================================================================

// Register handles user registration.
// @Summary Register a new user
// @Description Creates a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Registration details"
// @Success 201 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Set metadata
	req.UserAgent = r.UserAgent()
	req.IP = r.RemoteAddr

	resp, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		h.handleServiceError(w, err, "Registration failed")
		return
	}

	h.sendSuccess(w, http.StatusCreated, resp)
}

// ======================================================================
// Login
// ======================================================================

// Login handles user login.
// @Summary Login user
// @Description Authenticates a user and returns tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	req.UserAgent = r.UserAgent()
	req.IP = r.RemoteAddr

	resp, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		h.handleServiceError(w, err, "Login failed")
		return
	}

	h.sendSuccess(w, http.StatusOK, resp)
}

// ======================================================================
// Token Refresh
// ======================================================================

// RefreshToken handles refreshing the access token.
// @Summary Refresh access token
// @Description Refreshes the access token using a refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshRequest true "Refresh token"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/refresh [post]
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	resp, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		h.handleServiceError(w, err, "Refresh failed")
		return
	}

	h.sendSuccess(w, http.StatusOK, resp)
}

// ======================================================================
// Logout
// ======================================================================

// Logout handles user logout.
// @Summary Logout user
// @Description Logs out the user by invalidating the refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LogoutRequest true "Refresh token"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		h.handleServiceError(w, err, "Logout failed")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// ======================================================================
= Email Verification
// ======================================================================

// SendVerificationEmail handles sending the verification email.
// @Summary Send verification email
// @Description Sends a verification email to the user
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/verify-email/send [post]
func (h *AuthHandler) SendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.authService.SendVerificationEmail(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to send verification email")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Verification email sent",
	})
}

// VerifyEmail handles email verification with a token.
// @Summary Verify email
// @Description Verifies the user's email using a token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.VerifyEmailRequest true "Verification token"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.authService.VerifyEmail(r.Context(), req.Token); err != nil {
		h.handleServiceError(w, err, "Email verification failed")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Email verified successfully",
	})
}

// ======================================================================
= Password Reset
// ======================================================================

// ForgotPassword handles requesting a password reset.
// @Summary Request password reset
// @Description Sends a password reset email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Email address"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.authService.RequestPasswordReset(r.Context(), req.Email); err != nil {
		h.log.WithError(err).Debug("Password reset request failed")
	}

	// Always return success to prevent email enumeration
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "If an account exists with this email, a reset link has been sent",
	})
}

// ResetPassword handles resetting the password.
// @Summary Reset password
// @Description Resets the password using a valid token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Reset details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.authService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		h.handleServiceError(w, err, "Password reset failed")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password reset successfully",
	})
}

// ValidateResetToken validates a password reset token.
// @Summary Validate reset token
// @Description Validates if a password reset token is valid
// @Tags auth
// @Param token query string true "Reset token"
// @Produce json
// @Success 200 {object} dto.TokenValidationResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/validate-reset-token [get]
func (h *AuthHandler) ValidateResetToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.sendError(w, http.StatusBadRequest, "Token is required", nil)
		return
	}

	// Check if token exists and is valid
	isValid, err := h.authService.ValidateResetToken(r.Context(), token)
	if err != nil {
		h.handleServiceError(w, err, "Failed to validate token")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"valid":   isValid,
		"message": "Token is valid",
	})
}

// ======================================================================
= Session Management
// ======================================================================

// GetSessions handles retrieving the user's active sessions.
// @Summary Get active sessions
// @Description Retrieves all active sessions for the authenticated user
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SessionListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/sessions [get]
func (h *AuthHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	sessions, err := h.authService.GetActiveSessions(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get sessions")
		return
	}

	// Convert to response
	currentToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	sessionResponses := make([]*dto.SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		isCurrent := session.RefreshToken == currentToken
		sessionResponses = append(sessionResponses, &dto.SessionResponse{
			ID:        session.ID,
			UserID:    session.UserID,
			UserAgent: session.UserAgent,
			IP:        session.IP,
			CreatedAt: session.CreatedAt,
			ExpiresAt: session.ExpiresAt,
			IsCurrent: isCurrent,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"sessions": sessionResponses,
		"count":    len(sessionResponses),
	})
}

// RevokeSession handles revoking a specific session.
// @Summary Revoke session
// @Description Revokes a specific session
// @Tags auth
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/sessions/{id} [delete]
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	sessionID := vars["id"]
	if sessionID == "" {
		h.sendError(w, http.StatusBadRequest, "Session ID required", nil)
		return
	}

	if err := h.authService.RevokeSession(r.Context(), sessionID); err != nil {
		h.handleServiceError(w, err, "Failed to revoke session")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Session revoked successfully",
	})
}

// RevokeAllSessions handles revoking all sessions except the current one.
// @Summary Revoke all sessions
// @Description Revokes all sessions except the current one
// @Tags auth
// @Security BearerAuth
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/sessions/revoke-all [post]
func (h *AuthHandler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.authService.RevokeAllSessions(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to revoke all sessions")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "All sessions revoked successfully",
	})
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListUsers handles admin listing of all users.
// @Summary Admin list users
// @Description Lists all users for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, suspended, inactive)"
// @Param role query string false "Filter by role (user, moderator, admin)"
// @Param search query string false "Search by username or full name"
// @Success 200 {object} dto.UserListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users [get]
func (h *AuthHandler) AdminListUsers(w http.ResponseWriter, r *http.Request) {
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
	roleFilter := r.URL.Query().Get("role")
	search := r.URL.Query().Get("search")

	users, nextCursor, total, err := h.userService.ListUsers(r.Context(), cursor, limit, status, roleFilter, search)
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

// AdminGetUserDetails handles retrieving detailed user information.
// @Summary Admin get user details
// @Description Retrieves detailed information about a specific user
// @Tags admin
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} dto.UserDetailResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users/{id} [get]
func (h *AuthHandler) AdminGetUserDetails(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	details, err := h.userService.GetUserDetails(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user details")
		return
	}

	h.sendSuccess(w, http.StatusOK, details)
}

// AdminUpdateUserRole handles updating a user's role.
// @Summary Admin update user role
// @Description Updates a user's role
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
func (h *AuthHandler) AdminUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
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

	if err := h.userService.UpdateUserRole(r.Context(), userID, req.Role); err != nil {
		h.handleServiceError(w, err, "Failed to update user role")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User role updated successfully",
	})
}

// AdminSuspendUser handles suspending a user.
// @Summary Admin suspend user
// @Description Suspends a user account
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
func (h *AuthHandler) AdminSuspendUser(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
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

	if err := h.userService.SuspendUser(r.Context(), userID, req.Reason, req.Duration); err != nil {
		h.handleServiceError(w, err, "Failed to suspend user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User suspended successfully",
	})
}

// AdminUnsuspendUser handles unsuspending a user.
// @Summary Admin unsuspend user
// @Description Unsuspends a user account
// @Tags admin
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users/{id}/unsuspend [post]
func (h *AuthHandler) AdminUnsuspendUser(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if err := h.userService.UnsuspendUser(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to unsuspend user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User unsuspended successfully",
	})
}

// AdminDeleteUser handles deleting a user.
// @Summary Admin delete user
// @Description Permanently deletes a user account
// @Tags admin
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/users/{id} [delete]
func (h *AuthHandler) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	userID := vars["id"]
	if userID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if err := h.userService.DeleteUser(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to delete user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User deleted successfully",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *AuthHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *AuthHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *AuthHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *AuthHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrUserSuspended):
		h.sendError(w, http.StatusForbidden, "User is suspended", nil)
	case errors.Is(err, service.ErrUserInactive):
		h.sendError(w, http.StatusForbidden, "User is inactive", nil)
	case errors.Is(err, service.ErrUserNotVerified):
		h.sendError(w, http.StatusForbidden, "Email not verified", nil)
	case errors.Is(err, service.ErrInvalidCredentials):
		h.sendError(w, http.StatusUnauthorized, "Invalid credentials", nil)
	case errors.Is(err, service.ErrInvalidToken):
		h.sendError(w, http.StatusUnauthorized, "Invalid token", nil)
	case errors.Is(err, service.ErrTokenExpired):
		h.sendError(w, http.StatusUnauthorized, "Token expired", nil)
	case errors.Is(err, service.ErrAccountLocked):
		h.sendError(w, http.StatusTooManyRequests, "Account is temporarily locked", nil)
	case errors.Is(err, service.ErrEmailAlreadyVerified):
		h.sendError(w, http.StatusBadRequest, "Email already verified", nil)
	case errors.Is(err, service.ErrPasswordResetExpired):
		h.sendError(w, http.StatusBadRequest, "Password reset token expired", nil)
	case errors.Is(err, service.ErrPasswordResetInvalid):
		h.sendError(w, http.StatusBadRequest, "Invalid password reset token", nil)
	case errors.Is(err, service.ErrSessionNotFound):
		h.sendError(w, http.StatusNotFound, "Session not found", nil)
	case errors.Is(err, service.ErrDuplicateUsername):
		h.sendError(w, http.StatusConflict, "Username already taken", nil)
	case errors.Is(err, service.ErrDuplicateEmail):
		h.sendError(w, http.StatusConflict, "Email already registered", nil)
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

// HealthCheck returns the health status of the auth handler.
func (h *AuthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "auth_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}