// backend/internal/repository/interfaces/report_repo.go
package interfaces

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ======================================================================
// Common Errors
// ======================================================================

var (
	ErrReportNotFound      = errors.New("report not found")
	ErrReportAlreadyResolved = errors.New("report already resolved")
	ErrInvalidReportID     = errors.New("invalid report ID")
	ErrInvalidReporterID   = errors.New("invalid reporter ID")
	ErrInvalidTargetID     = errors.New("invalid target ID")
	ErrInvalidTargetType   = errors.New("invalid target type")
	ErrInvalidReportStatus = errors.New("invalid report status")
	ErrInvalidReportSeverity = errors.New("invalid report severity")
	ErrReportDuplicate     = errors.New("duplicate report from same user")
	ErrReportExpired       = errors.New("report has expired")
	ErrReportLimitExceeded = errors.New("report limit exceeded")
	ErrInvalidReviewerID   = errors.New("invalid reviewer ID")
	ErrReportAlreadyAssigned = errors.New("report already assigned")
)

// ======================================================================
// ReportFilter
// ======================================================================

// ReportFilter defines filtering options for report queries.
type ReportFilter struct {
	ReporterID   *string
	TargetID     *string
	TargetType   *string
	Status       *string
	Severity     *string
	Reason       *string
	ReviewerID   *string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	ResolvedFrom *time.Time
	ResolvedTo   *time.Time
	AssignedTo   *string
	HasReview    *bool
}

// HasCriteria checks if any filter criteria are set.
func (f *ReportFilter) HasCriteria() bool {
	return f.ReporterID != nil || f.TargetID != nil || f.TargetType != nil ||
		f.Status != nil || f.Severity != nil || f.Reason != nil ||
		f.ReviewerID != nil || f.CreatedFrom != nil || f.CreatedTo != nil ||
		f.ResolvedFrom != nil || f.ResolvedTo != nil || f.AssignedTo != nil ||
		f.HasReview != nil
}

// ======================================================================
// ReportPagination
// ======================================================================

// ReportSortField defines sortable fields for reports.
type ReportSortField string

const (
	SortReportByCreatedAt   ReportSortField = "created_at"
	SortReportByUpdatedAt   ReportSortField = "updated_at"
	SortReportByResolvedAt  ReportSortField = "resolved_at"
	SortReportBySeverity    ReportSortField = "severity"
	SortReportByStatus      ReportSortField = "status"
)

// ReportSortOrder defines sort order.
type ReportSortOrder string

const (
	ReportSortAsc  ReportSortOrder = "ASC"
	ReportSortDesc ReportSortOrder = "DESC"
)

// ReportPagination holds pagination options for reports.
type ReportPagination struct {
	Cursor string            `json:"cursor"`
	Limit  int               `json:"limit"`
	SortBy ReportSortField   `json:"sort_by"`
	Order  ReportSortOrder   `json:"order"`
}

// DefaultReportPagination returns default pagination options.
func DefaultReportPagination() *ReportPagination {
	return &ReportPagination{
		Limit:  20,
		SortBy: SortReportByCreatedAt,
		Order:  ReportSortDesc,
	}
}

// Validate checks pagination parameters.
func (p *ReportPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// ReportStats
// ======================================================================

// ReportStats represents aggregated report statistics.
type ReportStats struct {
	TotalReports      int64     `json:"total_reports"`
	PendingReports    int64     `json:"pending_reports"`
	UnderReviewReports int64    `json:"under_review_reports"`
	ResolvedReports   int64     `json:"resolved_reports"`
	DismissedReports  int64     `json:"dismissed_reports"`
	UniqueReporters   int64     `json:"unique_reporters"`
	UniqueTargets     int64     `json:"unique_targets"`
	AvgResolutionTime float64   `json:"avg_resolution_time"` // in seconds
	MaxResolutionTime float64   `json:"max_resolution_time"`
	MinResolutionTime float64   `json:"min_resolution_time"`
	LastReportCreated time.Time `json:"last_report_created"`
	LastReportResolved time.Time `json:"last_report_resolved"`
	MostActiveReporterID string `json:"most_active_reporter_id"`
	MostActiveReporterCount int64 `json:"most_active_reporter_count"`
	MostReportedTargetID string `json:"most_reported_target_id"`
	MostReportedTargetCount int64 `json:"most_reported_target_count"`
	SeverityStats      map[string]int64 `json:"severity_stats"`
	ReasonStats        map[string]int64 `json:"reason_stats"`
	TargetTypeStats    map[string]int64 `json:"target_type_stats"`
}

// ======================================================================
// DailyReportCount
// ======================================================================

// DailyReportCount represents daily report counts.
type DailyReportCount struct {
	Date         time.Time `json:"date"`
	Total        int64     `json:"total"`
	Pending      int64     `json:"pending"`
	UnderReview  int64     `json:"under_review"`
	Resolved     int64     `json:"resolved"`
	Dismissed    int64     `json:"dismissed"`
	UniqueReporters int64  `json:"unique_reporters"`
}

// ======================================================================
// ReportRepository Interface
// ======================================================================

// ReportRepository defines the interface for report data persistence.
type ReportRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a new report.
	Create(ctx context.Context, report *entities.Report) error

	// GetByID retrieves a report by its ID.
	GetByID(ctx context.Context, id string) (*entities.Report, error)

	// GetByIDs retrieves multiple reports by their IDs.
	GetByIDs(ctx context.Context, ids []string) ([]*entities.Report, error)

	// GetByTarget retrieves reports for a specific target.
	GetByTarget(ctx context.Context, targetID, targetType string, cursor string, limit int) ([]*entities.Report, string, error)

	// GetByReporter retrieves reports filed by a user.
	GetByReporter(ctx context.Context, reporterID string, cursor string, limit int) ([]*entities.Report, string, error)

	// GetByReviewer retrieves reports assigned to a reviewer.
	GetByReviewer(ctx context.Context, reviewerID string, cursor string, limit int) ([]*entities.Report, string, error)

	// Update updates a report (status, severity, review notes, etc.).
	Update(ctx context.Context, report *entities.Report) error

	// Delete removes a report (hard delete).
	Delete(ctx context.Context, id string) error

	// DeleteByTarget removes all reports for a target.
	DeleteByTarget(ctx context.Context, targetID, targetType string) error

	// --------------------------------------------------------------------
	// Status Management
	// --------------------------------------------------------------------

	// UpdateStatus updates the status of a report.
	UpdateStatus(ctx context.Context, id, status string, reviewerID string, notes string) error

	// ResolveReport marks a report as resolved.
	ResolveReport(ctx context.Context, id, reviewerID, notes string) error

	// DismissReport marks a report as dismissed.
	DismissReport(ctx context.Context, id, reviewerID, notes string) error

	// ReopenReport reopens a resolved/dismissed report.
	ReopenReport(ctx context.Context, id, reviewerID, notes string) error

	// AssignReviewer assigns a reviewer to a report.
	AssignReviewer(ctx context.Context, id, reviewerID string) error

	// UnassignReviewer removes the reviewer assignment.
	UnassignReviewer(ctx context.Context, id string) error

	// UpdateSeverity updates the severity of a report.
	UpdateSeverity(ctx context.Context, id, severity string) error

	// AddReviewNote adds a review note to a report.
	AddReviewNote(ctx context.Context, id, note string) error

	// --------------------------------------------------------------------
	// Existence Checks
	// --------------------------------------------------------------------

	// Exists checks if a report exists.
	Exists(ctx context.Context, id string) (bool, error)

	// CheckDuplicate checks if a user has already reported the same target.
	CheckDuplicate(ctx context.Context, reporterID, targetID, targetType string) (bool, error)

	// GetDuplicateReports returns duplicate reports for the same target.
	GetDuplicateReports(ctx context.Context, targetID, targetType string) ([]*entities.Report, error)

	// --------------------------------------------------------------------
	// List Operations
	// --------------------------------------------------------------------

	// List returns reports with filtering and pagination.
	List(ctx context.Context, filter *ReportFilter, pagination *ReportPagination) ([]*entities.Report, int64, error)

	// GetPending returns pending reports sorted by severity.
	GetPending(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error)

	// GetUnderReview returns reports under review.
	GetUnderReview(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error)

	// GetResolved returns resolved reports.
	GetResolved(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error)

	// GetDismissed returns dismissed reports.
	GetDismissed(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error)

	// GetByDateRange returns reports within a date range.
	GetByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Report, string, error)

	// GetBySeverity returns reports by severity.
	GetBySeverity(ctx context.Context, severity string, cursor string, limit int) ([]*entities.Report, string, error)

	// GetByReason returns reports by reason.
	GetByReason(ctx context.Context, reason string, cursor string, limit int) ([]*entities.Report, string, error)

	// GetByTargetType returns reports by target type.
	GetByTargetType(ctx context.Context, targetType string, cursor string, limit int) ([]*entities.Report, string, error)

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountTotal returns total number of reports.
	CountTotal(ctx context.Context) (int64, error)

	// CountByStatus returns number of reports by status.
	CountByStatus(ctx context.Context, status string) (int64, error)

	// CountBySeverity returns number of reports by severity.
	CountBySeverity(ctx context.Context, severity string) (int64, error)

	// CountByTarget returns number of reports for a target.
	CountByTarget(ctx context.Context, targetID, targetType string) (int64, error)

	// CountByReporter returns number of reports filed by a user.
	CountByReporter(ctx context.Context, reporterID string) (int64, error)

	// CountByReviewer returns number of reports assigned to a reviewer.
	CountByReviewer(ctx context.Context, reviewerID string) (int64, error)

	// CountByDateRange returns report count within a date range.
	CountByDateRange(ctx context.Context, start, end time.Time) (int64, error)

	// CountPendingBySeverity returns pending reports count grouped by severity.
	CountPendingBySeverity(ctx context.Context) (map[string]int64, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple reports in a transaction.
	BulkCreate(ctx context.Context, reports []*entities.Report) error

	// BulkDelete removes multiple reports.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkUpdateStatus updates status for multiple reports.
	BulkUpdateStatus(ctx context.Context, ids []string, status, reviewerID, notes string) error

	// BulkAssignReviewer assigns a reviewer to multiple reports.
	BulkAssignReviewer(ctx context.Context, ids []string, reviewerID string) error

	// BulkResolve resolves multiple reports.
	BulkResolve(ctx context.Context, ids []string, reviewerID, notes string) error

	// BulkDismiss dismisses multiple reports.
	BulkDismiss(ctx context.Context, ids []string, reviewerID, notes string) error

	// BulkDeleteByTarget removes reports for multiple targets.
	BulkDeleteByTarget(ctx context.Context, pairs []TargetPair) error

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetReportStats returns aggregated report statistics.
	GetReportStats(ctx context.Context) (*ReportStats, error)

	// GetUserReportStats returns report statistics for a specific user (as reporter).
	GetUserReportStats(ctx context.Context, userID string) (*ReportStats, error)

	// GetReviewerStats returns report statistics for a reviewer.
	GetReviewerStats(ctx context.Context, reviewerID string) (*ReportStats, error)

	// GetDailyReportStats returns daily report counts for a date range.
	GetDailyReportStats(ctx context.Context, start, end time.Time) ([]*DailyReportCount, error)

	// GetReportTypeStats returns report stats by target type.
	GetReportTypeStats(ctx context.Context) ([]*ReportTypeStat, error)

	// GetReasonStats returns report stats by reason.
	GetReasonStats(ctx context.Context) ([]*ReasonStat, error)

	// GetSeverityDistribution returns severity distribution.
	GetSeverityDistribution(ctx context.Context) (map[string]float64, error)

	// GetReportVelocity calculates report velocity (reports per day).
	GetReportVelocity(ctx context.Context, days int) (float64, error)

	// GetAverageResolutionTimeBySeverity returns avg resolution time by severity.
	GetAverageResolutionTimeBySeverity(ctx context.Context) (map[string]float64, error)

	// GetMostReportedTargets returns the most reported targets.
	GetMostReportedTargets(ctx context.Context, limit int, since time.Time) ([]*ReportedTarget, error)

	// GetMostActiveReporters returns the most active reporters.
	GetMostActiveReporters(ctx context.Context, limit int, since time.Time) ([]*ActiveReporter, error)

	// --------------------------------------------------------------------
	// Moderation Actions (integration with moderation)
	// --------------------------------------------------------------------

	// GetActionableReports returns reports that require action.
	GetActionableReports(ctx context.Context, limit int) ([]*entities.Report, error)

	// GetReportsForModeration returns reports for a moderator to review.
	GetReportsForModeration(ctx context.Context, moderatorID string, limit int) ([]*entities.Report, error)

	// RecordModerationAction records a moderation action taken based on a report.
	RecordModerationAction(ctx context.Context, reportID, action, performedBy string, details map[string]interface{}) error

	// GetModerationHistory returns moderation history for a report.
	GetModerationHistory(ctx context.Context, reportID string) ([]*ModerationAction, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) ReportRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo ReportRepository) error) error

	// --------------------------------------------------------------------
	// Health and Cleanup
	// --------------------------------------------------------------------

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close releases any resources.
	Close() error

	// GetRawDB returns the underlying database connection.
	GetRawDB() interface{}
}

// ======================================================================
// Supporting Types
// ======================================================================

// ReportTypeStat represents report statistics by target type.
type ReportTypeStat struct {
	TargetType   string `json:"target_type"`
	Count        int64  `json:"count"`
	Pending      int64  `json:"pending"`
	Resolved     int64  `json:"resolved"`
	Dismissed    int64  `json:"dismissed"`
}

// ReasonStat represents report statistics by reason.
type ReasonStat struct {
	Reason   string `json:"reason"`
	Count    int64  `json:"count"`
	Pending  int64  `json:"pending"`
	Resolved int64  `json:"resolved"`
}

// ReportedTarget represents a target with report count.
type ReportedTarget struct {
	TargetID   string `json:"target_id"`
	TargetType string `json:"target_type"`
	ReportCount int64 `json:"report_count"`
	LastReported time.Time `json:"last_reported"`
}

// ActiveReporter represents a reporter with report count.
type ActiveReporter struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	ReportCount int64    `json:"report_count"`
	LastReported time.Time `json:"last_reported"`
}

// TargetPair represents a target pair for bulk operations.
type TargetPair struct {
	TargetID   string `json:"target_id"`
	TargetType string `json:"target_type"`
}

// ModerationAction represents a moderation action.
type ModerationAction struct {
	ID          string                 `json:"id"`
	ReportID    string                 `json:"report_id"`
	Action      string                 `json:"action"` // "delete_tweet", "suspend_user", "warning", etc.
	PerformedBy string                 `json:"performed_by"`
	Details     map[string]interface{} `json:"details"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ======================================================================
// Report Status Constants
// ======================================================================

const (
	ReportStatusPending     = "pending"
	ReportStatusUnderReview = "under_review"
	ReportStatusResolved    = "resolved"
	ReportStatusDismissed   = "dismissed"
)

// Report Severity Constants
const (
	ReportSeverityLow    = "low"
	ReportSeverityMedium = "medium"
	ReportSeverityHigh   = "high"
	ReportSeverityCritical = "critical"
)

// ======================================================================
// Helper Functions
// ======================================================================

// IsReportNotFound checks if an error indicates a report was not found.
func IsReportNotFound(err error) bool {
	return errors.Is(err, ErrReportNotFound)
}

// IsReportAlreadyResolved checks if an error indicates report already resolved.
func IsReportAlreadyResolved(err error) bool {
	return errors.Is(err, ErrReportAlreadyResolved)
}

// IsReportDuplicate checks if an error indicates a duplicate report.
func IsReportDuplicate(err error) bool {
	return errors.Is(err, ErrReportDuplicate)
}

// IsReportError checks if an error is report-related.
func IsReportError(err error) bool {
	return errors.Is(err, ErrReportNotFound) ||
		errors.Is(err, ErrReportAlreadyResolved) ||
		errors.Is(err, ErrInvalidReportID) ||
		errors.Is(err, ErrInvalidReporterID) ||
		errors.Is(err, ErrInvalidTargetID) ||
		errors.Is(err, ErrInvalidTargetType) ||
		errors.Is(err, ErrInvalidReportStatus) ||
		errors.Is(err, ErrInvalidReportSeverity) ||
		errors.Is(err, ErrReportDuplicate) ||
		errors.Is(err, ErrInvalidReviewerID)
}

// IsValidReportStatus checks if a status is valid.
func IsValidReportStatus(status string) bool {
	switch status {
	case ReportStatusPending, ReportStatusUnderReview, ReportStatusResolved, ReportStatusDismissed:
		return true
	}
	return false
}

// IsValidReportSeverity checks if a severity is valid.
func IsValidReportSeverity(severity string) bool {
	switch severity {
	case ReportSeverityLow, ReportSeverityMedium, ReportSeverityHigh, ReportSeverityCritical:
		return true
	}
	return false
}

// ======================================================================
// Mock Report Repository (for testing)
// ======================================================================

// MockReportRepository is a mock implementation for testing.
type MockReportRepository struct {
	Reports    map[string]*entities.Report
	NextID     int
	Error      error
	NextCursor string
}

// NewMockReportRepo creates a new mock repository.
func NewMockReportRepo() ReportRepository {
	return &MockReportRepository{
		Reports: make(map[string]*entities.Report),
	}
}

// Create mock implementation.
func (m *MockReportRepository) Create(ctx context.Context, report *entities.Report) error {
	if m.Error != nil {
		return m.Error
	}
	// Check duplicate
	for _, r := range m.Reports {
		if r.ReporterID == report.ReporterID && r.TargetID == report.TargetID && r.TargetType == report.TargetType {
			return ErrReportDuplicate
		}
	}
	m.Reports[report.ID] = report
	return nil
}

// GetByID mock implementation.
func (m *MockReportRepository) GetByID(ctx context.Context, id string) (*entities.Report, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if report, ok := m.Reports[id]; ok {
		return report, nil
	}
	return nil, ErrReportNotFound
}

// GetByIDs mock implementation.
func (m *MockReportRepository) GetByIDs(ctx context.Context, ids []string) ([]*entities.Report, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var reports []*entities.Report
	for _, id := range ids {
		if r, ok := m.Reports[id]; ok {
			reports = append(reports, r)
		}
	}
	return reports, nil
}

// GetByTarget mock implementation.
func (m *MockReportRepository) GetByTarget(ctx context.Context, targetID, targetType string, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.TargetID == targetID && r.TargetType == targetType {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetByReporter mock implementation.
func (m *MockReportRepository) GetByReporter(ctx context.Context, reporterID string, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.ReporterID == reporterID {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetByReviewer mock implementation.
func (m *MockReportRepository) GetByReviewer(ctx context.Context, reviewerID string, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.ReviewerID == reviewerID {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// Update mock implementation.
func (m *MockReportRepository) Update(ctx context.Context, report *entities.Report) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Reports[report.ID]; !ok {
		return ErrReportNotFound
	}
	m.Reports[report.ID] = report
	return nil
}

// Delete mock implementation.
func (m *MockReportRepository) Delete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Reports[id]; ok {
		delete(m.Reports, id)
		return nil
	}
	return ErrReportNotFound
}

// DeleteByTarget mock implementation.
func (m *MockReportRepository) DeleteByTarget(ctx context.Context, targetID, targetType string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, r := range m.Reports {
		if r.TargetID == targetID && r.TargetType == targetType {
			delete(m.Reports, id)
		}
	}
	return nil
}

// UpdateStatus mock implementation.
func (m *MockReportRepository) UpdateStatus(ctx context.Context, id, status string, reviewerID string, notes string) error {
	if m.Error != nil {
		return m.Error
	}
	if report, ok := m.Reports[id]; ok {
		report.Status = status
		report.ReviewerID = reviewerID
		report.ReviewNotes = notes
		if status == ReportStatusResolved || status == ReportStatusDismissed {
			now := time.Now()
			report.ResolvedAt = &now
		}
		return nil
	}
	return ErrReportNotFound
}

// ResolveReport mock implementation.
func (m *MockReportRepository) ResolveReport(ctx context.Context, id, reviewerID, notes string) error {
	return m.UpdateStatus(ctx, id, ReportStatusResolved, reviewerID, notes)
}

// DismissReport mock implementation.
func (m *MockReportRepository) DismissReport(ctx context.Context, id, reviewerID, notes string) error {
	return m.UpdateStatus(ctx, id, ReportStatusDismissed, reviewerID, notes)
}

// ReopenReport mock implementation.
func (m *MockReportRepository) ReopenReport(ctx context.Context, id, reviewerID, notes string) error {
	if m.Error != nil {
		return m.Error
	}
	if report, ok := m.Reports[id]; ok {
		report.Status = ReportStatusPending
		report.ReviewerID = reviewerID
		report.ReviewNotes = notes
		report.ResolvedAt = nil
		return nil
	}
	return ErrReportNotFound
}

// AssignReviewer mock implementation.
func (m *MockReportRepository) AssignReviewer(ctx context.Context, id, reviewerID string) error {
	if m.Error != nil {
		return m.Error
	}
	if report, ok := m.Reports[id]; ok {
		report.ReviewerID = reviewerID
		return nil
	}
	return ErrReportNotFound
}

// UnassignReviewer mock implementation.
func (m *MockReportRepository) UnassignReviewer(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if report, ok := m.Reports[id]; ok {
		report.ReviewerID = ""
		return nil
	}
	return ErrReportNotFound
}

// UpdateSeverity mock implementation.
func (m *MockReportRepository) UpdateSeverity(ctx context.Context, id, severity string) error {
	if m.Error != nil {
		return m.Error
	}
	if report, ok := m.Reports[id]; ok {
		report.Severity = severity
		return nil
	}
	return ErrReportNotFound
}

// AddReviewNote mock implementation.
func (m *MockReportRepository) AddReviewNote(ctx context.Context, id, note string) error {
	if m.Error != nil {
		return m.Error
	}
	if report, ok := m.Reports[id]; ok {
		if report.ReviewNotes != "" {
			report.ReviewNotes += "\n" + note
		} else {
			report.ReviewNotes = note
		}
		return nil
	}
	return ErrReportNotFound
}

// Exists mock implementation.
func (m *MockReportRepository) Exists(ctx context.Context, id string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	_, ok := m.Reports[id]
	return ok, nil
}

// CheckDuplicate mock implementation.
func (m *MockReportRepository) CheckDuplicate(ctx context.Context, reporterID, targetID, targetType string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	for _, r := range m.Reports {
		if r.ReporterID == reporterID && r.TargetID == targetID && r.TargetType == targetType {
			return true, nil
		}
	}
	return false, nil
}

// GetDuplicateReports mock implementation.
func (m *MockReportRepository) GetDuplicateReports(ctx context.Context, targetID, targetType string) ([]*entities.Report, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.TargetID == targetID && r.TargetType == targetType {
			reports = append(reports, r)
		}
	}
	return reports, nil
}

// List mock implementation.
func (m *MockReportRepository) List(ctx context.Context, filter *ReportFilter, pagination *ReportPagination) ([]*entities.Report, int64, error) {
	if m.Error != nil {
		return nil, 0, m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		reports = append(reports, r)
	}
	return reports, int64(len(reports)), nil
}

// GetPending mock implementation.
func (m *MockReportRepository) GetPending(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.Status == ReportStatusPending {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetUnderReview mock implementation.
func (m *MockReportRepository) GetUnderReview(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.Status == ReportStatusUnderReview {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetResolved mock implementation.
func (m *MockReportRepository) GetResolved(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.Status == ReportStatusResolved {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetDismissed mock implementation.
func (m *MockReportRepository) GetDismissed(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.Status == ReportStatusDismissed {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetByDateRange mock implementation.
func (m *MockReportRepository) GetByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.CreatedAt.After(start) && r.CreatedAt.Before(end) {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetBySeverity mock implementation.
func (m *MockReportRepository) GetBySeverity(ctx context.Context, severity string, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.Severity == severity {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetByReason mock implementation.
func (m *MockReportRepository) GetByReason(ctx context.Context, reason string, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.Reason == reason {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// GetByTargetType mock implementation.
func (m *MockReportRepository) GetByTargetType(ctx context.Context, targetType string, cursor string, limit int) ([]*entities.Report, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.TargetType == targetType {
			reports = append(reports, r)
		}
	}
	return reports, "", nil
}

// CountTotal mock implementation.
func (m *MockReportRepository) CountTotal(ctx context.Context) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return int64(len(m.Reports)), nil
}

// CountByStatus mock implementation.
func (m *MockReportRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, r := range m.Reports {
		if r.Status == status {
			count++
		}
	}
	return count, nil
}

// CountBySeverity mock implementation.
func (m *MockReportRepository) CountBySeverity(ctx context.Context, severity string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, r := range m.Reports {
		if r.Severity == severity {
			count++
		}
	}
	return count, nil
}

// CountByTarget mock implementation.
func (m *MockReportRepository) CountByTarget(ctx context.Context, targetID, targetType string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, r := range m.Reports {
		if r.TargetID == targetID && r.TargetType == targetType {
			count++
		}
	}
	return count, nil
}

// CountByReporter mock implementation.
func (m *MockReportRepository) CountByReporter(ctx context.Context, reporterID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, r := range m.Reports {
		if r.ReporterID == reporterID {
			count++
		}
	}
	return count, nil
}

// CountByReviewer mock implementation.
func (m *MockReportRepository) CountByReviewer(ctx context.Context, reviewerID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, r := range m.Reports {
		if r.ReviewerID == reviewerID {
			count++
		}
	}
	return count, nil
}

// CountByDateRange mock implementation.
func (m *MockReportRepository) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, r := range m.Reports {
		if r.CreatedAt.After(start) && r.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// CountPendingBySeverity mock implementation.
func (m *MockReportRepository) CountPendingBySeverity(ctx context.Context) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, r := range m.Reports {
		if r.Status == ReportStatusPending {
			result[r.Severity]++
		}
	}
	return result, nil
}

// BulkCreate mock implementation.
func (m *MockReportRepository) BulkCreate(ctx context.Context, reports []*entities.Report) error {
	if m.Error != nil {
		return m.Error
	}
	for _, r := range reports {
		_ = m.Create(ctx, r)
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockReportRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.Delete(ctx, id)
	}
	return nil
}

// BulkUpdateStatus mock implementation.
func (m *MockReportRepository) BulkUpdateStatus(ctx context.Context, ids []string, status, reviewerID, notes string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.UpdateStatus(ctx, id, status, reviewerID, notes)
	}
	return nil
}

// BulkAssignReviewer mock implementation.
func (m *MockReportRepository) BulkAssignReviewer(ctx context.Context, ids []string, reviewerID string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.AssignReviewer(ctx, id, reviewerID)
	}
	return nil
}

// BulkResolve mock implementation.
func (m *MockReportRepository) BulkResolve(ctx context.Context, ids []string, reviewerID, notes string) error {
	return m.BulkUpdateStatus(ctx, ids, ReportStatusResolved, reviewerID, notes)
}

// BulkDismiss mock implementation.
func (m *MockReportRepository) BulkDismiss(ctx context.Context, ids []string, reviewerID, notes string) error {
	return m.BulkUpdateStatus(ctx, ids, ReportStatusDismissed, reviewerID, notes)
}

// BulkDeleteByTarget mock implementation.
func (m *MockReportRepository) BulkDeleteByTarget(ctx context.Context, pairs []TargetPair) error {
	if m.Error != nil {
		return m.Error
	}
	for _, pair := range pairs {
		_ = m.DeleteByTarget(ctx, pair.TargetID, pair.TargetType)
	}
	return nil
}

// GetReportStats mock implementation.
func (m *MockReportRepository) GetReportStats(ctx context.Context) (*ReportStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	stats := &ReportStats{
		TotalReports: int64(len(m.Reports)),
		SeverityStats: make(map[string]int64),
		ReasonStats:   make(map[string]int64),
		TargetTypeStats: make(map[string]int64),
	}
	for _, r := range m.Reports {
		stats.SeverityStats[r.Severity]++
		stats.ReasonStats[r.Reason]++
		stats.TargetTypeStats[r.TargetType]++
		if r.Status == ReportStatusPending {
			stats.PendingReports++
		} else if r.Status == ReportStatusUnderReview {
			stats.UnderReviewReports++
		} else if r.Status == ReportStatusResolved {
			stats.ResolvedReports++
		} else if r.Status == ReportStatusDismissed {
			stats.DismissedReports++
		}
	}
	return stats, nil
}

// GetUserReportStats mock implementation.
func (m *MockReportRepository) GetUserReportStats(ctx context.Context, userID string) (*ReportStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	stats := &ReportStats{}
	for _, r := range m.Reports {
		if r.ReporterID == userID {
			stats.TotalReports++
			if r.Status == ReportStatusPending {
				stats.PendingReports++
			}
		}
	}
	return stats, nil
}

// GetReviewerStats mock implementation.
func (m *MockReportRepository) GetReviewerStats(ctx context.Context, reviewerID string) (*ReportStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	stats := &ReportStats{}
	for _, r := range m.Reports {
		if r.ReviewerID == reviewerID {
			stats.TotalReports++
			if r.Status == ReportStatusResolved {
				stats.ResolvedReports++
			} else if r.Status == ReportStatusDismissed {
				stats.DismissedReports++
			}
		}
	}
	return stats, nil
}

// GetDailyReportStats mock implementation.
func (m *MockReportRepository) GetDailyReportStats(ctx context.Context, start, end time.Time) ([]*DailyReportCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyReportCount{}, nil
}

// GetReportTypeStats mock implementation.
func (m *MockReportRepository) GetReportTypeStats(ctx context.Context) ([]*ReportTypeStat, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	// Aggregate by target type
	statsMap := make(map[string]*ReportTypeStat)
	for _, r := range m.Reports {
		if _, ok := statsMap[r.TargetType]; !ok {
			statsMap[r.TargetType] = &ReportTypeStat{TargetType: r.TargetType}
		}
		statsMap[r.TargetType].Count++
	}
	var result []*ReportTypeStat
	for _, v := range statsMap {
		result = append(result, v)
	}
	return result, nil
}

// GetReasonStats mock implementation.
func (m *MockReportRepository) GetReasonStats(ctx context.Context) ([]*ReasonStat, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	statsMap := make(map[string]*ReasonStat)
	for _, r := range m.Reports {
		if _, ok := statsMap[r.Reason]; !ok {
			statsMap[r.Reason] = &ReasonStat{Reason: r.Reason}
		}
		statsMap[r.Reason].Count++
	}
	var result []*ReasonStat
	for _, v := range statsMap {
		result = append(result, v)
	}
	return result, nil
}

// GetSeverityDistribution mock implementation.
func (m *MockReportRepository) GetSeverityDistribution(ctx context.Context) (map[string]float64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	stats, _ := m.GetReportStats(ctx)
	total := float64(stats.TotalReports)
	if total == 0 {
		return map[string]float64{}, nil
	}
	dist := make(map[string]float64)
	for sev, count := range stats.SeverityStats {
		dist[sev] = (float64(count) / total) * 100
	}
	return dist, nil
}

// GetReportVelocity mock implementation.
func (m *MockReportRepository) GetReportVelocity(ctx context.Context, days int) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	// Count reports in last 'days' days
	since := time.Now().AddDate(0, 0, -days)
	count := int64(0)
	for _, r := range m.Reports {
		if r.CreatedAt.After(since) {
			count++
		}
	}
	return float64(count) / float64(days), nil
}

// GetAverageResolutionTimeBySeverity mock implementation.
func (m *MockReportRepository) GetAverageResolutionTimeBySeverity(ctx context.Context) (map[string]float64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return map[string]float64{}, nil
}

// GetMostReportedTargets mock implementation.
func (m *MockReportRepository) GetMostReportedTargets(ctx context.Context, limit int, since time.Time) ([]*ReportedTarget, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*ReportedTarget{}, nil
}

// GetMostActiveReporters mock implementation.
func (m *MockReportRepository) GetMostActiveReporters(ctx context.Context, limit int, since time.Time) ([]*ActiveReporter, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*ActiveReporter{}, nil
}

// GetActionableReports mock implementation.
func (m *MockReportRepository) GetActionableReports(ctx context.Context, limit int) ([]*entities.Report, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var reports []*entities.Report
	for _, r := range m.Reports {
		if r.Status == ReportStatusPending || r.Status == ReportStatusUnderReview {
			reports = append(reports, r)
		}
	}
	return reports, nil
}

// GetReportsForModeration mock implementation.
func (m *MockReportRepository) GetReportsForModeration(ctx context.Context, moderatorID string, limit int) ([]*entities.Report, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.GetActionableReports(ctx, limit)
}

// RecordModerationAction mock implementation.
func (m *MockReportRepository) RecordModerationAction(ctx context.Context, reportID, action, performedBy string, details map[string]interface{}) error {
	if m.Error != nil {
		return m.Error
	}
	return nil
}

// GetModerationHistory mock implementation.
func (m *MockReportRepository) GetModerationHistory(ctx context.Context, reportID string) ([]*ModerationAction, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*ModerationAction{}, nil
}

// WithTransaction mock implementation.
func (m *MockReportRepository) WithTransaction(ctx context.Context, tx *sql.Tx) ReportRepository {
	return m
}

// Transaction mock implementation.
func (m *MockReportRepository) Transaction(ctx context.Context, fn func(txRepo ReportRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockReportRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockReportRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockReportRepository) GetRawDB() interface{} {
	return nil
}