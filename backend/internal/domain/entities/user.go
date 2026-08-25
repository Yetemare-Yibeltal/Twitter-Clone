// backend/internal/domain/entities/user.go
package entities

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// User statuses
	UserStatusActive    = "active"
	UserStatusInactive  = "inactive"
	UserStatusSuspended = "suspended"
	UserStatusDeleted   = "deleted"
	
	// User roles
	UserRoleUser      = "user"
	UserRoleModerator = "moderator"
	UserRoleAdmin     = "admin"
	
	// Validation limits
	MinUsernameLength = 3
	MaxUsernameLength = 20
	MinPasswordLength = 8
	MaxPasswordLength = 72
	MaxFullNameLength = 100
	MaxBioLength      = 160
	MaxEmailLength    = 254
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrUserIDEmpty           = errors.New("user ID cannot be empty")
	ErrUsernameEmpty         = errors.New("username cannot be empty")
	ErrUsernameTooShort      = fmt.Errorf("username must be at least %d characters", MinUsernameLength)
	ErrUsernameTooLong       = fmt.Errorf("username must be at most %d characters", MaxUsernameLength)
	ErrUsernameInvalid       = errors.New("username contains invalid characters")
	ErrUsernameReserved      = errors.New("username is reserved")
	ErrEmailEmpty            = errors.New("email cannot be empty")
	ErrEmailInvalid          = errors.New("invalid email format")
	ErrEmailTooLong          = fmt.Errorf("email exceeds maximum length of %d characters", MaxEmailLength)
	ErrPasswordEmpty         = errors.New("password cannot be empty")
	ErrPasswordTooShort      = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	ErrPasswordTooLong       = fmt.Errorf("password must be at most %d characters", MaxPasswordLength)
	ErrPasswordWeak          = errors.New("password is too weak: must contain uppercase, lowercase, number, and special character")
	ErrFullNameEmpty         = errors.New("full name cannot be empty")
	ErrFullNameTooLong       = fmt.Errorf("full name exceeds maximum length of %d characters", MaxFullNameLength)
	ErrBioTooLong            = fmt.Errorf("bio exceeds maximum length of %d characters", MaxBioLength)
	ErrInvalidRole           = errors.New("invalid user role")
	ErrInvalidStatus         = errors.New("invalid user status")
	ErrUserAlreadyDeleted    = errors.New("user already deleted")
	ErrUserNotDeleted        = errors.New("user is not deleted")
	ErrUserSuspended         = errors.New("user is suspended")
	ErrUserInactive          = errors.New("user is inactive")
	ErrUserAlreadyActive     = errors.New("user is already active")
	ErrUserAlreadySuspended  = errors.New("user is already suspended")
)

// ======================================================================
// User Entity
// ======================================================================

// User represents a user in the system.
type User struct {
	ID              string     `db:"id" json:"id"`
	Username        string     `db:"username" json:"username"`
	Email           string     `db:"email" json:"email"`
	PasswordHash    string     `db:"password_hash" json:"-"`
	FullName        string     `db:"full_name" json:"full_name"`
	Bio             string     `db:"bio" json:"bio,omitempty"`
	AvatarURL       string     `db:"avatar_url" json:"avatar_url,omitempty"`
	BannerURL       string     `db:"banner_url" json:"banner_url,omitempty"`
	Location        string     `db:"location" json:"location,omitempty"`
	Website         string     `db:"website" json:"website,omitempty"`
	Role            string     `db:"role" json:"role"`
	Status          string     `db:"status" json:"status"`
	IsVerified      bool       `db:"is_verified" json:"is_verified"`
	IsPrivate       bool       `db:"is_private" json:"is_private"`
	TwitterHandle   string     `db:"twitter_handle" json:"twitter_handle,omitempty"`
	InstagramHandle string     `db:"instagram_handle" json:"instagram_handle,omitempty"`
	JoinedAt        time.Time  `db:"joined_at" json:"joined_at"`
	LastActiveAt    *time.Time `db:"last_active_at" json:"last_active_at,omitempty"`
	DeletedAt       *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
	Metadata        UserMetadata `db:"metadata" json:"metadata,omitempty"`
}

// UserMetadata holds optional user metadata.
type UserMetadata struct {
	Theme        string            `json:"theme,omitempty"`
	Language     string            `json:"language,omitempty"`
	Timezone     string            `json:"timezone,omitempty"`
	Notifications map[string]bool  `json:"notifications,omitempty"`
	Privacy      map[string]bool   `json:"privacy,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// Value implements driver.Valuer for JSON storage.
func (m UserMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements sql.Scanner for JSON retrieval.
func (m *UserMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = UserMetadata{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for UserMetadata: %T", value)
	}
	return json.Unmarshal(bytes, m)
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewUser creates a new user with default values.
func NewUser(username, email, password, fullName string) (*User, error) {
	u := &User{
		ID:         uuid.New().String(),
		Username:   username,
		Email:      email,
		FullName:   fullName,
		Role:       UserRoleUser,
		Status:     UserStatusActive,
		IsVerified: false,
		IsPrivate:  false,
		JoinedAt:   time.Now(),
		Metadata:   UserMetadata{},
	}
	if err := u.SetPassword(password); err != nil {
		return nil, err
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}
	return u, nil
}

// MustNewUser creates a new user and panics on error.
func MustNewUser(username, email, password, fullName string) *User {
	u, err := NewUser(username, email, password, fullName)
	if err != nil {
		panic(err)
	}
	return u
}

// ======================================================================
// Validation
// ======================================================================

// Validate performs comprehensive validation.
func (u *User) Validate() error {
	if strings.TrimSpace(u.ID) == "" {
		return ErrUserIDEmpty
	}
	// Username validation
	usernameTrimmed := strings.TrimSpace(u.Username)
	if usernameTrimmed == "" {
		return ErrUsernameEmpty
	}
	if len(usernameTrimmed) < MinUsernameLength {
		return ErrUsernameTooShort
	}
	if len(usernameTrimmed) > MaxUsernameLength {
		return ErrUsernameTooLong
	}
	if !isValidUsername(usernameTrimmed) {
		return ErrUsernameInvalid
	}
	if isReservedUsername(usernameTrimmed) {
		return ErrUsernameReserved
	}
	u.Username = strings.ToLower(usernameTrimmed)
	// Email validation
	emailTrimmed := strings.TrimSpace(u.Email)
	if emailTrimmed == "" {
		return ErrEmailEmpty
	}
	if len(emailTrimmed) > MaxEmailLength {
		return ErrEmailTooLong
	}
	if !isValidEmail(emailTrimmed) {
		return ErrEmailInvalid
	}
	u.Email = strings.ToLower(emailTrimmed)
	// Full name validation
	fullNameTrimmed := strings.TrimSpace(u.FullName)
	if fullNameTrimmed == "" {
		return ErrFullNameEmpty
	}
	if len(fullNameTrimmed) > MaxFullNameLength {
		return ErrFullNameTooLong
	}
	u.FullName = fullNameTrimmed
	// Bio validation
	if len(u.Bio) > MaxBioLength {
		return ErrBioTooLong
	}
	u.Bio = strings.TrimSpace(u.Bio)
	// Role validation
	if !isValidRole(u.Role) {
		return ErrInvalidRole
	}
	// Status validation
	if !isValidStatus(u.Status) {
		return ErrInvalidStatus
	}
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	return nil
}

// isValidUsername checks if a username contains only allowed characters.
func isValidUsername(username string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	return re.MatchString(username)
}

// isValidEmail checks if an email is valid.
func isValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// isValidRole checks if a role is valid.
func isValidRole(role string) bool {
	switch role {
	case UserRoleUser, UserRoleModerator, UserRoleAdmin:
		return true
	}
	return false
}

// isValidStatus checks if a status is valid.
func isValidStatus(status string) bool {
	switch status {
	case UserStatusActive, UserStatusInactive, UserStatusSuspended, UserStatusDeleted:
		return true
	}
	return false
}

// isReservedUsername checks if a username is reserved.
func isReservedUsername(username string) bool {
	reserved := map[string]bool{
		"admin": true, "administrator": true, "root": true,
		"system": true, "support": true, "help": true,
		"info": true, "noreply": true, "postmaster": true,
		"webmaster": true, "hostmaster": true, "abuse": true,
		"security": true, "privacy": true, "moderator": true,
		"mod": true, "owner": true, "manager": true,
		"user": true, "users": true, "guest": true,
		"test": true, "testing": true, "demo": true,
		"example": true, "sample": true, "anonymous": true,
		"default": true, "null": true, "undefined": true,
	}
	return reserved[strings.ToLower(username)]
}

// ======================================================================
// Password Management
// ======================================================================

// SetPassword hashes and sets the user's password.
func (u *User) SetPassword(password string) error {
	if password == "" {
		return ErrPasswordEmpty
	}
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	if !isStrongPassword(password) {
		return ErrPasswordWeak
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword verifies the provided password against the stored hash.
func (u *User) CheckPassword(password string) bool {
	if u.PasswordHash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// NeedsRehash checks if the password hash needs rehashing (e.g., cost changed).
func (u *User) NeedsRehash() bool {
	cost, err := bcrypt.Cost([]byte(u.PasswordHash))
	if err != nil {
		return true
	}
	return cost < bcrypt.DefaultCost
}

// isStrongPassword checks if a password meets complexity requirements.
func isStrongPassword(password string) bool {
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`\d`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(password)
	// At least 3 of 4 character classes
	classes := 0
	if hasUpper {
		classes++
	}
	if hasLower {
		classes++
	}
	if hasDigit {
		classes++
	}
	if hasSpecial {
		classes++
	}
	return classes >= 3
}

// ======================================================================
// Profile Management
// ======================================================================

// UpdateProfile updates the user's profile fields.
func (u *User) UpdateProfile(fullName, bio, location, website string) error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.Status == UserStatusSuspended {
		return ErrUserSuspended
	}
	u.FullName = strings.TrimSpace(fullName)
	if u.FullName == "" {
		return ErrFullNameEmpty
	}
	if len(u.FullName) > MaxFullNameLength {
		return ErrFullNameTooLong
	}
	u.Bio = strings.TrimSpace(bio)
	if len(u.Bio) > MaxBioLength {
		return ErrBioTooLong
	}
	u.Location = strings.TrimSpace(location)
	u.Website = strings.TrimSpace(website)
	return nil
}

// UpdateAvatar updates the user's avatar URL.
func (u *User) UpdateAvatar(avatarURL string) error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.Status == UserStatusSuspended {
		return ErrUserSuspended
	}
	u.AvatarURL = strings.TrimSpace(avatarURL)
	return nil
}

// UpdateBanner updates the user's banner URL.
func (u *User) UpdateBanner(bannerURL string) error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.Status == UserStatusSuspended {
		return ErrUserSuspended
	}
	u.BannerURL = strings.TrimSpace(bannerURL)
	return nil
}

// UpdateMetadata updates user metadata.
func (u *User) UpdateMetadata(metadata UserMetadata) error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	u.Metadata = metadata
	return nil
}

// ======================================================================
= Status Management
// ======================================================================

// Activate activates the user.
func (u *User) Activate() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.Status == UserStatusActive {
		return ErrUserAlreadyActive
	}
	u.Status = UserStatusActive
	return nil
}

// Deactivate deactivates the user.
func (u *User) Deactivate() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.Status == UserStatusInactive {
		return errors.New("user is already inactive")
	}
	u.Status = UserStatusInactive
	return nil
}

// Suspend suspends the user.
func (u *User) Suspend() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.Status == UserStatusSuspended {
		return ErrUserAlreadySuspended
	}
	u.Status = UserStatusSuspended
	return nil
}

// Unsuspend unsuspends the user.
func (u *User) Unsuspend() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.Status != UserStatusSuspended {
		return errors.New("user is not suspended")
	}
	u.Status = UserStatusActive
	return nil
}

// IsActive checks if the user is active.
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive && u.DeletedAt == nil
}

// IsSuspended checks if the user is suspended.
func (u *User) IsSuspended() bool {
	return u.Status == UserStatusSuspended
}

// IsInactive checks if the user is inactive.
func (u *User) IsInactive() bool {
	return u.Status == UserStatusInactive
}

// ======================================================================
= Deletion Operations
// ======================================================================

// SoftDelete marks the user as deleted.
func (u *User) SoftDelete() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	now := time.Now()
	u.DeletedAt = &now
	u.Status = UserStatusDeleted
	return nil
}

// Restore restores a soft-deleted user.
func (u *User) Restore() error {
	if u.DeletedAt == nil {
		return ErrUserNotDeleted
	}
	u.DeletedAt = nil
	u.Status = UserStatusActive
	return nil
}

// IsDeleted checks if the user is deleted.
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

// ======================================================================
= Verification
// ======================================================================

// Verify marks the user as verified.
func (u *User) Verify() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.IsVerified {
		return errors.New("user is already verified")
	}
	u.IsVerified = true
	return nil
}

// Unverify removes verified status.
func (u *User) Unverify() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if !u.IsVerified {
		return errors.New("user is not verified")
	}
	u.IsVerified = false
	return nil
}

// ======================================================================
// Privacy
// ======================================================================

// SetPrivate makes the account private.
func (u *User) SetPrivate() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	u.IsPrivate = true
	return nil
}

// SetPublic makes the account public.
func (u *User) SetPublic() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	u.IsPrivate = false
	return nil
}

// ======================================================================
// Role Management
// ======================================================================

// SetRole sets the user's role.
func (u *User) SetRole(role string) error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if !isValidRole(role) {
		return ErrInvalidRole
	}
	u.Role = role
	return nil
}

// IsAdmin checks if the user is an admin.
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// IsModerator checks if the user is a moderator.
func (u *User) IsModerator() bool {
	return u.Role == UserRoleModerator || u.IsAdmin()
}

// IsRegularUser checks if the user is a regular user.
func (u *User) IsRegularUser() bool {
	return u.Role == UserRoleUser
}

// ======================================================================
// Activity
// ======================================================================

// UpdateLastActive updates the last active timestamp.
func (u *User) UpdateLastActive() {
	now := time.Now()
	u.LastActiveAt = &now
}

// DaysSinceJoin returns the number of days since the user joined.
func (u *User) DaysSinceJoin() int {
	return int(time.Since(u.JoinedAt).Hours() / 24)
}

// ======================================================================
// Helper Methods
// ======================================================================

// String returns a human-readable representation.
func (u *User) String() string {
	return fmt.Sprintf("User{ID:%s, username:%s, email:%s, role:%s, status:%s, joined:%v}",
		u.ID, u.Username, u.Email, u.Role, u.Status, u.JoinedAt)
}

// Clone returns a deep copy of the user.
func (u *User) Clone() *User {
	clone := *u
	if u.LastActiveAt != nil {
		t := *u.LastActiveAt
		clone.LastActiveAt = &t
	}
	if u.DeletedAt != nil {
		t := *u.DeletedAt
		clone.DeletedAt = &t
	}
	return &clone
}

// Equals checks if two users are the same by ID.
func (u *User) Equals(other *User) bool {
	return u.ID == other.ID
}

// IsEmpty returns true if the user is zero value.
func (u *User) IsEmpty() bool {
	return u.ID == "" && u.Username == "" && u.Email == ""
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (u User) Value() (driver.Value, error) {
	return json.Marshal(u)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (u *User) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for User: %T", value)
	}
	return json.Unmarshal(bytes, u)
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (u *User) MarshalJSON() ([]byte, error) {
	type Alias User
	return json.Marshal(&struct {
		*Alias
		Status     string `json:"status"`
		IsActive   bool   `json:"is_active"`
		IsAdmin    bool   `json:"is_admin"`
		IsModerator bool  `json:"is_moderator"`
		DaysJoined int    `json:"days_joined"`
	}{
		Alias:       (*Alias)(u),
		Status:      u.Status,
		IsActive:    u.IsActive(),
		IsAdmin:     u.IsAdmin(),
		IsModerator: u.IsModerator(),
		DaysJoined:  u.DaysSinceJoin(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (u *User) UnmarshalJSON(data []byte) error {
	type Alias User
	aux := &struct {
		*Alias
		Status string `json:"status"`
	}{
		Alias: (*Alias)(u),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Status != "" {
		u.Status = aux.Status
	}
	return nil
}

// ======================================================================
= User Collection
// ======================================================================

// UserCollection provides advanced operations on a slice of users.
type UserCollection []*User

// Len returns the number of users.
func (c UserCollection) Len() int { return len(c) }

// Filter returns a new collection with users matching the predicate.
func (c UserCollection) Filter(predicate func(*User) bool) UserCollection {
	result := UserCollection{}
	for _, u := range c {
		if predicate(u) {
			result = append(result, u)
		}
	}
	return result
}

// SortByJoinedAt sorts users by join date (newest first).
func (c UserCollection) SortByJoinedAt() {
	sort.Slice(c, func(i, j int) bool {
		return c[i].JoinedAt.After(c[j].JoinedAt)
	})
}

// SortByUsername sorts users by username.
func (c UserCollection) SortByUsername() {
	sort.Slice(c, func(i, j int) bool {
		return c[i].Username < c[j].Username
	})
}

// GetActive returns active users.
func (c UserCollection) GetActive() UserCollection {
	return c.Filter(func(u *User) bool {
		return u.IsActive()
	})
}

// GetAdmins returns admin users.
func (c UserCollection) GetAdmins() UserCollection {
	return c.Filter(func(u *User) bool {
		return u.IsAdmin()
	})
}

// GetByRole returns users with a specific role.
func (c UserCollection) GetByRole(role string) UserCollection {
	return c.Filter(func(u *User) bool {
		return u.Role == role
	})
}

// ======================================================================
= User Statistics
// ======================================================================

// UserStats represents user statistics.
type UserStats struct {
	TotalUsers     int64 `json:"total_users"`
	ActiveUsers    int64 `json:"active_users"`
	SuspendedUsers int64 `json:"suspended_users"`
	DeletedUsers   int64 `json:"deleted_users"`
	Admins         int64 `json:"admins"`
	Moderators     int64 `json:"moderators"`
	VerifiedUsers  int64 `json:"verified_users"`
}

// CalculateStats calculates statistics from a user collection.
func (c UserCollection) CalculateStats() *UserStats {
	stats := &UserStats{
		TotalUsers: int64(len(c)),
	}
	for _, u := range c {
		if u.IsActive() {
			stats.ActiveUsers++
		}
		if u.IsSuspended() {
			stats.SuspendedUsers++
		}
		if u.IsDeleted() {
			stats.DeletedUsers++
		}
		if u.IsAdmin() {
			stats.Admins++
		}
		if u.IsModerator() {
			stats.Moderators++
		}
		if u.IsVerified {
			stats.VerifiedUsers++
		}
	}
	return stats
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// UserBuilder helps construct users for testing.
type UserBuilder struct {
	user *User
}

// NewUserBuilder creates a new user builder.
func NewUserBuilder() *UserBuilder {
	return &UserBuilder{
		user: &User{
			ID:         uuid.New().String(),
			Username:   "",
			Email:      "",
			PasswordHash: "",
			FullName:   "",
			Bio:        "",
			Role:       UserRoleUser,
			Status:     UserStatusActive,
			IsVerified: false,
			IsPrivate:  false,
			JoinedAt:   time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *UserBuilder) WithID(id string) *UserBuilder {
	b.user.ID = id
	return b
}

// WithUsername sets the username.
func (b *UserBuilder) WithUsername(username string) *UserBuilder {
	b.user.Username = username
	return b
}

// WithEmail sets the email.
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.user.Email = email
	return b
}

// WithFullName sets the full name.
func (b *UserBuilder) WithFullName(fullName string) *UserBuilder {
	b.user.FullName = fullName
	return b
}

// WithBio sets the bio.
func (b *UserBuilder) WithBio(bio string) *UserBuilder {
	b.user.Bio = bio
	return b
}

// WithAvatarURL sets the avatar URL.
func (b *UserBuilder) WithAvatarURL(url string) *UserBuilder {
	b.user.AvatarURL = url
	return b
}

// WithRole sets the role.
func (b *UserBuilder) WithRole(role string) *UserBuilder {
	b.user.Role = role
	return b
}

// WithStatus sets the status.
func (b *UserBuilder) WithStatus(status string) *UserBuilder {
	b.user.Status = status
	return b
}

// WithPassword sets the password hash.
func (b *UserBuilder) WithPassword(password string) *UserBuilder {
	_ = b.user.SetPassword(password)
	return b
}

// WithVerified sets verified status.
func (b *UserBuilder) WithVerified(verified bool) *UserBuilder {
	b.user.IsVerified = verified
	return b
}

// WithPrivate sets private status.
func (b *UserBuilder) WithPrivate(private bool) *UserBuilder {
	b.user.IsPrivate = private
	return b
}

// WithJoinedAt sets the join date.
func (b *UserBuilder) WithJoinedAt(t time.Time) *UserBuilder {
	b.user.JoinedAt = t
	return b
}

// WithLastActive sets the last active timestamp.
func (b *UserBuilder) WithLastActive(t time.Time) *UserBuilder {
	b.user.LastActiveAt = &t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *UserBuilder) WithDeleted(t time.Time) *UserBuilder {
	b.user.DeletedAt = &t
	b.user.Status = UserStatusDeleted
	return b
}

// Build validates and returns the user.
func (b *UserBuilder) Build() (*User, error) {
	if err := b.user.Validate(); err != nil {
		return nil, err
	}
	return b.user, nil
}

// MustBuild builds without error (panics on error).
func (b *UserBuilder) MustBuild() *User {
	u, err := b.Build()
	if err != nil {
		panic(err)
	}
	return u
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestUser1 = MustNewUser("john_doe", "john@example.com", "Test@1234", "John Doe")
	TestUser2 = MustNewUser("jane_smith", "jane@example.com", "Test@1234", "Jane Smith")
	TestAdmin = MustNewUser("admin", "admin@example.com", "Admin@1234", "Admin User")
)

// MustNewAdminUser creates an admin user for testing.
func MustNewAdminUser(username, email, password, fullName string) *User {
	u, err := NewUser(username, email, password, fullName)
	if err != nil {
		panic(err)
	}
	u.Role = UserRoleAdmin
	return u
}

// MustNewSuspendedUser creates a suspended user for testing.
func MustNewSuspendedUser(username, email, password, fullName string) *User {
	u, err := NewUser(username, email, password, fullName)
	if err != nil {
		panic(err)
	}
	_ = u.Suspend()
	return u
}

// MustNewDeletedUser creates a deleted user for testing.
func MustNewDeletedUser(username, email, password, fullName string) *User {
	u, err := NewUser(username, email, password, fullName)
	if err != nil {
		panic(err)
	}
	_ = u.SoftDelete()
	return u
}