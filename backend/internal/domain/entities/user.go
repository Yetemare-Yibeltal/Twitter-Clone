// backend/internal/domain/entities/user.go
package entities

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserRole defines the role of a user.
type UserRole string

const (
	RoleUser      UserRole = "user"
	RoleModerator UserRole = "moderator"
	RoleAdmin     UserRole = "admin"
)

// UserStatus represents the current status of a user.
type UserStatus string

const (
	StatusActive    UserStatus = "active"
	StatusInactive  UserStatus = "inactive"
	StatusSuspended UserStatus = "suspended"
	StatusDeleted   UserStatus = "deleted"
)

// Common validation errors.
var (
	ErrEmptyUsername      = errors.New("username cannot be empty")
	ErrInvalidUsername    = errors.New("username must be 3-20 characters, alphanumeric, underscore, or dot")
	ErrEmptyEmail         = errors.New("email cannot be empty")
	ErrInvalidEmail       = errors.New("invalid email format")
	ErrEmptyPassword      = errors.New("password cannot be empty")
	ErrWeakPassword       = errors.New("password must be at least 8 characters with a mix of letters, numbers, and symbols")
	ErrEmptyFullName      = errors.New("full name cannot be empty")
	ErrBioTooLong         = errors.New("bio must not exceed 160 characters")
	ErrInvalidRole        = errors.New("invalid role")
	ErrInvalidStatus      = errors.New("invalid status")
	ErrUserAlreadyDeleted = errors.New("user already deleted")
	ErrUserSuspended      = errors.New("user is suspended")
	ErrUserInactive       = errors.New("user is inactive")
)

// User represents the core domain entity for a user.
type User struct {
	// Primary identifiers
	ID        string    `db:"id" json:"id"`
	Username  string    `db:"username" json:"username"`
	Email     string    `db:"email" json:"email"`
	Password  string    `db:"-" json:"-"`                // plain-text password (not stored)
	PasswordHash string `db:"password_hash" json:"-"`    // hashed password stored in DB

	// Profile fields
	FullName  string `db:"full_name" json:"full_name"`
	Bio       string `db:"bio" json:"bio"`
	AvatarURL string `db:"avatar_url" json:"avatar_url"`

	// Flags
	IsVerified  bool `db:"is_verified" json:"is_verified"`
	IsSuspended bool `db:"is_suspended" json:"is_suspended"`
	IsActive    bool `db:"is_active" json:"is_active"`

	// Role
	Role UserRole `db:"role" json:"role"`

	// Counters (denormalized)
	TweetCount     int64 `db:"tweet_count" json:"tweet_count"`
	FollowerCount  int64 `db:"follower_count" json:"follower_count"`
	FollowingCount int64 `db:"following_count" json:"following_count"`

	// Timestamps
	LastActive *time.Time `db:"last_active" json:"last_active"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`

	// Suspension reason
	SuspendedReason string `db:"suspended_reason" json:"suspended_reason,omitempty"`
}

// NewUser creates a new User instance with default values and validation.
func NewUser(username, email, password, fullName string) (*User, error) {
	user := &User{
		Username:  username,
		Email:     email,
		Password:  password,
		FullName:  fullName,
		IsActive:  true,
		IsVerified: false,
		IsSuspended: false,
		Role:      RoleUser,
		TweetCount: 0,
		FollowerCount: 0,
		FollowingCount: 0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := user.Validate(); err != nil {
		return nil, err
	}
	if err := user.HashPassword(); err != nil {
		return nil, err
	}
	return user, nil
}

// Validate checks all fields for correctness.
func (u *User) Validate() error {
	// Validate username
	if strings.TrimSpace(u.Username) == "" {
		return ErrEmptyUsername
	}
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_.]{3,20}$`)
	if !usernameRegex.MatchString(u.Username) {
		return ErrInvalidUsername
	}

	// Validate email
	if strings.TrimSpace(u.Email) == "" {
		return ErrEmptyEmail
	}
	if !isValidEmail(u.Email) {
		return ErrInvalidEmail
	}

	// Validate password (only if set, not hashed)
	if u.Password != "" {
		if len(u.Password) < 8 {
			return ErrWeakPassword
		}
		// Check for at least one letter, one number, and one symbol
		hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(u.Password)
		hasNumber := regexp.MustCompile(`[0-9]`).MatchString(u.Password)
		hasSymbol := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};:'",.<>/?]`).MatchString(u.Password)
		if !hasLetter || !hasNumber || !hasSymbol {
			return ErrWeakPassword
		}
	}

	// Validate full name
	if strings.TrimSpace(u.FullName) == "" {
		return ErrEmptyFullName
	}

	// Validate bio length
	if len(u.Bio) > 160 {
		return ErrBioTooLong
	}

	// Validate role
	if u.Role != RoleUser && u.Role != RoleModerator && u.Role != RoleAdmin {
		return ErrInvalidRole
	}

	// Validate status flags consistency
	if u.IsSuspended && u.IsActive {
		return errors.New("user cannot be both suspended and active")
	}
	if u.DeletedAt != nil && u.IsActive {
		return errors.New("deleted user cannot be active")
	}

	return nil
}

// isValidEmail validates email format using regex.
func isValidEmail(email string) bool {
	// Basic email regex (RFC 5322 simplified)
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// HashPassword hashes the plain-text password and stores it in PasswordHash.
func (u *User) HashPassword() error {
	if u.Password == "" {
		return nil // allow empty if not provided (e.g., during updates)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword compares a plain-text password with the stored hash.
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// ChangePassword updates the user's password after validation.
func (u *User) ChangePassword(newPassword string) error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.IsSuspended {
		return ErrUserSuspended
	}
	if !u.IsActive {
		return ErrUserInactive
	}
	// Validate new password
	tmp := &User{Password: newPassword}
	if err := tmp.Validate(); err != nil {
		return err
	}
	u.Password = newPassword
	return u.HashPassword()
}

// UpdateProfile updates the user's profile fields (full name, bio, avatar).
func (u *User) UpdateProfile(fullName, bio, avatarURL string) error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.IsSuspended {
		return ErrUserSuspended
	}
	if !u.IsActive {
		return ErrUserInactive
	}
	u.FullName = fullName
	u.Bio = bio
	u.AvatarURL = avatarURL
	return u.Validate()
}

// Activate sets the user as active.
func (u *User) Activate() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	u.IsActive = true
	u.IsSuspended = false
	return nil
}

// Deactivate sets the user as inactive (not deleted, just inactive).
func (u *User) Deactivate() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	u.IsActive = false
	return nil
}

// Suspend suspends the user with a reason.
func (u *User) Suspend(reason string) error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.IsSuspended {
		return errors.New("user is already suspended")
	}
	u.IsSuspended = true
	u.IsActive = false
	u.SuspendedReason = reason
	return nil
}

// Unsuspend reinstates a suspended user.
func (u *User) Unsuspend() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if !u.IsSuspended {
		return errors.New("user is not suspended")
	}
	u.IsSuspended = false
	u.IsActive = true
	u.SuspendedReason = ""
	return nil
}

// Verify marks the user as verified.
func (u *User) Verify() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	if u.IsSuspended {
		return ErrUserSuspended
	}
	u.IsVerified = true
	return nil
}

// Unverify removes the verified status.
func (u *User) Unverify() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	u.IsVerified = false
	return nil
}

// Delete soft-deletes the user.
func (u *User) Delete() error {
	if u.DeletedAt != nil {
		return ErrUserAlreadyDeleted
	}
	now := time.Now()
	u.DeletedAt = &now
	u.IsActive = false
	return nil
}

// Restore restores a soft-deleted user.
func (u *User) Restore() error {
	if u.DeletedAt == nil {
		return errors.New("user is not deleted")
	}
	u.DeletedAt = nil
	u.IsActive = true
	return nil
}

// GetStatus returns the user's current status.
func (u *User) GetStatus() UserStatus {
	if u.DeletedAt != nil {
		return StatusDeleted
	}
	if u.IsSuspended {
		return StatusSuspended
	}
	if !u.IsActive {
		return StatusInactive
	}
	return StatusActive
}

// IsAdmin checks if the user has admin role.
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsModerator checks if the user has moderator or admin role.
func (u *User) IsModerator() bool {
	return u.Role == RoleModerator || u.Role == RoleAdmin
}

// CanModerate checks if the user can moderate content (moderator or admin).
func (u *User) CanModerate() bool {
	return u.IsModerator()
}

// IsOwner checks if the user owns a given resource (by ID).
func (u *User) IsOwner(resourceUserID string) bool {
	return u.ID == resourceUserID
}

// IncrementTweetCount increases the tweet counter by 1.
func (u *User) IncrementTweetCount() {
	u.TweetCount++
}

// DecrementTweetCount decreases the tweet counter by 1 (minimum 0).
func (u *User) DecrementTweetCount() {
	if u.TweetCount > 0 {
		u.TweetCount--
	}
}

// IncrementFollowerCount increases the follower count.
func (u *User) IncrementFollowerCount() {
	u.FollowerCount++
}

// DecrementFollowerCount decreases the follower count (minimum 0).
func (u *User) DecrementFollowerCount() {
	if u.FollowerCount > 0 {
		u.FollowerCount--
	}
}

// IncrementFollowingCount increases the following count.
func (u *User) IncrementFollowingCount() {
	u.FollowingCount++
}

// DecrementFollowingCount decreases the following count (minimum 0).
func (u *User) DecrementFollowingCount() {
	if u.FollowingCount > 0 {
		u.FollowingCount--
	}
}

// UpdateLastActive updates the last active timestamp to now.
func (u *User) UpdateLastActive() {
	now := time.Now()
	u.LastActive = &now
}

// BeforeSave prepares the user for persistence (e.g., hashing password).
func (u *User) BeforeSave() error {
	if u.Password != "" {
		if err := u.HashPassword(); err != nil {
			return err
		}
		u.Password = "" // clear plain-text after hashing
	}
	return u.Validate()
}

// ToPublic returns a copy of the user with sensitive fields removed.
func (u *User) ToPublic() *User {
	clone := *u
	clone.Password = ""
	clone.PasswordHash = ""
	return &clone
}

// String returns a human-readable representation.
func (u *User) String() string {
	return fmt.Sprintf("User{ID: %s, Username: %s, FullName: %s, Role: %s, Status: %s}",
		u.ID, u.Username, u.FullName, u.Role, u.GetStatus())
}

// Equals checks if two users are the same by ID.
func (u *User) Equals(other *User) bool {
	return u.ID == other.ID
}

// Clone returns a deep copy of the user.
func (u *User) Clone() *User {
	clone := *u
	if u.LastActive != nil {
		t := *u.LastActive
		clone.LastActive = &t
	}
	if u.DeletedAt != nil {
		t := *u.DeletedAt
		clone.DeletedAt = &t
	}
	return &clone
}

// SetRole safely sets the role.
func (u *User) SetRole(role UserRole) error {
	if role != RoleUser && role != RoleModerator && role != RoleAdmin {
		return ErrInvalidRole
	}
	u.Role = role
	return nil
}

// HasPermission checks if the user has a given permission (simplified).
// Could be extended with RBAC.
func (u *User) HasPermission(action string) bool {
	// For now, admins have all permissions, moderators have moderate, users have basic.
	switch action {
	case "admin":
		return u.IsAdmin()
	case "moderate":
		return u.IsModerator()
	case "write":
		return u.GetStatus() == StatusActive
	default:
		return false
	}
}

// IsProfileComplete checks if all required fields are filled.
func (u *User) IsProfileComplete() bool {
	return u.FullName != "" && u.AvatarURL != "" && u.Bio != ""
}

// DaysSinceJoin returns the number of days since the user joined.
func (u *User) DaysSinceJoin() int {
	return int(time.Since(u.CreatedAt).Hours() / 24)
}