// backend/internal/domain/valueobjects/username.go
package valueobjects

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Common errors for username validation.
var (
	ErrEmptyUsername         = errors.New("username cannot be empty")
	ErrUsernameTooShort      = errors.New("username must be at least 3 characters")
	ErrUsernameTooLong       = errors.New("username must be at most 20 characters")
	ErrInvalidChars          = errors.New("username contains invalid characters (only letters, numbers, underscore, dot, hyphen allowed)")
	ErrStartsOrEndsWithDot   = errors.New("username cannot start or end with a dot")
	ErrStartsOrEndsWithHyphen = errors.New("username cannot start or end with a hyphen")
	ErrStartsOrEndsWithUnderscore = errors.New("username cannot start or end with underscore")
	ErrConsecutiveSpecial    = errors.New("username cannot have consecutive special characters")
	ErrReservedUsername      = errors.New("username is reserved and cannot be used")
	ErrProfaneUsername       = errors.New("username contains inappropriate language")
	ErrUsernameAllDigits     = errors.New("username cannot consist solely of digits")
	ErrUsernameAlreadyTaken  = errors.New("username is already taken") // used in repo, but included for completeness
)

// Username represents a validated username value object.
type Username struct {
	value string
}

// NewUsername creates a new Username after validation and normalisation.
func NewUsername(username string) (Username, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return Username{}, ErrEmptyUsername
	}
	normalized, err := normalizeAndValidate(username)
	if err != nil {
		return Username{}, err
	}
	return Username{value: normalized}, nil
}

// MustNewUsername creates a Username and panics on error.
func MustNewUsername(username string) Username {
	u, err := NewUsername(username)
	if err != nil {
		panic(err)
	}
	return u
}

// normalizeAndValidate performs validation and normalisation.
func normalizeAndValidate(username string) (string, error) {
	// Convert to lowercase and trim again
	username = strings.ToLower(strings.TrimSpace(username))

	// Check length
	length := len(username)
	if length < 3 {
		return "", ErrUsernameTooShort
	}
	if length > 20 {
		return "", ErrUsernameTooLong
	}

	// Character validation: allowed letters, numbers, underscore, dot, hyphen
	validChar := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	if !validChar.MatchString(username) {
		return "", ErrInvalidChars
	}

	// Cannot start or end with dot, hyphen, or underscore
	if strings.HasPrefix(username, ".") || strings.HasSuffix(username, ".") {
		return "", ErrStartsOrEndsWithDot
	}
	if strings.HasPrefix(username, "-") || strings.HasSuffix(username, "-") {
		return "", ErrStartsOrEndsWithHyphen
	}
	if strings.HasPrefix(username, "_") || strings.HasSuffix(username, "_") {
		return "", ErrStartsOrEndsWithUnderscore
	}

	// No consecutive special characters (._-)
	special := regexp.MustCompile(`[._-]{2,}`)
	if special.MatchString(username) {
		return "", ErrConsecutiveSpecial
	}

	// Cannot be all digits
	if regexp.MustCompile(`^[0-9]+$`).MatchString(username) {
		return "", ErrUsernameAllDigits
	}

	// Check against reserved list
	if isReserved(username) {
		return "", ErrReservedUsername
	}

	// Check profanity (basic)
	if isProfane(username) {
		return "", ErrProfaneUsername
	}

	return username, nil
}

// isReserved checks against a list of reserved usernames.
func isReserved(username string) bool {
	reserved := map[string]bool{
		"admin": true, "administrator": true, "root": true, "sysadmin": true,
		"system": true, "support": true, "help": true, "info": true,
		"noreply": true, "no-reply": true, "noreply": true,
		"postmaster": true, "webmaster": true, "hostmaster": true,
		"abuse": true, "security": true, "privacy": true,
		"moderator": true, "mod": true, "owner": true,
		"admin1": true, "admin2": true, "manager": true,
		"user": true, "users": true, "guest": true,
		"test": true, "testing": true, "demo": true,
		"example": true, "sample": true, "anonymous": true,
		"default": true, "null": true, "undefined": true,
		"api": true, "app": true, "dev": true,
		"git": true, "github": true, "twitter": true,
		"facebook": true, "google": true, "microsoft": true,
		"apple": true, "amazon": true, "netflix": true,
		"spotify": true, "discord": true, "slack": true,
		"teams": true, "zoom": true, "meet": true,
	}
	return reserved[username]
}

// isProfane checks against a list of offensive terms (simplified).
func isProfane(username string) bool {
	// Basic profanity list (non-exhaustive)
	profanity := map[string]bool{
		"fuck": true, "shit": true, "ass": true, "bitch": true,
		"bastard": true, "damn": true, "hell": true, "crap": true,
		"dick": true, "pussy": true, "cock": true, "cunt": true,
		"whore": true, "slut": true, "retard": true, "idiot": true,
		"moron": true, "stupid": true, "dumb": true, "loser": true,
		"nigger": true, "nigga": true, "chink": true, "spic": true,
		"gook": true, "kike": true, "faggot": true, "dyke": true,
		"tranny": true, "queer": true, "paki": true, "raghead": true,
		"towelhead": true, "sandnigger": true, "beaner": true,
		"wetback": true, "zipperhead": true, "gringo": true,
	}
	// Check if username contains any profane word as a substring
	for word := range profanity {
		if strings.Contains(username, word) {
			return true
		}
	}
	return false
}

// String returns the username as a string.
func (u Username) String() string {
	return u.value
}

// Value returns the raw username value.
func (u Username) Value() string {
	return u.value
}

// Normalised returns a copy with lowercase (already done).
func (u Username) Normalised() Username {
	return u
}

// Equal checks if two usernames are equal (case-insensitive).
func (u Username) Equal(other Username) bool {
	return u.value == other.value
}

// EqualFold is a case-insensitive comparison.
func (u Username) EqualFold(other Username) bool {
	return strings.EqualFold(u.value, other.value)
}

// IsEmpty returns true if the username is zero value.
func (u Username) IsEmpty() bool {
	return u.value == ""
}

// IsValid returns true if the username is non-empty (always true for constructed).
func (u Username) IsValid() bool {
	return u.value != ""
}

// Obfuscate returns a partially hidden version (e.g., "joh***").
func (u Username) Obfuscate() string {
	if u.value == "" {
		return ""
	}
	if len(u.value) <= 3 {
		return u.value
	}
	return u.value[:2] + strings.Repeat("*", len(u.value)-2)
}

// MarshalJSON implements json.Marshaler.
func (u Username) MarshalJSON() ([]byte, error) {
	return []byte(`"` + u.value + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (u *Username) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := NewUsername(s)
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

// Value implements driver.Valuer for database storage.
func (u Username) Value() (driver.Value, error) {
	if u.IsEmpty() {
		return nil, nil // allow NULL
	}
	return u.value, nil
}

// Scan implements sql.Scanner for database retrieval.
func (u *Username) Scan(value interface{}) error {
	if value == nil {
		*u = Username{}
		return nil
	}
	switch v := value.(type) {
	case string:
		parsed, err := NewUsername(v)
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	case []byte:
		parsed, err := NewUsername(string(v))
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	default:
		return fmt.Errorf("unsupported type for Username: %T", value)
	}
}

// Format returns the username as a string.
func (u Username) Format() string {
	return u.value
}

// IsZero returns true if Username is zero value.
func (u Username) IsZero() bool {
	return u.value == ""
}

// SetEmpty creates a zero Username.
func SetEmptyUsername() Username {
	return Username{}
}

// IsNotEmpty returns true if non-empty.
func (u Username) IsNotEmpty() bool {
	return !u.IsEmpty()
}

// StartsWith checks if username starts with a prefix.
func (u Username) StartsWith(prefix string) bool {
	return strings.HasPrefix(u.value, prefix)
}

// EndsWith checks if username ends with a suffix.
func (u Username) EndsWith(suffix string) bool {
	return strings.HasSuffix(u.value, suffix)
}

// Contains checks if username contains a substring.
func (u Username) Contains(sub string) bool {
	return strings.Contains(u.value, sub)
}

// ----- Additional utilities for username generation -----

// GenerateFromEmail creates a username from an email address (sanitized).
func GenerateFromEmail(email Email) Username {
	local := email.GetLocalPart()
	// Remove special characters and truncate
	sanitized := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(local, "")
	if len(sanitized) > 20 {
		sanitized = sanitized[:20]
	}
	if len(sanitized) < 3 {
		// If too short, append random numbers (for testing)
		sanitized = sanitized + "123"
	}
	// Try to create, fallback if reserved
	u, err := NewUsername(sanitized)
	if err != nil {
		// If error, generate a fallback with timestamp
		fallback := fmt.Sprintf("user%d", time.Now().UnixNano()%100000)
		u, _ = NewUsername(fallback)
	}
	return u
}

// RandomUsername generates a random username for testing.
func RandomUsername() Username {
	// Simple random
	letters := "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	u, _ := NewUsername("user" + string(b))
	return u
}

// ValidateOnly checks if a username string is valid without constructing.
func ValidateOnly(username string) error {
	_, err := NewUsername(username)
	return err
}

// IsValidUsernameString is a convenience function.
func IsValidUsernameString(username string) bool {
	_, err := NewUsername(username)
	return err == nil
}

// ----- Type aliases for slices -----

// UsernameList is a slice of Username.
type UsernameList []Username

// Contains checks if a username exists in the list.
func (ul UsernameList) Contains(u Username) bool {
	for _, existing := range ul {
		if existing.Equal(u) {
			return true
		}
	}
	return false
}

// Strings returns the usernames as strings.
func (ul UsernameList) Strings() []string {
	res := make([]string, len(ul))
	for i, u := range ul {
		res[i] = u.String()
	}
	return res
}

// FromStrings converts a slice of strings to UsernameList, ignoring invalid.
func FromStrings(strs []string) UsernameList {
	var list UsernameList
	for _, s := range strs {
		if u, err := NewUsername(s); err == nil {
			list = append(list, u)
		}
	}
	return list
}

// MustFromStrings panics on any invalid string.
func MustFromStrings(strs []string) UsernameList {
	list := make(UsernameList, len(strs))
	for i, s := range strs {
		list[i] = MustNewUsername(s)
	}
	return list
}

// ---- Test helpers ----

var (
	TestUsername1 = MustNewUsername("john_doe")
	TestUsername2 = MustNewUsername("jane_smith")
	TestUsername3 = MustNewUsername("user123")
)

// IsReservedUsername exported for tests.
func IsReservedUsername(u Username) bool {
	return isReserved(u.value)
}

// IsProfaneUsername exported for tests.
func IsProfaneUsername(u Username) bool {
	return isProfane(u.value)
}