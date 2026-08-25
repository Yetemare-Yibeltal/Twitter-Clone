// backend/internal/utils/validator.go
package utils

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/idna"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// Email validation
	EmailMaxLength = 254
	EmailMinLength = 3

	// Username validation
	UsernameMinLength = 3
	UsernameMaxLength = 20
	UsernamePattern   = `^[a-zA-Z0-9_.-]+$`

	// Phone validation
	PhoneMinLength = 7
	PhoneMaxLength = 15

	// URL validation
	URLMaxLength = 2048

	// Date formats
	DateFormatISO     = "2006-01-02"
	DateTimeFormatISO = "2006-01-02T15:04:05Z07:00"
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrInvalidEmail         = errors.New("invalid email address")
	ErrEmailTooLong         = fmt.Errorf("email exceeds maximum length of %d characters", EmailMaxLength)
	ErrEmailTooShort        = fmt.Errorf("email must be at least %d characters", EmailMinLength)
	ErrInvalidUsername      = errors.New("invalid username")
	ErrUsernameTooShort     = fmt.Errorf("username must be at least %d characters", UsernameMinLength)
	ErrUsernameTooLong      = fmt.Errorf("username must be at most %d characters", UsernameMaxLength)
	ErrUsernameInvalidChars = errors.New("username contains invalid characters")
	ErrInvalidURL           = errors.New("invalid URL")
	ErrInvalidPhone         = errors.New("invalid phone number")
	ErrInvalidDate          = errors.New("invalid date format")
	ErrInvalidTime          = errors.New("invalid time format")
	ErrInvalidDuration      = errors.New("invalid duration format")
	ErrInvalidUUID          = errors.New("invalid UUID format")
	ErrInvalidHex           = errors.New("invalid hex string")
	ErrInvalidBase64        = errors.New("invalid base64 string")
	ErrInvalidJSON          = errors.New("invalid JSON format")
	ErrInvalidAlphanumeric  = errors.New("must be alphanumeric")
	ErrInvalidAlphabetic    = errors.New("must be alphabetic")
	ErrInvalidNumeric       = errors.New("must be numeric")
	ErrInvalidLength        = errors.New("invalid length")
	ErrInvalidRange         = errors.New("value out of range")
	ErrInvalidEnum          = errors.New("invalid enum value")
)

// ======================================================================
= Validation Functions
// ======================================================================

// ValidateEmail validates an email address.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalidEmail
	}
	if len(email) < EmailMinLength {
		return ErrEmailTooShort
	}
	if len(email) > EmailMaxLength {
		return ErrEmailTooLong
	}
	// Validate using net/mail
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return ErrInvalidEmail
	}
	// Additional validation: domain must have a dot, etc.
	parts := strings.Split(parsed.Address, "@")
	if len(parts) != 2 {
		return ErrInvalidEmail
	}
	local, domain := parts[0], parts[1]
	if local == "" || domain == "" {
		return ErrInvalidEmail
	}
	// Domain must contain at least one dot
	if !strings.Contains(domain, ".") {
		return ErrInvalidEmail
	}
	// Check IDN
	if _, err := idna.ToASCII(domain); err != nil {
		return ErrInvalidEmail
	}
	return nil
}

// ValidateUsername validates a username.
func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return ErrInvalidUsername
	}
	if len(username) < UsernameMinLength {
		return ErrUsernameTooShort
	}
	if len(username) > UsernameMaxLength {
		return ErrUsernameTooLong
	}
	matched, _ := regexp.MatchString(UsernamePattern, username)
	if !matched {
		return ErrUsernameInvalidChars
	}
	// Check reserved usernames
	reserved := map[string]bool{
		"admin": true, "administrator": true, "root": true,
		"system": true, "support": true, "help": true,
		"info": true, "noreply": true, "postmaster": true,
		"webmaster": true, "hostmaster": true, "abuse": true,
		"security": true, "privacy": true, "moderator": true,
		"mod": true, "owner": true, "manager": true,
		"user": true, "users": true, "guest": true,
		"test": true, "testing": true, "demo": true,
		"example": true, "sample": true, "anonymous": true,
	}
	if reserved[strings.ToLower(username)] {
		return errors.New("username is reserved")
	}
	return nil
}

// ValidateURL validates a URL.
func ValidateURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ErrInvalidURL
	}
	if len(rawURL) > URLMaxLength {
		return errors.New("URL exceeds maximum length")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ErrInvalidURL
	}
	if parsed.Scheme == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("URL must have http or https scheme")
	}
	if parsed.Host == "" {
		return errors.New("URL missing host")
	}
	return nil
}

// ValidatePhone validates a phone number (basic).
func ValidatePhone(phone string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ErrInvalidPhone
	}
	// Remove common separators
	clean := strings.ReplaceAll(phone, " ", "")
	clean = strings.ReplaceAll(clean, "-", "")
	clean = strings.ReplaceAll(clean, "(", "")
	clean = strings.ReplaceAll(clean, ")", "")
	clean = strings.ReplaceAll(clean, ".", "")
	clean = strings.ReplaceAll(clean, "+", "")
	if len(clean) < PhoneMinLength || len(clean) > PhoneMaxLength {
		return fmt.Errorf("phone number must be between %d and %d digits", PhoneMinLength, PhoneMaxLength)
	}
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(clean) {
		return errors.New("phone number contains invalid characters")
	}
	return nil
}

// ValidateDate validates a date string.
func ValidateDate(date string, format string) error {
	if format == "" {
		format = DateFormatISO
	}
	_, err := time.Parse(format, strings.TrimSpace(date))
	if err != nil {
		return ErrInvalidDate
	}
	return nil
}

// ValidateDateTime validates a datetime string.
func ValidateDateTime(datetime string) error {
	return ValidateDate(datetime, DateTimeFormatISO)
}

// ValidateDuration validates a duration string.
func ValidateDuration(duration string) error {
	_, err := time.ParseDuration(strings.TrimSpace(duration))
	if err != nil {
		return ErrInvalidDuration
	}
	return nil
}

// ValidateUUID validates a UUID.
func ValidateUUID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidUUID
	}
	matched, _ := regexp.MatchString(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`, id)
	if !matched {
		return ErrInvalidUUID
	}
	return nil
}

// ValidateHex validates a hex string.
func ValidateHex(hex string) error {
	hex = strings.TrimSpace(hex)
	if hex == "" {
		return ErrInvalidHex
	}
	if len(hex)%2 != 0 {
		return errors.New("hex string must have even length")
	}
	if !regexp.MustCompile(`^[0-9a-fA-F]+$`).MatchString(hex) {
		return ErrInvalidHex
	}
	return nil
}

// ValidateBase64 validates a base64 string.
func ValidateBase64(b64 string) error {
	b64 = strings.TrimSpace(b64)
	if b64 == "" {
		return ErrInvalidBase64
	}
	_, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ErrInvalidBase64
	}
	return nil
}

// ValidateAlphanumeric validates a string is alphanumeric.
func ValidateAlphanumeric(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return ErrInvalidAlphanumeric
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(s) {
		return ErrInvalidAlphanumeric
	}
	return nil
}

// ValidateAlphabetic validates a string is alphabetic.
func ValidateAlphabetic(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return ErrInvalidAlphabetic
	}
	if !regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(s) {
		return ErrInvalidAlphabetic
	}
	return nil
}

// ValidateNumeric validates a string is numeric.
func ValidateNumeric(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return ErrInvalidNumeric
	}
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(s) {
		return ErrInvalidNumeric
	}
	return nil
}

// ======================================================================
= Length Validation
// ======================================================================

// ValidateLength validates string length.
func ValidateLength(s string, min, max int) error {
	s = strings.TrimSpace(s)
	if min > 0 && len(s) < min {
		return fmt.Errorf("length must be at least %d", min)
	}
	if max > 0 && len(s) > max {
		return fmt.Errorf("length must be at most %d", max)
	}
	return nil
}

// ValidateRange validates a numeric range.
func ValidateRange[T int | int64 | float64](value T, min, max T) error {
	if value < min {
		return fmt.Errorf("value must be at least %v", min)
	}
	if value > max {
		return fmt.Errorf("value must be at most %v", max)
	}
	return nil
}

// ======================================================================
= Enum Validation
// ======================================================================

// ValidateEnum validates a value is in a list of allowed values.
func ValidateEnum(value string, allowed []string) error {
	value = strings.TrimSpace(value)
	for _, v := range allowed {
		if value == v {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrInvalidEnum, value)
}

// ValidateEnumWithLowercase validates a value is in a list (case-insensitive).
func ValidateEnumWithLowercase(value string, allowed []string) error {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, v := range allowed {
		if value == strings.ToLower(v) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrInvalidEnum, value)
}

// ======================================================================
= Password Strength
// ======================================================================

// PasswordStrength validates password strength.
type PasswordStrength struct {
	MinLength         int
	MaxLength         int
	RequireUpper      bool
	RequireLower      bool
	RequireDigit      bool
	RequireSpecial    bool
	MinComplexity     int
	DisallowCommon    bool
}

// DefaultPasswordStrength returns default strength requirements.
func DefaultPasswordStrength() PasswordStrength {
	return PasswordStrength{
		MinLength:      8,
		MaxLength:      72,
		RequireUpper:   true,
		RequireLower:   true,
		RequireDigit:   true,
		RequireSpecial: true,
		MinComplexity:  3,
		DisallowCommon: true,
	}
}

// ValidatePasswordStrength checks if a password meets strength requirements.
func ValidatePasswordStrength(password string, req PasswordStrength) []error {
	var errs []error
	password = strings.TrimSpace(password)
	if len(password) < req.MinLength {
		errs = append(errs, fmt.Errorf("password must be at least %d characters", req.MinLength))
	}
	if req.MaxLength > 0 && len(password) > req.MaxLength {
		errs = append(errs, fmt.Errorf("password must be at most %d characters", req.MaxLength))
	}
	if req.RequireUpper && !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		errs = append(errs, errors.New("password must contain at least one uppercase letter"))
	}
	if req.RequireLower && !regexp.MustCompile(`[a-z]`).MatchString(password) {
		errs = append(errs, errors.New("password must contain at least one lowercase letter"))
	}
	if req.RequireDigit && !regexp.MustCompile(`\d`).MatchString(password) {
		errs = append(errs, errors.New("password must contain at least one digit"))
	}
	if req.RequireSpecial && !regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(password) {
		errs = append(errs, errors.New("password must contain at least one special character"))
	}
	if req.DisallowCommon && isCommonPassword(password) {
		errs = append(errs, ErrPasswordCommon)
	}
	// Complexity classes
	classes := 0
	if req.RequireUpper && regexp.MustCompile(`[A-Z]`).MatchString(password) {
		classes++
	}
	if req.RequireLower && regexp.MustCompile(`[a-z]`).MatchString(password) {
		classes++
	}
	if req.RequireDigit && regexp.MustCompile(`\d`).MatchString(password) {
		classes++
	}
	if req.RequireSpecial && regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(password) {
		classes++
	}
	if req.MinComplexity > 0 && classes < req.MinComplexity {
		errs = append(errs, fmt.Errorf("password must contain at least %d different character types", req.MinComplexity))
	}
	return errs
}

// ======================================================================
= Combined Validation
// ======================================================================

// ValidationErrors aggregates multiple validation errors.
type ValidationErrors []error

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	msgs := make([]string, len(ve))
	for i, err := range ve {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}

// ValidateAll runs multiple validators and returns all errors.
func ValidateAll(validators ...func() error) error {
	var errs ValidationErrors
	for _, v := range validators {
		if err := v(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ======================================================================
= String Sanitization
// ======================================================================

// SanitizeString trims and removes control characters.
func SanitizeString(s string) string {
	s = strings.TrimSpace(s)
	// Remove control characters
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
}

// SanitizeEmail normalizes email (lowercase domain).
func SanitizeEmail(email string) string {
	email = strings.TrimSpace(email)
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		parts[1] = strings.ToLower(parts[1])
		email = parts[0] + "@" + parts[1]
	}
	return strings.ToLower(email)
}

// SanitizeUsername trims and lowercases.
func SanitizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// SanitizeURL removes trailing slash and spaces.
func SanitizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	rawURL = strings.TrimRight(rawURL, "/")
	return rawURL
}

// ======================================================================
= JSON Validation
// ======================================================================

// ValidateJSON validates a JSON string.
func ValidateJSON(jsonStr string) error {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return ErrInvalidJSON
	}
	var v interface{}
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return ErrInvalidJSON
	}
	return nil
}

// ======================================================================
= File Validation
// ======================================================================

// ValidateFileExtension validates file extension against allowed list.
func ValidateFileExtension(filename string, allowed []string) error {
	if filename == "" {
		return errors.New("filename is empty")
	}
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return errors.New("file has no extension")
	}
	ext := strings.ToLower(parts[len(parts)-1])
	for _, allowedExt := range allowed {
		if ext == allowedExt {
			return nil
		}
	}
	return fmt.Errorf("file extension %s is not allowed", ext)
}

// ValidateMimeType validates mime type against allowed list.
func ValidateMimeType(mime string, allowed []string) error {
	mime = strings.ToLower(strings.TrimSpace(mime))
	for _, allowedMime := range allowed {
		if mime == allowedMime {
			return nil
		}
	}
	return fmt.Errorf("mime type %s is not allowed", mime)
}

// ======================================================================
= IP Validation
// ======================================================================

// ValidateIP validates an IP address.
func ValidateIP(ip string) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return errors.New("IP address is empty")
	}
	if net.ParseIP(ip) == nil {
		return errors.New("invalid IP address")
	}
	return nil
}

// ValidateIPv4 validates an IPv4 address.
func ValidateIPv4(ip string) error {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return errors.New("IP address is empty")
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return errors.New("invalid IPv4 address")
	}
	return nil
}

// ======================================================================
= Domain Validation
// ======================================================================

// ValidateDomain validates a domain name.
func ValidateDomain(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return errors.New("domain is empty")
	}
	if len(domain) > 253 {
		return errors.New("domain too long")
	}
	// Check format
	if !regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`).MatchString(domain) {
		return errors.New("invalid domain format")
	}
	// Check TLD
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return errors.New("domain must have a TLD")
	}
	tld := parts[len(parts)-1]
	if len(tld) < 2 {
		return errors.New("TLD too short")
	}
	if !regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(tld) {
		return errors.New("TLD must be alphabetic")
	}
	return nil
}

// ======================================================================
= Credit Card Validation (Luhn)
// ======================================================================

// ValidateCreditCard validates a credit card number using Luhn algorithm.
func ValidateCreditCard(number string) error {
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")
	if number == "" {
		return errors.New("credit card number is empty")
	}
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(number) {
		return errors.New("credit card number must contain only digits")
	}
	if len(number) < 13 || len(number) > 19 {
		return errors.New("credit card number must be between 13 and 19 digits")
	}
	// Luhn algorithm
	sum := 0
	alternate := false
	for i := len(number) - 1; i >= 0; i-- {
		n := int(number[i] - '0')
		if alternate {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alternate = !alternate
	}
	if sum%10 != 0 {
		return errors.New("invalid credit card number (Luhn check failed)")
	}
	return nil
}

// ======================================================================
= Test Helpers
// ======================================================================

// MustValidateEmail panics on validation error (for tests).
func MustValidateEmail(email string) {
	if err := ValidateEmail(email); err != nil {
		panic(err)
	}
}

// MustValidateUsername panics on validation error (for tests).
func MustValidateUsername(username string) {
	if err := ValidateUsername(username); err != nil {
		panic(err)
	}
}