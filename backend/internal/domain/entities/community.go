// backend/internal/domain/entities/community.go
package entities

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ======================================================================
// Constants
// ======================================================================

// CommunityStatus represents the status of a community.
type CommunityStatus string

const (
	CommunityStatusActive    CommunityStatus = "active"
	CommunityStatusInactive  CommunityStatus = "inactive"
	CommunityStatusSuspended CommunityStatus = "suspended"
	CommunityStatusArchived  CommunityStatus = "archived"
)

// ValidCommunityStatuses returns all valid community statuses.
func ValidCommunityStatuses() []CommunityStatus {
	return []CommunityStatus{
		CommunityStatusActive,
		CommunityStatusInactive,
		CommunityStatusSuspended,
		CommunityStatusArchived,
	}
}

// IsValid checks if a community status is valid.
func (s CommunityStatus) IsValid() bool {
	for _, status := range ValidCommunityStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation of the status.
func (s CommunityStatus) String() string {
	return string(s)
}

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

// String returns the string representation of the visibility.
func (v CommunityVisibility) String() string {
	return string(v)
}

// CommunityRole represents the role of a member in a community.
type CommunityRole string

const (
	RoleOwner      CommunityRole = "owner"
	RoleAdmin      CommunityRole = "admin"
	RoleModerator  CommunityRole = "moderator"
	RoleMember     CommunityRole = "member"
	RoleBanned     CommunityRole = "banned"
)

// ValidCommunityRoles returns all valid community roles.
func ValidCommunityRoles() []CommunityRole {
	return []CommunityRole{
		RoleOwner,
		RoleAdmin,
		RoleModerator,
		RoleMember,
		RoleBanned,
	}
}

// IsValid checks if a community role is valid.
func (r CommunityRole) IsValid() bool {
	for _, role := range ValidCommunityRoles() {
		if r == role {
			return true
		}
	}
	return false
}

// String returns the string representation of the role.
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
// Errors
// ======================================================================

var (
	ErrCommunityIDEmpty         = errors.New("community ID cannot be empty")
	ErrCommunityNameEmpty       = errors.New("community name cannot be empty")
	ErrCommunityNameTooLong     = errors.New("community name exceeds maximum length")
	ErrCommunitySlugEmpty       = errors.New("community slug cannot be empty")
	ErrCommunitySlugInvalid     = errors.New("invalid community slug format")
	ErrCommunitySlugTooLong     = errors.New("community slug exceeds maximum length")
	ErrCommunityDescriptionTooLong = errors.New("community description exceeds maximum length")
	ErrCommunityStatusInvalid   = errors.New("invalid community status")
	ErrCommunityVisibilityInvalid = errors.New("invalid community visibility")
	ErrCommunityAlreadyDeleted  = errors.New("community already deleted")
	ErrCommunityNotDeleted      = errors.New("community is not deleted")
	ErrCommunityOwnerNotFound   = errors.New("community owner not found")
	ErrCommunityMemberNotFound  = errors.New("community member not found")
	ErrCommunityMemberExists    = errors.New("community member already exists")
)

// ======================================================================
// Community Entity
// ======================================================================

// Community represents a community in the system.
type Community struct {
	ID          string              `db:"id" json:"id"`
	Name        string              `db:"name" json:"name"`
	Slug        string              `db:"slug" json:"slug"`
	Description string              `db:"description" json:"description,omitempty"`
	AvatarURL   string              `db:"avatar_url" json:"avatar_url,omitempty"`
	BannerURL   string              `db:"banner_url" json:"banner_url,omitempty"`
	CreatedBy   string              `db:"created_by" json:"created_by"`
	Status      CommunityStatus     `db:"status" json:"status"`
	Visibility  CommunityVisibility `db:"visibility" json:"visibility"`
	MemberCount int64               `db:"member_count" json:"member_count"`
	PostCount   int64               `db:"post_count" json:"post_count"`
	Settings    CommunitySettings   `db:"settings" json:"settings,omitempty"`
	CreatedAt   time.Time           `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time           `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time          `db:"deleted_at" json:"deleted_at,omitempty"`
}

// CommunitySettings represents community settings.
type CommunitySettings struct {
	AllowPosts          bool   `json:"allow_posts"`
	AllowComments       bool   `json:"allow_comments"`
	AllowMedia          bool   `json:"allow_media"`
	RequireApproval     bool   `json:"require_approval"`
	MaxMembers          int64  `json:"max_members"`
	AutoAcceptMembers   bool   `json:"auto_accept_members"`
	AllowedHashtags     []string `json:"allowed_hashtags"`
	ModerationLevel     string `json:"moderation_level"` // "low", "medium", "high"
	WelcomeMessage      string `json:"welcome_message,omitempty"`
	Rules               string `json:"rules,omitempty"`
	CustomCSS           string `json:"custom_css,omitempty"`
	CustomJS            string `json:"custom_js,omitempty"`
}

// DefaultCommunitySettings returns default community settings.
func DefaultCommunitySettings() CommunitySettings {
	return CommunitySettings{
		AllowPosts:        true,
		AllowComments:     true,
		AllowMedia:        true,
		RequireApproval:   false,
		MaxMembers:        0, // unlimited
		AutoAcceptMembers: true,
		AllowedHashtags:   []string{},
		ModerationLevel:   "low",
	}
}

// Value implements driver.Valuer for JSON storage.
func (s CommunitySettings) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements sql.Scanner for JSON retrieval.
func (s *CommunitySettings) Scan(value interface{}) error {
	if value == nil {
		*s = DefaultCommunitySettings()
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for CommunitySettings: %T", value)
	}
	return json.Unmarshal(bytes, s)
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewCommunity creates a new community with default settings.
func NewCommunity(name, slug, description string, createdBy string, visibility CommunityVisibility) (*Community, error) {
	if !visibility.IsValid() {
		visibility = VisibilityPublic
	}
	c := &Community{
		ID:          uuid.New().String(),
		Name:        name,
		Slug:        slug,
		Description: description,
		CreatedBy:   createdBy,
		Status:      CommunityStatusActive,
		Visibility:  visibility,
		MemberCount: 1, // creator is first member
		PostCount:   0,
		Settings:    DefaultCommunitySettings(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// MustNewCommunity creates a new community and panics on error.
func MustNewCommunity(name, slug, description string, createdBy string, visibility CommunityVisibility) *Community {
	c, err := NewCommunity(name, slug, description, createdBy, visibility)
	if err != nil {
		panic(err)
	}
	return c
}

// ======================================================================
// Validation
// ======================================================================

// Validate performs comprehensive validation.
func (c *Community) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return ErrCommunityIDEmpty
	}
	nameTrimmed := strings.TrimSpace(c.Name)
	if nameTrimmed == "" {
		return ErrCommunityNameEmpty
	}
	if len(nameTrimmed) > 100 {
		return ErrCommunityNameTooLong
	}
	c.Name = nameTrimmed
	slugTrimmed := strings.TrimSpace(c.Slug)
	if slugTrimmed == "" {
		return ErrCommunitySlugEmpty
	}
	if len(slugTrimmed) > 50 {
		return ErrCommunitySlugTooLong
	}
	if !isValidSlug(slugTrimmed) {
		return ErrCommunitySlugInvalid
	}
	c.Slug = slugTrimmed
	if len(c.Description) > 500 {
		return ErrCommunityDescriptionTooLong
	}
	if !c.Status.IsValid() {
		return ErrCommunityStatusInvalid
	}
	if !c.Visibility.IsValid() {
		return ErrCommunityVisibilityInvalid
	}
	if c.DeletedAt != nil {
		return ErrCommunityAlreadyDeleted
	}
	if strings.TrimSpace(c.CreatedBy) == "" {
		return ErrCommunityOwnerNotFound
	}
	return nil
}

// isValidSlug checks if a slug is valid.
func isValidSlug(slug string) bool {
	if slug == "" {
		return false
	}
	// Slug should only contain lowercase letters, numbers, and hyphens
	for _, ch := range slug {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return false
		}
	}
	return true
}

// ======================================================================
// Status Management
// ======================================================================

// SetStatus sets the community status.
func (c *Community) SetStatus(status CommunityStatus) error {
	if c.DeletedAt != nil {
		return ErrCommunityAlreadyDeleted
	}
	if !status.IsValid() {
		return ErrCommunityStatusInvalid
	}
	c.Status = status
	c.UpdatedAt = time.Now()
	return nil
}

// Activate activates the community.
func (c *Community) Activate() error {
	return c.SetStatus(CommunityStatusActive)
}

// Inactivate inactivates the community.
func (c *Community) Inactivate() error {
	return c.SetStatus(CommunityStatusInactive)
}

// Suspend suspends the community.
func (c *Community) Suspend() error {
	return c.SetStatus(CommunityStatusSuspended)
}

// Archive archives the community.
func (c *Community) Archive() error {
	return c.SetStatus(CommunityStatusArchived)
}

// IsActive returns true if the community is active.
func (c *Community) IsActive() bool {
	return c.Status == CommunityStatusActive && c.DeletedAt == nil
}

// IsInactive returns true if the community is inactive.
func (c *Community) IsInactive() bool {
	return c.Status == CommunityStatusInactive && c.DeletedAt == nil
}

// IsSuspended returns true if the community is suspended.
func (c *Community) IsSuspended() bool {
	return c.Status == CommunityStatusSuspended && c.DeletedAt == nil
}

// IsArchived returns true if the community is archived.
func (c *Community) IsArchived() bool {
	return c.Status == CommunityStatusArchived
}

// ======================================================================
// Visibility Management
// ======================================================================

// SetVisibility sets the community visibility.
func (c *Community) SetVisibility(visibility CommunityVisibility) error {
	if c.DeletedAt != nil {
		return ErrCommunityAlreadyDeleted
	}
	if !visibility.IsValid() {
		return ErrCommunityVisibilityInvalid
	}
	c.Visibility = visibility
	c.UpdatedAt = time.Now()
	return nil
}

// IsPublic returns true if the community is public.
func (c *Community) IsPublic() bool {
	return c.Visibility == VisibilityPublic
}

// IsPrivate returns true if the community is private.
func (c *Community) IsPrivate() bool {
	return c.Visibility == VisibilityPrivate
}

// IsHidden returns true if the community is hidden.
func (c *Community) IsHidden() bool {
	return c.Visibility == VisibilityHidden
}

// ======================================================================
// Settings Management
// ======================================================================

// UpdateSettings updates community settings.
func (c *Community) UpdateSettings(settings CommunitySettings) error {
	if c.DeletedAt != nil {
		return ErrCommunityAlreadyDeleted
	}
	c.Settings = settings
	c.UpdatedAt = time.Now()
	return nil
}

// AllowPosts returns true if posting is allowed.
func (c *Community) AllowPosts() bool {
	return c.Settings.AllowPosts
}

// AllowComments returns true if commenting is allowed.
func (c *Community) AllowComments() bool {
	return c.Settings.AllowComments
}

// AllowMedia returns true if media is allowed.
func (c *Community) AllowMedia() bool {
	return c.Settings.AllowMedia
}

// RequireApproval returns true if posts require approval.
func (c *Community) RequireApproval() bool {
	return c.Settings.RequireApproval
}

// IsFull returns true if the community has reached max members.
func (c *Community) IsFull() bool {
	return c.Settings.MaxMembers > 0 && c.MemberCount >= c.Settings.MaxMembers
}

// ======================================================================
// Deletion Operations
// ======================================================================

// SoftDelete marks the community as deleted.
func (c *Community) SoftDelete() error {
	if c.DeletedAt != nil {
		return ErrCommunityAlreadyDeleted
	}
	now := time.Now()
	c.DeletedAt = &now
	c.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted community.
func (c *Community) Restore() error {
	if c.DeletedAt == nil {
		return ErrCommunityNotDeleted
	}
	c.DeletedAt = nil
	c.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the community is deleted.
func (c *Community) IsDeleted() bool {
	return c.DeletedAt != nil
}

// ======================================================================
// Count Management
// ======================================================================

// IncrementMemberCount increments the member count.
func (c *Community) IncrementMemberCount() {
	c.MemberCount++
	c.UpdatedAt = time.Now()
}

// DecrementMemberCount decrements the member count (minimum 0).
func (c *Community) DecrementMemberCount() {
	if c.MemberCount > 0 {
		c.MemberCount--
		c.UpdatedAt = time.Now()
	}
}

// IncrementPostCount increments the post count.
func (c *Community) IncrementPostCount() {
	c.PostCount++
	c.UpdatedAt = time.Now()
}

// DecrementPostCount decrements the post count (minimum 0).
func (c *Community) DecrementPostCount() {
	if c.PostCount > 0 {
		c.PostCount--
		c.UpdatedAt = time.Now()
	}
}

// ======================================================================
// Helper Methods
// ======================================================================

// IsOwner checks if a user is the owner.
func (c *Community) IsOwner(userID string) bool {
	return c.CreatedBy == userID
}

// IsCreatedBy checks if the community was created by a user.
func (c *Community) IsCreatedBy(userID string) bool {
	return c.CreatedBy == userID
}

// String returns a human-readable representation.
func (c *Community) String() string {
	return fmt.Sprintf("Community{ID:%s, name:%s, slug:%s, status:%s, members:%d, created:%v}",
		c.ID, c.Name, c.Slug, c.Status, c.MemberCount, c.CreatedAt)
}

// Clone returns a deep copy of the community.
func (c *Community) Clone() *Community {
	clone := *c
	if c.DeletedAt != nil {
		t := *c.DeletedAt
		clone.DeletedAt = &t
	}
	// Settings are already copied by value (struct)
	return &clone
}

// Equals checks if two communities are the same by ID.
func (c *Community) Equals(other *Community) bool {
	return c.ID == other.ID
}

// IsEmpty returns true if the community is zero value.
func (c *Community) IsEmpty() bool {
	return c.ID == "" && c.Name == "" && c.Slug == ""
}

// ======================================================================
// Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (c Community) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (c *Community) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for Community: %T", value)
	}
	return json.Unmarshal(bytes, c)
}

// ======================================================================
// JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (c *Community) MarshalJSON() ([]byte, error) {
	type Alias Community
	return json.Marshal(&struct {
		*Alias
		Status     string `json:"status"`
		Visibility string `json:"visibility"`
		IsActive   bool   `json:"is_active"`
		IsFull     bool   `json:"is_full"`
	}{
		Alias:      (*Alias)(c),
		Status:     string(c.Status),
		Visibility: string(c.Visibility),
		IsActive:   c.IsActive(),
		IsFull:     c.IsFull(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (c *Community) UnmarshalJSON(data []byte) error {
	type Alias Community
	aux := &struct {
		*Alias
		Status     string `json:"status"`
		Visibility string `json:"visibility"`
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Status != "" {
		c.Status = CommunityStatus(aux.Status)
	}
	if aux.Visibility != "" {
		c.Visibility = CommunityVisibility(aux.Visibility)
	}
	return nil
}

// ======================================================================
// Community Member Entity
// ======================================================================

// CommunityMember represents a member of a community.
type CommunityMember struct {
	CommunityID string        `db:"community_id" json:"community_id"`
	UserID      string        `db:"user_id" json:"user_id"`
	Role        CommunityRole `db:"role" json:"role"`
	JoinedAt    time.Time     `db:"joined_at" json:"joined_at"`
	UpdatedAt   time.Time     `db:"updated_at" json:"updated_at"`
}

// NewCommunityMember creates a new community member.
func NewCommunityMember(communityID, userID string, role CommunityRole) (*CommunityMember, error) {
	if !role.IsValid() {
		role = RoleMember
	}
	return &CommunityMember{
		CommunityID: communityID,
		UserID:      userID,
		Role:        role,
		JoinedAt:    time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// IsAdmin checks if the member has admin role.
func (m *CommunityMember) IsAdmin() bool {
	return m.Role.IsAdminRole()
}

// IsModerator checks if the member has moderator role.
func (m *CommunityMember) IsModerator() bool {
	return m.Role.IsModeratorRole()
}

// IsOwner checks if the member is the owner.
func (m *CommunityMember) IsOwner() bool {
	return m.Role == RoleOwner
}

// String returns a human-readable representation.
func (m *CommunityMember) String() string {
	return fmt.Sprintf("CommunityMember{community:%s, user:%s, role:%s, joined:%v}",
		m.CommunityID, m.UserID, m.Role, m.JoinedAt)
}

// ======================================================================
// Community Ban Entity
// ======================================================================

// CommunityBan represents a banned user from a community.
type CommunityBan struct {
	CommunityID string    `db:"community_id" json:"community_id"`
	UserID      string    `db:"user_id" json:"user_id"`
	Reason      string    `db:"reason" json:"reason,omitempty"`
	BannedBy    string    `db:"banned_by" json:"banned_by"`
	BannedAt    time.Time `db:"banned_at" json:"banned_at"`
	ExpiresAt   *time.Time `db:"expires_at" json:"expires_at,omitempty"`
}

// NewCommunityBan creates a new community ban.
func NewCommunityBan(communityID, userID, reason, bannedBy string) *CommunityBan {
	return &CommunityBan{
		CommunityID: communityID,
		UserID:      userID,
		Reason:      reason,
		BannedBy:    bannedBy,
		BannedAt:    time.Now(),
	}
}

// IsExpired checks if the ban has expired.
func (b *CommunityBan) IsExpired() bool {
	if b.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*b.ExpiresAt)
}

// String returns a human-readable representation.
func (b *CommunityBan) String() string {
	return fmt.Sprintf("CommunityBan{community:%s, user:%s, banned_by:%s, at:%v}",
		b.CommunityID, b.UserID, b.BannedBy, b.BannedAt)
}

// ======================================================================
// Community Post Entity
// ======================================================================

// CommunityPost represents a post in a community.
type CommunityPost struct {
	CommunityID string    `db:"community_id" json:"community_id"`
	TweetID     string    `db:"tweet_id" json:"tweet_id"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

// NewCommunityPost creates a new community post.
func NewCommunityPost(communityID, tweetID string) *CommunityPost {
	return &CommunityPost{
		CommunityID: communityID,
		TweetID:     tweetID,
		CreatedAt:   time.Now(),
	}
}

// String returns a human-readable representation.
func (p *CommunityPost) String() string {
	return fmt.Sprintf("CommunityPost{community:%s, tweet:%s, created:%v}",
		p.CommunityID, p.TweetID, p.CreatedAt)
}

// ======================================================================
// Builder Pattern (for tests)
// ======================================================================

// CommunityBuilder helps construct communities for testing.
type CommunityBuilder struct {
	community *Community
}

// NewCommunityBuilder creates a new community builder.
func NewCommunityBuilder() *CommunityBuilder {
	return &CommunityBuilder{
		community: &Community{
			ID:          uuid.New().String(),
			Name:        "",
			Slug:        "",
			Description: "",
			CreatedBy:   "",
			Status:      CommunityStatusActive,
			Visibility:  VisibilityPublic,
			MemberCount: 1,
			PostCount:   0,
			Settings:    DefaultCommunitySettings(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *CommunityBuilder) WithID(id string) *CommunityBuilder {
	b.community.ID = id
	return b
}

// WithName sets the name.
func (b *CommunityBuilder) WithName(name string) *CommunityBuilder {
	b.community.Name = name
	return b
}

// WithSlug sets the slug.
func (b *CommunityBuilder) WithSlug(slug string) *CommunityBuilder {
	b.community.Slug = slug
	return b
}

// WithDescription sets the description.
func (b *CommunityBuilder) WithDescription(desc string) *CommunityBuilder {
	b.community.Description = desc
	return b
}

// WithCreatedBy sets the creator.
func (b *CommunityBuilder) WithCreatedBy(userID string) *CommunityBuilder {
	b.community.CreatedBy = userID
	return b
}

// WithStatus sets the status.
func (b *CommunityBuilder) WithStatus(status CommunityStatus) *CommunityBuilder {
	b.community.Status = status
	return b
}

// WithVisibility sets the visibility.
func (b *CommunityBuilder) WithVisibility(visibility CommunityVisibility) *CommunityBuilder {
	b.community.Visibility = visibility
	return b
}

// WithMemberCount sets the member count.
func (b *CommunityBuilder) WithMemberCount(count int64) *CommunityBuilder {
	b.community.MemberCount = count
	return b
}

// WithPostCount sets the post count.
func (b *CommunityBuilder) WithPostCount(count int64) *CommunityBuilder {
	b.community.PostCount = count
	return b
}

// WithSettings sets the settings.
func (b *CommunityBuilder) WithSettings(settings CommunitySettings) *CommunityBuilder {
	b.community.Settings = settings
	return b
}

// WithCreatedAt sets the creation time.
func (b *CommunityBuilder) WithCreatedAt(t time.Time) *CommunityBuilder {
	b.community.CreatedAt = t
	b.community.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *CommunityBuilder) WithDeleted(t time.Time) *CommunityBuilder {
	b.community.DeletedAt = &t
	return b
}

// Build validates and returns the community.
func (b *CommunityBuilder) Build() (*Community, error) {
	if err := b.community.Validate(); err != nil {
		return nil, err
	}
	return b.community, nil
}

// MustBuild builds without error (panics on error).
func (b *CommunityBuilder) MustBuild() *Community {
	c, err := b.Build()
	if err != nil {
		panic(err)
	}
	return c
}

// ======================================================================
// Test Helpers
// ======================================================================

var (
	TestCommunity1 = MustNewCommunity("Test Community", "test-community", "Test description", "user1", VisibilityPublic)
	TestCommunity2 = MustNewCommunity("Private Community", "private-community", "Private description", "user2", VisibilityPrivate)
)

// MustNewCommunityWithSettings creates a community with settings.
func MustNewCommunityWithSettings(name, slug, description, createdBy string, visibility CommunityVisibility, settings CommunitySettings) *Community {
	c, err := NewCommunity(name, slug, description, createdBy, visibility)
	if err != nil {
		panic(err)
	}
	c.Settings = settings
	return c
}

// MustNewDeletedCommunity creates a deleted community for testing.
func MustNewDeletedCommunity(name, slug, description, createdBy string, visibility CommunityVisibility) *Community {
	c, err := NewCommunity(name, slug, description, createdBy, visibility)
	if err != nil {
		panic(err)
	}
	if err := c.SoftDelete(); err != nil {
		panic(err)
	}
	return c
}