// backend/internal/service/feed_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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
	DefaultFeedLimit      = 20
	MaxFeedLimit          = 100
	TrendingFeedLimit     = 50
	RecommendationLimit   = 10
	MaxPreferencesLimit   = 10
	DefaultDaysForMetrics = 7
)

var (
	ErrFeedEmpty              = errors.New("feed is empty")
	ErrInvalidCursor          = errors.New("invalid cursor")
	ErrFeedPreferencesNotFound = errors.New("feed preferences not found")
	ErrFeedItemAlreadyDismissed = errors.New("feed item already dismissed")
	ErrFeedGenerationFailed   = errors.New("feed generation failed")
	ErrInvalidFeedType        = errors.New("invalid feed type")
	ErrUserNotFound           = errors.New("user not found")
	ErrTweetNotFound          = errors.New("tweet not found")
)

// ======================================================================
// FeedService Interface
// ======================================================================

// FeedService defines the feed service interface.
type FeedService interface {
	GetHomeFeed(ctx context.Context, userID, cursor string, limit int, includeReplies, includeRetweets bool) ([]*dto.TweetResponse, string, error)
	GetUserFeed(ctx context.Context, username, cursor string, limit int, includeReplies bool, currentUserID string) ([]*dto.TweetResponse, string, int64, error)
	GetForYouFeed(ctx context.Context, userID, cursor string, limit int) ([]*dto.TweetResponse, string, error)
	GetTrendingFeed(ctx context.Context, limit int, since time.Time, currentUserID string) ([]*dto.TweetResponse, error)
	GetFeedRecommendations(ctx context.Context, userID string, limit int) ([]*dto.TweetResponse, error)
	GetFeedPreferences(ctx context.Context, userID string) (*dto.FeedPreferencesResponse, error)
	UpdateFeedPreferences(ctx context.Context, userID string, req *dto.UpdateFeedPreferencesRequest) (*dto.FeedPreferencesResponse, error)
	DismissFeedItem(ctx context.Context, userID, tweetID string) error
	GetFeedMetrics(ctx context.Context, days int) (*dto.FeedMetricsResponse, error)
	GetUserFeedStats(ctx context.Context, userID string) (*dto.UserFeedStatsResponse, error)
}

// ======================================================================
// FeedService Implementation
// ======================================================================

// feedService implements FeedService.
type feedService struct {
	tweetRepo      interfaces.TweetRepository
	userRepo       interfaces.UserRepository
	followRepo     interfaces.FollowRepository
	likeRepo       interfaces.LikeRepository
	retweetRepo    interfaces.RetweetRepository
	bookmarkRepo   interfaces.BookmarkRepository
	redisAdapter   adapter.RedisAdapter
	log            *logrus.Entry
}

// NewFeedService creates a new feed service.
func NewFeedService(
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	likeRepo interfaces.LikeRepository,
	retweetRepo interfaces.RetweetRepository,
	bookmarkRepo interfaces.BookmarkRepository,
	redisAdapter adapter.RedisAdapter,
) FeedService {
	return &feedService{
		tweetRepo:    tweetRepo,
		userRepo:     userRepo,
		followRepo:   followRepo,
		likeRepo:     likeRepo,
		retweetRepo:  retweetRepo,
		bookmarkRepo: bookmarkRepo,
		redisAdapter: redisAdapter,
		log:          logger.WithField("service", "feed"),
	}
}

// ======================================================================
// Get Home Feed
// ======================================================================

func (s *feedService) GetHomeFeed(ctx context.Context, userID, cursor string, limit int, includeReplies, includeRetweets bool) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > MaxFeedLimit {
		limit = DefaultFeedLimit
	}
	// Try cache first
	cacheKey := fmt.Sprintf("home_feed:%s:%s:%d:%v:%v", userID, cursor, limit, includeReplies, includeRetweets)
	if s.redisAdapter != nil {
		var cached struct {
			Tweets    []*dto.TweetResponse `json:"tweets"`
			NextCursor string              `json:"next_cursor"`
		}
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("user_id", userID).Debug("Home feed served from cache")
			return cached.Tweets, cached.NextCursor, nil
		}
	}
	// Get followed user IDs
	following, err := s.followRepo.GetFollowingIDs(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get following: %w", err)
	}
	userIDs := append(following, userID)
	// Get tweets
	tweets, nextCursor, err := s.tweetRepo.GetFeed(ctx, userIDs, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get feed: %w", err)
	}
	// Filter tweets based on includeReplies and includeRetweets
	filteredTweets := s.filterTweets(tweets, includeReplies, includeRetweets)
	// Build responses
	responses := make([]*dto.TweetResponse, 0, len(filteredTweets))
	for _, tweet := range filteredTweets {
		resp, err := s.buildTweetResponse(ctx, tweet, userID)
		if err != nil {
			s.log.WithError(err).Warn("Failed to build tweet response")
			continue
		}
		responses = append(responses, resp)
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil && len(responses) > 0 {
		cacheData := struct {
			Tweets    []*dto.TweetResponse `json:"tweets"`
			NextCursor string              `json:"next_cursor"`
		}{
			Tweets:    responses,
			NextCursor: nextCursor,
		}
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 30*time.Second)
	}
	return responses, nextCursor, nil
}

// ======================================================================
// Get User Feed
// ======================================================================

func (s *feedService) GetUserFeed(ctx context.Context, username, cursor string, limit int, includeReplies bool, currentUserID string) ([]*dto.TweetResponse, string, int64, error) {
	if limit < 1 || limit > MaxFeedLimit {
		limit = DefaultFeedLimit
	}
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", 0, ErrUserNotFound
		}
		return nil, "", 0, fmt.Errorf("failed to get user: %w", err)
	}
	tweets, nextCursor, err := s.tweetRepo.GetByUserID(ctx, user.ID, cursor, limit, includeReplies)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get user tweets: %w", err)
	}
	total, err := s.tweetRepo.CountByUserID(ctx, user.ID)
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
// Get For You Feed (Personalized)
// ======================================================================

func (s *feedService) GetForYouFeed(ctx context.Context, userID, cursor string, limit int) ([]*dto.TweetResponse, string, error) {
	if limit < 1 || limit > MaxFeedLimit {
		limit = DefaultFeedLimit
	}
	cacheKey := fmt.Sprintf("for_you_feed:%s:%s:%d", userID, cursor, limit)
	if s.redisAdapter != nil {
		var cached struct {
			Tweets    []*dto.TweetResponse `json:"tweets"`
			NextCursor string              `json:"next_cursor"`
		}
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("user_id", userID).Debug("For-you feed served from cache")
			return cached.Tweets, cached.NextCursor, nil
		}
	}
	// Get user's interests from feed preferences
	preferences, err := s.GetFeedPreferences(ctx, userID)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get preferences, using default")
	}
	// Get recommended tweets based on interests
	interests := []string{}
	if preferences != nil {
		interests = preferences.Interests
	}
	// Get tweets from users they follow first
	following, err := s.followRepo.GetFollowingIDs(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get following: %w", err)
	}
	userIDs := append(following, userID)
	// Get feed
	tweets, nextCursor, err := s.tweetRepo.GetFeed(ctx, userIDs, cursor, limit*2)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get feed: %w", err)
	}
	// Score and rank tweets based on relevance
	scoredTweets := s.scoreTweets(tweets, userID, interests)
	// Sort by score desc
	sort.Slice(scoredTweets, func(i, j int) bool {
		return scoredTweets[i].Score > scoredTweets[j].Score
	})
	// Take top limit
	if len(scoredTweets) > limit {
		scoredTweets = scoredTweets[:limit]
	}
	responses := make([]*dto.TweetResponse, 0, len(scoredTweets))
	for _, st := range scoredTweets {
		resp, err := s.buildTweetResponse(ctx, st.Tweet, userID)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	if s.redisAdapter != nil && len(responses) > 0 {
		cacheData := struct {
			Tweets    []*dto.TweetResponse `json:"tweets"`
			NextCursor string              `json:"next_cursor"`
		}{
			Tweets:    responses,
			NextCursor: nextCursor,
		}
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 30*time.Second)
	}
	return responses, nextCursor, nil
}

// ======================================================================
// Get Trending Feed
// ======================================================================

func (s *feedService) GetTrendingFeed(ctx context.Context, limit int, since time.Time, currentUserID string) ([]*dto.TweetResponse, error) {
	if limit < 1 || limit > TrendingFeedLimit {
		limit = 20
	}
	if since.IsZero() {
		since = time.Now().Add(-24 * time.Hour)
	}
	cacheKey := fmt.Sprintf("trending_feed:%d:%s", limit, since.Format("2006-01-02"))
	if s.redisAdapter != nil {
		var cached []*dto.TweetResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}
	// Get most liked tweets in the time range
	tweets, err := s.likeRepo.GetMostLikedTweets(ctx, limit, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get trending tweets: %w", err)
	}
	// If not enough, get from retweets as well
	if len(tweets) < limit {
		moreTweets, err := s.retweetRepo.GetMostRetweetedTweets(ctx, limit-len(tweets), since)
		if err == nil {
			tweets = append(tweets, moreTweets...)
		}
	}
	// Remove duplicates
	tweetMap := make(map[string]*entities.Tweet)
	for _, t := range tweets {
		tweetMap[t.ID] = t
	}
	uniqueTweets := make([]*entities.Tweet, 0, len(tweetMap))
	for _, t := range tweetMap {
		uniqueTweets = append(uniqueTweets, t)
	}
	responses := make([]*dto.TweetResponse, 0, len(uniqueTweets))
	for _, tweet := range uniqueTweets {
		resp, err := s.buildTweetResponse(ctx, tweet, currentUserID)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	if s.redisAdapter != nil && len(responses) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, responses, 5*time.Minute)
	}
	return responses, nil
}

// ======================================================================
// Get Feed Recommendations
// ======================================================================

func (s *feedService) GetFeedRecommendations(ctx context.Context, userID string, limit int) ([]*dto.TweetResponse, error) {
	if limit < 1 || limit > RecommendationLimit {
		limit = RecommendationLimit
	}
	cacheKey := fmt.Sprintf("feed_recommendations:%s:%d", userID, limit)
	if s.redisAdapter != nil {
		var cached []*dto.TweetResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}
	// Get user's interests
	preferences, err := s.GetFeedPreferences(ctx, userID)
	if err != nil {
		s.log.WithError(err).Warn("Failed to get preferences")
	}
	interests := []string{}
	if preferences != nil {
		interests = preferences.Interests
	}
	// Get popular tweets from users they don't follow
	following, err := s.followRepo.GetFollowingIDs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get following: %w", err)
	}
	followMap := make(map[string]bool)
	for _, id := range following {
		followMap[id] = true
	}
	// Get most liked tweets overall
	tweets, err := s.likeRepo.GetMostLikedTweets(ctx, limit*3, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %w", err)
	}
	// Filter out tweets from followed users
	var filtered []*entities.Tweet
	for _, t := range tweets {
		if !followMap[t.UserID] && t.UserID != userID {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	responses := make([]*dto.TweetResponse, 0, len(filtered))
	for _, tweet := range filtered {
		resp, err := s.buildTweetResponse(ctx, tweet, userID)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	if s.redisAdapter != nil && len(responses) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, responses, 5*time.Minute)
	}
	return responses, nil
}

// ======================================================================
// Feed Preferences
// ======================================================================

func (s *feedService) GetFeedPreferences(ctx context.Context, userID string) (*dto.FeedPreferencesResponse, error) {
	cacheKey := fmt.Sprintf("feed_preferences:%s", userID)
	if s.redisAdapter != nil {
		var cached dto.FeedPreferencesResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	// Preferences are stored in a simple key-value store or user metadata
	// For simplicity, return default preferences
	preferences := &dto.FeedPreferencesResponse{
		UserID:       userID,
		Interests:    []string{},
		ShowReplies:  true,
		ShowRetweets: true,
		FeedType:     "chronological",
		UpdatedAt:    time.Now(),
	}
	// Try to get from Redis hash
	if s.redisAdapter != nil {
		prefData, err := s.redisAdapter.HGetAll(ctx, "feed_pref:"+userID)
		if err == nil && len(prefData) > 0 {
			if interests, ok := prefData["interests"]; ok {
				preferences.Interests = strings.Split(interests, ",")
			}
			if showReplies, ok := prefData["show_replies"]; ok {
				preferences.ShowReplies = showReplies == "true"
			}
			if showRetweets, ok := prefData["show_retweets"]; ok {
				preferences.ShowRetweets = showRetweets == "true"
			}
			if feedType, ok := prefData["feed_type"]; ok {
				preferences.FeedType = feedType
			}
		}
	}
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, preferences, 1*time.Hour)
	}
	return preferences, nil
}

func (s *feedService) UpdateFeedPreferences(ctx context.Context, userID string, req *dto.UpdateFeedPreferencesRequest) (*dto.FeedPreferencesResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	preferences := &dto.FeedPreferencesResponse{
		UserID:       userID,
		Interests:    req.Interests,
		ShowReplies:  req.ShowReplies,
		ShowRetweets: req.ShowRetweets,
		FeedType:     req.FeedType,
		UpdatedAt:    time.Now(),
	}
	// Store in Redis hash
	if s.redisAdapter != nil {
		prefMap := map[string]interface{}{
			"interests":      strings.Join(req.Interests, ","),
			"show_replies":   req.ShowReplies,
			"show_retweets":  req.ShowRetweets,
			"feed_type":      req.FeedType,
			"updated_at":     time.Now().Unix(),
		}
		if err := s.redisAdapter.HSet(ctx, "feed_pref:"+userID, prefMap); err != nil {
			s.log.WithError(err).Warn("Failed to save feed preferences")
		}
	}
	// Invalidate cache
	_ = s.invalidateFeedCache(ctx, userID)
	return preferences, nil
}

// ======================================================================
// Dismiss Feed Item
// ======================================================================

func (s *feedService) DismissFeedItem(ctx context.Context, userID, tweetID string) error {
	// Store dismissed tweets in Redis set
	if s.redisAdapter == nil {
		return errors.New("redis not available")
	}
	key := fmt.Sprintf("dismissed:%s", userID)
	added, err := s.redisAdapter.SAdd(ctx, key, tweetID)
	if err != nil {
		return fmt.Errorf("failed to dismiss feed item: %w", err)
	}
	if added == 0 {
		return ErrFeedItemAlreadyDismissed
	}
	// Set expiry (30 days)
	_ = s.redisAdapter.Expire(ctx, key, 30*24*time.Hour)
	// Invalidate feed cache
	_ = s.invalidateFeedCache(ctx, userID)
	return nil
}

// ======================================================================
// Get Feed Metrics (Admin)
// ======================================================================

func (s *feedService) GetFeedMetrics(ctx context.Context, days int) (*dto.FeedMetricsResponse, error) {
	if days < 1 || days > 30 {
		days = DefaultDaysForMetrics
	}
	startDate := time.Now().AddDate(0, 0, -days)
	// Get tweet metrics
	tweetStats, err := s.tweetRepo.GetTweetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tweet stats: %w", err)
	}
	// Get daily stats
	dailyStats, err := s.tweetRepo.GetDailyTweetStats(ctx, startDate, time.Now())
	if err != nil {
		dailyStats = []*interfaces.DailyTweetCount{}
	}
	// Calculate engagement rate
	engagementRate := 0.0
	if tweetStats.TotalTweets > 0 {
		engagementRate = float64(tweetStats.TotalLikes+tweetStats.TotalRetweets) / float64(tweetStats.TotalTweets)
	}
	return &dto.FeedMetricsResponse{
		TotalTweets:     tweetStats.TotalTweets,
		TotalLikes:      tweetStats.TotalLikes,
		TotalRetweets:   tweetStats.TotalRetweets,
		AverageLikes:    tweetStats.AverageLikes,
		AverageRetweets: tweetStats.AverageRetweets,
		EngagementRate:  engagementRate,
		DailyStats:      dailyStats,
		PeriodDays:      days,
		Timestamp:       time.Now(),
	}, nil
}

// ======================================================================
// Get User Feed Stats
// ======================================================================

func (s *feedService) GetUserFeedStats(ctx context.Context, userID string) (*dto.UserFeedStatsResponse, error) {
	// Get user's tweet stats
	tweetStats, err := s.tweetRepo.GetUserTweetStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tweet stats: %w", err)
	}
	// Get interaction counts
	likes, err := s.likeRepo.CountByUserID(ctx, userID)
	if err != nil {
		likes = 0
	}
	retweets, err := s.retweetRepo.CountByUserID(ctx, userID)
	if err != nil {
		retweets = 0
	}
	// Get follow counts
	followers, err := s.followRepo.CountFollowers(ctx, userID)
	if err != nil {
		followers = 0
	}
	following, err := s.followRepo.CountFollowing(ctx, userID)
	if err != nil {
		following = 0
	}
	// Calculate engagement rate for user's tweets
	engagementRate := 0.0
	if tweetStats.TotalTweets > 0 {
		engagementRate = float64(tweetStats.TotalLikes+tweetStats.TotalRetweets) / float64(tweetStats.TotalTweets)
	}
	// Get user for join date
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &dto.UserFeedStatsResponse{
		UserID:         userID,
		TotalTweets:    tweetStats.TotalTweets,
		TotalLikes:     likes,
		TotalRetweets:  retweets,
		TotalFollowers: followers,
		TotalFollowing: following,
		EngagementRate: engagementRate,
		LastTweetAt:    tweetStats.LastTweetAt,
		JoinedAt:       user.CreatedAt,
	}, nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// filterTweets filters tweets based on includeReplies and includeRetweets.
func (s *feedService) filterTweets(tweets []*entities.Tweet, includeReplies, includeRetweets bool) []*entities.Tweet {
	if includeReplies && includeRetweets {
		return tweets
	}
	filtered := make([]*entities.Tweet, 0, len(tweets))
	for _, t := range tweets {
		if !includeReplies && t.ParentTweetID != nil && *t.ParentTweetID != "" {
			continue
		}
		if !includeRetweets && t.RetweetOfID != nil && *t.RetweetOfID != "" {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

// scoreTweets scores tweets for personalized feed.
func (s *feedService) scoreTweets(tweets []*entities.Tweet, userID string, interests []string) []*ScoredTweet {
	scored := make([]*ScoredTweet, 0, len(tweets))
	for _, tweet := range tweets {
		score := 0.0
		// Recent tweets get higher score
		hoursSince := time.Since(tweet.CreatedAt).Hours()
		if hoursSince < 1 {
			score += 5.0
		} else if hoursSince < 6 {
			score += 3.0
		} else if hoursSince < 24 {
			score += 1.0
		}
		// Tweets with hashtags matching interests get boost
		if len(interests) > 0 {
			for _, interest := range interests {
				if strings.Contains(strings.ToLower(tweet.Content), strings.ToLower(interest)) {
					score += 2.0
				}
			}
		}
		// Tweets from people who follow back get boost
		// This would require checking mutual follows
		scored = append(scored, &ScoredTweet{
			Tweet: tweet,
			Score: score,
		})
	}
	return scored
}

// buildTweetResponse builds a tweet response DTO.
func (s *feedService) buildTweetResponse(ctx context.Context, tweet *entities.Tweet, currentUserID string) (*dto.TweetResponse, error) {
	user, err := s.userRepo.GetByID(ctx, tweet.UserID)
	if err != nil {
		return nil, err
	}
	likeCount, err := s.likeRepo.CountByTweetID(ctx, tweet.ID)
	if err != nil {
		likeCount = 0
	}
	retweetCount, err := s.retweetRepo.CountByTweetID(ctx, tweet.ID)
	if err != nil {
		retweetCount = 0
	}
	replyCount, err := s.tweetRepo.CountReplies(ctx, tweet.ID)
	if err != nil {
		replyCount = 0
	}
	liked := false
	retweeted := false
	bookmarked := false
	if currentUserID != "" {
		liked, _ = s.likeRepo.Exists(ctx, tweet.ID, currentUserID)
		retweeted, _ = s.retweetRepo.Exists(ctx, tweet.ID, currentUserID)
		bookmarked, _ = s.bookmarkRepo.Exists(ctx, tweet.ID, currentUserID)
	}
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
		CreatedAt:    tweet.CreatedAt,
		UpdatedAt:    tweet.UpdatedAt,
	}, nil
}

// invalidateFeedCache invalidates feed cache for a user.
func (s *feedService) invalidateFeedCache(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	patterns := []string{
		fmt.Sprintf("home_feed:%s:*", userID),
		fmt.Sprintf("for_you_feed:%s:*", userID),
		fmt.Sprintf("feed_recommendations:%s:*", userID),
		fmt.Sprintf("feed_preferences:%s", userID),
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
// ScoredTweet (internal)
// ======================================================================

type ScoredTweet struct {
	Tweet *entities.Tweet
	Score float64
}

// ======================================================================
// Service Registration
// ======================================================================

// Global feed service instance (optional)
var defaultFeedService FeedService

// InitFeedService initializes the global feed service.
func InitFeedService(
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	likeRepo interfaces.LikeRepository,
	retweetRepo interfaces.RetweetRepository,
	bookmarkRepo interfaces.BookmarkRepository,
	redisAdapter adapter.RedisAdapter,
) {
	defaultFeedService = NewFeedService(
		tweetRepo,
		userRepo,
		followRepo,
		likeRepo,
		retweetRepo,
		bookmarkRepo,
		redisAdapter,
	)
}

// GetFeedService returns the global feed service.
func GetFeedService() FeedService {
	if defaultFeedService == nil {
		panic("feed service not initialized")
	}
	return defaultFeedService
}