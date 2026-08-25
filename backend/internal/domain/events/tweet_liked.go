// backend/internal/domain/events/tweet_liked.go
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
	ErrTweetLikedIDEmpty      = errors.New("tweet liked event ID cannot be empty")
	ErrTweetLikedUserIDEmpty  = errors.New("tweet liked event user ID cannot be empty")
	ErrTweetLikedTweetIDEmpty = errors.New("tweet liked event tweet ID cannot be empty")
	ErrTweetLikedInvalidType  = errors.New("invalid tweet liked event type")
	ErrTweetLikedAlreadyLiked = errors.New("tweet already liked by this user")
	ErrTweetLikedNotFound     = errors.New("like not found for this user")
)

// ======================================================================
// LikeType
// ======================================================================

// LikeType represents the type of like.
type LikeType string

const (
	LikeTypeRegular LikeType = "regular"
	LikeTypeSuper   LikeType = "super"
)

// ValidLikeTypes returns all valid like types.
func ValidLikeTypes() []LikeType {
	return []LikeType{LikeTypeRegular, LikeTypeSuper}
}

// IsValid checks if a like type is valid.
func (t LikeType) IsValid() bool {
	for _, typ := range ValidLikeTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation of the like type.
func (t LikeType) String() string {
	return string(t)
}

// ======================================================================
// TweetLikedEvent
// ======================================================================

// TweetLikedEvent represents the event when a tweet is liked.
type TweetLikedEvent struct {
	BaseEvent
	TweetID      string    `json:"tweet_id"`
	UserID       string    `json:"user_id"`
	LikeID       string    `json:"like_id"`
	LikeType     LikeType  `json:"like_type"`
	IsUndo       bool      `json:"is_undo"` // true if this is an unlike event
	LikedAt      time.Time `json:"liked_at"`
	TweetUserID  string    `json:"tweet_user_id"` // owner of the liked tweet
	Count        int64     `json:"count"`         // total likes after this event
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewTweetLikedEvent creates a new tweet liked event.
func NewTweetLikedEvent(tweetID, userID, likeID string, likeType LikeType, tweetUserID string, count int64) (*TweetLikedEvent, error) {
	if tweetID == "" {
		return nil, ErrTweetLikedTweetIDEmpty
	}
	if userID == "" {
		return nil, ErrTweetLikedUserIDEmpty
	}
	if likeID == "" {
		return nil, errors.New("like ID cannot be empty")
	}
	if !likeType.IsValid() {
		return nil, ErrTweetLikedInvalidType
	}
	if tweetUserID == "" {
		return nil, errors.New("tweet user ID cannot be empty")
	}
	if count < 0 {
		return nil, errors.New("count cannot be negative")
	}
	data := map[string]interface{}{
		"tweet_id":      tweetID,
		"user_id":       userID,
		"like_id":       likeID,
		"like_type":     string(likeType),
		"is_undo":       false,
		"tweet_user_id": tweetUserID,
		"count":         count,
	}
	base, err := NewBaseEvent(EventTypeTweetLiked, data)
	if err != nil {
		return nil, err
	}
	base.WithSource("like_service")
	base.WithMetadata("action", "like")
	base.WithMetadata("timestamp", time.Now().UTC().Unix())
	return &TweetLikedEvent{
		BaseEvent:   *base,
		TweetID:     tweetID,
		UserID:      userID,
		LikeID:      likeID,
		LikeType:    likeType,
		IsUndo:      false,
		LikedAt:     time.Now().UTC(),
		TweetUserID: tweetUserID,
		Count:       count,
	}, nil
}

// NewTweetUnlikedEvent creates a new tweet unliked event.
func NewTweetUnlikedEvent(tweetID, userID, likeID string, tweetUserID string, count int64) (*TweetLikedEvent, error) {
	if tweetID == "" {
		return nil, ErrTweetLikedTweetIDEmpty
	}
	if userID == "" {
		return nil, ErrTweetLikedUserIDEmpty
	}
	if likeID == "" {
		return nil, errors.New("like ID cannot be empty")
	}
	if tweetUserID == "" {
		return nil, errors.New("tweet user ID cannot be empty")
	}
	if count < 0 {
		return nil, errors.New("count cannot be negative")
	}
	data := map[string]interface{}{
		"tweet_id":      tweetID,
		"user_id":       userID,
		"like_id":       likeID,
		"is_undo":       true,
		"tweet_user_id": tweetUserID,
		"count":         count,
	}
	base, err := NewBaseEvent(EventTypeTweetLiked, data)
	if err != nil {
		return nil, err
	}
	base.WithSource("like_service")
	base.WithMetadata("action", "unlike")
	base.WithMetadata("timestamp", time.Now().UTC().Unix())
	return &TweetLikedEvent{
		BaseEvent:   *base,
		TweetID:     tweetID,
		UserID:      userID,
		LikeID:      likeID,
		LikeType:    LikeTypeRegular,
		IsUndo:      true,
		LikedAt:     time.Now().UTC(),
		TweetUserID: tweetUserID,
		Count:       count,
	}, nil
}

// NewTweetLikedEventWithTime creates a tweet liked event with custom time.
func NewTweetLikedEventWithTime(tweetID, userID, likeID string, likeType LikeType, tweetUserID string, count int64, likedAt time.Time) (*TweetLikedEvent, error) {
	event, err := NewTweetLikedEvent(tweetID, userID, likeID, likeType, tweetUserID, count)
	if err != nil {
		return nil, err
	}
	event.LikedAt = likedAt
	return event, nil
}

// MustNewTweetLikedEvent creates a tweet liked event and panics on error.
func MustNewTweetLikedEvent(tweetID, userID, likeID string, likeType LikeType, tweetUserID string, count int64) *TweetLikedEvent {
	event, err := NewTweetLikedEvent(tweetID, userID, likeID, likeType, tweetUserID, count)
	if err != nil {
		panic(err)
	}
	return event
}

// MustNewTweetUnlikedEvent creates a tweet unliked event and panics on error.
func MustNewTweetUnlikedEvent(tweetID, userID, likeID string, tweetUserID string, count int64) *TweetLikedEvent {
	event, err := NewTweetUnlikedEvent(tweetID, userID, likeID, tweetUserID, count)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// Validation
// ======================================================================

// Validate validates the tweet liked event.
func (e *TweetLikedEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	if e.TweetID == "" {
		return ErrTweetLikedTweetIDEmpty
	}
	if e.UserID == "" {
		return ErrTweetLikedUserIDEmpty
	}
	if e.LikeID == "" {
		return errors.New("like ID cannot be empty")
	}
	if !e.IsUndo && !e.LikeType.IsValid() {
		return ErrTweetLikedInvalidType
	}
	if e.TweetUserID == "" {
		return errors.New("tweet user ID cannot be empty")
	}
	if e.Count < 0 {
		return errors.New("count cannot be negative")
	}
	return nil
}

// ======================================================================
// Getters
// ======================================================================

// GetTweetID returns the tweet ID.
func (e *TweetLikedEvent) GetTweetID() string {
	return e.TweetID
}

// GetUserID returns the user ID.
func (e *TweetLikedEvent) GetUserID() string {
	return e.UserID
}

// GetLikeID returns the like ID.
func (e *TweetLikedEvent) GetLikeID() string {
	return e.LikeID
}

// GetLikeType returns the like type.
func (e *TweetLikedEvent) GetLikeType() LikeType {
	return e.LikeType
}

// GetTweetUserID returns the tweet owner's user ID.
func (e *TweetLikedEvent) GetTweetUserID() string {
	return e.TweetUserID
}

// GetCount returns the total likes count.
func (e *TweetLikedEvent) GetCount() int64 {
	return e.Count
}

// GetLikedAt returns the like timestamp.
func (e *TweetLikedEvent) GetLikedAt() time.Time {
	return e.LikedAt
}

// IsLike returns true if this is a like event.
func (e *TweetLikedEvent) IsLike() bool {
	return !e.IsUndo
}

// IsUnlike returns true if this is an unlike event.
func (e *TweetLikedEvent) IsUnlike() bool {
	return e.IsUndo
}

// IsSuperLike returns true if this is a super like.
func (e *TweetLikedEvent) IsSuperLike() bool {
	return e.LikeType == LikeTypeSuper && !e.IsUndo
}

// IsRegularLike returns true if this is a regular like.
func (e *TweetLikedEvent) IsRegularLike() bool {
	return e.LikeType == LikeTypeRegular && !e.IsUndo
}

// ======================================================================
// Helper Methods
// ======================================================================

// IsFromUser checks if the like is from a specific user.
func (e *TweetLikedEvent) IsFromUser(userID string) bool {
	return e.UserID == userID
}

// IsOnTweet checks if the like is on a specific tweet.
func (e *TweetLikedEvent) IsOnTweet(tweetID string) bool {
	return e.TweetID == tweetID
}

// IsForTweetOwner checks if the like is for the tweet owner.
func (e *TweetLikedEvent) IsForTweetOwner() bool {
	return e.UserID != e.TweetUserID
}

// IsSelfLike checks if a user liked their own tweet.
func (e *TweetLikedEvent) IsSelfLike() bool {
	return e.UserID == e.TweetUserID && !e.IsUndo
}

// String returns a string representation of the event.
func (e *TweetLikedEvent) String() string {
	action := "liked"
	if e.IsUndo {
		action = "unliked"
	}
	return fmt.Sprintf("TweetLikedEvent{id:%s, tweet:%s, user:%s, action:%s, type:%s, count:%d, time:%s}",
		e.ID(), e.TweetID, e.UserID, action, e.LikeType, e.Count, e.LikedAt.Format(time.RFC3339))
}

// Clone creates a deep copy of the event.
func (e *TweetLikedEvent) Clone() Event {
	clone := &TweetLikedEvent{
		BaseEvent:   *e.BaseEvent.Clone().(*BaseEvent),
		TweetID:     e.TweetID,
		UserID:      e.UserID,
		LikeID:      e.LikeID,
		LikeType:    e.LikeType,
		IsUndo:      e.IsUndo,
		LikedAt:     e.LikedAt,
		TweetUserID: e.TweetUserID,
		Count:       e.Count,
	}
	return clone
}

// ======================================================================
// JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (e *TweetLikedEvent) MarshalJSON() ([]byte, error) {
	type Alias TweetLikedEvent
	return json.Marshal(&struct {
		*Alias
		EventID      string `json:"event_id"`
		EventType    string `json:"event_type"`
		LikeTypeStr  string `json:"like_type"`
		IsLikeAction bool   `json:"is_like"`
	}{
		Alias:        (*Alias)(e),
		EventID:      e.ID(),
		EventType:    e.Type(),
		LikeTypeStr:  string(e.LikeType),
		IsLikeAction: e.IsLike(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (e *TweetLikedEvent) UnmarshalJSON(data []byte) error {
	type Alias TweetLikedEvent
	aux := &struct {
		*Alias
		EventID      string `json:"event_id"`
		EventType    string `json:"event_type"`
		LikeTypeStr  string `json:"like_type"`
		IsLikeAction bool   `json:"is_like"`
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.LikeTypeStr != "" {
		e.LikeType = LikeType(aux.LikeTypeStr)
	}
	return nil
}

// ======================================================================
// Event Handlers (convenience)
// ======================================================================

// TweetLikedEventHandler is a convenience handler for tweet liked events.
type TweetLikedEventHandler struct {
	handler func(event *TweetLikedEvent) error
	name    string
}

// NewTweetLikedEventHandler creates a new tweet liked event handler.
func NewTweetLikedEventHandler(name string, handler func(event *TweetLikedEvent) error) *TweetLikedEventHandler {
	if name == "" {
		name = "TweetLikedEventHandler"
	}
	return &TweetLikedEventHandler{
		handler: handler,
		name:    name,
	}
}

// Handle processes the event.
func (h *TweetLikedEventHandler) Handle(event Event) error {
	tweetLikedEvent, ok := event.(*TweetLikedEvent)
	if !ok {
		return errors.New("event is not a TweetLikedEvent")
	}
	return h.handler(tweetLikedEvent)
}

// Handles returns the event types this handler handles.
func (h *TweetLikedEventHandler) Handles() []string {
	return []string{EventTypeTweetLiked}
}

// Priority returns the handler priority.
func (h *TweetLikedEventHandler) Priority() int {
	return 1
}

// Name returns the handler name.
func (h *TweetLikedEventHandler) Name() string {
	return h.name
}

// ======================================================================
// Builder Pattern
// ======================================================================

// TweetLikedEventBuilder helps construct tweet liked events for testing.
type TweetLikedEventBuilder struct {
	event *TweetLikedEvent
}

// NewTweetLikedEventBuilder creates a new builder.
func NewTweetLikedEventBuilder() *TweetLikedEventBuilder {
	return &TweetLikedEventBuilder{
		event: &TweetLikedEvent{
			BaseEvent: BaseEvent{
				id:        uuid.New().String(),
				eventType: EventTypeTweetLiked,
				timestamp: time.Now().UTC(),
				source:    "test",
				data:      make(map[string]interface{}),
				priority:  PriorityNormal,
				version:   1,
				metadata:  make(map[string]interface{}),
			},
			TweetID:     "",
			UserID:      "",
			LikeID:      "",
			LikeType:    LikeTypeRegular,
			IsUndo:      false,
			LikedAt:     time.Now().UTC(),
			TweetUserID: "",
			Count:       1,
		},
	}
}

// WithID sets the event ID.
func (b *TweetLikedEventBuilder) WithID(id string) *TweetLikedEventBuilder {
	b.event.id = id
	return b
}

// WithTweetID sets the tweet ID.
func (b *TweetLikedEventBuilder) WithTweetID(tweetID string) *TweetLikedEventBuilder {
	b.event.TweetID = tweetID
	return b
}

// WithUserID sets the user ID.
func (b *TweetLikedEventBuilder) WithUserID(userID string) *TweetLikedEventBuilder {
	b.event.UserID = userID
	return b
}

// WithLikeID sets the like ID.
func (b *TweetLikedEventBuilder) WithLikeID(likeID string) *TweetLikedEventBuilder {
	b.event.LikeID = likeID
	return b
}

// WithLikeType sets the like type.
func (b *TweetLikedEventBuilder) WithLikeType(likeType LikeType) *TweetLikedEventBuilder {
	b.event.LikeType = likeType
	return b
}

// WithUndo sets the undo flag.
func (b *TweetLikedEventBuilder) WithUndo(undo bool) *TweetLikedEventBuilder {
	b.event.IsUndo = undo
	return b
}

// WithTweetUserID sets the tweet owner ID.
func (b *TweetLikedEventBuilder) WithTweetUserID(tweetUserID string) *TweetLikedEventBuilder {
	b.event.TweetUserID = tweetUserID
	return b
}

// WithCount sets the total likes count.
func (b *TweetLikedEventBuilder) WithCount(count int64) *TweetLikedEventBuilder {
	b.event.Count = count
	return b
}

// WithLikedAt sets the like timestamp.
func (b *TweetLikedEventBuilder) WithLikedAt(t time.Time) *TweetLikedEventBuilder {
	b.event.LikedAt = t
	b.event.timestamp = t
	return b
}

// WithSource sets the event source.
func (b *TweetLikedEventBuilder) WithSource(source string) *TweetLikedEventBuilder {
	b.event.source = source
	return b
}

// WithMetadata adds metadata.
func (b *TweetLikedEventBuilder) WithMetadata(key string, value interface{}) *TweetLikedEventBuilder {
	b.event.metadata[key] = value
	return b
}

// Build validates and returns the event.
func (b *TweetLikedEventBuilder) Build() (*TweetLikedEvent, error) {
	if err := b.event.Validate(); err != nil {
		return nil, err
	}
	return b.event, nil
}

// MustBuild builds without error (panics on error).
func (b *TweetLikedEventBuilder) MustBuild() *TweetLikedEvent {
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
	TestTweetLikedEvent1 = MustNewTweetLikedEvent("tweet1", "user1", "like1", LikeTypeRegular, "user2", 5)
	TestTweetLikedEvent2 = MustNewTweetLikedEvent("tweet1", "user2", "like2", LikeTypeSuper, "user2", 6)
	TestTweetUnlikedEvent1 = MustNewTweetUnlikedEvent("tweet1", "user1", "like1", "user2", 4)
)

// MustNewTestTweetLikedEvent creates a test event with defaults.
func MustNewTestTweetLikedEvent(tweetID, userID string) *TweetLikedEvent {
	return MustNewTweetLikedEvent(tweetID, userID, uuid.New().String(), LikeTypeRegular, "other_user", 1)
}

// MustNewTestTweetUnlikedEvent creates a test unlike event.
func MustNewTestTweetUnlikedEvent(tweetID, userID string) *TweetLikedEvent {
	return MustNewTweetUnlikedEvent(tweetID, userID, uuid.New().String(), "other_user", 0)
}

// MustNewTestSuperLikeEvent creates a test super like event.
func MustNewTestSuperLikeEvent(tweetID, userID, tweetUserID string) *TweetLikedEvent {
	return MustNewTweetLikedEvent(tweetID, userID, uuid.New().String(), LikeTypeSuper, tweetUserID, 1)
}