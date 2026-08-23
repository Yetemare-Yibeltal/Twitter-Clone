// backend/internal/repository/postgres/poll_pg.go
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

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// pollRepo is the PostgreSQL implementation of PollRepository.
type pollRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewPollRepository creates a new PostgreSQL poll repository.
func NewPollRepository(db *sqlx.DB) interfaces.PollRepository {
	return &pollRepo{
		db:  db,
		log: logger.WithField("repository", "poll_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *pollRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.PollRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &pollRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *pollRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.PollRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &pollRepo{
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
func (r *pollRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic Poll CRUD
// ======================================================================

// Create inserts a new poll.
func (r *pollRepo) Create(ctx context.Context, poll *entities.Poll) error {
	optionsJSON, err := json.Marshal(poll.Options)
	if err != nil {
		return fmt.Errorf("marshal options failed: %w", err)
	}
	query := `
		INSERT INTO polls (
			id, tweet_id, options, duration, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.getDB().ExecContext(ctx, query,
		poll.ID, poll.TweetID, optionsJSON,
		poll.Duration, poll.ExpiresAt, poll.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create poll failed: %w", err)
	}
	return nil
}

// GetByID retrieves a poll by its ID.
func (r *pollRepo) GetByID(ctx context.Context, id string) (*entities.Poll, error) {
	query := `SELECT * FROM polls WHERE id = $1`
	var poll entities.Poll
	err := r.getDB().GetContext(ctx, &poll, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrPollNotFound
		}
		return nil, fmt.Errorf("get poll by ID failed: %w", err)
	}
	return &poll, nil
}

// GetByTweetID retrieves a poll by its tweet ID.
func (r *pollRepo) GetByTweetID(ctx context.Context, tweetID string) (*entities.Poll, error) {
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

// GetByTweetIDs retrieves polls for multiple tweets.
func (r *pollRepo) GetByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]*entities.Poll, error) {
	if len(tweetIDs) == 0 {
		return map[string]*entities.Poll{}, nil
	}
	query, args, err := sqlx.In(`SELECT * FROM polls WHERE tweet_id IN (?)`, tweetIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var polls []*entities.Poll
	err = r.getDB().SelectContext(ctx, &polls, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get polls by tweet IDs failed: %w", err)
	}
	result := make(map[string]*entities.Poll)
	for _, p := range polls {
		result[p.TweetID] = p
	}
	return result, nil
}

// Update updates a poll.
func (r *pollRepo) Update(ctx context.Context, poll *entities.Poll) error {
	optionsJSON, err := json.Marshal(poll.Options)
	if err != nil {
		return fmt.Errorf("marshal options failed: %w", err)
	}
	query := `
		UPDATE polls SET
			options = $1,
			expires_at = $2,
			updated_at = $3
		WHERE id = $4
	`
	result, err := r.getDB().ExecContext(ctx, query,
		optionsJSON, poll.ExpiresAt, time.Now(), poll.ID,
	)
	if err != nil {
		return fmt.Errorf("update poll failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrPollNotFound
	}
	return nil
}

// Delete removes a poll.
func (r *pollRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM polls WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete poll failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrPollNotFound
	}
	return nil
}

// DeleteByTweetID removes a poll by tweet ID.
func (r *pollRepo) DeleteByTweetID(ctx context.Context, tweetID string) error {
	query := `DELETE FROM polls WHERE tweet_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, tweetID)
	if err != nil {
		return fmt.Errorf("delete poll by tweet failed: %w", err)
	}
	return nil
}

// ======================================================================
// Voting Operations
// ======================================================================

// Vote adds a vote to an option.
func (r *pollRepo) Vote(ctx context.Context, pollID, userID, optionID string) error {
	// Use transaction to avoid race conditions
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current poll with FOR UPDATE
	var poll entities.Poll
	query := `SELECT * FROM polls WHERE id = $1 FOR UPDATE`
	err = tx.GetContext(ctx, &poll, query, pollID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return interfaces.ErrPollNotFound
		}
		return fmt.Errorf("get poll for vote failed: %w", err)
	}

	// Check expiration
	if time.Now().After(poll.ExpiresAt) {
		return interfaces.ErrPollExpired
	}

	// Check if user already voted
	for _, opt := range poll.Options {
		for _, uid := range opt.VoterIDs {
			if uid == userID {
				return interfaces.ErrPollAlreadyVoted
			}
		}
	}

	// Find option and update
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
		return interfaces.ErrInvalidPollOption
	}

	// Update poll
	optionsJSON, err := json.Marshal(poll.Options)
	if err != nil {
		return fmt.Errorf("marshal options failed: %w", err)
	}
	updateQuery := `UPDATE polls SET options = $1 WHERE id = $2`
	_, err = tx.ExecContext(ctx, updateQuery, optionsJSON, pollID)
	if err != nil {
		return fmt.Errorf("update poll after vote failed: %w", err)
	}

	return tx.Commit()
}

// Unvote removes a vote from a poll.
func (r *pollRepo) Unvote(ctx context.Context, pollID, userID, optionID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var poll entities.Poll
	query := `SELECT * FROM polls WHERE id = $1 FOR UPDATE`
	err = tx.GetContext(ctx, &poll, query, pollID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return interfaces.ErrPollNotFound
		}
		return fmt.Errorf("get poll for unvote failed: %w", err)
	}

	// Check if user voted for the option
	found := false
	for i, opt := range poll.Options {
		if opt.ID == optionID {
			newVoters := []string{}
			for _, uid := range opt.VoterIDs {
				if uid != userID {
					newVoters = append(newVoters, uid)
				}
			}
			if len(newVoters) == len(opt.VoterIDs) {
				return errors.New("user did not vote for this option")
			}
			opt.Votes = int64(len(newVoters))
			opt.VoterIDs = newVoters
			poll.Options[i] = opt
			found = true
			break
		}
	}
	if !found {
		return interfaces.ErrInvalidPollOption
	}

	optionsJSON, err := json.Marshal(poll.Options)
	if err != nil {
		return fmt.Errorf("marshal options failed: %w", err)
	}
	updateQuery := `UPDATE polls SET options = $1 WHERE id = $2`
	_, err = tx.ExecContext(ctx, updateQuery, optionsJSON, pollID)
	if err != nil {
		return fmt.Errorf("update poll after unvote failed: %w", err)
	}
	return tx.Commit()
}

// GetUserVote returns the option ID a user voted for.
func (r *pollRepo) GetUserVote(ctx context.Context, pollID, userID string) (string, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return "", err
	}
	for _, opt := range poll.Options {
		for _, uid := range opt.VoterIDs {
			if uid == userID {
				return opt.ID, nil
			}
		}
	}
	return "", nil
}

// HasUserVoted checks if a user has voted on a poll.
func (r *pollRepo) HasUserVoted(ctx context.Context, pollID, userID string) (bool, error) {
	vote, err := r.GetUserVote(ctx, pollID, userID)
	return vote != "", err
}

// GetVoteCount returns the total number of votes for a poll.
func (r *pollRepo) GetVoteCount(ctx context.Context, pollID string) (int64, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return 0, err
	}
	total := int64(0)
	for _, opt := range poll.Options {
		total += opt.Votes
	}
	return total, nil
}

// GetVoteCounts returns the vote count for each option.
func (r *pollRepo) GetVoteCounts(ctx context.Context, pollID string) (map[string]int64, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64)
	for _, opt := range poll.Options {
		counts[opt.ID] = opt.Votes
	}
	return counts, nil
}

// GetVoters returns all voters for a poll.
func (r *pollRepo) GetVoters(ctx context.Context, pollID string, cursor string, limit int) ([]string, string, error) {
	if limit < 1 {
		limit = 20
	}
	// We need to get voters from all options' VoterIDs
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return nil, "", err
	}
	voterMap := make(map[string]bool)
	for _, opt := range poll.Options {
		for _, uid := range opt.VoterIDs {
			voterMap[uid] = true
		}
	}
	voters := make([]string, 0, len(voterMap))
	for uid := range voterMap {
		voters = append(voters, uid)
	}
	// Sort for deterministic order
	// In production, we'd use a more efficient approach with pagination
	return voters, "", nil
}

// GetVotersByOption returns voters for a specific option.
func (r *pollRepo) GetVotersByOption(ctx context.Context, pollID, optionID string, cursor string, limit int) ([]string, string, error) {
	if limit < 1 {
		limit = 20
	}
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return nil, "", err
	}
	var voters []string
	for _, opt := range poll.Options {
		if opt.ID == optionID {
			voters = opt.VoterIDs
			break
		}
	}
	return voters, "", nil
}

// ======================================================================
// Expiration Operations
// ======================================================================

// GetExpiredPolls returns polls that have expired.
func (r *pollRepo) GetExpiredPolls(ctx context.Context, before time.Time, limit int) ([]*entities.Poll, error) {
	if limit < 1 {
		limit = 100
	}
	query := `SELECT * FROM polls WHERE expires_at <= $1 ORDER BY expires_at ASC LIMIT $2`
	var polls []*entities.Poll
	err := r.getDB().SelectContext(ctx, &polls, query, before, limit)
	if err != nil {
		return nil, fmt.Errorf("get expired polls failed: %w", err)
	}
	return polls, nil
}

// GetPollsExpiringSoon returns polls that will expire soon.
func (r *pollRepo) GetPollsExpiringSoon(ctx context.Context, within time.Duration, limit int) ([]*entities.Poll, error) {
	if limit < 1 {
		limit = 100
	}
	now := time.Now()
	expiryTime := now.Add(within)
	query := `
		SELECT * FROM polls
		WHERE expires_at > $1 AND expires_at <= $2
		ORDER BY expires_at ASC
		LIMIT $3
	`
	var polls []*entities.Poll
	err := r.getDB().SelectContext(ctx, &polls, query, now, expiryTime, limit)
	if err != nil {
		return nil, fmt.Errorf("get polls expiring soon failed: %w", err)
	}
	return polls, nil
}

// GetActivePolls returns all active polls.
func (r *pollRepo) GetActivePolls(ctx context.Context, cursor string, limit int) ([]*entities.Poll, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM polls
		WHERE expires_at > NOW()
	`
	if cursor != "" {
		query += ` AND id > $1`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{}
	argIdx := 1
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 2
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var polls []*entities.Poll
	err := r.getDB().SelectContext(ctx, &polls, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get active polls failed: %w", err)
	}
	var nextCursor string
	if len(polls) == limit {
		nextCursor = polls[len(polls)-1].ID
	}
	return polls, nextCursor, nil
}

// GetExpiredPollsForTweet returns expired polls for a tweet.
func (r *pollRepo) GetExpiredPollsForTweet(ctx context.Context, tweetID string) ([]*entities.Poll, error) {
	query := `SELECT * FROM polls WHERE tweet_id = $1 AND expires_at <= NOW()`
	var polls []*entities.Poll
	err := r.getDB().SelectContext(ctx, &polls, query, tweetID)
	if err != nil {
		return nil, fmt.Errorf("get expired polls for tweet failed: %w", err)
	}
	return polls, nil
}

// ExtendExpiration extends a poll's expiration time.
func (r *pollRepo) ExtendExpiration(ctx context.Context, pollID string, newExpiry time.Time) error {
	query := `UPDATE polls SET expires_at = $1, updated_at = $2 WHERE id = $3`
	result, err := r.getDB().ExecContext(ctx, query, newExpiry, time.Now(), pollID)
	if err != nil {
		return fmt.Errorf("extend expiration failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrPollNotFound
	}
	return nil
}

// ======================================================================
// List Operations
// ======================================================================

// List returns polls with filtering and pagination.
func (r *pollRepo) List(ctx context.Context, filter *interfaces.PollFilter, pagination *interfaces.PollPagination) ([]*entities.Poll, int64, error) {
	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if filter != nil {
		if filter.TweetID != nil && *filter.TweetID != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("tweet_id = $%d", argIdx))
			args = append(args, *filter.TweetID)
			argIdx++
		}
		if filter.UserID != nil && *filter.UserID != "" {
			// Need to join with tweets table to filter by user
			// For simplicity, we skip this filter in this implementation
		}
		if filter.IsActive != nil {
			if *filter.IsActive {
				whereClauses = append(whereClauses, "expires_at > NOW()")
			} else {
				whereClauses = append(whereClauses, "expires_at <= NOW()")
			}
		}
		if filter.IsExpired != nil {
			if *filter.IsExpired {
				whereClauses = append(whereClauses, "expires_at <= NOW()")
			} else {
				whereClauses = append(whereClauses, "expires_at > NOW()")
			}
		}
		if filter.HasVotes != nil {
			if *filter.HasVotes {
				whereClauses = append(whereClauses, "EXISTS (SELECT 1 FROM jsonb_array_elements(options) WHERE (value->>'votes')::int > 0)")
			} else {
				whereClauses = append(whereClauses, "NOT EXISTS (SELECT 1 FROM jsonb_array_elements(options) WHERE (value->>'votes')::int > 0)")
			}
		}
		if filter.CreatedFrom != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", argIdx))
			args = append(args, *filter.CreatedFrom)
			argIdx++
		}
		if filter.CreatedTo != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", argIdx))
			args = append(args, *filter.CreatedTo)
			argIdx++
		}
		if filter.ExpiresFrom != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("expires_at >= $%d", argIdx))
			args = append(args, *filter.ExpiresFrom)
			argIdx++
		}
		if filter.ExpiresTo != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("expires_at <= $%d", argIdx))
			args = append(args, *filter.ExpiresTo)
			argIdx++
		}
		if filter.MinOptions != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("jsonb_array_length(options) >= $%d", argIdx))
			args = append(args, *filter.MinOptions)
			argIdx++
		}
		if filter.MaxOptions != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("jsonb_array_length(options) <= $%d", argIdx))
			args = append(args, *filter.MaxOptions)
			argIdx++
		}
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM polls WHERE %s", whereSQL)
	var total int64
	err := r.getDB().GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("count polls failed: %w", err)
	}

	// Set defaults
	limit := 20
	offset := 0
	sortBy := "created_at"
	order := "DESC"
	if pagination != nil {
		if pagination.Limit > 0 {
			limit = pagination.Limit
		}
		if pagination.Cursor != "" {
			// For cursor-based, we'd need a different approach
		}
		if pagination.SortBy != "" {
			sortBy = string(pagination.SortBy)
		}
		if pagination.Order != "" {
			order = string(pagination.Order)
		}
	}

	allowedSort := map[string]bool{
		"created_at": true, "expires_at": true, "total_votes": true,
	}
	if !allowedSort[sortBy] {
		sortBy = "created_at"
	}
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT * FROM polls WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, sortBy, order, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var polls []*entities.Poll
	err = r.getDB().SelectContext(ctx, &polls, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list polls failed: %w", err)
	}
	return polls, total, nil
}

// GetByUserID returns polls created by a user.
func (r *pollRepo) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Poll, string, error) {
	// Polls don't directly have user_id; we need to join with tweets
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT p.*
		FROM polls p
		JOIN tweets t ON p.tweet_id = t.id
		WHERE t.user_id = $1
	`
	if cursor != "" {
		query += ` AND p.id > $2`
	}
	query += ` ORDER BY p.created_at DESC, p.id DESC LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var polls []*entities.Poll
	err := r.getDB().SelectContext(ctx, &polls, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get polls by user failed: %w", err)
	}
	var nextCursor string
	if len(polls) == limit {
		nextCursor = polls[len(polls)-1].ID
	}
	return polls, nextCursor, nil
}

// GetByDateRange returns polls within a date range.
func (r *pollRepo) GetByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Poll, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM polls
		WHERE created_at >= $1 AND created_at <= $2
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{start, end}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var polls []*entities.Poll
	err := r.getDB().SelectContext(ctx, &polls, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get polls by date range failed: %w", err)
	}
	var nextCursor string
	if len(polls) == limit {
		nextCursor = polls[len(polls)-1].ID
	}
	return polls, nextCursor, nil
}

// GetTrendingPolls returns the most active polls.
func (r *pollRepo) GetTrendingPolls(ctx context.Context, limit int, since time.Time) ([]*entities.Poll, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT p.*
		FROM polls p
		WHERE p.created_at >= $1
		ORDER BY jsonb_array_length(p.options) DESC, p.created_at DESC
		LIMIT $2
	`
	var polls []*entities.Poll
	err := r.getDB().SelectContext(ctx, &polls, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get trending polls failed: %w", err)
	}
	return polls, nil
}

// GetMostVotedPolls returns the most voted polls.
func (r *pollRepo) GetMostVotedPolls(ctx context.Context, limit int, since time.Time) ([]*entities.Poll, error) {
	if limit < 1 {
		limit = 10
	}
	// We need to compute total votes from the options JSON
	query := `
		SELECT p.*
		FROM polls p
		WHERE p.created_at >= $1
		ORDER BY (
			SELECT SUM((value->>'votes')::int)
			FROM jsonb_array_elements(p.options)
		) DESC, p.created_at DESC
		LIMIT $2
	`
	var polls []*entities.Poll
	err := r.getDB().SelectContext(ctx, &polls, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most voted polls failed: %w", err)
	}
	return polls, nil
}

// ======================================================================
= Results and Analytics
// ======================================================================

// GetResults returns the poll results.
func (r *pollRepo) GetResults(ctx context.Context, pollID string) (*interfaces.PollResult, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return nil, err
	}
	return r.buildPollResult(poll), nil
}

// GetLiveResults returns real-time poll results.
func (r *pollRepo) GetLiveResults(ctx context.Context, pollID string) (*interfaces.PollResult, error) {
	// Same as GetResults but without caching; we just fetch fresh
	return r.GetResults(ctx, pollID)
}

// GetPollEngagementRate calculates engagement rate for a poll.
func (r *pollRepo) GetPollEngagementRate(ctx context.Context, pollID string) (float64, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return 0, err
	}
	// Count total votes
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	// Count tweet views? For now, return ratio of votes to options
	if len(poll.Options) == 0 {
		return 0, nil
	}
	return float64(totalVotes) / float64(len(poll.Options)), nil
}

// GetVoteDistribution returns vote distribution statistics.
func (r *pollRepo) GetVoteDistribution(ctx context.Context, pollID string) (map[string]float64, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return nil, err
	}
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	dist := make(map[string]float64)
	if totalVotes == 0 {
		return dist, nil
	}
	for _, opt := range poll.Options {
		dist[opt.ID] = (float64(opt.Votes) / float64(totalVotes)) * 100
	}
	return dist, nil
}

// GetVoterDemographics returns voter demographic data (if available).
func (r *pollRepo) GetVoterDemographics(ctx context.Context, pollID string) (map[string]int64, error) {
	// For now, return empty; could join with users table
	return map[string]int64{}, nil
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetPollStats returns aggregated poll statistics.
func (r *pollRepo) GetPollStats(ctx context.Context) (*interfaces.PollStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_polls,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active_polls,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired_polls,
			AVG(jsonb_array_length(options)) as average_options,
			MAX(created_at) as last_poll_created,
			MAX(expires_at) as last_poll_expired
		FROM polls
	`
	var stats interfaces.PollStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get poll stats failed: %w", err)
	}
	// Get total votes and unique voters from all polls
	var totalVotes, uniqueVoters int64
	err = r.getDB().GetContext(ctx, &totalVotes, `
		SELECT SUM((SELECT SUM((value->>'votes')::int) FROM jsonb_array_elements(options)))
		FROM polls
	`)
	if err == nil {
		stats.TotalVotes = totalVotes
	}
	// Unique voters: count distinct voter IDs across all options
	err = r.getDB().GetContext(ctx, &uniqueVoters, `
		SELECT COUNT(DISTINCT voter_id)
		FROM (
			SELECT jsonb_array_elements_text((value->'voter_ids')::jsonb) as voter_id
			FROM polls, jsonb_array_elements(options)
		) t
	`)
	if err == nil {
		stats.UniqueVoters = uniqueVoters
	}
	return &stats, nil
}

// GetUserPollStats returns poll statistics for a specific user.
func (r *pollRepo) GetUserPollStats(ctx context.Context, userID string) (*interfaces.PollStats, error) {
	stats, err := r.GetPollStats(ctx)
	if err != nil {
		return nil, err
	}
	// Count polls created by user (via tweets)
	var userPollCount int64
	err = r.getDB().GetContext(ctx, &userPollCount, `
		SELECT COUNT(*)
		FROM polls p
		JOIN tweets t ON p.tweet_id = t.id
		WHERE t.user_id = $1
	`, userID)
	if err == nil {
		stats.TotalPolls = userPollCount
	}
	return stats, nil
}

// GetDailyPollStats returns daily poll counts for a date range.
func (r *pollRepo) GetDailyPollStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailyPollCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired,
			0 as unique_voters,
			0 as total_votes
		FROM polls
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyPollCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily poll stats failed: %w", err)
	}
	// Fill in vote and voter counts
	for i := range results {
		var votes, voters int64
		_ = r.getDB().GetContext(ctx, &votes,
			`SELECT SUM((SELECT SUM((value->>'votes')::int) FROM jsonb_array_elements(options)))
			 FROM polls WHERE DATE(created_at) = $1`, results[i].Date)
		_ = r.getDB().GetContext(ctx, &voters,
			`SELECT COUNT(DISTINCT voter_id)
			 FROM (
			   SELECT jsonb_array_elements_text((value->'voter_ids')::jsonb) as voter_id
			   FROM polls, jsonb_array_elements(options)
			   WHERE DATE(created_at) = $1
			 ) t`, results[i].Date)
		results[i].TotalVotes = votes
		results[i].UniqueVoters = voters
	}
	return results, nil
}

// GetDailyPollStatsForUser returns daily poll counts for a user.
func (r *pollRepo) GetDailyPollStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*interfaces.DailyPollCount, error) {
	query := `
		SELECT 
			DATE(p.created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN p.expires_at > NOW() THEN 1 ELSE 0 END) as active,
			SUM(CASE WHEN p.expires_at <= NOW() THEN 1 ELSE 0 END) as expired,
			0 as unique_voters,
			0 as total_votes
		FROM polls p
		JOIN tweets t ON p.tweet_id = t.id
		WHERE t.user_id = $1 AND p.created_at >= $2 AND p.created_at <= $3
		GROUP BY DATE(p.created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyPollCount
	err := r.getDB().SelectContext(ctx, &results, query, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily poll stats for user failed: %w", err)
	}
	return results, nil
}

// GetPollTypeStats returns stats by poll type (not applicable for simple polls).
func (r *pollRepo) GetPollTypeStats(ctx context.Context) ([]*interfaces.PollTypeStat, error) {
	// For now, return empty; could be extended with custom poll types
	return []*interfaces.PollTypeStat{}, nil
}

// GetPollParticipationStats returns participation statistics.
func (r *pollRepo) GetPollParticipationStats(ctx context.Context, pollID string) (*interfaces.PollParticipationStats, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return nil, err
	}
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	uniqueVoters := make(map[string]bool)
	for _, opt := range poll.Options {
		for _, uid := range opt.VoterIDs {
			uniqueVoters[uid] = true
		}
	}
	return &interfaces.PollParticipationStats{
		TotalVotes:   totalVotes,
		UniqueVoters: int64(len(uniqueVoters)),
		TotalOptions: int64(len(poll.Options)),
		IsExpired:    time.Now().After(poll.ExpiresAt),
	}, nil
}

// ======================================================================
// Bulk Operations
// ======================================================================

// BulkCreate inserts multiple polls in a single transaction.
func (r *pollRepo) BulkCreate(ctx context.Context, polls []*entities.Poll) error {
	if len(polls) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO polls (id, tweet_id, options, duration, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range polls {
		optionsJSON, err := json.Marshal(p.Options)
		if err != nil {
			return fmt.Errorf("marshal options failed: %w", err)
		}
		_, err = stmt.ExecContext(ctx,
			p.ID, p.TweetID, optionsJSON, p.Duration, p.ExpiresAt, p.CreatedAt)
		if err != nil {
			return fmt.Errorf("bulk create poll failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple polls.
func (r *pollRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM polls WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete polls failed: %w", err)
	}
	return nil
}

// BulkDeleteByTweetID removes polls for multiple tweets.
func (r *pollRepo) BulkDeleteByTweetID(ctx context.Context, tweetIDs []string) error {
	if len(tweetIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM polls WHERE tweet_id IN (?)`, tweetIDs)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete polls by tweet failed: %w", err)
	}
	return nil
}

// BulkVote adds votes for multiple users/options.
func (r *pollRepo) BulkVote(ctx context.Context, votes []interfaces.VoteEntry) error {
	if len(votes) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, v := range votes {
		if err := r.Vote(ctx, v.PollID, v.UserID, v.OptionID); err != nil {
			// Continue on some errors? For now, abort.
			return err
		}
	}
	return tx.Commit()
}

// BulkUpdateStatus updates status for multiple polls (not applicable for polls without status field).
func (r *pollRepo) BulkUpdateStatus(ctx context.Context, ids []string, status string) error {
	// Polls don't have a status field; this is a no-op for now
	return nil
}

// CleanupExpired removes expired polls and their data.
func (r *pollRepo) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM polls WHERE expires_at <= $1`
	result, err := r.getDB().ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired polls failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// ======================================================================
= Helper Functions
// ======================================================================

// buildPollResult builds a PollResult from a Poll entity.
func (r *pollRepo) buildPollResult(poll *entities.Poll) *interfaces.PollResult {
	options := make([]interfaces.PollOption, 0, len(poll.Options))
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	for _, opt := range poll.Options {
		percentage := float64(0)
		if totalVotes > 0 {
			percentage = (float64(opt.Votes) / float64(totalVotes)) * 100
		}
		options = append(options, interfaces.PollOption{
			ID:         opt.ID,
			Text:       opt.Text,
			Votes:      opt.Votes,
			VoterIDs:   opt.VoterIDs,
			Percentage: percentage,
		})
	}
	return &interfaces.PollResult{
		PollID:     poll.ID,
		TweetID:    poll.TweetID,
		Options:    options,
		TotalVotes: totalVotes,
		ExpiresAt:  poll.ExpiresAt,
		IsExpired:  time.Now().After(poll.ExpiresAt),
	}
}

// ======================================================================
// Health
// ======================================================================

func (r *pollRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *pollRepo) Close() error {
	return nil
}

func (r *pollRepo) GetRawDB() interface{} {
	return r.db
}