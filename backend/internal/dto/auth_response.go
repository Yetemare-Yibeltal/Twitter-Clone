// backend/internal/dto/auth_response.go
package dto

import (
	"encoding/json"
	"time"
)

// ======================================================================
// Auth Response Types
// ======================================================================

// AuthResponse represents the main authentication response.
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int64       `json:"expires_in"`
	User         interface{} `json:"user"`
}

// LoginResponse represents login response.
type LoginResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int64       `json:"expires_in"`
	User         UserResponse `json:"user"`
}

// RegisterResponse represents registration response.
type RegisterResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int64       `json:"expires_in"`
	User         UserResponse `json:"user"`
	EmailSent    bool        `json:"email_sent"`
}

// RefreshTokenResponse represents refresh token response.
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// VerificationResponse represents email verification response.
type VerificationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	UserID  string `json:"user_id,omitempty"`
}

// ResetPasswordResponse represents password reset response.
type ResetPasswordResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ForgotPasswordResponse represents forgot password response.
type ForgotPasswordResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Email   string `json:"email,omitempty"`
}

// LogoutResponse represents logout response.
type LogoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ======================================================================
// User Response DTOs
// ======================================================================

// UserResponse represents user data in authentication responses.
type UserResponse struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	FullName    string    `json:"full_name"`
	Bio         string    `json:"bio,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	BannerURL   string    `json:"banner_url,omitempty"`
	IsVerified  bool      `json:"is_verified"`
	IsPrivate   bool      `json:"is_private"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	TweetCount  int64     `json:"tweet_count"`
	FollowerCount int64   `json:"follower_count"`
	FollowingCount int64  `json:"following_count"`
	JoinedAt    time.Time `json:"joined_at"`
	LastActive  *time.Time `json:"last_active_at,omitempty"`
}

// MinimalUserResponse represents minimal user data.
type MinimalUserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url"`
}

// ======================================================================
// Session Response DTOs
// ======================================================================

// SessionResponse represents session data in responses.
type SessionResponse struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	UserAgent   string    `json:"user_agent,omitempty"`
	IP          string    `json:"ip,omitempty"`
	DeviceName  string    `json:"device_name,omitempty"`
	DeviceID    string    `json:"device_id,omitempty"`
	OS          string    `json:"os,omitempty"`
	Browser     string    `json:"browser,omitempty"`
	Location    string    `json:"location,omitempty"`
	Status      string    `json:"status"`
	Type        string    `json:"type"`
	IsCurrent   bool      `json:"is_current"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastActive  *time.Time `json:"last_active_at,omitempty"`
}

// SessionListResponse represents list of sessions.
type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	Total    int64             `json:"total"`
}

// ======================================================================
// Error Response DTOs
// ======================================================================

// ErrorResponse represents error responses.
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message,omitempty"`
	Code    int                    `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ValidationErrorResponse represents validation error responses.
type ValidationErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors"`
}

// ======================================================================
// Success Response DTOs
// ======================================================================

// SuccessResponse represents generic success responses.
type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// HealthResponse represents health check response.
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// ======================================================================
= Builder Methods
// ======================================================================

// NewAuthResponse creates a new auth response.
func NewAuthResponse(accessToken, refreshToken string, expiresIn int64, user interface{}) *AuthResponse {
	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		User:         user,
	}
}

// NewLoginResponse creates a new login response.
func NewLoginResponse(accessToken, refreshToken string, expiresIn int64, user UserResponse) *LoginResponse {
	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		User:         user,
	}
}

// NewRegisterResponse creates a new register response.
func NewRegisterResponse(accessToken, refreshToken string, expiresIn int64, user UserResponse, emailSent bool) *RegisterResponse {
	return &RegisterResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		User:         user,
		EmailSent:    emailSent,
	}
}

// NewRefreshTokenResponse creates a new refresh token response.
func NewRefreshTokenResponse(accessToken, refreshToken string, expiresIn int64) *RefreshTokenResponse {
	return &RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	}
}

// NewVerificationResponse creates a new verification response.
func NewVerificationResponse(success bool, message, userID string) *VerificationResponse {
	return &VerificationResponse{
		Success: success,
		Message: message,
		UserID:  userID,
	}
}

// NewResetPasswordResponse creates a new reset password response.
func NewResetPasswordResponse(success bool, message string) *ResetPasswordResponse {
	return &ResetPasswordResponse{
		Success: success,
		Message: message,
	}
}

// NewForgotPasswordResponse creates a new forgot password response.
func NewForgotPasswordResponse(success bool, message, email string) *ForgotPasswordResponse {
	return &ForgotPasswordResponse{
		Success: success,
		Message: message,
		Email:   email,
	}
}

// NewLogoutResponse creates a new logout response.
func NewLogoutResponse(success bool, message string) *LogoutResponse {
	return &LogoutResponse{
		Success: success,
		Message: message,
	}
}

// NewErrorResponse creates a new error response.
func NewErrorResponse(err string, message string, code int) *ErrorResponse {
	return &ErrorResponse{
		Error:   err,
		Message: message,
		Code:    code,
		Details: make(map[string]interface{}),
	}
}

// NewValidationErrorResponse creates a new validation error response.
func NewValidationErrorResponse(message string, errors map[string]string) *ValidationErrorResponse {
	return &ValidationErrorResponse{
		Error:   "Validation Error",
		Message: message,
		Errors:  errors,
	}
}

// NewSuccessResponse creates a new success response.
func NewSuccessResponse(message string, data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// NewHealthResponse creates a new health response.
func NewHealthResponse(status, version string) *HealthResponse {
	return &HealthResponse{
		Status:    status,
		Version:   version,
		Timestamp: time.Now().UTC(),
	}
}

// ======================================================================
= Builder Methods for UserResponse
// ======================================================================

// NewUserResponse creates a new user response.
func NewUserResponse(id, username, email, fullName, role, status string, isVerified, isPrivate bool) *UserResponse {
	return &UserResponse{
		ID:            id,
		Username:      username,
		Email:         email,
		FullName:      fullName,
		IsVerified:    isVerified,
		IsPrivate:     isPrivate,
		Role:          role,
		Status:        status,
		JoinedAt:      time.Now().UTC(),
		TweetCount:    0,
		FollowerCount: 0,
		FollowingCount: 0,
	}
}

// WithBio sets the bio.
func (u *UserResponse) WithBio(bio string) *UserResponse {
	u.Bio = bio
	return u
}

// WithAvatarURL sets the avatar URL.
func (u *UserResponse) WithAvatarURL(url string) *UserResponse {
	u.AvatarURL = url
	return u
}

// WithBannerURL sets the banner URL.
func (u *UserResponse) WithBannerURL(url string) *UserResponse {
	u.BannerURL = url
	return u
}

// WithTweetCount sets the tweet count.
func (u *UserResponse) WithTweetCount(count int64) *UserResponse {
	u.TweetCount = count
	return u
}

// WithFollowerCount sets the follower count.
func (u *UserResponse) WithFollowerCount(count int64) *UserResponse {
	u.FollowerCount = count
	return u
}

// WithFollowingCount sets the following count.
func (u *UserResponse) WithFollowingCount(count int64) *UserResponse {
	u.FollowingCount = count
	return u
}

// WithJoinedAt sets the joined at time.
func (u *UserResponse) WithJoinedAt(t time.Time) *UserResponse {
	u.JoinedAt = t
	return u
}

// WithLastActive sets the last active time.
func (u *UserResponse) WithLastActive(t time.Time) *UserResponse {
	u.LastActive = &t
	return u
}

// ======================================================================
= Builder Methods for SessionResponse
// ======================================================================

// NewSessionResponse creates a new session response.
func NewSessionResponse(id, userID, status, sessionType string, expiresAt time.Time) *SessionResponse {
	return &SessionResponse{
		ID:        id,
		UserID:    userID,
		Status:    status,
		Type:      sessionType,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}
}

// WithUserAgent sets the user agent.
func (s *SessionResponse) WithUserAgent(userAgent string) *SessionResponse {
	s.UserAgent = userAgent
	return s
}

// WithIP sets the IP address.
func (s *SessionResponse) WithIP(ip string) *SessionResponse {
	s.IP = ip
	return s
}

// WithDeviceName sets the device name.
func (s *SessionResponse) WithDeviceName(deviceName string) *SessionResponse {
	s.DeviceName = deviceName
	return s
}

// WithDeviceID sets the device ID.
func (s *SessionResponse) WithDeviceID(deviceID string) *SessionResponse {
	s.DeviceID = deviceID
	return s
}

// WithOS sets the OS.
func (s *SessionResponse) WithOS(os string) *SessionResponse {
	s.OS = os
	return s
}

// WithBrowser sets the browser.
func (s *SessionResponse) WithBrowser(browser string) *SessionResponse {
	s.Browser = browser
	return s
}

// WithLocation sets the location.
func (s *SessionResponse) WithLocation(location string) *SessionResponse {
	s.Location = location
	return s
}

// WithIsCurrent sets the is current flag.
func (s *SessionResponse) WithIsCurrent(isCurrent bool) *SessionResponse {
	s.IsCurrent = isCurrent
	return s
}

// WithLastActive sets the last active time.
func (s *SessionResponse) WithLastActive(t time.Time) *SessionResponse {
	s.LastActive = &t
	return s
}

// ======================================================================
= Conversion Helpers
// ======================================================================

// ToUserResponse converts user data to UserResponse.
// This is a placeholder; actual implementation would depend on entity structure.
func ToUserResponse(id, username, email, fullName, role, status string, isVerified, isPrivate bool) UserResponse {
	return UserResponse{
		ID:         id,
		Username:   username,
		Email:      email,
		FullName:   fullName,
		IsVerified: isVerified,
		IsPrivate:  isPrivate,
		Role:       role,
		Status:     status,
		JoinedAt:   time.Now().UTC(),
	}
}

// ToMinimalUserResponse creates a minimal user response.
func ToMinimalUserResponse(id, username, fullName, avatarURL string) MinimalUserResponse {
	return MinimalUserResponse{
		ID:        id,
		Username:  username,
		FullName:  fullName,
		AvatarURL: avatarURL,
	}
}

// ======================================================================
= JSON Serialization Helpers
// ======================================================================

// MarshalJSON implements custom JSON marshaling for AuthResponse.
func (r *AuthResponse) MarshalJSON() ([]byte, error) {
	type Alias AuthResponse
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

// MarshalJSON implements custom JSON marshaling for LoginResponse.
func (r *LoginResponse) MarshalJSON() ([]byte, error) {
	type Alias LoginResponse
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

// MarshalJSON implements custom JSON marshaling for ErrorResponse.
func (r *ErrorResponse) MarshalJSON() ([]byte, error) {
	type Alias ErrorResponse
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

// ======================================================================
= Response Factory
// ======================================================================

// ResponseFactory provides methods to create standard responses.
type ResponseFactory struct{}

// NewResponseFactory creates a new response factory.
func NewResponseFactory() *ResponseFactory {
	return &ResponseFactory{}
}

// Auth creates an auth response.
func (f *ResponseFactory) Auth(accessToken, refreshToken string, expiresIn int64, user interface{}) *AuthResponse {
	return NewAuthResponse(accessToken, refreshToken, expiresIn, user)
}

// Login creates a login response.
func (f *ResponseFactory) Login(accessToken, refreshToken string, expiresIn int64, user UserResponse) *LoginResponse {
	return NewLoginResponse(accessToken, refreshToken, expiresIn, user)
}

// Register creates a register response.
func (f *ResponseFactory) Register(accessToken, refreshToken string, expiresIn int64, user UserResponse, emailSent bool) *RegisterResponse {
	return NewRegisterResponse(accessToken, refreshToken, expiresIn, user, emailSent)
}

// Refresh creates a refresh token response.
func (f *ResponseFactory) Refresh(accessToken, refreshToken string, expiresIn int64) *RefreshTokenResponse {
	return NewRefreshTokenResponse(accessToken, refreshToken, expiresIn)
}

// Success creates a success response.
func (f *ResponseFactory) Success(message string, data interface{}) *SuccessResponse {
	return NewSuccessResponse(message, data)
}

// Error creates an error response.
func (f *ResponseFactory) Error(err, message string, code int) *ErrorResponse {
	return NewErrorResponse(err, message, code)
}

// ValidationError creates a validation error response.
func (f *ResponseFactory) ValidationError(message string, errors map[string]string) *ValidationErrorResponse {
	return NewValidationErrorResponse(message, errors)
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestAuthResponse creates a test auth response.
func NewTestAuthResponse() *AuthResponse {
	return &AuthResponse{
		AccessToken:  "test_access_token",
		RefreshToken: "test_refresh_token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		User: map[string]interface{}{
			"id":         "user123",
			"username":   "testuser",
			"email":      "test@example.com",
			"full_name":  "Test User",
			"is_verified": true,
		},
	}
}

// NewTestLoginResponse creates a test login response.
func NewTestLoginResponse() *LoginResponse {
	user := UserResponse{
		ID:         "user123",
		Username:   "testuser",
		Email:      "test@example.com",
		FullName:   "Test User",
		IsVerified: true,
		Role:       "user",
		Status:     "active",
		JoinedAt:   time.Now().UTC(),
	}
	return NewLoginResponse("test_access_token", "test_refresh_token", 3600, user)
}

// NewTestErrorResponse creates a test error response.
func NewTestErrorResponse() *ErrorResponse {
	return NewErrorResponse("Unauthorized", "Invalid credentials", 401)
}

// NewTestValidationErrorResponse creates a test validation error response.
func NewTestValidationErrorResponse() *ValidationErrorResponse {
	errors := map[string]string{
		"email":    "Email is required",
		"password": "Password must be at least 8 characters",
	}
	return NewValidationErrorResponse("Validation failed", errors)
}

// ======================================================================
= Constants
// ======================================================================

// Auth constants for response types.
const (
	TokenTypeBearer = "Bearer"
	TokenTypeBasic  = "Basic"
	TokenTypeAPIKey = "APIKey"
)

// Default token expiry values.
const (
	DefaultAccessTokenExpiry  = 3600  // 1 hour
	DefaultRefreshTokenExpiry = 604800 // 7 days
)

// ======================================================================
= Response Status Helpers
// ======================================================================

// IsSuccess checks if an error response is actually success (code 200-299).
func IsSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// IsClientError checks if an error response is a client error (code 400-499).
func IsClientError(statusCode int) bool {
	return statusCode >= 400 && statusCode < 500
}

// IsServerError checks if an error response is a server error (code 500-599).
func IsServerError(statusCode int) bool {
	return statusCode >= 500 && statusCode < 600
}

// ======================================================================
= Response Helpers
// ======================================================================

// OK returns a 200 OK response with data.
func OK(data interface{}) *SuccessResponse {
	return NewSuccessResponse("OK", data)
}

// Created returns a 201 Created response.
func Created(data interface{}) *SuccessResponse {
	return NewSuccessResponse("Created", data)
}

// Accepted returns a 202 Accepted response.
func Accepted(message string, data interface{}) *SuccessResponse {
	if message == "" {
		message = "Accepted"
	}
	return NewSuccessResponse(message, data)
}

// NoContent returns a 204 No Content response.
func NoContent() *SuccessResponse {
	return NewSuccessResponse("No Content", nil)
}

// BadRequest returns a 400 Bad Request response.
func BadRequest(message string) *ErrorResponse {
	if message == "" {
		message = "Bad Request"
	}
	return NewErrorResponse("Bad Request", message, 400)
}

// Unauthorized returns a 401 Unauthorized response.
func Unauthorized(message string) *ErrorResponse {
	if message == "" {
		message = "Unauthorized"
	}
	return NewErrorResponse("Unauthorized", message, 401)
}

// Forbidden returns a 403 Forbidden response.
func Forbidden(message string) *ErrorResponse {
	if message == "" {
		message = "Forbidden"
	}
	return NewErrorResponse("Forbidden", message, 403)
}

// NotFound returns a 404 Not Found response.
func NotFound(message string) *ErrorResponse {
	if message == "" {
		message = "Not Found"
	}
	return NewErrorResponse("Not Found", message, 404)
}

// Conflict returns a 409 Conflict response.
func Conflict(message string) *ErrorResponse {
	if message == "" {
		message = "Conflict"
	}
	return NewErrorResponse("Conflict", message, 409)
}

// TooManyRequests returns a 429 Too Many Requests response.
func TooManyRequests(message string) *ErrorResponse {
	if message == "" {
		message = "Too Many Requests"
	}
	return NewErrorResponse("Too Many Requests", message, 429)
}

// InternalServerError returns a 500 Internal Server Error response.
func InternalServerError(message string) *ErrorResponse {
	if message == "" {
		message = "Internal Server Error"
	}
	return NewErrorResponse("Internal Server Error", message, 500)
}

// NotImplemented returns a 501 Not Implemented response.
func NotImplemented(message string) *ErrorResponse {
	if message == "" {
		message = "Not Implemented"
	}
	return NewErrorResponse("Not Implemented", message, 501)
}

// ServiceUnavailable returns a 503 Service Unavailable response.
func ServiceUnavailable(message string) *ErrorResponse {
	if message == "" {
		message = "Service Unavailable"
	}
	return NewErrorResponse("Service Unavailable", message, 503)
}

// ValidationError returns a validation error response.
func ValidationError(message string, errors map[string]string) *ValidationErrorResponse {
	return NewValidationErrorResponse(message, errors)
}