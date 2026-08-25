// backend/internal/dto/user_request.go
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
	MaxBioLength      = 160
	MaxFullNameLength = 100
	MinFullNameLength = 1
	MaxLocationLength = 100
	MaxWebsiteLength  = 200
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrFullNameRequired  = errors.New("full name is required")
	ErrFullNameTooShort  = fmt.Errorf("full name must be at least %d character", MinFullNameLength)
	ErrFullNameTooLong   = fmt.Errorf("full name exceeds maximum of %d characters", MaxFullNameLength)
	ErrBioTooLong        = fmt.Errorf("bio exceeds maximum of %d characters", MaxBioLength)
	ErrLocationTooLong   = fmt.Errorf("location exceeds maximum of %d characters", MaxLocationLength)
	ErrWebsiteInvalid    = errors.New("invalid website URL")
	ErrWebsiteTooLong    = fmt.Errorf("website exceeds maximum of %d characters", MaxWebsiteLength)
	ErrInvalidAvatarURL  = errors.New("invalid avatar URL")
	ErrInvalidBannerURL  = errors.New("invalid banner URL")
)

// ======================================================================
// Request DTOs
// ======================================================================

// UpdateProfileRequest represents the request to update a user's profile.
type UpdateProfileRequest struct {
	FullName    string  `json:"full_name,omitempty"`
	Bio         string  `json:"bio,omitempty"`
	Location    string  `json:"location,omitempty"`
	Website     string  `json:"website,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	BannerURL   *string `json:"banner_url,omitempty"`
}

// Validate validates the update profile request.
func (r *UpdateProfileRequest) Validate() error {
	if r.FullName != "" {
		fullName := strings.TrimSpace(r.FullName)
		if fullName == "" {
			return ErrFullNameRequired
		}
		if len(fullName) < MinFullNameLength {
			return ErrFullNameTooShort
		}
		if len(fullName) > MaxFullNameLength {
			return ErrFullNameTooLong
		}
		r.FullName = fullName
	}
	if len(r.Bio) > MaxBioLength {
		return ErrBioTooLong
	}
	if len(r.Location) > MaxLocationLength {
		return ErrLocationTooLong
	}
	if r.Website != "" {
		if len(r.Website) > MaxWebsiteLength {
			return ErrWebsiteTooLong
		}
		if !isValidURL(r.Website) {
			return ErrWebsiteInvalid
		}
	}
	if r.AvatarURL != nil && *r.AvatarURL != "" && !isValidURL(*r.AvatarURL) {
		return ErrInvalidAvatarURL
	}
	if r.BannerURL != nil && *r.BannerURL != "" && !isValidURL(*r.BannerURL) {
		return ErrInvalidBannerURL
	}
	return nil
}

// Sanitize sanitizes the update profile request.
func (r *UpdateProfileRequest) Sanitize() {
	if r.FullName != "" {
		r.FullName = strings.TrimSpace(r.FullName)
	}
	r.Bio = strings.TrimSpace(r.Bio)
	r.Location = strings.TrimSpace(r.Location)
	r.Website = strings.TrimSpace(r.Website)
	if r.AvatarURL != nil {
		trimmed := strings.TrimSpace(*r.AvatarURL)
		r.AvatarURL = &trimmed
	}
	if r.BannerURL != nil {
		trimmed := strings.TrimSpace(*r.BannerURL)
		r.BannerURL = &trimmed
	}
}

// UpdateSettingsRequest represents the request to update user settings.
type UpdateSettingsRequest struct {
	Theme        string            `json:"theme,omitempty"`
	Language     string            `json:"language,omitempty"`
	Timezone     string            `json:"timezone,omitempty"`
	Notifications map[string]bool  `json:"notifications,omitempty"`
	Privacy      map[string]bool   `json:"privacy,omitempty"`
}

// Validate validates the update settings request.
func (r *UpdateSettingsRequest) Validate() error {
	if r.Theme != "" {
		validThemes := map[string]bool{"light": true, "dark": true, "system": true}
		if !validThemes[r.Theme] {
			return errors.New("invalid theme")
		}
	}
	return nil
}

// Sanitize sanitizes the update settings request.
func (r *UpdateSettingsRequest) Sanitize() {
	r.Theme = strings.TrimSpace(r.Theme)
	r.Language = strings.TrimSpace(r.Language)
	r.Timezone = strings.TrimSpace(r.Timezone)
	if r.Notifications == nil {
		r.Notifications = make(map[string]bool)
	}
	if r.Privacy == nil {
		r.Privacy = make(map[string]bool)
	}
}

// SearchUsersRequest represents the request to search users.
type SearchUsersRequest struct {
	Query      string `json:"q" binding:"required"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	SortOrder  string `json:"sort_order,omitempty"`
	IncludeInactive bool `json:"include_inactive,omitempty"`
}

// Validate validates the search users request.
func (r *SearchUsersRequest) Validate() error {
	if strings.TrimSpace(r.Query) == "" {
		return ErrSearchQueryEmpty
	}
	if r.Limit < 0 || r.Limit > MaxSearchResultsLimit {
		return ErrInvalidLimit
	}
	if r.Offset < 0 {
		return errors.New("offset cannot be negative")
	}
	if r.SortBy != "" {
		allowed := map[string]bool{
			"username": true, "full_name": true, "joined_at": true,
			"follower_count": true, "tweet_count": true,
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

// Sanitize sanitizes the search users request.
func (r *SearchUsersRequest) Sanitize() {
	r.Query = strings.TrimSpace(r.Query)
	if r.Limit < 1 {
		r.Limit = DefaultSearchLimit
	}
	if r.Limit > MaxSearchResultsLimit {
		r.Limit = MaxSearchResultsLimit
	}
	if r.Offset < 0 {
		r.Offset = 0
	}
}

// GetUserSuggestionsRequest represents the request to get user suggestions.
type GetUserSuggestionsRequest struct {
	Limit      int      `json:"limit,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
	Algorithm  string   `json:"algorithm,omitempty"`
}

// Validate validates the get user suggestions request.
func (r *GetUserSuggestionsRequest) Validate() error {
	if r.Limit < 0 || r.Limit > MaxSuggestionsLimit {
		return errors.New("limit must be between 1 and 50")
	}
	if r.Algorithm != "" {
		validAlgorithms := map[string]bool{
			"mutual": true, "popular": true, "random": true, "ai": true,
		}
		if !validAlgorithms[r.Algorithm] {
			return errors.New("invalid algorithm")
		}
	}
	return nil
}

// Sanitize sanitizes the get user suggestions request.
func (r *GetUserSuggestionsRequest) Sanitize() {
	if r.Limit < 1 {
		r.Limit = DefaultSuggestionsLimit
	}
	if r.Limit > MaxSuggestionsLimit {
		r.Limit = MaxSuggestionsLimit
	}
	cleaned := make([]string, 0, len(r.Exclude))
	for _, id := range r.Exclude {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.Exclude = cleaned
	if r.Algorithm == "" {
		r.Algorithm = "mutual"
	}
}

// UpdateUserRequest represents the request for admin user updates.
type UpdateUserRequest struct {
	ID         string  `json:"id" binding:"required"`
	Username   *string `json:"username,omitempty"`
	Email      *string `json:"email,omitempty"`
	FullName   *string `json:"full_name,omitempty"`
	Bio        *string `json:"bio,omitempty"`
	AvatarURL  *string `json:"avatar_url,omitempty"`
	BannerURL  *string `json:"banner_url,omitempty"`
	Location   *string `json:"location,omitempty"`
	Website    *string `json:"website,omitempty"`
	IsVerified *bool   `json:"is_verified,omitempty"`
	IsPrivate  *bool   `json:"is_private,omitempty"`
}

// Validate validates the update user request.
func (r *UpdateUserRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrUserIDRequired
	}
	if r.Username != nil {
		if err := ValidateUsername(*r.Username); err != nil {
			return err
		}
	}
	if r.Email != nil {
		if err := ValidateEmail(*r.Email); err != nil {
			return err
		}
	}
	if r.FullName != nil && len(*r.FullName) > MaxFullNameLength {
		return ErrFullNameTooLong
	}
	if r.Bio != nil && len(*r.Bio) > MaxBioLength {
		return ErrBioTooLong
	}
	if r.AvatarURL != nil && *r.AvatarURL != "" && !isValidURL(*r.AvatarURL) {
		return ErrInvalidAvatarURL
	}
	if r.BannerURL != nil && *r.BannerURL != "" && !isValidURL(*r.BannerURL) {
		return ErrInvalidBannerURL
	}
	return nil
}

// Sanitize sanitizes the update user request.
func (r *UpdateUserRequest) Sanitize() {
	r.ID = strings.TrimSpace(r.ID)
	if r.Username != nil {
		trimmed := strings.TrimSpace(*r.Username)
		r.Username = &trimmed
	}
	if r.Email != nil {
		trimmed := strings.TrimSpace(*r.Email)
		r.Email = &trimmed
	}
	if r.FullName != nil {
		trimmed := strings.TrimSpace(*r.FullName)
		r.FullName = &trimmed
	}
	if r.Bio != nil {
		trimmed := strings.TrimSpace(*r.Bio)
		r.Bio = &trimmed
	}
	if r.AvatarURL != nil {
		trimmed := strings.TrimSpace(*r.AvatarURL)
		r.AvatarURL = &trimmed
	}
	if r.BannerURL != nil {
		trimmed := strings.TrimSpace(*r.BannerURL)
		r.BannerURL = &trimmed
	}
	if r.Location != nil {
		trimmed := strings.TrimSpace(*r.Location)
		r.Location = &trimmed
	}
	if r.Website != nil {
		trimmed := strings.TrimSpace(*r.Website)
		r.Website = &trimmed
	}
}

// ======================================================================
= Helper Functions
// ======================================================================

// isValidURL validates a URL.
func isValidURL(urlStr string) bool {
	return strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://")
}

// ValidateUsername validates a username (re-exported from utils).
func ValidateUsername(username string) error {
	if len(username) < MinUsernameLength {
		return ErrUsernameTooShort
	}
	if len(username) > MaxUsernameLength {
		return ErrUsernameTooLong
	}
	return nil
}

// ValidateEmail validates an email (re-exported from utils).
func ValidateEmail(email string) error {
	if email == "" {
		return ErrEmailEmpty
	}
	if !strings.Contains(email, "@") {
		return ErrEmailInvalid
	}
	return nil
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestUpdateProfileRequest creates a test update profile request.
func NewTestUpdateProfileRequest() *UpdateProfileRequest {
	return &UpdateProfileRequest{
		FullName: "John Doe Updated",
		Bio:      "Updated bio",
		Location: "New York",
		Website:  "https://johndoe.com",
	}
}

// NewTestSearchUsersRequest creates a test search users request.
func NewTestSearchUsersRequest() *SearchUsersRequest {
	return &SearchUsersRequest{
		Query:  "john",
		Limit:  20,
		SortBy: "username",
	}
}

// NewTestUpdateSettingsRequest creates a test update settings request.
func NewTestUpdateSettingsRequest() *UpdateSettingsRequest {
	return &UpdateSettingsRequest{
		Theme:    "dark",
		Language: "en",
		Timezone: "UTC",
		Notifications: map[string]bool{
			"likes": true, "retweets": true, "follows": true,
		},
	}
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagUsers = "Users"
)