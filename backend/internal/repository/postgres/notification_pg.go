// backend/internal/repository/postgres/notification_pg.go
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
// Basic Notification Operations
// ======================================================================

// Create inserts a new notification.
func (r *notificationRepo) Create(ctx context.Context, notification *entities.Notification) error {
	query := `
		INSERT INTO notifications (
			id, user_id, from_user_id, type, reference_id, 
			read, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	metadataJSON, err := json.Marshal(notification.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	_, err = r.getDB().ExecContext(ctx, query,
		notification.ID, notification.UserID, notification.FromUserID,
		notification.Type, notification.ReferenceID,
		notification.Read, metadataJSON, notification.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create notification failed: %w", err)
	}
	return nil
}

// GetByID retrieves a notification by its ID.
func (r *notificationRepo) GetByID(ctx context.Context, id string) (*entities.Notification, error) {
	query := `SELECT * FROM notifications WHERE id = $1`
	var notification entities.Notification
	err := r.getDB().GetContext(ctx, &notification, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrNotificationNotFound
		}
		return nil, fmt.Errorf("get notification by ID failed: %w", err)
	}
	return &notification, nil
}

// Update updates a notification.
func (r *notificationRepo) Update(ctx context.Context, notification *entities.Notification) error {
	query := `
		UPDATE notifications SET
			read = $1,
			metadata = $2
		WHERE id = $3
	`
	metadataJSON, err := json.Marshal(notification.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	result, err := r.getDB().ExecContext(ctx, query,
		notification.Read, metadataJSON, notification.ID,
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

// ======================================================================
// Read Status Operations
// ======================================================================

// MarkAsRead marks a notification as read.
func (r *notificationRepo) MarkAsRead(ctx context.Context, id string) error {
	query := `UPDATE notifications SET read = true WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark notification as read failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrNotificationNotFound
	}
	return nil
}

// MarkAllAsRead marks all notifications for a user as read.
func (r *notificationRepo) MarkAllAsRead(ctx context.Context, userID string) error {
	query := `UPDATE notifications SET read = true WHERE user_id = $1 AND read = false`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("mark all notifications as read failed: %w", err)
	}
	return nil
}

// MarkMultipleAsRead marks multiple notifications as read.
func (r *notificationRepo) MarkMultipleAsRead(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE notifications SET read = true WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark multiple notifications as read failed: %w", err)
	}
	return nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountUnread returns the number of unread notifications for a user.
func (r *notificationRepo) CountUnread(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications failed: %w", err)
	}
	return count, nil
}

// CountByUserID returns the total number of notifications for a user.
func (r *notificationRepo) CountByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count notifications by user failed: %w", err)
	}
	return count, nil
}

// CountByType returns the number of notifications of a specific type for a user.
func (r *notificationRepo) CountByType(ctx context.Context, userID, notificationType string) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND type = $2`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, notificationType)
	if err != nil {
		return 0, fmt.Errorf("count notifications by type failed: %w", err)
	}
	return count, nil
}

// CountByDateRange returns the number of notifications in a date range.
func (r *notificationRepo) CountByDateRange(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, start, end)
	if err != nil {
		return 0, fmt.Errorf("count notifications by date range failed: %w", err)
	}
	return count, nil
}

// ======================================================================
// List Operations
// ======================================================================

// GetByUserID returns all notifications for a user with pagination.
func (r *notificationRepo) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID}
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*entities.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get notifications by user failed: %w", err)
	}

	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetUnreadByUserID returns unread notifications for a user with pagination.
func (r *notificationRepo) GetUnreadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND read = false
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID}
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*entities.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get unread notifications failed: %w", err)
	}

	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetByType returns notifications of a specific type for a user.
func (r *notificationRepo) GetByType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND type = $2
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, notificationType}
	argIndex := 3
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*entities.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get notifications by type failed: %w", err)
	}

	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// GetByFromUserID returns notifications from a specific user.
func (r *notificationRepo) GetByFromUserID(ctx context.Context, fromUserID string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE from_user_id = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{fromUserID}
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*entities.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get notifications by from user failed: %w", err)
	}

	var nextCursor string
	if len(notifications) == limit {
		nextCursor = notifications[len(notifications)-1].ID
	}
	return notifications, nextCursor, nil
}

// ======================================================================
= Bulk Operations
// ======================================================================

// BulkCreate inserts multiple notifications in a single transaction.
func (r *notificationRepo) BulkCreate(ctx context.Context, notifications []*entities.Notification) error {
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
			read, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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
		_, err := stmt.ExecContext(ctx,
			n.ID, n.UserID, n.FromUserID, n.Type, n.ReferenceID,
			n.Read, metadataJSON, n.CreatedAt,
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
		return fmt.Errorf("bulk delete notifications by user failed: %w", err)
	}
	return nil
}

// BulkDeleteByType removes all notifications of a type for a user.
func (r *notificationRepo) BulkDeleteByType(ctx context.Context, userID, notificationType string) error {
	query := `DELETE FROM notifications WHERE user_id = $1 AND type = $2`
	_, err := r.getDB().ExecContext(ctx, query, userID, notificationType)
	if err != nil {
		return fmt.Errorf("bulk delete notifications by type failed: %w", err)
	}
	return nil
}

// BulkDeleteOlderThan removes notifications older than a date.
func (r *notificationRepo) BulkDeleteOlderThan(ctx context.Context, userID string, before time.Time) error {
	query := `DELETE FROM notifications WHERE user_id = $1 AND created_at < $2`
	_, err := r.getDB().ExecContext(ctx, query, userID, before)
	if err != nil {
		return fmt.Errorf("bulk delete older notifications failed: %w", err)
	}
	return nil
}

// ======================================================================
= Advanced Queries
// ======================================================================

// GetRecentNotifications returns recent notifications for a user.
func (r *notificationRepo) GetRecentNotifications(ctx context.Context, userID string, limit int) ([]*entities.Notification, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	var notifications []*entities.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent notifications failed: %w", err)
	}
	return notifications, nil
}

// GetNotificationsByReferenceID returns notifications by reference ID.
func (r *notificationRepo) GetNotificationsByReferenceID(ctx context.Context, referenceID string) ([]*entities.Notification, error) {
	query := `SELECT * FROM notifications WHERE reference_id = $1 ORDER BY created_at DESC`
	var notifications []*entities.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, referenceID)
	if err != nil {
		return nil, fmt.Errorf("get notifications by reference ID failed: %w", err)
	}
	return notifications, nil
}

// GetNotificationsByUserAndType returns notifications by user and type with pagination.
func (r *notificationRepo) GetNotificationsByUserAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM notifications
		WHERE user_id = $1 AND type = $2
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, notificationType}
	argIndex := 3
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var notifications []*entities.Notification
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

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetNotificationStats returns aggregated notification statistics.
func (r *notificationRepo) GetNotificationStats(ctx context.Context) (*NotificationStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT from_user_id) as unique_senders,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count,
			MAX(created_at) as latest,
			MIN(created_at) as earliest
		FROM notifications
	`
	var stats NotificationStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get notification stats failed: %w", err)
	}
	return &stats, nil
}

// NotificationStats represents aggregated notification statistics.
type NotificationStats struct {
	Total        int64     `db:"total"`
	UniqueUsers  int64     `db:"unique_users"`
	UniqueSenders int64    `db:"unique_senders"`
	ReadCount    int64     `db:"read_count"`
	UnreadCount  int64     `db:"unread_count"`
	Latest       time.Time `db:"latest"`
	Earliest     time.Time `db:"earliest"`
}

// GetDailyNotifications returns daily notification counts for a date range.
func (r *notificationRepo) GetDailyNotifications(ctx context.Context, start, end time.Time) ([]*DailyNotificationCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			COUNT(DISTINCT user_id) as unique_users,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count
		FROM notifications
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyNotificationCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily notifications failed: %w", err)
	}
	return results, nil
}

// DailyNotificationCount represents daily notification counts.
type DailyNotificationCount struct {
	Date        time.Time `db:"date"`
	Total       int64     `db:"total"`
	UniqueUsers int64     `db:"unique_users"`
	ReadCount   int64     `db:"read_count"`
	UnreadCount int64     `db:"unread_count"`
}

// GetNotificationTypeStats returns notification stats by type.
func (r *notificationRepo) GetNotificationTypeStats(ctx context.Context, userID string) ([]*NotificationTypeStat, error) {
	query := `
		SELECT 
			type,
			COUNT(*) as count,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count,
			MAX(created_at) as latest
		FROM notifications
		WHERE user_id = $1
		GROUP BY type
		ORDER BY count DESC
	`
	var stats []*NotificationTypeStat
	err := r.getDB().SelectContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get notification type stats failed: %w", err)
	}
	return stats, nil
}

// NotificationTypeStat represents notification statistics by type.
type NotificationTypeStat struct {
	Type        string    `db:"type"`
	Count       int64     `db:"count"`
	ReadCount   int64     `db:"read_count"`
	UnreadCount int64     `db:"unread_count"`
	Latest      time.Time `db:"latest"`
}

// ======================================================================
= Notification Grouping
// ======================================================================

// GroupNotifications groups notifications by type and reference.
func (r *notificationRepo) GroupNotifications(ctx context.Context, userID string, limit int) ([]*entities.Notification, error) {
	if limit < 1 {
		limit = 20
	}
	// Get the latest notification per group (type + reference_id)
	query := `
		SELECT DISTINCT ON (type, reference_id) *
		FROM notifications
		WHERE user_id = $1
		ORDER BY type, reference_id, created_at DESC
		LIMIT $2
	`
	var notifications []*entities.Notification
	err := r.getDB().SelectContext(ctx, &notifications, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("group notifications failed: %w", err)
	}
	return notifications, nil
}

// GetGroupedNotificationsWithCount returns grouped notifications with counts.
func (r *notificationRepo) GetGroupedNotificationsWithCount(ctx context.Context, userID string, cursor string, limit int) ([]*GroupedNotification, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT 
			type,
			reference_id,
			COUNT(*) as count,
			MAX(created_at) as latest,
			SUM(CASE WHEN read = true THEN 1 ELSE 0 END) as read_count,
			SUM(CASE WHEN read = false THEN 1 ELSE 0 END) as unread_count,
			(SELECT id FROM notifications n2 
			 WHERE n2.user_id = $1 AND n2.type = n.type AND n2.reference_id = n.reference_id 
			 ORDER BY created_at DESC LIMIT 1) as latest_id
		FROM notifications n
		WHERE user_id = $1
	`
	if cursor != "" {
		query += ` AND (MAX(created_at), type, reference_id) < (SELECT created_at, type, reference_id FROM notifications WHERE id = $2)`
	}
	query += `
		GROUP BY type, reference_id
		ORDER BY latest DESC
		LIMIT $?
	`

	args := []interface{}{userID}
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var results []*GroupedNotification
	err := r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get grouped notifications failed: %w", err)
	}

	var nextCursor string
	if len(results) == limit {
		nextCursor = results[len(results)-1].LatestID
	}
	return results, nextCursor, nil
}

// GroupedNotification represents a grouped notification.
type GroupedNotification struct {
	Type        string    `db:"type"`
	ReferenceID string    `db:"reference_id"`
	Count       int64     `db:"count"`
	Latest      time.Time `db:"latest"`
	ReadCount   int64     `db:"read_count"`
	UnreadCount int64     `db:"unread_count"`
	LatestID    string    `db:"latest_id"`
}

// ======================================================================
= Health
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