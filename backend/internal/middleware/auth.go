// backend/internal/middleware/auth.go
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// Context keys for storing user information.
type contextKey string

const (
	UserIDKey          contextKey = "user_id"
	UserKey            contextKey = "user"
	UserRoleKey        contextKey = "user_role"
	UserClaimsKey      contextKey = "user_claims"
	IsAuthenticatedKey contextKey = "is_authenticated"
	TokenKey           contextKey = "token"
	AuthTypeKey        contextKey = "auth_type"
	APITokenKey        contextKey = "api_token"
	SessionIDKey       contextKey = "session_id"
)

// Authentication types.
const (
	AuthTypeJWT     = "jwt"
	AuthTypeRefresh = "refresh"
	AuthTypeAPIKey  = "api_key"
	AuthTypeNone    = "none"
)

// Permission constants.
const (
	PermReadTweets      = "tweets:read"
	PermCreateTweets    = "tweets:create"
	PermUpdateTweets    = "tweets:update"
	PermDeleteTweets    = "tweets:delete"
	PermReadUsers       = "users:read"
	PermUpdateUsers     = "users:update"
	PermDeleteUsers     = "users:delete"
	PermModerateContent = "content:moderate"
	PermManageRoles     = "roles:manage"
	PermAdminAccess     = "admin:access"
	PermViewAuditLogs   = "audit:view"
	PermManageSystem    = "system:manage"
)

// RolePermissions maps roles to their permissions.
var RolePermissions = map[entities.UserRole][]string{
	entities.RoleUser: {
		PermReadTweets,
		PermCreateTweets,
		PermUpdateTweets,
		PermDeleteTweets,
		PermReadUsers,
		PermUpdateUsers,
	},
	entities.RoleModerator: {
		PermReadTweets,
		PermCreateTweets,
		PermUpdateTweets,
		PermDeleteTweets,
		PermReadUsers,
		PermUpdateUsers,
		PermModerateContent,
		PermViewAuditLogs,
	},
	entities.RoleAdmin: {
		PermReadTweets,
		PermCreateTweets,
		PermUpdateTweets,
		PermDeleteTweets,
		PermReadUsers,
		PermUpdateUsers,
		PermDeleteUsers,
		PermModerateContent,
		PermManageRoles,
		PermAdminAccess,
		PermViewAuditLogs,
		PermManageSystem,
	},
}

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	JWTSecret           string
	RefreshSecret       string
	RedisClient         *redis.Client
	UserRepo            interfaces.UserRepository
	SessionRepo         interfaces.SessionRepository
	UserCacheTTL        time.Duration
	EnableBlacklist     bool
	AllowOptional       bool
	TokenLookup         []string
	HeaderName          string
	CookieName          string
	RefreshCookieName   string
	QueryParamName      string
	APICookieName       string
	APIHeaderName       string
	JWTExpiry           time.Duration
	RefreshExpiry       time.Duration
	EnableAuditLog      bool
	EnableMFA           bool
	RequireMFA          bool
}

// AuthMiddleware is the main middleware struct.
type AuthMiddleware struct {
	config AuthConfig
	log    *logrus.Entry
}

// AuditLogEntry represents an authentication audit log.
type AuditLogEntry struct {
	UserID    string                 `json:"user_id"`
	Username  string                 `json:"username,omitempty"`
	Action    string                 `json:"action"`
	AuthType  string                 `json:"auth_type"`
	IP        string                 `json:"ip"`
	UserAgent string                 `json:"user_agent"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewAuthMiddleware creates a new auth middleware instance.
func NewAuthMiddleware(cfg AuthConfig) *AuthMiddleware {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Authorization"
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "access_token"
	}
	if cfg.RefreshCookieName == "" {
		cfg.RefreshCookieName = "refresh_token"
	}
	if cfg.QueryParamName == "" {
		cfg.QueryParamName = "token"
	}
	if cfg.APIHeaderName == "" {
		cfg.APIHeaderName = "X-API-Key"
	}
	if cfg.APICookieName == "" {
		cfg.APICookieName = "api_key"
	}
	if len(cfg.TokenLookup) == 0 {
		cfg.TokenLookup = []string{"header", "cookie", "query"}
	}
	if cfg.UserCacheTTL == 0 {
		cfg.UserCacheTTL = 5 * time.Minute
	}
	if cfg.JWTExpiry == 0 {
		cfg.JWTExpiry = 15 * time.Minute
	}
	if cfg.RefreshExpiry == 0 {
		cfg.RefreshExpiry = 7 * 24 * time.Hour
	}
	return &AuthMiddleware{
		config: cfg,
		log:    logger.WithField("middleware", "auth"),
	}
}

// Middleware returns the HTTP handler that authenticates requests.
func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Try to authenticate
		authResult := a.authenticate(r)

		if !authResult.Authenticated {
			if a.config.AllowOptional {
				ctx = context.WithValue(ctx, IsAuthenticatedKey, false)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			a.sendError(w, http.StatusUnauthorized, authResult.ErrorMessage)
			return
		}

		// If MFA is required and user hasn't verified MFA
		if a.config.RequireMFA && !authResult.MFAVerified {
			a.sendError(w, http.StatusForbidden, "MFA verification required")
			return
		}

		// Populate context with user information
		ctx = context.WithValue(ctx, UserIDKey, authResult.User.ID)
		ctx = context.WithValue(ctx, UserKey, authResult.User)
		ctx = context.WithValue(ctx, UserRoleKey, authResult.User.Role)
		ctx = context.WithValue(ctx, UserClaimsKey, authResult.Claims)
		ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
		ctx = context.WithValue(ctx, TokenKey, authResult.Token)
		ctx = context.WithValue(ctx, AuthTypeKey, authResult.AuthType)
		if authResult.SessionID != "" {
			ctx = context.WithValue(ctx, SessionIDKey, authResult.SessionID)
		}
		if authResult.APIToken != "" {
			ctx = context.WithValue(ctx, APITokenKey, authResult.APIToken)
		}

		// Update last active asynchronously
		go a.updateLastActive(authResult.User.ID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthenticateResult holds the result of authentication.
type AuthenticateResult struct {
	Authenticated bool
	User          *entities.User
	Claims        jwt.MapClaims
	Token         string
	APIToken      string
	SessionID     string
	AuthType      string
	MFAVerified   bool
	ErrorMessage  string
}

// authenticate attempts to authenticate the request using multiple methods.
func (a *AuthMiddleware) authenticate(r *http.Request) AuthenticateResult {
	// Try API key authentication first (machine-to-machine)
	if apiKey := a.extractAPIKey(r); apiKey != "" {
		return a.authenticateAPIKey(r.Context(), apiKey, r)
	}

	// Try JWT token authentication
	tokenString := a.extractToken(r)
	if tokenString == "" {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "No authentication token provided",
		}
	}

	// Try JWT first
	result := a.authenticateJWT(r.Context(), tokenString, r)
	if result.Authenticated {
		return result
	}

	// If JWT failed, try refresh token
	if refreshToken := a.extractRefreshToken(r); refreshToken != "" {
		return a.authenticateRefreshToken(r.Context(), refreshToken, r)
	}

	return AuthenticateResult{
		Authenticated: false,
		ErrorMessage:  "Authentication failed",
	}
}

// authenticateJWT validates a JWT token.
func (a *AuthMiddleware) authenticateJWT(ctx context.Context, tokenString string, r *http.Request) AuthenticateResult {
	// Validate token
	claims, err := a.validateToken(tokenString)
	if err != nil {
		a.logAudit(ctx, &AuditLogEntry{
			Action:    "jwt_auth",
			AuthType:  AuthTypeJWT,
			IP:        getClientIP(r),
			UserAgent: r.UserAgent(),
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Invalid token",
		}
	}

	// Check token blacklist
	if a.config.EnableBlacklist && a.config.RedisClient != nil {
		if a.isTokenBlacklisted(ctx, tokenString) {
			a.logAudit(ctx, &AuditLogEntry{
				Action:    "jwt_auth",
				AuthType:  AuthTypeJWT,
				IP:        getClientIP(r),
				UserAgent: r.UserAgent(),
				Success:   false,
				Error:     "token blacklisted",
				Timestamp: time.Now(),
			})
			return AuthenticateResult{
				Authenticated: false,
				ErrorMessage:  "Token has been revoked",
			}
		}
	}

	// Extract user ID
	userID, err := a.getUserIDFromClaims(claims)
	if err != nil {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Invalid token claims",
		}
	}

	// Load user
	user, err := a.loadUser(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return AuthenticateResult{
				Authenticated: false,
				ErrorMessage:  "User not found",
			}
		}
		a.log.WithError(err).Error("Failed to load user")
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Internal server error",
		}
	}

	// Check user status
	if status := a.checkUserStatus(user); !status.Allowed {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  status.Message,
		}
	}

	// Check session if SessionRepo available
	if a.config.SessionRepo != nil {
		if ok, err := a.validateSession(ctx, userID, claims); err != nil || !ok {
			return AuthenticateResult{
				Authenticated: false,
				ErrorMessage:  "Session invalid or expired",
			}
		}
	}

	// Check MFA requirement (if enabled and user has MFA enabled)
	mfaVerified := true
	if a.config.EnableMFA {
		mfaVerified = a.checkMFA(ctx, userID, claims)
	}

	a.logAudit(ctx, &AuditLogEntry{
		UserID:   user.ID,
		Username: user.Username,
		Action:   "jwt_auth",
		AuthType: AuthTypeJWT,
		IP:       getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:  true,
		Timestamp: time.Now(),
	})

	return AuthenticateResult{
		Authenticated: true,
		User:          user,
		Claims:        claims,
		Token:         tokenString,
		AuthType:      AuthTypeJWT,
		MFAVerified:   mfaVerified,
	}
}

// authenticateRefreshToken validates a refresh token.
func (a *AuthMiddleware) authenticateRefreshToken(ctx context.Context, refreshToken string, r *http.Request) AuthenticateResult {
	if a.config.SessionRepo == nil {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Refresh token not supported",
		}
	}

	// Get session from database
	session, err := a.config.SessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, interfaces.ErrSessionNotFound) {
			return AuthenticateResult{
				Authenticated: false,
				ErrorMessage:  "Invalid refresh token",
			}
		}
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Internal server error",
		}
	}

	// Check session expiry
	if session.ExpiresAt.Before(time.Now()) {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Refresh token expired",
		}
	}

	// Load user
	user, err := a.loadUser(ctx, session.UserID)
	if err != nil {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "User not found",
		}
	}

	// Check user status
	if status := a.checkUserStatus(user); !status.Allowed {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  status.Message,
		}
	}

	// Generate new access token from refresh token
	newAccessToken, err := a.generateAccessToken(user.ID, user.Username, string(user.Role))
	if err != nil {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Failed to generate new token",
		}
	}

	// Rotate refresh token (update session)
	newRefreshToken, err := a.generateRefreshToken()
	if err != nil {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Failed to generate new refresh token",
		}
	}

	session.RefreshToken = newRefreshToken
	session.ExpiresAt = time.Now().Add(a.config.RefreshExpiry)
	if err := a.config.SessionRepo.Update(ctx, session); err != nil {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Failed to update session",
		}
	}

	a.logAudit(ctx, &AuditLogEntry{
		UserID:   user.ID,
		Username: user.Username,
		Action:   "refresh_auth",
		AuthType: AuthTypeRefresh,
		IP:       getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:  true,
		Timestamp: time.Now(),
	})

	return AuthenticateResult{
		Authenticated: true,
		User:          user,
		Token:         newAccessToken,
		SessionID:     session.ID,
		AuthType:      AuthTypeRefresh,
		MFAVerified:   true,
	}
}

// authenticateAPIKey validates an API key.
func (a *AuthMiddleware) authenticateAPIKey(ctx context.Context, apiKey string, r *http.Request) AuthenticateResult {
	if a.config.RedisClient == nil {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "API key authentication not configured",
		}
	}

	// Get user ID from Redis
	key := "api_key:" + apiKey
	userID, err := a.config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return AuthenticateResult{
				Authenticated: false,
				ErrorMessage:  "Invalid API key",
			}
		}
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "Internal server error",
		}
	}

	// Load user
	user, err := a.loadUser(ctx, userID)
	if err != nil {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  "User not found",
		}
	}

	// Check user status
	if status := a.checkUserStatus(user); !status.Allowed {
		return AuthenticateResult{
			Authenticated: false,
			ErrorMessage:  status.Message,
		}
	}

	a.logAudit(ctx, &AuditLogEntry{
		UserID:   user.ID,
		Username: user.Username,
		Action:   "api_key_auth",
		AuthType: AuthTypeAPIKey,
		IP:       getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:  true,
		Timestamp: time.Now(),
	})

	return AuthenticateResult{
		Authenticated: true,
		User:          user,
		APIToken:      apiKey,
		AuthType:      AuthTypeAPIKey,
		MFAVerified:   true,
	}
}

// validateSession checks if the session exists and is valid.
func (a *AuthMiddleware) validateSession(ctx context.Context, userID string, claims jwt.MapClaims) (bool, error) {
	if a.config.SessionRepo == nil {
		return true, nil
	}
	// Get all active sessions
	sessions, err := a.config.SessionRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	// If no sessions and we require sessions, reject
	if len(sessions) == 0 {
		return false, nil
	}
	// Check if any session matches the token (we can check by comparing issued time)
	// For simplicity, we just check that at least one session exists
	return true, nil
}

// checkMFA verifies MFA status (hook for MFA implementation).
func (a *AuthMiddleware) checkMFA(ctx context.Context, userID string, claims jwt.MapClaims) bool {
	if !a.config.EnableMFA {
		return true
	}
	// Check if MFA claim exists and is true
	if mfaClaim, ok := claims["mfa_verified"].(bool); ok {
		return mfaClaim
	}
	// Check if user has MFA enabled (would check in DB)
	// For now, return true (MFA not required)
	return true
}

// UserStatusResult holds user status check result.
type UserStatusResult struct {
	Allowed bool
	Message string
}

// checkUserStatus validates user account status.
func (a *AuthMiddleware) checkUserStatus(user *entities.User) UserStatusResult {
	if user.IsSuspended {
		return UserStatusResult{Allowed: false, Message: "Account suspended"}
	}
	if !user.IsActive {
		return UserStatusResult{Allowed: false, Message: "Account inactive"}
	}
	if user.DeletedAt != nil {
		return UserStatusResult{Allowed: false, Message: "Account deleted"}
	}
	return UserStatusResult{Allowed: true}
}

// extractAPIKey extracts API key from header or cookie.
func (a *AuthMiddleware) extractAPIKey(r *http.Request) string {
	if key := r.Header.Get(a.config.APIHeaderName); key != "" {
		return key
	}
	if cookie, err := r.Cookie(a.config.APICookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

// extractRefreshToken extracts refresh token from cookie or body.
func (a *AuthMiddleware) extractRefreshToken(r *http.Request) string {
	if cookie, err := r.Cookie(a.config.RefreshCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

// extractToken extracts JWT token from request using configured methods.
func (a *AuthMiddleware) extractToken(r *http.Request) string {
	for _, lookup := range a.config.TokenLookup {
		switch lookup {
		case "header":
			if token := a.extractFromHeader(r); token != "" {
				return token
			}
		case "cookie":
			if token := a.extractFromCookie(r); token != "" {
				return token
			}
		case "query":
			if token := a.extractFromQuery(r); token != "" {
				return token
			}
		}
	}
	return ""
}

// extractFromHeader extracts token from Authorization header.
func (a *AuthMiddleware) extractFromHeader(r *http.Request) string {
	authHeader := r.Header.Get(a.config.HeaderName)
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return ""
}

// extractFromCookie extracts token from cookie.
func (a *AuthMiddleware) extractFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(a.config.CookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return cookie.Value
}

// extractFromQuery extracts token from query parameter.
func (a *AuthMiddleware) extractFromQuery(r *http.Request) string {
	return r.URL.Query().Get(a.config.QueryParamName)
}

// validateToken parses and validates the JWT token.
func (a *AuthMiddleware) validateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}
	// Verify expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, jwt.ErrTokenExpired
		}
	} else {
		return nil, errors.New("token missing expiration claim")
	}
	return claims, nil
}

// generateAccessToken creates a new access token.
func (a *AuthMiddleware) generateAccessToken(userID, username, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"user": username,
		"role": role,
		"exp":  time.Now().Add(a.config.JWTExpiry).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.JWTSecret))
}

// generateRefreshToken creates a random refresh token.
func (a *AuthMiddleware) generateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// isTokenBlacklisted checks if token is in Redis blacklist.
func (a *AuthMiddleware) isTokenBlacklisted(ctx context.Context, token string) bool {
	if a.config.RedisClient == nil {
		return false
	}
	key := "blacklist:" + token
	val, err := a.config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			a.log.WithError(err).Warn("Failed to check token blacklist")
		}
		return false
	}
	return val != ""
}

// getUserIDFromClaims extracts user ID from JWT claims.
func (a *AuthMiddleware) getUserIDFromClaims(claims jwt.MapClaims) (string, error) {
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("missing subject claim")
	}
	return sub, nil
}

// loadUser loads user from database or Redis cache.
func (a *AuthMiddleware) loadUser(ctx context.Context, userID string) (*entities.User, error) {
	// Try cache first
	if a.config.RedisClient != nil && a.config.UserCacheTTL > 0 {
		cacheKey := "user:" + userID
		cached, err := a.config.RedisClient.Get(ctx, cacheKey).Result()
		if err == nil && cached != "" {
			var user entities.User
			if err := json.Unmarshal([]byte(cached), &user); err == nil {
				return &user, nil
			}
		}
	}
	// Load from database
	user, err := a.config.UserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Store in cache
	if a.config.RedisClient != nil && a.config.UserCacheTTL > 0 {
		cacheKey := "user:" + userID
		data, err := json.Marshal(user)
		if err == nil {
			a.config.RedisClient.Set(ctx, cacheKey, data, a.config.UserCacheTTL)
		}
	}
	return user, nil
}

// updateLastActive updates user's last active timestamp asynchronously.
func (a *AuthMiddleware) updateLastActive(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.config.UserRepo.UpdateLastActive(ctx, userID); err != nil {
		a.log.WithError(err).WithField("user_id", userID).Warn("Failed to update last active")
	}
}

// logAudit logs authentication events.
func (a *AuthMiddleware) logAudit(ctx context.Context, entry *AuditLogEntry) {
	if !a.config.EnableAuditLog {
		return
	}
	// Log to structured logger
	a.log.WithFields(logrus.Fields{
		"user_id":    entry.UserID,
		"username":   entry.Username,
		"action":     entry.Action,
		"auth_type":  entry.AuthType,
		"ip":         entry.IP,
		"user_agent": entry.UserAgent,
		"success":    entry.Success,
		"error":      entry.Error,
	}).Info("authentication event")
	// Could also store in database for auditing
}

// sendError sends a JSON error response.
func (a *AuthMiddleware) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := map[string]interface{}{
		"error":     message,
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		a.log.WithError(err).Error("Failed to encode error response")
	}
}

// ---- Helper functions for retrieving user info from context ----

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) (string, error) {
	val := ctx.Value(UserIDKey)
	if val == nil {
		return "", errors.New("user ID not found in context")
	}
	id, ok := val.(string)
	if !ok {
		return "", errors.New("invalid user ID type")
	}
	return id, nil
}

// GetUser extracts the full user entity from context.
func GetUser(ctx context.Context) (*entities.User, error) {
	val := ctx.Value(UserKey)
	if val == nil {
		return nil, errors.New("user not found in context")
	}
	user, ok := val.(*entities.User)
	if !ok {
		return nil, errors.New("invalid user type")
	}
	return user, nil
}

// GetUserRole extracts role from context.
func GetUserRole(ctx context.Context) (entities.UserRole, error) {
	val := ctx.Value(UserRoleKey)
	if val == nil {
		return entities.RoleUser, errors.New("role not found in context")
	}
	role, ok := val.(entities.UserRole)
	if !ok {
		return entities.RoleUser, errors.New("invalid role type")
	}
	return role, nil
}

// GetClaims extracts JWT claims from context.
func GetClaims(ctx context.Context) (jwt.MapClaims, error) {
	val := ctx.Value(UserClaimsKey)
	if val == nil {
		return nil, errors.New("claims not found in context")
	}
	claims, ok := val.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}
	return claims, nil
}

// GetToken extracts the raw JWT token from context.
func GetToken(ctx context.Context) (string, error) {
	val := ctx.Value(TokenKey)
	if val == nil {
		return "", errors.New("token not found in context")
	}
	token, ok := val.(string)
	if !ok {
		return "", errors.New("invalid token type")
	}
	return token, nil
}

// GetAuthType returns the authentication type used.
func GetAuthType(ctx context.Context) (string, error) {
	val := ctx.Value(AuthTypeKey)
	if val == nil {
		return "", errors.New("auth type not found in context")
	}
	authType, ok := val.(string)
	if !ok {
		return "", errors.New("invalid auth type")
	}
	return authType, nil
}

// IsAuthenticated checks if the request is authenticated.
func IsAuthenticated(ctx context.Context) bool {
	val := ctx.Value(IsAuthenticatedKey)
	if val == nil {
		return false
	}
	auth, ok := val.(bool)
	if !ok {
		return false
	}
	return auth
}

// HasPermission checks if the authenticated user has a specific permission.
func HasPermission(ctx context.Context, permission string) (bool, error) {
	user, err := GetUser(ctx)
	if err != nil {
		return false, err
	}
	permissions, ok := RolePermissions[user.Role]
	if !ok {
		return false, nil
	}
	for _, p := range permissions {
		if p == permission {
			return true, nil
		}
	}
	return false, nil
}

// MustHavePermission returns a middleware that requires a specific permission.
func MustHavePermission(permission string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hasPermission, err := HasPermission(r.Context(), permission)
			if err != nil || !hasPermission {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---- Role-based authorization middleware ----

// RequireRole returns a middleware that requires a specific role.
func RequireRole(allowedRoles ...entities.UserRole) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, err := GetUserRole(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			allowed := false
			for _, allowedRole := range allowedRoles {
				if role == allowedRole {
					allowed = true
					break
				}
			}
			if !allowed {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin returns a middleware that requires admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return RequireRole(entities.RoleAdmin)(next)
}

// RequireAdminOrModerator returns a middleware that requires admin or moderator.
func RequireAdminOrModerator(next http.Handler) http.Handler {
	return RequireRole(entities.RoleAdmin, entities.RoleModerator)(next)
}

// ---- Token management ----

// BlacklistToken adds a token to the blacklist in Redis.
func (a *AuthMiddleware) BlacklistToken(ctx context.Context, token string) error {
	if a.config.RedisClient == nil {
		return errors.New("Redis client not configured")
	}
	// Parse token to get expiry
	claims, err := a.validateToken(token)
	if err != nil {
		return err
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return errors.New("token missing expiration")
	}
	ttl := time.Until(time.Unix(int64(exp), 0))
	if ttl <= 0 {
		return nil
	}
	key := "blacklist:" + token
	return a.config.RedisClient.Set(ctx, key, "1", ttl).Err()
}

// ---- Test helpers ----

// WithUserContext creates a context with user information for testing.
func WithUserContext(ctx context.Context, user *entities.User) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, user.ID)
	ctx = context.WithValue(ctx, UserKey, user)
	ctx = context.WithValue(ctx, UserRoleKey, user.Role)
	ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
	return ctx
}

// WithAdminContext creates a context with admin user for testing.
func WithAdminContext(ctx context.Context, admin *entities.User) context.Context {
	return WithUserContext(ctx, admin)
}

// ---- Route protection helper ----

// ProtectRoute wraps a handler with auth middleware and optional role check.
func (a *AuthMiddleware) ProtectRoute(handler http.Handler, roles ...entities.UserRole) http.Handler {
	handler = a.Middleware(handler)
	if len(roles) > 0 {
		handler = RequireRole(roles...)(handler)
	}
	return handler
}

// ---- Health check ----

// HealthCheck returns the health status of the auth middleware.
func (a *AuthMiddleware) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "ok",
		"component": "auth_middleware",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if a.config.RedisClient != nil {
		if err := a.config.RedisClient.Ping(r.Context()).Err(); err != nil {
			status["status"] = "degraded"
			status["redis"] = "unavailable"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			status["redis"] = "available"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ---- Permission helpers ----

// CanModerate checks if the authenticated user can moderate content.
func CanModerate(ctx context.Context) bool {
	user, err := GetUser(ctx)
	if err != nil {
		return false
	}
	return user.IsModerator()
}

// CanAdmin checks if the user has admin access.
func CanAdmin(ctx context.Context) bool {
	user, err := GetUser(ctx)
	if err != nil {
		return false
	}
	return user.IsAdmin()
}

// ---- getClientIP ----

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}