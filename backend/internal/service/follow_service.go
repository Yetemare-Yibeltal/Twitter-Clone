// backend/internal/service/follow_service.go
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

var (
	ErrAlreadyFollowing     = errors.New("already following this user")
	ErrNotFollowing         = errors.New("not following this user")
	ErrCannotFollowSelf     = errors.New("cannot follow yourself")
	ErrUserNotFound         = errors.New("user not found")
	ErrFollowNotFound       = errors.New("follow relationship not found")
	ErrUserSuspended        = errors.New("user is suspended")
	ErrUserInactive         = errors.New("user is inactive")
	ErrInvalidFollowRequest = errors.New("invalid follow request")
)

// ======================================================================
// FollowService Interface
// ======================================================================

// FollowService defines the follow service interface.
type FollowService interface {
	// Follow creates a follow relationship.
	Follow(ctx context.Context, followerID, followeeID string) (*dto.FollowResponse, error)
	
	// Unfollow removes a follow relationship.
	Unfollow(ctx context.Context, followerID, followeeID string) (*dto.FollowResponse, error)
	
	// GetFollowers returns the list of followers for a user.
	GetFollowers(ctx context.Context, userID, cursor string, limit int, currentUserID string) ([]*dto.FollowerResponse, string, int64, error)
	
	// GetFollowing returns the list of users a user is following.
	GetFollowing(ctx context.Context, userID, cursor string, limit int, currentUserID string) ([]*dto.FollowerResponse, string, int64, error)
	
	// GetMutualFollows returns mutual follows between two users.
	GetMutualFollows(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]*dto.FollowerResponse, string, int64, error)
	
	// CheckFollowStatus checks if user1 follows user2.
	CheckFollowStatus(ctx context.Context, userID1, userID2 string) (bool, bool, error)
	
	// GetFollowCounts returns follower and following counts.
	GetFollowCounts(ctx context.Context, userID string) (*dto.FollowCountsResponse, error)
	
	// GetSuggestions returns suggested users to follow.
	GetSuggestions(ctx context.Context, userID string, limit int) ([]*dto.SuggestionResponse, error)
	
	// GetFollowStats returns follow statistics.
	GetFollowStats(ctx context.Context) (*dto.FollowStatsResponse, error)
	
	// IsFollowing checks if followerID is following followeeID.
	IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error)
	
	// GetFollowerIDs returns all follower IDs for a user.
	GetFollowerIDs(ctx context.Context, userID string) ([]string, error)
	
	// GetFollowingIDs returns all following IDs for a user.
	GetFollowingIDs(ctx context.Context, userID string) ([]string, error)
}

// ======================================================================
// followService Implementation
// ======================================================================

// followService implements FollowService.
type followService struct {
	followRepo       interfaces.FollowRepository
	userRepo         interfaces.UserRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	wsHub            *adapter.WebSocketHub
	log              *logrus.Entry
}

// NewFollowService creates a new follow service.
func NewFollowService(
	followRepo interfaces.FollowRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
	wsHub *adapter.WebSocketHub,
) FollowService {
	return &followService{
		followRepo:       followRepo,
		userRepo:         userRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		wsHub:            wsHub,
		log:              logger.WithField("service", "follow"),
	}
}

// ======================================================================
// Follow
// ======================================================================

// Follow creates a follow relationship.
func (s *followService) Follow(ctx context.Context, followerID, followeeID string) (*dto.FollowResponse, error) {
	// Validate
	if followerID == followeeID {
		return nil, ErrCannotFollowSelf
	}
	// Check if users exist
	follower, err := s.userRepo.GetByID(ctx, followerID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get follower: %w", err)
	}
	followee, err := s.userRepo.GetByID(ctx, followeeID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get followee: %w", err)
	}
	// Check user status
	if followee.IsSuspended() {
		return nil, ErrUserSuspended
	}
	if followee.IsInactive() {
		return nil, ErrUserInactive
	}
	// Check if already following
	exists, err := s.followRepo.Exists(ctx, followerID, followeeID)
	if err != nil {
		return nil, fmt.Errorf("failed to check follow existence: %w", err)
	}
	if exists {
		return nil, ErrAlreadyFollowing
	}
	// Create follow
	follow := &entities.Follow{
		FollowerID: followerID,
		FolloweeID: followeeID,
		CreatedAt:  time.Now(),
	}
	if err := s.followRepo.Create(ctx, follow); err != nil {
		return nil, fmt.Errorf("failed to create follow: %w", err)
	}
	// Update user counts
	if err := s.userRepo.IncrementFollowingCount(ctx, followerID); err != nil {
		s.log.WithError(err).Warn("Failed to increment following count")
	}
	if err := s.userRepo.IncrementFollowerCount(ctx, followeeID); err != nil {
		s.log.WithError(err).Warn("Failed to increment follower count")
	}
	// Invalidate caches
	_ = s.invalidateFollowCache(ctx, followerID)
	_ = s.invalidateFollowCache(ctx, followeeID)
	// Create notification
	if err := s.createFollowNotification(ctx, followeeID, followerID); err != nil {
		s.log.WithError(err).Warn("Failed to create follow notification")
	}
	// Send WebSocket notification
	if s.wsHub != nil {
		s.wsHub.SendFollow(followeeID, followerID, true)
	}
	s.log.WithFields(logrus.Fields{
		"follower_id": followerID,
		"followee_id": followeeID,
	}).Info("User followed")
	return &dto.FollowResponse{
		Following:        true,
		FollowerID:       followerID,
		FolloweeID:       followeeID,
		FollowerUsername: follower.Username,
		FolloweeUsername: followee.Username,
	}, nil
}

// ======================================================================
// Unfollow
// ======================================================================

// Unfollow removes a follow relationship.
func (s *followService) Unfollow(ctx context.Context, followerID, followeeID string) (*dto.FollowResponse, error) {
	// Validate
	if followerID == followeeID {
		return nil, ErrCannotFollowSelf
	}
	// Check if users exist
	follower, err := s.userRepo.GetByID(ctx, followerID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get follower: %w", err)
	}
	followee, err := s.userRepo.GetByID(ctx, followeeID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get followee: %w", err)
	}
	// Check if following
	exists, err := s.followRepo.Exists(ctx, followerID, followeeID)
	if err != nil {
		return nil, fmt.Errorf("failed to check follow existence: %w", err)
	}
	if !exists {
		return nil, ErrNotFollowing
	}
	// Remove follow
	if err := s.followRepo.Delete(ctx, followerID, followeeID); err != nil {
		if errors.Is(err, interfaces.ErrFollowNotFound) {
			return nil, ErrNotFollowing
		}
		return nil, fmt.Errorf("failed to delete follow: %w", err)
	}
	// Update user counts
	if err := s.userRepo.DecrementFollowingCount(ctx, followerID); err != nil {
		s.log.WithError(err).Warn("Failed to decrement following count")
	}
	if err := s.userRepo.DecrementFollowerCount(ctx, followeeID); err != nil {
		s.log.WithError(err).Warn("Failed to decrement follower count")
	}
	// Invalidate caches
	_ = s.invalidateFollowCache(ctx, followerID)
	_ = s.invalidateFollowCache(ctx, followeeID)
	// Send WebSocket notification
	if s.wsHub != nil {
		s.wsHub.SendFollow(followeeID, followerID, false)
	}
	s.log.WithFields(logrus.Fields{
		"follower_id": followerID,
		"followee_id": followeeID,
	}).Info("User unfollowed")
	return &dto.FollowResponse{
		Following:        false,
		FollowerID:       followerID,
		FolloweeID:       followeeID,
		FollowerUsername: follower.Username,
		FolloweeUsername: followee.Username,
	}, nil
}

// ======================================================================
// Get Followers
// ======================================================================

// GetFollowers returns the list of followers for a user.
func (s *followService) GetFollowers(ctx context.Context, userID, cursor string, limit int, currentUserID string) ([]*dto.FollowerResponse, string, int64, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", 0, ErrUserNotFound
		}
		return nil, "", 0, fmt.Errorf("failed to get user: %w", err)
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Try cache first
	cacheKey := fmt.Sprintf("followers:%s:%s:%d", userID, cursor, limit)
	if s.redisAdapter != nil {
		var cached []*dto.FollowerResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("user_id", userID).Debug("Followers served from cache")
			return cached, cursor, 0, nil
		}
	}
	// Get followers from repository
	follows, nextCursor, err := s.followRepo.GetFollowers(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get followers: %w", err)
	}
	// Get total count
	total, err := s.followRepo.CountFollowers(ctx, userID)
	if err != nil {
		total = 0
	}
	// Build response
	responses := make([]*dto.FollowerResponse, 0, len(follows))
	for _, f := range follows {
		user, err := s.userRepo.GetByID(ctx, f.FollowerID)
		if err != nil {
			continue
		}
		// Check if current user follows this follower
		isFollowing := false
		if currentUserID != "" && currentUserID != userID {
			isFollowing, _ = s.followRepo.Exists(ctx, currentUserID, f.FollowerID)
		}
		// Check if mutual
		isMutual := false
		if currentUserID != "" {
			mutual, _ := s.followRepo.AreMutual(ctx, currentUserID, f.FollowerID)
			isMutual = mutual
		}
		responses = append(responses, &dto.FollowerResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			IsFollowing: isFollowing,
			IsMutual:    isMutual,
			FollowedAt:  f.CreatedAt,
		})
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil && len(responses) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, responses, 30*time.Second)
	}
	return responses, nextCursor, total, nil
}

// ======================================================================
// Get Following
// ======================================================================

// GetFollowing returns the list of users a user is following.
func (s *followService) GetFollowing(ctx context.Context, userID, cursor string, limit int, currentUserID string) ([]*dto.FollowerResponse, string, int64, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", 0, ErrUserNotFound
		}
		return nil, "", 0, fmt.Errorf("failed to get user: %w", err)
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Try cache first
	cacheKey := fmt.Sprintf("following:%s:%s:%d", userID, cursor, limit)
	if s.redisAdapter != nil {
		var cached []*dto.FollowerResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("user_id", userID).Debug("Following served from cache")
			return cached, cursor, 0, nil
		}
	}
	// Get following from repository
	follows, nextCursor, err := s.followRepo.GetFollowing(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get following: %w", err)
	}
	// Get total count
	total, err := s.followRepo.CountFollowing(ctx, userID)
	if err != nil {
		total = 0
	}
	// Build response
	responses := make([]*dto.FollowerResponse, 0, len(follows))
	for _, f := range follows {
		user, err := s.userRepo.GetByID(ctx, f.FolloweeID)
		if err != nil {
			continue
		}
		// Check if current user follows this user
		isFollowing := false
		if currentUserID != "" && currentUserID != userID {
			isFollowing, _ = s.followRepo.Exists(ctx, currentUserID, f.FolloweeID)
		}
		// Check if mutual
		isMutual := false
		if currentUserID != "" {
			mutual, _ := s.followRepo.AreMutual(ctx, currentUserID, f.FolloweeID)
			isMutual = mutual
		}
		responses = append(responses, &dto.FollowerResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			IsFollowing: isFollowing,
			IsMutual:    isMutual,
			FollowedAt:  f.CreatedAt,
		})
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil && len(responses) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, responses, 30*time.Second)
	}
	return responses, nextCursor, total, nil
}

// ======================================================================
// Get Mutual Follows
// ======================================================================

// GetMutualFollows returns mutual follows between two users.
func (s *followService) GetMutualFollows(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]*dto.FollowerResponse, string, int64, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check users exist
	_, err := s.userRepo.GetByID(ctx, userID1)
	if err != nil {
		return nil, "", 0, ErrUserNotFound
	}
	_, err = s.userRepo.GetByID(ctx, userID2)
	if err != nil {
		return nil, "", 0, ErrUserNotFound
	}
	// Get mutual follow IDs
	ids, nextCursor, err := s.followRepo.GetMutualFollows(ctx, userID1, userID2, cursor, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get mutual follows: %w", err)
	}
	// Get total count
	total, err := s.followRepo.CountMutual(ctx, userID1, userID2)
	if err != nil {
		total = int64(len(ids))
	}
	// Build responses
	responses := make([]*dto.FollowerResponse, 0, len(ids))
	for _, id := range ids {
		user, err := s.userRepo.GetByID(ctx, id)
		if err != nil {
			continue
		}
		responses = append(responses, &dto.FollowerResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			IsFollowing: true,
			IsMutual:    true,
			FollowedAt:  time.Now(),
		})
	}
	return responses, nextCursor, total, nil
}

// ======================================================================
// Check Follow Status
// ======================================================================

// CheckFollowStatus checks if user1 follows user2.
func (s *followService) CheckFollowStatus(ctx context.Context, userID1, userID2 string) (bool, bool, error) {
	if userID1 == userID2 {
		return false, false, nil
	}
	// Check if user1 follows user2
	isFollowing, err := s.followRepo.Exists(ctx, userID1, userID2)
	if err != nil {
		return false, false, fmt.Errorf("failed to check follow: %w", err)
	}
	// Check if mutual
	isMutual := false
	if isFollowing {
		mutual, err := s.followRepo.AreMutual(ctx, userID1, userID2)
		if err != nil {
			return isFollowing, false, nil
		}
		isMutual = mutual
	}
	return isFollowing, isMutual, nil
}

// ======================================================================
// Get Follow Counts
// ======================================================================

// GetFollowCounts returns follower and following counts.
func (s *followService) GetFollowCounts(ctx context.Context, userID string) (*dto.FollowCountsResponse, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("follow_counts:%s", userID)
	if s.redisAdapter != nil {
		var cached dto.FollowCountsResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	followers, err := s.followRepo.CountFollowers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count followers: %w", err)
	}
	following, err := s.followRepo.CountFollowing(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count following: %w", err)
	}
	response := &dto.FollowCountsResponse{
		UserID:    userID,
		Followers: followers,
		Following: following,
	}
	// Cache for 1 minute
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, response, 1*time.Minute)
	}
	return response, nil
}

// ======================================================================
// Get Suggestions
// ======================================================================

// GetSuggestions returns suggested users to follow.
func (s *followService) GetSuggestions(ctx context.Context, userID string, limit int) ([]*dto.SuggestionResponse, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	// Try cache first
	cacheKey := fmt.Sprintf("suggestions:%s:%d", userID, limit)
	if s.redisAdapter != nil {
		var cached []*dto.SuggestionResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}
	// Get recommendations from repository
	ids, err := s.followRepo.GetFollowRecommendations(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %w", err)
	}
	// Build responses
	responses := make([]*dto.SuggestionResponse, 0, len(ids))
	for _, id := range ids {
		user, err := s.userRepo.GetByID(ctx, id)
		if err != nil {
			continue
		}
		// Get mutual count
		mutualCount, _ := s.followRepo.CountMutual(ctx, userID, id)
		responses = append(responses, &dto.SuggestionResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			MutualCount: mutualCount,
		})
	}
	// Cache for 5 minutes
	if s.redisAdapter != nil && len(responses) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, responses, 5*time.Minute)
	}
	return responses, nil
}

// ======================================================================
// Get Follow Stats
// ======================================================================

// GetFollowStats returns follow statistics.
func (s *followService) GetFollowStats(ctx context.Context) (*dto.FollowStatsResponse, error) {
	stats, err := s.followRepo.GetFollowStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get follow stats: %w", err)
	}
	return &dto.FollowStatsResponse{
		TotalFollows:    stats.TotalFollows,
		UniqueFollowers: stats.UniqueFollowers,
		UniqueFollowees: stats.UniqueFollowees,
		LastFollow:      stats.LastFollow,
		FirstFollow:     stats.FirstFollow,
	}, nil
}

// ======================================================================
// IsFollowing
// ======================================================================

// IsFollowing checks if followerID is following followeeID.
func (s *followService) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	return s.followRepo.Exists(ctx, followerID, followeeID)
}

// ======================================================================
// Get Follower IDs
// ======================================================================

// GetFollowerIDs returns all follower IDs for a user.
func (s *followService) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	return s.followRepo.GetFollowerIDs(ctx, userID)
}

// ======================================================================
// Get Following IDs
// ======================================================================

// GetFollowingIDs returns all following IDs for a user.
func (s *followService) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	return s.followRepo.GetFollowingIDs(ctx, userID)
}

// ======================================================================
// Cache Invalidation
// ======================================================================

// invalidateFollowCache invalidates follow-related caches for a user.
func (s *followService) invalidateFollowCache(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	patterns := []string{
		fmt.Sprintf("followers:%s:*", userID),
		fmt.Sprintf("following:%s:*", userID),
		fmt.Sprintf("follow_counts:%s", userID),
		fmt.Sprintf("suggestions:%s:*", userID),
	}
	for _, pattern := range patterns {
		iter := s.redisAdapter.Scan(ctx, 0, pattern, 100)
		var keys []string
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
		if len(keys) > 0 {
			_ = s.redisAdapter.Delete(ctx, keys...)
		}
	}
	return nil
}

// ======================================================================
// Notification Helper
// ======================================================================

// createFollowNotification creates a follow notification.
func (s *followService) createFollowNotification(ctx context.Context, followeeID, followerID string) error {
	notification := &entities.Notification{
		ID:          uuid.New().String(),
		UserID:      followeeID,
		FromUserID:  followerID,
		Type:        "follow",
		ReferenceID: followerID,
		Read:        false,
		CreatedAt:   time.Now(),
	}
	return s.notificationRepo.Create(ctx, notification)
}

// ======================================================================
// Global Instance
// ======================================================================

var defaultFollowService FollowService

// InitFollowService initializes the global follow service.
func InitFollowService(
	followRepo interfaces.FollowRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
	wsHub *adapter.WebSocketHub,
) {
	defaultFollowService = NewFollowService(
		followRepo,
		userRepo,
		notificationRepo,
		redisAdapter,
		wsHub,
	)
}

// GetFollowService returns the global follow service.
func GetFollowService() FollowService {
	if defaultFollowService == nil {
		panic("follow service not initialized")
	}
	return defaultFollowService
}