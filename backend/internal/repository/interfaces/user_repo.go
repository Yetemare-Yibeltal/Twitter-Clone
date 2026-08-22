// backend/internal/repository/interfaces/user_repo.go
package interfaces

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/domain/valueobjects"
)

// Common repository errors.
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrDuplicateUsername  = errors.New("username already taken")
	ErrDuplicateEmail     = errors.New("email already registered")
	ErrInvalidUserID      = errors.New("invalid user ID")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserSuspended      = errors.New("user is suspended")
	ErrUserDeactivated    = errors.New("user is deactivated")
)

// UserFilter defines filtering options for listing users.
type UserFilter struct {
	Username   *string
	Email      *string
	FullName   *string
	Bio        *string
	IsActive   *bool
	IsVerified *bool
	IsSuspended *bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Role       *string // "user", "admin", "moderator"
	Search     *string // full-text search across username, email, full_name, bio
}

// UserSortField represents sortable fields.
type UserSortField string

const (
	SortByCreatedAt UserSortField = "created_at"
	SortByUpdatedAt UserSortField = "updated_at"
	SortByUsername  UserSortField = "username"
	SortByFullName  UserSortField = "full_name"
	SortByEmail     UserSortField = "email"
	SortByTweetCount UserSortField = "tweet_count"
	SortByFollowerCount UserSortField = "follower_count"
)

// UserSortOrder represents sort order.
type UserSortOrder string

const (
	SortAsc  UserSortOrder = "ASC"
	SortDesc UserSortOrder = "DESC"
)

// PaginationOptions holds pagination and sorting.
type PaginationOptions struct {
	Limit  int
	Offset int
	SortBy UserSortField
	Order  UserSortOrder
}

// UserStats holds aggregated statistics for a user.
type UserStats struct {
	UserID         string
	TotalTweets    int64
	TotalFollowers int64
	TotalFollowing int64
	TotalLikes     int64
	TotalRetweets  int64
	TotalReplies   int64
	TotalBookmarks int64
	JoinedAt       time.Time
	LastActive     *time.Time
	IsVerified     bool
	IsSuspended    bool
}

// UserActivity represents a user's activity timeline.
type UserActivity struct {
	ID           string
	UserID       string
	ActivityType string // "tweet", "like", "retweet", "follow", "reply", "login"
	ReferenceID  *string // tweet ID or other reference
	CreatedAt    time.Time
	Metadata     map[string]interface{} // JSONB
}

// UserRepository defines the interface for user data persistence.
type UserRepository interface {
	// Create a new user. Returns the created entity or error.
	Create(ctx context.Context, user *entities.User) error

	// GetByID retrieves a user by primary key.
	GetByID(ctx context.Context, id string) (*entities.User, error)

	// GetByUsername retrieves a user by unique username.
	GetByUsername(ctx context.Context, username string) (*entities.User, error)

	// GetByEmail retrieves a user by unique email.
	GetByEmail(ctx context.Context, email string) (*entities.User, error)

	// GetByUsernameOrEmail retrieves a user by either username or email (case-insensitive).
	GetByUsernameOrEmail(ctx context.Context, identifier string) (*entities.User, error)

	// GetByIDs returns multiple users by their IDs (bulk read).
	GetByIDs(ctx context.Context, ids []string) ([]*entities.User, error)

	// Update updates an existing user (full or partial). Expects non-nil user.
	Update(ctx context.Context, user *entities.User) error

	// UpdateFields updates specific fields of a user (optimistic locking optional).
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error

	// Delete deletes a user (hard or soft delete depending on configuration).
	Delete(ctx context.Context, id string) error

	// SoftDelete marks a user as deleted without removing from DB.
	SoftDelete(ctx context.Context, id string) error

	// Restore restores a soft-deleted user.
	Restore(ctx context.Context, id string) error

	// List retrieves a paginated list of users with optional filtering and sorting.
	List(ctx context.Context, filter *UserFilter, pagination *PaginationOptions) ([]*entities.User, int64, error)

	// Count returns the total number of users matching the filter.
	Count(ctx context.Context, filter *UserFilter) (int64, error)

	// Search performs full‑text search across user fields (username, email, full_name, bio).
	Search(ctx context.Context, query string, pagination *PaginationOptions) ([]*entities.User, int64, error)

	// Exists checks if a user with the given ID exists (optionally skip soft-deleted).
	Exists(ctx context.Context, id string) (bool, error)

	// ExistsByUsername checks if a username is already taken.
	ExistsByUsername(ctx context.Context, username string) (bool, error)

	// ExistsByEmail checks if an email is already registered.
	ExistsByEmail(ctx context.Context, email string) (bool, error)

	// GetStats retrieves aggregated stats for a user.
	GetStats(ctx context.Context, userID string) (*UserStats, error)

	// GetStatsForUsers retrieves stats for multiple users (bulk).
	GetStatsForUsers(ctx context.Context, userIDs []string) (map[string]*UserStats, error)

	// RecordActivity logs a user activity.
	RecordActivity(ctx context.Context, activity *UserActivity) error

	// GetUserActivities retrieves recent activities for a user (paginated).
	GetUserActivities(ctx context.Context, userID string, limit, offset int) ([]*UserActivity, int64, error)

	// UpdateLastActive updates the user's last_active timestamp.
	UpdateLastActive(ctx context.Context, userID string) error

	// IncrementTweetCount increments the user's tweet counter (atomic).
	IncrementTweetCount(ctx context.Context, userID string) error

	// DecrementTweetCount decrements the tweet counter (atomic).
	DecrementTweetCount(ctx context.Context, userID string) error

	// IncrementFollowerCount increments follower count.
	IncrementFollowerCount(ctx context.Context, userID string) error

	// DecrementFollowerCount decrements follower count.
	DecrementFollowerCount(ctx context.Context, userID string) error

	// IncrementFollowingCount increments following count.
	IncrementFollowingCount(ctx context.Context, userID string) error

	// DecrementFollowingCount decrements following count.
	DecrementFollowingCount(ctx context.Context, userID string) error

	// UpdateVerificationStatus sets user's verified flag.
	UpdateVerificationStatus(ctx context.Context, userID string, verified bool) error

	// UpdateSuspensionStatus sets user's suspended flag.
	UpdateSuspensionStatus(ctx context.Context, userID string, suspended bool, reason string) error

	// BulkCreate inserts multiple users in a single transaction.
	BulkCreate(ctx context.Context, users []*entities.User) error

	// BulkUpdate updates multiple users in a single transaction.
	BulkUpdate(ctx context.Context, users []*entities.User) error

	// BulkDelete deletes multiple users (hard or soft) in a transaction.
	BulkDelete(ctx context.Context, ids []string) error

	// GetRecentlyJoined returns users who joined within the given duration.
	GetRecentlyJoined(ctx context.Context, duration time.Duration, limit int) ([]*entities.User, error)

	// GetActiveUsers returns users active within the given duration.
	GetActiveUsers(ctx context.Context, duration time.Duration, limit int) ([]*entities.User, error)

	// GetTopUsersByFollowers returns top users by follower count.
	GetTopUsersByFollowers(ctx context.Context, limit int) ([]*entities.User, error)

	// GetTopUsersByTweets returns top users by tweet count.
	GetTopUsersByTweets(ctx context.Context, limit int) ([]*entities.User, error)

	// GetUsersWithRole returns users with a specific role.
	GetUsersWithRole(ctx context.Context, role string, pagination *PaginationOptions) ([]*entities.User, int64, error)

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo UserRepository) error) error

	// WithTransaction returns a new repository instance using the given transaction.
	// This is used internally for transaction propagation.
	WithTransaction(ctx context.Context, tx *sql.Tx) UserRepository

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close releases any resources (pool connections).
	Close() error

	// GetRawDB returns the underlying *sql.DB or *sqlx.DB for advanced use (optional).
	GetRawDB() interface{}
}

// Optional extension interface for auditing.
type UserRepositoryAudit interface {
	UserRepository
	// GetAuditLog retrieves audit entries for a user.
	GetAuditLog(ctx context.Context, userID string, limit, offset int) ([]*AuditLogEntry, int64, error)
	// RecordAudit logs an audit entry.
	RecordAudit(ctx context.Context, entry *AuditLogEntry) error
}

// AuditLogEntry represents a user audit log.
type AuditLogEntry struct {
	ID         string
	UserID     string
	Action     string // "login", "logout", "update_profile", "change_password", "suspended", "verified"
	IP         string
	UserAgent  string
	Details    map[string]interface{} // JSON
	CreatedAt  time.Time
}

// Constants for user roles.
const (
	RoleUser     = "user"
	RoleModerator = "moderator"
	RoleAdmin    = "admin"
)

// Helper methods for UserFilter.
func (f *UserFilter) HasCriteria() bool {
	return f.Username != nil || f.Email != nil || f.FullName != nil || f.Bio != nil ||
		f.IsActive != nil || f.IsVerified != nil || f.IsSuspended != nil ||
		f.CreatedFrom != nil || f.CreatedTo != nil || f.Role != nil || f.Search != nil
}

// Validate checks if the filter is valid.
func (f *UserFilter) Validate() error {
	if f.CreatedFrom != nil && f.CreatedTo != nil && f.CreatedFrom.After(*f.CreatedTo) {
		return errors.New("CreatedFrom must be before CreatedTo")
	}
	if f.Role != nil {
		validRoles := map[string]bool{
			RoleUser: true, RoleModerator: true, RoleAdmin: true,
		}
		if !validRoles[*f.Role] {
			return errors.New("invalid role")
		}
	}
	return nil
}

// DefaultPagination returns sensible defaults.
func DefaultPagination() *PaginationOptions {
	return &PaginationOptions{
		Limit:  20,
		Offset: 0,
		SortBy: SortByCreatedAt,
		Order:  SortDesc,
	}
}

// NewUserActivity creates a new activity record helper.
func NewUserActivity(userID, activityType string, refID *string, meta map[string]interface{}) *UserActivity {
	return &UserActivity{
		UserID:       userID,
		ActivityType: activityType,
		ReferenceID:  refID,
		CreatedAt:    time.Now(),
		Metadata:     meta,
	}
}

// Activity types constants.
const (
	ActivityTweet   = "tweet"
	ActivityLike    = "like"
	ActivityRetweet = "retweet"
	ActivityFollow  = "follow"
	ActivityReply   = "reply"
	ActivityLogin   = "login"
	ActivitySignup  = "signup"
	ActivityLogout  = "logout"
)

// UserRepositoryFactory is a function type for creating a repository with a specific DB connection.
type UserRepositoryFactory func(db interface{}) UserRepository

// Convenience error helpers.
func IsUserNotFound(err error) bool {
	return errors.Is(err, ErrUserNotFound)
}

func IsUserAlreadyExists(err error) bool {
	return errors.Is(err, ErrUserAlreadyExists) ||
		errors.Is(err, ErrDuplicateUsername) ||
		errors.Is(err, ErrDuplicateEmail)
}

