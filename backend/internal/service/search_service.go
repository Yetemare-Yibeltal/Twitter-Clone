// backend/internal/service/search_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
	MaxSearchResults     = 100
	DefaultSearchResults = 20
	MinSearchQueryLength = 2
	MaxSearchQueryLength = 200
)

var (
	ErrSearchQueryTooShort = fmt.Errorf("search query must be at least %d characters", MinSearchQueryLength)
	ErrSearchQueryTooLong  = fmt.Errorf("search query must be at most %d characters", MaxSearchQueryLength)
	ErrSearchQueryEmpty    = errors.New("search query cannot be empty")
	ErrSearchInvalidFilter = errors.New("invalid search filter")
	ErrSearchNoResults     = errors.New("no results found")
)

// ======================================================================
= SearchService Interface
// ======================================================================

// SearchService defines the search service interface.
type SearchService interface {
	// SearchTweets searches for tweets matching the query.
	SearchTweets(ctx context.Context, query string, filters *dto.SearchFilters, cursor string, limit int) ([]*dto.TweetResponse, string, int64, error)
	
	// SearchUsers searches for users matching the query.
	SearchUsers(ctx context.Context, query string, cursor string, limit int, currentUserID string) ([]*dto.UserSearchResponse, string, int64, error)
	
	// SearchHashtags searches for hashtags matching the query.
	SearchHashtags(ctx context.Context, query string, cursor string, limit int) ([]*dto.HashtagResponse, string, int64, error)
	
	// SearchAll performs a combined search across tweets, users, and hashtags.
	SearchAll(ctx context.Context, query string, cursor string, limit int, currentUserID string) (*dto.CombinedSearchResponse, error)
	
	// GetSearchSuggestions returns search suggestions based on partial query.
	GetSearchSuggestions(ctx context.Context, query string, limit int) (*dto.SearchSuggestionsResponse, error)
	
	// GetTrendingSearches returns trending search queries.
	GetTrendingSearches(ctx context.Context, limit int) ([]*dto.TrendingSearchResponse, error)
	
	// GetSearchStats returns search statistics.
	GetSearchStats(ctx context.Context) (*dto.SearchStatsResponse, error)
	
	// RecordSearch records a search query for analytics.
	RecordSearch(ctx context.Context, query string, userID string, resultCount int) error
}

// ======================================================================
= SearchService Implementation
// ======================================================================

// searchService implements SearchService.
type searchService struct {
	tweetRepo    interfaces.TweetRepository
	userRepo     interfaces.UserRepository
	followRepo   interfaces.FollowRepository
	redisAdapter adapter.RedisAdapter
	log          *logrus.Entry
}

// NewSearchService creates a new search service.
func NewSearchService(
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	redisAdapter adapter.RedisAdapter,
) SearchService {
	return &searchService{
		tweetRepo:    tweetRepo,
		userRepo:     userRepo,
		followRepo:   followRepo,
		redisAdapter: redisAdapter,
		log:          logger.WithField("service", "search"),
	}
}

// ======================================================================
= Search Tweets
// ======================================================================

// SearchTweets searches for tweets matching the query.
func (s *searchService) SearchTweets(ctx context.Context, query string, filters *dto.SearchFilters, cursor string, limit int) ([]*dto.TweetResponse, string, int64, error) {
	// Validate query
	if err := s.validateQuery(query); err != nil {
		return nil, "", 0, err
	}
	
	if limit < 1 || limit > MaxSearchResults {
		limit = DefaultSearchResults
	}
	
	// Build cache key
	cacheKey := s.buildTweetCacheKey(query, filters, cursor, limit)
	
	// Try cache first
	if s.redisAdapter != nil {
		var cached struct {
			Tweets    []*dto.TweetResponse `json:"tweets"`
			NextCursor string              `json:"next_cursor"`
			Total     int64                `json:"total"`
		}
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("query", query).Debug("Tweet search served from cache")
			return cached.Tweets, cached.NextCursor, cached.Total, nil
		}
	}
	
	// Parse query for filters and operators
	parsedQuery := s.parseSearchQuery(query)
	
	// Build search query
	searchQuery := parsedQuery.Text
	if searchQuery == "" {
		searchQuery = query
	}
	
	// Apply filters from parsed query
	if filters == nil {
		filters = &dto.SearchFilters{}
	}
	if parsedQuery.FromUser != "" {
		filters.FromUser = parsedQuery.FromUser
	}
	if parsedQuery.ToUser != "" {
		filters.ToUser = parsedQuery.ToUser
	}
	if !parsedQuery.Since.IsZero() {
		filters.Since = parsedQuery.Since
	}
	if !parsedQuery.Until.IsZero() {
		filters.Until = parsedQuery.Until
	}
	filters.IncludeReplies = parsedQuery.IncludeReplies
	filters.IncludeRetweets = parsedQuery.IncludeRetweets
	filters.MediaOnly = parsedQuery.MediaOnly
	filters.Query = searchQuery
	
	// Get tweets from repository
	tweets, nextCursor, err := s.tweetRepo.Search(ctx, searchQuery, cursor, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to search tweets: %w", err)
	}
	
	// Filter results
	filteredTweets := s.applyTweetFilters(tweets, filters)
	
	// Build responses
	responses := make([]*dto.TweetResponse, 0, len(filteredTweets))
	for _, tweet := range filteredTweets {
		resp, err := s.buildTweetResponse(ctx, tweet, "")
		if err != nil {
			s.log.WithError(err).Warn("Failed to build tweet response")
			continue
		}
		responses = append(responses, resp)
	}
	
	// Get total count
	total := int64(len(responses))
	
	// Cache for 1 minute
	if s.redisAdapter != nil && len(responses) > 0 {
		cacheData := struct {
			Tweets    []*dto.TweetResponse `json:"tweets"`
			NextCursor string              `json:"next_cursor"`
			Total     int64                `json:"total"`
		}{
			Tweets:    responses,
			NextCursor: nextCursor,
			Total:     total,
		}
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 1*time.Minute)
	}
	
	// Record search for analytics
	_ = s.RecordSearch(ctx, query, "", total)
	
	return responses, nextCursor, total, nil
}

// ======================================================================
= Search Users
// ======================================================================

// SearchUsers searches for users matching the query.
func (s *searchService) SearchUsers(ctx context.Context, query string, cursor string, limit int, currentUserID string) ([]*dto.UserSearchResponse, string, int64, error) {
	// Validate query
	if err := s.validateQuery(query); err != nil {
		return nil, "", 0, err
	}
	
	if limit < 1 || limit > MaxSearchResults {
		limit = DefaultSearchResults
	}
	
	// Build cache key
	cacheKey := fmt.Sprintf("user_search:%s:%s:%d", query, cursor, limit)
	
	// Try cache first
	if s.redisAdapter != nil {
		var cached struct {
			Users     []*dto.UserSearchResponse `json:"users"`
			NextCursor string                   `json:"next_cursor"`
			Total     int64                     `json:"total"`
		}
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached.Users, cached.NextCursor, cached.Total, nil
		}
	}
	
	// Search users from repository
	users, total, err := s.userRepo.Search(ctx, query, nil)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to search users: %w", err)
	}
	
	// Build responses
	responses := make([]*dto.UserSearchResponse, 0, len(users))
	for _, user := range users {
		// Check if current user follows this user
		isFollowing := false
		if currentUserID != "" {
			isFollowing, _ = s.followRepo.Exists(ctx, currentUserID, user.ID)
		}
		
		// Check if mutual
		isMutual := false
		if currentUserID != "" && isFollowing {
			mutual, _ := s.followRepo.AreMutual(ctx, currentUserID, user.ID)
			isMutual = mutual
		}
		
		responses = append(responses, &dto.UserSearchResponse{
			ID:          user.ID,
			Username:    user.Username,
			FullName:    user.FullName,
			AvatarURL:   user.AvatarURL,
			Bio:         user.Bio,
			IsVerified:  user.IsVerified,
			IsFollowing: isFollowing,
			IsMutual:    isMutual,
			FollowerCount: user.FollowerCount,
			TweetCount:  user.TweetCount,
		})
	}
	
	// Get total count
	total = int64(len(responses))
	
	// Cache for 5 minutes
	if s.redisAdapter != nil && len(responses) > 0 {
		cacheData := struct {
			Users     []*dto.UserSearchResponse `json:"users"`
			NextCursor string                   `json:"next_cursor"`
			Total     int64                     `json:"total"`
		}{
			Users:     responses,
			NextCursor: "",
			Total:     total,
		}
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 5*time.Minute)
	}
	
	// Record search
	_ = s.RecordSearch(ctx, query, currentUserID, total)
	
	return responses, "", total, nil
}

// ======================================================================
= Search Hashtags
// ======================================================================

// SearchHashtags searches for hashtags matching the query.
func (s *searchService) SearchHashtags(ctx context.Context, query string, cursor string, limit int) ([]*dto.HashtagResponse, string, int64, error) {
	// Validate query
	if err := s.validateQuery(query); err != nil {
		return nil, "", 0, err
	}
	
	if limit < 1 || limit > MaxSearchResults {
		limit = DefaultSearchResults
	}
	
	// Build cache key
	cacheKey := fmt.Sprintf("hashtag_search:%s:%s:%d", query, cursor, limit)
	
	// Try cache first
	if s.redisAdapter != nil {
		var cached struct {
			Hashtags  []*dto.HashtagResponse `json:"hashtags"`
			NextCursor string                `json:"next_cursor"`
			Total     int64                  `json:"total"`
		}
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached.Hashtags, cached.NextCursor, cached.Total, nil
		}
	}
	
	// Clean and format query
	cleanQuery := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(query)), "#")
	
	// Search for hashtags from tweets (this would be more efficient with a dedicated hashtag table)
	// For now, we'll get trending and filter
	trends, err := s.tweetRepo.GetTrending(ctx, 50)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get hashtags: %w", err)
	}
	
	// Filter and rank
	responses := make([]*dto.HashtagResponse, 0)
	for _, trend := range trends {
		if strings.Contains(strings.ToLower(trend.Hashtag), cleanQuery) {
			responses = append(responses, &dto.HashtagResponse{
				Hashtag:  "#" + trend.Hashtag,
				Count:    trend.Count,
				Trending: true,
			})
		}
	}
	
	// Sort by count
	for i := 0; i < len(responses); i++ {
		for j := i + 1; j < len(responses); j++ {
			if responses[j].Count > responses[i].Count {
				responses[i], responses[j] = responses[j], responses[i]
			}
		}
	}
	
	// Limit
	if len(responses) > limit {
		responses = responses[:limit]
	}
	
	total := int64(len(responses))
	
	// Cache for 5 minutes
	if s.redisAdapter != nil && len(responses) > 0 {
		cacheData := struct {
			Hashtags  []*dto.HashtagResponse `json:"hashtags"`
			NextCursor string                `json:"next_cursor"`
			Total     int64                  `json:"total"`
		}{
			Hashtags:  responses,
			NextCursor: "",
			Total:     total,
		}
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 5*time.Minute)
	}
	
	return responses, "", total, nil
}

// ======================================================================
= Search All
// ======================================================================

// SearchAll performs a combined search across tweets, users, and hashtags.
func (s *searchService) SearchAll(ctx context.Context, query string, cursor string, limit int, currentUserID string) (*dto.CombinedSearchResponse, error) {
	// Validate query
	if err := s.validateQuery(query); err != nil {
		return nil, err
	}
	
	// Determine limits for each category
	tweetLimit := limit
	userLimit := limit / 2
	if userLimit < 1 {
		userLimit = 1
	}
	hashtagLimit := limit / 4
	if hashtagLimit < 1 {
		hashtagLimit = 1
	}
	
	// Search tweets
	tweets, _, tweetTotal, err := s.SearchTweets(ctx, query, nil, cursor, tweetLimit)
	if err != nil {
		s.log.WithError(err).Warn("Failed to search tweets in combined search")
		tweets = []*dto.TweetResponse{}
		tweetTotal = 0
	}
	
	// Search users
	users, _, userTotal, err := s.SearchUsers(ctx, query, "", userLimit, currentUserID)
	if err != nil {
		s.log.WithError(err).Warn("Failed to search users in combined search")
		users = []*dto.UserSearchResponse{}
		userTotal = 0
	}
	
	// Search hashtags
	hashtags, _, hashtagTotal, err := s.SearchHashtags(ctx, query, "", hashtagLimit)
	if err != nil {
		s.log.WithError(err).Warn("Failed to search hashtags in combined search")
		hashtags = []*dto.HashtagResponse{}
		hashtagTotal = 0
	}
	
	return &dto.CombinedSearchResponse{
		Query:         query,
		Tweets:        tweets,
		TweetCount:    tweetTotal,
		Users:         users,
		UserCount:     userTotal,
		Hashtags:      hashtags,
		HashtagCount:  hashtagTotal,
		TotalResults:  tweetTotal + userTotal + hashtagTotal,
	}, nil
}

// ======================================================================
= Get Search Suggestions
// ======================================================================

// GetSearchSuggestions returns search suggestions based on partial query.
func (s *searchService) GetSearchSuggestions(ctx context.Context, query string, limit int) (*dto.SearchSuggestionsResponse, error) {
	if limit < 1 || limit > 20 {
		limit = 10
	}
	
	if len(strings.TrimSpace(query)) < 2 {
		return &dto.SearchSuggestionsResponse{
			Query:       query,
			Suggestions: []string{},
		}, nil
	}
	
	// Build cache key
	cacheKey := fmt.Sprintf("suggestions:%s:%d", query, limit)
	if s.redisAdapter != nil {
		var cached dto.SearchSuggestionsResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	
	suggestions := make([]string, 0)
	
	// Get user suggestions
	users, _, _, err := s.SearchUsers(ctx, query, "", limit, "")
	if err == nil {
		for _, user := range users {
			suggestions = append(suggestions, "@"+user.Username)
		}
	}
	
	// Get hashtag suggestions
	hashtags, _, _, err := s.SearchHashtags(ctx, query, "", limit)
	if err == nil {
		for _, tag := range hashtags {
			suggestions = append(suggestions, tag.Hashtag)
		}
	}
	
	// Get common search queries from Redis
	if s.redisAdapter != nil {
		// Get trending searches
		trending, _ := s.getTrendingSearchesFromCache(ctx, 10)
		for _, t := range trending {
			if strings.Contains(strings.ToLower(t), strings.ToLower(query)) {
				suggestions = append(suggestions, t)
			}
		}
	}
	
	// Remove duplicates
	seen := make(map[string]bool)
	unique := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	
	if len(unique) > limit {
		unique = unique[:limit]
	}
	
	response := &dto.SearchSuggestionsResponse{
		Query:       query,
		Suggestions: unique,
	}
	
	// Cache for 1 hour
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, response, 1*time.Hour)
	}
	
	return response, nil
}

// ======================================================================
= Get Trending Searches
// ======================================================================

// GetTrendingSearches returns trending search queries.
func (s *searchService) GetTrendingSearches(ctx context.Context, limit int) ([]*dto.TrendingSearchResponse, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	
	// Try cache first
	cacheKey := fmt.Sprintf("trending_searches:%d", limit)
	if s.redisAdapter != nil {
		var cached []*dto.TrendingSearchResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}
	
	// Get from Redis (we store search history in sorted sets)
	trending, err := s.getTrendingSearchesFromCache(ctx, limit)
	if err != nil {
		// Fallback: use empty list
		trending = []string{}
	}
	
	responses := make([]*dto.TrendingSearchResponse, 0, len(trending))
	for i, term := range trending {
		// Get count for each term
		count, _ := s.getSearchCount(ctx, term)
		responses = append(responses, &dto.TrendingSearchResponse{
			Query:    term,
			Count:    count,
			Position: i + 1,
		})
	}
	
	// Cache for 15 minutes
	if s.redisAdapter != nil && len(responses) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, responses, 15*time.Minute)
	}
	
	return responses, nil
}

// ======================================================================
= Get Search Stats
// ======================================================================

// GetSearchStats returns search statistics.
func (s *searchService) GetSearchStats(ctx context.Context) (*dto.SearchStatsResponse, error) {
	// Try cache first
	cacheKey := "search_stats"
	if s.redisAdapter != nil {
		var cached dto.SearchStatsResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	
	// Get stats from Redis
	totalSearches, _ := s.getTotalSearches(ctx)
	uniqueSearches, _ := s.getUniqueSearches(ctx)
	topQueries, _ := s.getTrendingSearchesFromCache(ctx, 10)
	
	// Get daily search counts (last 7 days)
	dailyCounts := []*dto.DailySearchCount{}
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		key := fmt.Sprintf("search:date:%s", date.Format("2006-01-02"))
		count, _ := s.redisAdapter.Get(ctx, key)
		var cnt int64
		fmt.Sscanf(count, "%d", &cnt)
		dailyCounts = append(dailyCounts, &dto.DailySearchCount{
			Date:  date,
			Count: cnt,
		})
	}
	
	response := &dto.SearchStatsResponse{
		TotalSearches:  totalSearches,
		UniqueSearches: uniqueSearches,
		TopQueries:     topQueries,
		DailyCounts:    dailyCounts,
	}
	
	// Cache for 1 hour
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, response, 1*time.Hour)
	}
	
	return response, nil
}

// ======================================================================
= Record Search
// ======================================================================

// RecordSearch records a search query for analytics.
func (s *searchService) RecordSearch(ctx context.Context, query string, userID string, resultCount int) error {
	if s.redisAdapter == nil {
		return nil
	}
	
	// Normalize query
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || len(query) < 3 {
		return nil
	}
	
	// Increment total searches
	_ = s.redisAdapter.Incr(ctx, "search:total")
	
	// Increment query count (sorted set for trending)
	_ = s.redisAdapter.ZIncrBy(ctx, "search:trending", 1, query)
	
	// Add to daily count
	dateKey := fmt.Sprintf("search:date:%s", time.Now().Format("2006-01-02"))
	_ = s.redisAdapter.Incr(ctx, dateKey)
	_ = s.redisAdapter.Expire(ctx, dateKey, 30*24*time.Hour)
	
	// Record user search if userID provided
	if userID != "" {
		userKey := fmt.Sprintf("search:user:%s", userID)
		_ = s.redisAdapter.ZAdd(ctx, userKey, float64(time.Now().Unix()), query)
		_ = s.redisAdapter.Expire(ctx, userKey, 30*24*time.Hour)
	}
	
	// Store query with result count
	if resultCount > 0 {
		_ = s.redisAdapter.ZAdd(ctx, "search:results", float64(resultCount), query)
	}
	
	// Increment unique searches (hash)
	_ = s.redisAdapter.SAdd(ctx, "search:unique", query)
	
	s.log.WithFields(logrus.Fields{
		"query":        query,
		"user_id":      userID,
		"result_count": resultCount,
	}).Debug("Search recorded")
	
	return nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// validateQuery validates the search query.
func (s *searchService) validateQuery(query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return ErrSearchQueryEmpty
	}
	if len(query) < MinSearchQueryLength {
		return ErrSearchQueryTooShort
	}
	if len(query) > MaxSearchQueryLength {
		return ErrSearchQueryTooLong
	}
	return nil
}

// buildTweetCacheKey builds a cache key for tweet search.
func (s *searchService) buildTweetCacheKey(query string, filters *dto.SearchFilters, cursor string, limit int) string {
	key := fmt.Sprintf("tweet_search:%s:%d", query, limit)
	if filters != nil {
		if filters.FromUser != "" {
			key += ":from:" + filters.FromUser
		}
		if filters.ToUser != "" {
			key += ":to:" + filters.ToUser
		}
		if !filters.Since.IsZero() {
			key += ":since:" + filters.Since.Format("2006-01-02")
		}
		if !filters.Until.IsZero() {
			key += ":until:" + filters.Until.Format("2006-01-02")
		}
		if filters.IncludeReplies {
			key += ":replies"
		}
		if filters.IncludeRetweets {
			key += ":retweets"
		}
		if filters.MediaOnly {
			key += ":media"
		}
	}
	if cursor != "" {
		key += ":cursor:" + cursor
	}
	return key
}

// parseSearchQuery parses search query for operators and filters.
func (s *searchService) parseSearchQuery(query string) *ParsedSearchQuery {
	result := &ParsedSearchQuery{
		Text:            query,
		IncludeReplies:  true,
		IncludeRetweets: true,
	}
	
	parts := strings.Fields(query)
	var textParts []string
	
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "from:"):
			result.FromUser = strings.TrimPrefix(part, "from:")
		case strings.HasPrefix(part, "to:"):
			result.ToUser = strings.TrimPrefix(part, "to:")
		case strings.HasPrefix(part, "since:"):
			if t, err := time.Parse("2006-01-02", strings.TrimPrefix(part, "since:")); err == nil {
				result.Since = t
			}
		case strings.HasPrefix(part, "until:"):
			if t, err := time.Parse("2006-01-02", strings.TrimPrefix(part, "until:")); err == nil {
				result.Until = t
			}
		case part == "filter:replies":
			result.IncludeReplies = true
		case part == "filter:noreplies":
			result.IncludeReplies = false
		case part == "filter:retweets":
			result.IncludeRetweets = true
		case part == "filter:noretweets":
			result.IncludeRetweets = false
		case part == "filter:media":
			result.MediaOnly = true
		case strings.HasPrefix(part, "#"):
			result.Hashtags = append(result.Hashtags, part)
		default:
			textParts = append(textParts, part)
		}
	}
	
	if len(textParts) > 0 {
		result.Text = strings.Join(textParts, " ")
	}
	
	return result
}

// applyTweetFilters applies filters to tweet results.
func (s *searchService) applyTweetFilters(tweets []*entities.Tweet, filters *dto.SearchFilters) []*entities.Tweet {
	if filters == nil {
		return tweets
	}
	
	var filtered []*entities.Tweet
	
	for _, tweet := range tweets {
		// Filter by replies
		if !filters.IncludeReplies && tweet.ParentTweetID != nil && *tweet.ParentTweetID != "" {
			continue
		}
		
		// Filter by retweets
		if !filters.IncludeRetweets && tweet.RetweetOfID != nil && *tweet.RetweetOfID != "" {
			continue
		}
		
		// Filter by media
		if filters.MediaOnly && len(tweet.MediaURLs) == 0 {
			continue
		}
		
		// Filter by date
		if !filters.Since.IsZero() && tweet.CreatedAt.Before(filters.Since) {
			continue
		}
		if !filters.Until.IsZero() && tweet.CreatedAt.After(filters.Until) {
			continue
		}
		
		filtered = append(filtered, tweet)
	}
	
	return filtered
}

// buildTweetResponse builds a tweet response DTO.
func (s *searchService) buildTweetResponse(ctx context.Context, tweet *entities.Tweet, currentUserID string) (*dto.TweetResponse, error) {
	// Get user
	user, err := s.userRepo.GetByID(ctx, tweet.UserID)
	if err != nil {
		return nil, err
	}
	
	// Get counts
	likeCount, _ := s.tweetRepo.GetLikeCount(ctx, tweet.ID)
	retweetCount, _ := s.tweetRepo.GetRetweetCount(ctx, tweet.ID)
	replyCount, _ := s.tweetRepo.CountReplies(ctx, tweet.ID)
	
	// Check interaction status
	liked := false
	retweeted := false
	bookmarked := false
	
	if currentUserID != "" {
		liked, _ = s.tweetRepo.IsLiked(ctx, tweet.ID, currentUserID)
		retweeted, _ = s.tweetRepo.IsRetweeted(ctx, tweet.ID, currentUserID)
		bookmarked, _ = s.tweetRepo.IsBookmarked(ctx, tweet.ID, currentUserID)
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

// getTrendingSearchesFromCache gets trending searches from Redis.
func (s *searchService) getTrendingSearchesFromCache(ctx context.Context, limit int) ([]string, error) {
	if s.redisAdapter == nil {
		return []string{}, nil
	}
	
	results, err := s.redisAdapter.ZRevRange(ctx, "search:trending", 0, int64(limit-1))
	if err != nil {
		return []string{}, err
	}
	return results, nil
}

// getSearchCount gets the count for a search term.
func (s *searchService) getSearchCount(ctx context.Context, query string) (int64, error) {
	if s.redisAdapter == nil {
		return 0, nil
	}
	
	score, err := s.redisAdapter.ZScore(ctx, "search:trending", query)
	if err != nil {
		return 0, nil
	}
	return int64(score), nil
}

// getTotalSearches gets total search count.
func (s *searchService) getTotalSearches(ctx context.Context) (int64, error) {
	if s.redisAdapter == nil {
		return 0, nil
	}
	
	count, err := s.redisAdapter.Get(ctx, "search:total")
	if err != nil {
		return 0, nil
	}
	var total int64
	fmt.Sscanf(count, "%d", &total)
	return total, nil
}

// getUniqueSearches gets unique search count.
func (s *searchService) getUniqueSearches(ctx context.Context) (int64, error) {
	if s.redisAdapter == nil {
		return 0, nil
	}
	
	count, err := s.redisAdapter.SCard(ctx, "search:unique")
	if err != nil {
		return 0, nil
	}
	return count, nil
}

// ======================================================================
= ParsedSearchQuery Struct
// ======================================================================

// ParsedSearchQuery represents a parsed search query with filters.
type ParsedSearchQuery struct {
	Text            string
	FromUser        string
	ToUser          string
	Since           time.Time
	Until           time.Time
	IncludeReplies  bool
	IncludeRetweets bool
	MediaOnly       bool
	Hashtags        []string
}