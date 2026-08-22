// backend/internal/repository/interfaces/bookmark_repo.go
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
	ErrBookmarkNotFound      = errors.New("bookmark not found")
	ErrAlreadyBookmarked     = errors.New("already bookmarked this tweet")
	ErrInvalidBookmarkID     = errors.New("invalid bookmark ID")
	ErrInvalidTweetID        = errors.New("invalid tweet ID")
	ErrInvalidUserID         = errors.New("invalid user ID")
	ErrBookmarkDisabled      = errors.New("bookmarking is disabled for this tweet")
	ErrBookmarkNotFoundByUser = errors.New("bookmark not found for this user and tweet")
)

// ======================================================================
// BookmarkFilter
// ======================================================================

// BookmarkFilter defines filtering options for bookmark queries.
type BookmarkFilter struct {
	TweetID    *string
	UserID     *string
	CreatedFrom *time.Time
	CreatedTo  *time.Time
	MinBookmarks *int64
	MaxBookmarks *int64
}

// HasCriteria checks if any filter criteria are set.
func (f *BookmarkFilter) HasCriteria() bool {
	return f.TweetID != nil || f.UserID != nil ||
		f.CreatedFrom != nil || f.CreatedTo != nil ||
		f.MinBookmarks != nil || f.MaxBookmarks != nil
}

// ======================================================================
// BookmarkPagination
// ======================================================================

// BookmarkSortField defines sortable fields for bookmarks.
type BookmarkSortField string

const (
	SortBookmarkByCreatedAt BookmarkSortField = "created_at"
	SortBookmarkByUpdatedAt BookmarkSortField = "updated_at"
)

// BookmarkSortOrder defines sort order.
type BookmarkSortOrder string

const (
	BookmarkSortAsc  BookmarkSortOrder = "ASC"
	BookmarkSortDesc BookmarkSortOrder = "DESC"
)

// BookmarkPagination holds pagination options for bookmarks.
type BookmarkPagination struct {
	Cursor string              `json:"cursor"`
	Limit  int                 `json:"limit"`
	SortBy BookmarkSortField   `json:"sort_by"`
	Order  BookmarkSortOrder   `json:"order"`
}

// DefaultBookmarkPagination returns default pagination options.
func DefaultBookmarkPagination() *BookmarkPagination {
	return &BookmarkPagination{
		Limit:  20,
		SortBy: SortBookmarkByCreatedAt,
		Order:  BookmarkSortDesc,
	}
}

// Validate checks pagination parameters.
func (p *BookmarkPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// BookmarkStats
// ======================================================================

// BookmarkStats represents aggregated bookmark statistics.
type BookmarkStats struct {
	TotalBookmarks       int64     `json:"total_bookmarks"`
	UniqueUsers          int64     `json:"unique_users"`
	UniqueTweets         int64     `json:"unique_tweets"`
	BookmarksPerUser     float64   `json:"bookmarks_per_user"`
	BookmarksPerTweet    float64   `json:"bookmarks_per_tweet"`
	LastBookmark         time.Time `json:"last_bookmark"`
	FirstBookmark        time.Time `json:"first_bookmark"`
	MostBookmarkedTweetID string   `json:"most_bookmarked_tweet_id"`
	MostBookmarkedTweetCount int64 `json:"most_bookmarked_tweet_count"`
	MostActiveBookmarkerID string  `json:"most_active_bookmarker_id"`
	MostActiveBookmarkerCount int64 `json:"most_active_bookmarker_count"`
}

// ======================================================================
= DailyBookmarkCount
// ======================================================================

// DailyBookmarkCount represents daily bookmark counts.
type DailyBookmarkCount struct {
	Date         time.Time `json:"date"`
	Total        int64     `json:"total"`
	UniqueUsers  int64     `json:"unique_users"`
	UniqueTweets int64     `json:"unique_tweets"`
}

// ======================================================================
= BookmarkRepository Interface
// ======================================================================

// BookmarkRepository defines the interface for bookmark data persistence.
type BookmarkRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a new bookmark.
	Create(ctx context.Context, bookmark *entities.Bookmark) error

	// GetByID retrieves a bookmark by its ID.
	GetByID(ctx context.Context, id string) (*entities.Bookmark, error)

	// GetByTweetAndUser retrieves a bookmark by tweet ID and user ID.
	GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Bookmark, error)

	// Delete removes a bookmark.
	Delete(ctx context.Context, id string) error

	// DeleteByTweetAndUser removes a bookmark by tweet and user.
	DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error

	// --------------------------------------------------------------------
	// Existence Checks
	// --------------------------------------------------------------------

	// Exists checks if a user has bookmarked a tweet.
	Exists(ctx context.Context, tweetID, userID string) (bool, error)

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountByTweetID returns the number of bookmarks for a tweet.
	CountByTweetID(ctx context.Context, tweetID string) (int64, error)

	// CountByUserID returns the total number of bookmarks made by a user.
	CountByUserID(ctx context.Context, userID string) (int64, error)

	// CountByTweetIDs returns bookmark counts for multiple tweets (bulk).
	CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error)

	// CountByUserIDs returns bookmark counts for multiple users (bulk).
	CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error)

	// CountByDateRange returns bookmark count within a date range.
	CountByDateRange(ctx context.Context, start, end time.Time) (int64, error)

	// CountByDateRangeForUser returns bookmark count for a user within a date range.
	CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error)

	// --------------------------------------------------------------------
	// List Operations
	// --------------------------------------------------------------------

	// GetByTweetID returns all bookmarks for a tweet with pagination.
	GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Bookmark, string, error)

	// GetByUserID returns all bookmarks made by a user with pagination.
	GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Bookmark, string, error)

	// GetBookmarkedTweetIDs returns all tweet IDs bookmarked by a user.
	GetBookmarkedTweetIDs(ctx context.Context, userID string) ([]string, error)

	// GetBookmarkedTweets returns full tweet objects bookmarked by a user.
	GetBookmarkedTweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// GetBookmarkers returns users who bookmarked a specific tweet.
	GetBookmarkers(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.User, string, error)

	// GetBookmarkersWithTime returns users who bookmarked a tweet with time of bookmark.
	GetBookmarkersWithTime(ctx context.Context, tweetID string, cursor string, limit int) ([]*BookmarkWithUser, string, error)

	// --------------------------------------------------------------------
	// Timeline and Feed
	// --------------------------------------------------------------------

	// GetBookmarksTimeline returns bookmarks in reverse chronological order for a user's feed.
	GetBookmarksTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Bookmark, string, error)

	// GetRecentBookmarks returns the most recent bookmarks for a user.
	GetRecentBookmarks(ctx context.Context, userID string, limit int) ([]*entities.Bookmark, error)

	// GetRecentBookmarksForTweets returns recent bookmarks for a list of tweets.
	GetRecentBookmarksForTweets(ctx context.Context, tweetIDs []string, limit int) (map[string][]*entities.Bookmark, error)

	// --------------------------------------------------------------------
	// Advanced Queries
	// --------------------------------------------------------------------

	// GetMostBookmarkedTweets returns the most bookmarked tweets.
	GetMostBookmarkedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error)

	// GetMostBookmarkedTweetsByCategory returns most bookmarked tweets by category.
	GetMostBookmarkedTweetsByCategory(ctx context.Context, category string, limit int, since time.Time) ([]*entities.Tweet, error)

	// GetMostActiveBookmarkers returns users with the most bookmarks.
	GetMostActiveBookmarkers(ctx context.Context, limit int, since time.Time) ([]*BookmarkerStats, error)

	// GetBookmarksByHour returns bookmarks grouped by hour of day.
	GetBookmarksByHour(ctx context.Context, tweetID string) ([]*HourlyBookmarkCount, error)

	// GetBookmarksByDayOfWeek returns bookmarks grouped by day of week.
	GetBookmarksByDayOfWeek(ctx context.Context, tweetID string) ([]*DayOfWeekBookmarkCount, error)

	// GetBookmarksByDateRange returns bookmarks within a date range.
	GetBookmarksByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Bookmark, string, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple bookmarks in a single transaction.
	BulkCreate(ctx context.Context, bookmarks []*entities.Bookmark) error

	// BulkDelete removes multiple bookmarks in a single transaction.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkDeleteByTweetID removes all bookmarks for a tweet.
	BulkDeleteByTweetID(ctx context.Context, tweetID string) error

	// BulkDeleteByUserID removes all bookmarks made by a user.
	BulkDeleteByUserID(ctx context.Context, userID string) error

	// BulkDeleteByTweetAndUser removes bookmarks for multiple tweet-user pairs.
	BulkDeleteByTweetAndUser(ctx context.Context, pairs []TweetUserPair) error

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetBookmarkStats returns aggregated bookmark statistics.
	GetBookmarkStats(ctx context.Context) (*BookmarkStats, error)

	// GetUserBookmarkStats returns bookmark statistics for a specific user.
	GetUserBookmarkStats(ctx context.Context, userID string) (*BookmarkStats, error)

	// GetTweetBookmarkStats returns bookmark statistics for a specific tweet.
	GetTweetBookmarkStats(ctx context.Context, tweetID string) (*BookmarkStats, error)

	// GetDailyBookmarkStats returns daily bookmark counts for a date range.
	GetDailyBookmarkStats(ctx context.Context, start, end time.Time) ([]*DailyBookmarkCount, error)

	// GetDailyBookmarkStatsForUser returns daily bookmark counts for a user.
	GetDailyBookmarkStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyBookmarkCount, error)

	// GetBookmarkEngagementRate calculates bookmark engagement rate for a tweet.
	GetBookmarkEngagementRate(ctx context.Context, tweetID string) (float64, error)

	// GetBookmarkConversionRate calculates conversion rate from view to bookmark.
	GetBookmarkConversionRate(ctx context.Context, tweetID string) (float64, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) BookmarkRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo BookmarkRepository) error) error

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

// BookmarkWithUser represents a bookmark with associated user data.
type BookmarkWithUser struct {
	Bookmark *entities.Bookmark `json:"bookmark"`
	User     *entities.User     `json:"user"`
}

// BookmarkerStats represents statistics for a user who bookmarks.
type BookmarkerStats struct {
	UserID        string `json:"user_id"`
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	AvatarURL     string `json:"avatar_url"`
	BookmarkCount int64  `json:"bookmark_count"`
}

// TweetUserPair represents a tweet-user pair for bulk delete.
type TweetUserPair struct {
	TweetID string `json:"tweet_id"`
	UserID  string `json:"user_id"`
}

// HourlyBookmarkCount represents bookmark counts by hour.
type HourlyBookmarkCount struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// DayOfWeekBookmarkCount represents bookmark counts by day of week.
type DayOfWeekBookmarkCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

// ======================================================================
= Helper Functions
// ======================================================================

// IsBookmarkNotFound checks if an error indicates a bookmark was not found.
func IsBookmarkNotFound(err error) bool {
	return errors.Is(err, ErrBookmarkNotFound)
}

// IsAlreadyBookmarked checks if an error indicates already bookmarked.
func IsAlreadyBookmarked(err error) bool {
	return errors.Is(err, ErrAlreadyBookmarked)
}

// IsBookmarkError checks if an error is bookmark-related.
func IsBookmarkError(err error) bool {
	return errors.Is(err, ErrBookmarkNotFound) ||
		errors.Is(err, ErrAlreadyBookmarked) ||
		errors.Is(err, ErrInvalidBookmarkID) ||
		errors.Is(err, ErrInvalidTweetID) ||
		errors.Is(err, ErrInvalidUserID)
}

// ======================================================================
= Mock Bookmark Repository (for testing)
// ======================================================================

// MockBookmarkRepository is a mock implementation for testing.
type MockBookmarkRepository struct {
	Bookmarks      map[string]*entities.Bookmark
	TweetBookmarks map[string]map[string]bool // tweetID -> userID -> bookmarked
	UserBookmarks  map[string]map[string]bool // userID -> tweetID -> bookmarked
	Error          error
	NextCursor     string
}

// NewMockBookmarkRepo creates a new mock repository.
func NewMockBookmarkRepo() BookmarkRepository {
	return &MockBookmarkRepository{
		Bookmarks:      make(map[string]*entities.Bookmark),
		TweetBookmarks: make(map[string]map[string]bool),
		UserBookmarks:  make(map[string]map[string]bool),
	}
}

// Create mock implementation.
func (m *MockBookmarkRepository) Create(ctx context.Context, bookmark *entities.Bookmark) error {
	if m.Error != nil {
		return m.Error
	}
	if m.TweetBookmarks[bookmark.TweetID] != nil && m.TweetBookmarks[bookmark.TweetID][bookmark.UserID] {
		return ErrAlreadyBookmarked
	}
	m.Bookmarks[bookmark.ID] = bookmark
	if m.TweetBookmarks[bookmark.TweetID] == nil {
		m.TweetBookmarks[bookmark.TweetID] = make(map[string]bool)
	}
	m.TweetBookmarks[bookmark.TweetID][bookmark.UserID] = true
	if m.UserBookmarks[bookmark.UserID] == nil {
		m.UserBookmarks[bookmark.UserID] = make(map[string]bool)
	}
	m.UserBookmarks[bookmark.UserID][bookmark.TweetID] = true
	return nil
}

// GetByID mock implementation.
func (m *MockBookmarkRepository) GetByID(ctx context.Context, id string) (*entities.Bookmark, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if bookmark, ok := m.Bookmarks[id]; ok {
		return bookmark, nil
	}
	return nil, ErrBookmarkNotFound
}

// GetByTweetAndUser mock implementation.
func (m *MockBookmarkRepository) GetByTweetAndUser(ctx context.Context, tweetID, userID string) (*entities.Bookmark, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	for _, bookmark := range m.Bookmarks {
		if bookmark.TweetID == tweetID && bookmark.UserID == userID {
			return bookmark, nil
		}
	}
	return nil, ErrBookmarkNotFound
}

// Delete mock implementation.
func (m *MockBookmarkRepository) Delete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if bookmark, ok := m.Bookmarks[id]; ok {
		delete(m.Bookmarks, id)
		if m.TweetBookmarks[bookmark.TweetID] != nil {
			delete(m.TweetBookmarks[bookmark.TweetID], bookmark.UserID)
		}
		if m.UserBookmarks[bookmark.UserID] != nil {
			delete(m.UserBookmarks[bookmark.UserID], bookmark.TweetID)
		}
		return nil
	}
	return ErrBookmarkNotFound
}

// DeleteByTweetAndUser mock implementation.
func (m *MockBookmarkRepository) DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, bookmark := range m.Bookmarks {
		if bookmark.TweetID == tweetID && bookmark.UserID == userID {
			delete(m.Bookmarks, id)
			if m.TweetBookmarks[tweetID] != nil {
				delete(m.TweetBookmarks[tweetID], userID)
			}
			if m.UserBookmarks[userID] != nil {
				delete(m.UserBookmarks[userID], tweetID)
			}
			return nil
		}
	}
	return ErrBookmarkNotFound
}

// Exists mock implementation.
func (m *MockBookmarkRepository) Exists(ctx context.Context, tweetID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.TweetBookmarks[tweetID] == nil {
		return false, nil
	}
	return m.TweetBookmarks[tweetID][userID], nil
}

// CountByTweetID mock implementation.
func (m *MockBookmarkRepository) CountByTweetID(ctx context.Context, tweetID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.TweetBookmarks[tweetID] == nil {
		return 0, nil
	}
	return int64(len(m.TweetBookmarks[tweetID])), nil
}

// CountByUserID mock implementation.
func (m *MockBookmarkRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.UserBookmarks[userID] == nil {
		return 0, nil
	}
	return int64(len(m.UserBookmarks[userID])), nil
}

// CountByTweetIDs mock implementation.
func (m *MockBookmarkRepository) CountByTweetIDs(ctx context.Context, tweetIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, id := range tweetIDs {
		if m.TweetBookmarks[id] != nil {
			result[id] = int64(len(m.TweetBookmarks[id]))
		} else {
			result[id] = 0
		}
	}
	return result, nil
}

// CountByUserIDs mock implementation.
func (m *MockBookmarkRepository) CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, id := range userIDs {
		if m.UserBookmarks[id] != nil {
			result[id] = int64(len(m.UserBookmarks[id]))
		} else {
			result[id] = 0
		}
	}
	return result, nil
}

// CountByDateRange mock implementation.
func (m *MockBookmarkRepository) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, bookmark := range m.Bookmarks {
		if bookmark.CreatedAt.After(start) && bookmark.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// CountByDateRangeForUser mock implementation.
func (m *MockBookmarkRepository) CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, bookmark := range m.Bookmarks {
		if bookmark.UserID == userID && bookmark.CreatedAt.After(start) && bookmark.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// GetByTweetID mock implementation.
func (m *MockBookmarkRepository) GetByTweetID(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var bookmarks []*entities.Bookmark
	for _, bookmark := range m.Bookmarks {
		if bookmark.TweetID == tweetID {
			bookmarks = append(bookmarks, bookmark)
		}
	}
	return bookmarks, "", nil
}

// GetByUserID mock implementation.
func (m *MockBookmarkRepository) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var bookmarks []*entities.Bookmark
	for _, bookmark := range m.Bookmarks {
		if bookmark.UserID == userID {
			bookmarks = append(bookmarks, bookmark)
		}
	}
	return bookmarks, "", nil
}

// GetBookmarkedTweetIDs mock implementation.
func (m *MockBookmarkRepository) GetBookmarkedTweetIDs(ctx context.Context, userID string) ([]string, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var ids []string
	for _, bookmark := range m.Bookmarks {
		if bookmark.UserID == userID {
			ids = append(ids, bookmark.TweetID)
		}
	}
	return ids, nil
}

// GetBookmarkedTweets mock implementation.
func (m *MockBookmarkRepository) GetBookmarkedTweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*entities.Tweet{}, "", nil
}

// GetBookmarkers mock implementation.
func (m *MockBookmarkRepository) GetBookmarkers(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.User, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*entities.User{}, "", nil
}

// GetBookmarkersWithTime mock implementation.
func (m *MockBookmarkRepository) GetBookmarkersWithTime(ctx context.Context, tweetID string, cursor string, limit int) ([]*BookmarkWithUser, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*BookmarkWithUser{}, "", nil
}

// GetBookmarksTimeline mock implementation.
func (m *MockBookmarkRepository) GetBookmarksTimeline(ctx context.Context, userIDs []string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var bookmarks []*entities.Bookmark
	for _, bookmark := range m.Bookmarks {
		for _, uid := range userIDs {
			if bookmark.UserID == uid {
				bookmarks = append(bookmarks, bookmark)
				break
			}
		}
	}
	return bookmarks, "", nil
}

// GetRecentBookmarks mock implementation.
func (m *MockBookmarkRepository) GetRecentBookmarks(ctx context.Context, userID string, limit int) ([]*entities.Bookmark, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var bookmarks []*entities.Bookmark
	for _, bookmark := range m.Bookmarks {
		if bookmark.UserID == userID {
			bookmarks = append(bookmarks, bookmark)
		}
	}
	return bookmarks, nil
}

// GetRecentBookmarksForTweets mock implementation.
func (m *MockBookmarkRepository) GetRecentBookmarksForTweets(ctx context.Context, tweetIDs []string, limit int) (map[string][]*entities.Bookmark, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string][]*entities.Bookmark)
	for _, tid := range tweetIDs {
		var bookmarks []*entities.Bookmark
		for _, bookmark := range m.Bookmarks {
			if bookmark.TweetID == tid {
				bookmarks = append(bookmarks, bookmark)
			}
		}
		result[tid] = bookmarks
	}
	return result, nil
}

// GetMostBookmarkedTweets mock implementation.
func (m *MockBookmarkRepository) GetMostBookmarkedTweets(ctx context.Context, limit int, since time.Time) ([]*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Tweet{}, nil
}

// GetMostBookmarkedTweetsByCategory mock implementation.
func (m *MockBookmarkRepository) GetMostBookmarkedTweetsByCategory(ctx context.Context, category string, limit int, since time.Time) ([]*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Tweet{}, nil
}

// GetMostActiveBookmarkers mock implementation.
func (m *MockBookmarkRepository) GetMostActiveBookmarkers(ctx context.Context, limit int, since time.Time) ([]*BookmarkerStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*BookmarkerStats{}, nil
}

// GetBookmarksByHour mock implementation.
func (m *MockBookmarkRepository) GetBookmarksByHour(ctx context.Context, tweetID string) ([]*HourlyBookmarkCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*HourlyBookmarkCount{}, nil
}

// GetBookmarksByDayOfWeek mock implementation.
func (m *MockBookmarkRepository) GetBookmarksByDayOfWeek(ctx context.Context, tweetID string) ([]*DayOfWeekBookmarkCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DayOfWeekBookmarkCount{}, nil
}

// GetBookmarksByDateRange mock implementation.
func (m *MockBookmarkRepository) GetBookmarksByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var bookmarks []*entities.Bookmark
	for _, bookmark := range m.Bookmarks {
		if bookmark.UserID == userID && bookmark.CreatedAt.After(start) && bookmark.CreatedAt.Before(end) {
			bookmarks = append(bookmarks, bookmark)
		}
	}
	return bookmarks, "", nil
}

// BulkCreate mock implementation.
func (m *MockBookmarkRepository) BulkCreate(ctx context.Context, bookmarks []*entities.Bookmark) error {
	if m.Error != nil {
		return m.Error
	}
	for _, bookmark := range bookmarks {
		if err := m.Create(ctx, bookmark); err != nil && !errors.Is(err, ErrAlreadyBookmarked) {
			return err
		}
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockBookmarkRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.Delete(ctx, id)
	}
	return nil
}

// BulkDeleteByTweetID mock implementation.
func (m *MockBookmarkRepository) BulkDeleteByTweetID(ctx context.Context, tweetID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, bookmark := range m.Bookmarks {
		if bookmark.TweetID == tweetID {
			delete(m.Bookmarks, id)
			if m.TweetBookmarks[tweetID] != nil {
				delete(m.TweetBookmarks[tweetID], bookmark.UserID)
			}
			if m.UserBookmarks[bookmark.UserID] != nil {
				delete(m.UserBookmarks[bookmark.UserID], tweetID)
			}
		}
	}
	return nil
}

// BulkDeleteByUserID mock implementation.
func (m *MockBookmarkRepository) BulkDeleteByUserID(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, bookmark := range m.Bookmarks {
		if bookmark.UserID == userID {
			delete(m.Bookmarks, id)
			if m.TweetBookmarks[bookmark.TweetID] != nil {
				delete(m.TweetBookmarks[bookmark.TweetID], userID)
			}
			if m.UserBookmarks[userID] != nil {
				delete(m.UserBookmarks[userID], bookmark.TweetID)
			}
		}
	}
	return nil
}

// BulkDeleteByTweetAndUser mock implementation.
func (m *MockBookmarkRepository) BulkDeleteByTweetAndUser(ctx context.Context, pairs []TweetUserPair) error {
	if m.Error != nil {
		return m.Error
	}
	for _, pair := range pairs {
		_ = m.DeleteByTweetAndUser(ctx, pair.TweetID, pair.UserID)
	}
	return nil
}

// GetBookmarkStats mock implementation.
func (m *MockBookmarkRepository) GetBookmarkStats(ctx context.Context) (*BookmarkStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &BookmarkStats{
		TotalBookmarks: int64(len(m.Bookmarks)),
	}, nil
}

// GetUserBookmarkStats mock implementation.
func (m *MockBookmarkRepository) GetUserBookmarkStats(ctx context.Context, userID string) (*BookmarkStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	count := int64(0)
	for _, bookmark := range m.Bookmarks {
		if bookmark.UserID == userID {
			count++
		}
	}
	return &BookmarkStats{TotalBookmarks: count}, nil
}

// GetTweetBookmarkStats mock implementation.
func (m *MockBookmarkRepository) GetTweetBookmarkStats(ctx context.Context, tweetID string) (*BookmarkStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	count := int64(0)
	for _, bookmark := range m.Bookmarks {
		if bookmark.TweetID == tweetID {
			count++
		}
	}
	return &BookmarkStats{TotalBookmarks: count}, nil
}

// GetDailyBookmarkStats mock implementation.
func (m *MockBookmarkRepository) GetDailyBookmarkStats(ctx context.Context, start, end time.Time) ([]*DailyBookmarkCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyBookmarkCount{}, nil
}

// GetDailyBookmarkStatsForUser mock implementation.
func (m *MockBookmarkRepository) GetDailyBookmarkStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyBookmarkCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyBookmarkCount{}, nil
}

// GetBookmarkEngagementRate mock implementation.
func (m *MockBookmarkRepository) GetBookmarkEngagementRate(ctx context.Context, tweetID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// GetBookmarkConversionRate mock implementation.
func (m *MockBookmarkRepository) GetBookmarkConversionRate(ctx context.Context, tweetID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// WithTransaction mock implementation.
func (m *MockBookmarkRepository) WithTransaction(ctx context.Context, tx *sql.Tx) BookmarkRepository {
	return m
}

// Transaction mock implementation.
func (m *MockBookmarkRepository) Transaction(ctx context.Context, fn func(txRepo BookmarkRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockBookmarkRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockBookmarkRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockBookmarkRepository) GetRawDB() interface{} {
	return nil
}