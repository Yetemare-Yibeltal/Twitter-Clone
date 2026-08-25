// backend/internal/domain/entities/tweet.go
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

const (
	MaxTweetContentLength = 280
	MaxMediaCount         = 4
	MaxMediaURLSize       = 2048
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrEmptyTweetContent      = errors.New("tweet content cannot be empty")
	ErrTweetContentTooLong    = fmt.Errorf("tweet content exceeds maximum length of %d characters", MaxTweetContentLength)
	ErrTweetMediaTooMany      = fmt.Errorf("maximum %d media files allowed", MaxMediaCount)
	ErrTweetMediaURLInvalid   = errors.New("invalid media URL")
	ErrTweetIDEmpty           = errors.New("tweet ID cannot be empty")
	ErrTweetUserIDEmpty       = errors.New("user ID cannot be empty")
	ErrTweetCannotRetweetSelf = errors.New("cannot retweet your own tweet")
	ErrTweetCannotQuoteSelf   = errors.New("cannot quote your own tweet")
	ErrTweetAlreadyDeleted    = errors.New("tweet already deleted")
	ErrTweetNotDeleted        = errors.New("tweet is not deleted")
	ErrTweetInvalidParent     = errors.New("invalid parent tweet ID")
	ErrTweetInvalidRetweetOf  = errors.New("invalid retweet-of ID")
)

// ======================================================================
= Tweet Entity
// ======================================================================

// Tweet represents a tweet in the system.
type Tweet struct {
	// Primary identifiers
	ID     string `db:"id" json:"id"`
	UserID string `db:"user_id" json:"user_id"`

	// Content
	Content   string   `db:"content" json:"content"`
	MediaURLs []string `db:"media_urls" json:"media_urls,omitempty"`

	// Relationship
	ParentTweetID *string `db:"parent_tweet_id" json:"parent_tweet_id,omitempty"` // reply to
	RetweetOfID   *string `db:"retweet_of_id" json:"retweet_of_id,omitempty"`     // retweet of

	// Features
	IsPoll bool `db:"is_poll" json:"is_poll"`

	// Timestamps
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ======================================================================
= Factory Methods
// ======================================================================

// NewTweet creates a new tweet instance with validation.
func NewTweet(userID, content string, mediaURLs []string, parentID, retweetOfID *string, isPoll bool) (*Tweet, error) {
	t := &Tweet{
		ID:            uuid.New().String(),
		UserID:        userID,
		Content:       content,
		MediaURLs:     mediaURLs,
		ParentTweetID: parentID,
		RetweetOfID:   retweetOfID,
		IsPoll:        isPoll,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t, nil
}

// MustNewTweet creates a new tweet and panics on error.
func MustNewTweet(userID, content string, mediaURLs []string, parentID, retweetOfID *string, isPoll bool) *Tweet {
	t, err := NewTweet(userID, content, mediaURLs, parentID, retweetOfID, isPoll)
	if err != nil {
		panic(err)
	}
	return t
}

// ======================================================================
= Validation
// ======================================================================

// Validate performs comprehensive validation.
func (t *Tweet) Validate() error {
	// ID validation
	if strings.TrimSpace(t.ID) == "" {
		return ErrTweetIDEmpty
	}
	if strings.TrimSpace(t.UserID) == "" {
		return ErrTweetUserIDEmpty
	}

	// Content validation
	contentTrimmed := strings.TrimSpace(t.Content)
	if contentTrimmed == "" && len(t.MediaURLs) == 0 && t.RetweetOfID == nil && !t.IsPoll {
		return ErrEmptyTweetContent
	}
	if len(contentTrimmed) > MaxTweetContentLength {
		return ErrTweetContentTooLong
	}
	t.Content = contentTrimmed // store trimmed content

	// Media validation
	if len(t.MediaURLs) > MaxMediaCount {
		return ErrTweetMediaTooMany
	}
	for _, url := range t.MediaURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			return ErrTweetMediaURLInvalid
		}
		if !isValidTweetMediaURL(url) {
			return ErrTweetMediaURLInvalid
		}
	}
	// Clean media URLs
	cleaned := make([]string, 0, len(t.MediaURLs))
	for _, url := range t.MediaURLs {
		if trimmed := strings.TrimSpace(url); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	t.MediaURLs = cleaned

	// Parent/Retweet validation
	if t.ParentTweetID != nil && *t.ParentTweetID == "" {
		return ErrTweetInvalidParent
	}
	if t.RetweetOfID != nil && *t.RetweetOfID == "" {
		return ErrTweetInvalidRetweetOf
	}
	if t.ParentTweetID != nil && t.RetweetOfID != nil {
		return errors.New("tweet cannot be both a reply and a retweet")
	}
	if t.RetweetOfID != nil && t.UserID == *t.RetweetOfID {
		return ErrTweetCannotRetweetSelf
	}
	// Note: quote check is done at service level since we don't have the original tweet here.

	return nil
}

// isValidTweetMediaURL validates media URL format.
func isValidTweetMediaURL(url string) bool {
	if len(url) > MaxMediaURLSize {
		return false
	}
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

// ======================================================================
= Business Logic Methods
// ======================================================================

// EditContent updates the tweet content.
func (t *Tweet) EditContent(newContent string) error {
	if t.DeletedAt != nil {
		return ErrTweetAlreadyDeleted
	}
	oldContent := t.Content
	t.Content = strings.TrimSpace(newContent)
	if err := t.Validate(); err != nil {
		t.Content = oldContent
		return err
	}
	t.UpdatedAt = time.Now()
	return nil
}

// AddMedia adds media URLs to the tweet.
func (t *Tweet) AddMedia(urls ...string) error {
	if t.DeletedAt != nil {
		return ErrTweetAlreadyDeleted
	}
	if len(t.MediaURLs)+len(urls) > MaxMediaCount {
		return ErrTweetMediaTooMany
	}
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" || !isValidTweetMediaURL(url) {
			return ErrTweetMediaURLInvalid
		}
		t.MediaURLs = append(t.MediaURLs, url)
	}
	t.UpdatedAt = time.Now()
	return nil
}

// RemoveMedia removes a media URL by index.
func (t *Tweet) RemoveMedia(index int) error {
	if t.DeletedAt != nil {
		return ErrTweetAlreadyDeleted
	}
	if index < 0 || index >= len(t.MediaURLs) {
		return errors.New("invalid media index")
	}
	t.MediaURLs = append(t.MediaURLs[:index], t.MediaURLs[index+1:]...)
	t.UpdatedAt = time.Now()
	return nil
}

// SoftDelete marks the tweet as deleted.
func (t *Tweet) SoftDelete() error {
	if t.DeletedAt != nil {
		return ErrTweetAlreadyDeleted
	}
	now := time.Now()
	t.DeletedAt = &now
	t.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted tweet.
func (t *Tweet) Restore() error {
	if t.DeletedAt == nil {
		return ErrTweetNotDeleted
	}
	t.DeletedAt = nil
	t.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the tweet is deleted.
func (t *Tweet) IsDeleted() bool {
	return t.DeletedAt != nil
}

// IsReply checks if the tweet is a reply.
func (t *Tweet) IsReply() bool {
	return t.ParentTweetID != nil && *t.ParentTweetID != ""
}

// IsRetweet checks if the tweet is a retweet.
func (t *Tweet) IsRetweet() bool {
	return t.RetweetOfID != nil && *t.RetweetOfID != ""
}

// IsQuote checks if the tweet is a quote (retweet with content).
func (t *Tweet) IsQuote() bool {
	return t.IsRetweet() && t.Content != ""
}

// HasMedia checks if the tweet has media.
func (t *Tweet) HasMedia() bool {
	return len(t.MediaURLs) > 0
}

// ======================================================================
= Helper Methods
// ======================================================================

// Preview returns a shortened preview of the content.
func (t *Tweet) Preview(maxLen int) string {
	if maxLen <= 0 {
		maxLen = 50
	}
	if len(t.Content) <= maxLen {
		return t.Content
	}
	return t.Content[:maxLen] + "..."
}

// String returns a human-readable representation.
func (t *Tweet) String() string {
	return fmt.Sprintf("Tweet{ID:%s, user:%s, content:%q, created:%v}", t.ID, t.UserID, t.Content, t.CreatedAt)
}

// Clone returns a deep copy of the tweet.
func (t *Tweet) Clone() *Tweet {
	clone := *t
	if t.ParentTweetID != nil {
		val := *t.ParentTweetID
		clone.ParentTweetID = &val
	}
	if t.RetweetOfID != nil {
		val := *t.RetweetOfID
		clone.RetweetOfID = &val
	}
	if t.DeletedAt != nil {
		val := *t.DeletedAt
		clone.DeletedAt = &val
	}
	clone.MediaURLs = make([]string, len(t.MediaURLs))
	copy(clone.MediaURLs, t.MediaURLs)
	return &clone
}

// Equals checks if two tweets have the same ID.
func (t *Tweet) Equals(other *Tweet) bool {
	return t.ID == other.ID
}

// IsEmpty returns true if the tweet is zero value.
func (t *Tweet) IsEmpty() bool {
	return t.ID == "" && t.UserID == "" && t.Content == ""
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage of MediaURLs (optional).
func (t Tweet) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (t *Tweet) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for Tweet: %T", value)
	}
	return json.Unmarshal(bytes, t)
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling (omit deleted fields).
func (t *Tweet) MarshalJSON() ([]byte, error) {
	type Alias Tweet
	return json.Marshal(&struct {
		*Alias
		DeletedAt *time.Time `json:"deleted_at,omitempty"`
	}{
		Alias:     (*Alias)(t),
		DeletedAt: t.DeletedAt,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (t *Tweet) UnmarshalJSON(data []byte) error {
	type Alias Tweet
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	return json.Unmarshal(data, aux)
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// TweetBuilder helps construct tweets for testing.
type TweetBuilder struct {
	tweet *Tweet
}

// NewTweetBuilder creates a new tweet builder.
func NewTweetBuilder() *TweetBuilder {
	return &TweetBuilder{
		tweet: &Tweet{
			ID:            uuid.New().String(),
			UserID:        "",
			Content:       "",
			MediaURLs:     []string{},
			ParentTweetID: nil,
			RetweetOfID:   nil,
			IsPoll:        false,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *TweetBuilder) WithID(id string) *TweetBuilder {
	b.tweet.ID = id
	return b
}

// WithUserID sets the user ID.
func (b *TweetBuilder) WithUserID(userID string) *TweetBuilder {
	b.tweet.UserID = userID
	return b
}

// WithContent sets the content.
func (b *TweetBuilder) WithContent(content string) *TweetBuilder {
	b.tweet.Content = content
	return b
}

// WithMedia adds media URLs.
func (b *TweetBuilder) WithMedia(urls ...string) *TweetBuilder {
	b.tweet.MediaURLs = append(b.tweet.MediaURLs, urls...)
	return b
}

// WithParent sets the parent tweet ID.
func (b *TweetBuilder) WithParent(parentID string) *TweetBuilder {
	b.tweet.ParentTweetID = &parentID
	return b
}

// WithRetweet sets the retweet-of ID.
func (b *TweetBuilder) WithRetweet(retweetOfID string) *TweetBuilder {
	b.tweet.RetweetOfID = &retweetOfID
	return b
}

// WithPoll marks as poll.
func (b *TweetBuilder) WithPoll(isPoll bool) *TweetBuilder {
	b.tweet.IsPoll = isPoll
	return b
}

// WithCreatedAt sets the creation time.
func (b *TweetBuilder) WithCreatedAt(t time.Time) *TweetBuilder {
	b.tweet.CreatedAt = t
	b.tweet.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *TweetBuilder) WithDeleted(t time.Time) *TweetBuilder {
	b.tweet.DeletedAt = &t
	return b
}

// Build validates and returns the tweet.
func (b *TweetBuilder) Build() (*Tweet, error) {
	if err := b.tweet.Validate(); err != nil {
		return nil, err
	}
	return b.tweet, nil
}

// MustBuild builds without error (panics on error).
func (b *TweetBuilder) MustBuild() *Tweet {
	t, err := b.Build()
	if err != nil {
		panic(err)
	}
	return t
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestTweet1 = MustNewTweet("user1", "Hello world", []string{}, nil, nil, false)
	TestTweet2 = MustNewTweet("user2", "This is a reply", []string{}, ptr("tweet1"), nil, false)
	TestTweet3 = MustNewTweet("user1", "Check out this!", []string{"https://example.com/img.jpg"}, nil, ptr("tweet2"), false)
)

func ptr(s string) *string {
	return &s
}