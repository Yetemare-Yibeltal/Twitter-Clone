// backend/internal/handler/settings_handler.go
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

// SettingsHandler handles all settings-related HTTP endpoints.
type SettingsHandler struct {
	settingsService service.SettingsService
	userService     service.UserService
	log             *logrus.Entry
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(
	settingsService service.SettingsService,
	userService service.UserService,
) *SettingsHandler {
	return &SettingsHandler{
		settingsService: settingsService,
		userService:     userService,
		log:             logger.WithField("handler", "settings"),
	}
}

// ======================================================================
// Get Settings
// ======================================================================

// GetSettings handles retrieving all user settings.
// @Summary Get user settings
// @Description Retrieves all settings for the authenticated user
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SettingsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings [get]
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	settings, err := h.settingsService.GetSettings(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Update Profile Settings
// ======================================================================

// UpdateProfileSettings handles updating profile settings.
// @Summary Update profile settings
// @Description Updates the user's profile settings
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdateProfileSettingsRequest true "Profile settings"
// @Success 200 {object} dto.ProfileSettingsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/profile [put]
func (h *SettingsHandler) UpdateProfileSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdateProfileSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	settings, err := h.settingsService.UpdateProfileSettings(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update profile settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Update Notification Settings
// ======================================================================

// UpdateNotificationSettings handles updating notification preferences.
// @Summary Update notification settings
// @Description Updates the user's notification preferences
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdateNotificationSettingsRequest true "Notification settings"
// @Success 200 {object} dto.NotificationSettingsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/notifications [put]
func (h *SettingsHandler) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdateNotificationSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	settings, err := h.settingsService.UpdateNotificationSettings(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update notification settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Update Privacy Settings
// ======================================================================

// UpdatePrivacySettings handles updating privacy settings.
// @Summary Update privacy settings
// @Description Updates the user's privacy settings
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdatePrivacySettingsRequest true "Privacy settings"
// @Success 200 {object} dto.PrivacySettingsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/privacy [put]
func (h *SettingsHandler) UpdatePrivacySettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdatePrivacySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	settings, err := h.settingsService.UpdatePrivacySettings(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update privacy settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Update Appearance Settings
// ======================================================================

// UpdateAppearanceSettings handles updating appearance settings.
// @Summary Update appearance settings
// @Description Updates the user's appearance/theme settings
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdateAppearanceSettingsRequest true "Appearance settings"
// @Success 200 {object} dto.AppearanceSettingsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/appearance [put]
func (h *SettingsHandler) UpdateAppearanceSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdateAppearanceSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	settings, err := h.settingsService.UpdateAppearanceSettings(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update appearance settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Update Language Settings
// ======================================================================

// UpdateLanguageSettings handles updating language settings.
// @Summary Update language settings
// @Description Updates the user's language preferences
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UpdateLanguageSettingsRequest true "Language settings"
// @Success 200 {object} dto.LanguageSettingsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/language [put]
func (h *SettingsHandler) UpdateLanguageSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.UpdateLanguageSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	settings, err := h.settingsService.UpdateLanguageSettings(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update language settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Get Privacy Settings
// ======================================================================

// GetPrivacySettings handles retrieving privacy settings.
// @Summary Get privacy settings
// @Description Retrieves the user's privacy settings
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.PrivacySettingsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/privacy [get]
func (h *SettingsHandler) GetPrivacySettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	settings, err := h.settingsService.GetPrivacySettings(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get privacy settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Get Notification Settings
// ======================================================================

// GetNotificationSettings handles retrieving notification settings.
// @Summary Get notification settings
// @Description Retrieves the user's notification settings
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.NotificationSettingsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/notifications [get]
func (h *SettingsHandler) GetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	settings, err := h.settingsService.GetNotificationSettings(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get notification settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Get Appearance Settings
// ======================================================================

// GetAppearanceSettings handles retrieving appearance settings.
// @Summary Get appearance settings
// @Description Retrieves the user's appearance/theme settings
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.AppearanceSettingsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/appearance [get]
func (h *SettingsHandler) GetAppearanceSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	settings, err := h.settingsService.GetAppearanceSettings(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get appearance settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Get Language Settings
// ======================================================================

// GetLanguageSettings handles retrieving language settings.
// @Summary Get language settings
// @Description Retrieves the user's language preferences
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.LanguageSettingsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/language [get]
func (h *SettingsHandler) GetLanguageSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	settings, err := h.settingsService.GetLanguageSettings(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get language settings")
		return
	}

	h.sendSuccess(w, http.StatusOK, settings)
}

// ======================================================================
= Account Management
// ======================================================================

// ChangePassword handles changing the user's password.
// @Summary Change password
// @Description Changes the user's account password
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.ChangePasswordRequest true "Password change details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/change-password [post]
func (h *SettingsHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
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

	if err := h.settingsService.ChangePassword(r.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		h.handleServiceError(w, err, "Failed to change password")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password changed successfully",
	})
}

// DeactivateAccount handles deactivating the user's account.
// @Summary Deactivate account
// @Description Deactivates the user's account
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.DeactivateAccountRequest true "Deactivation details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/deactivate [post]
func (h *SettingsHandler) DeactivateAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.DeactivateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.settingsService.DeactivateAccount(r.Context(), userID, req.Reason); err != nil {
		h.handleServiceError(w, err, "Failed to deactivate account")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Account deactivated successfully",
	})
}

// ReactivateAccount handles reactivating the user's account.
// @Summary Reactivate account
// @Description Reactivates the user's deactivated account
// @Tags settings
// @Security BearerAuth
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/reactivate [post]
func (h *SettingsHandler) ReactivateAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.settingsService.ReactivateAccount(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to reactivate account")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Account reactivated successfully",
	})
}

// DeleteAccount handles permanently deleting the user's account.
// @Summary Delete account
// @Description Permanently deletes the user's account
// @Tags settings
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.DeleteAccountRequest true "Deletion details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/delete-account [delete]
func (h *SettingsHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.DeleteAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Verify password
	user, err := h.userService.GetUserByID(r.Context(), userID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "User not found", nil)
		return
	}
	if !user.CheckPassword(req.Password) {
		h.sendError(w, http.StatusUnauthorized, "Incorrect password", nil)
		return
	}

	if err := h.settingsService.DeleteAccount(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to delete account")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Account deleted successfully",
	})
}

// ======================================================================
= Export Data
// ======================================================================

// ExportData handles exporting the user's data.
// @Summary Export user data
// @Description Exports the user's data in JSON format
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.ExportDataResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/export [get]
func (h *SettingsHandler) ExportData(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	export, err := h.settingsService.ExportData(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to export data")
		return
	}

	h.sendSuccess(w, http.StatusOK, export)
}

// ======================================================================
= Session Management
// ======================================================================

// GetSessions handles retrieving all active sessions.
// @Summary Get active sessions
// @Description Retrieves all active sessions for the user
// @Tags settings
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.SessionsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/sessions [get]
func (h *SettingsHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	sessions, err := h.settingsService.GetSessions(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get sessions")
		return
	}

	h.sendSuccess(w, http.StatusOK, sessions)
}

// RevokeSession handles revoking a specific session.
// @Summary Revoke session
// @Description Revokes a specific session
// @Tags settings
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/sessions/{id} [delete]
func (h *SettingsHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
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

	if err := h.settingsService.RevokeSession(r.Context(), userID, sessionID); err != nil {
		h.handleServiceError(w, err, "Failed to revoke session")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Session revoked successfully",
	})
}

// RevokeAllSessions handles revoking all sessions except current.
// @Summary Revoke all sessions
// @Description Revokes all sessions except the current one
// @Tags settings
// @Security BearerAuth
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/settings/sessions/revoke-all [post]
func (h *SettingsHandler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.settingsService.RevokeAllSessions(r.Context(), userID); err != nil {
		h.handleServiceError(w, err, "Failed to revoke all sessions")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "All sessions revoked successfully",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *SettingsHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *SettingsHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *SettingsHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *SettingsHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrSettingsNotFound):
		h.sendError(w, http.StatusNotFound, "Settings not found", nil)
	case errors.Is(err, service.ErrInvalidOldPassword):
		h.sendError(w, http.StatusBadRequest, "Old password is incorrect", nil)
	case errors.Is(err, service.ErrNewPasswordSame):
		h.sendError(w, http.StatusBadRequest, "New password must be different from old password", nil)
	case errors.Is(err, service.ErrAccountAlreadyDeactivated):
		h.sendError(w, http.StatusBadRequest, "Account is already deactivated", nil)
	case errors.Is(err, service.ErrAccountNotDeactivated):
		h.sendError(w, http.StatusBadRequest, "Account is not deactivated", nil)
	case errors.Is(err, service.ErrAccountAlreadyDeleted):
		h.sendError(w, http.StatusBadRequest, "Account is already deleted", nil)
	case errors.Is(err, service.ErrSessionNotFound):
		h.sendError(w, http.StatusNotFound, "Session not found", nil)
	case errors.Is(err, service.ErrCannotRevokeCurrentSession):
		h.sendError(w, http.StatusBadRequest, "Cannot revoke the current session", nil)
	case errors.Is(err, service.ErrInvalidTheme):
		h.sendError(w, http.StatusBadRequest, "Invalid theme selection", nil)
	case errors.Is(err, service.ErrInvalidLanguage):
		h.sendError(w, http.StatusBadRequest, "Invalid language selection", nil)
	case errors.Is(err, service.ErrInvalidTimezone):
		h.sendError(w, http.StatusBadRequest, "Invalid timezone", nil)
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

// HealthCheck returns the health status of the settings handler.
func (h *SettingsHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "settings_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}