// backend/internal/dto/poll_request.go
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
	MinPollOptions        = 2
	MaxPollOptions        = 4
	MaxPollOptionLength   = 25
	MinPollOptionLength   = 1
	MinPollDuration       = 1 // minutes
	MaxPollDuration       = 7 * 24 * 60 // 7 days in minutes
	DefaultPollLimit      = 20
	MaxPollLimit          = 100
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrPollIDRequired         = errors.New("poll ID is required")
	ErrTweetIDRequired        = errors.New("tweet ID is required")
	ErrPollOptionRequired     = errors.New("poll option is required")
	ErrPollOptionsTooFew      = fmt.Errorf("poll must have at least %d options", MinPollOptions)
	ErrPollOptionsTooMany     = fmt.Errorf("poll can have at most %d options", MaxPollOptions)
	ErrPollOptionEmpty        = errors.New("poll option cannot be empty")
	ErrPollOptionTooShort     = fmt.Errorf("poll option must be at least %d characters", MinPollOptionLength)
	ErrPollOptionTooLong      = fmt.Errorf("poll option exceeds maximum of %d characters", MaxPollOptionLength)
	ErrPollOptionDuplicate    = errors.New("poll options must be unique")
	ErrPollDurationInvalid    = fmt.Errorf("poll duration must be between %d minutes and %d days", MinPollDuration, MaxPollDuration/1440)
	ErrPollAlreadyVoted       = errors.New("user already voted on this poll")
	ErrPollExpired            = errors.New("poll has expired")
	ErrPollNotFound           = errors.New("poll not found")
	ErrInvalidPollType        = errors.New("invalid poll type")
	ErrUserIDRequired         = errors.New("user ID is required")
	ErrInvalidLimit           = errors.New("limit must be between 1 and 100")
	ErrInvalidCursor          = errors.New("invalid cursor format")
	ErrPollAlreadyClosed      = errors.New("poll is already closed")
	ErrPollCannotBeClosed     = errors.New("poll cannot be closed")
	ErrInvalidVoteAction      = errors.New("invalid vote action")
)

// ======================================================================
// Poll Types
// ======================================================================

// PollType represents the type of poll.
type PollType string

const (
	PollTypeSingleChoice PollType = "single_choice"
	PollTypeMultipleChoice PollType = "multiple_choice"
)

// ValidPollTypes returns all valid poll types.
func ValidPollTypes() []PollType {
	return []PollType{
		PollTypeSingleChoice,
		PollTypeMultipleChoice,
	}
}

// IsValid checks if a poll type is valid.
func (t PollType) IsValid() bool {
	for _, typ := range ValidPollTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (t PollType) String() string {
	return string(t)
}

// PollStatus represents the status of a poll.
type PollStatus string

const (
	PollStatusActive   PollStatus = "active"
	PollStatusClosed   PollStatus = "closed"
	PollStatusExpired  PollStatus = "expired"
	PollStatusDraft    PollStatus = "draft"
)

// ValidPollStatuses returns all valid poll statuses.
func ValidPollStatuses() []PollStatus {
	return []PollStatus{
		PollStatusActive,
		PollStatusClosed,
		PollStatusExpired,
		PollStatusDraft,
	}
}

// IsValid checks if a poll status is valid.
func (s PollStatus) IsValid() bool {
	for _, status := range ValidPollStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (s PollStatus) String() string {
	return string(s)
}

// ======================================================================
// Request DTOs
// ======================================================================

// CreatePollRequest represents the request to create a poll.
type CreatePollRequest struct {
	TweetID     string    `json:"tweet_id,omitempty"`
	Question    string    `json:"question" binding:"required"`
	Options     []string  `json:"options" binding:"required"`
	Type        PollType  `json:"type"`
	Duration    int       `json:"duration"` // duration in minutes
	AllowMultiple bool    `json:"allow_multiple,omitempty"`
	MaxSelections int     `json:"max_selections,omitempty"`
	HideResults  bool     `json:"hide_results,omitempty"`
}

// Validate validates the create poll request.
func (r *CreatePollRequest) Validate() error {
	if strings.TrimSpace(r.Question) == "" {
		return errors.New("poll question is required")
	}
	if len(strings.TrimSpace(r.Question)) > 200 {
		return errors.New("poll question exceeds maximum of 200 characters")
	}
	if len(r.Options) < MinPollOptions {
		return ErrPollOptionsTooFew
	}
	if len(r.Options) > MaxPollOptions {
		return ErrPollOptionsTooMany
	}
	seen := make(map[string]bool)
	for i, opt := range r.Options {
		trimmed := strings.TrimSpace(opt)
		if trimmed == "" {
			return fmt.Errorf("option %d: %w", i+1, ErrPollOptionEmpty)
		}
		if len(trimmed) < MinPollOptionLength {
			return fmt.Errorf("option %d: %w", i+1, ErrPollOptionTooShort)
		}
		if len(trimmed) > MaxPollOptionLength {
			return fmt.Errorf("option %d: %w", i+1, ErrPollOptionTooLong)
		}
		if seen[trimmed] {
			return fmt.Errorf("option %d: %w", i+1, ErrPollOptionDuplicate)
		}
		seen[trimmed] = true
		r.Options[i] = trimmed
	}
	if r.Type != "" && !r.Type.IsValid() {
		return ErrInvalidPollType
	}
	if r.Duration < MinPollDuration {
		return fmt.Errorf("poll duration must be at least %d minutes", MinPollDuration)
	}
	if r.Duration > MaxPollDuration {
		return fmt.Errorf("poll duration must be at most %d days", MaxPollDuration/1440)
	}
	if r.AllowMultiple && r.MaxSelections > 0 && r.MaxSelections > len(r.Options) {
		return errors.New("max selections cannot exceed number of options")
	}
	if r.AllowMultiple && r.MaxSelections < 2 {
		return errors.New("max selections must be at least 2 for multiple choice")
	}
	return nil
}

// Sanitize sanitizes the create poll request.
func (r *CreatePollRequest) Sanitize() {
	r.TweetID = strings.TrimSpace(r.TweetID)
	r.Question = strings.TrimSpace(r.Question)
	if r.Type == "" {
		if r.AllowMultiple {
			r.Type = PollTypeMultipleChoice
		} else {
			r.Type = PollTypeSingleChoice
		}
	}
	if r.MaxSelections <= 0 {
		if r.AllowMultiple {
			r.MaxSelections = 2
		} else {
			r.MaxSelections = 1
		}
	}
	if r.Duration < 1 {
		r.Duration = 60 // default 1 hour
	}
}

// VotePollRequest represents the request to vote on a poll.
type VotePollRequest struct {
	PollID     string   `json:"poll_id" binding:"required"`
	UserID     string   `json:"user_id,omitempty"`
	OptionIDs  []string `json:"option_ids" binding:"required"`
}

// Validate validates the vote poll request.
func (r *VotePollRequest) Validate() error {
	if strings.TrimSpace(r.PollID) == "" {
		return ErrPollIDRequired
	}
	if len(r.OptionIDs) == 0 {
		return errors.New("at least one option ID is required")
	}
	if len(r.OptionIDs) > MaxPollOptions {
		return fmt.Errorf("cannot select more than %d options", MaxPollOptions)
	}
	for i, id := range r.OptionIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("option ID at index %d is empty", i)
		}
	}
	return nil
}

// Sanitize sanitizes the vote poll request.
func (r *VotePollRequest) Sanitize() {
	r.PollID = strings.TrimSpace(r.PollID)
	r.UserID = strings.TrimSpace(r.UserID)
	cleaned := make([]string, 0, len(r.OptionIDs))
	for _, id := range r.OptionIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.OptionIDs = cleaned
}

// GetPollResultsRequest represents the request to get poll results.
type GetPollResultsRequest struct {
	PollID   string `json:"poll_id" binding:"required"`
	UserID   string `json:"user_id,omitempty"`
	Detailed bool   `json:"detailed,omitempty"`
}

// Validate validates the get poll results request.
func (r *GetPollResultsRequest) Validate() error {
	if strings.TrimSpace(r.PollID) == "" {
		return ErrPollIDRequired
	}
	return nil
}

// Sanitize sanitizes the get poll results request.
func (r *GetPollResultsRequest) Sanitize() {
	r.PollID = strings.TrimSpace(r.PollID)
	r.UserID = strings.TrimSpace(r.UserID)
}

// ListPollsRequest represents the request to list polls.
type ListPollsRequest struct {
	TweetID    string `json:"tweet_id,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Type       string `json:"type,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	SortOrder  string `json:"sort_order,omitempty"`
	IncludeExpired bool `json:"include_expired,omitempty"`
	HasVotes   *bool  `json:"has_votes,omitempty"`
}

// Validate validates the list polls request.
func (r *ListPollsRequest) Validate() error {
	if r.Status != "" && !PollStatus(r.Status).IsValid() {
		return errors.New("invalid poll status")
	}
	if r.Type != "" && !PollType(r.Type).IsValid() {
		return ErrInvalidPollType
	}
	if r.Limit < 0 || r.Limit > MaxPollLimit {
		return ErrInvalidLimit
	}
	if r.SortBy != "" {
		allowed := map[string]bool{
			"created_at": true, "expires_at": true, "total_votes": true,
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

// Sanitize sanitizes the list polls request.
func (r *ListPollsRequest) Sanitize() {
	r.TweetID = strings.TrimSpace(r.TweetID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.Status = strings.TrimSpace(r.Status)
	r.Type = strings.TrimSpace(r.Type)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultPollLimit
	}
	if r.Limit > MaxPollLimit {
		r.Limit = MaxPollLimit
	}
	if r.SortBy != "" {
		r.SortBy = strings.ToLower(strings.TrimSpace(r.SortBy))
	}
	if r.SortOrder != "" {
		r.SortOrder = strings.ToLower(strings.TrimSpace(r.SortOrder))
	}
}

// ClosePollRequest represents the request to close a poll.
type ClosePollRequest struct {
	PollID string `json:"poll_id" binding:"required"`
	UserID string `json:"user_id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Validate validates the close poll request.
func (r *ClosePollRequest) Validate() error {
	if strings.TrimSpace(r.PollID) == "" {
		return ErrPollIDRequired
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	return nil
}

// Sanitize sanitizes the close poll request.
func (r *ClosePollRequest) Sanitize() {
	r.PollID = strings.TrimSpace(r.PollID)
	r.UserID = strings.TrimSpace(r.UserID)
	r.Reason = strings.TrimSpace(r.Reason)
}

// DeletePollRequest represents the request to delete a poll.
type DeletePollRequest struct {
	PollID string `json:"poll_id" binding:"required"`
	UserID string `json:"user_id,omitempty"`
	Force  bool   `json:"force,omitempty"`
}

// Validate validates the delete poll request.
func (r *DeletePollRequest) Validate() error {
	if strings.TrimSpace(r.PollID) == "" {
		return ErrPollIDRequired
	}
	return nil
}

// Sanitize sanitizes the delete poll request.
func (r *DeletePollRequest) Sanitize() {
	r.PollID = strings.TrimSpace(r.PollID)
	r.UserID = strings.TrimSpace(r.UserID)
}

// ======================================================================
// Response DTOs
// ======================================================================

// PollOptionResponse represents a poll option in responses.
type PollOptionResponse struct {
	ID          string  `json:"id"`
	Text        string  `json:"text"`
	Votes       int64   `json:"votes"`
	Percentage  float64 `json:"percentage"`
	IsVoted     bool    `json:"is_voted"`
}

// PollResponse represents a poll in responses.
type PollResponse struct {
	ID          string                 `json:"id"`
	TweetID     string                 `json:"tweet_id"`
	Question    string                 `json:"question"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Options     []PollOptionResponse   `json:"options"`
	TotalVotes  int64                  `json:"total_votes"`
	Duration    int                    `json:"duration"` // minutes
	ExpiresAt   time.Time              `json:"expires_at"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	HasVoted    bool                   `json:"has_voted"`
	UserVotes   []string               `json:"user_votes,omitempty"`
	HideResults bool                   `json:"hide_results"`
	IsExpired   bool                   `json:"is_expired"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PollDetailResponse represents a detailed poll response.
type PollDetailResponse struct {
	PollResponse
	Tweet        *TweetResponse        `json:"tweet,omitempty"`
	User         *MinimalUserResponse  `json:"user,omitempty"`
	VoterCount   int64                 `json:"voter_count"`
	TurnoutRate  float64               `json:"turnout_rate"`
	Results      []PollOptionResponse  `json:"results"`
}

// PollListResponse represents a paginated list of polls.
type PollListResponse struct {
	Data       []PollResponse `json:"data"`
	Total      int64          `json:"total"`
	NextCursor string         `json:"next_cursor"`
	HasMore    bool           `json:"has_more"`
	Limit      int            `json:"limit"`
}

// PollResultResponse represents poll results.
type PollResultResponse struct {
	PollID      string                 `json:"poll_id"`
	TweetID     string                 `json:"tweet_id"`
	Question    string                 `json:"question"`
	Options     []PollOptionResponse   `json:"options"`
	TotalVotes  int64                  `json:"total_votes"`
	ExpiresAt   time.Time              `json:"expires_at"`
	IsExpired   bool                   `json:"is_expired"`
	UserVote    string                 `json:"user_vote,omitempty"`
	UserVotes   []string               `json:"user_votes,omitempty"`
	VoterCount  int64                  `json:"voter_count"`
	TurnoutRate float64                `json:"turnout_rate"`
	WinnerID    string                 `json:"winner_id,omitempty"`
	WinnerText  string                 `json:"winner_text,omitempty"`
}

// PollStatsResponse represents poll statistics.
type PollStatsResponse struct {
	TotalPolls      int64            `json:"total_polls"`
	ActivePolls     int64            `json:"active_polls"`
	ExpiredPolls    int64            `json:"expired_polls"`
	TotalVotes      int64            `json:"total_votes"`
	UniqueVoters    int64            `json:"unique_voters"`
	AverageOptions  float64          `json:"average_options"`
	AverageVotes    float64          `json:"average_votes"`
	TypeStats       map[string]int64 `json:"type_stats"`
	StatusStats     map[string]int64 `json:"status_stats"`
	LastPollCreated time.Time        `json:"last_poll_created"`
	LastPollExpired time.Time        `json:"last_poll_expired"`
}

// PollParticipationStatsResponse represents participation statistics.
type PollParticipationStatsResponse struct {
	TotalVotes    int64   `json:"total_votes"`
	UniqueVoters  int64   `json:"unique_voters"`
	TotalOptions  int64   `json:"total_options"`
	AverageVotes  float64 `json:"average_votes"`
	TurnoutRate   float64 `json:"turnout_rate"`
	IsExpired     bool    `json:"is_expired"`
}

// VoteResponse represents the response after voting.
type VoteResponse struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	PollID       string   `json:"poll_id"`
	OptionIDs    []string `json:"option_ids"`
	TotalVotes   int64    `json:"total_votes"`
}

// ======================================================================
// Builder Methods for PollResponse
// ======================================================================

// NewPollResponse creates a new poll response.
func NewPollResponse(id, tweetID, question, pollType string, expiresAt time.Time) *PollResponse {
	return &PollResponse{
		ID:         id,
		TweetID:    tweetID,
		Question:   question,
		Type:       pollType,
		Status:     string(PollStatusActive),
		Options:    []PollOptionResponse{},
		TotalVotes: 0,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
}

// AddOption adds an option to the poll response.
func (r *PollResponse) AddOption(id, text string, votes int64, percentage float64, isVoted bool) {
	r.Options = append(r.Options, PollOptionResponse{
		ID:         id,
		Text:       text,
		Votes:      votes,
		Percentage: percentage,
		IsVoted:    isVoted,
	})
}

// WithTotalVotes sets the total votes.
func (r *PollResponse) WithTotalVotes(total int64) *PollResponse {
	r.TotalVotes = total
	return r
}

// WithHasVoted sets the has voted flag.
func (r *PollResponse) WithHasVoted(hasVoted bool) *PollResponse {
	r.HasVoted = hasVoted
	return r
}

// WithUserVotes sets the user votes.
func (r *PollResponse) WithUserVotes(votes []string) *PollResponse {
	r.UserVotes = votes
	return r
}

// WithStatus sets the status.
func (r *PollResponse) WithStatus(status string) *PollResponse {
	r.Status = status
	return r
}

// WithHideResults sets the hide results flag.
func (r *PollResponse) WithHideResults(hide bool) *PollResponse {
	r.HideResults = hide
	return r
}

// WithMetadata sets the metadata.
func (r *PollResponse) WithMetadata(metadata map[string]interface{}) *PollResponse {
	r.Metadata = metadata
	return r
}

// WithIsExpired sets the is expired flag.
func (r *PollResponse) WithIsExpired(expired bool) *PollResponse {
	r.IsExpired = expired
	return r
}

// ======================================================================
// Builder Methods for PollResultResponse
// ======================================================================

// NewPollResultResponse creates a new poll result response.
func NewPollResultResponse(pollID, tweetID, question string, expiresAt time.Time) *PollResultResponse {
	return &PollResultResponse{
		PollID:    pollID,
		TweetID:   tweetID,
		Question:  question,
		Options:   []PollOptionResponse{},
		ExpiresAt: expiresAt,
	}
}

// AddOption adds an option to the result.
func (r *PollResultResponse) AddOption(id, text string, votes int64, percentage float64) {
	r.Options = append(r.Options, PollOptionResponse{
		ID:         id,
		Text:       text,
		Votes:      votes,
		Percentage: percentage,
	})
}

// WithTotalVotes sets the total votes.
func (r *PollResultResponse) WithTotalVotes(total int64) *PollResultResponse {
	r.TotalVotes = total
	return r
}

// WithUserVote sets the user vote.
func (r *PollResultResponse) WithUserVote(vote string) *PollResultResponse {
	r.UserVote = vote
	return r
}

// WithUserVotes sets the user votes.
func (r *PollResultResponse) WithUserVotes(votes []string) *PollResultResponse {
	r.UserVotes = votes
	return r
}

// WithWinner sets the winner.
func (r *PollResultResponse) WithWinner(id, text string) *PollResultResponse {
	r.WinnerID = id
	r.WinnerText = text
	return r
}

// WithVoterCount sets the voter count.
func (r *PollResultResponse) WithVoterCount(count int64) *PollResultResponse {
	r.VoterCount = count
	return r
}

// WithTurnoutRate sets the turnout rate.
func (r *PollResultResponse) WithTurnoutRate(rate float64) *PollResultResponse {
	r.TurnoutRate = rate
	return r
}

// ======================================================================
// Builder Methods for PollStatsResponse
// ======================================================================

// NewPollStatsResponse creates a new poll stats response.
func NewPollStatsResponse() *PollStatsResponse {
	return &PollStatsResponse{
		TypeStats:   make(map[string]int64),
		StatusStats: make(map[string]int64),
	}
}

// WithTotalPolls sets the total polls.
func (r *PollStatsResponse) WithTotalPolls(total int64) *PollStatsResponse {
	r.TotalPolls = total
	return r
}

// WithActivePolls sets the active polls.
func (r *PollStatsResponse) WithActivePolls(active int64) *PollStatsResponse {
	r.ActivePolls = active
	return r
}

// WithExpiredPolls sets the expired polls.
func (r *PollStatsResponse) WithExpiredPolls(expired int64) *PollStatsResponse {
	r.ExpiredPolls = expired
	return r
}

// WithTotalVotes sets the total votes.
func (r *PollStatsResponse) WithTotalVotes(votes int64) *PollStatsResponse {
	r.TotalVotes = votes
	return r
}

// WithUniqueVoters sets the unique voters.
func (r *PollStatsResponse) WithUniqueVoters(voters int64) *PollStatsResponse {
	r.UniqueVoters = voters
	return r
}

// AddTypeStat adds a type statistic.
func (r *PollStatsResponse) AddTypeStat(pollType string, count int64) {
	r.TypeStats[pollType] = count
}

// AddStatusStat adds a status statistic.
func (r *PollStatsResponse) AddStatusStat(status string, count int64) {
	r.StatusStats[status] = count
}

// WithLastPollCreated sets the last poll created time.
func (r *PollStatsResponse) WithLastPollCreated(t time.Time) *PollStatsResponse {
	r.LastPollCreated = t
	return r
}

// WithLastPollExpired sets the last poll expired time.
func (r *PollStatsResponse) WithLastPollExpired(t time.Time) *PollStatsResponse {
	r.LastPollExpired = t
	return r
}

// ======================================================================
// Builder Methods for PollListResponse
// ======================================================================

// NewPollListResponse creates a new poll list response.
func NewPollListResponse() *PollListResponse {
	return &PollListResponse{
		Data:  []PollResponse{},
		Total: 0,
	}
}

// Add adds a poll to the response.
func (r *PollListResponse) Add(poll PollResponse) {
	r.Data = append(r.Data, poll)
}

// WithTotal sets the total count.
func (r *PollListResponse) WithTotal(total int64) *PollListResponse {
	r.Total = total
	return r
}

// WithNextCursor sets the next cursor.
func (r *PollListResponse) WithNextCursor(cursor string) *PollListResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *PollListResponse) WithLimit(limit int) *PollListResponse {
	r.Limit = limit
	return r
}

// ======================================================================
// Builder Methods for VoteResponse
// ======================================================================

// NewVoteResponse creates a new vote response.
func NewVoteResponse(pollID string, optionIDs []string) *VoteResponse {
	return &VoteResponse{
		Success:   true,
		Message:   "Vote recorded successfully",
		PollID:    pollID,
		OptionIDs: optionIDs,
	}
}

// WithTotalVotes sets the total votes.
func (r *VoteResponse) WithTotalVotes(total int64) *VoteResponse {
	r.TotalVotes = total
	return r
}

// WithMessage sets the message.
func (r *VoteResponse) WithMessage(message string) *VoteResponse {
	r.Message = message
	return r
}

// ======================================================================
// Conversion Helpers
// ======================================================================

// ToPollOptionResponse converts poll option data to response.
func ToPollOptionResponse(id, text string, votes int64, percentage float64, isVoted bool) PollOptionResponse {
	return PollOptionResponse{
		ID:         id,
		Text:       text,
		Votes:      votes,
		Percentage: percentage,
		IsVoted:    isVoted,
	}
}

// ToPollResponse converts poll data to response.
func ToPollResponse(id, tweetID, question, pollType, status string, options []PollOptionResponse, totalVotes int64, expiresAt, createdAt, updatedAt time.Time) PollResponse {
	return PollResponse{
		ID:         id,
		TweetID:    tweetID,
		Question:   question,
		Type:       pollType,
		Status:     status,
		Options:    options,
		TotalVotes: totalVotes,
		ExpiresAt:  expiresAt,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		IsExpired:  time.Now().After(expiresAt),
	}
}

// ToPollStatsResponse converts poll stats data to response.
func ToPollStatsResponse(totalPolls, activePolls, expiredPolls, totalVotes, uniqueVoters int64, avgOptions, avgVotes float64) PollStatsResponse {
	return PollStatsResponse{
		TotalPolls:     totalPolls,
		ActivePolls:    activePolls,
		ExpiredPolls:   expiredPolls,
		TotalVotes:     totalVotes,
		UniqueVoters:   uniqueVoters,
		AverageOptions: avgOptions,
		AverageVotes:   avgVotes,
	}
}

// ======================================================================
= JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *PollResponse) MarshalJSON() ([]byte, error) {
	type Alias PollResponse
	return json.Marshal(&struct {
		*Alias
		Type   string `json:"type"`
		Status string `json:"status"`
	}{
		Alias:  (*Alias)(r),
		Type:   r.Type,
		Status: r.Status,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *PollResponse) UnmarshalJSON(data []byte) error {
	type Alias PollResponse
	aux := &struct {
		*Alias
		Type   string `json:"type"`
		Status string `json:"status"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
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
= Test Helpers
// ======================================================================

// NewTestCreatePollRequest creates a test create poll request.
func NewTestCreatePollRequest() *CreatePollRequest {
	return &CreatePollRequest{
		Question: "What is your favorite programming language?",
		Options:  []string{"Go", "Python", "JavaScript", "Rust"},
		Type:     PollTypeSingleChoice,
		Duration: 60, // 1 hour
	}
}

// NewTestVotePollRequest creates a test vote poll request.
func NewTestVotePollRequest(pollID string) *VotePollRequest {
	return &VotePollRequest{
		PollID:    pollID,
		OptionIDs: []string{"option1"},
	}
}

// NewTestPollResponse creates a test poll response.
func NewTestPollResponse() *PollResponse {
	resp := NewPollResponse(
		"poll1", "tweet1", "What is your favorite color?",
		string(PollTypeSingleChoice), time.Now().UTC().Add(24*time.Hour),
	)
	resp.AddOption("opt1", "Red", 10, 40.0, false)
	resp.AddOption("opt2", "Blue", 15, 60.0, false)
	resp.WithTotalVotes(25)
	return resp
}

// NewTestPollListResponse creates a test poll list response.
func NewTestPollListResponse() *PollListResponse {
	list := NewPollListResponse()
	list.Add(*NewTestPollResponse())
	list.WithTotal(1).WithNextCursor("cursor123").WithLimit(20)
	return list
}

// NewTestPollResultResponse creates a test poll result response.
func NewTestPollResultResponse() *PollResultResponse {
	resp := NewPollResultResponse(
		"poll1", "tweet1", "What is your favorite color?",
		time.Now().UTC().Add(24*time.Hour),
	)
	resp.AddOption("opt1", "Red", 10, 40.0)
	resp.AddOption("opt2", "Blue", 15, 60.0)
	resp.WithTotalVotes(25).WithWinner("opt2", "Blue")
	resp.WithVoterCount(20).WithTurnoutRate(80.0)
	return resp
}

// NewTestPollStatsResponse creates a test poll stats response.
func NewTestPollStatsResponse() *PollStatsResponse {
	stats := NewPollStatsResponse()
	stats.WithTotalPolls(50).WithActivePolls(20).WithExpiredPolls(30)
	stats.WithTotalVotes(500).WithUniqueVoters(200)
	stats.AddTypeStat("single_choice", 40)
	stats.AddTypeStat("multiple_choice", 10)
	stats.AddStatusStat("active", 20)
	stats.AddStatusStat("expired", 30)
	return stats
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagPolls = "Polls"
)