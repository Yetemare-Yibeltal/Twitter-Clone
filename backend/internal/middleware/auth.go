// backend/internal/middleware/auth.go
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// Context keys for storing user information.
type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	UserKey      contextKey = "user"
	UserRoleKey  contextKey = "user_role"
	UserClaimsKey contextKey = "user_claims"
	IsAuthenticatedKey contextKey = "is_authenticated"
)

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	JWTSecret        string
	RedisAdapter     adapter.RedisAdapter
	UserRepo         interfaces.UserRepository
	// Optional: cache user TTL
	UserCacheTTL     time.Duration
	// Optional: token blacklist enabled
	EnableBlacklist  bool
	// Optional: allow unauthenticated requests
	AllowOptional    bool
}

// AuthMiddleware is the main middleware struct.
type AuthMiddleware struct {
	config     AuthConfig
	log        *logrus.Entry
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(cfg AuthConfig) *AuthMiddleware {
	return &AuthMiddleware{
		config: cfg,
		log:    logger.WithField("middleware", "auth"),
	}
}

// Middleware returns a middleware function that authenticates requests.
func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Try to extract token
		tokenString := a.extractToken(r)
		if tokenString == "" {
			if a.config.AllowOptional {
				// Set unauthenticated context and proceed
				ctx = context.WithValue(ctx, IsAuthenticatedKey, false)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			a.sendError(w, http.StatusUnauthorized, "Missing authentication token")
			return
		}
		// Validate token and get claims
		claims, err := a.validateToken(tokenString)
		if err != nil {
			a.log.WithError(err).Debug("Token validation failed")
			if a.config.AllowOptional {
				ctx = context.WithValue(ctx, IsAuthenticatedKey, false)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			a.sendError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}
		// Check blacklist if enabled
		if a.config.EnableBlacklist && a.config.RedisAdapter != nil {
			if a.isTokenBlacklisted(ctx, tokenString) {
				a.sendError(w, http.StatusUnauthorized, "Token revoked")
				return
			}
		}
		// Get user ID from claims
		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			a.sendError(w, http.StatusUnauthorized, "Invalid token claims")
			return
		}
		// Get role from claims
		role, _ := claims["role"].(string)
		// Load user from DB (or cache)
		user, err := a.loadUser(ctx, userID)
		if err != nil {
			a.log.WithError(err).WithField("user_id", userID).Warn("Failed to load user")
			if a.config.AllowOptional {
				// Allow but mark as unauthenticated? Or deny? We'll deny to be safe.
				a.sendError(w, http.StatusUnauthorized, "User not found")
				return
			}
			a.sendError(w, http.StatusUnauthorized, "User not found")
			return
		}
		// Check user status
		if user.IsSuspended {
			a.sendError(w, http.StatusForbidden, "Account suspended")
			return
		}
		if !user.IsActive {
			a.sendError(w, http.StatusForbidden, "Account inactive")
			return
		}
		// Store user and claims in context
		ctx = context.WithValue(ctx, UserIDKey, user.ID)
		ctx = context.WithValue(ctx, UserKey, user)
		ctx = context.WithValue(ctx, UserRoleKey, entities.UserRole(role))
		ctx = context.WithValue(ctx, UserClaimsKey, claims)
		ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
		// Update last active (optional, could be done async)
		// go a.userRepo.UpdateLastActive(context.Background(), user.ID)
		// Proceed
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalMiddleware returns a middleware that authenticates if token present,
// but allows unauthenticated requests.
func (a *AuthMiddleware) OptionalMiddleware(next http.Handler) http.Handler {
	// Set AllowOptional true for this instance.
	clone := *a
	clone.config.AllowOptional = true
	return clone.Middleware(next)
}

// extractToken extracts JWT token from Authorization header or cookie.
func (a *AuthMiddleware) extractToken(r *http.Request) string {
	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
		// Maybe token is directly in header (no Bearer)
		if len(parts) == 1 {
			return parts[0]
		}
	}
	// Check cookie
	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	// Check query parameter (for WebSocket or debugging)
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	return ""
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
	// Check expiry
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, errors.New("token expired")
		}
	} else {
		return nil, errors.New("token missing expiry")
	}
	return claims, nil
}

// isTokenBlacklisted checks if the token is in Redis blacklist.
func (a *AuthMiddleware) isTokenBlacklisted(ctx context.Context, token string) bool {
	if a.config.RedisAdapter == nil {
		return false
	}
	key := "blacklist:" + token
	exists, err := a.config.RedisAdapter.Exists(ctx, key)
	if err != nil {
		a.log.WithError(err).Warn("Failed to check token blacklist")
		return false
	}
	return exists > 0
}

// loadUser loads user from DB or cache.
func (a *AuthMiddleware) loadUser(ctx context.Context, userID string) (*entities.User, error) {
	// Try cache first if Redis available and TTL set
	if a.config.RedisAdapter != nil && a.config.UserCacheTTL > 0 {
		cacheKey := "user:" + userID
		var user entities.User
		if err := a.config.RedisAdapter.GetJSON(ctx, cacheKey, &user); err == nil {
			return &user, nil
		}
	}
	// Load from DB
	user, err := a.config.UserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Store in cache if Redis available
	if a.config.RedisAdapter != nil && a.config.UserCacheTTL > 0 {
		cacheKey := "user:" + userID
		_ = a.config.RedisAdapter.CacheSet(ctx, cacheKey, user, a.config.UserCacheTTL)
	}
	return user, nil
}

// sendError sends an HTTP error response.
func (a *AuthMiddleware) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
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

// ---- Role-based authorization middleware ----

// RequireRole returns a middleware that requires a specific role.
func RequireRole(allowedRoles ...entities.UserRole) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, err := GetUserRole(r.Context())
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			allowed := false
			for _, role := range allowedRoles {
				if userRole == role {
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

// RequireAdmin is a shortcut for RequireRole(RoleAdmin).
func RequireAdmin(next http.Handler) http.Handler {
	return RequireRole(entities.RoleAdmin)(next)
}

// RequireAdminOrModerator is a shortcut.
func RequireAdminOrModerator(next http.Handler) http.Handler {
	return RequireRole(entities.RoleAdmin, entities.RoleModerator)(next)
}

// ---- Blacklist management ----

// BlacklistToken adds a token to the blacklist with its remaining TTL.
func (a *AuthMiddleware) BlacklistToken(ctx context.Context, token string) error {
	if a.config.RedisAdapter == nil {
		return errors.New("redis not configured")
	}
	// Parse token to get expiry
	claims, err := a.validateToken(token)
	if err != nil {
		return err
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return errors.New("token missing expiry")
	}
	ttl := time.Until(time.Unix(int64(exp), 0))
	if ttl <= 0 {
		// Already expired, no need to blacklist
		return nil
	}
	key := "blacklist:" + token
	return a.config.RedisAdapter.Set(ctx, key, "1", ttl)
}

// ---- Helper to generate auth context for tests ----

// WithUserContext creates a context with user info for testing.
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

// ---- OpenAPI/ Swagger annotations (optional) ----
// These are just for documentation; not needed for code.

// ---- Security ----

// SecureHeaders middleware adds security headers (could be separate).
// But we can include a helper.

// ---- Logging ----

func (a *AuthMiddleware) logWithClaims(claims jwt.MapClaims) *logrus.Entry {
	entry := a.log.WithFields(logrus.Fields{
		"sub": claims["sub"],
		"exp": claims["exp"],
	})
	if role, ok := claims["role"]; ok {
		entry = entry.WithField("role", role)
	}
	return entry
}

// ---- Optional: token refresh logic ----
// This could be implemented here but usually handled in service.

// ---- Route protection helpers ----

// ProtectRoute wraps a handler with auth middleware and optional role.
func (a *AuthMiddleware) ProtectRoute(handler http.Handler, roles ...entities.UserRole) http.Handler {
	// First apply auth
	handler = a.Middleware(handler)
	if len(roles) > 0 {
		handler = RequireRole(roles...)(handler)
	}
	return handler
}

// ---- Health check for middleware ----
func (a *AuthMiddleware) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check if Redis is available if configured
	if a.config.RedisAdapter != nil {
		if err := a.config.RedisAdapter.Ping(r.Context()); err != nil {
			http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","component":"auth_middleware"}`))
}