// backend/internal/service/admin_service.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	ErrAdminUserNotFound     = errors.New("admin user not found")
	ErrAdminActionInvalid    = errors.New("invalid admin action")
	ErrAdminPermissionDenied = errors.New("admin permission denied")
	ErrAdminReportNotFound   = errors.New("report not found")
	ErrAdminTweetNotFound    = errors.New("tweet not found")
	ErrAdminUserSuspended    = errors.New("user is already suspended")
	ErrAdminUserNotSuspended = errors.New("user is not suspended")
	ErrAdminUserVerified     = errors.New("user is already verified")
	ErrAdminUserNotVerified  = errors.New("user is not verified")
	ErrAdminInvalidStatus    = errors.New("invalid user status")
	ErrAdminRoleInvalid      = errors.New("invalid user role")
	ErrAdminSettingsInvalid  = errors.New("invalid system settings")
)

// ======================================================================
// AdminService Interface
// ======================================================================

// AdminService defines the admin service interface.
type AdminService interface {
	// User management
	GetUsers(ctx context.Context, filter *dto.AdminUserFilterRequest) (*dto.UserListResponse, error)
	GetUser(ctx context.Context, userID string) (*dto.UserProfileResponse, error)
	UpdateUser(ctx context.Context, req *dto.AdminUserUpdateRequest) (*dto.UserResponse, error)
	DeleteUser(ctx context.Context, userID string) error
	SuspendUser(ctx context.Context, userID, reason string, duration int) error
	UnsuspendUser(ctx context.Context, userID string) error
	VerifyUser(ctx context.Context, userID string) error
	UnverifyUser(ctx context.Context, userID string) error

	// Tweet moderation
	GetTweets(ctx context.Context, filter *dto.AdminTweetFilterRequest) (*dto.TweetListResponse, error)
	DeleteTweet(ctx context.Context, tweetID, reason string) error
	RestoreTweet(ctx context.Context, tweetID string) error

	// Report management
	GetReports(ctx context.Context, filter *dto.AdminReportFilterRequest) (*dto.ReportListResponse, error)
	ResolveReport(ctx context.Context, reportID, reviewerID, notes string) error
	DismissReport(ctx context.Context, reportID, reviewerID, notes string) error
	EscalateReport(ctx context.Context, reportID, reviewerID, notes string) error
	ReopenReport(ctx context.Context, reportID, reviewerID, notes string) error

	// System settings
	GetSystemSettings(ctx context.Context) (*dto.AdminSystemSettingsResponse, error)
	UpdateSystemSettings(ctx context.Context, req *dto.AdminSystemSettingsRequest) (*dto.AdminSystemSettingsResponse, error)

	// Analytics
	GetAnalytics(ctx context.Context, req *dto.AdminAnalyticsRequest) (*dto.AdminAnalyticsResponse, error)
	GetDashboardStats(ctx context.Context) (*dto.AdminDashboardStats, error)

	// Audit logs
	GetAuditLogs(ctx context.Context, filter *dto.AdminAuditLogFilterRequest) (*dto.AuditLogListResponse, error)
	RecordAuditLog(ctx context.Context, userID, action, resource, ip, userAgent string, details map[string]interface{}) error
}

// ======================================================================
// adminService Implementation
// ======================================================================

// adminService implements AdminService.
type adminService struct {
	userRepo         interfaces.UserRepository
	tweetRepo        interfaces.TweetRepository
	followRepo       interfaces.FollowRepository
	likeRepo         interfaces.LikeRepository
	retweetRepo      interfaces.RetweetRepository
	reportRepo       interfaces.ReportRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	log              *logrus.Entry
}

// NewAdminService creates a new admin service.
func NewAdminService(
	userRepo interfaces.UserRepository,
	tweetRepo interfaces.TweetRepository,
	followRepo interfaces.FollowRepository,
	likeRepo interfaces.LikeRepository,
	retweetRepo interfaces.RetweetRepository,
	reportRepo interfaces.ReportRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) AdminService {
	return &adminService{
		userRepo:         userRepo,
		tweetRepo:        tweetRepo,
		followRepo:       followRepo,
		likeRepo:         likeRepo,
		retweetRepo:      retweetRepo,
		reportRepo:       reportRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		log:              logger.WithField("service", "admin"),
	}
}

// ======================================================================
// User Management
// ======================================================================

// GetUsers returns a paginated list of users with filters.
func (s *adminService) GetUsers(ctx context.Context, filter *dto.AdminUserFilterRequest) (*dto.UserListResponse, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	filter.Sanitize()

	// Build repository filter
	repoFilter := &interfaces.UserFilter{}
	if filter.Username != nil {
		repoFilter.Username = filter.Username
	}
	if filter.Email != nil {
		repoFilter.Email = filter.Email
	}
	if filter.FullName != nil {
		repoFilter.FullName = filter.FullName
	}
	if filter.Role != nil {
		repoFilter.Role = filter.Role
	}
	if filter.Status != nil {
		// Convert status to is_active, is_suspended, deleted_at filters
		if *filter.Status == "active" {
			active := true
			repoFilter.IsActive = &active
		} else if *filter.Status == "suspended" {
			suspended := true
			repoFilter.IsSuspended = &suspended
		} else if *filter.Status == "deleted" {
			// Deleted users are those with deleted_at not null
			// We'll handle this with a custom filter
		}
	}
	if filter.IsVerified != nil {
		repoFilter.IsVerified = filter.IsVerified
	}
	if filter.IsSuspended != nil {
		repoFilter.IsSuspended = filter.IsSuspended
	}
	if filter.JoinedFrom != nil {
		repoFilter.CreatedFrom = filter.JoinedFrom
	}
	if filter.JoinedTo != nil {
		repoFilter.CreatedTo = filter.JoinedTo
	}
	if filter.Search != nil {
		repoFilter.Search = filter.Search
	}

	// Pagination
	pagination := &interfaces.PaginationOptions{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
	if filter.SortBy != "" {
		pagination.SortBy = interfaces.UserSortField(filter.SortBy)
	}
	if filter.SortOrder != "" {
		pagination.Order = interfaces.UserSortOrder(filter.SortOrder)
	}

	users, total, err := s.userRepo.List(ctx, repoFilter, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	responses := make([]dto.UserResponse, 0, len(users))
	for _, user := range users {
		resp := dto.NewUserResponse().
			WithID(user.ID).
			WithUsername(user.Username).
			WithEmail(user.Email).
			WithFullName(user.FullName).
			WithBio(user.Bio).
			WithAvatarURL(user.AvatarURL).
			WithBannerURL(user.BannerURL).
			WithRole(user.Role).
			WithStatus(user.Status).
			WithVerified(user.IsVerified).
			WithPrivate(user.IsPrivate).
			WithCounts(user.TweetCount, user.FollowerCount, user.FollowingCount).
			WithJoinedAt(user.CreatedAt)
		if user.LastActive != nil {
			resp.WithLastActive(*user.LastActive)
		}
		responses = append(responses, *resp)
	}

	return &dto.UserListResponse{
		Data:       responses,
		Total:      total,
		Limit:      filter.Limit,
		HasMore:    false,
	}, nil
}

// GetUser returns a user by ID.
func (s *adminService) GetUser(ctx context.Context, userID string) (*dto.UserProfileResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrAdminUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get counts
	followers, err := s.followRepo.CountFollowers(ctx, user.ID)
	if err != nil {
		followers = 0
	}
	following, err := s.followRepo.CountFollowing(ctx, user.ID)
	if err != nil {
		following = 0
	}
	tweetCount, err := s.tweetRepo.CountByUserID(ctx, user.ID)
	if err != nil {
		tweetCount = 0
	}

	resp := dto.NewUserProfileResponse().
		WithID(user.ID).
		WithUsername(user.Username).
		WithEmail(user.Email).
		WithFullName(user.FullName).
		WithBio(user.Bio).
		WithAvatarURL(user.AvatarURL).
		WithBannerURL(user.BannerURL).
		WithRole(user.Role).
		WithStats(followers, following, tweetCount).
		WithJoinedAt(user.CreatedAt)
	if user.LastActive != nil {
		resp.WithLastActive(*user.LastActive)
	}

	return resp, nil
}

// UpdateUser updates a user (admin).
func (s *adminService) UpdateUser(ctx context.Context, req *dto.AdminUserUpdateRequest) (*dto.UserResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Sanitize()

	user, err := s.userRepo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrAdminUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if req.Username != nil {
		// Check if username is taken
		exists, err := s.userRepo.ExistsByUsername(ctx, *req.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to check username: %w", err)
		}
		if exists && *req.Username != user.Username {
			return nil, ErrDuplicateUsername
		}
		user.Username = *req.Username
	}
	if req.Email != nil {
		exists, err := s.userRepo.ExistsByEmail(ctx, *req.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to check email: %w", err)
		}
		if exists && *req.Email != user.Email {
			return nil, ErrDuplicateEmail
		}
		user.Email = *req.Email
	}
	if req.FullName != nil {
		user.FullName = *req.FullName
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}
	if req.BannerURL != nil {
		user.BannerURL = *req.BannerURL
	}
	if req.Location != nil {
		user.Location = *req.Location
	}
	if req.Website != nil {
		user.Website = *req.Website
	}
	if req.Role != nil {
		if !isValidRole(*req.Role) {
			return nil, ErrAdminRoleInvalid
		}
		user.Role = *req.Role
	}
	if req.Status != nil {
		if !isValidStatus(*req.Status) {
			return nil, ErrAdminInvalidStatus
		}
		user.Status = *req.Status
	}
	if req.IsVerified != nil {
		user.IsVerified = *req.IsVerified
	}
	if req.IsPrivate != nil {
		user.IsPrivate = *req.IsPrivate
	}

	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, req.ID, "admin_update_user", "user", "", "", map[string]interface{}{
		"updated_fields": req,
	})

	resp := dto.NewUserResponse().
		WithID(user.ID).
		WithUsername(user.Username).
		WithEmail(user.Email).
		WithFullName(user.FullName).
		WithBio(user.Bio).
		WithAvatarURL(user.AvatarURL).
		WithBannerURL(user.BannerURL).
		WithRole(user.Role).
		WithStatus(user.Status).
		WithVerified(user.IsVerified).
		WithPrivate(user.IsPrivate).
		WithCounts(user.TweetCount, user.FollowerCount, user.FollowingCount).
		WithJoinedAt(user.CreatedAt)

	return resp, nil
}

// DeleteUser deletes a user (admin).
func (s *adminService) DeleteUser(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrAdminUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if err := s.userRepo.SoftDelete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, userID, "admin_delete_user", "user", "", "", map[string]interface{}{
		"username": user.Username,
		"email":    user.Email,
	})

	return nil
}

// SuspendUser suspends a user.
func (s *adminService) SuspendUser(ctx context.Context, userID, reason string, duration int) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrAdminUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.IsSuspended {
		return ErrAdminUserSuspended
	}

	if err := user.Suspend(reason); err != nil {
		return err
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}

	// Create notification for user
	_ = s.createNotification(ctx, userID, "", "suspended", userID, map[string]interface{}{
		"reason":   reason,
		"duration": duration,
	})

	// Record audit log
	_ = s.RecordAuditLog(ctx, userID, "admin_suspend_user", "user", "", "", map[string]interface{}{
		"username":   user.Username,
		"reason":     reason,
		"duration":   duration,
	})

	return nil
}

// UnsuspendUser unsuspends a user.
func (s *adminService) UnsuspendUser(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrAdminUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if !user.IsSuspended {
		return ErrAdminUserNotSuspended
	}

	if err := user.Unsuspend(); err != nil {
		return err
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to unsuspend user: %w", err)
	}

	// Create notification for user
	_ = s.createNotification(ctx, userID, "", "unsuspended", userID, nil)

	// Record audit log
	_ = s.RecordAuditLog(ctx, userID, "admin_unsuspend_user", "user", "", "", map[string]interface{}{
		"username": user.Username,
	})

	return nil
}

// VerifyUser verifies a user.
func (s *adminService) VerifyUser(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrAdminUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.IsVerified {
		return ErrAdminUserVerified
	}

	if err := user.Verify(); err != nil {
		return err
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, userID, "admin_verify_user", "user", "", "", map[string]interface{}{
		"username": user.Username,
	})

	return nil
}

// UnverifyUser unverifies a user.
func (s *adminService) UnverifyUser(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrAdminUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if !user.IsVerified {
		return ErrAdminUserNotVerified
	}

	if err := user.Unverify(); err != nil {
		return err
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to unverify user: %w", err)
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, userID, "admin_unverify_user", "user", "", "", map[string]interface{}{
		"username": user.Username,
	})

	return nil
}

// ======================================================================
// Tweet Moderation
// ======================================================================

// GetTweets returns a paginated list of tweets for moderation.
func (s *adminService) GetTweets(ctx context.Context, filter *dto.AdminTweetFilterRequest) (*dto.TweetListResponse, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	filter.Sanitize()

	// Build repository filter
	repoFilter := &interfaces.TweetFilter{}
	if filter.UserID != nil {
		repoFilter.UserID = filter.UserID
	}
	if filter.Content != nil {
		repoFilter.Search = filter.Content
	}
	if filter.HasMedia != nil {
		repoFilter.HasMedia = filter.HasMedia
	}
	if filter.IsPoll != nil {
		repoFilter.IsPoll = filter.IsPoll
	}
	if filter.CreatedFrom != nil {
		repoFilter.CreatedFrom = filter.CreatedFrom
	}
	if filter.CreatedTo != nil {
		repoFilter.CreatedTo = filter.CreatedTo
	}
	if filter.MinLikes != nil {
		repoFilter.MinLikes = filter.MinLikes
	}
	if filter.MinRetweets != nil {
		repoFilter.MinRetweets = filter.MinRetweets
	}

	pagination := &interfaces.TweetPagination{
		Limit:  filter.Limit,
		Cursor: "",
	}
	if filter.SortBy != "" {
		pagination.SortBy = interfaces.TweetSortField(filter.SortBy)
	}
	if filter.SortOrder != "" {
		pagination.Order = interfaces.TweetSortOrder(filter.SortOrder)
	}

	tweets, total, err := s.tweetRepo.SearchWithFilters(ctx, "", repoFilter, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to get tweets: %w", err)
	}

	responses := make([]*dto.TweetResponse, 0, len(tweets))
	for _, tweet := range tweets {
		resp, err := s.buildTweetResponse(ctx, tweet, "")
		if err != nil {
			continue
		}
		responses = append(responses, resp)
	}

	return &dto.TweetListResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		HasMore: false,
	}, nil
}

// DeleteTweet deletes a tweet (admin).
func (s *adminService) DeleteTweet(ctx context.Context, tweetID, reason string) error {
	tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrAdminTweetNotFound
		}
		return fmt.Errorf("failed to get tweet: %w", err)
	}

	if err := s.tweetRepo.SoftDelete(ctx, tweetID); err != nil {
		return fmt.Errorf("failed to delete tweet: %w", err)
	}

	// Decrement user tweet count
	_ = s.userRepo.DecrementTweetCount(ctx, tweet.UserID)

	// Record audit log
	_ = s.RecordAuditLog(ctx, "", "admin_delete_tweet", "tweet", "", "", map[string]interface{}{
		"tweet_id": tweetID,
		"content":  tweet.Content,
		"user_id":  tweet.UserID,
		"reason":   reason,
	})

	return nil
}

// RestoreTweet restores a deleted tweet.
func (s *adminService) RestoreTweet(ctx context.Context, tweetID string) error {
	tweet, err := s.tweetRepo.GetByID(ctx, tweetID)
	if err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrAdminTweetNotFound
		}
		return fmt.Errorf("failed to get tweet: %w", err)
	}

	if tweet.DeletedAt == nil {
		return errors.New("tweet is not deleted")
	}

	if err := s.tweetRepo.Restore(ctx, tweetID); err != nil {
		return fmt.Errorf("failed to restore tweet: %w", err)
	}

	// Increment user tweet count
	_ = s.userRepo.IncrementTweetCount(ctx, tweet.UserID)

	// Record audit log
	_ = s.RecordAuditLog(ctx, "", "admin_restore_tweet", "tweet", "", "", map[string]interface{}{
		"tweet_id": tweetID,
		"content":  tweet.Content,
		"user_id":  tweet.UserID,
	})

	return nil
}

// ======================================================================
// Report Management
// ======================================================================

// GetReports returns a paginated list of reports.
func (s *adminService) GetReports(ctx context.Context, filter *dto.AdminReportFilterRequest) (*dto.ReportListResponse, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	filter.Sanitize()

	// Build repository filter
	repoFilter := &interfaces.ReportFilter{}
	if filter.TargetID != nil {
		repoFilter.TargetID = filter.TargetID
	}
	if filter.TargetType != nil {
		repoFilter.TargetType = filter.TargetType
	}
	if filter.ReporterID != nil {
		repoFilter.ReporterID = filter.ReporterID
	}
	if filter.Status != nil {
		repoFilter.Status = filter.Status
	}
	if filter.Severity != nil {
		repoFilter.Severity = filter.Severity
	}
	if filter.ReviewerID != nil {
		repoFilter.ReviewerID = filter.ReviewerID
	}
	if filter.CreatedFrom != nil {
		repoFilter.CreatedFrom = filter.CreatedFrom
	}
	if filter.CreatedTo != nil {
		repoFilter.CreatedTo = filter.CreatedTo
	}

	pagination := &interfaces.ReportPagination{
		Limit:  filter.Limit,
		Cursor: "",
	}
	if filter.SortBy != "" {
		pagination.SortBy = interfaces.ReportSortField(filter.SortBy)
	}
	if filter.SortOrder != "" {
		pagination.Order = interfaces.ReportSortOrder(filter.SortOrder)
	}

	reports, total, err := s.reportRepo.List(ctx, repoFilter, pagination)
	if err != nil {
		return nil, fmt.Errorf("failed to get reports: %w", err)
	}

	responses := make([]*dto.ReportResponse, 0, len(reports))
	for _, report := range reports {
		resp := &dto.ReportResponse{
			ID:          report.ID,
			ReporterID:  report.ReporterID,
			TargetID:    report.TargetID,
			TargetType:  report.TargetType,
			Type:        string(report.Type),
			Reason:      report.Reason,
			Description: report.Description,
			Status:      string(report.Status),
			Severity:    string(report.Severity),
			ReviewerID:  report.ReviewerID,
			ReviewNotes: report.ReviewNotes,
			ResolvedAt:  report.ResolvedAt,
			CreatedAt:   report.CreatedAt,
			UpdatedAt:   report.UpdatedAt,
		}
		responses = append(responses, resp)
	}

	return &dto.ReportListResponse{
		Data:  responses,
		Total: total,
		Limit: filter.Limit,
	}, nil
}

// ResolveReport resolves a report.
func (s *adminService) ResolveReport(ctx context.Context, reportID, reviewerID, notes string) error {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrAdminReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}

	if report.Status == interfaces.ReportStatusResolved {
		return errors.New("report already resolved")
	}

	if err := s.reportRepo.ResolveReport(ctx, reportID, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to resolve report: %w", err)
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, reviewerID, "admin_resolve_report", "report", "", "", map[string]interface{}{
		"report_id": reportID,
		"target_id": report.TargetID,
		"notes":     notes,
	})

	return nil
}

// DismissReport dismisses a report.
func (s *adminService) DismissReport(ctx context.Context, reportID, reviewerID, notes string) error {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrAdminReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}

	if report.Status == interfaces.ReportStatusDismissed {
		return errors.New("report already dismissed")
	}

	if err := s.reportRepo.DismissReport(ctx, reportID, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to dismiss report: %w", err)
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, reviewerID, "admin_dismiss_report", "report", "", "", map[string]interface{}{
		"report_id": reportID,
		"target_id": report.TargetID,
		"notes":     notes,
	})

	return nil
}

// EscalateReport escalates a report.
func (s *adminService) EscalateReport(ctx context.Context, reportID, reviewerID, notes string) error {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrAdminReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}

	if report.Status == interfaces.ReportStatusEscalated {
		return errors.New("report already escalated")
	}

	if err := s.reportRepo.UpdateStatus(ctx, reportID, interfaces.ReportStatusEscalated, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to escalate report: %w", err)
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, reviewerID, "admin_escalate_report", "report", "", "", map[string]interface{}{
		"report_id": reportID,
		"target_id": report.TargetID,
		"notes":     notes,
	})

	return nil
}

// ReopenReport reopens a resolved/dismissed report.
func (s *adminService) ReopenReport(ctx context.Context, reportID, reviewerID, notes string) error {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrAdminReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}

	if report.Status != interfaces.ReportStatusResolved && report.Status != interfaces.ReportStatusDismissed {
		return errors.New("report is not resolved or dismissed")
	}

	if err := s.reportRepo.ReopenReport(ctx, reportID, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to reopen report: %w", err)
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, reviewerID, "admin_reopen_report", "report", "", "", map[string]interface{}{
		"report_id": reportID,
		"target_id": report.TargetID,
		"notes":     notes,
	})

	return nil
}

// ======================================================================
// System Settings
// ======================================================================

// GetSystemSettings returns current system settings.
func (s *adminService) GetSystemSettings(ctx context.Context) (*dto.AdminSystemSettingsResponse, error) {
	cacheKey := "system_settings"
	if s.redisAdapter != nil {
		var cached dto.AdminSystemSettingsResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}

	// Default settings
	settings := &dto.AdminSystemSettingsResponse{
		SiteName:        "Twitter Clone",
		SiteDescription: "A full-featured social media platform",
		MaxTweetLength:  280,
		MaxMediaCount:   4,
		MaxImageSizeMB:  10,
		MaxVideoSizeMB:  50,
		AllowRegistration: true,
		RequireEmailVerification: true,
		DefaultLanguage: "en",
		DefaultTheme:    "light",
		MaintenanceMode: false,
		UpdatedAt:       time.Now().UTC(),
	}

	// Try to load from Redis hash if available
	if s.redisAdapter != nil {
		prefData, err := s.redisAdapter.HGetAll(ctx, "system_settings")
		if err == nil && len(prefData) > 0 {
			if val, ok := prefData["site_name"]; ok {
				settings.SiteName = val
			}
			if val, ok := prefData["site_description"]; ok {
				settings.SiteDescription = val
			}
			if val, ok := prefData["max_tweet_length"]; ok {
				var i int
				fmt.Sscanf(val, "%d", &i)
				if i > 0 {
					settings.MaxTweetLength = i
				}
			}
			if val, ok := prefData["allow_registration"]; ok {
				settings.AllowRegistration = val == "true"
			}
			if val, ok := prefData["maintenance_mode"]; ok {
				settings.MaintenanceMode = val == "true"
			}
		}
	}

	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, settings, 1*time.Hour)
	}

	return settings, nil
}

// UpdateSystemSettings updates system settings.
func (s *adminService) UpdateSystemSettings(ctx context.Context, req *dto.AdminSystemSettingsRequest) (*dto.AdminSystemSettingsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Sanitize()

	// Get current settings
	current, err := s.GetSystemSettings(ctx)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.SiteName != nil {
		current.SiteName = *req.SiteName
	}
	if req.SiteDescription != nil {
		current.SiteDescription = *req.SiteDescription
	}
	if req.MaxTweetLength != nil {
		current.MaxTweetLength = *req.MaxTweetLength
	}
	if req.MaxMediaCount != nil {
		current.MaxMediaCount = *req.MaxMediaCount
	}
	if req.MaxImageSizeMB != nil {
		current.MaxImageSizeMB = *req.MaxImageSizeMB
	}
	if req.MaxVideoSizeMB != nil {
		current.MaxVideoSizeMB = *req.MaxVideoSizeMB
	}
	if req.AllowRegistration != nil {
		current.AllowRegistration = *req.AllowRegistration
	}
	if req.RequireEmailVerification != nil {
		current.RequireEmailVerification = *req.RequireEmailVerification
	}
	if req.DefaultLanguage != nil {
		current.DefaultLanguage = *req.DefaultLanguage
	}
	if req.DefaultTheme != nil {
		current.DefaultTheme = *req.DefaultTheme
	}
	if req.MaintenanceMode != nil {
		current.MaintenanceMode = *req.MaintenanceMode
	}
	if req.MaintenanceMessage != nil {
		current.MaintenanceMessage = *req.MaintenanceMessage
	}
	current.UpdatedAt = time.Now().UTC()

	// Store in Redis
	if s.redisAdapter != nil {
		settingsMap := map[string]interface{}{
			"site_name":        current.SiteName,
			"site_description": current.SiteDescription,
			"max_tweet_length": current.MaxTweetLength,
			"max_media_count":  current.MaxMediaCount,
			"max_image_size_mb": current.MaxImageSizeMB,
			"max_video_size_mb": current.MaxVideoSizeMB,
			"allow_registration": current.AllowRegistration,
			"require_email_verification": current.RequireEmailVerification,
			"default_language": current.DefaultLanguage,
			"default_theme":    current.DefaultTheme,
			"maintenance_mode": current.MaintenanceMode,
			"maintenance_message": current.MaintenanceMessage,
			"updated_at":       current.UpdatedAt.Unix(),
		}
		_ = s.redisAdapter.HSet(ctx, "system_settings", settingsMap)
		_ = s.redisAdapter.Delete(ctx, "system_settings") // Invalidate cache
	}

	// Record audit log
	_ = s.RecordAuditLog(ctx, "", "admin_update_settings", "system", "", "", map[string]interface{}{
		"updated_fields": req,
	})

	return current, nil
}

// ======================================================================
// Analytics
// ======================================================================

// GetAnalytics returns analytics data.
func (s *adminService) GetAnalytics(ctx context.Context, req *dto.AdminAnalyticsRequest) (*dto.AdminAnalyticsResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req.Sanitize()

	// Get stats based on metric
	var result *dto.AdminAnalyticsResponse

	switch req.Metric {
	case "users":
		stats, err := s.getUserAnalytics(ctx, req)
		if err != nil {
			return nil, err
		}
		result = stats
	case "tweets":
		stats, err := s.getTweetAnalytics(ctx, req)
		if err != nil {
			return nil, err
		}
		result = stats
	case "likes":
		stats, err := s.getLikeAnalytics(ctx, req)
		if err != nil {
			return nil, err
		}
		result = stats
	case "retweets":
		stats, err := s.getRetweetAnalytics(ctx, req)
		if err != nil {
			return nil, err
		}
		result = stats
	case "engagement":
		stats, err := s.getEngagementAnalytics(ctx, req)
		if err != nil {
			return nil, err
		}
		result = stats
	case "growth":
		stats, err := s.getGrowthAnalytics(ctx, req)
		if err != nil {
			return nil, err
		}
		result = stats
	default:
		return nil, errors.New("unsupported metric")
	}

	return result, nil
}

// getUserAnalytics returns user analytics.
func (s *adminService) getUserAnalytics(ctx context.Context, req *dto.AdminAnalyticsRequest) (*dto.AdminAnalyticsResponse, error) {
	total, err := s.userRepo.CountTotal(ctx)
	if err != nil {
		return nil, err
	}

	active, err := s.userRepo.Count(ctx, &interfaces.UserFilter{IsActive: boolPtr(true)})
	if err != nil {
		active = 0
	}

	suspended, err := s.userRepo.Count(ctx, &interfaces.UserFilter{IsSuspended: boolPtr(true)})
	if err != nil {
		suspended = 0
	}

	// Daily new users (last 7 days)
	dailyData := []dto.AnalyticsDataPoint{}
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		end := start.Add(24 * time.Hour)
		count, err := s.userRepo.Count(ctx, &interfaces.UserFilter{
			CreatedFrom: &start,
			CreatedTo:   &end,
		})
		if err != nil {
			count = 0
		}
		dailyData = append(dailyData, dto.AnalyticsDataPoint{
			Date:  start,
			Value: float64(count),
		})
	}

	return &dto.AdminAnalyticsResponse{
		Metric:     "users",
		Total:      total,
		Active:     active,
		Inactive:   total - active,
		Suspended:  suspended,
		DailyData:  dailyData,
		Period:     req.Period,
	}, nil
}

// getTweetAnalytics returns tweet analytics.
func (s *adminService) getTweetAnalytics(ctx context.Context, req *dto.AdminAnalyticsRequest) (*dto.AdminAnalyticsResponse, error) {
	stats, err := s.tweetRepo.GetTweetStats(ctx)
	if err != nil {
		return nil, err
	}

	// Daily tweet counts
	dailyData := []dto.AnalyticsDataPoint{}
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		end := start.Add(24 * time.Hour)
		count, err := s.tweetRepo.CountByDateRange(ctx, start, end)
		if err != nil {
			count = 0
		}
		dailyData = append(dailyData, dto.AnalyticsDataPoint{
			Date:  start,
			Value: float64(count),
		})
	}

	return &dto.AdminAnalyticsResponse{
		Metric:      "tweets",
		Total:       stats.TotalTweets,
		Active:      stats.TotalTweets,
		DailyData:   dailyData,
		Period:      req.Period,
	}, nil
}

// getLikeAnalytics returns like analytics.
func (s *adminService) getLikeAnalytics(ctx context.Context, req *dto.AdminAnalyticsRequest) (*dto.AdminAnalyticsResponse, error) {
	stats, err := s.likeRepo.GetLikeStats(ctx)
	if err != nil {
		return nil, err
	}

	return &dto.AdminAnalyticsResponse{
		Metric:   "likes",
		Total:    stats.TotalLikes,
		Active:   stats.UniqueUsers,
		Period:   req.Period,
	}, nil
}

// getRetweetAnalytics returns retweet analytics.
func (s *adminService) getRetweetAnalytics(ctx context.Context, req *dto.AdminAnalyticsRequest) (*dto.AdminAnalyticsResponse, error) {
	stats, err := s.retweetRepo.GetRetweetStats(ctx)
	if err != nil {
		return nil, err
	}

	return &dto.AdminAnalyticsResponse{
		Metric:   "retweets",
		Total:    stats.TotalRetweets,
		Active:   stats.UniqueUsers,
		Period:   req.Period,
	}, nil
}

// getEngagementAnalytics returns engagement analytics.
func (s *adminService) getEngagementAnalytics(ctx context.Context, req *dto.AdminAnalyticsRequest) (*dto.AdminAnalyticsResponse, error) {
	// Calculate engagement rate
	tweetStats, err := s.tweetRepo.GetTweetStats(ctx)
	if err != nil {
		return nil, err
	}

	likeStats, err := s.likeRepo.GetLikeStats(ctx)
	if err != nil {
		return nil, err
	}

	retweetStats, err := s.retweetRepo.GetRetweetStats(ctx)
	if err != nil {
		return nil, err
	}

	engagementRate := 0.0
	if tweetStats.TotalTweets > 0 {
		engagementRate = float64(likeStats.TotalLikes+retweetStats.TotalRetweets) / float64(tweetStats.TotalTweets)
	}

	return &dto.AdminAnalyticsResponse{
		Metric:   "engagement",
		Total:    likeStats.TotalLikes + retweetStats.TotalRetweets,
		Active:   engagementRate,
		Period:   req.Period,
	}, nil
}

// getGrowthAnalytics returns growth analytics.
func (s *adminService) getGrowthAnalytics(ctx context.Context, req *dto.AdminAnalyticsRequest) (*dto.AdminAnalyticsResponse, error) {
	// Get user growth over time
	dailyData := []dto.AnalyticsDataPoint{}
	for i := 30; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		end := start.Add(24 * time.Hour)
		count, err := s.userRepo.Count(ctx, &interfaces.UserFilter{
			CreatedFrom: &start,
			CreatedTo:   &end,
		})
		if err != nil {
			count = 0
		}
		dailyData = append(dailyData, dto.AnalyticsDataPoint{
			Date:  start,
			Value: float64(count),
		})
	}

	return &dto.AdminAnalyticsResponse{
		Metric:    "growth",
		DailyData: dailyData,
		Period:    req.Period,
	}, nil
}

// GetDashboardStats returns dashboard statistics.
func (s *adminService) GetDashboardStats(ctx context.Context) (*dto.AdminDashboardStats, error) {
	// Get user stats
	totalUsers, err := s.userRepo.CountTotal(ctx)
	if err != nil {
		totalUsers = 0
	}

	activeUsers, err := s.userRepo.Count(ctx, &interfaces.UserFilter{IsActive: boolPtr(true)})
	if err != nil {
		activeUsers = 0
	}

	suspendedUsers, err := s.userRepo.Count(ctx, &interfaces.UserFilter{IsSuspended: boolPtr(true)})
	if err != nil {
		suspendedUsers = 0
	}

	// Get tweet stats
	tweetStats, err := s.tweetRepo.GetTweetStats(ctx)
	if err != nil {
		tweetStats = &interfaces.TweetStats{}
	}

	// Get like stats
	likeStats, err := s.likeRepo.GetLikeStats(ctx)
	if err != nil {
		likeStats = &interfaces.LikeStats{}
	}

	// Get retweet stats
	retweetStats, err := s.retweetRepo.GetRetweetStats(ctx)
	if err != nil {
		retweetStats = &interfaces.RetweetStats{}
	}

	// Get report stats
	reportStats, err := s.reportRepo.GetReportStats(ctx)
	if err != nil {
		reportStats = &interfaces.ReportStats{}
	}

	// New users today
	today := time.Now()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	newUsersToday, err := s.userRepo.Count(ctx, &interfaces.UserFilter{
		CreatedFrom: &startOfDay,
	})
	if err != nil {
		newUsersToday = 0
	}

	// New tweets today
	newTweetsToday, err := s.tweetRepo.CountByDateRange(ctx, startOfDay, time.Now())
	if err != nil {
		newTweetsToday = 0
	}

	// Calculate engagement rate
	engagementRate := 0.0
	if tweetStats.TotalTweets > 0 {
		engagementRate = float64(likeStats.TotalLikes+retweetStats.TotalRetweets) / float64(tweetStats.TotalTweets)
	}

	return &dto.AdminDashboardStats{
		TotalUsers:      totalUsers,
		ActiveUsers:     activeUsers,
		SuspendedUsers:  suspendedUsers,
		TotalTweets:     tweetStats.TotalTweets,
		TotalLikes:      likeStats.TotalLikes,
		TotalRetweets:   retweetStats.TotalRetweets,
		TotalReports:    reportStats.TotalReports,
		PendingReports:  reportStats.PendingReports,
		NewUsersToday:   newUsersToday,
		NewTweetsToday:  newTweetsToday,
		EngagementRate:  engagementRate,
		LastUpdated:     time.Now().UTC(),
	}, nil
}

// ======================================================================
// Audit Logs
// ======================================================================

// GetAuditLogs returns audit logs.
func (s *adminService) GetAuditLogs(ctx context.Context, filter *dto.AdminAuditLogFilterRequest) (*dto.AuditLogListResponse, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	filter.Sanitize()

	// This would query the audit logs table
	// For now, return empty response
	return &dto.AuditLogListResponse{
		Data:  []dto.AuditLogResponse{},
		Total: 0,
		Limit: filter.Limit,
	}, nil
}

// RecordAuditLog records an audit log entry.
func (s *adminService) RecordAuditLog(ctx context.Context, userID, action, resource, ip, userAgent string, details map[string]interface{}) error {
	// This would insert into audit logs table
	// For now, just log it
	s.log.WithFields(logrus.Fields{
		"user_id":    userID,
		"action":     action,
		"resource":   resource,
		"ip":         ip,
		"user_agent": userAgent,
		"details":    details,
	}).Info("Audit log")
	return nil
}

// ======================================================================
// Helper Methods
// ======================================================================

// buildTweetResponse builds a tweet response DTO.
func (s *adminService) buildTweetResponse(ctx context.Context, tweet *entities.Tweet, currentUserID string) (*dto.TweetResponse, error) {
	user, err := s.userRepo.GetByID(ctx, tweet.UserID)
	if err != nil {
		return nil, err
	}

	likeCount, err := s.likeRepo.CountByTweetID(ctx, tweet.ID)
	if err != nil {
		likeCount = 0
	}

	retweetCount, err := s.retweetRepo.CountByTweetID(ctx, tweet.ID)
	if err != nil {
		retweetCount = 0
	}

	replyCount, err := s.tweetRepo.CountReplies(ctx, tweet.ID)
	if err != nil {
		replyCount = 0
	}

	liked := false
	retweeted := false
	bookmarked := false
	if currentUserID != "" {
		liked, _ = s.likeRepo.Exists(ctx, tweet.ID, currentUserID)
		retweeted, _ = s.retweetRepo.Exists(ctx, tweet.ID, currentUserID)
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
		Bookmarked:   bookmarked,
		CreatedAt:    tweet.CreatedAt,
		UpdatedAt:    tweet.UpdatedAt,
	}, nil
}

// createNotification creates a notification.
func (s *adminService) createNotification(ctx context.Context, userID, fromUserID, notifType, referenceID string, metadata map[string]interface{}) error {
	notification := &entities.Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		FromUserID:  fromUserID,
		Type:        notifType,
		ReferenceID: referenceID,
		Read:        false,
		CreatedAt:   time.Now(),
	}
	return s.notificationRepo.Create(ctx, notification)
}

// boolPtr returns a pointer to a bool.
func boolPtr(b bool) *bool {
	return &b
}

// isValidRole checks if a role is valid.
func isValidRole(role string) bool {
	return role == entities.UserRoleUser ||
		role == entities.UserRoleModerator ||
		role == entities.UserRoleAdmin
}

// isValidStatus checks if a status is valid.
func isValidStatus(status string) bool {
	return status == entities.UserStatusActive ||
		status == entities.UserStatusInactive ||
		status == entities.UserStatusSuspended ||
		status == entities.UserStatusDeleted
}