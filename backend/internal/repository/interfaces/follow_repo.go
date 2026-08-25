// backend/internal/repository/interfaces/follow_repo.go
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
	ErrFollowNotFound      = errors.New("follow relationship not found")
	ErrAlreadyFollowing    = errors.New("already following this user")
	ErrCannotFollowSelf    = errors.New("cannot follow yourself")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidFollowID     = errors.New("invalid follow ID")
	ErrInvalidUserID       = errors.New("invalid user ID")
	ErrFollowBlocked       = errors.New("follow relationship is blocked")
	ErrFollowPending       = errors.New("follow request is pending")
	ErrFollowRequestExists = errors.New("follow request already exists")
)

// ======================================================================
// FollowStatus
// ======================================================================

// FollowStatus represents the status of a follow relationship.
type FollowStatus string

const (
	FollowStatusPending  FollowStatus = "pending"
	FollowStatusAccepted FollowStatus = "accepted"
	FollowStatusRejected FollowStatus = "rejected"
	FollowStatusBlocked  FollowStatus = "blocked"
)

// IsValid checks if the status is valid.
func (s FollowStatus) IsValid() bool {
	switch s {
	case FollowStatusPending, FollowStatusAccepted, FollowStatusRejected, FollowStatusBlocked:
		return true
	}
	return false
}

// ======================================================================
// FollowFilter
// ======================================================================

// FollowFilter defines filtering options for follow queries.
type FollowFilter struct {
	FollowerID   *string
	FolloweeID   *string
	Status       *FollowStatus
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	Mutual       *bool
}

// HasCriteria checks if any filter criteria are set.
func (f *FollowFilter) HasCriteria() bool {
	return f.FollowerID != nil || f.FolloweeID != nil || f.Status != nil ||
		f.CreatedFrom != nil || f.CreatedTo != nil || f.Mutual != nil
}

// ======================================================================
// FollowPagination
// ======================================================================

// FollowSortField defines sortable fields for follows.
type FollowSortField string

const (
	SortFollowByCreatedAt FollowSortField = "created_at"
	SortFollowByUpdatedAt FollowSortField = "updated_at"
	SortFollowByStatus    FollowSortField = "status"
)

// FollowSortOrder defines sort order.
type FollowSortOrder string

const (
	FollowSortAsc  FollowSortOrder = "ASC"
	FollowSortDesc FollowSortOrder = "DESC"
)

// FollowPagination holds pagination options for follows.
type FollowPagination struct {
	Cursor string           `json:"cursor"`
	Limit  int              `json:"limit"`
	SortBy FollowSortField  `json:"sort_by"`
	Order  FollowSortOrder  `json:"order"`
}

// DefaultFollowPagination returns default pagination options.
func DefaultFollowPagination() *FollowPagination {
	return &FollowPagination{
		Limit:  20,
		SortBy: SortFollowByCreatedAt,
		Order:  FollowSortDesc,
	}
}

// Validate checks pagination parameters.
func (p *FollowPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// FollowStats
// ======================================================================

// FollowStats represents aggregated follow statistics.
type FollowStats struct {
	TotalFollows    int64     `json:"total_follows"`
	UniqueFollowers int64     `json:"unique_followers"`
	UniqueFollowees int64     `json:"unique_followees"`
	PendingFollows  int64     `json:"pending_follows"`
	AcceptedFollows int64     `json:"accepted_follows"`
	RejectedFollows int64     `json:"rejected_follows"`
	BlockedFollows  int64     `json:"blocked_follows"`
	LastFollow      time.Time `json:"last_follow"`
	FirstFollow     time.Time `json:"first_follow"`
	AverageFollowers float64  `json:"average_followers"`
	MaxFollowers    int64     `json:"max_followers"`
	MinFollowers    int64     `json:"min_followers"`
}

// ======================================================================
// DailyFollowCount
// ======================================================================

// DailyFollowCount represents daily follow counts.
type DailyFollowCount struct {
	Date          time.Time `json:"date"`
	Total         int64     `json:"total"`
	NewFollowers  int64     `json:"new_followers"`
	NewFollowees  int64     `json:"new_followees"`
	PendingCount  int64     `json:"pending_count"`
	AcceptedCount int64     `json:"accepted_count"`
}

// ======================================================================
// FollowRecommendation
// ======================================================================

// FollowRecommendation represents a recommended user to follow.
type FollowRecommendation struct {
	UserID       string  `json:"user_id"`
	Username     string  `json:"username"`
	FullName     string  `json:"full_name"`
	AvatarURL    string  `json:"avatar_url"`
	MutualCount  int64   `json:"mutual_count"`
	FollowerCount int64  `json:"follower_count"`
	Score        float64 `json:"score"`
	Reason       string  `json:"reason"`
}

// ======================================================================
// FollowRepository Interface
// ======================================================================

// FollowRepository defines the interface for follow data persistence.
type FollowRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a follow relationship.
	Create(ctx context.Context, follow *Follow) error

	// GetByID retrieves a follow by its ID.
	GetByID(ctx context.Context, id string) (*Follow, error)

	// GetByFollowerAndFollowee retrieves a follow relationship.
	GetByFollowerAndFollowee(ctx context.Context, followerID, followeeID string) (*Follow, error)

	// Delete removes a follow relationship.
	Delete(ctx context.Context, followerID, followeeID string) error

	// DeleteByID removes a follow by ID.
	DeleteByID(ctx context.Context, id string) error

	// UpdateStatus updates the status of a follow relationship.
	UpdateStatus(ctx context.Context, id string, status FollowStatus) error

	// --------------------------------------------------------------------
	// Existence Checks
	// --------------------------------------------------------------------

	// Exists checks if a follow relationship exists.
	Exists(ctx context.Context, followerID, followeeID string) (bool, error)

	// ExistsWithStatus checks if a follow relationship exists with a specific status.
	ExistsWithStatus(ctx context.Context, followerID, followeeID string, status FollowStatus) (bool, error)

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountFollowers returns the number of followers for a user.
	CountFollowers(ctx context.Context, userID string) (int64, error)

	// CountFollowersWithStatus returns the number of followers with a specific status.
	CountFollowersWithStatus(ctx context.Context, userID string, status FollowStatus) (int64, error)

	// CountFollowing returns the number of users a user is following.
	CountFollowing(ctx context.Context, userID string) (int64, error)

	// CountFollowingWithStatus returns the number of following with a specific status.
	CountFollowingWithStatus(ctx context.Context, userID string, status FollowStatus) (int64, error)

	// CountMutual returns the number of mutual follows between two users.
	CountMutual(ctx context.Context, userID1, userID2 string) (int64, error)

	// CountPendingRequests returns the number of pending follow requests.
	CountPendingRequests(ctx context.Context, userID string) (int64, error)

	// CountPendingRequestsFromUser returns pending requests from a specific user.
	CountPendingRequestsFromUser(ctx context.Context, userID, fromUserID string) (int64, error)

	// --------------------------------------------------------------------
	// List Operations
	// --------------------------------------------------------------------

	// GetFollowers returns the list of followers for a user with pagination.
	GetFollowers(ctx context.Context, userID string, cursor string, limit int) ([]*Follow, string, error)

	// GetFollowersWithStatus returns followers with a specific status.
	GetFollowersWithStatus(ctx context.Context, userID string, status FollowStatus, cursor string, limit int) ([]*Follow, string, error)

	// GetFollowing returns the list of users a user is following with pagination.
	GetFollowing(ctx context.Context, userID string, cursor string, limit int) ([]*Follow, string, error)

	// GetFollowingWithStatus returns following with a specific status.
	GetFollowingWithStatus(ctx context.Context, userID string, status FollowStatus, cursor string, limit int) ([]*Follow, string, error)

	// GetPendingRequests returns pending follow requests for a user.
	GetPendingRequests(ctx context.Context, userID string, cursor string, limit int) ([]*Follow, string, error)

	// GetFollowerIDs returns all follower IDs for a user (no pagination).
	GetFollowerIDs(ctx context.Context, userID string) ([]string, error)

	// GetFollowingIDs returns all following IDs for a user (no pagination).
	GetFollowingIDs(ctx context.Context, userID string) ([]string, error)

	// --------------------------------------------------------------------
	// Mutual Follows
	// --------------------------------------------------------------------

	// AreMutual checks if two users follow each other.
	AreMutual(ctx context.Context, userID1, userID2 string) (bool, error)

	// GetMutualFollows returns the list of mutual follows between two users.
	GetMutualFollows(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error)

	// GetMutualFollowsDetailed returns detailed mutual follow information.
	GetMutualFollowsDetailed(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]*Follow, string, error)

	// GetMutualCountsForUsers returns mutual counts for multiple users.
	GetMutualCountsForUsers(ctx context.Context, userID string, targetUserIDs []string) (map[string]int64, error)

	// --------------------------------------------------------------------
	// Follow Recommendations
	// --------------------------------------------------------------------

	// GetFollowRecommendations returns suggested users to follow.
	GetFollowRecommendations(ctx context.Context, userID string, limit int) ([]string, error)

	// GetFollowRecommendationsWithScore returns recommendations with scores.
	GetFollowRecommendationsWithScore(ctx context.Context, userID string, limit int) ([]*FollowRecommendation, error)

	// GetPeopleAlsoFollow returns users also followed by followers.
	GetPeopleAlsoFollow(ctx context.Context, userID string, limit int) ([]string, error)

	// GetPopularUsers returns users with most followers (discovery).
	GetPopularUsers(ctx context.Context, limit int, excludeUserID string) ([]string, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple follows in a single transaction.
	BulkCreate(ctx context.Context, follows []*Follow) error

	// BulkDelete removes multiple follows in a single transaction.
	BulkDelete(ctx context.Context, followerIDs, followeeIDs []string) error

	// BulkDeleteByUserID removes all follows where the user is involved.
	BulkDeleteByUserID(ctx context.Context, userID string) error

	// BulkDeleteByFollowerID removes all follows by a specific follower.
	BulkDeleteByFollowerID(ctx context.Context, followerID string) error

	// BulkDeleteByFolloweeID removes all follows to a specific followee.
	BulkDeleteByFolloweeID(ctx context.Context, followeeID string) error

	// BulkUpdateStatus updates status for multiple follows.
	BulkUpdateStatus(ctx context.Context, ids []string, status FollowStatus) error

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetFollowStats returns aggregated follow statistics.
	GetFollowStats(ctx context.Context) (*FollowStats, error)

	// GetUserFollowStats returns follow statistics for a specific user.
	GetUserFollowStats(ctx context.Context, userID string) (*FollowStats, error)

	// GetDailyFollowStats returns daily follow counts for a date range.
	GetDailyFollowStats(ctx context.Context, start, end time.Time) ([]*DailyFollowCount, error)

	// GetFollowGrowthRate calculates follow growth rate over a period.
	GetFollowGrowthRate(ctx context.Context, userID string, days int) (float64, error)

	// GetTopFollowers returns users with the most followers (global).
	GetTopFollowers(ctx context.Context, limit int) ([]*User, error)

	// GetTopFollowees returns users followed by the most people.
	GetTopFollowees(ctx context.Context, limit int) ([]*User, error)

	// --------------------------------------------------------------------
	// Advanced Queries
	// --------------------------------------------------------------------

	// GetFollowingIntersection returns users followed by both users.
	GetFollowingIntersection(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error)

	// GetFollowerIntersection returns users who follow both users.
	GetFollowerIntersection(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error)

	// GetFollowPaths returns the follow path between two users (graph traversal).
	GetFollowPaths(ctx context.Context, userID1, userID2 string, maxDepth int) ([][]string, error)

	// GetFollowersByDateRange returns followers within a date range.
	GetFollowersByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*Follow, string, error)

	// GetFollowingByDateRange returns following within a date range.
	GetFollowingByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*Follow, string, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) FollowRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo FollowRepository) error) error

	// --------------------------------------------------------------------
	// Health and Cleanup
	// --------------------------------------------------------------------

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close releases any resources.
	Close() error

	// GetRawDB returns the underlying database connection.
	GetRawDB() interface{}

	// CleanupExpired removes expired or stale follow requests.
	CleanupExpired(ctx context.Context, before time.Time) (int64, error)
}

// ======================================================================
// Supporting Types
// ======================================================================

// Follow represents a follow relationship (used by repository).
type Follow struct {
	ID         string     `db:"id" json:"id"`
	FollowerID string     `db:"follower_id" json:"follower_id"`
	FolloweeID string     `db:"followee_id" json:"followee_id"`
	Status     string     `db:"status" json:"status"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// User is a minimal representation for follow queries.
type User struct {
	ID         string `db:"id" json:"id"`
	Username   string `db:"username" json:"username"`
	FullName   string `db:"full_name" json:"full_name"`
	AvatarURL  string `db:"avatar_url" json:"avatar_url"`
}

// ======================================================================
// Helper Functions
// ======================================================================

// IsFollowNotFound checks if an error indicates a follow was not found.
func IsFollowNotFound(err error) bool {
	return errors.Is(err, ErrFollowNotFound)
}

// IsFollowError checks if an error is follow-related.
func IsFollowError(err error) bool {
	return errors.Is(err, ErrFollowNotFound) ||
		errors.Is(err, ErrAlreadyFollowing) ||
		errors.Is(err, ErrCannotFollowSelf) ||
		errors.Is(err, ErrFollowBlocked) ||
		errors.Is(err, ErrFollowPending) ||
		errors.Is(err, ErrFollowRequestExists)
}

// ======================================================================
// Mock Follow Repository (for testing)
// ======================================================================

// MockFollowRepository is a mock implementation for testing.
type MockFollowRepository struct {
	Follows    map[string]*Follow
	UserFollows map[string]map[string]bool // userID -> followed userIDs
	Error      error
	NextCursor string
}

// NewMockFollowRepo creates a new mock repository.
func NewMockFollowRepo() FollowRepository {
	return &MockFollowRepository{
		Follows:     make(map[string]*Follow),
		UserFollows: make(map[string]map[string]bool),
	}
}

// Create mock implementation.
func (m *MockFollowRepository) Create(ctx context.Context, follow *Follow) error {
	if m.Error != nil {
		return m.Error
	}
	m.Follows[follow.ID] = follow
	if m.UserFollows[follow.FollowerID] == nil {
		m.UserFollows[follow.FollowerID] = make(map[string]bool)
	}
	m.UserFollows[follow.FollowerID][follow.FolloweeID] = true
	return nil
}

// GetByID mock implementation.
func (m *MockFollowRepository) GetByID(ctx context.Context, id string) (*Follow, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if follow, ok := m.Follows[id]; ok {
		return follow, nil
	}
	return nil, ErrFollowNotFound
}

// GetByFollowerAndFollowee mock implementation.
func (m *MockFollowRepository) GetByFollowerAndFollowee(ctx context.Context, followerID, followeeID string) (*Follow, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	for _, follow := range m.Follows {
		if follow.FollowerID == followerID && follow.FolloweeID == followeeID {
			return follow, nil
		}
	}
	return nil, ErrFollowNotFound
}

// Delete mock implementation.
func (m *MockFollowRepository) Delete(ctx context.Context, followerID, followeeID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, follow := range m.Follows {
		if follow.FollowerID == followerID && follow.FolloweeID == followeeID {
			delete(m.Follows, id)
			if m.UserFollows[followerID] != nil {
				delete(m.UserFollows[followerID], followeeID)
			}
			return nil
		}
	}
	return ErrFollowNotFound
}

// DeleteByID mock implementation.
func (m *MockFollowRepository) DeleteByID(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if follow, ok := m.Follows[id]; ok {
		delete(m.Follows, id)
		if m.UserFollows[follow.FollowerID] != nil {
			delete(m.UserFollows[follow.FollowerID], follow.FolloweeID)
		}
		return nil
	}
	return ErrFollowNotFound
}

// UpdateStatus mock implementation.
func (m *MockFollowRepository) UpdateStatus(ctx context.Context, id string, status FollowStatus) error {
	if m.Error != nil {
		return m.Error
	}
	if follow, ok := m.Follows[id]; ok {
		follow.Status = string(status)
		return nil
	}
	return ErrFollowNotFound
}

// Exists mock implementation.
func (m *MockFollowRepository) Exists(ctx context.Context, followerID, followeeID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if m.UserFollows[followerID] == nil {
		return false, nil
	}
	return m.UserFollows[followerID][followeeID], nil
}

// ExistsWithStatus mock implementation.
func (m *MockFollowRepository) ExistsWithStatus(ctx context.Context, followerID, followeeID string, status FollowStatus) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	for _, follow := range m.Follows {
		if follow.FollowerID == followerID && follow.FolloweeID == followeeID && follow.Status == string(status) {
			return true, nil
		}
	}
	return false, nil
}

// CountFollowers mock implementation.
func (m *MockFollowRepository) CountFollowers(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID && follow.Status == string(FollowStatusAccepted) {
			count++
		}
	}
	return count, nil
}

// CountFollowersWithStatus mock implementation.
func (m *MockFollowRepository) CountFollowersWithStatus(ctx context.Context, userID string, status FollowStatus) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID && follow.Status == string(status) {
			count++
		}
	}
	return count, nil
}

// CountFollowing mock implementation.
func (m *MockFollowRepository) CountFollowing(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, follow := range m.Follows {
		if follow.FollowerID == userID && follow.Status == string(FollowStatusAccepted) {
			count++
		}
	}
	return count, nil
}

// CountFollowingWithStatus mock implementation.
func (m *MockFollowRepository) CountFollowingWithStatus(ctx context.Context, userID string, status FollowStatus) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, follow := range m.Follows {
		if follow.FollowerID == userID && follow.Status == string(status) {
			count++
		}
	}
	return count, nil
}

// CountMutual mock implementation.
func (m *MockFollowRepository) CountMutual(ctx context.Context, userID1, userID2 string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	// Get following of user1
	following1 := make(map[string]bool)
	for _, follow := range m.Follows {
		if follow.FollowerID == userID1 && follow.Status == string(FollowStatusAccepted) {
			following1[follow.FolloweeID] = true
		}
	}
	// Count those that user2 also follows
	count := int64(0)
	for _, follow := range m.Follows {
		if follow.FollowerID == userID2 && follow.Status == string(FollowStatusAccepted) && following1[follow.FolloweeID] {
			count++
		}
	}
	return count, nil
}

// CountPendingRequests mock implementation.
func (m *MockFollowRepository) CountPendingRequests(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID && follow.Status == string(FollowStatusPending) {
			count++
		}
	}
	return count, nil
}

// CountPendingRequestsFromUser mock implementation.
func (m *MockFollowRepository) CountPendingRequestsFromUser(ctx context.Context, userID, fromUserID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID && follow.FollowerID == fromUserID && follow.Status == string(FollowStatusPending) {
			count++
		}
	}
	return count, nil
}

// GetFollowers mock implementation.
func (m *MockFollowRepository) GetFollowers(ctx context.Context, userID string, cursor string, limit int) ([]*Follow, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var follows []*Follow
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID {
			follows = append(follows, follow)
		}
	}
	return follows, "", nil
}

// GetFollowersWithStatus mock implementation.
func (m *MockFollowRepository) GetFollowersWithStatus(ctx context.Context, userID string, status FollowStatus, cursor string, limit int) ([]*Follow, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var follows []*Follow
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID && follow.Status == string(status) {
			follows = append(follows, follow)
		}
	}
	return follows, "", nil
}

// GetFollowing mock implementation.
func (m *MockFollowRepository) GetFollowing(ctx context.Context, userID string, cursor string, limit int) ([]*Follow, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var follows []*Follow
	for _, follow := range m.Follows {
		if follow.FollowerID == userID {
			follows = append(follows, follow)
		}
	}
	return follows, "", nil
}

// GetFollowingWithStatus mock implementation.
func (m *MockFollowRepository) GetFollowingWithStatus(ctx context.Context, userID string, status FollowStatus, cursor string, limit int) ([]*Follow, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var follows []*Follow
	for _, follow := range m.Follows {
		if follow.FollowerID == userID && follow.Status == string(status) {
			follows = append(follows, follow)
		}
	}
	return follows, "", nil
}

// GetPendingRequests mock implementation.
func (m *MockFollowRepository) GetPendingRequests(ctx context.Context, userID string, cursor string, limit int) ([]*Follow, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var follows []*Follow
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID && follow.Status == string(FollowStatusPending) {
			follows = append(follows, follow)
		}
	}
	return follows, "", nil
}

// GetFollowerIDs mock implementation.
func (m *MockFollowRepository) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var ids []string
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID {
			ids = append(ids, follow.FollowerID)
		}
	}
	return ids, nil
}

// GetFollowingIDs mock implementation.
func (m *MockFollowRepository) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var ids []string
	for _, follow := range m.Follows {
		if follow.FollowerID == userID {
			ids = append(ids, follow.FolloweeID)
		}
	}
	return ids, nil
}

// AreMutual mock implementation.
func (m *MockFollowRepository) AreMutual(ctx context.Context, userID1, userID2 string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	exists1, _ := m.Exists(ctx, userID1, userID2)
	exists2, _ := m.Exists(ctx, userID2, userID1)
	return exists1 && exists2, nil
}

// GetMutualFollows mock implementation.
func (m *MockFollowRepository) GetMutualFollows(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var mutual []string
	// Get following of user1
	following1 := make(map[string]bool)
	for _, follow := range m.Follows {
		if follow.FollowerID == userID1 && follow.Status == string(FollowStatusAccepted) {
			following1[follow.FolloweeID] = true
		}
	}
	// Check user2's following
	for _, follow := range m.Follows {
		if follow.FollowerID == userID2 && follow.Status == string(FollowStatusAccepted) && following1[follow.FolloweeID] {
			mutual = append(mutual, follow.FolloweeID)
		}
	}
	return mutual, "", nil
}

// GetMutualFollowsDetailed mock implementation.
func (m *MockFollowRepository) GetMutualFollowsDetailed(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]*Follow, string, error) {
	ids, _, err := m.GetMutualFollows(ctx, userID1, userID2, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	var follows []*Follow
	for _, id := range ids {
		for _, follow := range m.Follows {
			if follow.FolloweeID == id {
				follows = append(follows, follow)
				break
			}
		}
	}
	return follows, "", nil
}

// GetMutualCountsForUsers mock implementation.
func (m *MockFollowRepository) GetMutualCountsForUsers(ctx context.Context, userID string, targetUserIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, targetID := range targetUserIDs {
		count, _ := m.CountMutual(ctx, userID, targetID)
		result[targetID] = count
	}
	return result, nil
}

// GetFollowRecommendations mock implementation.
func (m *MockFollowRepository) GetFollowRecommendations(ctx context.Context, userID string, limit int) ([]string, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	// Get users that followers of user are following
	recommended := []string{}
	// Get followers of user
	var followers []string
	for _, follow := range m.Follows {
		if follow.FolloweeID == userID && follow.Status == string(FollowStatusAccepted) {
			followers = append(followers, follow.FollowerID)
		}
	}
	// Get who they follow
	seen := make(map[string]bool)
	seen[userID] = true
	for _, fid := range followers {
		for _, follow := range m.Follows {
			if follow.FollowerID == fid && follow.Status == string(FollowStatusAccepted) && !seen[follow.FolloweeID] {
				seen[follow.FolloweeID] = true
				recommended = append(recommended, follow.FolloweeID)
			}
		}
	}
	if len(recommended) > limit {
		recommended = recommended[:limit]
	}
	return recommended, nil
}

// GetFollowRecommendationsWithScore mock implementation.
func (m *MockFollowRepository) GetFollowRecommendationsWithScore(ctx context.Context, userID string, limit int) ([]*FollowRecommendation, error) {
	ids, err := m.GetFollowRecommendations(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	var recs []*FollowRecommendation
	for _, id := range ids {
		mutual, _ := m.CountMutual(ctx, userID, id)
		followerCount, _ := m.CountFollowers(ctx, id)
		recs = append(recs, &FollowRecommendation{
			UserID:       id,
			MutualCount:  mutual,
			FollowerCount: followerCount,
			Score:        float64(mutual)*2 + float64(followerCount)*0.5,
		})
	}
	return recs, nil
}

// GetPeopleAlsoFollow mock implementation.
func (m *MockFollowRepository) GetPeopleAlsoFollow(ctx context.Context, userID string, limit int) ([]string, error) {
	return m.GetFollowRecommendations(ctx, userID, limit)
}

// GetPopularUsers mock implementation.
func (m *MockFollowRepository) GetPopularUsers(ctx context.Context, limit int, excludeUserID string) ([]string, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	// Count followers for each user
	userCounts := make(map[string]int64)
	for _, follow := range m.Follows {
		if follow.Status == string(FollowStatusAccepted) {
			userCounts[follow.FolloweeID]++
		}
	}
	// Sort by count
	users := []string{}
	for uid := range userCounts {
		if uid != excludeUserID {
			users = append(users, uid)
		}
	}
	// Simple sort (bubble)
	for i := 0; i < len(users); i++ {
		for j := i + 1; j < len(users); j++ {
			if userCounts[users[j]] > userCounts[users[i]] {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
	if len(users) > limit {
		users = users[:limit]
	}
	return users, nil
}

// BulkCreate mock implementation.
func (m *MockFollowRepository) BulkCreate(ctx context.Context, follows []*Follow) error {
	if m.Error != nil {
		return m.Error
	}
	for _, follow := range follows {
		_ = m.Create(ctx, follow)
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockFollowRepository) BulkDelete(ctx context.Context, followerIDs, followeeIDs []string) error {
	if m.Error != nil {
		return m.Error
	}
	for i := range followerIDs {
		_ = m.Delete(ctx, followerIDs[i], followeeIDs[i])
	}
	return nil
}

// BulkDeleteByUserID mock implementation.
func (m *MockFollowRepository) BulkDeleteByUserID(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, follow := range m.Follows {
		if follow.FollowerID == userID || follow.FolloweeID == userID {
			delete(m.Follows, id)
			if m.UserFollows[follow.FollowerID] != nil {
				delete(m.UserFollows[follow.FollowerID], follow.FolloweeID)
			}
		}
	}
	return nil
}

// BulkDeleteByFollowerID mock implementation.
func (m *MockFollowRepository) BulkDeleteByFollowerID(ctx context.Context, followerID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, follow := range m.Follows {
		if follow.FollowerID == followerID {
			delete(m.Follows, id)
			if m.UserFollows[followerID] != nil {
				delete(m.UserFollows[followerID], follow.FolloweeID)
			}
		}
	}
	return nil
}

// BulkDeleteByFolloweeID mock implementation.
func (m *MockFollowRepository) BulkDeleteByFolloweeID(ctx context.Context, followeeID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, follow := range m.Follows {
		if follow.FolloweeID == followeeID {
			delete(m.Follows, id)
			if m.UserFollows[follow.FollowerID] != nil {
				delete(m.UserFollows[follow.FollowerID], followeeID)
			}
		}
	}
	return nil
}

// BulkUpdateStatus mock implementation.
func (m *MockFollowRepository) BulkUpdateStatus(ctx context.Context, ids []string, status FollowStatus) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.UpdateStatus(ctx, id, status)
	}
	return nil
}

// GetFollowStats mock implementation.
func (m *MockFollowRepository) GetFollowStats(ctx context.Context) (*FollowStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	stats := &FollowStats{
		TotalFollows: int64(len(m.Follows)),
	}
	uniqueFollowers := make(map[string]bool)
	uniqueFollowees := make(map[string]bool)
	for _, f := range m.Follows {
		if f.Status == string(FollowStatusAccepted) {
			uniqueFollowers[f.FollowerID] = true
			uniqueFollowees[f.FolloweeID] = true
		}
	}
	stats.UniqueFollowers = int64(len(uniqueFollowers))
	stats.UniqueFollowees = int64(len(uniqueFollowees))
	return stats, nil
}

// GetUserFollowStats mock implementation.
func (m *MockFollowRepository) GetUserFollowStats(ctx context.Context, userID string) (*FollowStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	followers, _ := m.CountFollowers(ctx, userID)
	following, _ := m.CountFollowing(ctx, userID)
	return &FollowStats{
		TotalFollows: followers + following,
	}, nil
}

// GetDailyFollowStats mock implementation.
func (m *MockFollowRepository) GetDailyFollowStats(ctx context.Context, start, end time.Time) ([]*DailyFollowCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyFollowCount{}, nil
}

// GetFollowGrowthRate mock implementation.
func (m *MockFollowRepository) GetFollowGrowthRate(ctx context.Context, userID string, days int) (float64, error) {
	return 0.0, nil
}

// GetTopFollowers mock implementation.
func (m *MockFollowRepository) GetTopFollowers(ctx context.Context, limit int) ([]*User, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*User{}, nil
}

// GetTopFollowees mock implementation.
func (m *MockFollowRepository) GetTopFollowees(ctx context.Context, limit int) ([]*User, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*User{}, nil
}

// GetFollowingIntersection mock implementation.
func (m *MockFollowRepository) GetFollowingIntersection(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	following1, _ := m.GetFollowingIDs(ctx, userID1)
	following2, _ := m.GetFollowingIDs(ctx, userID2)
	set := make(map[string]bool)
	for _, id := range following1 {
		set[id] = true
	}
	var intersection []string
	for _, id := range following2 {
		if set[id] {
			intersection = append(intersection, id)
		}
	}
	return intersection, "", nil
}

// GetFollowerIntersection mock implementation.
func (m *MockFollowRepository) GetFollowerIntersection(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	followers1, _ := m.GetFollowerIDs(ctx, userID1)
	followers2, _ := m.GetFollowerIDs(ctx, userID2)
	set := make(map[string]bool)
	for _, id := range followers1 {
		set[id] = true
	}
	var intersection []string
	for _, id := range followers2 {
		if set[id] {
			intersection = append(intersection, id)
		}
	}
	return intersection, "", nil
}

// GetFollowPaths mock implementation.
func (m *MockFollowRepository) GetFollowPaths(ctx context.Context, userID1, userID2 string, maxDepth int) ([][]string, error) {
	return [][]string{}, nil
}

// GetFollowersByDateRange mock implementation.
func (m *MockFollowRepository) GetFollowersByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*Follow, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var follows []*Follow
	for _, f := range m.Follows {
		if f.FolloweeID == userID && f.CreatedAt.After(start) && f.CreatedAt.Before(end) {
			follows = append(follows, f)
		}
	}
	return follows, "", nil
}

// GetFollowingByDateRange mock implementation.
func (m *MockFollowRepository) GetFollowingByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*Follow, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var follows []*Follow
	for _, f := range m.Follows {
		if f.FollowerID == userID && f.CreatedAt.After(start) && f.CreatedAt.Before(end) {
			follows = append(follows, f)
		}
	}
	return follows, "", nil
}

// WithTransaction mock implementation.
func (m *MockFollowRepository) WithTransaction(ctx context.Context, tx *sql.Tx) FollowRepository {
	return m
}

// Transaction mock implementation.
func (m *MockFollowRepository) Transaction(ctx context.Context, fn func(txRepo FollowRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockFollowRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockFollowRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockFollowRepository) GetRawDB() interface{} {
	return nil
}

// CleanupExpired mock implementation.
func (m *MockFollowRepository) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for id, f := range m.Follows {
		if f.CreatedAt.Before(before) && f.Status == string(FollowStatusPending) {
			delete(m.Follows, id)
			if m.UserFollows[f.FollowerID] != nil {
				delete(m.UserFollows[f.FollowerID], f.FolloweeID)
			}
			count++
		}
	}
	return count, nil
}