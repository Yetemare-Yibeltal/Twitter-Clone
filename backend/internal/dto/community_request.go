// backend/internal/dto/community_request.go
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
	MaxCommunityNameLength        = 100
	MinCommunityNameLength        = 3
	MaxCommunitySlugLength        = 50
	MinCommunitySlugLength        = 3
	MaxCommunityDescriptionLength = 500
	MaxCommunityRulesLength       = 5000
	MaxCommunityWelcomeLength     = 500
	MaxCommunityRoleNameLength    = 50
	MaxCommunitiesPerRequest      = 100
	DefaultCommunitiesLimit       = 20
	MaxMembersPerBatch            = 1000
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrCommunityIDRequired        = errors.New("community ID is required")
	ErrCommunityNameRequired      = errors.New("community name is required")
	ErrCommunityNameTooShort      = fmt.Errorf("community name must be at least %d characters", MinCommunityNameLength)
	ErrCommunityNameTooLong       = fmt.Errorf("community name exceeds maximum of %d characters", MaxCommunityNameLength)
	ErrCommunitySlugRequired      = errors.New("community slug is required")
	ErrCommunitySlugTooShort      = fmt.Errorf("community slug must be at least %d characters", MinCommunitySlugLength)
	ErrCommunitySlugTooLong       = fmt.Errorf("community slug exceeds maximum of %d characters", MaxCommunitySlugLength)
	ErrCommunitySlugInvalid       = errors.New("community slug contains invalid characters")
	ErrCommunityDescriptionTooLong = fmt.Errorf("community description exceeds maximum of %d characters", MaxCommunityDescriptionLength)
	ErrInvalidVisibility          = errors.New("invalid community visibility")
	ErrInvalidCommunityStatus     = errors.New("invalid community status")
	ErrInvalidCommunityRole       = errors.New("invalid community role")
	ErrUserIDRequired             = errors.New("user ID is required")
	ErrInvalidAction              = errors.New("invalid action")
	ErrReasonRequired             = errors.New("reason is required")
	ErrReasonTooLong              = fmt.Errorf("reason exceeds maximum of %d characters", MaxReasonLength)
	ErrNoteTooLong                = fmt.Errorf("note exceeds maximum of %d characters", MaxNoteLength)
	ErrMemberIDsRequired          = errors.New("member IDs are required")
	ErrMembersTooMany             = fmt.Errorf("member IDs exceeds maximum of %d", MaxMembersPerBatch)
	ErrCommunitiesEmpty           = errors.New("communities list cannot be empty")
	ErrCommunitiesTooMany         = fmt.Errorf("communities list exceeds maximum of %d", MaxCommunitiesPerRequest)
)

// ======================================================================
// Community Visibility and Status Types
// ======================================================================

// CommunityVisibility represents the visibility of a community.
type CommunityVisibility string

const (
	VisibilityPublic  CommunityVisibility = "public"
	VisibilityPrivate CommunityVisibility = "private"
	VisibilityHidden  CommunityVisibility = "hidden"
)

// ValidVisibilities returns all valid visibility values.
func ValidVisibilities() []CommunityVisibility {
	return []CommunityVisibility{
		VisibilityPublic,
		VisibilityPrivate,
		VisibilityHidden,
	}
}

// IsValid checks if a visibility value is valid.
func (v CommunityVisibility) IsValid() bool {
	for _, vis := range ValidVisibilities() {
		if v == vis {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (v CommunityVisibility) String() string {
	return string(v)
}

// CommunityStatus represents the status of a community.
type CommunityStatus string

const (
	StatusActive    CommunityStatus = "active"
	StatusInactive  CommunityStatus = "inactive"
	StatusSuspended CommunityStatus = "suspended"
	StatusArchived  CommunityStatus = "archived"
)

// ValidStatuses returns all valid status values.
func ValidStatuses() []CommunityStatus {
	return []CommunityStatus{
		StatusActive,
		StatusInactive,
		StatusSuspended,
		StatusArchived,
	}
}

// IsValid checks if a status value is valid.
func (s CommunityStatus) IsValid() bool {
	for _, status := range ValidStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (s CommunityStatus) String() string {
	return string(s)
}

// CommunityRole represents the role of a member.
type CommunityRole string

const (
	RoleOwner      CommunityRole = "owner"
	RoleAdmin      CommunityRole = "admin"
	RoleModerator  CommunityRole = "moderator"
	RoleMember     CommunityRole = "member"
	RoleBanned     CommunityRole = "banned"
)

// ValidRoles returns all valid role values.
func ValidRoles() []CommunityRole {
	return []CommunityRole{
		RoleOwner,
		RoleAdmin,
		RoleModerator,
		RoleMember,
		RoleBanned,
	}
}

// IsValid checks if a role value is valid.
func (r CommunityRole) IsValid() bool {
	for _, role := range ValidRoles() {
		if r == role {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (r CommunityRole) String() string {
	return string(r)
}

// IsAdminRole checks if the role has admin privileges.
func (r CommunityRole) IsAdminRole() bool {
	return r == RoleOwner || r == RoleAdmin
}

// IsModeratorRole checks if the role has moderator privileges.
func (r CommunityRole) IsModeratorRole() bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleModerator
}

// ======================================================================
// Request DTOs
// ======================================================================

// CreateCommunityRequest represents the request to create a community.
type CreateCommunityRequest struct {
	Name        string              `json:"name" binding:"required"`
	Slug        string              `json:"slug,omitempty"`
	Description string              `json:"description,omitempty"`
	AvatarURL   string              `json:"avatar_url,omitempty"`
	BannerURL   string              `json:"banner_url,omitempty"`
	Visibility  CommunityVisibility `json:"visibility"`
	Settings    CommunitySettings   `json:"settings,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
}

// Validate validates the create community request.
func (r *CreateCommunityRequest) Validate() error {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		return ErrCommunityNameRequired
	}
	if len(name) < MinCommunityNameLength {
		return ErrCommunityNameTooShort
	}
	if len(name) > MaxCommunityNameLength {
		return ErrCommunityNameTooLong
	}
	r.Name = name
	if r.Slug != "" {
		slug := strings.TrimSpace(r.Slug)
		if len(slug) < MinCommunitySlugLength {
			return ErrCommunitySlugTooShort
		}
		if len(slug) > MaxCommunitySlugLength {
			return ErrCommunitySlugTooLong
		}
		if !isValidSlug(slug) {
			return ErrCommunitySlugInvalid
		}
		r.Slug = slug
	}
	if len(r.Description) > MaxCommunityDescriptionLength {
		return ErrCommunityDescriptionTooLong
	}
	if !r.Visibility.IsValid() {
		return ErrInvalidVisibility
	}
	if err := r.Settings.Validate(); err != nil {
		return err
	}
	return nil
}

// Sanitize sanitizes the create community request.
func (r *CreateCommunityRequest) Sanitize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Slug = strings.TrimSpace(r.Slug)
	r.Description = strings.TrimSpace(r.Description)
	r.AvatarURL = strings.TrimSpace(r.AvatarURL)
	r.BannerURL = strings.TrimSpace(r.BannerURL)
	if r.Slug == "" {
		r.Slug = generateSlug(r.Name)
	}
	r.Settings.Sanitize()
	if r.Tags == nil {
		r.Tags = []string{}
	}
	for i := range r.Tags {
		r.Tags[i] = strings.TrimSpace(r.Tags[i])
	}
}

// UpdateCommunityRequest represents the request to update a community.
type UpdateCommunityRequest struct {
	ID          string               `json:"id" binding:"required"`
	Name        *string              `json:"name,omitempty"`
	Slug        *string              `json:"slug,omitempty"`
	Description *string              `json:"description,omitempty"`
	AvatarURL   *string              `json:"avatar_url,omitempty"`
	BannerURL   *string              `json:"banner_url,omitempty"`
	Visibility  *CommunityVisibility `json:"visibility,omitempty"`
	Status      *CommunityStatus     `json:"status,omitempty"`
	Settings    *CommunitySettings   `json:"settings,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
}

// Validate validates the update community request.
func (r *UpdateCommunityRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrCommunityIDRequired
	}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return ErrCommunityNameRequired
		}
		if len(name) < MinCommunityNameLength {
			return ErrCommunityNameTooShort
		}
		if len(name) > MaxCommunityNameLength {
			return ErrCommunityNameTooLong
		}
	}
	if r.Slug != nil {
		slug := strings.TrimSpace(*r.Slug)
		if slug == "" {
			return ErrCommunitySlugRequired
		}
		if len(slug) < MinCommunitySlugLength {
			return ErrCommunitySlugTooShort
		}
		if len(slug) > MaxCommunitySlugLength {
			return ErrCommunitySlugTooLong
		}
		if !isValidSlug(slug) {
			return ErrCommunitySlugInvalid
		}
	}
	if r.Description != nil && len(*r.Description) > MaxCommunityDescriptionLength {
		return ErrCommunityDescriptionTooLong
	}
	if r.Visibility != nil && !r.Visibility.IsValid() {
		return ErrInvalidVisibility
	}
	if r.Status != nil && !r.Status.IsValid() {
		return ErrInvalidCommunityStatus
	}
	if r.Settings != nil {
		if err := r.Settings.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Sanitize sanitizes the update community request.
func (r *UpdateCommunityRequest) Sanitize() {
	r.ID = strings.TrimSpace(r.ID)
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		r.Name = &trimmed
	}
	if r.Slug != nil {
		trimmed := strings.TrimSpace(*r.Slug)
		r.Slug = &trimmed
	}
	if r.Description != nil {
		trimmed := strings.TrimSpace(*r.Description)
		r.Description = &trimmed
	}
	if r.AvatarURL != nil {
		trimmed := strings.TrimSpace(*r.AvatarURL)
		r.AvatarURL = &trimmed
	}
	if r.BannerURL != nil {
		trimmed := strings.TrimSpace(*r.BannerURL)
		r.BannerURL = &trimmed
	}
	if r.Settings != nil {
		r.Settings.Sanitize()
	}
	if r.Tags != nil {
		for i := range r.Tags {
			r.Tags[i] = strings.TrimSpace(r.Tags[i])
		}
	}
}

// CommunitySettings represents community settings.
type CommunitySettings struct {
	AllowPosts        bool     `json:"allow_posts"`
	AllowComments     bool     `json:"allow_comments"`
	AllowMedia        bool     `json:"allow_media"`
	RequireApproval   bool     `json:"require_approval"`
	MaxMembers        int64    `json:"max_members"`
	AutoAcceptMembers bool     `json:"auto_accept_members"`
	AllowedHashtags   []string `json:"allowed_hashtags"`
	ModerationLevel   string   `json:"moderation_level"` // "low", "medium", "high"
	WelcomeMessage    string   `json:"welcome_message,omitempty"`
	Rules             string   `json:"rules,omitempty"`
}

// Validate validates the community settings.
func (s *CommunitySettings) Validate() error {
	if s.MaxMembers < 0 {
		return errors.New("max members cannot be negative")
	}
	if s.ModerationLevel != "" {
		validLevels := map[string]bool{"low": true, "medium": true, "high": true}
		if !validLevels[s.ModerationLevel] {
			return errors.New("invalid moderation level")
		}
	}
	if len(s.WelcomeMessage) > MaxCommunityWelcomeLength {
		return fmt.Errorf("welcome message exceeds maximum of %d characters", MaxCommunityWelcomeLength)
	}
	if len(s.Rules) > MaxCommunityRulesLength {
		return fmt.Errorf("rules exceeds maximum of %d characters", MaxCommunityRulesLength)
	}
	for _, tag := range s.AllowedHashtags {
		if strings.TrimSpace(tag) == "" {
			return errors.New("allowed hashtag cannot be empty")
		}
	}
	return nil
}

// Sanitize sanitizes the community settings.
func (s *CommunitySettings) Sanitize() {
	s.WelcomeMessage = strings.TrimSpace(s.WelcomeMessage)
	s.Rules = strings.TrimSpace(s.Rules)
	cleaned := make([]string, 0, len(s.AllowedHashtags))
	for _, tag := range s.AllowedHashtags {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			cleaned = append(cleaned, strings.ToLower(trimmed))
		}
	}
	s.AllowedHashtags = cleaned
}

// GetCommunitiesRequest represents the request to list communities.
type GetCommunitiesRequest struct {
	UserID      string `json:"user_id,omitempty"`
	MemberID    string `json:"member_id,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Status      string `json:"status,omitempty"`
	Search      string `json:"search,omitempty"`
	Tag         string `json:"tag,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	SortBy      string `json:"sort_by,omitempty"`
	SortOrder   string `json:"sort_order,omitempty"`
	IncludeArchived bool `json:"include_archived,omitempty"`
}

// Validate validates the get communities request.
func (r *GetCommunitiesRequest) Validate() error {
	if r.Visibility != "" && !CommunityVisibility(r.Visibility).IsValid() {
		return ErrInvalidVisibility
	}
	if r.Status != "" && !CommunityStatus(r.Status).IsValid() {
		return ErrInvalidCommunityStatus
	}
	if r.Limit < 0 || r.Limit > 100 {
		return errors.New("limit must be between 0 and 100")
	}
	if r.SortBy != "" {
		allowed := map[string]bool{
			"created_at": true, "updated_at": true, "name": true,
			"member_count": true, "post_count": true,
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

// Sanitize sanitizes the get communities request.
func (r *GetCommunitiesRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.MemberID = strings.TrimSpace(r.MemberID)
	r.Visibility = strings.TrimSpace(r.Visibility)
	r.Status = strings.TrimSpace(r.Status)
	r.Search = strings.TrimSpace(r.Search)
	r.Tag = strings.TrimSpace(r.Tag)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultCommunitiesLimit
	}
	if r.Limit > 100 {
		r.Limit = 100
	}
	if r.SortBy != "" {
		r.SortBy = strings.ToLower(strings.TrimSpace(r.SortBy))
	}
	if r.SortOrder != "" {
		r.SortOrder = strings.ToLower(strings.TrimSpace(r.SortOrder))
	}
}

// JoinCommunityRequest represents the request to join a community.
type JoinCommunityRequest struct {
	CommunityID string `json:"community_id" binding:"required"`
	UserID      string `json:"user_id,omitempty"`
}

// Validate validates the join community request.
func (r *JoinCommunityRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	return nil
}

// Sanitize sanitizes the join community request.
func (r *JoinCommunityRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	r.UserID = strings.TrimSpace(r.UserID)
}

// LeaveCommunityRequest represents the request to leave a community.
type LeaveCommunityRequest struct {
	CommunityID string `json:"community_id" binding:"required"`
	UserID      string `json:"user_id,omitempty"`
}

// Validate validates the leave community request.
func (r *LeaveCommunityRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	return nil
}

// Sanitize sanitizes the leave community request.
func (r *LeaveCommunityRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	r.UserID = strings.TrimSpace(r.UserID)
}

// UpdateMemberRoleRequest represents the request to update a member's role.
type UpdateMemberRoleRequest struct {
	CommunityID string         `json:"community_id" binding:"required"`
	UserID      string         `json:"user_id" binding:"required"`
	Role        CommunityRole  `json:"role" binding:"required"`
	Reason      string         `json:"reason,omitempty"`
}

// Validate validates the update member role request.
func (r *UpdateMemberRoleRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	if !r.Role.IsValid() {
		return ErrInvalidCommunityRole
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	return nil
}

// Sanitize sanitizes the update member role request.
func (r *UpdateMemberRoleRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.Reason = strings.TrimSpace(r.Reason)
}

// RemoveMemberRequest represents the request to remove a member.
type RemoveMemberRequest struct {
	CommunityID string `json:"community_id" binding:"required"`
	UserID      string `json:"user_id" binding:"required"`
	Reason      string `json:"reason,omitempty"`
}

// Validate validates the remove member request.
func (r *RemoveMemberRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	return nil
}

// Sanitize sanitizes the remove member request.
func (r *RemoveMemberRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.Reason = strings.TrimSpace(r.Reason)
}

// BanMemberRequest represents the request to ban a member.
type BanMemberRequest struct {
	CommunityID string `json:"community_id" binding:"required"`
	UserID      string `json:"user_id" binding:"required"`
	Reason      string `json:"reason" binding:"required"`
	Duration    int    `json:"duration,omitempty"` // duration in days
	Note        string `json:"note,omitempty"`
}

// Validate validates the ban member request.
func (r *BanMemberRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	if strings.TrimSpace(r.Reason) == "" {
		return ErrReasonRequired
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	if len(r.Note) > MaxNoteLength {
		return ErrNoteTooLong
	}
	if r.Duration < 0 {
		return errors.New("duration cannot be negative")
	}
	return nil
}

// Sanitize sanitizes the ban member request.
func (r *BanMemberRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.Reason = strings.TrimSpace(r.Reason)
	r.Note = strings.TrimSpace(r.Note)
}

// UnbanMemberRequest represents the request to unban a member.
type UnbanMemberRequest struct {
	CommunityID string `json:"community_id" binding:"required"`
	UserID      string `json:"user_id" binding:"required"`
	Reason      string `json:"reason,omitempty"`
}

// Validate validates the unban member request.
func (r *UnbanMemberRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	return nil
}

// Sanitize sanitizes the unban member request.
func (r *UnbanMemberRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.Reason = strings.TrimSpace(r.Reason)
}

// AddPostRequest represents the request to add a post to a community.
type AddPostRequest struct {
	CommunityID string `json:"community_id" binding:"required"`
	TweetID     string `json:"tweet_id" binding:"required"`
}

// Validate validates the add post request.
func (r *AddPostRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	if strings.TrimSpace(r.TweetID) == "" {
		return ErrTweetIDRequired
	}
	return nil
}

// Sanitize sanitizes the add post request.
func (r *AddPostRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	r.TweetID = strings.TrimSpace(r.TweetID)
}

// RemovePostRequest represents the request to remove a post from a community.
type RemovePostRequest struct {
	CommunityID string `json:"community_id" binding:"required"`
	TweetID     string `json:"tweet_id" binding:"required"`
	Reason      string `json:"reason,omitempty"`
}

// Validate validates the remove post request.
func (r *RemovePostRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	if strings.TrimSpace(r.TweetID) == "" {
		return ErrTweetIDRequired
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	return nil
}

// Sanitize sanitizes the remove post request.
func (r *RemovePostRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	r.TweetID = strings.TrimSpace(r.TweetID)
	r.Reason = strings.TrimSpace(r.Reason)
}

// BulkAddMembersRequest represents the request to add multiple members.
type BulkAddMembersRequest struct {
	CommunityID string   `json:"community_id" binding:"required"`
	UserIDs     []string `json:"user_ids" binding:"required"`
	Role        string   `json:"role,omitempty"`
}

// Validate validates the bulk add members request.
func (r *BulkAddMembersRequest) Validate() error {
	if strings.TrimSpace(r.CommunityID) == "" {
		return ErrCommunityIDRequired
	}
	if len(r.UserIDs) == 0 {
		return ErrMemberIDsRequired
	}
	if len(r.UserIDs) > MaxMembersPerBatch {
		return ErrMembersTooMany
	}
	for i, id := range r.UserIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("user ID at index %d is empty", i)
		}
	}
	if r.Role != "" && !CommunityRole(r.Role).IsValid() {
		return ErrInvalidCommunityRole
	}
	return nil
}

// Sanitize sanitizes the bulk add members request.
func (r *BulkAddMembersRequest) Sanitize() {
	r.CommunityID = strings.TrimSpace(r.CommunityID)
	cleaned := make([]string, 0, len(r.UserIDs))
	for _, id := range r.UserIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.UserIDs = cleaned
	if r.Role == "" {
		r.Role = string(RoleMember)
	}
}

// ======================================================================
// Response DTOs
// ======================================================================

// CommunityResponse represents a community in responses.
type CommunityResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	BannerURL   string    `json:"banner_url,omitempty"`
	CreatedBy   string    `json:"created_by"`
	Visibility  string    `json:"visibility"`
	Status      string    `json:"status"`
	MemberCount int64     `json:"member_count"`
	PostCount   int64     `json:"post_count"`
	IsMember    bool      `json:"is_member"`
	IsAdmin     bool      `json:"is_admin"`
	IsModerator bool      `json:"is_moderator"`
	JoinedAt    *time.Time `json:"joined_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags,omitempty"`
}

// CommunityDetailResponse represents a detailed community response.
type CommunityDetailResponse struct {
	CommunityResponse
	Settings      CommunitySettings     `json:"settings"`
	Creator       *MinimalUserResponse  `json:"creator,omitempty"`
	RecentPosts   []TweetResponse       `json:"recent_posts,omitempty"`
	RecentMembers []MinimalUserResponse `json:"recent_members,omitempty"`
	Role          string                `json:"role,omitempty"`
	Permissions   []string              `json:"permissions,omitempty"`
	Stats         CommunityStatsResponse `json:"stats,omitempty"`
}

// CommunityListResponse represents a paginated list of communities.
type CommunityListResponse struct {
	Data       []CommunityResponse `json:"data"`
	Total      int64               `json:"total"`
	NextCursor string              `json:"next_cursor"`
	HasMore    bool                `json:"has_more"`
	Limit      int                 `json:"limit"`
}

// MemberResponse represents a community member in responses.
type MemberResponse struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	IsActive  bool      `json:"is_active"`
}

// MemberListResponse represents a paginated list of members.
type MemberListResponse struct {
	Data       []MemberResponse `json:"data"`
	Total      int64            `json:"total"`
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
	Limit      int              `json:"limit"`
}

// CommunityStatsResponse represents community statistics.
type CommunityStatsResponse struct {
	MemberCount   int64     `json:"member_count"`
	PostCount     int64     `json:"post_count"`
	ActiveMembers int64     `json:"active_members"`
	GrowthRate    float64   `json:"growth_rate"`
	EngagementRate float64  `json:"engagement_rate"`
	LastActivity  time.Time `json:"last_activity"`
	Views         int64     `json:"views"`
}

// CommunityPostListResponse represents a paginated list of community posts.
type CommunityPostListResponse struct {
	Data       []TweetResponse `json:"data"`
	Total      int64           `json:"total"`
	NextCursor string          `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
	Limit      int             `json:"limit"`
}

// ======================================================================
// Helper Functions
// ======================================================================

// isValidSlug checks if a slug is valid.
func isValidSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for _, ch := range slug {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return false
		}
	}
	return true
}

// generateSlug generates a slug from a name.
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	replacer := strings.NewReplacer(
		".", "", ",", "", "!", "", "?", "", "'", "",
		"\"", "", "(", "", ")", "", "[", "", "]", "",
		"{", "", "}", "", ":", "", ";", "", "@", "",
		"#", "", "$", "", "%", "", "^", "", "&", "",
		"*", "", "+", "", "=", "", "`", "", "~", "",
		"|", "", "\\", "", "/", "", ">", "", "<", "",
	)
	slug = replacer.Replace(slug)
	// Collapse multiple hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if len(slug) > MaxCommunitySlugLength {
		slug = slug[:MaxCommunitySlugLength]
	}
	if slug == "" {
		slug = "community-" + time.Now().Format("20060102150405")
	}
	return slug
}

// ======================================================================
// Builder Methods for CommunityResponse
// ======================================================================

// NewCommunityResponse creates a new community response.
func NewCommunityResponse(id, name, slug, createdBy, visibility string) *CommunityResponse {
	return &CommunityResponse{
		ID:          id,
		Name:        name,
		Slug:        slug,
		CreatedBy:   createdBy,
		Visibility:  visibility,
		Status:      string(StatusActive),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

// WithDescription sets the description.
func (r *CommunityResponse) WithDescription(desc string) *CommunityResponse {
	r.Description = desc
	return r
}

// WithAvatarURL sets the avatar URL.
func (r *CommunityResponse) WithAvatarURL(url string) *CommunityResponse {
	r.AvatarURL = url
	return r
}

// WithBannerURL sets the banner URL.
func (r *CommunityResponse) WithBannerURL(url string) *CommunityResponse {
	r.BannerURL = url
	return r
}

// WithMemberCount sets the member count.
func (r *CommunityResponse) WithMemberCount(count int64) *CommunityResponse {
	r.MemberCount = count
	return r
}

// WithPostCount sets the post count.
func (r *CommunityResponse) WithPostCount(count int64) *CommunityResponse {
	r.PostCount = count
	return r
}

// WithIsMember sets the is member flag.
func (r *CommunityResponse) WithIsMember(isMember bool) *CommunityResponse {
	r.IsMember = isMember
	return r
}

// WithIsAdmin sets the is admin flag.
func (r *CommunityResponse) WithIsAdmin(isAdmin bool) *CommunityResponse {
	r.IsAdmin = isAdmin
	r.IsModerator = isAdmin
	return r
}

// WithIsModerator sets the is moderator flag.
func (r *CommunityResponse) WithIsModerator(isModerator bool) *CommunityResponse {
	r.IsModerator = isModerator
	return r
}

// WithJoinedAt sets the joined at time.
func (r *CommunityResponse) WithJoinedAt(t time.Time) *CommunityResponse {
	r.JoinedAt = &t
	return r
}

// ======================================================================
= JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *CommunityResponse) MarshalJSON() ([]byte, error) {
	type Alias CommunityResponse
	return json.Marshal(&struct {
		*Alias
		Visibility string `json:"visibility"`
	}{
		Alias:      (*Alias)(r),
		Visibility: r.Visibility,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *CommunityResponse) UnmarshalJSON(data []byte) error {
	type Alias CommunityResponse
	aux := &struct {
		*Alias
		Visibility string `json:"visibility"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Visibility != "" {
		r.Visibility = aux.Visibility
	}
	return nil
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestCreateCommunityRequest creates a test create request.
func NewTestCreateCommunityRequest() *CreateCommunityRequest {
	return &CreateCommunityRequest{
		Name:        "Test Community",
		Description: "This is a test community",
		Visibility:  VisibilityPublic,
		Settings: CommunitySettings{
			AllowPosts:        true,
			AllowComments:     true,
			AllowMedia:        true,
			RequireApproval:   false,
			MaxMembers:        1000,
			AutoAcceptMembers: true,
			ModerationLevel:   "low",
		},
	}
}

// NewTestCommunityResponse creates a test community response.
func NewTestCommunityResponse() *CommunityResponse {
	resp := NewCommunityResponse("comm1", "Test Community", "test-community", "user1", "public")
	resp.WithDescription("Test description").WithMemberCount(10).WithPostCount(5)
	return resp
}

// NewTestCommunityListResponse creates a test community list response.
func NewTestCommunityListResponse() *CommunityListResponse {
	return &CommunityListResponse{
		Data:       []CommunityResponse{*NewTestCommunityResponse()},
		Total:      1,
		NextCursor: "",
		HasMore:    false,
		Limit:      20,
	}
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagCommunities = "Communities"
)