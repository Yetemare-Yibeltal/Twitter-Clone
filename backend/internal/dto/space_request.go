// backend/internal/dto/space_request.go
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
	MaxSpaceTitleLength       = 100
	MinSpaceTitleLength       = 3
	MaxSpaceDescriptionLength = 500
	MaxSpaceTopicLength       = 100
	MaxSpaceScheduledDuration = 8 * 60 // 8 hours in minutes
	MinSpaceScheduledDuration = 5      // 5 minutes
	DefaultSpaceLimit         = 20
	MaxSpaceLimit             = 100
	MaxSpeakersPerSpace       = 10
	MaxListenersPerSpace      = 1000
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrSpaceIDRequired          = errors.New("space ID is required")
	ErrSpaceTitleRequired       = errors.New("space title is required")
	ErrSpaceTitleTooShort       = fmt.Errorf("space title must be at least %d characters", MinSpaceTitleLength)
	ErrSpaceTitleTooLong        = fmt.Errorf("space title exceeds maximum of %d characters", MaxSpaceTitleLength)
	ErrSpaceDescriptionTooLong  = fmt.Errorf("space description exceeds maximum of %d characters", MaxSpaceDescriptionLength)
	ErrSpaceTopicTooLong        = fmt.Errorf("space topic exceeds maximum of %d characters", MaxSpaceTopicLength)
	ErrInvalidSpaceStatus       = errors.New("invalid space status")
	ErrInvalidSpaceVisibility   = errors.New("invalid space visibility")
	ErrInvalidSpaceType         = errors.New("invalid space type")
	ErrUserIDRequired           = errors.New("user ID is required")
	ErrSpaceDurationInvalid     = fmt.Errorf("space duration must be between %d and %d minutes", MinSpaceScheduledDuration, MaxSpaceScheduledDuration)
	ErrSpaceSchedulePast        = errors.New("scheduled time cannot be in the past")
	ErrSpaceAlreadyStarted      = errors.New("space has already started")
	ErrSpaceAlreadyEnded        = errors.New("space has already ended")
	ErrSpaceNotFound            = errors.New("space not found")
	ErrUserNotInSpace           = errors.New("user is not in this space")
	ErrUserAlreadyInSpace       = errors.New("user is already in this space")
	ErrUserNotSpeaker           = errors.New("user is not a speaker in this space")
	ErrUserAlreadySpeaker       = errors.New("user is already a speaker in this space")
	ErrMaxSpeakersReached       = fmt.Errorf("maximum speakers (%d) reached", MaxSpeakersPerSpace)
	ErrMaxListenersReached      = fmt.Errorf("maximum listeners (%d) reached", MaxListenersPerSpace)
	ErrSpaceFull                = errors.New("space has reached maximum capacity")
	ErrInvalidLimit             = errors.New("limit must be between 1 and 100")
	ErrInvalidCursor            = errors.New("invalid cursor format")
)

// ======================================================================
// Space Types and Status
// ======================================================================

// SpaceStatus represents the status of a space.
type SpaceStatus string

const (
	SpaceStatusScheduled SpaceStatus = "scheduled"
	SpaceStatusLive      SpaceStatus = "live"
	SpaceStatusEnded     SpaceStatus = "ended"
	SpaceStatusCancelled SpaceStatus = "cancelled"
)

// ValidSpaceStatuses returns all valid space statuses.
func ValidSpaceStatuses() []SpaceStatus {
	return []SpaceStatus{
		SpaceStatusScheduled,
		SpaceStatusLive,
		SpaceStatusEnded,
		SpaceStatusCancelled,
	}
}

// IsValid checks if a space status is valid.
func (s SpaceStatus) IsValid() bool {
	for _, status := range ValidSpaceStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (s SpaceStatus) String() string {
	return string(s)
}

// SpaceVisibility represents the visibility of a space.
type SpaceVisibility string

const (
	VisibilityPublic  SpaceVisibility = "public"
	VisibilityPrivate SpaceVisibility = "private"
	VisibilityUnlisted SpaceVisibility = "unlisted"
)

// ValidVisibilities returns all valid visibility values.
func ValidVisibilities() []SpaceVisibility {
	return []SpaceVisibility{
		VisibilityPublic,
		VisibilityPrivate,
		VisibilityUnlisted,
	}
}

// IsValid checks if a visibility value is valid.
func (v SpaceVisibility) IsValid() bool {
	for _, vis := range ValidVisibilities() {
		if v == vis {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (v SpaceVisibility) String() string {
	return string(v)
}

// SpaceType represents the type of space.
type SpaceType string

const (
	SpaceTypeOpen      SpaceType = "open"
	SpaceTypeInviteOnly SpaceType = "invite_only"
	SpaceTypeTicketed  SpaceType = "ticketed"
	SpaceTypeClub      SpaceType = "club"
)

// ValidSpaceTypes returns all valid space types.
func ValidSpaceTypes() []SpaceType {
	return []SpaceType{
		SpaceTypeOpen,
		SpaceTypeInviteOnly,
		SpaceTypeTicketed,
		SpaceTypeClub,
	}
}

// IsValid checks if a space type is valid.
func (t SpaceType) IsValid() bool {
	for _, typ := range ValidSpaceTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (t SpaceType) String() string {
	return string(t)
}

// SpeakerStatus represents the status of a speaker in a space.
type SpeakerStatus string

const (
	SpeakerStatusInvited    SpeakerStatus = "invited"
	SpeakerStatusAccepted   SpeakerStatus = "accepted"
	SpeakerStatusSpeaking   SpeakerStatus = "speaking"
	SpeakerStatusMuted      SpeakerStatus = "muted"
	SpeakerStatusHandRaised SpeakerStatus = "hand_raised"
	SpeakerStatusLeft       SpeakerStatus = "left"
)

// ValidSpeakerStatuses returns all valid speaker statuses.
func ValidSpeakerStatuses() []SpeakerStatus {
	return []SpeakerStatus{
		SpeakerStatusInvited,
		SpeakerStatusAccepted,
		SpeakerStatusSpeaking,
		SpeakerStatusMuted,
		SpeakerStatusHandRaised,
		SpeakerStatusLeft,
	}
}

// IsValid checks if a speaker status is valid.
func (s SpeakerStatus) IsValid() bool {
	for _, status := range ValidSpeakerStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (s SpeakerStatus) String() string {
	return string(s)
}

// ======================================================================
// Request DTOs
// ======================================================================

// CreateSpaceRequest represents the request to create a space.
type CreateSpaceRequest struct {
	Title       string            `json:"title" binding:"required"`
	Description string            `json:"description,omitempty"`
	Topic       string            `json:"topic,omitempty"`
	Visibility  SpaceVisibility   `json:"visibility"`
	Type        SpaceType         `json:"type"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	Duration    int               `json:"duration"` // minutes
	MaxListeners int              `json:"max_listeners,omitempty"`
	InviteOnly  bool              `json:"invite_only,omitempty"`
	InvitedUsers []string         `json:"invited_users,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Validate validates the create space request.
func (r *CreateSpaceRequest) Validate() error {
	title := strings.TrimSpace(r.Title)
	if title == "" {
		return ErrSpaceTitleRequired
	}
	if len(title) < MinSpaceTitleLength {
		return ErrSpaceTitleTooShort
	}
	if len(title) > MaxSpaceTitleLength {
		return ErrSpaceTitleTooLong
	}
	r.Title = title
	if len(r.Description) > MaxSpaceDescriptionLength {
		return ErrSpaceDescriptionTooLong
	}
	if len(r.Topic) > MaxSpaceTopicLength {
		return ErrSpaceTopicTooLong
	}
	if !r.Visibility.IsValid() {
		return ErrInvalidSpaceVisibility
	}
	if !r.Type.IsValid() {
		return ErrInvalidSpaceType
	}
	if r.Duration < MinSpaceScheduledDuration || r.Duration > MaxSpaceScheduledDuration {
		return ErrSpaceDurationInvalid
	}
	if r.ScheduledAt != nil && r.ScheduledAt.Before(time.Now()) {
		return ErrSpaceSchedulePast
	}
	if r.MaxListeners > MaxListenersPerSpace {
		return ErrMaxListenersReached
	}
	if r.MaxListeners < 0 {
		return errors.New("max_listeners cannot be negative")
	}
	if r.InviteOnly && r.Type == SpaceTypeOpen {
		return errors.New("open space cannot be invite-only")
	}
	for _, u := range r.InvitedUsers {
		if strings.TrimSpace(u) == "" {
			return errors.New("invited user ID cannot be empty")
		}
	}
	return nil
}

// Sanitize sanitizes the create space request.
func (r *CreateSpaceRequest) Sanitize() {
	r.Title = strings.TrimSpace(r.Title)
	r.Description = strings.TrimSpace(r.Description)
	r.Topic = strings.TrimSpace(r.Topic)
	if r.Visibility == "" {
		r.Visibility = VisibilityPublic
	}
	if r.Type == "" {
		r.Type = SpaceTypeOpen
	}
	if r.Duration < 1 {
		r.Duration = 30 // default 30 minutes
	}
	cleaned := make([]string, 0, len(r.InvitedUsers))
	for _, u := range r.InvitedUsers {
		if trimmed := strings.TrimSpace(u); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.InvitedUsers = cleaned
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
}

// UpdateSpaceRequest represents the request to update a space.
type UpdateSpaceRequest struct {
	ID          string            `json:"id" binding:"required"`
	Title       *string           `json:"title,omitempty"`
	Description *string           `json:"description,omitempty"`
	Topic       *string           `json:"topic,omitempty"`
	Visibility  *SpaceVisibility  `json:"visibility,omitempty"`
	Type        *SpaceType        `json:"type,omitempty"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	Duration    *int              `json:"duration,omitempty"`
	MaxListeners *int             `json:"max_listeners,omitempty"`
	Status      *SpaceStatus      `json:"status,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Validate validates the update space request.
func (r *UpdateSpaceRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrSpaceIDRequired
	}
	if r.Title != nil {
		title := strings.TrimSpace(*r.Title)
		if title == "" {
			return ErrSpaceTitleRequired
		}
		if len(title) < MinSpaceTitleLength {
			return ErrSpaceTitleTooShort
		}
		if len(title) > MaxSpaceTitleLength {
			return ErrSpaceTitleTooLong
		}
	}
	if r.Description != nil && len(*r.Description) > MaxSpaceDescriptionLength {
		return ErrSpaceDescriptionTooLong
	}
	if r.Topic != nil && len(*r.Topic) > MaxSpaceTopicLength {
		return ErrSpaceTopicTooLong
	}
	if r.Visibility != nil && !r.Visibility.IsValid() {
		return ErrInvalidSpaceVisibility
	}
	if r.Type != nil && !r.Type.IsValid() {
		return ErrInvalidSpaceType
	}
	if r.ScheduledAt != nil && r.ScheduledAt.Before(time.Now()) {
		return ErrSpaceSchedulePast
	}
	if r.Duration != nil && (*r.Duration < MinSpaceScheduledDuration || *r.Duration > MaxSpaceScheduledDuration) {
		return ErrSpaceDurationInvalid
	}
	if r.MaxListeners != nil && *r.MaxListeners > MaxListenersPerSpace {
		return ErrMaxListenersReached
	}
	if r.Status != nil && !r.Status.IsValid() {
		return ErrInvalidSpaceStatus
	}
	return nil
}

// Sanitize sanitizes the update space request.
func (r *UpdateSpaceRequest) Sanitize() {
	r.ID = strings.TrimSpace(r.ID)
	if r.Title != nil {
		trimmed := strings.TrimSpace(*r.Title)
		r.Title = &trimmed
	}
	if r.Description != nil {
		trimmed := strings.TrimSpace(*r.Description)
		r.Description = &trimmed
	}
	if r.Topic != nil {
		trimmed := strings.TrimSpace(*r.Topic)
		r.Topic = &trimmed
	}
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
}

// GetSpacesRequest represents the request to list spaces.
type GetSpacesRequest struct {
	UserID      string `json:"user_id,omitempty"`
	Status      string `json:"status,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Type        string `json:"type,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	SortBy      string `json:"sort_by,omitempty"`
	SortOrder   string `json:"sort_order,omitempty"`
	IncludePast bool   `json:"include_past,omitempty"`
	Search      string `json:"search,omitempty"`
}

// Validate validates the get spaces request.
func (r *GetSpacesRequest) Validate() error {
	if r.Status != "" && !SpaceStatus(r.Status).IsValid() {
		return ErrInvalidSpaceStatus
	}
	if r.Visibility != "" && !SpaceVisibility(r.Visibility).IsValid() {
		return ErrInvalidSpaceVisibility
	}
	if r.Type != "" && !SpaceType(r.Type).IsValid() {
		return ErrInvalidSpaceType
	}
	if r.Limit < 0 || r.Limit > MaxSpaceLimit {
		return ErrInvalidLimit
	}
	if r.SortBy != "" {
		allowed := map[string]bool{
			"created_at": true, "scheduled_at": true, "title": true,
			"listener_count": true, "speaker_count": true,
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

// Sanitize sanitizes the get spaces request.
func (r *GetSpacesRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Status = strings.TrimSpace(r.Status)
	r.Visibility = strings.TrimSpace(r.Visibility)
	r.Type = strings.TrimSpace(r.Type)
	r.Cursor = strings.TrimSpace(r.Cursor)
	r.Search = strings.TrimSpace(r.Search)
	if r.Limit < 1 {
		r.Limit = DefaultSpaceLimit
	}
	if r.Limit > MaxSpaceLimit {
		r.Limit = MaxSpaceLimit
	}
	if r.SortBy != "" {
		r.SortBy = strings.ToLower(strings.TrimSpace(r.SortBy))
	}
	if r.SortOrder != "" {
		r.SortOrder = strings.ToLower(strings.TrimSpace(r.SortOrder))
	}
}

// JoinSpaceRequest represents the request to join a space.
type JoinSpaceRequest struct {
	SpaceID string `json:"space_id" binding:"required"`
	UserID  string `json:"user_id,omitempty"`
	AsSpeaker bool `json:"as_speaker,omitempty"`
	InviteCode string `json:"invite_code,omitempty"`
}

// Validate validates the join space request.
func (r *JoinSpaceRequest) Validate() error {
	if strings.TrimSpace(r.SpaceID) == "" {
		return ErrSpaceIDRequired
	}
	return nil
}

// Sanitize sanitizes the join space request.
func (r *JoinSpaceRequest) Sanitize() {
	r.SpaceID = strings.TrimSpace(r.SpaceID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.InviteCode = strings.TrimSpace(r.InviteCode)
}

// LeaveSpaceRequest represents the request to leave a space.
type LeaveSpaceRequest struct {
	SpaceID string `json:"space_id" binding:"required"`
	UserID  string `json:"user_id,omitempty"`
}

// Validate validates the leave space request.
func (r *LeaveSpaceRequest) Validate() error {
	if strings.TrimSpace(r.SpaceID) == "" {
		return ErrSpaceIDRequired
	}
	return nil
}

// Sanitize sanitizes the leave space request.
func (r *LeaveSpaceRequest) Sanitize() {
	r.SpaceID = strings.TrimSpace(r.SpaceID)
	r.UserID = strings.TrimSpace(r.UserID)
}

// SpeakerRequest represents speaker management in a space.
type SpeakerRequest struct {
	SpaceID   string        `json:"space_id" binding:"required"`
	UserID    string        `json:"user_id" binding:"required"`
	Action    string        `json:"action" binding:"required"` // "invite", "accept", "remove", "mute", "unmute", "hand_raise", "speak"
	Status    SpeakerStatus `json:"status,omitempty"`
}

// Validate validates the speaker request.
func (r *SpeakerRequest) Validate() error {
	if strings.TrimSpace(r.SpaceID) == "" {
		return ErrSpaceIDRequired
	}
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	validActions := map[string]bool{
		"invite": true, "accept": true, "remove": true,
		"mute": true, "unmute": true, "hand_raise": true, "speak": true,
	}
	if !validActions[r.Action] {
		return errors.New("invalid speaker action")
	}
	if r.Status != "" && !r.Status.IsValid() {
		return errors.New("invalid speaker status")
	}
	return nil
}

// Sanitize sanitizes the speaker request.
func (r *SpeakerRequest) Sanitize() {
	r.SpaceID = strings.TrimSpace(r.SpaceID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.Action = strings.TrimSpace(r.Action)
}

// EndSpaceRequest represents the request to end a space.
type EndSpaceRequest struct {
	SpaceID string `json:"space_id" binding:"required"`
	UserID  string `json:"user_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Validate validates the end space request.
func (r *EndSpaceRequest) Validate() error {
	if strings.TrimSpace(r.SpaceID) == "" {
		return ErrSpaceIDRequired
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	return nil
}

// Sanitize sanitizes the end space request.
func (r *EndSpaceRequest) Sanitize() {
	r.SpaceID = strings.TrimSpace(r.SpaceID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.Reason = strings.TrimSpace(r.Reason)
}

// GetSpaceSpeakersRequest represents the request to get speakers in a space.
type GetSpaceSpeakersRequest struct {
	SpaceID string `json:"space_id" binding:"required"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Status  string `json:"status,omitempty"`
}

// Validate validates the get space speakers request.
func (r *GetSpaceSpeakersRequest) Validate() error {
	if strings.TrimSpace(r.SpaceID) == "" {
		return ErrSpaceIDRequired
	}
	if r.Status != "" && !SpeakerStatus(r.Status).IsValid() {
		return errors.New("invalid speaker status")
	}
	if r.Limit < 0 || r.Limit > MaxSpaceLimit {
		return ErrInvalidLimit
	}
	return nil
}

// Sanitize sanitizes the get space speakers request.
func (r *GetSpaceSpeakersRequest) Sanitize() {
	r.SpaceID = strings.TrimSpace(r.SpaceID)
	r.Cursor = strings.TrimSpace(r.Cursor)
	r.Status = strings.TrimSpace(r.Status)
	if r.Limit < 1 {
		r.Limit = DefaultSpaceLimit
	}
	if r.Limit > MaxSpaceLimit {
		r.Limit = MaxSpaceLimit
	}
}

// GetSpaceListenersRequest represents the request to get listeners in a space.
type GetSpaceListenersRequest struct {
	SpaceID string `json:"space_id" binding:"required"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// Validate validates the get space listeners request.
func (r *GetSpaceListenersRequest) Validate() error {
	if strings.TrimSpace(r.SpaceID) == "" {
		return ErrSpaceIDRequired
	}
	if r.Limit < 0 || r.Limit > MaxSpaceLimit {
		return ErrInvalidLimit
	}
	return nil
}

// Sanitize sanitizes the get space listeners request.
func (r *GetSpaceListenersRequest) Sanitize() {
	r.SpaceID = strings.TrimSpace(r.SpaceID)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultSpaceLimit
	}
	if r.Limit > MaxSpaceLimit {
		r.Limit = MaxSpaceLimit
	}
}

// ======================================================================
// Response DTOs
// ======================================================================

// SpaceResponse represents a space in responses.
type SpaceResponse struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description,omitempty"`
	Topic        string            `json:"topic,omitempty"`
	CreatedBy    string            `json:"created_by"`
	Visibility   string            `json:"visibility"`
	Type         string            `json:"type"`
	Status       string            `json:"status"`
	ScheduledAt  *time.Time        `json:"scheduled_at,omitempty"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	EndedAt      *time.Time        `json:"ended_at,omitempty"`
	Duration     int               `json:"duration"`
	SpeakerCount int               `json:"speaker_count"`
	ListenerCount int              `json:"listener_count"`
	MaxListeners int               `json:"max_listeners"`
	IsJoined     bool              `json:"is_joined"`
	IsSpeaker    bool              `json:"is_speaker"`
	IsCreator    bool              `json:"is_creator"`
	InviteCode   string            `json:"invite_code,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// SpaceDetailResponse represents a detailed space response.
type SpaceDetailResponse struct {
	SpaceResponse
	Creator      *MinimalUserResponse  `json:"creator,omitempty"`
	Speakers     []SpeakerResponse     `json:"speakers,omitempty"`
	Listeners    []MinimalUserResponse `json:"listeners,omitempty"`
	InvitedUsers []MinimalUserResponse `json:"invited_users,omitempty"`
	Stats        SpaceStatsResponse    `json:"stats,omitempty"`
}

// SpeakerResponse represents a speaker in a space.
type SpeakerResponse struct {
	UserID     string        `json:"user_id"`
	Username   string        `json:"username"`
	FullName   string        `json:"full_name"`
	AvatarURL  string        `json:"avatar_url,omitempty"`
	Status     SpeakerStatus `json:"status"`
	JoinedAt   time.Time     `json:"joined_at"`
	IsSpeaking bool          `json:"is_speaking"`
	IsMuted    bool          `json:"is_muted"`
	IsHost     bool          `json:"is_host"`
}

// SpaceListResponse represents a paginated list of spaces.
type SpaceListResponse struct {
	Data       []SpaceResponse `json:"data"`
	Total      int64           `json:"total"`
	NextCursor string          `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
	Limit      int             `json:"limit"`
}

// SpaceStatsResponse represents space statistics.
type SpaceStatsResponse struct {
	TotalSpaces   int64 `json:"total_spaces"`
	ActiveSpaces  int64 `json:"active_spaces"`
	ScheduledSpaces int64 `json:"scheduled_spaces"`
	EndedSpaces   int64 `json:"ended_spaces"`
	TotalSpeakers int64 `json:"total_speakers"`
	TotalListeners int64 `json:"total_listeners"`
	AvgDuration   float64 `json:"avg_duration"`
	MaxConcurrent int64 `json:"max_concurrent"`
}

// SpaceParticipantResponse represents a space participant.
type SpaceParticipantResponse struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
	Role       string    `json:"role"` // "speaker", "listener"
	JoinedAt   time.Time `json:"joined_at"`
	LeftAt     *time.Time `json:"left_at,omitempty"`
}

// SpaceParticipantListResponse represents a paginated list of participants.
type SpaceParticipantListResponse struct {
	Data       []SpaceParticipantResponse `json:"data"`
	Total      int64                      `json:"total"`
	NextCursor string                     `json:"next_cursor"`
	HasMore    bool                       `json:"has_more"`
	Limit      int                        `json:"limit"`
}

// ======================================================================
// Builder Methods for SpaceResponse
// ======================================================================

// NewSpaceResponse creates a new space response.
func NewSpaceResponse(id, title, createdBy, visibility, spaceType string) *SpaceResponse {
	return &SpaceResponse{
		ID:          id,
		Title:       title,
		CreatedBy:   createdBy,
		Visibility:  visibility,
		Type:        spaceType,
		Status:      string(SpaceStatusScheduled),
		Duration:    30,
		MaxListeners: MaxListenersPerSpace,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

// WithDescription sets the description.
func (r *SpaceResponse) WithDescription(desc string) *SpaceResponse {
	r.Description = desc
	return r
}

// WithTopic sets the topic.
func (r *SpaceResponse) WithTopic(topic string) *SpaceResponse {
	r.Topic = topic
	return r
}

// WithStatus sets the status.
func (r *SpaceResponse) WithStatus(status string) *SpaceResponse {
	r.Status = status
	return r
}

// WithScheduledAt sets the scheduled time.
func (r *SpaceResponse) WithScheduledAt(t time.Time) *SpaceResponse {
	r.ScheduledAt = &t
	return r
}

// WithStartedAt sets the started time.
func (r *SpaceResponse) WithStartedAt(t time.Time) *SpaceResponse {
	r.StartedAt = &t
	return r
}

// WithEndedAt sets the ended time.
func (r *SpaceResponse) WithEndedAt(t time.Time) *SpaceResponse {
	r.EndedAt = &t
	return r
}

// WithSpeakerCount sets the speaker count.
func (r *SpaceResponse) WithSpeakerCount(count int) *SpaceResponse {
	r.SpeakerCount = count
	return r
}

// WithListenerCount sets the listener count.
func (r *SpaceResponse) WithListenerCount(count int) *SpaceResponse {
	r.ListenerCount = count
	return r
}

// WithIsJoined sets the is joined flag.
func (r *SpaceResponse) WithIsJoined(joined bool) *SpaceResponse {
	r.IsJoined = joined
	return r
}

// WithIsSpeaker sets the is speaker flag.
func (r *SpaceResponse) WithIsSpeaker(speaker bool) *SpaceResponse {
	r.IsSpeaker = speaker
	return r
}

// WithIsCreator sets the is creator flag.
func (r *SpaceResponse) WithIsCreator(creator bool) *SpaceResponse {
	r.IsCreator = creator
	return r
}

// WithInviteCode sets the invite code.
func (r *SpaceResponse) WithInviteCode(code string) *SpaceResponse {
	r.InviteCode = code
	return r
}

// WithMetadata sets the metadata.
func (r *SpaceResponse) WithMetadata(metadata map[string]string) *SpaceResponse {
	r.Metadata = metadata
	return r
}

// ======================================================================
// Builder Methods for SpeakerResponse
// ======================================================================

// NewSpeakerResponse creates a new speaker response.
func NewSpeakerResponse(userID, username, fullName string, status SpeakerStatus) *SpeakerResponse {
	return &SpeakerResponse{
		UserID:     userID,
		Username:   username,
		FullName:   fullName,
		Status:     status,
		JoinedAt:   time.Now().UTC(),
		IsSpeaking: status == SpeakerStatusSpeaking,
		IsMuted:    status == SpeakerStatusMuted,
	}
}

// WithAvatarURL sets the avatar URL.
func (r *SpeakerResponse) WithAvatarURL(url string) *SpeakerResponse {
	r.AvatarURL = url
	return r
}

// WithIsHost sets the is host flag.
func (r *SpeakerResponse) WithIsHost(host bool) *SpeakerResponse {
	r.IsHost = host
	return r
}

// WithJoinedAt sets the joined at time.
func (r *SpeakerResponse) WithJoinedAt(t time.Time) *SpeakerResponse {
	r.JoinedAt = t
	return r
}

// ======================================================================
// Builder Methods for SpaceListResponse
// ======================================================================

// NewSpaceListResponse creates a new space list response.
func NewSpaceListResponse() *SpaceListResponse {
	return &SpaceListResponse{
		Data:  []SpaceResponse{},
		Total: 0,
	}
}

// Add adds a space to the response.
func (r *SpaceListResponse) Add(space SpaceResponse) {
	r.Data = append(r.Data, space)
}

// WithTotal sets the total count.
func (r *SpaceListResponse) WithTotal(total int64) *SpaceListResponse {
	r.Total = total
	return r
}

// WithNextCursor sets the next cursor.
func (r *SpaceListResponse) WithNextCursor(cursor string) *SpaceListResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *SpaceListResponse) WithLimit(limit int) *SpaceListResponse {
	r.Limit = limit
	return r
}

// ======================================================================
// Builder Methods for SpaceParticipantListResponse
// ======================================================================

// NewSpaceParticipantListResponse creates a new participant list response.
func NewSpaceParticipantListResponse() *SpaceParticipantListResponse {
	return &SpaceParticipantListResponse{
		Data:  []SpaceParticipantResponse{},
		Total: 0,
	}
}

// Add adds a participant to the response.
func (r *SpaceParticipantListResponse) Add(participant SpaceParticipantResponse) {
	r.Data = append(r.Data, participant)
}

// WithTotal sets the total count.
func (r *SpaceParticipantListResponse) WithTotal(total int64) *SpaceParticipantListResponse {
	r.Total = total
	return r
}

// WithNextCursor sets the next cursor.
func (r *SpaceParticipantListResponse) WithNextCursor(cursor string) *SpaceParticipantListResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *SpaceParticipantListResponse) WithLimit(limit int) *SpaceParticipantListResponse {
	r.Limit = limit
	return r
}

// ======================================================================
// Builder Methods for SpaceStatsResponse
// ======================================================================

// NewSpaceStatsResponse creates a new space stats response.
func NewSpaceStatsResponse() *SpaceStatsResponse {
	return &SpaceStatsResponse{}
}

// WithTotalSpaces sets the total spaces.
func (r *SpaceStatsResponse) WithTotalSpaces(total int64) *SpaceStatsResponse {
	r.TotalSpaces = total
	return r
}

// WithActiveSpaces sets the active spaces.
func (r *SpaceStatsResponse) WithActiveSpaces(active int64) *SpaceStatsResponse {
	r.ActiveSpaces = active
	return r
}

// WithScheduledSpaces sets the scheduled spaces.
func (r *SpaceStatsResponse) WithScheduledSpaces(scheduled int64) *SpaceStatsResponse {
	r.ScheduledSpaces = scheduled
	return r
}

// WithEndedSpaces sets the ended spaces.
func (r *SpaceStatsResponse) WithEndedSpaces(ended int64) *SpaceStatsResponse {
	r.EndedSpaces = ended
	return r
}

// WithTotalSpeakers sets the total speakers.
func (r *SpaceStatsResponse) WithTotalSpeakers(total int64) *SpaceStatsResponse {
	r.TotalSpeakers = total
	return r
}

// WithTotalListeners sets the total listeners.
func (r *SpaceStatsResponse) WithTotalListeners(total int64) *SpaceStatsResponse {
	r.TotalListeners = total
	return r
}

// WithAvgDuration sets the average duration.
func (r *SpaceStatsResponse) WithAvgDuration(avg float64) *SpaceStatsResponse {
	r.AvgDuration = avg
	return r
}

// WithMaxConcurrent sets the max concurrent.
func (r *SpaceStatsResponse) WithMaxConcurrent(max int64) *SpaceStatsResponse {
	r.MaxConcurrent = max
	return r
}

// ======================================================================
// Conversion Helpers
// ======================================================================

// ToSpaceResponse converts space data to response.
func ToSpaceResponse(id, title, createdBy, visibility, spaceType, status string, duration, maxListeners int, createdAt, updatedAt time.Time) SpaceResponse {
	return SpaceResponse{
		ID:           id,
		Title:        title,
		CreatedBy:    createdBy,
		Visibility:   visibility,
		Type:         spaceType,
		Status:       status,
		Duration:     duration,
		MaxListeners: maxListeners,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

// ToSpeakerResponse converts speaker data to response.
func ToSpeakerResponse(userID, username, fullName, avatarURL string, status SpeakerStatus, joinedAt time.Time, isHost, isSpeaking, isMuted bool) SpeakerResponse {
	return SpeakerResponse{
		UserID:     userID,
		Username:   username,
		FullName:   fullName,
		AvatarURL:  avatarURL,
		Status:     status,
		JoinedAt:   joinedAt,
		IsHost:     isHost,
		IsSpeaking: isSpeaking,
		IsMuted:    isMuted,
	}
}

// ======================================================================
// JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *SpaceResponse) MarshalJSON() ([]byte, error) {
	type Alias SpaceResponse
	return json.Marshal(&struct {
		*Alias
		Visibility string `json:"visibility"`
		Type       string `json:"type"`
		Status     string `json:"status"`
	}{
		Alias:      (*Alias)(r),
		Visibility: r.Visibility,
		Type:       r.Type,
		Status:     r.Status,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *SpaceResponse) UnmarshalJSON(data []byte) error {
	type Alias SpaceResponse
	aux := &struct {
		*Alias
		Visibility string `json:"visibility"`
		Type       string `json:"type"`
		Status     string `json:"status"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Visibility != "" {
		r.Visibility = aux.Visibility
	}
	if aux.Type != "" {
		r.Type = aux.Type
	}
	if aux.Status != "" {
		r.Status = aux.Status
	}
	return nil
}

// ======================================================================
// Test Helpers
// ======================================================================

// NewTestCreateSpaceRequest creates a test create space request.
func NewTestCreateSpaceRequest() *CreateSpaceRequest {
	return &CreateSpaceRequest{
		Title:       "Golang Community Space",
		Description: "Discussion about Go programming",
		Topic:       "Golang best practices",
		Visibility:  VisibilityPublic,
		Type:        SpaceTypeOpen,
		Duration:    60,
		MaxListeners: 100,
	}
}

// NewTestUpdateSpaceRequest creates a test update space request.
func NewTestUpdateSpaceRequest(id string) *UpdateSpaceRequest {
	status := SpaceStatusLive
	return &UpdateSpaceRequest{
		ID:     id,
		Title:  strPtr("Updated Title"),
		Status: &status,
	}
}

// NewTestSpaceResponse creates a test space response.
func NewTestSpaceResponse() *SpaceResponse {
	resp := NewSpaceResponse(
		"space1", "Golang Community Space", "user1",
		string(VisibilityPublic), string(SpaceTypeOpen),
	)
	resp.WithDescription("Discussion about Go programming")
	resp.WithTopic("Golang best practices")
	resp.WithStatus(string(SpaceStatusLive))
	resp.WithSpeakerCount(3).WithListenerCount(50)
	return resp
}

// NewTestSpeakerResponse creates a test speaker response.
func NewTestSpeakerResponse() *SpeakerResponse {
	resp := NewSpeakerResponse(
		"user1", "john_doe", "John Doe", SpeakerStatusSpeaking,
	)
	resp.WithAvatarURL("https://example.com/avatar.jpg")
	resp.WithIsHost(true)
	return resp
}

// NewTestSpaceListResponse creates a test space list response.
func NewTestSpaceListResponse() *SpaceListResponse {
	list := NewSpaceListResponse()
	list.Add(*NewTestSpaceResponse())
	list.WithTotal(1).WithNextCursor("cursor123").WithLimit(20)
	return list
}

// NewTestSpaceStatsResponse creates a test space stats response.
func NewTestSpaceStatsResponse() *SpaceStatsResponse {
	stats := NewSpaceStatsResponse()
	stats.WithTotalSpaces(50).WithActiveSpaces(10)
	stats.WithScheduledSpaces(15).WithEndedSpaces(25)
	stats.WithTotalSpeakers(200).WithTotalListeners(1000)
	stats.WithAvgDuration(45.5).WithMaxConcurrent(5)
	return stats
}

// ======================================================================
// Helper Functions
// ======================================================================

func strPtr(s string) *string {
	return &s
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagSpaces = "Spaces"
)