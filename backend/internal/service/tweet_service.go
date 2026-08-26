// backend/internal/service/tweet_service.go
package service

import (
	"context"
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
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	MaxTweetContentLength = 280
	MaxMediaCount         = 4
	MaxPollOptions        = 4
	MinPollOptions        = 2
	MaxTrendingLimit      = 50
	DefaultFeedLimit      = 20
	MaxFeedLimit          = 100
	DefaultTweetLimit     = 20
)

var (
	ErrTweetNotFound       = errors.New("tweet not found")
	ErrTweetDeleted        = errors.New("tweet has been deleted")
	ErrUnauthorized        = errors.New("not authorized to perform this action")
	ErrInvalidContent      = errors.New("invalid tweet content")
	ErrContentTooLong      = fmt.Errorf("tweet content exceeds maximum length of %d characters", MaxTweetContentLength)
	ErrEmptyContent        = errors.New("tweet content cannot be empty")
	ErrAlreadyLiked        = errors.New("already liked this tweet")
	ErrAlreadyRetweeted    = errors.New("already retweeted this tweet")
	ErrAlreadyBookmarked   = errors.New("already bookmarked this tweet")
	ErrCannotRetweetOwn    = errors.New("cannot retweet your own tweet")
	ErrCannotQuoteOwn      = errors.New("cannot quote your own tweet")
	ErrPollExpired         = errors.New("poll has expired")
	ErrPollAlreadyVoted    = errors.New("already voted on this poll")
	ErrInvalidPollOption   = errors.New("invalid poll option")
	ErrMaxMediaExceeded    = errors.New("maximum 4 media files allowed")
	ErrMediaTypeNotAllowed = errors.New("media type not allowed")
	ErrMediaSizeExceeded   = errors.New("media file size exceeds limit")
	ErrInvalidPollDuration = errors.New("poll duration must be between 1 minute and 7 days")
	ErrUserNotFound        = errors.New("user not found")
	ErrUserSuspended       = errors.New("user is suspended")
	ErrUserInactive        = errors.New("user is inactive")
	ErrFeedEmpty           = errors.New("feed is empty")
	ErrInvalidCursor       = errors.New("invalid cursor")
)

// ======================================================================
// TweetService Interface
// ======================================================================

// TweetService defines the tweet service interface.
type TweetService interface {
	// Basic tweet operations
	CreateTweet(ctx context.Context, userID string, req *dto.CreateTweetRequest) (*dto.TweetResponse, error)
	GetTweet(ctx context.Context, tweetID string) (*dto.TweetDetailResponse, error)
	UpdateTweet(ctx context.Context, tweetID, userID string, req *dto.UpdateTweetRequest) (*dto.TweetResponse, error)
	DeleteTweet(ctx context.Context, tweetID, userID string) error
	GetTweetByID(ctx context.Context, tweetID string) (*entities.Tweet, error)
	
	// Feed operations
	GetFeed(ctx context.Context, userID, cursor string, limit int) ([]*dto.TweetResponse, string, error)
	GetUserTweets(ctx context.Context, userID, cursor string, limit int, includeReplies bool) ([]*dto.TweetResponse, string, error)
	GetUserTweetsWithStats(ctx context.Context, userID, cursor string, limit int, includeReplies bool, currentUserID string) ([]*dto.TweetResponse, string, int64, error)
	GetReplies(ctx context.Context, tweetID, cursor string, limit int) ([]*dto.TweetResponse, string, error)
	
	// Interactions
	LikeTweet(ctx context.Context, tweetID, userID string) error
	UnlikeTweet(ctx context.Context, tweetID, userID string) error
	RetweetTweet(ctx context.Context, tweetID, userID string) error
	UnretweetTweet(ctx context.Context, tweetID, userID string) error
	BookmarkTweet(ctx context.Context, tweetID, userID string) error
	UnbookmarkTweet(ctx context.Context, tweetID, userID string) error
	QuoteTweet(ctx context.Context, tweetID, userID string, req *dto.QuoteTweetRequest) (*dto.TweetResponse, error)
	
	// Poll operations
	VotePoll(ctx context.Context, pollID, userID, optionID string) (*dto.PollResult, error)
	GetPollResults(ctx context.Context, pollID string) (*dto.PollResult, error)
	
	// Trending
	GetTrending(ctx context.Context, limit int) ([]*dto.TrendingTopic, error)
}

// ======================================================================
// tweetService Implementation
// ======================================================================

// tweetService implements TweetService.
type tweetService struct {
	tweetRepo       interfaces.TweetRepository
	userRepo        interfaces.UserRepository
	followRepo      interfaces.FollowRepository
	likeRepo        interfaces.LikeRepository
	retweetRepo     interfaces.RetweetRepository
	bookmarkRepo    interfaces.BookmarkRepository
	pollRepo        interfaces.PollRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter    adapter.RedisAdapter
	wsHub           *adapter.WebSocketHub
	log             *logrus.Entry
	maxContentLen   int
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
	wsHub *adapter.WebSocketHub,
	maxContentLen int,
) TweetService {
	if maxContentLen == 0 {
		maxContentLen = MaxTweetContentLength
	}
	return &tweetService{
		tweetRepo:       tweetRepo,
		userRepo:        userRepo,
		followRepo:      followRepo,
		likeRepo:        likeRepo,
		retweetRepo:     retweetRepo,
		bookmarkRepo:    bookmarkRepo,
		pollRepo:        pollRepo,
		notificationRepo: notificationRepo,
		redisAdapter:    redisAdapter,
		wsHub:           wsHub,
		log:             logger.WithField("service", "tweet"),
		maxContentLen:   maxContentLen,
	}
}

// ======================================================================
// Create Tweet
// ======================================================================

// CreateTweet creates a new tweet.
func (s *tweetService) CreateTweet(ctx context.Context, userID string, req *dto.CreateTweetRequest) (*dto.TweetResponse, error) {
	// Validate content
	content := strings.TrimSpace(req.Content)
	if content == "" && len(req.MediaURLs) == 0 && req.Poll == nil {
		return nil, ErrEmptyContent
	}
	if len(content) > s.maxContentLen {
		return nil, ErrContentTooLong
	}
	if len(req.MediaURLs) > MaxMediaCount {
		return nil, ErrMaxMediaExceeded
	}
	// Validate poll
	if req.Poll != nil {
		if len(req.Poll.Options) < MinPollOptions || len(req.Poll.Options) > MaxPollOptions {
			return nil, ErrInvalidPollOption
		}
		if req.Poll.Duration < 1*time.Minute || req.Poll.Duration > 7*24*time.Hour {
			return nil, ErrInvalidPollDuration
		}
	}
	// Get user for username
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user.IsSuspended() {
		return nil, ErrUserSuspended
	}
	if user.IsInactive() {
		return nil, ErrUserInactive
	}
	// Create tweet entity
	tweet := &entities.Tweet{
		ID:            uuid.New().String(),
		UserID:        userID,
		Content:       content,
		MediaURLs:     req.MediaURLs,
		ParentTweetID: req.ParentTweetID,
		RetweetOfID:   nil,
		IsPoll:        req.Poll != nil,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	// Start transaction
	err = s.tweetRepo.Transaction(ctx, func(txRepo interfaces.TweetRepository) error {
		// Save tweet
		if err := txRepo.Create(ctx, tweet); err != nil {
			return err
		}
		// Create poll if present
		if req.Poll != nil {
			poll := &entities.Poll{
				ID:        uuid.New().String(),
				TweetID:   tweet.ID,
				Options:   make([]entities.PollOption, 0, len(req.Poll.Options)),
				Duration:  req.Poll.Duration,
				ExpiresAt: time.Now().Add(req.Poll.Duration),
				CreatedAt: time.Now(),
			}
			for _, optText := range req.Poll.Options {
				poll.Options = append(poll.Options, entities.PollOption{
					ID:       uuid.New().String(),
					Text:     optText,
					Votes:    0,
					VoterIDs: []string{},
				})
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
	// Extract mentions and create notifications
	mentions := s.extractMentions(content)
	for _, mention := range mentions {
		mentionedUser, err := s.userRepo.GetByUsername(ctx, mention)
		if err != nil {
			continue
		}
		if mentionedUser.ID != userID {
			_ = s.createNotification(ctx, mentionedUser.ID, userID, tweet.ID, "mention")
		}
	}
	// If this is a reply, notify parent tweet owner
	if req.ParentTweetID != nil && *req.ParentTweetID != "" {
		parentTweet, err := s.tweetRepo.GetByID(ctx, *req.ParentTweetID)
		if err == nil && parentTweet.UserID != userID && parentTweet.DeletedAt == nil {
			_ = s.createNotification(ctx, parentTweet.UserID, userID, tweet.ID, "reply")
		}
	}
	// Invalidate feed cache for followers
	_ = s.invalidateFeedCacheForFollowers(ctx, userID)
	// Broadcast to followers via WebSocket
	if s.wsHub != nil {
		response, _ := s.buildTweetResponse(ctx, tweet, userID)
		if response != nil {
			s.wsHub.SendNewTweet(userID, response)
		}
	}
	// Build response
	return s.buildTweetResponse(ctx, tweet, userID)
}

// ======================================================================
// Get Tweet
// ======================================================================

// GetTweet retrieves a tweet with details.
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
		replies = []*entities.Tweet{}
	}
	// Build reply responses
	replyResponses := make([]*dto.TweetResponse, 0, len(replies))
	for _, reply := range replies {
		resp, err := s.buildTweetResponse(ctx, reply, "")
		if err != nil {
			continue
		}
		replyResponses = append(replyResponses, resp)
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
	var poll *dto.PollResult
	if tweet.IsPoll {
		p, err := s.pollRepo.GetByTweetID(ctx, tweetID)
		if err == nil {
			poll = s.buildPollResponse(p, "")
		}
	}
	// Build main tweet response
	mainResponse, err := s.buildTweetResponse(ctx, tweet, "")
	if err != nil {
		return nil, err
	}
	return &dto.TweetDetailResponse{
		Tweet:         mainResponse,
		ParentTweet:   parentTweet,
		RetweetSource: retweetSource,
		Replies:       replyResponses,
		Poll:          poll,
	}, nil
}

// ======================================================================
// Update Tweet
// ======================================================================

// UpdateTweet updates a tweet's content.
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
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, ErrEmptyContent
	}
	if len(content) > s.maxContentLen {
		return nil, ErrContentTooLong
	}
	tweet.Content = content
	tweet.UpdatedAt = time.Now()
	if err := s.tweetRepo.Update(ctx, tweet); err != nil {
		return nil, fmt.Errorf("failed to update tweet: %w", err)
	}
	// Invalidate caches
	_ = s.invalidateFeedCacheForUser(ctx, userID)
	return s.buildTweetResponse(ctx, tweet, userID)
}

// ======================================================================
// Delete Tweet
// ======================================================================

// DeleteTweet deletes a tweet.
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
	isAdmin := false
	if !isOwner {
		user, err := s.userRepo.GetByID(ctx, userID)
		if err == nil && user.Role == entities.RoleAdmin {
			isAdmin = true
		}
	}
	if !isOwner && !isAdmin {
		return ErrUnauthorized
	}
	// Soft delete
	if err := s.tweetRepo.SoftDelete(ctx, tweetID); err != nil {
		return fmt.Errorf("failed to delete tweet: %w", err)
	}
	// Decrement user tweet count if owner
	if isOwner {
		_ = s.userRepo.DecrementTweetCount(ctx, userID)
	}
	// Invalidate caches
	_ = s.invalidateFeedCacheForUser(ctx, tweet.UserID)
	return nil
}

// GetTweetByID retrieves a tweet entity by ID.
func (s *tweetService) GetTweetByID(ctx context.Context, tweetID string) (*entities.Tweet, error) {
	return s.tweetRepo.GetByID(ctx, tweetID)
}

// ======================================================================
// Get Feed
// ======================================================================

// GetFeed returns the home feed for a user.
func (s *tweetService) GetFeed(ctx context.Context, userID, cursor string, limit int) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > MaxFeedLimit {
		limit = DefaultFeedLimit
	}
	// Try cache first
	cacheKey := fmt.Sprintf("feed:%s:%s:%d", userID, cursor, limit)
	if s.redisAdapter != nil {
		var cached struct {
			Tweets    []*dto.TweetResponse `json:"tweets"`
			NextCursor string              `json:"next_cursor"`
		}
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("user_id", userID).Debug("Feed served from cache")
			return cached.Tweets, cached.NextCursor, nil
		}
	}
	// Get followed user IDs
	following, err := s.followRepo.GetFollowingIDs(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get following: %w", err)
	}
	// Include self
	userIDs := append(following, userID)
	// Get tweets
	tweets, nextCursor, err := s.tweetRepo.GetFeed(ctx, userIDs, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get feed: %w", err)
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
	// Cache for 15 seconds
	if s.redisAdapter != nil && len(responses) > 0 {
		cacheData := struct {
			Tweets    []*dto.TweetResponse `json:"tweets"`
			NextCursor string              `json:"next_cursor"`
		}{
			Tweets:    responses,
			NextCursor: nextCursor,
		}
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 15*time.Second)
	}
	return responses, nextCursor, nil
}

// ======================================================================
// Get User Tweets
// ======================================================================

// GetUserTweets returns tweets from a specific user.
func (s *tweetService) GetUserTweets(ctx context.Context, userID, cursor string, limit int, includeReplies bool) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > MaxFeedLimit {
		limit = DefaultFeedLimit
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", ErrUserNotFound
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}
	tweets, nextCursor, err := s.tweetRepo.GetByUserID(ctx, userID, cursor, limit, includeReplies)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user tweets: %w", err)
	}
	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, tweet := range tweets {
		resp, err := s.buildTweetResponse(ctx, tweet, "")
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	return responses, nextCursor, nil
}

// GetUserTweetsWithStats returns tweets from a specific user with stats.
func (s *tweetService) GetUserTweetsWithStats(ctx context.Context, userID, cursor string, limit int, includeReplies bool, currentUserID string) ([]*dto.TweetResponse, string, int64, error) {
	if limit < 1 || limit > MaxFeedLimit {
		limit = DefaultFeedLimit
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", 0, ErrUserNotFound
		}
		return nil, "", 0, fmt.Errorf("failed to get user: %w", err)
	}
	tweets, nextCursor, err := s.tweetRepo.GetByUserID(ctx, userID, cursor, limit, includeReplies)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get user tweets: %w", err)
	}
	total, err := s.tweetRepo.CountByUserID(ctx, userID)
	if err != nil {
		total = int64(len(tweets))
	}
	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, tweet := range tweets {
		resp, err := s.buildTweetResponse(ctx, tweet, currentUserID)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	return responses, nextCursor, total, nil
}

// ======================================================================
// Get Replies
// ======================================================================

// GetReplies returns replies to a tweet.
func (s *tweetService) GetReplies(ctx context.Context, tweetID, cursor string, limit int) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > MaxFeedLimit {
		limit = DefaultFeedLimit
	}
	// Verify tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, "", ErrTweetNotFound
		}
		return nil, "", fmt.Errorf("failed to get tweet: %w", err)
	}
	tweets, nextCursor, err := s.tweetRepo.GetReplies(ctx, tweetID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get replies: %w", err)
	}
	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, tweet := range tweets {
		resp, err := s.buildTweetResponse(ctx, tweet, "")
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	return responses, nextCursor, nil
}

// ======================================================================
// Like/Unlike
// ======================================================================

// LikeTweet adds a like to a tweet.
func (s *tweetService) LikeTweet(ctx context.Context, tweetID, userID string) error {
	// Check if tweet exists
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
	// Check if already liked
	exists, err := s.likeRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check like status: %w", err)
	}
	if exists {
		return ErrAlreadyLiked
	}
	// Create like
	like := &entities.Like{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if err := s.likeRepo.Create(ctx, like); err != nil {
		return fmt.Errorf("failed to like tweet: %w", err)
	}
	// Create notification
	if tweet.UserID != userID {
		_ = s.createNotification(ctx, tweet.UserID, userID, tweetID, "like")
	}
	// Invalidate caches
	_ = s.invalidateTweetCache(ctx, tweetID)
	return nil
}

// UnlikeTweet removes a like from a tweet.
func (s *tweetService) UnlikeTweet(ctx context.Context, tweetID, userID string) error {
	// Check if tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return err
	}
	// Check if liked
	exists, err := s.likeRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check like status: %w", err)
	}
	if !exists {
		return nil
	}
	// Remove like
	if err := s.likeRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
		return fmt.Errorf("failed to unlike tweet: %w", err)
	}
	// Invalidate caches
	_ = s.invalidateTweetCache(ctx, tweetID)
	return nil
}

// ======================================================================
// Retweet/Unretweet
// ======================================================================

// RetweetTweet adds a retweet to a tweet.
func (s *tweetService) RetweetTweet(ctx context.Context, tweetID, userID string) error {
	// Check if tweet exists
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
	retweet := &entities.Retweet{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if err := s.retweetRepo.Create(ctx, retweet); err != nil {
		return fmt.Errorf("failed to retweet: %w", err)
	}
	// Create notification
	_ = s.createNotification(ctx, tweet.UserID, userID, tweetID, "retweet")
	// Invalidate caches
	_ = s.invalidateTweetCache(ctx, tweetID)
	return nil
}

// UnretweetTweet removes a retweet from a tweet.
func (s *tweetService) UnretweetTweet(ctx context.Context, tweetID, userID string) error {
	// Check if tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return err
	}
	// Check if retweeted
	exists, err := s.retweetRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check retweet status: %w", err)
	}
	if !exists {
		return nil
	}
	// Remove retweet
	if err := s.retweetRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
		return fmt.Errorf("failed to unretweet: %w", err)
	}
	// Invalidate caches
	_ = s.invalidateTweetCache(ctx, tweetID)
	return nil
}

// ======================================================================
// Bookmark/Unbookmark
// ======================================================================

// BookmarkTweet adds a bookmark to a tweet.
func (s *tweetService) BookmarkTweet(ctx context.Context, tweetID, userID string) error {
	// Check if tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return err
	}
	// Check if already bookmarked
	exists, err := s.bookmarkRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check bookmark status: %w", err)
	}
	if exists {
		return ErrAlreadyBookmarked
	}
	// Create bookmark
	bookmark := &entities.Bookmark{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	if err := s.bookmarkRepo.Create(ctx, bookmark); err != nil {
		return fmt.Errorf("failed to bookmark: %w", err)
	}
	return nil
}

// UnbookmarkTweet removes a bookmark from a tweet.
func (s *tweetService) UnbookmarkTweet(ctx context.Context, tweetID, userID string) error {
	// Check if tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return err
	}
	// Check if bookmarked
	exists, err := s.bookmarkRepo.Exists(ctx, tweetID, userID)
	if err != nil {
		return fmt.Errorf("failed to check bookmark status: %w", err)
	}
	if !exists {
		return nil
	}
	// Remove bookmark
	if err := s.bookmarkRepo.DeleteByTweetAndUser(ctx, tweetID, userID); err != nil {
		return fmt.Errorf("failed to unbookmark: %w", err)
	}
	return nil
}

// ======================================================================
// Quote Tweet
// ======================================================================

// QuoteTweet creates a quote tweet.
func (s *tweetService) QuoteTweet(ctx context.Context, tweetID, userID string, req *dto.QuoteTweetRequest) (*dto.TweetResponse, error) {
	// Validate original tweet exists
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
	content := strings.TrimSpace(req.Content)
	if content == "" && len(req.MediaURLs) == 0 {
		return nil, ErrEmptyContent
	}
	if len(content) > s.maxContentLen {
		return nil, ErrContentTooLong
	}
	if len(req.MediaURLs) > MaxMediaCount {
		return nil, ErrMaxMediaExceeded
	}
	// Create quote tweet
	tweet := &entities.Tweet{
		ID:            uuid.New().String(),
		UserID:        userID,
		Content:       content,
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
	_ = s.userRepo.IncrementTweetCount(ctx, userID)
	// Create notification
	_ = s.createNotification(ctx, original.UserID, userID, tweetID, "quote")
	// Invalidate caches
	_ = s.invalidateFeedCacheForUser(ctx, userID)
	return s.buildTweetResponse(ctx, tweet, userID)
}

// ======================================================================
// Poll Operations
// ======================================================================

// VotePoll adds a vote to a poll.
func (s *tweetService) VotePoll(ctx context.Context, pollID, userID, optionID string) (*dto.PollResult, error) {
	// Get poll
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return nil, ErrPollNotFound
		}
		return nil, err
	}
	// Check if expired
	if time.Now().After(poll.ExpiresAt) {
		return nil, ErrPollExpired
	}
	// Check if already voted
	for _, opt := range poll.Options {
		for _, uid := range opt.VoterIDs {
			if uid == userID {
				return nil, ErrPollAlreadyVoted
			}
		}
	}
	// Find option
	found := false
	for i, opt := range poll.Options {
		if opt.ID == optionID {
			poll.Options[i].Votes++
			if poll.Options[i].VoterIDs == nil {
				poll.Options[i].VoterIDs = []string{}
			}
			poll.Options[i].VoterIDs = append(poll.Options[i].VoterIDs, userID)
			found = true
			break
		}
	}
	if !found {
		return nil, ErrInvalidPollOption
	}
	// Update poll
	if err := s.pollRepo.Update(ctx, poll); err != nil {
		return nil, fmt.Errorf("failed to update poll: %w", err)
	}
	// Invalidate cache
	_ = s.invalidatePollCache(ctx, pollID, poll.TweetID)
	return s.buildPollResponse(poll, userID), nil
}

// GetPollResults returns poll results.
func (s *tweetService) GetPollResults(ctx context.Context, pollID string) (*dto.PollResult, error) {
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return nil, ErrPollNotFound
		}
		return nil, err
	}
	return s.buildPollResponse(poll, ""), nil
}

// ======================================================================
// Get Trending
// ======================================================================

// GetTrending returns trending topics.
func (s *tweetService) GetTrending(ctx context.Context, limit int) ([]*dto.TrendingTopic, error) {
	if limit < 1 || limit > MaxTrendingLimit {
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
		return nil, fmt.Errorf("failed to get trending: %w", err)
	}
	// Cache for 5 minutes
	if s.redisAdapter != nil && len(trends) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, trends, 5*time.Minute)
	}
	return trends, nil
}

// ======================================================================
// Helper Methods
// ======================================================================

// buildTweetResponse builds a tweet response DTO.
func (s *tweetService) buildTweetResponse(ctx context.Context, tweet *entities.Tweet, currentUserID string) (*dto.TweetResponse, error) {
	// Get user
	user, err := s.userRepo.GetByID(ctx, tweet.UserID)
	if err != nil {
		return nil, err
	}
	// Get counts
	likeCount, _ := s.likeRepo.CountByTweetID(ctx, tweet.ID)
	retweetCount, _ := s.retweetRepo.CountByTweetID(ctx, tweet.ID)
	replyCount, _ := s.tweetRepo.CountReplies(ctx, tweet.ID)
	// Check interaction status
	liked := false
	retweeted := false
	bookmarked := false
	if currentUserID != "" {
		liked, _ = s.likeRepo.Exists(ctx, tweet.ID, currentUserID)
		retweeted, _ = s.retweetRepo.Exists(ctx, tweet.ID, currentUserID)
		bookmarked, _ = s.bookmarkRepo.Exists(ctx, tweet.ID, currentUserID)
	}
	// Extract mentions and hashtags
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
func (s *tweetService) buildPollResponse(poll *entities.Poll, currentUserID string) *dto.PollResult {
	options := make([]dto.PollOption, 0, len(poll.Options))
	totalVotes := int64(0)
	votedOptionID := ""

	// Calculate total votes
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	// Build options with percentages
	for _, opt := range poll.Options {
		percentage := float64(0)
		if totalVotes > 0 {
			percentage = (float64(opt.Votes) / float64(totalVotes)) * 100
		}
		isVoted := false
		if currentUserID != "" && opt.VoterIDs != nil {
			for _, uid := range opt.VoterIDs {
				if uid == currentUserID {
					isVoted = true
					votedOptionID = opt.ID
					break
				}
			}
		}
		options = append(options, dto.PollOption{
			ID:         opt.ID,
			Text:       opt.Text,
			Votes:      opt.Votes,
			Percentage: percentage,
			IsVoted:    isVoted,
		})
	}
	return &dto.PollResult{
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

// invalidateFeedCacheForUser invalidates feed cache for a specific user.
func (s *tweetService) invalidateFeedCacheForUser(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
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

// invalidateFeedCacheForFollowers invalidates feed cache for all followers.
func (s *tweetService) invalidateFeedCacheForFollowers(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	// Get followers
	followers, err := s.followRepo.GetFollowerIDs(ctx, userID)
	if err != nil {
		return err
	}
	for _, followerID := range followers {
		_ = s.invalidateFeedCacheForUser(ctx, followerID)
	}
	return nil
}

// invalidateTweetCache invalidates cache for a specific tweet.
func (s *tweetService) invalidateTweetCache(ctx context.Context, tweetID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	// Since we might cache tweet details, invalidate any tweet-specific caches
	// For now, we invalidate the trending cache
	return s.redisAdapter.Delete(ctx, "trending")
}

// invalidatePollCache invalidates poll caches.
func (s *tweetService) invalidatePollCache(ctx context.Context, pollID, tweetID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	keys := []string{
		fmt.Sprintf("poll:%s", pollID),
		fmt.Sprintf("poll:tweet:%s", tweetID),
		fmt.Sprintf("poll_results:%s", pollID),
	}
	return s.redisAdapter.Delete(ctx, keys...)
}

// ======================================================================
// Global Instance
// ======================================================================

var defaultTweetService TweetService

// InitTweetService initializes the global tweet service.
func InitTweetService(
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	likeRepo interfaces.LikeRepository,
	retweetRepo interfaces.RetweetRepository,
	bookmarkRepo interfaces.BookmarkRepository,
	pollRepo interfaces.PollRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
	wsHub *adapter.WebSocketHub,
	maxContentLen int,
) {
	defaultTweetService = NewTweetService(
		tweetRepo,
		userRepo,
		followRepo,
		likeRepo,
		retweetRepo,
		bookmarkRepo,
		pollRepo,
		notificationRepo,
		redisAdapter,
		wsHub,
		maxContentLen,
	)
}

// GetTweetService returns the global tweet service.
func GetTweetService() TweetService {
	if defaultTweetService == nil {
		panic("tweet service not initialized")
	}
	return defaultTweetService
}