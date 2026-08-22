// backend/internal/repository/interfaces/retweet_repo.go
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
	ErrRetweetNotFound      = errors.New("retweet not found")
	ErrAlreadyRetweeted     = errors.New("already retweeted this tweet")
	ErrInvalidRetweetID     = errors.New("invalid retweet ID")
	ErrInvalidTweetID       = errors.New("invalid tweet ID")
	ErrInvalidUserID        = errors.New("invalid user ID")
	ErrCannotRetweetOwn     = errors.New("cannot retweet your own tweet")
	ErrRetweetDisabled      = errors.New("retweeting is disabled for this tweet")
	ErrRetweetNotFoundByUser = errors.New("retweet not found for this user and tweet")
)

// ======================================================================
// RetweetFilter
// ======================================================================

// RetweetFilter defines filtering options for retweet queries.
type RetweetFilter struct {
	TweetID    *string
	UserID     *string
	CreatedFrom *time.Time
	CreatedTo  *time.Time
	MinRetweets *int64
	MaxRetweets *int64
}

// HasCriteria checks if any filter criteria are set.
func (f *RetweetFilter) HasCriteria() bool {
	return f.TweetID != nil || f.UserID != nil ||
		f.CreatedFrom != nil || f.CreatedTo != nil ||
		f.MinRetweets != nil || f.MaxRetweets != nil
}

// ======================================================================
// RetweetPagination
// ======================================================================

// RetweetSortField defines sortable fields for retweets.
type RetweetSortField string

const (
	SortRetweetByCreatedAt RetweetSortField = "created_at"
	SortRetweetByUpdatedAt RetweetSortField = "updated_at"
)

// RetweetSortOrder defines sort order.
type RetweetSortOrder string

const (
	RetweetSortAsc  RetweetSortOrder = "ASC"
	RetweetSortDesc RetweetSortOrder = "DESC"
)

// RetweetPagination holds pagination options for retweets.
type RetweetPagination struct {
	Cursor string             `json:"cursor"`
	Limit  int                `json:"limit"`
	SortBy RetweetSortField   `json:"sort_by"`
	Order  RetweetSortOrder   `json:"order"`
}

// DefaultRetweetPagination returns default pagination options.
func DefaultRetweetPagination() *RetweetPagination {
	return &RetweetPagination{
		Limit:  20,
		SortBy: SortRetweetByCreatedAt,
		Order:  RetweetSortDesc,
	}
}

// Validate checks pagination parameters.
func (p *RetweetPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// RetweetStats
// ======================================================================

// RetweetStats represents aggregated retweet statistics.
type RetweetStats struct {
	TotalRetweets        int64     `json:"total_retweets"`
	UniqueUsers          int64     `json:"unique_users"`
	UniqueTweets         int64     `json:"unique_tweets"`
	RetweetsPerUser      float64   `json:"retweets_per_user"`
	RetweetsPerTweet     float64   `json:"retweets_per_tweet"`
	LastRetweet          time.Time `json:"last_retweet"`
	FirstRetweet         time.Time `json:"first_retweet"`
	MostRetweetedTweetID string    `json:"most_retweeted_tweet_id"`
	MostRetweetedTweetCount int64  `json:"most_retweeted_tweet_count"`
	MostActiveRetweeterID string   `json:"most_active_retweeter_id"`
	MostActiveRetweeterCount int64 `json:"most_active_retweeter_count"`
}

// ======================================================================
= DailyRetweetCount
// ======================================================================

// DailyRetweetCount represents daily retweet counts.
type DailyRetweetCount struct {
	Date         time.Time `json:"date"`
	Total        int64     `json:"total"`
	UniqueUsers  int64     `json:"unique_users"`
	UniqueTweets int64     `json:"unique_tweets"`
}

// ======================================================================
= RetweetRepository Interface
// ======================================================================

// RetweetRepository defines the interface for retweet data persistence.
type RetweetRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a new retweet.
	Create(ctx context.Context, retweet *entities.Retweet) error

	// GetByID retrieves a retweet by its ID.
	GetByID(ctx context.Context, id string) (*entities.Retweet, error)

	// GetByTweetAndUser retrieves a retweet by tweet ID and user ID.
	GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Retweet, error)

	// Delete removes a retweet.
	Delete(ctx context.Context, id string) error

	// DeleteByTweetAndUser removes a retweet by tweet and user.
	DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error

	// --------------------------------------------------------------------
	// Existence Checks
	// --------------------------------------------------------------------

	// Exists checks if a user has retweeted a tweet.
	Exists(ctx context.Context, tweetID, userID string) (bool, error)

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountByTweetID returns the number of retweets for a tweet.
	CountByTweetID(ctx context.Context, tweetID string) (int64, error)

	// CountByUserID returns the total number of retweets made by a user.
	CountByUserID(ctx context.Context, userID string) (int64, error)

	// CountByTweetIDs returns retweet counts for multiple tweets (bulk).
	CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error)

	// CountByUserIDs returns retweet counts for multiple users (bulk).
	CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error)

	// CountByDateRange returns retweet count within a date range.
	CountByDateRange(ctx context.Context, start, end time.Time) (int64, error)

	// CountByDateRangeForUser returns retweet count for a user within a date range.
	CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error)

	// --------------------------------------------------------------------
	// List Operations
	// --------------------------------------------------------------------

	// GetByTweetID returns all retweets for a tweet with pagination.
	GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Retweet, string, error)

	// GetByUserID returns all retweets made by a user with pagination.
	GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Retweet, string, error)

	// GetRetweetedTweetIDs returns all tweet IDs retweeted by a user.
	GetRetweetedTweetIDs(ctx context.Context, userID string) ([]string, error)

	// GetRetweetedTweets returns full tweet objects retweeted by a user.
	GetRetweetedTweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// GetRetweeters returns users who retweeted a specific tweet.
	GetRetweeters(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.User, string, error)

	// GetRetweetersWithTime returns users who retweeted a tweet with time of retweet.
	GetRetweetersWithTime(ctx context.Context, tweetID string, cursor string, limit int) ([]*RetweetWithUser, string, error)

	// --------------------------------------------------------------------
	// Timeline and Feed
	// --------------------------------------------------------------------

	// GetRetweetsTimeline returns retweets in reverse chronological order for a user's feed.
	GetRetweetsTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Retweet, string, error)

	// GetRecentRetweets returns the most recent retweets for a user.
	GetRecentRetweets(ctx context.Context, userID string, limit int) ([]*entities.Retweet, error)

	// GetRecentRetweetsForTweets returns recent retweets for a list of tweets.
	GetRecentRetweetsForTweets(ctx context.Context, tweetIDs []string, limit int) (map[string][]*entities.Retweet, error)

	// --------------------------------------------------------------------
	// Advanced Queries
	// --------------------------------------------------------------------

	// GetMostRetweetedTweets returns the most retweeted tweets (trending).
	GetMostRetweetedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error)

	// GetMostRetweetedTweetsByCategory returns most retweeted tweets by category.
	GetMostRetweetedTweetsByCategory(ctx context.Context, category string, limit int, since time.Time) ([]*entities.Tweet, error)

	// GetMostActiveRetweeters returns users with the most retweets.
	GetMostActiveRetweeters(ctx context.Context, limit int, since time.Time) ([]*RetweeterStats, error)

	// GetRetweetsByHour returns retweets grouped by hour of day.
	GetRetweetsByHour(ctx context.Context, tweetID string) ([]*HourlyRetweetCount, error)

	// GetRetweetsByDayOfWeek returns retweets grouped by day of week.
	GetRetweetsByDayOfWeek(ctx context.Context, tweetID string) ([]*DayOfWeekRetweetCount, error)

	// GetRetweetChains returns retweet chains for a tweet (who retweeted whom).
	GetRetweetChains(ctx context.Context, tweetID string, maxDepth int) ([]*RetweetChain, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple retweets in a single transaction.
	BulkCreate(ctx context.Context, retweets []*entities.Retweet) error

	// BulkDelete removes multiple retweets in a single transaction.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkDeleteByTweetID removes all retweets for a tweet.
	BulkDeleteByTweetID(ctx context.Context, tweetID string) error

	// BulkDeleteByUserID removes all retweets made by a user.
	BulkDeleteByUserID(ctx context.Context, userID string) error

	// BulkDeleteByTweetAndUser removes retweets for multiple tweet-user pairs.
	BulkDeleteByTweetAndUser(ctx context.Context, pairs []TweetUserPair) error

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetRetweetStats returns aggregated retweet statistics.
	GetRetweetStats(ctx context.Context) (*RetweetStats, error)

	// GetUserRetweetStats returns retweet statistics for a specific user.
	GetUserRetweetStats(ctx context.Context, userID string) (*RetweetStats, error)

	// GetTweetRetweetStats returns retweet statistics for a specific tweet.
	GetTweetRetweetStats(ctx context.Context, tweetID string) (*RetweetStats, error)

	// GetDailyRetweetStats returns daily retweet counts for a date range.
	GetDailyRetweetStats(ctx context.Context, start, end time.Time) ([]*DailyRetweetCount, error)

	// GetDailyRetweetStatsForUser returns daily retweet counts for a user.
	GetDailyRetweetStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyRetweetCount, error)

	// GetRetweetEngagementRate calculates retweet engagement rate for a tweet.
	GetRetweetEngagementRate(ctx context.Context, tweetID string) (float64, error)

	// GetRetweetConversionRate calculates conversion rate from view to retweet.
	GetRetweetConversionRate(ctx context.Context, tweetID string) (float64, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) RetweetRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo RetweetRepository) error) error

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
= Supporting Types
// ======================================================================

// RetweetWithUser represents a retweet with associated user data.
type RetweetWithUser struct {
	Retweet *entities.Retweet `json:"retweet"`
	User    *entities.User    `json:"user"`
}

// RetweeterStats represents statistics for a user who retweets.
type RetweeterStats struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	FullName     string `json:"full_name"`
	AvatarURL    string `json:"avatar_url"`
	RetweetCount int64  `json:"retweet_count"`
}

// TweetUserPair represents a tweet-user pair for bulk delete.
type TweetUserPair struct {
	TweetID string `json:"tweet_id"`
	UserID  string `json:"user_id"`
}

// HourlyRetweetCount represents retweet counts by hour.
type HourlyRetweetCount struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// DayOfWeekRetweetCount represents retweet counts by day of week.
type DayOfWeekRetweetCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// RetweetChain represents a retweet chain.
type RetweetChain struct {
	RootTweetID string   `json:"root_tweet_id"`
	Nodes       []*RetweetNode `json:"nodes"`
}

// RetweetNode represents a node in a retweet chain.
type RetweetNode struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	RetweetedAt time.Time `json:"retweeted_at"`
	Depth       int       `json:"depth"`
}

// ======================================================================
= Helper Functions
// ======================================================================

// IsRetweetNotFound checks if an error indicates a retweet was not found.
func IsRetweetNotFound(err error) bool {
	return errors.Is(err, ErrRetweetNotFound)
}

// IsAlreadyRetweeted checks if an error indicates already retweeted.
func IsAlreadyRetweeted(err error) bool {
	return errors.Is(err, ErrAlreadyRetweeted)
}

// IsRetweetError checks if an error is retweet-related.
func IsRetweetError(err error) bool {
	return errors.Is(err, ErrRetweetNotFound) ||
		errors.Is(err, ErrAlreadyRetweeted) ||
		errors.Is(err, ErrInvalidRetweetID) ||
		errors.Is(err, ErrInvalidTweetID) ||
		errors.Is(err, ErrInvalidUserID) ||
		errors.Is(err, ErrCannotRetweetOwn)
}

// ======================================================================
= Mock Retweet Repository (for testing)
// ======================================================================

// MockRetweetRepository is a mock implementation for testing.
type MockRetweetRepository struct {
	Retweets      map[string]*entities.Retweet
	TweetRetweets map[string]map[string]bool // tweetID -> userID -> retweeted
	UserRetweets  map[string]map[string]bool // userID -> tweetID -> retweeted
	Error         error
	NextCursor    string
}

// NewMockRetweetRepo creates a new mock repository.
func NewMockRetweetRepo() RetweetRepository {
	return &MockRetweetRepository{
		Retweets:      make(map[string]*entities.Retweet),
		TweetRetweets: make(map[string]map[string]bool),
		UserRetweets:  make(map[string]map[string]bool),
	}
}

// Create mock implementation.
func (m *MockRetweetRepository) Create(ctx context.Context, retweet *entities.Retweet) error {
	if m.Error != nil {
		return m.Error
	}
	if m.TweetRetweets[retweet.TweetID] != nil && m.TweetRetweets[retweet.TweetID][retweet.UserID] {
		return ErrAlreadyRetweeted
	}
	m.Retweets[retweet.ID] = retweet
	if m.TweetRetweets[retweet.TweetID] == nil {
		m.TweetRetweets[retweet.TweetID] = make(map[string]bool)
	}
	m.TweetRetweets[retweet.TweetID][retweet.UserID] = true
	if m.UserRetweets[retweet.UserID] == nil {
		m.UserRetweets[retweet.UserID] = make(map[string]bool)
	}
	m.UserRetweets[retweet.UserID][retweet.TweetID] = true
	return nil
}

// GetByID mock implementation.
func (m *MockRetweetRepository) GetByID(ctx context.Context, id string) (*entities.Retweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if retweet, ok := m.Retweets[id]; ok {
		return retweet, nil
	}
	return nil, ErrRetweetNotFound
}

// GetByTweetAndUser mock implementation.
func (m *MockRetweetRepository) GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Retweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	for _, retweet := range m.Retweets {
		if retweet.TweetID == tweetID && retweet.UserID == userID {
			return retweet, nil
		}
	}
	return nil, ErrRetweetNotFound
}

// Delete mock implementation.
func (m *MockRetweetRepository) Delete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if retweet, ok := m.Retweets[id]; ok {
		delete(m.Retweets, id)
		if m.TweetRetweets[retweet.TweetID] != nil {
			delete(m.TweetRetweets[retweet.TweetID], retweet.UserID)
		}
		if m.UserRetweets[retweet.UserID] != nil {
			delete(m.UserRetweets[retweet.UserID], retweet.TweetID)
		}
		return nil
	}
	return ErrRetweetNotFound
}

// DeleteByTweetAndUser mock implementation.
func (m *MockRetweetRepository) DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, retweet := range m.Retweets {
		if retweet.TweetID == tweetID && retweet.UserID == userID {
			delete(m.Retweets, id)
			if m.TweetRetweets[tweetID] != nil {
				delete(m.TweetRetweets[tweetID], userID)
			}
			if m.UserRetweets[userID] != nil {
				delete(m.UserRetweets[userID], tweetID)
			}
			return nil
		}
	}
	return ErrRetweetNotFound
}

// Exists mock implementation.
func (m *MockRetweetRepository) Exists(ctx context.Context, tweetID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.TweetRetweets[tweetID] == nil {
		return false, nil
	}
	return m.TweetRetweets[tweetID][userID], nil
}

// CountByTweetID mock implementation.
func (m *MockRetweetRepository) CountByTweetID(ctx context.Context, tweetID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.TweetRetweets[tweetID] == nil {
		return 0, nil
	}
	return int64(len(m.TweetRetweets[tweetID])), nil
}

// CountByUserID mock implementation.
func (m *MockRetweetRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.UserRetweets[userID] == nil {
		return 0, nil
	}
	return int64(len(m.UserRetweets[userID])), nil
}

// CountByTweetIDs mock implementation.
func (m *MockRetweetRepository) CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, id := range tweetIDs {
		if m.TweetRetweets[id] != nil {
			result[id] = int64(len(m.TweetRetweets[id]))
		} else {
			result[id] = 0
		}
	}
	return result, nil
}

// CountByUserIDs mock implementation.
func (m *MockRetweetRepository) CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, id := range userIDs {
		if m.UserRetweets[id] != nil {
			result[id] = int64(len(m.UserRetweets[id]))
		} else {
			result[id] = 0
		}
	}
	return result, nil
}

// CountByDateRange mock implementation.
func (m *MockRetweetRepository) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, retweet := range m.Retweets {
		if retweet.CreatedAt.After(start) && retweet.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// CountByDateRangeForUser mock implementation.
func (m *MockRetweetRepository) CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, retweet := range m.Retweets {
		if retweet.UserID == userID && retweet.CreatedAt.After(start) && retweet.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// GetByTweetID mock implementation.
func (m *MockRetweetRepository) GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var retweets []*entities.Retweet
	for _, retweet := range m.Retweets {
		if retweet.TweetID == tweetID {
			retweets = append(retweets, retweet)
		}
	}
	return retweets, "", nil
}

// GetByUserID mock implementation.
func (m *MockRetweetRepository) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var retweets []*entities.Retweet
	for _, retweet := range m.Retweets {
		if retweet.UserID == userID {
			retweets = append(retweets, retweet)
		}
	}
	return retweets, "", nil
}

// GetRetweetedTweetIDs mock implementation.
func (m *MockRetweetRepository) GetRetweetedTweetIDs(ctx context.Context, userID string) ([]string, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var ids []string
	for _, retweet := range m.Retweets {
		if retweet.UserID == userID {
			ids = append(ids, retweet.TweetID)
		}
	}
	return ids, nil
}

// GetRetweetedTweets mock implementation.
func (m *MockRetweetRepository) GetRetweetedTweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*entities.Tweet{}, "", nil
}

// GetRetweeters mock implementation.
func (m *MockRetweetRepository) GetRetweeters(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.User, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*entities.User{}, "", nil
}

// GetRetweetersWithTime mock implementation.
func (m *MockRetweetRepository) GetRetweetersWithTime(ctx context.Context, tweetID string, cursor string, limit int) ([]*RetweetWithUser, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*RetweetWithUser{}, "", nil
}

// GetRetweetsTimeline mock implementation.
func (m *MockRetweetRepository) GetRetweetsTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var retweets []*entities.Retweet
	for _, retweet := range m.Retweets {
		for _, uid := range userIDs {
			if retweet.UserID == uid {
				retweets = append(retweets, retweet)
				break
			}
		}
	}
	return retweets, "", nil
}

// GetRecentRetweets mock implementation.
func (m *MockRetweetRepository) GetRecentRetweets(ctx context.Context, userID string, limit int) ([]*entities.Retweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var retweets []*entities.Retweet
	for _, retweet := range m.Retweets {
		if retweet.UserID == userID {
			retweets = append(retweets, retweet)
		}
	}
	return retweets, nil
}

// GetRecentRetweetsForTweets mock implementation.
func (m *MockRetweetRepository) GetRecentRetweetsForTweets(ctx context.Context, tweetIDs []string, limit int) (map[string][]*entities.Retweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string][]*entities.Retweet)
	for _, tid := range tweetIDs {
		var retweets []*entities.Retweet
		for _, retweet := range m.Retweets {
			if retweet.TweetID == tid {
				retweets = append(retweets, retweet)
			}
		}
		result[tid] = retweets
	}
	return result, nil
}

// GetMostRetweetedTweets mock implementation.
func (m *MockRetweetRepository) GetMostRetweetedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Tweet{}, nil
}

// GetMostRetweetedTweetsByCategory mock implementation.
func (m *MockRetweetRepository) GetMostRetweetedTweetsByCategory(ctx context.Context, category string, limit int, since time.Time) ([]*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Tweet{}, nil
}

// GetMostActiveRetweeters mock implementation.
func (m *MockRetweetRepository) GetMostActiveRetweeters(ctx context.Context, limit int, since time.Time) ([]*RetweeterStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*RetweeterStats{}, nil
}

// GetRetweetsByHour mock implementation.
func (m *MockRetweetRepository) GetRetweetsByHour(ctx context.Context, tweetID string) ([]*HourlyRetweetCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*HourlyRetweetCount{}, nil
}

// GetRetweetsByDayOfWeek mock implementation.
func (m *MockRetweetRepository) GetRetweetsByDayOfWeek(ctx context.Context, tweetID string) ([]*DayOfWeekRetweetCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DayOfWeekRetweetCount{}, nil
}

// GetRetweetChains mock implementation.
func (m *MockRetweetRepository) GetRetweetChains(ctx context.Context, tweetID string, maxDepth int) ([]*RetweetChain, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*RetweetChain{}, nil
}

// BulkCreate mock implementation.
func (m *MockRetweetRepository) BulkCreate(ctx context.Context, retweets []*entities.Retweet) error {
	if m.Error != nil {
		return m.Error
	}
	for _, retweet := range retweets {
		if err := m.Create(ctx, retweet); err != nil && !errors.Is(err, ErrAlreadyRetweeted) {
			return err
		}
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockRetweetRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.Delete(ctx, id)
	}
	return nil
}

// BulkDeleteByTweetID mock implementation.
func (m *MockRetweetRepository) BulkDeleteByTweetID(ctx context.Context, tweetID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, retweet := range m.Retweets {
		if retweet.TweetID == tweetID {
			delete(m.Retweets, id)
			if m.TweetRetweets[tweetID] != nil {
				delete(m.TweetRetweets[tweetID], retweet.UserID)
			}
			if m.UserRetweets[retweet.UserID] != nil {
				delete(m.UserRetweets[retweet.UserID], tweetID)
			}
		}
	}
	return nil
}

// BulkDeleteByUserID mock implementation.
func (m *MockRetweetRepository) BulkDeleteByUserID(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, retweet := range m.Retweets {
		if retweet.UserID == userID {
			delete(m.Retweets, id)
			if m.TweetRetweets[retweet.TweetID] != nil {
				delete(m.TweetRetweets[retweet.TweetID], userID)
			}
			if m.UserRetweets[userID] != nil {
				delete(m.UserRetweets[userID], retweet.TweetID)
			}
		}
	}
	return nil
}

// BulkDeleteByTweetAndUser mock implementation.
func (m *MockRetweetRepository) BulkDeleteByTweetAndUser(ctx context.Context, pairs []TweetUserPair) error {
	if m.Error != nil {
		return m.Error
	}
	for _, pair := range pairs {
		_ = m.DeleteByTweetAndUser(ctx, pair.TweetID, pair.UserID)
	}
	return nil
}

// GetRetweetStats mock implementation.
func (m *MockRetweetRepository) GetRetweetStats(ctx context.Context) (*RetweetStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &RetweetStats{
		TotalRetweets: int64(len(m.Retweets)),
	}, nil
}

// GetUserRetweetStats mock implementation.
func (m *MockRetweetRepository) GetUserRetweetStats(ctx context.Context, userID string) (*RetweetStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	count := int64(0)
	for _, retweet := range m.Retweets {
		if retweet.UserID == userID {
			count++
		}
	}
	return &RetweetStats{TotalRetweets: count}, nil
}

// GetTweetRetweetStats mock implementation.
func (m *MockRetweetRepository) GetTweetRetweetStats(ctx context.Context, tweetID string) (*RetweetStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	count := int64(0)
	for _, retweet := range m.Retweets {
		if retweet.TweetID == tweetID {
			count++
		}
	}
	return &RetweetStats{TotalRetweets: count}, nil
}

// GetDailyRetweetStats mock implementation.
func (m *MockRetweetRepository) GetDailyRetweetStats(ctx context.Context, start, end time.Time) ([]*DailyRetweetCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyRetweetCount{}, nil
}

// GetDailyRetweetStatsForUser mock implementation.
func (m *MockRetweetRepository) GetDailyRetweetStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyRetweetCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyRetweetCount{}, nil
}

// GetRetweetEngagementRate mock implementation.
func (m *MockRetweetRepository) GetRetweetEngagementRate(ctx context.Context, tweetID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// GetRetweetConversionRate mock implementation.
func (m *MockRetweetRepository) GetRetweetConversionRate(ctx context.Context, tweetID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// WithTransaction mock implementation.
func (m *MockRetweetRepository) WithTransaction(ctx context.Context, tx *sql.Tx) RetweetRepository {
	return m
}

// Transaction mock implementation.
func (m *MockRetweetRepository) Transaction(ctx context.Context, fn func(txRepo RetweetRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockRetweetRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockRetweetRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockRetweetRepository) GetRawDB() interface{} {
	return nil
}