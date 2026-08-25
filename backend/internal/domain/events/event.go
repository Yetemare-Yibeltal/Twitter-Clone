// backend/internal/domain/events/event.go
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ======================================================================
// Constants
// ======================================================================

// Event types
const (
	EventTypeTweetCreated   = "tweet.created"
	EventTypeTweetUpdated   = "tweet.updated"
	EventTypeTweetDeleted   = "tweet.deleted"
	EventTypeTweetLiked     = "tweet.liked"
	EventTypeTweetUnliked   = "tweet.unliked"
	EventTypeTweetRetweeted = "tweet.retweeted"
	EventTypeTweetUnretweeted = "tweet.unretweeted"
	EventTypeTweetBookmarked = "tweet.bookmarked"
	EventTypeTweetUnbookmarked = "tweet.unbookmarked"
	EventTypeTweetQuoted    = "tweet.quoted"
	EventTypeTweetReplied   = "tweet.replied"
	EventTypeTweetReported  = "tweet.reported"
	EventTypeTweetMentioned = "tweet.mentioned"

	EventTypeUserFollowed   = "user.followed"
	EventTypeUserUnfollowed = "user.unfollowed"
	EventTypeUserUpdated    = "user.updated"
	EventTypeUserDeleted    = "user.deleted"
	EventTypeUserVerified   = "user.verified"
	EventTypeUserSuspended  = "user.suspended"
	EventTypeUserActivated  = "user.activated"
	EventTypeUserLoggedIn   = "user.logged_in"
	EventTypeUserLoggedOut  = "user.logged_out"
	EventTypeUserRegistered = "user.registered"
	EventTypeUserPasswordChanged = "user.password_changed"

	EventTypeCommunityCreated = "community.created"
	EventTypeCommunityUpdated = "community.updated"
	EventTypeCommunityDeleted = "community.deleted"
	EventTypeCommunityJoined  = "community.joined"
	EventTypeCommunityLeft    = "community.left"
	EventTypeCommunityMemberRoleChanged = "community.member_role_changed"
	EventTypeCommunityMemberBanned = "community.member_banned"
	EventTypeCommunityMemberUnbanned = "community.member_unbanned"

	EventTypeMessageSent   = "message.sent"
	EventTypeMessageRead   = "message.read"
	EventTypeMessageDeleted = "message.deleted"

	EventTypeNotificationCreated = "notification.created"
	EventTypeNotificationRead    = "notification.read"
	EventTypeNotificationDeleted = "notification.deleted"

	EventTypeReportCreated  = "report.created"
	EventTypeReportResolved = "report.resolved"
	EventTypeReportDismissed = "report.dismissed"
	EventTypeReportEscalated = "report.escalated"

	EventTypePollVoted    = "poll.voted"
	EventTypePollCreated  = "poll.created"
	EventTypePollClosed   = "poll.closed"

	EventTypeSearchPerformed = "search.performed"

	EventTypeSystemError    = "system.error"
	EventTypeSystemHealth   = "system.health"
)

// EventPriority represents the priority of an event.
type EventPriority int

const (
	PriorityLow    EventPriority = 0
	PriorityNormal EventPriority = 1
	PriorityHigh   EventPriority = 2
	PriorityCritical EventPriority = 3
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrEventIDEmpty        = errors.New("event ID cannot be empty")
	ErrEventTypeEmpty      = errors.New("event type cannot be empty")
	ErrEventTimestampZero  = errors.New("event timestamp cannot be zero")
	ErrEventSourceEmpty    = errors.New("event source cannot be empty")
	ErrEventDataNil        = errors.New("event data cannot be nil")
	ErrInvalidEventType    = errors.New("invalid event type")
	ErrEventBusStopped     = errors.New("event bus has been stopped")
	ErrEventHandlerFailed  = errors.New("event handler failed")
	ErrEventAlreadyHandled = errors.New("event already handled")
	ErrEventTimeout        = errors.New("event processing timeout")
)

// ======================================================================
// Event Interface
// ======================================================================

// Event defines the base interface for all domain events.
type Event interface {
	ID() string
	Type() string
	Timestamp() time.Time
	Source() string
	Data() interface{}
	Priority() EventPriority
	Version() int
	Metadata() map[string]interface{}
	Validate() error
	IsValid() bool
	String() string
	Clone() Event
	WithMetadata(key string, value interface{}) Event
	WithSource(source string) Event
}

// ======================================================================
// Base Event Implementation
// ======================================================================

// BaseEvent provides a base implementation of the Event interface.
type BaseEvent struct {
	id        string                 `json:"id"`
	eventType string                 `json:"type"`
	timestamp time.Time              `json:"timestamp"`
	source    string                 `json:"source"`
	data      interface{}            `json:"data"`
	priority  EventPriority          `json:"priority"`
	version   int                    `json:"version"`
	metadata  map[string]interface{} `json:"metadata"`
}

// NewBaseEvent creates a new base event.
func NewBaseEvent(eventType string, data interface{}) (*BaseEvent, error) {
	if eventType == "" {
		return nil, ErrEventTypeEmpty
	}
	if data == nil {
		return nil, ErrEventDataNil
	}
	return &BaseEvent{
		id:        uuid.New().String(),
		eventType: eventType,
		timestamp: time.Now().UTC(),
		source:    "system",
		data:      data,
		priority:  PriorityNormal,
		version:   1,
		metadata:  make(map[string]interface{}),
	}, nil
}

// NewBaseEventWithSource creates a new base event with a source.
func NewBaseEventWithSource(eventType string, data interface{}, source string) (*BaseEvent, error) {
	event, err := NewBaseEvent(eventType, data)
	if err != nil {
		return nil, err
	}
	event.source = source
	return event, nil
}

// MustNewBaseEvent creates a new base event and panics on error.
func MustNewBaseEvent(eventType string, data interface{}) *BaseEvent {
	event, err := NewBaseEvent(eventType, data)
	if err != nil {
		panic(err)
	}
	return event
}

// ID returns the event ID.
func (e *BaseEvent) ID() string {
	return e.id
}

// Type returns the event type.
func (e *BaseEvent) Type() string {
	return e.eventType
}

// Timestamp returns the event timestamp.
func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// Source returns the event source.
func (e *BaseEvent) Source() string {
	return e.source
}

// Data returns the event data.
func (e *BaseEvent) Data() interface{} {
	return e.data
}

// Priority returns the event priority.
func (e *BaseEvent) Priority() EventPriority {
	return e.priority
}

// Version returns the event version.
func (e *BaseEvent) Version() int {
	return e.version
}

// Metadata returns the event metadata.
func (e *BaseEvent) Metadata() map[string]interface{} {
	return e.metadata
}

// Validate validates the event.
func (e *BaseEvent) Validate() error {
	if e.id == "" {
		return ErrEventIDEmpty
	}
	if e.eventType == "" {
		return ErrEventTypeEmpty
	}
	if e.timestamp.IsZero() {
		return ErrEventTimestampZero
	}
	if e.source == "" {
		return ErrEventSourceEmpty
	}
	if e.data == nil {
		return ErrEventDataNil
	}
	return nil
}

// IsValid checks if the event is valid.
func (e *BaseEvent) IsValid() bool {
	return e.Validate() == nil
}

// String returns a string representation of the event.
func (e *BaseEvent) String() string {
	return fmt.Sprintf("Event{id:%s, type:%s, source:%s, timestamp:%s}",
		e.id, e.eventType, e.source, e.timestamp.Format(time.RFC3339))
}

// Clone creates a deep copy of the event.
func (e *BaseEvent) Clone() Event {
	clone := &BaseEvent{
		id:        uuid.New().String(),
		eventType: e.eventType,
		timestamp: time.Now().UTC(),
		source:    e.source,
		data:      e.data,
		priority:  e.priority,
		version:   e.version + 1,
		metadata:  make(map[string]interface{}),
	}
	for k, v := range e.metadata {
		clone.metadata[k] = v
	}
	return clone
}

// WithMetadata adds metadata to the event.
func (e *BaseEvent) WithMetadata(key string, value interface{}) Event {
	e.metadata[key] = value
	return e
}

// WithSource sets the event source.
func (e *BaseEvent) WithSource(source string) Event {
	e.source = source
	return e
}

// ======================================================================
// Specific Event Types
// ======================================================================

// TweetCreatedEvent represents a tweet creation event.
type TweetCreatedEvent struct {
	BaseEvent
	TweetID   string `json:"tweet_id"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	MediaURLs []string `json:"media_urls,omitempty"`
}

// NewTweetCreatedEvent creates a new tweet created event.
func NewTweetCreatedEvent(tweetID, userID, content string, mediaURLs []string) (*TweetCreatedEvent, error) {
	data := map[string]interface{}{
		"tweet_id":   tweetID,
		"user_id":    userID,
		"content":    content,
		"media_urls": mediaURLs,
	}
	base, err := NewBaseEvent(EventTypeTweetCreated, data)
	if err != nil {
		return nil, err
	}
	return &TweetCreatedEvent{
		BaseEvent: *base,
		TweetID:   tweetID,
		UserID:    userID,
		Content:   content,
		MediaURLs: mediaURLs,
	}, nil
}

// MustNewTweetCreatedEvent creates a tweet created event and panics on error.
func MustNewTweetCreatedEvent(tweetID, userID, content string, mediaURLs []string) *TweetCreatedEvent {
	event, err := NewTweetCreatedEvent(tweetID, userID, content, mediaURLs)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// TweetLikedEvent
// ======================================================================

// TweetLikedEvent represents a tweet like event.
type TweetLikedEvent struct {
	BaseEvent
	TweetID string `json:"tweet_id"`
	UserID  string `json:"user_id"`
}

// NewTweetLikedEvent creates a new tweet liked event.
func NewTweetLikedEvent(tweetID, userID string) (*TweetLikedEvent, error) {
	data := map[string]interface{}{
		"tweet_id": tweetID,
		"user_id":  userID,
	}
	base, err := NewBaseEvent(EventTypeTweetLiked, data)
	if err != nil {
		return nil, err
	}
	return &TweetLikedEvent{
		BaseEvent: *base,
		TweetID:   tweetID,
		UserID:    userID,
	}, nil
}

// MustNewTweetLikedEvent creates a tweet liked event and panics on error.
func MustNewTweetLikedEvent(tweetID, userID string) *TweetLikedEvent {
	event, err := NewTweetLikedEvent(tweetID, userID)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// TweetRetweetedEvent
// ======================================================================

// TweetRetweetedEvent represents a tweet retweet event.
type TweetRetweetedEvent struct {
	BaseEvent
	TweetID     string `json:"tweet_id"`
	UserID      string `json:"user_id"`
	RetweetType string `json:"retweet_type"`
}

// NewTweetRetweetedEvent creates a new tweet retweeted event.
func NewTweetRetweetedEvent(tweetID, userID, retweetType string) (*TweetRetweetedEvent, error) {
	data := map[string]interface{}{
		"tweet_id":     tweetID,
		"user_id":      userID,
		"retweet_type": retweetType,
	}
	base, err := NewBaseEvent(EventTypeTweetRetweeted, data)
	if err != nil {
		return nil, err
	}
	return &TweetRetweetedEvent{
		BaseEvent:   *base,
		TweetID:     tweetID,
		UserID:      userID,
		RetweetType: retweetType,
	}, nil
}

// MustNewTweetRetweetedEvent creates a tweet retweeted event and panics on error.
func MustNewTweetRetweetedEvent(tweetID, userID, retweetType string) *TweetRetweetedEvent {
	event, err := NewTweetRetweetedEvent(tweetID, userID, retweetType)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// UserFollowedEvent
// ======================================================================

// UserFollowedEvent represents a user follow event.
type UserFollowedEvent struct {
	BaseEvent
	FollowerID string `json:"follower_id"`
	FolloweeID string `json:"followee_id"`
}

// NewUserFollowedEvent creates a new user followed event.
func NewUserFollowedEvent(followerID, followeeID string) (*UserFollowedEvent, error) {
	data := map[string]interface{}{
		"follower_id": followerID,
		"followee_id": followeeID,
	}
	base, err := NewBaseEvent(EventTypeUserFollowed, data)
	if err != nil {
		return nil, err
	}
	return &UserFollowedEvent{
		BaseEvent:  *base,
		FollowerID: followerID,
		FolloweeID: followeeID,
	}, nil
}

// MustNewUserFollowedEvent creates a user followed event and panics on error.
func MustNewUserFollowedEvent(followerID, followeeID string) *UserFollowedEvent {
	event, err := NewUserFollowedEvent(followerID, followeeID)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// UserUnfollowedEvent
// ======================================================================

// UserUnfollowedEvent represents a user unfollow event.
type UserUnfollowedEvent struct {
	BaseEvent
	FollowerID string `json:"follower_id"`
	FolloweeID string `json:"followee_id"`
}

// NewUserUnfollowedEvent creates a new user unfollowed event.
func NewUserUnfollowedEvent(followerID, followeeID string) (*UserUnfollowedEvent, error) {
	data := map[string]interface{}{
		"follower_id": followerID,
		"followee_id": followeeID,
	}
	base, err := NewBaseEvent(EventTypeUserUnfollowed, data)
	if err != nil {
		return nil, err
	}
	return &UserUnfollowedEvent{
		BaseEvent:  *base,
		FollowerID: followerID,
		FolloweeID: followeeID,
	}, nil
}

// MustNewUserUnfollowedEvent creates a user unfollowed event and panics on error.
func MustNewUserUnfollowedEvent(followerID, followeeID string) *UserUnfollowedEvent {
	event, err := NewUserUnfollowedEvent(followerID, followeeID)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// MessageSentEvent
// ======================================================================

// MessageSentEvent represents a message sent event.
type MessageSentEvent struct {
	BaseEvent
	MessageID  string `json:"message_id"`
	SenderID   string `json:"sender_id"`
	ReceiverID string `json:"receiver_id"`
	Content    string `json:"content"`
}

// NewMessageSentEvent creates a new message sent event.
func NewMessageSentEvent(messageID, senderID, receiverID, content string) (*MessageSentEvent, error) {
	data := map[string]interface{}{
		"message_id":  messageID,
		"sender_id":   senderID,
		"receiver_id": receiverID,
		"content":     content,
	}
	base, err := NewBaseEvent(EventTypeMessageSent, data)
	if err != nil {
		return nil, err
	}
	return &MessageSentEvent{
		BaseEvent:  *base,
		MessageID:  messageID,
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
	}, nil
}

// MustNewMessageSentEvent creates a message sent event and panics on error.
func MustNewMessageSentEvent(messageID, senderID, receiverID, content string) *MessageSentEvent {
	event, err := NewMessageSentEvent(messageID, senderID, receiverID, content)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// CommunityCreatedEvent
// ======================================================================

// CommunityCreatedEvent represents a community creation event.
type CommunityCreatedEvent struct {
	BaseEvent
	CommunityID string `json:"community_id"`
	CreatedBy   string `json:"created_by"`
	Name        string `json:"name"`
}

// NewCommunityCreatedEvent creates a new community created event.
func NewCommunityCreatedEvent(communityID, createdBy, name string) (*CommunityCreatedEvent, error) {
	data := map[string]interface{}{
		"community_id": communityID,
		"created_by":   createdBy,
		"name":         name,
	}
	base, err := NewBaseEvent(EventTypeCommunityCreated, data)
	if err != nil {
		return nil, err
	}
	return &CommunityCreatedEvent{
		BaseEvent:   *base,
		CommunityID: communityID,
		CreatedBy:   createdBy,
		Name:        name,
	}, nil
}

// MustNewCommunityCreatedEvent creates a community created event and panics on error.
func MustNewCommunityCreatedEvent(communityID, createdBy, name string) *CommunityCreatedEvent {
	event, err := NewCommunityCreatedEvent(communityID, createdBy, name)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// Event Handler Interface
// ======================================================================

// EventHandler defines the interface for event handlers.
type EventHandler interface {
	Handle(event Event) error
	Handles() []string // Returns the event types this handler handles
	Priority() int
	Name() string
}

// ======================================================================
// Event Bus Interface
// ======================================================================

// EventBus defines the interface for the event bus.
type EventBus interface {
	Publish(event Event) error
	PublishAsync(event Event)
	Subscribe(handler EventHandler) error
	Unsubscribe(handler EventHandler) error
	Start() error
	Stop() error
	IsRunning() bool
	GetStats() EventBusStats
	Clear() error
}

// ======================================================================
// EventBus Stats
// ======================================================================

// EventBusStats represents event bus statistics.
type EventBusStats struct {
	TotalEventsPublished   int64            `json:"total_events_published"`
	TotalEventsHandled     int64            `json:"total_events_handled"`
	TotalEventsFailed      int64            `json:"total_events_failed"`
	PendingEvents          int              `json:"pending_events"`
	RegisteredHandlers     int              `json:"registered_handlers"`
	HandlersByType         map[string]int   `json:"handlers_by_type"`
	LastEventPublished     time.Time        `json:"last_event_published"`
	LastEventHandled       time.Time        `json:"last_event_handled"`
	EventsByType           map[string]int64 `json:"events_by_type"`
}

// ======================================================================
// EventBus Implementation
// ======================================================================

// SimpleEventBus is a simple in-memory event bus.
type SimpleEventBus struct {
	handlers    map[string][]EventHandler
	eventQueue  chan Event
	stopCh      chan struct{}
	stats       EventBusStats
	mu          sync.RWMutex
	wg          sync.WaitGroup
	running     bool
	workerCount int
}

// NewSimpleEventBus creates a new simple event bus.
func NewSimpleEventBus(workerCount int) *SimpleEventBus {
	if workerCount < 1 {
		workerCount = 5
	}
	return &SimpleEventBus{
		handlers:    make(map[string][]EventHandler),
		eventQueue:  make(chan Event, 1000),
		stopCh:      make(chan struct{}),
		stats: EventBusStats{
			HandlersByType: make(map[string]int),
			EventsByType:   make(map[string]int64),
		},
		workerCount: workerCount,
	}
}

// Start starts the event bus.
func (b *SimpleEventBus) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running {
		return nil
	}
	b.running = true
	b.stopCh = make(chan struct{})
	for i := 0; i < b.workerCount; i++ {
		b.wg.Add(1)
		go b.worker()
	}
	return nil
}

// Stop stops the event bus.
func (b *SimpleEventBus) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.running {
		return nil
	}
	b.running = false
	close(b.stopCh)
	b.wg.Wait()
	close(b.eventQueue)
	return nil
}

// IsRunning returns true if the event bus is running.
func (b *SimpleEventBus) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// worker processes events from the queue.
func (b *SimpleEventBus) worker() {
	defer b.wg.Done()
	for {
		select {
		case event, ok := <-b.eventQueue:
			if !ok {
				return
			}
			b.processEvent(event)
		case <-b.stopCh:
			return
		}
	}
}

// processEvent processes a single event.
func (b *SimpleEventBus) processEvent(event Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type()]
	b.mu.RUnlock()

	if len(handlers) == 0 {
		return
	}

	for _, handler := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					b.mu.Lock()
					b.stats.TotalEventsFailed++
					b.mu.Unlock()
				}
			}()
			if err := handler.Handle(event); err != nil {
				b.mu.Lock()
				b.stats.TotalEventsFailed++
				b.mu.Unlock()
			} else {
				b.mu.Lock()
				b.stats.TotalEventsHandled++
				b.stats.LastEventHandled = time.Now()
				b.mu.Unlock()
			}
		}()
	}
}

// Publish publishes an event synchronously.
func (b *SimpleEventBus) Publish(event Event) error {
	if !b.IsRunning() {
		return ErrEventBusStopped
	}
	if err := event.Validate(); err != nil {
		return err
	}
	select {
	case b.eventQueue <- event:
		b.mu.Lock()
		b.stats.TotalEventsPublished++
		b.stats.LastEventPublished = time.Now()
		b.stats.EventsByType[event.Type()]++
		b.mu.Unlock()
		return nil
	default:
		return errors.New("event queue is full")
	}
}

// PublishAsync publishes an event asynchronously.
func (b *SimpleEventBus) PublishAsync(event Event) {
	if !b.IsRunning() {
		return
	}
	if err := event.Validate(); err != nil {
		return
	}
	select {
	case b.eventQueue <- event:
		b.mu.Lock()
		b.stats.TotalEventsPublished++
		b.stats.LastEventPublished = time.Now()
		b.stats.EventsByType[event.Type()]++
		b.mu.Unlock()
	default:
		// Queue is full, drop the event
	}
}

// Subscribe registers an event handler.
func (b *SimpleEventBus) Subscribe(handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if handler == nil {
		return errors.New("handler cannot be nil")
	}
	eventTypes := handler.Handles()
	if len(eventTypes) == 0 {
		return errors.New("handler must handle at least one event type")
	}
	for _, eventType := range eventTypes {
		if eventType == "" {
			return errors.New("event type cannot be empty")
		}
		b.handlers[eventType] = append(b.handlers[eventType], handler)
		b.stats.HandlersByType[eventType]++
	}
	b.stats.RegisteredHandlers++
	return nil
}

// Unsubscribe unregisters an event handler.
func (b *SimpleEventBus) Unsubscribe(handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if handler == nil {
		return errors.New("handler cannot be nil")
	}
	for eventType, handlers := range b.handlers {
		for i, h := range handlers {
			if h == handler {
				b.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
				b.stats.HandlersByType[eventType]--
				if len(b.handlers[eventType]) == 0 {
					delete(b.handlers, eventType)
				}
				break
			}
		}
	}
	b.stats.RegisteredHandlers--
	return nil
}

// GetStats returns event bus statistics.
func (b *SimpleEventBus) GetStats() EventBusStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	stats := b.stats
	stats.PendingEvents = len(b.eventQueue)
	return stats
}

// Clear removes all event handlers.
func (b *SimpleEventBus) Clear() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = make(map[string][]EventHandler)
	b.stats.RegisteredHandlers = 0
	b.stats.HandlersByType = make(map[string]int)
	return nil
}

// ======================================================================
// Event Serialization
// ======================================================================

// SerializeEvent serializes an event to JSON.
func SerializeEvent(event Event) ([]byte, error) {
	if event == nil {
		return nil, errors.New("event cannot be nil")
	}
	data := struct {
		ID        string                 `json:"id"`
		Type      string                 `json:"type"`
		Timestamp time.Time              `json:"timestamp"`
		Source    string                 `json:"source"`
		Data      interface{}            `json:"data"`
		Priority  int                    `json:"priority"`
		Version   int                    `json:"version"`
		Metadata  map[string]interface{} `json:"metadata"`
	}{
		ID:        event.ID(),
		Type:      event.Type(),
		Timestamp: event.Timestamp(),
		Source:    event.Source(),
		Data:      event.Data(),
		Priority:  int(event.Priority()),
		Version:   event.Version(),
		Metadata:  event.Metadata(),
	}
	return json.Marshal(data)
}

// DeserializeEvent deserializes an event from JSON.
func DeserializeEvent(data []byte) (Event, error) {
	var raw struct {
		ID        string                 `json:"id"`
		Type      string                 `json:"type"`
		Timestamp time.Time              `json:"timestamp"`
		Source    string                 `json:"source"`
		Data      json.RawMessage        `json:"data"`
		Priority  int                    `json:"priority"`
		Version   int                    `json:"version"`
		Metadata  map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	// Create base event
	base := &BaseEvent{
		id:        raw.ID,
		eventType: raw.Type,
		timestamp: raw.Timestamp,
		source:    raw.Source,
		data:      raw.Data,
		priority:  EventPriority(raw.Priority),
		version:   raw.Version,
		metadata:  raw.Metadata,
	}
	return base, nil
}

// ======================================================================
= Global Event Bus
// ======================================================================

var (
	defaultEventBus *SimpleEventBus
	eventBusOnce    sync.Once
)

// InitEventBus initializes the global event bus.
func InitEventBus(workerCount int) {
	eventBusOnce.Do(func() {
		defaultEventBus = NewSimpleEventBus(workerCount)
		_ = defaultEventBus.Start()
	})
}

// GetEventBus returns the global event bus.
func GetEventBus() *SimpleEventBus {
	if defaultEventBus == nil {
		InitEventBus(5)
	}
	return defaultEventBus
}

// PublishEvent publishes an event using the global event bus.
func PublishEvent(event Event) error {
	return GetEventBus().Publish(event)
}

// PublishEventAsync publishes an event asynchronously.
func PublishEventAsync(event Event) {
	GetEventBus().PublishAsync(event)
}

// SubscribeHandler subscribes a handler to the global event bus.
func SubscribeHandler(handler EventHandler) error {
	return GetEventBus().Subscribe(handler)
}