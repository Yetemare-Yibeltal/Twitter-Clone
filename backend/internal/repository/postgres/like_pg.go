// backend/internal/repository/postgres/like_pg.go
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

// likeRepo is the PostgreSQL implementation of LikeRepository.
type likeRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewLikeRepository creates a new PostgreSQL like repository.
func NewLikeRepository(db *sqlx.DB) interfaces.LikeRepository {
	return &likeRepo{
		db:  db,
		log: logger.WithField("repository", "like_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *likeRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.LikeRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &likeRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *likeRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.LikeRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &likeRepo{
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
func (r *likeRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic Like Operations
// ======================================================================

// Create inserts a new like.
func (r *likeRepo) Create(ctx context.Context, like *entities.Like) error {
	query := `
		INSERT INTO likes (id, tweet_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		like.ID, like.TweetID, like.UserID, like.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create like failed: %w", err)
	}
	return nil
}

// GetByID retrieves a like by its ID.
func (r *likeRepo) GetByID(ctx context.Context, id string) (*entities.Like, error) {
	query := `SELECT * FROM likes WHERE id = $1`
	var like entities.Like
	err := r.getDB().GetContext(ctx, &like, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrLikeNotFound
		}
		return nil, fmt.Errorf("get like by ID failed: %w", err)
	}
	return &like, nil
}

// GetByTweetAndUser retrieves a like by tweet ID and user ID.
func (r *likeRepo) GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Like, error) {
	query := `SELECT * FROM likes WHERE tweet_id = $1 AND user_id = $2`
	var like entities.Like
	err := r.getDB().GetContext(ctx, &like, query, tweetID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrLikeNotFound
		}
		return nil, fmt.Errorf("get like by tweet and user failed: %w", err)
	}
	return &like, nil
}

// Delete removes a like.
func (r *likeRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM likes WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete like failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrLikeNotFound
	}
	return nil
}

// DeleteByTweetAndUser removes a like by tweet and user.
func (r *likeRepo) DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error {
	query := `DELETE FROM likes WHERE tweet_id = $1 AND user_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, tweetID, userID)
	if err != nil {
		return fmt.Errorf("delete like by tweet and user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrLikeNotFound
	}
	return nil
}

// ======================================================================
// Existence Checks
// ======================================================================

// Exists checks if a user has liked a tweet.
func (r *likeRepo) Exists(ctx context.Context, tweetID, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM likes WHERE tweet_id = $1 AND user_id = $2)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("check like existence failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountByTweetID returns the number of likes for a tweet.
func (r *likeRepo) CountByTweetID(ctx context.Context, tweetID string) (int64, error) {
	query := `SELECT COUNT(*) FROM likes WHERE tweet_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, tweetID)
	if err != nil {
		return 0, fmt.Errorf("count likes by tweet failed: %w", err)
	}
	return count, nil
}

// CountByUserID returns the total number of likes made by a user.
func (r *likeRepo) CountByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM likes WHERE user_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count likes by user failed: %w", err)
	}
	return count, nil
}

// CountByTweetIDs returns like counts for multiple tweets (bulk).
func (r *likeRepo) CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error) {
	if len(tweetIDs) == 0 {
		return map[string]int64{}, nil
	}
	query := `
		SELECT tweet_id, COUNT(*) as count
		FROM likes
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
		return nil, fmt.Errorf("count likes by tweet IDs failed: %w", err)
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

// GetByTweetID returns all likes for a tweet with pagination.
func (r *likeRepo) GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Like, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM likes
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

	var likes []*entities.Like
	err := r.getDB().SelectContext(ctx, &likes, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get likes by tweet failed: %w", err)
	}

	var nextCursor string
	if len(likes) == limit {
		nextCursor = likes[len(likes)-1].ID
	}
	return likes, nextCursor, nil
}

// GetByUserID returns all likes made by a user with pagination.
func (r *likeRepo) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Like, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM likes
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

	var likes []*entities.Like
	err := r.getDB().SelectContext(ctx, &likes, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get likes by user failed: %w", err)
	}

	var nextCursor string
	if len(likes) == limit {
		nextCursor = likes[len(likes)-1].ID
	}
	return likes, nextCursor, nil
}

// GetLikedTweetIDs returns all tweet IDs liked by a user.
func (r *likeRepo) GetLikedTweetIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT tweet_id FROM likes WHERE user_id = $1 ORDER BY created_at DESC`
	var tweetIDs []string
	err := r.getDB().SelectContext(ctx, &tweetIDs, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get liked tweet IDs failed: %w", err)
	}
	return tweetIDs, nil
}

// ======================================================================
= Bulk Operations
// ======================================================================

// BulkCreate inserts multiple likes in a single transaction.
func (r *likeRepo) BulkCreate(ctx context.Context, likes []*entities.Like) error {
	if len(likes) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO likes (id, tweet_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, l := range likes {
		_, err := stmt.ExecContext(ctx, l.ID, l.TweetID, l.UserID, l.CreatedAt)
		if err != nil {
			return fmt.Errorf("bulk create like failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple likes in a single transaction.
func (r *likeRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM likes WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete likes failed: %w", err)
	}
	return nil
}

// BulkDeleteByTweetID removes all likes for a tweet.
func (r *likeRepo) BulkDeleteByTweetID(ctx context.Context, tweetID string) error {
	query := `DELETE FROM likes WHERE tweet_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, tweetID)
	if err != nil {
		return fmt.Errorf("bulk delete likes by tweet failed: %w", err)
	}
	return nil
}

// BulkDeleteByUserID removes all likes made by a user.
func (r *likeRepo) BulkDeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM likes WHERE user_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("bulk delete likes by user failed: %w", err)
	}
	return nil
}

// ======================================================================
= Advanced Queries
// ======================================================================

// GetMostLikedTweets returns the most liked tweets (trending).
func (r *likeRepo) GetMostLikedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT t.*
		FROM tweets t
		JOIN likes l ON t.id = l.tweet_id
		WHERE l.created_at >= $1
		  AND t.deleted_at IS NULL
		GROUP BY t.id
		ORDER BY COUNT(l.id) DESC
		LIMIT $2
	`
	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most liked tweets failed: %w", err)
	}
	return tweets, nil
}

// GetLikesTimeline returns likes in reverse chronological order for a user's feed.
func (r *likeRepo) GetLikesTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Like, string, error) {
	if len(userIDs) == 0 {
		return []*entities.Like{}, "", nil
	}
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT l.*
		FROM likes l
		WHERE l.user_id IN (?)
	`
	if cursor != "" {
		query += ` AND l.id > $2`
	}
	query += ` ORDER BY l.created_at DESC, l.id DESC LIMIT $?`

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

	var likes []*entities.Like
	err = r.getDB().SelectContext(ctx, &likes, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get likes timeline failed: %w", err)
	}

	var nextCursor string
	if len(likes) == limit {
		nextCursor = likes[len(likes)-1].ID
	}
	return likes, nextCursor, nil
}

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetLikeStats returns aggregated like statistics.
func (r *likeRepo) GetLikeStats(ctx context.Context) (*LikeStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_likes,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT tweet_id) as unique_tweets,
			MAX(created_at) as last_like,
			MIN(created_at) as first_like
		FROM likes
	`
	var stats LikeStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get like stats failed: %w", err)
	}
	return &stats, nil
}

// LikeStats represents aggregated like statistics.
type LikeStats struct {
	TotalLikes   int64     `db:"total_likes"`
	UniqueUsers  int64     `db:"unique_users"`
	UniqueTweets int64     `db:"unique_tweets"`
	LastLike     time.Time `db:"last_like"`
	FirstLike    time.Time `db:"first_like"`
}

// GetDailyLikes returns daily like counts for a date range.
func (r *likeRepo) GetDailyLikes(ctx context.Context, start, end time.Time) ([]*DailyLikeCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count,
			COUNT(DISTINCT user_id) as unique_users
		FROM likes
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyLikeCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily likes failed: %w", err)
	}
	return results, nil
}

// DailyLikeCount represents daily like counts.
type DailyLikeCount struct {
	Date        time.Time `db:"date"`
	Count       int64     `db:"count"`
	UniqueUsers int64     `db:"unique_users"`
}

// ======================================================================
= Health
// ======================================================================

func (r *likeRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *likeRepo) Close() error {
	return nil
}

func (r *likeRepo) GetRawDB() interface{} {
	return r.db
}