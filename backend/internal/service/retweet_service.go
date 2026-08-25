// backend/internal/service/retweet_service.go
package service

import (
	"context"
	"errors"
	"fmt"
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
	MaxRetweetsPerBatch = 100
)

var (
	ErrRetweetNotFound      = errors.New("retweet not found")
	ErrAlreadyRetweeted     = errors.New("already retweeted this tweet")
	ErrTweetNotFound        = errors.New("tweet not found")
	ErrUserNotFound         = errors.New("user not found")
	ErrCannotRetweetOwn     = errors.New("cannot retweet your own tweet")
	ErrRetweetServiceError  = errors.New("retweet service error")
)

// ======================================================================
// RetweetService Interface
// ======================================================================

// RetweetService defines the retweet service interface.
type RetweetService interface {
	// ToggleRetweet toggles a retweet on a tweet.
	ToggleRetweet(ctx context.Context, tweetID, userID string) (*dto.RetweetResponse, error)
	
	// Retweet adds a retweet to a tweet.
	Retweet(ctx context.Context, tweetID, userID string) error
	
	// Unretweet removes a retweet from a tweet.
	Unretweet(ctx context.Context, tweetID, userID string) error
	
	// IsRetweeted checks if a user has retweeted a tweet.
	IsRetweeted(ctx context.Context, tweetID, userID string) (bool, error)
	
	// GetRetweetCount returns the number of retweets for a tweet.
	GetRetweetCount(ctx context.Context, tweetID string) (int64, error)
	
	// GetUserRetweets returns all retweets made by a user.
	GetUserRetweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Retweet, string, error)
	
	// GetTweetRetweets returns all retweets for a tweet.
	GetTweetRetweets(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Retweet, string, error)
	
	// GetRetweeters returns users who retweeted a tweet.
	GetRetweeters(ctx context.Context, tweetID string, cursor string, limit int) ([]*dto.UserResponse, string, error)
	
	// GetRetweetStats returns retweet statistics.
	GetRetweetStats(ctx context.Context) (*dto.RetweetStatsResponse, error)
	
	// GetUserRetweetStats returns retweet statistics for a user.
	GetUserRetweetStats(ctx context.Context, userID string) (*dto.RetweetStatsResponse, error)
	
	// BulkRetweet adds retweets to multiple tweets.
	BulkRetweet(ctx context.Context, userID string, tweetIDs []string) ([]string, error)
	
	// BulkUnretweet removes retweets from multiple tweets.
	BulkUnretweet(ctx context.Context, userID string, tweetIDs []string) ([]string, error)
}

// ======================================================================
// retweetService Implementation
// ======================================================================

// retweetService implements RetweetService.
type retweetService struct {
	retweetRepo      interfaces.RetweetRepository
	tweetRepo        interfaces.TweetRepository
	userRepo         interfaces.UserRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	log              *logrus.Entry
}

// NewRetweetService creates a new retweet service.
func NewRetweetService(
	retweetRepo interfaces.RetweetRepository,
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) RetweetService {
	return &retweetService{
		retweetRepo:      retweetRepo,
		tweetRepo:        tweetRepo,
		userRepo:         userRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		log:              logger.WithField("service", "retweet"),
	}
}

// ======================================================================
// Toggle Retweet
// ======================================================================

// ToggleRetweet toggles a retweet on a tweet.
func (s *retweetService) ToggleRetweet(ctx context.Context, tweetID, userID string) (*dto.RetweetResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
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
	if tweet.UserID == userID {
		return nil, ErrCannotRetweetOwn
	}
	// Check if already retweeted
	exists, err := s.retweetRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check retweet status: %w", err)
	}
	retweeted := !exists
	if exists {
		// Unretweet
		if err := s.retweetRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
			return nil, fmt.Errorf("failed to unretweet: %w", err)
		}
		// Invalidate cache
		_ = s.invalidateRetweetCache(ctx, tweetID, userID)
		s.log.WithFields(logrus.Fields{
			"user_id":  userID,
			"tweet_id": tweetID,
		}).Info("Tweet unretweeted")
	} else {
		// Retweet
		if err := s.retweetRepo.Create(ctx, &entities.Retweet{
			ID:        uuid.New().String(),
			TweetID:   tweetID,
			UserID:    userID,
			CreatedAt: time.Now(),
		}); err != nil {
			return nil, fmt.Errorf("failed to retweet: %w", err)
		}
		// Create notification for tweet owner
		if tweet.UserID != userID {
			_ = s.createRetweetNotification(ctx, tweet.UserID, userID, tweetID)
		}
		// Invalidate cache
		_ = s.invalidateRetweetCache(ctx, tweetID, userID)
		s.log.WithFields(logrus.Fields{
			"user_id":  userID,
			"tweet_id": tweetID,
		}).Info("Tweet retweeted")
	}
	// Get updated count
	count, err := s.retweetRepo.CountByTweetID(ctx, tweetID)
	if err != nil {
		count = 0
	}
	return &dto.RetweetResponse{
		Retweeted:    retweeted,
		RetweetCount: count,
	}, nil
}

// ======================================================================
// Retweet
// ======================================================================

// Retweet adds a retweet to a tweet.
func (s *retweetService) Retweet(ctx context.Context, tweetID, userID string) error {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Check if tweet exists
	tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return fmt.Errorf("failed to get tweet: %w", err)
	}
	if tweet.DeletedAt != nil {
		return ErrTweetNotFound
	}
	if tweet.UserID == userID {
		return ErrCannotRetweetOwn
	}
	// Check if already retweeted
	exists, err := s.retweetRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check retweet status: %w", err)
	}
	if exists {
		return ErrAlreadyRetweeted
	}
	// Create retweet
	if err := s.retweetRepo.Create(ctx, &entities.Retweet{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to create retweet: %w", err)
	}
	// Create notification for tweet owner
	if tweet.UserID != userID {
		_ = s.createRetweetNotification(ctx, tweet.UserID, userID, tweetID)
	}
	// Invalidate cache
	_ = s.invalidateRetweetCache(ctx, tweetID, userID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"tweet_id": tweetID,
	}).Info("Tweet retweeted")
	return nil
}

// ======================================================================
// Unretweet
// ======================================================================

// Unretweet removes a retweet from a tweet.
func (s *retweetService) Unretweet(ctx context.Context, tweetID, userID string) error {
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
	// Check if retweeted
	exists, err := s.retweetRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check retweet status: %w", err)
	}
	if !exists {
		return ErrRetweetNotFound
	}
	// Remove retweet
	if err := s.retweetRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
		return fmt.Errorf("failed to unretweet: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateRetweetCache(ctx, tweetID, userID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"tweet_id": tweetID,
	}).Info("Tweet unretweeted")
	return nil
}

// ======================================================================
// IsRetweeted
// ======================================================================

// IsRetweeted checks if a user has retweeted a tweet.
func (s *retweetService) IsRetweeted(ctx context.Context, tweetID, userID string) (bool, error) {
	// Try cache first
	if s.redisAdapter != nil {
		cacheKey := fmt.Sprintf("retweeted:%s:%s", tweetID, userID)
		var retweeted bool
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &retweeted); err == nil {
			return retweeted, nil
		}
	}
	retweeted, err := s.retweetRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check retweet status: %w", err)
	}
	// Cache for 10 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, fmt.Sprintf("retweeted:%s:%s", tweetID, userID), retweeted, 10*time.Second)
	}
	return retweeted, nil
}

// ======================================================================
// GetRetweetCount
// ======================================================================

// GetRetweetCount returns the number of retweets for a tweet.
func (s *retweetService) GetRetweetCount(ctx context.Context, tweetID string) (int64, error) {
	// Try cache first
	if s.redisAdapter != nil {
		cacheKey := fmt.Sprintf("retweet_count:%s", tweetID)
		var count int64
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &count); err == nil {
			return count, nil
		}
	}
	count, err := s.retweetRepo.CountByTweetID(ctx, tweetID)
	if err != nil {
		return 0, fmt.Errorf("failed to get retweet count: %w", err)
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, fmt.Sprintf("retweet_count:%s", tweetID), count, 30*time.Second)
	}
	return count, nil
}

// ======================================================================
// GetUserRetweets
// ======================================================================

// GetUserRetweets returns all retweets made by a user.
func (s *retweetService) GetUserRetweets(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Retweet, string, error) {
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
	retweets, nextCursor, err := s.retweetRepo.GetByUserID(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user retweets: %w", err)
	}
	return retweets, nextCursor, nil
}

// ======================================================================
// GetTweetRetweets
// ======================================================================

// GetTweetRetweets returns all retweets for a tweet.
func (s *retweetService) GetTweetRetweets(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Retweet, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, "", ErrTweetNotFound
		}
		return nil, "", fmt.Errorf("failed to get tweet: %w", err)
	}
	retweets, nextCursor, err := s.retweetRepo.GetByTweetID(ctx, tweetID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get tweet retweets: %w", err)
	}
	return retweets, nextCursor, nil
}

// ======================================================================
// GetRetweeters
// ======================================================================

// GetRetweeters returns users who retweeted a tweet.
func (s *retweetService) GetRetweeters(ctx context.Context, tweetID string, cursor string, limit int) ([]*dto.UserResponse, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, "", ErrTweetNotFound
		}
		return nil, "", fmt.Errorf("failed to get tweet: %w", err)
	}
	retweets, nextCursor, err := s.retweetRepo.GetByTweetID(ctx, tweetID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get retweets: %w", err)
	}
	responses := make([]*dto.UserResponse, 0, len(retweets))
	for _, retweet := range retweets {
		user, err := s.userRepo.GetByID(ctx, retweet.UserID)
		if err != nil {
			continue
		}
		resp := dto.NewUserResponse().
			WithID(user.ID).
			WithUsername(user.Username).
			WithFullName(user.FullName).
			WithAvatarURL(user.AvatarURL).
			WithVerified(user.IsVerified)
		responses = append(responses, resp)
	}
	return responses, nextCursor, nil
}

// ======================================================================
// GetRetweetStats
// ======================================================================

// GetRetweetStats returns retweet statistics.
func (s *retweetService) GetRetweetStats(ctx context.Context) (*dto.RetweetStatsResponse, error) {
	stats, err := s.retweetRepo.GetRetweetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get retweet stats: %w", err)
	}
	return &dto.RetweetStatsResponse{
		TotalRetweets:   stats.TotalRetweets,
		UniqueUsers:     stats.UniqueUsers,
		UniqueTweets:    stats.UniqueTweets,
		RetweetsPerUser: stats.RetweetsPerUser,
		RetweetsPerTweet: stats.RetweetsPerTweet,
		LastRetweet:     stats.LastRetweet,
		FirstRetweet:    stats.FirstRetweet,
		MostRetweetedTweet: stats.MostRetweetedTweetID,
		MostActiveUser:  stats.MostActiveRetweeterID,
	}, nil
}

// ======================================================================
// GetUserRetweetStats
// ======================================================================

// GetUserRetweetStats returns retweet statistics for a user.
func (s *retweetService) GetUserRetweetStats(ctx context.Context, userID string) (*dto.RetweetStatsResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	stats, err := s.retweetRepo.GetUserRetweetStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user retweet stats: %w", err)
	}
	return &dto.RetweetStatsResponse{
		TotalRetweets: stats.TotalRetweets,
		UniqueTweets:  stats.UniqueTweets,
		LastRetweet:   stats.LastRetweet,
		FirstRetweet:  stats.FirstRetweet,
	}, nil
}

// ======================================================================
// Bulk Retweet/Unretweet
// ======================================================================

// BulkRetweet adds retweets to multiple tweets.
func (s *retweetService) BulkRetweet(ctx context.Context, userID string, tweetIDs []string) ([]string, error) {
	if len(tweetIDs) == 0 {
		return []string{}, nil
	}
	if len(tweetIDs) > MaxRetweetsPerBatch {
		return nil, fmt.Errorf("cannot retweet more than %d tweets at once", MaxRetweetsPerBatch)
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	retweeted := []string{}
	for _, tweetID := range tweetIDs {
		// Check if already retweeted
		exists, err := s.retweetRepo.Exists(ctx, tweetID, userID)
		if err != nil {
			continue
		}
		if exists {
			continue
		}
		// Check if tweet exists
		tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
		if err != nil || tweet.DeletedAt != nil || tweet.UserID == userID {
			continue
		}
		// Create retweet
		if err := s.retweetRepo.Create(ctx, &entities.Retweet{
			ID:        uuid.New().String(),
			TweetID:   tweetID,
			UserID:    userID,
			CreatedAt: time.Now(),
		}); err != nil {
			continue
		}
		// Create notification for tweet owner
		if tweet.UserID != userID {
			_ = s.createRetweetNotification(ctx, tweet.UserID, userID, tweetID)
		}
		// Invalidate cache
		_ = s.invalidateRetweetCache(ctx, tweetID, userID)
		retweeted = append(retweeted, tweetID)
	}
	return retweeted, nil
}

// BulkUnretweet removes retweets from multiple tweets.
func (s *retweetService) BulkUnretweet(ctx context.Context, userID string, tweetIDs []string) ([]string, error) {
	if len(tweetIDs) == 0 {
		return []string{}, nil
	}
	if len(tweetIDs) > MaxRetweetsPerBatch {
		return nil, fmt.Errorf("cannot unretweet more than %d tweets at once", MaxRetweetsPerBatch)
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	unretweeted := []string{}
	for _, tweetID := range tweetIDs {
		// Check if retweeted
		exists, err := s.retweetRepo.Exists(ctx, tweetID, userID)
		if err != nil || !exists {
			continue
		}
		// Remove retweet
		if err := s.retweetRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
			continue
		}
		// Invalidate cache
		_ = s.invalidateRetweetCache(ctx, tweetID, userID)
		unretweeted = append(unretweeted, tweetID)
	}
	return unretweeted, nil
}

// ======================================================================
// Notification Helper
// ======================================================================

// createRetweetNotification creates a retweet notification.
func (s *retweetService) createRetweetNotification(ctx context.Context, userID, fromUserID, tweetID string) error {
	notification := &entities.Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		FromUserID:  fromUserID,
		Type:        "retweet",
		ReferenceID: tweetID,
		Read:        false,
		CreatedAt:   time.Now(),
	}
	return s.notificationRepo.Create(ctx, notification)
}

// ======================================================================
= Cache Invalidation
// ======================================================================

// invalidateRetweetCache invalidates retweet caches.
func (s *retweetService) invalidateRetweetCache(ctx context.Context, tweetID, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	keys := []string{
		fmt.Sprintf("retweeted:%s:%s", tweetID, userID),
		fmt.Sprintf("retweet_count:%s", tweetID),
	}
	// Also invalidate user retweets list cache
	patterns := []string{
		fmt.Sprintf("user_retweets:%s:*", userID),
		fmt.Sprintf("tweet_retweets:%s:*", tweetID),
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