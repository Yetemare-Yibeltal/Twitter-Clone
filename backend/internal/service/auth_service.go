// backend/internal/service/auth_service.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/domain/valueobjects"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// Common auth errors.
var (
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrUserNotFound            = errors.New("user not found")
	ErrUserSuspended           = errors.New("user account is suspended")
	ErrUserInactive            = errors.New("user account is inactive")
	ErrUserNotVerified         = errors.New("email not verified")
	ErrInvalidToken            = errors.New("invalid or expired token")
	ErrTokenExpired            = errors.New("token has expired")
	ErrEmailAlreadyVerified    = errors.New("email already verified")
	ErrPasswordResetExpired    = errors.New("password reset token has expired")
	ErrPasswordResetInvalid    = errors.New("invalid password reset token")
	ErrAccountLocked           = errors.New("account is temporarily locked due to too many failed attempts")
	ErrEmailNotSent            = errors.New("failed to send email")
	ErrSessionNotFound         = errors.New("session not found")
)

// AuthService defines the authentication service interface.
type AuthService interface {
	// Registration
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error)
	
	// Login
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.AuthResponse, error)
	
	// Refresh token
	RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthResponse, error)
	
	// Logout
	Logout(ctx context.Context, refreshToken string) error
	
	// Email verification
	SendVerificationEmail(ctx context.Context, userID string) error
	VerifyEmail(ctx context.Context, token string) error
	
	// Password reset
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	
	// Change password (authenticated)
	ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error
	
	// Session management
	GetActiveSessions(ctx context.Context, userID string) ([]*interfaces.Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAllSessions(ctx context.Context, userID string) error
	
	// Account lockout
	IsAccountLocked(ctx context.Context, userID string) (bool, error)
	ResetFailedAttempts(ctx context.Context, userID string) error
}

// authService implements AuthService.
type authService struct {
	userRepo        interfaces.UserRepository
	sessionRepo     interfaces.SessionRepository
	notificationRepo interfaces.NotificationRepository
	emailAdapter    adapter.EmailAdapter
	redisAdapter    adapter.RedisAdapter
	jwtSecret       string
	jwtExpiry       time.Duration
	refreshExpiry   time.Duration
	maxFailedAttempts int
	lockoutDuration   time.Duration
	log             *logrus.Entry
}

// NewAuthService creates a new auth service.
func NewAuthService(
	userRepo interfaces.UserRepository,
	sessionRepo interfaces.SessionRepository,
	notificationRepo interfaces.NotificationRepository,
	emailAdapter adapter.EmailAdapter,
	redisAdapter adapter.RedisAdapter,
	jwtSecret string,
	jwtExpiry time.Duration,
	refreshExpiry time.Duration,
	maxFailedAttempts int,
	lockoutDuration time.Duration,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		sessionRepo:      sessionRepo,
		notificationRepo: notificationRepo,
		emailAdapter:     emailAdapter,
		redisAdapter:     redisAdapter,
		jwtSecret:        jwtSecret,
		jwtExpiry:        jwtExpiry,
		refreshExpiry:    refreshExpiry,
		maxFailedAttempts: maxFailedAttempts,
		lockoutDuration:   lockoutDuration,
		log:              logger.WithField("service", "auth"),
	}
}

// Register registers a new user.
func (s *authService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.AuthResponse, error) {
	// Validate email
	email, err := valueobjects.NewEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}
	// Validate username
	username, err := valueobjects.NewUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("invalid username: %w", err)
	}
	// Check if email already exists
	exists, err := s.userRepo.ExistsByEmail(ctx, email.String())
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, interfaces.ErrDuplicateEmail
	}
	// Check if username already exists
	exists, err = s.userRepo.ExistsByUsername(ctx, username.String())
	if err != nil {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if exists {
		return nil, interfaces.ErrDuplicateUsername
	}
	// Create user entity
	user, err := entities.NewUser(username.String(), email.String(), req.Password, req.FullName)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	// If verification is required, set not verified (default is false)
	// Save user
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	// Send verification email (if configured)
	if req.SendVerificationEmail {
		if err := s.SendVerificationEmail(ctx, user.ID); err != nil {
			s.log.WithError(err).WithField("user_id", user.ID).Warn("failed to send verification email")
		}
	}
	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}
	// Store session
	session := &interfaces.Session{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    req.UserAgent,
		IP:           req.IP,
		ExpiresAt:    time.Now().Add(s.refreshExpiry),
		CreatedAt:    time.Now(),
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	// Log activity
	s.log.WithFields(logrus.Fields{
		"user_id":   user.ID,
		"username":  user.Username,
		"email":     user.Email,
	}).Info("user registered successfully")
	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.jwtExpiry.Seconds()),
		User:         user.ToPublic(),
	}, nil
}

// Login authenticates a user.
func (s *authService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.AuthResponse, error) {
	// Find user by email or username
	user, err := s.userRepo.GetByUsernameOrEmail(ctx, req.Identifier)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	// Check if user is locked out
	locked, err := s.IsAccountLocked(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check lockout: %w", err)
	}
	if locked {
		return nil, ErrAccountLocked
	}
	// Verify password
	if !user.CheckPassword(req.Password) {
		// Increment failed attempts
		_ = s.recordFailedAttempt(ctx, user.ID)
		return nil, ErrInvalidCredentials
	}
	// Check account status
	if user.IsSuspended {
		return nil, ErrUserSuspended
	}
	if !user.IsActive {
		return nil, ErrUserInactive
	}
	// Reset failed attempts on success
	_ = s.ResetFailedAttempts(ctx, user.ID)
	// Update last active
	if err := s.userRepo.UpdateLastActive(ctx, user.ID); err != nil {
		s.log.WithError(err).Warn("failed to update last active")
	}
	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}
	// Store session
	session := &interfaces.Session{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    req.UserAgent,
		IP:           req.IP,
		ExpiresAt:    time.Now().Add(s.refreshExpiry),
		CreatedAt:    time.Now(),
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	// Log
	s.log.WithFields(logrus.Fields{
		"user_id":  user.ID,
		"username": user.Username,
		"ip":       req.IP,
	}).Info("user logged in")
	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.jwtExpiry.Seconds()),
		User:         user.ToPublic(),
	}, nil
}

// RefreshToken refreshes the access token using a refresh token.
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthResponse, error) {
	// Validate refresh token
	session, err := s.sessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, interfaces.ErrSessionNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	// Check expiry
	if session.ExpiresAt.Before(time.Now()) {
		return nil, ErrTokenExpired
	}
	// Get user
	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Check status
	if user.IsSuspended {
		return nil, ErrUserSuspended
	}
	if !user.IsActive {
		return nil, ErrUserInactive
	}
	// Generate new tokens (rotate)
	newAccess, newRefresh, err := s.generateTokens(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}
	// Update session with new refresh token
	session.RefreshToken = newRefresh
	session.ExpiresAt = time.Now().Add(s.refreshExpiry)
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}
	// Delete old refresh token from Redis if stored (optional)
	// ...

	s.log.WithField("user_id", user.ID).Debug("refresh token rotated")
	return &dto.AuthResponse{
		AccessToken:  newAccess,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.jwtExpiry.Seconds()),
		User:         user.ToPublic(),
	}, nil
}

// Logout invalidates a refresh token.
func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.sessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, interfaces.ErrSessionNotFound) {
			// Already invalid
			return nil
		}
		return fmt.Errorf("failed to get session: %w", err)
	}
	if err := s.sessionRepo.Delete(ctx, session.ID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	s.log.WithField("user_id", session.UserID).Info("user logged out")
	return nil
}

// SendVerificationEmail sends an email verification link.
func (s *authService) SendVerificationEmail(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsVerified {
		return ErrEmailAlreadyVerified
	}
	// Generate verification token (JWT or random)
	token, err := s.generateVerificationToken(user.ID, user.Email)
	if err != nil {
		return fmt.Errorf("failed to generate verification token: %w", err)
	}
	// Send email (async)
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", s.frontendBaseURL, token)
	msg := &adapter.EmailMessage{
		To:      []string{user.Email},
		Subject: "Verify your email address",
		HTMLBody: fmt.Sprintf(`
			<h1>Welcome %s!</h1>
			<p>Please verify your email by clicking the link below:</p>
			<a href="%s">%s</a>
			<p>This link expires in 24 hours.</p>
		`, user.FullName, verificationLink, verificationLink),
		TextBody: fmt.Sprintf("Verify your email: %s", verificationLink),
	}
	if err := s.emailAdapter.Queue(ctx, msg); err != nil {
		return fmt.Errorf("failed to queue verification email: %w", err)
	}
	return nil
}

// VerifyEmail verifies the email using a token.
func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	// Parse token and extract user ID
	userID, email, err := s.parseVerificationToken(token)
	if err != nil {
		return ErrInvalidToken
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsVerified {
		return ErrEmailAlreadyVerified
	}
	// Ensure email matches (optional)
	if user.Email != email {
		return ErrInvalidToken
	}
	// Update user
	user.IsVerified = true
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user verification: %w", err)
	}
	s.log.WithField("user_id", userID).Info("email verified")
	return nil
}

// RequestPasswordReset sends a password reset email.
func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	emailVO, err := valueobjects.NewEmail(email)
	if err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}
	user, err := s.userRepo.GetByEmail(ctx, emailVO.String())
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			// Don't reveal if user exists for security
			return nil
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Generate reset token
	token, err := s.generateResetToken(user.ID, user.Email)
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}
	// Store token in Redis with expiry (e.g., 1 hour)
	key := "password_reset:" + token
	if err := s.redisAdapter.Set(ctx, key, user.ID, 1*time.Hour); err != nil {
		return fmt.Errorf("failed to store reset token: %w", err)
	}
	// Send email
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.frontendBaseURL, token)
	msg := &adapter.EmailMessage{
		To:      []string{user.Email},
		Subject: "Reset your password",
		HTMLBody: fmt.Sprintf(`
			<h1>Password Reset</h1>
			<p>You requested to reset your password. Click the link below:</p>
			<a href="%s">%s</a>
			<p>This link expires in 1 hour.</p>
			<p>If you didn't request this, ignore this email.</p>
		`, resetLink, resetLink),
		TextBody: fmt.Sprintf("Reset your password: %s", resetLink),
	}
	if err := s.emailAdapter.Queue(ctx, msg); err != nil {
		return fmt.Errorf("failed to queue reset email: %w", err)
	}
	return nil
}

// ResetPassword resets the password using a token.
func (s *authService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Retrieve user ID from Redis
	key := "password_reset:" + token
	userID, err := s.redisAdapter.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrPasswordResetExpired
		}
		return fmt.Errorf("failed to get reset token: %w", err)
	}
	// Delete token immediately (one-time use)
	_ = s.redisAdapter.Delete(ctx, key)
	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Change password
	if err := user.ChangePassword(newPassword); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	s.log.WithField("user_id", user.ID).Info("password reset successfully")
	return nil
}

// ChangePassword changes a user's password (authenticated).
func (s *authService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.CheckPassword(oldPassword) {
		return ErrInvalidCredentials
	}
	if err := user.ChangePassword(newPassword); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	s.log.WithField("user_id", userID).Info("password changed")
	return nil
}

// GetActiveSessions returns all active sessions for a user.
func (s *authService) GetActiveSessions(ctx context.Context, userID string) ([]*interfaces.Session, error) {
	return s.sessionRepo.GetByUserID(ctx, userID)
}

// RevokeSession revokes a specific session.
func (s *authService) RevokeSession(ctx context.Context, sessionID string) error {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.UserID != "" { // ensure user owns session? we'll validate in handler
	}
	return s.sessionRepo.Delete(ctx, sessionID)
}

// RevokeAllSessions revokes all sessions for a user.
func (s *authService) RevokeAllSessions(ctx context.Context, userID string) error {
	sessions, err := s.sessionRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if err := s.sessionRepo.Delete(ctx, sess.ID); err != nil {
			s.log.WithError(err).WithField("session_id", sess.ID).Warn("failed to delete session")
		}
	}
	return nil
}

// IsAccountLocked checks if a user account is temporarily locked.
func (s *authService) IsAccountLocked(ctx context.Context, userID string) (bool, error) {
	key := "failed_attempts:" + userID
	countStr, err := s.redisAdapter.Get(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get failed attempts: %w", err)
	}
	count := 0
	fmt.Sscanf(countStr, "%d", &count)
	return count >= s.maxFailedAttempts, nil
}

// ResetFailedAttempts resets the failed login counter.
func (s *authService) ResetFailedAttempts(ctx context.Context, userID string) error {
	key := "failed_attempts:" + userID
	return s.redisAdapter.Delete(ctx, key)
}

// recordFailedAttempt increments the failed attempt counter.
func (s *authService) recordFailedAttempt(ctx context.Context, userID string) error {
	key := "failed_attempts:" + userID
	// Increment and set TTL
	_, err := s.redisAdapter.Incr(ctx, key)
	if err != nil {
		return err
	}
	// Set expiry if not set
	ttl, err := s.redisAdapter.TTL(ctx, key)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		return s.redisAdapter.Expire(ctx, key, s.lockoutDuration)
	}
	return nil
}

// --- Token generation helpers ---

// generateTokens creates access and refresh tokens.
func (s *authService) generateTokens(userID, username string, role entities.UserRole) (string, string, error) {
	// Access token
	accessClaims := jwt.MapClaims{
		"sub":  userID,
		"user": username,
		"role": string(role),
		"exp":  time.Now().Add(s.jwtExpiry).Unix(),
		"iat":  time.Now().Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessString, err := accessToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", err
	}

	// Refresh token (opaque, random bytes)
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return "", "", err
	}
	refreshString := base64.URLEncoding.EncodeToString(refreshBytes)
	return accessString, refreshString, nil
}

// generateVerificationToken creates a JWT for email verification.
func (s *authService) generateVerificationToken(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"type":  "verification",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// parseVerificationToken parses and validates the verification token.
func (s *authService) parseVerificationToken(tokenString string) (userID, email string, err error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return "", "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", ErrInvalidToken
	}
	if claims["type"] != "verification" {
		return "", "", ErrInvalidToken
	}
	userID, ok = claims["sub"].(string)
	if !ok {
		return "", "", ErrInvalidToken
	}
	email, ok = claims["email"].(string)
	if !ok {
		return "", "", ErrInvalidToken
	}
	return userID, email, nil
}

// generateResetToken creates a random token for password reset.
func (s *authService) generateResetToken(userID, email string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// set frontendBaseURL (assigned from config)
var frontendBaseURL string

// SetFrontendBaseURL sets the base URL for frontend links.
func SetFrontendBaseURL(url string) {
	frontendBaseURL = url
}

// --- DTOs ---

// (These are already defined in dto package, but we include references)