// backend/internal/repository/postgres/report_pg.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return interfaces.ErrReportDuplicate
		}
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

// GetByIDs retrieves multiple reports by their IDs.
func (r *reportRepo) GetByIDs(ctx context.Context, ids []string) ([]*entities.Report, error) {
	if len(ids) == 0 {
		return []*entities.Report{}, nil
	}
	query, args, err := sqlx.In(`SELECT * FROM reports WHERE id IN (?)`, ids)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var reports []*entities.Report
	err = r.getDB().SelectContext(ctx, &reports, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get reports by IDs failed: %w", err)
	}
	return reports, nil
}

// GetByTarget retrieves reports for a specific target.
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

// GetByReporter retrieves reports filed by a user.
func (r *reportRepo) GetByReporter(ctx context.Context, reporterID string, cursor string, limit int) ([]*entities.Report, string, error) {
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

// GetByReviewer retrieves reports assigned to a reviewer.
func (r *reportRepo) GetByReviewer(ctx context.Context, reviewerID string, cursor string, limit int) ([]*entities.Report, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM reports
		WHERE reviewer_id = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{reviewerID}
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
		return nil, "", fmt.Errorf("get reports by reviewer failed: %w", err)
	}
	var nextCursor string
	if len(reports) == limit {
		nextCursor = reports[len(reports)-1].ID
	}
	return reports, nextCursor, nil
}

// Update updates a report.
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

// DeleteByTarget removes all reports for a target.
func (r *reportRepo) DeleteByTarget(ctx context.Context, targetID, targetType string) error {
	query := `DELETE FROM reports WHERE target_id = $1 AND target_type = $2`
	_, err := r.getDB().ExecContext(ctx, query, targetID, targetType)
	if err != nil {
		return fmt.Errorf("delete reports by target failed: %w", err)
	}
	return nil
}

// ======================================================================
// Status Management
// ======================================================================

// UpdateStatus updates the status of a report.
func (r *reportRepo) UpdateStatus(ctx context.Context, id, status string, reviewerID string, notes string) error {
	resolvedAt := time.Time{}
	if status == interfaces.ReportStatusResolved || status == interfaces.ReportStatusDismissed {
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
	return r.UpdateStatus(ctx, id, interfaces.ReportStatusResolved, reviewerID, notes)
}

// DismissReport marks a report as dismissed.
func (r *reportRepo) DismissReport(ctx context.Context, id, reviewerID, notes string) error {
	return r.UpdateStatus(ctx, id, interfaces.ReportStatusDismissed, reviewerID, notes)
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
		interfaces.ReportStatusPending, reviewerID, notes, time.Now(), id,
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

// AssignReviewer assigns a reviewer to a report.
func (r *reportRepo) AssignReviewer(ctx context.Context, id, reviewerID string) error {
	query := `UPDATE reports SET reviewer_id = $1, updated_at = $2 WHERE id = $3`
	result, err := r.getDB().ExecContext(ctx, query, reviewerID, time.Now(), id)
	if err != nil {
		return fmt.Errorf("assign reviewer failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrReportNotFound
	}
	return nil
}

// UnassignReviewer removes the reviewer assignment.
func (r *reportRepo) UnassignReviewer(ctx context.Context, id string) error {
	query := `UPDATE reports SET reviewer_id = NULL, updated_at = $1 WHERE id = $2`
	result, err := r.getDB().ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("unassign reviewer failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrReportNotFound
	}
	return nil
}

// UpdateSeverity updates the severity of a report.
func (r *reportRepo) UpdateSeverity(ctx context.Context, id, severity string) error {
	query := `UPDATE reports SET severity = $1, updated_at = $2 WHERE id = $3`
	result, err := r.getDB().ExecContext(ctx, query, severity, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update severity failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrReportNotFound
	}
	return nil
}

// AddReviewNote adds a review note to a report.
func (r *reportRepo) AddReviewNote(ctx context.Context, id, note string) error {
	var currentNotes string
	err := r.getDB().GetContext(ctx, &currentNotes, `SELECT review_notes FROM reports WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return interfaces.ErrReportNotFound
		}
		return fmt.Errorf("get review notes failed: %w", err)
	}
	if currentNotes != "" {
		currentNotes += "\n" + note
	} else {
		currentNotes = note
	}
	query := `UPDATE reports SET review_notes = $1, updated_at = $2 WHERE id = $3`
	_, err = r.getDB().ExecContext(ctx, query, currentNotes, time.Now(), id)
	if err != nil {
		return fmt.Errorf("add review note failed: %w", err)
	}
	return nil
}

// ======================================================================
// Existence Checks
// ======================================================================

// Exists checks if a report exists.
func (r *reportRepo) Exists(ctx context.Context, id string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM reports WHERE id = $1)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, id)
	if err != nil {
		return false, fmt.Errorf("check report existence failed: %w", err)
	}
	return exists, nil
}

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
// List Operations
// ======================================================================

// List returns reports with filtering and pagination.
func (r *reportRepo) List(ctx context.Context, filter *interfaces.ReportFilter, pagination *interfaces.ReportPagination) ([]*entities.Report, int64, error) {
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
		if filter.Reason != nil && *filter.Reason != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("reason = $%d", argIdx))
			args = append(args, *filter.Reason)
			argIdx++
		}
		if filter.ReviewerID != nil && *filter.ReviewerID != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("reviewer_id = $%d", argIdx))
			args = append(args, *filter.ReviewerID)
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
		if filter.ResolvedFrom != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("resolved_at >= $%d", argIdx))
			args = append(args, *filter.ResolvedFrom)
			argIdx++
		}
		if filter.ResolvedTo != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("resolved_at <= $%d", argIdx))
			args = append(args, *filter.ResolvedTo)
			argIdx++
		}
		if filter.AssignedTo != nil && *filter.AssignedTo != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("reviewer_id = $%d", argIdx))
			args = append(args, *filter.AssignedTo)
			argIdx++
		}
		if filter.HasReview != nil {
			if *filter.HasReview {
				whereClauses = append(whereClauses, fmt.Sprintf("review_notes IS NOT NULL AND review_notes != ''"))
			} else {
				whereClauses = append(whereClauses, fmt.Sprintf("(review_notes IS NULL OR review_notes = '')"))
			}
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
		if pagination.Cursor != "" {
			// For simplicity, use offset with cursor as marker
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
		"status": true, "resolved_at": true,
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

// GetPending returns pending reports sorted by severity.
func (r *reportRepo) GetPending(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	filter := &interfaces.ReportFilter{
		Status: stringPtr(interfaces.ReportStatusPending),
	}
	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		SortBy: interfaces.SortReportBySeverity,
		Order:  interfaces.ReportSortDesc,
	}
	if cursor != "" {
		pagination.Cursor = cursor
	}
	return r.List(ctx, filter, pagination)
}

// GetUnderReview returns reports under review.
func (r *reportRepo) GetUnderReview(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	filter := &interfaces.ReportFilter{
		Status: stringPtr(interfaces.ReportStatusUnderReview),
	}
	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		SortBy: interfaces.SortReportByCreatedAt,
		Order:  interfaces.ReportSortDesc,
	}
	if cursor != "" {
		pagination.Cursor = cursor
	}
	return r.List(ctx, filter, pagination)
}

// GetResolved returns resolved reports.
func (r *reportRepo) GetResolved(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	filter := &interfaces.ReportFilter{
		Status: stringPtr(interfaces.ReportStatusResolved),
	}
	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		SortBy: interfaces.SortReportByResolvedAt,
		Order:  interfaces.ReportSortDesc,
	}
	if cursor != "" {
		pagination.Cursor = cursor
	}
	return r.List(ctx, filter, pagination)
}

// GetDismissed returns dismissed reports.
func (r *reportRepo) GetDismissed(ctx context.Context, cursor string, limit int) ([]*entities.Report, string, error) {
	filter := &interfaces.ReportFilter{
		Status: stringPtr(interfaces.ReportStatusDismissed),
	}
	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		SortBy: interfaces.SortReportByResolvedAt,
		Order:  interfaces.ReportSortDesc,
	}
	if cursor != "" {
		pagination.Cursor = cursor
	}
	return r.List(ctx, filter, pagination)
}

// GetByDateRange returns reports within a date range.
func (r *reportRepo) GetByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Report, string, error) {
	filter := &interfaces.ReportFilter{
		CreatedFrom: &start,
		CreatedTo:   &end,
	}
	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		SortBy: interfaces.SortReportByCreatedAt,
		Order:  interfaces.ReportSortDesc,
	}
	if cursor != "" {
		pagination.Cursor = cursor
	}
	return r.List(ctx, filter, pagination)
}

// GetBySeverity returns reports by severity.
func (r *reportRepo) GetBySeverity(ctx context.Context, severity string, cursor string, limit int) ([]*entities.Report, string, error) {
	filter := &interfaces.ReportFilter{
		Severity: &severity,
	}
	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		SortBy: interfaces.SortReportByCreatedAt,
		Order:  interfaces.ReportSortDesc,
	}
	if cursor != "" {
		pagination.Cursor = cursor
	}
	return r.List(ctx, filter, pagination)
}

// GetByReason returns reports by reason.
func (r *reportRepo) GetByReason(ctx context.Context, reason string, cursor string, limit int) ([]*entities.Report, string, error) {
	filter := &interfaces.ReportFilter{
		Reason: &reason,
	}
	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		SortBy: interfaces.SortReportByCreatedAt,
		Order:  interfaces.ReportSortDesc,
	}
	if cursor != "" {
		pagination.Cursor = cursor
	}
	return r.List(ctx, filter, pagination)
}

// GetByTargetType returns reports by target type.
func (r *reportRepo) GetByTargetType(ctx context.Context, targetType string, cursor string, limit int) ([]*entities.Report, string, error) {
	filter := &interfaces.ReportFilter{
		TargetType: &targetType,
	}
	pagination := &interfaces.ReportPagination{
		Limit:  limit,
		SortBy: interfaces.SortReportByCreatedAt,
		Order:  interfaces.ReportSortDesc,
	}
	if cursor != "" {
		pagination.Cursor = cursor
	}
	return r.List(ctx, filter, pagination)
}

// ======================================================================
// Count Operations
// ======================================================================

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

// CountByStatus returns number of reports by status.
func (r *reportRepo) CountByStatus(ctx context.Context, status string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE status = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, status)
	if err != nil {
		return 0, fmt.Errorf("count reports by status failed: %w", err)
	}
	return count, nil
}

// CountBySeverity returns number of reports by severity.
func (r *reportRepo) CountBySeverity(ctx context.Context, severity string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE severity = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, severity)
	if err != nil {
		return 0, fmt.Errorf("count reports by severity failed: %w", err)
	}
	return count, nil
}

// CountByTarget returns number of reports for a target.
func (r *reportRepo) CountByTarget(ctx context.Context, targetID, targetType string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE target_id = $1 AND target_type = $2`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, targetID, targetType)
	if err != nil {
		return 0, fmt.Errorf("count reports by target failed: %w", err)
	}
	return count, nil
}

// CountByReporter returns number of reports filed by a user.
func (r *reportRepo) CountByReporter(ctx context.Context, reporterID string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE reporter_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, reporterID)
	if err != nil {
		return 0, fmt.Errorf("count reports by reporter failed: %w", err)
	}
	return count, nil
}

// CountByReviewer returns number of reports assigned to a reviewer.
func (r *reportRepo) CountByReviewer(ctx context.Context, reviewerID string) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE reviewer_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, reviewerID)
	if err != nil {
		return 0, fmt.Errorf("count reports by reviewer failed: %w", err)
	}
	return count, nil
}

// CountByDateRange returns report count within a date range.
func (r *reportRepo) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM reports WHERE created_at >= $1 AND created_at <= $2`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, start, end)
	if err != nil {
		return 0, fmt.Errorf("count reports by date range failed: %w", err)
	}
	return count, nil
}

// CountPendingBySeverity returns pending reports count grouped by severity.
func (r *reportRepo) CountPendingBySeverity(ctx context.Context) (map[string]int64, error) {
	query := `
		SELECT severity, COUNT(*) as count
		FROM reports
		WHERE status = 'pending'
		GROUP BY severity
	`
	var results []struct {
		Severity string `db:"severity"`
		Count    int64  `db:"count"`
	}
	err := r.getDB().SelectContext(ctx, &results, query)
	if err != nil {
		return nil, fmt.Errorf("count pending by severity failed: %w", err)
	}
	counts := make(map[string]int64)
	for _, r := range results {
		counts[r.Severity] = r.Count
	}
	return counts, nil
}

// ======================================================================
// Bulk Operations
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
	if status == interfaces.ReportStatusResolved || status == interfaces.ReportStatusDismissed {
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

// BulkAssignReviewer assigns a reviewer to multiple reports.
func (r *reportRepo) BulkAssignReviewer(ctx context.Context, ids []string, reviewerID string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`
		UPDATE reports SET reviewer_id = ?, updated_at = ?
		WHERE id IN (?)
	`, reviewerID, time.Now(), ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk assign reviewer failed: %w", err)
	}
	return nil
}

// BulkResolve resolves multiple reports.
func (r *reportRepo) BulkResolve(ctx context.Context, ids []string, reviewerID, notes string) error {
	return r.BulkUpdateStatus(ctx, ids, interfaces.ReportStatusResolved, reviewerID, notes)
}

// BulkDismiss dismisses multiple reports.
func (r *reportRepo) BulkDismiss(ctx context.Context, ids []string, reviewerID, notes string) error {
	return r.BulkUpdateStatus(ctx, ids, interfaces.ReportStatusDismissed, reviewerID, notes)
}

// BulkDeleteByTarget removes reports for multiple targets.
func (r *reportRepo) BulkDeleteByTarget(ctx context.Context, pairs []interfaces.TargetPair) error {
	if len(pairs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM reports WHERE target_id = $1 AND target_type = $2`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, pair := range pairs {
		_, err := stmt.ExecContext(ctx, pair.TargetID, pair.TargetType)
		if err != nil {
			return fmt.Errorf("bulk delete by target failed: %w", err)
		}
	}
	return tx.Commit()
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetReportStats returns aggregated report statistics.
func (r *reportRepo) GetReportStats(ctx context.Context) (*interfaces.ReportStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_reports,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_reports,
			SUM(CASE WHEN status = 'under_review' THEN 1 ELSE 0 END) as under_review_reports,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved_reports,
			SUM(CASE WHEN status = 'dismissed' THEN 1 ELSE 0 END) as dismissed_reports,
			COUNT(DISTINCT reporter_id) as unique_reporters,
			COUNT(DISTINCT target_id) as unique_targets,
			AVG(EXTRACT(EPOCH FROM (resolved_at - created_at))) as avg_resolution_time,
			MAX(created_at) as last_report_created,
			MAX(resolved_at) as last_report_resolved
		FROM reports
	`
	var stats interfaces.ReportStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get report stats failed: %w", err)
	}

	// Get severity stats
	query2 := `
		SELECT severity, COUNT(*) as count
		FROM reports
		GROUP BY severity
	`
	var sevResults []struct {
		Severity string `db:"severity"`
		Count    int64  `db:"count"`
	}
	err = r.getDB().SelectContext(ctx, &sevResults, query2)
	if err != nil {
		return nil, fmt.Errorf("get severity stats failed: %w", err)
	}
	stats.SeverityStats = make(map[string]int64)
	for _, r := range sevResults {
		stats.SeverityStats[r.Severity] = r.Count
	}

	// Get reason stats
	query3 := `
		SELECT reason, COUNT(*) as count
		FROM reports
		GROUP BY reason
	`
	var reasonResults []struct {
		Reason string `db:"reason"`
		Count  int64  `db:"count"`
	}
	err = r.getDB().SelectContext(ctx, &reasonResults, query3)
	if err != nil {
		return nil, fmt.Errorf("get reason stats failed: %w", err)
	}
	stats.ReasonStats = make(map[string]int64)
	for _, r := range reasonResults {
		stats.ReasonStats[r.Reason] = r.Count
	}

	// Get target type stats
	query4 := `
		SELECT target_type, COUNT(*) as count
		FROM reports
		GROUP BY target_type
	`
	var typeResults []struct {
		TargetType string `db:"target_type"`
		Count      int64  `db:"count"`
	}
	err = r.getDB().SelectContext(ctx, &typeResults, query4)
	if err != nil {
		return nil, fmt.Errorf("get target type stats failed: %w", err)
	}
	stats.TargetTypeStats = make(map[string]int64)
	for _, r := range typeResults {
		stats.TargetTypeStats[r.TargetType] = r.Count
	}

	return &stats, nil
}

// GetUserReportStats returns report statistics for a specific user (as reporter).
func (r *reportRepo) GetUserReportStats(ctx context.Context, userID string) (*interfaces.ReportStats, error) {
	stats, err := r.GetReportStats(ctx)
	if err != nil {
		return nil, err
	}
	// Filter by user
	query := `
		SELECT 
			COUNT(*) as total_reports,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_reports,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved_reports,
			SUM(CASE WHEN status = 'dismissed' THEN 1 ELSE 0 END) as dismissed_reports
		FROM reports
		WHERE reporter_id = $1
	`
	var userStats interfaces.ReportStats
	err = r.getDB().GetContext(ctx, &userStats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user report stats failed: %w", err)
	}
	stats.TotalReports = userStats.TotalReports
	stats.PendingReports = userStats.PendingReports
	stats.ResolvedReports = userStats.ResolvedReports
	stats.DismissedReports = userStats.DismissedReports
	return stats, nil
}

// GetReviewerStats returns report statistics for a reviewer.
func (r *reportRepo) GetReviewerStats(ctx context.Context, reviewerID string) (*interfaces.ReportStats, error) {
	stats, err := r.GetReportStats(ctx)
	if err != nil {
		return nil, err
	}
	// Filter by reviewer
	query := `
		SELECT 
			COUNT(*) as total_reports,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved_reports,
			SUM(CASE WHEN status = 'dismissed' THEN 1 ELSE 0 END) as dismissed_reports,
			AVG(EXTRACT(EPOCH FROM (resolved_at - created_at))) as avg_resolution_time
		FROM reports
		WHERE reviewer_id = $1
	`
	var reviewerStats interfaces.ReportStats
	err = r.getDB().GetContext(ctx, &reviewerStats, query, reviewerID)
	if err != nil {
		return nil, fmt.Errorf("get reviewer stats failed: %w", err)
	}
	stats.TotalReports = reviewerStats.TotalReports
	stats.ResolvedReports = reviewerStats.ResolvedReports
	stats.DismissedReports = reviewerStats.DismissedReports
	stats.AvgResolutionTime = reviewerStats.AvgResolutionTime
	return stats, nil
}

// GetDailyReportStats returns daily report counts for a date range.
func (r *reportRepo) GetDailyReportStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailyReportCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'under_review' THEN 1 ELSE 0 END) as under_review,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved,
			SUM(CASE WHEN status = 'dismissed' THEN 1 ELSE 0 END) as dismissed,
			COUNT(DISTINCT reporter_id) as unique_reporters
		FROM reports
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyReportCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily report stats failed: %w", err)
	}
	return results, nil
}

// GetReportTypeStats returns report stats by target type.
func (r *reportRepo) GetReportTypeStats(ctx context.Context) ([]*interfaces.ReportTypeStat, error) {
	query := `
		SELECT 
			target_type,
			COUNT(*) as count,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved,
			SUM(CASE WHEN status = 'dismissed' THEN 1 ELSE 0 END) as dismissed
		FROM reports
		GROUP BY target_type
		ORDER BY count DESC
	`
	var stats []*interfaces.ReportTypeStat
	err := r.getDB().SelectContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get report type stats failed: %w", err)
	}
	return stats, nil
}

// GetReasonStats returns report stats by reason.
func (r *reportRepo) GetReasonStats(ctx context.Context) ([]*interfaces.ReasonStat, error) {
	query := `
		SELECT 
			reason,
			COUNT(*) as count,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved
		FROM reports
		GROUP BY reason
		ORDER BY count DESC
	`
	var stats []*interfaces.ReasonStat
	err := r.getDB().SelectContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get reason stats failed: %w", err)
	}
	return stats, nil
}

// GetSeverityDistribution returns severity distribution.
func (r *reportRepo) GetSeverityDistribution(ctx context.Context) (map[string]float64, error) {
	stats, err := r.GetReportStats(ctx)
	if err != nil {
		return nil, err
	}
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

// GetReportVelocity calculates report velocity (reports per day).
func (r *reportRepo) GetReportVelocity(ctx context.Context, days int) (float64, error) {
	since := time.Now().AddDate(0, 0, -days)
	count, err := r.CountByDateRange(ctx, since, time.Now())
	if err != nil {
		return 0, err
	}
	return float64(count) / float64(days), nil
}

// GetAverageResolutionTimeBySeverity returns avg resolution time by severity.
func (r *reportRepo) GetAverageResolutionTimeBySeverity(ctx context.Context) (map[string]float64, error) {
	query := `
		SELECT 
			severity,
			AVG(EXTRACT(EPOCH FROM (resolved_at - created_at))) as avg_time
		FROM reports
		WHERE status IN ('resolved', 'dismissed')
		  AND resolved_at IS NOT NULL
		GROUP BY severity
	`
	var results []struct {
		Severity string  `db:"severity"`
		AvgTime  float64 `db:"avg_time"`
	}
	err := r.getDB().SelectContext(ctx, &results, query)
	if err != nil {
		return nil, fmt.Errorf("get avg resolution time by severity failed: %w", err)
	}
	avgTimes := make(map[string]float64)
	for _, r := range results {
		avgTimes[r.Severity] = r.AvgTime
	}
	return avgTimes, nil
}

// GetMostReportedTargets returns the most reported targets.
func (r *reportRepo) GetMostReportedTargets(ctx context.Context, limit int, since time.Time) ([]*interfaces.ReportedTarget, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT 
			target_id,
			target_type,
			COUNT(*) as report_count,
			MAX(created_at) as last_reported
		FROM reports
		WHERE created_at >= $1
		GROUP BY target_id, target_type
		ORDER BY report_count DESC
		LIMIT $2
	`
	var targets []*interfaces.ReportedTarget
	err := r.getDB().SelectContext(ctx, &targets, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most reported targets failed: %w", err)
	}
	return targets, nil
}

// GetMostActiveReporters returns the most active reporters.
func (r *reportRepo) GetMostActiveReporters(ctx context.Context, limit int, since time.Time) ([]*interfaces.ActiveReporter, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT 
			reporter_id as user_id,
			u.username,
			COUNT(*) as report_count,
			MAX(r.created_at) as last_reported
		FROM reports r
		JOIN users u ON r.reporter_id = u.id
		WHERE r.created_at >= $1
		GROUP BY reporter_id, u.username
		ORDER BY report_count DESC
		LIMIT $2
	`
	var reporters []*interfaces.ActiveReporter
	err := r.getDB().SelectContext(ctx, &reporters, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get most active reporters failed: %w", err)
	}
	return reporters, nil
}

// ======================================================================
// Moderation Actions
// ======================================================================

// GetActionableReports returns reports that require action.
func (r *reportRepo) GetActionableReports(ctx context.Context, limit int) ([]*entities.Report, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM reports
		WHERE status IN ('pending', 'under_review')
		ORDER BY 
			CASE severity
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				WHEN 'low' THEN 4
			END ASC,
			created_at ASC
		LIMIT $1
	`
	var reports []*entities.Report
	err := r.getDB().SelectContext(ctx, &reports, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get actionable reports failed: %w", err)
	}
	return reports, nil
}

// GetReportsForModeration returns reports for a moderator to review.
func (r *reportRepo) GetReportsForModeration(ctx context.Context, moderatorID string, limit int) ([]*entities.Report, error) {
	// If moderatorID is empty, get all actionable reports
	if moderatorID == "" {
		return r.GetActionableReports(ctx, limit)
	}
	// Get reports assigned to this moderator
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM reports
		WHERE (status = 'pending' OR status = 'under_review')
		  AND (reviewer_id = $1 OR reviewer_id IS NULL)
		ORDER BY 
			CASE severity
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				WHEN 'low' THEN 4
			END ASC,
			created_at ASC
		LIMIT $2
	`
	var reports []*entities.Report
	err := r.getDB().SelectContext(ctx, &reports, query, moderatorID, limit)
	if err != nil {
		return nil, fmt.Errorf("get reports for moderation failed: %w", err)
	}
	return reports, nil
}

// RecordModerationAction records a moderation action taken based on a report.
func (r *reportRepo) RecordModerationAction(ctx context.Context, reportID, action, performedBy string, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal details failed: %w", err)
	}
	query := `
		INSERT INTO moderation_actions (id, report_id, action, performed_by, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.getDB().ExecContext(ctx, query,
		uuid.New().String(), reportID, action, performedBy, detailsJSON, time.Now())
	if err != nil {
		return fmt.Errorf("record moderation action failed: %w", err)
	}
	return nil
}

// GetModerationHistory returns moderation history for a report.
func (r *reportRepo) GetModerationHistory(ctx context.Context, reportID string) ([]*interfaces.ModerationAction, error) {
	query := `
		SELECT id, report_id, action, performed_by, details, created_at
		FROM moderation_actions
		WHERE report_id = $1
		ORDER BY created_at DESC
	`
	var actions []*interfaces.ModerationAction
	err := r.getDB().SelectContext(ctx, &actions, query, reportID)
	if err != nil {
		return nil, fmt.Errorf("get moderation history failed: %w", err)
	}
	return actions, nil
}

// ======================================================================
// Health
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

// ======================================================================
// Helper Functions
// ======================================================================

func stringPtr(s string) *string {
	return &s
}