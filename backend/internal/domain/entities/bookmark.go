// backend/internal/domain/entities/bookmark.go
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

// BookmarkCategory represents the category of a bookmark.
type BookmarkCategory string

const (
	BookmarkCategoryReadLater    BookmarkCategory = "read_later"
	BookmarkCategoryFavorites    BookmarkCategory = "favorites"
	BookmarkCategoryImportant    BookmarkCategory = "important"
	BookmarkCategoryWatchLater   BookmarkCategory = "watch_later"
	BookmarkCategoryCustom       BookmarkCategory = "custom"
)

// ValidBookmarkCategories returns all valid bookmark categories.
func ValidBookmarkCategories() []BookmarkCategory {
	return []BookmarkCategory{
		BookmarkCategoryReadLater,
		BookmarkCategoryFavorites,
		BookmarkCategoryImportant,
		BookmarkCategoryWatchLater,
		BookmarkCategoryCustom,
	}
}

// IsValid checks if a bookmark category is valid.
func (c BookmarkCategory) IsValid() bool {
	for _, cat := range ValidBookmarkCategories() {
		if c == cat {
			return true
		}
	}
	return false
}

// String returns the string representation of the category.
func (c BookmarkCategory) String() string {
	return string(c)
}

// ======================================================================
// Errors
// ======================================================================

var (
	ErrBookmarkIDEmpty          = errors.New("bookmark ID cannot be empty")
	ErrBookmarkTweetIDEmpty     = errors.New("tweet ID cannot be empty")
	ErrBookmarkUserIDEmpty      = errors.New("user ID cannot be empty")
	ErrBookmarkAlreadyExists    = errors.New("bookmark already exists")
	ErrBookmarkNotFound         = errors.New("bookmark not found")
	ErrBookmarkAlreadyDeleted   = errors.New("bookmark already deleted")
	ErrBookmarkNotDeleted       = errors.New("bookmark is not deleted")
	ErrInvalidBookmarkCategory  = errors.New("invalid bookmark category")
	ErrBookmarkNotesTooLong     = errors.New("bookmark notes exceed maximum length")
	ErrBookmarkNameRequired     = errors.New("bookmark name is required for custom category")
	ErrBookmarkNameTooLong      = errors.New("bookmark name exceeds maximum length")
)

// ======================================================================
// Bookmark Entity
// ======================================================================

// Bookmark represents a bookmark on a tweet.
type Bookmark struct {
	ID         string            `db:"id" json:"id"`
	TweetID    string            `db:"tweet_id" json:"tweet_id"`
	UserID     string            `db:"user_id" json:"user_id"`
	Category   BookmarkCategory  `db:"category" json:"category"`
	Name       string            `db:"name" json:"name,omitempty"`        // custom name for custom category
	Notes      string            `db:"notes" json:"notes,omitempty"`      // user notes
	Metadata   map[string]interface{} `db:"metadata" json:"metadata,omitempty"`
	CreatedAt  time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time         `db:"updated_at" json:"updated_at"`
	DeletedAt  *time.Time        `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewBookmark creates a new bookmark with default category.
func NewBookmark(tweetID, userID string) (*Bookmark, error) {
	return NewBookmarkWithCategory(tweetID, userID, BookmarkCategoryReadLater, "", "")
}

// NewBookmarkWithCategory creates a new bookmark with a specific category.
func NewBookmarkWithCategory(tweetID, userID string, category BookmarkCategory, name, notes string) (*Bookmark, error) {
	b := &Bookmark{
		ID:         uuid.New().String(),
		TweetID:    tweetID,
		UserID:     userID,
		Category:   category,
		Name:       name,
		Notes:      notes,
		Metadata:   make(map[string]interface{}),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return b, nil
}

// NewBookmarkWithMetadata creates a bookmark with metadata.
func NewBookmarkWithMetadata(tweetID, userID string, category BookmarkCategory, name, notes string, metadata map[string]interface{}) (*Bookmark, error) {
	b, err := NewBookmarkWithCategory(tweetID, userID, category, name, notes)
	if err != nil {
		return nil, err
	}
	b.Metadata = metadata
	return b, nil
}

// MustNewBookmark creates a new bookmark and panics on error.
func MustNewBookmark(tweetID, userID string) *Bookmark {
	b, err := NewBookmark(tweetID, userID)
	if err != nil {
		panic(err)
	}
	return b
}

// MustNewBookmarkWithCategory creates a bookmark with category and panics.
func MustNewBookmarkWithCategory(tweetID, userID string, category BookmarkCategory, name, notes string) *Bookmark {
	b, err := NewBookmarkWithCategory(tweetID, userID, category, name, notes)
	if err != nil {
		panic(err)
	}
	return b
}

// ======================================================================
// Validation
// ======================================================================

// Validate performs comprehensive validation.
func (b *Bookmark) Validate() error {
	if strings.TrimSpace(b.ID) == "" {
		return ErrBookmarkIDEmpty
	}
	if strings.TrimSpace(b.TweetID) == "" {
		return ErrBookmarkTweetIDEmpty
	}
	if strings.TrimSpace(b.UserID) == "" {
		return ErrBookmarkUserIDEmpty
	}
	if !b.Category.IsValid() {
		return ErrInvalidBookmarkCategory
	}
	if b.Category == BookmarkCategoryCustom {
		if strings.TrimSpace(b.Name) == "" {
			return ErrBookmarkNameRequired
		}
		if len(b.Name) > 100 {
			return ErrBookmarkNameTooLong
		}
	}
	if len(b.Notes) > 500 {
		return ErrBookmarkNotesTooLong
	}
	if b.DeletedAt != nil {
		return ErrBookmarkAlreadyDeleted
	}
	return nil
}

// ======================================================================
// Category Management
// ======================================================================

// SetCategory sets the bookmark category.
func (b *Bookmark) SetCategory(category BookmarkCategory, name string) error {
	if b.DeletedAt != nil {
		return ErrBookmarkAlreadyDeleted
	}
	if !category.IsValid() {
		return ErrInvalidBookmarkCategory
	}
	if category == BookmarkCategoryCustom && strings.TrimSpace(name) == "" {
		return ErrBookmarkNameRequired
	}
	b.Category = category
	if category == BookmarkCategoryCustom {
		b.Name = strings.TrimSpace(name)
	} else {
		b.Name = "" // clear custom name
	}
	b.UpdatedAt = time.Now()
	return nil
}

// SetNotes sets the bookmark notes.
func (b *Bookmark) SetNotes(notes string) error {
	if b.DeletedAt != nil {
		return ErrBookmarkAlreadyDeleted
	}
	if len(notes) > 500 {
		return ErrBookmarkNotesTooLong
	}
	b.Notes = strings.TrimSpace(notes)
	b.UpdatedAt = time.Now()
	return nil
}

// SetMetadata sets metadata.
func (b *Bookmark) SetMetadata(key string, value interface{}) error {
	if b.DeletedAt != nil {
		return ErrBookmarkAlreadyDeleted
	}
	if b.Metadata == nil {
		b.Metadata = make(map[string]interface{})
	}
	b.Metadata[key] = value
	b.UpdatedAt = time.Now()
	return nil
}

// RemoveMetadata removes a metadata key.
func (b *Bookmark) RemoveMetadata(key string) error {
	if b.DeletedAt != nil {
		return ErrBookmarkAlreadyDeleted
	}
	delete(b.Metadata, key)
	b.UpdatedAt = time.Now()
	return nil
}

// ======================================================================
// Deletion Operations
// ======================================================================

// SoftDelete marks the bookmark as deleted.
func (b *Bookmark) SoftDelete() error {
	if b.DeletedAt != nil {
		return ErrBookmarkAlreadyDeleted
	}
	now := time.Now()
	b.DeletedAt = &now
	b.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted bookmark.
func (b *Bookmark) Restore() error {
	if b.DeletedAt == nil {
		return ErrBookmarkNotDeleted
	}
	b.DeletedAt = nil
	b.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the bookmark is deleted.
func (b *Bookmark) IsDeleted() bool {
	return b.DeletedAt != nil
}

// IsActive checks if the bookmark is active (not deleted).
func (b *Bookmark) IsActive() bool {
	return b.DeletedAt == nil
}

// ======================================================================
// Helper Methods
// ======================================================================

// IsFromUser checks if the bookmark is from a specific user.
func (b *Bookmark) IsFromUser(userID string) bool {
	return b.UserID == userID
}

// IsOnTweet checks if the bookmark is on a specific tweet.
func (b *Bookmark) IsOnTweet(tweetID string) bool {
	return b.TweetID == tweetID
}

// IsReadLater returns true if category is read_later.
func (b *Bookmark) IsReadLater() bool {
	return b.Category == BookmarkCategoryReadLater
}

// IsFavorites returns true if category is favorites.
func (b *Bookmark) IsFavorites() bool {
	return b.Category == BookmarkCategoryFavorites
}

// IsImportant returns true if category is important.
func (b *Bookmark) IsImportant() bool {
	return b.Category == BookmarkCategoryImportant
}

// IsWatchLater returns true if category is watch_later.
func (b *Bookmark) IsWatchLater() bool {
	return b.Category == BookmarkCategoryWatchLater
}

// IsCustom returns true if category is custom.
func (b *Bookmark) IsCustom() bool {
	return b.Category == BookmarkCategoryCustom
}

// GetDisplayName returns the display name of the bookmark.
func (b *Bookmark) GetDisplayName() string {
	if b.IsCustom() && b.Name != "" {
		return b.Name
	}
	return string(b.Category)
}

// HasNotes returns true if the bookmark has notes.
func (b *Bookmark) HasNotes() bool {
	return strings.TrimSpace(b.Notes) != ""
}

// GetNotePreview returns a preview of the notes.
func (b *Bookmark) GetNotePreview(maxLen int) string {
	if !b.HasNotes() {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 50
	}
	if len(b.Notes) <= maxLen {
		return b.Notes
	}
	return b.Notes[:maxLen] + "..."
}

// String returns a human-readable representation.
func (b *Bookmark) String() string {
	return fmt.Sprintf("Bookmark{ID:%s, tweet:%s, user:%s, category:%s, created:%v}",
		b.ID, b.TweetID, b.UserID, b.Category, b.CreatedAt)
}

// Clone returns a deep copy of the bookmark.
func (b *Bookmark) Clone() *Bookmark {
	clone := *b
	if b.DeletedAt != nil {
		t := *b.DeletedAt
		clone.DeletedAt = &t
	}
	if b.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range b.Metadata {
			clone.Metadata[k] = v
		}
	}
	return &clone
}

// Equals checks if two bookmarks are the same by ID.
func (b *Bookmark) Equals(other *Bookmark) bool {
	return b.ID == other.ID
}

// IsEmpty returns true if the bookmark is zero value.
func (b *Bookmark) IsEmpty() bool {
	return b.ID == "" && b.TweetID == "" && b.UserID == ""
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (b Bookmark) Value() (driver.Value, error) {
	return json.Marshal(b)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (b *Bookmark) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for Bookmark: %T", value)
	}
	return json.Unmarshal(bytes, b)
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (b *Bookmark) MarshalJSON() ([]byte, error) {
	type Alias Bookmark
	return json.Marshal(&struct {
		*Alias
		Category    string `json:"category"`
		IsActive    bool   `json:"is_active"`
		DisplayName string `json:"display_name"`
	}{
		Alias:       (*Alias)(b),
		Category:    string(b.Category),
		IsActive:    b.IsActive(),
		DisplayName: b.GetDisplayName(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (b *Bookmark) UnmarshalJSON(data []byte) error {
	type Alias Bookmark
	aux := &struct {
		*Alias
		Category string `json:"category"`
	}{
		Alias: (*Alias)(b),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Category != "" {
		b.Category = BookmarkCategory(aux.Category)
	}
	return nil
}

// ======================================================================
= Bookmark Group (for batch operations)
// ======================================================================

// BookmarkGroup represents a group of bookmarks.
type BookmarkGroup struct {
	Bookmarks []*Bookmark `json:"bookmarks"`
	Total     int64       `json:"total"`
}

// NewBookmarkGroup creates a new bookmark group.
func NewBookmarkGroup() *BookmarkGroup {
	return &BookmarkGroup{
		Bookmarks: []*Bookmark{},
		Total:     0,
	}
}

// Add adds a bookmark to the group.
func (g *BookmarkGroup) Add(b *Bookmark) {
	g.Bookmarks = append(g.Bookmarks, b)
	g.Total++
}

// Contains checks if a bookmark is in the group.
func (g *BookmarkGroup) Contains(id string) bool {
	for _, b := range g.Bookmarks {
		if b.ID == id {
			return true
		}
	}
	return false
}

// FilterByUser returns bookmarks by a specific user.
func (g *BookmarkGroup) FilterByUser(userID string) []*Bookmark {
	result := []*Bookmark{}
	for _, b := range g.Bookmarks {
		if b.UserID == userID {
			result = append(result, b)
		}
	}
	return result
}

// FilterByTweet returns bookmarks on a specific tweet.
func (g *BookmarkGroup) FilterByTweet(tweetID string) []*Bookmark {
	result := []*Bookmark{}
	for _, b := range g.Bookmarks {
		if b.TweetID == tweetID {
			result = append(result, b)
		}
	}
	return result
}

// FilterByCategory returns bookmarks of a specific category.
func (g *BookmarkGroup) FilterByCategory(category BookmarkCategory) []*Bookmark {
	result := []*Bookmark{}
	for _, b := range g.Bookmarks {
		if b.Category == category {
			result = append(result, b)
		}
	}
	return result
}

// GetReadLater returns all read_later bookmarks.
func (g *BookmarkGroup) GetReadLater() []*Bookmark {
	return g.FilterByCategory(BookmarkCategoryReadLater)
}

// GetFavorites returns all favorites.
func (g *BookmarkGroup) GetFavorites() []*Bookmark {
	return g.FilterByCategory(BookmarkCategoryFavorites)
}

// GetImportant returns all important bookmarks.
func (g *BookmarkGroup) GetImportant() []*Bookmark {
	return g.FilterByCategory(BookmarkCategoryImportant)
}

// GetWatchLater returns all watch_later bookmarks.
func (g *BookmarkGroup) GetWatchLater() []*Bookmark {
	return g.FilterByCategory(BookmarkCategoryWatchLater)
}

// GetCustom returns all custom bookmarks.
func (g *BookmarkGroup) GetCustom() []*Bookmark {
	return g.FilterByCategory(BookmarkCategoryCustom)
}

// ======================================================================
= Bookmark Statistics
// ======================================================================

// BookmarkStats represents bookmark statistics.
type BookmarkStats struct {
	TotalBookmarks    int64            `json:"total_bookmarks"`
	CategoryStats     map[string]int64 `json:"category_stats"`
	UniqueUsers       int64            `json:"unique_users"`
	UniqueTweets      int64            `json:"unique_tweets"`
	BookmarksWithNotes int64           `json:"bookmarks_with_notes"`
}

// CalculateStats calculates statistics from a bookmark group.
func (g *BookmarkGroup) CalculateStats() *BookmarkStats {
	stats := &BookmarkStats{
		TotalBookmarks:    int64(len(g.Bookmarks)),
		CategoryStats:     make(map[string]int64),
		UniqueUsers:       0,
		UniqueTweets:      0,
		BookmarksWithNotes: 0,
	}
	users := make(map[string]bool)
	tweets := make(map[string]bool)
	for _, b := range g.Bookmarks {
		users[b.UserID] = true
		tweets[b.TweetID] = true
		stats.CategoryStats[string(b.Category)]++
		if b.HasNotes() {
			stats.BookmarksWithNotes++
		}
	}
	stats.UniqueUsers = int64(len(users))
	stats.UniqueTweets = int64(len(tweets))
	return stats
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// BookmarkBuilder helps construct bookmarks for testing.
type BookmarkBuilder struct {
	bookmark *Bookmark
}

// NewBookmarkBuilder creates a new bookmark builder.
func NewBookmarkBuilder() *BookmarkBuilder {
	return &BookmarkBuilder{
		bookmark: &Bookmark{
			ID:         uuid.New().String(),
			TweetID:    "",
			UserID:     "",
			Category:   BookmarkCategoryReadLater,
			Name:       "",
			Notes:      "",
			Metadata:   make(map[string]interface{}),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *BookmarkBuilder) WithID(id string) *BookmarkBuilder {
	b.bookmark.ID = id
	return b
}

// WithTweetID sets the tweet ID.
func (b *BookmarkBuilder) WithTweetID(tweetID string) *BookmarkBuilder {
	b.bookmark.TweetID = tweetID
	return b
}

// WithUserID sets the user ID.
func (b *BookmarkBuilder) WithUserID(userID string) *BookmarkBuilder {
	b.bookmark.UserID = userID
	return b
}

// WithCategory sets the category.
func (b *BookmarkBuilder) WithCategory(category BookmarkCategory) *BookmarkBuilder {
	b.bookmark.Category = category
	return b
}

// WithName sets the custom name.
func (b *BookmarkBuilder) WithName(name string) *BookmarkBuilder {
	b.bookmark.Name = name
	return b
}

// WithNotes sets the notes.
func (b *BookmarkBuilder) WithNotes(notes string) *BookmarkBuilder {
	b.bookmark.Notes = notes
	return b
}

// WithMetadata sets metadata.
func (b *BookmarkBuilder) WithMetadata(key string, value interface{}) *BookmarkBuilder {
	if b.bookmark.Metadata == nil {
		b.bookmark.Metadata = make(map[string]interface{})
	}
	b.bookmark.Metadata[key] = value
	return b
}

// WithCreatedAt sets the creation time.
func (b *BookmarkBuilder) WithCreatedAt(t time.Time) *BookmarkBuilder {
	b.bookmark.CreatedAt = t
	b.bookmark.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *BookmarkBuilder) WithDeleted(t time.Time) *BookmarkBuilder {
	b.bookmark.DeletedAt = &t
	return b
}

// Build validates and returns the bookmark.
func (b *BookmarkBuilder) Build() (*Bookmark, error) {
	if err := b.bookmark.Validate(); err != nil {
		return nil, err
	}
	return b.bookmark, nil
}

// MustBuild builds without error (panics on error).
func (b *BookmarkBuilder) MustBuild() *Bookmark {
	bk, err := b.Build()
	if err != nil {
		panic(err)
	}
	return bk
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestBookmark1 = MustNewBookmark("tweet1", "user1")
	TestBookmark2 = MustNewBookmarkWithCategory("tweet1", "user2", BookmarkCategoryFavorites, "", "")
	TestBookmark3 = MustNewBookmarkWithCategory("tweet2", "user1", BookmarkCategoryCustom, "My Custom", "Important stuff")
)

// MustNewBookmarkWithCategoryAndNotes creates a bookmark with category and notes.
func MustNewBookmarkWithCategoryAndNotes(tweetID, userID string, category BookmarkCategory, name, notes string) *Bookmark {
	b, err := NewBookmarkWithCategory(tweetID, userID, category, name, notes)
	if err != nil {
		panic(err)
	}
	return b
}

// MustNewDeletedBookmark creates a deleted bookmark for testing.
func MustNewDeletedBookmark(tweetID, userID string) *Bookmark {
	b, err := NewBookmark(tweetID, userID)
	if err != nil {
		panic(err)
	}
	if err := b.SoftDelete(); err != nil {
		panic(err)
	}
	return b
}

// ======================================================================
= Bookmark Collection (additional helper)
// ======================================================================

// BookmarkCollection provides advanced operations on a slice of bookmarks.
type BookmarkCollection []*Bookmark

// Len returns the number of bookmarks.
func (c BookmarkCollection) Len() int { return len(c) }

// Filter returns a new collection with bookmarks matching the predicate.
func (c BookmarkCollection) Filter(predicate func(*Bookmark) bool) BookmarkCollection {
	result := BookmarkCollection{}
	for _, b := range c {
		if predicate(b) {
			result = append(result, b)
		}
	}
	return result
}

// SortByCreatedAt sorts bookmarks by creation time (newest first).
func (c BookmarkCollection) SortByCreatedAt() {
	sort.Slice(c, func(i, j int) bool {
		return c[i].CreatedAt.After(c[j].CreatedAt)
	})
}

// SortByCategory sorts bookmarks by category.
func (c BookmarkCollection) SortByCategory() {
	sort.Slice(c, func(i, j int) bool {
		return c[i].Category < c[j].Category
	})
}

// GetByCategory returns bookmarks of a specific category.
func (c BookmarkCollection) GetByCategory(category BookmarkCategory) BookmarkCollection {
	return c.Filter(func(b *Bookmark) bool {
		return b.Category == category
	})
}

// GetByUserID returns bookmarks by a specific user.
func (c BookmarkCollection) GetByUserID(userID string) BookmarkCollection {
	return c.Filter(func(b *Bookmark) bool {
		return b.UserID == userID
	})
}

// GetByTweetID returns bookmarks on a specific tweet.
func (c BookmarkCollection) GetByTweetID(tweetID string) BookmarkCollection {
	return c.Filter(func(b *Bookmark) bool {
		return b.TweetID == tweetID
	})
}

// ToGroup converts the collection to a BookmarkGroup.
func (c BookmarkCollection) ToGroup() *BookmarkGroup {
	group := NewBookmarkGroup()
	for _, b := range c {
		group.Add(b)
	}
	return group
}