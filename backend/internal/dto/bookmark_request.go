// backend/internal/dto/bookmark_request.go
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
	MaxBookmarkNameLength  = 100
	MaxBookmarkNotesLength = 500
	MinBookmarkNameLength  = 1
	MaxBookmarksPerRequest = 100
	DefaultBookmarksLimit  = 20
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrBookmarkIDRequired      = errors.New("bookmark ID is required")
	ErrTweetIDRequired         = errors.New("tweet ID is required")
	ErrBookmarkNameRequired    = errors.New("bookmark name is required")
	ErrBookmarkNameTooLong     = fmt.Errorf("bookmark name exceeds maximum length of %d characters", MaxBookmarkNameLength)
	ErrBookmarkNameTooShort    = fmt.Errorf("bookmark name must be at least %d characters", MinBookmarkNameLength)
	ErrBookmarkNotesTooLong    = fmt.Errorf("bookmark notes exceeds maximum length of %d characters", MaxBookmarkNotesLength)
	ErrInvalidCategory         = errors.New("invalid bookmark category")
	ErrInvalidBookmarkIDs      = errors.New("one or more bookmark IDs are invalid")
	ErrBookmarksEmpty          = errors.New("bookmarks list cannot be empty")
	ErrBookmarksTooMany        = fmt.Errorf("bookmarks list exceeds maximum of %d", MaxBookmarksPerRequest)
)

// ======================================================================
// Bookmark Category Types
// ======================================================================

// BookmarkCategory represents the category of a bookmark.
type BookmarkCategory string

const (
	CategoryReadLater  BookmarkCategory = "read_later"
	CategoryFavorites  BookmarkCategory = "favorites"
	CategoryImportant  BookmarkCategory = "important"
	CategoryWatchLater BookmarkCategory = "watch_later"
	CategoryCustom     BookmarkCategory = "custom"
)

// ValidBookmarkCategories returns all valid bookmark categories.
func ValidBookmarkCategories() []BookmarkCategory {
	return []BookmarkCategory{
		CategoryReadLater,
		CategoryFavorites,
		CategoryImportant,
		CategoryWatchLater,
		CategoryCustom,
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

// String returns the string representation.
func (c BookmarkCategory) String() string {
	return string(c)
}

// ======================================================================
// Request DTOs
// ======================================================================

// CreateBookmarkRequest represents the request to create a bookmark.
type CreateBookmarkRequest struct {
	TweetID  string            `json:"tweet_id" binding:"required"`
	Category BookmarkCategory  `json:"category"`
	Name     string            `json:"name,omitempty"`
	Notes    string            `json:"notes,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the create bookmark request.
func (r *CreateBookmarkRequest) Validate() error {
	if strings.TrimSpace(r.TweetID) == "" {
		return ErrTweetIDRequired
	}
	if !r.Category.IsValid() {
		return ErrInvalidCategory
	}
	if r.Category == CategoryCustom {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			return ErrBookmarkNameRequired
		}
		if len(name) > MaxBookmarkNameLength {
			return ErrBookmarkNameTooLong
		}
		if len(name) < MinBookmarkNameLength {
			return ErrBookmarkNameTooShort
		}
	}
	if len(r.Notes) > MaxBookmarkNotesLength {
		return ErrBookmarkNotesTooLong
	}
	return nil
}

// Sanitize sanitizes the create bookmark request.
func (r *CreateBookmarkRequest) Sanitize() {
	r.TweetID = strings.TrimSpace(r.TweetID)
	if r.Category == CategoryCustom {
		r.Name = strings.TrimSpace(r.Name)
	}
	r.Notes = strings.TrimSpace(r.Notes)
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
}

// UpdateBookmarkRequest represents the request to update a bookmark.
type UpdateBookmarkRequest struct {
	ID       string            `json:"id" binding:"required"`
	Category *BookmarkCategory `json:"category,omitempty"`
	Name     *string           `json:"name,omitempty"`
	Notes    *string           `json:"notes,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the update bookmark request.
func (r *UpdateBookmarkRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrBookmarkIDRequired
	}
	if r.Category != nil && !r.Category.IsValid() {
		return ErrInvalidCategory
	}
	if r.Name != nil {
		name := strings.TrimSpace(*r.Name)
		if name == "" {
			return ErrBookmarkNameRequired
		}
		if len(name) > MaxBookmarkNameLength {
			return ErrBookmarkNameTooLong
		}
		if len(name) < MinBookmarkNameLength {
			return ErrBookmarkNameTooShort
		}
	}
	if r.Notes != nil && len(*r.Notes) > MaxBookmarkNotesLength {
		return ErrBookmarkNotesTooLong
	}
	return nil
}

// Sanitize sanitizes the update bookmark request.
func (r *UpdateBookmarkRequest) Sanitize() {
	r.ID = strings.TrimSpace(r.ID)
	if r.Name != nil {
		trimmed := strings.TrimSpace(*r.Name)
		r.Name = &trimmed
	}
	if r.Notes != nil {
		trimmed := strings.TrimSpace(*r.Notes)
		r.Notes = &trimmed
	}
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
}

// GetBookmarksRequest represents the request to list bookmarks.
type GetBookmarksRequest struct {
	UserID   string `json:"user_id,omitempty"`
	Category string `json:"category,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	SortBy   string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"`
	Search   string `json:"search,omitempty"`
}

// Validate validates the get bookmarks request.
func (r *GetBookmarksRequest) Validate() error {
	if r.Category != "" {
		category := BookmarkCategory(r.Category)
		if !category.IsValid() {
			return ErrInvalidCategory
		}
	}
	if r.Limit < 0 || r.Limit > 100 {
		return errors.New("limit must be between 0 and 100")
	}
	if r.SortBy != "" {
		allowed := map[string]bool{
			"created_at": true, "updated_at": true, "name": true,
			"category": true,
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

// Sanitize sanitizes the get bookmarks request.
func (r *GetBookmarksRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Category = strings.TrimSpace(r.Category)
	r.Cursor = strings.TrimSpace(r.Cursor)
	r.Search = strings.TrimSpace(r.Search)
	if r.Limit < 1 {
		r.Limit = DefaultBookmarksLimit
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

// DeleteBookmarkRequest represents the request to delete a bookmark.
type DeleteBookmarkRequest struct {
	ID     string   `json:"id"`
	IDs    []string `json:"ids,omitempty"`
	All    bool     `json:"all,omitempty"`
	UserID string   `json:"user_id,omitempty"`
}

// Validate validates the delete bookmark request.
func (r *DeleteBookmarkRequest) Validate() error {
	if r.ID == "" && len(r.IDs) == 0 && !r.All {
		return errors.New("either id, ids, or all must be provided")
	}
	if r.ID != "" && len(r.IDs) > 0 {
		return errors.New("cannot specify both id and ids")
	}
	if r.ID != "" && strings.TrimSpace(r.ID) == "" {
		return ErrBookmarkIDRequired
	}
	if len(r.IDs) > MaxBookmarksPerRequest {
		return ErrBookmarksTooMany
	}
	for _, id := range r.IDs {
		if strings.TrimSpace(id) == "" {
			return ErrInvalidBookmarkIDs
		}
	}
	return nil
}

// Sanitize sanitizes the delete bookmark request.
func (r *DeleteBookmarkRequest) Sanitize() {
	r.ID = strings.TrimSpace(r.ID)
	cleaned := make([]string, 0, len(r.IDs))
	for _, id := range r.IDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.IDs = cleaned
	if r.ID != "" && len(r.IDs) == 0 {
		r.IDs = []string{r.ID}
		r.ID = ""
	}
}

// BulkCreateBookmarkRequest represents the request to create multiple bookmarks.
type BulkCreateBookmarkRequest struct {
	Bookmarks []CreateBookmarkRequest `json:"bookmarks" binding:"required,dive"`
}

// Validate validates the bulk create request.
func (r *BulkCreateBookmarkRequest) Validate() error {
	if len(r.Bookmarks) == 0 {
		return ErrBookmarksEmpty
	}
	if len(r.Bookmarks) > MaxBookmarksPerRequest {
		return ErrBookmarksTooMany
	}
	for i, b := range r.Bookmarks {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("bookmark %d: %w", i, err)
		}
	}
	return nil
}

// Sanitize sanitizes the bulk create request.
func (r *BulkCreateBookmarkRequest) Sanitize() {
	for i := range r.Bookmarks {
		r.Bookmarks[i].Sanitize()
	}
}

// ======================================================================
// Response DTOs
// ======================================================================

// BookmarkResponse represents a bookmark in responses.
type BookmarkResponse struct {
	ID          string            `json:"id"`
	TweetID     string            `json:"tweet_id"`
	UserID      string            `json:"user_id"`
	Category    string            `json:"category"`
	Name        string            `json:"name,omitempty"`
	Notes       string            `json:"notes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Tweet       *TweetResponse    `json:"tweet,omitempty"`
	DisplayName string            `json:"display_name"`
	HasNotes    bool              `json:"has_notes"`
}

// BookmarkDetailResponse represents a detailed bookmark response.
type BookmarkDetailResponse struct {
	BookmarkResponse
	Tweet      TweetResponse      `json:"tweet"`
	User       MinimalUserResponse `json:"user"`
	Categories []string           `json:"categories"`
	Stats      BookmarkStatsResponse `json:"stats,omitempty"`
}

// BookmarkListResponse represents a paginated list of bookmarks.
type BookmarkListResponse struct {
	Data       []BookmarkResponse `json:"data"`
	Total      int64              `json:"total"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
	Limit      int                `json:"limit"`
}

// BookmarkStatsResponse represents bookmark statistics.
type BookmarkStatsResponse struct {
	TotalBookmarks int64            `json:"total_bookmarks"`
	CategoryStats  map[string]int64 `json:"category_stats"`
	UniqueTweets   int64            `json:"unique_tweets"`
	WithNotes      int64            `json:"with_notes"`
	LastBookmark   *time.Time       `json:"last_bookmark,omitempty"`
}

// ======================================================================
= Builder Methods for BookmarkResponse
// ======================================================================

// NewBookmarkResponse creates a new bookmark response.
func NewBookmarkResponse(id, tweetID, userID, category string) *BookmarkResponse {
	return &BookmarkResponse{
		ID:        id,
		TweetID:   tweetID,
		UserID:    userID,
		Category:  category,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Metadata:  make(map[string]string),
	}
}

// WithName sets the name.
func (r *BookmarkResponse) WithName(name string) *BookmarkResponse {
	r.Name = name
	r.DisplayName = name
	return r
}

// WithNotes sets the notes.
func (r *BookmarkResponse) WithNotes(notes string) *BookmarkResponse {
	r.Notes = notes
	r.HasNotes = notes != ""
	return r
}

// WithTweet sets the tweet.
func (r *BookmarkResponse) WithTweet(tweet *TweetResponse) *BookmarkResponse {
	r.Tweet = tweet
	return r
}

// WithMetadata sets the metadata.
func (r *BookmarkResponse) WithMetadata(key, value string) *BookmarkResponse {
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata[key] = value
	return r
}

// WithDisplayName sets the display name.
func (r *BookmarkResponse) WithDisplayName(name string) *BookmarkResponse {
	r.DisplayName = name
	return r
}

// WithCreatedAt sets the created at time.
func (r *BookmarkResponse) WithCreatedAt(t time.Time) *BookmarkResponse {
	r.CreatedAt = t
	return r
}

// WithUpdatedAt sets the updated at time.
func (r *BookmarkResponse) WithUpdatedAt(t time.Time) *BookmarkResponse {
	r.UpdatedAt = t
	return r
}

// ======================================================================
= Builder Methods for BookmarkListResponse
// ======================================================================

// NewBookmarkListResponse creates a new bookmark list response.
func NewBookmarkListResponse() *BookmarkListResponse {
	return &BookmarkListResponse{
		Data:  []BookmarkResponse{},
		Total: 0,
	}
}

// Add adds a bookmark to the response.
func (r *BookmarkListResponse) Add(bookmark BookmarkResponse) {
	r.Data = append(r.Data, bookmark)
}

// WithTotal sets the total count.
func (r *BookmarkListResponse) WithTotal(total int64) *BookmarkListResponse {
	r.Total = total
	return r
}

// WithNextCursor sets the next cursor.
func (r *BookmarkListResponse) WithNextCursor(cursor string) *BookmarkListResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *BookmarkListResponse) WithLimit(limit int) *BookmarkListResponse {
	r.Limit = limit
	return r
}

// ======================================================================
= Builder Methods for BookmarkStatsResponse
// ======================================================================

// NewBookmarkStatsResponse creates a new bookmark stats response.
func NewBookmarkStatsResponse() *BookmarkStatsResponse {
	return &BookmarkStatsResponse{
		CategoryStats: make(map[string]int64),
	}
}

// AddCategoryStat adds a category stat.
func (r *BookmarkStatsResponse) AddCategoryStat(category string, count int64) {
	r.CategoryStats[category] = count
}

// WithTotalBookmarks sets the total bookmarks.
func (r *BookmarkStatsResponse) WithTotalBookmarks(total int64) *BookmarkStatsResponse {
	r.TotalBookmarks = total
	return r
}

// WithUniqueTweets sets the unique tweets.
func (r *BookmarkStatsResponse) WithUniqueTweets(count int64) *BookmarkStatsResponse {
	r.UniqueTweets = count
	return r
}

// WithNotes sets the with notes count.
func (r *BookmarkStatsResponse) WithNotes(count int64) *BookmarkStatsResponse {
	r.WithNotes = count
	return r
}

// WithLastBookmark sets the last bookmark time.
func (r *BookmarkStatsResponse) WithLastBookmark(t time.Time) *BookmarkStatsResponse {
	r.LastBookmark = &t
	return r
}

// ======================================================================
= Conversion Helpers
// ======================================================================

// ToBookmarkResponse converts bookmark data to response.
func ToBookmarkResponse(id, tweetID, userID, category, name, notes string, createdAt, updatedAt time.Time) BookmarkResponse {
	return BookmarkResponse{
		ID:          id,
		TweetID:     tweetID,
		UserID:      userID,
		Category:    category,
		Name:        name,
		Notes:       notes,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		DisplayName: name,
		HasNotes:    notes != "",
		Metadata:    make(map[string]string),
	}
}

// ToBookmarkDetailResponse converts to a detailed response.
func ToBookmarkDetailResponse(base BookmarkResponse, tweet TweetResponse, user MinimalUserResponse, categories []string) BookmarkDetailResponse {
	return BookmarkDetailResponse{
		BookmarkResponse: base,
		Tweet:            tweet,
		User:             user,
		Categories:       categories,
	}
}

// ======================================================================
= JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *BookmarkResponse) MarshalJSON() ([]byte, error) {
	type Alias BookmarkResponse
	return json.Marshal(&struct {
		*Alias
		Category string `json:"category"`
	}{
		Alias:    (*Alias)(r),
		Category: r.Category,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *BookmarkResponse) UnmarshalJSON(data []byte) error {
	type Alias BookmarkResponse
	aux := &struct {
		*Alias
		Category string `json:"category"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Category != "" {
		r.Category = aux.Category
	}
	return nil
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestCreateBookmarkRequest creates a test create request.
func NewTestCreateBookmarkRequest() *CreateBookmarkRequest {
	return &CreateBookmarkRequest{
		TweetID:  "tweet123",
		Category: CategoryReadLater,
		Notes:    "Read this later",
	}
}

// NewTestUpdateBookmarkRequest creates a test update request.
func NewTestUpdateBookmarkRequest(id string) *UpdateBookmarkRequest {
	category := CategoryFavorites
	notes := "Updated notes"
	return &UpdateBookmarkRequest{
		ID:       id,
		Category: &category,
		Notes:    &notes,
	}
}

// NewTestGetBookmarksRequest creates a test get request.
func NewTestGetBookmarksRequest() *GetBookmarksRequest {
	return &GetBookmarksRequest{
		Limit:  20,
		SortBy: "created_at",
		SortOrder: "desc",
	}
}

// NewTestBookmarkResponse creates a test bookmark response.
func NewTestBookmarkResponse() *BookmarkResponse {
	resp := NewBookmarkResponse("bookmark1", "tweet1", "user1", "read_later")
	resp.WithName("My Bookmark").WithNotes("Important stuff")
	return resp
}

// NewTestBookmarkListResponse creates a test bookmark list response.
func NewTestBookmarkListResponse() *BookmarkListResponse {
	list := NewBookmarkListResponse()
	list.Add(*NewTestBookmarkResponse())
	list.WithTotal(1).WithNextCursor("cursor123").WithLimit(20)
	return list
}

// ======================================================================
= Validation Helpers
// ======================================================================

// ValidateBookmarkCategory validates a category string.
func ValidateBookmarkCategory(category string) bool {
	return BookmarkCategory(category).IsValid()
}

// GetValidCategories returns all valid categories as strings.
func GetValidCategories() []string {
	categories := ValidBookmarkCategories()
	strs := make([]string, len(categories))
	for i, c := range categories {
		strs[i] = string(c)
	}
	return strs
}

// ======================================================================
= Constants for API Documentation
// ======================================================================

// Bookmark constants for API documentation.
const (
	APITagBookmarks = "Bookmarks"
)