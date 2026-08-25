// backend/internal/domain/entities/like.go
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

// LikeType represents the type of like (regular or super like).
type LikeType string

const (
	LikeTypeRegular LikeType = "regular"
	LikeTypeSuper   LikeType = "super"
)

// ValidLikeTypes returns all valid like types.
func ValidLikeTypes() []LikeType {
	return []LikeType{
		LikeTypeRegular,
		LikeTypeSuper,
	}
}

// IsValid checks if a like type is valid.
func (t LikeType) IsValid() bool {
	for _, typ := range ValidLikeTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation of the like type.
func (t LikeType) String() string {
	return string(t)
}

// ======================================================================
// Errors
// ======================================================================

var (
	ErrLikeIDEmpty        = errors.New("like ID cannot be empty")
	ErrLikeTweetIDEmpty   = errors.New("tweet ID cannot be empty")
	ErrLikeUserIDEmpty    = errors.New("user ID cannot be empty")
	ErrLikeAlreadyExists  = errors.New("like already exists")
	ErrLikeNotFound       = errors.New("like not found")
	ErrLikeAlreadyDeleted = errors.New("like already deleted")
	ErrLikeNotDeleted     = errors.New("like is not deleted")
	ErrInvalidLikeType    = errors.New("invalid like type")
	ErrLikeSelf           = errors.New("cannot like your own tweet (for super likes)")
)

// ======================================================================
// Like Entity
// ======================================================================

// Like represents a like on a tweet.
type Like struct {
	ID        string    `db:"id" json:"id"`
	TweetID   string    `db:"tweet_id" json:"tweet_id"`
	UserID    string    `db:"user_id" json:"user_id"`
	Type      LikeType  `db:"type" json:"type"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewLike creates a new like with default type.
func NewLike(tweetID, userID string) (*Like, error) {
	return NewLikeWithType(tweetID, userID, LikeTypeRegular)
}

// NewLikeWithType creates a new like with a specific type.
func NewLikeWithType(tweetID, userID string, likeType LikeType) (*Like, error) {
	l := &Like{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		UserID:    userID,
		Type:      likeType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return l, nil
}

// NewSuperLike creates a new super like.
func NewSuperLike(tweetID, userID string) (*Like, error) {
	return NewLikeWithType(tweetID, userID, LikeTypeSuper)
}

// MustNewLike creates a new like and panics on error.
func MustNewLike(tweetID, userID string) *Like {
	l, err := NewLike(tweetID, userID)
	if err != nil {
		panic(err)
	}
	return l
}

// MustNewSuperLike creates a new super like and panics on error.
func MustNewSuperLike(tweetID, userID string) *Like {
	l, err := NewSuperLike(tweetID, userID)
	if err != nil {
		panic(err)
	}
	return l
}

// ======================================================================
// Validation
// ======================================================================

// Validate performs comprehensive validation.
func (l *Like) Validate() error {
	if strings.TrimSpace(l.ID) == "" {
		return ErrLikeIDEmpty
	}
	if strings.TrimSpace(l.TweetID) == "" {
		return ErrLikeTweetIDEmpty
	}
	if strings.TrimSpace(l.UserID) == "" {
		return ErrLikeUserIDEmpty
	}
	if !l.Type.IsValid() {
		return ErrInvalidLikeType
	}
	if l.DeletedAt != nil {
		return ErrLikeAlreadyDeleted
	}
	return nil
}

// ======================================================================
= Type Management
// ======================================================================

// IsRegular returns true if the like is regular.
func (l *Like) IsRegular() bool {
	return l.Type == LikeTypeRegular
}

// IsSuper returns true if the like is super.
func (l *Like) IsSuper() bool {
	return l.Type == LikeTypeSuper
}

// SetType sets the like type.
func (l *Like) SetType(likeType LikeType) error {
	if l.DeletedAt != nil {
		return ErrLikeAlreadyDeleted
	}
	if !likeType.IsValid() {
		return ErrInvalidLikeType
	}
	l.Type = likeType
	l.UpdatedAt = time.Now()
	return nil
}

// PromoteToSuper promotes a regular like to super like.
func (l *Like) PromoteToSuper() error {
	if l.DeletedAt != nil {
		return ErrLikeAlreadyDeleted
	}
	if l.IsSuper() {
		return errors.New("like is already super")
	}
	l.Type = LikeTypeSuper
	l.UpdatedAt = time.Now()
	return nil
}

// DemoteToRegular demotes a super like to regular like.
func (l *Like) DemoteToRegular() error {
	if l.DeletedAt != nil {
		return ErrLikeAlreadyDeleted
	}
	if l.IsRegular() {
		return errors.New("like is already regular")
	}
	l.Type = LikeTypeRegular
	l.UpdatedAt = time.Now()
	return nil
}

// ======================================================================
= Deletion Operations
// ======================================================================

// SoftDelete marks the like as deleted.
func (l *Like) SoftDelete() error {
	if l.DeletedAt != nil {
		return ErrLikeAlreadyDeleted
	}
	now := time.Now()
	l.DeletedAt = &now
	l.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted like.
func (l *Like) Restore() error {
	if l.DeletedAt == nil {
		return ErrLikeNotDeleted
	}
	l.DeletedAt = nil
	l.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the like is deleted.
func (l *Like) IsDeleted() bool {
	return l.DeletedAt != nil
}

// IsActive checks if the like is active (not deleted).
func (l *Like) IsActive() bool {
	return l.DeletedAt == nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// IsFromUser checks if the like is from a specific user.
func (l *Like) IsFromUser(userID string) bool {
	return l.UserID == userID
}

// IsOnTweet checks if the like is on a specific tweet.
func (l *Like) IsOnTweet(tweetID string) bool {
	return l.TweetID == tweetID
}

// String returns a human-readable representation.
func (l *Like) String() string {
	return fmt.Sprintf("Like{ID:%s, tweet:%s, user:%s, type:%s, created:%v}",
		l.ID, l.TweetID, l.UserID, l.Type, l.CreatedAt)
}

// Clone returns a deep copy of the like.
func (l *Like) Clone() *Like {
	clone := *l
	if l.DeletedAt != nil {
		t := *l.DeletedAt
		clone.DeletedAt = &t
	}
	return &clone
}

// Equals checks if two likes are the same by ID.
func (l *Like) Equals(other *Like) bool {
	return l.ID == other.ID
}

// IsEmpty returns true if the like is zero value.
func (l *Like) IsEmpty() bool {
	return l.ID == "" && l.TweetID == "" && l.UserID == ""
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (l Like) Value() (driver.Value, error) {
	return json.Marshal(l)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (l *Like) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for Like: %T", value)
	}
	return json.Unmarshal(bytes, l)
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (l *Like) MarshalJSON() ([]byte, error) {
	type Alias Like
	return json.Marshal(&struct {
		*Alias
		Type     string `json:"type"`
		IsActive bool   `json:"is_active"`
	}{
		Alias:    (*Alias)(l),
		Type:     string(l.Type),
		IsActive: l.IsActive(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (l *Like) UnmarshalJSON(data []byte) error {
	type Alias Like
	aux := &struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(l),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Type != "" {
		l.Type = LikeType(aux.Type)
	}
	return nil
}

// ======================================================================
= Like Group (for batch operations)
// ======================================================================

// LikeGroup represents a group of likes.
type LikeGroup struct {
	Likes []*Like `json:"likes"`
	Total int64   `json:"total"`
}

// NewLikeGroup creates a new like group.
func NewLikeGroup() *LikeGroup {
	return &LikeGroup{
		Likes: []*Like{},
		Total: 0,
	}
}

// Add adds a like to the group.
func (g *LikeGroup) Add(l *Like) {
	g.Likes = append(g.Likes, l)
	g.Total++
}

// Contains checks if a like is in the group.
func (g *LikeGroup) Contains(id string) bool {
	for _, l := range g.Likes {
		if l.ID == id {
			return true
		}
	}
	return false
}

// FilterByUser returns likes by a specific user.
func (g *LikeGroup) FilterByUser(userID string) []*Like {
	result := []*Like{}
	for _, l := range g.Likes {
		if l.UserID == userID {
			result = append(result, l)
		}
	}
	return result
}

// FilterByTweet returns likes on a specific tweet.
func (g *LikeGroup) FilterByTweet(tweetID string) []*Like {
	result := []*Like{}
	for _, l := range g.Likes {
		if l.TweetID == tweetID {
			result = append(result, l)
		}
	}
	return result
}

// FilterByType returns likes of a specific type.
func (g *LikeGroup) FilterByType(likeType LikeType) []*Like {
	result := []*Like{}
	for _, l := range g.Likes {
		if l.Type == likeType {
			result = append(result, l)
		}
	}
	return result
}

// GetSuperLikes returns all super likes.
func (g *LikeGroup) GetSuperLikes() []*Like {
	return g.FilterByType(LikeTypeSuper)
}

// GetRegularLikes returns all regular likes.
func (g *LikeGroup) GetRegularLikes() []*Like {
	return g.FilterByType(LikeTypeRegular)
}

// ======================================================================
= Like Statistics
// ======================================================================

// LikeStats represents like statistics.
type LikeStats struct {
	TotalLikes    int64 `json:"total_likes"`
	RegularLikes  int64 `json:"regular_likes"`
	SuperLikes    int64 `json:"super_likes"`
	UniqueUsers   int64 `json:"unique_users"`
}

// CalculateStats calculates statistics from a like group.
func (g *LikeGroup) CalculateStats() *LikeStats {
	stats := &LikeStats{
		TotalLikes:   int64(len(g.Likes)),
		RegularLikes: 0,
		SuperLikes:   0,
		UniqueUsers:  0,
	}
	users := make(map[string]bool)
	for _, l := range g.Likes {
		users[l.UserID] = true
		if l.IsRegular() {
			stats.RegularLikes++
		} else if l.IsSuper() {
			stats.SuperLikes++
		}
	}
	stats.UniqueUsers = int64(len(users))
	return stats
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// LikeBuilder helps construct likes for testing.
type LikeBuilder struct {
	like *Like
}

// NewLikeBuilder creates a new like builder.
func NewLikeBuilder() *LikeBuilder {
	return &LikeBuilder{
		like: &Like{
			ID:        uuid.New().String(),
			TweetID:   "",
			UserID:    "",
			Type:      LikeTypeRegular,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *LikeBuilder) WithID(id string) *LikeBuilder {
	b.like.ID = id
	return b
}

// WithTweetID sets the tweet ID.
func (b *LikeBuilder) WithTweetID(tweetID string) *LikeBuilder {
	b.like.TweetID = tweetID
	return b
}

// WithUserID sets the user ID.
func (b *LikeBuilder) WithUserID(userID string) *LikeBuilder {
	b.like.UserID = userID
	return b
}

// WithType sets the like type.
func (b *LikeBuilder) WithType(likeType LikeType) *LikeBuilder {
	b.like.Type = likeType
	return b
}

// WithCreatedAt sets the creation time.
func (b *LikeBuilder) WithCreatedAt(t time.Time) *LikeBuilder {
	b.like.CreatedAt = t
	b.like.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *LikeBuilder) WithDeleted(t time.Time) *LikeBuilder {
	b.like.DeletedAt = &t
	return b
}

// Build validates and returns the like.
func (b *LikeBuilder) Build() (*Like, error) {
	if err := b.like.Validate(); err != nil {
		return nil, err
	}
	return b.like, nil
}

// MustBuild builds without error (panics on error).
func (b *LikeBuilder) MustBuild() *Like {
	l, err := b.Build()
	if err != nil {
		panic(err)
	}
	return l
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestLike1 = MustNewLike("tweet1", "user1")
	TestLike2 = MustNewLike("tweet1", "user2")
	TestLike3 = MustNewSuperLike("tweet2", "user1")
)

// MustNewLikeWithType creates a like with type and panics on error.
func MustNewLikeWithType(tweetID, userID string, likeType LikeType) *Like {
	l, err := NewLikeWithType(tweetID, userID, likeType)
	if err != nil {
		panic(err)
	}
	return l
}

// MustNewDeletedLike creates a deleted like for testing.
func MustNewDeletedLike(tweetID, userID string) *Like {
	l, err := NewLike(tweetID, userID)
	if err != nil {
		panic(err)
	}
	if err := l.SoftDelete(); err != nil {
		panic(err)
	}
	return l
}