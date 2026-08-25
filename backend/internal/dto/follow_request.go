// backend/internal/dto/follow_request.go
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
	MaxFollowerLimit      = 100
	DefaultFollowerLimit  = 20
	MaxSuggestionsLimit   = 50
	DefaultSuggestionsLimit = 10
	MaxMutualLimit        = 100
	DefaultMutualLimit    = 20
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrFollowIDRequired      = errors.New("follow ID is required")
	ErrFollowerIDRequired    = errors.New("follower ID is required")
	ErrFolloweeIDRequired    = errors.New("followee ID is required")
	ErrCannotFollowSelf      = errors.New("cannot follow yourself")
	ErrInvalidFollowAction   = errors.New("invalid follow action")
	ErrInvalidFollowStatus   = errors.New("invalid follow status")
	ErrUserIDRequired        = errors.New("user ID is required")
	ErrInvalidLimit          = errors.New("limit must be between 1 and 100")
	ErrInvalidCursor         = errors.New("invalid cursor format")
	ErrAlreadyFollowing      = errors.New("already following this user")
	ErrNotFollowing          = errors.New("not following this user")
	ErrFollowNotFound        = errors.New("follow relationship not found")
)

// ======================================================================
// Follow Status and Action Types
// ======================================================================

// FollowStatus represents the status of a follow relationship.
type FollowStatus string

const (
	FollowStatusPending  FollowStatus = "pending"
	FollowStatusAccepted FollowStatus = "accepted"
	FollowStatusRejected FollowStatus = "rejected"
	FollowStatusBlocked  FollowStatus = "blocked"
)

// ValidFollowStatuses returns all valid follow statuses.
func ValidFollowStatuses() []FollowStatus {
	return []FollowStatus{
		FollowStatusPending,
		FollowStatusAccepted,
		FollowStatusRejected,
		FollowStatusBlocked,
	}
}

// IsValid checks if a follow status is valid.
func (s FollowStatus) IsValid() bool {
	for _, status := range ValidFollowStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (s FollowStatus) String() string {
	return string(s)
}

// FollowAction represents the action to perform on a follow.
type FollowAction string

const (
	ActionFollow   FollowAction = "follow"
	ActionUnfollow FollowAction = "unfollow"
	ActionAccept   FollowAction = "accept"
	ActionReject   FollowAction = "reject"
	ActionBlock    FollowAction = "block"
	ActionUnblock  FollowAction = "unblock"
)

// ValidFollowActions returns all valid follow actions.
func ValidFollowActions() []FollowAction {
	return []FollowAction{
		ActionFollow,
		ActionUnfollow,
		ActionAccept,
		ActionReject,
		ActionBlock,
		ActionUnblock,
	}
}

// IsValid checks if a follow action is valid.
func (a FollowAction) IsValid() bool {
	for _, action := range ValidFollowActions() {
		if a == action {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (a FollowAction) String() string {
	return string(a)
}

// ======================================================================
// Request DTOs
// ======================================================================

// FollowRequest represents the request to follow a user.
type FollowRequest struct {
	FolloweeID string `json:"followee_id" binding:"required"`
	FollowerID string `json:"follower_id,omitempty"` // typically the authenticated user
}

// Validate validates the follow request.
func (r *FollowRequest) Validate() error {
	if strings.TrimSpace(r.FolloweeID) == "" {
		return ErrFolloweeIDRequired
	}
	if strings.TrimSpace(r.FollowerID) == "" {
		return ErrFollowerIDRequired
	}
	if r.FollowerID == r.FolloweeID {
		return ErrCannotFollowSelf
	}
	return nil
}

// Sanitize sanitizes the follow request.
func (r *FollowRequest) Sanitize() {
	r.FolloweeID = strings.TrimSpace(r.FolloweeID)
	r.FollowerID = strings.TrimSpace(r.FollowerID)
}

// UnfollowRequest represents the request to unfollow a user.
type UnfollowRequest struct {
	FolloweeID string `json:"followee_id" binding:"required"`
	FollowerID string `json:"follower_id,omitempty"`
}

// Validate validates the unfollow request.
func (r *UnfollowRequest) Validate() error {
	if strings.TrimSpace(r.FolloweeID) == "" {
		return ErrFolloweeIDRequired
	}
	if strings.TrimSpace(r.FollowerID) == "" {
		return ErrFollowerIDRequired
	}
	if r.FollowerID == r.FolloweeID {
		return ErrCannotFollowSelf
	}
	return nil
}

// Sanitize sanitizes the unfollow request.
func (r *UnfollowRequest) Sanitize() {
	r.FolloweeID = strings.TrimSpace(r.FolloweeID)
	r.FollowerID = strings.TrimSpace(r.FollowerID)
}

// FollowActionRequest represents a follow action request.
type FollowActionRequest struct {
	FollowID   string       `json:"follow_id,omitempty"`
	FollowerID string       `json:"follower_id,omitempty"`
	FolloweeID string       `json:"followee_id,omitempty"`
	Action     FollowAction `json:"action" binding:"required"`
}

// Validate validates the follow action request.
func (r *FollowActionRequest) Validate() error {
	if !r.Action.IsValid() {
		return ErrInvalidFollowAction
	}
	switch r.Action {
	case ActionAccept, ActionReject, ActionBlock, ActionUnblock:
		if strings.TrimSpace(r.FollowID) == "" {
			return ErrFollowIDRequired
		}
	case ActionFollow:
		if strings.TrimSpace(r.FolloweeID) == "" {
			return ErrFolloweeIDRequired
		}
		if strings.TrimSpace(r.FollowerID) == "" {
			return ErrFollowerIDRequired
		}
		if r.FollowerID == r.FolloweeID {
			return ErrCannotFollowSelf
		}
	case ActionUnfollow:
		if strings.TrimSpace(r.FolloweeID) == "" {
			return ErrFolloweeIDRequired
		}
		if strings.TrimSpace(r.FollowerID) == "" {
			return ErrFollowerIDRequired
		}
	}
	return nil
}

// Sanitize sanitizes the follow action request.
func (r *FollowActionRequest) Sanitize() {
	r.FollowID = strings.TrimSpace(r.FollowID)
	r.FollowerID = strings.TrimSpace(r.FollowerID)
	r.FolloweeID = strings.TrimSpace(r.FolloweeID)
	r.Action = FollowAction(strings.TrimSpace(string(r.Action)))
}

// GetFollowersRequest represents the request to get followers.
type GetFollowersRequest struct {
	UserID      string `json:"user_id" binding:"required"`
	Status      string `json:"status,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	IncludeMutual bool `json:"include_mutual,omitempty"`
	SortBy      string `json:"sort_by,omitempty"`
	SortOrder   string `json:"sort_order,omitempty"`
}

// Validate validates the get followers request.
func (r *GetFollowersRequest) Validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	if r.Status != "" && !FollowStatus(r.Status).IsValid() {
		return ErrInvalidFollowStatus
	}
	if r.Limit < 0 || r.Limit > MaxFollowerLimit {
		return ErrInvalidLimit
	}
	if r.SortBy != "" {
		allowed := map[string]bool{
			"joined_at": true, "created_at": true, "username": true,
			"full_name": true,
		}
		if !allowed[r.SortBy] {
			return errors.New("invalid sort field")
		}
	}
	if r.SortOrder != "" && r.SortOrder != "asc" && r.SortOrder != "desc" {
		return errors.New("invalid sort order")
	}
	return nil
}

// Sanitize sanitizes the get followers request.
func (r *GetFollowersRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Status = strings.TrimSpace(r.Status)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultFollowerLimit
	}
	if r.Limit > MaxFollowerLimit {
		r.Limit = MaxFollowerLimit
	}
	if r.SortBy != "" {
		r.SortBy = strings.ToLower(strings.TrimSpace(r.SortBy))
	}
	if r.SortOrder != "" {
		r.SortOrder = strings.ToLower(strings.TrimSpace(r.SortOrder))
	}
}

// GetFollowingRequest represents the request to get following list.
type GetFollowingRequest struct {
	UserID      string `json:"user_id" binding:"required"`
	Status      string `json:"status,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	IncludeMutual bool `json:"include_mutual,omitempty"`
	SortBy      string `json:"sort_by,omitempty"`
	SortOrder   string `json:"sort_order,omitempty"`
}

// Validate validates the get following request.
func (r *GetFollowingRequest) Validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	if r.Status != "" && !FollowStatus(r.Status).IsValid() {
		return ErrInvalidFollowStatus
	}
	if r.Limit < 0 || r.Limit > MaxFollowerLimit {
		return ErrInvalidLimit
	}
	if r.SortBy != "" {
		allowed := map[string]bool{
			"joined_at": true, "created_at": true, "username": true,
			"full_name": true,
		}
		if !allowed[r.SortBy] {
			return errors.New("invalid sort field")
		}
	}
	if r.SortOrder != "" && r.SortOrder != "asc" && r.SortOrder != "desc" {
		return errors.New("invalid sort order")
	}
	return nil
}

// Sanitize sanitizes the get following request.
func (r *GetFollowingRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Status = strings.TrimSpace(r.Status)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultFollowerLimit
	}
	if r.Limit > MaxFollowerLimit {
		r.Limit = MaxFollowerLimit
	}
	if r.SortBy != "" {
		r.SortBy = strings.ToLower(strings.TrimSpace(r.SortBy))
	}
	if r.SortOrder != "" {
		r.SortOrder = strings.ToLower(strings.TrimSpace(r.SortOrder))
	}
}

// GetMutualFollowsRequest represents the request to get mutual follows.
type GetMutualFollowsRequest struct {
	UserID1 string `json:"user_id1" binding:"required"`
	UserID2 string `json:"user_id2" binding:"required"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// Validate validates the get mutual follows request.
func (r *GetMutualFollowsRequest) Validate() error {
	if strings.TrimSpace(r.UserID1) == "" {
		return ErrUserIDRequired
	}
	if strings.TrimSpace(r.UserID2) == "" {
		return ErrUserIDRequired
	}
	if r.UserID1 == r.UserID2 {
		return errors.New("cannot get mutual follows between same user")
	}
	if r.Limit < 0 || r.Limit > MaxMutualLimit {
		return ErrInvalidLimit
	}
	return nil
}

// Sanitize sanitizes the get mutual follows request.
func (r *GetMutualFollowsRequest) Sanitize() {
	r.UserID1 = strings.TrimSpace(r.UserID1)
	r.UserID2 = strings.TrimSpace(r.UserID2)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultMutualLimit
	}
	if r.Limit > MaxMutualLimit {
		r.Limit = MaxMutualLimit
	}
}

// GetFollowSuggestionsRequest represents the request to get follow suggestions.
type GetFollowSuggestionsRequest struct {
	UserID   string `json:"user_id,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Exclude  []string `json:"exclude,omitempty"`
	Algorithm string `json:"algorithm,omitempty"` // "mutual", "popular", "random", "ai"
}

// Validate validates the get follow suggestions request.
func (r *GetFollowSuggestionsRequest) Validate() error {
	if r.Limit < 0 || r.Limit > MaxSuggestionsLimit {
		return ErrInvalidLimit
	}
	if r.Algorithm != "" {
		validAlgorithms := map[string]bool{
			"mutual": true, "popular": true, "random": true, "ai": true,
		}
		if !validAlgorithms[r.Algorithm] {
			return errors.New("invalid algorithm")
		}
	}
	return nil
}

// Sanitize sanitizes the get follow suggestions request.
func (r *GetFollowSuggestionsRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultSuggestionsLimit
	}
	if r.Limit > MaxSuggestionsLimit {
		r.Limit = MaxSuggestionsLimit
	}
	cleaned := make([]string, 0, len(r.Exclude))
	for _, id := range r.Exclude {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.Exclude = cleaned
	if r.Algorithm == "" {
		r.Algorithm = "mutual"
	}
}

// CheckFollowStatusRequest represents the request to check follow status.
type CheckFollowStatusRequest struct {
	FollowerID string `json:"follower_id" binding:"required"`
	FolloweeID string `json:"followee_id" binding:"required"`
}

// Validate validates the check follow status request.
func (r *CheckFollowStatusRequest) Validate() error {
	if strings.TrimSpace(r.FollowerID) == "" {
		return ErrFollowerIDRequired
	}
	if strings.TrimSpace(r.FolloweeID) == "" {
		return ErrFolloweeIDRequired
	}
	if r.FollowerID == r.FolloweeID {
		return ErrCannotFollowSelf
	}
	return nil
}

// Sanitize sanitizes the check follow status request.
func (r *CheckFollowStatusRequest) Sanitize() {
	r.FollowerID = strings.TrimSpace(r.FollowerID)
	r.FolloweeID = strings.TrimSpace(r.FolloweeID)
}

// FollowCountsRequest represents the request to get follow counts.
type FollowCountsRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// Validate validates the follow counts request.
func (r *FollowCountsRequest) Validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	return nil
}

// Sanitize sanitizes the follow counts request.
func (r *FollowCountsRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
}

// BulkFollowRequest represents the request to follow multiple users.
type BulkFollowRequest struct {
	FolloweeIDs []string `json:"followee_ids" binding:"required"`
	FollowerID  string   `json:"follower_id,omitempty"`
}

// Validate validates the bulk follow request.
func (r *BulkFollowRequest) Validate() error {
	if strings.TrimSpace(r.FollowerID) == "" {
		return ErrFollowerIDRequired
	}
	if len(r.FolloweeIDs) == 0 {
		return errors.New("followee IDs are required")
	}
	if len(r.FolloweeIDs) > MaxMembersPerBatch {
		return fmt.Errorf("followee IDs exceeds maximum of %d", MaxMembersPerBatch)
	}
	for i, id := range r.FolloweeIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("followee ID at index %d is empty", i)
		}
		if id == r.FollowerID {
			return fmt.Errorf("cannot follow yourself at index %d", i)
		}
	}
	return nil
}

// Sanitize sanitizes the bulk follow request.
func (r *BulkFollowRequest) Sanitize() {
	r.FollowerID = strings.TrimSpace(r.FollowerID)
	cleaned := make([]string, 0, len(r.FolloweeIDs))
	for _, id := range r.FolloweeIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" && trimmed != r.FollowerID {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.FolloweeIDs = cleaned
}

// ======================================================================
// Response DTOs
// ======================================================================

// FollowResponse represents a follow relationship in responses.
type FollowResponse struct {
	ID             string    `json:"id"`
	FollowerID     string    `json:"follower_id"`
	FolloweeID     string    `json:"followee_id"`
	Status         string    `json:"status"`
	Following      bool      `json:"following"`
	FollowerUsername string  `json:"follower_username,omitempty"`
	FolloweeUsername string  `json:"followee_username,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FollowerResponse represents a follower in lists.
type FollowerResponse struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	FullName    string    `json:"full_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Bio         string    `json:"bio,omitempty"`
	IsVerified  bool      `json:"is_verified"`
	IsFollowing bool      `json:"is_following"` // does the current user follow this user?
	IsMutual    bool      `json:"is_mutual"`    // mutual follow
	FollowedAt  time.Time `json:"followed_at"`
}

// FollowerListResponse represents a paginated list of followers.
type FollowerListResponse struct {
	Data       []FollowerResponse `json:"data"`
	Total      int64              `json:"total"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
	Limit      int                `json:"limit"`
}

// FollowCountsResponse represents follow counts.
type FollowCountsResponse struct {
	UserID    string `json:"user_id"`
	Followers int64  `json:"followers"`
	Following int64  `json:"following"`
	Mutual    int64  `json:"mutual,omitempty"`
}

// SuggestionResponse represents a follow suggestion.
type SuggestionResponse struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	FullName    string  `json:"full_name"`
	AvatarURL   string  `json:"avatar_url,omitempty"`
	Bio         string  `json:"bio,omitempty"`
	IsVerified  bool    `json:"is_verified"`
	MutualCount int64   `json:"mutual_count"`
	FollowerCount int64 `json:"follower_count"`
	Reason      string  `json:"reason,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

// FollowSuggestionsResponse represents follow suggestions.
type FollowSuggestionsResponse struct {
	Data       []SuggestionResponse `json:"data"`
	Total      int64                `json:"total"`
	NextCursor string               `json:"next_cursor"`
	HasMore    bool                 `json:"has_more"`
	Limit      int                  `json:"limit"`
}

// MutualFollowsResponse represents mutual follows.
type MutualFollowsResponse struct {
	Data       []FollowerResponse `json:"data"`
	Total      int64              `json:"total"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
	Limit      int                `json:"limit"`
}

// FollowStatsResponse represents follow statistics.
type FollowStatsResponse struct {
	TotalFollows    int64     `json:"total_follows"`
	UniqueFollowers int64     `json:"unique_followers"`
	UniqueFollowees int64     `json:"unique_followees"`
	PendingFollows  int64     `json:"pending_follows"`
	AcceptedFollows int64     `json:"accepted_follows"`
	RejectedFollows int64     `json:"rejected_follows"`
	BlockedFollows  int64     `json:"blocked_follows"`
	LastFollow      time.Time `json:"last_follow"`
	FirstFollow     time.Time `json:"first_follow"`
}

// FollowStatusResponse represents follow status check response.
type FollowStatusResponse struct {
	FollowerID string `json:"follower_id"`
	FolloweeID string `json:"followee_id"`
	IsFollowing bool  `json:"is_following"`
	IsMutual    bool  `json:"is_mutual"`
	Status      string `json:"status"`
}

// ======================================================================
// Builder Methods for FollowResponse
// ======================================================================

// NewFollowResponse creates a new follow response.
func NewFollowResponse(id, followerID, followeeID, status string) *FollowResponse {
	return &FollowResponse{
		ID:         id,
		FollowerID: followerID,
		FolloweeID: followeeID,
		Status:     status,
		Following:  status == string(FollowStatusAccepted),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
}

// WithFollowerUsername sets the follower username.
func (r *FollowResponse) WithFollowerUsername(username string) *FollowResponse {
	r.FollowerUsername = username
	return r
}

// WithFolloweeUsername sets the followee username.
func (r *FollowResponse) WithFolloweeUsername(username string) *FollowResponse {
	r.FolloweeUsername = username
	return r
}

// WithFollowing sets the following flag.
func (r *FollowResponse) WithFollowing(following bool) *FollowResponse {
	r.Following = following
	return r
}

// WithCreatedAt sets the created at time.
func (r *FollowResponse) WithCreatedAt(t time.Time) *FollowResponse {
	r.CreatedAt = t
	return r
}

// ======================================================================
// Builder Methods for FollowerResponse
// ======================================================================

// NewFollowerResponse creates a new follower response.
func NewFollowerResponse(id, username, fullName string) *FollowerResponse {
	return &FollowerResponse{
		ID:         id,
		Username:   username,
		FullName:   fullName,
		FollowedAt: time.Now().UTC(),
	}
}

// WithAvatarURL sets the avatar URL.
func (r *FollowerResponse) WithAvatarURL(url string) *FollowerResponse {
	r.AvatarURL = url
	return r
}

// WithBio sets the bio.
func (r *FollowerResponse) WithBio(bio string) *FollowerResponse {
	r.Bio = bio
	return r
}

// WithIsVerified sets the verified flag.
func (r *FollowerResponse) WithIsVerified(verified bool) *FollowerResponse {
	r.IsVerified = verified
	return r
}

// WithIsFollowing sets the following flag.
func (r *FollowerResponse) WithIsFollowing(following bool) *FollowerResponse {
	r.IsFollowing = following
	return r
}

// WithIsMutual sets the mutual flag.
func (r *FollowerResponse) WithIsMutual(mutual bool) *FollowerResponse {
	r.IsMutual = mutual
	return r
}

// WithFollowedAt sets the followed at time.
func (r *FollowerResponse) WithFollowedAt(t time.Time) *FollowerResponse {
	r.FollowedAt = t
	return r
}

// ======================================================================
// Builder Methods for FollowSuggestionsResponse
// ======================================================================

// NewFollowSuggestionsResponse creates a new follow suggestions response.
func NewFollowSuggestionsResponse() *FollowSuggestionsResponse {
	return &FollowSuggestionsResponse{
		Data:  []SuggestionResponse{},
		Total: 0,
	}
}

// Add adds a suggestion to the response.
func (r *FollowSuggestionsResponse) Add(suggestion SuggestionResponse) {
	r.Data = append(r.Data, suggestion)
}

// WithTotal sets the total count.
func (r *FollowSuggestionsResponse) WithTotal(total int64) *FollowSuggestionsResponse {
	r.Total = total
	return r
}

// WithNextCursor sets the next cursor.
func (r *FollowSuggestionsResponse) WithNextCursor(cursor string) *FollowSuggestionsResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *FollowSuggestionsResponse) WithLimit(limit int) *FollowSuggestionsResponse {
	r.Limit = limit
	return r
}

// ======================================================================
// Conversion Helpers
// ======================================================================

// ToFollowerResponse converts user data to follower response.
func ToFollowerResponse(id, username, fullName, avatarURL, bio string, isVerified bool, followedAt time.Time) FollowerResponse {
	return FollowerResponse{
		ID:         id,
		Username:   username,
		FullName:   fullName,
		AvatarURL:  avatarURL,
		Bio:        bio,
		IsVerified: isVerified,
		FollowedAt: followedAt,
	}
}

// ToFollowCountsResponse creates a follow counts response.
func ToFollowCountsResponse(userID string, followers, following int64) FollowCountsResponse {
	return FollowCountsResponse{
		UserID:    userID,
		Followers: followers,
		Following: following,
	}
}

// ToSuggestionResponse converts user data to suggestion response.
func ToSuggestionResponse(id, username, fullName, avatarURL, bio string, isVerified bool, mutualCount, followerCount int64, reason string, score float64) SuggestionResponse {
	return SuggestionResponse{
		ID:            id,
		Username:      username,
		FullName:      fullName,
		AvatarURL:     avatarURL,
		Bio:           bio,
		IsVerified:    isVerified,
		MutualCount:   mutualCount,
		FollowerCount: followerCount,
		Reason:        reason,
		Score:         score,
	}
}

// ======================================================================
= JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *FollowResponse) MarshalJSON() ([]byte, error) {
	type Alias FollowResponse
	return json.Marshal(&struct {
		*Alias
		Status string `json:"status"`
	}{
		Alias:  (*Alias)(r),
		Status: r.Status,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *FollowResponse) UnmarshalJSON(data []byte) error {
	type Alias FollowResponse
	aux := &struct {
		*Alias
		Status string `json:"status"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Status != "" {
		r.Status = aux.Status
	}
	return nil
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestFollowRequest creates a test follow request.
func NewTestFollowRequest(followerID, followeeID string) *FollowRequest {
	return &FollowRequest{
		FollowerID: followerID,
		FolloweeID: followeeID,
	}
}

// NewTestUnfollowRequest creates a test unfollow request.
func NewTestUnfollowRequest(followerID, followeeID string) *UnfollowRequest {
	return &UnfollowRequest{
		FollowerID: followerID,
		FolloweeID: followeeID,
	}
}

// NewTestGetFollowersRequest creates a test get followers request.
func NewTestGetFollowersRequest(userID string) *GetFollowersRequest {
	return &GetFollowersRequest{
		UserID: userID,
		Limit:  20,
	}
}

// NewTestFollowResponse creates a test follow response.
func NewTestFollowResponse() *FollowResponse {
	resp := NewFollowResponse("follow1", "user1", "user2", "accepted")
	resp.WithFollowerUsername("john_doe").WithFolloweeUsername("jane_smith")
	return resp
}

// NewTestFollowerResponse creates a test follower response.
func NewTestFollowerResponse() *FollowerResponse {
	resp := NewFollowerResponse("user1", "john_doe", "John Doe")
	resp.WithAvatarURL("https://example.com/avatar.jpg").WithIsVerified(true)
	return resp
}

// NewTestFollowSuggestionsResponse creates a test suggestions response.
func NewTestFollowSuggestionsResponse() *FollowSuggestionsResponse {
	resp := NewFollowSuggestionsResponse()
	suggestion := ToSuggestionResponse(
		"user1", "john_doe", "John Doe", "https://example.com/avatar.jpg",
		"Software Engineer", true, 5, 100, "Mutual followers", 95.5,
	)
	resp.Add(suggestion)
	resp.WithTotal(1).WithNextCursor("cursor123").WithLimit(10)
	return resp
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagFollows = "Follows"
)