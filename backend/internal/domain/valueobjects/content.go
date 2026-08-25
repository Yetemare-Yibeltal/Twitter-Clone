// backend/internal/domain/valueobjects/content.go
package valueobjects

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ======================================================================
// Constants
// ======================================================================

const (
	MaxContentLength     = 280
	MinContentLength     = 1
	MaxHashtagLength     = 50
	MaxMentionLength     = 20
	MaxMediaCount        = 4
	DefaultContentLength = 280
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrContentEmpty           = errors.New("content cannot be empty")
	ErrContentTooLong         = fmt.Errorf("content exceeds maximum length of %d characters", MaxContentLength)
	ErrContentTooShort        = fmt.Errorf("content must be at least %d character", MinContentLength)
	ErrContentContainsInvalidChars = errors.New("content contains invalid characters")
	ErrContentOnlyWhitespace  = errors.New("content cannot be only whitespace")
	ErrContentContainsControlChars = errors.New("content contains control characters")
	ErrContentHashtagTooLong  = fmt.Errorf("hashtag exceeds maximum length of %d characters", MaxHashtagLength)
	ErrContentMentionTooLong  = fmt.Errorf("mention exceeds maximum length of %d characters", MaxMentionLength)
	ErrContentHashtagInvalid  = errors.New("invalid hashtag format")
	ErrContentMentionInvalid  = errors.New("invalid mention format")
	ErrContentHashtagTooMany  = errors.New("too many hashtags in content")
	ErrContentMentionTooMany  = errors.New("too many mentions in content")
)

// ======================================================================
// Content Entity
// ======================================================================

// Content represents a validated tweet content value object.
type Content struct {
	value      string   `json:"value"`
	hashtags   []string `json:"hashtags,omitempty"`
	mentions   []string `json:"mentions,omitempty"`
	wordCount  int      `json:"word_count"`
	charCount  int      `json:"char_count"`
	isEmpty    bool     `json:"is_empty"`
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewContent creates a new content value object with validation.
func NewContent(text string) (*Content, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, ErrContentEmpty
	}
	charCount := utf8.RuneCountInString(trimmed)
	if charCount < MinContentLength {
		return nil, ErrContentTooShort
	}
	if charCount > MaxContentLength {
		return nil, ErrContentTooLong
	}
	// Validate characters
	if err := validateContentCharacters(trimmed); err != nil {
		return nil, err
	}
	// Extract hashtags and mentions
	hashtags, err := extractValidHashtags(trimmed)
	if err != nil {
		return nil, err
	}
	mentions, err := extractValidMentions(trimmed)
	if err != nil {
		return nil, err
	}
	// Check limits
	if len(hashtags) > 50 {
		return nil, ErrContentHashtagTooMany
	}
	if len(mentions) > 50 {
		return nil, ErrContentMentionTooMany
	}
	wordCount := len(strings.Fields(trimmed))
	return &Content{
		value:      trimmed,
		hashtags:   hashtags,
		mentions:   mentions,
		wordCount:  wordCount,
		charCount:  charCount,
		isEmpty:    false,
	}, nil
}

// NewContentFromRaw creates a content without validation (use with caution).
func NewContentFromRaw(text string) *Content {
	trimmed := strings.TrimSpace(text)
	charCount := utf8.RuneCountInString(trimmed)
	wordCount := len(strings.Fields(trimmed))
	hashtags, _ := extractValidHashtags(trimmed)
	mentions, _ := extractValidMentions(trimmed)
	return &Content{
		value:      trimmed,
		hashtags:   hashtags,
		mentions:   mentions,
		wordCount:  wordCount,
		charCount:  charCount,
		isEmpty:    trimmed == "",
	}
}

// MustNewContent creates a content and panics on error.
func MustNewContent(text string) *Content {
	content, err := NewContent(text)
	if err != nil {
		panic(err)
	}
	return content
}

// ======================================================================
// Validation
// ======================================================================

// validateContentCharacters checks for invalid characters.
func validateContentCharacters(text string) error {
	// Check for control characters (except newline, tab, carriage return)
	for _, r := range text {
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			return ErrContentContainsControlChars
		}
	}
	// Check for other invalid Unicode characters
	if strings.ContainsRune(text, '\uFFFD') {
		return ErrContentContainsInvalidChars
	}
	// Check for zero-width characters
	zeroWidthPattern := regexp.MustCompile(`[\u200B-\u200D\uFEFF]`)
	if zeroWidthPattern.MatchString(text) {
		return ErrContentContainsInvalidChars
	}
	return nil
}

// Validate validates the content.
func (c *Content) Validate() error {
	if c.isEmpty {
		return ErrContentEmpty
	}
	if c.charCount < MinContentLength {
		return ErrContentTooShort
	}
	if c.charCount > MaxContentLength {
		return ErrContentTooLong
	}
	return nil
}

// ======================================================================
// Getters
// ======================================================================

// Value returns the raw content string.
func (c *Content) Value() string {
	return c.value
}

// String returns the content as a string.
func (c *Content) String() string {
	return c.value
}

// Hashtags returns the extracted hashtags.
func (c *Content) Hashtags() []string {
	return c.hashtags
}

// Mentions returns the extracted mentions.
func (c *Content) Mentions() []string {
	return c.mentions
}

// WordCount returns the number of words.
func (c *Content) WordCount() int {
	return c.wordCount
}

// CharCount returns the number of characters.
func (c *Content) CharCount() int {
	return c.charCount
}

// IsEmpty returns true if the content is empty.
func (c *Content) IsEmpty() bool {
	return c.isEmpty
}

// IsValid returns true if the content is valid.
func (c *Content) IsValid() bool {
	return c.Validate() == nil
}

// Length returns the content length in characters.
func (c *Content) Length() int {
	return c.charCount
}

// RemainingLength returns the remaining characters allowed.
func (c *Content) RemainingLength() int {
	return MaxContentLength - c.charCount
}

// IsFull returns true if the content is at max length.
func (c *Content) IsFull() bool {
	return c.charCount >= MaxContentLength
}

// ======================================================================
// Utility Methods
// ======================================================================

// Preview returns a preview of the content.
func (c *Content) Preview(maxLen int) string {
	if maxLen <= 0 {
		maxLen = 50
	}
	if len(c.value) <= maxLen {
		return c.value
	}
	return c.value[:maxLen] + "..."
}

// HashtagCount returns the number of hashtags.
func (c *Content) HashtagCount() int {
	return len(c.hashtags)
}

// MentionCount returns the number of mentions.
func (c *Content) MentionCount() int {
	return len(c.mentions)
}

// HasHashtags returns true if the content has hashtags.
func (c *Content) HasHashtags() bool {
	return len(c.hashtags) > 0
}

// HasMentions returns true if the content has mentions.
func (c *Content) HasMentions() bool {
	return len(c.mentions) > 0
}

// Contains checks if the content contains a substring.
func (c *Content) Contains(substr string) bool {
	return strings.Contains(c.value, substr)
}

// StartsWith checks if the content starts with a prefix.
func (c *Content) StartsWith(prefix string) bool {
	return strings.HasPrefix(c.value, prefix)
}

// EndsWith checks if the content ends with a suffix.
func (c *Content) EndsWith(suffix string) bool {
	return strings.HasSuffix(c.value, suffix)
}

// ======================================================================
= Extraction Functions
// ======================================================================

// extractValidHashtags extracts hashtags from content.
func extractValidHashtags(text string) ([]string, error) {
	re := regexp.MustCompile(`#([A-Za-z0-9_]+)`)
	matches := re.FindAllStringSubmatch(text, -1)
	hashtags := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		tag := strings.ToLower(match[1])
		if tag == "" {
			continue
		}
		if len(tag) > MaxHashtagLength {
			return nil, ErrContentHashtagTooLong
		}
		if !seen[tag] {
			seen[tag] = true
			hashtags = append(hashtags, tag)
		}
	}
	return hashtags, nil
}

// extractValidMentions extracts mentions from content.
func extractValidMentions(text string) ([]string, error) {
	re := regexp.MustCompile(`@([A-Za-z0-9_.-]+)`)
	matches := re.FindAllStringSubmatch(text, -1)
	mentions := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		mention := strings.ToLower(match[1])
		if mention == "" {
			continue
		}
		if len(mention) > MaxMentionLength {
			return nil, ErrContentMentionTooLong
		}
		if !seen[mention] {
			seen[mention] = true
			mentions = append(mentions, mention)
		}
	}
	return mentions, nil
}

// ======================================================================
= Content Modification
// ======================================================================

// Append appends text to the content.
func (c *Content) Append(text string) (*Content, error) {
	newText := c.value + text
	return NewContent(newText)
}

// Prepend prepends text to the content.
func (c *Content) Prepend(text string) (*Content, error) {
	newText := text + c.value
	return NewContent(newText)
}

// ReplaceAll replaces all occurrences of old with new.
func (c *Content) ReplaceAll(old, new string) (*Content, error) {
	newText := strings.ReplaceAll(c.value, old, new)
	return NewContent(newText)
}

// Truncate truncates the content to max length.
func (c *Content) Truncate(maxLen int) (*Content, error) {
	if maxLen <= 0 {
		maxLen = MaxContentLength
	}
	if c.charCount <= maxLen {
		return c, nil
	}
	// Find the last space within maxLen to avoid cutting words
	truncated := c.value[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 {
		truncated = truncated[:lastSpace]
	}
	return NewContent(truncated)
}

// ======================================================================
= Comparison and Equality
// ======================================================================

// Equals compares two content values.
func (c *Content) Equals(other *Content) bool {
	if other == nil {
		return false
	}
	return c.value == other.value
}

// EqualsString compares content to a string.
func (c *Content) EqualsString(text string) bool {
	return c.value == text
}

// Compare returns a comparison result with another content.
func (c *Content) Compare(other *Content) int {
	if c.value < other.value {
		return -1
	}
	if c.value > other.value {
		return 1
	}
	return 0
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for database storage.
func (c Content) Value() (driver.Value, error) {
	return c.value, nil
}

// Scan implements sql.Scanner for database retrieval.
func (c *Content) Scan(value interface{}) error {
	if value == nil {
		*c = Content{}
		return nil
	}
	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("unsupported type for Content: %T", value)
	}
	content, err := NewContent(str)
	if err != nil {
		// If validation fails, store as raw
		*c = *NewContentFromRaw(str)
		return nil
	}
	*c = *content
	return nil
}

// ======================================================================
= JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (c *Content) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.value)
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (c *Content) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	content, err := NewContent(str)
	if err != nil {
		// If validation fails, store as raw
		*c = *NewContentFromRaw(str)
		return nil
	}
	*c = *content
	return nil
}

// ======================================================================
// String Helpers
// ======================================================================

// ShortString returns a shortened version of the content.
func (c *Content) ShortString(maxLen int) string {
	if maxLen <= 0 {
		maxLen = 20
	}
	if len(c.value) <= maxLen {
		return c.value
	}
	return c.value[:maxLen] + "..."
}

// WordList returns the content as a list of words.
func (c *Content) WordList() []string {
	return strings.Fields(c.value)
}

// SentenceList returns the content as a list of sentences.
func (c *Content) SentenceList() []string {
	re := regexp.MustCompile(`[.!?]+`)
	parts := re.Split(c.value, -1)
	sentences := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			sentences = append(sentences, trimmed)
		}
	}
	return sentences
}

// ======================================================================
= Content Statistics
// ======================================================================

// ContentStats represents content statistics.
type ContentStats struct {
	CharCount       int     `json:"char_count"`
	WordCount       int     `json:"word_count"`
	HashtagCount    int     `json:"hashtag_count"`
	MentionCount    int     `json:"mention_count"`
	RemainingChars  int     `json:"remaining_chars"`
	IsFull          bool    `json:"is_full"`
	IsEmpty         bool    `json:"is_empty"`
	ReadabilityScore float64 `json:"readability_score"`
	UniqueWords     int     `json:"unique_words"`
}

// GetStats returns statistics about the content.
func (c *Content) GetStats() *ContentStats {
	wordList := c.WordList()
	uniqueWords := make(map[string]bool)
	for _, w := range wordList {
		uniqueWords[strings.ToLower(w)] = true
	}
	// Calculate readability score (simple Flesch-like)
	readabilityScore := 0.0
	if c.wordCount > 0 {
		// Very simplistic readability: average word length
		totalChars := 0
		for _, w := range wordList {
			totalChars += len(w)
		}
		avgWordLen := float64(totalChars) / float64(c.wordCount)
		readabilityScore = 100 - (avgWordLen * 2)
		if readabilityScore < 0 {
			readabilityScore = 0
		}
		if readabilityScore > 100 {
			readabilityScore = 100
		}
	}
	return &ContentStats{
		CharCount:        c.charCount,
		WordCount:        c.wordCount,
		HashtagCount:     c.HashtagCount(),
		MentionCount:     c.MentionCount(),
		RemainingChars:   c.RemainingLength(),
		IsFull:           c.IsFull(),
		IsEmpty:          c.isEmpty,
		ReadabilityScore: readabilityScore,
		UniqueWords:      len(uniqueWords),
	}
}

// ======================================================================
= Builder Pattern
// ======================================================================

// ContentBuilder helps construct content for testing.
type ContentBuilder struct {
	content *Content
}

// NewContentBuilder creates a new content builder.
func NewContentBuilder() *ContentBuilder {
	return &ContentBuilder{
		content: &Content{
			value:      "",
			hashtags:   []string{},
			mentions:   []string{},
			wordCount:  0,
			charCount:  0,
			isEmpty:    true,
		},
	}
}

// WithText sets the text.
func (b *ContentBuilder) WithText(text string) *ContentBuilder {
	content, err := NewContent(text)
	if err == nil {
		b.content = content
	}
	return b
}

// WithRawText sets raw text without validation.
func (b *ContentBuilder) WithRawText(text string) *ContentBuilder {
	b.content = NewContentFromRaw(text)
	return b
}

// Build returns the content.
func (b *ContentBuilder) Build() *Content {
	return b.content
}

// MustBuild builds and validates, panics on error.
func (b *ContentBuilder) MustBuild() *Content {
	if err := b.content.Validate(); err != nil {
		panic(err)
	}
	return b.content
}

// ======================================================================
// Test Helpers
// ======================================================================

var (
	TestContent1 = MustNewContent("Hello world")
	TestContent2 = MustNewContent("Check out this #amazing tweet!")
	TestContent3 = MustNewContent("@user1 Hello there!")
)

// MustNewTestContent creates a test content with default values.
func MustNewTestContent(text string) *Content {
	return MustNewContent(text)
}

// MustNewTestContentWithHashtags creates content with hashtags.
func MustNewTestContentWithHashtags(hashtags ...string) *Content {
	text := strings.Join(hashtags, " ")
	return MustNewContent(text)
}

// MustNewTestContentWithMentions creates content with mentions.
func MustNewTestContentWithMentions(mentions ...string) *Content {
	text := strings.Join(mentions, " ")
	return MustNewContent(text)
}

// ======================================================================
= Global Functions
// ======================================================================

// ValidateContent validates content without creating an object.
func ValidateContent(text string) error {
	_, err := NewContent(text)
	return err
}

// IsValidContent checks if content is valid.
func IsValidContent(text string) bool {
	_, err := NewContent(text)
	return err == nil
}

// ExtractHashtags extracts hashtags from content without validation.
func ExtractHashtags(text string) []string {
	hashtags, _ := extractValidHashtags(text)
	return hashtags
}

// ExtractMentions extracts mentions from content without validation.
func ExtractMentions(text string) []string {
	mentions, _ := extractValidMentions(text)
	return mentions
}

// SanitizeContent removes invalid characters from content.
func SanitizeContent(text string) string {
	// Remove control characters
	result := ""
	for _, r := range text {
		if r >= 32 || r == '\n' || r == '\t' || r == '\r' {
			result += string(r)
		}
	}
	return strings.TrimSpace(result)
}

// TruncateContent truncates content to max length without creating object.
func TruncateContent(text string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = MaxContentLength
	}
	charCount := utf8.RuneCountInString(text)
	if charCount <= maxLen {
		return text
	}
	// Find a good break point
	truncated := text[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		truncated = truncated[:lastSpace]
	}
	return truncated + "..."
}