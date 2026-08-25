// backend/internal/domain/entities/follow.go
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

// String returns the string representation of the status.
func (s FollowStatus) String() string {
	return string(s)
}

// ======================================================================
// Errors
// ======================================================================

var (
	ErrFollowIDEmpty        = errors.New("follow ID cannot be empty")
	ErrFollowerIDEmpty      = errors.New("follower ID cannot be empty")
	ErrFolloweeIDEmpty      = errors.New("followee ID cannot be empty")
	ErrFollowSelf           = errors.New("cannot follow yourself")
	ErrInvalidFollowStatus  = errors.New("invalid follow status")
	ErrFollowAlreadyExists  = errors.New("follow relationship already exists")
	ErrFollowNotFound       = errors.New("follow relationship not found")
	ErrFollowAlreadyAccepted = errors.New("follow already accepted")
	ErrFollowAlreadyRejected = errors.New("follow already rejected")
	ErrFollowAlreadyBlocked  = errors.New("follow already blocked")
	ErrFollowAlreadyPending  = errors.New("follow already pending")
	ErrFollowCannotAccept   = errors.New("cannot accept this follow")
	ErrFollowCannotReject   = errors.New("cannot reject this follow")
	ErrFollowCannotBlock    = errors.New("cannot block this follow")
	ErrFollowCannotUnblock  = errors.New("cannot unblock this follow")
	ErrFollowAlreadyDeleted = errors.New("follow already deleted")
	ErrFollowNotDeleted     = errors.New("follow is not deleted")
)

// ======================================================================
// Follow Entity
// ======================================================================

// Follow represents a follow relationship between two users.
type Follow struct {
	ID         string       `db:"id" json:"id"`
	FollowerID string       `db:"follower_id" json:"follower_id"`
	FolloweeID string       `db:"followee_id" json:"followee_id"`
	Status     FollowStatus `db:"status" json:"status"`
	CreatedAt  time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time    `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time   `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ======================================================================
= Factory Methods
// ======================================================================

// NewFollow creates a new follow relationship with default status.
func NewFollow(followerID, followeeID string) (*Follow, error) {
	if followerID == followeeID {
		return nil, ErrFollowSelf
	}
	f := &Follow{
		ID:         uuid.New().String(),
		FollowerID: followerID,
		FolloweeID: followeeID,
		Status:     FollowStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return f, nil
}

// NewAcceptedFollow creates a new accepted follow relationship.
func NewAcceptedFollow(followerID, followeeID string) (*Follow, error) {
	f, err := NewFollow(followerID, followeeID)
	if err != nil {
		return nil, err
	}
	f.Status = FollowStatusAccepted
	return f, nil
}

// MustNewFollow creates a new follow and panics on error.
func MustNewFollow(followerID, followeeID string) *Follow {
	f, err := NewFollow(followerID, followeeID)
	if err != nil {
		panic(err)
	}
	return f
}

// ======================================================================
= Validation
// ======================================================================

// Validate performs comprehensive validation.
func (f *Follow) Validate() error {
	if strings.TrimSpace(f.ID) == "" {
		return ErrFollowIDEmpty
	}
	if strings.TrimSpace(f.FollowerID) == "" {
		return ErrFollowerIDEmpty
	}
	if strings.TrimSpace(f.FolloweeID) == "" {
		return ErrFolloweeIDEmpty
	}
	if f.FollowerID == f.FolloweeID {
		return ErrFollowSelf
	}
	if !f.Status.IsValid() {
		return ErrInvalidFollowStatus
	}
	if f.DeletedAt != nil {
		return ErrFollowAlreadyDeleted
	}
	return nil
}

// ======================================================================
= Status Management
// ======================================================================

// Accept accepts a pending follow request.
func (f *Follow) Accept() error {
	if f.DeletedAt != nil {
		return ErrFollowAlreadyDeleted
	}
	if f.Status == FollowStatusAccepted {
		return ErrFollowAlreadyAccepted
	}
	if f.Status != FollowStatusPending {
		return ErrFollowCannotAccept
	}
	f.Status = FollowStatusAccepted
	f.UpdatedAt = time.Now()
	return nil
}

// Reject rejects a pending follow request.
func (f *Follow) Reject() error {
	if f.DeletedAt != nil {
		return ErrFollowAlreadyDeleted
	}
	if f.Status == FollowStatusRejected {
		return ErrFollowAlreadyRejected
	}
	if f.Status != FollowStatusPending {
		return ErrFollowCannotReject
	}
	f.Status = FollowStatusRejected
	f.UpdatedAt = time.Now()
	return nil
}

// Block blocks a follow relationship.
func (f *Follow) Block() error {
	if f.DeletedAt != nil {
		return ErrFollowAlreadyDeleted
	}
	if f.Status == FollowStatusBlocked {
		return ErrFollowAlreadyBlocked
	}
	f.Status = FollowStatusBlocked
	f.UpdatedAt = time.Now()
	return nil
}

// Unblock unblocks a follow relationship (sets to pending).
func (f *Follow) Unblock() error {
	if f.DeletedAt != nil {
		return ErrFollowAlreadyDeleted
	}
	if f.Status != FollowStatusBlocked {
		return ErrFollowCannotUnblock
	}
	f.Status = FollowStatusPending
	f.UpdatedAt = time.Now()
	return nil
}

// IsPending returns true if the follow is pending.
func (f *Follow) IsPending() bool {
	return f.Status == FollowStatusPending
}

// IsAccepted returns true if the follow is accepted.
func (f *Follow) IsAccepted() bool {
	return f.Status == FollowStatusAccepted
}

// IsRejected returns true if the follow is rejected.
func (f *Follow) IsRejected() bool {
	return f.Status == FollowStatusRejected
}

// IsBlocked returns true if the follow is blocked.
func (f *Follow) IsBlocked() bool {
	return f.Status == FollowStatusBlocked
}

// IsActive returns true if the follow is active (accepted).
func (f *Follow) IsActive() bool {
	return f.IsAccepted() && f.DeletedAt == nil
}

// ======================================================================
= Deletion Operations
// ======================================================================

// SoftDelete marks the follow as deleted.
func (f *Follow) SoftDelete() error {
	if f.DeletedAt != nil {
		return ErrFollowAlreadyDeleted
	}
	now := time.Now()
	f.DeletedAt = &now
	f.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted follow.
func (f *Follow) Restore() error {
	if f.DeletedAt == nil {
		return ErrFollowNotDeleted
	}
	f.DeletedAt = nil
	f.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the follow is deleted.
func (f *Follow) IsDeleted() bool {
	return f.DeletedAt != nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// IsSelf checks if a user is following themselves.
func (f *Follow) IsSelf() bool {
	return f.FollowerID == f.FolloweeID
}

// IsFollower checks if a user is the follower.
func (f *Follow) IsFollower(userID string) bool {
	return f.FollowerID == userID
}

// IsFollowee checks if a user is the followee.
func (f *Follow) IsFollowee(userID string) bool {
	return f.FolloweeID == userID
}

// IsParticipant checks if a user is either the follower or followee.
func (f *Follow) IsParticipant(userID string) bool {
	return f.FollowerID == userID || f.FolloweeID == userID
}

// GetOtherParticipant returns the other participant ID.
func (f *Follow) GetOtherParticipant(userID string) (string, error) {
	if !f.IsParticipant(userID) {
		return "", errors.New("user is not a participant")
	}
	if f.FollowerID == userID {
		return f.FolloweeID, nil
	}
	return f.FollowerID, nil
}

// CanTransitionTo checks if the status can transition to a new status.
func (f *Follow) CanTransitionTo(newStatus FollowStatus) bool {
	if f.DeletedAt != nil {
		return false
	}
	switch f.Status {
	case FollowStatusPending:
		return newStatus == FollowStatusAccepted ||
			newStatus == FollowStatusRejected ||
			newStatus == FollowStatusBlocked
	case FollowStatusAccepted:
		return newStatus == FollowStatusBlocked ||
			newStatus == FollowStatusPending
	case FollowStatusRejected:
		return newStatus == FollowStatusPending ||
			newStatus == FollowStatusBlocked
	case FollowStatusBlocked:
		return newStatus == FollowStatusPending
	default:
		return false
	}
}

// ======================================================================
= Comparison
// ======================================================================

// Equals checks if two follows are the same by ID.
func (f *Follow) Equals(other *Follow) bool {
	return f.ID == other.ID
}

// IsEmpty returns true if the follow is zero value.
func (f *Follow) IsEmpty() bool {
	return f.ID == "" && f.FollowerID == "" && f.FolloweeID == ""
}

// Clone returns a deep copy of the follow.
func (f *Follow) Clone() *Follow {
	clone := *f
	if f.DeletedAt != nil {
		t := *f.DeletedAt
		clone.DeletedAt = &t
	}
	return &clone
}

// ======================================================================
= String Representation
// ======================================================================

// String returns a human-readable representation.
func (f *Follow) String() string {
	return fmt.Sprintf("Follow{ID:%s, follower:%s, followee:%s, status:%s, created:%v}",
		f.ID, f.FollowerID, f.FolloweeID, f.Status, f.CreatedAt)
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (f Follow) Value() (driver.Value, error) {
	return json.Marshal(f)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (f *Follow) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for Follow: %T", value)
	}
	return json.Unmarshal(bytes, f)
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (f *Follow) MarshalJSON() ([]byte, error) {
	type Alias Follow
	return json.Marshal(&struct {
		*Alias
		Status string `json:"status"`
		IsActive bool `json:"is_active"`
	}{
		Alias:    (*Alias)(f),
		Status:   string(f.Status),
		IsActive: f.IsActive(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (f *Follow) UnmarshalJSON(data []byte) error {
	type Alias Follow
	aux := &struct {
		*Alias
		Status string `json:"status"`
	}{
		Alias: (*Alias)(f),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Status != "" {
		f.Status = FollowStatus(aux.Status)
	}
	return nil
}

// ======================================================================
= Follow Group (for batch operations)
// ======================================================================

// FollowGroup represents a group of follows.
type FollowGroup struct {
	Follows []*Follow `json:"follows"`
	Total   int64     `json:"total"`
}

// NewFollowGroup creates a new follow group.
func NewFollowGroup() *FollowGroup {
	return &FollowGroup{
		Follows: []*Follow{},
		Total:   0,
	}
}

// Add adds a follow to the group.
func (g *FollowGroup) Add(f *Follow) {
	g.Follows = append(g.Follows, f)
	g.Total++
}

// Contains checks if a follow is in the group.
func (g *FollowGroup) Contains(id string) bool {
	for _, f := range g.Follows {
		if f.ID == id {
			return true
		}
	}
	return false
}

// FilterByStatus returns follows with a specific status.
func (g *FollowGroup) FilterByStatus(status FollowStatus) []*Follow {
	result := []*Follow{}
	for _, f := range g.Follows {
		if f.Status == status {
			result = append(result, f)
		}
	}
	return result
}

// FilterByFollower returns follows by a specific follower.
func (g *FollowGroup) FilterByFollower(userID string) []*Follow {
	result := []*Follow{}
	for _, f := range g.Follows {
		if f.FollowerID == userID {
			result = append(result, f)
		}
	}
	return result
}

// FilterByFollowee returns follows for a specific followee.
func (g *FollowGroup) FilterByFollowee(userID string) []*Follow {
	result := []*Follow{}
	for _, f := range g.Follows {
		if f.FolloweeID == userID {
			result = append(result, f)
		}
	}
	return result
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// FollowBuilder helps construct follows for testing.
type FollowBuilder struct {
	follow *Follow
}

// NewFollowBuilder creates a new follow builder.
func NewFollowBuilder() *FollowBuilder {
	return &FollowBuilder{
		follow: &Follow{
			ID:         uuid.New().String(),
			FollowerID: "",
			FolloweeID: "",
			Status:     FollowStatusPending,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *FollowBuilder) WithID(id string) *FollowBuilder {
	b.follow.ID = id
	return b
}

// WithFollower sets the follower ID.
func (b *FollowBuilder) WithFollower(followerID string) *FollowBuilder {
	b.follow.FollowerID = followerID
	return b
}

// WithFollowee sets the followee ID.
func (b *FollowBuilder) WithFollowee(followeeID string) *FollowBuilder {
	b.follow.FolloweeID = followeeID
	return b
}

// WithStatus sets the status.
func (b *FollowBuilder) WithStatus(status FollowStatus) *FollowBuilder {
	b.follow.Status = status
	return b
}

// WithCreatedAt sets the creation time.
func (b *FollowBuilder) WithCreatedAt(t time.Time) *FollowBuilder {
	b.follow.CreatedAt = t
	b.follow.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *FollowBuilder) WithDeleted(t time.Time) *FollowBuilder {
	b.follow.DeletedAt = &t
	return b
}

// Build validates and returns the follow.
func (b *FollowBuilder) Build() (*Follow, error) {
	if err := b.follow.Validate(); err != nil {
		return nil, err
	}
	return b.follow, nil
}

// MustBuild builds without error (panics on error).
func (b *FollowBuilder) MustBuild() *Follow {
	f, err := b.Build()
	if err != nil {
		panic(err)
	}
	return f
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestFollow1 = MustNewFollow("user1", "user2")
	TestFollow2 = MustNewFollow("user3", "user1")
	TestFollow3 = MustNewFollow("user2", "user3")
)

// MustNewAcceptedFollow creates an accepted follow for testing.
func MustNewAcceptedFollow(followerID, followeeID string) *Follow {
	f, err := NewAcceptedFollow(followerID, followeeID)
	if err != nil {
		panic(err)
	}
	return f
}

// MustNewBlockedFollow creates a blocked follow for testing.
func MustNewBlockedFollow(followerID, followeeID string) *Follow {
	f, err := NewFollow(followerID, followeeID)
	if err != nil {
		panic(err)
	}
	if err := f.Block(); err != nil {
		panic(err)
	}
	return f
}