// backend/internal/repository/postgres/poll_pg.go
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
// Basic Poll Operations
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

// Update updates a poll (e.g., options after voting).
func (r *pollRepo) Update(ctx context.Context, poll *entities.Poll) error {
	optionsJSON, err := json.Marshal(poll.Options)
	if err != nil {
		return fmt.Errorf("marshal options failed: %w", err)
	}
	query := `
		UPDATE polls SET options = $1, expires_at = $2
		WHERE id = $3
	`
	result, err := r.getDB().ExecContext(ctx, query, optionsJSON, poll.ExpiresAt, poll.ID)
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
= Voting Operations
// ======================================================================

// Vote adds a vote to an option with optimistic locking.
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
	var voted bool
	for _, opt := range poll.Options {
		for _, uid := range opt.VoterIDs {
			if uid == userID {
				voted = true
				break
			}
		}
		if voted {
			break
		}
	}
	if voted {
		return interfaces.ErrPollAlreadyVoted
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

// ======================================================================
= Vote Removal (for undo)
// ======================================================================

// Unvote removes a user's vote from a poll.
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

// ======================================================================
= Vote Count and Results
// ======================================================================

// GetResults returns the poll results with vote counts.
func (r *pollRepo) GetResults(ctx context.Context, pollID string) (*entities.Poll, error) {
	return r.GetByID(ctx, pollID)
}

// GetTotalVotes returns the total number of votes for a poll.
func (r *pollRepo) GetTotalVotes(ctx context.Context, pollID string) (int64, error) {
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

// HasUserVoted checks if a user has voted on a poll.
func (r *pollRepo) HasUserVoted(ctx context.Context, pollID, userID string) (bool, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return false, err
	}
	for _, opt := range poll.Options {
		for _, uid := range opt.VoterIDs {
			if uid == userID {
				return true, nil
			}
		}
	}
	return false, nil
}

// GetUserVote returns the option ID a user voted for, or empty string.
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

// ======================================================================
= Expiration Handling
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

// ======================================================================
= Analytics
// ======================================================================

// GetPollStats returns aggregated poll statistics.
func (r *pollRepo) GetPollStats(ctx context.Context) (*PollStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_polls,
			COUNT(DISTINCT tweet_id) as unique_tweets,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired_polls,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active_polls,
			AVG(EXTRACT(EPOCH FROM (expires_at - created_at))) as avg_duration_seconds,
			MAX(created_at) as latest_poll,
			MIN(created_at) as earliest_poll
		FROM polls
	`
	var stats PollStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get poll stats failed: %w", err)
	}
	return &stats, nil
}

// PollStats represents aggregated poll statistics.
type PollStats struct {
	TotalPolls        int64     `db:"total_polls"`
	UniqueTweets      int64     `db:"unique_tweets"`
	ExpiredPolls      int64     `db:"expired_polls"`
	ActivePolls       int64     `db:"active_polls"`
	AvgDurationSeconds float64  `db:"avg_duration_seconds"`
	LatestPoll        time.Time `db:"latest_poll"`
	EarliestPoll      time.Time `db:"earliest_poll"`
}

// GetDailyPolls returns daily poll creation counts.
func (r *pollRepo) GetDailyPolls(ctx context.Context, start, end time.Time) ([]*DailyPollCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count,
			COUNT(DISTINCT tweet_id) as unique_tweets,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired_count
		FROM polls
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyPollCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily polls failed: %w", err)
	}
	return results, nil
}

// DailyPollCount represents daily poll counts.
type DailyPollCount struct {
	Date         time.Time `db:"date"`
	Count        int64     `db:"count"`
	UniqueTweets int64     `db:"unique_tweets"`
	ExpiredCount int64     `db:"expired_count"`
}

// GetPollParticipationStats returns participation stats for a poll.
func (r *pollRepo) GetPollParticipationStats(ctx context.Context, pollID string) (*PollParticipationStats, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return nil, err
	}
	totalVotes := int64(0)
	uniqueVoters := make(map[string]bool)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
		for _, uid := range opt.VoterIDs {
			uniqueVoters[uid] = true
		}
	}
	stats := &PollParticipationStats{
		TotalVotes:    totalVotes,
		UniqueVoters:  int64(len(uniqueVoters)),
		TotalOptions:  int64(len(poll.Options)),
		IsExpired:     time.Now().After(poll.ExpiresAt),
	}
	return stats, nil
}

// PollParticipationStats represents participation statistics.
type PollParticipationStats struct {
	TotalVotes   int64 `json:"total_votes"`
	UniqueVoters int64 `json:"unique_voters"`
	TotalOptions int64 `json:"total_options"`
	IsExpired    bool  `json:"is_expired"`
}

// GetOptionPopularity returns a ranking of options by vote count.
func (r *pollRepo) GetOptionPopularity(ctx context.Context, pollID string) ([]OptionVote, error) {
	poll, err := r.GetByID(ctx, pollID)
	if err != nil {
		return nil, err
	}
	options := make([]OptionVote, 0, len(poll.Options))
	for _, opt := range poll.Options {
		options = append(options, OptionVote{
			OptionID: opt.ID,
			Text:     opt.Text,
			Votes:    opt.Votes,
		})
	}
	// Sort by votes desc
	for i := 0; i < len(options); i++ {
		for j := i + 1; j < len(options); j++ {
			if options[j].Votes > options[i].Votes {
				options[i], options[j] = options[j], options[i]
			}
		}
	}
	return options, nil
}

// OptionVote represents an option with vote count.
type OptionVote struct {
	OptionID string `json:"option_id"`
	Text     string `json:"text"`
	Votes    int64  `json:"votes"`
}

// ======================================================================
= Bulk Operations
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
		_, err = stmt.ExecContext(ctx, p.ID, p.TweetID, optionsJSON, p.Duration, p.ExpiresAt, p.CreatedAt)
		if err != nil {
			return fmt.Errorf("bulk create poll failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple polls in a single transaction.
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

// ======================================================================
= Health
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