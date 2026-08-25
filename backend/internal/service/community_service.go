// backend/internal/service/community_service.go
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

var (
	ErrCommunityNotFound      = errors.New("community not found")
	ErrCommunityDeleted       = errors.New("community has been deleted")
	ErrDuplicateSlug          = errors.New("slug already exists")
	ErrMemberNotFound         = errors.New("member not found")
	ErrMemberAlreadyExists    = errors.New("member already exists")
	ErrNotMember              = errors.New("user is not a member")
	ErrNotAdmin               = errors.New("user is not an admin")
	ErrNotModerator           = errors.New("user is not a moderator")
	ErrCannotRemoveOwner      = errors.New("cannot remove the community owner")
	ErrCannotDemoteOwner      = errors.New("cannot demote the community owner")
	ErrUserAlreadyBanned      = errors.New("user is already banned")
	ErrBanNotFound            = errors.New("ban not found")
	ErrPostNotFound           = errors.New("post not found")
	ErrPostAlreadyExists      = errors.New("post already exists in community")
	ErrCommunityNameRequired  = errors.New("community name is required")
	ErrCommunityNameTooLong   = errors.New("community name too long")
	ErrCommunitySlugRequired  = errors.New("community slug is required")
	ErrInvalidSlug            = errors.New("invalid slug format")
	ErrCommunityIsPrivate     = errors.New("community is private")
	ErrCommunityIsFull        = errors.New("community has reached maximum members")
	ErrInvalidRole            = errors.New("invalid role")
)

// ======================================================================
// CommunityService Interface
// ======================================================================

type CommunityService interface {
	Create(ctx context.Context, userID string, req *dto.CreateCommunityRequest) (*dto.CommunityResponse, error)
	GetByID(ctx context.Context, id string, userID string) (*dto.CommunityDetailResponse, error)
	GetBySlug(ctx context.Context, slug string, userID string) (*dto.CommunityDetailResponse, error)
	Update(ctx context.Context, userID, communityID string, req *dto.UpdateCommunityRequest) (*dto.CommunityResponse, error)
	Delete(ctx context.Context, userID, communityID string) error
	List(ctx context.Context, filter *dto.CommunityFilter, pagination *dto.PaginationRequest, userID string) (*dto.CommunityListResponse, error)
	Search(ctx context.Context, query string, pagination *dto.PaginationRequest, userID string) (*dto.CommunityListResponse, error)
	Join(ctx context.Context, userID, communityID string) error
	Leave(ctx context.Context, userID, communityID string) error
	UpdateMemberRole(ctx context.Context, userID, communityID, targetUserID, newRole string) error
	RemoveMember(ctx context.Context, userID, communityID, targetUserID string) error
	BanUser(ctx context.Context, userID, communityID, targetUserID, reason string) error
	UnbanUser(ctx context.Context, userID, communityID, targetUserID string) error
	GetMembers(ctx context.Context, communityID string, role string, cursor string, limit int, userID string) (*dto.MemberListResponse, error)
	GetUserCommunities(ctx context.Context, userID string, cursor string, limit int) (*dto.CommunityListResponse, error)
	AddPost(ctx context.Context, userID, communityID, tweetID string) error
	RemovePost(ctx context.Context, userID, communityID, tweetID string) error
	GetPosts(ctx context.Context, communityID string, cursor string, limit int, userID string) (*dto.CommunityPostListResponse, error)
	GetCommunityStats(ctx context.Context) (*dto.CommunityStatsResponse, error)
	GetUserCommunityStats(ctx context.Context, userID string) (*dto.UserCommunityStatsResponse, error)
	GetTrendingCommunities(ctx context.Context, limit int) ([]*dto.CommunityResponse, error)
	GetRecommendations(ctx context.Context, userID string, limit int) ([]*dto.CommunityResponse, error)
}

// ======================================================================
// communityService Implementation
// ======================================================================

// communityService implements CommunityService.
type communityService struct {
	communityRepo    interfaces.CommunityRepository
	userRepo         interfaces.UserRepository
	tweetRepo        interfaces.TweetRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	log              *logrus.Entry
}

// NewCommunityService creates a new community service.
func NewCommunityService(
	communityRepo interfaces.CommunityRepository,
	userRepo interfaces.UserRepository,
	tweetRepo interfaces.TweetRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) CommunityService {
	return &communityService{
		communityRepo:    communityRepo,
		userRepo:         userRepo,
		tweetRepo:        tweetRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		log:              logger.WithField("service", "community"),
	}
}

// ======================================================================
// Create Community
// ======================================================================

func (s *communityService) Create(ctx context.Context, userID string, req *dto.CreateCommunityRequest) (*dto.CommunityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	slug := req.Slug
	if slug == "" {
		slug = generateSlug(req.Name)
	}
	community := &entities.Community{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		AvatarURL:   req.AvatarURL,
		BannerURL:   req.BannerURL,
		CreatedBy:   userID,
		IsPrivate:   req.IsPrivate,
		MemberCount: 1, // creator is first member
		PostCount:   0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.communityRepo.Create(ctx, community); err != nil {
		if errors.Is(err, interfaces.ErrDuplicateSlug) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("failed to create community: %w", err)
	}
	// Add creator as owner
	if err := s.communityRepo.AddMember(ctx, community.ID, userID, "owner"); err != nil {
		s.log.WithError(err).Warn("Failed to add creator as member")
	}
	// Invalidate cache
	_ = s.invalidateCommunityCache(ctx, community.ID)
	return s.buildCommunityResponse(ctx, community, userID)
}

// ======================================================================
// Get Community
// ======================================================================

func (s *communityService) GetByID(ctx context.Context, id string, userID string) (*dto.CommunityDetailResponse, error) {
	cacheKey := fmt.Sprintf("community:%s", id)
	if s.redisAdapter != nil {
		var cached dto.CommunityDetailResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	community, err := s.communityRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, interfaces.ErrCommunityNotFound) {
			return nil, ErrCommunityNotFound
		}
		return nil, err
	}
	if community.DeletedAt != nil {
		return nil, ErrCommunityDeleted
	}
	// Check if user is member (if private)
	if community.IsPrivate {
		isMember, err := s.communityRepo.IsMember(ctx, id, userID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, ErrCommunityIsPrivate
		}
	}
	resp, err := s.buildCommunityDetailResponse(ctx, community, userID)
	if err != nil {
		return nil, err
	}
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, resp, 1*time.Minute)
	}
	return resp, nil
}

func (s *communityService) GetBySlug(ctx context.Context, slug string, userID string) (*dto.CommunityDetailResponse, error) {
	community, err := s.communityRepo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, interfaces.ErrCommunityNotFound) {
			return nil, ErrCommunityNotFound
		}
		return nil, err
	}
	return s.GetByID(ctx, community.ID, userID)
}

// ======================================================================
// Update Community
// ======================================================================

func (s *communityService) Update(ctx context.Context, userID, communityID string, req *dto.UpdateCommunityRequest) (*dto.CommunityResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	community, err := s.communityRepo.GetByID(ctx, communityID)
	if err != nil {
		return nil, err
	}
	if community.DeletedAt != nil {
		return nil, ErrCommunityDeleted
	}
	// Check admin permission
	isAdmin, err := s.communityRepo.IsAdmin(ctx, communityID, userID)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		return nil, ErrNotAdmin
	}
	if req.Name != "" {
		community.Name = req.Name
	}
	if req.Description != nil {
		community.Description = *req.Description
	}
	if req.AvatarURL != nil {
		community.AvatarURL = *req.AvatarURL
	}
	if req.BannerURL != nil {
		community.BannerURL = *req.BannerURL
	}
	if req.IsPrivate != nil {
		community.IsPrivate = *req.IsPrivate
	}
	community.UpdatedAt = time.Now()
	if err := s.communityRepo.Update(ctx, community); err != nil {
		return nil, fmt.Errorf("failed to update community: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, community.ID)
	return s.buildCommunityResponse(ctx, community, userID)
}

// ======================================================================
// Delete Community
// ======================================================================

func (s *communityService) Delete(ctx context.Context, userID, communityID string) error {
	community, err := s.communityRepo.GetByID(ctx, communityID)
	if err != nil {
		return err
	}
	if community.DeletedAt != nil {
		return ErrCommunityDeleted
	}
	// Check if user is owner
	role, err := s.communityRepo.GetMemberRole(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if role != "owner" {
		return ErrNotAdmin
	}
	if err := s.communityRepo.SoftDelete(ctx, communityID); err != nil {
		return fmt.Errorf("failed to delete community: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, community.ID)
	return nil
}

// ======================================================================
// List and Search
// ======================================================================

func (s *communityService) List(ctx context.Context, filter *dto.CommunityFilter, pagination *dto.PaginationRequest, userID string) (*dto.CommunityListResponse, error) {
	if pagination == nil {
		pagination = &dto.PaginationRequest{Limit: 20, Offset: 0}
	}
	if pagination.Limit < 1 || pagination.Limit > 100 {
		pagination.Limit = 20
	}
	// Build filter
	repoFilter := &interfaces.CommunityFilter{}
	if filter != nil {
		if filter.Name != nil {
			repoFilter.Name = filter.Name
		}
		if filter.Slug != nil {
			repoFilter.Slug = filter.Slug
		}
		if filter.CreatedBy != nil {
			repoFilter.CreatedBy = filter.CreatedBy
		}
		if filter.IsPrivate != nil {
			repoFilter.IsPrivate = filter.IsPrivate
		}
		if filter.HasMember != nil {
			repoFilter.HasMember = filter.HasMember
		}
	}
	repoPagination := &interfaces.CommunityPagination{
		Limit:  pagination.Limit,
		Cursor: pagination.Cursor,
	}
	communities, total, err := s.communityRepo.List(ctx, repoFilter, repoPagination)
	if err != nil {
		return nil, fmt.Errorf("failed to list communities: %w", err)
	}
	responses := make([]*dto.CommunityResponse, 0, len(communities))
	for _, c := range communities {
		resp, err := s.buildCommunityResponse(ctx, c, userID)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	return &dto.CommunityListResponse{
		Data:       responses,
		Total:      total,
		NextCursor: "",
		HasMore:    false,
	}, nil
}

func (s *communityService) Search(ctx context.Context, query string, pagination *dto.PaginationRequest, userID string) (*dto.CommunityListResponse, error) {
	if pagination == nil {
		pagination = &dto.PaginationRequest{Limit: 20, Offset: 0}
	}
	if pagination.Limit < 1 || pagination.Limit > 100 {
		pagination.Limit = 20
	}
	repoPagination := &interfaces.CommunityPagination{
		Limit:  pagination.Limit,
		Cursor: pagination.Cursor,
	}
	communities, total, err := s.communityRepo.Search(ctx, query, repoPagination)
	if err != nil {
		return nil, fmt.Errorf("failed to search communities: %w", err)
	}
	responses := make([]*dto.CommunityResponse, 0, len(communities))
	for _, c := range communities {
		resp, err := s.buildCommunityResponse(ctx, c, userID)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	return &dto.CommunityListResponse{
		Data:       responses,
		Total:      total,
		NextCursor: "",
		HasMore:    false,
	}, nil
}

// ======================================================================
// Membership Management
// ======================================================================

func (s *communityService) Join(ctx context.Context, userID, communityID string) error {
	community, err := s.communityRepo.GetByID(ctx, communityID)
	if err != nil {
		return err
	}
	if community.DeletedAt != nil {
		return ErrCommunityDeleted
	}
	if community.IsPrivate {
		return ErrCommunityIsPrivate
	}
	isMember, err := s.communityRepo.IsMember(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if isMember {
		return ErrMemberAlreadyExists
	}
	banned, err := s.communityRepo.IsBanned(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if banned {
		return ErrUserAlreadyBanned
	}
	if err := s.communityRepo.AddMember(ctx, communityID, userID, "member"); err != nil {
		return fmt.Errorf("failed to join community: %w", err)
	}
	// Create notification for admins
	_ = s.notifyAdmins(ctx, communityID, userID, "joined")
	_ = s.invalidateCommunityCache(ctx, community.ID)
	return nil
}

func (s *communityService) Leave(ctx context.Context, userID, communityID string) error {
	community, err := s.communityRepo.GetByID(ctx, communityID)
	if err != nil {
		return err
	}
	if community.DeletedAt != nil {
		return ErrCommunityDeleted
	}
	role, err := s.communityRepo.GetMemberRole(ctx, communityID, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrMemberNotFound) {
			return ErrNotMember
		}
		return err
	}
	if role == "owner" {
		return errors.New("cannot leave as owner; transfer ownership first")
	}
	if err := s.communityRepo.RemoveMember(ctx, communityID, userID); err != nil {
		return fmt.Errorf("failed to leave community: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, community.ID)
	return nil
}

// ======================================================================
= Member Role Management
// ======================================================================

func (s *communityService) UpdateMemberRole(ctx context.Context, userID, communityID, targetUserID, newRole string) error {
	// Check if current user is admin/owner
	isAdmin, err := s.communityRepo.IsAdmin(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotAdmin
	}
	// Get target member role
	targetRole, err := s.communityRepo.GetMemberRole(ctx, communityID, targetUserID)
	if err != nil {
		if errors.Is(err, interfaces.ErrMemberNotFound) {
			return ErrNotMember
		}
		return err
	}
	// Cannot change owner
	if targetRole == "owner" {
		return ErrCannotDemoteOwner
	}
	// Validate new role
	validRoles := map[string]bool{"admin": true, "moderator": true, "member": true}
	if !validRoles[newRole] {
		return ErrInvalidRole
	}
	if err := s.communityRepo.UpdateMemberRole(ctx, communityID, targetUserID, newRole); err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, communityID)
	return nil
}

func (s *communityService) RemoveMember(ctx context.Context, userID, communityID, targetUserID string) error {
	// Check if current user is admin/owner
	isAdmin, err := s.communityRepo.IsAdmin(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotAdmin
	}
	targetRole, err := s.communityRepo.GetMemberRole(ctx, communityID, targetUserID)
	if err != nil {
		if errors.Is(err, interfaces.ErrMemberNotFound) {
			return ErrNotMember
		}
		return err
	}
	if targetRole == "owner" {
		return ErrCannotRemoveOwner
	}
	if err := s.communityRepo.RemoveMember(ctx, communityID, targetUserID); err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, communityID)
	return nil
}

// ======================================================================
= Ban Management
// ======================================================================

func (s *communityService) BanUser(ctx context.Context, userID, communityID, targetUserID, reason string) error {
	isAdmin, err := s.communityRepo.IsAdmin(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotAdmin
	}
	banned, err := s.communityRepo.IsBanned(ctx, communityID, targetUserID)
	if err != nil {
		return err
	}
	if banned {
		return ErrUserAlreadyBanned
	}
	if err := s.communityRepo.BanUser(ctx, communityID, targetUserID, reason); err != nil {
		return fmt.Errorf("failed to ban user: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, communityID)
	return nil
}

func (s *communityService) UnbanUser(ctx context.Context, userID, communityID, targetUserID string) error {
	isAdmin, err := s.communityRepo.IsAdmin(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return ErrNotAdmin
	}
	if err := s.communityRepo.UnbanUser(ctx, communityID, targetUserID); err != nil {
		if errors.Is(err, interfaces.ErrBanNotFound) {
			return ErrBanNotFound
		}
		return fmt.Errorf("failed to unban user: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, communityID)
	return nil
}

// ======================================================================
= Get Members
// ======================================================================

func (s *communityService) GetMembers(ctx context.Context, communityID string, role string, cursor string, limit int, userID string) (*dto.MemberListResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check if community exists
	_, err := s.communityRepo.GetByID(ctx, communityID)
	if err != nil {
		return nil, err
	}
	members, nextCursor, err := s.communityRepo.GetMembers(ctx, communityID, role, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get members: %w", err)
	}
	total, err := s.communityRepo.GetMemberCount(ctx, communityID)
	if err != nil {
		total = int64(len(members))
	}
	responses := make([]*dto.MemberResponse, 0, len(members))
	for _, m := range members {
		responses = append(responses, &dto.MemberResponse{
			UserID:    m.UserID,
			Username:  m.Username,
			FullName:  m.FullName,
			AvatarURL: m.AvatarURL,
			Role:      m.Role,
			JoinedAt:  m.JoinedAt,
			IsActive:  m.IsActive,
		})
	}
	return &dto.MemberListResponse{
		Data:       responses,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
		Total:      total,
	}, nil
}

// ======================================================================
= Get User Communities
// ======================================================================

func (s *communityService) GetUserCommunities(ctx context.Context, userID string, cursor string, limit int) (*dto.CommunityListResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	communities, nextCursor, err := s.communityRepo.GetUserCommunities(ctx, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get user communities: %w", err)
	}
	responses := make([]*dto.CommunityResponse, 0, len(communities))
	for _, c := range communities {
		resp, err := s.buildCommunityResponse(ctx, c, userID)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	total, err := s.communityRepo.CountByMemberID(ctx, userID)
	if err != nil {
		total = int64(len(responses))
	}
	return &dto.CommunityListResponse{
		Data:       responses,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
		Total:      total,
	}, nil
}

// ======================================================================
= Community Posts
// ======================================================================

func (s *communityService) AddPost(ctx context.Context, userID, communityID, tweetID string) error {
	// Check membership
	isMember, err := s.communityRepo.IsMember(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotMember
	}
	// Check if tweet exists
	_, err = s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return err
	}
	if err := s.communityRepo.AddPost(ctx, communityID, tweetID); err != nil {
		if errors.Is(err, interfaces.ErrPostAlreadyExists) {
			return ErrPostAlreadyExists
		}
		return fmt.Errorf("failed to add post: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, communityID)
	return nil
}

func (s *communityService) RemovePost(ctx context.Context, userID, communityID, tweetID string) error {
	// Check admin/moderator permission
	isModerator, err := s.communityRepo.IsModerator(ctx, communityID, userID)
	if err != nil {
		return err
	}
	if !isModerator {
		return ErrNotModerator
	}
	if err := s.communityRepo.RemovePost(ctx, communityID, tweetID); err != nil {
		if errors.Is(err, interfaces.ErrPostNotFound) {
			return ErrPostNotFound
		}
		return fmt.Errorf("failed to remove post: %w", err)
	}
	_ = s.invalidateCommunityCache(ctx, communityID)
	return nil
}

// GetPosts returns posts in a community.
func (s *communityService) GetPosts(ctx context.Context, communityID string, cursor string, limit int, userID string) (*dto.CommunityPostListResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	// Check membership if private
	community, err := s.communityRepo.GetByID(ctx, communityID)
	if err != nil {
		return nil, err
	}
	if community.IsPrivate {
		isMember, err := s.communityRepo.IsMember(ctx, communityID, userID)
		if err != nil || !isMember {
			return nil, ErrCommunityIsPrivate
		}
	}
	tweets, nextCursor, err := s.communityRepo.GetPosts(ctx, communityID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts: %w", err)
	}
	total, err := s.communityRepo.GetPostCount(ctx, communityID)
	if err != nil {
		total = int64(len(tweets))
	}
	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, t := range tweets {
		resp, err := buildTweetResponse(ctx, t, userID, s.userRepo, s.likeRepo, s.retweetRepo, s.tweetRepo)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	return &dto.CommunityPostListResponse{
		Data:       responses,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
		Total:      total,
	}, nil
}

// ======================================================================
= Stats and Analytics
// ======================================================================

func (s *communityService) GetCommunityStats(ctx context.Context) (*dto.CommunityStatsResponse, error) {
	stats, err := s.communityRepo.GetCommunityStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get community stats: %w", err)
	}
	return &dto.CommunityStatsResponse{
		TotalCommunities:   stats.TotalCommunities,
		PublicCommunities:  stats.PublicCommunities,
		PrivateCommunities: stats.PrivateCommunities,
		TotalMembers:       stats.TotalMembers,
		TotalPosts:         stats.TotalPosts,
		AverageMembers:     stats.AverageMembers,
		AveragePosts:       stats.AveragePosts,
	}, nil
}

func (s *communityService) GetUserCommunityStats(ctx context.Context, userID string) (*dto.UserCommunityStatsResponse, error) {
	created, err := s.communityRepo.CountByUserID(ctx, userID)
	if err != nil {
		created = 0
	}
	joined, err := s.communityRepo.CountByMemberID(ctx, userID)
	if err != nil {
		joined = 0
	}
	return &dto.UserCommunityStatsResponse{
		UserID:         userID,
		CommunitiesCreated: created,
		CommunitiesJoined:  joined,
	}, nil
}

// ======================================================================
= Trending and Recommendations
// ======================================================================

func (s *communityService) GetTrendingCommunities(ctx context.Context, limit int) ([]*dto.CommunityResponse, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	cacheKey := fmt.Sprintf("trending_communities:%d", limit)
	if s.redisAdapter != nil {
		var cached []*dto.CommunityResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}
	communities, err := s.communityRepo.GetTrendingCommunities(ctx, limit, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("failed to get trending communities: %w", err)
	}
	responses := make([]*dto.CommunityResponse, 0, len(communities))
	for _, c := range communities {
		resp, err := s.buildCommunityResponse(ctx, c, "")
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	if s.redisAdapter != nil && len(responses) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, responses, 5*time.Minute)
	}
	return responses, nil
}

func (s *communityService) GetRecommendations(ctx context.Context, userID string, limit int) ([]*dto.CommunityResponse, error) {
	if limit < 1 || limit > 50 {
		limit = 10
	}
	cacheKey := fmt.Sprintf("community_recommendations:%s:%d", userID, limit)
	if s.redisAdapter != nil {
		var cached []*dto.CommunityResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}
	communities, err := s.communityRepo.GetRecommendations(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %w", err)
	}
	responses := make([]*dto.CommunityResponse, 0, len(communities))
	for _, c := range communities {
		resp, err := s.buildCommunityResponse(ctx, c, userID)
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}
	if s.redisAdapter != nil && len(responses) > 0 {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, responses, 5*time.Minute)
	}
	return responses, nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// buildCommunityResponse builds a community response DTO.
func (s *communityService) buildCommunityResponse(ctx context.Context, community *entities.Community, userID string) (*dto.CommunityResponse, error) {
	isMember := false
	isAdmin := false
	if userID != "" {
		isMember, _ = s.communityRepo.IsMember(ctx, community.ID, userID)
		isAdmin, _ = s.communityRepo.IsAdmin(ctx, community.ID, userID)
	}
	return &dto.CommunityResponse{
		ID:          community.ID,
		Name:        community.Name,
		Slug:        community.Slug,
		Description: community.Description,
		AvatarURL:   community.AvatarURL,
		BannerURL:   community.BannerURL,
		CreatedBy:   community.CreatedBy,
		IsPrivate:   community.IsPrivate,
		MemberCount: community.MemberCount,
		PostCount:   community.PostCount,
		IsMember:    isMember,
		IsAdmin:     isAdmin,
		CreatedAt:   community.CreatedAt,
		UpdatedAt:   community.UpdatedAt,
	}, nil
}

// buildCommunityDetailResponse builds a detailed community response.
func (s *communityService) buildCommunityDetailResponse(ctx context.Context, community *entities.Community, userID string) (*dto.CommunityDetailResponse, error) {
	resp, err := s.buildCommunityResponse(ctx, community, userID)
	if err != nil {
		return nil, err
	}
	// Get creator info
	creator, err := s.userRepo.GetByID(ctx, community.CreatedBy)
	if err == nil {
		resp.CreatorUsername = creator.Username
		resp.CreatorAvatar = creator.AvatarURL
	}
	return &dto.CommunityDetailResponse{
		CommunityResponse: resp,
		CreatorUsername:   resp.CreatorUsername,
		CreatorAvatar:     resp.CreatorAvatar,
	}, nil
}

// notifyAdmins creates notifications for community admins.
func (s *communityService) notifyAdmins(ctx context.Context, communityID, userID, action string) error {
	members, _, err := s.communityRepo.GetMembers(ctx, communityID, "admin", "", 100)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.UserID == userID {
			continue
		}
		notif := &entities.Notification{
			ID:          uuid.New().String(),
			UserID:      m.UserID,
			FromUserID:  userID,
			Type:        "community_" + action,
			ReferenceID: communityID,
			Read:        false,
			CreatedAt:   time.Now(),
		}
		_ = s.notificationRepo.Create(ctx, notif)
	}
	return nil
}

// invalidateCommunityCache invalidates community cache.
func (s *communityService) invalidateCommunityCache(ctx context.Context, communityID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	_ = s.redisAdapter.Delete(ctx, fmt.Sprintf("community:%s", communityID))
	_ = s.redisAdapter.Delete(ctx, "trending_communities:*")
	return nil
}

// generateSlug generates a slug from a name.
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	replacer := strings.NewReplacer(
		".", "", ",", "", "!", "", "?", "", "'", "", 
		"\"", "", "(", "", ")", "", "[", "", "]", "",
		"{", "", "}", "", ":", "", ";", "", "@", "",
		"#", "", "$", "", "%", "", "^", "", "&", "",
		"*", "", "+", "", "=", "", "`", "", "~", "",
		"|", "", "\\", "", "/", "", ">", "", "<", "",
	)
	slug = replacer.Replace(slug)
	// Collapse multiple hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if len(slug) > 50 {
		slug = slug[:50]
	}
	if slug == "" {
		slug = "community-" + uuid.New().String()[:8]
	}
	return slug
}

// buildTweetResponse helper (placeholder; actual implementation would use service method)
func buildTweetResponse(ctx context.Context, tweet *entities.Tweet, currentUserID string,
	userRepo interfaces.UserRepository, likeRepo interfaces.LikeRepository,
	retweetRepo interfaces.RetweetRepository, tweetRepo interfaces.TweetRepository) (*dto.TweetResponse, error) {
	// This is a simplified version; in production, inject the actual tweet service method
	user, err := userRepo.GetByID(ctx, tweet.UserID)
	if err != nil {
		return nil, err
	}
	likeCount, _ := likeRepo.CountByTweetID(ctx, tweet.ID)
	retweetCount, _ := retweetRepo.CountByTweetID(ctx, tweet.ID)
	replyCount, _ := tweetRepo.CountReplies(ctx, tweet.ID)
	liked := false
	retweeted := false
	if currentUserID != "" {
		liked, _ = likeRepo.Exists(ctx, tweet.ID, currentUserID)
		retweeted, _ = retweetRepo.Exists(ctx, tweet.ID, currentUserID)
	}
	return &dto.TweetResponse{
		ID:           tweet.ID,
		Content:      tweet.Content,
		UserID:       tweet.UserID,
		Username:     user.Username,
		FullName:     user.FullName,
		AvatarURL:    user.AvatarURL,
		MediaURLs:    tweet.MediaURLs,
		LikeCount:    likeCount,
		RetweetCount: retweetCount,
		ReplyCount:   replyCount,
		Liked:        liked,
		Retweeted:    retweeted,
		CreatedAt:    tweet.CreatedAt,
		UpdatedAt:    tweet.UpdatedAt,
	}, nil
}