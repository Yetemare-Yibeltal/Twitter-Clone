// backend/internal/repository/postgres/follow_pg.go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// followRepo is the PostgreSQL implementation of FollowRepository.
type followRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewFollowRepository creates a new PostgreSQL follow repository.
func NewFollowRepository(db *sqlx.DB) interfaces.FollowRepository {
	return &followRepo{
		db:  db,
		log: logger.WithField("repository", "follow_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *followRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.FollowRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &followRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *followRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.FollowRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &followRepo{
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
func (r *followRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic Follow Operations
// ======================================================================

// Create inserts a new follow relationship.
func (r *followRepo) Create(ctx context.Context, follow *entities.Follow) error {
	query := `
		INSERT INTO follows (id, follower_id, followee_id, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		follow.ID, follow.FollowerID, follow.FolloweeID, follow.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create follow failed: %w", err)
	}
	return nil
}

// GetByID retrieves a follow by its ID.
func (r *followRepo) GetByID(ctx context.Context, id string) (*entities.Follow, error) {
	query := `SELECT * FROM follows WHERE id = $1`
	var follow entities.Follow
	err := r.getDB().GetContext(ctx, &follow, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrFollowNotFound
		}
		return nil, fmt.Errorf("get follow by ID failed: %w", err)
	}
	return &follow, nil
}

// GetByFollowerAndFollowee retrieves a follow by follower and followee IDs.
func (r *followRepo) GetByFollowerAndFollowee(ctx context.Context, followerID, followeeID string) (*entities.Follow, error) {
	query := `SELECT * FROM follows WHERE follower_id = $1 AND followee_id = $2`
	var follow entities.Follow
	err := r.getDB().GetContext(ctx, &follow, query, followerID, followeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrFollowNotFound
		}
		return nil, fmt.Errorf("get follow by follower and followee failed: %w", err)
	}
	return &follow, nil
}

// Delete removes a follow relationship.
func (r *followRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM follows WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete follow failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrFollowNotFound
	}
	return nil
}

// DeleteByFollowerAndFollowee removes a follow by follower and followee IDs.
func (r *followRepo) DeleteByFollowerAndFollowee(ctx context.Context, followerID, followeeID string) error {
	query := `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("delete follow by follower and followee failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrFollowNotFound
	}
	return nil
}

// ======================================================================
// Existence Checks
// ======================================================================

// Exists checks if a follow relationship exists.
func (r *followRepo) Exists(ctx context.Context, followerID, followeeID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, followerID, followeeID)
	if err != nil {
		return false, fmt.Errorf("check follow existence failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountFollowers returns the number of followers for a user.
func (r *followRepo) CountFollowers(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM follows WHERE followee_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count followers failed: %w", err)
	}
	return count, nil
}

// CountFollowing returns the number of users a user is following.
func (r *followRepo) CountFollowing(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM follows WHERE follower_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count following failed: %w", err)
	}
	return count, nil
}

// CountFollowersByUserIDs returns follower counts for multiple users (bulk).
func (r *followRepo) CountFollowersByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if len(userIDs) == 0 {
		return map[string]int64{}, nil
	}
	query := `
		SELECT followee_id, COUNT(*) as count
		FROM follows
		WHERE followee_id IN (?)
		GROUP BY followee_id
	`
	query, args, err := sqlx.In(query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)

	var results []struct {
		FolloweeID string `db:"followee_id"`
		Count      int64  `db:"count"`
	}
	err = r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count followers by user IDs failed: %w", err)
	}

	counts := make(map[string]int64, len(results))
	for _, r := range results {
		counts[r.FolloweeID] = r.Count
	}
	return counts, nil
}

// CountFollowingByUserIDs returns following counts for multiple users (bulk).
func (r *followRepo) CountFollowingByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if len(userIDs) == 0 {
		return map[string]int64{}, nil
	}
	query := `
		SELECT follower_id, COUNT(*) as count
		FROM follows
		WHERE follower_id IN (?)
		GROUP BY follower_id
	`
	query, args, err := sqlx.In(query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)

	var results []struct {
		FollowerID string `db:"follower_id"`
		Count      int64  `db:"count"`
	}
	err = r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count following by user IDs failed: %w", err)
	}

	counts := make(map[string]int64, len(results))
	for _, r := range results {
		counts[r.FollowerID] = r.Count
	}
	return counts, nil
}

// ======================================================================
// List Operations
// ======================================================================

// GetFollowers returns all followers of a user with pagination.
func (r *followRepo) GetFollowers(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE followee_id = $1
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

	var follows []*entities.Follow
	err := r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get followers failed: %w", err)
	}

	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// GetFollowing returns all users a user is following with pagination.
func (r *followRepo) GetFollowing(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE follower_id = $1
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

	var follows []*entities.Follow
	err := r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get following failed: %w", err)
	}

	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// GetFollowerIDs returns all follower IDs of a user.
func (r *followRepo) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT follower_id FROM follows WHERE followee_id = $1 ORDER BY created_at DESC`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get follower IDs failed: %w", err)
	}
	return ids, nil
}

// GetFollowingIDs returns all user IDs a user is following.
func (r *followRepo) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT followee_id FROM follows WHERE follower_id = $1 ORDER BY created_at DESC`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get following IDs failed: %w", err)
	}
	return ids, nil
}

// GetFollowerIDsBatch returns follower IDs for multiple users (bulk).
func (r *followRepo) GetFollowerIDsBatch(ctx context.Context, userIDs []string) (map[string][]string, error) {
	if len(userIDs) == 0 {
		return map[string][]string{}, nil
	}
	query := `
		SELECT followee_id, follower_id
		FROM follows
		WHERE followee_id IN (?)
		ORDER BY created_at DESC
	`
	query, args, err := sqlx.In(query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)

	var results []struct {
		FolloweeID string `db:"followee_id"`
		FollowerID string `db:"follower_id"`
	}
	err = r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get follower IDs batch failed: %w", err)
	}

	followerMap := make(map[string][]string)
	for _, r := range results {
		followerMap[r.FolloweeID] = append(followerMap[r.FolloweeID], r.FollowerID)
	}
	return followerMap, nil
}

// GetFollowingIDsBatch returns following IDs for multiple users (bulk).
func (r *followRepo) GetFollowingIDsBatch(ctx context.Context, userIDs []string) (map[string][]string, error) {
	if len(userIDs) == 0 {
		return map[string][]string{}, nil
	}
	query := `
		SELECT follower_id, followee_id
		FROM follows
		WHERE follower_id IN (?)
		ORDER BY created_at DESC
	`
	query, args, err := sqlx.In(query, userIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)

	var results []struct {
		FollowerID string `db:"follower_id"`
		FolloweeID string `db:"followee_id"`
	}
	err = r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get following IDs batch failed: %w", err)
	}

	followingMap := make(map[string][]string)
	for _, r := range results {
		followingMap[r.FollowerID] = append(followingMap[r.FollowerID], r.FolloweeID)
	}
	return followingMap, nil
}

// ======================================================================
= Mutual Follows
// ======================================================================

// GetMutualFollows returns users that two users both follow or both are followed by.
func (r *followRepo) GetMutualFollows(ctx context.Context, userID1, userID2 string) ([]string, error) {
	query := `
		SELECT followee_id
		FROM follows f1
		WHERE f1.follower_id = $1
		AND EXISTS (
			SELECT 1 FROM follows f2
			WHERE f2.follower_id = $2
			AND f2.followee_id = f1.followee_id
		)
	`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID1, userID2)
	if err != nil {
		return nil, fmt.Errorf("get mutual follows failed: %w", err)
	}
	return ids, nil
}

// IsMutualFollow checks if two users follow each other.
func (r *followRepo) IsMutualFollow(ctx context.Context, userID1, userID2 string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM follows
			WHERE follower_id = $1 AND followee_id = $2
		) AND EXISTS (
			SELECT 1 FROM follows
			WHERE follower_id = $2 AND followee_id = $1
		)
	`
	var isMutual bool
	err := r.getDB().GetContext(ctx, &isMutual, query, userID1, userID2)
	if err != nil {
		return false, fmt.Errorf("check mutual follow failed: %w", err)
	}
	return isMutual, nil
}

// ======================================================================
= Bulk Operations
// ======================================================================

// BulkCreate inserts multiple follows in a single transaction.
func (r *followRepo) BulkCreate(ctx context.Context, follows []*entities.Follow) error {
	if len(follows) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO follows (id, follower_id, followee_id, created_at)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range follows {
		_, err := stmt.ExecContext(ctx, f.ID, f.FollowerID, f.FolloweeID, f.CreatedAt)
		if err != nil {
			return fmt.Errorf("bulk create follow failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple follows in a single transaction.
func (r *followRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM follows WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete follows failed: %w", err)
	}
	return nil
}

// BulkDeleteByFollower removes all follows made by a user.
func (r *followRepo) BulkDeleteByFollower(ctx context.Context, followerID string) error {
	query := `DELETE FROM follows WHERE follower_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, followerID)
	if err != nil {
		return fmt.Errorf("bulk delete by follower failed: %w", err)
	}
	return nil
}

// BulkDeleteByFollowee removes all follows targeted at a user.
func (r *followRepo) BulkDeleteByFollowee(ctx context.Context, followeeID string) error {
	query := `DELETE FROM follows WHERE followee_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, followeeID)
	if err != nil {
		return fmt.Errorf("bulk delete by followee failed: %w", err)
	}
	return nil
}

// ======================================================================
= Advanced Queries
// ======================================================================

// GetTopFollowedUsers returns the most followed users.
func (r *followRepo) GetTopFollowedUsers(ctx context.Context, limit int) ([]*entities.User, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT u.*
		FROM users u
		JOIN follows f ON u.id = f.followee_id
		WHERE u.deleted_at IS NULL
		GROUP BY u.id
		ORDER BY COUNT(f.id) DESC
		LIMIT $1
	`
	var users []*entities.User
	err := r.getDB().SelectContext(ctx, &users, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get top followed users failed: %w", err)
	}
	return users, nil
}

// GetTopFollowingUsers returns users who follow the most people.
func (r *followRepo) GetTopFollowingUsers(ctx context.Context, limit int) ([]*entities.User, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT u.*
		FROM users u
		JOIN follows f ON u.id = f.follower_id
		WHERE u.deleted_at IS NULL
		GROUP BY u.id
		ORDER BY COUNT(f.id) DESC
		LIMIT $1
	`
	var users []*entities.User
	err := r.getDB().SelectContext(ctx, &users, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get top following users failed: %w", err)
	}
	return users, nil
}

// GetFollowTimeline returns follows in reverse chronological order for a user's feed.
func (r *followRepo) GetFollowTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Follow, string, error) {
	if len(userIDs) == 0 {
		return []*entities.Follow{}, "", nil
	}
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT f.*
		FROM follows f
		WHERE f.follower_id IN (?)
	`
	if cursor != "" {
		query += ` AND f.id > $2`
	}
	query += ` ORDER BY f.created_at DESC, f.id DESC LIMIT $?`

	args := []interface{}{userIDs}
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)

	query, args, err := sqlx.In(query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)

	var follows []*entities.Follow
	err = r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get follow timeline failed: %w", err)
	}

	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetFollowStats returns aggregated follow statistics.
func (r *followRepo) GetFollowStats(ctx context.Context) (*FollowStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_follows,
			COUNT(DISTINCT follower_id) as total_followers,
			COUNT(DISTINCT followee_id) as total_followees,
			MAX(created_at) as last_follow,
			MIN(created_at) as first_follow
		FROM follows
	`
	var stats FollowStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get follow stats failed: %w", err)
	}
	return &stats, nil
}

// FollowStats represents aggregated follow statistics.
type FollowStats struct {
	TotalFollows   int64     `db:"total_follows"`
	TotalFollowers int64     `db:"total_followers"`
	TotalFollowees int64     `db:"total_followees"`
	LastFollow     time.Time `db:"last_follow"`
	FirstFollow    time.Time `db:"first_follow"`
}

// GetDailyFollows returns daily follow counts for a date range.
func (r *followRepo) GetDailyFollows(ctx context.Context, start, end time.Time) ([]*DailyFollowCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count,
			COUNT(DISTINCT follower_id) as unique_followers
		FROM follows
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyFollowCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily follows failed: %w", err)
	}
	return results, nil
}

// DailyFollowCount represents daily follow counts.
type DailyFollowCount struct {
	Date            time.Time `db:"date"`
	Count           int64     `db:"count"`
	UniqueFollowers int64     `db:"unique_followers"`
}

// ======================================================================
= Health
// ======================================================================

func (r *followRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *followRepo) Close() error {
	return nil
}

func (r *followRepo) GetRawDB() interface{} {
	return r.db
}