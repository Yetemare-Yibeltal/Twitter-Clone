// backend/internal/repository/postgres/notification_pg.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// notificationRepo is the PostgreSQL implementation of NotificationRepository.
type notificationRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewNotificationRepository creates a new PostgreSQL notification repository.
func NewNotificationRepository(db *sqlx.DB) interfaces.NotificationRepository {
	return &notificationRepo{
		db:  db,
		log: logger.WithField("repository", "notification_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *notificationRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.NotificationRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &notificationRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *notificationRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.NotificationRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &notificationRepo{
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
func (r *notificationRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic CRUD
// ======================================================================

// Create inserts a new notification.
func (r *notificationRepo) Create(ctx context.Context, notification *interfaces.Notification) error {
	metadataJSON, err := json.Marshal(notification.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	query := `
		INSERT INTO notifications (
			id, user_id, from_user_id, type, reference_id,
			read, read_at, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = r.getDB().ExecContext(ctx, query,
		notification.ID, notification.UserID, notification.FromUserID,
		notification.Type, notification.ReferenceID,
		notification.Read, notification.ReadAt,
		metadataJSON, notification.CreatedAt, notification.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create notification failed: %w", err)
	}
	return nil
}

// GetByID retrieves a notification by its ID.
func (r *notificationRepo) GetByID(ctx context.Context, id string) (*interfaces.Notification, error) {
	query := `SELECT * FROM notifications WHERE id = $1 AND deleted_at IS NULL`
	var notification interfaces.Notification
	err := r.getDB().GetContext(ctx, &notification, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrNotificationNotFound
		}
		return nil, fmt.Errorf("get notification by ID failed: %w", err)
	}
	return &notification, nil
}

// GetByUserAndType retrieves notifications by user and type.
func (r *notificationRepo) GetByUserAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND type = $2 AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, notificationType}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get notifications by user and type failed: %w", err)
	}
	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetByReferenceID retrieves notifications by reference ID.
func (r *notificationRepo) GetByReferenceID(ctx context.Context, referenceID string) ([]*interfaces.Notification, error) {
	query := `SELECT * FROM notifications WHERE reference_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC`
	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, referenceID)
	if err != nil {
		return nil, fmt.Errorf("get notifications by reference ID failed: %w", err)
	}
	return notifications, nil
}

// Update updates a notification (e.g., read status).
func (r *notificationRepo) Update(ctx context.Context, notification *interfaces.Notification) error {
	metadataJSON, err := json.Marshal(notification.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	query := `
		UPDATE notifications SET
			read = $1,
			read_at = $2,
			metadata = $3,
			updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL
	`
	result, err := r.getDB().ExecContext(ctx, query,
		notification.Read, notification.ReadAt, metadataJSON,
		time.Now(), notification.ID,
	)
	if err != nil {
		return fmt.Errorf("update notification failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrNotificationNotFound
	}
	return nil
}

// Delete removes a notification.
func (r *notificationRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM notifications WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete notification failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrNotificationNotFound
	}
	return nil
}

// DeleteByUserAndReference removes notifications by user and reference.
func (r *notificationRepo) DeleteByUserAndReference(ctx context.Context, userID, referenceID string) error {
	query := `DELETE FROM notifications WHERE user_id = $1 AND reference_id = $2`
	_, err := r.getDB().ExecContext(ctx, query, userID, referenceID)
	if err != nil {
		return fmt.Errorf("delete by user and reference failed: %w", err)
	}
	return nil
}

// ======================================================================
// Read Status Operations
// ======================================================================

// MarkAsRead marks a notification as read.
func (r *notificationRepo) MarkAsRead(ctx context.Context, id string) error {
	query := `UPDATE notifications SET read = true, read_at = $1, updated_at = $2 WHERE id = $3`
	now := time.Now()
	result, err := r.getDB().ExecContext(ctx, query, now, now, id)
	if err != nil {
		return fmt.Errorf("mark as read failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrNotificationNotFound
	}
	return nil
}

// MarkAllAsRead marks all notifications for a user as read.
func (r *notificationRepo) MarkAllAsRead(ctx context.Context, userID string) error {
	now := time.Now()
	query := `UPDATE notifications SET read = true, read_at = $1, updated_at = $2 WHERE user_id = $3 AND read = false AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, now, now, userID)
	if err != nil {
		return fmt.Errorf("mark all as read failed: %w", err)
	}
	return nil
}

// MarkMultipleAsRead marks multiple notifications as read.
func (r *notificationRepo) MarkMultipleAsRead(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	query, args, err := sqlx.In(`UPDATE notifications SET read = true, read_at = ?, updated_at = ? WHERE id IN (?)`, now, now, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark multiple as read failed: %w", err)
	}
	return nil
}

// MarkAsUnread marks a notification as unread.
func (r *notificationRepo) MarkAsUnread(ctx context.Context, id string) error {
	query := `UPDATE notifications SET read = false, read_at = NULL, updated_at = $1 WHERE id = $2`
	result, err := r.getDB().ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("mark as unread failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrNotificationNotFound
	}
	return nil
}

// MarkAllAsUnread marks all notifications for a user as unread.
func (r *notificationRepo) MarkAllAsUnread(ctx context.Context, userID string) error {
	query := `UPDATE notifications SET read = false, read_at = NULL, updated_at = $1 WHERE user_id = $2 AND read = true AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("mark all as unread failed: %w", err)
	}
	return nil
}

// ======================================================================
// Existence Checks
// ======================================================================

// Exists checks if a notification exists.
func (r *notificationRepo) Exists(ctx context.Context, id string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM notifications WHERE id = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, id)
	if err != nil {
		return false, fmt.Errorf("exists check failed: %w", err)
	}
	return exists, nil
}

// ExistsByUserAndReference checks if a notification exists for a user and reference.
func (r *notificationRepo) ExistsByUserAndReference(ctx context.Context, userID, referenceID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM notifications WHERE user_id = $1 AND reference_id = $2 AND deleted_at IS NULL)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, userID, referenceID)
	if err != nil {
		return false, fmt.Errorf("exists by user and reference failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountByUserID returns total notifications for a user.
func (r *notificationRepo) CountByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count by user ID failed: %w", err)
	}
	return count, nil
}

// CountUnread returns total unread notifications for a user.
func (r *notificationRepo) CountUnread(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread failed: %w", err)
	}
	return count, nil
}

// CountRead returns total read notifications for a user.
func (r *notificationRepo) CountRead(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = true AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count read failed: %w", err)
	}
	return count, nil
}

// CountByType returns notifications count by type for a user.
func (r *notificationRepo) CountByType(ctx context.Context, userID, notificationType string) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND type = $2 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, notificationType)
	if err != nil {
		return 0, fmt.Errorf("count by type failed: %w", err)
	}
	return count, nil
}

// CountByDateRange returns notifications count within a date range.
func (r *notificationRepo) CountByDateRange(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, start, end)
	if err != nil {
		return 0, fmt.Errorf("count by date range failed: %w", err)
	}
	return count, nil
}

// CountByUserIDs returns notification counts for multiple users.
func (r *notificationRepo) CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if len(userIDs) == 0 {
		return map[string]int64{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT user_id, COUNT(*) as count
		FROM notifications
		WHERE user_id IN (?) AND deleted_at IS NULL
		GROUP BY user_id
	`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var results []struct {
		UserID string `db:"user_id"`
		Count  int64  `db:"count"`
	}
	err = r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count by user IDs failed: %w", err)
	}
	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.UserID] = r.Count
	}
	return counts, nil
}

// CountUnreadByUserIDs returns unread notification counts for multiple users.
func (r *notificationRepo) CountUnreadByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if len(userIDs) == 0 {
		return map[string]int64{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT user_id, COUNT(*) as count
		FROM notifications
		WHERE user_id IN (?) AND read = false AND deleted_at IS NULL
		GROUP BY user_id
	`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var results []struct {
		UserID string `db:"user_id"`
		Count  int64  `db:"count"`
	}
	err = r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count unread by user IDs failed: %w", err)
	}
	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.UserID] = r.Count
	}
	return counts, nil
}

// ======================================================================
// List Operations
// ======================================================================

// GetByUserID returns notifications for a user with pagination.
func (r *notificationRepo) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get by user ID failed: %w", err)
	}
	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetUnreadByUserID returns unread notifications for a user.
func (r *notificationRepo) GetUnreadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND read = false AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get unread by user ID failed: %w", err)
	}
	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetReadByUserID returns read notifications for a user.
func (r *notificationRepo) GetReadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND read = true AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get read by user ID failed: %w", err)
	}
	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetByFromUserID returns notifications from a specific sender.
func (r *notificationRepo) GetByFromUserID(ctx context.Context, fromUserID string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE from_user_id = $1 AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{fromUserID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get by from user ID failed: %w", err)
	}
	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetRecentByUserID returns recent notifications for a user.
func (r *notificationRepo) GetRecentByUserID(ctx context.Context, userID string, limit int) ([]*interfaces.Notification, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2
	`
	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent by user ID failed: %w", err)
	}
	return notifications, nil
}

// GetByUserIDAndType returns notifications by user and type.
func (r *notificationRepo) GetByUserIDAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	return r.GetByUserAndType(ctx, userID, notificationType, cursor, limit)
}

// ======================================================================
// Grouped Notifications
// ======================================================================

// GroupByType groups notifications by type.
func (r *notificationRepo) GroupByType(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.GroupedNotification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT 
			type,
			MAX(created_at) as latest_at,
			COUNT(*) as count,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count
		FROM notifications
		WHERE user_id = $1 AND deleted_at IS NULL
		GROUP BY type
		ORDER BY latest_at DESC
	`
	if cursor != "" {
		query += ` HAVING type > $2`
	}
	query += ` LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var groups []*interfaces.GroupedNotification
	err := r.getDB().SelectContext(ctx, &groups, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("group by type failed: %w", err)
	}
	var nextCursor string
	if len(groups) == limit {
		nextCursor = groups[len(groups)-1].Type
	}
	return groups, nextCursor, nil
}

// GroupByReference groups notifications by reference ID.
func (r *notificationRepo) GroupByReference(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.GroupedNotification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT 
			reference_id,
			MAX(created_at) as latest_at,
			COUNT(*) as count,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count
		FROM notifications
		WHERE user_id = $1 AND reference_id IS NOT NULL AND deleted_at IS NULL
		GROUP BY reference_id
		ORDER BY latest_at DESC
	`
	if cursor != "" {
		query += ` HAVING reference_id > $2`
	}
	query += ` LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var groups []*interfaces.GroupedNotification
	err := r.getDB().SelectContext(ctx, &groups, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("group by reference failed: %w", err)
	}
	var nextCursor string
	if len(groups) == limit {
		nextCursor = groups[len(groups)-1].ReferenceID
	}
	return groups, nextCursor, nil
}

// GroupByTypeAndReference groups notifications by type and reference.
func (r *notificationRepo) GroupByTypeAndReference(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.GroupedNotification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT 
			type,
			reference_id,
			MAX(created_at) as latest_at,
			COUNT(*) as count,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count
		FROM notifications
		WHERE user_id = $1 AND deleted_at IS NULL
		GROUP BY type, reference_id
		ORDER BY latest_at DESC
	`
	if cursor != "" {
		query += ` HAVING type > $2 OR (type = $2 AND reference_id > $3)`
	}
	query += ` LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		parts := strings.SplitN(cursor, "|", 2)
		if len(parts) == 2 {
			args = append(args, parts[0], parts[1])
			argIdx = 4
		}
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var groups []*interfaces.GroupedNotification
	err := r.getDB().SelectContext(ctx, &groups, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("group by type and reference failed: %w", err)
	}
	var nextCursor string
	if len(groups) == limit {
		last := groups[len(groups)-1]
		nextCursor = fmt.Sprintf("%s|%s", last.Type, last.ReferenceID)
	}
	return groups, nextCursor, nil
}

// GetGroupedCount returns count of grouped notifications.
func (r *notificationRepo) GetGroupedCount(ctx context.Context, userID string) (int64, error) {
	query := `
		SELECT COUNT(DISTINCT type || '-' || COALESCE(reference_id, ''))
		FROM notifications
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("get grouped count failed: %w", err)
	}
	return count, nil
}

// ======================================================================
// Advanced Queries
// ======================================================================

// GetNotificationsByDateRange returns notifications within a date range.
func (r *notificationRepo) GetNotificationsByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $4`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, start, end}
	argIdx := 4
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 5
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get notifications by date range failed: %w", err)
	}
	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetUnreadByDateRange returns unread notifications within a date range.
func (r *notificationRepo) GetUnreadByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND read = false AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $4`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, start, end}
	argIdx := 4
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 5
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get unread by date range failed: %w", err)
	}
	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetNotificationsByReferenceIDAndType returns notifications by reference and type.
func (r *notificationRepo) GetNotificationsByReferenceIDAndType(ctx context.Context, referenceID, notificationType string) ([]*interfaces.Notification, error) {
	query := `
		SELECT * FROM notifications
		WHERE reference_id = $1 AND type = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	var notifications []*interfaces.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, referenceID, notificationType)
	if err != nil {
		return nil, fmt.Errorf("get notifications by reference and type failed: %w", err)
	}
	return notifications, nil
}

// GetNotificationsByMultipleReferences returns notifications for multiple references.
func (r *notificationRepo) GetNotificationsByMultipleReferences(ctx context.Context, referenceIDs []string) ([]*interfaces.Notification, error) {
	if len(referenceIDs) == 0 {
		return []*interfaces.Notification{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT * FROM notifications
		WHERE reference_id IN (?) AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, referenceIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var notifications []*interfaces.Notification
	err = r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get notifications by multiple references failed: %w", err)
	}
	return notifications, nil
}

// ======================================================================
// Bulk Operations
// ======================================================================

// BulkCreate inserts multiple notifications in a single transaction.
func (r *notificationRepo) BulkCreate(ctx context.Context, notifications []*interfaces.Notification) error {
	if len(notifications) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO notifications (
			id, user_id, from_user_id, type, reference_id,
			read, read_at, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, n := range notifications {
		metadataJSON, err := json.Marshal(n.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata failed: %w", err)
		}
		_, err = stmt.ExecContext(ctx,
			n.ID, n.UserID, n.FromUserID, n.Type, n.ReferenceID,
			n.Read, n.ReadAt, metadataJSON, n.CreatedAt, n.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("bulk create notification failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple notifications in a single transaction.
func (r *notificationRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM notifications WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete notifications failed: %w", err)
	}
	return nil
}

// BulkDeleteByUserID removes all notifications for a user.
func (r *notificationRepo) BulkDeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM notifications WHERE user_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("bulk delete by user ID failed: %w", err)
	}
	return nil
}

// BulkDeleteByType removes notifications by type for a user.
func (r *notificationRepo) BulkDeleteByType(ctx context.Context, userID, notificationType string) error {
	query := `DELETE FROM notifications WHERE user_id = $1 AND type = $2`
	_, err := r.getDB().ExecContext(ctx, query, userID, notificationType)
	if err != nil {
		return fmt.Errorf("bulk delete by type failed: %w", err)
	}
	return nil
}

// BulkDeleteByReference removes notifications by reference ID.
func (r *notificationRepo) BulkDeleteByReference(ctx context.Context, referenceID string) error {
	query := `DELETE FROM notifications WHERE reference_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, referenceID)
	if err != nil {
		return fmt.Errorf("bulk delete by reference failed: %w", err)
	}
	return nil
}

// BulkDeleteOlderThan removes notifications older than a date.
func (r *notificationRepo) BulkDeleteOlderThan(ctx context.Context, userID string, before time.Time) error {
	query := `DELETE FROM notifications WHERE user_id = $1 AND created_at < $2`
	_, err := r.getDB().ExecContext(ctx, query, userID, before)
	if err != nil {
		return fmt.Errorf("bulk delete older than failed: %w", err)
	}
	return nil
}

// BulkDeleteOlderThanAll removes all notifications older than a date.
func (r *notificationRepo) BulkDeleteOlderThanAll(ctx context.Context, before time.Time) error {
	query := `DELETE FROM notifications WHERE created_at < $1`
	_, err := r.getDB().ExecContext(ctx, query, before)
	if err != nil {
		return fmt.Errorf("bulk delete older than all failed: %w", err)
	}
	return nil
}

// BulkMarkAsRead marks multiple notifications as read.
func (r *notificationRepo) BulkMarkAsRead(ctx context.Context, ids []string) error {
	return r.MarkMultipleAsRead(ctx, ids)
}

// BulkMarkAsUnread marks multiple notifications as unread.
func (r *notificationRepo) BulkMarkAsUnread(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	query, args, err := sqlx.In(`UPDATE notifications SET read = false, read_at = NULL, updated_at = ? WHERE id IN (?)`, now, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk mark as unread failed: %w", err)
	}
	return nil
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetNotificationStats returns aggregated notification statistics.
func (r *notificationRepo) GetNotificationStats(ctx context.Context) (*interfaces.NotificationStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT from_user_id) as unique_senders,
			MAX(created_at) as last_notification,
			MIN(created_at) as first_notification
		FROM notifications
		WHERE deleted_at IS NULL
	`
	var stats interfaces.NotificationStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get notification stats failed: %w", err)
	}
	// Get type stats
	typeStats, err := r.getNotificationTypeStats(ctx, "")
	if err != nil {
		typeStats = []*interfaces.NotificationTypeStat{}
	}
	stats.TypeStats = make(map[string]int64)
	for _, ts := range typeStats {
		stats.TypeStats[ts.Type] = ts.Count
	}
	return &stats, nil
}

// GetUserNotificationStats returns notification stats for a specific user.
func (r *notificationRepo) GetUserNotificationStats(ctx context.Context, userID string) (*interfaces.NotificationStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread,
			COUNT(DISTINCT from_user_id) as unique_senders,
			MAX(created_at) as last_notification,
			MIN(created_at) as first_notification
		FROM notifications
		WHERE user_id = $1 AND deleted_at IS NULL
	`
	var stats interfaces.NotificationStats
	err := r.getDB().GetContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user notification stats failed: %w", err)
	}
	// Get type stats for user
	typeStats, err := r.getNotificationTypeStats(ctx, userID)
	if err != nil {
		typeStats = []*interfaces.NotificationTypeStat{}
	}
	stats.TypeStats = make(map[string]int64)
	for _, ts := range typeStats {
		stats.TypeStats[ts.Type] = ts.Count
	}
	return &stats, nil
}

// GetDailyNotificationStats returns daily notification counts for a date range.
func (r *notificationRepo) GetDailyNotificationStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailyNotificationCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread,
			COUNT(DISTINCT user_id) as unique_users
		FROM notifications
		WHERE created_at >= $1 AND created_at <= $2 AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyNotificationCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily notification stats failed: %w", err)
	}
	return results, nil
}

// GetDailyNotificationStatsForUser returns daily notification counts for a user.
func (r *notificationRepo) GetDailyNotificationStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*interfaces.DailyNotificationCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread,
			1 as unique_users
		FROM notifications
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyNotificationCount
	err := r.getDB().SelectContext(ctx, &results, query, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily notification stats for user failed: %w", err)
	}
	return results, nil
}

// GetNotificationTypeStats returns notification stats by type.
func (r *notificationRepo) GetNotificationTypeStats(ctx context.Context, userID string) ([]*interfaces.NotificationTypeStat, error) {
	query := `
		SELECT 
			type,
			COUNT(*) as count,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count,
			MAX(created_at) as latest_at
		FROM notifications
		WHERE deleted_at IS NULL
	`
	args := []interface{}{}
	if userID != "" {
		query += ` AND user_id = $1`
		args = append(args, userID)
	}
	query += ` GROUP BY type ORDER BY count DESC`

	var stats []*interfaces.NotificationTypeStat
	err := r.getDB().SelectContext(ctx, &stats, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get notification type stats failed: %w", err)
	}
	return stats, nil
}

// GetNotificationTrends returns notification trends over time.
func (r *notificationRepo) GetNotificationTrends(ctx context.Context, userID string, days int) ([]*interfaces.TrendData, error) {
	if days < 1 {
		days = 30
	}
	startDate := time.Now().AddDate(0, 0, -days)
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as value
		FROM notifications
		WHERE user_id = $1 AND created_at >= $2 AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var trends []*interfaces.TrendData
	err := r.getDB().SelectContext(ctx, &trends, query, userID, startDate)
	if err != nil {
		return nil, fmt.Errorf("get notification trends failed: %w", err)
	}
	return trends, nil
}

// GetAverageResponseTime calculates average time between notification and read.
func (r *notificationRepo) GetAverageResponseTime(ctx context.Context, userID string) (float64, error) {
	query := `
		SELECT AVG(EXTRACT(EPOCH FROM (read_at - created_at)))
		FROM notifications
		WHERE user_id = $1 AND read = true AND read_at IS NOT NULL AND deleted_at IS NULL
	`
	var avg float64
	err := r.getDB().GetContext(ctx, &avg, query, userID)
	if err != nil {
		return 0, fmt.Errorf("get average response time failed: %w", err)
	}
	return avg, nil
}

// ======================================================================
// Health
// ======================================================================

func (r *notificationRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *notificationRepo) Close() error {
	return nil
}

func (r *notificationRepo) GetRawDB() interface{} {
	return r.db
}