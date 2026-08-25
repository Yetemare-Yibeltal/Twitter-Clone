// backend/internal/dto/user_response.go
package dto

import (
	"encoding/json"
	"time"
)

// ======================================================================
// User Response DTOs
// ======================================================================

// UserResponse represents a user in API responses.
type UserResponse struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	FullName      string     `json:"full_name"`
	Bio           string     `json:"bio,omitempty"`
	AvatarURL     string     `json:"avatar_url,omitempty"`
	BannerURL     string     `json:"banner_url,omitempty"`
	Location      string     `json:"location,omitempty"`
	Website       string     `json:"website,omitempty"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	IsVerified    bool       `json:"is_verified"`
	IsPrivate     bool       `json:"is_private"`
	TweetCount    int64      `json:"tweet_count"`
	FollowerCount int64      `json:"follower_count"`
	FollowingCount int64     `json:"following_count"`
	IsFollowing   bool       `json:"is_following"`
	IsMutual      bool       `json:"is_mutual"`
	JoinedAt      time.Time  `json:"joined_at"`
	LastActive    *time.Time `json:"last_active_at,omitempty"`
}

// MinimalUserResponse represents minimal user data.
type MinimalUserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url,omitempty"`
	IsVerified bool  `json:"is_verified"`
}

// UserProfileResponse represents a user's profile with additional stats.
type UserProfileResponse struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	FullName      string     `json:"full_name"`
	Bio           string     `json:"bio,omitempty"`
	AvatarURL     string     `json:"avatar_url,omitempty"`
	BannerURL     string     `json:"banner_url,omitempty"`
	Location      string     `json:"location,omitempty"`
	Website       string     `json:"website,omitempty"`
	IsVerified    bool       `json:"is_verified"`
	IsSuspended   bool       `json:"is_suspended"`
	IsActive      bool       `json:"is_active"`
	Role          string     `json:"role"`
	Followers     int64      `json:"followers"`
	Following     int64      `json:"following"`
	TweetCount    int64      `json:"tweet_count"`
	IsFollowing   bool       `json:"is_following"`
	IsMutual      bool       `json:"is_mutual"`
	JoinedAt      time.Time  `json:"joined_at"`
	LastActive    *time.Time `json:"last_active_at,omitempty"`
	RecentTweets  []*TweetResponse `json:"recent_tweets,omitempty"`
}

// UserListResponse represents a paginated list of users.
type UserListResponse struct {
	Data       []UserResponse `json:"data"`
	Total      int64          `json:"total"`
	NextCursor string         `json:"next_cursor"`
	HasMore    bool           `json:"has_more"`
	Limit      int            `json:"limit"`
}

// UserStatsResponse represents user statistics.
type UserStatsResponse struct {
	UserID       string     `json:"user_id"`
	Username     string     `json:"username"`
	FullName     string     `json:"full_name"`
	Followers    int64      `json:"followers"`
	Following    int64      `json:"following"`
	TweetCount   int64      `json:"tweet_count"`
	LikeCount    int64      `json:"like_count"`
	JoinedAt     time.Time  `json:"joined_at"`
	LastActive   *time.Time `json:"last_active_at,omitempty"`
	IsVerified   bool       `json:"is_verified"`
	IsSuspended  bool       `json:"is_suspended"`
}

// UserSearchResponse represents a user in search results.
type UserSearchResponse struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	AvatarURL     string `json:"avatar_url,omitempty"`
	Bio           string `json:"bio,omitempty"`
	IsVerified    bool   `json:"is_verified"`
	IsFollowing   bool   `json:"is_following"`
	IsMutual      bool   `json:"is_mutual"`
	FollowerCount int64  `json:"follower_count"`
	TweetCount    int64  `json:"tweet_count"`
}

// FollowerResponse represents a follower in lists.
type FollowerResponse struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	FullName    string    `json:"full_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Bio         string    `json:"bio,omitempty"`
	IsVerified  bool      `json:"is_verified"`
	IsFollowing bool      `json:"is_following"`
	IsMutual    bool      `json:"is_mutual"`
	FollowedAt  time.Time `json:"followed_at"`
}

// FollowCountsResponse represents follow counts.
type FollowCountsResponse struct {
	UserID    string `json:"user_id"`
	Followers int64  `json:"followers"`
	Following int64  `json:"following"`
}

// SuggestionResponse represents a follow suggestion.
type SuggestionResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	FullName    string `json:"full_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Bio         string `json:"bio,omitempty"`
	IsVerified  bool   `json:"is_verified"`
	MutualCount int64  `json:"mutual_count"`
}

// FollowStatusResponse represents follow status check response.
type FollowStatusResponse struct {
	FollowerID  string `json:"follower_id"`
	FolloweeID  string `json:"followee_id"`
	IsFollowing bool   `json:"is_following"`
	IsMutual    bool   `json:"is_mutual"`
}

// UserFeedStatsResponse represents user feed statistics.
type UserFeedStatsResponse struct {
	UserID         string     `json:"user_id"`
	TotalTweets    int64      `json:"total_tweets"`
	TotalLikes     int64      `json:"total_likes"`
	TotalRetweets  int64      `json:"total_retweets"`
	TotalFollowers int64      `json:"total_followers"`
	TotalFollowing int64      `json:"total_following"`
	EngagementRate float64    `json:"engagement_rate"`
	LastTweetAt    time.Time  `json:"last_tweet_at"`
	JoinedAt       time.Time  `json:"joined_at"`
}

// ======================================================================
// Builder Methods for UserResponse
// ======================================================================

// NewUserResponse creates a new user response.
func NewUserResponse() *UserResponse {
	return &UserResponse{
		JoinedAt: time.Now().UTC(),
	}
}

// WithID sets the user ID.
func (r *UserResponse) WithID(id string) *UserResponse {
	r.ID = id
	return r
}

// WithUsername sets the username.
func (r *UserResponse) WithUsername(username string) *UserResponse {
	r.Username = username
	return r
}

// WithEmail sets the email.
func (r *UserResponse) WithEmail(email string) *UserResponse {
	r.Email = email
	return r
}

// WithFullName sets the full name.
func (r *UserResponse) WithFullName(fullName string) *UserResponse {
	r.FullName = fullName
	return r
}

// WithBio sets the bio.
func (r *UserResponse) WithBio(bio string) *UserResponse {
	r.Bio = bio
	return r
}

// WithAvatarURL sets the avatar URL.
func (r *UserResponse) WithAvatarURL(url string) *UserResponse {
	r.AvatarURL = url
	return r
}

// WithBannerURL sets the banner URL.
func (r *UserResponse) WithBannerURL(url string) *UserResponse {
	r.BannerURL = url
	return r
}

// WithRole sets the role.
func (r *UserResponse) WithRole(role string) *UserResponse {
	r.Role = role
	return r
}

// WithStatus sets the status.
func (r *UserResponse) WithStatus(status string) *UserResponse {
	r.Status = status
	return r
}

// WithVerified sets the verified flag.
func (r *UserResponse) WithVerified(verified bool) *UserResponse {
	r.IsVerified = verified
	return r
}

// WithPrivate sets the private flag.
func (r *UserResponse) WithPrivate(private bool) *UserResponse {
	r.IsPrivate = private
	return r
}

// WithCounts sets tweet, follower, and following counts.
func (r *UserResponse) WithCounts(tweets, followers, following int64) *UserResponse {
	r.TweetCount = tweets
	r.FollowerCount = followers
	r.FollowingCount = following
	return r
}

// WithFollowing sets the following status.
func (r *UserResponse) WithFollowing(following bool) *UserResponse {
	r.IsFollowing = following
	return r
}

// WithMutual sets the mutual status.
func (r *UserResponse) WithMutual(mutual bool) *UserResponse {
	r.IsMutual = mutual
	return r
}

// WithJoinedAt sets the joined at time.
func (r *UserResponse) WithJoinedAt(t time.Time) *UserResponse {
	r.JoinedAt = t
	return r
}

// WithLastActive sets the last active time.
func (r *UserResponse) WithLastActive(t time.Time) *UserResponse {
	r.LastActive = &t
	return r
}

// ======================================================================
// Builder Methods for MinimalUserResponse
// ======================================================================

// NewMinimalUserResponse creates a new minimal user response.
func NewMinimalUserResponse(id, username, fullName string) *MinimalUserResponse {
	return &MinimalUserResponse{
		ID:        id,
		Username:  username,
		FullName:  fullName,
		IsVerified: false,
	}
}

// WithAvatarURL sets the avatar URL.
func (r *MinimalUserResponse) WithAvatarURL(url string) *MinimalUserResponse {
	r.AvatarURL = url
	return r
}

// WithVerified sets the verified flag.
func (r *MinimalUserResponse) WithVerified(verified bool) *MinimalUserResponse {
	r.IsVerified = verified
	return r
}

// ======================================================================
// Builder Methods for UserProfileResponse
// ======================================================================

// NewUserProfileResponse creates a new user profile response.
func NewUserProfileResponse() *UserProfileResponse {
	return &UserProfileResponse{
		RecentTweets: []*TweetResponse{},
		JoinedAt:     time.Now().UTC(),
	}
}

// WithID sets the user ID.
func (r *UserProfileResponse) WithID(id string) *UserProfileResponse {
	r.ID = id
	return r
}

// WithUsername sets the username.
func (r *UserProfileResponse) WithUsername(username string) *UserProfileResponse {
	r.Username = username
	return r
}

// WithEmail sets the email.
func (r *UserProfileResponse) WithEmail(email string) *UserProfileResponse {
	r.Email = email
	return r
}

// WithFullName sets the full name.
func (r *UserProfileResponse) WithFullName(fullName string) *UserProfileResponse {
	r.FullName = fullName
	return r
}

// WithBio sets the bio.
func (r *UserProfileResponse) WithBio(bio string) *UserProfileResponse {
	r.Bio = bio
	return r
}

// WithAvatarURL sets the avatar URL.
func (r *UserProfileResponse) WithAvatarURL(url string) *UserProfileResponse {
	r.AvatarURL = url
	return r
}

// WithBannerURL sets the banner URL.
func (r *UserProfileResponse) WithBannerURL(url string) *UserProfileResponse {
	r.BannerURL = url
	return r
}

// WithRole sets the role.
func (r *UserProfileResponse) WithRole(role string) *UserProfileResponse {
	r.Role = role
	return r
}

// WithStats sets follower, following, and tweet counts.
func (r *UserProfileResponse) WithStats(followers, following, tweets int64) *UserProfileResponse {
	r.Followers = followers
	r.Following = following
	r.TweetCount = tweets
	return r
}

// WithFollowStatus sets following and mutual status.
func (r *UserProfileResponse) WithFollowStatus(isFollowing, isMutual bool) *UserProfileResponse {
	r.IsFollowing = isFollowing
	r.IsMutual = isMutual
	return r
}

// WithJoinedAt sets the joined at time.
func (r *UserProfileResponse) WithJoinedAt(t time.Time) *UserProfileResponse {
	r.JoinedAt = t
	return r
}

// WithLastActive sets the last active time.
func (r *UserProfileResponse) WithLastActive(t time.Time) *UserProfileResponse {
	r.LastActive = &t
	return r
}

// WithRecentTweets sets the recent tweets.
func (r *UserProfileResponse) WithRecentTweets(tweets []*TweetResponse) *UserProfileResponse {
	r.RecentTweets = tweets
	return r
}

// ======================================================================
// Builder Methods for UserListResponse
// ======================================================================

// NewUserListResponse creates a new user list response.
func NewUserListResponse() *UserListResponse {
	return &UserListResponse{
		Data: []UserResponse{},
	}
}

// Add adds a user to the response.
func (r *UserListResponse) Add(user UserResponse) {
	r.Data = append(r.Data, user)
}

// WithTotal sets the total count.
func (r *UserListResponse) WithTotal(total int64) *UserListResponse {
	r.Total = total
	return r
}

// WithNextCursor sets the next cursor.
func (r *UserListResponse) WithNextCursor(cursor string) *UserListResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *UserListResponse) WithLimit(limit int) *UserListResponse {
	r.Limit = limit
	return r
}

// ======================================================================
// Conversion Helpers
// ======================================================================

// ToUserResponse converts user data to a response.
func ToUserResponse(id, username, email, fullName, role, status string, isVerified, isPrivate bool, joinedAt time.Time) *UserResponse {
	return &UserResponse{
		ID:         id,
		Username:   username,
		Email:      email,
		FullName:   fullName,
		Role:       role,
		Status:     status,
		IsVerified: isVerified,
		IsPrivate:  isPrivate,
		JoinedAt:   joinedAt,
	}
}

// ToMinimalUserResponse converts user data to a minimal response.
func ToMinimalUserResponse(id, username, fullName, avatarURL string, isVerified bool) *MinimalUserResponse {
	return &MinimalUserResponse{
		ID:         id,
		Username:   username,
		FullName:   fullName,
		AvatarURL:  avatarURL,
		IsVerified: isVerified,
	}
}

// ToFollowerResponse converts user data to a follower response.
func ToFollowerResponse(id, username, fullName, avatarURL, bio string, isVerified bool, followedAt time.Time) *FollowerResponse {
	return &FollowerResponse{
		ID:         id,
		Username:   username,
		FullName:   fullName,
		AvatarURL:  avatarURL,
		Bio:        bio,
		IsVerified: isVerified,
		FollowedAt: followedAt,
	}
}

// ======================================================================
// JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *UserResponse) MarshalJSON() ([]byte, error) {
	type Alias UserResponse
	return json.Marshal(&struct {
		*Alias
		JoinedAt string `json:"joined_at"`
	}{
		Alias:    (*Alias)(r),
		JoinedAt: r.JoinedAt.Format(time.RFC3339),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *UserResponse) UnmarshalJSON(data []byte) error {
	type Alias UserResponse
	aux := &struct {
		*Alias
		JoinedAt string `json:"joined_at"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.JoinedAt != "" {
		t, err := time.Parse(time.RFC3339, aux.JoinedAt)
		if err == nil {
			r.JoinedAt = t
		}
	}
	return nil
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestUserResponse creates a test user response.
func NewTestUserResponse() *UserResponse {
	return NewUserResponse().
		WithID("user1").
		WithUsername("john_doe").
		WithEmail("john@example.com").
		WithFullName("John Doe").
		WithBio("Software Engineer").
		WithAvatarURL("https://example.com/avatar.jpg").
		WithRole("user").
		WithStatus("active").
		WithVerified(true).
		WithPrivate(false).
		WithCounts(100, 50, 75).
		WithFollowing(false).
		WithMutual(false)
}

// NewTestUserListResponse creates a test user list response.
func NewTestUserListResponse() *UserListResponse {
	list := NewUserListResponse()
	list.Add(*NewTestUserResponse())
	list.WithTotal(1).WithNextCursor("cursor123").WithLimit(20)
	return list
}

// NewTestMinimalUserResponse creates a test minimal user response.
func NewTestMinimalUserResponse() *MinimalUserResponse {
	return NewMinimalUserResponse("user1", "john_doe", "John Doe").
		WithAvatarURL("https://example.com/avatar.jpg").
		WithVerified(true)
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagUsers = "Users"
)