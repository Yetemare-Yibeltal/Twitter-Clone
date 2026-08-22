// backend/internal/repository/postgres/retweet_pg.go
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

// retweetRepo is the PostgreSQL implementation of RetweetRepository.
type retweetRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewRetweetRepository creates a new PostgreSQL retweet repository.
func NewRetweetRepository(db *sqlx.DB) interfaces.RetweetRepository {
	return &retweetRepo{
		db:  db,
		log: logger.WithField("repository", "retweet_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *retweetRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.RetweetRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &retweetRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *retweetRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.RetweetRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &retweetRepo{
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
func (r *retweetRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic Retweet Operations
// ======================================================================

// Create inserts a new retweet.
func (r *retweetRepo) Create(ctx context.Context, retweet *entities.Retweet) error {
	query := `
		INSERT INTO retweets (id, tweet_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		retweet.ID, retweet.TweetID, retweet.UserID, retweet.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create retweet failed: %w", err)
	}
	return nil
}

// GetByID retrieves a retweet by its ID.
func (r *retweetRepo) GetByID(ctx context.Context, id string) (*entities.Retweet, error) {
	query := `SELECT * FROM retweets WHERE id = $1`
	var retweet entities.Retweet
	err := r.getDB().GetContext(ctx, &retweet, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrRetweetNotFound
		}
		return nil, fmt.Errorf("get retweet by ID failed: %w", err)
	}
	return &retweet, nil
}

// GetByTweetAndUser retrieves a retweet by tweet ID and user ID.
func (r *retweetRepo) GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Retweet, error) {
	query := `SELECT * FROM retweets WHERE tweet_id = $1 AND user_id = $2`
	var retweet entities.Retweet
	err := r.getDB().GetContext(ctx, &retweet, query, tweetID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrRetweetNotFound
		}
		return nil, fmt.Errorf("get retweet by tweet and user failed: %w", err)
	}
	return &retweet, nil
}

// Delete removes a retweet.
func (r *retweetRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM retweets WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete retweet failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrRetweetNotFound
	}
	return nil
}

// DeleteByTweetAndUser removes a retweet by tweet and user.
func (r *retweetRepo) DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error {
	query := `DELETE FROM retweets WHERE tweet_id = $1 AND user_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, tweetID, userID)
	if err != nil {
		return fmt.Errorf("delete retweet by tweet and user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrRetweetNotFound
	}
	return nil
}

// ======================================================================
// Existence Checks
// ======================================================================

// Exists checks if a user has retweeted a tweet.
func (r *retweetRepo) Exists(ctx context.Context, tweetID, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM retweets WHERE tweet_id = $1 AND user_id = $2)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("check retweet existence failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountByTweetID returns the number of retweets for a tweet.
func (r *retweetRepo) CountByTweetID(ctx context.Context, tweetID string) (int64, error) {
	query := `SELECT COUNT(*) FROM retweets WHERE tweet_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, tweetID)
	if err != nil {
		return 0, fmt.Errorf("count retweets by tweet failed: %w", err)
	}
	return count, nil
}

// CountByUserID returns the total number of retweets made by a user.
func (r *retweetRepo) CountByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM retweets WHERE user_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count retweets by user failed: %w", err)
	}
	return count, nil
}

// CountByTweetIDs returns retweet counts for multiple tweets (bulk).
func (r *retweetRepo) CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error) {
	if len(tweetIDs) == 0 {
		return map[string]int64{}, nil
	}
	query := `
		SELECT tweet_id, COUNT(*) as count
		FROM retweets
		WHERE tweet_id IN (?)
		GROUP BY tweet_id
	`
	query, args, err := sqlx.In(query, tweetIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)

	var results []struct {
		TweetID string `db:"tweet_id"`
		Count   int64  `db:"count"`
	}
	err = r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count retweets by tweet IDs failed: %w", err)
	}

	counts := make(map[string]int64, len(results))
	for _, r := range results {
		counts[r.TweetID] = r.Count
	}
	return counts, nil
}

// ======================================================================
// List Operations
// ======================================================================

// GetByTweetID returns all retweets for a tweet with pagination.
func (r *retweetRepo) GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM retweets
		WHERE tweet_id = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{tweetID}
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var retweets []*entities.Retweet
	err := r.getDB().SelectContext(ctx, &retweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get retweets by tweet failed: %w", err)
	}

	var nextCursor string
	if len(retweets) == limit {
		nextCursor = retweets[len(retweets)-1].ID
	}
	return retweets, nextCursor, nil
}

// GetByUserID returns all retweets made by a user with pagination.
func (r *retweetRepo) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM retweets
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

	var retweets []*entities.Retweet
	err := r.getDB().SelectContext(ctx, &retweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get retweets by user failed: %w", err)
	}

	var nextCursor string
	if len(retweets) == limit {
		nextCursor = retweets[len(retweets)-1].ID
	}
	return retweets, nextCursor, nil
}

// GetRetweetedTweetIDs returns all tweet IDs retweeted by a user.
func (r *retweetRepo) GetRetweetedTweetIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT tweet_id FROM retweets WHERE user_id = $1 ORDER BY created_at DESC`
	var tweetIDs []string
	err := r.getDB().SelectContext(ctx, &tweetIDs, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get retweeted tweet IDs failed: %w", err)
	}
	return tweetIDs, nil
}

// ======================================================================
// Bulk Operations
// ======================================================================

// BulkCreate inserts multiple retweets in a single transaction.
func (r *retweetRepo) BulkCreate(ctx context.Context, retweets []*entities.Retweet) error {
	if len(retweets) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO retweets (id, tweet_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, rt := range retweets {
		_, err := stmt.ExecContext(ctx, rt.ID, rt.TweetID, rt.UserID, rt.CreatedAt)
		if err != nil {
			return fmt.Errorf("bulk create retweet failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple retweets in a single transaction.
func (r *retweetRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM retweets WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete retweets failed: %w", err)
	}
	return nil
}

// BulkDeleteByTweetID removes all retweets for a tweet.
func (r *retweetRepo) BulkDeleteByTweetID(ctx context.Context, tweetID string) error {
	query := `DELETE FROM retweets WHERE tweet_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, tweetID)
	if err != nil {
		return fmt.Errorf("bulk delete retweets by tweet failed: %w", err)
	}
	return nil
}

// BulkDeleteByUserID removes all retweets made by a user.
func (r *retweetRepo) BulkDeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM retweets WHERE user_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("bulk delete retweets by user failed: %w", err)
	}
	return nil
}

// ======================================================================
// Advanced Queries
// ======================================================================

// GetMostRetweetedTweets returns the most retweeted tweets (trending).
func (r *retweetRepo) GetMostRetweetedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT t.*
		FROM tweets t
		JOIN retweets rt ON t.id = rt.tweet_id
		WHERE rt.created_at >= $1
		  AND t.deleted_at IS NULL
		GROUP BY t.id
		ORDER BY COUNT(rt.id) DESC
		LIMIT $2
	`
	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most retweeted tweets failed: %w", err)
	}
	return tweets, nil
}

// GetRetweetTimeline returns retweets in reverse chronological order for a user's feed.
func (r *retweetRepo) GetRetweetTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	if len(userIDs) == 0 {
		return []*entities.Retweet{}, "", nil
	}
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT rt.*
		FROM retweets rt
		WHERE rt.user_id IN (?)
	`
	if cursor != "" {
		query += ` AND rt.id > $2`
	}
	query += ` ORDER BY rt.created_at DESC, rt.id DESC LIMIT $?`

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

	var retweets []*entities.Retweet
	err = r.getDB().SelectContext(ctx, &retweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get retweet timeline failed: %w", err)
	}

	var nextCursor string
	if len(retweets) == limit {
		nextCursor = retweets[len(retweets)-1].ID
	}
	return retweets, nextCursor, nil
}

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetRetweetStats returns aggregated retweet statistics.
func (r *retweetRepo) GetRetweetStats(ctx context.Context) (*RetweetStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_retweets,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT tweet_id) as unique_tweets,
			MAX(created_at) as last_retweet,
			MIN(created_at) as first_retweet
		FROM retweets
	`
	var stats RetweetStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get retweet stats failed: %w", err)
	}
	return &stats, nil
}

// RetweetStats represents aggregated retweet statistics.
type RetweetStats struct {
	TotalRetweets  int64     `db:"total_retweets"`
	UniqueUsers    int64     `db:"unique_users"`
	UniqueTweets   int64     `db:"unique_tweets"`
	LastRetweet    time.Time `db:"last_retweet"`
	FirstRetweet   time.Time `db:"first_retweet"`
}

// GetDailyRetweets returns daily retweet counts for a date range.
func (r *retweetRepo) GetDailyRetweets(ctx context.Context, start, end time.Time) ([]*DailyRetweetCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count,
			COUNT(DISTINCT user_id) as unique_users
		FROM retweets
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyRetweetCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily retweets failed: %w", err)
	}
	return results, nil
}

// DailyRetweetCount represents daily retweet counts.
type DailyRetweetCount struct {
	Date        time.Time `db:"date"`
	Count       int64     `db:"count"`
	UniqueUsers int64     `db:"unique_users"`
}

// ======================================================================
= Health
// ======================================================================

func (r *retweetRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *retweetRepo) Close() error {
	return nil
}

func (r *retweetRepo) GetRawDB() interface{} {
	return r.db
}