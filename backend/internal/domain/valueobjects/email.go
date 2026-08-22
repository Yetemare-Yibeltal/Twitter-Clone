// backend/internal/domain/valueobjects/email.go
package valueobjects

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Common errors for email validation.
var (
	ErrEmptyEmail          = errors.New("email address is empty")
	ErrInvalidFormat       = errors.New("invalid email format")
	ErrMissingAtSign       = errors.New("email must contain @ symbol")
	ErrLocalPartEmpty      = errors.New("email local part cannot be empty")
	ErrDomainEmpty         = errors.New("email domain cannot be empty")
	ErrLocalPartTooLong    = errors.New("local part exceeds maximum length (64 characters)")
	ErrDomainTooLong       = errors.New("domain exceeds maximum length (255 characters)")
	ErrEmailTooLong        = errors.New("email exceeds maximum length (320 characters)")
	ErrInvalidLocalPart    = errors.New("local part contains invalid characters")
	ErrInvalidDomain       = errors.New("domain contains invalid characters or format")
	ErrDoubleDot           = errors.New("email contains consecutive dots")
	ErrStartsOrEndsWithDot = errors.New("email local part cannot start or end with a dot")
	ErrQuotedStringUnclosed = errors.New("quoted string in local part is not closed")
	ErrInvalidDomainTLD    = errors.New("domain TLD is invalid or too short")
	ErrDisposableEmail     = errors.New("disposable email address not allowed")
	ErrFreeEmailProvider   = errors.New("free email provider not allowed") // optional
)

// Email represents a validated email address value object.
type Email struct {
	value string
}

// NewEmail creates a new Email instance after validating and normalising the input.
func NewEmail(email string) (Email, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return Email{}, ErrEmptyEmail
	}
	normalized, err := normalizeAndValidate(email)
	if err != nil {
		return Email{}, err
	}
	return Email{value: normalized}, nil
}

// MustNewEmail creates an Email and panics on error.
func MustNewEmail(email string) Email {
	e, err := NewEmail(email)
	if err != nil {
		panic(err)
	}
	return e
}

// Parse is an alias for NewEmail.
func Parse(email string) (Email, error) {
	return NewEmail(email)
}

// normalizeAndValidate performs comprehensive validation and normalisation.
func normalizeAndValidate(email string) (string, error) {
	// Trim and lowercase domain part later (normalisation).
	original := email

	// Check total length (RFC 5321: 320 characters max).
	if len(original) > 320 {
		return "", ErrEmailTooLong
	}

	// Must contain @
	atIndex := strings.LastIndex(original, "@")
	if atIndex == -1 {
		return "", ErrMissingAtSign
	}

	localPart := original[:atIndex]
	domainPart := original[atIndex+1:]

	// Local part validation
	if localPart == "" {
		return "", ErrLocalPartEmpty
	}
	if len(localPart) > 64 {
		return "", ErrLocalPartTooLong
	}

	// Domain validation
	if domainPart == "" {
		return "", ErrDomainEmpty
	}
	if len(domainPart) > 255 {
		return "", ErrDomainTooLong
	}

	// Validate local part characters and structure
	if err := validateLocalPart(localPart); err != nil {
		return "", err
	}

	// Validate domain part
	if err := validateDomain(domainPart); err != nil {
		return "", err
	}

	// Normalise: lowercase domain part, but keep local part as is (case-sensitive? Usually we lowercase local too)
	// For consistency, we lower-case both (RFC allows case-sensitive local but in practice it's case-insensitive).
	// We'll lower-case both for uniqueness.
	normalized := strings.ToLower(original)
	return normalized, nil
}

// validateLocalPart validates the local part of an email address.
func validateLocalPart(local string) error {
	// Can't start or end with dot
	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") {
		return ErrStartsOrEndsWithDot
	}
	// No consecutive dots
	if strings.Contains(local, "..") {
		return ErrDoubleDot
	}

	// If it's quoted, validate quoted string
	if strings.HasPrefix(local, "\"") {
		if !strings.HasSuffix(local, "\"") {
			return ErrQuotedStringUnclosed
		}
		// Inside quoted string, allowed characters: any except backslash, quote, and control chars
		// For simplicity we'll just accept it.
		return nil
	}

	// For unquoted local part: allowed: letters, digits, and special chars: ! # $ % & ' * + - / = ? ^ _ ` { | } ~
	// Also . is allowed but we've handled dot rules.
	validChars := regexp.MustCompile(`^[a-zA-Z0-9!#$%&'*+/=?^_` + "`" + `{|}~.-]+$`)
	if !validChars.MatchString(local) {
		return ErrInvalidLocalPart
	}
	return nil
}

// validateDomain validates the domain part of an email address.
func validateDomain(domain string) error {
	// Domain can be an IP address in brackets: [192.168.1.1]
	if strings.HasPrefix(domain, "[") && strings.HasSuffix(domain, "]") {
		// Simple validation: check if inside is a valid IP (we'll skip complex validation)
		ip := domain[1 : len(domain)-1]
		// Basic IP format: dotted decimal or IPv6
		// For simplicity, we'll just accept it.
		// TODO: proper IP validation
		return nil
	}

	// Domain must contain at least one dot (unless it's localhost or a TLD-only?)
	if !strings.Contains(domain, ".") {
		return ErrInvalidDomain
	}

	// Domain labels: each label between dots must be valid
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" {
			return ErrInvalidDomain
		}
		// Label length: max 63
		if len(label) > 63 {
			return ErrInvalidDomain
		}
		// Label must start and end with alphanumeric, and may contain hyphens
		if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$`).MatchString(label) {
			// Allow single-character labels (like 'a')
			if len(label) == 1 && regexp.MustCompile(`^[a-zA-Z0-9]$`).MatchString(label) {
				continue
			}
			return ErrInvalidDomain
		}
	}

	// TLD check: last label must be at least 2 characters and alphabetic
	tld := labels[len(labels)-1]
	if len(tld) < 2 || !regexp.MustCompile(`^[a-zA-Z]{2,}$`).MatchString(tld) {
		return ErrInvalidDomainTLD
	}
	return nil
}

// String returns the email as a string.
func (e Email) String() string {
	return e.value
}

// Value returns the raw email value.
func (e Email) Value() string {
	return e.value
}

// Normalised returns a copy with the domain lowercased (already done in construction).
func (e Email) Normalised() Email {
	// Already normalised
	return e
}

// Equal checks if two emails are equal (case-insensitive by design).
func (e Email) Equal(other Email) bool {
	return e.value == other.value
}

// EqualFold is a case-insensitive comparison.
func (e Email) EqualFold(other Email) bool {
	return strings.EqualFold(e.value, other.value)
}

// IsEmpty checks if the email is zero value.
func (e Email) IsEmpty() bool {
	return e.value == ""
}

// IsValid returns true if the email is valid (always true for constructed).
func (e Email) IsValid() bool {
	return e.value != ""
}

// GetLocalPart returns the local part of the email.
func (e Email) GetLocalPart() string {
	parts := strings.Split(e.value, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

// GetDomain returns the domain part of the email.
func (e Email) GetDomain() string {
	parts := strings.Split(e.value, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// Obfuscate returns a partially hidden version of the email (e.g., j***@example.com).
func (e Email) Obfuscate() string {
	if e.value == "" {
		return ""
	}
	parts := strings.Split(e.value, "@")
	local := parts[0]
	domain := parts[1]
	if len(local) <= 2 {
		return local + "@" + domain
	}
	obfLocal := local[:2] + strings.Repeat("*", len(local)-2)
	return obfLocal + "@" + domain
}

// GetProvider returns the email provider (e.g., "gmail", "yahoo", etc.) or "other".
func (e Email) GetProvider() string {
	domain := e.GetDomain()
	domain = strings.ToLower(domain)
	// Common providers
	providers := map[string]string{
		"gmail.com":       "gmail",
		"yahoo.com":       "yahoo",
		"yahoo.co.uk":     "yahoo",
		"hotmail.com":     "hotmail",
		"outlook.com":     "outlook",
		"live.com":        "outlook",
		"msn.com":         "outlook",
		"aol.com":         "aol",
		"protonmail.com":  "protonmail",
		"proton.me":       "protonmail",
		"icloud.com":      "icloud",
		"me.com":          "icloud",
		"mac.com":         "icloud",
		"zoho.com":        "zoho",
		"mail.com":        "mailcom",
		"yandex.com":      "yandex",
		"yandex.ru":       "yandex",
		"gmx.com":         "gmx",
		"gmx.net":         "gmx",
		"fastmail.com":    "fastmail",
		"hey.com":         "hey",
		"tutanota.com":    "tutanota",
		"tutanota.de":     "tutanota",
	}
	if provider, ok := providers[domain]; ok {
		return provider
	}
	return "other"
}

// IsCommonProvider returns true if the email is from a major provider.
func (e Email) IsCommonProvider() bool {
	return e.GetProvider() != "other"
}

// IsDisposable returns true if the domain is in a list of disposable email providers.
func (e Email) IsDisposable() bool {
	domain := e.GetDomain()
	domain = strings.ToLower(domain)
	// Built-in disposable domains (incomplete; can be extended)
	disposableDomains := map[string]bool{
		"mailinator.com":    true,
		"guerrillamail.com": true,
		"guerrillamail.net": true,
		"guerrillamail.org": true,
		"guerrillamail.biz": true,
		"guerrillamail.info": true,
		"tempmail.com":      true,
		"tempmail.net":      true,
		"temp-mail.org":     true,
		"10minutemail.com":  true,
		"10minutemail.net":  true,
		"10minutemail.org":  true,
		"throwawayemail.com": true,
		"trashmail.com":     true,
		"trashmail.net":     true,
		"spamgourmet.com":   true,
		"spam.la":           true,
		"yopmail.com":       true,
		"yopmail.fr":        true,
		"yopmail.net":       true,
		"maildrop.cc":       true,
		"getairmail.com":    true,
		"getnada.com":       true,
		"mintemail.com":     true,
		"thankyou2010.com":  true,
		"trash2009.com":     true,
		"wegwerfmail.de":    true,
		"wegwerfmail.net":   true,
		"wegwerfmail.org":   true,
		"mytempemail.com":   true,
		"tempinbox.com":     true,
		"tempinbox.co":      true,
		"tempinbox.info":    true,
		"fakeinbox.com":     true,
		"fakeinbox.net":     true,
		"fakemail.com":      true,
		"fakemail.net":      true,
		"emailondeck.com":   true,
		"e4ward.com":        true,
		"inboxalias.com":    true,
		"inboxbear.com":     true,
		"inboxkitten.com":   true,
		"inboxstore.me":     true,
		"mail-tester.com":   true,
		"mail2world.com":    true,
		"mailcatch.com":     true,
		"mailfa.com":        true,
		"mailin.fr":         true,
		"mailexpire.com":    true,
		"mailnesia.com":     true,
		"mailnator.com":     true,
		"mailpoof.com":      true,
		"mailprive.com":     true,
		"mailexpire.com":    true,
		"mailsac.com":       true,
	}
	return disposableDomains[domain]
}

// IsFreeProvider returns true if the email is from a free provider (like Gmail, Yahoo, etc.).
func (e Email) IsFreeProvider() bool {
	// This is similar to common providers but more comprehensive.
	freeDomains := map[string]bool{
		"gmail.com": true, "yahoo.com": true, "hotmail.com": true,
		"outlook.com": true, "live.com": true, "msn.com": true,
		"aol.com": true, "protonmail.com": true, "proton.me": true,
		"icloud.com": true, "me.com": true, "mac.com": true,
		"zoho.com": true, "mail.com": true, "yandex.com": true,
		"yandex.ru": true, "gmx.com": true, "gmx.net": true,
		"fastmail.com": true, "hey.com": true, "tutanota.com": true,
		"tutanota.de": true, "googlemail.com": true,
		"inbox.com": true, "inbox.ru": true, "list.ru": true,
		"bk.ru": true, "mail.ru": true, "internet.ru": true,
	}
	return freeDomains[e.GetDomain()]
}

// IsEducational returns true if the domain ends with .edu (or similar).
func (e Email) IsEducational() bool {
	domain := e.GetDomain()
	domain = strings.ToLower(domain)
	return strings.HasSuffix(domain, ".edu") ||
		strings.HasSuffix(domain, ".ac.uk") ||
		strings.HasSuffix(domain, ".edu.au") ||
		strings.HasSuffix(domain, ".edu.sg") ||
		strings.HasSuffix(domain, ".edu.tr") ||
		strings.HasSuffix(domain, ".ac.za")
}

// IsGovernment returns true if the domain ends with .gov or .mil.
func (e Email) IsGovernment() bool {
	domain := e.GetDomain()
	domain = strings.ToLower(domain)
	return strings.HasSuffix(domain, ".gov") ||
		strings.HasSuffix(domain, ".mil") ||
		strings.HasSuffix(domain, ".gov.uk") ||
		strings.HasSuffix(domain, ".gov.au") ||
		strings.HasSuffix(domain, ".gov.in")
}

// MarshalJSON implements the json.Marshaler interface.
func (e Email) MarshalJSON() ([]byte, error) {
	return []byte(`"` + e.value + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (e *Email) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := NewEmail(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}

// Value implements the driver.Valuer interface for database storage.
func (e Email) Value() (driver.Value, error) {
	if e.IsEmpty() {
		return nil, nil // allow NULL in DB
	}
	return e.value, nil
}

// Scan implements the sql.Scanner interface for database retrieval.
func (e *Email) Scan(value interface{}) error {
	if value == nil {
		*e = Email{}
		return nil
	}
	switch v := value.(type) {
	case string:
		parsed, err := NewEmail(v)
		if err != nil {
			return err
		}
		*e = parsed
		return nil
	case []byte:
		parsed, err := NewEmail(string(v))
		if err != nil {
			return err
		}
		*e = parsed
		return nil
	default:
		return fmt.Errorf("unsupported type for Email: %T", value)
	}
}

// String is used by fmt.Print, etc.
func (e Email) String() string {
	return e.value
}

// Format returns the email as a string.
func (e Email) Format() string {
	return e.value
}

// IsZero returns true if the Email is the zero value.
func (e Email) IsZero() bool {
	return e.value == ""
}

// SetEmpty creates a zero Email for optional fields.
func SetEmpty() Email {
	return Email{}
}

// IsNotEmpty returns true if the email is non-empty.
func (e Email) IsNotEmpty() bool {
	return !e.IsEmpty()
}

// ----- Additional utilities -----

// EmailList is a slice of Email.
type EmailList []Email

// Contains checks if an email exists in the list.
func (el EmailList) Contains(email Email) bool {
	for _, e := range el {
		if e.Equal(email) {
			return true
		}
	}
	return false
}

// Strings returns the emails as strings.
func (el EmailList) Strings() []string {
	res := make([]string, len(el))
	for i, e := range el {
		res[i] = e.String()
	}
	return res
}

// FromStrings converts a slice of strings to EmailList, ignoring invalid ones.
func FromStrings(strs []string) EmailList {
	var emails EmailList
	for _, s := range strs {
		if e, err := NewEmail(s); err == nil {
			emails = append(emails, e)
		}
	}
	return emails
}

// MustFromStrings converts a slice of strings to EmailList, panicking on any error.
func MustFromStrings(strs []string) EmailList {
	emails := make(EmailList, len(strs))
	for i, s := range strs {
		emails[i] = MustNewEmail(s)
	}
	return emails
}

// ---- Testing helpers (can be used in tests) ----
var (
	// Common test emails
	TestEmail1 = MustNewEmail("test@example.com")
	TestEmail2 = MustNewEmail("user@domain.com")
	TestEmail3 = MustNewEmail("john.doe@gmail.com")
)

// GenerateFakeEmail creates a random fake email (for testing).
func GenerateFakeEmail(domain string) Email {
	// simple random local part
	local := fmt.Sprintf("user%d", time.Now().UnixNano()%10000)
	return MustNewEmail(local + "@" + domain)
}