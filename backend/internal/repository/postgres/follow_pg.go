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
		INSERT INTO follows (follower_id, followee_id, created_at)
		VALUES ($1, $2, $3)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		follow.FollowerID, follow.FolloweeID, follow.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create follow failed: %w", err)
	}
	return nil
}

// Delete removes a follow relationship.
func (r *followRepo) Delete(ctx context.Context, followerID, followeeID string) error {
	query := `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("delete follow failed: %w", err)
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

// CountMutual returns the number of mutual follows between two users.
func (r *followRepo) CountMutual(ctx context.Context, userID1, userID2 string) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM follows f1
		JOIN follows f2 ON f1.followee_id = f2.follower_id
		WHERE f1.follower_id = $1 AND f2.followee_id = $2
	`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID1, userID2)
	if err != nil {
		return 0, fmt.Errorf("count mutual follows failed: %w", err)
	}
	return count, nil
}

// ======================================================================
// List Operations
// ======================================================================

// GetFollowers returns the list of users following a user with pagination.
func (r *followRepo) GetFollowers(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE followee_id = $1
	`
	if cursor != "" {
		query += ` AND follower_id > $2`
	}
	query += ` ORDER BY created_at DESC, follower_id DESC LIMIT $?`

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
		nextCursor = follows[len(follows)-1].FollowerID
	}
	return follows, nextCursor, nil
}

// GetFollowing returns the list of users a user is following with pagination.
func (r *followRepo) GetFollowing(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE follower_id = $1
	`
	if cursor != "" {
		query += ` AND followee_id > $2`
	}
	query += ` ORDER BY created_at DESC, followee_id DESC LIMIT $?`

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
		nextCursor = follows[len(follows)-1].FolloweeID
	}
	return follows, nextCursor, nil
}

// GetFollowerIDs returns all follower IDs for a user (no pagination, for internal use).
func (r *followRepo) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT follower_id FROM follows WHERE followee_id = $1 ORDER BY created_at DESC`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get follower IDs failed: %w", err)
	}
	return ids, nil
}

// GetFollowingIDs returns all following IDs for a user (no pagination).
func (r *followRepo) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT followee_id FROM follows WHERE follower_id = $1 ORDER BY created_at DESC`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get following IDs failed: %w", err)
	}
	return ids, nil
}

// ======================================================================
= Mutual Follows
// ======================================================================

// AreMutual checks if two users follow each other.
func (r *followRepo) AreMutual(ctx context.Context, userID1, userID2 string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2
		) AND EXISTS (
			SELECT 1 FROM follows WHERE follower_id = $2 AND followee_id = $1
		)
	`
	var mutual bool
	err := r.getDB().GetContext(ctx, &mutual, query, userID1, userID2)
	if err != nil {
		return false, fmt.Errorf("check mutual follow failed: %w", err)
	}
	return mutual, nil
}

// GetMutualFollows returns the list of mutual follows between two users.
func (r *followRepo) GetMutualFollows(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT f1.followee_id as user_id
		FROM follows f1
		JOIN follows f2 ON f1.followee_id = f2.follower_id
		WHERE f1.follower_id = $1 AND f2.followee_id = $2
	`
	if cursor != "" {
		query += ` AND f1.followee_id > $3`
	}
	query += ` ORDER BY f1.created_at DESC, f1.followee_id DESC LIMIT $?`

	args := []interface{}{userID1, userID2}
	argIndex := 3
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get mutual follows failed: %w", err)
	}

	var nextCursor string
	if len(ids) == limit {
		nextCursor = ids[len(ids)-1]
	}
	return ids, nextCursor, nil
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
		INSERT INTO follows (follower_id, followee_id, created_at)
		VALUES ($1, $2, $3)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range follows {
		_, err := stmt.ExecContext(ctx, f.FollowerID, f.FolloweeID, f.CreatedAt)
		if err != nil {
			return fmt.Errorf("bulk create follow failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple follows in a single transaction.
func (r *followRepo) BulkDelete(ctx context.Context, followerIDs []string, followeeIDs []string) error {
	if len(followerIDs) == 0 || len(followeeIDs) == 0 {
		return nil
	}
	// Use a composite IN query: (follower_id, followee_id) IN ((?,?),(?,?)...)
	// For simplicity, we'll delete one by one in a transaction.
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range followerIDs {
		_, err := stmt.ExecContext(ctx, followerIDs[i], followeeIDs[i])
		if err != nil {
			return fmt.Errorf("bulk delete follow failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDeleteByUserID removes all follows where the user is either follower or followee.
func (r *followRepo) BulkDeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM follows WHERE follower_id = $1 OR followee_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("bulk delete by user failed: %w", err)
	}
	return nil
}

// ======================================================================
= Advanced Queries
// ======================================================================

// GetFollowRecommendations returns suggested users to follow based on mutual follows.
func (r *followRepo) GetFollowRecommendations(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit < 1 {
		limit = 10
	}
	// Find users followed by people that the user follows, excluding already followed and self.
	query := `
		SELECT DISTINCT f2.followee_id
		FROM follows f1
		JOIN follows f2 ON f1.followee_id = f2.follower_id
		WHERE f1.follower_id = $1
		  AND f2.followee_id != $1
		  AND f2.followee_id NOT IN (SELECT followee_id FROM follows WHERE follower_id = $1)
		ORDER BY RANDOM()
		LIMIT $2
	`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get follow recommendations failed: %w", err)
	}
	return ids, nil
}

// GetFollowCountsForUsers returns follower and following counts for multiple users (bulk).
func (r *followRepo) GetFollowCountsForUsers(ctx context.Context, userIDs []string) (map[string]FollowCounts, error) {
	if len(userIDs) == 0 {
		return map[string]FollowCounts{}, nil
	}
	query := `
		SELECT 
			user_id,
			(SELECT COUNT(*) FROM follows WHERE followee_id = user_id) as followers,
			(SELECT COUNT(*) FROM follows WHERE follower_id = user_id) as following
		FROM (
			SELECT unnest($1::text[]) as user_id
		) t
	`
	// Using unnest with array parameter; sqlx supports this.
	var results []struct {
		UserID    string `db:"user_id"`
		Followers int64  `db:"followers"`
		Following int64  `db:"following"`
	}
	err := r.getDB().SelectContext(ctx, &results, query, pq.Array(userIDs))
	if err != nil {
		return nil, fmt.Errorf("get follow counts for users failed: %w", err)
	}
	counts := make(map[string]FollowCounts, len(results))
	for _, r := range results {
		counts[r.UserID] = FollowCounts{Followers: r.Followers, Following: r.Following}
	}
	return counts, nil
}

// FollowCounts holds follower and following counts.
type FollowCounts struct {
	Followers int64 `db:"followers"`
	Following int64 `db:"following"`
}

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetFollowStats returns aggregated follow statistics.
func (r *followRepo) GetFollowStats(ctx context.Context) (*FollowStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_follows,
			COUNT(DISTINCT follower_id) as unique_followers,
			COUNT(DISTINCT followee_id) as unique_followees,
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
	UniqueFollowers int64    `db:"unique_followers"`
	UniqueFollowees int64    `db:"unique_followees"`
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