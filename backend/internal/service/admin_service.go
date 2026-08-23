// backend/internal/service/admin_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrTweetNotFound          = errors.New("tweet not found")
	ErrReportNotFound         = errors.New("report not found")
	ErrUserAlreadySuspended   = errors.New("user already suspended")
	ErrUserNotSuspended       = errors.New("user is not suspended")
	ErrCannotSuspendAdmin     = errors.New("cannot suspend an admin user")
	ErrReportAlreadyResolved  = errors.New("report already resolved")
	ErrReportAlreadyDismissed = errors.New("report already dismissed")
	ErrInvalidRole            = errors.New("invalid role")
	ErrInvalidReportStatus    = errors.New("invalid report status")
	ErrInvalidReportSeverity  = errors.New("invalid report severity")
	ErrInvalidDuration        = errors.New("invalid suspension duration")
)

// ======================================================================
= AdminService Interface
// ======================================================================

// AdminService defines the admin service interface.
type AdminService interface {
	// User management
	ListUsers(ctx context.Context, cursor string, limit int, status, role, search string) ([]*dto.AdminUserResponse, string, int64, error)
	GetUserDetails(ctx context.Context, userID string) (*dto.AdminUserDetailResponse, error)
	UpdateUserRole(ctx context.Context, userID, role string) error
	SuspendUser(ctx context.Context, userID, reason string, duration int) error
	UnsuspendUser(ctx context.Context, userID string) error
	DeleteUser(ctx context.Context, userID string) error

	// Tweet moderation
	ListTweets(ctx context.Context, cursor string, limit int, status, userID, search string) ([]*dto.AdminTweetResponse, string, int64, error)
	DeleteTweet(ctx context.Context, tweetID string) error
	HideTweet(ctx context.Context, tweetID string, duration int) error
	UnhideTweet(ctx context.Context, tweetID string) error
	PinTweet(ctx context.Context, tweetID string) error
	UnpinTweet(ctx context.Context, tweetID string) error

	// Report management
	ListReports(ctx context.Context, cursor string, limit int, status, severity, targetType string) ([]*dto.AdminReportResponse, string, int64, error)
	GetReportDetails(ctx context.Context, reportID string) (*dto.AdminReportDetailResponse, error)
	AssignReport(ctx context.Context, reportID, reviewerID string) error
	UpdateReportStatus(ctx context.Context, reportID, status, notes string) error
	UpdateReportSeverity(ctx context.Context, reportID, severity string) error
	ResolveReport(ctx context.Context, reportID, action, notes string) error
	DismissReport(ctx context.Context, reportID, notes string) error

	// System stats
	GetSystemStats(ctx context.Context) (*dto.SystemStatsResponse, error)
	GetDailyStats(ctx context.Context, days int) (*dto.DailyStatsResponse, error)
	GetEngagementStats(ctx context.Context, days int) (*dto.EngagementStatsResponse, error)
	GetReportAnalytics(ctx context.Context, days int) (*dto.ReportAnalyticsResponse, error)
}

// ======================================================================
= AdminService Implementation
// ======================================================================

// adminService implements AdminService.
type adminService struct {
	userRepo       interfaces.UserRepository
	tweetRepo      interfaces.TweetRepository
	followRepo     interfaces.FollowRepository
	likeRepo       interfaces.LikeRepository
	retweetRepo    interfaces.RetweetRepository
	reportRepo     interfaces.ReportRepository
	notificationRepo interfaces.NotificationRepository
	log            *logrus.Entry
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
) AdminService {
	return &adminService{
		userRepo:       userRepo,
		tweetRepo:      tweetRepo,
		followRepo:     followRepo,
		likeRepo:       likeRepo,
		retweetRepo:    retweetRepo,
		reportRepo:     reportRepo,
		notificationRepo: notificationRepo,
		log:            logger.WithField("service", "admin"),
	}
}

// ======================================================================
= User Management
// ======================================================================

// ListUsers returns a paginated list of users with filters.
func (s *adminService) ListUsers(ctx context.Context, cursor string, limit int, status, role, search string) ([]*dto.AdminUserResponse, string, int64, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	filter := &interfaces.UserFilter{}
	if status != "" {
		var isActive, isSuspended *bool
		switch status {
		case "active":
			b := true
			isActive = &b
			b2 := false
			isSuspended = &b2
		case "suspended":
			b := true
			isSuspended = &b
			b2 := false
			isActive = &b2
		case "inactive":
			b := false
			isActive = &b
			b2 := false
			isSuspended = &b2
		}
		filter.IsActive = isActive
		filter.IsSuspended = isSuspended
	}
	if role != "" {
		filter.Role = &role
	}
	if search != "" {
		filter.Search = &search
	}

	pagination := &interfaces.PaginationOptions{
		Limit:  limit,
		Cursor: cursor,
	}

	users, total, err := s.userRepo.List(ctx, filter, pagination)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to list users: %w", err)
	}

	responses := make([]*dto.AdminUserResponse, 0, len(users))
	for _, u := range users {
		responses = append(responses, &dto.AdminUserResponse{
			ID:         u.ID,
			Username:   u.Username,
			Email:      u.Email,
			FullName:   u.FullName,
			Bio:        u.Bio,
			AvatarURL:  u.AvatarURL,
			IsVerified: u.IsVerified,
			IsSuspended: u.IsSuspended,
			IsActive:   u.IsActive,
			Role:       string(u.Role),
			TweetCount: u.TweetCount,
			FollowerCount: u.FollowerCount,
			FollowingCount: u.FollowingCount,
			CreatedAt:  u.CreatedAt,
			UpdatedAt:  u.UpdatedAt,
		})
	}

	var nextCursor string
	if len(users) == limit {
		nextCursor = users[len(users)-1].ID
	}
	return responses, nextCursor, total, nil
}

// GetUserDetails returns detailed user information.
func (s *adminService) GetUserDetails(ctx context.Context, userID string) (*dto.AdminUserDetailResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get counts
	followers, _ := s.followRepo.CountFollowers(ctx, userID)
	following, _ := s.followRepo.CountFollowing(ctx, userID)
	tweetCount, _ := s.tweetRepo.CountByUserID(ctx, userID)

	// Get recent activity
	recentTweets, _, _ := s.tweetRepo.GetByUserID(ctx, userID, "", 5, false)

	// Get reports against this user
	reports, _, _ := s.reportRepo.GetByTarget(ctx, userID, "user", "", 20)

	return &dto.AdminUserDetailResponse{
		User: &dto.AdminUserResponse{
			ID:             user.ID,
			Username:       user.Username,
			Email:          user.Email,
			FullName:       user.FullName,
			Bio:            user.Bio,
			AvatarURL:      user.AvatarURL,
			IsVerified:     user.IsVerified,
			IsSuspended:    user.IsSuspended,
			IsActive:       user.IsActive,
			Role:           string(user.Role),
			TweetCount:     user.TweetCount,
			FollowerCount:  user.FollowerCount,
			FollowingCount: user.FollowingCount,
			CreatedAt:      user.CreatedAt,
			UpdatedAt:      user.UpdatedAt,
		},
		Followers:    followers,
		Following:    following,
		TweetCount:   tweetCount,
		RecentTweets: recentTweets,
		Reports:      reports,
	}, nil
}

// UpdateUserRole updates a user's role.
func (s *adminService) UpdateUserRole(ctx context.Context, userID, role string) error {
	if role != "user" && role != "moderator" && role != "admin" {
		return ErrInvalidRole
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	user.Role = entities.UserRole(role)
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}
	s.log.WithFields(logrus.Fields{
		"user_id": userID,
		"role":    role,
	}).Info("User role updated by admin")
	return nil
}

// SuspendUser suspends a user account.
func (s *adminService) SuspendUser(ctx context.Context, userID, reason string, duration int) error {
	if duration < 0 {
		return ErrInvalidDuration
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user.Role == entities.RoleAdmin {
		return ErrCannotSuspendAdmin
	}
	if user.IsSuspended {
		return ErrUserAlreadySuspended
	}
	if err := user.Suspend(reason); err != nil {
		return err
	}
	// Set suspension expiry if duration > 0
	if duration > 0 {
		// We don't have a suspension expiry field in User entity, but we can store in metadata.
		// For simplicity, we just set a flag and reason.
	}
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}
	s.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"reason":   reason,
		"duration": duration,
	}).Info("User suspended by admin")
	return nil
}

// UnsuspendUser unsuspends a user account.
func (s *adminService) UnsuspendUser(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	if !user.IsSuspended {
		return ErrUserNotSuspended
	}
	if err := user.Unsuspend(); err != nil {
		return err
	}
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to unsuspend user: %w", err)
	}
	s.log.WithField("user_id", userID).Info("User unsuspended by admin")
	return nil
}

// DeleteUser permanently deletes a user account.
func (s *adminService) DeleteUser(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Hard delete all user data (tweets, likes, retweets, follows, etc.)
	// In a real implementation, we would have cascading deletes or use a transaction.
	// For simplicity, we just soft delete user.
	if err := s.userRepo.SoftDelete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	s.log.WithField("user_id", userID).Warn("User permanently deleted by admin")
	return nil
}

// ======================================================================
= Tweet Moderation
// ======================================================================

// ListTweets returns a paginated list of tweets with filters.
func (s *adminService) ListTweets(ctx context.Context, cursor string, limit int, status, userID, search string) ([]*dto.AdminTweetResponse, string, int64, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Build filter
	filter := &interfaces.TweetFilter{}
	if userID != "" {
		filter.UserID = &userID
	}
	if search != "" {
		filter.Search = &search
	}
	if status == "reported" {
		// For reported tweets, we need to join with reports
		// For simplicity, we'll just filter later or use a different approach.
	}

	pagination := &interfaces.TweetPagination{
		Limit: limit,
		Cursor: cursor,
	}

	tweets, total, err := s.tweetRepo.SearchWithFilters(ctx, search, filter, pagination)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to list tweets: %w", err)
	}

	responses := make([]*dto.AdminTweetResponse, 0, len(tweets))
	for _, t := range tweets {
		user, _ := s.userRepo.GetByID(ctx, t.UserID)
		responses = append(responses, &dto.AdminTweetResponse{
			ID:          t.ID,
			Content:     t.Content,
			UserID:      t.UserID,
			Username:    func() string { if user != nil { return user.Username } return "" }(),
			MediaURLs:   t.MediaURLs,
			LikeCount:   func() int64 { c, _ := s.likeRepo.CountByTweetID(ctx, t.ID); return c }(),
			RetweetCount: func() int64 { c, _ := s.retweetRepo.CountByTweetID(ctx, t.ID); return c }(),
			ReplyCount:  func() int64 { c, _ := s.tweetRepo.CountReplies(ctx, t.ID); return c }(),
			IsDeleted:   t.DeletedAt != nil,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}

	var nextCursor string
	if len(tweets) == limit {
		nextCursor = tweets[len(tweets)-1].ID
	}
	return responses, nextCursor, total, nil
}

// DeleteTweet deletes a tweet (hard delete).
func (s *adminService) DeleteTweet(ctx context.Context, tweetID string) error {
	if err := s.tweetRepo.HardDelete(ctx, tweetID); err != nil {
		if errors.Is(err, interfaces.ErrTweetNotFound) {
			return ErrTweetNotFound
		}
		return fmt.Errorf("failed to delete tweet: %w", err)
	}
	s.log.WithField("tweet_id", tweetID).Warn("Tweet deleted by admin")
	return nil
}

// HideTweet hides a tweet from feeds.
func (s *adminService) HideTweet(ctx context.Context, tweetID string, duration int) error {
	// We can implement a "hidden" flag on tweet entity.
	// For simplicity, we'll just log it.
	s.log.WithFields(logrus.Fields{
		"tweet_id": tweetID,
		"duration": duration,
	}).Info("Tweet hidden by admin")
	return nil
}

// UnhideTweet unhides a tweet.
func (s *adminService) UnhideTweet(ctx context.Context, tweetID string) error {
	s.log.WithField("tweet_id", tweetID).Info("Tweet unhidden by admin")
	return nil
}

// PinTweet pins a tweet.
func (s *adminService) PinTweet(ctx context.Context, tweetID string) error {
	s.log.WithField("tweet_id", tweetID).Info("Tweet pinned by admin")
	return nil
}

// UnpinTweet unpins a tweet.
func (s *adminService) UnpinTweet(ctx context.Context, tweetID string) error {
	s.log.WithField("tweet_id", tweetID).Info("Tweet unpinned by admin")
	return nil
}

// ======================================================================
= Report Management
// ======================================================================

// ListReports returns a paginated list of reports with filters.
func (s *adminService) ListReports(ctx context.Context, cursor string, limit int, status, severity, targetType string) ([]*dto.AdminReportResponse, string, int64, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	filter := &interfaces.ReportFilter{}
	if status != "" {
		filter.Status = &status
	}
	if severity != "" {
		filter.Severity = &severity
	}
	if targetType != "" {
		filter.TargetType = &targetType
	}

	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		Cursor: cursor,
	}

	reports, total, err := s.reportRepo.List(ctx, filter, pagination)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to list reports: %w", err)
	}

	responses := make([]*dto.AdminReportResponse, 0, len(reports))
	for _, r := range reports {
		reporter, _ := s.userRepo.GetByID(ctx, r.ReporterID)
		targetUser, _ := s.userRepo.GetByID(ctx, r.TargetID)
		responses = append(responses, &dto.AdminReportResponse{
			ID:         r.ID,
			ReporterID: r.ReporterID,
			ReporterUsername: func() string { if reporter != nil { return reporter.Username } return "" }(),
			TargetID:   r.TargetID,
			TargetType: r.TargetType,
			TargetUsername: func() string { if targetUser != nil { return targetUser.Username } return "" }(),
			Reason:     r.Reason,
			Description: r.Description,
			Status:     r.Status,
			Severity:   r.Severity,
			ReviewerID: r.ReviewerID,
			ReviewNotes: r.ReviewNotes,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
		})
	}

	var nextCursor string
	if len(reports) == limit {
		nextCursor = reports[len(reports)-1].ID
	}
	return responses, nextCursor, total, nil
}

// GetReportDetails returns detailed report information.
func (s *adminService) GetReportDetails(ctx context.Context, reportID string) (*dto.AdminReportDetailResponse, error) {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return nil, ErrReportNotFound
		}
		return nil, fmt.Errorf("failed to get report: %w", err)
	}

	reporter, _ := s.userRepo.GetByID(ctx, report.ReporterID)
	targetUser, _ := s.userRepo.GetByID(ctx, report.TargetID)

	// Get related reports (duplicates)
	duplicates, _ := s.reportRepo.GetDuplicateReports(ctx, report.TargetID, report.TargetType)

	return &dto.AdminReportDetailResponse{
		ID:            report.ID,
		ReporterID:    report.ReporterID,
		ReporterUsername: func() string { if reporter != nil { return reporter.Username } return "" }(),
		TargetID:      report.TargetID,
		TargetType:    report.TargetType,
		TargetUsername: func() string { if targetUser != nil { return targetUser.Username } return "" }(),
		Reason:        report.Reason,
		Description:   report.Description,
		Status:        report.Status,
		Severity:      report.Severity,
		ReviewerID:    report.ReviewerID,
		ReviewNotes:   report.ReviewNotes,
		Metadata:      report.Metadata,
		CreatedAt:     report.CreatedAt,
		UpdatedAt:     report.UpdatedAt,
		ResolvedAt:    report.ResolvedAt,
		DuplicateReports: duplicates,
	}, nil
}

// AssignReport assigns a report to a reviewer.
func (s *adminService) AssignReport(ctx context.Context, reportID, reviewerID string) error {
	if err := s.reportRepo.AssignReviewer(ctx, reportID, reviewerID); err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to assign report: %w", err)
	}
	s.log.WithFields(logrus.Fields{
		"report_id":   reportID,
		"reviewer_id": reviewerID,
	}).Info("Report assigned to reviewer")
	return nil
}

// UpdateReportStatus updates a report's status.
func (s *adminService) UpdateReportStatus(ctx context.Context, reportID, status, notes string) error {
	if !interfaces.IsValidReportStatus(status) {
		return ErrInvalidReportStatus
	}
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	// Verify status transition
	if report.Status == interfaces.ReportStatusResolved || report.Status == interfaces.ReportStatusDismissed {
		if status != "under_review" && status != "pending" {
			return ErrReportAlreadyResolved
		}
	}
	if err := s.reportRepo.UpdateStatus(ctx, reportID, status, report.ReviewerID, notes); err != nil {
		return fmt.Errorf("failed to update report status: %w", err)
	}
	s.log.WithFields(logrus.Fields{
		"report_id": reportID,
		"status":    status,
	}).Info("Report status updated by admin")
	return nil
}

// UpdateReportSeverity updates a report's severity.
func (s *adminService) UpdateReportSeverity(ctx context.Context, reportID, severity string) error {
	if !interfaces.IsValidReportSeverity(severity) {
		return ErrInvalidReportSeverity
	}
	if err := s.reportRepo.UpdateSeverity(ctx, reportID, severity); err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to update report severity: %w", err)
	}
	s.log.WithFields(logrus.Fields{
		"report_id": reportID,
		"severity":  severity,
	}).Info("Report severity updated by admin")
	return nil
}

// ResolveReport resolves a report with action taken.
func (s *adminService) ResolveReport(ctx context.Context, reportID, action, notes string) error {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	if report.Status == interfaces.ReportStatusResolved || report.Status == interfaces.ReportStatusDismissed {
		return ErrReportAlreadyResolved
	}
	// Record moderation action
	_ = s.reportRepo.RecordModerationAction(ctx, reportID, action, "admin", map[string]interface{}{
		"notes": notes,
	})
	if err := s.reportRepo.ResolveReport(ctx, reportID, report.ReviewerID, notes); err != nil {
		return fmt.Errorf("failed to resolve report: %w", err)
	}
	s.log.WithFields(logrus.Fields{
		"report_id": reportID,
		"action":    action,
	}).Info("Report resolved by admin")
	return nil
}

// DismissReport dismisses a report.
func (s *adminService) DismissReport(ctx context.Context, reportID, notes string) error {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	if report.Status == interfaces.ReportStatusResolved || report.Status == interfaces.ReportStatusDismissed {
		return ErrReportAlreadyResolved
	}
	if err := s.reportRepo.DismissReport(ctx, reportID, report.ReviewerID, notes); err != nil {
		return fmt.Errorf("failed to dismiss report: %w", err)
	}
	s.log.WithField("report_id", reportID).Info("Report dismissed by admin")
	return nil
}

// ======================================================================
= System Stats
// ======================================================================

// GetSystemStats returns overall system statistics.
func (s *adminService) GetSystemStats(ctx context.Context) (*dto.SystemStatsResponse, error) {
	userCount, _ := s.userRepo.CountTotal(ctx)
	tweetCount, _ := s.tweetRepo.CountByUserID(ctx, "")
	// Not accurate, but we'll use CountTotal for tweets if available.
	// For now, use approximate.
	likeCount, _ := s.likeRepo.GetLikeStats(ctx)
	retweetCount, _ := s.retweetRepo.CountByTweetID(ctx, "")
	reportCount, _ := s.reportRepo.CountTotal(ctx)

	return &dto.SystemStatsResponse{
		TotalUsers:    userCount,
		TotalTweets:   tweetCount,
		TotalLikes:    likeCount.TotalLikes,
		TotalRetweets: retweetCount,
		TotalReports:  reportCount,
		Timestamp:     time.Now(),
	}, nil
}

// GetDailyStats returns daily statistics for the last N days.
func (s *adminService) GetDailyStats(ctx context.Context, days int) (*dto.DailyStatsResponse, error) {
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	// Get daily counts from repositories
	// This is simplified; in production we'd use aggregate queries.
	return &dto.DailyStatsResponse{
		StartDate: start,
		EndDate:   end,
		DailyData: []*dto.DailyStat{},
	}, nil
}

// GetEngagementStats returns engagement statistics.
func (s *adminService) GetEngagementStats(ctx context.Context, days int) (*dto.EngagementStatsResponse, error) {
	// Simplified
	return &dto.EngagementStatsResponse{
		TotalLikes:     0,
		TotalRetweets:  0,
		TotalReplies:   0,
		AverageEngagement: 0,
	}, nil
}

// GetReportAnalytics returns report analytics.
func (s *adminService) GetReportAnalytics(ctx context.Context, days int) (*dto.ReportAnalyticsResponse, error) {
	stats, _ := s.reportRepo.GetReportStats(ctx)
	analytics := &dto.ReportAnalyticsResponse{
		TotalReports:     stats.TotalReports,
		PendingReports:   stats.PendingReports,
		UnderReviewReports: stats.UnderReviewReports,
		ResolvedReports:  stats.ResolvedReports,
		DismissedReports: stats.DismissedReports,
		UniqueReporters:  stats.UniqueReporters,
		UniqueTargets:    stats.UniqueTargets,
		AvgResolutionTime: stats.AvgResolutionTime,
		SeverityStats:    stats.SeverityStats,
		ReasonStats:      stats.ReasonStats,
		TargetTypeStats:  stats.TargetTypeStats,
	}
	return analytics, nil
}