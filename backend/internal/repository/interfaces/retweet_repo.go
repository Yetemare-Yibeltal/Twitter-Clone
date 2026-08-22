// backend/internal/repository/interfaces/tweet_repo.go
package interfaces

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
)

// ======================================================================
= Common Errors
// ======================================================================

var (
	ErrTweetNotFound      = errors.New("tweet not found")
	ErrTweetDeleted       = errors.New("tweet has been deleted")
	ErrAlreadyLiked       = errors.New("already liked this tweet")
	ErrAlreadyRetweeted   = errors.New("already retweeted this tweet")
	ErrAlreadyBookmarked  = errors.New("already bookmarked this tweet")
	ErrLikeNotFound       = errors.New("like not found")
	ErrRetweetNotFound    = errors.New("retweet not found")
	ErrBookmarkNotFound   = errors.New("bookmark not found")
	ErrPollNotFound       = errors.New("poll not found")
	ErrPollExpired        = errors.New("poll has expired")
	ErrPollAlreadyVoted   = errors.New("already voted on this poll")
	ErrInvalidPollOption  = errors.New("invalid poll option")
	ErrInvalidContent     = errors.New("invalid tweet content")
	ErrContentTooLong     = errors.New("tweet content exceeds maximum length")
	ErrEmptyContent       = errors.New("tweet content cannot be empty")
)

// ======================================================================
= TweetRepository Interface
// ======================================================================

// TweetRepository defines the interface for tweet data persistence.
type TweetRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create inserts a new tweet.
	Create(ctx context.Context, tweet *entities.Tweet) error

	// GetByID retrieves a tweet by its primary key, excluding soft-deleted.
	GetByID(ctx context.Context, id string) (*entities.Tweet, error)

	// Update updates a tweet's content and media (owner only).
	Update(ctx context.Context, tweet *entities.Tweet) error

	// SoftDelete marks a tweet as deleted (sets deleted_at).
	SoftDelete(ctx context.Context, id string) error

	// HardDelete permanently removes a tweet.
	HardDelete(ctx context.Context, id string) error

	// --------------------------------------------------------------------
	// Feed & List Queries
	// --------------------------------------------------------------------

	// GetFeed returns tweets from a list of user IDs, ordered by created_at DESC,
	// with cursor-based pagination. Excludes deleted tweets.
	GetFeed(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// GetByUserID returns tweets by a specific user, optionally including replies.
	GetByUserID(ctx context.Context, userID, cursor string, limit int, includeReplies bool) ([]*entities.Tweet, string, error)

	// GetReplies returns replies to a specific tweet, ordered by created_at ASC (oldest first).
	GetReplies(ctx context.Context, tweetID, cursor string, limit int) ([]*entities.Tweet, string, error)

	// CountReplies returns the total number of replies to a tweet.
	CountReplies(ctx context.Context, tweetID string) (int64, error)

	// --------------------------------------------------------------------
	// Search
	// --------------------------------------------------------------------

	// Search performs full-text search on tweet content and returns matching tweets.
	Search(ctx context.Context, query string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// --------------------------------------------------------------------
	// Trending
	// --------------------------------------------------------------------

	// GetTrending returns trending topics (hashtags) based on tweet frequency over the last 24h.
	GetTrending(ctx context.Context, limit int) ([]*dto.TrendingTopic, error)

	// --------------------------------------------------------------------
	// Interaction Counts
	// --------------------------------------------------------------------

	// GetLikeCount returns the number of likes for a tweet.
	GetLikeCount(ctx context.Context, tweetID string) (int64, error)

	// GetRetweetCount returns the number of retweets for a tweet.
	GetRetweetCount(ctx context.Context, tweetID string) (int64, error)

	// --------------------------------------------------------------------
	// Likes
	// --------------------------------------------------------------------

	// Like adds a like from a user to a tweet. Returns ErrAlreadyLiked if already liked.
	Like(ctx context.Context, tweetID, userID string) error

	// Unlike removes a like. Returns ErrLikeNotFound if like doesn't exist.
	Unlike(ctx context.Context, tweetID, userID string) error

	// IsLiked checks if a user has liked a tweet.
	IsLiked(ctx context.Context, tweetID, userID string) (bool, error)

	// --------------------------------------------------------------------
	// Retweets
	// --------------------------------------------------------------------

	// Retweet adds a retweet. Returns ErrAlreadyRetweeted if already retweeted.
	Retweet(ctx context.Context, tweetID, userID string) error

	// Unretweet removes a retweet. Returns ErrRetweetNotFound if retweet doesn't exist.
	Unretweet(ctx context.Context, tweetID, userID string) error

	// IsRetweeted checks if a user has retweeted a tweet.
	IsRetweeted(ctx context.Context, tweetID, userID string) (bool, error)

	// --------------------------------------------------------------------
	// Bookmarks
	// --------------------------------------------------------------------

	// Bookmark adds a bookmark. Returns ErrAlreadyBookmarked if already bookmarked.
	Bookmark(ctx context.Context, tweetID, userID string) error

	// Unbookmark removes a bookmark. Returns ErrBookmarkNotFound if bookmark doesn't exist.
	Unbookmark(ctx context.Context, tweetID, userID string) error

	// IsBookmarked checks if a user has bookmarked a tweet.
	IsBookmarked(ctx context.Context, tweetID, userID string) (bool, error)

	// GetBookmarksByUser returns tweets bookmarked by a user, with pagination.
	GetBookmarksByUser(ctx context.Context, userID, cursor string, limit int) ([]*entities.Tweet, string, error)

	// --------------------------------------------------------------------
	// Polls
	// --------------------------------------------------------------------

	// CreatePoll creates a new poll associated with a tweet.
	CreatePoll(ctx context.Context, poll *entities.Poll) error

	// GetPollByTweetID retrieves a poll by its tweet ID.
	GetPollByTweetID(ctx context.Context, tweetID string) (*entities.Poll, error)

	// VotePoll adds a vote from a user to a poll option. Returns ErrPollExpired, ErrPollAlreadyVoted, or ErrInvalidPollOption.
	VotePoll(ctx context.Context, pollID, userID, optionID string) error

	// GetPollResults returns the poll with up‑to‑date vote counts.
	GetPollResults(ctx context.Context, pollID string) (*entities.Poll, error)

	// --------------------------------------------------------------------
	// Advanced / Bulk Operations
	// --------------------------------------------------------------------

	// DeleteAllByUserID soft-deletes all tweets by a user (for account deletion).
	DeleteAllByUserID(ctx context.Context, userID string) error

	// GetTweetsByIDs retrieves multiple tweets by their IDs in bulk.
	GetTweetsByIDs(ctx context.Context, ids []string) ([]*entities.Tweet, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository instance using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) TweetRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo TweetRepository) error) error

	// --------------------------------------------------------------------
	// Health and Cleanup
	// --------------------------------------------------------------------

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close releases any resources (pool connections).
	Close() error

	// GetRawDB returns the underlying *sql.DB or *sqlx.DB for advanced use (optional).
	GetRawDB() interface{}
}

// ======================================================================
= Additional Interfaces (optional extensions)
// ======================================================================

// TweetRepositorySearch provides advanced search capabilities.
type TweetRepositorySearch interface {
	TweetRepository
	// FullTextSearch performs full-text search with custom ranking.
	FullTextSearch(ctx context.Context, query string, minRank float64, cursor string, limit int) ([]*entities.Tweet, string, error)
}

// TweetRepositoryAnalytics provides analytics methods.
type TweetRepositoryAnalytics interface {
	TweetRepository
	// GetTweetStats returns aggregated stats for a tweet.
	GetTweetStats(ctx context.Context, tweetID string) (*TweetStats, error)
	// GetDailyTweetCount returns tweet count for a user per day.
	GetDailyTweetCount(ctx context.Context, userID string, days int) ([]*DailyCount, error)
}

// TweetStats represents aggregated tweet stats.
type TweetStats struct {
	TweetID      string    `json:"tweet_id"`
	LikeCount    int64     `json:"like_count"`
	RetweetCount int64     `json:"retweet_count"`
	ReplyCount   int64     `json:"reply_count"`
	BookmarkCount int64    `json:"bookmark_count"`
	ViewCount    int64     `json:"view_count"`
	LastUpdated  time.Time `json:"last_updated"`
}

// DailyCount represents a daily count.
type DailyCount struct {
	Date  time.Time `json:"date"`
	Count int64     `json:"count"`
}

// ======================================================================
= Helper Types and Constants
// ======================================================================

// TweetSortField defines sortable fields for tweets.
type TweetSortField string

const (
	SortByCreatedAt TweetSortField = "created_at"
	SortByUpdatedAt TweetSortField = "updated_at"
	SortByLikeCount TweetSortField = "like_count"
	SortByRetweetCount TweetSortField = "retweet_count"
	SortByReplyCount TweetSortField = "reply_count"
)

// TweetSortOrder defines sort order.
type TweetSortOrder string

const (
	SortAsc  TweetSortOrder = "ASC"
	SortDesc TweetSortOrder = "DESC"
)

// PaginationOptions for tweets.
type TweetPagination struct {
	Cursor   string          `json:"cursor"`
	Limit    int             `json:"limit"`
	SortBy   TweetSortField  `json:"sort_by"`
	Order    TweetSortOrder  `json:"order"`
}

// DefaultTweetPagination returns default pagination options.
func DefaultTweetPagination() *TweetPagination {
	return &TweetPagination{
		Limit: 20,
		SortBy: SortByCreatedAt,
		Order:  SortDesc,
	}
}

// Validate checks pagination parameters.
func (p *TweetPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return ErrInvalidLimit
	}
	return nil
}

// ======================================================================
= Error Helpers
// ======================================================================

// IsTweetNotFound checks if an error indicates a tweet was not found.
func IsTweetNotFound(err error) bool {
	return errors.Is(err, ErrTweetNotFound) || errors.Is(err, ErrTweetDeleted)
}

// IsAlreadyLiked checks if an error indicates already liked.
func IsAlreadyLiked(err error) bool {
	return errors.Is(err, ErrAlreadyLiked)
}

// IsAlreadyRetweeted checks if an error indicates already retweeted.
func IsAlreadyRetweeted(err error) bool {
	return errors.Is(err, ErrAlreadyRetweeted)
}

// IsAlreadyBookmarked checks if an error indicates already bookmarked.
func IsAlreadyBookmarked(err error) bool {
	return errors.Is(err, ErrAlreadyBookmarked)
}

// IsPollError checks if an error is poll-related.
func IsPollError(err error) bool {
	return errors.Is(err, ErrPollExpired) ||
		errors.Is(err, ErrPollAlreadyVoted) ||
		errors.Is(err, ErrInvalidPollOption) ||
		errors.Is(err, ErrPollNotFound)
}

// ======================================================================
= Mock Repository (for testing)
// ======================================================================

// MockTweetRepository is a mock implementation for testing.
type MockTweetRepository struct {
	Tweets map[string]*entities.Tweet
	Likes  map[string]map[string]bool // tweetID -> userID -> liked
	Retweets map[string]map[string]bool // tweetID -> userID -> retweeted
	Bookmarks map[string]map[string]bool // tweetID -> userID -> bookmarked
	Polls  map[string]*entities.Poll
	Error  error
}

// NewMockTweetRepo creates a new mock repository.
func NewMockTweetRepo() TweetRepository {
	return &MockTweetRepository{
		Tweets:    make(map[string]*entities.Tweet),
		Likes:     make(map[string]map[string]bool),
		Retweets:  make(map[string]map[string]bool),
		Bookmarks: make(map[string]map[string]bool),
		Polls:     make(map[string]*entities.Poll),
	}
}

// (Mock methods would be implemented here, but are omitted for brevity)

// ======================================================================
= Repository Factory
// ======================================================================

// TweetRepositoryFactory is a function type for creating a repository with a specific DB connection.
type TweetRepositoryFactory func(db interface{}) TweetRepository