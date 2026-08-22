// backend/internal/repository/interfaces/poll_repo.go
package interfaces

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ======================================================================
// Common Errors
// ======================================================================

var (
	ErrPollNotFound        = errors.New("poll not found")
	ErrPollExpired         = errors.New("poll has expired")
	ErrPollAlreadyVoted    = errors.New("user already voted on this poll")
	ErrInvalidPollOption   = errors.New("invalid poll option")
	ErrInvalidPollID       = errors.New("invalid poll ID")
	ErrInvalidTweetID      = errors.New("invalid tweet ID")
	ErrInvalidUserID       = errors.New("invalid user ID")
	ErrPollNotActive       = errors.New("poll is not active")
	ErrPollResultsHidden   = errors.New("poll results are hidden until expiration")
	ErrPollVoteTooLate     = errors.New("poll voting has closed")
	ErrPollOptionNotFound  = errors.New("poll option not found")
	ErrPollOptionDuplicate = errors.New("duplicate poll option")
	ErrPollOptionTooMany   = errors.New("too many poll options")
	ErrPollOptionTooFew    = errors.New("too few poll options")
	ErrPollDurationInvalid = errors.New("invalid poll duration")
)

// ======================================================================
// PollFilter
// ======================================================================

// PollFilter defines filtering options for poll queries.
type PollFilter struct {
	TweetID     *string
	UserID      *string
	IsActive    *bool
	IsExpired   *bool
	HasVotes    *bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	ExpiresFrom *time.Time
	ExpiresTo   *time.Time
	MinOptions  *int
	MaxOptions  *int
}

// HasCriteria checks if any filter criteria are set.
func (f *PollFilter) HasCriteria() bool {
	return f.TweetID != nil || f.UserID != nil || f.IsActive != nil ||
		f.IsExpired != nil || f.HasVotes != nil || f.CreatedFrom != nil ||
		f.CreatedTo != nil || f.ExpiresFrom != nil || f.ExpiresTo != nil ||
		f.MinOptions != nil || f.MaxOptions != nil
}

// ======================================================================
// PollPagination
// ======================================================================

// PollSortField defines sortable fields for polls.
type PollSortField string

const (
	SortPollByCreatedAt PollSortField = "created_at"
	SortPollByExpiresAt PollSortField = "expires_at"
	SortPollByTotalVotes PollSortField = "total_votes"
)

// PollSortOrder defines sort order.
type PollSortOrder string

const (
	PollSortAsc  PollSortOrder = "ASC"
	PollSortDesc PollSortOrder = "DESC"
)

// PollPagination holds pagination options for polls.
type PollPagination struct {
	Cursor string          `json:"cursor"`
	Limit  int             `json:"limit"`
	SortBy PollSortField   `json:"sort_by"`
	Order  PollSortOrder   `json:"order"`
}

// DefaultPollPagination returns default pagination options.
func DefaultPollPagination() *PollPagination {
	return &PollPagination{
		Limit:  20,
		SortBy: SortPollByCreatedAt,
		Order:  PollSortDesc,
	}
}

// Validate checks pagination parameters.
func (p *PollPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// PollStats
// ======================================================================

// PollStats represents aggregated poll statistics.
type PollStats struct {
	TotalPolls      int64     `json:"total_polls"`
	ActivePolls     int64     `json:"active_polls"`
	ExpiredPolls    int64     `json:"expired_polls"`
	TotalVotes      int64     `json:"total_votes"`
	UniqueVoters    int64     `json:"unique_voters"`
	UniqueTweets    int64     `json:"unique_tweets"`
	AverageOptions  float64   `json:"average_options"`
	AverageVotes    float64   `json:"average_votes"`
	LastPollCreated time.Time `json:"last_poll_created"`
	LastPollExpired time.Time `json:"last_poll_expired"`
	MostVotedPollID string    `json:"most_voted_poll_id"`
	MostVotedPollVotes int64  `json:"most_voted_poll_votes"`
	MostActiveVoterID string  `json:"most_active_voter_id"`
	MostActiveVoterVotes int64 `json:"most_active_voter_votes"`
}

// ======================================================================
// DailyPollCount
// ======================================================================

// DailyPollCount represents daily poll counts.
type DailyPollCount struct {
	Date         time.Time `json:"date"`
	Total        int64     `json:"total"`
	Active       int64     `json:"active"`
	Expired      int64     `json:"expired"`
	UniqueVoters int64     `json:"unique_voters"`
	TotalVotes   int64     `json:"total_votes"`
}

// ======================================================================
// PollOption
// ======================================================================

// PollOption represents a poll option.
type PollOption struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Votes       int64    `json:"votes"`
	VoterIDs    []string `json:"voter_ids,omitempty"`
	Percentage  float64  `json:"percentage,omitempty"`
}

// ======================================================================
// PollResult
// ======================================================================

// PollResult represents poll results.
type PollResult struct {
	PollID       string       `json:"poll_id"`
	TweetID      string       `json:"tweet_id"`
	Options      []PollOption `json:"options"`
	TotalVotes   int64        `json:"total_votes"`
	ExpiresAt    time.Time    `json:"expires_at"`
	IsExpired    bool         `json:"is_expired"`
	VotedOptionID string      `json:"voted_option_id,omitempty"`
}

// ======================================================================
// PollRepository Interface
// ======================================================================

// PollRepository defines the interface for poll data persistence.
type PollRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a new poll.
	Create(ctx context.Context, poll *entities.Poll) error

	// GetByID retrieves a poll by its ID.
	GetByID(ctx context.Context, id string) (*entities.Poll, error)

	// GetByTweetID retrieves a poll by its tweet ID.
	GetByTweetID(ctx context.Context, tweetID string) (*entities.Poll, error)

	// GetByTweetIDs retrieves polls for multiple tweets.
	GetByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]*entities.Poll, error)

	// Update updates a poll (e.g., after voting).
	Update(ctx context.Context, poll *entities.Poll) error

	// Delete removes a poll.
	Delete(ctx context.Context, id string) error

	// DeleteByTweetID removes a poll by tweet ID.
	DeleteByTweetID(ctx context.Context, tweetID string) error

	// --------------------------------------------------------------------
	// Voting Operations
	// --------------------------------------------------------------------

	// Vote adds a vote to an option.
	Vote(ctx context.Context, pollID, userID, optionID string) error

	// Unvote removes a vote from a poll.
	Unvote(ctx context.Context, pollID, userID, optionID string) error

	// GetUserVote returns the option ID a user voted for.
	GetUserVote(ctx context.Context, pollID, userID string) (string, error)

	// HasUserVoted checks if a user has voted on a poll.
	HasUserVoted(ctx context.Context, pollID, userID string) (bool, error)

	// GetVoteCount returns the total number of votes for a poll.
	GetVoteCount(ctx context.Context, pollID string) (int64, error)

	// GetVoteCounts returns the vote count for each option.
	GetVoteCounts(ctx context.Context, pollID string) (map[string]int64, error)

	// GetVoters returns all voters for a poll.
	GetVoters(ctx context.Context, pollID string, cursor string, limit int) ([]string, string, error)

	// GetVotersByOption returns voters for a specific option.
	GetVotersByOption(ctx context.Context, pollID, optionID string, cursor string, limit int) ([]string, string, error)

	// --------------------------------------------------------------------
	// Expiration Operations
	// --------------------------------------------------------------------

	// GetExpiredPolls returns polls that have expired.
	GetExpiredPolls(ctx context.Context, before time.Time, limit int) ([]*entities.Poll, error)

	// GetPollsExpiringSoon returns polls that will expire soon.
	GetPollsExpiringSoon(ctx context.Context, within time.Duration, limit int) ([]*entities.Poll, error)

	// GetActivePolls returns all active polls.
	GetActivePolls(ctx context.Context, cursor string, limit int) ([]*entities.Poll, string, error)

	// GetExpiredPollsForTweet returns expired polls for a tweet.
	GetExpiredPollsForTweet(ctx context.Context, tweetID string) ([]*entities.Poll, error)

	// ExtendExpiration extends a poll's expiration time.
	ExtendExpiration(ctx context.Context, pollID string, newExpiry time.Time) error

	// --------------------------------------------------------------------
	// List Operations
	// --------------------------------------------------------------------

	// List returns polls with filtering and pagination.
	List(ctx context.Context, filter *PollFilter, pagination *PollPagination) ([]*entities.Poll, int64, error)

	// GetByUserID returns polls created by a user.
	GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Poll, string, error)

	// GetByDateRange returns polls within a date range.
	GetByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Poll, string, error)

	// GetTrendingPolls returns the most active polls.
	GetTrendingPolls(ctx context.Context, limit int, since time.Time) ([]*entities.Poll, error)

	// GetMostVotedPolls returns the most voted polls.
	GetMostVotedPolls(ctx context.Context, limit int, since time.Time) ([]*entities.Poll, error)

	// --------------------------------------------------------------------
	// Results and Analytics
	// --------------------------------------------------------------------

	// GetResults returns the poll results.
	GetResults(ctx context.Context, pollID string) (*PollResult, error)

	// GetLiveResults returns real-time poll results.
	GetLiveResults(ctx context.Context, pollID string) (*PollResult, error)

	// GetPollEngagementRate calculates engagement rate for a poll.
	GetPollEngagementRate(ctx context.Context, pollID string) (float64, error)

	// GetVoteDistribution returns vote distribution statistics.
	GetVoteDistribution(ctx context.Context, pollID string) (map[string]float64, error)

	// GetVoterDemographics returns voter demographic data (if available).
	GetVoterDemographics(ctx context.Context, pollID string) (map[string]int64, error)

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetPollStats returns aggregated poll statistics.
	GetPollStats(ctx context.Context) (*PollStats, error)

	// GetUserPollStats returns poll statistics for a specific user.
	GetUserPollStats(ctx context.Context, userID string) (*PollStats, error)

	// GetDailyPollStats returns daily poll counts for a date range.
	GetDailyPollStats(ctx context.Context, start, end time.Time) ([]*DailyPollCount, error)

	// GetDailyPollStatsForUser returns daily poll counts for a user.
	GetDailyPollStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyPollCount, error)

	// GetPollTypeStats returns stats by poll type.
	GetPollTypeStats(ctx context.Context) ([]*PollTypeStat, error)

	// GetPollParticipationStats returns participation statistics.
	GetPollParticipationStats(ctx context.Context, pollID string) (*PollParticipationStats, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple polls in a single transaction.
	BulkCreate(ctx context.Context, polls []*entities.Poll) error

	// BulkDelete removes multiple polls in a single transaction.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkDeleteByTweetID removes polls for multiple tweets.
	BulkDeleteByTweetID(ctx context.Context, tweetIDs []string) error

	// BulkVote adds votes for multiple users/options.
	BulkVote(ctx context.Context, votes []VoteEntry) error

	// BulkUpdateStatus updates status for multiple polls.
	BulkUpdateStatus(ctx context.Context, ids []string, status string) error

	// CleanupExpired removes expired polls and their data.
	CleanupExpired(ctx context.Context, before time.Time) (int64, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) PollRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo PollRepository) error) error

	// --------------------------------------------------------------------
	// Health and Cleanup
	// --------------------------------------------------------------------

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close releases any resources.
	Close() error

	// GetRawDB returns the underlying database connection.
	GetRawDB() interface{}
}

// ======================================================================
// Supporting Types
// ======================================================================

// VoteEntry represents a vote entry for bulk operations.
type VoteEntry struct {
	PollID   string `json:"poll_id"`
	UserID   string `json:"user_id"`
	OptionID string `json:"option_id"`
}

// PollTypeStat represents poll statistics by type.
type PollTypeStat struct {
	Type        string `json:"type"`
	Count       int64  `json:"count"`
	TotalVotes  int64  `json:"total_votes"`
	UniqueUsers int64  `json:"unique_users"`
}

// PollParticipationStats represents participation statistics.
type PollParticipationStats struct {
	TotalVotes    int64   `json:"total_votes"`
	UniqueVoters  int64   `json:"unique_voters"`
	TotalOptions  int64   `json:"total_options"`
	AverageVotes  float64 `json:"average_votes"`
	TurnoutRate   float64 `json:"turnout_rate"`
	IsExpired     bool    `json:"is_expired"`
}

// ======================================================================
// Helper Functions
// ======================================================================

// IsPollNotFound checks if an error indicates a poll was not found.
func IsPollNotFound(err error) bool {
	return errors.Is(err, ErrPollNotFound)
}

// IsPollExpired checks if an error indicates a poll expired.
func IsPollExpired(err error) bool {
	return errors.Is(err, ErrPollExpired) || errors.Is(err, ErrPollVoteTooLate)
}

// IsPollAlreadyVoted checks if an error indicates already voted.
func IsPollAlreadyVoted(err error) bool {
	return errors.Is(err, ErrPollAlreadyVoted)
}

// IsPollError checks if an error is poll-related.
func IsPollError(err error) bool {
	return errors.Is(err, ErrPollNotFound) ||
		errors.Is(err, ErrPollExpired) ||
		errors.Is(err, ErrPollAlreadyVoted) ||
		errors.Is(err, ErrInvalidPollOption) ||
		errors.Is(err, ErrInvalidPollID) ||
		errors.Is(err, ErrInvalidTweetID) ||
		errors.Is(err, ErrInvalidUserID)
}

// ======================================================================
// Mock Poll Repository (for testing)
// ======================================================================

// MockPollRepository is a mock implementation for testing.
type MockPollRepository struct {
	Polls      map[string]*entities.Poll
	Votes      map[string]map[string]string // pollID -> userID -> optionID
	Error      error
	NextCursor string
}

// NewMockPollRepo creates a new mock repository.
func NewMockPollRepo() PollRepository {
	return &MockPollRepository{
		Polls: make(map[string]*entities.Poll),
		Votes: make(map[string]map[string]string),
	}
}

// Create mock implementation.
func (m *MockPollRepository) Create(ctx context.Context, poll *entities.Poll) error {
	if m.Error != nil {
		return m.Error
	}
	m.Polls[poll.ID] = poll
	return nil
}

// GetByID mock implementation.
func (m *MockPollRepository) GetByID(ctx context.Context, id string) (*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if poll, ok := m.Polls[id]; ok {
		return poll, nil
	}
	return nil, ErrPollNotFound
}

// GetByTweetID mock implementation.
func (m *MockPollRepository) GetByTweetID(ctx context.Context, tweetID string) (*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	for _, poll := range m.Polls {
		if poll.TweetID == tweetID {
			return poll, nil
		}
	}
	return nil, ErrPollNotFound
}

// GetByTweetIDs mock implementation.
func (m *MockPollRepository) GetByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]*entities.Poll)
	for _, tid := range tweetIDs {
		for _, poll := range m.Polls {
			if poll.TweetID == tid {
				result[tid] = poll
				break
			}
		}
	}
	return result, nil
}

// Update mock implementation.
func (m *MockPollRepository) Update(ctx context.Context, poll *entities.Poll) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Polls[poll.ID]; !ok {
		return ErrPollNotFound
	}
	m.Polls[poll.ID] = poll
	return nil
}

// Delete mock implementation.
func (m *MockPollRepository) Delete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Polls[id]; ok {
		delete(m.Polls, id)
		return nil
	}
	return ErrPollNotFound
}

// DeleteByTweetID mock implementation.
func (m *MockPollRepository) DeleteByTweetID(ctx context.Context, tweetID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, poll := range m.Polls {
		if poll.TweetID == tweetID {
			delete(m.Polls, id)
		}
	}
	return nil
}

// Vote mock implementation.
func (m *MockPollRepository) Vote(ctx context.Context, pollID, userID, optionID string) error {
	if m.Error != nil {
		return m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return ErrPollNotFound
	}
	if time.Now().After(poll.ExpiresAt) {
		return ErrPollExpired
	}
	if m.Votes[pollID] == nil {
		m.Votes[pollID] = make(map[string]string)
	}
	if _, ok := m.Votes[pollID][userID]; ok {
		return ErrPollAlreadyVoted
	}
	// Find option
	found := false
	for i, opt := range poll.Options {
		if opt.ID == optionID {
			poll.Options[i].Votes++
			if poll.Options[i].VoterIDs == nil {
				poll.Options[i].VoterIDs = []string{}
			}
			poll.Options[i].VoterIDs = append(poll.Options[i].VoterIDs, userID)
			found = true
			break
		}
	}
	if !found {
		return ErrInvalidPollOption
	}
	m.Votes[pollID][userID] = optionID
	m.Polls[pollID] = poll
	return nil
}

// Unvote mock implementation.
func (m *MockPollRepository) Unvote(ctx context.Context, pollID, userID, optionID string) error {
	if m.Error != nil {
		return m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return ErrPollNotFound
	}
	if m.Votes[pollID] == nil {
		return ErrPollAlreadyVoted
	}
	if _, ok := m.Votes[pollID][userID]; !ok {
		return ErrPollAlreadyVoted
	}
	// Find option
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
			poll.Options[i].Votes = int64(len(newVoters))
			poll.Options[i].VoterIDs = newVoters
			break
		}
	}
	delete(m.Votes[pollID], userID)
	m.Polls[pollID] = poll
	return nil
}

// GetUserVote mock implementation.
func (m *MockPollRepository) GetUserVote(ctx context.Context, pollID, userID string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	if m.Votes[pollID] == nil {
		return "", nil
	}
	if optionID, ok := m.Votes[pollID][userID]; ok {
		return optionID, nil
	}
	return "", nil
}

// HasUserVoted mock implementation.
func (m *MockPollRepository) HasUserVoted(ctx context.Context, pollID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Votes[pollID] == nil {
		return false, nil
	}
	_, ok := m.Votes[pollID][userID]
	return ok, nil
}

// GetVoteCount mock implementation.
func (m *MockPollRepository) GetVoteCount(ctx context.Context, pollID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return 0, ErrPollNotFound
	}
	count := int64(0)
	for _, opt := range poll.Options {
		count += opt.Votes
	}
	return count, nil
}

// GetVoteCounts mock implementation.
func (m *MockPollRepository) GetVoteCounts(ctx context.Context, pollID string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return nil, ErrPollNotFound
	}
	result := make(map[string]int64)
	for _, opt := range poll.Options {
		result[opt.ID] = opt.Votes
	}
	return result, nil
}

// GetVoters mock implementation.
func (m *MockPollRepository) GetVoters(ctx context.Context, pollID string, cursor string, limit int) ([]string, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	if m.Votes[pollID] == nil {
		return []string{}, "", nil
	}
	var voters []string
	for userID := range m.Votes[pollID] {
		voters = append(voters, userID)
	}
	return voters, "", nil
}

// GetVotersByOption mock implementation.
func (m *MockPollRepository) GetVotersByOption(ctx context.Context, pollID, optionID string, cursor string, limit int) ([]string, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return nil, "", ErrPollNotFound
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

// GetExpiredPolls mock implementation.
func (m *MockPollRepository) GetExpiredPolls(ctx context.Context, before time.Time, limit int) ([]*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var expired []*entities.Poll
	for _, poll := range m.Polls {
		if poll.ExpiresAt.Before(before) {
			expired = append(expired, poll)
		}
	}
	return expired, nil
}

// GetPollsExpiringSoon mock implementation.
func (m *MockPollRepository) GetPollsExpiringSoon(ctx context.Context, within time.Duration, limit int) ([]*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	now := time.Now()
	var soon []*entities.Poll
	for _, poll := range m.Polls {
		if poll.ExpiresAt.After(now) && poll.ExpiresAt.Before(now.Add(within)) {
			soon = append(soon, poll)
		}
	}
	return soon, nil
}

// GetActivePolls mock implementation.
func (m *MockPollRepository) GetActivePolls(ctx context.Context, cursor string, limit int) ([]*entities.Poll, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var active []*entities.Poll
	now := time.Now()
	for _, poll := range m.Polls {
		if poll.ExpiresAt.After(now) {
			active = append(active, poll)
		}
	}
	return active, "", nil
}

// GetExpiredPollsForTweet mock implementation.
func (m *MockPollRepository) GetExpiredPollsForTweet(ctx context.Context, tweetID string) ([]*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var expired []*entities.Poll
	now := time.Now()
	for _, poll := range m.Polls {
		if poll.TweetID == tweetID && poll.ExpiresAt.Before(now) {
			expired = append(expired, poll)
		}
	}
	return expired, nil
}

// ExtendExpiration mock implementation.
func (m *MockPollRepository) ExtendExpiration(ctx context.Context, pollID string, newExpiry time.Time) error {
	if m.Error != nil {
		return m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return ErrPollNotFound
	}
	poll.ExpiresAt = newExpiry
	return nil
}

// List mock implementation.
func (m *MockPollRepository) List(ctx context.Context, filter *PollFilter, pagination *PollPagination) ([]*entities.Poll, int64, error) {
	if m.Error != nil {
		return nil, 0, m.Error
	}
	var polls []*entities.Poll
	for _, poll := range m.Polls {
		polls = append(polls, poll)
	}
	return polls, int64(len(polls)), nil
}

// GetByUserID mock implementation.
func (m *MockPollRepository) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Poll, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	// We don't have user association in poll entity, return empty
	return []*entities.Poll{}, "", nil
}

// GetByDateRange mock implementation.
func (m *MockPollRepository) GetByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Poll, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var polls []*entities.Poll
	for _, poll := range m.Polls {
		if poll.CreatedAt.After(start) && poll.CreatedAt.Before(end) {
			polls = append(polls, poll)
		}
	}
	return polls, "", nil
}

// GetTrendingPolls mock implementation.
func (m *MockPollRepository) GetTrendingPolls(ctx context.Context, limit int, since time.Time) ([]*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Poll{}, nil
}

// GetMostVotedPolls mock implementation.
func (m *MockPollRepository) GetMostVotedPolls(ctx context.Context, limit int, since time.Time) ([]*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Poll{}, nil
}

// GetResults mock implementation.
func (m *MockPollRepository) GetResults(ctx context.Context, pollID string) (*PollResult, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return nil, ErrPollNotFound
	}
	options := make([]PollOption, 0, len(poll.Options))
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	for _, opt := range poll.Options {
		percentage := float64(0)
		if totalVotes > 0 {
			percentage = (float64(opt.Votes) / float64(totalVotes)) * 100
		}
		options = append(options, PollOption{
			ID:         opt.ID,
			Text:       opt.Text,
			Votes:      opt.Votes,
			VoterIDs:   opt.VoterIDs,
			Percentage: percentage,
		})
	}
	return &PollResult{
		PollID:     pollID,
		TweetID:    poll.TweetID,
		Options:    options,
		TotalVotes: totalVotes,
		ExpiresAt:  poll.ExpiresAt,
		IsExpired:  time.Now().After(poll.ExpiresAt),
	}, nil
}

// GetLiveResults mock implementation.
func (m *MockPollRepository) GetLiveResults(ctx context.Context, pollID string) (*PollResult, error) {
	return m.GetResults(ctx, pollID)
}

// GetPollEngagementRate mock implementation.
func (m *MockPollRepository) GetPollEngagementRate(ctx context.Context, pollID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// GetVoteDistribution mock implementation.
func (m *MockPollRepository) GetVoteDistribution(ctx context.Context, pollID string) (map[string]float64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return nil, ErrPollNotFound
	}
	result := make(map[string]float64)
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	if totalVotes == 0 {
		return result, nil
	}
	for _, opt := range poll.Options {
		result[opt.ID] = (float64(opt.Votes) / float64(totalVotes)) * 100
	}
	return result, nil
}

// GetVoterDemographics mock implementation.
func (m *MockPollRepository) GetVoterDemographics(ctx context.Context, pollID string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return map[string]int64{}, nil
}

// GetPollStats mock implementation.
func (m *MockPollRepository) GetPollStats(ctx context.Context) (*PollStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &PollStats{
		TotalPolls: int64(len(m.Polls)),
	}, nil
}

// GetUserPollStats mock implementation.
func (m *MockPollRepository) GetUserPollStats(ctx context.Context, userID string) (*PollStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.GetPollStats(ctx)
}

// GetDailyPollStats mock implementation.
func (m *MockPollRepository) GetDailyPollStats(ctx context.Context, start, end time.Time) ([]*DailyPollCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyPollCount{}, nil
}

// GetDailyPollStatsForUser mock implementation.
func (m *MockPollRepository) GetDailyPollStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyPollCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyPollCount{}, nil
}

// GetPollTypeStats mock implementation.
func (m *MockPollRepository) GetPollTypeStats(ctx context.Context) ([]*PollTypeStat, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*PollTypeStat{}, nil
}

// GetPollParticipationStats mock implementation.
func (m *MockPollRepository) GetPollParticipationStats(ctx context.Context, pollID string) (*PollParticipationStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return nil, ErrPollNotFound
	}
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	return &PollParticipationStats{
		TotalVotes:   totalVotes,
		UniqueVoters: totalVotes, // In real implementation, count distinct users
		TotalOptions: int64(len(poll.Options)),
		IsExpired:    time.Now().After(poll.ExpiresAt),
	}, nil
}

// BulkCreate mock implementation.
func (m *MockPollRepository) BulkCreate(ctx context.Context, polls []*entities.Poll) error {
	if m.Error != nil {
		return m.Error
	}
	for _, poll := range polls {
		m.Polls[poll.ID] = poll
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockPollRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		delete(m.Polls, id)
	}
	return nil
}

// BulkDeleteByTweetID mock implementation.
func (m *MockPollRepository) BulkDeleteByTweetID(ctx context.Context, tweetIDs []string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, poll := range m.Polls {
		for _, tid := range tweetIDs {
			if poll.TweetID == tid {
				delete(m.Polls, id)
				break
			}
		}
	}
	return nil
}

// BulkVote mock implementation.
func (m *MockPollRepository) BulkVote(ctx context.Context, votes []VoteEntry) error {
	if m.Error != nil {
		return m.Error
	}
	for _, v := range votes {
		_ = m.Vote(ctx, v.PollID, v.UserID, v.OptionID)
	}
	return nil
}

// BulkUpdateStatus mock implementation.
func (m *MockPollRepository) BulkUpdateStatus(ctx context.Context, ids []string, status string) error {
	if m.Error != nil {
		return m.Error
	}
	// Status field doesn't exist in poll entity, so this is a no-op for mock
	return nil
}

// CleanupExpired mock implementation.
func (m *MockPollRepository) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for id, poll := range m.Polls {
		if poll.ExpiresAt.Before(before) {
			delete(m.Polls, id)
			count++
		}
	}
	return count, nil
}

// WithTransaction mock implementation.
func (m *MockPollRepository) WithTransaction(ctx context.Context, tx *sql.Tx) PollRepository {
	return m
}

// Transaction mock implementation.
func (m *MockPollRepository) Transaction(ctx context.Context, fn func(txRepo PollRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockPollRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockPollRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockPollRepository) GetRawDB() interface{} {
	return nil
}