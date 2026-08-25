// backend/internal/domain/entities/report.go
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

// ReportStatus represents the status of a report.
type ReportStatus string

const (
	ReportStatusPending     ReportStatus = "pending"
	ReportStatusUnderReview ReportStatus = "under_review"
	ReportStatusResolved    ReportStatus = "resolved"
	ReportStatusDismissed   ReportStatus = "dismissed"
	ReportStatusEscalated   ReportStatus = "escalated"
	ReportStatusClosed      ReportStatus = "closed"
)

// ValidReportStatuses returns all valid report statuses.
func ValidReportStatuses() []ReportStatus {
	return []ReportStatus{
		ReportStatusPending,
		ReportStatusUnderReview,
		ReportStatusResolved,
		ReportStatusDismissed,
		ReportStatusEscalated,
		ReportStatusClosed,
	}
}

// IsValid checks if a report status is valid.
func (s ReportStatus) IsValid() bool {
	for _, status := range ValidReportStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation of the status.
func (s ReportStatus) String() string {
	return string(s)
}

// IsTerminal checks if the status is terminal (no further changes).
func (s ReportStatus) IsTerminal() bool {
	return s == ReportStatusResolved || s == ReportStatusDismissed || s == ReportStatusClosed
}

// ReportType represents the type of report.
type ReportType string

const (
	ReportTypeSpam        ReportType = "spam"
	ReportTypeHarassment  ReportType = "harassment"
	ReportTypeHateSpeech  ReportType = "hate_speech"
	ReportTypeInappropriate ReportType = "inappropriate"
	ReportTypeMisleading  ReportType = "misleading"
	ReportTypeCopyright   ReportType = "copyright"
	ReportTypeImpersonation ReportType = "impersonation"
	ReportTypeSelfHarm    ReportType = "self_harm"
	ReportTypeViolence    ReportType = "violence"
	ReportTypeNudity      ReportType = "nudity"
	ReportTypeOther       ReportType = "other"
)

// ValidReportTypes returns all valid report types.
func ValidReportTypes() []ReportType {
	return []ReportType{
		ReportTypeSpam,
		ReportTypeHarassment,
		ReportTypeHateSpeech,
		ReportTypeInappropriate,
		ReportTypeMisleading,
		ReportTypeCopyright,
		ReportTypeImpersonation,
		ReportTypeSelfHarm,
		ReportTypeViolence,
		ReportTypeNudity,
		ReportTypeOther,
	}
}

// IsValid checks if a report type is valid.
func (t ReportType) IsValid() bool {
	for _, typ := range ValidReportTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation of the type.
func (t ReportType) String() string {
	return string(t)
}

// ReportSeverity represents the severity level of a report.
type ReportSeverity string

const (
	SeverityLow      ReportSeverity = "low"
	SeverityMedium   ReportSeverity = "medium"
	SeverityHigh     ReportSeverity = "high"
	SeverityCritical ReportSeverity = "critical"
)

// ValidReportSeverities returns all valid report severities.
func ValidReportSeverities() []ReportSeverity {
	return []ReportSeverity{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}
}

// IsValid checks if a report severity is valid.
func (s ReportSeverity) IsValid() bool {
	for _, sev := range ValidReportSeverities() {
		if s == sev {
			return true
		}
	}
	return false
}

// String returns the string representation of the severity.
func (s ReportSeverity) String() string {
	return string(s)
}

// Priority returns the numeric priority of the severity.
func (s ReportSeverity) Priority() int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

// ======================================================================
// Errors
// ======================================================================

var (
	ErrReportIDEmpty          = errors.New("report ID cannot be empty")
	ErrReporterIDEmpty        = errors.New("reporter ID cannot be empty")
	ErrTargetIDEmpty          = errors.New("target ID cannot be empty")
	ErrTargetTypeEmpty        = errors.New("target type cannot be empty")
	ErrReasonEmpty            = errors.New("reason cannot be empty")
	ErrReasonTooLong          = errors.New("reason exceeds maximum length")
	ErrInvalidReportStatus    = errors.New("invalid report status")
	ErrInvalidReportType      = errors.New("invalid report type")
	ErrInvalidReportSeverity  = errors.New("invalid report severity")
	ErrReportAlreadyResolved  = errors.New("report already resolved")
	ErrReportAlreadyDismissed = errors.New("report already dismissed")
	ErrReportAlreadyDeleted   = errors.New("report already deleted")
	ErrReportNotDeleted       = errors.New("report is not deleted")
	ErrReportCannotReopen     = errors.New("cannot reopen resolved/dismissed report")
	ErrReportCannotEscalate   = errors.New("cannot escalate non-review report")
	ErrReportReviewerRequired = errors.New("reviewer ID is required")
	ErrReportNotesTooLong     = errors.New("review notes exceed maximum length")
	ErrReportDuplicate        = errors.New("duplicate report from same user")
)

// ======================================================================
// Report Entity
// ======================================================================

// Report represents a user report against a target (tweet, user, etc.).
type Report struct {
	ID          string          `db:"id" json:"id"`
	ReporterID  string          `db:"reporter_id" json:"reporter_id"`
	TargetID    string          `db:"target_id" json:"target_id"`
	TargetType  string          `db:"target_type" json:"target_type"` // "tweet", "user", "community", "message"
	Type        ReportType      `db:"type" json:"type"`
	Reason      string          `db:"reason" json:"reason"`
	Description string          `db:"description" json:"description,omitempty"`
	Status      ReportStatus    `db:"status" json:"status"`
	Severity    ReportSeverity  `db:"severity" json:"severity"`
	ReviewerID  string          `db:"reviewer_id" json:"reviewer_id,omitempty"`
	ReviewNotes string          `db:"review_notes" json:"review_notes,omitempty"`
	ResolvedAt  *time.Time      `db:"resolved_at" json:"resolved_at,omitempty"`
	Metadata    ReportMetadata  `db:"metadata" json:"metadata,omitempty"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time      `db:"deleted_at" json:"deleted_at,omitempty"`
}

// ReportMetadata holds optional report metadata.
type ReportMetadata struct {
	IPAddress    string            `json:"ip_address,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	Referer      string            `json:"referer,omitempty"`
	EvidenceURLs []string          `json:"evidence_urls,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
	Automated    bool              `json:"automated"`
	Priority     int               `json:"priority"`
}

// Value implements driver.Valuer for JSON storage.
func (m ReportMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements sql.Scanner for JSON retrieval.
func (m *ReportMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = ReportMetadata{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for ReportMetadata: %T", value)
	}
	return json.Unmarshal(bytes, m)
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewReport creates a new report with default values.
func NewReport(reporterID, targetID, targetType string, reportType ReportType, reason, description string) (*Report, error) {
	if !reportType.IsValid() {
		reportType = ReportTypeOther
	}
	r := &Report{
		ID:          uuid.New().String(),
		ReporterID:  reporterID,
		TargetID:    targetID,
		TargetType:  targetType,
		Type:        reportType,
		Reason:      reason,
		Description: description,
		Status:      ReportStatusPending,
		Severity:    SeverityMedium,
		Metadata:    ReportMetadata{Priority: 0},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return r, nil
}

// NewReportWithSeverity creates a new report with severity.
func NewReportWithSeverity(reporterID, targetID, targetType string, reportType ReportType, reason, description string, severity ReportSeverity) (*Report, error) {
	r, err := NewReport(reporterID, targetID, targetType, reportType, reason, description)
	if err != nil {
		return nil, err
	}
	if severity.IsValid() {
		r.Severity = severity
	}
	return r, nil
}

// MustNewReport creates a new report and panics on error.
func MustNewReport(reporterID, targetID, targetType string, reportType ReportType, reason, description string) *Report {
	r, err := NewReport(reporterID, targetID, targetType, reportType, reason, description)
	if err != nil {
		panic(err)
	}
	return r
}

// ======================================================================
// Validation
// ======================================================================

// Validate performs comprehensive validation.
func (r *Report) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrReportIDEmpty
	}
	if strings.TrimSpace(r.ReporterID) == "" {
		return ErrReporterIDEmpty
	}
	if strings.TrimSpace(r.TargetID) == "" {
		return ErrTargetIDEmpty
	}
	if strings.TrimSpace(r.TargetType) == "" {
		return ErrTargetTypeEmpty
	}
	if !r.Type.IsValid() {
		return ErrInvalidReportType
	}
	reasonTrimmed := strings.TrimSpace(r.Reason)
	if reasonTrimmed == "" {
		return ErrReasonEmpty
	}
	if len(reasonTrimmed) > 200 {
		return ErrReasonTooLong
	}
	r.Reason = reasonTrimmed
	if len(r.Description) > 1000 {
		return errors.New("description exceeds maximum length")
	}
	if !r.Status.IsValid() {
		return ErrInvalidReportStatus
	}
	if !r.Severity.IsValid() {
		return ErrInvalidReportSeverity
	}
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	return nil
}

// ======================================================================
// Status Management
// ======================================================================

// SetStatus sets the report status.
func (r *Report) SetStatus(status ReportStatus) error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if !status.IsValid() {
		return ErrInvalidReportStatus
	}
	if r.Status.IsTerminal() && status != r.Status {
		return ErrReportCannotReopen
	}
	r.Status = status
	r.UpdatedAt = time.Now()
	if status.IsTerminal() {
		now := time.Now()
		r.ResolvedAt = &now
	}
	return nil
}

// MarkUnderReview marks the report as under review.
func (r *Report) MarkUnderReview(reviewerID string) error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if r.Status == ReportStatusResolved || r.Status == ReportStatusDismissed {
		return ErrReportCannotReopen
	}
	if strings.TrimSpace(reviewerID) == "" {
		return ErrReportReviewerRequired
	}
	r.Status = ReportStatusUnderReview
	r.ReviewerID = reviewerID
	r.UpdatedAt = time.Now()
	return nil
}

// Resolve resolves the report.
func (r *Report) Resolve(reviewerID, notes string) error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if r.Status == ReportStatusResolved {
		return ErrReportAlreadyResolved
	}
	if strings.TrimSpace(reviewerID) == "" {
		return ErrReportReviewerRequired
	}
	r.Status = ReportStatusResolved
	r.ReviewerID = reviewerID
	r.ReviewNotes = notes
	now := time.Now()
	r.ResolvedAt = &now
	r.UpdatedAt = now
	return nil
}

// Dismiss dismisses the report.
func (r *Report) Dismiss(reviewerID, notes string) error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if r.Status == ReportStatusDismissed {
		return ErrReportAlreadyDismissed
	}
	if strings.TrimSpace(reviewerID) == "" {
		return ErrReportReviewerRequired
	}
	r.Status = ReportStatusDismissed
	r.ReviewerID = reviewerID
	r.ReviewNotes = notes
	now := time.Now()
	r.ResolvedAt = &now
	r.UpdatedAt = now
	return nil
}

// Escalate escalates the report.
func (r *Report) Escalate(reviewerID, notes string) error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if r.Status.IsTerminal() {
		return ErrReportCannotEscalate
	}
	if strings.TrimSpace(reviewerID) == "" {
		return ErrReportReviewerRequired
	}
	r.Status = ReportStatusEscalated
	r.ReviewerID = reviewerID
	r.ReviewNotes = notes
	r.UpdatedAt = time.Now()
	return nil
}

// Reopen reopens a resolved/dismissed report.
func (r *Report) Reopen(reviewerID, notes string) error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if !r.Status.IsTerminal() {
		return errors.New("report is not resolved or dismissed")
	}
	if strings.TrimSpace(reviewerID) == "" {
		return ErrReportReviewerRequired
	}
	r.Status = ReportStatusPending
	r.ReviewerID = reviewerID
	r.ReviewNotes = notes
	r.ResolvedAt = nil
	r.UpdatedAt = time.Now()
	return nil
}

// IsPending checks if the report is pending.
func (r *Report) IsPending() bool {
	return r.Status == ReportStatusPending
}

// IsUnderReview checks if the report is under review.
func (r *Report) IsUnderReview() bool {
	return r.Status == ReportStatusUnderReview
}

// IsResolved checks if the report is resolved.
func (r *Report) IsResolved() bool {
	return r.Status == ReportStatusResolved
}

// IsDismissed checks if the report is dismissed.
func (r *Report) IsDismissed() bool {
	return r.Status == ReportStatusDismissed
}

// IsEscalated checks if the report is escalated.
func (r *Report) IsEscalated() bool {
	return r.Status == ReportStatusEscalated
}

// IsTerminal checks if the report status is terminal.
func (r *Report) IsTerminal() bool {
	return r.Status.IsTerminal()
}

// IsActive checks if the report is active (not terminal and not deleted).
func (r *Report) IsActive() bool {
	return !r.IsTerminal() && r.DeletedAt == nil
}

// ======================================================================
// Severity Management
// ======================================================================

// SetSeverity sets the report severity.
func (r *Report) SetSeverity(severity ReportSeverity) error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if !severity.IsValid() {
		return ErrInvalidReportSeverity
	}
	if r.Status.IsTerminal() {
		return errors.New("cannot change severity of terminal report")
	}
	r.Severity = severity
	r.UpdatedAt = time.Now()
	return nil
}

// GetPriority returns the numeric priority of the report.
func (r *Report) GetPriority() int {
	return r.Severity.Priority()
}

// ======================================================================
// Reviewer Management
// ======================================================================

// AssignReviewer assigns a reviewer to the report.
func (r *Report) AssignReviewer(reviewerID string) error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if r.Status.IsTerminal() {
		return errors.New("cannot assign reviewer to terminal report")
	}
	if strings.TrimSpace(reviewerID) == "" {
		return ErrReportReviewerRequired
	}
	r.ReviewerID = reviewerID
	r.UpdatedAt = time.Now()
	return nil
}

// UnassignReviewer removes the reviewer assignment.
func (r *Report) UnassignReviewer() error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	if r.Status.IsTerminal() {
		return errors.New("cannot unassign reviewer from terminal report")
	}
	r.ReviewerID = ""
	r.UpdatedAt = time.Now()
	return nil
}

// HasReviewer checks if a reviewer is assigned.
func (r *Report) HasReviewer() bool {
	return strings.TrimSpace(r.ReviewerID) != ""
}

// IsReviewedBy checks if the report is reviewed by a specific reviewer.
func (r *Report) IsReviewedBy(reviewerID string) bool {
	return r.ReviewerID == reviewerID
}

// ======================================================================
= Deletion Operations
// ======================================================================

// SoftDelete marks the report as deleted.
func (r *Report) SoftDelete() error {
	if r.DeletedAt != nil {
		return ErrReportAlreadyDeleted
	}
	now := time.Now()
	r.DeletedAt = &now
	r.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted report.
func (r *Report) Restore() error {
	if r.DeletedAt == nil {
		return ErrReportNotDeleted
	}
	r.DeletedAt = nil
	r.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the report is deleted.
func (r *Report) IsDeleted() bool {
	return r.DeletedAt != nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// IsReporter checks if a user is the reporter.
func (r *Report) IsReporter(userID string) bool {
	return r.ReporterID == userID
}

// IsTarget checks if the report targets a specific ID.
func (r *Report) IsTarget(targetID string) bool {
	return r.TargetID == targetID
}

// IsTargetType checks if the report targets a specific type.
func (r *Report) IsTargetType(targetType string) bool {
	return r.TargetType == targetType
}

// IsType checks if the report is of a specific type.
func (r *Report) IsType(reportType ReportType) bool {
	return r.Type == reportType
}

// String returns a human-readable representation.
func (r *Report) String() string {
	return fmt.Sprintf("Report{ID:%s, reporter:%s, target:%s, type:%s, status:%s, severity:%s, created:%v}",
		r.ID, r.ReporterID, r.TargetID, r.Type, r.Status, r.Severity, r.CreatedAt)
}

// Clone returns a deep copy of the report.
func (r *Report) Clone() *Report {
	clone := *r
	if r.ResolvedAt != nil {
		t := *r.ResolvedAt
		clone.ResolvedAt = &t
	}
	if r.DeletedAt != nil {
		t := *r.DeletedAt
		clone.DeletedAt = &t
	}
	if r.Metadata.EvidenceURLs != nil {
		clone.Metadata.EvidenceURLs = make([]string, len(r.Metadata.EvidenceURLs))
		copy(clone.Metadata.EvidenceURLs, r.Metadata.EvidenceURLs)
	}
	if r.Metadata.CustomFields != nil {
		clone.Metadata.CustomFields = make(map[string]string)
		for k, v := range r.Metadata.CustomFields {
			clone.Metadata.CustomFields[k] = v
		}
	}
	return &clone
}

// Equals checks if two reports are the same by ID.
func (r *Report) Equals(other *Report) bool {
	return r.ID == other.ID
}

// IsEmpty returns true if the report is zero value.
func (r *Report) IsEmpty() bool {
	return r.ID == "" && r.ReporterID == "" && r.TargetID == ""
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (r Report) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (r *Report) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for Report: %T", value)
	}
	return json.Unmarshal(bytes, r)
}

// ======================================================================
// JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *Report) MarshalJSON() ([]byte, error) {
	type Alias Report
	return json.Marshal(&struct {
		*Alias
		Status   string `json:"status"`
		Type     string `json:"type"`
		Severity string `json:"severity"`
		Priority int    `json:"priority"`
		IsActive bool   `json:"is_active"`
	}{
		Alias:    (*Alias)(r),
		Status:   string(r.Status),
		Type:     string(r.Type),
		Severity: string(r.Severity),
		Priority: r.GetPriority(),
		IsActive: r.IsActive(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *Report) UnmarshalJSON(data []byte) error {
	type Alias Report
	aux := &struct {
		*Alias
		Status   string `json:"status"`
		Type     string `json:"type"`
		Severity string `json:"severity"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Status != "" {
		r.Status = ReportStatus(aux.Status)
	}
	if aux.Type != "" {
		r.Type = ReportType(aux.Type)
	}
	if aux.Severity != "" {
		r.Severity = ReportSeverity(aux.Severity)
	}
	return nil
}

// ======================================================================
// Report Group (for batch operations)
// ======================================================================

// ReportGroup represents a group of reports.
type ReportGroup struct {
	Reports []*Report `json:"reports"`
	Total   int64     `json:"total"`
}

// NewReportGroup creates a new report group.
func NewReportGroup() *ReportGroup {
	return &ReportGroup{
		Reports: []*Report{},
		Total:   0,
	}
}

// Add adds a report to the group.
func (g *ReportGroup) Add(r *Report) {
	g.Reports = append(g.Reports, r)
	g.Total++
}

// Contains checks if a report is in the group.
func (g *ReportGroup) Contains(id string) bool {
	for _, r := range g.Reports {
		if r.ID == id {
			return true
		}
	}
	return false
}

// FilterByStatus returns reports with a specific status.
func (g *ReportGroup) FilterByStatus(status ReportStatus) []*Report {
	result := []*Report{}
	for _, r := range g.Reports {
		if r.Status == status {
			result = append(result, r)
		}
	}
	return result
}

// FilterByType returns reports of a specific type.
func (g *ReportGroup) FilterByType(reportType ReportType) []*Report {
	result := []*Report{}
	for _, r := range g.Reports {
		if r.Type == reportType {
			result = append(result, r)
		}
	}
	return result
}

// FilterBySeverity returns reports with a specific severity.
func (g *ReportGroup) FilterBySeverity(severity ReportSeverity) []*Report {
	result := []*Report{}
	for _, r := range g.Reports {
		if r.Severity == severity {
			result = append(result, r)
		}
	}
	return result
}

// FilterByReporter returns reports by a specific reporter.
func (g *ReportGroup) FilterByReporter(reporterID string) []*Report {
	result := []*Report{}
	for _, r := range g.Reports {
		if r.ReporterID == reporterID {
			result = append(result, r)
		}
	}
	return result
}

// FilterByTarget returns reports targeting a specific ID.
func (g *ReportGroup) FilterByTarget(targetID string) []*Report {
	result := []*Report{}
	for _, r := range g.Reports {
		if r.TargetID == targetID {
			result = append(result, r)
		}
	}
	return result
}

// FilterByTargetType returns reports targeting a specific type.
func (g *ReportGroup) FilterByTargetType(targetType string) []*Report {
	result := []*Report{}
	for _, r := range g.Reports {
		if r.TargetType == targetType {
			result = append(result, r)
		}
	}
	return result
}

// GetPending returns pending reports.
func (g *ReportGroup) GetPending() []*Report {
	return g.FilterByStatus(ReportStatusPending)
}

// GetUnderReview returns reports under review.
func (g *ReportGroup) GetUnderReview() []*Report {
	return g.FilterByStatus(ReportStatusUnderReview)
}

// GetResolved returns resolved reports.
func (g *ReportGroup) GetResolved() []*Report {
	return g.FilterByStatus(ReportStatusResolved)
}

// GetDismissed returns dismissed reports.
func (g *ReportGroup) GetDismissed() []*Report {
	return g.FilterByStatus(ReportStatusDismissed)
}

// GetEscalated returns escalated reports.
func (g *ReportGroup) GetEscalated() []*Report {
	return g.FilterByStatus(ReportStatusEscalated)
}

// GetByPriority returns reports sorted by priority (highest first).
func (g *ReportGroup) GetByPriority() []*Report {
	result := make([]*Report, len(g.Reports))
	copy(result, g.Reports)
	sort.Slice(result, func(i, j int) bool {
		return result[i].GetPriority() > result[j].GetPriority()
	})
	return result
}

// ======================================================================
= Report Statistics
// ======================================================================

// ReportStats represents report statistics.
type ReportStats struct {
	TotalReports   int64            `json:"total_reports"`
	PendingCount   int64            `json:"pending_count"`
	UnderReview    int64            `json:"under_review"`
	ResolvedCount  int64            `json:"resolved_count"`
	DismissedCount int64            `json:"dismissed_count"`
	EscalatedCount int64            `json:"escalated_count"`
	ClosedCount    int64            `json:"closed_count"`
	StatusStats    map[string]int64 `json:"status_stats"`
	TypeStats      map[string]int64 `json:"type_stats"`
	SeverityStats  map[string]int64 `json:"severity_stats"`
	UniqueReporters int64           `json:"unique_reporters"`
	UniqueTargets  int64            `json:"unique_targets"`
	AvgResolution  float64          `json:"avg_resolution_hours"`
}

// CalculateStats calculates statistics from a report group.
func (g *ReportGroup) CalculateStats() *ReportStats {
	stats := &ReportStats{
		TotalReports: int64(len(g.Reports)),
		StatusStats:  make(map[string]int64),
		TypeStats:    make(map[string]int64),
		SeverityStats: make(map[string]int64),
	}
	reporters := make(map[string]bool)
	targets := make(map[string]bool)
	var totalResolution time.Duration
	var resolvedCount int64

	for _, r := range g.Reports {
		reporters[r.ReporterID] = true
		targets[r.TargetID] = true
		stats.StatusStats[string(r.Status)]++
		stats.TypeStats[string(r.Type)]++
		stats.SeverityStats[string(r.Severity)]++

		switch r.Status {
		case ReportStatusPending:
			stats.PendingCount++
		case ReportStatusUnderReview:
			stats.UnderReview++
		case ReportStatusResolved:
			stats.ResolvedCount++
			if r.ResolvedAt != nil {
				resolution := r.ResolvedAt.Sub(r.CreatedAt)
				totalResolution += resolution
				resolvedCount++
			}
		case ReportStatusDismissed:
			stats.DismissedCount++
		case ReportStatusEscalated:
			stats.EscalatedCount++
		case ReportStatusClosed:
			stats.ClosedCount++
		}
	}
	stats.UniqueReporters = int64(len(reporters))
	stats.UniqueTargets = int64(len(targets))
	if resolvedCount > 0 {
		stats.AvgResolution = totalResolution.Hours() / float64(resolvedCount)
	}
	return stats
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// ReportBuilder helps construct reports for testing.
type ReportBuilder struct {
	report *Report
}

// NewReportBuilder creates a new report builder.
func NewReportBuilder() *ReportBuilder {
	return &ReportBuilder{
		report: &Report{
			ID:          uuid.New().String(),
			ReporterID:  "",
			TargetID:    "",
			TargetType:  "tweet",
			Type:        ReportTypeSpam,
			Reason:      "Spam content",
			Description: "",
			Status:      ReportStatusPending,
			Severity:    SeverityMedium,
			Metadata:    ReportMetadata{Priority: 0},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *ReportBuilder) WithID(id string) *ReportBuilder {
	b.report.ID = id
	return b
}

// WithReporterID sets the reporter ID.
func (b *ReportBuilder) WithReporterID(reporterID string) *ReportBuilder {
	b.report.ReporterID = reporterID
	return b
}

// WithTargetID sets the target ID.
func (b *ReportBuilder) WithTargetID(targetID string) *ReportBuilder {
	b.report.TargetID = targetID
	return b
}

// WithTargetType sets the target type.
func (b *ReportBuilder) WithTargetType(targetType string) *ReportBuilder {
	b.report.TargetType = targetType
	return b
}

// WithType sets the report type.
func (b *ReportBuilder) WithType(reportType ReportType) *ReportBuilder {
	b.report.Type = reportType
	return b
}

// WithReason sets the reason.
func (b *ReportBuilder) WithReason(reason string) *ReportBuilder {
	b.report.Reason = reason
	return b
}

// WithDescription sets the description.
func (b *ReportBuilder) WithDescription(desc string) *ReportBuilder {
	b.report.Description = desc
	return b
}

// WithStatus sets the status.
func (b *ReportBuilder) WithStatus(status ReportStatus) *ReportBuilder {
	b.report.Status = status
	return b
}

// WithSeverity sets the severity.
func (b *ReportBuilder) WithSeverity(severity ReportSeverity) *ReportBuilder {
	b.report.Severity = severity
	return b
}

// WithReviewerID sets the reviewer ID.
func (b *ReportBuilder) WithReviewerID(reviewerID string) *ReportBuilder {
	b.report.ReviewerID = reviewerID
	return b
}

// WithReviewNotes sets the review notes.
func (b *ReportBuilder) WithReviewNotes(notes string) *ReportBuilder {
	b.report.ReviewNotes = notes
	return b
}

// WithResolvedAt sets the resolved timestamp.
func (b *ReportBuilder) WithResolvedAt(t time.Time) *ReportBuilder {
	b.report.ResolvedAt = &t
	return b
}

// WithMetadata sets metadata.
func (b *ReportBuilder) WithMetadata(metadata ReportMetadata) *ReportBuilder {
	b.report.Metadata = metadata
	return b
}

// WithCreatedAt sets the creation time.
func (b *ReportBuilder) WithCreatedAt(t time.Time) *ReportBuilder {
	b.report.CreatedAt = t
	b.report.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *ReportBuilder) WithDeleted(t time.Time) *ReportBuilder {
	b.report.DeletedAt = &t
	return b
}

// Build validates and returns the report.
func (b *ReportBuilder) Build() (*Report, error) {
	if err := b.report.Validate(); err != nil {
		return nil, err
	}
	return b.report, nil
}

// MustBuild builds without error (panics on error).
func (b *ReportBuilder) MustBuild() *Report {
	r, err := b.Build()
	if err != nil {
		panic(err)
	}
	return r
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestReport1 = MustNewReport("user1", "tweet1", "tweet", ReportTypeSpam, "Spam content", "This is spam")
	TestReport2 = MustNewReport("user2", "user3", "user", ReportTypeHarassment, "Harassment", "User is harassing others")
	TestReport3 = MustNewReport("user1", "tweet2", "tweet", ReportTypeInappropriate, "Inappropriate content", "Contains inappropriate material")
)

// MustNewReportWithSeverity creates a report with severity and panics.
func MustNewReportWithSeverity(reporterID, targetID, targetType string, reportType ReportType, reason, description string, severity ReportSeverity) *Report {
	r, err := NewReportWithSeverity(reporterID, targetID, targetType, reportType, reason, description, severity)
	if err != nil {
		panic(err)
	}
	return r
}

// MustNewResolvedReport creates a resolved report for testing.
func MustNewResolvedReport(reporterID, targetID, targetType string, reportType ReportType, reason string) *Report {
	r := MustNewReport(reporterID, targetID, targetType, reportType, reason, "")
	_ = r.Resolve("reviewer1", "Resolved after review")
	return r
}

// MustNewDismissedReport creates a dismissed report for testing.
func MustNewDismissedReport(reporterID, targetID, targetType string, reportType ReportType, reason string) *Report {
	r := MustNewReport(reporterID, targetID, targetType, reportType, reason, "")
	_ = r.Dismiss("reviewer1", "Dismissed as invalid")
	return r
}

// MustNewDeletedReport creates a deleted report for testing.
func MustNewDeletedReport(reporterID, targetID, targetType string, reportType ReportType, reason string) *Report {
	r := MustNewReport(reporterID, targetID, targetType, reportType, reason, "")
	_ = r.SoftDelete()
	return r
}