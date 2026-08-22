// backend/internal/repository/postgres/message_pg.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// messageRepo is the PostgreSQL implementation of MessageRepository.
type messageRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewMessageRepository creates a new PostgreSQL message repository.
func NewMessageRepository(db *sqlx.DB) interfaces.MessageRepository {
	return &messageRepo{
		db:  db,
		log: logger.WithField("repository", "message_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *messageRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.MessageRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &messageRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *messageRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.MessageRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &messageRepo{
		db:  r.db,
		tx:  tx,
		log: r.log.WithField("transaction", true),
	}
	err = fn(txRepo)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed after error: %v (original: %w)", rbErr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// getDB returns the current DB connection.
func (r *messageRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic Message CRUD
// ======================================================================

// Create inserts a new message.
func (r *messageRepo) Create(ctx context.Context, message *entities.Message) error {
	metadataJSON, err := json.Marshal(message.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	query := `
		INSERT INTO messages (
			id, sender_id, receiver_id, content, media_urls,
			read, read_at, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = r.getDB().ExecContext(ctx, query,
		message.ID, message.SenderID, message.ReceiverID, message.Content,
		pq.Array(message.MediaURLs), message.Read, message.ReadAt,
		metadataJSON, message.CreatedAt, message.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create message failed: %w", err)
	}
	return nil
}

// GetByID retrieves a message by its ID.
func (r *messageRepo) GetByID(ctx context.Context, id string) (*entities.Message, error) {
	query := `SELECT * FROM messages WHERE id = $1 AND deleted_at IS NULL`
	var message entities.Message
	err := r.getDB().GetContext(ctx, &message, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrMessageNotFound
		}
		return nil, fmt.Errorf("get message by ID failed: %w", err)
	}
	return &message, nil
}

// Update updates a message (e.g., content edit).
func (r *messageRepo) Update(ctx context.Context, message *entities.Message) error {
	metadataJSON, err := json.Marshal(message.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	query := `
		UPDATE messages SET
			content = $1,
			media_urls = $2,
			metadata = $3,
			updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL
	`
	result, err := r.getDB().ExecContext(ctx, query,
		message.Content, pq.Array(message.MediaURLs), metadataJSON,
		time.Now(), message.ID,
	)
	if err != nil {
		return fmt.Errorf("update message failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrMessageNotFound
	}
	return nil
}

// SoftDelete marks a message as deleted (for both users).
func (r *messageRepo) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE messages SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("soft delete message failed: %w", err)
	}
	return nil
}

// HardDelete permanently removes a message.
func (r *messageRepo) HardDelete(ctx context.Context, id string) error {
	query := `DELETE FROM messages WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("hard delete message failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrMessageNotFound
	}
	return nil
}

// ======================================================================
// Conversation Queries
// ======================================================================

// GetConversation returns messages between two users with pagination.
func (r *messageRepo) GetConversation(ctx context.Context, user1ID, user2ID string, cursor string, limit int) ([]*entities.Message, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM messages
		WHERE (
			(sender_id = $1 AND receiver_id = $2) OR
			(sender_id = $2 AND receiver_id = $1)
		)
		AND deleted_at IS NULL
	`
	if cursor != "" {
		// Cursor format: "timestamp|id" or just id for simplicity
		// We'll use created_at and id
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				query += ` AND (created_at < $3 OR (created_at = $3 AND id < $4))`
			}
		}
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{user1ID, user2ID}
	argIdx := 3
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				args = append(args, ts, parts[1])
				argIdx = 5
			}
		}
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var messages []*entities.Message
	err := r.getDB().SelectContext(ctx, &messages, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get conversation failed: %w", err)
	}

	var nextCursor string
	if len(messages) == limit {
		last := messages[len(messages)-1]
		nextCursor = fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	return messages, nextCursor, nil
}

// GetConversations returns a list of conversations for a user.
func (r *messageRepo) GetConversations(ctx context.Context, userID string) ([]*entities.Conversation, error) {
	// We need to find the distinct other party and the last message for each.
	query := `
		SELECT 
			CASE 
				WHEN sender_id = $1 THEN receiver_id
				WHEN receiver_id = $1 THEN sender_id
			END AS other_user_id,
			MAX(created_at) as last_message_at,
			(SELECT id FROM messages m2 
			 WHERE (m2.sender_id = $1 AND m2.receiver_id = other_user_id)
				OR (m2.sender_id = other_user_id AND m2.receiver_id = $1)
			 AND m2.deleted_at IS NULL
			 ORDER BY m2.created_at DESC LIMIT 1) as last_message_id,
			(SELECT content FROM messages m3 WHERE m3.id = last_message_id) as last_message_content,
			(SELECT read FROM messages m3 WHERE m3.id = last_message_id) as last_message_read,
			COUNT(CASE WHEN read = false AND receiver_id = $1 THEN 1 END) as unread_count
		FROM messages
		WHERE (sender_id = $1 OR receiver_id = $1)
		  AND deleted_at IS NULL
		GROUP BY other_user_id
		ORDER BY last_message_at DESC
	`
	var results []struct {
		OtherUserID       string    `db:"other_user_id"`
		LastMessageAt     time.Time `db:"last_message_at"`
		LastMessageID     string    `db:"last_message_id"`
		LastMessageContent string   `db:"last_message_content"`
		LastMessageRead   bool      `db:"last_message_read"`
		UnreadCount       int       `db:"unread_count"`
	}
	err := r.getDB().SelectContext(ctx, &results, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get conversations failed: %w", err)
	}

	conversations := make([]*entities.Conversation, 0, len(results))
	for _, r := range results {
		conv := &entities.Conversation{
			OtherUserID:       r.OtherUserID,
			LastMessageID:     r.LastMessageID,
			LastMessageContent: r.LastMessageContent,
			LastMessageAt:     r.LastMessageAt,
			LastMessageRead:   r.LastMessageRead,
			UnreadCount:       r.UnreadCount,
		}
		conversations = append(conversations, conv)
	}
	return conversations, nil
}

// GetConversationSummary returns summary of a conversation (last message, unread count).
func (r *messageRepo) GetConversationSummary(ctx context.Context, user1ID, user2ID string) (*entities.Conversation, error) {
	query := `
		SELECT 
			$2 as other_user_id,
			MAX(created_at) as last_message_at,
			(SELECT id FROM messages m2 
			 WHERE (m2.sender_id = $1 AND m2.receiver_id = $2)
				OR (m2.sender_id = $2 AND m2.receiver_id = $1)
			 AND m2.deleted_at IS NULL
			 ORDER BY m2.created_at DESC LIMIT 1) as last_message_id,
			(SELECT content FROM messages m3 WHERE m3.id = last_message_id) as last_message_content,
			(SELECT read FROM messages m3 WHERE m3.id = last_message_id) as last_message_read,
			COUNT(CASE WHEN read = false AND receiver_id = $1 THEN 1 END) as unread_count
		FROM messages
		WHERE (sender_id = $1 AND receiver_id = $2)
		   OR (sender_id = $2 AND receiver_id = $1)
		  AND deleted_at IS NULL
	`
	var result struct {
		OtherUserID       string    `db:"other_user_id"`
		LastMessageAt     time.Time `db:"last_message_at"`
		LastMessageID     string    `db:"last_message_id"`
		LastMessageContent string   `db:"last_message_content"`
		LastMessageRead   bool      `db:"last_message_read"`
		UnreadCount       int       `db:"unread_count"`
	}
	err := r.getDB().GetContext(ctx, &result, query, user1ID, user2ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get conversation summary failed: %w", err)
	}
	return &entities.Conversation{
		OtherUserID:       result.OtherUserID,
		LastMessageID:     result.LastMessageID,
		LastMessageContent: result.LastMessageContent,
		LastMessageAt:     result.LastMessageAt,
		LastMessageRead:   result.LastMessageRead,
		UnreadCount:       result.UnreadCount,
	}, nil
}

// ======================================================================
= Read Status Operations
// ======================================================================

// MarkAsRead marks a message as read.
func (r *messageRepo) MarkAsRead(ctx context.Context, id string) error {
	query := `UPDATE messages SET read = true, read_at = $1 WHERE id = $2 AND deleted_at IS NULL`
	result, err := r.getDB().ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("mark message as read failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrMessageNotFound
	}
	return nil
}

// MarkConversationAsRead marks all messages in a conversation as read.
func (r *messageRepo) MarkConversationAsRead(ctx context.Context, userID, otherUserID string) error {
	query := `
		UPDATE messages
		SET read = true, read_at = $1
		WHERE receiver_id = $2 AND sender_id = $3 AND read = false AND deleted_at IS NULL
	`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), userID, otherUserID)
	if err != nil {
		return fmt.Errorf("mark conversation as read failed: %w", err)
	}
	return nil
}

// MarkAllAsRead marks all messages for a user as read.
func (r *messageRepo) MarkAllAsRead(ctx context.Context, userID string) error {
	query := `
		UPDATE messages
		SET read = true, read_at = $1
		WHERE receiver_id = $2 AND read = false AND deleted_at IS NULL
	`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("mark all messages as read failed: %w", err)
	}
	return nil
}

// ======================================================================
= Count Operations
// ======================================================================

// CountUnread returns the total number of unread messages for a user.
func (r *messageRepo) CountUnread(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM messages WHERE receiver_id = $1 AND read = false AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread messages failed: %w", err)
	}
	return count, nil
}

// CountUnreadFromUser returns unread messages from a specific sender.
func (r *messageRepo) CountUnreadFromUser(ctx context.Context, userID, senderID string) (int64, error) {
	query := `
		SELECT COUNT(*) 
		FROM messages 
		WHERE receiver_id = $1 AND sender_id = $2 AND read = false AND deleted_at IS NULL
	`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, senderID)
	if err != nil {
		return 0, fmt.Errorf("count unread from user failed: %w", err)
	}
	return count, nil
}

// CountTotalConversations returns the number of distinct conversations for a user.
func (r *messageRepo) CountTotalConversations(ctx context.Context, userID string) (int64, error) {
	query := `
		SELECT COUNT(DISTINCT 
			CASE 
				WHEN sender_id = $1 THEN receiver_id
				WHEN receiver_id = $1 THEN sender_id
			END
		) FROM messages WHERE (sender_id = $1 OR receiver_id = $1) AND deleted_at IS NULL
	`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count total conversations failed: %w", err)
	}
	return count, nil
}

// CountTotalMessages returns total messages for a user.
func (r *messageRepo) CountTotalMessages(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM messages WHERE (sender_id = $1 OR receiver_id = $1) AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count total messages failed: %w", err)
	}
	return count, nil
}

// ======================================================================
= Advanced Queries
// ======================================================================

// GetLatestMessages returns the most recent messages for a user.
func (r *messageRepo) GetLatestMessages(ctx context.Context, userID string, limit int) ([]*entities.Message, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT * FROM messages
		WHERE (sender_id = $1 OR receiver_id = $1)
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2
	`
	var messages []*entities.Message
	err := r.getDB().SelectContext(ctx, &messages, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get latest messages failed: %w", err)
	}
	return messages, nil
}

// GetMessagesByDateRange returns messages within a date range.
func (r *messageRepo) GetMessagesByDateRange(ctx context.Context, userID, otherUserID string, start, end time.Time, cursor string, limit int) ([]*entities.Message, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM messages
		WHERE (
			(sender_id = $1 AND receiver_id = $2) OR
			(sender_id = $2 AND receiver_id = $1)
		)
		AND created_at >= $3 AND created_at <= $4
		AND deleted_at IS NULL
	`
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				query += ` AND (created_at < $5 OR (created_at = $5 AND id < $6))`
			}
		}
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, otherUserID, start, end}
	argIdx := 5
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				args = append(args, ts, parts[1])
				argIdx = 7
			}
		}
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var messages []*entities.Message
	err := r.getDB().SelectContext(ctx, &messages, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get messages by date range failed: %w", err)
	}

	var nextCursor string
	if len(messages) == limit {
		last := messages[len(messages)-1]
		nextCursor = fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	return messages, nextCursor, nil
}

// ======================================================================
= Bulk Operations
// ======================================================================

// BulkCreate inserts multiple messages in a transaction.
func (r *messageRepo) BulkCreate(ctx context.Context, messages []*entities.Message) error {
	if len(messages) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO messages (
			id, sender_id, receiver_id, content, media_urls,
			read, read_at, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, msg := range messages {
		metadataJSON, err := json.Marshal(msg.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata failed: %w", err)
		}
		_, err = stmt.ExecContext(ctx,
			msg.ID, msg.SenderID, msg.ReceiverID, msg.Content,
			pq.Array(msg.MediaURLs), msg.Read, msg.ReadAt,
			metadataJSON, msg.CreatedAt, msg.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("bulk create message failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple messages.
func (r *messageRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM messages WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete messages failed: %w", err)
	}
	return nil
}

// BulkSoftDelete soft deletes multiple messages.
func (r *messageRepo) BulkSoftDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE messages SET deleted_at = $1 WHERE id IN (?)`, time.Now(), ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk soft delete messages failed: %w", err)
	}
	return nil
}

// BulkMarkAsRead marks multiple messages as read.
func (r *messageRepo) BulkMarkAsRead(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE messages SET read = true, read_at = $1 WHERE id IN (?)`, time.Now(), ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk mark messages as read failed: %w", err)
	}
	return nil
}

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetMessageStats returns aggregated message statistics.
func (r *messageRepo) GetMessageStats(ctx context.Context) (*MessageStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_messages,
			COUNT(DISTINCT sender_id) as unique_senders,
			COUNT(DISTINCT receiver_id) as unique_receivers,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count,
			MAX(created_at) as latest_message,
			MIN(created_at) as earliest_message,
			AVG(LENGTH(content)) as avg_length
		FROM messages
		WHERE deleted_at IS NULL
	`
	var stats MessageStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get message stats failed: %w", err)
	}
	return &stats, nil
}

// MessageStats represents aggregated message statistics.
type MessageStats struct {
	TotalMessages    int64     `db:"total_messages"`
	UniqueSenders    int64     `db:"unique_senders"`
	UniqueReceivers  int64     `db:"unique_receivers"`
	ReadCount        int64     `db:"read_count"`
	UnreadCount      int64     `db:"unread_count"`
	LatestMessage    time.Time `db:"latest_message"`
	EarliestMessage  time.Time `db:"earliest_message"`
	AvgLength        *float64  `db:"avg_length"`
}

// GetDailyMessageStats returns daily message counts.
func (r *messageRepo) GetDailyMessageStats(ctx context.Context, start, end time.Time) ([]*DailyMessageCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			COUNT(DISTINCT sender_id) as unique_senders,
			COUNT(DISTINCT receiver_id) as unique_receivers,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count
		FROM messages
		WHERE created_at >= $1 AND created_at <= $2 AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyMessageCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily message stats failed: %w", err)
	}
	return results, nil
}

// DailyMessageCount represents daily message counts.
type DailyMessageCount struct {
	Date           time.Time `db:"date"`
	Total          int64     `db:"total"`
	UniqueSenders  int64     `db:"unique_senders"`
	UniqueReceivers int64    `db:"unique_receivers"`
	ReadCount      int64     `db:"read_count"`
	UnreadCount    int64     `db:"unread_count"`
}

// GetUserMessageStats returns message stats for a specific user.
func (r *messageRepo) GetUserMessageStats(ctx context.Context, userID string) (*UserMessageStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_sent,
			SUM(CASE WHEN read = true AND receiver_id = $1 THEN 1 ELSE 0 END) as read_by_receiver,
			COUNT(CASE WHEN receiver_id = $1 THEN 1 END) as total_received,
			SUM(CASE WHEN receiver_id = $1 AND read = false THEN 1 ELSE 0 END) as unread_received,
			MAX(created_at) as latest_sent,
			(SELECT MAX(created_at) FROM messages WHERE receiver_id = $1 AND deleted_at IS NULL) as latest_received
		FROM messages
		WHERE sender_id = $1 AND deleted_at IS NULL
	`
	var stats UserMessageStats
	err := r.getDB().GetContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user message stats failed: %w", err)
	}
	return &stats, nil
}

// UserMessageStats represents message statistics for a user.
type UserMessageStats struct {
	TotalSent       int64     `db:"total_sent"`
	ReadByReceiver  int64     `db:"read_by_receiver"`
	TotalReceived   int64     `db:"total_received"`
	UnreadReceived  int64     `db:"unread_received"`
	LatestSent      time.Time `db:"latest_sent"`
	LatestReceived  time.Time `db:"latest_received"`
}

// GetTopConversations returns the most active conversations for a user.
func (r *messageRepo) GetTopConversations(ctx context.Context, userID string, limit int) ([]*TopConversation, error) {
	if limit < 1 {
		limit = 5
	}
	query := `
		SELECT 
			CASE 
				WHEN sender_id = $1 THEN receiver_id
				WHEN receiver_id = $1 THEN sender_id
			END AS other_user_id,
			COUNT(*) as message_count,
			MAX(created_at) as last_message_at
		FROM messages
		WHERE (sender_id = $1 OR receiver_id = $1)
		  AND deleted_at IS NULL
		GROUP BY other_user_id
		ORDER BY message_count DESC, last_message_at DESC
		LIMIT $2
	`
	var results []*TopConversation
	err := r.getDB().SelectContext(ctx, &results, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get top conversations failed: %w", err)
	}
	return results, nil
}

// TopConversation represents a top conversation.
type TopConversation struct {
	OtherUserID   string    `db:"other_user_id"`
	MessageCount  int64     `db:"message_count"`
	LastMessageAt time.Time `db:"last_message_at"`
}

// ======================================================================
= Health
// ======================================================================

func (r *messageRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *messageRepo) Close() error {
	return nil
}

func (r *messageRepo) GetRawDB() interface{} {
	return r.db
}