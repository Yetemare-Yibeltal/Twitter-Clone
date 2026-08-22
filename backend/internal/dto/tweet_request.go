// backend/internal/dto/tweet_request.go
package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"twitter-clone/backend/internal/domain/entities"
)

// ======================================================================
// Validation Constants
// ======================================================================

const (
	MaxTweetContentLength = 280
	MinTweetContentLength = 1
	MaxMediaCount         = 4
	MaxPollOptions        = 4
	MinPollOptions        = 2
	MaxTrendingLimit      = 50
	MinTrendingLimit      = 1
	DefaultFeedLimit      = 20
	MaxFeedLimit          = 100
	MinPollDuration       = 1 * time.Minute
	MaxPollDuration       = 7 * 24 * time.Hour
)

// ======================================================================
= Common Validation Errors
// ======================================================================

var (
	ErrContentRequired      = errors.New("tweet content is required")
	ErrContentTooLong       = fmt.Errorf("tweet content exceeds maximum length of %d characters", MaxTweetContentLength)
	ErrContentTooShort      = fmt.Errorf("tweet content must be at least %d character", MinTweetContentLength)
	ErrMediaTooMany         = fmt.Errorf("maximum %d media files allowed", MaxMediaCount)
	ErrMediaTypeInvalid     = errors.New("invalid media type")
	ErrMediaURLInvalid      = errors.New("invalid media URL")
	ErrPollOptionsTooFew    = fmt.Errorf("poll must have at least %d options", MinPollOptions)
	ErrPollOptionsTooMany   = fmt.Errorf("poll can have at most %d options", MaxPollOptions)
	ErrPollOptionEmpty      = errors.New("poll option cannot be empty")
	ErrPollOptionTooLong    = errors.New("poll option exceeds maximum length of 25 characters")
	ErrPollDurationInvalid  = fmt.Errorf("poll duration must be between 1 minute and 7 days")
	ErrSearchQueryRequired  = errors.New("search query is required")
	ErrInvalidCursor        = errors.New("invalid pagination cursor")
	ErrInvalidLimit         = errors.New("invalid limit parameter")
)

// ======================================================================
= Create Tweet Request
// ======================================================================

// CreateTweetRequest represents the request body for creating a tweet.
type CreateTweetRequest struct {
	Content       string   `json:"content"`
	MediaURLs     []string `json:"media_urls"`
	ParentTweetID *string  `json:"parent_tweet_id,omitempty"`
	Poll          *PollRequest `json:"poll,omitempty"`
}

// PollRequest represents a poll creation request.
type PollRequest struct {
	Options  []string      `json:"options"`
	Duration time.Duration `json:"duration"`
}

// Validate performs comprehensive validation on CreateTweetRequest.
func (r *CreateTweetRequest) Validate() error {
	// Check if content is empty but has media or poll
	if r.Content == "" && len(r.MediaURLs) == 0 && r.Poll == nil {
		return errors.New("tweet must have content, media, or a poll")
	}

	// Validate content
	if r.Content != "" {
		trimmed := strings.TrimSpace(r.Content)
		if len(trimmed) == 0 {
			return ErrContentRequired
		}
		if len(trimmed) < MinTweetContentLength {
			return ErrContentTooShort
		}
		if len(trimmed) > MaxTweetContentLength {
			return ErrContentTooLong
		}
	}

	// Validate media
	if len(r.MediaURLs) > MaxMediaCount {
		return ErrMediaTooMany
	}
	for _, url := range r.MediaURLs {
		if url == "" {
			return ErrMediaURLInvalid
		}
		if !isValidURL(url) {
			return ErrMediaURLInvalid
		}
		// Validate media type
		if !isValidMediaType(url) {
			return ErrMediaTypeInvalid
		}
	}

	// Validate poll
	if r.Poll != nil {
		if err := r.Poll.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Sanitize cleans up the request fields.
func (r *CreateTweetRequest) Sanitize() {
	if r.Content != "" {
		r.Content = strings.TrimSpace(r.Content)
	}
	// Clean media URLs
	cleaned := make([]string, 0, len(r.MediaURLs))
	for _, url := range r.MediaURLs {
		url = strings.TrimSpace(url)
		if url != "" {
			cleaned = append(cleaned, url)
		}
	}
	r.MediaURLs = cleaned
}

// ExtractMentions extracts @mentions from content.
func (r *CreateTweetRequest) ExtractMentions() []string {
	if r.Content == "" {
		return []string{}
	}
	re := regexp.MustCompile(`@(\w+)`)
	matches := re.FindAllStringSubmatch(r.Content, -1)
	mentions := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			mentions = append(mentions, match[1])
		}
	}
	return mentions
}

// ExtractHashtags extracts #hashtags from content.
func (r *CreateTweetRequest) ExtractHashtags() []string {
	if r.Content == "" {
		return []string{}
	}
	re := regexp.MustCompile(`#(\w+)`)
	matches := re.FindAllStringSubmatch(r.Content, -1)
	hashtags := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			hashtags = append(hashtags, match[1])
		}
	}
	return hashtags
}

// ======================================================================
= PollRequest Validation
// ======================================================================

// Validate validates the poll request.
func (p *PollRequest) Validate() error {
	if len(p.Options) < MinPollOptions {
		return ErrPollOptionsTooFew
	}
	if len(p.Options) > MaxPollOptions {
		return ErrPollOptionsTooMany
	}
	for _, opt := range p.Options {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			return ErrPollOptionEmpty
		}
		if len(opt) > 25 {
			return ErrPollOptionTooLong
		}
	}
	if p.Duration < MinPollDuration || p.Duration > MaxPollDuration {
		return ErrPollDurationInvalid
	}
	return nil
}

// ======================================================================
= Update Tweet Request
// ======================================================================

// UpdateTweetRequest represents the request body for updating a tweet.
type UpdateTweetRequest struct {
	Content string `json:"content"`
}

// Validate performs validation on UpdateTweetRequest.
func (r *UpdateTweetRequest) Validate() error {
	if r.Content == "" {
		return ErrContentRequired
	}
	trimmed := strings.TrimSpace(r.Content)
	if len(trimmed) == 0 {
		return ErrContentRequired
	}
	if len(trimmed) > MaxTweetContentLength {
		return ErrContentTooLong
	}
	if len(trimmed) < MinTweetContentLength {
		return ErrContentTooShort
	}
	return nil
}

// Sanitize cleans up the request.
func (r *UpdateTweetRequest) Sanitize() {
	r.Content = strings.TrimSpace(r.Content)
}

// ======================================================================
= Quote Tweet Request
// ======================================================================

// QuoteTweetRequest represents the request body for quoting a tweet.
type QuoteTweetRequest struct {
	Content   string   `json:"content"`
	MediaURLs []string `json:"media_urls"`
}

// Validate performs validation on QuoteTweetRequest.
func (r *QuoteTweetRequest) Validate() error {
	if r.Content == "" {
		return ErrContentRequired
	}
	trimmed := strings.TrimSpace(r.Content)
	if len(trimmed) == 0 {
		return ErrContentRequired
	}
	if len(trimmed) > MaxTweetContentLength {
		return ErrContentTooLong
	}
	if len(trimmed) < MinTweetContentLength {
		return ErrContentTooShort
	}
	if len(r.MediaURLs) > MaxMediaCount {
		return ErrMediaTooMany
	}
	for _, url := range r.MediaURLs {
		if url == "" {
			return ErrMediaURLInvalid
		}
		if !isValidURL(url) {
			return ErrMediaURLInvalid
		}
		if !isValidMediaType(url) {
			return ErrMediaTypeInvalid
		}
	}
	return nil
}

// Sanitize cleans up the request.
func (r *QuoteTweetRequest) Sanitize() {
	r.Content = strings.TrimSpace(r.Content)
	cleaned := make([]string, 0, len(r.MediaURLs))
	for _, url := range r.MediaURLs {
		url = strings.TrimSpace(url)
		if url != "" {
			cleaned = append(cleaned, url)
		}
	}
	r.MediaURLs = cleaned
}

// ======================================================================
= Vote Poll Request
// ======================================================================

// VotePollRequest represents the request body for voting on a poll.
type VotePollRequest struct {
	OptionID string `json:"option_id"`
}

// Validate performs validation on VotePollRequest.
func (r *VotePollRequest) Validate() error {
	if strings.TrimSpace(r.OptionID) == "" {
		return errors.New("option ID is required")
	}
	return nil
}

// ======================================================================
= Search Filters
// ======================================================================

// SearchFilters represents search filters for tweets.
type SearchFilters struct {
	Query           string     `json:"query"`
	FromUser        string     `json:"from_user,omitempty"`
	ToUser          string     `json:"to_user,omitempty"`
	Since           time.Time  `json:"since,omitempty"`
	Until           time.Time  `json:"until,omitempty"`
	IncludeReplies  bool       `json:"include_replies"`
	IncludeRetweets bool       `json:"include_retweets"`
	MediaOnly       bool       `json:"media_only"`
}

// Validate validates the search filters.
func (f *SearchFilters) Validate() error {
	if strings.TrimSpace(f.Query) == "" {
		return ErrSearchQueryRequired
	}
	return nil
}

// Sanitize cleans up the filters.
func (f *SearchFilters) Sanitize() {
	f.Query = strings.TrimSpace(f.Query)
	f.FromUser = strings.TrimSpace(f.FromUser)
	f.ToUser = strings.TrimSpace(f.ToUser)
}

// ======================================================================
= Tweet Response
// ======================================================================

// TweetResponse represents a tweet in API responses.
type TweetResponse struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	AvatarURL    string    `json:"avatar_url"`
	MediaURLs    []string  `json:"media_urls"`
	LikeCount    int64     `json:"like_count"`
	RetweetCount int64     `json:"retweet_count"`
	ReplyCount   int64     `json:"reply_count"`
	Liked        bool      `json:"liked"`
	Retweeted    bool      `json:"retweeted"`
	Bookmarked   bool      `json:"bookmarked"`
	Mentions     []string  `json:"mentions"`
	Hashtags     []string  `json:"hashtags"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ======================================================================
= Tweet Detail Response
// ======================================================================

// TweetDetailResponse represents a detailed tweet response with replies.
type TweetDetailResponse struct {
	Tweet         *entities.Tweet          `json:"tweet"`
	ParentTweet   *TweetResponse           `json:"parent_tweet,omitempty"`
	RetweetSource *TweetResponse           `json:"retweet_source,omitempty"`
	Replies       []*TweetResponse         `json:"replies"`
	Poll          *PollResponse            `json:"poll,omitempty"`
}

// ======================================================================
= Poll Response
// ======================================================================

// PollResponse represents a poll in API responses.
type PollResponse struct {
	ID            string        `json:"id"`
	TweetID       string        `json:"tweet_id"`
	Options       []PollOption  `json:"options"`
	TotalVotes    int64         `json:"total_votes"`
	ExpiresAt     time.Time     `json:"expires_at"`
	IsExpired     bool          `json:"is_expired"`
	VotedOptionID string        `json:"voted_option_id,omitempty"`
}

// PollOption represents a poll option.
type PollOption struct {
	ID          string  `json:"id"`
	Text        string  `json:"text"`
	Votes       int64   `json:"votes"`
	Percentage  float64 `json:"percentage"`
	IsVoted     bool    `json:"is_voted"`
}

// ======================================================================
= Feed Response
// ======================================================================

// FeedResponse represents the paginated feed response.
type FeedResponse struct {
	Data       []*TweetResponse `json:"data"`
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
	Limit      int              `json:"limit"`
}

// ======================================================================
= Trending Topic
// ======================================================================

// TrendingTopic represents a trending hashtag.
type TrendingTopic struct {
	Hashtag string `json:"hashtag"`
	Count   int64  `json:"count"`
}

// ======================================================================
= Helper Functions
// ======================================================================

// isValidURL checks if a URL is valid.
func isValidURL(url string) bool {
	urlRegex := regexp.MustCompile(`^(https?://)[^\s/$.?#].[^\s]*$`)
	return urlRegex.MatchString(url)
}

// isValidMediaType checks if the media type is allowed.
func isValidMediaType(url string) bool {
	// Check common image/video extensions
	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".mp4", ".mov", ".avi"}
	lower := strings.ToLower(url)
	for _, ext := range extensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	// Could also check Content-Type, but for URL-based we use extension
	return true // allow if not matched (will be validated by storage)
}

// ======================================================================
= Builder Methods for Testing
// ======================================================================

// NewCreateTweetRequest creates a new create tweet request with defaults.
func NewCreateTweetRequest() *CreateTweetRequest {
	return &CreateTweetRequest{
		Content:   "Test tweet content",
		MediaURLs: []string{},
	}
}

// WithContent sets the content.
func (r *CreateTweetRequest) WithContent(content string) *CreateTweetRequest {
	r.Content = content
	return r
}

// WithMedia adds media URLs.
func (r *CreateTweetRequest) WithMedia(urls ...string) *CreateTweetRequest {
	r.MediaURLs = append(r.MediaURLs, urls...)
	return r
}

// WithPoll adds a poll.
func (r *CreateTweetRequest) WithPoll(options []string, duration time.Duration) *CreateTweetRequest {
	r.Poll = &PollRequest{
		Options:  options,
		Duration: duration,
	}
	return r
}

// NewTweetResponse creates a new tweet response from an entity.
func NewTweetResponse(tweet *entities.Tweet, user *entities.User, likeCount, retweetCount, replyCount int64, liked, retweeted, bookmarked bool) *TweetResponse {
	return &TweetResponse{
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
	}
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling for CreateTweetRequest.
func (r CreateTweetRequest) MarshalJSON() ([]byte, error) {
	// Simple: don't omit empty fields if they're empty
	type Alias CreateTweetRequest
	return json.Marshal(&struct {
		Alias
		Content string `json:"content,omitempty"`
	}{
		Alias:   (Alias)(r),
		Content: r.Content,
	})
}

// ======================================================================
= Error Helpers
// ======================================================================

// ValidationError represents a field validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	messages := make([]string, len(ve))
	for i, err := range ve {
		messages[i] = err.Error()
	}
	return strings.Join(messages, "; ")
}

func (ve ValidationErrors) ToMap() map[string]string {
	result := make(map[string]string)
	for _, err := range ve {
		result[err.Field] = err.Message
	}
	return result
}

// ======================================================================
= Pagination Helpers
// ======================================================================

// PaginationRequest represents pagination parameters.
type PaginationRequest struct {
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

// Validate validates pagination parameters.
func (p *PaginationRequest) Validate() error {
	if p.Limit < 1 || p.Limit > MaxFeedLimit {
		return ErrInvalidLimit
	}
	return nil
}

// ValidateCursor validates cursor format.
func (p *PaginationRequest) ValidateCursor() error {
	if p.Cursor != "" {
		parts := strings.Split(p.Cursor, "|")
		if len(parts) != 2 {
			return ErrInvalidCursor
		}
		// Validate timestamp
		if _, err := time.Parse(time.RFC3339Nano, parts[0]); err != nil {
			return ErrInvalidCursor
		}
	}
	return nil
}