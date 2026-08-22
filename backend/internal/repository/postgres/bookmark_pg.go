// backend/internal/repository/postgres/bookmark_pg.go
package postgres

import (
	"context"
	"database/sql"
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

// bookmarkRepo is the PostgreSQL implementation of BookmarkRepository.
type bookmarkRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewBookmarkRepository creates a new PostgreSQL bookmark repository.
func NewBookmarkRepository(db *sqlx.DB) interfaces.BookmarkRepository {
	return &bookmarkRepo{
		db:  db,
		log: logger.WithField("repository", "bookmark_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *bookmarkRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.BookmarkRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &bookmarkRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *bookmarkRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.BookmarkRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &bookmarkRepo{
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
func (r *bookmarkRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic Bookmark Operations
// ======================================================================

// Create inserts a new bookmark.
func (r *bookmarkRepo) Create(ctx context.Context, bookmark *entities.Bookmark) error {
	query := `
		INSERT INTO bookmarks (id, tweet_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		bookmark.ID, bookmark.TweetID, bookmark.UserID, bookmark.CreatedAt,
	)
	if err != nil {
		// Check for duplicate key violation (PostgreSQL error code 23505)
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return interfaces.ErrAlreadyBookmarked
		}
		return fmt.Errorf("create bookmark failed: %w", err)
	}
	return nil
}

// GetByID retrieves a bookmark by its ID.
func (r *bookmarkRepo) GetByID(ctx context.Context, id string) (*entities.Bookmark, error) {
	query := `SELECT * FROM bookmarks WHERE id = $1`
	var bookmark entities.Bookmark
	err := r.getDB().GetContext(ctx, &bookmark, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrBookmarkNotFound
		}
		return nil, fmt.Errorf("get bookmark by ID failed: %w", err)
	}
	return &bookmark, nil
}

// GetByTweetAndUser retrieves a bookmark by tweet ID and user ID.
func (r *bookmarkRepo) GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Bookmark, error) {
	query := `SELECT * FROM bookmarks WHERE tweet_id = $1 AND user_id = $2`
	var bookmark entities.Bookmark
	err := r.getDB().GetContext(ctx, &bookmark, query, tweetID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrBookmarkNotFound
		}
		return nil, fmt.Errorf("get bookmark by tweet and user failed: %w", err)
	}
	return &bookmark, nil
}

// Delete removes a bookmark.
func (r *bookmarkRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM bookmarks WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete bookmark failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrBookmarkNotFound
	}
	return nil
}

// DeleteByTweetAndUser removes a bookmark by tweet and user.
func (r *bookmarkRepo) DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error {
	query := `DELETE FROM bookmarks WHERE tweet_id = $1 AND user_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, tweetID, userID)
	if err != nil {
		return fmt.Errorf("delete bookmark by tweet and user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrBookmarkNotFound
	}
	return nil
}

// ======================================================================
// Existence Checks
// ======================================================================

// Exists checks if a user has bookmarked a tweet.
func (r *bookmarkRepo) Exists(ctx context.Context, tweetID, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM bookmarks WHERE tweet_id = $1 AND user_id = $2)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("check bookmark existence failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountByTweetID returns the number of bookmarks for a tweet.
func (r *bookmarkRepo) CountByTweetID(ctx context.Context, tweetID string) (int64, error) {
	query := `SELECT COUNT(*) FROM bookmarks WHERE tweet_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, tweetID)
	if err != nil {
		return 0, fmt.Errorf("count bookmarks by tweet failed: %w", err)
	}
	return count, nil
}

// CountByUserID returns the total number of bookmarks made by a user.
func (r *bookmarkRepo) CountByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM bookmarks WHERE user_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count bookmarks by user failed: %w", err)
	}
	return count, nil
}

// CountByTweetIDs returns bookmark counts for multiple tweets (bulk).
func (r *bookmarkRepo) CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error) {
	if len(tweetIDs) == 0 {
		return map[string]int64{}, nil
	}
	query := `
		SELECT tweet_id, COUNT(*) as count
		FROM bookmarks
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
		return nil, fmt.Errorf("count bookmarks by tweet IDs failed: %w", err)
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

// GetByTweetID returns all bookmarks for a tweet with pagination.
func (r *bookmarkRepo) GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM bookmarks
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

	var bookmarks []*entities.Bookmark
	err := r.getDB().SelectContext(ctx, &bookmarks, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get bookmarks by tweet failed: %w", err)
	}

	var nextCursor string
	if len(bookmarks) == limit {
		nextCursor = bookmarks[len(bookmarks)-1].ID
	}
	return bookmarks, nextCursor, nil
}

// GetByUserID returns all bookmarks made by a user with pagination.
func (r *bookmarkRepo) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM bookmarks
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

	var bookmarks []*entities.Bookmark
	err := r.getDB().SelectContext(ctx, &bookmarks, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get bookmarks by user failed: %w", err)
	}

	var nextCursor string
	if len(bookmarks) == limit {
		nextCursor = bookmarks[len(bookmarks)-1].ID
	}
	return bookmarks, nextCursor, nil
}

// GetBookmarkedTweetIDs returns all tweet IDs bookmarked by a user.
func (r *bookmarkRepo) GetBookmarkedTweetIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT tweet_id FROM bookmarks WHERE user_id = $1 ORDER BY created_at DESC`
	var tweetIDs []string
	err := r.getDB().SelectContext(ctx, &tweetIDs, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get bookmarked tweet IDs failed: %w", err)
	}
	return tweetIDs, nil
}

// ======================================================================
// Bookmarked Tweets with Join
// ======================================================================

// GetBookmarkedTweets returns full tweet objects for bookmarks by a user.
func (r *bookmarkRepo) GetBookmarkedTweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT t.*
		FROM tweets t
		INNER JOIN bookmarks b ON t.id = b.tweet_id
		WHERE b.user_id = $1
		  AND t.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND t.id > $2`
	}
	query += ` ORDER BY b.created_at DESC, t.id DESC LIMIT $?`

	args := []interface{}{userID}
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get bookmarked tweets failed: %w", err)
	}

	var nextCursor string
	if len(tweets) == limit {
		nextCursor = tweets[len(tweets)-1].ID
	}
	return tweets, nextCursor, nil
}

// GetBookmarkedTweetsWithMetadata returns bookmarks with tweet data and bookmark metadata.
func (r *bookmarkRepo) GetBookmarkedTweetsWithMetadata(ctx context.Context, userID string, cursor string, limit int) ([]*BookmarkedTweet, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT 
			t.*,
			b.id as bookmark_id,
			b.created_at as bookmarked_at
		FROM tweets t
		INNER JOIN bookmarks b ON t.id = b.tweet_id
		WHERE b.user_id = $1
		  AND t.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND b.id > $2`
	}
	query += ` ORDER BY b.created_at DESC, b.id DESC LIMIT $?`

	args := []interface{}{userID}
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var results []*BookmarkedTweet
	err := r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get bookmarked tweets with metadata failed: %w", err)
	}

	var nextCursor string
	if len(results) == limit {
		nextCursor = results[len(results)-1].BookmarkID
	}
	return results, nextCursor, nil
}

// BookmarkedTweet represents a tweet with bookmark metadata.
type BookmarkedTweet struct {
	entities.Tweet
	BookmarkID  string    `db:"bookmark_id"`
	BookmarkedAt time.Time `db:"bookmarked_at"`
}

// ======================================================================
= Bulk Operations
// ======================================================================

// BulkCreate inserts multiple bookmarks in a single transaction.
func (r *bookmarkRepo) BulkCreate(ctx context.Context, bookmarks []*entities.Bookmark) error {
	if len(bookmarks) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO bookmarks (id, tweet_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, b := range bookmarks {
		_, err := stmt.ExecContext(ctx, b.ID, b.TweetID, b.UserID, b.CreatedAt)
		if err != nil {
			return fmt.Errorf("bulk create bookmark failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple bookmarks in a single transaction.
func (r *bookmarkRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM bookmarks WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete bookmarks failed: %w", err)
	}
	return nil
}

// BulkDeleteByTweetID removes all bookmarks for a tweet.
func (r *bookmarkRepo) BulkDeleteByTweetID(ctx context.Context, tweetID string) error {
	query := `DELETE FROM bookmarks WHERE tweet_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, tweetID)
	if err != nil {
		return fmt.Errorf("bulk delete bookmarks by tweet failed: %w", err)
	}
	return nil
}

// BulkDeleteByUserID removes all bookmarks made by a user.
func (r *bookmarkRepo) BulkDeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM bookmarks WHERE user_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("bulk delete bookmarks by user failed: %w", err)
	}
	return nil
}

// ======================================================================
= Advanced Queries
// ======================================================================

// GetMostBookmarkedTweets returns the most bookmarked tweets.
func (r *bookmarkRepo) GetMostBookmarkedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT t.*
		FROM tweets t
		JOIN bookmarks b ON t.id = b.tweet_id
		WHERE b.created_at >= $1
		  AND t.deleted_at IS NULL
		GROUP BY t.id
		ORDER BY COUNT(b.id) DESC
		LIMIT $2
	`
	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most bookmarked tweets failed: %w", err)
	}
	return tweets, nil
}

// GetBookmarksTimeline returns bookmarks in reverse chronological order.
func (r *bookmarkRepo) GetBookmarksTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if len(userIDs) == 0 {
		return []*entities.Bookmark{}, "", nil
	}
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT b.*
		FROM bookmarks b
		WHERE b.user_id IN (?)
	`
	if cursor != "" {
		query += ` AND b.id > $2`
	}
	query += ` ORDER BY b.created_at DESC, b.id DESC LIMIT $?`

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

	var bookmarks []*entities.Bookmark
	err = r.getDB().SelectContext(ctx, &bookmarks, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get bookmarks timeline failed: %w", err)
	}

	var nextCursor string
	if len(bookmarks) == limit {
		nextCursor = bookmarks[len(bookmarks)-1].ID
	}
	return bookmarks, nextCursor, nil
}

// GetBookmarksByDateRange returns bookmarks within a date range.
func (r *bookmarkRepo) GetBookmarksByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM bookmarks
		WHERE user_id = $1
		  AND created_at >= $2
		  AND created_at <= $3
	`
	if cursor != "" {
		query += ` AND id > $4`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, start, end}
	argIndex := 4
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 5
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var bookmarks []*entities.Bookmark
	err := r.getDB().SelectContext(ctx, &bookmarks, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get bookmarks by date range failed: %w", err)
	}

	var nextCursor string
	if len(bookmarks) == limit {
		nextCursor = bookmarks[len(bookmarks)-1].ID
	}
	return bookmarks, nextCursor, nil
}

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetBookmarkStats returns aggregated bookmark statistics.
func (r *bookmarkRepo) GetBookmarkStats(ctx context.Context) (*BookmarkStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_bookmarks,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT tweet_id) as unique_tweets,
			MAX(created_at) as last_bookmark,
			MIN(created_at) as first_bookmark
		FROM bookmarks
	`
	var stats BookmarkStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get bookmark stats failed: %w", err)
	}
	return &stats, nil
}

// BookmarkStats represents aggregated bookmark statistics.
type BookmarkStats struct {
	TotalBookmarks int64     `db:"total_bookmarks"`
	UniqueUsers    int64     `db:"unique_users"`
	UniqueTweets   int64     `db:"unique_tweets"`
	LastBookmark   time.Time `db:"last_bookmark"`
	FirstBookmark  time.Time `db:"first_bookmark"`
}

// GetDailyBookmarks returns daily bookmark counts for a date range.
func (r *bookmarkRepo) GetDailyBookmarks(ctx context.Context, start, end time.Time) ([]*DailyBookmarkCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count,
			COUNT(DISTINCT user_id) as unique_users
		FROM bookmarks
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyBookmarkCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily bookmarks failed: %w", err)
	}
	return results, nil
}

// DailyBookmarkCount represents daily bookmark counts.
type DailyBookmarkCount struct {
	Date        time.Time `db:"date"`
	Count       int64     `db:"count"`
	UniqueUsers int64     `db:"unique_users"`
}

// GetUserBookmarkStats returns bookmark statistics for a specific user.
func (r *bookmarkRepo) GetUserBookmarkStats(ctx context.Context, userID string) (*UserBookmarkStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(DISTINCT tweet_id) as unique_tweets,
			MAX(created_at) as last_bookmark,
			MIN(created_at) as first_bookmark
		FROM bookmarks
		WHERE user_id = $1
	`
	var stats UserBookmarkStats
	err := r.getDB().GetContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user bookmark stats failed: %w", err)
	}
	return &stats, nil
}

// UserBookmarkStats represents bookmark statistics for a user.
type UserBookmarkStats struct {
	Total        int64     `db:"total"`
	UniqueTweets int64     `db:"unique_tweets"`
	LastBookmark time.Time `db:"last_bookmark"`
	FirstBookmark time.Time `db:"first_bookmark"`
}

// ======================================================================
= Health
// ======================================================================

func (r *bookmarkRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *bookmarkRepo) Close() error {
	return nil
}

func (r *bookmarkRepo) GetRawDB() interface{} {
	return r.db
}