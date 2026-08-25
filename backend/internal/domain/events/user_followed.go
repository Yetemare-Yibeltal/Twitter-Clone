// backend/internal/domain/events/user_followed.go
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
	ErrUserFollowedIDEmpty         = errors.New("user followed event ID cannot be empty")
	ErrUserFollowedFollowerIDEmpty = errors.New("follower ID cannot be empty")
	ErrUserFollowedFolloweeIDEmpty = errors.New("followee ID cannot be empty")
	ErrUserFollowedSelf            = errors.New("cannot follow yourself")
	ErrUserFollowedAlreadyExists   = errors.New("follow relationship already exists")
	ErrUserFollowedNotFound        = errors.New("follow relationship not found")
	ErrUserFollowedInvalidStatus   = errors.New("invalid follow status")
	ErrUserFollowedAlreadyFollowed = errors.New("already following this user")
	ErrUserFollowedNotFollowing    = errors.New("not following this user")
)

// ======================================================================
// FollowStatus
// ======================================================================

// FollowStatus represents the status of a follow relationship.
type FollowStatus string

const (
	FollowStatusPending  FollowStatus = "pending"
	FollowStatusAccepted FollowStatus = "accepted"
	FollowStatusRejected FollowStatus = "rejected"
	FollowStatusBlocked  FollowStatus = "blocked"
)

// ValidFollowStatuses returns all valid follow statuses.
func ValidFollowStatuses() []FollowStatus {
	return []FollowStatus{
		FollowStatusPending,
		FollowStatusAccepted,
		FollowStatusRejected,
		FollowStatusBlocked,
	}
}

// IsValid checks if a follow status is valid.
func (s FollowStatus) IsValid() bool {
	for _, status := range ValidFollowStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation of the status.
func (s FollowStatus) String() string {
	return string(s)
}

// ======================================================================
// UserFollowedEvent
// ======================================================================

// UserFollowedEvent represents the event when a user follows another user.
type UserFollowedEvent struct {
	BaseEvent
	FollowerID    string       `json:"follower_id"`
	FolloweeID    string       `json:"followee_id"`
	FollowID      string       `json:"follow_id"`
	Status        FollowStatus `json:"status"`
	IsUndo        bool         `json:"is_undo"` // true if this is an unfollow event
	FollowedAt    time.Time    `json:"followed_at"`
	FollowerUsername string    `json:"follower_username,omitempty"`
	FolloweeUsername string    `json:"followee_username,omitempty"`
	FollowerCount  int64       `json:"follower_count"` // followee's follower count after event
	FollowingCount int64       `json:"following_count"` // follower's following count after event
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewUserFollowedEvent creates a new user followed event.
func NewUserFollowedEvent(followerID, followeeID, followID string, status FollowStatus, followerCount, followingCount int64) (*UserFollowedEvent, error) {
	if followerID == "" {
		return nil, ErrUserFollowedFollowerIDEmpty
	}
	if followeeID == "" {
		return nil, ErrUserFollowedFolloweeIDEmpty
	}
	if followerID == followeeID {
		return nil, ErrUserFollowedSelf
	}
	if followID == "" {
		return nil, errors.New("follow ID cannot be empty")
	}
	if !status.IsValid() {
		return nil, ErrUserFollowedInvalidStatus
	}
	if followerCount < 0 {
		return nil, errors.New("follower count cannot be negative")
	}
	if followingCount < 0 {
		return nil, errors.New("following count cannot be negative")
	}
	data := map[string]interface{}{
		"follower_id":     followerID,
		"followee_id":     followeeID,
		"follow_id":       followID,
		"status":          string(status),
		"is_undo":         false,
		"follower_count":  followerCount,
		"following_count": followingCount,
	}
	base, err := NewBaseEvent(EventTypeUserFollowed, data)
	if err != nil {
		return nil, err
	}
	base.WithSource("follow_service")
	base.WithMetadata("action", "follow")
	base.WithMetadata("timestamp", time.Now().UTC().Unix())
	return &UserFollowedEvent{
		BaseEvent:      *base,
		FollowerID:     followerID,
		FolloweeID:     followeeID,
		FollowID:       followID,
		Status:         status,
		IsUndo:         false,
		FollowedAt:     time.Now().UTC(),
		FollowerCount:  followerCount,
		FollowingCount: followingCount,
	}, nil
}

// NewUserUnfollowedEvent creates a new user unfollowed event.
func NewUserUnfollowedEvent(followerID, followeeID, followID string, followerCount, followingCount int64) (*UserFollowedEvent, error) {
	if followerID == "" {
		return nil, ErrUserFollowedFollowerIDEmpty
	}
	if followeeID == "" {
		return nil, ErrUserFollowedFolloweeIDEmpty
	}
	if followerID == followeeID {
		return nil, ErrUserFollowedSelf
	}
	if followID == "" {
		return nil, errors.New("follow ID cannot be empty")
	}
	if followerCount < 0 {
		return nil, errors.New("follower count cannot be negative")
	}
	if followingCount < 0 {
		return nil, errors.New("following count cannot be negative")
	}
	data := map[string]interface{}{
		"follower_id":     followerID,
		"followee_id":     followeeID,
		"follow_id":       followID,
		"is_undo":         true,
		"follower_count":  followerCount,
		"following_count": followingCount,
	}
	base, err := NewBaseEvent(EventTypeUserFollowed, data)
	if err != nil {
		return nil, err
	}
	base.WithSource("follow_service")
	base.WithMetadata("action", "unfollow")
	base.WithMetadata("timestamp", time.Now().UTC().Unix())
	return &UserFollowedEvent{
		BaseEvent:      *base,
		FollowerID:     followerID,
		FolloweeID:     followeeID,
		FollowID:       followID,
		Status:         FollowStatusAccepted,
		IsUndo:         true,
		FollowedAt:     time.Now().UTC(),
		FollowerCount:  followerCount,
		FollowingCount: followingCount,
	}, nil
}

// NewUserFollowedEventWithTime creates a user followed event with custom time.
func NewUserFollowedEventWithTime(followerID, followeeID, followID string, status FollowStatus, followerCount, followingCount int64, followedAt time.Time) (*UserFollowedEvent, error) {
	event, err := NewUserFollowedEvent(followerID, followeeID, followID, status, followerCount, followingCount)
	if err != nil {
		return nil, err
	}
	event.FollowedAt = followedAt
	return event, nil
}

// NewUserFollowedEventWithUsernames creates a user followed event with usernames.
func NewUserFollowedEventWithUsernames(followerID, followeeID, followID string, status FollowStatus, followerCount, followingCount int64, followerUsername, followeeUsername string) (*UserFollowedEvent, error) {
	event, err := NewUserFollowedEvent(followerID, followeeID, followID, status, followerCount, followingCount)
	if err != nil {
		return nil, err
	}
	event.FollowerUsername = followerUsername
	event.FolloweeUsername = followeeUsername
	return event, nil
}

// MustNewUserFollowedEvent creates a user followed event and panics on error.
func MustNewUserFollowedEvent(followerID, followeeID, followID string, status FollowStatus, followerCount, followingCount int64) *UserFollowedEvent {
	event, err := NewUserFollowedEvent(followerID, followeeID, followID, status, followerCount, followingCount)
	if err != nil {
		panic(err)
	}
	return event
}

// MustNewUserUnfollowedEvent creates a user unfollowed event and panics on error.
func MustNewUserUnfollowedEvent(followerID, followeeID, followID string, followerCount, followingCount int64) *UserFollowedEvent {
	event, err := NewUserUnfollowedEvent(followerID, followeeID, followID, followerCount, followingCount)
	if err != nil {
		panic(err)
	}
	return event
}

// ======================================================================
// Validation
// ======================================================================

// Validate validates the user followed event.
func (e *UserFollowedEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	if e.FollowerID == "" {
		return ErrUserFollowedFollowerIDEmpty
	}
	if e.FolloweeID == "" {
		return ErrUserFollowedFolloweeIDEmpty
	}
	if e.FollowerID == e.FolloweeID {
		return ErrUserFollowedSelf
	}
	if e.FollowID == "" {
		return errors.New("follow ID cannot be empty")
	}
	if !e.IsUndo && !e.Status.IsValid() {
		return ErrUserFollowedInvalidStatus
	}
	if e.FollowerCount < 0 {
		return errors.New("follower count cannot be negative")
	}
	if e.FollowingCount < 0 {
		return errors.New("following count cannot be negative")
	}
	return nil
}

// ======================================================================
// Getters
// ======================================================================

// GetFollowerID returns the follower ID.
func (e *UserFollowedEvent) GetFollowerID() string {
	return e.FollowerID
}

// GetFolloweeID returns the followee ID.
func (e *UserFollowedEvent) GetFolloweeID() string {
	return e.FolloweeID
}

// GetFollowID returns the follow ID.
func (e *UserFollowedEvent) GetFollowID() string {
	return e.FollowID
}

// GetStatus returns the follow status.
func (e *UserFollowedEvent) GetStatus() FollowStatus {
	return e.Status
}

// GetFollowerCount returns the follower count.
func (e *UserFollowedEvent) GetFollowerCount() int64 {
	return e.FollowerCount
}

// GetFollowingCount returns the following count.
func (e *UserFollowedEvent) GetFollowingCount() int64 {
	return e.FollowingCount
}

// GetFollowedAt returns the follow timestamp.
func (e *UserFollowedEvent) GetFollowedAt() time.Time {
	return e.FollowedAt
}

// GetFollowerUsername returns the follower username.
func (e *UserFollowedEvent) GetFollowerUsername() string {
	return e.FollowerUsername
}

// GetFolloweeUsername returns the followee username.
func (e *UserFollowedEvent) GetFolloweeUsername() string {
	return e.FolloweeUsername
}

// IsFollow returns true if this is a follow event.
func (e *UserFollowedEvent) IsFollow() bool {
	return !e.IsUndo
}

// IsUnfollow returns true if this is an unfollow event.
func (e *UserFollowedEvent) IsUnfollow() bool {
	return e.IsUndo
}

// IsAccepted returns true if the follow is accepted.
func (e *UserFollowedEvent) IsAccepted() bool {
	return e.Status == FollowStatusAccepted
}

// IsPending returns true if the follow is pending.
func (e *UserFollowedEvent) IsPending() bool {
	return e.Status == FollowStatusPending
}

// IsMutual returns true if the follow is mutual (both follow each other).
// This requires additional context; this is a placeholder.
func (e *UserFollowedEvent) IsMutual() bool {
	// This would need to check if followee also follows follower
	// For now, return false
	return false
}

// ======================================================================
// Helper Methods
// ======================================================================

// IsFollower checks if a user is the follower.
func (e *UserFollowedEvent) IsFollower(userID string) bool {
	return e.FollowerID == userID
}

// IsFollowee checks if a user is the followee.
func (e *UserFollowedEvent) IsFollowee(userID string) bool {
	return e.FolloweeID == userID
}

// IsUser checks if a user is either the follower or followee.
func (e *UserFollowedEvent) IsUser(userID string) bool {
	return e.FollowerID == userID || e.FolloweeID == userID
}

// GetOtherUser returns the other user ID given one participant.
func (e *UserFollowedEvent) GetOtherUser(userID string) (string, error) {
	if e.FollowerID == userID {
		return e.FolloweeID, nil
	}
	if e.FolloweeID == userID {
		return e.FollowerID, nil
	}
	return "", errors.New("user is not a participant in this follow event")
}

// String returns a string representation of the event.
func (e *UserFollowedEvent) String() string {
	action := "followed"
	if e.IsUndo {
		action = "unfollowed"
	}
	return fmt.Sprintf("UserFollowedEvent{id:%s, follower:%s, followee:%s, action:%s, status:%s, time:%s}",
		e.ID(), e.FollowerID, e.FolloweeID, action, e.Status, e.FollowedAt.Format(time.RFC3339))
}

// Clone creates a deep copy of the event.
func (e *UserFollowedEvent) Clone() Event {
	clone := &UserFollowedEvent{
		BaseEvent:        *e.BaseEvent.Clone().(*BaseEvent),
		FollowerID:       e.FollowerID,
		FolloweeID:       e.FolloweeID,
		FollowID:         e.FollowID,
		Status:           e.Status,
		IsUndo:           e.IsUndo,
		FollowedAt:       e.FollowedAt,
		FollowerUsername: e.FollowerUsername,
		FolloweeUsername: e.FolloweeUsername,
		FollowerCount:    e.FollowerCount,
		FollowingCount:   e.FollowingCount,
	}
	return clone
}

// ======================================================================
// JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (e *UserFollowedEvent) MarshalJSON() ([]byte, error) {
	type Alias UserFollowedEvent
	return json.Marshal(&struct {
		*Alias
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		StatusStr  string `json:"status"`
		IsFollow   bool   `json:"is_follow"`
		IsAccepted bool   `json:"is_accepted"`
	}{
		Alias:      (*Alias)(e),
		EventID:    e.ID(),
		EventType:  e.Type(),
		StatusStr:  string(e.Status),
		IsFollow:   e.IsFollow(),
		IsAccepted: e.IsAccepted(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (e *UserFollowedEvent) UnmarshalJSON(data []byte) error {
	type Alias UserFollowedEvent
	aux := &struct {
		*Alias
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
		StatusStr string `json:"status"`
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.StatusStr != "" {
		e.Status = FollowStatus(aux.StatusStr)
	}
	return nil
}

// ======================================================================
// Event Handlers (convenience)
// ======================================================================

// UserFollowedEventHandler is a convenience handler for user followed events.
type UserFollowedEventHandler struct {
	handler func(event *UserFollowedEvent) error
	name    string
}

// NewUserFollowedEventHandler creates a new user followed event handler.
func NewUserFollowedEventHandler(name string, handler func(event *UserFollowedEvent) error) *UserFollowedEventHandler {
	if name == "" {
		name = "UserFollowedEventHandler"
	}
	return &UserFollowedEventHandler{
		handler: handler,
		name:    name,
	}
}

// Handle processes the event.
func (h *UserFollowedEventHandler) Handle(event Event) error {
	userFollowedEvent, ok := event.(*UserFollowedEvent)
	if !ok {
		return errors.New("event is not a UserFollowedEvent")
	}
	return h.handler(userFollowedEvent)
}

// Handles returns the event types this handler handles.
func (h *UserFollowedEventHandler) Handles() []string {
	return []string{EventTypeUserFollowed}
}

// Priority returns the handler priority.
func (h *UserFollowedEventHandler) Priority() int {
	return 1
}

// Name returns the handler name.
func (h *UserFollowedEventHandler) Name() string {
	return h.name
}

// ======================================================================
// Builder Pattern
// ======================================================================

// UserFollowedEventBuilder helps construct user followed events for testing.
type UserFollowedEventBuilder struct {
	event *UserFollowedEvent
}

// NewUserFollowedEventBuilder creates a new builder.
func NewUserFollowedEventBuilder() *UserFollowedEventBuilder {
	return &UserFollowedEventBuilder{
		event: &UserFollowedEvent{
			BaseEvent: BaseEvent{
				id:        uuid.New().String(),
				eventType: EventTypeUserFollowed,
				timestamp: time.Now().UTC(),
				source:    "test",
				data:      make(map[string]interface{}),
				priority:  PriorityNormal,
				version:   1,
				metadata:  make(map[string]interface{}),
			},
			FollowerID:     "",
			FolloweeID:     "",
			FollowID:       "",
			Status:         FollowStatusAccepted,
			IsUndo:         false,
			FollowedAt:     time.Now().UTC(),
			FollowerCount:  10,
			FollowingCount: 5,
		},
	}
}

// WithID sets the event ID.
func (b *UserFollowedEventBuilder) WithID(id string) *UserFollowedEventBuilder {
	b.event.id = id
	return b
}

// WithFollowerID sets the follower ID.
func (b *UserFollowedEventBuilder) WithFollowerID(followerID string) *UserFollowedEventBuilder {
	b.event.FollowerID = followerID
	return b
}

// WithFolloweeID sets the followee ID.
func (b *UserFollowedEventBuilder) WithFolloweeID(followeeID string) *UserFollowedEventBuilder {
	b.event.FolloweeID = followeeID
	return b
}

// WithFollowID sets the follow ID.
func (b *UserFollowedEventBuilder) WithFollowID(followID string) *UserFollowedEventBuilder {
	b.event.FollowID = followID
	return b
}

// WithStatus sets the follow status.
func (b *UserFollowedEventBuilder) WithStatus(status FollowStatus) *UserFollowedEventBuilder {
	b.event.Status = status
	return b
}

// WithUndo sets the undo flag.
func (b *UserFollowedEventBuilder) WithUndo(undo bool) *UserFollowedEventBuilder {
	b.event.IsUndo = undo
	return b
}

// WithFollowerCount sets the follower count.
func (b *UserFollowedEventBuilder) WithFollowerCount(count int64) *UserFollowedEventBuilder {
	b.event.FollowerCount = count
	return b
}

// WithFollowingCount sets the following count.
func (b *UserFollowedEventBuilder) WithFollowingCount(count int64) *UserFollowedEventBuilder {
	b.event.FollowingCount = count
	return b
}

// WithUsernames sets the usernames.
func (b *UserFollowedEventBuilder) WithUsernames(followerUsername, followeeUsername string) *UserFollowedEventBuilder {
	b.event.FollowerUsername = followerUsername
	b.event.FolloweeUsername = followeeUsername
	return b
}

// WithFollowedAt sets the follow timestamp.
func (b *UserFollowedEventBuilder) WithFollowedAt(t time.Time) *UserFollowedEventBuilder {
	b.event.FollowedAt = t
	b.event.timestamp = t
	return b
}

// WithSource sets the event source.
func (b *UserFollowedEventBuilder) WithSource(source string) *UserFollowedEventBuilder {
	b.event.source = source
	return b
}

// WithMetadata adds metadata.
func (b *UserFollowedEventBuilder) WithMetadata(key string, value interface{}) *UserFollowedEventBuilder {
	b.event.metadata[key] = value
	return b
}

// Build validates and returns the event.
func (b *UserFollowedEventBuilder) Build() (*UserFollowedEvent, error) {
	if err := b.event.Validate(); err != nil {
		return nil, err
	}
	return b.event, nil
}

// MustBuild builds without error (panics on error).
func (b *UserFollowedEventBuilder) MustBuild() *UserFollowedEvent {
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
	TestUserFollowedEvent1 = MustNewUserFollowedEvent("user1", "user2", "follow1", FollowStatusAccepted, 10, 5)
	TestUserFollowedEvent2 = MustNewUserFollowedEvent("user3", "user1", "follow2", FollowStatusPending, 8, 3)
	TestUserUnfollowedEvent1 = MustNewUserUnfollowedEvent("user1", "user2", "follow1", 9, 4)
)

// MustNewTestUserFollowedEvent creates a test event with defaults.
func MustNewTestUserFollowedEvent(followerID, followeeID string) *UserFollowedEvent {
	return MustNewUserFollowedEvent(followerID, followeeID, uuid.New().String(), FollowStatusAccepted, 10, 5)
}

// MustNewTestUserUnfollowedEvent creates a test unfollow event.
func MustNewTestUserUnfollowedEvent(followerID, followeeID string) *UserFollowedEvent {
	return MustNewUserUnfollowedEvent(followerID, followeeID, uuid.New().String(), 9, 4)
}

// MustNewTestPendingFollowEvent creates a test pending follow event.
func MustNewTestPendingFollowEvent(followerID, followeeID string) *UserFollowedEvent {
	return MustNewUserFollowedEvent(followerID, followeeID, uuid.New().String(), FollowStatusPending, 10, 5)
}

// MustNewTestBlockedFollowEvent creates a test blocked follow event.
func MustNewTestBlockedFollowEvent(followerID, followeeID string) *UserFollowedEvent {
	return MustNewUserFollowedEvent(followerID, followeeID, uuid.New().String(), FollowStatusBlocked, 10, 5)
}

// ======================================================================
// Follow Statistics (helper)
// ======================================================================

// FollowStats represents aggregated follow statistics.
type FollowStats struct {
	TotalFollows   int64 `json:"total_follows"`
	TotalUnfollows int64 `json:"total_unfollows"`
	NetFollows     int64 `json:"net_follows"`
	PendingCount   int64 `json:"pending_count"`
	AcceptedCount  int64 `json:"accepted_count"`
	BlockedCount   int64 `json:"blocked_count"`
}

// CalculateFollowStats calculates statistics from a list of follow events.
func CalculateFollowStats(events []*UserFollowedEvent) *FollowStats {
	stats := &FollowStats{}
	for _, e := range events {
		if e.IsFollow() {
			stats.TotalFollows++
			switch e.Status {
			case FollowStatusPending:
				stats.PendingCount++
			case FollowStatusAccepted:
				stats.AcceptedCount++
			case FollowStatusBlocked:
				stats.BlockedCount++
			}
		} else {
			stats.TotalUnfollows++
		}
	}
	stats.NetFollows = stats.TotalFollows - stats.TotalUnfollows
	return stats
}