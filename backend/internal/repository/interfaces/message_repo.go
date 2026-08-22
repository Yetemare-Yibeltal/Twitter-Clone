// backend/internal/repository/interfaces/message_repo.go
package interfaces

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"twitter-clone/backend/internal/domain/entities"
)

// ======================================================================
// Common Errors
// ======================================================================

var (
	ErrMessageNotFound      = errors.New("message not found")
	ErrMessageDeleted       = errors.New("message has been deleted")
	ErrConversationNotFound = errors.New("conversation not found")
	ErrInvalidMessageID     = errors.New("invalid message ID")
	ErrInvalidMessageContent = errors.New("invalid message content")
	ErrDuplicateMessage     = errors.New("duplicate message")
	ErrMessageAlreadyRead   = errors.New("message already read")
)

// ======================================================================
= MessageFilter
// ======================================================================

// MessageFilter defines filtering options for messages.
type MessageFilter struct {
	SenderID   *string
	ReceiverID *string
	Read       *bool
	FromDate   *time.Time
	ToDate     *time.Time
	HasMedia   *bool
	Search     *string // full-text search on content
}

// ======================================================================
= MessagePagination
// ======================================================================

// MessagePagination holds pagination options for messages.
type MessagePagination struct {
	Limit  int
	Cursor string
	SortBy string // "created_at", "read_at"
	Order  string // "ASC", "DESC"
}

// ======================================================================
= Conversation
// ======================================================================

// Conversation represents a conversation between two users.
type Conversation struct {
	OtherUserID         string    `json:"other_user_id"`
	LastMessageID       string    `json:"last_message_id"`
	LastMessageContent  string    `json:"last_message_content"`
	LastMessageAt       time.Time `json:"last_message_at"`
	LastMessageRead     bool      `json:"last_message_read"`
	UnreadCount         int       `json:"unread_count"`
}

// ======================================================================
= MessageStats
// ======================================================================

// MessageStats holds aggregated message statistics.
type MessageStats struct {
	TotalSent     int64     `json:"total_sent"`
	TotalReceived int64     `json:"total_received"`
	UnreadCount   int64     `json:"unread_count"`
	ReadCount     int64     `json:"read_count"`
	LastMessageAt time.Time `json:"last_message_at"`
	FirstMessageAt time.Time `json:"first_message_at"`
}

// ======================================================================
= MessageRepository Interface
// ======================================================================

// MessageRepository defines the interface for message data persistence.
type MessageRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create inserts a new message.
	Create(ctx context.Context, message *entities.Message) error

	// GetByID retrieves a message by its ID.
	GetByID(ctx context.Context, id string) (*entities.Message, error)

	// Update updates a message (e.g., content edit).
	Update(ctx context.Context, message *entities.Message) error

	// SoftDelete marks a message as deleted.
	SoftDelete(ctx context.Context, id string) error

	// HardDelete permanently removes a message.
	HardDelete(ctx context.Context, id string) error

	// --------------------------------------------------------------------
	// Conversation Queries
	// --------------------------------------------------------------------

	// GetConversation returns messages between two users with pagination.
	GetConversation(ctx context.Context, user1ID, user2ID string, cursor string, limit int) ([]*entities.Message, string, error)

	// GetConversations returns a list of conversations for a user.
	GetConversations(ctx context.Context, userID string) ([]*Conversation, error)

	// GetConversationSummary returns summary of a conversation (last message, unread count).
	GetConversationSummary(ctx context.Context, user1ID, user2ID string) (*Conversation, error)

	// --------------------------------------------------------------------
	// Read Status Operations
	// --------------------------------------------------------------------

	// MarkAsRead marks a message as read.
	MarkAsRead(ctx context.Context, id string) error

	// MarkConversationAsRead marks all messages in a conversation as read.
	MarkConversationAsRead(ctx context.Context, userID, otherUserID string) error

	// MarkAllAsRead marks all messages for a user as read.
	MarkAllAsRead(ctx context.Context, userID string) error

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountUnread returns the total number of unread messages for a user.
	CountUnread(ctx context.Context, userID string) (int64, error)

	// CountUnreadFromUser returns unread messages from a specific sender.
	CountUnreadFromUser(ctx context.Context, userID, senderID string) (int64, error)

	// CountTotalConversations returns the number of distinct conversations for a user.
	CountTotalConversations(ctx context.Context, userID string) (int64, error)

	// CountTotalMessages returns total messages for a user.
	CountTotalMessages(ctx context.Context, userID string) (int64, error)

	// CountByDateRange returns message count within a date range.
	CountByDateRange(ctx context.Context, userID string, start, end time.Time) (int64, error)

	// --------------------------------------------------------------------
	// Advanced Queries
	// --------------------------------------------------------------------

	// GetLatestMessages returns the most recent messages for a user.
	GetLatestMessages(ctx context.Context, userID string, limit int) ([]*entities.Message, error)

	// GetMessagesByDateRange returns messages within a date range.
	GetMessagesByDateRange(ctx context.Context, userID, otherUserID string, start, end time.Time, cursor string, limit int) ([]*entities.Message, string, error)

	// GetUnreadMessages returns all unread messages for a user.
	GetUnreadMessages(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Message, string, error)

	// GetMessagesBySender returns messages from a specific sender.
	GetMessagesBySender(ctx context.Context, userID, senderID string, cursor string, limit int) ([]*entities.Message, string, error)

	// --------------------------------------------------------------------
	// Search
	// --------------------------------------------------------------------

	// SearchMessages searches messages by content for a user.
	SearchMessages(ctx context.Context, userID, query string, cursor string, limit int) ([]*entities.Message, string, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple messages in a transaction.
	BulkCreate(ctx context.Context, messages []*entities.Message) error

	// BulkDelete removes multiple messages.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkSoftDelete soft deletes multiple messages.
	BulkSoftDelete(ctx context.Context, ids []string) error

	// BulkMarkAsRead marks multiple messages as read.
	BulkMarkAsRead(ctx context.Context, ids []string) error

	// BulkDeleteByUserID removes all messages for a user.
	BulkDeleteByUserID(ctx context.Context, userID string) error

	// BulkDeleteConversation removes all messages between two users.
	BulkDeleteConversation(ctx context.Context, user1ID, user2ID string) error

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetMessageStats returns aggregated message statistics.
	GetMessageStats(ctx context.Context) (*MessageStats, error)

	// GetUserMessageStats returns message stats for a specific user.
	GetUserMessageStats(ctx context.Context, userID string) (*MessageStats, error)

	// GetDailyMessageStats returns daily message counts for a date range.
	GetDailyMessageStats(ctx context.Context, start, end time.Time) ([]*DailyMessageCount, error)

	// GetTopConversations returns the most active conversations for a user.
	GetTopConversations(ctx context.Context, userID string, limit int) ([]*TopConversation, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) MessageRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo MessageRepository) error) error

	// --------------------------------------------------------------------
	// Health and Cleanup
	// --------------------------------------------------------------------

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close releases any resources.
	Close() error

	// GetRawDB returns the underlying *sql.DB or *sqlx.DB.
	GetRawDB() interface{}
}

// ======================================================================
= DailyMessageCount
// ======================================================================

// DailyMessageCount represents daily message counts.
type DailyMessageCount struct {
	Date           time.Time `json:"date"`
	Total          int64     `json:"total"`
	UniqueSenders  int64     `json:"unique_senders"`
	UniqueReceivers int64    `json:"unique_receivers"`
	ReadCount      int64     `json:"read_count"`
	UnreadCount    int64     `json:"unread_count"`
}

// ======================================================================
= TopConversation
// ======================================================================

// TopConversation represents a top conversation.
type TopConversation struct {
	OtherUserID   string    `json:"other_user_id"`
	MessageCount  int64     `json:"message_count"`
	LastMessageAt time.Time `json:"last_message_at"`
}

// ======================================================================
= Helper Functions
// ======================================================================

// IsMessageNotFound checks if an error indicates a message was not found.
func IsMessageNotFound(err error) bool {
	return errors.Is(err, ErrMessageNotFound) || errors.Is(err, ErrMessageDeleted)
}

// IsMessageInvalid checks if an error indicates invalid message content.
func IsMessageInvalid(err error) bool {
	return errors.Is(err, ErrInvalidMessageContent) || errors.Is(err, ErrInvalidMessageID)
}

// ======================================================================
= Mock Message Repository (for testing)
// ======================================================================

// MockMessageRepository is a mock implementation for testing.
type MockMessageRepository struct {
	Messages     map[string]*entities.Message
	Conversations map[string][]*entities.Message
	Error        error
}

// NewMockMessageRepo creates a new mock repository.
func NewMockMessageRepo() MessageRepository {
	return &MockMessageRepository{
		Messages:     make(map[string]*entities.Message),
		Conversations: make(map[string][]*entities.Message),
	}
}

// Create mock implementation.
func (m *MockMessageRepository) Create(ctx context.Context, msg *entities.Message) error {
	if m.Error != nil {
		return m.Error
	}
	m.Messages[msg.ID] = msg
	return nil
}

// GetByID mock implementation.
func (m *MockMessageRepository) GetByID(ctx context.Context, id string) (*entities.Message, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if msg, ok := m.Messages[id]; ok {
		return msg, nil
	}
	return nil, ErrMessageNotFound
}

// Update mock implementation.
func (m *MockMessageRepository) Update(ctx context.Context, msg *entities.Message) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Messages[msg.ID]; !ok {
		return ErrMessageNotFound
	}
	m.Messages[msg.ID] = msg
	return nil
}

// SoftDelete mock implementation.
func (m *MockMessageRepository) SoftDelete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if msg, ok := m.Messages[id]; ok {
		now := time.Now()
		msg.DeletedAt = &now
		return nil
	}
	return ErrMessageNotFound
}

// HardDelete mock implementation.
func (m *MockMessageRepository) HardDelete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Messages[id]; ok {
		delete(m.Messages, id)
		return nil
	}
	return ErrMessageNotFound
}

// GetConversation mock implementation.
func (m *MockMessageRepository) GetConversation(ctx context.Context, user1ID, user2ID string, cursor string, limit int) ([]*entities.Message, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var msgs []*entities.Message
	for _, msg := range m.Messages {
		if (msg.SenderID == user1ID && msg.ReceiverID == user2ID) ||
			(msg.SenderID == user2ID && msg.ReceiverID == user1ID) {
			msgs = append(msgs, msg)
		}
	}
	return msgs, "", nil
}

// GetConversations mock implementation.
func (m *MockMessageRepository) GetConversations(ctx context.Context, userID string) ([]*Conversation, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	// Simplified: return empty list
	return []*Conversation{}, nil
}

// GetConversationSummary mock implementation.
func (m *MockMessageRepository) GetConversationSummary(ctx context.Context, user1ID, user2ID string) (*Conversation, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &Conversation{
		OtherUserID:        user2ID,
		LastMessageID:      "",
		LastMessageContent: "",
		LastMessageAt:      time.Now(),
		LastMessageRead:    false,
		UnreadCount:        0,
	}, nil
}

// MarkAsRead mock implementation.
func (m *MockMessageRepository) MarkAsRead(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if msg, ok := m.Messages[id]; ok {
		msg.Read = true
		now := time.Now()
		msg.ReadAt = &now
		return nil
	}
	return ErrMessageNotFound
}

// MarkConversationAsRead mock implementation.
func (m *MockMessageRepository) MarkConversationAsRead(ctx context.Context, userID, otherUserID string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, msg := range m.Messages {
		if msg.ReceiverID == userID && msg.SenderID == otherUserID {
			msg.Read = true
			now := time.Now()
			msg.ReadAt = &now
		}
	}
	return nil
}

// MarkAllAsRead mock implementation.
func (m *MockMessageRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, msg := range m.Messages {
		if msg.ReceiverID == userID {
			msg.Read = true
			now := time.Now()
			msg.ReadAt = &now
		}
	}
	return nil
}

// CountUnread mock implementation.
func (m *MockMessageRepository) CountUnread(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, msg := range m.Messages {
		if msg.ReceiverID == userID && !msg.Read && msg.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

// CountUnreadFromUser mock implementation.
func (m *MockMessageRepository) CountUnreadFromUser(ctx context.Context, userID, senderID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, msg := range m.Messages {
		if msg.ReceiverID == userID && msg.SenderID == senderID && !msg.Read && msg.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

// CountTotalConversations mock implementation.
func (m *MockMessageRepository) CountTotalConversations(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	unique := make(map[string]bool)
	for _, msg := range m.Messages {
		if msg.SenderID == userID && msg.DeletedAt == nil {
			unique[msg.ReceiverID] = true
		}
		if msg.ReceiverID == userID && msg.DeletedAt == nil {
			unique[msg.SenderID] = true
		}
	}
	return int64(len(unique)), nil
}

// CountTotalMessages mock implementation.
func (m *MockMessageRepository) CountTotalMessages(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, msg := range m.Messages {
		if (msg.SenderID == userID || msg.ReceiverID == userID) && msg.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

// CountByDateRange mock implementation.
func (m *MockMessageRepository) CountByDateRange(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, msg := range m.Messages {
		if (msg.SenderID == userID || msg.ReceiverID == userID) &&
			msg.CreatedAt.After(start) && msg.CreatedAt.Before(end) && msg.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

// GetLatestMessages mock implementation.
func (m *MockMessageRepository) GetLatestMessages(ctx context.Context, userID string, limit int) ([]*entities.Message, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var msgs []*entities.Message
	for _, msg := range m.Messages {
		if (msg.SenderID == userID || msg.ReceiverID == userID) && msg.DeletedAt == nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs, nil
}

// GetMessagesByDateRange mock implementation.
func (m *MockMessageRepository) GetMessagesByDateRange(ctx context.Context, userID, otherUserID string, start, end time.Time, cursor string, limit int) ([]*entities.Message, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var msgs []*entities.Message
	for _, msg := range m.Messages {
		if ((msg.SenderID == userID && msg.ReceiverID == otherUserID) ||
			(msg.SenderID == otherUserID && msg.ReceiverID == userID)) &&
			msg.CreatedAt.After(start) && msg.CreatedAt.Before(end) && msg.DeletedAt == nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs, "", nil
}

// GetUnreadMessages mock implementation.
func (m *MockMessageRepository) GetUnreadMessages(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Message, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var msgs []*entities.Message
	for _, msg := range m.Messages {
		if msg.ReceiverID == userID && !msg.Read && msg.DeletedAt == nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs, "", nil
}

// GetMessagesBySender mock implementation.
func (m *MockMessageRepository) GetMessagesBySender(ctx context.Context, userID, senderID string, cursor string, limit int) ([]*entities.Message, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var msgs []*entities.Message
	for _, msg := range m.Messages {
		if msg.ReceiverID == userID && msg.SenderID == senderID && msg.DeletedAt == nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs, "", nil
}

// SearchMessages mock implementation.
func (m *MockMessageRepository) SearchMessages(ctx context.Context, userID, query string, cursor string, limit int) ([]*entities.Message, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var msgs []*entities.Message
	for _, msg := range m.Messages {
		if (msg.SenderID == userID || msg.ReceiverID == userID) &&
			strings.Contains(msg.Content, query) && msg.DeletedAt == nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs, "", nil
}

// BulkCreate mock implementation.
func (m *MockMessageRepository) BulkCreate(ctx context.Context, messages []*entities.Message) error {
	if m.Error != nil {
		return m.Error
	}
	for _, msg := range messages {
		m.Messages[msg.ID] = msg
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockMessageRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		delete(m.Messages, id)
	}
	return nil
}

// BulkSoftDelete mock implementation.
func (m *MockMessageRepository) BulkSoftDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	now := time.Now()
	for _, id := range ids {
		if msg, ok := m.Messages[id]; ok {
			msg.DeletedAt = &now
		}
	}
	return nil
}

// BulkMarkAsRead mock implementation.
func (m *MockMessageRepository) BulkMarkAsRead(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	now := time.Now()
	for _, id := range ids {
		if msg, ok := m.Messages[id]; ok {
			msg.Read = true
			msg.ReadAt = &now
		}
	}
	return nil
}

// BulkDeleteByUserID mock implementation.
func (m *MockMessageRepository) BulkDeleteByUserID(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, msg := range m.Messages {
		if msg.SenderID == userID || msg.ReceiverID == userID {
			delete(m.Messages, id)
		}
	}
	return nil
}

// BulkDeleteConversation mock implementation.
func (m *MockMessageRepository) BulkDeleteConversation(ctx context.Context, user1ID, user2ID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, msg := range m.Messages {
		if (msg.SenderID == user1ID && msg.ReceiverID == user2ID) ||
			(msg.SenderID == user2ID && msg.ReceiverID == user1ID) {
			delete(m.Messages, id)
		}
	}
	return nil
}

// GetMessageStats mock implementation.
func (m *MockMessageRepository) GetMessageStats(ctx context.Context) (*MessageStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &MessageStats{
		TotalSent:     0,
		TotalReceived: 0,
		UnreadCount:   0,
		ReadCount:     0,
		LastMessageAt: time.Now(),
		FirstMessageAt: time.Now(),
	}, nil
}

// GetUserMessageStats mock implementation.
func (m *MockMessageRepository) GetUserMessageStats(ctx context.Context, userID string) (*MessageStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var stats MessageStats
	for _, msg := range m.Messages {
		if msg.DeletedAt == nil {
			if msg.SenderID == userID {
				stats.TotalSent++
			}
			if msg.ReceiverID == userID {
				stats.TotalReceived++
				if msg.Read {
					stats.ReadCount++
				} else {
					stats.UnreadCount++
				}
			}
		}
	}
	return &stats, nil
}

// GetDailyMessageStats mock implementation.
func (m *MockMessageRepository) GetDailyMessageStats(ctx context.Context, start, end time.Time) ([]*DailyMessageCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyMessageCount{}, nil
}

// GetTopConversations mock implementation.
func (m *MockMessageRepository) GetTopConversations(ctx context.Context, userID string, limit int) ([]*TopConversation, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*TopConversation{}, nil
}

// WithTransaction mock implementation.
func (m *MockMessageRepository) WithTransaction(ctx context.Context, tx *sql.Tx) MessageRepository {
	return m
}

// Transaction mock implementation.
func (m *MockMessageRepository) Transaction(ctx context.Context, fn func(txRepo MessageRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockMessageRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockMessageRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockMessageRepository) GetRawDB() interface{} {
	return nil
}