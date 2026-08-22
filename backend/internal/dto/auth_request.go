// backend/internal/dto/auth_request.go
package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Validation constants.
const (
	MinUsernameLength = 3
	MaxUsernameLength = 20
	MinPasswordLength = 8
	MaxPasswordLength = 72 // bcrypt max
	MaxFullNameLength = 100
	MaxBioLength      = 160
	MaxEmailLength    = 254
)

// Common validation errors.
var (
	ErrUsernameRequired     = errors.New("username is required")
	ErrUsernameTooShort     = fmt.Errorf("username must be at least %d characters", MinUsernameLength)
	ErrUsernameTooLong      = fmt.Errorf("username must be at most %d characters", MaxUsernameLength)
	ErrUsernameInvalid      = errors.New("username can only contain letters, numbers, underscore, dot, and hyphen")
	ErrUsernameReserved     = errors.New("username is reserved")
	ErrEmailRequired        = errors.New("email is required")
	ErrEmailInvalid         = errors.New("invalid email format")
	ErrPasswordRequired     = errors.New("password is required")
	ErrPasswordTooShort     = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	ErrPasswordTooLong      = fmt.Errorf("password must be at most %d characters", MaxPasswordLength)
	ErrPasswordWeak         = errors.New("password must contain at least one uppercase, one lowercase, one number, and one special character")
	ErrPasswordMismatch     = errors.New("passwords do not match")
	ErrFullNameRequired     = errors.New("full name is required")
	ErrFullNameTooLong      = fmt.Errorf("full name must be at most %d characters", MaxFullNameLength)
	ErrTokenRequired        = errors.New("refresh token is required")
	ErrInvalidToken         = errors.New("invalid token format")
	ErrOldPasswordRequired  = errors.New("old password is required")
	ErrNewPasswordRequired  = errors.New("new password is required")
	ErrConfirmPasswordRequired = errors.New("confirm password is required")
	ErrBioTooLong           = fmt.Errorf("bio must be at most %d characters", MaxBioLength)
	ErrIdentifierRequired   = errors.New("identifier (email or username) is required")
)

// UsernameRegex is the compiled regex for username validation.
var UsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// EmailRegex is the compiled regex for email validation.
var EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// PasswordRegex validates password strength: at least one uppercase, one lowercase, one number, one special.
var PasswordRegex = regexp.MustCompile(`^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]).+$`)

// RegisterRequest represents the request body for user registration.
type RegisterRequest struct {
	Username            string `json:"username" binding:"required,min=3,max=20,alphanum_underscore_dot_hyphen"`
	Email               string `json:"email" binding:"required,email,max=254"`
	Password            string `json:"password" binding:"required,min=8,max=72"`
	ConfirmPassword     string `json:"confirm_password" binding:"required"`
	FullName            string `json:"full_name" binding:"required,max=100"`
	Bio                 string `json:"bio" binding:"max=160"`
	AvatarURL           string `json:"avatar_url" binding:"omitempty,url"`
	SendVerificationEmail bool `json:"send_verification_email"`
	UserAgent           string `json:"-"`
	IP                  string `json:"-"`
}

// Validate performs comprehensive validation on RegisterRequest.
func (r *RegisterRequest) Validate() error {
	// Validate username
	if strings.TrimSpace(r.Username) == "" {
		return ErrUsernameRequired
	}
	if len(r.Username) < MinUsernameLength {
		return ErrUsernameTooShort
	}
	if len(r.Username) > MaxUsernameLength {
		return ErrUsernameTooLong
	}
	if !UsernameRegex.MatchString(r.Username) {
		return ErrUsernameInvalid
	}
	// Check reserved usernames
	if isReservedUsername(r.Username) {
		return ErrUsernameReserved
	}

	// Validate email
	if strings.TrimSpace(r.Email) == "" {
		return ErrEmailRequired
	}
	if len(r.Email) > MaxEmailLength {
		return fmt.Errorf("email exceeds maximum length of %d characters", MaxEmailLength)
	}
	if !EmailRegex.MatchString(r.Email) {
		return ErrEmailInvalid
	}

	// Validate password
	if r.Password == "" {
		return ErrPasswordRequired
	}
	if len(r.Password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(r.Password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	if !PasswordRegex.MatchString(r.Password) {
		return ErrPasswordWeak
	}
	if r.Password != r.ConfirmPassword {
		return ErrPasswordMismatch
	}

	// Validate full name
	if strings.TrimSpace(r.FullName) == "" {
		return ErrFullNameRequired
	}
	if len(r.FullName) > MaxFullNameLength {
		return ErrFullNameTooLong
	}

	// Validate bio
	if len(r.Bio) > MaxBioLength {
		return ErrBioTooLong
	}

	// Validate avatar URL (optional)
	if r.AvatarURL != "" && !isValidURL(r.AvatarURL) {
		return errors.New("invalid avatar URL format")
	}

	return nil
}

// Sanitize cleans up the request fields.
func (r *RegisterRequest) Sanitize() {
	r.Username = strings.ToLower(strings.TrimSpace(r.Username))
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.FullName = strings.TrimSpace(r.FullName)
	r.Bio = strings.TrimSpace(r.Bio)
	r.Password = strings.TrimSpace(r.Password)
	r.ConfirmPassword = strings.TrimSpace(r.ConfirmPassword)
}

// ToMap converts the request to a map for logging.
func (r *RegisterRequest) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"username": r.Username,
		"email":    r.Email,
		"full_name": r.FullName,
		"bio":      r.Bio,
		"has_avatar": r.AvatarURL != "",
	}
}

// LoginRequest represents the request body for user login.
type LoginRequest struct {
	Identifier   string `json:"identifier" binding:"required"` // email or username
	Password     string `json:"password" binding:"required"`
	RememberMe   bool   `json:"remember_me"`
	UserAgent    string `json:"-"`
	IP           string `json:"-"`
}

// Validate performs validation on LoginRequest.
func (r *LoginRequest) Validate() error {
	if strings.TrimSpace(r.Identifier) == "" {
		return ErrIdentifierRequired
	}
	if r.Password == "" {
		return ErrPasswordRequired
	}
	return nil
}

// IsEmailLogin returns true if identifier looks like an email.
func (r *LoginRequest) IsEmailLogin() bool {
	return strings.Contains(r.Identifier, "@")
}

// Sanitize cleans up the request.
func (r *LoginRequest) Sanitize() {
	r.Identifier = strings.TrimSpace(r.Identifier)
	r.Password = strings.TrimSpace(r.Password)
}

// RefreshRequest represents the request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Validate performs validation on RefreshRequest.
func (r *RefreshRequest) Validate() error {
	if strings.TrimSpace(r.RefreshToken) == "" {
		return ErrTokenRequired
	}
	if len(r.RefreshToken) < 32 {
		return ErrInvalidToken
	}
	return nil
}

// Sanitize cleans up the request.
func (r *RefreshRequest) Sanitize() {
	r.RefreshToken = strings.TrimSpace(r.RefreshToken)
}

// LogoutRequest represents the request body for logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Validate performs validation on LogoutRequest.
func (r *LogoutRequest) Validate() error {
	if strings.TrimSpace(r.RefreshToken) == "" {
		return ErrTokenRequired
	}
	return nil
}

// Sanitize cleans up the request.
func (r *LogoutRequest) Sanitize() {
	r.RefreshToken = strings.TrimSpace(r.RefreshToken)
}

// VerifyEmailRequest represents the request body for email verification.
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// Validate performs validation on VerifyEmailRequest.
func (r *VerifyEmailRequest) Validate() error {
	if strings.TrimSpace(r.Token) == "" {
		return errors.New("verification token is required")
	}
	return nil
}

// Sanitize cleans up the request.
func (r *VerifyEmailRequest) Sanitize() {
	r.Token = strings.TrimSpace(r.Token)
}

// ForgotPasswordRequest represents the request body for forgot password.
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// Validate performs validation on ForgotPasswordRequest.
func (r *ForgotPasswordRequest) Validate() error {
	if strings.TrimSpace(r.Email) == "" {
		return ErrEmailRequired
	}
	if !EmailRegex.MatchString(r.Email) {
		return ErrEmailInvalid
	}
	return nil
}

// Sanitize cleans up the request.
func (r *ForgotPasswordRequest) Sanitize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

// ResetPasswordRequest represents the request body for password reset.
type ResetPasswordRequest struct {
	Token           string `json:"token" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=72"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// Validate performs validation on ResetPasswordRequest.
func (r *ResetPasswordRequest) Validate() error {
	if strings.TrimSpace(r.Token) == "" {
		return errors.New("reset token is required")
	}
	if r.NewPassword == "" {
		return ErrPasswordRequired
	}
	if len(r.NewPassword) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(r.NewPassword) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	if !PasswordRegex.MatchString(r.NewPassword) {
		return ErrPasswordWeak
	}
	if r.NewPassword != r.ConfirmPassword {
		return ErrPasswordMismatch
	}
	return nil
}

// Sanitize cleans up the request.
func (r *ResetPasswordRequest) Sanitize() {
	r.Token = strings.TrimSpace(r.Token)
	r.NewPassword = strings.TrimSpace(r.NewPassword)
	r.ConfirmPassword = strings.TrimSpace(r.ConfirmPassword)
}

// ChangePasswordRequest represents the request body for changing password (authenticated).
type ChangePasswordRequest struct {
	OldPassword     string `json:"old_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=72"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// Validate performs validation on ChangePasswordRequest.
func (r *ChangePasswordRequest) Validate() error {
	if r.OldPassword == "" {
		return ErrOldPasswordRequired
	}
	if r.NewPassword == "" {
		return ErrNewPasswordRequired
	}
	if len(r.NewPassword) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(r.NewPassword) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	if !PasswordRegex.MatchString(r.NewPassword) {
		return ErrPasswordWeak
	}
	if r.NewPassword != r.ConfirmPassword {
		return ErrPasswordMismatch
	}
	if r.OldPassword == r.NewPassword {
		return errors.New("new password must be different from old password")
	}
	return nil
}

// Sanitize cleans up the request.
func (r *ChangePasswordRequest) Sanitize() {
	r.OldPassword = strings.TrimSpace(r.OldPassword)
	r.NewPassword = strings.TrimSpace(r.NewPassword)
	r.ConfirmPassword = strings.TrimSpace(r.ConfirmPassword)
}

// --- Validation Utilities ---

// isReservedUsername checks for reserved usernames.
func isReservedUsername(username string) bool {
	reserved := map[string]bool{
		"admin": true, "administrator": true, "root": true,
		"sysadmin": true, "system": true, "support": true,
		"help": true, "info": true, "noreply": true,
		"postmaster": true, "webmaster": true, "hostmaster": true,
		"abuse": true, "security": true, "privacy": true,
		"moderator": true, "mod": true, "owner": true,
		"manager": true, "user": true, "users": true,
		"guest": true, "test": true, "demo": true,
		"example": true, "sample": true, "anonymous": true,
		"default": true, "null": true, "undefined": true,
		"api": true, "app": true, "dev": true,
	}
	return reserved[strings.ToLower(username)]
}

// isValidURL checks if a URL is valid (basic).
func isValidURL(url string) bool {
	urlRegex := regexp.MustCompile(`^(https?://)[^\s/$.?#].[^\s]*$`)
	return urlRegex.MatchString(url)
}

// --- Helper builder methods for tests ---

// NewRegisterRequest creates a new register request with defaults.
func NewRegisterRequest() *RegisterRequest {
	return &RegisterRequest{
		Username:            "testuser",
		Email:               "test@example.com",
		Password:            "Test@1234",
		ConfirmPassword:     "Test@1234",
		FullName:            "Test User",
		Bio:                 "Test bio",
		SendVerificationEmail: true,
	}
}

// WithUsername sets the username.
func (r *RegisterRequest) WithUsername(username string) *RegisterRequest {
	r.Username = username
	return r
}

// WithEmail sets the email.
func (r *RegisterRequest) WithEmail(email string) *RegisterRequest {
	r.Email = email
	return r
}

// WithPassword sets the password.
func (r *RegisterRequest) WithPassword(password string) *RegisterRequest {
	r.Password = password
	r.ConfirmPassword = password
	return r
}

// NewLoginRequest creates a new login request with defaults.
func NewLoginRequest() *LoginRequest {
	return &LoginRequest{
		Identifier: "test@example.com",
		Password:   "Test@1234",
	}
}

// WithIdentifier sets the identifier.
func (r *LoginRequest) WithIdentifier(identifier string) *LoginRequest {
	r.Identifier = identifier
	return r
}

// WithPassword sets the password.
func (r *LoginRequest) WithPassword(password string) *LoginRequest {
	r.Password = password
	return r
}

// --- Error helpers ---

// ValidationError represents a field validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (ve ValidationErrors) Error() string {
	messages := make([]string, len(ve))
	for i, err := range ve {
		messages[i] = err.Error()
	}
	return strings.Join(messages, "; ")
}

// ToMap converts validation errors to a map.
func (ve ValidationErrors) ToMap() map[string]string {
	result := make(map[string]string)
	for _, err := range ve {
		result[err.Field] = err.Message
	}
	return result
}

// --- Response DTOs ---

// AuthResponse represents the authentication response.
type AuthResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int64       `json:"expires_in"`
	User         interface{} `json:"user"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// SuccessResponse represents a success response.
type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// NewSuccessResponse creates a new success response.
func NewSuccessResponse(message string, data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// --- Session DTOs ---

// SessionResponse represents a session in responses.
type SessionResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserAgent string    `json:"user_agent"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IsCurrent bool      `json:"is_current"`
}

// --- DTO conversion functions ---

// ToSessionResponse converts a session entity to a response DTO.
func ToSessionResponse(session *Session, isCurrent bool) *SessionResponse {
	return &SessionResponse{
		ID:        session.ID,
		UserID:    session.UserID,
		UserAgent: session.UserAgent,
		IP:        session.IP,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
		IsCurrent: isCurrent,
	}
}

// --- JSON helpers for custom marshaling ---

// MarshalJSON implements custom JSON marshaling for RegisterRequest.
func (r RegisterRequest) MarshalJSON() ([]byte, error) {
	// Omit sensitive fields from logs
	type Alias RegisterRequest
	return json.Marshal(&struct {
		*Alias
		Password        string `json:"password,omitempty"`
		ConfirmPassword string `json:"confirm_password,omitempty"`
	}{
		Alias:           (*Alias)(&r),
		Password:        "[REDACTED]",
		ConfirmPassword: "[REDACTED]",
	})
}