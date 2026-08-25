// backend/internal/dto/search_request.go
package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ======================================================================
// Validation Constants
// ======================================================================

const (
	MinSearchQueryLength  = 2
	MaxSearchQueryLength  = 200
	MaxSearchResultsLimit = 100
	DefaultSearchLimit    = 20
	MaxSuggestionsLimit   = 20
	DefaultSuggestionsLimit = 10
	MaxTrendingLimit      = 50
	DefaultTrendingLimit  = 10
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrSearchQueryEmpty      = errors.New("search query is required")
	ErrSearchQueryTooShort   = fmt.Errorf("search query must be at least %d characters", MinSearchQueryLength)
	ErrSearchQueryTooLong    = fmt.Errorf("search query must be at most %d characters", MaxSearchQueryLength)
	ErrSearchInvalidFilter   = errors.New("invalid search filter")
	ErrSearchInvalidSortBy   = errors.New("invalid sort by field")
	ErrSearchInvalidSortOrder = errors.New("invalid sort order")
	ErrSearchInvalidType     = errors.New("invalid search type")
	ErrSearchInvalidDateRange = errors.New("invalid date range")
	ErrInvalidLimit          = errors.New("limit must be between 1 and 100")
	ErrInvalidCursor         = errors.New("invalid cursor format")
	ErrUserIDRequired        = errors.New("user ID is required")
	ErrSearchNoResults       = errors.New("no search results found")
	ErrSearchTimeout         = errors.New("search timed out")
)

// ======================================================================
// Search Types
// ======================================================================

// SearchType represents the type of search.
type SearchType string

const (
	SearchTypeTweets    SearchType = "tweets"
	SearchTypeUsers     SearchType = "users"
	SearchTypeHashtags  SearchType = "hashtags"
	SearchTypeAll       SearchType = "all"
	SearchTypeMedia     SearchType = "media"
	SearchTypePolls     SearchType = "polls"
	SearchTypeCommunities SearchType = "communities"
)

// ValidSearchTypes returns all valid search types.
func ValidSearchTypes() []SearchType {
	return []SearchType{
		SearchTypeTweets,
		SearchTypeUsers,
		SearchTypeHashtags,
		SearchTypeAll,
		SearchTypeMedia,
		SearchTypePolls,
		SearchTypeCommunities,
	}
}

// IsValid checks if a search type is valid.
func (t SearchType) IsValid() bool {
	for _, typ := range ValidSearchTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (t SearchType) String() string {
	return string(t)
}

// SearchSortField represents sortable fields for search results.
type SearchSortField string

const (
	SortByRelevance  SearchSortField = "relevance"
	SortByCreatedAt  SearchSortField = "created_at"
	SortByLikes      SearchSortField = "likes"
	SortByRetweets   SearchSortField = "retweets"
	SortByReplies    SearchSortField = "replies"
	SortByFollowers  SearchSortField = "followers"
	SortByEngagement SearchSortField = "engagement"
)

// ValidSearchSortFields returns all valid sort fields.
func ValidSearchSortFields() []SearchSortField {
	return []SearchSortField{
		SortByRelevance,
		SortByCreatedAt,
		SortByLikes,
		SortByRetweets,
		SortByReplies,
		SortByFollowers,
		SortByEngagement,
	}
}

// IsValid checks if a sort field is valid.
func (s SearchSortField) IsValid() bool {
	for _, field := range ValidSearchSortFields() {
		if s == field {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (s SearchSortField) String() string {
	return string(s)
}

// SearchSortOrder represents sort order.
type SearchSortOrder string

const (
	SortAsc  SearchSortOrder = "asc"
	SortDesc SearchSortOrder = "desc"
)

// IsValid checks if a sort order is valid.
func (o SearchSortOrder) IsValid() bool {
	return o == SortAsc || o == SortDesc
}

// String returns the string representation.
func (o SearchSortOrder) String() string {
	return string(o)
}

// ======================================================================
// Request DTOs
// ======================================================================

// SearchRequest represents the main search request.
type SearchRequest struct {
	Query       string            `json:"q" binding:"required"`
	Type        SearchType        `json:"type,omitempty"`
	Filters     SearchFilters     `json:"filters,omitempty"`
	Cursor      string            `json:"cursor,omitempty"`
	Limit       int               `json:"limit,omitempty"`
	SortBy      SearchSortField   `json:"sort_by,omitempty"`
	SortOrder   SearchSortOrder   `json:"sort_order,omitempty"`
	IncludeDeleted bool           `json:"include_deleted,omitempty"`
	Timeout     int               `json:"timeout,omitempty"` // timeout in seconds
}

// SearchFilters represents search filters.
type SearchFilters struct {
	FromUser        []string   `json:"from_user,omitempty"`
	ToUser          []string   `json:"to_user,omitempty"`
	MentioningUser  string     `json:"mentioning_user,omitempty"`
	Hashtags        []string   `json:"hashtags,omitempty"`
	MediaOnly       bool       `json:"media_only,omitempty"`
	PollOnly        bool       `json:"poll_only,omitempty"`
	ReplyOnly       bool       `json:"reply_only,omitempty"`
	RetweetOnly     bool       `json:"retweet_only,omitempty"`
	QuoteOnly       bool       `json:"quote_only,omitempty"`
	MinLikes        int64      `json:"min_likes,omitempty"`
	MaxLikes        int64      `json:"max_likes,omitempty"`
	MinRetweets     int64      `json:"min_retweets,omitempty"`
	MaxRetweets     int64      `json:"max_retweets,omitempty"`
	MinReplies      int64      `json:"min_replies,omitempty"`
	MaxReplies      int64      `json:"max_replies,omitempty"`
	Since           *time.Time `json:"since,omitempty"`
	Until           *time.Time `json:"until,omitempty"`
	Language        string     `json:"language,omitempty"`
	Location        *LocationFilter `json:"location,omitempty"`
}

// LocationFilter represents geo-location search.
type LocationFilter struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Radius    float64 `json:"radius"` // in kilometers
}

// Validate validates the search request.
func (r *SearchRequest) Validate() error {
	query := strings.TrimSpace(r.Query)
	if query == "" {
		return ErrSearchQueryEmpty
	}
	if len(query) < MinSearchQueryLength {
		return ErrSearchQueryTooShort
	}
	if len(query) > MaxSearchQueryLength {
		return ErrSearchQueryTooLong
	}
	r.Query = query
	if r.Type != "" && !r.Type.IsValid() {
		return ErrSearchInvalidType
	}
	if r.Limit < 0 || r.Limit > MaxSearchResultsLimit {
		return ErrInvalidLimit
	}
	if r.SortBy != "" && !r.SortBy.IsValid() {
		return ErrSearchInvalidSortBy
	}
	if r.SortOrder != "" && !r.SortOrder.IsValid() {
		return ErrSearchInvalidSortOrder
	}
	if err := r.Filters.Validate(); err != nil {
		return err
	}
	if r.Timeout < 0 || r.Timeout > 30 {
		return errors.New("timeout must be between 0 and 30 seconds")
	}
	return nil
}

// Sanitize sanitizes the search request.
func (r *SearchRequest) Sanitize() {
	r.Query = strings.TrimSpace(r.Query)
	if r.Limit < 1 {
		r.Limit = DefaultSearchLimit
	}
	if r.Limit > MaxSearchResultsLimit {
		r.Limit = MaxSearchResultsLimit
	}
	if r.Timeout < 1 {
		r.Timeout = 10
	}
	if r.Type == "" {
		r.Type = SearchTypeAll
	}
	r.Cursor = strings.TrimSpace(r.Cursor)
	r.Filters.Sanitize()
}

// Validate validates the search filters.
func (f *SearchFilters) Validate() error {
	if f.MinLikes > 0 && f.MaxLikes > 0 && f.MinLikes > f.MaxLikes {
		return errors.New("min_likes cannot be greater than max_likes")
	}
	if f.MinRetweets > 0 && f.MaxRetweets > 0 && f.MinRetweets > f.MaxRetweets {
		return errors.New("min_retweets cannot be greater than max_retweets")
	}
	if f.MinReplies > 0 && f.MaxReplies > 0 && f.MinReplies > f.MaxReplies {
		return errors.New("min_replies cannot be greater than max_replies")
	}
	if f.Since != nil && f.Until != nil && f.Since.After(*f.Until) {
		return ErrSearchInvalidDateRange
	}
	if f.Location != nil {
		if f.Location.Radius <= 0 {
			return errors.New("radius must be greater than 0")
		}
		if f.Location.Radius > 10000 {
			return errors.New("radius cannot exceed 10000 km")
		}
	}
	return nil
}

// Sanitize sanitizes the search filters.
func (f *SearchFilters) Sanitize() {
	cleanedFrom := make([]string, 0, len(f.FromUser))
	for _, u := range f.FromUser {
		if trimmed := strings.TrimSpace(u); trimmed != "" {
			cleanedFrom = append(cleanedFrom, trimmed)
		}
	}
	f.FromUser = cleanedFrom
	cleanedTo := make([]string, 0, len(f.ToUser))
	for _, u := range f.ToUser {
		if trimmed := strings.TrimSpace(u); trimmed != "" {
			cleanedTo = append(cleanedTo, trimmed)
		}
	}
	f.ToUser = cleanedTo
	cleanedHashtags := make([]string, 0, len(f.Hashtags))
	for _, h := range f.Hashtags {
		if trimmed := strings.TrimSpace(h); trimmed != "" {
			cleanedHashtags = append(cleanedHashtags, strings.ToLower(trimmed))
		}
	}
	f.Hashtags = cleanedHashtags
	f.MentioningUser = strings.TrimSpace(f.MentioningUser)
	f.Language = strings.TrimSpace(f.Language)
}

// AdvancedSearchRequest represents an advanced search request.
type AdvancedSearchRequest struct {
	Query        string   `json:"q" binding:"required"`
	Include      []string `json:"include,omitempty"`
	Exclude      []string `json:"exclude,omitempty"`
	Must         []string `json:"must,omitempty"`
	Should       []string `json:"should,omitempty"`
	MustNot      []string `json:"must_not,omitempty"`
	MatchPhrase  string   `json:"match_phrase,omitempty"`
	Fuzzy        bool     `json:"fuzzy,omitempty"`
	Synonym      bool     `json:"synonym,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	Offset       int      `json:"offset,omitempty"`
	SortBy       string   `json:"sort_by,omitempty"`
	SortOrder    string   `json:"sort_order,omitempty"`
}

// Validate validates the advanced search request.
func (r *AdvancedSearchRequest) Validate() error {
	if strings.TrimSpace(r.Query) == "" {
		return ErrSearchQueryEmpty
	}
	if len(r.Query) < MinSearchQueryLength {
		return ErrSearchQueryTooShort
	}
	if len(r.Query) > MaxSearchQueryLength {
		return ErrSearchQueryTooLong
	}
	if r.Limit < 0 || r.Limit > MaxSearchResultsLimit {
		return ErrInvalidLimit
	}
	if r.Offset < 0 {
		return errors.New("offset cannot be negative")
	}
	if r.SortBy != "" {
		if !SearchSortField(r.SortBy).IsValid() {
			return ErrSearchInvalidSortBy
		}
	}
	if r.SortOrder != "" && !SearchSortOrder(r.SortOrder).IsValid() {
		return ErrSearchInvalidSortOrder
	}
	return nil
}

// Sanitize sanitizes the advanced search request.
func (r *AdvancedSearchRequest) Sanitize() {
	r.Query = strings.TrimSpace(r.Query)
	cleanedInclude := make([]string, 0, len(r.Include))
	for _, v := range r.Include {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			cleanedInclude = append(cleanedInclude, trimmed)
		}
	}
	r.Include = cleanedInclude
	cleanedExclude := make([]string, 0, len(r.Exclude))
	for _, v := range r.Exclude {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			cleanedExclude = append(cleanedExclude, trimmed)
		}
	}
	r.Exclude = cleanedExclude
	cleanedMust := make([]string, 0, len(r.Must))
	for _, v := range r.Must {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			cleanedMust = append(cleanedMust, trimmed)
		}
	}
	r.Must = cleanedMust
	cleanedShould := make([]string, 0, len(r.Should))
	for _, v := range r.Should {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			cleanedShould = append(cleanedShould, trimmed)
		}
	}
	r.Should = cleanedShould
	cleanedMustNot := make([]string, 0, len(r.MustNot))
	for _, v := range r.MustNot {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			cleanedMustNot = append(cleanedMustNot, trimmed)
		}
	}
	r.MustNot = cleanedMustNot
	if r.Limit < 1 {
		r.Limit = DefaultSearchLimit
	}
	if r.Limit > MaxSearchResultsLimit {
		r.Limit = MaxSearchResultsLimit
	}
	if r.SortBy == "" {
		r.SortBy = string(SortByRelevance)
	}
	if r.SortOrder == "" {
		r.SortOrder = string(SortDesc)
	}
}

// GetSearchSuggestionsRequest represents the request for search suggestions.
type GetSearchSuggestionsRequest struct {
	Query  string `json:"q" binding:"required"`
	Limit  int    `json:"limit,omitempty"`
	Type   string `json:"type,omitempty"` // "users", "hashtags", "trending", "all"
	UserID string `json:"user_id,omitempty"`
}

// Validate validates the search suggestions request.
func (r *GetSearchSuggestionsRequest) Validate() error {
	if strings.TrimSpace(r.Query) == "" {
		return ErrSearchQueryEmpty
	}
	if len(r.Query) < 1 {
		return errors.New("query is too short for suggestions")
	}
	if r.Limit < 0 || r.Limit > MaxSuggestionsLimit {
		return errors.New("limit must be between 1 and 20")
	}
	if r.Type != "" {
		validTypes := map[string]bool{"users": true, "hashtags": true, "trending": true, "all": true}
		if !validTypes[r.Type] {
			return errors.New("invalid suggestion type")
		}
	}
	return nil
}

// Sanitize sanitizes the search suggestions request.
func (r *GetSearchSuggestionsRequest) Sanitize() {
	r.Query = strings.TrimSpace(r.Query)
	if r.Limit < 1 {
		r.Limit = DefaultSuggestionsLimit
	}
	if r.Limit > MaxSuggestionsLimit {
		r.Limit = MaxSuggestionsLimit
	}
	r.Type = strings.TrimSpace(r.Type)
	if r.Type == "" {
		r.Type = "all"
	}
	r.UserID = strings.TrimSpace(r.UserID)
}

// GetTrendingSearchesRequest represents the request for trending searches.
type GetTrendingSearchesRequest struct {
	Limit     int       `json:"limit,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Category  string    `json:"category,omitempty"` // "all", "tweets", "users", "hashtags"
	Location  string    `json:"location,omitempty"`
}

// Validate validates the trending searches request.
func (r *GetTrendingSearchesRequest) Validate() error {
	if r.Limit < 0 || r.Limit > MaxTrendingLimit {
		return errors.New("limit must be between 1 and 50")
	}
	if r.Category != "" {
		validCategories := map[string]bool{"all": true, "tweets": true, "users": true, "hashtags": true}
		if !validCategories[r.Category] {
			return errors.New("invalid category")
		}
	}
	return nil
}

// Sanitize sanitizes the trending searches request.
func (r *GetTrendingSearchesRequest) Sanitize() {
	if r.Limit < 1 {
		r.Limit = DefaultTrendingLimit
	}
	if r.Limit > MaxTrendingLimit {
		r.Limit = MaxTrendingLimit
	}
	if r.Category == "" {
		r.Category = "all"
	}
	r.Location = strings.TrimSpace(r.Location)
}

// RecordSearchRequest represents the request to record a search.
type RecordSearchRequest struct {
	Query       string `json:"query" binding:"required"`
	UserID      string `json:"user_id,omitempty"`
	ResultCount int    `json:"result_count,omitempty"`
	SearchType  string `json:"search_type,omitempty"`
	IP          string `json:"ip,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`
}

// Validate validates the record search request.
func (r *RecordSearchRequest) Validate() error {
	if strings.TrimSpace(r.Query) == "" {
		return ErrSearchQueryEmpty
	}
	if r.ResultCount < 0 {
		return errors.New("result_count cannot be negative")
	}
	if r.SearchType != "" && !SearchType(r.SearchType).IsValid() {
		return ErrSearchInvalidType
	}
	return nil
}

// Sanitize sanitizes the record search request.
func (r *RecordSearchRequest) Sanitize() {
	r.Query = strings.TrimSpace(r.Query)
	r.UserID = strings.TrimSpace(r.UserID)
	r.SearchType = strings.TrimSpace(r.SearchType)
	r.IP = strings.TrimSpace(r.IP)
	r.UserAgent = strings.TrimSpace(r.UserAgent)
	if r.SearchType == "" {
		r.SearchType = string(SearchTypeAll)
	}
}

// ======================================================================
// Response DTOs
// ======================================================================

// SearchResult represents a single search result.
type SearchResult struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Score       float64                `json:"score"`
	Data        map[string]interface{} `json:"data"`
	Highlights  map[string]string      `json:"highlights,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// SearchResponse represents the main search response.
type SearchResponse struct {
	Results        []SearchResult            `json:"results"`
	Total          int64                     `json:"total"`
	NextCursor     string                    `json:"next_cursor"`
	HasMore        bool                      `json:"has_more"`
	Limit          int                       `json:"limit"`
	SearchTimeMs   int64                     `json:"search_time_ms"`
	Facets         map[string]map[string]int64 `json:"facets,omitempty"`
	Query          string                    `json:"query"`
	Type           string                    `json:"type"`
}

// TweetSearchResponse represents tweet search response.
type TweetSearchResponse struct {
	Data       []TweetResponse `json:"data"`
	Total      int64           `json:"total"`
	NextCursor string          `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
	Limit      int             `json:"limit"`
	Query      string          `json:"query"`
}

// UserSearchResponse represents user search response.
type UserSearchResponse struct {
	Data       []UserSearchResult `json:"data"`
	Total      int64              `json:"total"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
	Limit      int                `json:"limit"`
	Query      string             `json:"query"`
}

// UserSearchResult represents a user in search results.
type UserSearchResult struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	FullName    string `json:"full_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Bio         string `json:"bio,omitempty"`
	IsVerified  bool   `json:"is_verified"`
	IsFollowing bool   `json:"is_following"`
	IsMutual    bool   `json:"is_mutual"`
	FollowerCount int64 `json:"follower_count"`
	TweetCount  int64  `json:"tweet_count"`
	Score       float64 `json:"score"`
}

// HashtagSearchResponse represents hashtag search response.
type HashtagSearchResponse struct {
	Data       []HashtagResult `json:"data"`
	Total      int64           `json:"total"`
	NextCursor string          `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
	Limit      int             `json:"limit"`
	Query      string          `json:"query"`
}

// HashtagResult represents a hashtag in search results.
type HashtagResult struct {
	Hashtag    string    `json:"hashtag"`
	Count      int64     `json:"count"`
	Trending   bool      `json:"trending"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
	Score      float64   `json:"score"`
}

// CombinedSearchResponse represents combined search results.
type CombinedSearchResponse struct {
	Tweets       []TweetResponse         `json:"tweets"`
	TweetCount   int64                   `json:"tweet_count"`
	Users        []UserSearchResult      `json:"users"`
	UserCount    int64                   `json:"user_count"`
	Hashtags     []HashtagResult         `json:"hashtags"`
	HashtagCount int64                   `json:"hashtag_count"`
	TotalResults int64                   `json:"total_results"`
	Query        string                  `json:"query"`
	Facets       map[string]map[string]int64 `json:"facets,omitempty"`
}

// SearchSuggestionsResponse represents search suggestions.
type SearchSuggestionsResponse struct {
	Suggestions []SearchSuggestion `json:"suggestions"`
	Query       string             `json:"query"`
	Total       int64              `json:"total"`
}

// SearchSuggestion represents a single search suggestion.
type SearchSuggestion struct {
	Text     string                 `json:"text"`
	Type     string                 `json:"type"` // "user", "hashtag", "trending"
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TrendingSearchResponse represents a trending search.
type TrendingSearchResponse struct {
	Term      string    `json:"term"`
	Count     int64     `json:"count"`
	Position  int       `json:"position"`
	Category  string    `json:"category"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// TrendingSearchesResponse represents trending searches.
type TrendingSearchesResponse struct {
	Data       []TrendingSearchResponse `json:"data"`
	Total      int64                    `json:"total"`
	Limit      int                      `json:"limit"`
	Category   string                   `json:"category"`
	Location   string                   `json:"location,omitempty"`
}

// SearchStatsResponse represents search statistics.
type SearchStatsResponse struct {
	TotalSearches   int64               `json:"total_searches"`
	UniqueSearches  int64               `json:"unique_searches"`
	TotalResults    int64               `json:"total_results"`
	AvgResultsPerSearch float64         `json:"avg_results_per_search"`
	AvgSearchTimeMs float64             `json:"avg_search_time_ms"`
	TopQueries      []string            `json:"top_queries"`
	DailyCounts     []DailySearchCount  `json:"daily_counts"`
	SearchByType    map[string]int64    `json:"search_by_type"`
	PopularKeywords []string            `json:"popular_keywords"`
}

// DailySearchCount represents daily search counts.
type DailySearchCount struct {
	Date   time.Time `json:"date"`
	Count  int64     `json:"count"`
	Unique int64     `json:"unique"`
}

// ======================================================================
// Builder Methods for SearchResponse
// ======================================================================

// NewSearchResponse creates a new search response.
func NewSearchResponse(query, searchType string, limit int) *SearchResponse {
	return &SearchResponse{
		Query:      query,
		Type:       searchType,
		Limit:      limit,
		Results:    []SearchResult{},
		Total:      0,
		HasMore:    false,
		Facets:     make(map[string]map[string]int64),
	}
}

// AddResult adds a search result.
func (r *SearchResponse) AddResult(result SearchResult) {
	r.Results = append(r.Results, result)
}

// WithTotal sets the total count.
func (r *SearchResponse) WithTotal(total int64) *SearchResponse {
	r.Total = total
	return r
}

// WithNextCursor sets the next cursor.
func (r *SearchResponse) WithNextCursor(cursor string) *SearchResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithSearchTime sets the search time.
func (r *SearchResponse) WithSearchTime(ms int64) *SearchResponse {
	r.SearchTimeMs = ms
	return r
}

// AddFacet adds a facet value.
func (r *SearchResponse) AddFacet(facet string, value string, count int64) {
	if r.Facets[facet] == nil {
		r.Facets[facet] = make(map[string]int64)
	}
	r.Facets[facet][value] = count
}

// ======================================================================
// Builder Methods for SearchSuggestionsResponse
// ======================================================================

// NewSearchSuggestionsResponse creates a new search suggestions response.
func NewSearchSuggestionsResponse(query string) *SearchSuggestionsResponse {
	return &SearchSuggestionsResponse{
		Query:       query,
		Suggestions: []SearchSuggestion{},
		Total:       0,
	}
}

// AddSuggestion adds a suggestion.
func (r *SearchSuggestionsResponse) AddSuggestion(text, suggestionType string, score float64) {
	r.Suggestions = append(r.Suggestions, SearchSuggestion{
		Text:  text,
		Type:  suggestionType,
		Score: score,
	})
}

// WithTotal sets the total count.
func (r *SearchSuggestionsResponse) WithTotal(total int64) *SearchSuggestionsResponse {
	r.Total = total
	return r
}

// ======================================================================
// Builder Methods for TrendingSearchesResponse
// ======================================================================

// NewTrendingSearchesResponse creates a new trending searches response.
func NewTrendingSearchesResponse(category, location string, limit int) *TrendingSearchesResponse {
	return &TrendingSearchesResponse{
		Data:     []TrendingSearchResponse{},
		Total:    0,
		Limit:    limit,
		Category: category,
		Location: location,
	}
}

// AddTrending adds a trending search.
func (r *TrendingSearchesResponse) AddTrending(term string, count int64, position int, category string, firstSeen, lastSeen time.Time) {
	r.Data = append(r.Data, TrendingSearchResponse{
		Term:      term,
		Count:     count,
		Position:  position,
		Category:  category,
		FirstSeen: firstSeen,
		LastSeen:  lastSeen,
	})
}

// WithTotal sets the total count.
func (r *TrendingSearchesResponse) WithTotal(total int64) *TrendingSearchesResponse {
	r.Total = total
	return r
}

// ======================================================================
// Builder Methods for SearchStatsResponse
// ======================================================================

// NewSearchStatsResponse creates a new search stats response.
func NewSearchStatsResponse() *SearchStatsResponse {
	return &SearchStatsResponse{
		SearchByType:    make(map[string]int64),
		DailyCounts:     []DailySearchCount{},
		TopQueries:      []string{},
		PopularKeywords: []string{},
	}
}

// WithTotalSearches sets the total searches.
func (r *SearchStatsResponse) WithTotalSearches(total int64) *SearchStatsResponse {
	r.TotalSearches = total
	return r
}

// WithUniqueSearches sets the unique searches.
func (r *SearchStatsResponse) WithUniqueSearches(unique int64) *SearchStatsResponse {
	r.UniqueSearches = unique
	return r
}

// WithTotalResults sets the total results.
func (r *SearchStatsResponse) WithTotalResults(total int64) *SearchStatsResponse {
	r.TotalResults = total
	return r
}

// WithAvgResults sets the average results per search.
func (r *SearchStatsResponse) WithAvgResults(avg float64) *SearchStatsResponse {
	r.AvgResultsPerSearch = avg
	return r
}

// WithAvgSearchTime sets the average search time.
func (r *SearchStatsResponse) WithAvgSearchTime(ms float64) *SearchStatsResponse {
	r.AvgSearchTimeMs = ms
	return r
}

// AddTypeStat adds a search type statistic.
func (r *SearchStatsResponse) AddTypeStat(searchType string, count int64) {
	r.SearchByType[searchType] = count
}

// WithDailyCounts sets the daily counts.
func (r *SearchStatsResponse) WithDailyCounts(counts []DailySearchCount) *SearchStatsResponse {
	r.DailyCounts = counts
	return r
}

// WithTopQueries sets the top queries.
func (r *SearchStatsResponse) WithTopQueries(queries []string) *SearchStatsResponse {
	r.TopQueries = queries
	return r
}

// WithPopularKeywords sets the popular keywords.
func (r *SearchStatsResponse) WithPopularKeywords(keywords []string) *SearchStatsResponse {
	r.PopularKeywords = keywords
	return r
}

// ======================================================================
// JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *SearchRequest) MarshalJSON() ([]byte, error) {
	type Alias SearchRequest
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(r),
		Type:  string(r.Type),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *SearchRequest) UnmarshalJSON(data []byte) error {
	type Alias SearchRequest
	aux := &struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Type != "" {
		r.Type = SearchType(aux.Type)
	}
	return nil
}

// ======================================================================
// Test Helpers
// ======================================================================

// NewTestSearchRequest creates a test search request.
func NewTestSearchRequest() *SearchRequest {
	return &SearchRequest{
		Query:     "golang",
		Type:      SearchTypeAll,
		Limit:     20,
		SortBy:    SortByRelevance,
		SortOrder: SortDesc,
	}
}

// NewTestAdvancedSearchRequest creates a test advanced search request.
func NewTestAdvancedSearchRequest() *AdvancedSearchRequest {
	return &AdvancedSearchRequest{
		Query:    "golang",
		Must:     []string{"tutorial"},
		Should:   []string{"beginner", "advanced"},
		Limit:    20,
		SortBy:   string(SortByRelevance),
		SortOrder: string(SortDesc),
	}
}

// NewTestSearchSuggestionsRequest creates a test suggestions request.
func NewTestSearchSuggestionsRequest() *GetSearchSuggestionsRequest {
	return &GetSearchSuggestionsRequest{
		Query: "gola",
		Limit: 10,
		Type:  "all",
	}
}

// NewTestSearchResponse creates a test search response.
func NewTestSearchResponse() *SearchResponse {
	resp := NewSearchResponse("golang", "all", 20)
	result := SearchResult{
		ID:    "tweet1",
		Type:  "tweet",
		Score: 0.95,
		Data: map[string]interface{}{
			"content": "Learning Golang is awesome!",
			"user":    "john_doe",
		},
		CreatedAt: time.Now().UTC(),
	}
	resp.AddResult(result)
	resp.WithTotal(100).WithNextCursor("cursor123").WithSearchTime(45)
	return resp
}

// NewTestTrendingSearchesResponse creates a test trending response.
func NewTestTrendingSearchesResponse() *TrendingSearchesResponse {
	resp := NewTrendingSearchesResponse("all", "US", 10)
	resp.AddTrending("golang", 1000, 1, "tweets", time.Now().UTC(), time.Now().UTC())
	resp.AddTrending("programming", 800, 2, "tweets", time.Now().UTC(), time.Now().UTC())
	resp.WithTotal(2)
	return resp
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagSearch = "Search"
)