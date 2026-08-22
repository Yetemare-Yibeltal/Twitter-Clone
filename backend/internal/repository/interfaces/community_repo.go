// backend/internal/repository/interfaces/community_repo.go
package interfaces

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ======================================================================
// Common Errors
// ======================================================================

var (
	ErrCommunityNotFound      = errors.New("community not found")
	ErrCommunityDeleted       = errors.New("community has been deleted")
	ErrDuplicateSlug          = errors.New("slug already exists")
	ErrMemberNotFound         = errors.New("member not found")
	ErrMemberAlreadyExists    = errors.New("member already exists")
	ErrNotCommunityMember     = errors.New("user is not a member of this community")
	ErrNotCommunityAdmin      = errors.New("user is not an admin of this community")
	ErrNotCommunityModerator  = errors.New("user is not a moderator of this community")
	ErrInvalidCommunityRole   = errors.New("invalid community role")
	ErrCommunityFull          = errors.New("community has reached maximum members")
	ErrCommunityPrivate       = errors.New("community is private")
	ErrUserAlreadyBanned      = errors.New("user is already banned from this community")
	ErrBanNotFound            = errors.New("ban not found")
	ErrPostNotFound           = errors.New("post not found")
	ErrPostAlreadyExists      = errors.New("post already exists in this community")
	ErrCannotRemoveOwner      = errors.New("cannot remove the community owner")
	ErrCannotDemoteOwner      = errors.New("cannot demote the community owner")
	ErrCommunityNameRequired  = errors.New("community name is required")
	ErrCommunityNameTooLong   = errors.New("community name is too long")
	ErrCommunityDescriptionTooLong = errors.New("community description is too long")
	ErrCommunitySlugRequired  = errors.New("community slug is required")
	ErrInvalidCommunitySlug   = errors.New("invalid community slug format")
)

// ======================================================================
// CommunityFilter
// ======================================================================

// CommunityFilter defines filtering options for community queries.
type CommunityFilter struct {
	Name        *string
	Slug        *string
	CreatedBy   *string
	IsPrivate   *bool
	IsActive    *bool
	HasMember   *string
	MinMembers  *int64
	MaxMembers  *int64
	MinPosts    *int64
	MaxPosts    *int64
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Search      *string // full-text search
	Tag         *string
}

// HasCriteria checks if any filter criteria are set.
func (f *CommunityFilter) HasCriteria() bool {
	return f.Name != nil || f.Slug != nil || f.CreatedBy != nil ||
		f.IsPrivate != nil || f.IsActive != nil || f.HasMember != nil ||
		f.MinMembers != nil || f.MaxMembers != nil || f.MinPosts != nil ||
		f.MaxPosts != nil || f.CreatedFrom != nil || f.CreatedTo != nil ||
		f.Search != nil || f.Tag != nil
}

// ======================================================================
// CommunityPagination
// ======================================================================

// CommunitySortField defines sortable fields for communities.
type CommunitySortField string

const (
	SortCommunityByCreatedAt   CommunitySortField = "created_at"
	SortCommunityByUpdatedAt   CommunitySortField = "updated_at"
	SortCommunityByName        CommunitySortField = "name"
	SortCommunityByMemberCount CommunitySortField = "member_count"
	SortCommunityByPostCount   CommunitySortField = "post_count"
)

// CommunitySortOrder defines sort order.
type CommunitySortOrder string

const (
	CommunitySortAsc  CommunitySortOrder = "ASC"
	CommunitySortDesc CommunitySortOrder = "DESC"
)

// CommunityPagination holds pagination options for communities.
type CommunityPagination struct {
	Cursor string               `json:"cursor"`
	Limit  int                  `json:"limit"`
	SortBy CommunitySortField   `json:"sort_by"`
	Order  CommunitySortOrder   `json:"order"`
}

// DefaultCommunityPagination returns default pagination options.
func DefaultCommunityPagination() *CommunityPagination {
	return &CommunityPagination{
		Limit:  20,
		SortBy: SortCommunityByCreatedAt,
		Order:  CommunitySortDesc,
	}
}

// Validate checks pagination parameters.
func (p *CommunityPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// CommunityStats
// ======================================================================

// CommunityStats represents aggregated community statistics.
type CommunityStats struct {
	TotalCommunities int64     `json:"total_communities"`
	PublicCommunities int64    `json:"public_communities"`
	PrivateCommunities int64   `json:"private_communities"`
	TotalMembers     int64     `json:"total_members"`
	TotalPosts       int64     `json:"total_posts"`
	AverageMembers   float64   `json:"average_members"`
	AveragePosts     float64   `json:"average_posts"`
	MaxMembers       int64     `json:"max_members"`
	MinMembers       int64     `json:"min_members"`
	LastCommunityCreated time.Time `json:"last_community_created"`
	MostActiveCommunityID string `json:"most_active_community_id"`
	MostActiveCommunityPosts int64 `json:"most_active_community_posts"`
	MostPopularCommunityID string `json:"most_popular_community_id"`
	MostPopularCommunityMembers int64 `json:"most_popular_community_members"`
}

// ======================================================================
// DailyCommunityCount
// ======================================================================

// DailyCommunityCount represents daily community counts.
type DailyCommunityCount struct {
	Date         time.Time `json:"date"`
	Total        int64     `json:"total"`
	Public       int64     `json:"public"`
	Private      int64     `json:"private"`
	NewMembers   int64     `json:"new_members"`
	NewPosts     int64     `json:"new_posts"`
}

// ======================================================================
// CommunityMember
// ======================================================================

// CommunityMember represents a community member with role.
type CommunityMember struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	AvatarURL  string    `json:"avatar_url"`
	Role       string    `json:"role"`
	JoinedAt   time.Time `json:"joined_at"`
	IsActive   bool      `json:"is_active"`
}

// ======================================================================
// CommunityBan
// ======================================================================

// CommunityBan represents a banned user.
type CommunityBan struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	AvatarURL  string    `json:"avatar_url"`
	Reason     string    `json:"reason"`
	BannedAt   time.Time `json:"banned_at"`
	BannedBy   string    `json:"banned_by"`
}

// ======================================================================
// CommunityRepository Interface
// ======================================================================

// CommunityRepository defines the interface for community data persistence.
type CommunityRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a new community.
	Create(ctx context.Context, community *entities.Community) error

	// GetByID retrieves a community by its ID.
	GetByID(ctx context.Context, id string) (*entities.Community, error)

	// GetBySlug retrieves a community by its slug.
	GetBySlug(ctx context.Context, slug string) (*entities.Community, error)

	// GetByIDs retrieves multiple communities by their IDs.
	GetByIDs(ctx context.Context, ids []string) ([]*entities.Community, error)

	// Update updates a community.
	Update(ctx context.Context, community *entities.Community) error

	// SoftDelete marks a community as deleted.
	SoftDelete(ctx context.Context, id string) error

	// HardDelete permanently removes a community.
	HardDelete(ctx context.Context, id string) error

	// Restore restores a soft-deleted community.
	Restore(ctx context.Context, id string) error

	// --------------------------------------------------------------------
	// List and Search
	// --------------------------------------------------------------------

	// List returns communities with filtering and pagination.
	List(ctx context.Context, filter *CommunityFilter, pagination *CommunityPagination) ([]*entities.Community, int64, error)

	// Search performs full-text search on communities.
	Search(ctx context.Context, query string, pagination *CommunityPagination) ([]*entities.Community, int64, error)

	// GetByUserID returns communities created by a user.
	GetByUserID(ctx context.Context, userID string, pagination *CommunityPagination) ([]*entities.Community, int64, error)

	// GetByMemberID returns communities a user is a member of.
	GetByMemberID(ctx context.Context, userID string, pagination *CommunityPagination) ([]*entities.Community, int64, error)

	// GetByAdminID returns communities where a user is an admin.
	GetByAdminID(ctx context.Context, userID string, pagination *CommunityPagination) ([]*entities.Community, int64, error)

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountTotal returns total number of communities.
	CountTotal(ctx context.Context) (int64, error)

	// CountByUserID returns number of communities created by a user.
	CountByUserID(ctx context.Context, userID string) (int64, error)

	// CountByMemberID returns number of communities a user is a member of.
	CountByMemberID(ctx context.Context, userID string) (int64, error)

	// CountByDateRange returns community count within a date range.
	CountByDateRange(ctx context.Context, start, end time.Time) (int64, error)

	// --------------------------------------------------------------------
	// Membership Management
	// --------------------------------------------------------------------

	// AddMember adds a user to a community with a role.
	AddMember(ctx context.Context, communityID, userID string, role string) error

	// RemoveMember removes a user from a community.
	RemoveMember(ctx context.Context, communityID, userID string) error

	// UpdateMemberRole updates the role of a community member.
	UpdateMemberRole(ctx context.Context, communityID, userID, newRole string) error

	// GetMemberRole returns the role of a member in a community.
	GetMemberRole(ctx context.Context, communityID, userID string) (string, error)

	// IsMember checks if a user is a member of a community.
	IsMember(ctx context.Context, communityID, userID string) (bool, error)

	// IsAdmin checks if a user is an admin of a community.
	IsAdmin(ctx context.Context, communityID, userID string) (bool, error)

	// IsModerator checks if a user is a moderator of a community.
	IsModerator(ctx context.Context, communityID, userID string) (bool, error)

	// GetMemberCount returns the number of members in a community.
	GetMemberCount(ctx context.Context, communityID string) (int64, error)

	// GetMembers returns members of a community with pagination and role filter.
	GetMembers(ctx context.Context, communityID string, role string, cursor string, limit int) ([]*CommunityMember, string, error)

	// GetMemberUserIDs returns all user IDs of members.
	GetMemberUserIDs(ctx context.Context, communityID string) ([]string, error)

	// GetUserCommunities returns communities a user belongs to.
	GetUserCommunities(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Community, string, error)

	// --------------------------------------------------------------------
	// Moderation - Bans
	// --------------------------------------------------------------------

	// BanUser bans a user from a community.
	BanUser(ctx context.Context, communityID, userID, reason string) error

	// UnbanUser removes a ban from a user.
	UnbanUser(ctx context.Context, communityID, userID string) error

	// IsBanned checks if a user is banned from a community.
	IsBanned(ctx context.Context, communityID, userID string) (bool, error)

	// GetBannedUsers returns banned users for a community.
	GetBannedUsers(ctx context.Context, communityID string, cursor string, limit int) ([]*CommunityBan, string, error)

	// GetBanReason returns the reason a user was banned.
	GetBanReason(ctx context.Context, communityID, userID string) (string, error)

	// --------------------------------------------------------------------
	// Community Posts
	// --------------------------------------------------------------------

	// AddPost adds a tweet to a community as a post.
	AddPost(ctx context.Context, communityID, tweetID string) error

	// RemovePost removes a post from a community.
	RemovePost(ctx context.Context, communityID, tweetID string) error

	// GetPosts returns posts (tweets) in a community with pagination.
	GetPosts(ctx context.Context, communityID string, cursor string, limit int) ([]*entities.Tweet, string, error)

	// GetPostCount returns the number of posts in a community.
	GetPostCount(ctx context.Context, communityID string) (int64, error)

	// GetPostByTweetID checks if a tweet is posted in a community.
	GetPostByTweetID(ctx context.Context, communityID, tweetID string) (bool, error)

	// GetPostsByDateRange returns posts within a date range.
	GetPostsByDateRange(ctx context.Context, communityID string, start, end time.Time, cursor string, limit int) ([]*entities.Tweet, string, error)

	// GetTopPosts returns the most popular posts in a community.
	GetTopPosts(ctx context.Context, communityID string, limit int, since time.Time) ([]*entities.Tweet, error)

	// --------------------------------------------------------------------
	// Role-based Permissions
	// --------------------------------------------------------------------

	// GetCommunityRoles returns all roles and their permissions.
	GetCommunityRoles(ctx context.Context) ([]*CommunityRole, error)

	// GetUserPermissions returns permissions for a user in a community.
	GetUserPermissions(ctx context.Context, communityID, userID string) (*CommunityPermissions, error)

	// SetCustomRole sets custom role permissions for a community.
	SetCustomRole(ctx context.Context, communityID, role string, permissions []string) error

	// --------------------------------------------------------------------
	// Advanced Queries
	// --------------------------------------------------------------------

	// GetTrendingCommunities returns trending communities.
	GetTrendingCommunities(ctx context.Context, limit int, since time.Time) ([]*entities.Community, error)

	// GetSimilarCommunities returns similar communities.
	GetSimilarCommunities(ctx context.Context, communityID string, limit int) ([]*entities.Community, error)

	// GetRecommendations returns recommended communities for a user.
	GetRecommendations(ctx context.Context, userID string, limit int) ([]*entities.Community, error)

	// GetCommunitiesByTags returns communities with specific tags.
	GetCommunitiesByTags(ctx context.Context, tags []string, pagination *CommunityPagination) ([]*entities.Community, int64, error)

	// GetActivitySummary returns community activity summary.
	GetActivitySummary(ctx context.Context, communityID string) (*CommunityActivitySummary, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple communities in a transaction.
	BulkCreate(ctx context.Context, communities []*entities.Community) error

	// BulkDelete removes multiple communities.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkAddMembers adds multiple members to a community.
	BulkAddMembers(ctx context.Context, communityID string, userIDs []string, role string) error

	// BulkRemoveMembers removes multiple members from a community.
	BulkRemoveMembers(ctx context.Context, communityID string, userIDs []string) error

	// BulkUpdateRoles updates roles for multiple members.
	BulkUpdateRoles(ctx context.Context, communityID string, updates map[string]string) error

	// BulkBanUsers bans multiple users from a community.
	BulkBanUsers(ctx context.Context, communityID string, userIDs []string, reason string) error

	// BulkUnbanUsers unbans multiple users from a community.
	BulkUnbanUsers(ctx context.Context, communityID string, userIDs []string) error

	// BulkAddPosts adds multiple posts to a community.
	BulkAddPosts(ctx context.Context, communityID string, tweetIDs []string) error

	// BulkRemovePosts removes multiple posts from a community.
	BulkRemovePosts(ctx context.Context, communityID string, tweetIDs []string) error

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetCommunityStats returns aggregated community statistics.
	GetCommunityStats(ctx context.Context) (*CommunityStats, error)

	// GetUserCommunityStats returns community stats for a specific user.
	GetUserCommunityStats(ctx context.Context, userID string) (*CommunityStats, error)

	// GetDailyCommunityStats returns daily community counts for a date range.
	GetDailyCommunityStats(ctx context.Context, start, end time.Time) ([]*DailyCommunityCount, error)

	// GetCommunityGrowthRate calculates community growth rate over a period.
	GetCommunityGrowthRate(ctx context.Context, communityID string, days int) (float64, error)

	// GetTopCommunities returns the top communities by member count or post count.
	GetTopCommunities(ctx context.Context, sortBy string, limit int) ([]*entities.Community, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) CommunityRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo CommunityRepository) error) error

	// --------------------------------------------------------------------
	// Health and Cleanup
	// --------------------------------------------------------------------

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close releases any resources.
	Close() error

	// GetRawDB returns the underlying database connection.
	GetRawDB() interface{}
}

// ======================================================================
// Supporting Types
// ======================================================================

// CommunityRole represents a community role with permissions.
type CommunityRole struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	IsDefault   bool     `json:"is_default"`
}

// CommunityPermissions represents permissions in a community.
type CommunityPermissions struct {
	CanPost      bool `json:"can_post"`
	CanComment   bool `json:"can_comment"`
	CanVote      bool `json:"can_vote"`
	CanModerate  bool `json:"can_moderate"`
	CanManage    bool `json:"can_manage"`
	CanInvite    bool `json:"can_invite"`
	CanBan       bool `json:"can_ban"`
	CanPin       bool `json:"can_pin"`
	CanDelete    bool `json:"can_delete"`
	IsAdmin      bool `json:"is_admin"`
	IsModerator  bool `json:"is_moderator"`
}

// CommunityActivitySummary represents community activity.
type CommunityActivitySummary struct {
	NewMembers    int64     `json:"new_members"`
	NewPosts      int64     `json:"new_posts"`
	ActiveMembers int64     `json:"active_members"`
	EngagementRate float64  `json:"engagement_rate"`
	LastActivity  time.Time `json:"last_activity"`
	TopPostID     string    `json:"top_post_id"`
	TopPostLikes  int64     `json:"top_post_likes"`
}

// ======================================================================
// Helper Functions
// ======================================================================

// IsCommunityNotFound checks if an error indicates a community was not found.
func IsCommunityNotFound(err error) bool {
	return errors.Is(err, ErrCommunityNotFound) || errors.Is(err, ErrCommunityDeleted)
}

// IsCommunityMemberError checks if an error is membership-related.
func IsCommunityMemberError(err error) bool {
	return errors.Is(err, ErrNotCommunityMember) ||
		errors.Is(err, ErrMemberNotFound) ||
		errors.Is(err, ErrMemberAlreadyExists)
}

// IsCommunityPermissionError checks if an error is permission-related.
func IsCommunityPermissionError(err error) bool {
	return errors.Is(err, ErrNotCommunityAdmin) ||
		errors.Is(err, ErrNotCommunityModerator)
}

// ======================================================================
// CommunityRole Constants
// ======================================================================

const (
	CommunityRoleOwner      = "owner"
	CommunityRoleAdmin     = "admin"
	CommunityRoleModerator = "moderator"
	CommunityRoleMember    = "member"
	CommunityRoleBanned    = "banned"
)

// ======================================================================
// Mock Community Repository (for testing)
// ======================================================================

// MockCommunityRepository is a mock implementation for testing.
type MockCommunityRepository struct {
	Communities  map[string]*entities.Community
	Members      map[string]map[string]string // communityID -> userID -> role
	Bans         map[string]map[string]string // communityID -> userID -> reason
	Posts        map[string][]string          // communityID -> tweetIDs
	Error        error
	NextCursor   string
}

// NewMockCommunityRepo creates a new mock repository.
func NewMockCommunityRepo() CommunityRepository {
	return &MockCommunityRepository{
		Communities: make(map[string]*entities.Community),
		Members:     make(map[string]map[string]string),
		Bans:        make(map[string]map[string]string),
		Posts:       make(map[string][]string),
	}
}

// Create mock implementation.
func (m *MockCommunityRepository) Create(ctx context.Context, community *entities.Community) error {
	if m.Error != nil {
		return m.Error
	}
	// Check duplicate slug
	for _, c := range m.Communities {
		if c.Slug == community.Slug {
			return ErrDuplicateSlug
		}
	}
	m.Communities[community.ID] = community
	return nil
}

// GetByID mock implementation.
func (m *MockCommunityRepository) GetByID(ctx context.Context, id string) (*entities.Community, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if community, ok := m.Communities[id]; ok && community.DeletedAt == nil {
		return community, nil
	}
	return nil, ErrCommunityNotFound
}

// GetBySlug mock implementation.
func (m *MockCommunityRepository) GetBySlug(ctx context.Context, slug string) (*entities.Community, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	for _, community := range m.Communities {
		if community.Slug == slug && community.DeletedAt == nil {
			return community, nil
		}
	}
	return nil, ErrCommunityNotFound
}

// GetByIDs mock implementation.
func (m *MockCommunityRepository) GetByIDs(ctx context.Context, ids []string) ([]*entities.Community, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var communities []*entities.Community
	for _, id := range ids {
		if c, ok := m.Communities[id]; ok && c.DeletedAt == nil {
			communities = append(communities, c)
		}
	}
	return communities, nil
}

// Update mock implementation.
func (m *MockCommunityRepository) Update(ctx context.Context, community *entities.Community) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Communities[community.ID]; !ok {
		return ErrCommunityNotFound
	}
	m.Communities[community.ID] = community
	return nil
}

// SoftDelete mock implementation.
func (m *MockCommunityRepository) SoftDelete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if community, ok := m.Communities[id]; ok {
		now := time.Now()
		community.DeletedAt = &now
		return nil
	}
	return ErrCommunityNotFound
}

// HardDelete mock implementation.
func (m *MockCommunityRepository) HardDelete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Communities[id]; ok {
		delete(m.Communities, id)
		delete(m.Members, id)
		delete(m.Bans, id)
		delete(m.Posts, id)
		return nil
	}
	return ErrCommunityNotFound
}

// Restore mock implementation.
func (m *MockCommunityRepository) Restore(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if community, ok := m.Communities[id]; ok && community.DeletedAt != nil {
		community.DeletedAt = nil
		return nil
	}
	return ErrCommunityNotFound
}

// List mock implementation.
func (m *MockCommunityRepository) List(ctx context.Context, filter *CommunityFilter, pagination *CommunityPagination) ([]*entities.Community, int64, error) {
	if m.Error != nil {
		return nil, 0, m.Error
	}
	var communities []*entities.Community
	for _, c := range m.Communities {
		if c.DeletedAt == nil {
			communities = append(communities, c)
		}
	}
	return communities, int64(len(communities)), nil
}

// Search mock implementation.
func (m *MockCommunityRepository) Search(ctx context.Context, query string, pagination *CommunityPagination) ([]*entities.Community, int64, error) {
	if m.Error != nil {
		return nil, 0, m.Error
	}
	var communities []*entities.Community
	for _, c := range m.Communities {
		if c.DeletedAt == nil && strings.Contains(strings.ToLower(c.Name), strings.ToLower(query)) {
			communities = append(communities, c)
		}
	}
	return communities, int64(len(communities)), nil
}

// GetByUserID mock implementation.
func (m *MockCommunityRepository) GetByUserID(ctx context.Context, userID string, pagination *CommunityPagination) ([]*entities.Community, int64, error) {
	if m.Error != nil {
		return nil, 0, m.Error
	}
	var communities []*entities.Community
	for _, c := range m.Communities {
		if c.DeletedAt == nil && c.CreatedBy == userID {
			communities = append(communities, c)
		}
	}
	return communities, int64(len(communities)), nil
}

// GetByMemberID mock implementation.
func (m *MockCommunityRepository) GetByMemberID(ctx context.Context, userID string, pagination *CommunityPagination) ([]*entities.Community, int64, error) {
	if m.Error != nil {
		return nil, 0, m.Error
	}
	var communities []*entities.Community
	for cid, members := range m.Members {
		if _, ok := members[userID]; ok {
			if c, ok := m.Communities[cid]; ok && c.DeletedAt == nil {
				communities = append(communities, c)
			}
		}
	}
	return communities, int64(len(communities)), nil
}

// GetByAdminID mock implementation.
func (m *MockCommunityRepository) GetByAdminID(ctx context.Context, userID string, pagination *CommunityPagination) ([]*entities.Community, int64, error) {
	if m.Error != nil {
		return nil, 0, m.Error
	}
	var communities []*entities.Community
	for cid, members := range m.Members {
		if role, ok := members[userID]; ok && (role == CommunityRoleAdmin || role == CommunityRoleOwner) {
			if c, ok := m.Communities[cid]; ok && c.DeletedAt == nil {
				communities = append(communities, c)
			}
		}
	}
	return communities, int64(len(communities)), nil
}

// CountTotal mock implementation.
func (m *MockCommunityRepository) CountTotal(ctx context.Context) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, c := range m.Communities {
		if c.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

// CountByUserID mock implementation.
func (m *MockCommunityRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, c := range m.Communities {
		if c.DeletedAt == nil && c.CreatedBy == userID {
			count++
		}
	}
	return count, nil
}

// CountByMemberID mock implementation.
func (m *MockCommunityRepository) CountByMemberID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for cid, members := range m.Members {
		if _, ok := members[userID]; ok {
			if c, ok := m.Communities[cid]; ok && c.DeletedAt == nil {
				count++
			}
		}
	}
	return count, nil
}

// CountByDateRange mock implementation.
func (m *MockCommunityRepository) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, c := range m.Communities {
		if c.DeletedAt == nil && c.CreatedAt.After(start) && c.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// AddMember mock implementation.
func (m *MockCommunityRepository) AddMember(ctx context.Context, communityID, userID string, role string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Communities[communityID]; !ok {
		return ErrCommunityNotFound
	}
	if m.Members[communityID] == nil {
		m.Members[communityID] = make(map[string]string)
	}
	if _, ok := m.Members[communityID][userID]; ok {
		return ErrMemberAlreadyExists
	}
	m.Members[communityID][userID] = role
	// Update member count
	if c, ok := m.Communities[communityID]; ok {
		c.MemberCount++
	}
	return nil
}

// RemoveMember mock implementation.
func (m *MockCommunityRepository) RemoveMember(ctx context.Context, communityID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Communities[communityID]; !ok {
		return ErrCommunityNotFound
	}
	if m.Members[communityID] == nil {
		return ErrMemberNotFound
	}
	if _, ok := m.Members[communityID][userID]; !ok {
		return ErrMemberNotFound
	}
	delete(m.Members[communityID], userID)
	if c, ok := m.Communities[communityID]; ok && c.MemberCount > 0 {
		c.MemberCount--
	}
	return nil
}

// UpdateMemberRole mock implementation.
func (m *MockCommunityRepository) UpdateMemberRole(ctx context.Context, communityID, userID, newRole string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Communities[communityID]; !ok {
		return ErrCommunityNotFound
	}
	if m.Members[communityID] == nil {
		return ErrMemberNotFound
	}
	if _, ok := m.Members[communityID][userID]; !ok {
		return ErrMemberNotFound
	}
	if m.Members[communityID][userID] == CommunityRoleOwner && newRole != CommunityRoleOwner {
		return ErrCannotDemoteOwner
	}
	m.Members[communityID][userID] = newRole
	return nil
}

// GetMemberRole mock implementation.
func (m *MockCommunityRepository) GetMemberRole(ctx context.Context, communityID, userID string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	if m.Members[communityID] == nil {
		return "", ErrMemberNotFound
	}
	if role, ok := m.Members[communityID][userID]; ok {
		return role, nil
	}
	return "", ErrMemberNotFound
}

// IsMember mock implementation.
func (m *MockCommunityRepository) IsMember(ctx context.Context, communityID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Members[communityID] == nil {
		return false, nil
	}
	_, ok := m.Members[communityID][userID]
	return ok, nil
}

// IsAdmin mock implementation.
func (m *MockCommunityRepository) IsAdmin(ctx context.Context, communityID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Members[communityID] == nil {
		return false, nil
	}
	role, ok := m.Members[communityID][userID]
	return ok && (role == CommunityRoleAdmin || role == CommunityRoleOwner), nil
}

// IsModerator mock implementation.
func (m *MockCommunityRepository) IsModerator(ctx context.Context, communityID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Members[communityID] == nil {
		return false, nil
	}
	role, ok := m.Members[communityID][userID]
	return ok && (role == CommunityRoleModerator || role == CommunityRoleAdmin || role == CommunityRoleOwner), nil
}

// GetMemberCount mock implementation.
func (m *MockCommunityRepository) GetMemberCount(ctx context.Context, communityID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.Members[communityID] == nil {
		return 0, nil
	}
	return int64(len(m.Members[communityID])), nil
}

// GetMembers mock implementation.
func (m *MockCommunityRepository) GetMembers(ctx context.Context, communityID string, role string, cursor string, limit int) ([]*CommunityMember, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var members []*CommunityMember
	if m.Members[communityID] == nil {
		return members, "", nil
	}
	for userID, userRole := range m.Members[communityID] {
		if role == "" || userRole == role {
			members = append(members, &CommunityMember{
				UserID:   userID,
				Role:     userRole,
				JoinedAt: time.Now(),
			})
		}
	}
	return members, "", nil
}

// GetMemberUserIDs mock implementation.
func (m *MockCommunityRepository) GetMemberUserIDs(ctx context.Context, communityID string) ([]string, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var ids []string
	if m.Members[communityID] != nil {
		for id := range m.Members[communityID] {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// GetUserCommunities mock implementation.
func (m *MockCommunityRepository) GetUserCommunities(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Community, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var communities []*entities.Community
	for cid, members := range m.Members {
		if _, ok := members[userID]; ok {
			if c, ok := m.Communities[cid]; ok && c.DeletedAt == nil {
				communities = append(communities, c)
			}
		}
	}
	return communities, "", nil
}

// BanUser mock implementation.
func (m *MockCommunityRepository) BanUser(ctx context.Context, communityID, userID, reason string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Communities[communityID]; !ok {
		return ErrCommunityNotFound
	}
	if m.Bans[communityID] == nil {
		m.Bans[communityID] = make(map[string]string)
	}
	if _, ok := m.Bans[communityID][userID]; ok {
		return ErrUserAlreadyBanned
	}
	m.Bans[communityID][userID] = reason
	// Also remove from members if present
	_ = m.RemoveMember(ctx, communityID, userID)
	return nil
}

// UnbanUser mock implementation.
func (m *MockCommunityRepository) UnbanUser(ctx context.Context, communityID, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if m.Bans[communityID] == nil {
		return ErrBanNotFound
	}
	if _, ok := m.Bans[communityID][userID]; !ok {
		return ErrBanNotFound
	}
	delete(m.Bans[communityID], userID)
	return nil
}

// IsBanned mock implementation.
func (m *MockCommunityRepository) IsBanned(ctx context.Context, communityID, userID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Bans[communityID] == nil {
		return false, nil
	}
	_, ok := m.Bans[communityID][userID]
	return ok, nil
}

// GetBannedUsers mock implementation.
func (m *MockCommunityRepository) GetBannedUsers(ctx context.Context, communityID string, cursor string, limit int) ([]*CommunityBan, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var bans []*CommunityBan
	if m.Bans[communityID] != nil {
		for userID, reason := range m.Bans[communityID] {
			bans = append(bans, &CommunityBan{
				UserID:   userID,
				Reason:   reason,
				BannedAt: time.Now(),
			})
		}
	}
	return bans, "", nil
}

// GetBanReason mock implementation.
func (m *MockCommunityRepository) GetBanReason(ctx context.Context, communityID, userID string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	if m.Bans[communityID] == nil {
		return "", ErrBanNotFound
	}
	if reason, ok := m.Bans[communityID][userID]; ok {
		return reason, nil
	}
	return "", ErrBanNotFound
}

// AddPost mock implementation.
func (m *MockCommunityRepository) AddPost(ctx context.Context, communityID, tweetID string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Communities[communityID]; !ok {
		return ErrCommunityNotFound
	}
	if m.Posts[communityID] == nil {
		m.Posts[communityID] = []string{}
	}
	// Check if already exists
	for _, id := range m.Posts[communityID] {
		if id == tweetID {
			return ErrPostAlreadyExists
		}
	}
	m.Posts[communityID] = append(m.Posts[communityID], tweetID)
	if c, ok := m.Communities[communityID]; ok {
		c.PostCount++
	}
	return nil
}

// RemovePost mock implementation.
func (m *MockCommunityRepository) RemovePost(ctx context.Context, communityID, tweetID string) error {
	if m.Error != nil {
		return m.Error
	}
	if m.Posts[communityID] == nil {
		return ErrPostNotFound
	}
	for i, id := range m.Posts[communityID] {
		if id == tweetID {
			m.Posts[communityID] = append(m.Posts[communityID][:i], m.Posts[communityID][i+1:]...)
			if c, ok := m.Communities[communityID]; ok && c.PostCount > 0 {
				c.PostCount--
			}
			return nil
		}
	}
	return ErrPostNotFound
}

// GetPosts mock implementation.
func (m *MockCommunityRepository) GetPosts(ctx context.Context, communityID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	// Return empty list since we don't have tweet entities in mock
	return []*entities.Tweet{}, "", nil
}

// GetPostCount mock implementation.
func (m *MockCommunityRepository) GetPostCount(ctx context.Context, communityID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.Posts[communityID] == nil {
		return 0, nil
	}
	return int64(len(m.Posts[communityID])), nil
}

// GetPostByTweetID mock implementation.
func (m *MockCommunityRepository) GetPostByTweetID(ctx context.Context, communityID, tweetID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.Posts[communityID] == nil {
		return false, nil
	}
	for _, id := range m.Posts[communityID] {
		if id == tweetID {
			return true, nil
		}
	}
	return false, nil
}

// GetPostsByDateRange mock implementation.
func (m *MockCommunityRepository) GetPostsByDateRange(ctx context.Context, communityID string, start, end time.Time, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*entities.Tweet{}, "", nil
}

// GetTopPosts mock implementation.
func (m *MockCommunityRepository) GetTopPosts(ctx context.Context, communityID string, limit int, since time.Time) ([]*entities.Tweet, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Tweet{}, nil
}

// GetCommunityRoles mock implementation.
func (m *MockCommunityRepository) GetCommunityRoles(ctx context.Context) ([]*CommunityRole, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*CommunityRole{
		{ID: "1", Name: CommunityRoleOwner, Description: "Owner of the community", Permissions: []string{"*"}},
		{ID: "2", Name: CommunityRoleAdmin, Description: "Administrator of the community", Permissions: []string{"*"}},
		{ID: "3", Name: CommunityRoleModerator, Description: "Moderator of the community", Permissions: []string{"moderate"}},
		{ID: "4", Name: CommunityRoleMember, Description: "Member of the community", Permissions: []string{"post", "comment"}},
	}, nil
}

// GetUserPermissions mock implementation.
func (m *MockCommunityRepository) GetUserPermissions(ctx context.Context, communityID, userID string) (*CommunityPermissions, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	role, err := m.GetMemberRole(ctx, communityID, userID)
	if err != nil {
		return &CommunityPermissions{}, nil
	}
	perms := &CommunityPermissions{}
	switch role {
	case CommunityRoleOwner, CommunityRoleAdmin:
		perms.CanPost = true
		perms.CanComment = true
		perms.CanVote = true
		perms.CanModerate = true
		perms.CanManage = true
		perms.CanInvite = true
		perms.CanBan = true
		perms.CanPin = true
		perms.CanDelete = true
		perms.IsAdmin = true
		perms.IsModerator = true
	case CommunityRoleModerator:
		perms.CanPost = true
		perms.CanComment = true
		perms.CanVote = true
		perms.CanModerate = true
		perms.CanInvite = true
		perms.CanPin = true
		perms.CanDelete = true
		perms.IsModerator = true
	default:
		perms.CanPost = true
		perms.CanComment = true
		perms.CanVote = true
	}
	return perms, nil
}

// SetCustomRole mock implementation.
func (m *MockCommunityRepository) SetCustomRole(ctx context.Context, communityID, role string, permissions []string) error {
	if m.Error != nil {
		return m.Error
	}
	return nil
}

// GetTrendingCommunities mock implementation.
func (m *MockCommunityRepository) GetTrendingCommunities(ctx context.Context, limit int, since time.Time) ([]*entities.Community, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var communities []*entities.Community
	for _, c := range m.Communities {
		if c.DeletedAt == nil {
			communities = append(communities, c)
		}
	}
	return communities, nil
}

// GetSimilarCommunities mock implementation.
func (m *MockCommunityRepository) GetSimilarCommunities(ctx context.Context, communityID string, limit int) ([]*entities.Community, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Community{}, nil
}

// GetRecommendations mock implementation.
func (m *MockCommunityRepository) GetRecommendations(ctx context.Context, userID string, limit int) ([]*entities.Community, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*entities.Community{}, nil
}

// GetCommunitiesByTags mock implementation.
func (m *MockCommunityRepository) GetCommunitiesByTags(ctx context.Context, tags []string, pagination *CommunityPagination) ([]*entities.Community, int64, error) {
	if m.Error != nil {
		return nil, 0, m.Error
	}
	return []*entities.Community{}, 0, nil
}

// GetActivitySummary mock implementation.
func (m *MockCommunityRepository) GetActivitySummary(ctx context.Context, communityID string) (*CommunityActivitySummary, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &CommunityActivitySummary{
		NewMembers:    0,
		NewPosts:      0,
		ActiveMembers: 0,
		EngagementRate: 0.0,
		LastActivity:  time.Now(),
	}, nil
}

// BulkCreate mock implementation.
func (m *MockCommunityRepository) BulkCreate(ctx context.Context, communities []*entities.Community) error {
	if m.Error != nil {
		return m.Error
	}
	for _, c := range communities {
		_ = m.Create(ctx, c)
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockCommunityRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.HardDelete(ctx, id)
	}
	return nil
}

// BulkAddMembers mock implementation.
func (m *MockCommunityRepository) BulkAddMembers(ctx context.Context, communityID string, userIDs []string, role string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, uid := range userIDs {
		_ = m.AddMember(ctx, communityID, uid, role)
	}
	return nil
}

// BulkRemoveMembers mock implementation.
func (m *MockCommunityRepository) BulkRemoveMembers(ctx context.Context, communityID string, userIDs []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, uid := range userIDs {
		_ = m.RemoveMember(ctx, communityID, uid)
	}
	return nil
}

// BulkUpdateRoles mock implementation.
func (m *MockCommunityRepository) BulkUpdateRoles(ctx context.Context, communityID string, updates map[string]string) error {
	if m.Error != nil {
		return m.Error
	}
	for uid, role := range updates {
		_ = m.UpdateMemberRole(ctx, communityID, uid, role)
	}
	return nil
}

// BulkBanUsers mock implementation.
func (m *MockCommunityRepository) BulkBanUsers(ctx context.Context, communityID string, userIDs []string, reason string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, uid := range userIDs {
		_ = m.BanUser(ctx, communityID, uid, reason)
	}
	return nil
}

// BulkUnbanUsers mock implementation.
func (m *MockCommunityRepository) BulkUnbanUsers(ctx context.Context, communityID string, userIDs []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, uid := range userIDs {
		_ = m.UnbanUser(ctx, communityID, uid)
	}
	return nil
}

// BulkAddPosts mock implementation.
func (m *MockCommunityRepository) BulkAddPosts(ctx context.Context, communityID string, tweetIDs []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, tid := range tweetIDs {
		_ = m.AddPost(ctx, communityID, tid)
	}
	return nil
}

// BulkRemovePosts mock implementation.
func (m *MockCommunityRepository) BulkRemovePosts(ctx context.Context, communityID string, tweetIDs []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, tid := range tweetIDs {
		_ = m.RemovePost(ctx, communityID, tid)
	}
	return nil
}

// GetCommunityStats mock implementation.
func (m *MockCommunityRepository) GetCommunityStats(ctx context.Context) (*CommunityStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &CommunityStats{
		TotalCommunities: int64(len(m.Communities)),
	}, nil
}

// GetUserCommunityStats mock implementation.
func (m *MockCommunityRepository) GetUserCommunityStats(ctx context.Context, userID string) (*CommunityStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.GetCommunityStats(ctx)
}

// GetDailyCommunityStats mock implementation.
func (m *MockCommunityRepository) GetDailyCommunityStats(ctx context.Context, start, end time.Time) ([]*DailyCommunityCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyCommunityCount{}, nil
}

// GetCommunityGrowthRate mock implementation.
func (m *MockCommunityRepository) GetCommunityGrowthRate(ctx context.Context, communityID string, days int) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// GetTopCommunities mock implementation.
func (m *MockCommunityRepository) GetTopCommunities(ctx context.Context, sortBy string, limit int) ([]*entities.Community, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var communities []*entities.Community
	for _, c := range m.Communities {
		if c.DeletedAt == nil {
			communities = append(communities, c)
		}
	}
	return communities, nil
}

// WithTransaction mock implementation.
func (m *MockCommunityRepository) WithTransaction(ctx context.Context, tx *sql.Tx) CommunityRepository {
	return m
}

// Transaction mock implementation.
func (m *MockCommunityRepository) Transaction(ctx context.Context, fn func(txRepo CommunityRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockCommunityRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockCommunityRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockCommunityRepository) GetRawDB() interface{} {
	return nil
}