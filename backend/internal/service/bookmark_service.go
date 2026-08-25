// backend/internal/service/bookmark_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	MaxBookmarksPerBatch = 100
	MaxBookmarkNotesLen  = 500
	MaxBookmarkNameLen   = 100
)

// BookmarkCategory represents valid bookmark categories.
type BookmarkCategory string

const (
	CategoryReadLater  BookmarkCategory = "read_later"
	CategoryFavorites  BookmarkCategory = "favorites"
	CategoryImportant  BookmarkCategory = "important"
	CategoryWatchLater BookmarkCategory = "watch_later"
	CategoryCustom     BookmarkCategory = "custom"
)

var (
	ErrBookmarkNotFound      = errors.New("bookmark not found")
	ErrAlreadyBookmarked     = errors.New("already bookmarked this tweet")
	ErrInvalidCategory       = errors.New("invalid bookmark category")
	ErrCategoryNameRequired  = errors.New("category name is required for custom category")
	ErrBookmarkNotesTooLong  = errors.New("bookmark notes exceed maximum length")
	ErrBookmarkNameTooLong   = errors.New("bookmark name exceeds maximum length")
	ErrTweetNotFound         = errors.New("tweet not found")
	ErrUserNotFound          = errors.New("user not found")
	ErrCannotBookmarkOwn     = errors.New("cannot bookmark your own tweet")
)

// ======================================================================
// BookmarkService Interface
// ======================================================================

// BookmarkService defines the bookmark service interface.
type BookmarkService interface {
	// Create adds a bookmark to a tweet.
	Create(ctx context.Context, tweetID, userID, category, name, notes string) (*entities.Bookmark, error)
	
	// Delete removes a bookmark.
	Delete(ctx context.Context, bookmarkID, userID string) error
	
	// DeleteByTweetAndUser removes a bookmark by tweet and user.
	DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error
	
	// GetByID retrieves a bookmark by ID.
	GetByID(ctx context.Context, bookmarkID string) (*entities.Bookmark, error)
	
	// IsBookmarked checks if a user has bookmarked a tweet.
	IsBookmarked(ctx context.Context, tweetID, userID string) (bool, error)
	
	// GetBookmarkCount returns the number of bookmarks for a tweet.
	GetBookmarkCount(ctx context.Context, tweetID string) (int64, error)
	
	// GetUserBookmarks returns all bookmarks made by a user.
	GetUserBookmarks(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Bookmark, string, error)
	
	// GetUserBookmarksWithTweets returns bookmarks with tweet data for a user.
	GetUserBookmarksWithTweets(ctx context.Context, userID string, cursor string, limit int) ([]*dto.BookmarkResponse, string, error)
	
	// GetBookmarksByCategory returns bookmarks by category for a user.
	GetBookmarksByCategory(ctx context.Context, userID, category string, cursor string, limit int) ([]*entities.Bookmark, string, error)
	
	// UpdateBookmark updates a bookmark's category, name, or notes.
	UpdateBookmark(ctx context.Context, bookmarkID, userID, category, name, notes string) (*entities.Bookmark, error)
	
	// GetBookmarkStats returns bookmark statistics.
	GetBookmarkStats(ctx context.Context) (*dto.BookmarkStatsResponse, error)
	
	// GetUserBookmarkStats returns bookmark statistics for a user.
	GetUserBookmarkStats(ctx context.Context, userID string) (*dto.BookmarkStatsResponse, error)
	
	// BulkCreate adds bookmarks to multiple tweets.
	BulkCreate(ctx context.Context, userID string, tweetIDs []string, category string) ([]string, error)
	
	// BulkDelete removes bookmarks from multiple tweets.
	BulkDelete(ctx context.Context, userID string, bookmarkIDs []string) ([]string, error)
	
	// GetCategories returns all valid bookmark categories.
	GetCategories() []string
}

// ======================================================================
// bookmarkService Implementation
// ======================================================================

// bookmarkService implements BookmarkService.
type bookmarkService struct {
	bookmarkRepo     interfaces.BookmarkRepository
	tweetRepo        interfaces.TweetRepository
	userRepo         interfaces.UserRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	log              *logrus.Entry
}

// NewBookmarkService creates a new bookmark service.
func NewBookmarkService(
	bookmarkRepo interfaces.BookmarkRepository,
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) BookmarkService {
	return &bookmarkService{
		bookmarkRepo:     bookmarkRepo,
		tweetRepo:        tweetRepo,
		userRepo:         userRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		log:              logger.WithField("service", "bookmark"),
	}
}

// ======================================================================
// Create Bookmark
// ======================================================================

// Create adds a bookmark to a tweet.
func (s *bookmarkService) Create(ctx context.Context, tweetID, userID, category, name, notes string) (*entities.Bookmark, error) {
	// Validate category
	if category != "" && !s.isValidCategory(category) {
		return nil, ErrInvalidCategory
	}
	if category == string(CategoryCustom) && strings.TrimSpace(name) == "" {
		return nil, ErrCategoryNameRequired
	}
	if len(notes) > MaxBookmarkNotesLen {
		return nil, ErrBookmarkNotesTooLong
	}
	if len(name) > MaxBookmarkNameLen {
		return nil, ErrBookmarkNameTooLong
	}
	// Check if user exists
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Check if tweet exists
	tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, ErrTweetNotFound
		}
		return nil, fmt.Errorf("failed to get tweet: %w", err)
	}
	if tweet.DeletedAt != nil {
		return nil, ErrTweetNotFound
	}
	// Check if already bookmarked
	exists, err := s.bookmarkRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check bookmark status: %w", err)
	}
	if exists {
		return nil, ErrAlreadyBookmarked
	}
	// Create bookmark
	bookmark := &entities.Bookmark{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		UserID:    userID,
		Category:  category,
		Name:      name,
		Notes:     notes,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if category == string(CategoryCustom) && name != "" {
		bookmark.Name = name
	}
	if err := s.bookmarkRepo.Create(ctx, bookmark); err != nil {
		return nil, fmt.Errorf("failed to create bookmark: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateBookmarkCache(ctx, tweetID, userID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"tweet_id": tweetID,
		"category": category,
	}).Info("Tweet bookmarked")
	return bookmark, nil
}

// ======================================================================
= Delete Bookmark
// ======================================================================

// Delete removes a bookmark.
func (s *bookmarkService) Delete(ctx context.Context, bookmarkID, userID string) error {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Get bookmark
	bookmark, err := s.bookmarkRepo.GetByID(ctx, bookmarkID)
	if err != nil {
		if errors.Is(err, interfaces.ErrBookmarkNotFound) {
			return ErrBookmarkNotFound
		}
		return fmt.Errorf("failed to get bookmark: %w", err)
	}
	if bookmark.UserID != userID {
		return errors.New("bookmark does not belong to user")
	}
	// Delete bookmark
	if err := s.bookmarkRepo.Delete(ctx, bookmarkID); err != nil {
		return fmt.Errorf("failed to delete bookmark: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateBookmarkCache(ctx, bookmark.TweetID, userID)
	s.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"tweet_id":    bookmark.TweetID,
		"bookmark_id": bookmarkID,
	}).Info("Bookmark deleted")
	return nil
}

// DeleteByTweetAndUser removes a bookmark by tweet and user.
func (s *bookmarkService) DeleteByTweetAndUser(ctx context.Context, tweetID, userID string) error {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Check if tweet exists
	_, err = s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return fmt.Errorf("failed to get tweet: %w", err)
	}
	// Delete bookmark
	if err := s.bookmarkRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
		if errors.Is(err, interfaces.ErrBookmarkNotFound) {
			return ErrBookmarkNotFound
		}
		return fmt.Errorf("failed to delete bookmark: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateBookmarkCache(ctx, tweetID, userID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"tweet_id": tweetID,
	}).Info("Bookmark deleted by tweet and user")
	return nil
}

// ======================================================================
// Get Bookmark
// ======================================================================

// GetByID retrieves a bookmark by ID.
func (s *bookmarkService) GetByID(ctx context.Context, bookmarkID string) (*entities.Bookmark, error) {
	bookmark, err := s.bookmarkRepo.GetByID(ctx, bookmarkID)
	if err != nil {
		if errors.Is(err, interfaces.ErrBookmarkNotFound) {
			return nil, ErrBookmarkNotFound
		}
		return nil, fmt.Errorf("failed to get bookmark: %w", err)
	}
	return bookmark, nil
}

// ======================================================================
// IsBookmarked
// ======================================================================

// IsBookmarked checks if a user has bookmarked a tweet.
func (s *bookmarkService) IsBookmarked(ctx context.Context, tweetID, userID string) (bool, error) {
	// Try cache first
	if s.redisAdapter != nil {
		cacheKey := fmt.Sprintf("bookmarked:%s:%s", tweetID, userID)
		var bookmarked bool
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &bookmarked); err == nil {
			return bookmarked, nil
		}
	}
	bookmarked, err := s.bookmarkRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check bookmark status: %w", err)
	}
	// Cache for 10 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, fmt.Sprintf("bookmarked:%s:%s", tweetID, userID), bookmarked, 10*time.Second)
	}
	return bookmarked, nil
}

// ======================================================================
// GetBookmarkCount
// ======================================================================

// GetBookmarkCount returns the number of bookmarks for a tweet.
func (s *bookmarkService) GetBookmarkCount(ctx context.Context, tweetID string) (int64, error) {
	// Try cache first
	if s.redisAdapter != nil {
		cacheKey := fmt.Sprintf("bookmark_count:%s", tweetID)
		var count int64
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &count); err == nil {
			return count, nil
		}
	}
	count, err := s.bookmarkRepo.CountByTweetID(ctx, tweetID)
	if err != nil {
		return 0, fmt.Errorf("failed to get bookmark count: %w", err)
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, fmt.Sprintf("bookmark_count:%s", tweetID), count, 30*time.Second)
	}
	return count, nil
}

// ======================================================================
// GetUserBookmarks
// ======================================================================

// GetUserBookmarks returns all bookmarks made by a user.
func (s *bookmarkService) GetUserBookmarks(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", ErrUserNotFound
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}
	// Try cache first
	cacheKey := fmt.Sprintf("user_bookmarks:%s:%s:%d", userID, cursor, limit)
	if s.redisAdapter != nil {
		var cached []*entities.Bookmark
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached, cursor, nil
		}
	}
	bookmarks, nextCursor, err := s.bookmarkRepo.GetByUserID(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user bookmarks: %w", err)
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil && len(bookmarks) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, bookmarks, 30*time.Second)
	}
	return bookmarks, nextCursor, nil
}

// ======================================================================
// GetUserBookmarksWithTweets
// ======================================================================

// GetUserBookmarksWithTweets returns bookmarks with tweet data for a user.
func (s *bookmarkService) GetUserBookmarksWithTweets(ctx context.Context, userID string, cursor string, limit int) ([]*dto.BookmarkResponse, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", ErrUserNotFound
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}
	bookmarks, nextCursor, err := s.bookmarkRepo.GetByUserID(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user bookmarks: %w", err)
	}
	responses := make([]*dto.BookmarkResponse, 0, len(bookmarks))
	for _, bm := range bookmarks {
		tweet, err := s.tweetRepo.GetByID(ctx, bm.TweetID)
		if err != nil || tweet.DeletedAt != nil {
			continue
		}
		user, err := s.userRepo.GetByID(ctx, tweet.UserID)
		if err != nil {
			continue
		}
		resp := &dto.BookmarkResponse{
			ID:          bm.ID,
			TweetID:     bm.TweetID,
			UserID:      bm.UserID,
			Category:    bm.Category,
			Name:        bm.Name,
			Notes:       bm.Notes,
			CreatedAt:   bm.CreatedAt,
			UpdatedAt:   bm.UpdatedAt,
			DisplayName: bm.GetDisplayName(),
			HasNotes:    bm.HasNotes(),
		}
		responses = append(responses, resp)
	}
	return responses, nextCursor, nil
}

// ======================================================================
// GetBookmarksByCategory
// ======================================================================

// GetBookmarksByCategory returns bookmarks by category for a user.
func (s *bookmarkService) GetBookmarksByCategory(ctx context.Context, userID, category string, cursor string, limit int) ([]*entities.Bookmark, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if !s.isValidCategory(category) {
		return nil, "", ErrInvalidCategory
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", ErrUserNotFound
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}
	// Since we don't have a direct method, get all and filter
	bookmarks, nextCursor, err := s.bookmarkRepo.GetByUserID(ctx, userID, cursor, limit*2)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user bookmarks: %w", err)
	}
	filtered := make([]*entities.Bookmark, 0, len(bookmarks))
	for _, bm := range bookmarks {
		if bm.Category == category {
			filtered = append(filtered, bm)
		}
	}
	// Trim to limit
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nextCursor, nil
}

// ======================================================================
// UpdateBookmark
// ======================================================================

// UpdateBookmark updates a bookmark's category, name, or notes.
func (s *bookmarkService) UpdateBookmark(ctx context.Context, bookmarkID, userID, category, name, notes string) (*entities.Bookmark, error) {
	// Validate category
	if category != "" && !s.isValidCategory(category) {
		return nil, ErrInvalidCategory
	}
	if category == string(CategoryCustom) && strings.TrimSpace(name) == "" {
		return nil, ErrCategoryNameRequired
	}
	if len(notes) > MaxBookmarkNotesLen {
		return nil, ErrBookmarkNotesTooLong
	}
	if len(name) > MaxBookmarkNameLen {
		return nil, ErrBookmarkNameTooLong
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Get bookmark
	bookmark, err := s.bookmarkRepo.GetByID(ctx, bookmarkID)
	if err != nil {
		if errors.Is(err, interfaces.ErrBookmarkNotFound) {
			return nil, ErrBookmarkNotFound
		}
		return nil, fmt.Errorf("failed to get bookmark: %w", err)
	}
	if bookmark.UserID != userID {
		return nil, errors.New("bookmark does not belong to user")
	}
	// Update fields
	if category != "" {
		bookmark.Category = category
	}
	if category == string(CategoryCustom) && name != "" {
		bookmark.Name = name
	}
	if notes != "" {
		bookmark.Notes = notes
	}
	bookmark.UpdatedAt = time.Now()
	if err := s.bookmarkRepo.Update(ctx, bookmark); err != nil {
		return nil, fmt.Errorf("failed to update bookmark: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateBookmarkCache(ctx, bookmark.TweetID, userID)
	s.log.WithFields(logrus.Fields{
		"user_id":     userID,
		"tweet_id":    bookmark.TweetID,
		"bookmark_id": bookmarkID,
		"category":    category,
	}).Info("Bookmark updated")
	return bookmark, nil
}

// ======================================================================
// GetBookmarkStats
// ======================================================================

// GetBookmarkStats returns bookmark statistics.
func (s *bookmarkService) GetBookmarkStats(ctx context.Context) (*dto.BookmarkStatsResponse, error) {
	stats, err := s.bookmarkRepo.GetBookmarkStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get bookmark stats: %w", err)
	}
	// Get category stats
	categoryStats := make(map[string]int64)
	// This would require a more complex query; for now, return empty
	return &dto.BookmarkStatsResponse{
		TotalBookmarks: stats.TotalBookmarks,
		UniqueUsers:    stats.UniqueUsers,
		UniqueTweets:   stats.UniqueTweets,
		CategoryStats:  categoryStats,
		LastBookmark:   stats.LastBookmark,
		FirstBookmark:  stats.FirstBookmark,
	}, nil
}

// ======================================================================
// GetUserBookmarkStats
// ======================================================================

// GetUserBookmarkStats returns bookmark statistics for a user.
func (s *bookmarkService) GetUserBookmarkStats(ctx context.Context, userID string) (*dto.BookmarkStatsResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	bookmarks, err := s.bookmarkRepo.GetByUserID(ctx, userID, "", 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get user bookmarks: %w", err)
	}
	stats := &dto.BookmarkStatsResponse{
		CategoryStats: make(map[string]int64),
	}
	for _, bm := range bookmarks {
		stats.TotalBookmarks++
		stats.CategoryStats[bm.Category]++
		if bm.HasNotes() {
			stats.WithNotes++
		}
	}
	// Unique tweets
	tweetSet := make(map[string]bool)
	for _, bm := range bookmarks {
		tweetSet[bm.TweetID] = true
	}
	stats.UniqueTweets = int64(len(tweetSet))
	if len(bookmarks) > 0 {
		stats.LastBookmark = &bookmarks[0].CreatedAt
		stats.FirstBookmark = &bookmarks[len(bookmarks)-1].CreatedAt
	}
	return stats, nil
}

// ======================================================================
// Bulk Operations
// ======================================================================

// BulkCreate adds bookmarks to multiple tweets.
func (s *bookmarkService) BulkCreate(ctx context.Context, userID string, tweetIDs []string, category string) ([]string, error) {
	if len(tweetIDs) == 0 {
		return []string{}, nil
	}
	if len(tweetIDs) > MaxBookmarksPerBatch {
		return nil, fmt.Errorf("cannot bookmark more than %d tweets at once", MaxBookmarksPerBatch)
	}
	if category != "" && !s.isValidCategory(category) {
		return nil, ErrInvalidCategory
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	bookmarked := []string{}
	for _, tweetID := range tweetIDs {
		// Check if already bookmarked
		exists, err := s.bookmarkRepo.Exists(ctx, tweetID, userID)
		if err != nil {
			continue
		}
		if exists {
			continue
		}
		// Check if tweet exists
		tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
		if err != nil || tweet.DeletedAt != nil {
			continue
		}
		// Create bookmark
		bookmark := &entities.Bookmark{
			ID:        uuid.New().String(),
			TweetID:   tweetID,
			UserID:    userID,
			Category:  category,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.bookmarkRepo.Create(ctx, bookmark); err != nil {
			continue
		}
		_ = s.invalidateBookmarkCache(ctx, tweetID, userID)
		bookmarked = append(bookmarked, tweetID)
	}
	return bookmarked, nil
}

// BulkDelete removes bookmarks from multiple tweets.
func (s *bookmarkService) BulkDelete(ctx context.Context, userID string, bookmarkIDs []string) ([]string, error) {
	if len(bookmarkIDs) == 0 {
		return []string{}, nil
	}
	if len(bookmarkIDs) > MaxBookmarksPerBatch {
		return nil, fmt.Errorf("cannot delete more than %d bookmarks at once", MaxBookmarksPerBatch)
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	deleted := []string{}
	for _, bookmarkID := range bookmarkIDs {
		bookmark, err := s.bookmarkRepo.GetByID(ctx, bookmarkID)
		if err != nil || bookmark.UserID != userID {
			continue
		}
		if err := s.bookmarkRepo.Delete(ctx, bookmarkID); err != nil {
			continue
		}
		_ = s.invalidateBookmarkCache(ctx, bookmark.TweetID, userID)
		deleted = append(deleted, bookmarkID)
	}
	return deleted, nil
}

// ======================================================================
// GetCategories
// ======================================================================

// GetCategories returns all valid bookmark categories.
func (s *bookmarkService) GetCategories() []string {
	return []string{
		string(CategoryReadLater),
		string(CategoryFavorites),
		string(CategoryImportant),
		string(CategoryWatchLater),
		string(CategoryCustom),
	}
}

// ======================================================================
// Helper Methods
// ======================================================================

// isValidCategory checks if a category is valid.
func (s *bookmarkService) isValidCategory(category string) bool {
	if category == "" {
		return true
	}
	valid := map[string]bool{
		string(CategoryReadLater):  true,
		string(CategoryFavorites):  true,
		string(CategoryImportant):  true,
		string(CategoryWatchLater): true,
		string(CategoryCustom):     true,
	}
	return valid[category]
}

// ======================================================================
// Cache Invalidation
// ======================================================================

// invalidateBookmarkCache invalidates bookmark caches.
func (s *bookmarkService) invalidateBookmarkCache(ctx context.Context, tweetID, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	keys := []string{
		fmt.Sprintf("bookmarked:%s:%s", tweetID, userID),
		fmt.Sprintf("bookmark_count:%s", tweetID),
	}
	patterns := []string{
		fmt.Sprintf("user_bookmarks:%s:*", userID),
		fmt.Sprintf("bookmark_stats:%s:*", userID),
	}
	for _, pattern := range patterns {
		iter := s.redisAdapter.Scan(ctx, 0, pattern, 100)
		var keysBatch []string
		for {
			keysBatch, nextCursor, err := iter.Next()
			if err != nil {
				break
			}
			keys = append(keys, keysBatch...)
			if nextCursor == 0 {
				break
			}
		}
	}
	if len(keys) > 0 {
		return s.redisAdapter.Delete(ctx, keys...)
	}
	return nil
}

// ======================================================================
// Global Instance
// ======================================================================

var defaultBookmarkService BookmarkService

// InitBookmarkService initializes the global bookmark service.
func InitBookmarkService(
	bookmarkRepo interfaces.BookmarkRepository,
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) {
	defaultBookmarkService = NewBookmarkService(
		bookmarkRepo,
		tweetRepo,
		userRepo,
		notificationRepo,
		redisAdapter,
	)
}

// GetBookmarkService returns the global bookmark service.
func GetBookmarkService() BookmarkService {
	if defaultBookmarkService == nil {
		panic("bookmark service not initialized")
	}
	return defaultBookmarkService
}