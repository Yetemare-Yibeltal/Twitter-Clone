// backend/internal/domain/entities/notification.go
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

// Notification types
const (
	NotifTypeLike     = "like"
	NotifTypeRetweet  = "retweet"
	NotifTypeFollow   = "follow"
	NotifTypeReply    = "reply"
	NotifTypeMention  = "mention"
	NotifTypeQuote    = "quote"
	NotifTypeMessage  = "message"
	NotifTypeJoin     = "join"
	NotifTypeLeave    = "leave"
	NotifTypeCommunity = "community"
	NotifTypeSystem   = "system"
)

// Notification priorities
const (
	PriorityLow    = 0
	PriorityMedium = 1
	PriorityHigh   = 2
	PriorityUrgent = 3
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrNotificationIDEmpty      = errors.New("notification ID cannot be empty")
	ErrNotificationUserIDEmpty  = errors.New("user ID cannot be empty")
	ErrNotificationTypeInvalid  = errors.New("invalid notification type")
	ErrNotificationAlreadyRead  = errors.New("notification already read")
	ErrNotificationAlreadyDeleted = errors.New("notification already deleted")
	ErrNotificationNotDeleted   = errors.New("notification is not deleted")
	ErrNotificationReferenceEmpty = errors.New("reference ID cannot be empty")
)

// ======================================================================
= NotificationMetadata
// ======================================================================

// NotificationMetadata holds optional notification metadata.
type NotificationMetadata struct {
	GroupID     string            `json:"group_id,omitempty"`
	Priority    int               `json:"priority,omitempty"`
	ActionURL   string            `json:"action_url,omitempty"`
	ImageURL    string            `json:"image_url,omitempty"`
	Summary     string            `json:"summary,omitempty"`
	CustomData  map[string]string `json:"custom_data,omitempty"`
}

// Value implements driver.Valuer for JSON storage.
func (m NotificationMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements sql.Scanner for JSON retrieval.
func (m *NotificationMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = NotificationMetadata{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for NotificationMetadata: %T", value)
	}
	return json.Unmarshal(bytes, m)
}

// ======================================================================
= Notification Entity
// ======================================================================

// Notification represents a notification for a user.
type Notification struct {
	ID          string               `db:"id" json:"id"`
	UserID      string               `db:"user_id" json:"user_id"`
	FromUserID  string               `db:"from_user_id" json:"from_user_id,omitempty"`
	Type        string               `db:"type" json:"type"`
	ReferenceID string               `db:"reference_id" json:"reference_id,omitempty"`
	Read        bool                 `db:"read" json:"read"`
	ReadAt      *time.Time           `db:"read_at" json:"read_at,omitempty"`
	Metadata    NotificationMetadata `db:"metadata" json:"metadata,omitempty"`
	CreatedAt   time.Time            `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time            `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time           `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ======================================================================
= Factory Methods
// ======================================================================

// NewNotification creates a new notification instance with validation.
func NewNotification(userID, fromUserID, notifType, referenceID string) (*Notification, error) {
	n := &Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		FromUserID:  fromUserID,
		Type:        notifType,
		ReferenceID: referenceID,
		Read:        false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := n.Validate(); err != nil {
		return nil, err
	}
	return n, nil
}

// MustNewNotification creates a new notification and panics on error.
func MustNewNotification(userID, fromUserID, notifType, referenceID string) *Notification {
	n, err := NewNotification(userID, fromUserID, notifType, referenceID)
	if err != nil {
		panic(err)
	}
	return n
}

// ======================================================================
= Validation
// ======================================================================

// Validate performs comprehensive validation.
func (n *Notification) Validate() error {
	if strings.TrimSpace(n.ID) == "" {
		return ErrNotificationIDEmpty
	}
	if strings.TrimSpace(n.UserID) == "" {
		return ErrNotificationUserIDEmpty
	}
	if !isValidNotificationType(n.Type) {
		return ErrNotificationTypeInvalid
	}
	if n.ReferenceID != "" {
		if strings.TrimSpace(n.ReferenceID) == "" {
			return ErrNotificationReferenceEmpty
		}
	}
	return nil
}

// isValidNotificationType checks if the notification type is valid.
func isValidNotificationType(notifType string) bool {
	validTypes := map[string]bool{
		NotifTypeLike: true, NotifTypeRetweet: true,
		NotifTypeFollow: true, NotifTypeReply: true,
		NotifTypeMention: true, NotifTypeQuote: true,
		NotifTypeMessage: true, NotifTypeJoin: true,
		NotifTypeLeave: true, NotifTypeCommunity: true,
		NotifTypeSystem: true,
	}
	return validTypes[notifType]
}

// ======================================================================
= Business Logic Methods
// ======================================================================

// MarkAsRead marks the notification as read.
func (n *Notification) MarkAsRead() error {
	if n.DeletedAt != nil {
		return ErrNotificationAlreadyDeleted
	}
	if n.Read {
		return ErrNotificationAlreadyRead
	}
	n.Read = true
	now := time.Now()
	n.ReadAt = &now
	n.UpdatedAt = now
	return nil
}

// MarkAsUnread marks the notification as unread.
func (n *Notification) MarkAsUnread() error {
	if n.DeletedAt != nil {
		return ErrNotificationAlreadyDeleted
	}
	if !n.Read {
		return errors.New("notification already unread")
	}
	n.Read = false
	n.ReadAt = nil
	n.UpdatedAt = time.Now()
	return nil
}

// SoftDelete marks the notification as deleted.
func (n *Notification) SoftDelete() error {
	if n.DeletedAt != nil {
		return ErrNotificationAlreadyDeleted
	}
	now := time.Now()
	n.DeletedAt = &now
	n.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted notification.
func (n *Notification) Restore() error {
	if n.DeletedAt == nil {
		return ErrNotificationNotDeleted
	}
	n.DeletedAt = nil
	n.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the notification is deleted.
func (n *Notification) IsDeleted() bool {
	return n.DeletedAt != nil
}

// IsRead checks if the notification is read.
func (n *Notification) IsRead() bool {
	return n.Read
}

// IsForUser checks if the notification is for a specific user.
func (n *Notification) IsForUser(userID string) bool {
	return n.UserID == userID
}

// IsFromUser checks if the notification is from a specific user.
func (n *Notification) IsFromUser(userID string) bool {
	return n.FromUserID == userID
}

// IsType checks if the notification is of a specific type.
func (n *Notification) IsType(notifType string) bool {
	return n.Type == notifType
}

// GetPriority returns the notification priority.
func (n *Notification) GetPriority() int {
	if n.Metadata.Priority > 0 {
		return n.Metadata.Priority
	}
	// Default priority based on type
	switch n.Type {
	case NotifTypeSystem, NotifTypeCommunity:
		return PriorityHigh
	case NotifTypeLike, NotifTypeRetweet, NotifTypeFollow:
		return PriorityMedium
	default:
		return PriorityLow
	}
}

// ======================================================================
= Helper Methods
// ======================================================================

// String returns a human-readable representation.
func (n *Notification) String() string {
	return fmt.Sprintf("Notification{ID:%s, user:%s, type:%s, read:%v, created:%v}",
		n.ID, n.UserID, n.Type, n.Read, n.CreatedAt)
}

// Clone returns a deep copy of the notification.
func (n *Notification) Clone() *Notification {
	clone := *n
	if n.ReadAt != nil {
		t := *n.ReadAt
		clone.ReadAt = &t
	}
	if n.DeletedAt != nil {
		t := *n.DeletedAt
		clone.DeletedAt = &t
	}
	return &clone
}

// Equals checks if two notifications have the same ID.
func (n *Notification) Equals(other *Notification) bool {
	return n.ID == other.ID
}

// IsEmpty returns true if the notification is zero value.
func (n *Notification) IsEmpty() bool {
	return n.ID == "" && n.UserID == "" && n.Type == ""
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (n Notification) Value() (driver.Value, error) {
	return json.Marshal(n)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (n *Notification) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for Notification: %T", value)
	}
	return json.Unmarshal(bytes, n)
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (n *Notification) MarshalJSON() ([]byte, error) {
	type Alias Notification
	return json.Marshal(&struct {
		*Alias
		Priority int `json:"priority"`
	}{
		Alias:    (*Alias)(n),
		Priority: n.GetPriority(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (n *Notification) UnmarshalJSON(data []byte) error {
	type Alias Notification
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(n),
	}
	return json.Unmarshal(data, aux)
}

// ======================================================================
= Grouping and Aggregation
// ======================================================================

// NotificationGroup represents a group of similar notifications.
type NotificationGroup struct {
	Type        string    `json:"type"`
	ReferenceID string    `json:"reference_id"`
	Count       int64     `json:"count"`
	LatestAt    time.Time `json:"latest_at"`
	ReadCount   int64     `json:"read_count"`
	UnreadCount int64     `json:"unread_count"`
	FromUserIDs []string  `json:"from_user_ids"`
}

// GroupKey returns a group key for a notification.
func (n *Notification) GroupKey() string {
	if n.Metadata.GroupID != "" {
		return n.Metadata.GroupID
	}
	// If group_id not set, use type + reference_id
	if n.ReferenceID != "" {
		return n.Type + ":" + n.ReferenceID
	}
	return n.Type
}

// ShouldGroup checks if notifications should be grouped.
func (n *Notification) ShouldGroup() bool {
	// Types that should be grouped
	groupableTypes := map[string]bool{
		NotifTypeLike: true,
		NotifTypeRetweet: true,
		NotifTypeFollow: false, // follow notifications are usually not grouped
		NotifTypeReply: true,
		NotifTypeMention: true,
	}
	return groupableTypes[n.Type]
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// NotificationBuilder helps construct notifications for testing.
type NotificationBuilder struct {
	notif *Notification
}

// NewNotificationBuilder creates a new notification builder.
func NewNotificationBuilder() *NotificationBuilder {
	return &NotificationBuilder{
		notif: &Notification{
			ID:          uuid.New().String(),
			UserID:      "",
			FromUserID:  "",
			Type:        NotifTypeSystem,
			ReferenceID: "",
			Read:        false,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *NotificationBuilder) WithID(id string) *NotificationBuilder {
	b.notif.ID = id
	return b
}

// WithUserID sets the user ID.
func (b *NotificationBuilder) WithUserID(userID string) *NotificationBuilder {
	b.notif.UserID = userID
	return b
}

// WithFromUserID sets the from user ID.
func (b *NotificationBuilder) WithFromUserID(fromUserID string) *NotificationBuilder {
	b.notif.FromUserID = fromUserID
	return b
}

// WithType sets the notification type.
func (b *NotificationBuilder) WithType(notifType string) *NotificationBuilder {
	b.notif.Type = notifType
	return b
}

// WithReferenceID sets the reference ID.
func (b *NotificationBuilder) WithReferenceID(refID string) *NotificationBuilder {
	b.notif.ReferenceID = refID
	return b
}

// WithRead marks as read.
func (b *NotificationBuilder) WithRead(read bool) *NotificationBuilder {
	b.notif.Read = read
	if read {
		now := time.Now()
		b.notif.ReadAt = &now
	}
	return b
}

// WithMetadata sets the metadata.
func (b *NotificationBuilder) WithMetadata(meta NotificationMetadata) *NotificationBuilder {
	b.notif.Metadata = meta
	return b
}

// WithCreatedAt sets the creation time.
func (b *NotificationBuilder) WithCreatedAt(t time.Time) *NotificationBuilder {
	b.notif.CreatedAt = t
	b.notif.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *NotificationBuilder) WithDeleted(t time.Time) *NotificationBuilder {
	b.notif.DeletedAt = &t
	return b
}

// Build validates and returns the notification.
func (b *NotificationBuilder) Build() (*Notification, error) {
	if err := b.notif.Validate(); err != nil {
		return nil, err
	}
	return b.notif, nil
}

// MustBuild builds without error (panics on error).
func (b *NotificationBuilder) MustBuild() *Notification {
	n, err := b.Build()
	if err != nil {
		panic(err)
	}
	return n
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestNotification1 = MustNewNotification("user1", "user2", NotifTypeLike, "tweet1")
	TestNotification2 = MustNewNotification("user1", "user3", NotifTypeFollow, "user3")
	TestNotification3 = MustNewNotification("user2", "user1", NotifTypeMention, "tweet2")
)