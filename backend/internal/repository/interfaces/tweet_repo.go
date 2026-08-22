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
// Common Errors
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
	ErrInvalidTweetID     = errors.New("invalid tweet ID")
	ErrInvalidUserID      = errors.New("invalid user ID")
	ErrMaxMediaExceeded   = errors.New("maximum media count exceeded")
)

// ======================================================================
// TweetFilter
// ======================================================================

// TweetFilter defines filtering options for tweets.
type TweetFilter struct {
	UserID         *string
	ParentTweetID  *string
	RetweetOfID    *string
	IsPoll         *bool
	HasMedia       *bool
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	MinLikes       *int64
	MinRetweets    *int64
	MinReplies     *int64
	Search         *string // full-text search on content
	Hashtags       []string
	Mentions       []string
}

// HasCriteria checks if any filter criteria are set.
func (f *TweetFilter) HasCriteria() bool {
	return f.UserID != nil || f.ParentTweetID != nil || f.RetweetOfID != nil ||
		f.IsPoll != nil || f.HasMedia != nil || f.CreatedFrom != nil ||
		f.CreatedTo != nil || f.MinLikes != nil || f.MinRetweets != nil ||
		f.MinReplies != nil || f.Search != nil || len(f.Hashtags) > 0 ||
		len(f.Mentions) > 0
}

// ======================================================================
= TweetPagination
// ======================================================================

// TweetSortField defines sortable fields for tweets.
type TweetSortField string

const (
	SortByCreatedAt    TweetSortField = "created_at"
	SortByUpdatedAt    TweetSortField = "updated_at"
	SortByLikeCount    TweetSortField = "like_count"
	SortByRetweetCount TweetSortField = "retweet_count"
	SortByReplyCount   TweetSortField = "reply_count"
	SortByRelevance    TweetSortField = "relevance"
)

// TweetSortOrder defines sort order.
type TweetSortOrder string

const (
	SortAsc  TweetSortOrder = "ASC"
	SortDesc TweetSortOrder = "DESC"
)

// TweetPagination holds pagination options for tweets.
type TweetPagination struct {
	Cursor   string          `json:"cursor"`
	Limit    int             `json:"limit"`
	SortBy   TweetSortField  `json:"sort_by"`
	Order    TweetSortOrder  `json:"order"`
}

// DefaultTweetPagination returns default pagination options.
func DefaultTweetPagination() *TweetPagination {
	return &TweetPagination{
		Limit:  20,
		SortBy: SortByCreatedAt,
		Order:  SortDesc,
	}
}

// Validate checks pagination parameters.
func (p *TweetPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
= TweetStats
// ======================================================================

// TweetStats represents aggregated tweet statistics.
type TweetStats struct {
	TotalTweets    int64     `json:"total_tweets"`
	TotalLikes     int64     `json:"total_likes"`
	TotalRetweets  int64     `json:"total_retweets"`
	TotalReplies   int64     `json:"total_replies"`
	AverageLikes   float64   `json:"average_likes"`
	AverageRetweets float64  `json:"average_retweets"`
	MostLikedTweet string    `json:"most_liked_tweet"`
	MostRetweetedTweet string `json:"most_retweeted_tweet"`
	LastTweetAt    time.Time `json:"last_tweet_at"`
	FirstTweetAt   time.Time `json:"first_tweet_at"`
}

// ======================================================================
= DailyTweetCount
// ======================================================================

// DailyTweetCount represents daily tweet counts.
type DailyTweetCount struct {
	Date   time.Time `json:"date"`
	Count  int64     `json:"count"`
	Likes  int64     `json:"likes"`
	Retweets int64   `json:"retweets"`
	Replies int64    `json:"replies"`
}

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

	// GetByIDs retrieves multiple tweets by their IDs.
	GetByIDs(ctx context.Context, ids []string) ([]*entities.Tweet, error)

	// Update updates a tweet's content and media (owner only).
	Update(ctx context.Context, tweet *entities.Tweet) error

	// SoftDelete marks a tweet as deleted (sets deleted_at).
	SoftDelete(ctx context.Context, id string) error

	// HardDelete permanently removes a tweet.
	HardDelete(ctx context.Context, id string) error

	// Restore restores a soft-deleted tweet.
	Restore(ctx context.Context, id string) error

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

	// GetByParentID returns tweets by parent ID (direct replies).
	GetByParentID(ctx context.Context, parentID string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// GetByRetweetOfID returns retweets of a specific tweet.
	GetByRetweetOfID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// CountByUserID returns the total number of tweets by a user.
	CountByUserID(ctx context.Context, userID string) (int64, error)

	// CountReplies returns the total number of replies to a tweet.
	CountReplies(ctx context.Context, tweetID string) (int64, error)

	// CountRetweets returns the total number of retweets of a tweet.
	CountRetweets(ctx context.Context, tweetID string) (int64, error)

	// --------------------------------------------------------------------
	// Search
	// --------------------------------------------------------------------

	// Search performs full-text search on tweet content and returns matching tweets.
	Search(ctx context.Context, query string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// SearchWithFilters performs full-text search with additional filters.
	SearchWithFilters(ctx context.Context, query string, filter *TweetFilter, pagination *TweetPagination) ([]*entities.Tweet, int64, error)

	// --------------------------------------------------------------------
	// Trending
	// --------------------------------------------------------------------

	// GetTrending returns trending topics (hashtags) based on tweet frequency.
	GetTrending(ctx context.Context, limit int) ([]*dto.TrendingTopic, error)

	// GetTrendingWithTimeRange returns trending topics within a time range.
	GetTrendingWithTimeRange(ctx context.Context, since time.Time, limit int) ([]*dto.TrendingTopic, error)

	// --------------------------------------------------------------------
	// Interaction Counts
	// --------------------------------------------------------------------

	// GetLikeCount returns the number of likes for a tweet.
	GetLikeCount(ctx context.Context, tweetID string) (int64, error)

	// GetRetweetCount returns the number of retweets for a tweet.
	GetRetweetCount(ctx context.Context, tweetID string) (int64, error)

	// GetReplyCount returns the number of replies for a tweet.
	GetReplyCount(ctx context.Context, tweetID string) (int64, error)

	// GetBookmarkCount returns the number of bookmarks for a tweet.
	GetBookmarkCount(ctx context.Context, tweetID string) (int64, error)

	// GetInteractionCounts returns all interaction counts for a tweet.
	GetInteractionCounts(ctx context.Context, tweetID string) (*InteractionCounts, error)

	// --------------------------------------------------------------------
	// Likes
	// --------------------------------------------------------------------

	// Like adds a like from a user to a tweet. Returns ErrAlreadyLiked if already liked.
	Like(ctx context.Context, tweetID, userID string) error

	// Unlike removes a like. Returns ErrLikeNotFound if like doesn't exist.
	Unlike(ctx context.Context, tweetID, userID string) error

	// IsLiked checks if a user has liked a tweet.
	IsLiked(ctx context.Context, tweetID, userID string) (bool, error)

	// GetLikesByTweetID returns users who liked a tweet.
	GetLikesByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Like, string, error)

	// GetLikesByUserID returns tweets liked by a user.
	GetLikesByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Like, string, error)

	// --------------------------------------------------------------------
	// Retweets
	// --------------------------------------------------------------------

	// Retweet adds a retweet. Returns ErrAlreadyRetweeted if already retweeted.
	Retweet(ctx context.Context, tweetID, userID string) error

	// Unretweet removes a retweet. Returns ErrRetweetNotFound if retweet doesn't exist.
	Unretweet(ctx context.Context, tweetID, userID string) error

	// IsRetweeted checks if a user has retweeted a tweet.
	IsRetweeted(ctx context.Context, tweetID, userID string) (bool, error)

	// GetRetweetsByTweetID returns users who retweeted a tweet.
	GetRetweetsByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Retweet, string, error)

	// GetRetweetsByUserID returns tweets retweeted by a user.
	GetRetweetsByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Retweet, string, error)

	// --------------------------------------------------------------------
	// Bookmarks
	// --------------------------------------------------------------------

	// Bookmark adds a bookmark. Returns ErrAlreadyBookmarked if already bookmarked.
	Bookmark(ctx context.Context, tweetID, userID string) error

	// Unbookmark removes a bookmark. Returns ErrBookmarkNotFound if bookmark doesn't exist.
	Unbookmark(ctx context.Context, tweetID, userID string) error

	// IsBookmarked checks if a user has bookmarked a tweet.
	IsBookmarked(ctx context.Context, tweetID, userID string) (bool, error)

	// GetBookmarksByUserID returns tweets bookmarked by a user.
	GetBookmarksByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Bookmark, string, error)

	// --------------------------------------------------------------------
	// Polls
	// --------------------------------------------------------------------

	// CreatePoll creates a new poll associated with a tweet.
	CreatePoll(ctx context.Context, poll *entities.Poll) error

	// GetPollByTweetID retrieves a poll by its tweet ID.
	GetPollByTweetID(ctx context.Context, tweetID string) (*entities.Poll, error)

	// GetPollByID retrieves a poll by its ID.
	GetPollByID(ctx context.Context, pollID string) (*entities.Poll, error)

	// VotePoll adds a vote from a user to a poll option.
	VotePoll(ctx context.Context, pollID, userID, optionID string) error

	// UnvotePoll removes a vote from a poll option.
	UnvotePoll(ctx context.Context, pollID, userID, optionID string) error

	// GetPollResults returns the poll with up‑to‑date vote counts.
	GetPollResults(ctx context.Context, pollID string) (*entities.Poll, error)

	// GetUserVote returns the option ID a user voted for.
	GetUserVote(ctx context.Context, pollID, userID string) (string, error)

	// HasUserVoted checks if a user has voted on a poll.
	HasUserVoted(ctx context.Context, pollID, userID string) (bool, error)

	// GetExpiredPolls returns polls that have expired.
	GetExpiredPolls(ctx context.Context, before time.Time, limit int) ([]*entities.Poll, error)

	// --------------------------------------------------------------------
	// Advanced / Bulk Operations
	// --------------------------------------------------------------------

	// DeleteAllByUserID soft-deletes all tweets by a user (for account deletion).
	DeleteAllByUserID(ctx context.Context, userID string) error

	// BulkCreate inserts multiple tweets in a single transaction.
	BulkCreate(ctx context.Context, tweets []*entities.Tweet) error

	// BulkSoftDelete soft-deletes multiple tweets.
	BulkSoftDelete(ctx context.Context, ids []string) error

	// BulkHardDelete hard-deletes multiple tweets.
	BulkHardDelete(ctx context.Context, ids []string) error

	// GetTweetsByDateRange returns tweets within a date range.
	GetTweetsByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Tweet, string, error)

	// GetMostLikedTweets returns the most liked tweets.
	GetMostLikedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error)

	// GetMostRetweetedTweets returns the most retweeted tweets.
	GetMostRetweetedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error)

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetTweetStats returns aggregated tweet statistics.
	GetTweetStats(ctx context.Context) (*TweetStats, error)

	// GetDailyTweetStats returns daily tweet counts for a date range.
	GetDailyTweetStats(ctx context.Context, start, end time.Time) ([]*DailyTweetCount, error)

	// GetUserTweetStats returns tweet statistics for a specific user.
	GetUserTweetStats(ctx context.Context, userID string) (*TweetStats, error)

	// GetTweetEngagementRate calculates engagement rate for a tweet.
	GetTweetEngagementRate(ctx context.Context, tweetID string) (float64, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
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
= InteractionCounts
// ======================================================================

// InteractionCounts represents all interaction counts for a tweet.
type InteractionCounts struct {
	Likes    int64 `json:"likes"`
	Retweets int64 `json:"retweets"`
	Replies  int64 `json:"replies"`
	Bookmarks int64 `json:"bookmarks"`
	Views    int64 `json:"views"`
}

// ======================================================================
= Helper Functions
// ======================================================================

// IsTweetNotFound checks if an error indicates a tweet was not found.
func IsTweetNotFound(err error) bool {
	return errors.Is(err, ErrTweetNotFound) || errors.Is(err, ErrTweetDeleted)
}

// IsInteractionError checks if an error is interaction-related (like, retweet, bookmark).
func IsInteractionError(err error) bool {
	return errors.Is(err, ErrAlreadyLiked) || errors.Is(err, ErrAlreadyRetweeted) ||
		errors.Is(err, ErrAlreadyBookmarked) || errors.Is(err, ErrLikeNotFound) ||
		errors.Is(err, ErrRetweetNotFound) || errors.Is(err, ErrBookmarkNotFound)
}

// IsPollError checks if an error is poll-related.
func IsPollError(err error) bool {
	return errors.Is(err, ErrPollNotFound) || errors.Is(err, ErrPollExpired) ||
		errors.Is(err, ErrPollAlreadyVoted) || errors.Is(err, ErrInvalidPollOption)
}

// ======================================================================
= Mock Tweet Repository (for testing)
// ======================================================================

// MockTweetRepository is a mock implementation for testing.
type MockTweetRepository struct {
	Tweets     map[string]*entities.Tweet
	Likes      map[string]map[string]bool
	Retweets   map[string]map[string]bool
	Bookmarks  map[string]map[string]bool
	Polls      map[string]*entities.Poll
	Error      error
	NextCursor string
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

// Create mock implementation.
func (m *MockTweetRepository) Create(ctx context.Context, tweet *entities.Tweet) error {
	if m.Error != nil {
		return m.Error
	}
	m.Tweets[tweet.ID] = tweet
	return nil
}

// GetByID mock implementation.
func (m *MockTweetRepository) GetByID(ctx context.Context, id string) (*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if tweet, ok := m.Tweets[id]; ok && tweet.DeletedAt == nil {
		return tweet, nil
	}
	return nil, ErrTweetNotFound
}

// GetByIDs mock implementation.
func (m *MockTweetRepository) GetByIDs(ctx context.Context, ids []string) ([]*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var tweets []*entities.Tweet
	for _, id := range ids {
		if tweet, ok := m.Tweets[id]; ok && tweet.DeletedAt == nil {
			tweets = append(tweets, tweet)
		}
	}
	return tweets, nil
}

// Update mock implementation.
func (m *MockTweetRepository) Update(ctx context.Context, tweet *entities.Tweet) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Tweets[tweet.ID]; !ok {
		return ErrTweetNotFound
	}
	m.Tweets[tweet.ID] = tweet
	return nil
}

// SoftDelete mock implementation.
func (m *MockTweetRepository) SoftDelete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if tweet, ok := m.Tweets[id]; ok {
		now := time.Now()
		tweet.DeletedAt = &now
		return nil
	}
	return ErrTweetNotFound
}

// HardDelete mock implementation.
func (m *MockTweetRepository) HardDelete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Tweets[id]; ok {
		delete(m.Tweets, id)
		return nil
	}
	return ErrTweetNotFound
}

// Restore mock implementation.
func (m *MockTweetRepository) Restore(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if tweet, ok := m.Tweets[id]; ok {
		tweet.DeletedAt = nil
		return nil
	}
	return ErrTweetNotFound
}

// GetFeed mock implementation.
func (m *MockTweetRepository) GetFeed(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var tweets []*entities.Tweet
	for _, tweet := range m.Tweets {
		if tweet.DeletedAt == nil {
			for _, uid := range userIDs {
				if tweet.UserID == uid {
					tweets = append(tweets, tweet)
					break
				}
			}
		}
	}
	return tweets, "", nil
}

// GetByUserID mock implementation.
func (m *MockTweetRepository) GetByUserID(ctx context.Context, userID, cursor string, limit int, includeReplies bool) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var tweets []*entities.Tweet
	for _, tweet := range m.Tweets {
		if tweet.UserID == userID && tweet.DeletedAt == nil {
			if !includeReplies && tweet.ParentTweetID != nil && *tweet.ParentTweetID != "" {
				continue
			}
			tweets = append(tweets, tweet)
		}
	}
	return tweets, "", nil
}

// GetReplies mock implementation.
func (m *MockTweetRepository) GetReplies(ctx context.Context, tweetID, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var tweets []*entities.Tweet
	for _, tweet := range m.Tweets {
		if tweet.ParentTweetID != nil && *tweet.ParentTweetID == tweetID && tweet.DeletedAt == nil {
			tweets = append(tweets, tweet)
		}
	}
	return tweets, "", nil
}

// GetByParentID mock implementation.
func (m *MockTweetRepository) GetByParentID(ctx context.Context, parentID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	return m.GetReplies(ctx, parentID, cursor, limit)
}

// GetByRetweetOfID mock implementation.
func (m *MockTweetRepository) GetByRetweetOfID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var tweets []*entities.Tweet
	for _, tweet := range m.Tweets {
		if tweet.RetweetOfID != nil && *tweet.RetweetOfID == tweetID && tweet.DeletedAt == nil {
			tweets = append(tweets, tweet)
		}
	}
	return tweets, "", nil
}

// CountByUserID mock implementation.
func (m *MockTweetRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, tweet := range m.Tweets {
		if tweet.UserID == userID && tweet.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

// CountReplies mock implementation.
func (m *MockTweetRepository) CountReplies(ctx context.Context, tweetID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, tweet := range m.Tweets {
		if tweet.ParentTweetID != nil && *tweet.ParentTweetID == tweetID && tweet.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

// CountRetweets mock implementation.
func (m *MockTweetRepository) CountRetweets(ctx context.Context, tweetID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, tweet := range m.Tweets {
		if tweet.RetweetOfID != nil && *tweet.RetweetOfID == tweetID && tweet.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

// Search mock implementation.
func (m *MockTweetRepository) Search(ctx context.Context, query string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var tweets []*entities.Tweet
	for _, tweet := range m.Tweets {
		if tweet.DeletedAt == nil && containsIgnoreCase(tweet.Content, query) {
			tweets = append(tweets, tweet)
		}
	}
	return tweets, "", nil
}

// SearchWithFilters mock implementation.
func (m *MockTweetRepository) SearchWithFilters(ctx context.Context, query string, filter *TweetFilter, pagination *TweetPagination) ([]*entities.Tweet, int64, error) {
	return m.Search(ctx, query, "", 20)
}

// GetTrending mock implementation.
func (m *MockTweetRepository) GetTrending(ctx context.Context, limit int) ([]*dto.TrendingTopic, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*dto.TrendingTopic{}, nil
}

// GetTrendingWithTimeRange mock implementation.
func (m *MockTweetRepository) GetTrendingWithTimeRange(ctx context.Context, since time.Time, limit int) ([]*dto.TrendingTopic, error) {
	return m.GetTrending(ctx, limit)
}

// GetLikeCount mock implementation.
func (m *MockTweetRepository) GetLikeCount(ctx context.Context, tweetID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if likes, ok := m.Likes[tweetID]; ok {
		return int64(len(likes)), nil
	}
	return 0, nil
}

// GetRetweetCount mock implementation.
func (m *MockTweetRepository) GetRetweetCount(ctx context.Context, tweetID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if retweets, ok := m.Retweets[tweetID]; ok {
		return int64(len(retweets)), nil
	}
	return 0, nil
}

// GetReplyCount mock implementation.
func (m *MockTweetRepository) GetReplyCount(ctx context.Context, tweetID string) (int64, error) {
	return m.CountReplies(ctx, tweetID)
}

// GetBookmarkCount mock implementation.
func (m *MockTweetRepository) GetBookmarkCount(ctx context.Context, tweetID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if bookmarks, ok := m.Bookmarks[tweetID]; ok {
		return int64(len(bookmarks)), nil
	}
	return 0, nil
}

// GetInteractionCounts mock implementation.
func (m *MockTweetRepository) GetInteractionCounts(ctx context.Context, tweetID string) (*InteractionCounts, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	likes, _ := m.GetLikeCount(ctx, tweetID)
	retweets, _ := m.GetRetweetCount(ctx, tweetID)
	replies, _ := m.GetReplyCount(ctx, tweetID)
	bookmarks, _ := m.GetBookmarkCount(ctx, tweetID)
	return &InteractionCounts{
		Likes:    likes,
		Retweets: retweets,
		Replies:  replies,
		Bookmarks: bookmarks,
		Views:    0,
	}, nil
}

// Like mock implementation.
func (m *MockTweetRepository) Like(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Tweets[tweetID]; !ok {
		return ErrTweetNotFound
	}
	if m.Likes[tweetID] == nil {
		m.Likes[tweetID] = make(map[string]bool)
	}
	if m.Likes[tweetID][userID] {
		return ErrAlreadyLiked
	}
	m.Likes[tweetID][userID] = true
	return nil
}

// Unlike mock implementation.
func (m *MockTweetRepository) Unlike(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if m.Likes[tweetID] == nil || !m.Likes[tweetID][userID] {
		return ErrLikeNotFound
	}
	delete(m.Likes[tweetID], userID)
	return nil
}

// IsLiked mock implementation.
func (m *MockTweetRepository) IsLiked(ctx context.Context, tweetID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Likes[tweetID] == nil {
		return false, nil
	}
	return m.Likes[tweetID][userID], nil
}

// GetLikesByTweetID mock implementation.
func (m *MockTweetRepository) GetLikesByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Like, string, error) {
	return []*entities.Like{}, "", nil
}

// GetLikesByUserID mock implementation.
func (m *MockTweetRepository) GetLikesByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Like, string, error) {
	return []*entities.Like{}, "", nil
}

// Retweet mock implementation.
func (m *MockTweetRepository) Retweet(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Tweets[tweetID]; !ok {
		return ErrTweetNotFound
	}
	if m.Retweets[tweetID] == nil {
		m.Retweets[tweetID] = make(map[string]bool)
	}
	if m.Retweets[tweetID][userID] {
		return ErrAlreadyRetweeted
	}
	m.Retweets[tweetID][userID] = true
	return nil
}

// Unretweet mock implementation.
func (m *MockTweetRepository) Unretweet(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if m.Retweets[tweetID] == nil || !m.Retweets[tweetID][userID] {
		return ErrRetweetNotFound
	}
	delete(m.Retweets[tweetID], userID)
	return nil
}

// IsRetweeted mock implementation.
func (m *MockTweetRepository) IsRetweeted(ctx context.Context, tweetID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Retweets[tweetID] == nil {
		return false, nil
	}
	return m.Retweets[tweetID][userID], nil
}

// GetRetweetsByTweetID mock implementation.
func (m *MockTweetRepository) GetRetweetsByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	return []*entities.Retweet{}, "", nil
}

// GetRetweetsByUserID mock implementation.
func (m *MockTweetRepository) GetRetweetsByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	return []*entities.Retweet{}, "", nil
}

// Bookmark mock implementation.
func (m *MockTweetRepository) Bookmark(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Tweets[tweetID]; !ok {
		return ErrTweetNotFound
	}
	if m.Bookmarks[tweetID] == nil {
		m.Bookmarks[tweetID] = make(map[string]bool)
	}
	if m.Bookmarks[tweetID][userID] {
		return ErrAlreadyBookmarked
	}
	m.Bookmarks[tweetID][userID] = true
	return nil
}

// Unbookmark mock implementation.
func (m *MockTweetRepository) Unbookmark(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if m.Bookmarks[tweetID] == nil || !m.Bookmarks[tweetID][userID] {
		return ErrBookmarkNotFound
	}
	delete(m.Bookmarks[tweetID], userID)
	return nil
}

// IsBookmarked mock implementation.
func (m *MockTweetRepository) IsBookmarked(ctx context.Context, tweetID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Bookmarks[tweetID] == nil {
		return false, nil
	}
	return m.Bookmarks[tweetID][userID], nil
}

// GetBookmarksByUserID mock implementation.
func (m *MockTweetRepository) GetBookmarksByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	return []*entities.Bookmark{}, "", nil
}

// CreatePoll mock implementation.
func (m *MockTweetRepository) CreatePoll(ctx context.Context, poll *entities.Poll) error {
	if m.Error != nil {
		return m.Error
	}
	m.Polls[poll.ID] = poll
	return nil
}

// GetPollByTweetID mock implementation.
func (m *MockTweetRepository) GetPollByTweetID(ctx context.Context, tweetID string) (*entities.Poll, error) {
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

// GetPollByID mock implementation.
func (m *MockTweetRepository) GetPollByID(ctx context.Context, pollID string) (*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if poll, ok := m.Polls[pollID]; ok {
		return poll, nil
	}
	return nil, ErrPollNotFound
}

// VotePoll mock implementation.
func (m *MockTweetRepository) VotePoll(ctx context.Context, pollID, userID, optionID string) error {
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
	for _, opt := range poll.Options {
		for _, uid := range opt.VoterIDs {
			if uid == userID {
				return ErrPollAlreadyVoted
			}
		}
	}
	for i, opt := range poll.Options {
		if opt.ID == optionID {
			poll.Options[i].Votes++
			if poll.Options[i].VoterIDs == nil {
				poll.Options[i].VoterIDs = []string{}
			}
			poll.Options[i].VoterIDs = append(poll.Options[i].VoterIDs, userID)
			return nil
		}
	}
	return ErrInvalidPollOption
}

// UnvotePoll mock implementation.
func (m *MockTweetRepository) UnvotePoll(ctx context.Context, pollID, userID, optionID string) error {
	if m.Error != nil {
		return m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return ErrPollNotFound
	}
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
			return nil
		}
	}
	return ErrInvalidPollOption
}

// GetPollResults mock implementation.
func (m *MockTweetRepository) GetPollResults(ctx context.Context, pollID string) (*entities.Poll, error) {
	return m.GetPollByID(ctx, pollID)
}

// GetUserVote mock implementation.
func (m *MockTweetRepository) GetUserVote(ctx context.Context, pollID, userID string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	poll, ok := m.Polls[pollID]
	if !ok {
		return "", ErrPollNotFound
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

// HasUserVoted mock implementation.
func (m *MockTweetRepository) HasUserVoted(ctx context.Context, pollID, userID string) (bool, error) {
	vote, err := m.GetUserVote(ctx, pollID, userID)
	return vote != "", err
}

// GetExpiredPolls mock implementation.
func (m *MockTweetRepository) GetExpiredPolls(ctx context.Context, before time.Time, limit int) ([]*entities.Poll, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var polls []*entities.Poll
	for _, poll := range m.Polls {
		if poll.ExpiresAt.Before(before) {
			polls = append(polls, poll)
		}
	}
	return polls, nil
}

// DeleteAllByUserID mock implementation.
func (m *MockTweetRepository) DeleteAllByUserID(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, tweet := range m.Tweets {
		if tweet.UserID == userID {
			now := time.Now()
			tweet.DeletedAt = &now
			m.Tweets[id] = tweet
		}
	}
	return nil
}

// BulkCreate mock implementation.
func (m *MockTweetRepository) BulkCreate(ctx context.Context, tweets []*entities.Tweet) error {
	if m.Error != nil {
		return m.Error
	}
	for _, tweet := range tweets {
		m.Tweets[tweet.ID] = tweet
	}
	return nil
}

// BulkSoftDelete mock implementation.
func (m *MockTweetRepository) BulkSoftDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	now := time.Now()
	for _, id := range ids {
		if tweet, ok := m.Tweets[id]; ok {
			tweet.DeletedAt = &now
		}
	}
	return nil
}

// BulkHardDelete mock implementation.
func (m *MockTweetRepository) BulkHardDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		delete(m.Tweets, id)
	}
	return nil
}

// GetTweetsByDateRange mock implementation.
func (m *MockTweetRepository) GetTweetsByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var tweets []*entities.Tweet
	for _, tweet := range m.Tweets {
		if tweet.DeletedAt == nil && tweet.UserID == userID && tweet.CreatedAt.After(start) && tweet.CreatedAt.Before(end) {
			tweets = append(tweets, tweet)
		}
	}
	return tweets, "", nil
}

// GetMostLikedTweets mock implementation.
func (m *MockTweetRepository) GetMostLikedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error) {
	return []*entities.Tweet{}, nil
}

// GetMostRetweetedTweets mock implementation.
func (m *MockTweetRepository) GetMostRetweetedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error) {
	return []*entities.Tweet{}, nil
}

// GetTweetStats mock implementation.
func (m *MockTweetRepository) GetTweetStats(ctx context.Context) (*TweetStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &TweetStats{
		TotalTweets:    int64(len(m.Tweets)),
		TotalLikes:     0,
		TotalRetweets:  0,
		TotalReplies:   0,
		AverageLikes:   0,
		AverageRetweets: 0,
	}, nil
}

// GetDailyTweetStats mock implementation.
func (m *MockTweetRepository) GetDailyTweetStats(ctx context.Context, start, end time.Time) ([]*DailyTweetCount, error) {
	return []*DailyTweetCount{}, nil
}

// GetUserTweetStats mock implementation.
func (m *MockTweetRepository) GetUserTweetStats(ctx context.Context, userID string) (*TweetStats, error) {
	return m.GetTweetStats(ctx)
}

// GetTweetEngagementRate mock implementation.
func (m *MockTweetRepository) GetTweetEngagementRate(ctx context.Context, tweetID string) (float64, error) {
	return 0.0, nil
}

// WithTransaction mock implementation.
func (m *MockTweetRepository) WithTransaction(ctx context.Context, tx *sql.Tx) TweetRepository {
	return m
}

// Transaction mock implementation.
func (m *MockTweetRepository) Transaction(ctx context.Context, fn func(txRepo TweetRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockTweetRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockTweetRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockTweetRepository) GetRawDB() interface{} {
	return nil
}

// ======================================================================
= Helper Functions
// ======================================================================

// containsIgnoreCase checks if a string contains another string case-insensitively.
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		strings.Contains(strings.ToLower(s), strings.ToLower(substr)))
}