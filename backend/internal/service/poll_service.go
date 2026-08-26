// backend/internal/service/poll_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	MinPollOptions     = 2
	MaxPollOptions     = 4
	MaxPollOptionLen   = 25
	MinPollDuration    = 1 // minute
	MaxPollDuration    = 7 * 24 * 60 // 7 days in minutes
	DefaultPollLimit   = 20
	MaxPollLimit       = 100
)

var (
	ErrPollNotFound      = errors.New("poll not found")
	ErrPollExpired       = errors.New("poll has expired")
	ErrPollAlreadyVoted  = errors.New("user already voted on this poll")
	ErrInvalidPollOption = errors.New("invalid poll option")
	ErrPollTooManyOptions = errors.New("too many poll options")
	ErrPollTooFewOptions  = errors.New("too few poll options")
	ErrPollOptionEmpty   = errors.New("poll option cannot be empty")
	ErrPollOptionTooLong = errors.New("poll option is too long")
	ErrPollDurationInvalid = errors.New("poll duration is invalid")
	ErrPollAlreadyClosed = errors.New("poll is already closed")
	ErrPollCannotClose   = errors.New("poll cannot be closed")
	ErrTweetNotFound     = errors.New("tweet not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrPollQuestionEmpty = errors.New("poll question is required")
	ErrPollQuestionTooLong = errors.New("poll question is too long")
)

// ======================================================================
// PollService Interface
// ======================================================================

// PollService defines the poll service interface.
type PollService interface {
	// Create creates a new poll.
	Create(ctx context.Context, tweetID string, req *dto.CreatePollRequest) (*dto.PollResponse, error)
	
	// GetByID retrieves a poll by ID.
	GetByID(ctx context.Context, pollID, userID string) (*dto.PollResponse, error)
	
	// GetByTweetID retrieves a poll by tweet ID.
	GetByTweetID(ctx context.Context, tweetID, userID string) (*dto.PollResponse, error)
	
	// Vote adds a vote to a poll.
	Vote(ctx context.Context, pollID, userID, optionID string) (*dto.VoteResponse, error)
	
	// Unvote removes a vote from a poll.
	Unvote(ctx context.Context, pollID, userID, optionID string) error
	
	// GetResults returns poll results.
	GetResults(ctx context.Context, pollID, userID string) (*dto.PollResultResponse, error)
	
	// ClosePoll closes a poll.
	ClosePoll(ctx context.Context, pollID, userID string) error
	
	// DeletePoll deletes a poll.
	DeletePoll(ctx context.Context, pollID, userID string) error
	
	// ListPolls returns a paginated list of polls.
	ListPolls(ctx context.Context, req *dto.ListPollsRequest, userID string) (*dto.PollListResponse, error)
	
	// GetPollStats returns poll statistics.
	GetPollStats(ctx context.Context) (*dto.PollStatsResponse, error)
	
	// GetUserPollStats returns poll statistics for a user.
	GetUserPollStats(ctx context.Context, userID string) (*dto.PollStatsResponse, error)
}

// ======================================================================
// pollService Implementation
// ======================================================================

// pollService implements PollService.
type pollService struct {
	pollRepo         interfaces.PollRepository
	tweetRepo        interfaces.TweetRepository
	userRepo         interfaces.UserRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	log              *logrus.Entry
}

// NewPollService creates a new poll service.
func NewPollService(
	pollRepo interfaces.PollRepository,
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) PollService {
	return &pollService{
		pollRepo:         pollRepo,
		tweetRepo:        tweetRepo,
		userRepo:         userRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		log:              logger.WithField("service", "poll"),
	}
}

// ======================================================================
// Create Poll
// ======================================================================

// Create creates a new poll.
func (s *pollService) Create(ctx context.Context, tweetID string, req *dto.CreatePollRequest) (*dto.PollResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Sanitize()
	// Check if tweet exists
	tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, ErrTweetNotFound
		}
		return nil, fmt.Errorf("failed to get tweet: %w", err)
	}
	if tweet.DeletedAt != nil {
		return nil, ErrTweetNotFound
	}
	// Create poll
	poll := &entities.Poll{
		ID:        uuid.New().String(),
		TweetID:   tweetID,
		Duration:  time.Duration(req.Duration) * time.Minute,
		ExpiresAt: time.Now().Add(time.Duration(req.Duration) * time.Minute),
		CreatedAt: time.Now(),
	}
	// Set options
	options := make([]entities.PollOption, 0, len(req.Options))
	for _, optText := range req.Options {
		options = append(options, entities.PollOption{
			ID:       uuid.New().String(),
			Text:     optText,
			Votes:    0,
			VoterIDs: []string{},
		})
	}
	poll.Options = options
	// Save poll
	if err := s.pollRepo.Create(ctx, poll); err != nil {
		return nil, fmt.Errorf("failed to create poll: %w", err)
	}
	// Update tweet as poll
	tweet.IsPoll = true
	if err := s.tweetRepo.Update(ctx, tweet); err != nil {
		s.log.WithError(err).Warn("Failed to update tweet as poll")
	}
	// Invalidate cache
	_ = s.invalidatePollCache(ctx, poll.ID, tweetID)
	s.log.WithFields(logrus.Fields{
		"poll_id":  poll.ID,
		"tweet_id": tweetID,
		"options":  len(options),
	}).Info("Poll created")
	return s.toPollResponse(poll, ""), nil
}

// ======================================================================
// Get Poll
// ======================================================================

// GetByID retrieves a poll by ID.
func (s *pollService) GetByID(ctx context.Context, pollID, userID string) (*dto.PollResponse, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("poll:%s", pollID)
	if s.redisAdapter != nil {
		var cached dto.PollResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("poll_id", pollID).Debug("Poll served from cache")
			return &cached, nil
		}
	}
	// Get from repository
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return nil, ErrPollNotFound
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}
	// Check if user has voted
	hasVoted := false
	votedOptionID := ""
	if userID != "" {
		hasVoted, _ = s.pollRepo.HasUserVoted(ctx, pollID, userID)
		if hasVoted {
			votedOptionID, _ = s.pollRepo.GetUserVote(ctx, pollID, userID)
		}
	}
	// Build response
	resp := s.toPollResponse(poll, votedOptionID)
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, resp, 30*time.Second)
	}
	return resp, nil
}

// GetByTweetID retrieves a poll by tweet ID.
func (s *pollService) GetByTweetID(ctx context.Context, tweetID, userID string) (*dto.PollResponse, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("poll:tweet:%s", tweetID)
	if s.redisAdapter != nil {
		var cached dto.PollResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	// Check if tweet exists
	_, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return nil, ErrTweetNotFound
		}
		return nil, fmt.Errorf("failed to get tweet: %w", err)
	}
	// Get poll
	poll, err := s.pollRepo.GetByTweetID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return nil, ErrPollNotFound
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}
	// Check if user has voted
	hasVoted := false
	votedOptionID := ""
	if userID != "" {
		hasVoted, _ = s.pollRepo.HasUserVoted(ctx, poll.ID, userID)
		if hasVoted {
			votedOptionID, _ = s.pollRepo.GetUserVote(ctx, poll.ID, userID)
		}
	}
	resp := s.toPollResponse(poll, votedOptionID)
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, resp, 30*time.Second)
	}
	return resp, nil
}

// ======================================================================
// Vote
// ======================================================================

// Vote adds a vote to a poll.
func (s *pollService) Vote(ctx context.Context, pollID, userID, optionID string) (*dto.VoteResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Get poll
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return nil, ErrPollNotFound
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}
	// Check if expired
	if time.Now().After(poll.ExpiresAt) {
		return nil, ErrPollExpired
	}
	// Check if already voted
	hasVoted, err := s.pollRepo.HasUserVoted(ctx, pollID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check user vote: %w", err)
	}
	if hasVoted {
		return nil, ErrPollAlreadyVoted
	}
	// Vote
	if err := s.pollRepo.Vote(ctx, pollID, userID, optionID); err != nil {
		if errors.Is(err, interfaces.ErrInvalidPollOption) {
			return nil, ErrInvalidPollOption
		}
		if errors.Is(err, interfaces.ErrPollExpired) {
			return nil, ErrPollExpired
		}
		return nil, fmt.Errorf("failed to vote: %w", err)
	}
	// Invalidate cache
	_ = s.invalidatePollCache(ctx, pollID, poll.TweetID)
	s.log.WithFields(logrus.Fields{
		"poll_id":   pollID,
		"user_id":   userID,
		"option_id": optionID,
	}).Info("Vote recorded")
	return &dto.VoteResponse{
		Success:   true,
		Message:   "Vote recorded successfully",
		PollID:    pollID,
		OptionIDs: []string{optionID},
	}, nil
}

// ======================================================================
// Unvote
// ======================================================================

// Unvote removes a vote from a poll.
func (s *pollService) Unvote(ctx context.Context, pollID, userID, optionID string) error {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Get poll
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return ErrPollNotFound
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}
	// Check if expired
	if time.Now().After(poll.ExpiresAt) {
		return ErrPollExpired
	}
	// Check if user has voted
	hasVoted, err := s.pollRepo.HasUserVoted(ctx, pollID, userID)
	if err != nil {
		return fmt.Errorf("failed to check user vote: %w", err)
	}
	if !hasVoted {
		return errors.New("user has not voted on this poll")
	}
	// Unvote
	if err := s.pollRepo.Unvote(ctx, pollID, userID, optionID); err != nil {
		if errors.Is(err, interfaces.ErrInvalidPollOption) {
			return ErrInvalidPollOption
		}
		return fmt.Errorf("failed to unvote: %w", err)
	}
	// Invalidate cache
	_ = s.invalidatePollCache(ctx, pollID, poll.TweetID)
	s.log.WithFields(logrus.Fields{
		"poll_id":   pollID,
		"user_id":   userID,
		"option_id": optionID,
	}).Info("Vote removed")
	return nil
}

// ======================================================================
// Get Results
// ======================================================================

// GetResults returns poll results.
func (s *pollService) GetResults(ctx context.Context, pollID, userID string) (*dto.PollResultResponse, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("poll_results:%s", pollID)
	if s.redisAdapter != nil {
		var cached dto.PollResultResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	// Get poll
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return nil, ErrPollNotFound
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}
	// Get total votes
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	// Get user vote
	userVote := ""
	if userID != "" {
		userVote, _ = s.pollRepo.GetUserVote(ctx, pollID, userID)
	}
	// Build response
	options := make([]dto.PollOption, 0, len(poll.Options))
	winnerID := ""
	winnerText := ""
	maxVotes := int64(0)
	for _, opt := range poll.Options {
		percentage := 0.0
		if totalVotes > 0 {
			percentage = (float64(opt.Votes) / float64(totalVotes)) * 100
		}
		isVoted := userVote == opt.ID
		options = append(options, dto.PollOption{
			ID:         opt.ID,
			Text:       opt.Text,
			Votes:      opt.Votes,
			Percentage: percentage,
			IsVoted:    isVoted,
		})
		if opt.Votes > maxVotes {
			maxVotes = opt.Votes
			winnerID = opt.ID
			winnerText = opt.Text
		}
	}
	resp := &dto.PollResultResponse{
		PollID:      poll.ID,
		TweetID:     poll.TweetID,
		Question:    "",
		Options:     options,
		TotalVotes:  totalVotes,
		ExpiresAt:   poll.ExpiresAt,
		IsExpired:   time.Now().After(poll.ExpiresAt),
		UserVote:    userVote,
		WinnerID:    winnerID,
		WinnerText:  winnerText,
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, resp, 30*time.Second)
	}
	return resp, nil
}

// ======================================================================
// Close Poll
// ======================================================================

// ClosePoll closes a poll.
func (s *pollService) ClosePoll(ctx context.Context, pollID, userID string) error {
	// Get poll
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return ErrPollNotFound
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}
	// Check if already expired or closed
	if time.Now().After(poll.ExpiresAt) {
		return ErrPollExpired
	}
	// Close poll by setting expiry to now
	poll.ExpiresAt = time.Now()
	if err := s.pollRepo.Update(ctx, poll); err != nil {
		return fmt.Errorf("failed to close poll: %w", err)
	}
	// Invalidate cache
	_ = s.invalidatePollCache(ctx, poll.ID, poll.TweetID)
	s.log.WithFields(logrus.Fields{
		"poll_id": pollID,
		"user_id": userID,
	}).Info("Poll closed")
	return nil
}

// ======================================================================
// Delete Poll
// ======================================================================

// DeletePoll deletes a poll.
func (s *pollService) DeletePoll(ctx context.Context, pollID, userID string) error {
	// Get poll
	poll, err := s.pollRepo.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, interfaces.ErrPollNotFound) {
			return ErrPollNotFound
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}
	// Check if user owns the tweet (authorization check)
	tweet, err := s.tweetRepo.GetByID(ctx, poll.TweetID)
	if err != nil {
		return fmt.Errorf("failed to get tweet: %w", err)
	}
	if tweet.UserID != userID {
		return errors.New("not authorized to delete this poll")
	}
	// Delete poll
	if err := s.pollRepo.Delete(ctx, pollID); err != nil {
		return fmt.Errorf("failed to delete poll: %w", err)
	}
	// Update tweet as not poll
	tweet.IsPoll = false
	if err := s.tweetRepo.Update(ctx, tweet); err != nil {
		s.log.WithError(err).Warn("Failed to update tweet as not poll")
	}
	// Invalidate cache
	_ = s.invalidatePollCache(ctx, poll.ID, poll.TweetID)
	s.log.WithFields(logrus.Fields{
		"poll_id": pollID,
		"user_id": userID,
	}).Info("Poll deleted")
	return nil
}

// ======================================================================
// List Polls
// ======================================================================

// ListPolls returns a paginated list of polls.
func (s *pollService) ListPolls(ctx context.Context, req *dto.ListPollsRequest, userID string) (*dto.PollListResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Sanitize()
	// Build filter
	filter := &interfaces.PollFilter{}
	if req.TweetID != "" {
		filter.TweetID = &req.TweetID
	}
	if req.UserID != "" {
		// We can't filter by user directly, but we can join with tweets
		// For simplicity, we'll handle this in the repository method
	}
	if req.Status != "" {
		isActive := req.Status == "active"
		isExpired := req.Status == "expired"
		if isActive {
			filter.IsActive = &isActive
		} else if isExpired {
			filter.IsExpired = &isExpired
		}
	}
	if req.Type != "" {
		// Type is not directly stored; skip for now
	}
	if req.HasVotes != nil {
		filter.HasVotes = req.HasVotes
	}
	// Pagination
	pagination := &interfaces.PollPagination{
		Limit:  req.Limit,
		Cursor: req.Cursor,
	}
	if req.SortBy != "" {
		pagination.SortBy = interfaces.PollSortField(req.SortBy)
	}
	if req.SortOrder != "" {
		pagination.Order = interfaces.PollSortOrder(req.SortOrder)
	}
	// Get polls
	polls, total, err := s.pollRepo.List(ctx, filter, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to list polls: %w", err)
	}
	// Build responses
	responses := make([]dto.PollResponse, 0, len(polls))
	for _, poll := range polls {
		resp := s.toPollResponse(poll, "")
		responses = append(responses, *resp)
	}
	return &dto.PollListResponse{
		Data:       responses,
		Total:      total,
		NextCursor: "",
		HasMore:    false,
		Limit:      req.Limit,
	}, nil
}

// ======================================================================
// Stats
// ======================================================================

// GetPollStats returns poll statistics.
func (s *pollService) GetPollStats(ctx context.Context) (*dto.PollStatsResponse, error) {
	// Try cache
	cacheKey := "poll_stats"
	if s.redisAdapter != nil {
		var cached dto.PollStatsResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	stats, err := s.pollRepo.GetPollStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll stats: %w", err)
	}
	response := &dto.PollStatsResponse{
		TotalPolls:      stats.TotalPolls,
		ActivePolls:     stats.ActivePolls,
		ExpiredPolls:    stats.ExpiredPolls,
		TotalVotes:      stats.TotalVotes,
		UniqueVoters:    stats.UniqueVoters,
		AverageOptions:  stats.AverageOptions,
		AverageVotes:    stats.AverageVotes,
		TypeStats:       stats.TypeStats,
		StatusStats:     stats.StatusStats,
		LastPollCreated: stats.LastPollCreated,
		LastPollExpired: stats.LastPollExpired,
	}
	// Cache for 5 minutes
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, response, 5*time.Minute)
	}
	return response, nil
}

// GetUserPollStats returns poll statistics for a user.
func (s *pollService) GetUserPollStats(ctx context.Context, userID string) (*dto.PollStatsResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	stats, err := s.pollRepo.GetUserPollStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user poll stats: %w", err)
	}
	return &dto.PollStatsResponse{
		TotalPolls:      stats.TotalPolls,
		ActivePolls:     stats.ActivePolls,
		ExpiredPolls:    stats.ExpiredPolls,
		TotalVotes:      stats.TotalVotes,
		UniqueVoters:    stats.UniqueVoters,
		AverageOptions:  stats.AverageOptions,
		AverageVotes:    stats.AverageVotes,
	}, nil
}

// ======================================================================
// Helper Methods
// ======================================================================

// toPollResponse converts a poll entity to a response.
func (s *pollService) toPollResponse(poll *entities.Poll, votedOptionID string) *dto.PollResponse {
	totalVotes := int64(0)
	for _, opt := range poll.Options {
		totalVotes += opt.Votes
	}
	options := make([]dto.PollOption, 0, len(poll.Options))
	for _, opt := range poll.Options {
		percentage := 0.0
		if totalVotes > 0 {
			percentage = (float64(opt.Votes) / float64(totalVotes)) * 100
		}
		isVoted := votedOptionID == opt.ID
		options = append(options, dto.PollOption{
			ID:         opt.ID,
			Text:       opt.Text,
			Votes:      opt.Votes,
			Percentage: percentage,
			IsVoted:    isVoted,
		})
	}
	resp := &dto.PollResponse{
		ID:         poll.ID,
		TweetID:    poll.TweetID,
		Options:    options,
		TotalVotes: totalVotes,
		Duration:   int(poll.Duration.Minutes()),
		ExpiresAt:  poll.ExpiresAt,
		CreatedAt:  poll.CreatedAt,
		UpdatedAt:  poll.UpdatedAt,
		IsExpired:  time.Now().After(poll.ExpiresAt),
		HasVoted:   votedOptionID != "",
		UserVotes:  []string{},
	}
	if votedOptionID != "" {
		resp.UserVotes = []string{votedOptionID}
	}
	return resp
}

// invalidatePollCache invalidates poll caches.
func (s *pollService) invalidatePollCache(ctx context.Context, pollID, tweetID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	keys := []string{
		fmt.Sprintf("poll:%s", pollID),
		fmt.Sprintf("poll:tweet:%s", tweetID),
		fmt.Sprintf("poll_results:%s", pollID),
	}
	if err := s.redisAdapter.Delete(ctx, keys...); err != nil {
		return err
	}
	// Also invalidate list caches (pattern-based)
	_ = s.redisAdapter.Delete(ctx, "poll_stats")
	return nil
}