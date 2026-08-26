// backend/internal/service/report_service.go
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
	MaxReportReasonLen    = 200
	MaxReportDescriptionLen = 1000
	MaxReportNotesLen     = 500
	DefaultReportLimit    = 20
	MaxReportLimit        = 100
)

var (
	ErrReportNotFound        = errors.New("report not found")
	ErrReportAlreadyResolved = errors.New("report already resolved")
	ErrReportAlreadyDismissed = errors.New("report already dismissed")
	ErrReportAlreadyEscalated = errors.New("report already escalated")
	ErrReportDuplicate       = errors.New("duplicate report from same user")
	ErrInvalidReportStatus   = errors.New("invalid report status")
	ErrInvalidReportSeverity = errors.New("invalid report severity")
	ErrInvalidReportType     = errors.New("invalid report type")
	ErrReportReviewerRequired = errors.New("reviewer ID is required")
	ErrReportCannotReopen    = errors.New("cannot reopen resolved/dismissed report")
	ErrReportCannotEscalate  = errors.New("cannot escalate non-review report")
	ErrReportNotesTooLong    = errors.New("report notes exceed maximum length")
	ErrUserNotFound          = errors.New("user not found")
	ErrTweetNotFound         = errors.New("tweet not found")
	ErrInvalidTargetType     = errors.New("invalid target type")
	ErrReasonRequired        = errors.New("reason is required")
	ErrReasonTooLong         = errors.New("reason is too long")
	ErrDescriptionTooLong    = errors.New("description is too long")
)

// ======================================================================
// ReportService Interface
// ======================================================================

// ReportService defines the report service interface.
type ReportService interface {
	// Create creates a new report.
	Create(ctx context.Context, reporterID, targetID, targetType, reportType, reason, description string) (*entities.Report, error)
	
	// GetByID retrieves a report by ID.
	GetByID(ctx context.Context, reportID string) (*entities.Report, error)
	
	// GetByTarget retrieves reports for a target.
	GetByTarget(ctx context.Context, targetID, targetType string, cursor string, limit int) ([]*entities.Report, string, error)
	
	// GetByReporter retrieves reports filed by a user.
	GetByReporter(ctx context.Context, reporterID string, cursor string, limit int) ([]*entities.Report, string, error)
	
	// List returns a paginated list of reports.
	List(ctx context.Context, filter *dto.AdminReportFilterRequest) (*dto.ReportListResponse, error)
	
	// UpdateStatus updates the status of a report.
	UpdateStatus(ctx context.Context, reportID, status, reviewerID, notes string) error
	
	// ResolveReport resolves a report.
	ResolveReport(ctx context.Context, reportID, reviewerID, notes string) error
	
	// DismissReport dismisses a report.
	DismissReport(ctx context.Context, reportID, reviewerID, notes string) error
	
	// EscalateReport escalates a report.
	EscalateReport(ctx context.Context, reportID, reviewerID, notes string) error
	
	// ReopenReport reopens a resolved/dismissed report.
	ReopenReport(ctx context.Context, reportID, reviewerID, notes string) error
	
	// AssignReviewer assigns a reviewer to a report.
	AssignReviewer(ctx context.Context, reportID, reviewerID string) error
	
	// GetReportStats returns report statistics.
	GetReportStats(ctx context.Context) (*dto.ReportStatsResponse, error)
	
	// GetUserReportStats returns report statistics for a user.
	GetUserReportStats(ctx context.Context, userID string) (*dto.ReportStatsResponse, error)
}

// ======================================================================
// reportService Implementation
// ======================================================================

// reportService implements ReportService.
type reportService struct {
	reportRepo       interfaces.ReportRepository
	userRepo         interfaces.UserRepository
	tweetRepo        interfaces.TweetRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	log              *logrus.Entry
}

// NewReportService creates a new report service.
func NewReportService(
	reportRepo interfaces.ReportRepository,
	userRepo interfaces.UserRepository,
	tweetRepo interfaces.TweetRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) ReportService {
	return &reportService{
		reportRepo:       reportRepo,
		userRepo:         userRepo,
		tweetRepo:        tweetRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		log:              logger.WithField("service", "report"),
	}
}

// ======================================================================
// Create Report
// ======================================================================

// Create creates a new report.
func (s *reportService) Create(ctx context.Context, reporterID, targetID, targetType, reportType, reason, description string) (*entities.Report, error) {
	// Validate reporter exists
	_, err := s.userRepo.GetByID(ctx, reporterID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get reporter: %w", err)
	}
	// Validate target type
	validTargetTypes := map[string]bool{"tweet": true, "user": true, "community": true, "message": true}
	if !validTargetTypes[targetType] {
		return nil, ErrInvalidTargetType
	}
	// Validate target exists
	switch targetType {
	case "tweet":
		_, err = s.tweetRepo.GetByID(ctx, targetID)
		if err != nil {
			if errors.Is(err, interfaces.ErrTweetNotFound) {
				return nil, ErrTweetNotFound
			}
			return nil, fmt.Errorf("failed to get tweet: %w", err)
		}
	case "user":
		_, err = s.userRepo.GetByID(ctx, targetID)
		if err != nil {
			if errors.Is(err, interfaces.ErrUserNotFound) {
				return nil, ErrUserNotFound
			}
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
	}
	// Validate report type
	if !isValidReportType(reportType) {
		return nil, ErrInvalidReportType
	}
	// Validate reason
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrReasonRequired
	}
	if len(reason) > MaxReportReasonLen {
		return nil, ErrReasonTooLong
	}
	// Validate description
	description = strings.TrimSpace(description)
	if len(description) > MaxReportDescriptionLen {
		return nil, ErrDescriptionTooLong
	}
	// Check for duplicate report
	duplicate, err := s.reportRepo.CheckDuplicate(ctx, reporterID, targetID, targetType)
	if err != nil {
		return nil, fmt.Errorf("failed to check duplicate report: %w", err)
	}
	if duplicate {
		return nil, ErrReportDuplicate
	}
	// Create report
	report := &entities.Report{
		ID:          uuid.New().String(),
		ReporterID:  reporterID,
		TargetID:    targetID,
		TargetType:  targetType,
		Type:        entities.ReportType(reportType),
		Reason:      reason,
		Description: description,
		Status:      entities.ReportStatusPending,
		Severity:    entities.ReportSeverityMedium,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.reportRepo.Create(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to create report: %w", err)
	}
	// Notify admins
	_ = s.notifyAdmins(ctx, report)
	s.log.WithFields(logrus.Fields{
		"report_id":   report.ID,
		"reporter_id": reporterID,
		"target_id":   targetID,
		"target_type": targetType,
		"type":        reportType,
	}).Info("Report created")
	return report, nil
}

// ======================================================================
// Get Report
// ======================================================================

// GetByID retrieves a report by ID.
func (s *reportService) GetByID(ctx context.Context, reportID string) (*entities.Report, error) {
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return nil, ErrReportNotFound
		}
		return nil, fmt.Errorf("failed to get report: %w", err)
	}
	return report, nil
}

// GetByTarget retrieves reports for a target.
func (s *reportService) GetByTarget(ctx context.Context, targetID, targetType string, cursor string, limit int) ([]*entities.Report, string, error) {
	if limit < 1 || limit > MaxReportLimit {
		limit = DefaultReportLimit
	}
	reports, nextCursor, err := s.reportRepo.GetByTarget(ctx, targetID, targetType, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get reports by target: %w", err)
	}
	return reports, nextCursor, nil
}

// GetByReporter retrieves reports filed by a user.
func (s *reportService) GetByReporter(ctx context.Context, reporterID string, cursor string, limit int) ([]*entities.Report, string, error) {
	if limit < 1 || limit > MaxReportLimit {
		limit = DefaultReportLimit
	}
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, reporterID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, "", ErrUserNotFound
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}
	reports, nextCursor, err := s.reportRepo.GetByReporter(ctx, reporterID, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get reports by reporter: %w", err)
	}
	return reports, nextCursor, nil
}

// ======================================================================
// List Reports
// ======================================================================

// List returns a paginated list of reports.
func (s *reportService) List(ctx context.Context, filter *dto.AdminReportFilterRequest) (*dto.ReportListResponse, error) {
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
	// Pagination
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
		return nil, fmt.Errorf("failed to list reports: %w", err)
	}
	// Build responses
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

// ======================================================================
= Status Management
// ======================================================================

// UpdateStatus updates the status of a report.
func (s *reportService) UpdateStatus(ctx context.Context, reportID, status, reviewerID, notes string) error {
	if !isValidReportStatus(status) {
		return ErrInvalidReportStatus
	}
	// Get report
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	// Check if already terminal
	if report.Status.IsTerminal() && status != string(report.Status) {
		return ErrReportCannotReopen
	}
	// Update status
	if err := s.reportRepo.UpdateStatus(ctx, reportID, status, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateReportCache(ctx, reportID)
	s.log.WithFields(logrus.Fields{
		"report_id": reportID,
		"status":    status,
		"reviewer":  reviewerID,
	}).Info("Report status updated")
	return nil
}

// ResolveReport resolves a report.
func (s *reportService) ResolveReport(ctx context.Context, reportID, reviewerID, notes string) error {
	// Get report
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	if report.Status == entities.ReportStatusResolved {
		return ErrReportAlreadyResolved
	}
	if reviewerID == "" {
		return ErrReportReviewerRequired
	}
	if len(notes) > MaxReportNotesLen {
		return ErrReportNotesTooLong
	}
	if err := s.reportRepo.ResolveReport(ctx, reportID, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to resolve report: %w", err)
	}
	// Notify reporter
	_ = s.createNotification(ctx, report.ReporterID, reviewerID, "report_resolved", reportID)
	// Invalidate cache
	_ = s.invalidateReportCache(ctx, reportID)
	s.log.WithFields(logrus.Fields{
		"report_id": reportID,
		"reviewer":  reviewerID,
	}).Info("Report resolved")
	return nil
}

// DismissReport dismisses a report.
func (s *reportService) DismissReport(ctx context.Context, reportID, reviewerID, notes string) error {
	// Get report
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	if report.Status == entities.ReportStatusDismissed {
		return ErrReportAlreadyDismissed
	}
	if reviewerID == "" {
		return ErrReportReviewerRequired
	}
	if len(notes) > MaxReportNotesLen {
		return ErrReportNotesTooLong
	}
	if err := s.reportRepo.DismissReport(ctx, reportID, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to dismiss report: %w", err)
	}
	// Notify reporter
	_ = s.createNotification(ctx, report.ReporterID, reviewerID, "report_dismissed", reportID)
	// Invalidate cache
	_ = s.invalidateReportCache(ctx, reportID)
	s.log.WithFields(logrus.Fields{
		"report_id": reportID,
		"reviewer":  reviewerID,
	}).Info("Report dismissed")
	return nil
}

// EscalateReport escalates a report.
func (s *reportService) EscalateReport(ctx context.Context, reportID, reviewerID, notes string) error {
	// Get report
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	if report.Status == entities.ReportStatusEscalated {
		return ErrReportAlreadyEscalated
	}
	if report.Status.IsTerminal() {
		return ErrReportCannotEscalate
	}
	if reviewerID == "" {
		return ErrReportReviewerRequired
	}
	if len(notes) > MaxReportNotesLen {
		return ErrReportNotesTooLong
	}
	if err := s.reportRepo.UpdateStatus(ctx, reportID, entities.ReportStatusEscalated, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to escalate report: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateReportCache(ctx, reportID)
	s.log.WithFields(logrus.Fields{
		"report_id": reportID,
		"reviewer":  reviewerID,
	}).Info("Report escalated")
	return nil
}

// ReopenReport reopens a resolved/dismissed report.
func (s *reportService) ReopenReport(ctx context.Context, reportID, reviewerID, notes string) error {
	// Get report
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	if !report.Status.IsTerminal() {
		return errors.New("report is not resolved or dismissed")
	}
	if reviewerID == "" {
		return ErrReportReviewerRequired
	}
	if len(notes) > MaxReportNotesLen {
		return ErrReportNotesTooLong
	}
	if err := s.reportRepo.ReopenReport(ctx, reportID, reviewerID, notes); err != nil {
		return fmt.Errorf("failed to reopen report: %w", err)
	}
	// Invalidate cache
	_ = s.invalidateReportCache(ctx, reportID)
	s.log.WithFields(logrus.Fields{
		"report_id": reportID,
		"reviewer":  reviewerID,
	}).Info("Report reopened")
	return nil
}

// AssignReviewer assigns a reviewer to a report.
func (s *reportService) AssignReviewer(ctx context.Context, reportID, reviewerID string) error {
	// Get report
	report, err := s.reportRepo.GetByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, interfaces.ErrReportNotFound) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get report: %w", err)
	}
	if report.Status.IsTerminal() {
		return errors.New("cannot assign reviewer to terminal report")
	}
	if reviewerID == "" {
		return ErrReportReviewerRequired
	}
	if err := s.reportRepo.AssignReviewer(ctx, reportID, reviewerID); err != nil {
		return fmt.Errorf("failed to assign reviewer: %w", err)
	}
	s.log.WithFields(logrus.Fields{
		"report_id":  reportID,
		"reviewer_id": reviewerID,
	}).Info("Reviewer assigned")
	return nil
}

// ======================================================================
// Stats
// ======================================================================

// GetReportStats returns report statistics.
func (s *reportService) GetReportStats(ctx context.Context) (*dto.ReportStatsResponse, error) {
	// Try cache
	cacheKey := "report_stats"
	if s.redisAdapter != nil {
		var cached dto.ReportStatsResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}
	stats, err := s.reportRepo.GetReportStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get report stats: %w", err)
	}
	response := &dto.ReportStatsResponse{
		TotalReports:      stats.TotalReports,
		PendingReports:    stats.PendingReports,
		UnderReviewReports: stats.UnderReviewReports,
		ResolvedReports:   stats.ResolvedReports,
		DismissedReports:  stats.DismissedReports,
		EscalatedReports:  stats.EscalatedReports,
		UniqueReporters:   stats.UniqueReporters,
		UniqueTargets:     stats.UniqueTargets,
		AvgResolutionTime: stats.AvgResolutionTime,
		SeverityStats:     stats.SeverityStats,
		ReasonStats:       stats.ReasonStats,
		TargetTypeStats:   stats.TargetTypeStats,
		LastReportCreated: stats.LastReportCreated,
		LastReportResolved: stats.LastReportResolved,
	}
	// Cache for 5 minutes
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, response, 5*time.Minute)
	}
	return response, nil
}

// GetUserReportStats returns report statistics for a user.
func (s *reportService) GetUserReportStats(ctx context.Context, userID string) (*dto.ReportStatsResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	stats, err := s.reportRepo.GetUserReportStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user report stats: %w", err)
	}
	return &dto.ReportStatsResponse{
		TotalReports:     stats.TotalReports,
		PendingReports:   stats.PendingReports,
		ResolvedReports:  stats.ResolvedReports,
		DismissedReports: stats.DismissedReports,
	}, nil
}

// ======================================================================
// Helper Methods
// ======================================================================

// isValidReportStatus checks if a status is valid.
func isValidReportStatus(status string) bool {
	valid := map[string]bool{
		"pending": true, "under_review": true, "resolved": true,
		"dismissed": true, "escalated": true, "closed": true,
	}
	return valid[status]
}

// isValidReportType checks if a report type is valid.
func isValidReportType(reportType string) bool {
	valid := map[string]bool{
		"spam": true, "harassment": true, "hate_speech": true,
		"inappropriate": true, "misleading": true, "copyright": true,
		"impersonation": true, "self_harm": true, "violence": true,
		"nudity": true, "other": true,
	}
	return valid[reportType]
}

// createNotification creates a notification for a user.
func (s *reportService) createNotification(ctx context.Context, userID, fromUserID, notifType, referenceID string) error {
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

// notifyAdmins notifies admins about a new report.
func (s *reportService) notifyAdmins(ctx context.Context, report *entities.Report) error {
	// In production, get all admin users and notify them
	// For now, log the notification
	s.log.WithFields(logrus.Fields{
		"report_id":   report.ID,
		"target_id":   report.TargetID,
		"target_type": report.TargetType,
	}).Info("Admin notification: New report created")
	return nil
}

// invalidateReportCache invalidates report caches.
func (s *reportService) invalidateReportCache(ctx context.Context, reportID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	_ = s.redisAdapter.Delete(ctx, fmt.Sprintf("report:%s", reportID))
	_ = s.redisAdapter.Delete(ctx, "report_stats")
	return nil
}

// ======================================================================
// Global Instance
// ======================================================================

var defaultReportService ReportService

// InitReportService initializes the global report service.
func InitReportService(
	reportRepo interfaces.ReportRepository,
	userRepo interfaces.UserRepository,
	tweetRepo interfaces.TweetRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
) {
	defaultReportService = NewReportService(
		reportRepo,
		userRepo,
		tweetRepo,
		notificationRepo,
		redisAdapter,
	)
}

// GetReportService returns the global report service.
func GetReportService() ReportService {
	if defaultReportService == nil {
		panic("report service not initialized")
	}
	return defaultReportService
}