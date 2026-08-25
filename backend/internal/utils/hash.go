// backend/internal/utils/hash.go
package utils

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// Bcrypt cost factors
	BcryptDefaultCost     = 10
	BcryptMinCost         = 4
	BcryptMaxCost         = 31
	BcryptRecommendedCost = 12

	// Password requirements
	MinPasswordLength     = 8
	MaxPasswordLength     = 72 // bcrypt max
	MinPasswordComplexity = 3  // number of character classes required

	// Token lengths
	DefaultTokenLength = 32
	MaxTokenLength     = 128

	// Hash algorithm identifiers
	HashAlgorithmBcrypt = "bcrypt"
	HashAlgorithmSHA256 = "sha256"
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrPasswordTooShort     = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	ErrPasswordTooLong      = fmt.Errorf("password must be at most %d characters", MaxPasswordLength)
	ErrPasswordWeak         = errors.New("password is too weak: must contain uppercase, lowercase, number, and special character")
	ErrPasswordMissingUpper = errors.New("password must contain at least one uppercase letter")
	ErrPasswordMissingLower = errors.New("password must contain at least one lowercase letter")
	ErrPasswordMissingDigit = errors.New("password must contain at least one digit")
	ErrPasswordMissingSpecial = errors.New("password must contain at least one special character")
	ErrPasswordContainsSpace = errors.New("password cannot contain spaces")
	ErrPasswordCommon       = errors.New("password is too common or easily guessable")
	ErrPasswordReused       = errors.New("password has been used before")
	ErrHashMismatch         = errors.New("hash does not match input")
	ErrHashGenerationFailed = errors.New("failed to generate hash")
	ErrTokenGenerationFailed = errors.New("failed to generate secure token")
	ErrTokenTooShort        = fmt.Errorf("token length must be at least %d", 8)
	ErrTokenTooLong         = fmt.Errorf("token length must be at most %d", MaxTokenLength)
	ErrInvalidCost          = errors.New("invalid bcrypt cost factor")
	ErrEmptyPassword        = errors.New("password cannot be empty")
	ErrEmptyHash            = errors.New("hash cannot be empty")
)

// ======================================================================
= Password Complexity
// ======================================================================

// ComplexityLevel represents password complexity levels.
type ComplexityLevel string

const (
	ComplexityLow    ComplexityLevel = "low"
	ComplexityMedium ComplexityLevel = "medium"
	ComplexityHigh   ComplexityLevel = "high"
	ComplexityStrong ComplexityLevel = "strong"
)

// PasswordRequirements holds password complexity requirements.
type PasswordRequirements struct {
	MinLength         int
	MaxLength         int
	RequireUpper      bool
	RequireLower      bool
	RequireDigit      bool
	RequireSpecial    bool
	MinComplexity     int // number of character classes required
	DisallowCommon    bool
	DisallowReuse     bool
	CommonList        []string
	MaxHistory        int
}

// DefaultPasswordRequirements returns sensible defaults.
func DefaultPasswordRequirements() PasswordRequirements {
	return PasswordRequirements{
		MinLength:      8,
		MaxLength:      72,
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: true,
		MinComplexity:  3,
		DisallowCommon: true,
		DisallowReuse:  false,
		CommonList:     getCommonPasswords(),
		MaxHistory:     5,
	}
}

// ======================================================================
= Password Validator
// ======================================================================

// PasswordValidator validates password strength.
type PasswordValidator struct {
	requirements PasswordRequirements
	history      []string // for password reuse detection
}

// NewPasswordValidator creates a new password validator.
func NewPasswordValidator(req PasswordRequirements) *PasswordValidator {
	return &PasswordValidator{
		requirements: req,
		history:      []string{},
	}
}

// Validate checks if a password meets all requirements.
func (pv *PasswordValidator) Validate(password string) error {
	// Check empty
	if strings.TrimSpace(password) == "" {
		return ErrEmptyPassword
	}
	// Check length
	if len(password) < pv.requirements.MinLength {
		return ErrPasswordTooShort
	}
	if len(password) > pv.requirements.MaxLength {
		return ErrPasswordTooLong
	}
	// Check for spaces
	if strings.Contains(password, " ") {
		return ErrPasswordContainsSpace
	}
	// Check character classes
	classCount := 0
	if pv.requirements.RequireUpper {
		if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
			return ErrPasswordMissingUpper
		}
		classCount++
	}
	if pv.requirements.RequireLower {
		if !regexp.MustCompile(`[a-z]`).MatchString(password) {
			return ErrPasswordMissingLower
		}
		classCount++
	}
	if pv.requirements.RequireDigit {
		if !regexp.MustCompile(`\d`).MatchString(password) {
			return ErrPasswordMissingDigit
		}
		classCount++
	}
	if pv.requirements.RequireSpecial {
		if !regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(password) {
			return ErrPasswordMissingSpecial
		}
		classCount++
	}
	// Check minimum complexity classes
	if pv.requirements.MinComplexity > 0 && classCount < pv.requirements.MinComplexity {
		return ErrPasswordWeak
	}
	// Check common passwords
	if pv.requirements.DisallowCommon && isCommonPassword(password) {
		return ErrPasswordCommon
	}
	// Check reuse
	if pv.requirements.DisallowReuse {
		for _, old := range pv.history {
			if old != "" && VerifyPassword(old, []byte(password)) == nil {
				return ErrPasswordReused
			}
		}
	}
	return nil
}

// AddToHistory adds a password hash to the history.
func (pv *PasswordValidator) AddToHistory(hash string) {
	pv.history = append(pv.history, hash)
	if len(pv.history) > pv.requirements.MaxHistory {
		pv.history = pv.history[1:]
	}
}

// GetComplexityLevel returns the complexity level of a password.
func (pv *PasswordValidator) GetComplexityLevel(password string) ComplexityLevel {
	score := 0
	if len(password) >= 10 {
		score++
	}
	if len(password) >= 14 {
		score++
	}
	if regexp.MustCompile(`[A-Z]`).MatchString(password) {
		score++
	}
	if regexp.MustCompile(`[a-z]`).MatchString(password) {
		score++
	}
	if regexp.MustCompile(`\d`).MatchString(password) {
		score++
	}
	if regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(password) {
		score++
	}
	if regexp.MustCompile(`.{16,}`).MatchString(password) {
		score++
	}
	switch {
	case score >= 7:
		return ComplexityStrong
	case score >= 5:
		return ComplexityHigh
	case score >= 3:
		return ComplexityMedium
	default:
		return ComplexityLow
	}
}

// ======================================================================
= Password Hasher
// ======================================================================

// PasswordHasher handles password hashing operations.
type PasswordHasher struct {
	cost int
}

// NewPasswordHasher creates a new password hasher.
func NewPasswordHasher(cost int) (*PasswordHasher, error) {
	if cost < BcryptMinCost || cost > BcryptMaxCost {
		return nil, ErrInvalidCost
	}
	return &PasswordHasher{cost: cost}, nil
}

// Hash generates a bcrypt hash of the password.
func (ph *PasswordHasher) Hash(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", ErrEmptyPassword
	}
	if len(password) > MaxPasswordLength {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), ph.cost)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrHashGenerationFailed, err)
	}
	return string(hash), nil
}

// Verify checks if a password matches a hash.
func (ph *PasswordHasher) Verify(password, hash string) error {
	if hash == "" {
		return ErrEmptyHash
	}
	if strings.TrimSpace(password) == "" {
		return ErrEmptyPassword
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrHashMismatch
		}
		return fmt.Errorf("verification failed: %w", err)
	}
	return nil
}

// HashAndVerify is a convenience that hashes and verifies the password.
func (ph *PasswordHasher) HashAndVerify(password string) (string, error) {
	hash, err := ph.Hash(password)
	if err != nil {
		return "", err
	}
	if err := ph.Verify(password, hash); err != nil {
		return "", err
	}
	return hash, nil
}

// NeedRehash checks if a hash needs to be rehashed (cost changed).
func (ph *PasswordHasher) NeedRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return true
	}
	return cost < ph.cost
}

// HashWithDefaultCost hashes with the default cost.
func HashWithDefaultCost(password string) (string, error) {
	hasher, _ := NewPasswordHasher(BcryptDefaultCost)
	return hasher.Hash(password)
}

// VerifyPassword verifies a password against a hash using default cost.
func VerifyPassword(password, hash string) error {
	hasher, _ := NewPasswordHasher(BcryptDefaultCost)
	return hasher.Verify(password, hash)
}

// ======================================================================
= Common Password Detection
// ======================================================================

// commonPasswords is a list of common/weak passwords.
var commonPasswords = []string{
	"password", "123456", "password123", "12345678", "qwerty", "abc123",
	"monkey", "letmein", "dragon", "111111", "master", "admin",
	"welcome", "password1", "123456789", "qwertyuiop", "login", "default",
	"passw0rd", "admin123", "12345", "qwerty123", "sunshine", "princess",
	"qwertyui", "iloveyou", "666666", "1234567890", "password1234",
	"1234567", "qwerty123", "hello", "freedom", "whatever", "trustno1",
	"1234", "password!", "password12345", "12345678910", "1234567891",
	"qwerty12345", "mypassword", "password22", "password000", "123123",
	"654321", "qwerty123456", "welcome123", "monkey123", "dragon123",
	"letmein123", "admin12345", "master123", "sunshine1", "princess1",
	"iloveyou1", "freedom1", "whatever1", "trustno11", "hello123",
}

// getCommonPasswords returns the list of common passwords.
func getCommonPasswords() []string {
	return commonPasswords
}

// isCommonPassword checks if a password is in the common list.
func isCommonPassword(password string) bool {
	lower := strings.ToLower(password)
	for _, common := range commonPasswords {
		if lower == common {
			return true
		}
	}
	return false
}

// ======================================================================
= Secure Token Generation
// ======================================================================

// GenerateSecureToken generates a cryptographically secure random token.
func GenerateSecureToken(length int) (string, error) {
	if length < 8 {
		return "", ErrTokenTooShort
	}
	if length > MaxTokenLength {
		return "", ErrTokenTooLong
	}
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateSecureTokenWithLength generates a token with specified byte length (output will be longer due to base64).
func GenerateSecureTokenWithLength(byteLength int) (string, error) {
	if byteLength < 16 {
		byteLength = 32
	}
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateNumericToken generates a numeric token (e.g., for 2FA).
func GenerateNumericToken(length int) (string, error) {
	if length < 4 || length > 12 {
		return "", errors.New("length must be between 4 and 12")
	}
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
	}
	token := ""
	for _, b := range bytes {
		token += fmt.Sprintf("%d", b%10)
	}
	return token, nil
}

// GenerateOTP generates a 6-digit OTP.
func GenerateOTP() (string, error) {
	return GenerateNumericToken(6)
}

// ======================================================================
= Secure Comparison
// ======================================================================

// SecureCompare compares two strings in constant time.
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// SecureCompareBytes compares two byte slices in constant time.
func SecureCompareBytes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// ======================================================================
= Random Generation Utilities
// ======================================================================

// GenerateRandomString generates a random alphanumeric string.
func GenerateRandomString(length int) (string, error) {
	if length < 1 {
		return "", errors.New("length must be at least 1")
	}
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
	}
	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}
	return string(bytes), nil
}

// GenerateRandomHex generates a random hex string.
func GenerateRandomHex(length int) (string, error) {
	if length < 1 {
		return "", errors.New("length must be at least 1")
	}
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGenerationFailed, err)
	}
	hex := fmt.Sprintf("%x", bytes)
	if len(hex) > length {
		hex = hex[:length]
	}
	return hex, nil
}

// GenerateUUID returns a new UUID string.
func GenerateUUID() string {
	return uuid.New().String()
}

// ======================================================================
= Password Utilities
// ======================================================================

// IsValidPassword checks if a password meets basic requirements.
func IsValidPassword(password string) bool {
	if len(password) < MinPasswordLength {
		return false
	}
	if len(password) > MaxPasswordLength {
		return false
	}
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`\d`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(password)
	classCount := 0
	if hasUpper {
		classCount++
	}
	if hasLower {
		classCount++
	}
	if hasDigit {
		classCount++
	}
	if hasSpecial {
		classCount++
	}
	return classCount >= 3
}

// PasswordStrengthScore calculates a password strength score (0-100).
func PasswordStrengthScore(password string) int {
	score := 0
	// Length
	if len(password) >= 8 {
		score += 10
	}
	if len(password) >= 12 {
		score += 10
	}
	if len(password) >= 16 {
		score += 10
	}
	// Character classes
	if regexp.MustCompile(`[A-Z]`).MatchString(password) {
		score += 10
	}
	if regexp.MustCompile(`[a-z]`).MatchString(password) {
		score += 10
	}
	if regexp.MustCompile(`\d`).MatchString(password) {
		score += 10
	}
	if regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(password) {
		score += 10
	}
	// Variety
	if len(regexp.MustCompile(`[A-Z]`).FindAllString(password, -1)) > 1 {
		score += 5
	}
	if len(regexp.MustCompile(`[a-z]`).FindAllString(password, -1)) > 1 {
		score += 5
	}
	if len(regexp.MustCompile(`\d`).FindAllString(password, -1)) > 1 {
		score += 5
	}
	if len(regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).FindAllString(password, -1)) > 1 {
		score += 5
	}
	// Common password penalty
	if isCommonPassword(password) {
		score -= 20
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// ======================================================================
= Hash Algorithm Utilities
// ======================================================================

// HashString hashes a string using SHA256.
func HashString(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// HashBytes hashes bytes using SHA256.
func HashBytes(input []byte) []byte {
	hash := sha256.Sum256(input)
	return hash[:]
}

// ======================================================================
= Password Policy Enforcement
// ======================================================================

// PasswordPolicy represents a password policy.
type PasswordPolicy struct {
	MinLength         int
	MaxLength         int
	RequireUpper      bool
	RequireLower      bool
	RequireDigit      bool
	RequireSpecial    bool
	MinComplexity     int
	DisallowCommon    bool
	DisallowReuse     bool
	MaxHistory        int
	ExpiryDays        int
}

// DefaultPasswordPolicy returns a default policy.
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:      8,
		MaxLength:      72,
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: true,
		MinComplexity:  3,
		DisallowCommon: true,
		DisallowReuse:  false,
		MaxHistory:     5,
		ExpiryDays:     90,
	}
}

// ValidatePasswordPolicy checks if a password meets policy requirements.
func ValidatePasswordPolicy(password string, policy PasswordPolicy) error {
	if len(password) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters", policy.MinLength)
	}
	if len(password) > policy.MaxLength {
		return fmt.Errorf("password must be at most %d characters", policy.MaxLength)
	}
	if policy.RequireUpper && !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return errors.New("password must contain at least one uppercase letter")
	}
	if policy.RequireLower && !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return errors.New("password must contain at least one lowercase letter")
	}
	if policy.RequireDigit && !regexp.MustCompile(`\d`).MatchString(password) {
		return errors.New("password must contain at least one digit")
	}
	if policy.RequireSpecial && !regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(password) {
		return errors.New("password must contain at least one special character")
	}
	if policy.DisallowCommon && isCommonPassword(password) {
		return ErrPasswordCommon
	}
	return nil
}