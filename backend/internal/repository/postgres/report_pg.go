// backend/internal/repository/postgres/report_pg.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// reportRepo is the PostgreSQL implementation of ReportRepository.
type reportRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewReportRepository creates a new PostgreSQL report repository.
func NewReportRepository(db *sqlx.DB) interfaces.ReportRepository {
	return &reportRepo{
		db:  db,
		log: logger.WithField("repository", "report_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *reportRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.ReportRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &reportRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *reportRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.ReportRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &reportRepo{
		db:  r.db,
		tx:  tx,
		log: r.log.WithField("transaction", true),
	}
	err = fn(txRepo)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed after error: %v (original: %w)", rbErr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// getDB returns the current DB connection.
func (r *reportRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic Report CRUD
// ======================================================================

// Create inserts a new report.
func (r *reportRepo) Create(ctx context.Context, report *entities.Report) error {
	metadataJSON, err := json.Marshal(report.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	query := `
		INSERT INTO reports (
			id, reporter_id, target_id, target_type, reason,
			description, status, severity, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = r.getDB().ExecContext(ctx, query,
		report.ID, report.ReporterID, report.TargetID, report.TargetType,
		report.Reason, report.Description, report.Status, report.Severity,
		metadataJSON, report.CreatedAt, report.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create report failed: %w", err)
	}
	return nil
}

// GetByID retrieves a report by its ID.
func (r *reportRepo) GetByID(ctx context.Context, id string) (*entities.Report, error) {
	query := `SELECT * FROM reports WHERE id = $1`
	var report entities.Report
	err := r.getDB().GetContext(ctx, &report, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrReportNotFound
		}
		return nil, fmt.Errorf("get report by ID failed: %w", err)
	}
	return &report, nil
}

// Update updates a report (status, review notes, etc.).
func (r *reportRepo) Update(ctx context.Context, report *entities.Report) error {
	metadataJSON, err := json.Marshal(report.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	query := `
		UPDATE reports SET
			status = $1,
			severity = $2,
			reviewer_id = $3,
			review_notes = $4,
			resolved_at = $5,
			metadata = $6,
			updated_at = $7
		WHERE id = $8
	`
	result, err := r.getDB().ExecContext(ctx, query,
		report.Status, report.Severity, report.ReviewerID,
		report.ReviewNotes, report.ResolvedAt, metadataJSON,
		time.Now(), report.ID,
	)
	if err != nil {
		return fmt.Errorf("update report failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrReportNotFound
	}
	return nil
}

// Delete removes a report.
func (r *reportRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM reports WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete report failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrReportNotFound
	}
	return nil
}

// ======================================================================
= Status Operations
// ======================================================================

// UpdateStatus updates the status of a report.
func (r *reportRepo) UpdateStatus(ctx context.Context, id, status string, reviewerID string, notes string) error {
	resolvedAt := time.Time{}
	if status == entities.ReportStatusResolved || status == entities.ReportStatusDismissed {
		resolvedAt = time.Now()
	}
	query := `
		UPDATE reports SET
			status = $1,
			reviewer_id = $2,
			review_notes = $3,
			resolved_at = $4,
			updated_at = $5
		WHERE id = $6
	`
	result, err := r.getDB().ExecContext(ctx, query,
		status, reviewerID, notes, resolvedAt, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update report status failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrReportNotFound
	}
	return nil
}

// ResolveReport marks a report as resolved.
func (r *reportRepo) ResolveReport(ctx context.Context, id, reviewerID, notes string) error {
	return r.UpdateStatus(ctx, id, entities.ReportStatusResolved, reviewerID, notes)
}

// DismissReport marks a report as dismissed.
func (r *reportRepo) DismissReport(ctx context.Context, id, reviewerID, notes string) error {
	return r.UpdateStatus(ctx, id, entities.ReportStatusDismissed, reviewerID, notes)
}

// ReopenReport reopens a resolved/dismissed report.
func (r *reportRepo) ReopenReport(ctx context.Context, id, reviewerID, notes string) error {
	query := `
		UPDATE reports SET
			status = $1,
			reviewer_id = $2,
			review_notes = $3,
			resolved_at = NULL,
			updated_at = $4
		WHERE id = $5
	`
	result, err := r.getDB().ExecContext(ctx, query,
		entities.ReportStatusPending, reviewerID, notes, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("reopen report failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrReportNotFound
	}
	return nil
}

// ======================================================================
= List and Filter Operations
// ======================================================================

// List returns reports with filtering and pagination.
func (r *reportRepo) List(ctx context.Context, filter *interfaces.ReportFilter, pagination *interfaces.PaginationOptions) ([]*entities.Report, int64, error) {
	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if filter != nil {
		if filter.ReporterID != nil && *filter.ReporterID != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("reporter_id = $%d", argIdx))
			args = append(args, *filter.ReporterID)
			argIdx++
		}
		if filter.TargetID != nil && *filter.TargetID != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("target_id = $%d", argIdx))
			args = append(args, *filter.TargetID)
			argIdx++
		}
		if filter.TargetType != nil && *filter.TargetType != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("target_type = $%d", argIdx))
			args = append(args, *filter.TargetType)
			argIdx++
		}
		if filter.Status != nil && *filter.Status != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
			args = append(args, *filter.Status)
			argIdx++
		}
		if filter.Severity != nil && *filter.Severity != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("severity = $%d", argIdx))
			args = append(args, *filter.Severity)
			argIdx++
		}
		if filter.CreatedFrom != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", argIdx))
			args = append(args, *filter.CreatedFrom)
			argIdx++
		}
		if filter.CreatedTo != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", argIdx))
			args = append(args, *filter.CreatedTo)
			argIdx++
		}
		if filter.ReviewerID != nil && *filter.ReviewerID != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("reviewer_id = $%d", argIdx))
			args = append(args, *filter.ReviewerID)
			argIdx++
		}
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM reports WHERE %s", whereSQL)
	var total int64
	err := r.getDB().GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("count reports failed: %w", err)
	}

	// Set defaults
	limit := 20
	offset := 0
	sortBy := "created_at"
	order := "DESC"
	if pagination != nil {
		if pagination.Limit > 0 {
			limit = pagination.Limit
		}
		if pagination.Offset > 0 {
			offset = pagination.Offset
		}
		if pagination.SortBy != "" {
			sortBy = string(pagination.SortBy)
		}
		if pagination.Order != "" {
			order = string(pagination.Order)
		}
	}

	allowedSort := map[string]bool{
		"created_at": true, "updated_at": true, "severity": true,
		"status": true,
	}
	if !allowedSort[sortBy] {
		sortBy = "created_at"
	}
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT * FROM reports WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, sortBy, order, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var reports []*entities.Report
	err = r.getDB().SelectContext(ctx, &reports, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports failed: %w", err)
	}
	return reports, total, nil
}

// GetByReporterID returns reports filed by a specific user.
func (r *reportRepo) GetByReporterID(ctx context.Context, reporterID string, cursor string, limit int) ([]*entities.Report, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM reports
		WHERE reporter_id = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{reporterID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var reports []*entities.Report
	err := r.getDB().SelectContext(ctx, &reports, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get reports by reporter failed: %w", err)
	}
	var nextCursor string
	if len(reports) == limit {
		nextCursor = reports[len(reports)-1].ID
	}
	return reports, nextCursor, nil
}

// GetByTarget returns reports for a specific target.
func (r *reportRepo) GetByTarget(ctx context.Context, targetID, targetType string, cursor string, limit int) ([]*entities.Report, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM reports
		WHERE target_id = $1 AND target_type = $2
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{targetID, targetType}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var reports []*entities.Report
	err := r.getDB().SelectContext(ctx, &reports, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get reports by target failed: %w", err)
	}
	var nextCursor string
	if len(reports) == limit {
		nextCursor = reports[len(reports)-1].ID
	}
	return reports, nextCursor, nil
}

// GetByStatus returns reports by status.
func (r *reportRepo) GetByStatus(ctx context.Context, status string, cursor string, limit int) ([]*entities.Report, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM reports
		WHERE status = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY severity DESC, created_at DESC, id DESC LIMIT $?`

	args := []interface{}{status}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var reports []*entities.Report
	err := r.getDB().SelectContext(ctx, &reports, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get reports by status failed: %w", err)
	}
	var nextCursor string
	if len(reports) == limit {
		nextCursor = reports[len(reports)-1].ID
	}
	return reports, nextCursor, nil
}

// GetPendingReports returns pending reports sorted by severity.
func (r *reportRepo) GetPendingReports(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	return r.GetByStatus(ctx, entities.ReportStatusPending, cursor, limit)
}

// ======================================================================
= Count Operations
// ======================================================================

// CountByStatus returns the number of reports by status.
func (r *reportRepo) CountByStatus(ctx context.Context, status string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE status = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, status)
	if err != nil {
		return 0, fmt.Errorf("count reports by status failed: %w", err)
	}
	return count, nil
}

// CountBySeverity returns the number of reports by severity.
func (r *reportRepo) CountBySeverity(ctx context.Context, severity string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE severity = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, severity)
	if err != nil {
		return 0, fmt.Errorf("count reports by severity failed: %w", err)
	}
	return count, nil
}

// CountByTarget returns the number of reports for a target.
func (r *reportRepo) CountByTarget(ctx context.Context, targetID, targetType string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE target_id = $1 AND target_type = $2`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, targetID, targetType)
	if err != nil {
		return 0, fmt.Errorf("count reports by target failed: %w", err)
	}
	return count, nil
}

// CountByReporter returns the number of reports filed by a user.
func (r *reportRepo) CountByReporter(ctx context.Context, reporterID string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE reporter_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, reporterID)
	if err != nil {
		return 0, fmt.Errorf("count reports by reporter failed: %w", err)
	}
	return count, nil
}

// CountTotal returns total number of reports.
func (r *reportRepo) CountTotal(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM reports`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count total reports failed: %w", err)
	}
	return count, nil
}

// ======================================================================
= Bulk Operations
// ======================================================================

// BulkCreate inserts multiple reports in a transaction.
func (r *reportRepo) BulkCreate(ctx context.Context, reports []*entities.Report) error {
	if len(reports) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO reports (
			id, reporter_id, target_id, target_type, reason,
			description, status, severity, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, rpt := range reports {
		metadataJSON, err := json.Marshal(rpt.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata failed: %w", err)
		}
		_, err = stmt.ExecContext(ctx,
			rpt.ID, rpt.ReporterID, rpt.TargetID, rpt.TargetType,
			rpt.Reason, rpt.Description, rpt.Status, rpt.Severity,
			metadataJSON, rpt.CreatedAt, rpt.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("bulk create report failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple reports.
func (r *reportRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM reports WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete reports failed: %w", err)
	}
	return nil
}

// BulkUpdateStatus updates status for multiple reports.
func (r *reportRepo) BulkUpdateStatus(ctx context.Context, ids []string, status, reviewerID, notes string) error {
	if len(ids) == 0 {
		return nil
	}
	resolvedAt := time.Time{}
	if status == entities.ReportStatusResolved || status == entities.ReportStatusDismissed {
		resolvedAt = time.Now()
	}
	query, args, err := sqlx.In(`
		UPDATE reports SET
			status = ?,
			reviewer_id = ?,
			review_notes = ?,
			resolved_at = ?,
			updated_at = ?
		WHERE id IN (?)
	`, status, reviewerID, notes, resolvedAt, time.Now(), ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk update report status failed: %w", err)
	}
	return nil
}

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetReportStats returns aggregated report statistics.
func (r *reportRepo) GetReportStats(ctx context.Context) (*ReportStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_reports,
			COUNT(DISTINCT reporter_id) as unique_reporters,
			COUNT(DISTINCT target_id) as unique_targets,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved,
			SUM(CASE WHEN status = 'dismissed' THEN 1 ELSE 0 END) as dismissed,
			SUM(CASE WHEN status = 'under_review' THEN 1 ELSE 0 END) as under_review,
			SUM(CASE WHEN severity = 'low' THEN 1 ELSE 0 END) as low_severity,
			SUM(CASE WHEN severity = 'medium' THEN 1 ELSE 0 END) as medium_severity,
			SUM(CASE WHEN severity = 'high' THEN 1 ELSE 0 END) as high_severity,
			AVG(EXTRACT(EPOCH FROM (resolved_at - created_at))) as avg_resolution_time,
			MAX(created_at) as latest_report,
			MIN(created_at) as earliest_report
		FROM reports
	`
	var stats ReportStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get report stats failed: %w", err)
	}
	return &stats, nil
}

// ReportStats represents aggregated report statistics.
type ReportStats struct {
	TotalReports      int64     `db:"total_reports"`
	UniqueReporters   int64     `db:"unique_reporters"`
	UniqueTargets     int64     `db:"unique_targets"`
	Pending           int64     `db:"pending"`
	Resolved          int64     `db:"resolved"`
	Dismissed         int64     `db:"dismissed"`
	UnderReview       int64     `db:"under_review"`
	LowSeverity       int64     `db:"low_severity"`
	MediumSeverity    int64     `db:"medium_severity"`
	HighSeverity      int64     `db:"high_severity"`
	AvgResolutionTime *float64  `db:"avg_resolution_time"`
	LatestReport      time.Time `db:"latest_report"`
	EarliestReport    time.Time `db:"earliest_report"`
}

// GetDailyReportStats returns daily report counts.
func (r *reportRepo) GetDailyReportStats(ctx context.Context, start, end time.Time) ([]*DailyReportCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			COUNT(DISTINCT reporter_id) as unique_reporters,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved,
			SUM(CASE WHEN status = 'dismissed' THEN 1 ELSE 0 END) as dismissed
		FROM reports
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyReportCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily report stats failed: %w", err)
	}
	return results, nil
}

// DailyReportCount represents daily report counts.
type DailyReportCount struct {
	Date           time.Time `db:"date"`
	Total          int64     `db:"total"`
	UniqueReporters int64    `db:"unique_reporters"`
	Pending        int64     `db:"pending"`
	Resolved       int64     `db:"resolved"`
	Dismissed      int64     `db:"dismissed"`
}

// GetReportTypeStats returns report stats by target type.
func (r *reportRepo) GetReportTypeStats(ctx context.Context) ([]*ReportTypeStat, error) {
	query := `
		SELECT 
			target_type,
			COUNT(*) as count,
			COUNT(DISTINCT target_id) as unique_targets,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending
		FROM reports
		GROUP BY target_type
		ORDER BY count DESC
	`
	var stats []*ReportTypeStat
	err := r.getDB().SelectContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get report type stats failed: %w", err)
	}
	return stats, nil
}

// ReportTypeStat represents report statistics by type.
type ReportTypeStat struct {
	TargetType   string `db:"target_type"`
	Count        int64  `db:"count"`
	UniqueTargets int64 `db:"unique_targets"`
	Pending      int64  `db:"pending"`
}

// GetReasonStats returns stats by reason.
func (r *reportRepo) GetReasonStats(ctx context.Context) ([]*ReasonStat, error) {
	query := `
		SELECT 
			reason,
			COUNT(*) as count,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending
		FROM reports
		GROUP BY reason
		ORDER BY count DESC
	`
	var stats []*ReasonStat
	err := r.getDB().SelectContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get reason stats failed: %w", err)
	}
	return stats, nil
}

// ReasonStat represents report statistics by reason.
type ReasonStat struct {
	Reason  string `db:"reason"`
	Count   int64  `db:"count"`
	Pending int64  `db:"pending"`
}

// ======================================================================
= Duplicate Detection
// ======================================================================

// CheckDuplicate checks if a user has already reported the same target.
func (r *reportRepo) CheckDuplicate(ctx context.Context, reporterID, targetID, targetType string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM reports
			WHERE reporter_id = $1
			  AND target_id = $2
			  AND target_type = $3
			  AND status != 'resolved'
			  AND status != 'dismissed'
		)
	`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, reporterID, targetID, targetType)
	if err != nil {
		return false, fmt.Errorf("check duplicate report failed: %w", err)
	}
	return exists, nil
}

// GetDuplicateReports returns duplicate reports for the same target.
func (r *reportRepo) GetDuplicateReports(ctx context.Context, targetID, targetType string) ([]*entities.Report, error) {
	query := `
		SELECT * FROM reports
		WHERE target_id = $1 AND target_type = $2
		  AND status != 'resolved'
		  AND status != 'dismissed'
		ORDER BY created_at ASC
	`
	var reports []*entities.Report
	err := r.getDB().SelectContext(ctx, &reports, query, targetID, targetType)
	if err != nil {
		return nil, fmt.Errorf("get duplicate reports failed: %w", err)
	}
	return reports, nil
}

// ======================================================================
= Health
// ======================================================================

func (r *reportRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *reportRepo) Close() error {
	return nil
}

func (r *reportRepo) GetRawDB() interface{} {
	return r.db
}