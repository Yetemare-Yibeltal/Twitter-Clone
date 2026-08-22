// backend/internal/domain/entities/message.go
package entities

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"regexp"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	MaxMessageContentLength = 5000
	MaxMessageMediaCount    = 10
	MaxMessageMediaURLSize  = 2048
)

var (
	ErrEmptyMessageContent   = errors.New("message content cannot be empty")
	ErrMessageContentTooLong = fmt.Errorf("message content exceeds maximum length of %d characters", MaxMessageContentLength)
	ErrMessageMediaTooMany   = fmt.Errorf("maximum %d media files allowed", MaxMessageMediaCount)
	ErrMessageMediaURLInvalid = errors.New("invalid media URL")
	ErrMessageSenderEmpty    = errors.New("sender ID cannot be empty")
	ErrMessageReceiverEmpty  = errors.New("receiver ID cannot be empty")
	ErrMessageSelfSend       = errors.New("cannot send a message to yourself")
	ErrMessageAlreadyRead    = errors.New("message already marked as read")
	ErrMessageAlreadyDeleted = errors.New("message already deleted")
)

// ======================================================================
= Message Entity
// ======================================================================

// Message represents a direct message between two users.
type Message struct {
	// Primary identifiers
	ID         string    `db:"id" json:"id"`
	SenderID   string    `db:"sender_id" json:"sender_id"`
	ReceiverID string    `db:"receiver_id" json:"receiver_id"`

	// Content
	Content   string   `db:"content" json:"content"`
	MediaURLs []string `db:"media_urls" json:"media_urls"`

	// Read status
	Read   bool       `db:"read" json:"read"`
	ReadAt *time.Time `db:"read_at" json:"read_at,omitempty"`

	// Metadata (for future extensions)
	Metadata MessageMetadata `db:"metadata" json:"metadata"`

	// Timestamps
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// MessageMetadata holds optional metadata.
type MessageMetadata struct {
	ReplyToID     string            `json:"reply_to_id,omitempty"`
	ForwardedFrom string            `json:"forwarded_from,omitempty"`
	IsEdited      bool              `json:"is_edited"`
	EditedAt      *time.Time        `json:"edited_at,omitempty"`
	CustomData    map[string]string `json:"custom_data,omitempty"`
}

// Value implements driver.Valuer for JSON storage.
func (m MessageMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements sql.Scanner for JSON retrieval.
func (m *MessageMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = MessageMetadata{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for MessageMetadata: %T", value)
	}
	return json.Unmarshal(bytes, m)
}

// ======================================================================
= Factory and Validation
// ======================================================================

// NewMessage creates a new Message instance with validation.
func NewMessage(senderID, receiverID, content string, mediaURLs []string) (*Message, error) {
	msg := &Message{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
		MediaURLs:  mediaURLs,
		Read:       false,
		Metadata:   MessageMetadata{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	return msg, nil
}

// Validate performs comprehensive validation.
func (m *Message) Validate() error {
	// Sender and receiver
	if strings.TrimSpace(m.SenderID) == "" {
		return ErrMessageSenderEmpty
	}
	if strings.TrimSpace(m.ReceiverID) == "" {
		return ErrMessageReceiverEmpty
	}
	if m.SenderID == m.ReceiverID {
		return ErrMessageSelfSend
	}

	// Content
	contentTrimmed := strings.TrimSpace(m.Content)
	if contentTrimmed == "" && len(m.MediaURLs) == 0 {
		return ErrEmptyMessageContent
	}
	if len(contentTrimmed) > MaxMessageContentLength {
		return ErrMessageContentTooLong
	}
	m.Content = contentTrimmed // store trimmed content

	// Media URLs
	if len(m.MediaURLs) > MaxMessageMediaCount {
		return ErrMessageMediaTooMany
	}
	for _, url := range m.MediaURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			return ErrMessageMediaURLInvalid
		}
		if !isValidMessageMediaURL(url) {
			return ErrMessageMediaURLInvalid
		}
	}
	// No need to modify URLs, but we might want to store trimmed.
	cleanedURLs := make([]string, 0, len(m.MediaURLs))
	for _, url := range m.MediaURLs {
		if trimmed := strings.TrimSpace(url); trimmed != "" {
			cleanedURLs = append(cleanedURLs, trimmed)
		}
	}
	m.MediaURLs = cleanedURLs

	// If we have content but it's empty after trim, but we have media, we allow empty content.
	// Already handled above.

	return nil
}

// isValidMessageMediaURL validates media URL format (basic).
func isValidMessageMediaURL(url string) bool {
	if len(url) > MaxMessageMediaURLSize {
		return false
	}
	// Allow http/https and relative paths
	re := regexp.MustCompile(`^(https?://|/)[^\s]+$`)
	return re.MatchString(url)
}

// ======================================================================
= Business Logic Methods
// ======================================================================

// MarkAsRead marks the message as read.
func (m *Message) MarkAsRead() error {
	if m.DeletedAt != nil {
		return ErrMessageAlreadyDeleted
	}
	if m.Read {
		return ErrMessageAlreadyRead
	}
	m.Read = true
	now := time.Now()
	m.ReadAt = &now
	m.UpdatedAt = now
	return nil
}

// MarkAsUnread reverts the read status.
func (m *Message) MarkAsUnread() error {
	if m.DeletedAt != nil {
		return ErrMessageAlreadyDeleted
	}
	m.Read = false
	m.ReadAt = nil
	m.UpdatedAt = time.Now()
	return nil
}

// EditContent updates the message content.
func (m *Message) EditContent(newContent string) error {
	if m.DeletedAt != nil {
		return ErrMessageAlreadyDeleted
	}
	oldContent := m.Content
	m.Content = strings.TrimSpace(newContent)
	if err := m.Validate(); err != nil {
		// revert on validation failure
		m.Content = oldContent
		return err
	}
	m.Metadata.IsEdited = true
	now := time.Now()
	m.Metadata.EditedAt = &now
	m.UpdatedAt = now
	return nil
}

// SoftDelete marks the message as deleted.
func (m *Message) SoftDelete() error {
	if m.DeletedAt != nil {
		return ErrMessageAlreadyDeleted
	}
	now := time.Now()
	m.DeletedAt = &now
	m.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted message.
func (m *Message) Restore() error {
	if m.DeletedAt == nil {
		return errors.New("message is not deleted")
	}
	m.DeletedAt = nil
	m.UpdatedAt = time.Now()
	return nil
}

// IsSender checks if the given user ID is the sender.
func (m *Message) IsSender(userID string) bool {
	return m.SenderID == userID
}

// IsReceiver checks if the given user ID is the receiver.
func (m *Message) IsReceiver(userID string) bool {
	return m.ReceiverID == userID
}

// IsParticipant checks if the user is either sender or receiver.
func (m *Message) IsParticipant(userID string) bool {
	return m.SenderID == userID || m.ReceiverID == userID
}

// GetOtherParticipant returns the other participant ID.
func (m *Message) GetOtherParticipant(userID string) (string, error) {
	if !m.IsParticipant(userID) {
		return "", errors.New("user is not a participant in this message")
	}
	if m.SenderID == userID {
		return m.ReceiverID, nil
	}
	return m.SenderID, nil
}

// ======================================================================
= Formatting and Utilities
// ======================================================================

// String returns a human-readable representation.
func (m *Message) String() string {
	return fmt.Sprintf("Message{ID:%s, from:%s, to:%s, content:%q, read:%v}",
		m.ID, m.SenderID, m.ReceiverID, m.Content, m.Read)
}

// Preview returns a shortened preview of the content.
func (m *Message) Preview(maxLen int) string {
	if maxLen <= 0 {
		maxLen = 50
	}
	if len(m.Content) <= maxLen {
		return m.Content
	}
	return m.Content[:maxLen] + "..."
}

// IsEmpty returns true if the message is zero value.
func (m *Message) IsEmpty() bool {
	return m.ID == "" && m.SenderID == "" && m.ReceiverID == "" && m.Content == ""
}

// Clone returns a deep copy of the message.
func (m *Message) Clone() *Message {
	clone := *m
	if m.ReadAt != nil {
		t := *m.ReadAt
		clone.ReadAt = &t
	}
	if m.DeletedAt != nil {
		t := *m.DeletedAt
		clone.DeletedAt = &t
	}
	if m.Metadata.EditedAt != nil {
		t := *m.Metadata.EditedAt
		clone.Metadata.EditedAt = &t
	}
	clone.MediaURLs = make([]string, len(m.MediaURLs))
	copy(clone.MediaURLs, m.MediaURLs)
	return &clone
}

// ======================================================================
= JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling (omit sensitive fields if needed).
func (m *Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	return json.Unmarshal(data, aux)
}

// ======================================================================
= Comparison
// ======================================================================

// Equals checks if two messages are the same by ID.
func (m *Message) Equals(other *Message) bool {
	return m.ID == other.ID
}

// ======================================================================
= Value Objects for Message Status
// ======================================================================

// MessageStatus represents the status of a message.
type MessageStatus string

const (
	MessageStatusSent     MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusDeleted   MessageStatus = "deleted"
)

// GetStatus returns the current status of the message.
func (m *Message) GetStatus() MessageStatus {
	if m.DeletedAt != nil {
		return MessageStatusDeleted
	}
	if m.Read {
		return MessageStatusRead
	}
	// For simplicity, we treat any sent message as "delivered" if not read
	// In a real system, we might have a separate delivery flag.
	return MessageStatusDelivered
}

// ======================================================================
= Message Builder (for tests)
// ======================================================================

// MessageBuilder helps construct messages for testing.
type MessageBuilder struct {
	msg *Message
}

// NewMessageBuilder creates a new builder.
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{
		msg: &Message{
			ID:         "",
			SenderID:   "",
			ReceiverID: "",
			Content:    "",
			MediaURLs:  []string{},
			Read:       false,
			Metadata:   MessageMetadata{},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *MessageBuilder) WithID(id string) *MessageBuilder {
	b.msg.ID = id
	return b
}

// WithSender sets the sender ID.
func (b *MessageBuilder) WithSender(senderID string) *MessageBuilder {
	b.msg.SenderID = senderID
	return b
}

// WithReceiver sets the receiver ID.
func (b *MessageBuilder) WithReceiver(receiverID string) *MessageBuilder {
	b.msg.ReceiverID = receiverID
	return b
}

// WithContent sets the content.
func (b *MessageBuilder) WithContent(content string) *MessageBuilder {
	b.msg.Content = content
	return b
}

// WithMedia adds media URLs.
func (b *MessageBuilder) WithMedia(urls ...string) *MessageBuilder {
	b.msg.MediaURLs = append(b.msg.MediaURLs, urls...)
	return b
}

// WithRead marks as read.
func (b *MessageBuilder) WithRead(read bool) *MessageBuilder {
	b.msg.Read = read
	if read {
		now := time.Now()
		b.msg.ReadAt = &now
	}
	return b
}

// WithMetadata sets metadata.
func (b *MessageBuilder) WithMetadata(meta MessageMetadata) *MessageBuilder {
	b.msg.Metadata = meta
	return b
}

// WithCreatedAt sets creation time.
func (b *MessageBuilder) WithCreatedAt(t time.Time) *MessageBuilder {
	b.msg.CreatedAt = t
	b.msg.UpdatedAt = t
	return b
}

// Build validates and returns the message.
func (b *MessageBuilder) Build() (*Message, error) {
	if err := b.msg.Validate(); err != nil {
		return nil, err
	}
	return b.msg, nil
}

// MustBuild builds without error (panics on error).
func (b *MessageBuilder) MustBuild() *Message {
	msg, err := b.Build()
	if err != nil {
		panic(err)
	}
	return msg
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestMessage1 = MustNewMessage("user1", "user2", "Hello", []string{})
	TestMessage2 = MustNewMessage("user2", "user1", "Hi there", []string{"https://example.com/img.jpg"})
	TestMessage3 = MustNewMessage("user1", "user3", "How are you?", []string{})
)

// MustNewMessage is a convenience for tests.
func MustNewMessage(senderID, receiverID, content string, mediaURLs []string) *Message {
	msg, err := NewMessage(senderID, receiverID, content, mediaURLs)
	if err != nil {
		panic(err)
	}
	return msg
}