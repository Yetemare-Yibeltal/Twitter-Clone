// backend/internal/service/tweet_service.go
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/domain/valueobjects"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// Common tweet service errors.
var (
	ErrTweetNotFound      = errors.New("tweet not found")
	ErrTweetDeleted       = errors.New("tweet has been deleted")
	ErrUnauthorized       = errors.New("not authorized")
	ErrInvalidContent     = errors.New("invalid tweet content")
	ErrContentTooLong     = errors.New("tweet content exceeds maximum length")
	ErrEmptyContent       = errors.New("tweet content cannot be empty")
	ErrAlreadyLiked       = errors.New("already liked this tweet")
	ErrAlreadyRetweeted   = errors.New("already retweeted this tweet")
	ErrAlreadyBookmarked  = errors.New("already bookmarked this tweet")
	ErrCannotRetweetOwn   = errors.New("cannot retweet your own tweet")
	ErrCannotQuoteOwn     = errors.New("cannot quote your own tweet")
	ErrPollExpired        = errors.New("poll has expired")
	ErrPollAlreadyVoted   = errors.New("already voted on this poll")
	ErrInvalidPollOption  = errors.New("invalid poll option")
	ErrMaxMediaExceeded   = errors.New("maximum 4 media files allowed")
	ErrMediaTypeNotAllowed = errors.New("media type not allowed")
	ErrMediaSizeExceeded  = errors.New("media file size exceeds limit")
)

// TweetService defines the tweet service interface.
type TweetService interface {
	// Basic tweet operations
	CreateTweet(ctx context.Context, userID string, req *dto.CreateTweetRequest) (*dto.TweetResponse, error)
	GetTweet(ctx context.Context, tweetID string) (*dto.TweetDetailResponse, error)
	UpdateTweet(ctx context.Context, tweetID, userID string, req *dto.UpdateTweetRequest) (*dto.TweetResponse, error)
	DeleteTweet(ctx context.Context, tweetID, userID string) error
	
	// Feed operations
	GetFeed(ctx context.Context, userID, cursor string, limit int) ([]*dto.TweetResponse, string, error)
	GetUserTweets(ctx context.Context, username, cursor string, limit int, includeReplies bool) ([]*dto.TweetResponse, string, error)
	GetReplies(ctx context.Context, tweetID, cursor string, limit int) ([]*dto.TweetResponse, string, error)
	
	// Quote tweet
	QuoteTweet(ctx context.Context, tweetID, userID string, req *dto.QuoteTweetRequest) (*dto.TweetResponse, error)
	
	// Search
	SearchTweets(ctx context.Context, filters *dto.SearchFilters, cursor string, limit int) ([]*dto.TweetResponse, string, error)
	
	// Trending
	GetTrending(ctx context.Context, limit int) ([]*dto.TrendingTopic, error)
}

// tweetService implements TweetService.
type tweetService struct {
	tweetRepo     interfaces.TweetRepository
	userRepo      interfaces.UserRepository
	followRepo    interfaces.FollowRepository
	likeRepo      interfaces.LikeRepository
	retweetRepo   interfaces.RetweetRepository
	bookmarkRepo  interfaces.BookmarkRepository
	pollRepo      interfaces.PollRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter  adapter.RedisAdapter
	storageAdapter adapter.StorageAdapter
	maxContentLen int
	log           *logrus.Entry
}

// NewTweetService creates a new tweet service.
func NewTweetService(
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	likeRepo interfaces.LikeRepository,
	retweetRepo interfaces.RetweetRepository,
	bookmarkRepo interfaces.BookmarkRepository,
	pollRepo interfaces.PollRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
	storageAdapter adapter.StorageAdapter,
	maxContentLen int,
) TweetService {
	if maxContentLen == 0 {
		maxContentLen = 280
	}
	return &tweetService{
		tweetRepo:     tweetRepo,
		userRepo:      userRepo,
		followRepo:    followRepo,
		likeRepo:      likeRepo,
		retweetRepo:   retweetRepo,
		bookmarkRepo:  bookmarkRepo,
		pollRepo:      pollRepo,
		notificationRepo: notificationRepo,
		redisAdapter:  redisAdapter,
		storageAdapter: storageAdapter,
		maxContentLen: maxContentLen,
		log:           logger.WithField("service", "tweet"),
	}
}

// ======================================================================
// Create Tweet
// ======================================================================

func (s *tweetService) CreateTweet(ctx context.Context, userID string, req *dto.CreateTweetRequest) (*dto.TweetResponse, error) {
	// Validate content
	if req.Content == "" && len(req.MediaURLs) == 0 && req.Poll == nil {
		return nil, ErrEmptyContent
	}
	if len(req.Content) > s.maxContentLen {
		return nil, ErrContentTooLong
	}

	// Validate media
	if len(req.MediaURLs) > 4 {
		return nil, ErrMaxMediaExceeded
	}

	// Validate poll
	if req.Poll != nil {
		if len(req.Poll.Options) < 2 || len(req.Poll.Options) > 4 {
			return nil, errors.New("poll must have 2-4 options")
		}
		if req.Poll.Duration < 1*time.Minute || req.Poll.Duration > 7*24*time.Hour {
			return nil, errors.New("poll duration must be between 1 minute and 7 days")
		}
	}

	// Create tweet entity
	tweet := &entities.Tweet{
		ID:            uuid.New().String(),
		UserID:        userID,
		Content:       req.Content,
		MediaURLs:     req.MediaURLs,
		ParentTweetID: req.ParentTweetID,
		RetweetOfID:   nil,
		IsPoll:        req.Poll != nil,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Start transaction
	err := s.tweetRepo.Transaction(ctx, func(txRepo interfaces.TweetRepository) error {
		// Save tweet
		if err := txRepo.Create(ctx, tweet); err != nil {
			return err
		}

		// Create poll if present
		if req.Poll != nil {
			poll := &entities.Poll{
				ID:        uuid.New().String(),
				TweetID:   tweet.ID,
				Options:   req.Poll.Options,
				Duration:  req.Poll.Duration,
				ExpiresAt: time.Now().Add(req.Poll.Duration),
				CreatedAt: time.Now(),
			}
			if err := s.pollRepo.Create(ctx, poll); err != nil {
				return err
			}
		}

		// Increment user tweet count
		if err := s.userRepo.IncrementTweetCount(ctx, userID); err != nil {
			s.log.WithError(err).Warn("Failed to increment tweet count")
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create tweet: %w", err)
	}

	// Cache invalidation for feed
	if err := s.invalidateFeedCache(ctx, userID); err != nil {
		s.log.WithError(err).Warn("Failed to invalidate feed cache")
	}

	// Build response
	return s.buildTweetResponse(ctx, tweet, userID)
}

// ======================================================================
// Get Tweet
// ======================================================================

func (s *tweetService) GetTweet(ctx context.Context, tweetID string) (*dto.TweetDetailResponse, error) {
	tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, ErrTweetNotFound
		}
		return nil, err
	}

	if tweet.DeletedAt != nil {
		return nil, ErrTweetDeleted
	}

	// Get replies
	replies, _, err := s.tweetRepo.GetReplies(ctx, tweetID, "", 10)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get replies")
	}

	// Get parent tweet if exists
	var parentTweet *dto.TweetResponse
	if tweet.ParentTweetID != nil && *tweet.ParentTweetID != "" {
		parent, err := s.tweetRepo.GetByID(ctx, *tweet.ParentTweetID)
		if err == nil && parent.DeletedAt == nil {
			parentTweet, _ = s.buildTweetResponse(ctx, parent, "")
		}
	}

	// Get retweet source if exists
	var retweetSource *dto.TweetResponse
	if tweet.RetweetOfID != nil && *tweet.RetweetOfID != "" {
		source, err := s.tweetRepo.GetByID(ctx, *tweet.RetweetOfID)
		if err == nil && source.DeletedAt == nil {
			retweetSource, _ = s.buildTweetResponse(ctx, source, "")
		}
	}

	// Get poll if exists
	var poll *dto.PollResponse
	if tweet.IsPoll {
		p, err := s.pollRepo.GetByTweetID(ctx, tweetID)
		if err == nil {
			poll = s.buildPollResponse(p, "")
		}
	}

	return &dto.TweetDetailResponse{
		Tweet:        tweet,
		ParentTweet:  parentTweet,
		RetweetSource: retweetSource,
		Replies:      replies,
		Poll:         poll,
	}, nil
}

// ======================================================================
// Update Tweet
// ======================================================================

func (s *tweetService) UpdateTweet(ctx context.Context, tweetID, userID string, req *dto.UpdateTweetRequest) (*dto.TweetResponse, error) {
	tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, ErrTweetNotFound
		}
		return nil, err
	}

	if tweet.DeletedAt != nil {
		return nil, ErrTweetDeleted
	}

	if tweet.UserID != userID {
		return nil, ErrUnauthorized
	}

	// Update content
	if req.Content != "" {
		if len(req.Content) > s.maxContentLen {
			return nil, ErrContentTooLong
		}
		tweet.Content = req.Content
	}

	tweet.UpdatedAt = time.Now()

	if err := s.tweetRepo.Update(ctx, tweet); err != nil {
		return nil, fmt.Errorf("failed to update tweet: %w", err)
	}

	return s.buildTweetResponse(ctx, tweet, userID)
}

// ======================================================================
// Delete Tweet
// ======================================================================

func (s *tweetService) DeleteTweet(ctx context.Context, tweetID, userID string) error {
	tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return err
	}

	if tweet.DeletedAt != nil {
		return ErrTweetDeleted
	}

	// Check authorization
	isOwner := tweet.UserID == userID
	isAdmin, _ := s.isAdmin(ctx, userID)

	if !isOwner && !isAdmin {
		return ErrUnauthorized
	}

	// Soft delete
	if err := s.tweetRepo.SoftDelete(ctx, tweetID); err != nil {
		return fmt.Errorf("failed to delete tweet: %w", err)
	}

	// Decrement user tweet count if owner
	if isOwner {
		if err := s.userRepo.DecrementTweetCount(ctx, userID); err != nil {
			s.log.WithError(err).Warn("Failed to decrement tweet count")
		}
	}

	// Invalidate cache
	if err := s.invalidateFeedCache(ctx, tweet.UserID); err != nil {
		s.log.WithError(err).Warn("Failed to invalidate feed cache")
	}

	return nil
}

// ======================================================================
// Get Feed
// ======================================================================

func (s *tweetService) GetFeed(ctx context.Context, userID, cursor string, limit int) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Try cache first
	cacheKey := fmt.Sprintf("feed:%s:%s:%d", userID, cursor, limit)
	if s.redisAdapter != nil {
		var cached []*dto.TweetResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("user_id", userID).Debug("Feed served from cache")
			return cached, cursor, nil
		}
	}

	// Get followed user IDs
	following, err := s.followRepo.GetFollowing(ctx, userID, 0, 1000)
	if err != nil {
		return nil, "", err
	}

	followingIDs := make([]string, 0, len(following)+1)
	followingIDs = append(followingIDs, userID) // Include own tweets
	for _, f := range following {
		followingIDs = append(followingIDs, f.FolloweeID)
	}

	// Get tweets
	tweets, nextCursor, err := s.tweetRepo.GetFeed(ctx, followingIDs, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	// Build responses
	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, tweet := range tweets {
		resp, err := s.buildTweetResponse(ctx, tweet, userID)
		if err != nil {
			s.log.WithError(err).Warn("Failed to build tweet response")
			continue
		}
		responses = append(responses, resp)
	}

	// Cache for 30 seconds
	if s.redisAdapter != nil && len(responses) > 0 {
		if err := s.redisAdapter.CacheSet(ctx, cacheKey, responses, 30*time.Second); err != nil {
			s.log.WithError(err).Warn("Failed to cache feed")
		}
	}

	return responses, nextCursor, nil
}

// ======================================================================
= Get User Tweets
// ======================================================================

func (s *tweetService) GetUserTweets(ctx context.Context, username, cursor string, limit int, includeReplies bool) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", errors.New("user not found")
		}
		return nil, "", err
	}

	tweets, nextCursor, err := s.tweetRepo.GetByUserID(ctx, user.ID, cursor, limit, includeReplies)
	if err != nil {
		return nil, "", err
	}

	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, tweet := range tweets {
		resp, err := s.buildTweetResponse(ctx, tweet, "")
		if err != nil {
			s.log.WithError(err).Warn("Failed to build tweet response")
			continue
		}
		responses = append(responses, resp)
	}

	return responses, nextCursor, nil
}

// ======================================================================
= Get Replies
// ======================================================================

func (s *tweetService) GetReplies(ctx context.Context, tweetID, cursor string, limit int) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Verify tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, "", ErrTweetNotFound
		}
		return nil, "", err
	}

	tweets, nextCursor, err := s.tweetRepo.GetReplies(ctx, tweetID, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, tweet := range tweets {
		resp, err := s.buildTweetResponse(ctx, tweet, "")
		if err != nil {
			s.log.WithError(err).Warn("Failed to build tweet response")
			continue
		}
		responses = append(responses, resp)
	}

	return responses, nextCursor, nil
}

// ======================================================================
= Quote Tweet
// ======================================================================

func (s *tweetService) QuoteTweet(ctx context.Context, tweetID, userID string, req *dto.QuoteTweetRequest) (*dto.TweetResponse, error) {
	// Verify original tweet exists
	original, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, ErrTweetNotFound
		}
		return nil, err
	}

	if original.DeletedAt != nil {
		return nil, ErrTweetDeleted
	}

	if original.UserID == userID {
		return nil, ErrCannotQuoteOwn
	}

	// Validate content
	if req.Content == "" {
		return nil, ErrEmptyContent
	}
	if len(req.Content) > s.maxContentLen {
		return nil, ErrContentTooLong
	}

	// Create quote tweet
	tweet := &entities.Tweet{
		ID:            uuid.New().String(),
		UserID:        userID,
		Content:       req.Content,
		MediaURLs:     req.MediaURLs,
		ParentTweetID: nil,
		RetweetOfID:   &tweetID,
		IsPoll:        false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.tweetRepo.Create(ctx, tweet); err != nil {
		return nil, fmt.Errorf("failed to create quote tweet: %w", err)
	}

	// Increment user tweet count
	if err := s.userRepo.IncrementTweetCount(ctx, userID); err != nil {
		s.log.WithError(err).Warn("Failed to increment tweet count")
	}

	// Create notification for original tweet owner
	if err := s.createNotification(ctx, original.UserID, userID, tweetID, "quote"); err != nil {
		s.log.WithError(err).Warn("Failed to create quote notification")
	}

	return s.buildTweetResponse(ctx, tweet, userID)
}

// ======================================================================
= Search Tweets
// ======================================================================

func (s *tweetService) SearchTweets(ctx context.Context, filters *dto.SearchFilters, cursor string, limit int) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	tweets, nextCursor, err := s.tweetRepo.Search(ctx, filters.Query, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, tweet := range tweets {
		resp, err := s.buildTweetResponse(ctx, tweet, "")
		if err != nil {
			s.log.WithError(err).Warn("Failed to build tweet response")
			continue
		}
		responses = append(responses, resp)
	}

	return responses, nextCursor, nil
}

// ======================================================================
= Get Trending
// ======================================================================

func (s *tweetService) GetTrending(ctx context.Context, limit int) ([]*dto.TrendingTopic, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}

	// Try cache first
	cacheKey := "trending"
	if s.redisAdapter != nil {
		var cached []*dto.TrendingTopic
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			if len(cached) > 0 {
				return cached, nil
			}
		}
	}

	// Get trending from repository
	trends, err := s.tweetRepo.GetTrending(ctx, limit)
	if err != nil {
		return nil, err
	}

	// Cache for 5 minutes
	if s.redisAdapter != nil {
		if err := s.redisAdapter.CacheSet(ctx, cacheKey, trends, 5*time.Minute); err != nil {
			s.log.WithError(err).Warn("Failed to cache trending")
		}
	}

	return trends, nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// buildTweetResponse builds a tweet response DTO.
func (s *tweetService) buildTweetResponse(ctx context.Context, tweet *entities.Tweet, currentUserID string) (*dto.TweetResponse, error) {
	// Get user
	user, err := s.userRepo.GetByID(ctx, tweet.UserID)
	if err != nil {
		return nil, err
	}

	// Get like count
	likeCount, err := s.likeRepo.CountByTweetID(ctx, tweet.ID)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get like count")
		likeCount = 0
	}

	// Get retweet count
	retweetCount, err := s.retweetRepo.CountByTweetID(ctx, tweet.ID)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get retweet count")
		retweetCount = 0
	}

	// Get reply count
	replyCount, err := s.tweetRepo.CountReplies(ctx, tweet.ID)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get reply count")
		replyCount = 0
	}

	// Check if liked by current user
	liked := false
	if currentUserID != "" {
		liked, err = s.likeRepo.Exists(ctx, tweet.ID, currentUserID)
		if err != nil {
			s.log.WithError(err).Warn("Failed to check like status")
		}
	}

	// Check if retweeted by current user
	retweeted := false
	if currentUserID != "" {
		retweeted, err = s.retweetRepo.Exists(ctx, tweet.ID, currentUserID)
		if err != nil {
			s.log.WithError(err).Warn("Failed to check retweet status")
		}
	}

	// Check if bookmarked by current user
	bookmarked := false
	if currentUserID != "" {
		bookmarked, err = s.bookmarkRepo.Exists(ctx, tweet.ID, currentUserID)
		if err != nil {
			s.log.WithError(err).Warn("Failed to check bookmark status")
		}
	}

	// Parse mentions and hashtags
	mentions := s.extractMentions(tweet.Content)
	hashtags := s.extractHashtags(tweet.Content)

	return &dto.TweetResponse{
		ID:           tweet.ID,
		Content:      tweet.Content,
		UserID:       tweet.UserID,
		Username:     user.Username,
		FullName:     user.FullName,
		AvatarURL:    user.AvatarURL,
		MediaURLs:    tweet.MediaURLs,
		LikeCount:    likeCount,
		RetweetCount: retweetCount,
		ReplyCount:   replyCount,
		Liked:        liked,
		Retweeted:    retweeted,
		Bookmarked:   bookmarked,
		Mentions:     mentions,
		Hashtags:     hashtags,
		CreatedAt:    tweet.CreatedAt,
		UpdatedAt:    tweet.UpdatedAt,
	}, nil
}

// buildPollResponse builds a poll response DTO.
func (s *tweetService) buildPollResponse(poll *entities.Poll, currentUserID string) *dto.PollResponse {
	options := make([]dto.PollOption, 0, len(poll.Options))
	totalVotes := int64(0)
	votedOptionID := ""

	// Calculate votes
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}

	// Build options with percentages
	for _, opt := range poll.Options {
		percentage := float64(0)
		if totalVotes > 0 {
			percentage = float64(opt.Votes) / float64(totalVotes) * 100
		}
		options = append(options, dto.PollOption{
			ID:          opt.ID,
			Text:        opt.Text,
			Votes:       opt.Votes,
			Percentage:  percentage,
			IsVoted:     opt.VoterIDs != nil && contains(opt.VoterIDs, currentUserID),
		})
		if opt.VoterIDs != nil && contains(opt.VoterIDs, currentUserID) {
			votedOptionID = opt.ID
		}
	}

	return &dto.PollResponse{
		ID:            poll.ID,
		TweetID:       poll.TweetID,
		Options:       options,
		TotalVotes:    totalVotes,
		ExpiresAt:     poll.ExpiresAt,
		IsExpired:     time.Now().After(poll.ExpiresAt),
		VotedOptionID: votedOptionID,
	}
}

// extractMentions extracts @mentions from content.
func (s *tweetService) extractMentions(content string) []string {
	re := regexp.MustCompile(`@(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	mentions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			mentions = append(mentions, match[1])
		}
	}
	return mentions
}

// extractHashtags extracts #hashtags from content.
func (s *tweetService) extractHashtags(content string) []string {
	re := regexp.MustCompile(`#(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)
	hashtags := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			hashtags = append(hashtags, match[1])
		}
	}
	return hashtags
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// createNotification creates a notification.
func (s *tweetService) createNotification(ctx context.Context, userID, fromUserID, referenceID, notificationType string) error {
	notification := &entities.Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		FromUserID:  fromUserID,
		Type:        notificationType,
		ReferenceID: referenceID,
		Read:        false,
		CreatedAt:   time.Now(),
	}
	return s.notificationRepo.Create(ctx, notification)
}

// invalidateFeedCache invalidates feed cache for a user.
func (s *tweetService) invalidateFeedCache(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	// Delete all feed cache entries for this user
	pattern := fmt.Sprintf("feed:%s:*", userID)
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
		return s.redisAdapter.Delete(ctx, keys...)
	}
	return nil
}

// isAdmin checks if a user is an admin.
func (s *tweetService) isAdmin(ctx context.Context, userID string) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.Role == entities.RoleAdmin, nil
}