// backend/internal/domain/events/tweet_created.go
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrTweetCreatedIDEmpty      = errors.New("tweet created event ID cannot be empty")
	ErrTweetCreatedUserIDEmpty  = errors.New("tweet created event user ID cannot be empty")
	ErrTweetCreatedTweetIDEmpty = errors.New("tweet created event tweet ID cannot be empty")
	ErrTweetCreatedContentEmpty = errors.New("tweet created event content cannot be empty")
	ErrTweetCreatedContentTooLong = errors.New("tweet created event content exceeds maximum length")
)

// ======================================================================
// TweetCreatedEvent
// ======================================================================

// TweetCreatedEvent represents the event when a tweet is created.
type TweetCreatedEvent struct {
	BaseEvent
	TweetID         string    `json:"tweet_id"`
	UserID          string    `json:"user_id"`
	Content         string    `json:"content"`
	MediaURLs       []string  `json:"media_urls,omitempty"`
	ParentTweetID   string    `json:"parent_tweet_id,omitempty"`
	RetweetOfID     string    `json:"retweet_of_id,omitempty"`
	IsPoll          bool      `json:"is_poll"`
	Mentions        []string  `json:"mentions,omitempty"`
	Hashtags        []string  `json:"hashtags,omitempty"`
	CharacterCount  int       `json:"character_count"`
	WordCount       int       `json:"word_count"`
	CreatedAt       time.Time `json:"created_at"`
}

// ======================================================================
// Constants
// ======================================================================

const (
	MaxTweetContentLength = 280
	MaxTweetMediaCount    = 4
)

// ======================================================================
// Factory Methods
// ======================================================================

// NewTweetCreatedEvent creates a new tweet created event.
func NewTweetCreatedEvent(tweetID, userID, content string, mediaURLs []string, parentTweetID, retweetOfID string, isPoll bool) (*TweetCreatedEvent, error) {
	if tweetID == "" {
		return nil, ErrTweetCreatedTweetIDEmpty
	}
	if userID == "" {
		return nil, ErrTweetCreatedUserIDEmpty
	}
	content = strings.TrimSpace(content)
	if content == "" && len(mediaURLs) == 0 {
		return nil, ErrTweetCreatedContentEmpty
	}
	if len(content) > MaxTweetContentLength {
		return nil, ErrTweetCreatedContentTooLong
	}
	if len(mediaURLs) > MaxTweetMediaCount {
		return nil, errors.New("maximum 4 media files allowed")
	}
	// Clean media URLs
	cleanedMedia := make([]string, 0, len(mediaURLs))
	for _, url := range mediaURLs {
		if trimmed := strings.TrimSpace(url); trimmed != "" {
			cleanedMedia = append(cleanedMedia, trimmed)
		}
	}
	// Extract mentions and hashtags
	mentions := extractMentions(content)
	hashtags := extractHashtags(content)
	// Count words
	wordCount := len(strings.Fields(content))
	// Create data for base event
	data := map[string]interface{}{
		"tweet_id":        tweetID,
		"user_id":         userID,
		"content":         content,
		"media_urls":      cleanedMedia,
		"parent_tweet_id": parentTweetID,
		"retweet_of_id":   retweetOfID,
		"is_poll":         isPoll,
		"mentions":        mentions,
		"hashtags":        hashtags,
		"character_count": len(content),
		"word_count":      wordCount,
	}
	base, err := NewBaseEvent(EventTypeTweetCreated, data)
	if err != nil {
		return nil, err
	}
	base.WithSource("tweet_service")
	return &TweetCreatedEvent{
		BaseEvent:      *base,
		TweetID:        tweetID,
		UserID:         userID,
		Content:        content,
		MediaURLs:      cleanedMedia,
		ParentTweetID:  parentTweetID,
		RetweetOfID:    retweetOfID,
		IsPoll:         isPoll,
		Mentions:       mentions,
		Hashtags:       hashtags,
		CharacterCount: len(content),
		WordCount:      wordCount,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// NewTweetCreatedEventWithTime creates a tweet created event with custom time.
func NewTweetCreatedEventWithTime(tweetID, userID, content string, mediaURLs []string, parentTweetID, retweetOfID string, isPoll bool, createdAt time.Time) (*TweetCreatedEvent, error) {
	event, err := NewTweetCreatedEvent(tweetID, userID, content, mediaURLs, parentTweetID, retweetOfID, isPoll)
	if err != nil {
		return nil, err
	}
	event.CreatedAt = createdAt
	return event, nil
}

// MustNewTweetCreatedEvent creates a tweet created event and panics on error.
func MustNewTweetCreatedEvent(tweetID, userID, content string, mediaURLs []string, parentTweetID, retweetOfID string, isPoll bool) *TweetCreatedEvent {
	event, err := NewTweetCreatedEvent(tweetID, userID, content, mediaURLs, parentTweetID, retweetOfID, isPoll)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// Validation
// ======================================================================

// Validate validates the tweet created event.
func (e *TweetCreatedEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	if e.TweetID == "" {
		return ErrTweetCreatedTweetIDEmpty
	}
	if e.UserID == "" {
		return ErrTweetCreatedUserIDEmpty
	}
	content := strings.TrimSpace(e.Content)
	if content == "" && len(e.MediaURLs) == 0 {
		return ErrTweetCreatedContentEmpty
	}
	if len(content) > MaxTweetContentLength {
		return ErrTweetCreatedContentTooLong
	}
	if len(e.MediaURLs) > MaxTweetMediaCount {
		return errors.New("maximum 4 media files allowed")
	}
	return nil
}

// ======================================================================
= Getters
// ======================================================================

// GetTweetID returns the tweet ID.
func (e *TweetCreatedEvent) GetTweetID() string {
	return e.TweetID
}

// GetUserID returns the user ID.
func (e *TweetCreatedEvent) GetUserID() string {
	return e.UserID
}

// GetContent returns the tweet content.
func (e *TweetCreatedEvent) GetContent() string {
	return e.Content
}

// GetMediaURLs returns the media URLs.
func (e *TweetCreatedEvent) GetMediaURLs() []string {
	return e.MediaURLs
}

// GetParentTweetID returns the parent tweet ID.
func (e *TweetCreatedEvent) GetParentTweetID() string {
	return e.ParentTweetID
}

// GetRetweetOfID returns the retweet of ID.
func (e *TweetCreatedEvent) GetRetweetOfID() string {
	return e.RetweetOfID
}

// IsPoll returns true if the tweet has a poll.
func (e *TweetCreatedEvent) IsPoll() bool {
	return e.IsPoll
}

// GetMentions returns the mentions.
func (e *TweetCreatedEvent) GetMentions() []string {
	return e.Mentions
}

// GetHashtags returns the hashtags.
func (e *TweetCreatedEvent) GetHashtags() []string {
	return e.Hashtags
}

// GetCharacterCount returns the character count.
func (e *TweetCreatedEvent) GetCharacterCount() int {
	return e.CharacterCount
}

// GetWordCount returns the word count.
func (e *TweetCreatedEvent) GetWordCount() int {
	return e.WordCount
}

// GetCreatedAt returns the creation time.
func (e *TweetCreatedEvent) GetCreatedAt() time.Time {
	return e.CreatedAt
}

// ======================================================================
// Helper Methods
// ======================================================================

// IsReply returns true if the tweet is a reply.
func (e *TweetCreatedEvent) IsReply() bool {
	return e.ParentTweetID != ""
}

// IsRetweet returns true if the tweet is a retweet.
func (e *TweetCreatedEvent) IsRetweet() bool {
	return e.RetweetOfID != ""
}

// IsQuote returns true if the tweet is a quote.
func (e *TweetCreatedEvent) IsQuote() bool {
	return e.IsRetweet() && e.Content != ""
}

// HasMedia returns true if the tweet has media.
func (e *TweetCreatedEvent) HasMedia() bool {
	return len(e.MediaURLs) > 0
}

// HasMentions returns true if the tweet has mentions.
func (e *TweetCreatedEvent) HasMentions() bool {
	return len(e.Mentions) > 0
}

// HasHashtags returns true if the tweet has hashtags.
func (e *TweetCreatedEvent) HasHashtags() bool {
	return len(e.Hashtags) > 0
}

// GetPreview returns a preview of the content.
func (e *TweetCreatedEvent) GetPreview(maxLen int) string {
	if maxLen <= 0 {
		maxLen = 50
	}
	if len(e.Content) <= maxLen {
		return e.Content
	}
	return e.Content[:maxLen] + "..."
}

// IsFromUser checks if the tweet is from a specific user.
func (e *TweetCreatedEvent) IsFromUser(userID string) bool {
	return e.UserID == userID
}

// IsOnTweet checks if the tweet is on a specific tweet.
func (e *TweetCreatedEvent) IsOnTweet(tweetID string) bool {
	return e.TweetID == tweetID
}

// ======================================================================
// String Representation
// ======================================================================

// String returns a string representation of the event.
func (e *TweetCreatedEvent) String() string {
	return fmt.Sprintf("TweetCreatedEvent{id:%s, tweet:%s, user:%s, content:%q, created:%s}",
		e.ID(), e.TweetID, e.UserID, e.GetPreview(30), e.CreatedAt.Format(time.RFC3339))
}

// ======================================================================
// Clone
// ======================================================================

// Clone creates a deep copy of the event.
func (e *TweetCreatedEvent) Clone() Event {
	clone := &TweetCreatedEvent{
		BaseEvent:      *e.BaseEvent.Clone().(*BaseEvent),
		TweetID:        e.TweetID,
		UserID:         e.UserID,
		Content:        e.Content,
		MediaURLs:      make([]string, len(e.MediaURLs)),
		ParentTweetID:  e.ParentTweetID,
		RetweetOfID:    e.RetweetOfID,
		IsPoll:         e.IsPoll,
		Mentions:       make([]string, len(e.Mentions)),
		Hashtags:       make([]string, len(e.Hashtags)),
		CharacterCount: e.CharacterCount,
		WordCount:      e.WordCount,
		CreatedAt:      e.CreatedAt,
	}
	copy(clone.MediaURLs, e.MediaURLs)
	copy(clone.Mentions, e.Mentions)
	copy(clone.Hashtags, e.Hashtags)
	return clone
}

// ======================================================================
// JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (e *TweetCreatedEvent) MarshalJSON() ([]byte, error) {
	type Alias TweetCreatedEvent
	return json.Marshal(&struct {
		*Alias
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
	}{
		Alias:     (*Alias)(e),
		EventID:   e.ID(),
		EventType: e.Type(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (e *TweetCreatedEvent) UnmarshalJSON(data []byte) error {
	type Alias TweetCreatedEvent
	aux := &struct {
		*Alias
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	return nil
}

// ======================================================================
// Helper Functions
// ======================================================================

// extractMentions extracts @mentions from content.
func extractMentions(content string) []string {
	re := regexp.MustCompile(`@(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	mentions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			mentions = append(mentions, match[1])
		}
	}
	return mentions
}

// extractHashtags extracts #hashtags from content.
func extractHashtags(content string) []string {
	re := regexp.MustCompile(`#(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	hashtags := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			hashtags = append(hashtags, match[1])
		}
	}
	return hashtags
}

// ======================================================================
// Builder Pattern
// ======================================================================

// TweetCreatedEventBuilder helps construct tweet created events for testing.
type TweetCreatedEventBuilder struct {
	event *TweetCreatedEvent
}

// NewTweetCreatedEventBuilder creates a new builder.
func NewTweetCreatedEventBuilder() *TweetCreatedEventBuilder {
	return &TweetCreatedEventBuilder{
		event: &TweetCreatedEvent{
			BaseEvent: BaseEvent{
				id:        uuid.New().String(),
				eventType: EventTypeTweetCreated,
				timestamp: time.Now().UTC(),
				source:    "test",
				data:      make(map[string]interface{}),
				priority:  PriorityNormal,
				version:   1,
				metadata:  make(map[string]interface{}),
			},
			TweetID:        "",
			UserID:         "",
			Content:        "",
			MediaURLs:      []string{},
			ParentTweetID:  "",
			RetweetOfID:    "",
			IsPoll:         false,
			Mentions:       []string{},
			Hashtags:       []string{},
			CharacterCount: 0,
			WordCount:      0,
			CreatedAt:      time.Now().UTC(),
		},
	}
}

// WithID sets the event ID.
func (b *TweetCreatedEventBuilder) WithID(id string) *TweetCreatedEventBuilder {
	b.event.id = id
	return b
}

// WithTweetID sets the tweet ID.
func (b *TweetCreatedEventBuilder) WithTweetID(tweetID string) *TweetCreatedEventBuilder {
	b.event.TweetID = tweetID
	return b
}

// WithUserID sets the user ID.
func (b *TweetCreatedEventBuilder) WithUserID(userID string) *TweetCreatedEventBuilder {
	b.event.UserID = userID
	return b
}

// WithContent sets the content.
func (b *TweetCreatedEventBuilder) WithContent(content string) *TweetCreatedEventBuilder {
	b.event.Content = content
	b.event.CharacterCount = len(content)
	b.event.WordCount = len(strings.Fields(content))
	b.event.Mentions = extractMentions(content)
	b.event.Hashtags = extractHashtags(content)
	return b
}

// WithMedia adds media URLs.
func (b *TweetCreatedEventBuilder) WithMedia(urls ...string) *TweetCreatedEventBuilder {
	b.event.MediaURLs = append(b.event.MediaURLs, urls...)
	return b
}

// WithParent sets the parent tweet ID.
func (b *TweetCreatedEventBuilder) WithParent(parentTweetID string) *TweetCreatedEventBuilder {
	b.event.ParentTweetID = parentTweetID
	return b
}

// WithRetweet sets the retweet of ID.
func (b *TweetCreatedEventBuilder) WithRetweet(retweetOfID string) *TweetCreatedEventBuilder {
	b.event.RetweetOfID = retweetOfID
	return b
}

// WithPoll marks as poll.
func (b *TweetCreatedEventBuilder) WithPoll(isPoll bool) *TweetCreatedEventBuilder {
	b.event.IsPoll = isPoll
	return b
}

// WithCreatedAt sets the creation time.
func (b *TweetCreatedEventBuilder) WithCreatedAt(t time.Time) *TweetCreatedEventBuilder {
	b.event.CreatedAt = t
	b.event.timestamp = t
	return b
}

// WithSource sets the event source.
func (b *TweetCreatedEventBuilder) WithSource(source string) *TweetCreatedEventBuilder {
	b.event.source = source
	return b
}

// WithMetadata adds metadata.
func (b *TweetCreatedEventBuilder) WithMetadata(key string, value interface{}) *TweetCreatedEventBuilder {
	b.event.metadata[key] = value
	return b
}

// Build validates and returns the event.
func (b *TweetCreatedEventBuilder) Build() (*TweetCreatedEvent, error) {
	if err := b.event.Validate(); err != nil {
		return nil, err
	}
	return b.event, nil
}

// MustBuild builds without error (panics on error).
func (b *TweetCreatedEventBuilder) MustBuild() *TweetCreatedEvent {
	e, err := b.Build()
	if err != nil {
		panic(err)
	}
	return e
}

// ======================================================================
// Test Helpers
// ======================================================================

var (
	TestTweetCreatedEvent1 = MustNewTweetCreatedEvent("tweet1", "user1", "Hello world", []string{}, "", "", false)
	TestTweetCreatedEvent2 = MustNewTweetCreatedEvent("tweet2", "user2", "Check out this #amazing tweet!", []string{"https://example.com/img.jpg"}, "", "", false)
	TestTweetCreatedEvent3 = MustNewTweetCreatedEvent("tweet3", "user1", "@user2 Hello there!", []string{}, "", "", false)
)

// MustNewTestTweetCreatedEvent creates a test event with default values.
func MustNewTestTweetCreatedEvent(tweetID, userID string) *TweetCreatedEvent {
	return MustNewTweetCreatedEvent(tweetID, userID, "Test tweet content", []string{}, "", "", false)
}

// MustNewTestReplyEvent creates a test reply event.
func MustNewTestReplyEvent(tweetID, userID, parentTweetID string) *TweetCreatedEvent {
	return MustNewTweetCreatedEvent(tweetID, userID, "This is a reply", []string{}, parentTweetID, "", false)
}

// MustNewTestRetweetEvent creates a test retweet event.
func MustNewTestRetweetEvent(tweetID, userID, retweetOfID string) *TweetCreatedEvent {
	return MustNewTweetCreatedEvent(tweetID, userID, "", []string{}, "", retweetOfID, false)
}