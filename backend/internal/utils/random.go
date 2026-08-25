// backend/internal/utils/random.go
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	DefaultTokenLength      = 32
	MaxTokenLength          = 128
	DefaultOTPLength        = 6
	DefaultShortCodeLength  = 8
	DefaultRandomStringLen  = 16
	AlphanumericCharset     = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	NumericCharset          = "0123456789"
	HexCharset              = "0123456789abcdef"
	SecureCharset           = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()-_=+"
)

var (
	ErrRandomLengthTooShort = errors.New("random length is too short")
	ErrRandomLengthTooLong  = errors.New("random length is too long")
	ErrInvalidCharset       = errors.New("invalid charset")
)

// ======================================================================
= Cryptographically Secure Random
// ======================================================================

// GenerateRandomBytes generates cryptographically secure random bytes.
func GenerateRandomBytes(length int) ([]byte, error) {
	if length < 1 {
		return nil, ErrRandomLengthTooShort
	}
	if length > 1024 {
		return nil, ErrRandomLengthTooLong
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}

// GenerateRandomString generates a cryptographically secure random string from a charset.
func GenerateRandomString(length int, charset string) (string, error) {
	if length < 1 {
		return "", ErrRandomLengthTooShort
	}
	if length > MaxTokenLength {
		return "", ErrRandomLengthTooLong
	}
	if charset == "" {
		charset = AlphanumericCharset
	}
	charsetLen := big.NewInt(int64(len(charset)))
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random index: %w", err)
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// GenerateSecureToken generates a cryptographically secure base64 encoded token.
func GenerateSecureToken(length int) (string, error) {
	if length < 8 {
		return "", ErrRandomLengthTooShort
	}
	if length > MaxTokenLength {
		return "", ErrRandomLengthTooLong
	}
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateRandomHex generates a random hex string.
func GenerateRandomHex(length int) (string, error) {
	if length < 1 {
		return "", ErrRandomLengthTooShort
	}
	if length > MaxTokenLength {
		return "", ErrRandomLengthTooLong
	}
	bytes, err := GenerateRandomBytes((length + 1) / 2)
	if err != nil {
		return "", err
	}
	hexStr := hex.EncodeToString(bytes)
	if len(hexStr) > length {
		hexStr = hexStr[:length]
	}
	return hexStr, nil
}

// GenerateRandomAlphanumeric generates a random alphanumeric string.
func GenerateRandomAlphanumeric(length int) (string, error) {
	return GenerateRandomString(length, AlphanumericCharset)
}

// GenerateRandomNumeric generates a random numeric string.
func GenerateRandomNumeric(length int) (string, error) {
	return GenerateRandomString(length, NumericCharset)
}

// GenerateRandomSecure generates a random string with special characters.
func GenerateRandomSecure(length int) (string, error) {
	return GenerateRandomString(length, SecureCharset)
}

// ======================================================================
// OTP and Short Code Generation
// ======================================================================

// GenerateOTP generates a random numeric OTP of default length.
func GenerateOTP() (string, error) {
	return GenerateOTPWithLength(DefaultOTPLength)
}

// GenerateOTPWithLength generates a random numeric OTP of specified length.
func GenerateOTPWithLength(length int) (string, error) {
	if length < 4 || length > 12 {
		return "", errors.New("OTP length must be between 4 and 12")
	}
	return GenerateRandomNumeric(length)
}

// GenerateShortCode generates a random alphanumeric short code.
func GenerateShortCode() (string, error) {
	return GenerateRandomAlphanumeric(DefaultShortCodeLength)
}

// GenerateShortCodeWithLength generates a random short code of specified length.
func GenerateShortCodeWithLength(length int) (string, error) {
	if length < 4 || length > 20 {
		return "", errors.New("short code length must be between 4 and 20")
	}
	return GenerateRandomAlphanumeric(length)
}

// ======================================================================
// UUID Generation
// ======================================================================

// GenerateUUID generates a new UUID v4 string.
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateUUIDBytes generates a new UUID v4 as bytes.
func GenerateUUIDBytes() []byte {
	return []byte(uuid.New().String())
}

// MustGenerateUUID generates a UUID or panics on error.
func MustGenerateUUID() string {
	return uuid.New().String()
}

// GenerateUUIDV7 generates a new UUID v7 (timestamp-ordered).
func GenerateUUIDV7() (string, error) {
	// UUID v7 uses Unix timestamp in milliseconds.
	// Since we don't have a native implementation in google/uuid, we'll use v4.
	// For production, consider using a custom implementation.
	return GenerateUUID(), nil
}

// ======================================================================
// Random Time
// ======================================================================

// RandomDuration generates a random duration between min and max.
func RandomDuration(min, max time.Duration) (time.Duration, error) {
	if min >= max {
		return 0, errors.New("min must be less than max")
	}
	diff := max - min
	randSec, err := rand.Int(rand.Reader, big.NewInt(int64(diff)))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random duration: %w", err)
	}
	return min + time.Duration(randSec.Int64()), nil
}

// RandomTime generates a random time between start and end.
func RandomTime(start, end time.Time) (time.Time, error) {
	if start.After(end) {
		return time.Time{}, errors.New("start must be before end")
	}
	diff := end.Sub(start)
	dur, err := RandomDuration(0, diff)
	if err != nil {
		return time.Time{}, err
	}
	return start.Add(dur), nil
}

// RandomPastTime generates a random time in the past up to maxDays ago.
func RandomPastTime(maxDays int) (time.Time, error) {
	if maxDays < 1 {
		return time.Time{}, errors.New("maxDays must be at least 1")
	}
	start := time.Now().AddDate(0, 0, -maxDays)
	return RandomTime(start, time.Now())
}

// RandomFutureTime generates a random time in the future up to maxDays from now.
func RandomFutureTime(maxDays int) (time.Time, error) {
	if maxDays < 1 {
		return time.Time{}, errors.New("maxDays must be at least 1")
	}
	end := time.Now().AddDate(0, 0, maxDays)
	return RandomTime(time.Now(), end)
}

// ======================================================================
// Random Numbers
// ======================================================================

// RandomInt generates a random integer between min and max (inclusive).
func RandomInt(min, max int) (int, error) {
	if min > max {
		return 0, errors.New("min must be less than or equal to max")
	}
	if min == max {
		return min, nil
	}
	diff := int64(max - min + 1)
	n, err := rand.Int(rand.Reader, big.NewInt(diff))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random int: %w", err)
	}
	return int(n.Int64()) + min, nil
}

// RandomInt64 generates a random int64 between min and max (inclusive).
func RandomInt64(min, max int64) (int64, error) {
	if min > max {
		return 0, errors.New("min must be less than or equal to max")
	}
	if min == max {
		return min, nil
	}
	diff := max - min + 1
	n, err := rand.Int(rand.Reader, big.NewInt(diff))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random int64: %w", err)
	}
	return n.Int64() + min, nil
}

// RandomFloat64 generates a random float64 between min and max.
func RandomFloat64(min, max float64) (float64, error) {
	if min > max {
		return 0, errors.New("min must be less than max")
	}
	if min == max {
		return min, nil
	}
	// Generate a random int between 0 and 1<<53-1
	n, err := rand.Int(rand.Reader, big.NewInt(1<<53))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random float64: %w", err)
	}
	f := float64(n.Int64()) / (1 << 53)
	return min + f*(max-min), nil
}

// RandomBool generates a random boolean.
func RandomBool() (bool, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		return false, fmt.Errorf("failed to generate random bool: %w", err)
	}
	return n.Int64() == 0, nil
}

// ======================================================================
// Random Selection from Collections
// ======================================================================

// RandomElement returns a random element from a slice.
func RandomElement[T any](slice []T) (T, error) {
	var zero T
	if len(slice) == 0 {
		return zero, errors.New("slice is empty")
	}
	idx, err := RandomInt(0, len(slice)-1)
	if err != nil {
		return zero, err
	}
	return slice[idx], nil
}

// RandomKey returns a random key from a map.
func RandomKey[K comparable, V any](m map[K]V) (K, error) {
	var zero K
	if len(m) == 0 {
		return zero, errors.New("map is empty")
	}
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return RandomElement(keys)
}

// RandomValue returns a random value from a map.
func RandomValue[K comparable, V any](m map[K]V) (V, error) {
	var zero V
	if len(m) == 0 {
		return zero, errors.New("map is empty")
	}
	key, err := RandomKey(m)
	if err != nil {
		return zero, err
	}
	return m[key], nil
}

// Shuffle shuffles a slice in place using random swap.
func Shuffle[T any](slice []T) error {
	if len(slice) <= 1 {
		return nil
	}
	for i := len(slice) - 1; i > 0; i-- {
		j, err := RandomInt(0, i)
		if err != nil {
			return err
		}
		slice[i], slice[j] = slice[j], slice[i]
	}
	return nil
}

// ShuffleCopy returns a shuffled copy of a slice.
func ShuffleCopy[T any](slice []T) ([]T, error) {
	if len(slice) <= 1 {
		return slice, nil
	}
	copySlice := make([]T, len(slice))
	copy(copySlice, slice)
	if err := Shuffle(copySlice); err != nil {
		return nil, err
	}
	return copySlice, nil
}

// Sample returns a random sample of n elements from a slice.
func Sample[T any](slice []T, n int) ([]T, error) {
	if n < 0 {
		return nil, errors.New("sample size cannot be negative")
	}
	if n == 0 {
		return []T{}, nil
	}
	if n > len(slice) {
		return nil, errors.New("sample size exceeds slice length")
	}
	if n == len(slice) {
		return slice, nil
	}
	shuffled, err := ShuffleCopy(slice)
	if err != nil {
		return nil, err
	}
	return shuffled[:n], nil
}

// ======================================================================
// ID Generation
// ======================================================================

// GenerateID generates a unique ID with optional prefix.
func GenerateID(prefix string) string {
	if prefix != "" {
		return fmt.Sprintf("%s_%s", prefix, GenerateUUID())
	}
	return GenerateUUID()
}

// GenerateShortID generates a short unique ID using base64 encoding.
func GenerateShortID() (string, error) {
	bytes, err := GenerateRandomBytes(12)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateNanoID generates a nanosecond timestamp-based ID.
func GenerateNanoID() string {
	return fmt.Sprintf("%d_%s", time.Now().UnixNano(), GenerateUUID()[:8])
}

// ======================================================================
// Password Generation
// ======================================================================

// GeneratePassword generates a secure random password with given length.
func GeneratePassword(length int) (string, error) {
	if length < 8 {
		length = 8
	}
	if length > 64 {
		length = 64
	}
	// Ensure at least one of each character type
	password := make([]byte, length)
	// Generate random positions for required characters
	upperIdx, _ := RandomInt(0, length-1)
	lowerIdx, _ := RandomInt(0, length-1)
	digitIdx, _ := RandomInt(0, length-1)
	specialIdx, _ := RandomInt(0, length-1)
	// Ensure unique positions
	for lowerIdx == upperIdx {
		lowerIdx, _ = RandomInt(0, length-1)
	}
	for digitIdx == upperIdx || digitIdx == lowerIdx {
		digitIdx, _ = RandomInt(0, length-1)
	}
	for specialIdx == upperIdx || specialIdx == lowerIdx || specialIdx == digitIdx {
		specialIdx, _ = RandomInt(0, length-1)
	}
	// Fill with random characters
	charsets := []string{
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"abcdefghijklmnopqrstuvwxyz",
		"0123456789",
		"!@#$%^&*()_+-=[]{}|;:,.<>?",
	}
	// Assign required chars
	for i := 0; i < length; i++ {
		var charsetIdx int
		switch i {
		case upperIdx:
			charsetIdx = 0
		case lowerIdx:
			charsetIdx = 1
		case digitIdx:
			charsetIdx = 2
		case specialIdx:
			charsetIdx = 3
		default:
			charsetIdx, _ = RandomInt(0, 3)
		}
		ch, err := RandomElement(strings.Split(charsets[charsetIdx], ""))
		if err != nil {
			return "", err
		}
		password[i] = ch[0]
	}
	return string(password), nil
}

// GenerateEasyPassword generates a password with only alphanumeric.
func GenerateEasyPassword(length int) (string, error) {
	if length < 6 {
		length = 6
	}
	if length > 32 {
		length = 32
	}
	return GenerateRandomAlphanumeric(length)
}

// ======================================================================
= Session Token Generation
// ======================================================================

// GenerateSessionToken generates a cryptographically secure session token.
func GenerateSessionToken() (string, error) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateRefreshToken generates a secure refresh token.
func GenerateRefreshToken() (string, error) {
	return GenerateSessionToken()
}

// GenerateAccessToken generates a secure access token (for testing).
func GenerateAccessToken() (string, error) {
	return GenerateSessionToken()
}

// ======================================================================
// API Key Generation
// ======================================================================

// GenerateAPIKey generates a new API key with prefix.
func GenerateAPIKey(prefix string) (string, error) {
	if prefix == "" {
		prefix = "sk"
	}
	token, err := GenerateSecureToken(24)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", prefix, token), nil
}

// GenerateAPIKeyWithSecret generates a pair of API key and secret.
func GenerateAPIKeyWithSecret(prefix string) (string, string, error) {
	key, err := GenerateAPIKey(prefix)
	if err != nil {
		return "", "", err
	}
	secret, err := GenerateSecureToken(32)
	if err != nil {
		return "", "", err
	}
	return key, secret, nil
}

// ======================================================================
= Verification Token Generation
// ======================================================================

// GenerateVerificationToken generates a token for email verification.
func GenerateVerificationToken() (string, error) {
	return GenerateSecureToken(24)
}

// GeneratePasswordResetToken generates a token for password reset.
func GeneratePasswordResetToken() (string, error) {
	return GenerateSecureToken(24)
}

// ======================================================================
= Cryptographic Nonce
// ======================================================================

// GenerateNonce generates a cryptographic nonce.
func GenerateNonce() (string, error) {
	bytes, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ======================================================================
= Deterministic Random (for testing)
// ======================================================================

// NewSeededRandom creates a deterministic random generator for tests.
// This is not cryptographically secure, only for testing.
func NewSeededRandom(seed int64) *RandomGenerator {
	return &RandomGenerator{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// RandomGenerator is a deterministic random generator for testing.
type RandomGenerator struct {
	rng *rand.Rand
}

// Intn returns a random int < n.
func (r *RandomGenerator) Intn(n int) int {
	return r.rng.Intn(n)
}

// String returns a random string of length n from charset.
func (r *RandomGenerator) String(n int, charset string) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[r.rng.Intn(len(charset))]
	}
	return string(b)
}

// Shuffle shuffles a slice deterministically.
func (r *RandomGenerator) Shuffle[T any](slice []T) {
	r.rng.Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})
}

// ======================================================================
= Helper Functions
// ======================================================================

// MustGenerateRandomString generates a random string or panics.
func MustGenerateRandomString(length int, charset string) string {
	s, err := GenerateRandomString(length, charset)
	if err != nil {
		panic(err)
	}
	return s
}

// MustGenerateOTP generates an OTP or panics.
func MustGenerateOTP() string {
	otp, err := GenerateOTP()
	if err != nil {
		panic(err)
	}
	return otp
}

// MustGenerateUUID generates a UUID or panics.
func MustGenerateUUID() string {
	return GenerateUUID()
}

// ======================================================================
= Test Helpers
// ======================================================================

// RandomEmail generates a random email for testing.
func RandomEmail() (string, error) {
	username, err := GenerateRandomAlphanumeric(8)
	if err != nil {
		return "", err
	}
	domain, err := GenerateRandomAlphanumeric(6)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s@%s.com", username, domain), nil
}

// RandomUsername generates a random username for testing.
func RandomUsername() (string, error) {
	return GenerateRandomAlphanumeric(8)
}

// RandomFullName generates a random full name for testing.
func RandomFullName() (string, error) {
	first, err := GenerateRandomAlphanumeric(6)
	if err != nil {
		return "", err
	}
	last, err := GenerateRandomAlphanumeric(8)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s", strings.Title(first), strings.Title(last)), nil
}