// backend/internal/handler/auth_handler.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/pkg/logger"
)

// AuthHandler handles all authentication-related HTTP endpoints.
type AuthHandler struct {
	authService service.AuthService
	log         *logrus.Entry
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         logger.WithField("handler", "auth"),
	}
}

// Register handles user registration.
// @Summary Register a new user
// @Description Creates a new user account and returns tokens
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
	// Inject context metadata
	req.UserAgent = r.UserAgent()
	req.IP = r.RemoteAddr
	// Call service
	resp, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		h.handleServiceError(w, err, "Registration failed")
		return
	}
	h.sendSuccess(w, http.StatusCreated, resp)
}

// Login handles user login.
// @Summary Login user
// @Description Authenticates user and returns tokens
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
	// Call service
	resp, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		h.handleServiceError(w, err, "Login failed")
		return
	}
	h.sendSuccess(w, http.StatusOK, resp)
}

// Refresh handles token refresh.
// @Summary Refresh access token
// @Description Returns a new access token using a valid refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshRequest true "Refresh token"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
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

// Logout handles user logout.
// @Summary Logout user
// @Description Invalidates the refresh token
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
	h.sendSuccess(w, http.StatusOK, dto.NewSuccessResponse("Logged out successfully", nil))
}

// VerifyEmail handles email verification.
// @Summary Verify email address
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
		h.handleServiceError(w, err, "Verification failed")
		return
	}
	h.sendSuccess(w, http.StatusOK, dto.NewSuccessResponse("Email verified successfully", nil))
}

// SendVerificationEmail handles resending verification email.
// @Summary Resend verification email
// @Description Sends a new verification email to the authenticated user
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/verify-email/resend [post]
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
	h.sendSuccess(w, http.StatusOK, dto.NewSuccessResponse("Verification email sent", nil))
}

// ForgotPassword handles password reset request.
// @Summary Request password reset
// @Description Sends a password reset email to the user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Email address"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
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
		// For security, don't reveal if user exists; just return success.
		h.log.WithError(err).Debug("Password reset request failed (user may not exist)")
	}
	// Always return success to avoid email enumeration
	h.sendSuccess(w, http.StatusOK, dto.NewSuccessResponse("If an account with this email exists, a reset link has been sent", nil))
}

// ResetPassword handles password reset with token.
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
	h.sendSuccess(w, http.StatusOK, dto.NewSuccessResponse("Password reset successfully", nil))
}

// ChangePassword handles password change for authenticated user.
// @Summary Change password
// @Description Changes the password for the authenticated user
// @Tags auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.ChangePasswordRequest true "Password change details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/change-password [post]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}
	var req dto.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}
	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}
	if err := h.authService.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		h.handleServiceError(w, err, "Password change failed")
		return
	}
	h.sendSuccess(w, http.StatusOK, dto.NewSuccessResponse("Password changed successfully", nil))
}

// GetSessions returns active sessions for the authenticated user.
// @Summary Get active sessions
// @Description Lists all active sessions for the user
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {array} dto.SessionResponse
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
	// Convert to response DTOs
	currentToken := r.Header.Get("Authorization")
	currentToken = strings.TrimPrefix(currentToken, "Bearer ")
	response := make([]*dto.SessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		isCurrent := sess.RefreshToken == currentToken // simplified
		response = append(response, dto.ToSessionResponse(sess, isCurrent))
	}
	h.sendSuccess(w, http.StatusOK, response)
}

// RevokeSession revokes a specific session.
// @Summary Revoke session
// @Description Revokes a specific session by ID
// @Tags auth
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/auth/sessions/{id} [delete]
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]
	if sessionID == "" {
		h.sendError(w, http.StatusBadRequest, "Session ID is required", nil)
		return
	}
	if err := h.authService.RevokeSession(r.Context(), sessionID); err != nil {
		h.handleServiceError(w, err, "Failed to revoke session")
		return
	}
	h.sendSuccess(w, http.StatusOK, dto.NewSuccessResponse("Session revoked", nil))
}

// RevokeAllSessions revokes all sessions for the authenticated user.
// @Summary Revoke all sessions
// @Description Revokes all active sessions for the user
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
	h.sendSuccess(w, http.StatusOK, dto.NewSuccessResponse("All sessions revoked", nil))
}

// ---- Helper methods ----

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
	// Try to parse as ValidationErrors
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	// Single validation error
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *AuthHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	// Map known errors
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		h.sendError(w, http.StatusUnauthorized, "Invalid credentials", nil)
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrUserSuspended):
		h.sendError(w, http.StatusForbidden, "Account suspended", nil)
	case errors.Is(err, service.ErrUserInactive):
		h.sendError(w, http.StatusForbidden, "Account inactive", nil)
	case errors.Is(err, service.ErrUserNotVerified):
		h.sendError(w, http.StatusForbidden, "Email not verified", nil)
	case errors.Is(err, service.ErrInvalidToken):
		h.sendError(w, http.StatusUnauthorized, "Invalid token", nil)
	case errors.Is(err, service.ErrTokenExpired):
		h.sendError(w, http.StatusUnauthorized, "Token expired", nil)
	case errors.Is(err, service.ErrAccountLocked):
		h.sendError(w, http.StatusTooManyRequests, "Account temporarily locked due to too many failed attempts", nil)
	case errors.Is(err, service.ErrEmailAlreadyVerified):
		h.sendError(w, http.StatusBadRequest, "Email already verified", nil)
	case errors.Is(err, service.ErrPasswordResetExpired):
		h.sendError(w, http.StatusBadRequest, "Password reset token expired", nil)
	case errors.Is(err, service.ErrPasswordResetInvalid):
		h.sendError(w, http.StatusBadRequest, "Invalid password reset token", nil)
	case errors.Is(err, service.ErrSessionNotFound):
		h.sendError(w, http.StatusNotFound, "Session not found", nil)
	case errors.Is(err, context.Canceled):
		h.sendError(w, http.StatusRequestTimeout, "Request cancelled", nil)
	case errors.Is(err, context.DeadlineExceeded):
		h.sendError(w, http.StatusGatewayTimeout, "Request timed out", nil)
	default:
		// Log unexpected errors
		h.log.WithError(err).Error(defaultMsg)
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}

// ---- Health check (optional) ----
// HealthCheck returns the health status of the auth service.
func (h *AuthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Simple ping
	h.sendSuccess(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}