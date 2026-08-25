// backend/internal/repository/postgres/like_pg.go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
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
// Basic CRUD
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
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return interfaces.ErrAlreadyLiked
		}
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
	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.TweetID] = r.Count
	}
	return counts, nil
}

// CountByUserIDs returns like counts for multiple users (bulk).
func (r *likeRepo) CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if len(userIDs) == 0 {
		return map[string]int64{}, nil
	}
	query := `
		SELECT user_id, COUNT(*) as count
		FROM likes
		WHERE user_id IN (?)
		GROUP BY user_id
	`
	query, args, err := sqlx.In(query, userIDs)
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
		return nil, fmt.Errorf("count likes by user IDs failed: %w", err)
	}
	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.UserID] = r.Count
	}
	return counts, nil
}

// CountByDateRange returns like count within a date range.
func (r *likeRepo) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM likes WHERE created_at >= $1 AND created_at <= $2`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, start, end)
	if err != nil {
		return 0, fmt.Errorf("count likes by date range failed: %w", err)
	}
	return count, nil
}

// CountByDateRangeForUser returns like count for a user within a date range.
func (r *likeRepo) CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM likes WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, start, end)
	if err != nil {
		return 0, fmt.Errorf("count likes by date range for user failed: %w", err)
	}
	return count, nil
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
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
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
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
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

// GetLikedTweets returns full tweet objects liked by a user.
func (r *likeRepo) GetLikedTweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT t.*
		FROM tweets t
		INNER JOIN likes l ON t.id = l.tweet_id
		WHERE l.user_id = $1
		  AND t.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND t.id > $2`
	}
	query += ` ORDER BY l.created_at DESC, t.id DESC LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get liked tweets failed: %w", err)
	}
	var nextCursor string
	if len(tweets) == limit {
		nextCursor = tweets[len(tweets)-1].ID
	}
	return tweets, nextCursor, nil
}

// GetLikers returns users who liked a specific tweet.
func (r *likeRepo) GetLikers(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.User, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT u.*
		FROM users u
		INNER JOIN likes l ON u.id = l.user_id
		WHERE l.tweet_id = $1
		  AND u.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND u.id > $2`
	}
	query += ` ORDER BY l.created_at DESC, u.id DESC LIMIT $?`

	args := []interface{}{tweetID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var users []*entities.User
	err := r.getDB().SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get likers failed: %w", err)
	}
	var nextCursor string
	if len(users) == limit {
		nextCursor = users[len(users)-1].ID
	}
	return users, nextCursor, nil
}

// GetLikersWithTime returns users who liked a tweet with time of like.
func (r *likeRepo) GetLikersWithTime(ctx context.Context, tweetID string, cursor string, limit int) ([]*interfaces.LikeWithUser, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT 
			l.*,
			u.id as user_id,
			u.username,
			u.full_name,
			u.avatar_url,
			u.bio,
			u.is_verified
		FROM likes l
		INNER JOIN users u ON l.user_id = u.id
		WHERE l.tweet_id = $1
		  AND u.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND l.id > $2`
	}
	query += ` ORDER BY l.created_at DESC, l.id DESC LIMIT $?`

	args := []interface{}{tweetID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var results []struct {
		entities.Like
		UserID     string `db:"user_id"`
		Username   string `db:"username"`
		FullName   string `db:"full_name"`
		AvatarURL  string `db:"avatar_url"`
		Bio        string `db:"bio"`
		IsVerified bool   `db:"is_verified"`
	}
	err := r.getDB().SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get likers with time failed: %w", err)
	}
	var nextCursor string
	if len(results) == limit {
		nextCursor = results[len(results)-1].ID
	}
	likesWithUser := make([]*interfaces.LikeWithUser, 0, len(results))
	for _, r := range results {
		user := &entities.User{
			ID:         r.UserID,
			Username:   r.Username,
			FullName:   r.FullName,
			AvatarURL:  r.AvatarURL,
			Bio:        r.Bio,
			IsVerified: r.IsVerified,
		}
		likesWithUser = append(likesWithUser, &interfaces.LikeWithUser{
			Like: &r.Like,
			User: user,
		})
	}
	return likesWithUser, nextCursor, nil
}

// ======================================================================
// Timeline and Feed
// ======================================================================

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
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
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

// GetRecentLikes returns the most recent likes for a user.
func (r *likeRepo) GetRecentLikes(ctx context.Context, userID string, limit int) ([]*entities.Like, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT * FROM likes
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	var likes []*entities.Like
	err := r.getDB().SelectContext(ctx, &likes, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent likes failed: %w", err)
	}
	return likes, nil
}

// GetRecentLikesForTweets returns recent likes for a list of tweets.
func (r *likeRepo) GetRecentLikesForTweets(ctx context.Context, tweetIDs []string, limit int) (map[string][]*entities.Like, error) {
	if len(tweetIDs) == 0 {
		return map[string][]*entities.Like{}, nil
	}
	if limit < 1 {
		limit = 5
	}
	query := `
		SELECT l.*
		FROM likes l
		WHERE l.tweet_id IN (?)
		ORDER BY l.created_at DESC
	`
	query, args, err := sqlx.In(query, tweetIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var likes []*entities.Like
	err = r.getDB().SelectContext(ctx, &likes, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get recent likes for tweets failed: %w", err)
	}
	result := make(map[string][]*entities.Like)
	for _, like := range likes {
		if len(result[like.TweetID]) < limit {
			result[like.TweetID] = append(result[like.TweetID], like)
		}
	}
	return result, nil
}

// ======================================================================
// Advanced Queries
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
		ORDER BY COUNT(l.id) DESC, t.created_at DESC
		LIMIT $2
	`
	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most liked tweets failed: %w", err)
	}
	return tweets, nil
}

// GetMostLikedTweetsByCategory returns most liked tweets by category (e.g., media, poll).
func (r *likeRepo) GetMostLikedTweetsByCategory(ctx context.Context, category string, limit int, since time.Time) ([]*entities.Tweet, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT t.*
		FROM tweets t
		JOIN likes l ON t.id = l.tweet_id
		WHERE l.created_at >= $1
		  AND t.deleted_at IS NULL
	`
	switch category {
	case "media":
		query += ` AND array_length(t.media_urls, 1) > 0`
	case "poll":
		query += ` AND t.is_poll = true`
	default:
		// No category filter
	}
	query += ` GROUP BY t.id ORDER BY COUNT(l.id) DESC, t.created_at DESC LIMIT $2`

	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most liked tweets by category failed: %w", err)
	}
	return tweets, nil
}

// GetMostActiveLikers returns users with the most likes.
func (r *likeRepo) GetMostActiveLikers(ctx context.Context, limit int, since time.Time) ([]*interfaces.LikerStats, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT 
			u.id as user_id,
			u.username,
			u.full_name,
			u.avatar_url,
			COUNT(l.id) as like_count
		FROM users u
		JOIN likes l ON u.id = l.user_id
		WHERE l.created_at >= $1
		  AND u.deleted_at IS NULL
		GROUP BY u.id, u.username, u.full_name, u.avatar_url
		ORDER BY like_count DESC
		LIMIT $2
	`
	var stats []*interfaces.LikerStats
	err := r.getDB().SelectContext(ctx, &stats, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most active likers failed: %w", err)
	}
	return stats, nil
}

// GetLikesByLocation returns likes grouped by geographic location (if available).
func (r *likeRepo) GetLikesByLocation(ctx context.Context, tweetID string) (map[string]int64, error) {
	// Location data is not stored in likes; return empty for now
	return map[string]int64{}, nil
}

// GetLikesByHour returns likes grouped by hour of day.
func (r *likeRepo) GetLikesByHour(ctx context.Context, tweetID string) ([]*interfaces.HourlyLikeCount, error) {
	query := `
		SELECT 
			EXTRACT(HOUR FROM created_at) as hour,
			COUNT(*) as count
		FROM likes
		WHERE tweet_id = $1
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY hour ASC
	`
	var results []*interfaces.HourlyLikeCount
	err := r.getDB().SelectContext(ctx, &results, query, tweetID)
	if err != nil {
		return nil, fmt.Errorf("get likes by hour failed: %w", err)
	}
	return results, nil
}

// GetLikesByDayOfWeek returns likes grouped by day of week.
func (r *likeRepo) GetLikesByDayOfWeek(ctx context.Context, tweetID string) ([]*interfaces.DayOfWeekLikeCount, error) {
	query := `
		SELECT 
			TO_CHAR(created_at, 'Day') as day,
			COUNT(*) as count
		FROM likes
		WHERE tweet_id = $1
		GROUP BY TO_CHAR(created_at, 'Day')
		ORDER BY MIN(EXTRACT(DOW FROM created_at))
	`
	var results []*interfaces.DayOfWeekLikeCount
	err := r.getDB().SelectContext(ctx, &results, query, tweetID)
	if err != nil {
		return nil, fmt.Errorf("get likes by day of week failed: %w", err)
	}
	return results, nil
}

// ======================================================================
// Bulk Operations
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

	for _, like := range likes {
		_, err := stmt.ExecContext(ctx, like.ID, like.TweetID, like.UserID, like.CreatedAt)
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
		return fmt.Errorf("bulk delete by tweet ID failed: %w", err)
	}
	return nil
}

// BulkDeleteByUserID removes all likes made by a user.
func (r *likeRepo) BulkDeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM likes WHERE user_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("bulk delete by user ID failed: %w", err)
	}
	return nil
}

// BulkDeleteByTweetAndUser removes likes for multiple tweet-user pairs.
func (r *likeRepo) BulkDeleteByTweetAndUser(ctx context.Context, pairs []interfaces.TweetUserPair) error {
	if len(pairs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM likes WHERE tweet_id = $1 AND user_id = $2`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, pair := range pairs {
		_, err := stmt.ExecContext(ctx, pair.TweetID, pair.UserID)
		if err != nil {
			return fmt.Errorf("bulk delete by tweet and user failed: %w", err)
		}
	}
	return tx.Commit()
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetLikeStats returns aggregated like statistics.
func (r *likeRepo) GetLikeStats(ctx context.Context) (*interfaces.LikeStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_likes,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT tweet_id) as unique_tweets,
			AVG(user_likes) as likes_per_user,
			AVG(tweet_likes) as likes_per_tweet,
			MAX(created_at) as last_like,
			MIN(created_at) as first_like
		FROM likes
	`
	var stats interfaces.LikeStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get like stats failed: %w", err)
	}
	// Get most liked tweet
	var mostLiked struct {
		TweetID string `db:"tweet_id"`
		Count   int64  `db:"count"`
	}
	err = r.getDB().GetContext(ctx, &mostLiked, `
		SELECT tweet_id, COUNT(*) as count
		FROM likes
		GROUP BY tweet_id
		ORDER BY count DESC
		LIMIT 1
	`)
	if err == nil {
		stats.MostLikedTweetID = mostLiked.TweetID
		stats.MostLikedTweetCount = mostLiked.Count
	}
	// Get most active user
	var mostActive struct {
		UserID string `db:"user_id"`
		Count  int64  `db:"count"`
	}
	err = r.getDB().GetContext(ctx, &mostActive, `
		SELECT user_id, COUNT(*) as count
		FROM likes
		GROUP BY user_id
		ORDER BY count DESC
		LIMIT 1
	`)
	if err == nil {
		stats.MostActiveUserID = mostActive.UserID
		stats.MostActiveUserLikes = mostActive.Count
	}
	return &stats, nil
}

// GetUserLikeStats returns like statistics for a specific user.
func (r *likeRepo) GetUserLikeStats(ctx context.Context, userID string) (*interfaces.LikeStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_likes,
			COUNT(DISTINCT tweet_id) as unique_tweets,
			MAX(created_at) as last_like,
			MIN(created_at) as first_like
		FROM likes
		WHERE user_id = $1
	`
	var stats interfaces.LikeStats
	err := r.getDB().GetContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user like stats failed: %w", err)
	}
	stats.UniqueUsers = 1
	stats.LikesPerUser = float64(stats.TotalLikes)
	return &stats, nil
}

// GetTweetLikeStats returns like statistics for a specific tweet.
func (r *likeRepo) GetTweetLikeStats(ctx context.Context, tweetID string) (*interfaces.LikeStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_likes,
			COUNT(DISTINCT user_id) as unique_users,
			MAX(created_at) as last_like,
			MIN(created_at) as first_like
		FROM likes
		WHERE tweet_id = $1
	`
	var stats interfaces.LikeStats
	err := r.getDB().GetContext(ctx, &stats, query, tweetID)
	if err != nil {
		return nil, fmt.Errorf("get tweet like stats failed: %w", err)
	}
	stats.UniqueTweets = 1
	stats.LikesPerTweet = float64(stats.TotalLikes)
	return &stats, nil
}

// GetDailyLikeStats returns daily like counts for a date range.
func (r *likeRepo) GetDailyLikeStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailyLikeCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT tweet_id) as unique_tweets
		FROM likes
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyLikeCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily like stats failed: %w", err)
	}
	return results, nil
}

// GetDailyLikeStatsForUser returns daily like counts for a user.
func (r *likeRepo) GetDailyLikeStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*interfaces.DailyLikeCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			1 as unique_users,
			COUNT(DISTINCT tweet_id) as unique_tweets
		FROM likes
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyLikeCount
	err := r.getDB().SelectContext(ctx, &results, query, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily like stats for user failed: %w", err)
	}
	return results, nil
}

// GetLikeEngagementRate calculates like engagement rate for a tweet.
func (r *likeRepo) GetLikeEngagementRate(ctx context.Context, tweetID string) (float64, error) {
	// Need to get tweet views and likes
	var likes, views int64
	err := r.getDB().GetContext(ctx, &likes, `SELECT COUNT(*) FROM likes WHERE tweet_id = $1`, tweetID)
	if err != nil {
		return 0, fmt.Errorf("get likes for engagement failed: %w", err)
	}
	err = r.getDB().GetContext(ctx, &views, `SELECT COALESCE(view_count, 0) FROM tweets WHERE id = $1`, tweetID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("get views for engagement failed: %w", err)
	}
	if views == 0 {
		return 0, nil
	}
	return float64(likes) / float64(views) * 100, nil
}

// GetLikeConversionRate calculates conversion rate from view to like.
func (r *likeRepo) GetLikeConversionRate(ctx context.Context, tweetID string) (float64, error) {
	return r.GetLikeEngagementRate(ctx, tweetID)
}

// ======================================================================
// Health
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