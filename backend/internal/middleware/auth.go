// backend/internal/middleware/auth.go
package middleware

import (
	"context"
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
)

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	JWTSecret        string
	RedisClient      *redis.Client
	UserRepo         interfaces.UserRepository
	UserCacheTTL     time.Duration
	EnableBlacklist  bool
	AllowOptional    bool
	TokenLookup      []string // "header", "cookie", "query"
	HeaderName       string
	CookieName       string
	QueryParamName   string
}

// AuthMiddleware is the main middleware struct.
type AuthMiddleware struct {
	config AuthConfig
	log    *logrus.Entry
}

// NewAuthMiddleware creates a new auth middleware instance.
func NewAuthMiddleware(cfg AuthConfig) *AuthMiddleware {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Authorization"
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "access_token"
	}
	if cfg.QueryParamName == "" {
		cfg.QueryParamName = "token"
	}
	if len(cfg.TokenLookup) == 0 {
		cfg.TokenLookup = []string{"header", "cookie", "query"}
	}
	if cfg.UserCacheTTL == 0 {
		cfg.UserCacheTTL = 5 * time.Minute
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

		// Extract token from request
		tokenString := a.extractToken(r)
		if tokenString == "" {
			if a.config.AllowOptional {
				ctx = context.WithValue(ctx, IsAuthenticatedKey, false)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			a.sendError(w, http.StatusUnauthorized, "Missing authentication token")
			return
		}

		// Validate token and extract claims
		claims, err := a.validateToken(tokenString)
		if err != nil {
			a.log.WithError(err).Debug("Token validation failed")
			if a.config.AllowOptional {
				ctx = context.WithValue(ctx, IsAuthenticatedKey, false)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			if errors.Is(err, jwt.ErrTokenExpired) {
				a.sendError(w, http.StatusUnauthorized, "Token has expired")
				return
			}
			a.sendError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Check token blacklist
		if a.config.EnableBlacklist && a.config.RedisClient != nil {
			if a.isTokenBlacklisted(ctx, tokenString) {
				a.sendError(w, http.StatusUnauthorized, "Token has been revoked")
				return
			}
		}

		// Extract user ID from claims
		userID, err := a.getUserIDFromClaims(claims)
		if err != nil {
			a.sendError(w, http.StatusUnauthorized, "Invalid token claims")
			return
		}

		// Load user from database or cache
		user, err := a.loadUser(ctx, userID)
		if err != nil {
			if errors.Is(err, interfaces.ErrUserNotFound) {
				a.sendError(w, http.StatusUnauthorized, "User not found")
				return
			}
			a.log.WithError(err).WithField("user_id", userID).Error("Failed to load user")
			a.sendError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		// Check user status
		if user.IsSuspended {
			a.sendError(w, http.StatusForbidden, "Account has been suspended")
			return
		}
		if !user.IsActive {
			a.sendError(w, http.StatusForbidden, "Account is inactive")
			return
		}

		// Extract role from claims
		role := a.getRoleFromClaims(claims)

		// Populate context with user information
		ctx = context.WithValue(ctx, UserIDKey, user.ID)
		ctx = context.WithValue(ctx, UserKey, user)
		ctx = context.WithValue(ctx, UserRoleKey, entities.UserRole(role))
		ctx = context.WithValue(ctx, UserClaimsKey, claims)
		ctx = context.WithValue(ctx, IsAuthenticatedKey, true)
		ctx = context.WithValue(ctx, TokenKey, tokenString)

		// Update last active asynchronously
		go a.updateLastActive(user.ID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalMiddleware returns a middleware that authenticates if token present.
func (a *AuthMiddleware) OptionalMiddleware(next http.Handler) http.Handler {
	clone := *a
	clone.config.AllowOptional = true
	return clone.Middleware(next)
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

// getRoleFromClaims extracts role from JWT claims.
func (a *AuthMiddleware) getRoleFromClaims(claims jwt.MapClaims) string {
	role, ok := claims["role"].(string)
	if !ok {
		return string(entities.RoleUser)
	}
	return role
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

// sendError sends a JSON error response.
func (a *AuthMiddleware) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := map[string]interface{}{
		"error":   message,
		"status":  status,
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
		return nil // Already expired
	}
	key := "blacklist:" + token
	return a.config.RedisClient.Set(ctx, key, "1", ttl).Err()
}

// RevokeAllUserTokens revokes all tokens for a user.
func (a *AuthMiddleware) RevokeAllUserTokens(ctx context.Context, userID string) error {
	if a.config.RedisClient == nil {
		return errors.New("Redis client not configured")
	}
	key := "user_tokens:" + userID
	// Store revocation timestamp
	return a.config.RedisClient.Set(ctx, key, time.Now().Unix(), 7*24*time.Hour).Err()
}

// IsUserTokenRevoked checks if all tokens for a user are revoked.
func (a *AuthMiddleware) IsUserTokenRevoked(ctx context.Context, userID string) (bool, error) {
	if a.config.RedisClient == nil {
		return false, nil
	}
	key := "user_tokens:" + userID
	_, err := a.config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
		"status": "ok",
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