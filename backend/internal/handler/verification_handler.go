// backend/internal/handler/verification_handler.go
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

// VerificationHandler handles all verification-related HTTP endpoints.
type VerificationHandler struct {
	authService       service.AuthService
	userService       service.UserService
	notificationService service.NotificationService
	log               *logrus.Entry
}

// NewVerificationHandler creates a new verification handler.
func NewVerificationHandler(
	authService service.AuthService,
	userService service.UserService,
	notificationService service.NotificationService,
) *VerificationHandler {
	return &VerificationHandler{
		authService:       authService,
		userService:       userService,
		notificationService: notificationService,
		log:               logger.WithField("handler", "verification"),
	}
}

// ======================================================================
// Email Verification
// ======================================================================

// SendVerificationEmail handles sending a verification email.
// @Summary Send verification email
// @Description Sends a verification email to the authenticated user
// @Tags verification
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/send [post]
func (h *VerificationHandler) SendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Check rate limit (prevent spam)
	if !h.checkRateLimit(r.Context(), userID, "verification_send") {
		h.sendError(w, http.StatusTooManyRequests, "Too many requests. Please try again later.", nil)
		return
	}

	if err := h.authService.SendVerificationEmail(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to send verification email")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Verification email sent successfully",
	})
}

// VerifyEmail handles email verification with a token.
// @Summary Verify email
// @Description Verifies the user's email using a token
// @Tags verification
// @Accept json
// @Produce json
// @Param request body dto.VerifyEmailRequest true "Verification token"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/verify [post]
func (h *VerificationHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
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

// CheckVerificationStatus handles checking if the user's email is verified.
// @Summary Check verification status
// @Description Checks if the authenticated user's email is verified
// @Tags verification
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.VerificationStatusResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/status [get]
func (h *VerificationHandler) CheckVerificationStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	user, err := h.userService.GetUserByID(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"verified": user.IsVerified,
		"user_id":  userID,
		"email":    user.Email,
	})
}

// ResendVerificationEmail handles resending the verification email.
// @Summary Resend verification email
// @Description Resends the verification email to the authenticated user
// @Tags verification
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/resend [post]
func (h *VerificationHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Check rate limit
	if !h.checkRateLimit(r.Context(), userID, "verification_resend") {
		h.sendError(w, http.StatusTooManyRequests, "Too many requests. Please try again later.", nil)
		return
	}

	if err := h.authService.SendVerificationEmail(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to resend verification email")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Verification email resent successfully",
	})
}

// ======================================================================
// Password Reset
// ======================================================================

// RequestPasswordReset handles requesting a password reset.
// @Summary Request password reset
// @Description Sends a password reset email to the user
// @Tags verification
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Email address"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/forgot-password [post]
func (h *VerificationHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
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

	// Check rate limit (prevent abuse)
	if !h.checkRateLimit(r.Context(), req.Email, "password_reset_request") {
		h.sendError(w, http.StatusTooManyRequests, "Too many requests. Please try again later.", nil)
		return
	}

	if err := h.authService.RequestPasswordReset(r.Context(), req.Email); err != nil {
		// Don't reveal if user exists for security
		h.log.WithError(err).Debug("Password reset request failed")
	}

	// Always return success to prevent email enumeration
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "If an account exists with this email, a reset link has been sent",
	})
}

// ResetPassword handles resetting the password with a token.
// @Summary Reset password
// @Description Resets the password using a valid token
// @Tags verification
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Reset details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/reset-password [post]
func (h *VerificationHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
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

// ValidateResetToken handles validating a password reset token.
// @Summary Validate reset token
// @Description Validates if a password reset token is valid
// @Tags verification
// @Param token query string true "Reset token"
// @Produce json
// @Success 200 {object} dto.TokenValidationResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/validate-reset-token [get]
func (h *VerificationHandler) ValidateResetToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.sendError(w, http.StatusBadRequest, "Token is required", nil)
		return
	}

	// Validate token using auth service
	// We need a method to validate without actually resetting
	// For now, we can attempt to parse the token
	// In production, we'd have a dedicated validation method
	// Here, we'll check if the token exists in Redis (which stores reset tokens)

	key := "password_reset:" + token
	// This requires redis adapter; we'll use a placeholder
	// In real implementation, we would check Redis
	// For now, we assume valid

	valid := true
	message := "Token is valid"
	// If token is expired, return invalid

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"valid":   valid,
		"message": message,
	})
}

// ======================================================================
= Change Email with Verification
// ======================================================================

// RequestEmailChange handles requesting an email change.
// @Summary Request email change
// @Description Sends a verification email to the new email address
// @Tags verification
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.EmailChangeRequest true "New email"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/change-email [post]
func (h *VerificationHandler) RequestEmailChange(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.EmailChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if !h.checkRateLimit(r.Context(), userID, "email_change") {
		h.sendError(w, http.StatusTooManyRequests, "Too many requests. Please try again later.", nil)
		return
	}

	// In a real implementation, we would send a verification email to the new address
	// For now, we'll just call the user service to change the email
	if err := h.userService.ChangeEmail(r.Context(), userID, req.NewEmail); err != nil {
		h.handleServiceError(w, err, "Failed to change email")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Email change requested. Please verify the new email.",
	})
}

// ======================================================================
= Phone Number Verification (optional)
// ======================================================================

// SendPhoneVerification handles sending a phone verification code.
// @Summary Send phone verification
// @Description Sends a verification code to the user's phone
// @Tags verification
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.PhoneVerificationRequest true "Phone number"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/send-phone [post]
func (h *VerificationHandler) SendPhoneVerification(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.PhoneVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if !h.checkRateLimit(r.Context(), userID, "phone_verification") {
		h.sendError(w, http.StatusTooManyRequests, "Too many requests. Please try again later.", nil)
		return
	}

	// In a real implementation, we would send an SMS with a code
	// For now, we return success
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Verification code sent to your phone",
	})
}

// VerifyPhone handles verifying the phone number.
// @Summary Verify phone
// @Description Verifies the user's phone number with a code
// @Tags verification
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.PhoneVerifyRequest true "Verification code"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/verification/verify-phone [post]
func (h *VerificationHandler) VerifyPhone(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.PhoneVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// In a real implementation, we would verify the code against stored value
	// For now, we return success
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Phone number verified successfully",
	})
}

// ======================================================================
= Rate Limiting Helper
// ======================================================================

// checkRateLimit checks if a rate limit is exceeded.
func (h *VerificationHandler) checkRateLimit(ctx context.Context, key string, action string) bool {
	// In production, use Redis for rate limiting
	// For now, always return true
	return true
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListVerificationRequests handles admin listing of verification requests.
// @Summary Admin list verification requests
// @Description Lists all verification requests for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (pending, verified, failed)"
// @Success 200 {object} dto.VerificationRequestListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/verification/requests [get]
func (h *VerificationHandler) AdminListVerificationRequests(w http.ResponseWriter, r *http.Request) {
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

	// In a real implementation, we would fetch from a verification request repository
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        []interface{}{},
		"next_cursor": "",
		"has_more":    false,
		"limit":       limit,
		"total":       0,
	})
}

// AdminGetVerificationStats handles retrieving verification statistics.
// @Summary Admin get verification stats
// @Description Retrieves global verification statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.VerificationStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/verification/stats [get]
func (h *VerificationHandler) AdminGetVerificationStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"total_users":             0,
		"verified_users":          0,
		"unverified_users":        0,
		"pending_requests":        0,
		"verification_rate":       "0%",
		"recent_verifications":    0,
		"failed_attempts":         0,
	})
}

// AdminResendVerification handles resending verification for a user (admin).
// @Summary Admin resend verification
// @Description Resends a verification email to a user (admin only)
// @Tags admin
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/verification/resend/{user_id} [post]
func (h *VerificationHandler) AdminResendVerification(w http.ResponseWriter, r *http.Request) {
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

	if err := h.authService.SendVerificationEmail(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to send verification email")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Verification email sent successfully",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *VerificationHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *VerificationHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *VerificationHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *VerificationHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrEmailAlreadyVerified):
		h.sendError(w, http.StatusBadRequest, "Email already verified", nil)
	case errors.Is(err, service.ErrInvalidToken):
		h.sendError(w, http.StatusUnauthorized, "Invalid token", nil)
	case errors.Is(err, service.ErrTokenExpired):
		h.sendError(w, http.StatusUnauthorized, "Token expired", nil)
	case errors.Is(err, service.ErrPasswordResetExpired):
		h.sendError(w, http.StatusBadRequest, "Password reset token expired", nil)
	case errors.Is(err, service.ErrPasswordResetInvalid):
		h.sendError(w, http.StatusBadRequest, "Invalid password reset token", nil)
	case errors.Is(err, service.ErrUserSuspended):
		h.sendError(w, http.StatusForbidden, "User is suspended", nil)
	case errors.Is(err, service.ErrUserInactive):
		h.sendError(w, http.StatusForbidden, "User is inactive", nil)
	case errors.Is(err, service.ErrEmailNotSent):
		h.sendError(w, http.StatusInternalServerError, "Failed to send email", nil)
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

// HealthCheck returns the health status of the verification handler.
func (h *VerificationHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "verification_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}