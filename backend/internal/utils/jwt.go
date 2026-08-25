// backend/internal/utils/jwt.go
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"twitter-clone/backend/internal/adapter"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	DefaultAccessTokenExpiry  = 15 * time.Minute
	DefaultRefreshTokenExpiry = 7 * 24 * time.Hour
	TokenTypeAccess           = "access"
	TokenTypeRefresh          = "refresh"
	TokenTypeVerification     = "verification"
	TokenTypePasswordReset    = "password_reset"
)

var (
	ErrInvalidToken         = errors.New("invalid token")
	ErrExpiredToken         = errors.New("token has expired")
	ErrTokenMalformed       = errors.New("token is malformed")
	ErrTokenSignatureInvalid = errors.New("token signature is invalid")
	ErrTokenClaimsInvalid   = errors.New("token claims are invalid")
	ErrTokenTypeMismatch    = errors.New("token type mismatch")
	ErrTokenBlacklisted     = errors.New("token is blacklisted")
	ErrTokenIssuerInvalid   = errors.New("invalid token issuer")
	ErrTokenAudienceInvalid = errors.New("invalid token audience")
	ErrTokenNotYetValid     = errors.New("token not yet valid")
	ErrTokenGenerationFailed = errors.New("failed to generate token")
	ErrTokenRefreshFailed   = errors.New("failed to refresh token")
)

// ======================================================================
= JWTConfig
// ======================================================================

// JWTConfig holds JWT configuration.
type JWTConfig struct {
	Secret          string
	Issuer          string
	Audience        string
	AccessExpiry    time.Duration
	RefreshExpiry   time.Duration
	VerificationExpiry time.Duration
	ResetExpiry     time.Duration
	BlacklistEnabled bool
}

// DefaultJWTConfig returns sensible defaults.
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		Secret:          "change-me-in-production",
		Issuer:          "twitter-clone",
		Audience:        "twitter-clone-users",
		AccessExpiry:    DefaultAccessTokenExpiry,
		RefreshExpiry:   DefaultRefreshTokenExpiry,
		VerificationExpiry: 24 * time.Hour,
		ResetExpiry:     1 * time.Hour,
		BlacklistEnabled: true,
	}
}

// ======================================================================
= Custom Claims
// ======================================================================

// CustomClaims extends jwt.RegisteredClaims with custom fields.
type CustomClaims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TokenType string `json:"token_type"`
	Email    string `json:"email,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// ======================================================================
= JWT Manager
// ======================================================================

// JWTManager handles JWT operations.
type JWTManager struct {
	config       JWTConfig
	redisAdapter adapter.RedisAdapter
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(config JWTConfig, redisAdapter adapter.RedisAdapter) *JWTManager {
	return &JWTManager{
		config:       config,
		redisAdapter: redisAdapter,
	}
}

// ======================================================================
= Token Generation
// ======================================================================

// GenerateAccessToken generates a new access token.
func (m *JWTManager) GenerateAccessToken(userID, username, role string) (string, error) {
	return m.generateToken(userID, username, role, "", "", TokenTypeAccess, m.config.AccessExpiry)
}

// GenerateRefreshToken generates a new refresh token (opaque).
func (m *JWTManager) GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateVerificationToken generates an email verification token.
func (m *JWTManager) GenerateVerificationToken(userID, email string) (string, error) {
	return m.generateToken(userID, "", "", email, "", TokenTypeVerification, m.config.VerificationExpiry)
}

// GeneratePasswordResetToken generates a password reset token.
func (m *JWTManager) GeneratePasswordResetToken(userID, email string) (string, error) {
	return m.generateToken(userID, "", "", email, "", TokenTypePasswordReset, m.config.ResetExpiry)
}

// GenerateCustomToken generates a token with custom claims.
func (m *JWTManager) GenerateCustomToken(claims CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.config.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
}

// generateToken is the internal token generation method.
func (m *JWTManager) generateToken(userID, username, role, email, deviceID, tokenType string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   userID,
			Issuer:    m.config.Issuer,
			Audience:  jwt.ClaimStrings{m.config.Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: tokenType,
		Email:     email,
		DeviceID:  deviceID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.config.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
}

// ======================================================================
= Token Validation
// ======================================================================

// ValidateToken validates a JWT token and returns the claims.
func (m *JWTManager) ValidateToken(tokenString string) (*CustomClaims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.config.Secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
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
	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, ErrTokenClaimsInvalid
	}
	// Validate issuer
	if claims.Issuer != m.config.Issuer {
		return nil, ErrTokenIssuerInvalid
	}
	// Validate audience
	if len(claims.Audience) > 0 && claims.Audience[0] != m.config.Audience {
		return nil, ErrTokenAudienceInvalid
	}
	// Validate not before
	if claims.NotBefore != nil && claims.NotBefore.Time.After(time.Now()) {
		return nil, ErrTokenNotYetValid
	}
	// Check blacklist
	if m.config.BlacklistEnabled && m.redisAdapter != nil {
		blacklisted, err := m.IsTokenBlacklisted(tokenString)
		if err != nil {
			return nil, err
		}
		if blacklisted {
			return nil, ErrTokenBlacklisted
		}
	}
	return claims, nil
}

// ValidateAccessToken validates an access token.
func (m *JWTManager) ValidateAccessToken(tokenString string) (*CustomClaims, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, ErrTokenTypeMismatch
	}
	return claims, nil
}

// ValidateRefreshToken validates a refresh token (checks if it exists in storage).
// This is usually handled by the session repository; here we just check the token format.
func (m *JWTManager) ValidateRefreshToken(refreshToken string) (bool, error) {
	if len(refreshToken) < 32 {
		return false, fmt.Errorf("invalid refresh token length")
	}
	// Additional validation could be done here (e.g., check against DB)
	return true, nil
}

// ValidateVerificationToken validates an email verification token.
func (m *JWTManager) ValidateVerificationToken(tokenString string) (*CustomClaims, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeVerification {
		return nil, ErrTokenTypeMismatch
	}
	if claims.Email == "" {
		return nil, errors.New("verification token missing email")
	}
	return claims, nil
}

// ValidatePasswordResetToken validates a password reset token.
func (m *JWTManager) ValidatePasswordResetToken(tokenString string) (*CustomClaims, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypePasswordReset {
		return nil, ErrTokenTypeMismatch
	}
	if claims.Email == "" {
		return nil, errors.New("reset token missing email")
	}
	return claims, nil
}

// ======================================================================
= Token Parsing (without validation)
// ======================================================================

// ParseToken parses a token without validating signature/expiry.
func (m *JWTManager) ParseToken(tokenString string) (*CustomClaims, error) {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &CustomClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, ErrTokenClaimsInvalid
	}
	return claims, nil
}

// GetUserIDFromToken extracts user ID from a token without full validation.
func (m *JWTManager) GetUserIDFromToken(tokenString string) (string, error) {
	claims, err := m.ParseToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// GetTokenExpiry returns the expiry time of a token.
func (m *JWTManager) GetTokenExpiry(tokenString string) (time.Time, error) {
	claims, err := m.ParseToken(tokenString)
	if err != nil {
		return time.Time{}, err
	}
	if claims.ExpiresAt == nil {
		return time.Time{}, errors.New("token has no expiry")
	}
	return claims.ExpiresAt.Time, nil
}

// ======================================================================
= Token Blacklist
// ======================================================================

// BlacklistToken adds a token to the blacklist.
func (m *JWTManager) BlacklistToken(tokenString string) error {
	if m.redisAdapter == nil {
		return errors.New("redis adapter not configured")
	}
	if !m.config.BlacklistEnabled {
		return nil
	}
	// Get token expiry
	expiry, err := m.GetTokenExpiry(tokenString)
	if err != nil {
		return err
	}
	ttl := time.Until(expiry)
	if ttl <= 0 {
		// Token already expired; no need to blacklist
		return nil
	}
	key := "jwt_blacklist:" + tokenString
	if err := m.redisAdapter.Set(context.Background(), key, "revoked", ttl); err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}
	return nil
}

// IsTokenBlacklisted checks if a token is blacklisted.
func (m *JWTManager) IsTokenBlacklisted(tokenString string) (bool, error) {
	if m.redisAdapter == nil {
		return false, nil
	}
	if !m.config.BlacklistEnabled {
		return false, nil
	}
	key := "jwt_blacklist:" + tokenString
	exists, err := m.redisAdapter.Exists(context.Background(), key)
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}
	return exists > 0, nil
}

// RevokeToken revokes a token by blacklisting it.
func (m *JWTManager) RevokeToken(tokenString string) error {
	return m.BlacklistToken(tokenString)
}

// RevokeAllUserTokens revokes all tokens for a user (by blacklisting all active tokens).
// This would require storing user's active token IDs; this is a placeholder.
func (m *JWTManager) RevokeAllUserTokens(userID string) error {
	// In a real implementation, you would store all issued token IDs per user
	// and delete them or blacklist them here.
	// For simplicity, we'll just log a warning.
	return nil
}

// ======================================================================
= Token Refresh
// ======================================================================

// RefreshAccessToken generates a new access token from a refresh token.
// The refresh token is validated against stored sessions; we just return a new access token.
func (m *JWTManager) RefreshAccessToken(userID, username, role string) (string, error) {
	return m.GenerateAccessToken(userID, username, role)
}

// ======================================================================
= Utility Functions
// ======================================================================

// GetTokenStringFromHeader extracts the token from the Authorization header.
func GetTokenStringFromHeader(authHeader string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}

// GetTokenStringFromCookie extracts the token from a cookie (if needed).
func GetTokenStringFromCookie(cookie string) string {
	return cookie
}

// ======================================================================
= Middleware Helpers
// ======================================================================

// ExtractUserIDFromContext extracts user ID from request context.
// This is used by middleware to get the user ID after validation.
func ExtractUserIDFromContext(ctx context.Context) string {
	// This would be implemented in middleware
	return ""
}

// ======================================================================
= Test Helpers
// ======================================================================

// GenerateTestToken generates a token for testing.
func GenerateTestToken(userID, username, role string) string {
	manager := NewJWTManager(DefaultJWTConfig(), nil)
	token, _ := manager.GenerateAccessToken(userID, username, role)
	return token
}

// ======================================================================
= Service Integration
// ======================================================================

// CreateJWTService creates a JWT service with given config and Redis.
func CreateJWTService(secret, issuer, audience string, accessExpiry, refreshExpiry time.Duration, redisAdapter adapter.RedisAdapter) *JWTManager {
	config := DefaultJWTConfig()
	config.Secret = secret
	config.Issuer = issuer
	config.Audience = audience
	config.AccessExpiry = accessExpiry
	config.RefreshExpiry = refreshExpiry
	return NewJWTManager(config, redisAdapter)
}

// ======================================================================
= Validation Extensions
// ======================================================================

// ValidateTokenWithRole validates a token and checks role.
func (m *JWTManager) ValidateTokenWithRole(tokenString string, allowedRoles []string) (*CustomClaims, error) {
	claims, err := m.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, err
	}
	if len(allowedRoles) > 0 {
		allowed := false
		for _, r := range allowedRoles {
			if claims.Role == r {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, errors.New("insufficient role permissions")
		}
	}
	return claims, nil
}

// ExtractClaimsFromRequest extracts claims from an HTTP request (using context).
// This is a placeholder for integration with middleware.
func ExtractClaimsFromRequest(r *http.Request) (*CustomClaims, error) {
	// In production, this would retrieve claims from context set by middleware.
	return nil, nil
}