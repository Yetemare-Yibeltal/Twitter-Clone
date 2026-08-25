// backend/internal/domain/entities/retweet.go
package entities

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ======================================================================
// Constants
// ======================================================================

// RetweetType represents the type of retweet.
type RetweetType string

const (
	RetweetTypeRegular RetweetType = "regular"
	RetweetTypeQuote   RetweetType = "quote" // retweet with comment
)

// ValidRetweetTypes returns all valid retweet types.
func ValidRetweetTypes() []RetweetType {
	return []RetweetType{
		RetweetTypeRegular,
		RetweetTypeQuote,
	}
}

// IsValid checks if a retweet type is valid.
func (t RetweetType) IsValid() bool {
	for _, typ := range ValidRetweetTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation of the retweet type.
func (t RetweetType) String() string {
	return string(t)
}

// ======================================================================
// Errors
// ======================================================================

var (
	ErrRetweetIDEmpty          = errors.New("retweet ID cannot be empty")
	ErrRetweetTweetIDEmpty     = errors.New("tweet ID cannot be empty")
	ErrRetweetUserIDEmpty      = errors.New("user ID cannot be empty")
	ErrRetweetAlreadyExists    = errors.New("retweet already exists")
	ErrRetweetNotFound         = errors.New("retweet not found")
	ErrRetweetAlreadyDeleted   = errors.New("retweet already deleted")
	ErrRetweetNotDeleted       = errors.New("retweet is not deleted")
	ErrInvalidRetweetType      = errors.New("invalid retweet type")
	ErrRetweetSelf             = errors.New("cannot retweet your own tweet")
	ErrRetweetQuoteEmpty       = errors.New("quote content cannot be empty")
	ErrRetweetQuoteTooLong     = errors.New("quote content exceeds maximum length")
	ErrRetweetAlreadyQuoted    = errors.New("tweet already quoted")
	ErrRetweetOriginalDeleted  = errors.New("original tweet has been deleted")
	ErrRetweetOriginalNotFound = errors.New("original tweet not found")
)

// ======================================================================
// Retweet Entity
// ======================================================================

// Retweet represents a retweet of a tweet.
type Retweet struct {
	ID           string       `db:"id" json:"id"`
	TweetID      string       `db:"tweet_id" json:"tweet_id"`
	UserID       string       `db:"user_id" json:"user_id"`
	Type         RetweetType  `db:"type" json:"type"`
	QuoteContent string       `db:"quote_content" json:"quote_content,omitempty"`
	CreatedAt    time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at" json:"updated_at"`
	DeletedAt    *time.Time   `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewRetweet creates a new retweet with default type.
func NewRetweet(tweetID, userID string) (*Retweet, error) {
	return NewRetweetWithType(tweetID, userID, RetweetTypeRegular, "")
}

// NewQuoteRetweet creates a new quote retweet.
func NewQuoteRetweet(tweetID, userID, quoteContent string) (*Retweet, error) {
	if strings.TrimSpace(quoteContent) == "" {
		return nil, ErrRetweetQuoteEmpty
	}
	if len(quoteContent) > MaxTweetContentLength {
		return nil, ErrRetweetQuoteTooLong
	}
	return NewRetweetWithType(tweetID, userID, RetweetTypeQuote, quoteContent)
}

// NewRetweetWithType creates a new retweet with a specific type.
func NewRetweetWithType(tweetID, userID string, retweetType RetweetType, quoteContent string) (*Retweet, error) {
	if tweetID == "" {
		return nil, ErrRetweetTweetIDEmpty
	}
	if userID == "" {
		return nil, ErrRetweetUserIDEmpty
	}
	// For quote retweets, validate content
	if retweetType == RetweetTypeQuote {
		trimmed := strings.TrimSpace(quoteContent)
		if trimmed == "" {
			return nil, ErrRetweetQuoteEmpty
		}
		if len(trimmed) > MaxTweetContentLength {
			return nil, ErrRetweetQuoteTooLong
		}
		quoteContent = trimmed
	}
	r := &Retweet{
		ID:           uuid.New().String(),
		TweetID:      tweetID,
		UserID:       userID,
		Type:         retweetType,
		QuoteContent: quoteContent,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return r, nil
}

// MustNewRetweet creates a new retweet and panics on error.
func MustNewRetweet(tweetID, userID string) *Retweet {
	r, err := NewRetweet(tweetID, userID)
	if err != nil {
		panic(err)
	}
	return r
}

// MustNewQuoteRetweet creates a new quote retweet and panics on error.
func MustNewQuoteRetweet(tweetID, userID, quoteContent string) *Retweet {
	r, err := NewQuoteRetweet(tweetID, userID, quoteContent)
	if err != nil {
		panic(err)
	}
	return r
}

// ======================================================================
// Validation
// ======================================================================

// Validate performs comprehensive validation.
func (r *Retweet) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrRetweetIDEmpty
	}
	if strings.TrimSpace(r.TweetID) == "" {
		return ErrRetweetTweetIDEmpty
	}
	if strings.TrimSpace(r.UserID) == "" {
		return ErrRetweetUserIDEmpty
	}
	if !r.Type.IsValid() {
		return ErrInvalidRetweetType
	}
	if r.Type == RetweetTypeQuote {
		trimmed := strings.TrimSpace(r.QuoteContent)
		if trimmed == "" {
			return ErrRetweetQuoteEmpty
		}
		if len(trimmed) > MaxTweetContentLength {
			return ErrRetweetQuoteTooLong
		}
		r.QuoteContent = trimmed
	}
	if r.DeletedAt != nil {
		return ErrRetweetAlreadyDeleted
	}
	return nil
}

// ======================================================================
= Type Management
// ======================================================================

// IsRegular returns true if the retweet is regular.
func (r *Retweet) IsRegular() bool {
	return r.Type == RetweetTypeRegular
}

// IsQuote returns true if the retweet is a quote.
func (r *Retweet) IsQuote() bool {
	return r.Type == RetweetTypeQuote
}

// SetType sets the retweet type.
func (r *Retweet) SetType(retweetType RetweetType) error {
	if r.DeletedAt != nil {
		return ErrRetweetAlreadyDeleted
	}
	if !retweetType.IsValid() {
		return ErrInvalidRetweetType
	}
	r.Type = retweetType
	r.UpdatedAt = time.Now()
	return nil
}

// SetQuoteContent sets the quote content.
func (r *Retweet) SetQuoteContent(content string) error {
	if r.DeletedAt != nil {
		return ErrRetweetAlreadyDeleted
	}
	if r.Type != RetweetTypeQuote {
		return errors.New("only quote retweets can have content")
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ErrRetweetQuoteEmpty
	}
	if len(trimmed) > MaxTweetContentLength {
		return ErrRetweetQuoteTooLong
	}
	r.QuoteContent = trimmed
	r.UpdatedAt = time.Now()
	return nil
}

// ConvertToQuote converts a regular retweet to a quote retweet.
func (r *Retweet) ConvertToQuote(content string) error {
	if r.DeletedAt != nil {
		return ErrRetweetAlreadyDeleted
	}
	if r.IsQuote() {
		return errors.New("retweet is already a quote")
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ErrRetweetQuoteEmpty
	}
	if len(trimmed) > MaxTweetContentLength {
		return ErrRetweetQuoteTooLong
	}
	r.Type = RetweetTypeQuote
	r.QuoteContent = trimmed
	r.UpdatedAt = time.Now()
	return nil
}

// ConvertToRegular converts a quote retweet to a regular retweet.
func (r *Retweet) ConvertToRegular() error {
	if r.DeletedAt != nil {
		return ErrRetweetAlreadyDeleted
	}
	if r.IsRegular() {
		return errors.New("retweet is already regular")
	}
	r.Type = RetweetTypeRegular
	r.QuoteContent = ""
	r.UpdatedAt = time.Now()
	return nil
}

// ======================================================================
// Deletion Operations
// ======================================================================

// SoftDelete marks the retweet as deleted.
func (r *Retweet) SoftDelete() error {
	if r.DeletedAt != nil {
		return ErrRetweetAlreadyDeleted
	}
	now := time.Now()
	r.DeletedAt = &now
	r.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted retweet.
func (r *Retweet) Restore() error {
	if r.DeletedAt == nil {
		return ErrRetweetNotDeleted
	}
	r.DeletedAt = nil
	r.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the retweet is deleted.
func (r *Retweet) IsDeleted() bool {
	return r.DeletedAt != nil
}

// IsActive checks if the retweet is active (not deleted).
func (r *Retweet) IsActive() bool {
	return r.DeletedAt == nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// IsFromUser checks if the retweet is from a specific user.
func (r *Retweet) IsFromUser(userID string) bool {
	return r.UserID == userID
}

// IsOnTweet checks if the retweet is on a specific tweet.
func (r *Retweet) IsOnTweet(tweetID string) bool {
	return r.TweetID == tweetID
}

// HasQuoteContent checks if the retweet has quote content.
func (r *Retweet) HasQuoteContent() bool {
	return r.Type == RetweetTypeQuote && strings.TrimSpace(r.QuoteContent) != ""
}

// GetQuotePreview returns a preview of the quote content.
func (r *Retweet) GetQuotePreview(maxLen int) string {
	if !r.HasQuoteContent() {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 50
	}
	if len(r.QuoteContent) <= maxLen {
		return r.QuoteContent
	}
	return r.QuoteContent[:maxLen] + "..."
}

// String returns a human-readable representation.
func (r *Retweet) String() string {
	return fmt.Sprintf("Retweet{ID:%s, tweet:%s, user:%s, type:%s, created:%v}",
		r.ID, r.TweetID, r.UserID, r.Type, r.CreatedAt)
}

// Clone returns a deep copy of the retweet.
func (r *Retweet) Clone() *Retweet {
	clone := *r
	if r.DeletedAt != nil {
		t := *r.DeletedAt
		clone.DeletedAt = &t
	}
	return &clone
}

// Equals checks if two retweets are the same by ID.
func (r *Retweet) Equals(other *Retweet) bool {
	return r.ID == other.ID
}

// IsEmpty returns true if the retweet is zero value.
func (r *Retweet) IsEmpty() bool {
	return r.ID == "" && r.TweetID == "" && r.UserID == ""
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (r Retweet) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (r *Retweet) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for Retweet: %T", value)
	}
	return json.Unmarshal(bytes, r)
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *Retweet) MarshalJSON() ([]byte, error) {
	type Alias Retweet
	return json.Marshal(&struct {
		*Alias
		Type        string `json:"type"`
		IsActive    bool   `json:"is_active"`
		HasQuote    bool   `json:"has_quote"`
		QuotePreview string `json:"quote_preview,omitempty"`
	}{
		Alias:        (*Alias)(r),
		Type:         string(r.Type),
		IsActive:     r.IsActive(),
		HasQuote:     r.HasQuoteContent(),
		QuotePreview: r.GetQuotePreview(50),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *Retweet) UnmarshalJSON(data []byte) error {
	type Alias Retweet
	aux := &struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Type != "" {
		r.Type = RetweetType(aux.Type)
	}
	return nil
}

// ======================================================================
= Retweet Group (for batch operations)
// ======================================================================

// RetweetGroup represents a group of retweets.
type RetweetGroup struct {
	Retweets []*Retweet `json:"retweets"`
	Total    int64      `json:"total"`
}

// NewRetweetGroup creates a new retweet group.
func NewRetweetGroup() *RetweetGroup {
	return &RetweetGroup{
		Retweets: []*Retweet{},
		Total:    0,
	}
}

// Add adds a retweet to the group.
func (g *RetweetGroup) Add(r *Retweet) {
	g.Retweets = append(g.Retweets, r)
	g.Total++
}

// Contains checks if a retweet is in the group.
func (g *RetweetGroup) Contains(id string) bool {
	for _, r := range g.Retweets {
		if r.ID == id {
			return true
		}
	}
	return false
}

// FilterByUser returns retweets by a specific user.
func (g *RetweetGroup) FilterByUser(userID string) []*Retweet {
	result := []*Retweet{}
	for _, r := range g.Retweets {
		if r.UserID == userID {
			result = append(result, r)
		}
	}
	return result
}

// FilterByTweet returns retweets on a specific tweet.
func (g *RetweetGroup) FilterByTweet(tweetID string) []*Retweet {
	result := []*Retweet{}
	for _, r := range g.Retweets {
		if r.TweetID == tweetID {
			result = append(result, r)
		}
	}
	return result
}

// FilterByType returns retweets of a specific type.
func (g *RetweetGroup) FilterByType(retweetType RetweetType) []*Retweet {
	result := []*Retweet{}
	for _, r := range g.Retweets {
		if r.Type == retweetType {
			result = append(result, r)
		}
	}
	return result
}

// GetQuoteRetweets returns all quote retweets.
func (g *RetweetGroup) GetQuoteRetweets() []*Retweet {
	return g.FilterByType(RetweetTypeQuote)
}

// GetRegularRetweets returns all regular retweets.
func (g *RetweetGroup) GetRegularRetweets() []*Retweet {
	return g.FilterByType(RetweetTypeRegular)
}

// ======================================================================
= Retweet Statistics
// ======================================================================

// RetweetStats represents retweet statistics.
type RetweetStats struct {
	TotalRetweets    int64 `json:"total_retweets"`
	RegularRetweets  int64 `json:"regular_retweets"`
	QuoteRetweets    int64 `json:"quote_retweets"`
	UniqueUsers      int64 `json:"unique_users"`
}

// CalculateStats calculates statistics from a retweet group.
func (g *RetweetGroup) CalculateStats() *RetweetStats {
	stats := &RetweetStats{
		TotalRetweets:   int64(len(g.Retweets)),
		RegularRetweets: 0,
		QuoteRetweets:   0,
		UniqueUsers:     0,
	}
	users := make(map[string]bool)
	for _, r := range g.Retweets {
		users[r.UserID] = true
		if r.IsRegular() {
			stats.RegularRetweets++
		} else if r.IsQuote() {
			stats.QuoteRetweets++
		}
	}
	stats.UniqueUsers = int64(len(users))
	return stats
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// RetweetBuilder helps construct retweets for testing.
type RetweetBuilder struct {
	retweet *Retweet
}

// NewRetweetBuilder creates a new retweet builder.
func NewRetweetBuilder() *RetweetBuilder {
	return &RetweetBuilder{
		retweet: &Retweet{
			ID:           uuid.New().String(),
			TweetID:      "",
			UserID:       "",
			Type:         RetweetTypeRegular,
			QuoteContent: "",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *RetweetBuilder) WithID(id string) *RetweetBuilder {
	b.retweet.ID = id
	return b
}

// WithTweetID sets the tweet ID.
func (b *RetweetBuilder) WithTweetID(tweetID string) *RetweetBuilder {
	b.retweet.TweetID = tweetID
	return b
}

// WithUserID sets the user ID.
func (b *RetweetBuilder) WithUserID(userID string) *RetweetBuilder {
	b.retweet.UserID = userID
	return b
}

// WithType sets the retweet type.
func (b *RetweetBuilder) WithType(retweetType RetweetType) *RetweetBuilder {
	b.retweet.Type = retweetType
	return b
}

// WithQuoteContent sets the quote content.
func (b *RetweetBuilder) WithQuoteContent(content string) *RetweetBuilder {
	b.retweet.QuoteContent = content
	return b
}

// WithCreatedAt sets the creation time.
func (b *RetweetBuilder) WithCreatedAt(t time.Time) *RetweetBuilder {
	b.retweet.CreatedAt = t
	b.retweet.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *RetweetBuilder) WithDeleted(t time.Time) *RetweetBuilder {
	b.retweet.DeletedAt = &t
	return b
}

// Build validates and returns the retweet.
func (b *RetweetBuilder) Build() (*Retweet, error) {
	if err := b.retweet.Validate(); err != nil {
		return nil, err
	}
	return b.retweet, nil
}

// MustBuild builds without error (panics on error).
func (b *RetweetBuilder) MustBuild() *Retweet {
	r, err := b.Build()
	if err != nil {
		panic(err)
	}
	return r
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestRetweet1 = MustNewRetweet("tweet1", "user1")
	TestRetweet2 = MustNewRetweet("tweet1", "user2")
	TestRetweet3 = MustNewQuoteRetweet("tweet2", "user1", "Great tweet!")
)

// MustNewRetweetWithType creates a retweet with type and panics on error.
func MustNewRetweetWithType(tweetID, userID string, retweetType RetweetType, quoteContent string) *Retweet {
	r, err := NewRetweetWithType(tweetID, userID, retweetType, quoteContent)
	if err != nil {
		panic(err)
	}
	return r
}

// MustNewDeletedRetweet creates a deleted retweet for testing.
func MustNewDeletedRetweet(tweetID, userID string) *Retweet {
	r, err := NewRetweet(tweetID, userID)
	if err != nil {
		panic(err)
	}
	if err := r.SoftDelete(); err != nil {
		panic(err)
	}
	return r
}