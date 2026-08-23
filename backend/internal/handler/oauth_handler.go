// backend/internal/handler/oauth_handler.go
package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"twitter-clone/backend/internal/config"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/pkg/logger"
)

// OAuthHandler handles all OAuth2-related HTTP endpoints.
type OAuthHandler struct {
	authService service.AuthService
	userService service.UserService
	config      *config.Config
	log         *logrus.Entry

	// OAuth2 providers
	googleConfig  *oauth2.Config
	githubConfig  *oauth2.Config
	// Add other providers as needed
}

// NewOAuthHandler creates a new OAuth handler.
func NewOAuthHandler(
	authService service.AuthService,
	userService service.UserService,
	cfg *config.Config,
) *OAuthHandler {
	h := &OAuthHandler{
		authService: authService,
		userService: userService,
		config:      cfg,
		log:         logger.WithField("handler", "oauth"),
	}

	// Initialize OAuth2 configs
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		h.googleConfig = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		}
	}

	if cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "" {
		h.githubConfig = &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  cfg.GitHubRedirectURL,
			Scopes: []string{
				"user:email",
				"read:user",
			},
			Endpoint: github.Endpoint,
		}
	}

	return h
}

// ======================================================================
// OAuth2 Provider Handlers
// ======================================================================

// OAuthLogin handles initiating OAuth2 login flow.
// @Summary OAuth2 login
// @Description Initiates OAuth2 login with the specified provider
// @Tags oauth
// @Param provider path string true "Provider (google, github)"
// @Param redirect query string false "Redirect URL after login"
// @Success 302 "Redirects to provider"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /oauth/{provider}/login [get]
func (h *OAuthHandler) OAuthLogin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := vars["provider"]
	if provider == "" {
		h.sendError(w, http.StatusBadRequest, "Provider is required", nil)
		return
	}

	// Generate state token
	state, err := h.generateStateToken()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, "Failed to generate state", nil)
		return
	}

	// Store state in cookie for verification
	h.setStateCookie(w, state)

	var config *oauth2.Config
	switch provider {
	case "google":
		config = h.googleConfig
	case "github":
		config = h.githubConfig
	default:
		h.sendError(w, http.StatusBadRequest, "Unsupported provider: "+provider, nil)
		return
	}

	if config == nil {
		h.sendError(w, http.StatusBadRequest, "Provider not configured", nil)
		return
	}

	// Build redirect URL
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	}
	redirectURL := config.AuthCodeURL(state, opts...)

	// Add custom redirect parameter
	if redirect := r.URL.Query().Get("redirect"); redirect != "" {
		// Encode the redirect parameter to be passed through OAuth state
		stateWithRedirect := state + "|" + redirect
		// We need to store it in a cookie or session
		h.setRedirectCookie(w, redirect)
		// Rebuild redirect URL with new state
		redirectURL = config.AuthCodeURL(state, opts...)
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// OAuthCallback handles OAuth2 callback.
// @Summary OAuth2 callback
// @Description Handles OAuth2 callback from provider
// @Tags oauth
// @Param provider path string true "Provider (google, github)"
// @Param code query string true "Authorization code"
// @Param state query string true "State token"
// @Success 302 "Redirects to frontend"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /oauth/{provider}/callback [get]
func (h *OAuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := vars["provider"]
	if provider == "" {
		h.sendError(w, http.StatusBadRequest, "Provider is required", nil)
		return
	}

	// Verify state
	state := r.URL.Query().Get("state")
	if state == "" {
		h.sendError(w, http.StatusBadRequest, "Missing state parameter", nil)
		return
	}

	if !h.verifyStateCookie(r, state) {
		h.sendError(w, http.StatusUnauthorized, "Invalid state parameter", nil)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.sendError(w, http.StatusBadRequest, "Missing code parameter", nil)
		return
	}

	var config *oauth2.Config
	switch provider {
	case "google":
		config = h.googleConfig
	case "github":
		config = h.githubConfig
	default:
		h.sendError(w, http.StatusBadRequest, "Unsupported provider: "+provider, nil)
		return
	}

	if config == nil {
		h.sendError(w, http.StatusBadRequest, "Provider not configured", nil)
		return
	}

	// Exchange code for token
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	token, err := config.Exchange(ctx, code)
	if err != nil {
		h.log.WithError(err).Error("Failed to exchange OAuth2 code")
		h.sendError(w, http.StatusInternalServerError, "Failed to exchange authorization code", nil)
		return
	}

	// Get user info from provider
	userInfo, err := h.getUserInfoFromProvider(provider, token)
	if err != nil {
		h.log.WithError(err).Error("Failed to get user info from provider")
		h.sendError(w, http.StatusInternalServerError, "Failed to get user info", nil)
		return
	}

	// Authenticate or create user
	authResponse, err := h.authenticateOAuthUser(r.Context(), provider, userInfo)
	if err != nil {
		h.handleServiceError(w, err, "Failed to authenticate OAuth user")
		return
	}

	// Redirect to frontend with tokens
	redirectURL := h.getRedirectCookie(r)
	if redirectURL == "" {
		redirectURL = h.config.FrontendURL + "/oauth/callback"
	}

	// Append tokens to redirect URL
	redirectURL = fmt.Sprintf("%s?access_token=%s&refresh_token=%s&token_type=Bearer&expires_in=%d",
		redirectURL,
		authResponse.AccessToken,
		authResponse.RefreshToken,
		authResponse.ExpiresIn,
	)

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// ======================================================================
= OAuth2 User Info Fetching
// ======================================================================

// OAuthUserInfo represents user info from OAuth provider.
type OAuthUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	AvatarURL     string `json:"avatar_url"`
	Provider      string `json:"provider"`
	ProviderUserID string `json:"provider_user_id"`
}

// getUserInfoFromProvider fetches user info from the specified provider.
func (h *OAuthHandler) getUserInfoFromProvider(provider string, token *oauth2.Token) (*OAuthUserInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var userInfo OAuthUserInfo
	userInfo.Provider = provider

	switch provider {
	case "google":
		return h.getGoogleUserInfo(ctx, token)
	case "github":
		return h.getGitHubUserInfo(ctx, token)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// getGoogleUserInfo fetches user info from Google.
func (h *OAuthHandler) getGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := h.googleConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Google user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google API returned status: %s", resp.Status)
	}

	var userInfo struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode Google user info: %w", err)
	}

	return &OAuthUserInfo{
		ID:             userInfo.ID,
		Email:          userInfo.Email,
		Name:           userInfo.Name,
		AvatarURL:      userInfo.Picture,
		Provider:       "google",
		ProviderUserID: userInfo.ID,
	}, nil
}

// getGitHubUserInfo fetches user info from GitHub.
func (h *OAuthHandler) getGitHubUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := h.githubConfig.Client(ctx, token)

	// Get user profile
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var userInfo struct {
		ID       int64  `json:"id"`
		Login    string `json:"login"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub user info: %w", err)
	}

	// If email is not public, fetch from emails endpoint
	if userInfo.Email == "" {
		emailResp, err := client.Get("https://api.github.com/user/emails")
		if err == nil {
			defer emailResp.Body.Close()
			var emails []struct {
				Email   string `json:"email"`
				Primary bool   `json:"primary"`
				Verified bool  `json:"verified"`
			}
			if err := json.NewDecoder(emailResp.Body).Decode(&emails); err == nil {
				for _, e := range emails {
					if e.Primary && e.Verified {
						userInfo.Email = e.Email
						break
					}
				}
			}
		}
	}

	return &OAuthUserInfo{
		ID:             strconv.FormatInt(userInfo.ID, 10),
		Email:          userInfo.Email,
		Name:           userInfo.Name,
		AvatarURL:      userInfo.AvatarURL,
		Provider:       "github",
		ProviderUserID: strconv.FormatInt(userInfo.ID, 10),
	}, nil
}

// ======================================================================
= OAuth User Authentication
// ======================================================================

// authenticateOAuthUser authenticates or creates a user from OAuth info.
func (h *OAuthHandler) authenticateOAuthUser(ctx context.Context, provider string, info *OAuthUserInfo) (*dto.AuthResponse, error) {
	// Check if user already exists with this OAuth provider
	user, err := h.userService.GetUserByOAuthProvider(ctx, provider, info.ProviderUserID)
	if err == nil && user != nil {
		// User exists, generate tokens and return
		return h.authService.GenerateOAuthTokens(ctx, user)
	}

	// Check if user exists with this email
	user, err = h.userService.GetUserByEmail(ctx, info.Email)
	if err == nil && user != nil {
		// Link OAuth account to existing user
		if err := h.userService.LinkOAuthAccount(ctx, user.ID, provider, info.ProviderUserID, info.AvatarURL); err != nil {
			return nil, fmt.Errorf("failed to link OAuth account: %w", err)
		}
		return h.authService.GenerateOAuthTokens(ctx, user)
	}

	// Create new user
	username := h.generateUsername(info.Name, info.Email)
	user, err = h.userService.CreateOAuthUser(ctx, username, info.Email, info.Name, info.AvatarURL, provider, info.ProviderUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth user: %w", err)
	}

	return h.authService.GenerateOAuthTokens(ctx, user)
}

// generateUsername generates a unique username from name or email.
func (h *OAuthHandler) generateUsername(name, email string) string {
	base := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	if base == "" {
		base = strings.Split(email, "@")[0]
	}
	// Ensure uniqueness
	username := base
	counter := 1
	for {
		exists, err := h.userService.UsernameExists(context.Background(), username)
		if err != nil || !exists {
			break
		}
		username = fmt.Sprintf("%s%d", base, counter)
		counter++
	}
	return username
}

// ======================================================================
= State and Cookie Management
// ======================================================================

// generateStateToken generates a secure random state token.
func (h *OAuthHandler) generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// setStateCookie sets the state cookie.
func (h *OAuthHandler) setStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.config.Environment == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes
	})
}

// verifyStateCookie verifies the state cookie.
func (h *OAuthHandler) verifyStateCookie(r *http.Request, state string) bool {
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value == "" {
		return false
	}
	if cookie.Value != state {
		return false
	}
	// Clear state cookie after verification
	http.SetCookie(r.ResponseWriter, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.config.Environment == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	return true
}

// setRedirectCookie sets the redirect cookie.
func (h *OAuthHandler) setRedirectCookie(w http.ResponseWriter, redirect string) {
	if redirect != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_redirect",
			Value:    redirect,
			Path:     "/",
			HttpOnly: true,
			Secure:   h.config.Environment == "production",
			SameSite: http.SameSiteLaxMode,
			MaxAge:   600, // 10 minutes
		})
	}
}

// getRedirectCookie gets the redirect cookie.
func (h *OAuthHandler) getRedirectCookie(r *http.Request) string {
	cookie, err := r.Cookie("oauth_redirect")
	if err != nil {
		return ""
	}
	redirect := cookie.Value
	// Clear cookie
	http.SetCookie(r.ResponseWriter, &http.Cookie{
		Name:     "oauth_redirect",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.config.Environment == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	return redirect
}

// ======================================================================
= OAuth Account Management (Authenticated)
// ======================================================================

// LinkOAuthAccount handles linking an OAuth account to the authenticated user.
// @Summary Link OAuth account
// @Description Links an OAuth account to the authenticated user
// @Tags oauth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.LinkOAuthRequest true "OAuth provider and code"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/oauth/link [post]
func (h *OAuthHandler) LinkOAuthAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.LinkOAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Get OAuth user info from provider using the provided code
	var config *oauth2.Config
	switch req.Provider {
	case "google":
		config = h.googleConfig
	case "github":
		config = h.githubConfig
	default:
		h.sendError(w, http.StatusBadRequest, "Unsupported provider: "+req.Provider, nil)
		return
	}

	if config == nil {
		h.sendError(w, http.StatusBadRequest, "Provider not configured", nil)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	token, err := config.Exchange(ctx, req.Code)
	if err != nil {
		h.log.WithError(err).Error("Failed to exchange OAuth2 code")
		h.sendError(w, http.StatusInternalServerError, "Failed to exchange authorization code", nil)
		return
	}

	info, err := h.getUserInfoFromProvider(req.Provider, token)
	if err != nil {
		h.log.WithError(err).Error("Failed to get user info from provider")
		h.sendError(w, http.StatusInternalServerError, "Failed to get user info", nil)
		return
	}

	// Check if the OAuth account is already linked to another user
	existingUser, err := h.userService.GetUserByOAuthProvider(r.Context(), req.Provider, info.ProviderUserID)
	if err == nil && existingUser != nil && existingUser.ID != userID {
		h.sendError(w, http.StatusConflict, "This OAuth account is already linked to another user", nil)
		return
	}

	// Link OAuth account to the authenticated user
	if err := h.userService.LinkOAuthAccount(r.Context(), userID, req.Provider, info.ProviderUserID, info.AvatarURL); err != nil {
		h.handleServiceError(w, err, "Failed to link OAuth account")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "OAuth account linked successfully",
	})
}

// UnlinkOAuthAccount handles unlinking an OAuth account.
// @Summary Unlink OAuth account
// @Description Unlinks an OAuth account from the authenticated user
// @Tags oauth
// @Security BearerAuth
// @Param provider path string true "Provider (google, github)"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/oauth/unlink/{provider} [delete]
func (h *OAuthHandler) UnlinkOAuthAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	provider := vars["provider"]
	if provider == "" {
		h.sendError(w, http.StatusBadRequest, "Provider is required", nil)
		return
	}

	if err := h.userService.UnlinkOAuthAccount(r.Context(), userID, provider); err != nil {
		h.handleServiceError(w, err, "Failed to unlink OAuth account")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "OAuth account unlinked successfully",
	})
}

// GetOAuthAccounts handles retrieving the user's linked OAuth accounts.
// @Summary Get linked OAuth accounts
// @Description Retrieves the list of OAuth accounts linked to the authenticated user
// @Tags oauth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.OAuthAccountsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/oauth/accounts [get]
func (h *OAuthHandler) GetOAuthAccounts(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	accounts, err := h.userService.GetOAuthAccounts(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get OAuth accounts")
		return
	}

	h.sendSuccess(w, http.StatusOK, accounts)
}

// ======================================================================
= OAuth2 Token Introspection (Admin)
// ======================================================================

// AdminGetOAuthStats handles retrieving OAuth statistics.
// @Summary Admin get OAuth stats
// @Description Retrieves global OAuth usage statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.OAuthStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/oauth/stats [get]
func (h *OAuthHandler) AdminGetOAuthStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	stats, err := h.userService.GetOAuthStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get OAuth stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// AdminListOAuthUsers handles listing users with OAuth accounts.
// @Summary Admin list OAuth users
// @Description Lists users who have OAuth accounts linked (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param provider query string false "Filter by provider"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Success 200 {object} dto.OAuthUserListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/oauth/users [get]
func (h *OAuthHandler) AdminListOAuthUsers(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	provider := r.URL.Query().Get("provider")
	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	users, nextCursor, total, err := h.userService.GetOAuthUsers(r.Context(), provider, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list OAuth users")
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

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *OAuthHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *OAuthHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *OAuthHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *OAuthHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrOAuthAccountNotFound):
		h.sendError(w, http.StatusNotFound, "OAuth account not found", nil)
	case errors.Is(err, service.ErrOAuthAccountAlreadyLinked):
		h.sendError(w, http.StatusConflict, "OAuth account already linked", nil)
	case errors.Is(err, service.ErrOAuthAccountLinkedToOther):
		h.sendError(w, http.StatusConflict, "OAuth account is linked to another user", nil)
	case errors.Is(err, service.ErrCannotUnlinkLastAuthMethod):
		h.sendError(w, http.StatusBadRequest, "Cannot unlink the last authentication method", nil)
	case errors.Is(err, service.ErrInvalidOAuthProvider):
		h.sendError(w, http.StatusBadRequest, "Invalid OAuth provider", nil)
	case errors.Is(err, service.ErrOAuthTokenInvalid):
		h.sendError(w, http.StatusUnauthorized, "Invalid OAuth token", nil)
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

// HealthCheck returns the health status of the OAuth handler.
func (h *OAuthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "oauth_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}