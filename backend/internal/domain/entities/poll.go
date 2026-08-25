// backend/internal/domain/entities/poll.go
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

const (
	MinPollOptions = 2
	MaxPollOptions = 4
	MaxPollOptionLength = 25
	MinPollDuration = 1 * time.Minute
	MaxPollDuration = 7 * 24 * time.Hour
)

// ======================================================================
// Errors
// ======================================================================

var (
	ErrPollIDEmpty           = errors.New("poll ID cannot be empty")
	ErrPollTweetIDEmpty      = errors.New("tweet ID cannot be empty")
	ErrPollTooFewOptions     = fmt.Errorf("poll must have at least %d options", MinPollOptions)
	ErrPollTooManyOptions    = fmt.Errorf("poll can have at most %d options", MaxPollOptions)
	ErrPollOptionEmpty       = errors.New("poll option cannot be empty")
	ErrPollOptionTooLong     = fmt.Errorf("poll option exceeds maximum length of %d characters", MaxPollOptionLength)
	ErrPollOptionDuplicate   = errors.New("poll options must be unique")
	ErrPollDurationInvalid   = fmt.Errorf("poll duration must be between 1 minute and 7 days")
	ErrPollExpired           = errors.New("poll has expired")
	ErrPollNotExpired        = errors.New("poll has not expired yet")
	ErrPollOptionNotFound    = errors.New("poll option not found")
	ErrPollAlreadyVoted      = errors.New("user already voted on this poll")
	ErrPollCannotVote        = errors.New("cannot vote on this poll")
	ErrPollCannotUnvote      = errors.New("cannot unvote on this poll")
	ErrPollAlreadyDeleted    = errors.New("poll already deleted")
	ErrPollNotDeleted        = errors.New("poll is not deleted")
	ErrPollVoterIDEmpty      = errors.New("voter ID cannot be empty")
)

// ======================================================================
= PollOption
// ======================================================================

// PollOption represents a single option in a poll.
type PollOption struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Votes    int64    `json:"votes"`
	VoterIDs []string `json:"voter_ids,omitempty"`
}

// ======================================================================
= Poll Entity
// ======================================================================

// Poll represents a poll associated with a tweet.
type Poll struct {
	ID        string        `db:"id" json:"id"`
	TweetID   string        `db:"tweet_id" json:"tweet_id"`
	Options   []PollOption  `db:"options" json:"options"`
	Duration  time.Duration `db:"duration" json:"duration"`
	ExpiresAt time.Time     `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time     `db:"created_at" json:"created_at"`
	UpdatedAt time.Time     `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time    `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ======================================================================
= Factory Methods
// ======================================================================

// NewPoll creates a new poll instance with validation.
func NewPoll(tweetID string, options []string, duration time.Duration) (*Poll, error) {
	p := &Poll{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		Options:   make([]PollOption, 0, len(options)),
		Duration:  duration,
		ExpiresAt: time.Now().Add(duration),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := p.SetOptions(options); err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// MustNewPoll creates a new poll and panics on error.
func MustNewPoll(tweetID string, options []string, duration time.Duration) *Poll {
	p, err := NewPoll(tweetID, options, duration)
	if err != nil {
		panic(err)
	}
	return p
}

// ======================================================================
= Validation
// ======================================================================

// Validate performs comprehensive validation.
func (p *Poll) Validate() error {
	// ID validation
	if strings.TrimSpace(p.ID) == "" {
		return ErrPollIDEmpty
	}
	if strings.TrimSpace(p.TweetID) == "" {
		return ErrPollTweetIDEmpty
	}

	// Duration validation
	if p.Duration < MinPollDuration || p.Duration > MaxPollDuration {
		return ErrPollDurationInvalid
	}

	// ExpiresAt validation
	if p.ExpiresAt.IsZero() {
		return errors.New("expires_at cannot be zero")
	}
	if p.ExpiresAt.Before(time.Now()) && p.DeletedAt == nil {
		return ErrPollExpired
	}

	// Options validation
	if len(p.Options) < MinPollOptions {
		return ErrPollTooFewOptions
	}
	if len(p.Options) > MaxPollOptions {
		return ErrPollTooManyOptions
	}
	seen := make(map[string]bool)
	for _, opt := range p.Options {
		trimmed := strings.TrimSpace(opt.Text)
		if trimmed == "" {
			return ErrPollOptionEmpty
		}
		if len(trimmed) > MaxPollOptionLength {
			return ErrPollOptionTooLong
		}
		if seen[trimmed] {
			return ErrPollOptionDuplicate
		}
		seen[trimmed] = true
	}

	return nil
}

// ======================================================================
= Option Management
// ======================================================================

// SetOptions sets the poll options from a list of strings.
func (p *Poll) SetOptions(options []string) error {
	if len(options) < MinPollOptions {
		return ErrPollTooFewOptions
	}
	if len(options) > MaxPollOptions {
		return ErrPollTooManyOptions
	}
	seen := make(map[string]bool)
	newOptions := make([]PollOption, 0, len(options))
	for _, optText := range options {
		trimmed := strings.TrimSpace(optText)
		if trimmed == "" {
			return ErrPollOptionEmpty
		}
		if len(trimmed) > MaxPollOptionLength {
			return ErrPollOptionTooLong
		}
		if seen[trimmed] {
			return ErrPollOptionDuplicate
		}
		seen[trimmed] = true
		newOptions = append(newOptions, PollOption{
			ID:       uuid.New().String(),
			Text:     trimmed,
			Votes:    0,
			VoterIDs: []string{},
		})
	}
	p.Options = newOptions
	p.UpdatedAt = time.Now()
	return nil
}

// GetOption returns a poll option by ID.
func (p *Poll) GetOption(optionID string) (*PollOption, error) {
	for i, opt := range p.Options {
		if opt.ID == optionID {
			return &p.Options[i], nil
		}
	}
	return nil, ErrPollOptionNotFound
}

// GetOptionIndex returns the index of an option by ID.
func (p *Poll) GetOptionIndex(optionID string) (int, error) {
	for i, opt := range p.Options {
		if opt.ID == optionID {
			return i, nil
		}
	}
	return -1, ErrPollOptionNotFound
}

// ======================================================================
= Voting Operations
// ======================================================================

// AddVote adds a vote to a poll option.
func (p *Poll) AddVote(userID, optionID string) error {
	if p.DeletedAt != nil {
		return ErrPollAlreadyDeleted
	}
	if p.IsExpired() {
		return ErrPollExpired
	}
	if strings.TrimSpace(userID) == "" {
		return ErrPollVoterIDEmpty
	}

	// Check if user already voted
	if p.HasUserVoted(userID) {
		return ErrPollAlreadyVoted
	}

	// Find option and add vote
	idx, err := p.GetOptionIndex(optionID)
	if err != nil {
		return err
	}
	p.Options[idx].Votes++
	if p.Options[idx].VoterIDs == nil {
		p.Options[idx].VoterIDs = []string{}
	}
	p.Options[idx].VoterIDs = append(p.Options[idx].VoterIDs, userID)
	p.UpdatedAt = time.Now()
	return nil
}

// RemoveVote removes a vote from a poll option.
func (p *Poll) RemoveVote(userID, optionID string) error {
	if p.DeletedAt != nil {
		return ErrPollAlreadyDeleted
	}
	if p.IsExpired() {
		return ErrPollExpired
	}
	if strings.TrimSpace(userID) == "" {
		return ErrPollVoterIDEmpty
	}

	// Check if user voted
	if !p.HasUserVoted(userID) {
		return ErrPollCannotUnvote
	}

	// Find option and remove vote
	idx, err := p.GetOptionIndex(optionID)
	if err != nil {
		return err
	}
	newVoters := []string{}
	for _, uid := range p.Options[idx].VoterIDs {
		if uid != userID {
			newVoters = append(newVoters, uid)
		}
	}
	if len(newVoters) == len(p.Options[idx].VoterIDs) {
		return errors.New("user did not vote for this option")
	}
	p.Options[idx].Votes = int64(len(newVoters))
	p.Options[idx].VoterIDs = newVoters
	p.UpdatedAt = time.Now()
	return nil
}

// HasUserVoted checks if a user has voted on the poll.
func (p *Poll) HasUserVoted(userID string) bool {
	for _, opt := range p.Options {
		for _, uid := range opt.VoterIDs {
			if uid == userID {
				return true
			}
		}
	}
	return false
}

// GetUserVote returns the option ID a user voted for.
func (p *Poll) GetUserVote(userID string) string {
	for _, opt := range p.Options {
		for _, uid := range opt.VoterIDs {
			if uid == userID {
				return opt.ID
			}
		}
	}
	return ""
}

// GetTotalVotes returns the total number of votes.
func (p *Poll) GetTotalVotes() int64 {
	total := int64(0)
	for _, opt := range p.Options {
		total += opt.Votes
	}
	return total
}

// GetVotePercentages returns the percentage for each option.
func (p *Poll) GetVotePercentages() map[string]float64 {
	total := p.GetTotalVotes()
	if total == 0 {
		return map[string]float64{}
	}
	percentages := make(map[string]float64)
	for _, opt := range p.Options {
		percentages[opt.ID] = (float64(opt.Votes) / float64(total)) * 100
	}
	return percentages
}

// ======================================================================
= Expiration Operations
// ======================================================================

// IsExpired checks if the poll has expired.
func (p *Poll) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

// IsActive checks if the poll is active (not expired and not deleted).
func (p *Poll) IsActive() bool {
	return !p.IsExpired() && p.DeletedAt == nil
}

// ExtendExpiration extends the poll's expiration time.
func (p *Poll) ExtendExpiration(additional time.Duration) error {
	if p.DeletedAt != nil {
		return ErrPollAlreadyDeleted
	}
	if additional < 0 {
		return errors.New("cannot extend by negative duration")
	}
	if p.Duration+additional > MaxPollDuration {
		return errors.New("would exceed maximum poll duration")
	}
	p.ExpiresAt = p.ExpiresAt.Add(additional)
	p.Duration += additional
	p.UpdatedAt = time.Now()
	return nil
}

// SetExpiration sets a specific expiration time.
func (p *Poll) SetExpiration(expiresAt time.Time) error {
	if p.DeletedAt != nil {
		return ErrPollAlreadyDeleted
	}
	if expiresAt.Before(time.Now()) {
		return errors.New("expiration time cannot be in the past")
	}
	newDuration := expiresAt.Sub(time.Now())
	if newDuration < MinPollDuration || newDuration > MaxPollDuration {
		return ErrPollDurationInvalid
	}
	p.ExpiresAt = expiresAt
	p.Duration = newDuration
	p.UpdatedAt = time.Now()
	return nil
}

// ======================================================================
= Deletion Operations
// ======================================================================

// SoftDelete marks the poll as deleted.
func (p *Poll) SoftDelete() error {
	if p.DeletedAt != nil {
		return ErrPollAlreadyDeleted
	}
	now := time.Now()
	p.DeletedAt = &now
	p.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted poll.
func (p *Poll) Restore() error {
	if p.DeletedAt == nil {
		return ErrPollNotDeleted
	}
	p.DeletedAt = nil
	p.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the poll is deleted.
func (p *Poll) IsDeleted() bool {
	return p.DeletedAt != nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// Clone returns a deep copy of the poll.
func (p *Poll) Clone() *Poll {
	clone := *p
	if p.DeletedAt != nil {
		val := *p.DeletedAt
		clone.DeletedAt = &val
	}
	clone.Options = make([]PollOption, len(p.Options))
	for i, opt := range p.Options {
		clone.Options[i] = opt
		clone.Options[i].VoterIDs = make([]string, len(opt.VoterIDs))
		copy(clone.Options[i].VoterIDs, opt.VoterIDs)
	}
	return &clone
}

// String returns a human-readable representation.
func (p *Poll) String() string {
	return fmt.Sprintf("Poll{ID:%s, tweet:%s, options:%d, expires:%v}", p.ID, p.TweetID, len(p.Options), p.ExpiresAt)
}

// Equals checks if two polls have the same ID.
func (p *Poll) Equals(other *Poll) bool {
	return p.ID == other.ID
}

// IsEmpty returns true if the poll is zero value.
func (p *Poll) IsEmpty() bool {
	return p.ID == "" && p.TweetID == "" && len(p.Options) == 0
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (p Poll) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (p *Poll) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for Poll: %T", value)
	}
	return json.Unmarshal(bytes, p)
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (p *Poll) MarshalJSON() ([]byte, error) {
	type Alias Poll
	return json.Marshal(&struct {
		*Alias
		IsExpired bool `json:"is_expired"`
		TotalVotes int64 `json:"total_votes"`
	}{
		Alias:      (*Alias)(p),
		IsExpired:  p.IsExpired(),
		TotalVotes: p.GetTotalVotes(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (p *Poll) UnmarshalJSON(data []byte) error {
	type Alias Poll
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	return json.Unmarshal(data, aux)
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// PollBuilder helps construct polls for testing.
type PollBuilder struct {
	poll *Poll
}

// NewPollBuilder creates a new poll builder.
func NewPollBuilder() *PollBuilder {
	return &PollBuilder{
		poll: &Poll{
			ID:        uuid.New().String(),
			TweetID:   "",
			Options:   []PollOption{},
			Duration:  24 * time.Hour,
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *PollBuilder) WithID(id string) *PollBuilder {
	b.poll.ID = id
	return b
}

// WithTweetID sets the tweet ID.
func (b *PollBuilder) WithTweetID(tweetID string) *PollBuilder {
	b.poll.TweetID = tweetID
	return b
}

// WithOptions sets the options.
func (b *PollBuilder) WithOptions(options []string) *PollBuilder {
	_ = b.poll.SetOptions(options)
	return b
}

// WithDuration sets the duration.
func (b *PollBuilder) WithDuration(duration time.Duration) *PollBuilder {
	b.poll.Duration = duration
	b.poll.ExpiresAt = time.Now().Add(duration)
	return b
}

// WithExpiresAt sets the expiration time.
func (b *PollBuilder) WithExpiresAt(expiresAt time.Time) *PollBuilder {
	b.poll.ExpiresAt = expiresAt
	b.poll.Duration = expiresAt.Sub(time.Now())
	return b
}

// WithCreatedAt sets the creation time.
func (b *PollBuilder) WithCreatedAt(t time.Time) *PollBuilder {
	b.poll.CreatedAt = t
	b.poll.UpdatedAt = t
	return b
}

// WithVotes sets vote counts for options.
func (b *PollBuilder) WithVotes(votes map[string]int64) *PollBuilder {
	for i, opt := range b.poll.Options {
		if count, ok := votes[opt.ID]; ok {
			b.poll.Options[i].Votes = count
		}
	}
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *PollBuilder) WithDeleted(t time.Time) *PollBuilder {
	b.poll.DeletedAt = &t
	return b
}

// Build validates and returns the poll.
func (b *PollBuilder) Build() (*Poll, error) {
	if err := b.poll.Validate(); err != nil {
		return nil, err
	}
	return b.poll, nil
}

// MustBuild builds without error (panics on error).
func (b *PollBuilder) MustBuild() *Poll {
	p, err := b.Build()
	if err != nil {
		panic(err)
	}
	return p
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestPoll1 = MustNewPoll("tweet1", []string{"Option A", "Option B"}, 24*time.Hour)
	TestPoll2 = MustNewPoll("tweet2", []string{"Yes", "No", "Maybe"}, 48*time.Hour)
	TestPoll3 = MustNewPoll("tweet3", []string{"Choice 1", "Choice 2", "Choice 3", "Choice 4"}, 7*24*time.Hour)
)