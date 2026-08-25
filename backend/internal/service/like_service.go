// backend/internal/service/like_service.go
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
	MaxLikesPerBatch = 100
)

var (
	ErrLikeNotFound      = errors.New("like not found")
	ErrAlreadyLiked      = errors.New("already liked this tweet")
	ErrTweetNotFound     = errors.New("tweet not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidLikeType   = errors.New("invalid like type")
	ErrCannotLikeOwn     = errors.New("cannot like your own tweet")
	ErrSuperLikeDisabled = errors.New("super likes are disabled")
	ErrLikeServiceError  = errors.New("like service error")
)

// ======================================================================
// LikeService Interface
// ======================================================================

// LikeService defines the like service interface.
type LikeService interface {
	// ToggleLike toggles a like on a tweet.
	ToggleLike(ctx context.Context, tweetID, userID string) (*dto.LikeResponse, error)
	
	// Like adds a like to a tweet.
	Like(ctx context.Context, tweetID, userID string, likeType entities.LikeType) error
	
	// Unlike removes a like from a tweet.
	Unlike(ctx context.Context, tweetID, userID string) error
	
	// IsLiked checks if a user has liked a tweet.
	IsLiked(ctx context.Context, tweetID, userID string) (bool, error)
	
	// GetLikeCount returns the number of likes for a tweet.
	GetLikeCount(ctx context.Context, tweetID string) (int64, error)
	
	// GetUserLikes returns all likes made by a user.
	GetUserLikes(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Like, string, error)
	
	// GetTweetLikes returns all likes for a tweet.
	GetTweetLikes(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Like, string, error)
	
	// GetLikers returns users who liked a tweet.
	GetLikers(ctx context.Context, tweetID string, cursor string, limit int) ([]*dto.UserResponse, string, error)
	
	// GetLikeStats returns like statistics.
	GetLikeStats(ctx context.Context) (*dto.LikeStatsResponse, error)
	
	// GetUserLikeStats returns like statistics for a user.
	GetUserLikeStats(ctx context.Context, userID string) (*dto.LikeStatsResponse, error)
	
	// BulkLike adds likes to multiple tweets.
	BulkLike(ctx context.Context, userID string, tweetIDs []string) ([]string, error)
	
	// BulkUnlike removes likes from multiple tweets.
	BulkUnlike(ctx context.Context, userID string, tweetIDs []string) ([]string, error)
}

// ======================================================================
// likeService Implementation
// ======================================================================

// likeService implements LikeService.
type likeService struct {
	likeRepo         interfaces.LikeRepository
	tweetRepo        interfaces.TweetRepository
	userRepo         interfaces.UserRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	log              *logrus.Entry
}

// NewLikeService creates a new like service.
func NewLikeService(
	likeRepo interfaces.LikeRepository,
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) LikeService {
	return &likeService{
		likeRepo:         likeRepo,
		tweetRepo:        tweetRepo,
		userRepo:         userRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		log:              logger.WithField("service", "like"),
	}
}

// ======================================================================
// Toggle Like
// ======================================================================

// ToggleLike toggles a like on a tweet.
func (s *likeService) ToggleLike(ctx context.Context, tweetID, userID string) (*dto.LikeResponse, error) {
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
	// Check if already liked
	exists, err := s.likeRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check like status: %w", err)
	}
	liked := !exists
	if exists {
		// Unlike
		if err := s.likeRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
			return nil, fmt.Errorf("failed to unlike: %w", err)
		}
		// Invalidate cache
		_ = s.invalidateLikeCache(ctx, tweetID, userID)
		s.log.WithFields(logrus.Fields{
			"user_id":  userID,
			"tweet_id": tweetID,
		}).Info("Tweet unliked")
	} else {
		// Like
		if err := s.likeRepo.Create(ctx, &entities.Like{
			ID:        uuid.New().String(),
			TweetID:   tweetID,
			UserID:    userID,
			CreatedAt: time.Now(),
		}); err != nil {
			return nil, fmt.Errorf("failed to like: %w", err)
		}
		// Create notification for tweet owner
		if tweet.UserID != userID {
			_ = s.createLikeNotification(ctx, tweet.UserID, userID, tweetID)
		}
		// Invalidate cache
		_ = s.invalidateLikeCache(ctx, tweetID, userID)
		s.log.WithFields(logrus.Fields{
			"user_id":  userID,
			"tweet_id": tweetID,
		}).Info("Tweet liked")
	}
	// Get updated count
	count, err := s.likeRepo.CountByTweetID(ctx, tweetID)
	if err != nil {
		count = 0
	}
	return &dto.LikeResponse{
		Liked:     liked,
		LikeCount: count,
	}, nil
}

// ======================================================================
// Like
// ======================================================================

// Like adds a like to a tweet.
func (s *likeService) Like(ctx context.Context, tweetID, userID string, likeType entities.LikeType) error {
	// Validate like type
	if likeType != entities.LikeTypeRegular && likeType != entities.LikeTypeSuper {
		return ErrInvalidLikeType
	}
	if likeType == entities.LikeTypeSuper {
		// Check if super likes are enabled
		// For now, allow super likes
	}
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
	// Check if already liked
	exists, err := s.likeRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check like status: %w", err)
	}
	if exists {
		return ErrAlreadyLiked
	}
	// Create like
	if err := s.likeRepo.Create(ctx, &entities.Like{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		UserID:    userID,
		Type:      likeType,
		CreatedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to create like: %w", err)
	}
	// Create notification for tweet owner
	if tweet.UserID != userID {
		_ = s.createLikeNotification(ctx, tweet.UserID, userID, tweetID)
	}
	// Invalidate cache
	_ = s.invalidateLikeCache(ctx, tweetID, userID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"tweet_id": tweetID,
		"type":     likeType,
	}).Info("Tweet liked")
	return nil
}

// ======================================================================
// Unlike
// ======================================================================

// Unlike removes a like from a tweet.
func (s *likeService) Unlike(ctx context.Context, tweetID, userID string) error {
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
	// Check if liked
	exists, err := s.likeRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check like status: %w", err)
	}
	if !exists {
		return ErrLikeNotFound
	}
	// Remove like
	if err := s.likeRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
		return fmt.Errorf("failed to unlike: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateLikeCache(ctx, tweetID, userID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"tweet_id": tweetID,
	}).Info("Tweet unliked")
	return nil
}

// ======================================================================
// IsLiked
// ======================================================================

// IsLiked checks if a user has liked a tweet.
func (s *likeService) IsLiked(ctx context.Context, tweetID, userID string) (bool, error) {
	// Try cache first
	if s.redisAdapter != nil {
		cacheKey := fmt.Sprintf("liked:%s:%s", tweetID, userID)
		var liked bool
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &liked); err == nil {
			return liked, nil
		}
	}
	liked, err := s.likeRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check like status: %w", err)
	}
	// Cache for 10 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, fmt.Sprintf("liked:%s:%s", tweetID, userID), liked, 10*time.Second)
	}
	return liked, nil
}

// ======================================================================
// GetLikeCount
// ======================================================================

// GetLikeCount returns the number of likes for a tweet.
func (s *likeService) GetLikeCount(ctx context.Context, tweetID string) (int64, error) {
	// Try cache first
	if s.redisAdapter != nil {
		cacheKey := fmt.Sprintf("like_count:%s", tweetID)
		var count int64
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &count); err == nil {
			return count, nil
		}
	}
	count, err := s.likeRepo.CountByTweetID(ctx, tweetID)
	if err != nil {
		return 0, fmt.Errorf("failed to get like count: %w", err)
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, fmt.Sprintf("like_count:%s", tweetID), count, 30*time.Second)
	}
	return count, nil
}

// ======================================================================
// GetUserLikes
// ======================================================================

// GetUserLikes returns all likes made by a user.
func (s *likeService) GetUserLikes(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Like, string, error) {
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
	likes, nextCursor, err := s.likeRepo.GetByUserID(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user likes: %w", err)
	}
	return likes, nextCursor, nil
}

// ======================================================================
// GetTweetLikes
// ======================================================================

// GetTweetLikes returns all likes for a tweet.
func (s *likeService) GetTweetLikes(ctx context.Context, tweetID string, cursor string, limit int) ([]*entities.Like, string, error) {
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
	likes, nextCursor, err := s.likeRepo.GetByTweetID(ctx, tweetID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get tweet likes: %w", err)
	}
	return likes, nextCursor, nil
}

// ======================================================================
// GetLikers
// ======================================================================

// GetLikers returns users who liked a tweet.
func (s *likeService) GetLikers(ctx context.Context, tweetID string, cursor string, limit int) ([]*dto.UserResponse, string, error) {
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
	likes, nextCursor, err := s.likeRepo.GetByTweetID(ctx, tweetID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get likes: %w", err)
	}
	responses := make([]*dto.UserResponse, 0, len(likes))
	for _, like := range likes {
		user, err := s.userRepo.GetByID(ctx, like.UserID)
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
// GetLikeStats
// ======================================================================

// GetLikeStats returns like statistics.
func (s *likeService) GetLikeStats(ctx context.Context) (*dto.LikeStatsResponse, error) {
	stats, err := s.likeRepo.GetLikeStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get like stats: %w", err)
	}
	return &dto.LikeStatsResponse{
		TotalLikes:     stats.TotalLikes,
		UniqueUsers:    stats.UniqueUsers,
		UniqueTweets:   stats.UniqueTweets,
		LikesPerUser:   stats.LikesPerUser,
		LikesPerTweet:  stats.LikesPerTweet,
		LastLike:       stats.LastLike,
		FirstLike:      stats.FirstLike,
		MostLikedTweet: stats.MostLikedTweetID,
		MostActiveUser: stats.MostActiveUserID,
	}, nil
}

// ======================================================================
// GetUserLikeStats
// ======================================================================

// GetUserLikeStats returns like statistics for a user.
func (s *likeService) GetUserLikeStats(ctx context.Context, userID string) (*dto.LikeStatsResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	stats, err := s.likeRepo.GetUserLikeStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user like stats: %w", err)
	}
	return &dto.LikeStatsResponse{
		TotalLikes:   stats.TotalLikes,
		UniqueTweets: stats.UniqueTweets,
		LastLike:     stats.LastLike,
		FirstLike:    stats.FirstLike,
	}, nil
}

// ======================================================================
// Bulk Like/Unlike
// ======================================================================

// BulkLike adds likes to multiple tweets.
func (s *likeService) BulkLike(ctx context.Context, userID string, tweetIDs []string) ([]string, error) {
	if len(tweetIDs) == 0 {
		return []string{}, nil
	}
	if len(tweetIDs) > MaxLikesPerBatch {
		return nil, fmt.Errorf("cannot like more than %d tweets at once", MaxLikesPerBatch)
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	liked := []string{}
	for _, tweetID := range tweetIDs {
		// Check if already liked
		exists, err := s.likeRepo.Exists(ctx, tweetID, userID)
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
		// Create like
		if err := s.likeRepo.Create(ctx, &entities.Like{
			ID:        uuid.New().String(),
			TweetID:   tweetID,
			UserID:    userID,
			CreatedAt: time.Now(),
		}); err != nil {
			continue
		}
		// Create notification for tweet owner
		if tweet.UserID != userID {
			_ = s.createLikeNotification(ctx, tweet.UserID, userID, tweetID)
		}
		// Invalidate cache
		_ = s.invalidateLikeCache(ctx, tweetID, userID)
		liked = append(liked, tweetID)
	}
	return liked, nil
}

// BulkUnlike removes likes from multiple tweets.
func (s *likeService) BulkUnlike(ctx context.Context, userID string, tweetIDs []string) ([]string, error) {
	if len(tweetIDs) == 0 {
		return []string{}, nil
	}
	if len(tweetIDs) > MaxLikesPerBatch {
		return nil, fmt.Errorf("cannot unlike more than %d tweets at once", MaxLikesPerBatch)
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	unliked := []string{}
	for _, tweetID := range tweetIDs {
		// Check if liked
		exists, err := s.likeRepo.Exists(ctx, tweetID, userID)
		if err != nil || !exists {
			continue
		}
		// Remove like
		if err := s.likeRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
			continue
		}
		// Invalidate cache
		_ = s.invalidateLikeCache(ctx, tweetID, userID)
		unliked = append(unliked, tweetID)
	}
	return unliked, nil
}

// ======================================================================
// Notification Helper
// ======================================================================

// createLikeNotification creates a like notification.
func (s *likeService) createLikeNotification(ctx context.Context, userID, fromUserID, tweetID string) error {
	notification := &entities.Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		FromUserID:  fromUserID,
		Type:        "like",
		ReferenceID: tweetID,
		Read:        false,
		CreatedAt:   time.Now(),
	}
	return s.notificationRepo.Create(ctx, notification)
}

// ======================================================================
= Cache Invalidation
// ======================================================================

// invalidateLikeCache invalidates like caches.
func (s *likeService) invalidateLikeCache(ctx context.Context, tweetID, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	keys := []string{
		fmt.Sprintf("liked:%s:%s", tweetID, userID),
		fmt.Sprintf("like_count:%s", tweetID),
	}
	// Also invalidate user likes list cache
	patterns := []string{
		fmt.Sprintf("user_likes:%s:*", userID),
		fmt.Sprintf("tweet_likes:%s:*", tweetID),
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