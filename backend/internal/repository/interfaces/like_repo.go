// backend/internal/repository/interfaces/like_repo.go
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
	ErrLikeNotFound      = errors.New("like not found")
	ErrAlreadyLiked      = errors.New("already liked this tweet")
	ErrInvalidLikeID     = errors.New("invalid like ID")
	ErrInvalidTweetID    = errors.New("invalid tweet ID")
	ErrInvalidUserID     = errors.New("invalid user ID")
	ErrLikeDisabled      = errors.New("liking is disabled for this tweet")
)

// ======================================================================
// LikeFilter
// ======================================================================

// LikeFilter defines filtering options for like queries.
type LikeFilter struct {
	TweetID    *string
	UserID     *string
	CreatedFrom *time.Time
	CreatedTo  *time.Time
	MinLikes   *int64
	MaxLikes   *int64
}

// HasCriteria checks if any filter criteria are set.
func (f *LikeFilter) HasCriteria() bool {
	return f.TweetID != nil || f.UserID != nil ||
		f.CreatedFrom != nil || f.CreatedTo != nil ||
		f.MinLikes != nil || f.MaxLikes != nil
}

// ======================================================================
// LikePagination
// ======================================================================

// LikeSortField defines sortable fields for likes.
type LikeSortField string

const (
	SortLikeByCreatedAt LikeSortField = "created_at"
	SortLikeByUpdatedAt LikeSortField = "updated_at"
)

// LikeSortOrder defines sort order.
type LikeSortOrder string

const (
	LikeSortAsc  LikeSortOrder = "ASC"
	LikeSortDesc LikeSortOrder = "DESC"
)

// LikePagination holds pagination options for likes.
type LikePagination struct {
	Cursor string          `json:"cursor"`
	Limit  int             `json:"limit"`
	SortBy LikeSortField   `json:"sort_by"`
	Order  LikeSortOrder   `json:"order"`
}

// DefaultLikePagination returns default pagination options.
func DefaultLikePagination() *LikePagination {
	return &LikePagination{
		Limit:  20,
		SortBy: SortLikeByCreatedAt,
		Order:  LikeSortDesc,
	}
}

// Validate checks pagination parameters.
func (p *LikePagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// LikeStats
// ======================================================================

// LikeStats represents aggregated like statistics.
type LikeStats struct {
	TotalLikes        int64     `json:"total_likes"`
	UniqueUsers       int64     `json:"unique_users"`
	UniqueTweets      int64     `json:"unique_tweets"`
	LikesPerUser      float64   `json:"likes_per_user"`
	LikesPerTweet     float64   `json:"likes_per_tweet"`
	LastLike          time.Time `json:"last_like"`
	FirstLike         time.Time `json:"first_like"`
	MostLikedTweetID  string    `json:"most_liked_tweet_id"`
	MostLikedTweetCount int64   `json:"most_liked_tweet_count"`
	MostActiveUserID  string    `json:"most_active_user_id"`
	MostActiveUserLikes int64   `json:"most_active_user_likes"`
}

// ======================================================================
= DailyLikeCount
// ======================================================================

// DailyLikeCount represents daily like counts.
type DailyLikeCount struct {
	Date        time.Time `json:"date"`
	Total       int64     `json:"total"`
	UniqueUsers int64     `json:"unique_users"`
	UniqueTweets int64    `json:"unique_tweets"`
}

// ======================================================================
= LikeRepository Interface
// ======================================================================

// LikeRepository defines the interface for like data persistence.
type LikeRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a new like.
	Create(ctx context.Context, like *entities.Like) error

	// GetByID retrieves a like by its ID.
	GetByID(ctx context.Context, id string) (*entities.Like, error)

	// GetByTweetAndUser retrieves a like by tweet ID and user ID.
	GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Like, error)

	// Delete removes a like.
	Delete(ctx context.Context, id string) error

	// DeleteByTweetAndUser removes a like by tweet and user.
	DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error

	// --------------------------------------------------------------------
	// Existence Checks
	// --------------------------------------------------------------------

	// Exists checks if a user has liked a tweet.
	Exists(ctx context.Context, tweetID, userID string) (bool, error)

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountByTweetID returns the number of likes for a tweet.
	CountByTweetID(ctx context.Context, tweetID string) (int64, error)

	// CountByUserID returns the total number of likes made by a user.
	CountByUserID(ctx context.Context, userID string) (int64, error)

	// CountByTweetIDs returns like counts for multiple tweets (bulk).
	CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error)

	// CountByUserIDs returns like counts for multiple users (bulk).
	CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error)

	// CountByDateRange returns like count within a date range.
	CountByDateRange(ctx context.Context, start, end time.Time) (int64, error)

	// CountByDateRangeForUser returns like count for a user within a date range.
	CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error)

	// --------------------------------------------------------------------
	// List Operations
	// --------------------------------------------------------------------

	// GetByTweetID returns all likes for a tweet with pagination.
	GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Like, string, error)

	// GetByUserID returns all likes made by a user with pagination.
	GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Like, string, error)

	// GetLikedTweetIDs returns all tweet IDs liked by a user.
	GetLikedTweetIDs(ctx context.Context, userID string) ([]string, error)

	// GetLikedTweets returns full tweet objects liked by a user.
	GetLikedTweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// GetLikers returns users who liked a specific tweet.
	GetLikers(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.User, string, error)

	// GetLikersWithTime returns users who liked a tweet with time of like.
	GetLikersWithTime(ctx context.Context, tweetID string, cursor string, limit int) ([]*LikeWithUser, string, error)

	// --------------------------------------------------------------------
	// Timeline and Feed
	// --------------------------------------------------------------------

	// GetLikesTimeline returns likes in reverse chronological order for a user's feed.
	GetLikesTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Like, string, error)

	// GetRecentLikes returns the most recent likes for a user.
	GetRecentLikes(ctx context.Context, userID string, limit int) ([]*entities.Like, error)

	// GetRecentLikesForTweets returns recent likes for a list of tweets.
	GetRecentLikesForTweets(ctx context.Context, tweetIDs []string, limit int) (map[string][]*entities.Like, error)

	// --------------------------------------------------------------------
	// Advanced Queries
	// --------------------------------------------------------------------

	// GetMostLikedTweets returns the most liked tweets (trending).
	GetMostLikedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error)

	// GetMostLikedTweetsByCategory returns most liked tweets by category (e.g., media, poll).
	GetMostLikedTweetsByCategory(ctx context.Context, category string, limit int, since time.Time) ([]*entities.Tweet, error)

	// GetMostActiveLikers returns users with the most likes.
	GetMostActiveLikers(ctx context.Context, limit int, since time.Time) ([]*LikerStats, error)

	// GetLikesByLocation returns likes grouped by geographic location (if available).
	GetLikesByLocation(ctx context.Context, tweetID string) (map[string]int64, error)

	// GetLikesByHour returns likes grouped by hour of day.
	GetLikesByHour(ctx context.Context, tweetID string) ([]*HourlyLikeCount, error)

	// GetLikesByDayOfWeek returns likes grouped by day of week.
	GetLikesByDayOfWeek(ctx context.Context, tweetID string) ([]*DayOfWeekLikeCount, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple likes in a single transaction.
	BulkCreate(ctx context.Context, likes []*entities.Like) error

	// BulkDelete removes multiple likes in a single transaction.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkDeleteByTweetID removes all likes for a tweet.
	BulkDeleteByTweetID(ctx context.Context, tweetID string) error

	// BulkDeleteByUserID removes all likes made by a user.
	BulkDeleteByUserID(ctx context.Context, userID string) error

	// BulkDeleteByTweetAndUser removes likes for multiple tweet-user pairs.
	BulkDeleteByTweetAndUser(ctx context.Context, pairs []TweetUserPair) error

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetLikeStats returns aggregated like statistics.
	GetLikeStats(ctx context.Context) (*LikeStats, error)

	// GetUserLikeStats returns like statistics for a specific user.
	GetUserLikeStats(ctx context.Context, userID string) (*LikeStats, error)

	// GetTweetLikeStats returns like statistics for a specific tweet.
	GetTweetLikeStats(ctx context.Context, tweetID string) (*LikeStats, error)

	// GetDailyLikeStats returns daily like counts for a date range.
	GetDailyLikeStats(ctx context.Context, start, end time.Time) ([]*DailyLikeCount, error)

	// GetDailyLikeStatsForUser returns daily like counts for a user.
	GetDailyLikeStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyLikeCount, error)

	// GetLikeEngagementRate calculates like engagement rate for a tweet.
	GetLikeEngagementRate(ctx context.Context, tweetID string) (float64, error)

	// GetLikeConversionRate calculates conversion rate from view to like.
	GetLikeConversionRate(ctx context.Context, tweetID string) (float64, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) LikeRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo LikeRepository) error) error

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

// LikeWithUser represents a like with associated user data.
type LikeWithUser struct {
	Like     *entities.Like `json:"like"`
	User     *entities.User `json:"user"`
}

// LikerStats represents statistics for a user who likes tweets.
type LikerStats struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	AvatarURL  string `json:"avatar_url"`
	LikeCount  int64  `json:"like_count"`
}

// TweetUserPair represents a tweet-user pair for bulk delete.
type TweetUserPair struct {
	TweetID string `json:"tweet_id"`
	UserID  string `json:"user_id"`
}

// HourlyLikeCount represents like counts by hour.
type HourlyLikeCount struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// DayOfWeekLikeCount represents like counts by day of week.
type DayOfWeekLikeCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// ======================================================================
= Helper Functions
// ======================================================================

// IsLikeNotFound checks if an error indicates a like was not found.
func IsLikeNotFound(err error) bool {
	return errors.Is(err, ErrLikeNotFound)
}

// IsAlreadyLiked checks if an error indicates already liked.
func IsAlreadyLiked(err error) bool {
	return errors.Is(err, ErrAlreadyLiked)
}

// IsLikeError checks if an error is like-related.
func IsLikeError(err error) bool {
	return errors.Is(err, ErrLikeNotFound) ||
		errors.Is(err, ErrAlreadyLiked) ||
		errors.Is(err, ErrInvalidLikeID) ||
		errors.Is(err, ErrInvalidTweetID) ||
		errors.Is(err, ErrInvalidUserID)
}

// ======================================================================
= Mock Like Repository (for testing)
// ======================================================================

// MockLikeRepository is a mock implementation for testing.
type MockLikeRepository struct {
	Likes     map[string]*entities.Like
	TweetLikes map[string]map[string]bool // tweetID -> userID -> liked
	UserLikes  map[string]map[string]bool // userID -> tweetID -> liked
	Error     error
	NextCursor string
}

// NewMockLikeRepo creates a new mock repository.
func NewMockLikeRepo() LikeRepository {
	return &MockLikeRepository{
		Likes:      make(map[string]*entities.Like),
		TweetLikes: make(map[string]map[string]bool),
		UserLikes:  make(map[string]map[string]bool),
	}
}

// Create mock implementation.
func (m *MockLikeRepository) Create(ctx context.Context, like *entities.Like) error {
	if m.Error != nil {
		return m.Error
	}
	// Check if already liked
	if m.TweetLikes[like.TweetID] != nil && m.TweetLikes[like.TweetID][like.UserID] {
		return ErrAlreadyLiked
	}
	m.Likes[like.ID] = like
	if m.TweetLikes[like.TweetID] == nil {
		m.TweetLikes[like.TweetID] = make(map[string]bool)
	}
	m.TweetLikes[like.TweetID][like.UserID] = true
	if m.UserLikes[like.UserID] == nil {
		m.UserLikes[like.UserID] = make(map[string]bool)
	}
	m.UserLikes[like.UserID][like.TweetID] = true
	return nil
}

// GetByID mock implementation.
func (m *MockLikeRepository) GetByID(ctx context.Context, id string) (*entities.Like, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if like, ok := m.Likes[id]; ok {
		return like, nil
	}
	return nil, ErrLikeNotFound
}

// GetByTweetAndUser mock implementation.
func (m *MockLikeRepository) GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Like, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	for _, like := range m.Likes {
		if like.TweetID == tweetID && like.UserID == userID {
			return like, nil
		}
	}
	return nil, ErrLikeNotFound
}

// Delete mock implementation.
func (m *MockLikeRepository) Delete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if like, ok := m.Likes[id]; ok {
		delete(m.Likes, id)
		if m.TweetLikes[like.TweetID] != nil {
			delete(m.TweetLikes[like.TweetID], like.UserID)
		}
		if m.UserLikes[like.UserID] != nil {
			delete(m.UserLikes[like.UserID], like.TweetID)
		}
		return nil
	}
	return ErrLikeNotFound
}

// DeleteByTweetAndUser mock implementation.
func (m *MockLikeRepository) DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, like := range m.Likes {
		if like.TweetID == tweetID && like.UserID == userID {
			delete(m.Likes, id)
			if m.TweetLikes[tweetID] != nil {
				delete(m.TweetLikes[tweetID], userID)
			}
			if m.UserLikes[userID] != nil {
				delete(m.UserLikes[userID], tweetID)
			}
			return nil
		}
	}
	return ErrLikeNotFound
}

// Exists mock implementation.
func (m *MockLikeRepository) Exists(ctx context.Context, tweetID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.TweetLikes[tweetID] == nil {
		return false, nil
	}
	return m.TweetLikes[tweetID][userID], nil
}

// CountByTweetID mock implementation.
func (m *MockLikeRepository) CountByTweetID(ctx context.Context, tweetID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.TweetLikes[tweetID] == nil {
		return 0, nil
	}
	return int64(len(m.TweetLikes[tweetID])), nil
}

// CountByUserID mock implementation.
func (m *MockLikeRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.UserLikes[userID] == nil {
		return 0, nil
	}
	return int64(len(m.UserLikes[userID])), nil
}

// CountByTweetIDs mock implementation.
func (m *MockLikeRepository) CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, id := range tweetIDs {
		if m.TweetLikes[id] != nil {
			result[id] = int64(len(m.TweetLikes[id]))
		} else {
			result[id] = 0
		}
	}
	return result, nil
}

// CountByUserIDs mock implementation.
func (m *MockLikeRepository) CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, id := range userIDs {
		if m.UserLikes[id] != nil {
			result[id] = int64(len(m.UserLikes[id]))
		} else {
			result[id] = 0
		}
	}
	return result, nil
}

// CountByDateRange mock implementation.
func (m *MockLikeRepository) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, like := range m.Likes {
		if like.CreatedAt.After(start) && like.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// CountByDateRangeForUser mock implementation.
func (m *MockLikeRepository) CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, like := range m.Likes {
		if like.UserID == userID && like.CreatedAt.After(start) && like.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// GetByTweetID mock implementation.
func (m *MockLikeRepository) GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Like, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var likes []*entities.Like
	for _, like := range m.Likes {
		if like.TweetID == tweetID {
			likes = append(likes, like)
		}
	}
	return likes, "", nil
}

// GetByUserID mock implementation.
func (m *MockLikeRepository) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Like, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var likes []*entities.Like
	for _, like := range m.Likes {
		if like.UserID == userID {
			likes = append(likes, like)
		}
	}
	return likes, "", nil
}

// GetLikedTweetIDs mock implementation.
func (m *MockLikeRepository) GetLikedTweetIDs(ctx context.Context, userID string) ([]string, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var ids []string
	for _, like := range m.Likes {
		if like.UserID == userID {
			ids = append(ids, like.TweetID)
		}
	}
	return ids, nil
}

// GetLikedTweets mock implementation.
func (m *MockLikeRepository) GetLikedTweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*entities.Tweet{}, "", nil
}

// GetLikers mock implementation.
func (m *MockLikeRepository) GetLikers(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.User, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*entities.User{}, "", nil
}

// GetLikersWithTime mock implementation.
func (m *MockLikeRepository) GetLikersWithTime(ctx context.Context, tweetID string, cursor string, limit int) ([]*LikeWithUser, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*LikeWithUser{}, "", nil
}

// GetLikesTimeline mock implementation.
func (m *MockLikeRepository) GetLikesTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Like, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var likes []*entities.Like
	for _, like := range m.Likes {
		for _, uid := range userIDs {
			if like.UserID == uid {
				likes = append(likes, like)
				break
			}
		}
	}
	return likes, "", nil
}

// GetRecentLikes mock implementation.
func (m *MockLikeRepository) GetRecentLikes(ctx context.Context, userID string, limit int) ([]*entities.Like, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var likes []*entities.Like
	for _, like := range m.Likes {
		if like.UserID == userID {
			likes = append(likes, like)
		}
	}
	return likes, nil
}

// GetRecentLikesForTweets mock implementation.
func (m *MockLikeRepository) GetRecentLikesForTweets(ctx context.Context, tweetIDs []string, limit int) (map[string][]*entities.Like, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string][]*entities.Like)
	for _, tid := range tweetIDs {
		var likes []*entities.Like
		for _, like := range m.Likes {
			if like.TweetID == tid {
				likes = append(likes, like)
			}
		}
		result[tid] = likes
	}
	return result, nil
}

// GetMostLikedTweets mock implementation.
func (m *MockLikeRepository) GetMostLikedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Tweet{}, nil
}

// GetMostLikedTweetsByCategory mock implementation.
func (m *MockLikeRepository) GetMostLikedTweetsByCategory(ctx context.Context, category string, limit int, since time.Time) ([]*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Tweet{}, nil
}

// GetMostActiveLikers mock implementation.
func (m *MockLikeRepository) GetMostActiveLikers(ctx context.Context, limit int, since time.Time) ([]*LikerStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*LikerStats{}, nil
}

// GetLikesByLocation mock implementation.
func (m *MockLikeRepository) GetLikesByLocation(ctx context.Context, tweetID string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return map[string]int64{}, nil
}

// GetLikesByHour mock implementation.
func (m *MockLikeRepository) GetLikesByHour(ctx context.Context, tweetID string) ([]*HourlyLikeCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*HourlyLikeCount{}, nil
}

// GetLikesByDayOfWeek mock implementation.
func (m *MockLikeRepository) GetLikesByDayOfWeek(ctx context.Context, tweetID string) ([]*DayOfWeekLikeCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DayOfWeekLikeCount{}, nil
}

// BulkCreate mock implementation.
func (m *MockLikeRepository) BulkCreate(ctx context.Context, likes []*entities.Like) error {
	if m.Error != nil {
		return m.Error
	}
	for _, like := range likes {
		if err := m.Create(ctx, like); err != nil && !errors.Is(err, ErrAlreadyLiked) {
			return err
		}
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockLikeRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.Delete(ctx, id)
	}
	return nil
}

// BulkDeleteByTweetID mock implementation.
func (m *MockLikeRepository) BulkDeleteByTweetID(ctx context.Context, tweetID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, like := range m.Likes {
		if like.TweetID == tweetID {
			delete(m.Likes, id)
			if m.TweetLikes[tweetID] != nil {
				delete(m.TweetLikes[tweetID], like.UserID)
			}
			if m.UserLikes[like.UserID] != nil {
				delete(m.UserLikes[like.UserID], tweetID)
			}
		}
	}
	return nil
}

// BulkDeleteByUserID mock implementation.
func (m *MockLikeRepository) BulkDeleteByUserID(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, like := range m.Likes {
		if like.UserID == userID {
			delete(m.Likes, id)
			if m.TweetLikes[like.TweetID] != nil {
				delete(m.TweetLikes[like.TweetID], userID)
			}
			if m.UserLikes[userID] != nil {
				delete(m.UserLikes[userID], like.TweetID)
			}
		}
	}
	return nil
}

// BulkDeleteByTweetAndUser mock implementation.
func (m *MockLikeRepository) BulkDeleteByTweetAndUser(ctx context.Context, pairs []TweetUserPair) error {
	if m.Error != nil {
		return m.Error
	}
	for _, pair := range pairs {
		_ = m.DeleteByTweetAndUser(ctx, pair.TweetID, pair.UserID)
	}
	return nil
}

// GetLikeStats mock implementation.
func (m *MockLikeRepository) GetLikeStats(ctx context.Context) (*LikeStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &LikeStats{
		TotalLikes: int64(len(m.Likes)),
	}, nil
}

// GetUserLikeStats mock implementation.
func (m *MockLikeRepository) GetUserLikeStats(ctx context.Context, userID string) (*LikeStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	count := int64(0)
	for _, like := range m.Likes {
		if like.UserID == userID {
			count++
		}
	}
	return &LikeStats{TotalLikes: count}, nil
}

// GetTweetLikeStats mock implementation.
func (m *MockLikeRepository) GetTweetLikeStats(ctx context.Context, tweetID string) (*LikeStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	count := int64(0)
	for _, like := range m.Likes {
		if like.TweetID == tweetID {
			count++
		}
	}
	return &LikeStats{TotalLikes: count}, nil
}

// GetDailyLikeStats mock implementation.
func (m *MockLikeRepository) GetDailyLikeStats(ctx context.Context, start, end time.Time) ([]*DailyLikeCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyLikeCount{}, nil
}

// GetDailyLikeStatsForUser mock implementation.
func (m *MockLikeRepository) GetDailyLikeStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyLikeCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyLikeCount{}, nil
}

// GetLikeEngagementRate mock implementation.
func (m *MockLikeRepository) GetLikeEngagementRate(ctx context.Context, tweetID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// GetLikeConversionRate mock implementation.
func (m *MockLikeRepository) GetLikeConversionRate(ctx context.Context, tweetID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// WithTransaction mock implementation.
func (m *MockLikeRepository) WithTransaction(ctx context.Context, tx *sql.Tx) LikeRepository {
	return m
}

// Transaction mock implementation.
func (m *MockLikeRepository) Transaction(ctx context.Context, fn func(txRepo LikeRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockLikeRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockLikeRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockLikeRepository) GetRawDB() interface{} {
	return nil
}