// backend/internal/repository/postgres/tweet_pg.go
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
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// tweetRepo is the PostgreSQL implementation of TweetRepository.
type tweetRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx // optional transaction
	log *logrus.Entry
}

// NewTweetRepository creates a new PostgreSQL tweet repository.
func NewTweetRepository(db *sqlx.DB) interfaces.TweetRepository {
	return &tweetRepo{
		db:  db,
		log: logger.WithField("repository", "tweet_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *tweetRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.TweetRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &tweetRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *tweetRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.TweetRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &tweetRepo{
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

// getDB returns the current DB connection (either the transaction or the main DB).
func (r *tweetRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// CRUD Operations
// ======================================================================

// Create inserts a new tweet.
func (r *tweetRepo) Create(ctx context.Context, tweet *entities.Tweet) error {
	query := `
		INSERT INTO tweets (
			id, user_id, content, media_urls, parent_tweet_id, retweet_of_id,
			is_poll, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		tweet.ID, tweet.UserID, tweet.Content,
		pq.Array(tweet.MediaURLs),
		tweet.ParentTweetID, tweet.RetweetOfID,
		tweet.IsPoll,
		tweet.CreatedAt, tweet.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create tweet failed: %w", err)
	}
	return nil
}

// GetByID retrieves a tweet by its ID.
func (r *tweetRepo) GetByID(ctx context.Context, id string) (*entities.Tweet, error) {
	query := `SELECT * FROM tweets WHERE id = $1 AND deleted_at IS NULL`
	var tweet entities.Tweet
	err := r.getDB().GetContext(ctx, &tweet, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrTweetNotFound
		}
		return nil, fmt.Errorf("get tweet by ID failed: %w", err)
	}
	return &tweet, nil
}

// Update updates a tweet's content and updated_at.
func (r *tweetRepo) Update(ctx context.Context, tweet *entities.Tweet) error {
	query := `
		UPDATE tweets SET
			content = $1,
			media_urls = $2,
			updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
	`
	result, err := r.getDB().ExecContext(ctx, query,
		tweet.Content,
		pq.Array(tweet.MediaURLs),
		time.Now(),
		tweet.ID,
	)
	if err != nil {
		return fmt.Errorf("update tweet failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrTweetNotFound
	}
	return nil
}

// SoftDelete marks a tweet as deleted.
func (r *tweetRepo) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE tweets SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL`
	result, err := r.getDB().ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("soft delete tweet failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrTweetNotFound
	}
	return nil
}

// HardDelete permanently removes a tweet.
func (r *tweetRepo) HardDelete(ctx context.Context, id string) error {
	query := `DELETE FROM tweets WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("hard delete tweet failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrTweetNotFound
	}
	return nil
}

// ======================================================================
// Feed and List Queries
// ======================================================================

// GetFeed returns tweets from a list of user IDs (followed + self), ordered by created_at DESC.
func (r *tweetRepo) GetFeed(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if len(userIDs) == 0 {
		return []*entities.Tweet{}, "", nil
	}
	if limit < 1 {
		limit = 20
	}

	// Build base query
	query := `
		SELECT t.*
		FROM tweets t
		WHERE t.user_id IN (?)
		  AND t.deleted_at IS NULL
		  AND t.parent_tweet_id IS NULL       -- exclude replies from feed (or include? usually include all)
		  AND t.retweet_of_id IS NULL         -- exclude retweets? usually include retweets as separate tweets
	`
	// Actually, we should include all except deleted. We'll include replies and retweets as separate entries.
	// Better: remove parent_tweet_id and retweet_of_id filters for a simpler feed.
	// We'll include all tweets from followed users.
	query = `
		SELECT t.*
		FROM tweets t
		WHERE t.user_id IN (?)
		  AND t.deleted_at IS NULL
	`

	// Apply cursor (created_at, id) for pagination.
	if cursor != "" {
		// Cursor is encoded as "timestamp|id"
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				query += ` AND (t.created_at < $2 OR (t.created_at = $2 AND t.id < $3))`
			}
		}
	}

	query += ` ORDER BY t.created_at DESC, t.id DESC LIMIT $?`

	// Build args
	args := []interface{}{userIDs}
	argIndex := 2
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				args = append(args, ts, parts[1])
				argIndex = 4
			}
		}
	}
	args = append(args, limit)

	// Use sqlx.In to handle IN clause
	query, args, err := sqlx.In(query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)

	var tweets []*entities.Tweet
	err = r.getDB().SelectContext(ctx, &tweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get feed failed: %w", err)
	}

	// Generate next cursor
	var nextCursor string
	if len(tweets) == limit {
		last := tweets[len(tweets)-1]
		nextCursor = fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	return tweets, nextCursor, nil
}

// GetByUserID returns tweets by a specific user.
func (r *tweetRepo) GetByUserID(ctx context.Context, userID, cursor string, limit int, includeReplies bool) ([]*entities.Tweet, string, error) {
	if limit < 1 {
		limit = 20
	}

	query := `
		SELECT t.*
		FROM tweets t
		WHERE t.user_id = $1
		  AND t.deleted_at IS NULL
	`
	if !includeReplies {
		query += ` AND t.parent_tweet_id IS NULL`
	}

	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				query += ` AND (t.created_at < $2 OR (t.created_at = $2 AND t.id < $3))`
			}
		}
	}

	query += ` ORDER BY t.created_at DESC, t.id DESC LIMIT $?`

	args := []interface{}{userID}
	argIndex := 2
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				args = append(args, ts, parts[1])
				argIndex = 4
			}
		}
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get user tweets failed: %w", err)
	}

	var nextCursor string
	if len(tweets) == limit {
		last := tweets[len(tweets)-1]
		nextCursor = fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	return tweets, nextCursor, nil
}

// GetReplies returns replies to a specific tweet.
func (r *tweetRepo) GetReplies(ctx context.Context, tweetID, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if limit < 1 {
		limit = 20
	}

	query := `
		SELECT t.*
		FROM tweets t
		WHERE t.parent_tweet_id = $1
		  AND t.deleted_at IS NULL
	`
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				query += ` AND (t.created_at < $2 OR (t.created_at = $2 AND t.id < $3))`
			}
		}
	}
	query += ` ORDER BY t.created_at ASC, t.id ASC LIMIT $?`

	args := []interface{}{tweetID}
	argIndex := 2
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				args = append(args, ts, parts[1])
				argIndex = 4
			}
		}
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get replies failed: %w", err)
	}

	var nextCursor string
	if len(tweets) == limit {
		last := tweets[len(tweets)-1]
		nextCursor = fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	return tweets, nextCursor, nil
}

// CountReplies returns the number of replies to a tweet.
func (r *tweetRepo) CountReplies(ctx context.Context, tweetID string) (int64, error) {
	query := `SELECT COUNT(*) FROM tweets WHERE parent_tweet_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, tweetID)
	if err != nil {
		return 0, fmt.Errorf("count replies failed: %w", err)
	}
	return count, nil
}

// ======================================================================
// Search with Full-Text
// ======================================================================

// Search performs full-text search on tweets content.
func (r *tweetRepo) Search(ctx context.Context, queryStr, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if limit < 1 {
		limit = 20
	}

	// Use PostgreSQL tsvector with English configuration.
	// We assume a GIN index exists on the tsvector column.
	// If not, we fall back to ILIKE (slower) but okay for dev.
	// For production, create a generated column or use GIN index.

	// Build search query with ts_rank for relevance.
	sqlQuery := `
		SELECT t.*,
		       ts_rank(to_tsvector('english', content), plainto_tsquery('english', $1)) AS rank
		FROM tweets t
		WHERE to_tsvector('english', content) @@ plainto_tsquery('english', $1)
		  AND t.deleted_at IS NULL
	`
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			// For search, we can order by rank and created_at.
			// We'll use the same cursor format but with rank? Simpler: use created_at.
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				sqlQuery += ` AND (t.created_at < $2 OR (t.created_at = $2 AND t.id < $3))`
			}
		}
	}
	sqlQuery += ` ORDER BY rank DESC, t.created_at DESC, t.id DESC LIMIT $?`

	args := []interface{}{queryStr}
	argIndex := 2
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				args = append(args, ts, parts[1])
				argIndex = 4
			}
		}
	}
	args = append(args, limit)
	sqlQuery = r.getDB().Rebind(sqlQuery)

	type tweetWithRank struct {
		entities.Tweet
		Rank float64 `db:"rank"`
	}
	var results []tweetWithRank
	err := r.getDB().SelectContext(ctx, &results, sqlQuery, args...)
	if err != nil {
		return nil, "", fmt.Errorf("search tweets failed: %w", err)
	}

	tweets := make([]*entities.Tweet, len(results))
	for i, tw := range results {
		tweets[i] = &tw.Tweet
	}

	var nextCursor string
	if len(tweets) == limit {
		last := tweets[len(tweets)-1]
		nextCursor = fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	return tweets, nextCursor, nil
}

// ======================================================================
// Trending
// ======================================================================

// GetTrending returns trending topics (hashtags) based on tweet frequency over the last 24h.
func (r *tweetRepo) GetTrending(ctx context.Context, limit int) ([]*dto.TrendingTopic, error) {
	// Extract hashtags from content using regex and count occurrences.
	// This is a simplified approach; in production, you might have a separate table for hashtags.
	query := `
		SELECT 
			LOWER(SUBSTRING(content FROM '#([A-Za-z0-9_]+)')) AS hashtag,
			COUNT(*) AS count
		FROM tweets
		WHERE content ~ '#[A-Za-z0-9_]+'
		  AND created_at > NOW() - INTERVAL '24 hours'
		  AND deleted_at IS NULL
		GROUP BY hashtag
		ORDER BY count DESC, hashtag
		LIMIT $1
	`
	var results []struct {
		Hashtag string `db:"hashtag"`
		Count   int64  `db:"count"`
	}
	err := r.getDB().SelectContext(ctx, &results, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get trending failed: %w", err)
	}

	topics := make([]*dto.TrendingTopic, 0, len(results))
	for _, r := range results {
		if r.Hashtag == "" {
			continue
		}
		topics = append(topics, &dto.TrendingTopic{
			Hashtag: r.Hashtag,
			Count:   r.Count,
		})
	}
	return topics, nil
}

// ======================================================================
// Interaction Counts
// ======================================================================

// GetLikeCount returns the number of likes for a tweet.
func (r *tweetRepo) GetLikeCount(ctx context.Context, tweetID string) (int64, error) {
	query := `SELECT COUNT(*) FROM likes WHERE tweet_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, tweetID)
	if err != nil {
		return 0, fmt.Errorf("get like count failed: %w", err)
	}
	return count, nil
}

// GetRetweetCount returns the number of retweets for a tweet.
func (r *tweetRepo) GetRetweetCount(ctx context.Context, tweetID string) (int64, error) {
	query := `SELECT COUNT(*) FROM retweets WHERE tweet_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, tweetID)
	if err != nil {
		return 0, fmt.Errorf("get retweet count failed: %w", err)
	}
	return count, nil
}

// ======================================================================
// Likes
// ======================================================================

// Like adds a like to a tweet.
func (r *tweetRepo) Like(ctx context.Context, tweetID, userID string) error {
	// Check if already liked
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM likes WHERE tweet_id=$1 AND user_id=$2)`, tweetID, userID)
	if err != nil {
		return fmt.Errorf("check like existence failed: %w", err)
	}
	if exists {
		return interfaces.ErrAlreadyLiked
	}

	// Insert like
	_, err = r.getDB().ExecContext(ctx, `INSERT INTO likes (tweet_id, user_id, created_at) VALUES ($1, $2, $3)`,
		tweetID, userID, time.Now())
	if err != nil {
		return fmt.Errorf("insert like failed: %w", err)
	}
	return nil
}

// Unlike removes a like from a tweet.
func (r *tweetRepo) Unlike(ctx context.Context, tweetID, userID string) error {
	result, err := r.getDB().ExecContext(ctx, `DELETE FROM likes WHERE tweet_id=$1 AND user_id=$2`, tweetID, userID)
	if err != nil {
		return fmt.Errorf("delete like failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrLikeNotFound
	}
	return nil
}

// IsLiked checks if a user has liked a tweet.
func (r *tweetRepo) IsLiked(ctx context.Context, tweetID, userID string) (bool, error) {
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM likes WHERE tweet_id=$1 AND user_id=$2)`, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("check like status failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
// Retweets
// ======================================================================

// Retweet adds a retweet.
func (r *tweetRepo) Retweet(ctx context.Context, tweetID, userID string) error {
	// Check if already retweeted
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM retweets WHERE tweet_id=$1 AND user_id=$2)`, tweetID, userID)
	if err != nil {
		return fmt.Errorf("check retweet existence failed: %w", err)
	}
	if exists {
		return interfaces.ErrAlreadyRetweeted
	}
	// Insert retweet
	_, err = r.getDB().ExecContext(ctx, `INSERT INTO retweets (tweet_id, user_id, created_at) VALUES ($1, $2, $3)`,
		tweetID, userID, time.Now())
	if err != nil {
		return fmt.Errorf("insert retweet failed: %w", err)
	}
	return nil
}

// Unretweet removes a retweet.
func (r *tweetRepo) Unretweet(ctx context.Context, tweetID, userID string) error {
	result, err := r.getDB().ExecContext(ctx, `DELETE FROM retweets WHERE tweet_id=$1 AND user_id=$2`, tweetID, userID)
	if err != nil {
		return fmt.Errorf("delete retweet failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrRetweetNotFound
	}
	return nil
}

// IsRetweeted checks if a user has retweeted a tweet.
func (r *tweetRepo) IsRetweeted(ctx context.Context, tweetID, userID string) (bool, error) {
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM retweets WHERE tweet_id=$1 AND user_id=$2)`, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("check retweet status failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
= Bookmarks
// ======================================================================

// Bookmark adds a bookmark.
func (r *tweetRepo) Bookmark(ctx context.Context, tweetID, userID string) error {
	// Check if already bookmarked
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM bookmarks WHERE tweet_id=$1 AND user_id=$2)`, tweetID, userID)
	if err != nil {
		return fmt.Errorf("check bookmark existence failed: %w", err)
	}
	if exists {
		return interfaces.ErrAlreadyBookmarked
	}
	_, err = r.getDB().ExecContext(ctx, `INSERT INTO bookmarks (tweet_id, user_id, created_at) VALUES ($1, $2, $3)`,
		tweetID, userID, time.Now())
	if err != nil {
		return fmt.Errorf("insert bookmark failed: %w", err)
	}
	return nil
}

// Unbookmark removes a bookmark.
func (r *tweetRepo) Unbookmark(ctx context.Context, tweetID, userID string) error {
	result, err := r.getDB().ExecContext(ctx, `DELETE FROM bookmarks WHERE tweet_id=$1 AND user_id=$2`, tweetID, userID)
	if err != nil {
		return fmt.Errorf("delete bookmark failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrBookmarkNotFound
	}
	return nil
}

// IsBookmarked checks if a user has bookmarked a tweet.
func (r *tweetRepo) IsBookmarked(ctx context.Context, tweetID, userID string) (bool, error) {
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM bookmarks WHERE tweet_id=$1 AND user_id=$2)`, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("check bookmark status failed: %w", err)
	}
	return exists, nil
}

// GetBookmarksByUser returns bookmarked tweets for a user.
func (r *tweetRepo) GetBookmarksByUser(ctx context.Context, userID, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT t.*
		FROM tweets t
		JOIN bookmarks b ON t.id = b.tweet_id
		WHERE b.user_id = $1
		  AND t.deleted_at IS NULL
	`
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				query += ` AND (t.created_at < $2 OR (t.created_at = $2 AND t.id < $3))`
			}
		}
	}
	query += ` ORDER BY t.created_at DESC, t.id DESC LIMIT $?`

	args := []interface{}{userID}
	argIndex := 2
	if cursor != "" {
		parts := strings.Split(cursor, "|")
		if len(parts) == 2 {
			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err == nil {
				args = append(args, ts, parts[1])
				argIndex = 4
			}
		}
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get bookmarks failed: %w", err)
	}
	var nextCursor string
	if len(tweets) == limit {
		last := tweets[len(tweets)-1]
		nextCursor = fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
	}
	return tweets, nextCursor, nil
}

// ======================================================================
= Polls
// ======================================================================

// CreatePoll creates a new poll.
func (r *tweetRepo) CreatePoll(ctx context.Context, poll *entities.Poll) error {
	query := `
		INSERT INTO polls (id, tweet_id, options, duration, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		poll.ID, poll.TweetID,
		pq.Array(poll.Options),
		poll.Duration,
		poll.ExpiresAt,
		poll.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create poll failed: %w", err)
	}
	return nil
}

// GetPollByTweetID retrieves a poll for a tweet.
func (r *tweetRepo) GetPollByTweetID(ctx context.Context, tweetID string) (*entities.Poll, error) {
	query := `SELECT * FROM polls WHERE tweet_id = $1`
	var poll entities.Poll
	err := r.getDB().GetContext(ctx, &poll, query, tweetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrPollNotFound
		}
		return nil, fmt.Errorf("get poll by tweet failed: %w", err)
	}
	return &poll, nil
}

// VotePoll adds a vote to a poll option.
func (r *tweetRepo) VotePoll(ctx context.Context, pollID, userID, optionID string) error {
	// We need to atomically update the poll's options JSON to increment votes for the option.
	// We'll use a transactional approach.
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current poll
	var poll entities.Poll
	err = tx.GetContext(ctx, &poll, `SELECT * FROM polls WHERE id = $1`, pollID)
	if err != nil {
		return err
	}
	// Check expiration
	if time.Now().After(poll.ExpiresAt) {
		return ErrPollExpired
	}
	// Check if user already voted
	for _, opt := range poll.Options {
		if opt.VoterIDs != nil {
			for _, uid := range opt.VoterIDs {
				if uid == userID {
					return ErrPollAlreadyVoted
				}
			}
		}
	}
	// Update option
	found := false
	for i, opt := range poll.Options {
		if opt.ID == optionID {
			opt.Votes++
			if opt.VoterIDs == nil {
				opt.VoterIDs = []string{}
			}
			opt.VoterIDs = append(opt.VoterIDs, userID)
			poll.Options[i] = opt
			found = true
			break
		}
	}
	if !found {
		return ErrInvalidPollOption
	}
	// Update poll
	_, err = tx.ExecContext(ctx,
		`UPDATE polls SET options = $1 WHERE id = $2`,
		pq.Array(poll.Options), pollID,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// GetPollResults returns the poll with vote counts.
func (r *tweetRepo) GetPollResults(ctx context.Context, pollID string) (*entities.Poll, error) {
	return r.GetPollByTweetID(ctx, pollID) // same as get, but we already have it
}

// ======================================================================
= Health
// ======================================================================

func (r *tweetRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *tweetRepo) Close() error {
	return nil
}

func (r *tweetRepo) GetRawDB() interface{} {
	return r.db
}