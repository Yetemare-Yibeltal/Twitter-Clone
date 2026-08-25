// backend/internal/dto/tweet_response.go
package dto

import (
	"encoding/json"
	"time"
)

// ======================================================================
// Tweet Response DTOs
// ======================================================================

// TweetResponse represents a tweet in API responses.
type TweetResponse struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	MediaURLs    []string  `json:"media_urls,omitempty"`
	LikeCount    int64     `json:"like_count"`
	RetweetCount int64     `json:"retweet_count"`
	ReplyCount   int64     `json:"reply_count"`
	Liked        bool      `json:"liked"`
	Retweeted    bool      `json:"retweeted"`
	Bookmarked   bool      `json:"bookmarked"`
	Mentions     []string  `json:"mentions,omitempty"`
	Hashtags     []string  `json:"hashtags,omitempty"`
	IsPoll       bool      `json:"is_poll"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TweetDetailResponse represents a detailed tweet with replies.
type TweetDetailResponse struct {
	Tweet         *TweetResponse   `json:"tweet"`
	ParentTweet   *TweetResponse   `json:"parent_tweet,omitempty"`
	RetweetSource *TweetResponse   `json:"retweet_source,omitempty"`
	Replies       []*TweetResponse `json:"replies"`
	Poll          *PollResponse    `json:"poll,omitempty"`
}

// TweetListResponse represents a paginated list of tweets.
type TweetListResponse struct {
	Data       []*TweetResponse `json:"data"`
	Total      int64            `json:"total"`
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
	Limit      int              `json:"limit"`
}

// FeedResponse represents the feed response.
type FeedResponse struct {
	Data       []*TweetResponse `json:"data"`
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
	Limit      int              `json:"limit"`
}

// ======================================================================
// Builder Methods for TweetResponse
// ======================================================================

// NewTweetResponse creates a new tweet response.
func NewTweetResponse() *TweetResponse {
	return &TweetResponse{
		MediaURLs:    []string{},
		Mentions:     []string{},
		Hashtags:     []string{},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}

// WithID sets the tweet ID.
func (r *TweetResponse) WithID(id string) *TweetResponse {
	r.ID = id
	return r
}

// WithContent sets the content.
func (r *TweetResponse) WithContent(content string) *TweetResponse {
	r.Content = content
	return r
}

// WithUserID sets the user ID.
func (r *TweetResponse) WithUserID(userID string) *TweetResponse {
	r.UserID = userID
	return r
}

// WithUsername sets the username.
func (r *TweetResponse) WithUsername(username string) *TweetResponse {
	r.Username = username
	return r
}

// WithFullName sets the full name.
func (r *TweetResponse) WithFullName(fullName string) *TweetResponse {
	r.FullName = fullName
	return r
}

// WithAvatarURL sets the avatar URL.
func (r *TweetResponse) WithAvatarURL(url string) *TweetResponse {
	r.AvatarURL = url
	return r
}

// WithMediaURLs sets the media URLs.
func (r *TweetResponse) WithMediaURLs(urls ...string) *TweetResponse {
	r.MediaURLs = append(r.MediaURLs, urls...)
	return r
}

// WithCounts sets like, retweet, and reply counts.
func (r *TweetResponse) WithCounts(likes, retweets, replies int64) *TweetResponse {
	r.LikeCount = likes
	r.RetweetCount = retweets
	r.ReplyCount = replies
	return r
}

// WithLiked sets the liked status.
func (r *TweetResponse) WithLiked(liked bool) *TweetResponse {
	r.Liked = liked
	return r
}

// WithRetweeted sets the retweeted status.
func (r *TweetResponse) WithRetweeted(retweeted bool) *TweetResponse {
	r.Retweeted = retweeted
	return r
}

// WithBookmarked sets the bookmarked status.
func (r *TweetResponse) WithBookmarked(bookmarked bool) *TweetResponse {
	r.Bookmarked = bookmarked
	return r
}

// WithMentions sets the mentions.
func (r *TweetResponse) WithMentions(mentions ...string) *TweetResponse {
	r.Mentions = append(r.Mentions, mentions...)
	return r
}

// WithHashtags sets the hashtags.
func (r *TweetResponse) WithHashtags(hashtags ...string) *TweetResponse {
	r.Hashtags = append(r.Hashtags, hashtags...)
	return r
}

// WithPoll sets the poll flag.
func (r *TweetResponse) WithPoll(isPoll bool) *TweetResponse {
	r.IsPoll = isPoll
	return r
}

// WithCreatedAt sets the creation time.
func (r *TweetResponse) WithCreatedAt(t time.Time) *TweetResponse {
	r.CreatedAt = t
	return r
}

// WithUpdatedAt sets the update time.
func (r *TweetResponse) WithUpdatedAt(t time.Time) *TweetResponse {
	r.UpdatedAt = t
	return r
}

// ======================================================================
// Builder Methods for TweetDetailResponse
// ======================================================================

// NewTweetDetailResponse creates a new tweet detail response.
func NewTweetDetailResponse() *TweetDetailResponse {
	return &TweetDetailResponse{
		Replies: []*TweetResponse{},
	}
}

// WithTweet sets the main tweet.
func (r *TweetDetailResponse) WithTweet(tweet *TweetResponse) *TweetDetailResponse {
	r.Tweet = tweet
	return r
}

// WithParentTweet sets the parent tweet.
func (r *TweetDetailResponse) WithParentTweet(parent *TweetResponse) *TweetDetailResponse {
	r.ParentTweet = parent
	return r
}

// WithRetweetSource sets the retweet source.
func (r *TweetDetailResponse) WithRetweetSource(source *TweetResponse) *TweetDetailResponse {
	r.RetweetSource = source
	return r
}

// WithReplies sets the replies.
func (r *TweetDetailResponse) WithReplies(replies []*TweetResponse) *TweetDetailResponse {
	r.Replies = replies
	return r
}

// WithPoll sets the poll.
func (r *TweetDetailResponse) WithPoll(poll *PollResponse) *TweetDetailResponse {
	r.Poll = poll
	return r
}

// ======================================================================
// Builder Methods for TweetListResponse
// ======================================================================

// NewTweetListResponse creates a new tweet list response.
func NewTweetListResponse() *TweetListResponse {
	return &TweetListResponse{
		Data: []*TweetResponse{},
	}
}

// Add adds a tweet to the response.
func (r *TweetListResponse) Add(tweet *TweetResponse) {
	r.Data = append(r.Data, tweet)
}

// WithTotal sets the total count.
func (r *TweetListResponse) WithTotal(total int64) *TweetListResponse {
	r.Total = total
	return r
}

// WithNextCursor sets the next cursor.
func (r *TweetListResponse) WithNextCursor(cursor string) *TweetListResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *TweetListResponse) WithLimit(limit int) *TweetListResponse {
	r.Limit = limit
	return r
}

// ======================================================================
// Builder Methods for FeedResponse
// ======================================================================

// NewFeedResponse creates a new feed response.
func NewFeedResponse() *FeedResponse {
	return &FeedResponse{
		Data: []*TweetResponse{},
	}
}

// Add adds a tweet to the feed.
func (r *FeedResponse) Add(tweet *TweetResponse) {
	r.Data = append(r.Data, tweet)
}

// WithNextCursor sets the next cursor.
func (r *FeedResponse) WithNextCursor(cursor string) *FeedResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *FeedResponse) WithLimit(limit int) *FeedResponse {
	r.Limit = limit
	return r
}

// ======================================================================
// Conversion Helpers
// ======================================================================

// ToTweetResponse converts tweet data to a response.
func ToTweetResponse(id, content, userID, username, fullName, avatarURL string, mediaURLs []string, likeCount, retweetCount, replyCount int64, liked, retweeted, bookmarked bool, mentions, hashtags []string, isPoll bool, createdAt, updatedAt time.Time) *TweetResponse {
	return &TweetResponse{
		ID:           id,
		Content:      content,
		UserID:       userID,
		Username:     username,
		FullName:     fullName,
		AvatarURL:    avatarURL,
		MediaURLs:    mediaURLs,
		LikeCount:    likeCount,
		RetweetCount: retweetCount,
		ReplyCount:   replyCount,
		Liked:        liked,
		Retweeted:    retweeted,
		Bookmarked:   bookmarked,
		Mentions:     mentions,
		Hashtags:     hashtags,
		IsPoll:       isPoll,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

// ToTweetListResponse converts a slice of tweets to a list response.
func ToTweetListResponse(tweets []*TweetResponse, total int64, nextCursor string, limit int) *TweetListResponse {
	return &TweetListResponse{
		Data:       tweets,
		Total:      total,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
		Limit:      limit,
	}
}

// ======================================================================
// JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *TweetResponse) MarshalJSON() ([]byte, error) {
	type Alias TweetResponse
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias:     (*Alias)(r),
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
		UpdatedAt: r.UpdatedAt.Format(time.RFC3339),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *TweetResponse) UnmarshalJSON(data []byte) error {
	type Alias TweetResponse
	aux := &struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, aux.CreatedAt)
		if err == nil {
			r.CreatedAt = t
		}
	}
	if aux.UpdatedAt != "" {
		t, err := time.Parse(time.RFC3339, aux.UpdatedAt)
		if err == nil {
			r.UpdatedAt = t
		}
	}
	return nil
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestTweetResponse creates a test tweet response.
func NewTestTweetResponse() *TweetResponse {
	return NewTweetResponse().
		WithID("tweet1").
		WithContent("Hello world!").
		WithUserID("user1").
		WithUsername("john_doe").
		WithFullName("John Doe").
		WithAvatarURL("https://example.com/avatar.jpg").
		WithMediaURLs("https://example.com/image.jpg").
		WithCounts(10, 5, 3).
		WithLiked(true).
		WithRetweeted(false).
		WithBookmarked(true).
		WithMentions("jane").
		WithHashtags("hello", "world").
		WithPoll(false)
}

// NewTestTweetListResponse creates a test tweet list response.
func NewTestTweetListResponse() *TweetListResponse {
	list := NewTweetListResponse()
	list.Add(NewTestTweetResponse())
	list.WithTotal(1).WithNextCursor("cursor123").WithLimit(20)
	return list
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagTweets = "Tweets"
)