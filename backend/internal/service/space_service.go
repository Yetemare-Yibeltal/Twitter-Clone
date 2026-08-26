// backend/internal/service/space_service.go
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

var (
	ErrSpaceNotFound          = errors.New("space not found")
	ErrSpaceAlreadyStarted    = errors.New("space has already started")
	ErrSpaceAlreadyEnded      = errors.New("space has already ended")
	ErrSpaceTitleRequired     = errors.New("space title is required")
	ErrSpaceTitleTooShort     = errors.New("space title is too short")
	ErrSpaceTitleTooLong      = errors.New("space title is too long")
	ErrSpaceDescriptionTooLong = errors.New("space description is too long")
	ErrSpaceTopicTooLong      = errors.New("space topic is too long")
	ErrInvalidSpaceStatus     = errors.New("invalid space status")
	ErrInvalidSpaceVisibility = errors.New("invalid space visibility")
	ErrInvalidSpaceType       = errors.New("invalid space type")
	ErrUserNotInSpace         = errors.New("user is not in this space")
	ErrUserAlreadyInSpace     = errors.New("user is already in this space")
	ErrUserNotSpeaker         = errors.New("user is not a speaker in this space")
	ErrUserAlreadySpeaker     = errors.New("user is already a speaker in this space")
	ErrMaxSpeakersReached     = errors.New("maximum speakers reached")
	ErrMaxListenersReached    = errors.New("maximum listeners reached")
	ErrSpaceFull              = errors.New("space has reached maximum capacity")
	ErrSpaceDurationInvalid   = errors.New("space duration is invalid")
	ErrSpaceSchedulePast      = errors.New("scheduled time cannot be in the past")
	ErrSpaceNotScheduled      = errors.New("space is not scheduled")
	ErrUserNotFound           = errors.New("user not found")
	ErrCannotModifySpace      = errors.New("cannot modify space")
	ErrSpaceAlreadyCancelled  = errors.New("space has already been cancelled")
)

// ======================================================================
// SpaceService Interface
// ======================================================================

// SpaceService defines the space service interface.
type SpaceService interface {
	// Create creates a new space.
	Create(ctx context.Context, userID string, req *dto.CreateSpaceRequest) (*dto.SpaceResponse, error)
	
	// GetByID retrieves a space by ID.
	GetByID(ctx context.Context, spaceID, userID string) (*dto.SpaceDetailResponse, error)
	
	// Update updates a space.
	Update(ctx context.Context, userID string, req *dto.UpdateSpaceRequest) (*dto.SpaceResponse, error)
	
	// Delete deletes a space.
	Delete(ctx context.Context, spaceID, userID string) error
	
	// List returns a paginated list of spaces.
	List(ctx context.Context, req *dto.GetSpacesRequest, userID string) (*dto.SpaceListResponse, error)
	
	// Join adds a user to a space.
	Join(ctx context.Context, spaceID, userID string, asSpeaker bool) (*dto.SpaceJoinResponse, error)
	
	// Leave removes a user from a space.
	Leave(ctx context.Context, spaceID, userID string) (*dto.SpaceLeaveResponse, error)
	
	// GetSpeakers returns speakers in a space.
	GetSpeakers(ctx context.Context, spaceID string, cursor string, limit int) (*dto.SpeakerListResponse, error)
	
	// GetListeners returns listeners in a space.
	GetListeners(ctx context.Context, spaceID string, cursor string, limit int) (*dto.ListenerListResponse, error)
	
	// AddSpeaker adds a user as a speaker.
	AddSpeaker(ctx context.Context, spaceID, userID, targetUserID string) error
	
	// RemoveSpeaker removes a user from speakers.
	RemoveSpeaker(ctx context.Context, spaceID, userID, targetUserID string) error
	
	// MuteSpeaker mutes a speaker.
	MuteSpeaker(ctx context.Context, spaceID, userID, targetUserID string) error
	
	// UnmuteSpeaker unmutes a speaker.
	UnmuteSpeaker(ctx context.Context, spaceID, userID, targetUserID string) error
	
	// EndSpace ends a live space.
	EndSpace(ctx context.Context, spaceID, userID, reason string) (*dto.SpaceEndResponse, error)
	
	// GetSpaceStats returns space statistics.
	GetSpaceStats(ctx context.Context) (*dto.SpaceStatsResponse, error)
	
	// GetUserSpaceStats returns space statistics for a user.
	GetUserSpaceStats(ctx context.Context, userID string) (*dto.SpaceStatsResponse, error)
}

// ======================================================================
// spaceService Implementation
// ======================================================================

// spaceService implements SpaceService.
type spaceService struct {
	spaceRepo        interfaces.SpaceRepository
	userRepo         interfaces.UserRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	log              *logrus.Entry
}

// NewSpaceService creates a new space service.
func NewSpaceService(
	spaceRepo interfaces.SpaceRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) SpaceService {
	return &spaceService{
		spaceRepo:        spaceRepo,
		userRepo:         userRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		log:              logger.WithField("service", "space"),
	}
}

// ======================================================================
// Create Space
// ======================================================================

// Create creates a new space.
func (s *spaceService) Create(ctx context.Context, userID string, req *dto.CreateSpaceRequest) (*dto.SpaceResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Sanitize()
	// Check if user exists
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Create space entity
	space := &entities.Space{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: req.Description,
		Topic:       req.Topic,
		CreatedBy:   userID,
		Visibility:  string(req.Visibility),
		Type:        string(req.Type),
		Status:      string(entities.SpaceStatusScheduled),
		Duration:    req.Duration,
		MaxListeners: req.MaxListeners,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if req.ScheduledAt != nil {
		space.ScheduledAt = req.ScheduledAt
	}
	if err := s.spaceRepo.Create(ctx, space); err != nil {
		return nil, fmt.Errorf("failed to create space: %w", err)
	}
	// Add creator as speaker
	_ = s.spaceRepo.AddSpeaker(ctx, space.ID, userID)
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, space.ID)
	s.log.WithFields(logrus.Fields{
		"user_id": userID,
		"space_id": space.ID,
		"title":   space.Title,
	}).Info("Space created")
	return s.toSpaceResponse(space, userID, true), nil
}

// ======================================================================
// Get Space
// ======================================================================

// GetByID retrieves a space by ID.
func (s *spaceService) GetByID(ctx context.Context, spaceID, userID string) (*dto.SpaceDetailResponse, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("space:%s", spaceID)
	if s.redisAdapter != nil {
		var cached dto.SpaceDetailResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("space_id", spaceID).Debug("Space served from cache")
			return &cached, nil
		}
	}
	// Get from repository
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return nil, ErrSpaceNotFound
	}
	// Check if user is in space
	isJoined := false
	isSpeaker := false
	isHost := false
	if userID != "" {
		isJoined, _ = s.spaceRepo.IsUserInSpace(ctx, spaceID, userID)
		isSpeaker, _ = s.spaceRepo.IsSpeaker(ctx, spaceID, userID)
		isHost = space.CreatedBy == userID
	}
	// Get speakers
	speakers, _, err := s.spaceRepo.GetSpeakers(ctx, spaceID, "", 10)
	if err != nil {
		speakers = []*entities.SpaceSpeaker{}
	}
	// Get listeners count
	listenerCount, err := s.spaceRepo.GetListenerCount(ctx, spaceID)
	if err != nil {
		listenerCount = 0
	}
	// Build response
	resp := s.toSpaceDetailResponse(space, userID, isJoined, isSpeaker, isHost, speakers, listenerCount)
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, resp, 30*time.Second)
	}
	return resp, nil
}

// ======================================================================
// Update Space
// ======================================================================

// Update updates a space.
func (s *spaceService) Update(ctx context.Context, userID string, req *dto.UpdateSpaceRequest) (*dto.SpaceResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Sanitize()
	// Get space
	space, err := s.spaceRepo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return nil, ErrSpaceNotFound
	}
	// Check authorization (only creator can update)
	if space.CreatedBy != userID {
		return nil, ErrCannotModifySpace
	}
	// Check if space can be updated
	if space.Status == string(entities.SpaceStatusEnded) || space.Status == string(entities.SpaceStatusCancelled) {
		return nil, ErrSpaceAlreadyEnded
	}
	// Update fields
	if req.Title != nil {
		space.Title = *req.Title
	}
	if req.Description != nil {
		space.Description = *req.Description
	}
	if req.Topic != nil {
		space.Topic = *req.Topic
	}
	if req.Visibility != nil {
		space.Visibility = string(*req.Visibility)
	}
	if req.Type != nil {
		space.Type = string(*req.Type)
	}
	if req.ScheduledAt != nil {
		if req.ScheduledAt.Before(time.Now()) {
			return nil, ErrSpaceSchedulePast
		}
		space.ScheduledAt = req.ScheduledAt
	}
	if req.Duration != nil {
		space.Duration = *req.Duration
	}
	if req.MaxListeners != nil {
		space.MaxListeners = *req.MaxListeners
	}
	if req.Status != nil {
		if !isValidSpaceStatus(string(*req.Status)) {
			return nil, ErrInvalidSpaceStatus
		}
		space.Status = string(*req.Status)
	}
	space.UpdatedAt = time.Now()
	if err := s.spaceRepo.Update(ctx, space); err != nil {
		return nil, fmt.Errorf("failed to update space: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, space.ID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"space_id": space.ID,
	}).Info("Space updated")
	return s.toSpaceResponse(space, userID, true), nil
}

// ======================================================================
// Delete Space
// ======================================================================

// Delete deletes a space.
func (s *spaceService) Delete(ctx context.Context, spaceID, userID string) error {
	// Get space
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return ErrSpaceNotFound
		}
		return fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return ErrSpaceNotFound
	}
	// Check authorization (only creator can delete)
	if space.CreatedBy != userID {
		return ErrCannotModifySpace
	}
	// Soft delete
	if err := s.spaceRepo.SoftDelete(ctx, spaceID); err != nil {
		return fmt.Errorf("failed to delete space: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, spaceID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"space_id": spaceID,
	}).Info("Space deleted")
	return nil
}

// ======================================================================
// List Spaces
// ======================================================================

// List returns a paginated list of spaces.
func (s *spaceService) List(ctx context.Context, req *dto.GetSpacesRequest, userID string) (*dto.SpaceListResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Sanitize()
	// Build filter
	filter := &interfaces.SpaceFilter{}
	if req.UserID != "" {
		filter.UserID = &req.UserID
	}
	if req.Status != "" {
		filter.Status = &req.Status
	}
	if req.Visibility != "" {
		filter.Visibility = &req.Visibility
	}
	if req.Type != "" {
		filter.Type = &req.Type
	}
	if req.Search != "" {
		filter.Search = &req.Search
	}
	if !req.IncludePast {
		// Only show scheduled and live spaces
		now := time.Now()
		filter.ScheduledFrom = &now
	}
	// Pagination
	pagination := &interfaces.SpacePagination{
		Limit:  req.Limit,
		Cursor: req.Cursor,
	}
	if req.SortBy != "" {
		pagination.SortBy = interfaces.SpaceSortField(req.SortBy)
	}
	if req.SortOrder != "" {
		pagination.Order = interfaces.SpaceSortOrder(req.SortOrder)
	}
	// Get spaces
	spaces, total, err := s.spaceRepo.List(ctx, filter, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to list spaces: %w", err)
	}
	// Build responses
	responses := make([]dto.SpaceResponse, 0, len(spaces))
	for _, space := range spaces {
		resp := s.toSpaceResponse(space, userID, false)
		responses = append(responses, *resp)
	}
	return &dto.SpaceListResponse{
		Data:       responses,
		Total:      total,
		NextCursor: "",
		HasMore:    false,
		Limit:      req.Limit,
	}, nil
}

// ======================================================================
// Join Space
// ======================================================================

// Join adds a user to a space.
func (s *spaceService) Join(ctx context.Context, spaceID, userID string, asSpeaker bool) (*dto.SpaceJoinResponse, error) {
	// Check if user exists
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Get space
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return nil, ErrSpaceNotFound
	}
	// Check if space is active
	if space.Status == string(entities.SpaceStatusEnded) || space.Status == string(entities.SpaceStatusCancelled) {
		return nil, ErrSpaceAlreadyEnded
	}
	// Check if already in space
	isJoined, err := s.spaceRepo.IsUserInSpace(ctx, spaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check user in space: %w", err)
	}
	if isJoined {
		return nil, ErrUserAlreadyInSpace
	}
	// Check capacity
	listenerCount, err := s.spaceRepo.GetListenerCount(ctx, spaceID)
	if err != nil {
		listenerCount = 0
	}
	if space.MaxListeners > 0 && listenerCount >= int64(space.MaxListeners) {
		return nil, ErrMaxListenersReached
	}
	// Check if can join as speaker
	if asSpeaker {
		speakerCount, err := s.spaceRepo.GetSpeakerCount(ctx, spaceID)
		if err != nil {
			speakerCount = 0
		}
		if speakerCount >= MaxSpeakersPerSpace {
			return nil, ErrMaxSpeakersReached
		}
	}
	// Add user to space
	if asSpeaker {
		if err := s.spaceRepo.AddSpeaker(ctx, spaceID, userID); err != nil {
			return nil, fmt.Errorf("failed to add speaker: %w", err)
		}
	} else {
		if err := s.spaceRepo.AddListener(ctx, spaceID, userID); err != nil {
			return nil, fmt.Errorf("failed to add listener: %w", err)
		}
	}
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, spaceID)
	s.log.WithFields(logrus.Fields{
		"user_id":    userID,
		"space_id":   spaceID,
		"as_speaker": asSpeaker,
	}).Info("User joined space")
	role := "listener"
	if asSpeaker {
		role = "speaker"
	}
	return &dto.SpaceJoinResponse{
		Success:  true,
		Message:  "Joined space successfully",
		SpaceID:  spaceID,
		Role:     role,
		JoinedAt: time.Now(),
		IsHost:   space.CreatedBy == userID,
	}, nil
}

// ======================================================================
// Leave Space
// ======================================================================

// Leave removes a user from a space.
func (s *spaceService) Leave(ctx context.Context, spaceID, userID string) (*dto.SpaceLeaveResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Get space
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return nil, ErrSpaceNotFound
	}
	// Check if in space
	isJoined, err := s.spaceRepo.IsUserInSpace(ctx, spaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check user in space: %w", err)
	}
	if !isJoined {
		return nil, ErrUserNotInSpace
	}
	// Check if user is speaker
	isSpeaker, err := s.spaceRepo.IsSpeaker(ctx, spaceID, userID)
	if err != nil {
		isSpeaker = false
	}
	// Remove user from space
	if isSpeaker {
		if err := s.spaceRepo.RemoveSpeaker(ctx, spaceID, userID); err != nil {
			return nil, fmt.Errorf("failed to remove speaker: %w", err)
		}
	} else {
		if err := s.spaceRepo.RemoveListener(ctx, spaceID, userID); err != nil {
			return nil, fmt.Errorf("failed to remove listener: %w", err)
		}
	}
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, spaceID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"space_id": spaceID,
	}).Info("User left space")
	return &dto.SpaceLeaveResponse{
		Success:  true,
		Message:  "Left space successfully",
		SpaceID:  spaceID,
		LeftAt:   time.Now(),
		Duration: 0,
	}, nil
}

// ======================================================================
// Speaker Management
// ======================================================================

// GetSpeakers returns speakers in a space.
func (s *spaceService) GetSpeakers(ctx context.Context, spaceID string, cursor string, limit int) (*dto.SpeakerListResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if space exists
	_, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, fmt.Errorf("failed to get space: %w", err)
	}
	speakers, nextCursor, err := s.spaceRepo.GetSpeakers(ctx, spaceID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get speakers: %w", err)
	}
	responses := make([]*dto.SpeakerResponse, 0, len(speakers))
	for _, sp := range speakers {
		user, err := s.userRepo.GetByID(ctx, sp.UserID)
		if err != nil {
			continue
		}
		resp := &dto.SpeakerResponse{
			UserID:     sp.UserID,
			Username:   user.Username,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			Status:     sp.Status,
			JoinedAt:   sp.JoinedAt,
			IsSpeaking: sp.Status == "speaking",
			IsMuted:    sp.Status == "muted",
			IsHost:     false,
		}
		responses = append(responses, resp)
	}
	return &dto.SpeakerListResponse{
		Data:       responses,
		Total:      int64(len(responses)),
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// GetListeners returns listeners in a space.
func (s *spaceService) GetListeners(ctx context.Context, spaceID string, cursor string, limit int) (*dto.ListenerListResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if space exists
	_, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, fmt.Errorf("failed to get space: %w", err)
	}
	listeners, nextCursor, err := s.spaceRepo.GetListeners(ctx, spaceID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get listeners: %w", err)
	}
	responses := make([]*dto.ListenerResponse, 0, len(listeners))
	for _, ls := range listeners {
		user, err := s.userRepo.GetByID(ctx, ls.UserID)
		if err != nil {
			continue
		}
		resp := &dto.ListenerResponse{
			UserID:    ls.UserID,
			Username:  user.Username,
			FullName:  user.FullName,
			AvatarURL: user.AvatarURL,
			JoinedAt:  ls.JoinedAt,
			IsActive:  true,
		}
		responses = append(responses, resp)
	}
	return &dto.ListenerListResponse{
		Data:       responses,
		Total:      int64(len(responses)),
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	}, nil
}

// AddSpeaker adds a user as a speaker.
func (s *spaceService) AddSpeaker(ctx context.Context, spaceID, userID, targetUserID string) error {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get target user: %w", err)
	}
	// Check if space exists
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return ErrSpaceNotFound
		}
		return fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return ErrSpaceNotFound
	}
	// Check if already speaker
	isSpeaker, err := s.spaceRepo.IsSpeaker(ctx, spaceID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to check speaker status: %w", err)
	}
	if isSpeaker {
		return ErrUserAlreadySpeaker
	}
	// Check speaker limit
	speakerCount, err := s.spaceRepo.GetSpeakerCount(ctx, spaceID)
	if err != nil {
		speakerCount = 0
	}
	if speakerCount >= MaxSpeakersPerSpace {
		return ErrMaxSpeakersReached
	}
	// Add speaker
	if err := s.spaceRepo.AddSpeaker(ctx, spaceID, targetUserID); err != nil {
		return fmt.Errorf("failed to add speaker: %w", err)
	}
	// If user was a listener, remove from listeners
	_ = s.spaceRepo.RemoveListener(ctx, spaceID, targetUserID)
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, spaceID)
	s.log.WithFields(logrus.Fields{
		"space_id":  spaceID,
		"user_id":   userID,
		"target_id": targetUserID,
	}).Info("Speaker added")
	return nil
}

// RemoveSpeaker removes a user from speakers.
func (s *spaceService) RemoveSpeaker(ctx context.Context, spaceID, userID, targetUserID string) error {
	// Check if space exists
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return ErrSpaceNotFound
		}
		return fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return ErrSpaceNotFound
	}
	// Check if target is speaker
	isSpeaker, err := s.spaceRepo.IsSpeaker(ctx, spaceID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to check speaker status: %w", err)
	}
	if !isSpeaker {
		return ErrUserNotSpeaker
	}
	// Remove speaker
	if err := s.spaceRepo.RemoveSpeaker(ctx, spaceID, targetUserID); err != nil {
		return fmt.Errorf("failed to remove speaker: %w", err)
	}
	// Add back as listener
	_ = s.spaceRepo.AddListener(ctx, spaceID, targetUserID)
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, spaceID)
	s.log.WithFields(logrus.Fields{
		"space_id":  spaceID,
		"user_id":   userID,
		"target_id": targetUserID,
	}).Info("Speaker removed")
	return nil
}

// MuteSpeaker mutes a speaker.
func (s *spaceService) MuteSpeaker(ctx context.Context, spaceID, userID, targetUserID string) error {
	// Check if space exists
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return ErrSpaceNotFound
		}
		return fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return ErrSpaceNotFound
	}
	// Check if target is speaker
	isSpeaker, err := s.spaceRepo.IsSpeaker(ctx, spaceID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to check speaker status: %w", err)
	}
	if !isSpeaker {
		return ErrUserNotSpeaker
	}
	// Update speaker status
	if err := s.spaceRepo.UpdateSpeakerStatus(ctx, spaceID, targetUserID, "muted"); err != nil {
		return fmt.Errorf("failed to mute speaker: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, spaceID)
	s.log.WithFields(logrus.Fields{
		"space_id":  spaceID,
		"user_id":   userID,
		"target_id": targetUserID,
	}).Info("Speaker muted")
	return nil
}

// UnmuteSpeaker unmutes a speaker.
func (s *spaceService) UnmuteSpeaker(ctx context.Context, spaceID, userID, targetUserID string) error {
	// Check if space exists
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return ErrSpaceNotFound
		}
		return fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return ErrSpaceNotFound
	}
	// Check if target is speaker
	isSpeaker, err := s.spaceRepo.IsSpeaker(ctx, spaceID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to check speaker status: %w", err)
	}
	if !isSpeaker {
		return ErrUserNotSpeaker
	}
	// Update speaker status
	if err := s.spaceRepo.UpdateSpeakerStatus(ctx, spaceID, targetUserID, "speaking"); err != nil {
		return fmt.Errorf("failed to unmute speaker: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, spaceID)
	s.log.WithFields(logrus.Fields{
		"space_id":  spaceID,
		"user_id":   userID,
		"target_id": targetUserID,
	}).Info("Speaker unmuted")
	return nil
}

// ======================================================================
// End Space
// ======================================================================

// EndSpace ends a live space.
func (s *spaceService) EndSpace(ctx context.Context, spaceID, userID, reason string) (*dto.SpaceEndResponse, error) {
	// Get space
	space, err := s.spaceRepo.GetByID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, interfaces.ErrSpaceNotFound) {
			return nil, ErrSpaceNotFound
		}
		return nil, fmt.Errorf("failed to get space: %w", err)
	}
	if space.DeletedAt != nil {
		return nil, ErrSpaceNotFound
	}
	// Check authorization (only creator can end)
	if space.CreatedBy != userID {
		return nil, ErrCannotModifySpace
	}
	// Check if already ended
	if space.Status == string(entities.SpaceStatusEnded) {
		return nil, ErrSpaceAlreadyEnded
	}
	// Update status
	now := time.Now()
	space.Status = string(entities.SpaceStatusEnded)
	space.EndedAt = &now
	space.UpdatedAt = now
	if err := s.spaceRepo.Update(ctx, space); err != nil {
		return nil, fmt.Errorf("failed to end space: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateSpaceCache(ctx, spaceID)
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"space_id": spaceID,
	}).Info("Space ended")
	return &dto.SpaceEndResponse{
		Success:  true,
		Message:  "Space ended successfully",
		SpaceID:  spaceID,
		EndedAt:  now,
		Duration: space.Duration,
	}, nil
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetSpaceStats returns space statistics.
func (s *spaceService) GetSpaceStats(ctx context.Context) (*dto.SpaceStatsResponse, error) {
	// Try cache
	cacheKey := "space_stats"
	if s.redisAdapter != nil {
		var cached dto.SpaceStatsResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	stats, err := s.spaceRepo.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get space stats: %w", err)
	}
	response := &dto.SpaceStatsResponse{
		TotalSpaces:      stats.TotalSpaces,
		ActiveSpaces:     stats.ActiveSpaces,
		ScheduledSpaces:  stats.ScheduledSpaces,
		EndedSpaces:      stats.EndedSpaces,
		CancelledSpaces:  stats.CancelledSpaces,
		TotalSpeakers:    stats.TotalSpeakers,
		TotalListeners:   stats.TotalListeners,
		AvgSpeakers:      stats.AvgSpeakers,
		AvgListeners:     stats.AvgListeners,
		AvgDuration:      stats.AvgDuration,
		MaxConcurrent:    stats.MaxConcurrent,
		TotalParticipants: stats.TotalParticipants,
	}
	// Cache for 5 minutes
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, response, 5*time.Minute)
	}
	return response, nil
}

// GetUserSpaceStats returns space statistics for a user.
func (s *spaceService) GetUserSpaceStats(ctx context.Context, userID string) (*dto.SpaceStatsResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	stats, err := s.spaceRepo.GetUserStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user space stats: %w", err)
	}
	return &dto.SpaceStatsResponse{
		TotalSpaces:      stats.TotalSpaces,
		ActiveSpaces:     stats.ActiveSpaces,
		ScheduledSpaces:  stats.ScheduledSpaces,
		EndedSpaces:      stats.EndedSpaces,
		CancelledSpaces:  stats.CancelledSpaces,
		TotalSpeakers:    stats.TotalSpeakers,
		TotalListeners:   stats.TotalListeners,
		AvgSpeakers:      stats.AvgSpeakers,
		AvgListeners:     stats.AvgListeners,
		AvgDuration:      stats.AvgDuration,
		MaxConcurrent:    stats.MaxConcurrent,
		TotalParticipants: stats.TotalParticipants,
	}, nil
}

// ======================================================================
// Helper Methods
// ======================================================================

// toSpaceResponse converts a space entity to a response.
func (s *spaceService) toSpaceResponse(space *entities.Space, userID string, isCreator bool) *dto.SpaceResponse {
	resp := &dto.SpaceResponse{
		ID:           space.ID,
		Title:        space.Title,
		Description:  space.Description,
		Topic:        space.Topic,
		CreatedBy:    space.CreatedBy,
		Visibility:   space.Visibility,
		Type:         space.Type,
		Status:       space.Status,
		Duration:     space.Duration,
		MaxListeners: space.MaxListeners,
		CreatedAt:    space.CreatedAt,
		UpdatedAt:    space.UpdatedAt,
		IsCreator:    isCreator,
		IsHost:       isCreator,
	}
	if space.ScheduledAt != nil {
		resp.ScheduledAt = space.ScheduledAt
	}
	if space.StartedAt != nil {
		resp.StartedAt = space.StartedAt
	}
	if space.EndedAt != nil {
		resp.EndedAt = space.EndedAt
	}
	resp.HasStarted = space.StartedAt != nil
	resp.HasEnded = space.EndedAt != nil
	return resp
}

// toSpaceDetailResponse converts a space to a detail response.
func (s *spaceService) toSpaceDetailResponse(space *entities.Space, userID string, isJoined, isSpeaker, isHost bool, speakers []*entities.SpaceSpeaker, listenerCount int64) *dto.SpaceDetailResponse {
	base := s.toSpaceResponse(space, userID, isHost)
	base.IsJoined = isJoined
	base.IsSpeaker = isSpeaker
	base.IsHost = isHost
	base.SpeakerCount = len(speakers)
	base.ListenerCount = int(listenerCount)
	detail := &dto.SpaceDetailResponse{
		SpaceResponse: *base,
	}
	// Add speakers
	for _, sp := range speakers {
		user, _ := s.userRepo.GetByID(context.Background(), sp.UserID)
		if user == nil {
			continue
		}
		speakerResp := &dto.SpeakerResponse{
			UserID:     sp.UserID,
			Username:   user.Username,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			Status:     sp.Status,
			JoinedAt:   sp.JoinedAt,
			IsSpeaking: sp.Status == "speaking",
			IsMuted:    sp.Status == "muted",
			IsHost:     space.CreatedBy == sp.UserID,
		}
		detail.Speakers = append(detail.Speakers, *speakerResp)
	}
	return detail
}

// isValidSpaceStatus checks if a status is valid.
func isValidSpaceStatus(status string) bool {
	valid := map[string]bool{
		"scheduled": true,
		"live":      true,
		"ended":     true,
		"cancelled": true,
	}
	return valid[status]
}

// invalidateSpaceCache invalidates space cache.
func (s *spaceService) invalidateSpaceCache(ctx context.Context, spaceID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	_ = s.redisAdapter.Delete(ctx, fmt.Sprintf("space:%s", spaceID))
	_ = s.redisAdapter.Delete(ctx, "space_stats")
	return nil
}