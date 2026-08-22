// backend/internal/middleware/auth.go
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Context Keys
// ======================================================================

type contextKey string

const (
	UserIDKey          contextKey = "user_id"
	UserKey            contextKey = "user"
	UserRoleKey        contextKey = "user_role"
	UserClaimsKey      contextKey = "user_claims"
	IsAuthenticatedKey contextKey = "is_authenticated"
	UserPermissionsKey contextKey = "user_permissions"
	TokenKey           contextKey = "token"
)

// ======================================================================
// Configuration
// ======================================================================

// AuthConfig holds all configuration for the auth middleware.
type AuthConfig struct {
	// JWT configuration
	JWTSecret        string        `json:"jwt_secret"`
	JWTIssuer        string        `json:"jwt_issuer"`
	JWTAudience      string        `json:"jwt_audience"`
	JWTExpiry        time.Duration `json:"jwt_expiry"`
	
	// Redis for caching and blacklist
	RedisAdapter     adapter.RedisAdapter `json:"-"`
	UserCacheTTL     time.Duration `json:"user_cache_ttl"`
	
	// User repository for loading user data
	UserRepo         interfaces.UserRepository `json:"-"`
	
	// Blacklist settings
	EnableBlacklist  bool          `json:"enable_blacklist"`
	BlacklistPrefix  string        `json:"blacklist_prefix"`
	
	// Cache settings
	EnableUserCache  bool          `json:"enable_user_cache"`
	UserCachePrefix  string        `json:"user_cache_prefix"`
	
	// Authentication mode
	AllowOptional    bool          `json:"allow_optional"`
	
	// WebSocket support
	AllowWebSocket   bool          `json:"allow_websocket"`
	
	// Logging
	LogLevel         string        `json:"log_level"`
	
	// Security
	RequireUserActive bool         `json:"require_user_active"`
	RequireUserVerified bool       `json:"require_user_verified"`
}

// DefaultAuthConfig returns sensible defaults.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		JWTSecret:         "change-me-in-production",
		JWTIssuer:         "twitter-clone",
		JWTAudience:       "twitter-clone-users",
		JWTExpiry:         15 * time.Minute,
		UserCacheTTL:      5 * time.Minute,
		EnableBlacklist:   true,
		BlacklistPrefix:   "blacklist:",
		EnableUserCache:   true,
		UserCachePrefix:   "user:",
		AllowOptional:     false,
		AllowWebSocket:    true,
		RequireUserActive: true,
		RequireUserVerified: false,
	}
}

// ======================================================================
// Auth Middleware
// ======================================================================

// AuthMiddleware is the main middleware struct.
type AuthMiddleware struct {
	config AuthConfig
	log    *logrus.Entry
	mu     sync.RWMutex
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(cfg AuthConfig) *AuthMiddleware {
	if cfg.JWTSecret == "change-me-in-production" {
		logger.Warn("JWT_SECRET is set to default value – please change for production")
	}
	return &AuthMiddleware{
		config: cfg,
		log:    logger.WithField("middleware", "auth"),
	}
}

// Middleware returns the HTTP middleware handler.
func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Extract token
		tokenString := a.extractToken(r)
		if tokenString == "" {
			if a.config.AllowOptional {
				ctx = a.setUnauthenticatedContext(ctx)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			a.sendError(w, http.StatusUnauthorized, "Missing authentication token", "token_missing")
			return
		}
		
		// Validate and parse token
		claims, err := a.validateToken(tokenString)
		if err != nil {
			a.log.WithError(err).Debug("Token validation failed")
			if a.config.AllowOptional {
				ctx = a.setUnauthenticatedContext(ctx)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			a.sendError(w, http.StatusUnauthorized, "Invalid or expired token", err.Error())
			return
		}
		
		// Check blacklist
		if a.config.EnableBlacklist && a.config.RedisAdapter != nil {
			if a.isTokenBlacklisted(ctx, tokenString) {
				a.sendError(w, http.StatusUnauthorized, "Token has been revoked", "token_revoked")
				return
			}
		}
		
		// Extract user ID from claims
		userID, err := a.extractUserID(claims)
		if err != nil {
			a.sendError(w, http.StatusUnauthorized, "Invalid token claims", err.Error())
			return
		}
		
		// Load user
		user, err := a.loadUser(ctx, userID)
		if err != nil {
			a.log.WithError(err).WithField("user_id", userID).Warn("Failed to load user")
			if a.config.AllowOptional {
				ctx = a.setUnauthenticatedContext(ctx)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			if errors.Is(err, interfaces.ErrUserNotFound) {
				a.sendError(w, http.StatusUnauthorized, "User not found", "user_not_found")
			} else {
				a.sendError(w, http.StatusInternalServerError, "Failed to load user", "user_load_error")
			}
			return
		}
		
		// Check user status
		if err := a.validateUserStatus(user); err != nil {
			a.sendError(w, http.StatusForbidden, err.Error(), "user_status_invalid")
			return
		}
		
		// Extract role from claims or user
		role := a.extractRole(claims, user)
		
		// Build permissions
		permissions := a.buildPermissions(user, role)
		
		// Set context
		ctx = a.setAuthenticatedContext(ctx, user, role, claims, permissions, tokenString)
		
		// Update last active (async, don't block)
		go a.updateLastActive(user.ID)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ======================================================================
// Token Extraction
// ======================================================================

// extractToken extracts JWT token from various sources.
func (a *AuthMiddleware) extractToken(r *http.Request) string {
	// 1. Authorization header (Bearer)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	
	// 2. Cookie
	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	
	// 3. Query parameter (for WebSocket, debugging)
	if token := r.URL.Query().Get("token"); token != "" && a.config.AllowWebSocket {
		return token
	}
	
	// 4. Custom header
	if token := r.Header.Get("X-Access-Token"); token != "" {
		return token
	}
	
	return ""
}

// extractTokenFromWebSocket extracts token from WebSocket request.
func (a *AuthMiddleware) extractTokenFromWebSocket(conn *websocket.Conn) string {
	// WebSocket handshake headers
	authHeader := conn.Request().Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	// Query param
	return conn.Request().URL.Query().Get("token")
}

// ======================================================================
// Token Validation
// ======================================================================

// validateToken parses and validates the JWT token.
func (a *AuthMiddleware) validateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})
	if err != nil {
		// Check specific error types
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrTokenMalformed
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, ErrTokenSignatureInvalid
		}
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidTokenClaims
	}
	
	// Validate issuer
	if a.config.JWTIssuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != a.config.JWTIssuer {
			return nil, ErrInvalidIssuer
		}
	}
	
	// Validate audience
	if a.config.JWTAudience != "" {
		if aud, ok := claims["aud"].(string); !ok || aud != a.config.JWTAudience {
			return nil, ErrInvalidAudience
		}
	}
	
	// Validate expiry
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, ErrTokenExpired
		}
	} else {
		return nil, ErrMissingExpiry
	}
	
	// Validate not before
	if nbf, ok := claims["nbf"].(float64); ok {
		if time.Now().Unix() < int64(nbf) {
			return nil, ErrTokenNotYetValid
		}
	}
	
	return claims, nil
}

// ======================================================================
// Token Blacklist
// ======================================================================

// isTokenBlacklisted checks if token is in Redis blacklist.
func (a *AuthMiddleware) isTokenBlacklisted(ctx context.Context, token string) bool {
	if a.config.RedisAdapter == nil {
		return false
	}
	key := a.config.BlacklistPrefix + token
	exists, err := a.config.RedisAdapter.Exists(ctx, key)
	if err != nil {
		a.log.WithError(err).Warn("Failed to check token blacklist")
		return false
	}
	return exists > 0
}

// BlacklistToken adds a token to the blacklist with its remaining TTL.
func (a *AuthMiddleware) BlacklistToken(ctx context.Context, token string) error {
	if a.config.RedisAdapter == nil {
		return errors.New("redis adapter not configured")
	}
	
	claims, err := a.validateToken(token)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			// Already expired, no need to blacklist
			return nil
		}
		return err
	}
	
	exp, ok := claims["exp"].(float64)
	if !ok {
		return errors.New("token missing expiry claim")
	}
	
	ttl := time.Until(time.Unix(int64(exp), 0))
	if ttl <= 0 {
		return nil // Already expired
	}
	
	key := a.config.BlacklistPrefix + token
	if err := a.config.RedisAdapter.Set(ctx, key, "revoked", ttl); err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}
	
	a.log.WithField("ttl", ttl).Debug("Token blacklisted")
	return nil
}

// ======================================================================
// User Loading with Caching
// ======================================================================

// loadUser loads user from cache or database.
func (a *AuthMiddleware) loadUser(ctx context.Context, userID string) (*entities.User, error) {
	// Try cache first
	if a.config.EnableUserCache && a.config.RedisAdapter != nil {
		cacheKey := a.config.UserCachePrefix + userID
		var user entities.User
		if err := a.config.RedisAdapter.GetJSON(ctx, cacheKey, &user); err == nil {
			a.log.WithField("user_id", userID).Debug("User loaded from cache")
			return &user, nil
		}
	}
	
	// Load from database
	user, err := a.config.UserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// Store in cache
	if a.config.EnableUserCache && a.config.RedisAdapter != nil && a.config.UserCacheTTL > 0 {
		cacheKey := a.config.UserCachePrefix + userID
		if err := a.config.RedisAdapter.CacheSet(ctx, cacheKey, user, a.config.UserCacheTTL); err != nil {
			a.log.WithError(err).Warn("Failed to cache user")
		}
	}
	
	return user, nil
}

// InvalidateUserCache invalidates cached user data.
func (a *AuthMiddleware) InvalidateUserCache(ctx context.Context, userID string) error {
	if !a.config.EnableUserCache || a.config.RedisAdapter == nil {
		return nil
	}
	cacheKey := a.config.UserCachePrefix + userID
	return a.config.RedisAdapter.Delete(ctx, cacheKey)
}

// ======================================================================
// User Status Validation
// ======================================================================

// validateUserStatus checks if user can authenticate.
func (a *AuthMiddleware) validateUserStatus(user *entities.User) error {
	if user == nil {
		return errors.New("user is nil")
	}
	
	if a.config.RequireUserActive && !user.IsActive {
		return errors.New("user account is inactive")
	}
	
	if user.IsSuspended {
		return errors.New("user account is suspended")
	}
	
	if a.config.RequireUserVerified && !user.IsVerified {
		return errors.New("email not verified")
	}
	
	if user.DeletedAt != nil {
		return errors.New("user account has been deleted")
	}
	
	return nil
}

// ======================================================================
= Claim Extraction
// ======================================================================

// extractUserID extracts user ID from claims.
func (a *AuthMiddleware) extractUserID(claims jwt.MapClaims) (string, error) {
	// Try "sub" claim
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		return sub, nil
	}
	// Try "user_id" claim
	if userID, ok := claims["user_id"].(string); ok && userID != "" {
		return userID, nil
	}
	// Try "id" claim
	if id, ok := claims["id"].(string); ok && id != "" {
		return id, nil
	}
	return "", errors.New("no user identifier found in claims")
}

// extractRole extracts role from claims or user.
func (a *AuthMiddleware) extractRole(claims jwt.MapClaims, user *entities.User) entities.UserRole {
	// Try claims first
	if roleStr, ok := claims["role"].(string); ok && roleStr != "" {
		switch entities.UserRole(roleStr) {
		case entities.RoleAdmin, entities.RoleModerator, entities.RoleUser:
			return entities.UserRole(roleStr)
		}
	}
	// Fallback to user role
	return user.Role
}

// ======================================================================
// Permissions
// ======================================================================

// UserPermissions represents a user's permissions.
type UserPermissions struct {
	CanAdmin      bool `json:"can_admin"`
	CanModerate   bool `json:"can_moderate"`
	CanDelete     bool `json:"can_delete"`
	CanBan        bool `json:"can_ban"`
	CanViewReports bool `json:"can_view_reports"`
	CanManageUsers bool `json:"can_manage_users"`
	CanManageTweets bool `json:"can_manage_tweets"`
}

// buildPermissions builds permissions based on user role.
func (a *AuthMiddleware) buildPermissions(user *entities.User, role entities.UserRole) *UserPermissions {
	perms := &UserPermissions{}
	
	switch role {
	case entities.RoleAdmin:
		perms.CanAdmin = true
		perms.CanModerate = true
		perms.CanDelete = true
		perms.CanBan = true
		perms.CanViewReports = true
		perms.CanManageUsers = true
		perms.CanManageTweets = true
		
	case entities.RoleModerator:
		perms.CanModerate = true
		perms.CanDelete = true
		perms.CanViewReports = true
		perms.CanManageTweets = true
		
	default:
		perms.CanModerate = false
		perms.CanDelete = false
	}
	
	return perms
}

// ======================================================================
// Context Management
// ======================================================================

// setAuthenticatedContext sets the authenticated context.
func (a *AuthMiddleware) setAuthenticatedContext(ctx context.Context, user *entities.User, role entities.UserRole, claims jwt.MapClaims, perms *UserPermissions, token string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, user.ID)
	ctx = context.WithValue(ctx, UserKey, user)
	ctx = context.WithValue(ctx, UserRoleKey, role)
	ctx = context.WithValue(ctx, UserClaimsKey, claims)
	ctx = context.WithValue(ctx, UserPermissionsKey, perms)
	ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
	ctx = context.WithValue(ctx, TokenKey, token)
	return ctx
}

// setUnauthenticatedContext sets the unauthenticated context.
func (a *AuthMiddleware) setUnauthenticatedContext(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, IsAuthenticatedKey, false)
	return ctx
}

// ======================================================================
// Error Types
// ======================================================================

var (
	ErrTokenExpired          = errors.New("token has expired")
	ErrTokenMalformed        = errors.New("token is malformed")
	ErrTokenSignatureInvalid = errors.New("token signature is invalid")
	ErrInvalidToken          = errors.New("token is invalid")
	ErrInvalidTokenClaims    = errors.New("invalid token claims")
	ErrInvalidIssuer         = errors.New("invalid token issuer")
	ErrInvalidAudience       = errors.New("invalid token audience")
	ErrMissingExpiry         = errors.New("token missing expiry claim")
	ErrTokenNotYetValid      = errors.New("token not yet valid")
)

// ======================================================================
// Response Helpers
// ======================================================================

// sendError sends an error response.
func (a *AuthMiddleware) sendError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"error":   message,
		"code":    code,
		"status":  status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		a.log.WithError(err).Error("Failed to encode error response")
	}
}

// ======================================================================
= Background Tasks
// ======================================================================

// updateLastActive updates user's last active timestamp (async).
func (a *AuthMiddleware) updateLastActive(userID string) {
	if a.config.UserRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.config.UserRepo.UpdateLastActive(ctx, userID); err != nil {
		a.log.WithError(err).WithField("user_id", userID).Warn("Failed to update last active")
	}
}

// ======================================================================
// Context Helper Functions (Public)
// ======================================================================

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

// GetPermissions extracts user permissions from context.
func GetPermissions(ctx context.Context) (*UserPermissions, error) {
	val := ctx.Value(UserPermissionsKey)
	if val == nil {
		return nil, errors.New("permissions not found in context")
	}
	perms, ok := val.(*UserPermissions)
	if !ok {
		return nil, errors.New("invalid permissions type")
	}
	return perms, nil
}

// IsAuthenticated checks if request is authenticated.
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

// GetToken returns the raw token from context.
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

// ======================================================================
// Role-Based Authorization Middleware
// ======================================================================

// RequireRole returns a middleware that requires a specific role.
func RequireRole(allowedRoles ...entities.UserRole) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, err := GetUserRole(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			for _, role := range allowedRoles {
				if userRole == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}

// RequireAdmin is a shortcut for RequireRole(entities.RoleAdmin).
func RequireAdmin(next http.Handler) http.Handler {
	return RequireRole(entities.RoleAdmin)(next)
}

// RequireAdminOrModerator is a shortcut.
func RequireAdminOrModerator(next http.Handler) http.Handler {
	return RequireRole(entities.RoleAdmin, entities.RoleModerator)(next)
}

// RequirePermission returns a middleware that requires a specific permission.
func RequirePermission(permission string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			perms, err := GetPermissions(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			hasPermission := false
			switch permission {
			case "admin":
				hasPermission = perms.CanAdmin
			case "moderate":
				hasPermission = perms.CanModerate
			case "delete":
				hasPermission = perms.CanDelete
			case "ban":
				hasPermission = perms.CanBan
			default:
				hasPermission = false
			}
			if !hasPermission {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ======================================================================
// Optional Authentication Wrapper
// ======================================================================

// OptionalAuth wraps a handler with optional authentication.
func (a *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	clone := *a
	clone.config.AllowOptional = true
	return clone.Middleware(next)
}

// ======================================================================
// WebSocket Authentication
// ======================================================================

// AuthenticateWebSocket authenticates a WebSocket connection.
func (a *AuthMiddleware) AuthenticateWebSocket(conn *websocket.Conn) (*entities.User, error) {
	tokenString := a.extractTokenFromWebSocket(conn)
	if tokenString == "" {
		return nil, errors.New("missing authentication token")
	}
	
	claims, err := a.validateToken(tokenString)
	if err != nil {
		return nil, err
	}
	
	if a.config.EnableBlacklist && a.config.RedisAdapter != nil {
		if a.isTokenBlacklisted(context.Background(), tokenString) {
			return nil, errors.New("token revoked")
		}
	}
	
	userID, err := a.extractUserID(claims)
	if err != nil {
		return nil, err
	}
	
	user, err := a.loadUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	
	if err := a.validateUserStatus(user); err != nil {
		return nil, err
	}
	
	return user, nil
}

// ======================================================================
// Test Helpers
// ======================================================================

// WithUserContext creates a context with user info for testing.
func WithUserContext(ctx context.Context, user *entities.User) context.Context {
	perms := &UserPermissions{}
	ctx = context.WithValue(ctx, UserIDKey, user.ID)
	ctx = context.WithValue(ctx, UserKey, user)
	ctx = context.WithValue(ctx, UserRoleKey, user.Role)
	ctx = context.WithValue(ctx, UserPermissionsKey, perms)
	ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
	return ctx
}

// WithAdminContext creates a context with admin user for testing.
func WithAdminContext(ctx context.Context, admin *entities.User) context.Context {
	admin.Role = entities.RoleAdmin
	return WithUserContext(ctx, admin)
}

// MockUserRepository for testing.
type MockUserRepository struct {
	Users map[string]*entities.User
	Error error
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*entities.User, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if user, ok := m.Users[id]; ok {
		return user, nil
	}
	return nil, interfaces.ErrUserNotFound
}

// ======================================================================
// Health Check
// ======================================================================

// HealthCheck checks the health of the middleware dependencies.
func (a *AuthMiddleware) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"component": "auth_middleware",
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	if a.config.RedisAdapter != nil {
		if err := a.config.RedisAdapter.Ping(r.Context()); err != nil {
			status["status"] = "degraded"
			status["redis"] = "unavailable"
			status["error"] = err.Error()
		} else {
			status["redis"] = "available"
		}
	}
	
	if a.config.UserRepo != nil {
		if err := a.config.UserRepo.Ping(r.Context()); err != nil {
			status["status"] = "degraded"
			status["database"] = "unavailable"
			status["error"] = err.Error()
		} else {
			status["database"] = "available"
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	if status["status"] == "ok" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(status)
}