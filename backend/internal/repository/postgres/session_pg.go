// backend/internal/repository/postgres/session_pg.go
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

// sessionRepo is the PostgreSQL implementation of SessionRepository.
type sessionRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewSessionRepository creates a new PostgreSQL session repository.
func NewSessionRepository(db *sqlx.DB) interfaces.SessionRepository {
	return &sessionRepo{
		db:  db,
		log: logger.WithField("repository", "session_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *sessionRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.SessionRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &sessionRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *sessionRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.SessionRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &sessionRepo{
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
func (r *sessionRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic CRUD
// ======================================================================

// Create inserts a new session.
func (r *sessionRepo) Create(ctx context.Context, session *entities.Session) error {
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	query := `
		INSERT INTO sessions (
			id, user_id, refresh_token, user_agent, ip,
			expires_at, created_at, updated_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = r.getDB().ExecContext(ctx, query,
		session.ID, session.UserID, session.RefreshToken,
		session.UserAgent, session.IP, session.ExpiresAt,
		session.CreatedAt, session.UpdatedAt, metadataJSON,
	)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			if pgErr.Constraint == "sessions_refresh_token_key" {
				return interfaces.ErrDuplicateRefreshToken
			}
		}
		return fmt.Errorf("create session failed: %w", err)
	}
	return nil
}

// GetByID retrieves a session by its ID.
func (r *sessionRepo) GetByID(ctx context.Context, id string) (*entities.Session, error) {
	query := `SELECT * FROM sessions WHERE id = $1`
	var session entities.Session
	err := r.getDB().GetContext(ctx, &session, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrSessionNotFound
		}
		return nil, fmt.Errorf("get session by ID failed: %w", err)
	}
	return &session, nil
}

// GetByRefreshToken retrieves a session by its refresh token.
func (r *sessionRepo) GetByRefreshToken(ctx context.Context, refreshToken string) (*entities.Session, error) {
	query := `SELECT * FROM sessions WHERE refresh_token = $1`
	var session entities.Session
	err := r.getDB().GetContext(ctx, &session, query, refreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrSessionNotFound
		}
		return nil, fmt.Errorf("get session by refresh token failed: %w", err)
	}
	return &session, nil
}

// GetByUserID retrieves all sessions for a user.
func (r *sessionRepo) GetByUserID(ctx context.Context, userID string) ([]*entities.Session, error) {
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get sessions by user failed: %w", err)
	}
	return sessions, nil
}

// Update updates a session (e.g., refresh token rotation).
func (r *sessionRepo) Update(ctx context.Context, session *entities.Session) error {
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	query := `
		UPDATE sessions SET
			refresh_token = $1,
			user_agent = $2,
			ip = $3,
			expires_at = $4,
			updated_at = $5,
			metadata = $6
		WHERE id = $7
	`
	result, err := r.getDB().ExecContext(ctx, query,
		session.RefreshToken, session.UserAgent, session.IP,
		session.ExpiresAt, time.Now(), metadataJSON, session.ID,
	)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			if pgErr.Constraint == "sessions_refresh_token_key" {
				return interfaces.ErrDuplicateRefreshToken
			}
		}
		return fmt.Errorf("update session failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrSessionNotFound
	}
	return nil
}

// UpdateRefreshToken updates the refresh token for a session.
func (r *sessionRepo) UpdateRefreshToken(ctx context.Context, id, newRefreshToken string, newExpiry time.Time) error {
	query := `
		UPDATE sessions SET
			refresh_token = $1,
			expires_at = $2,
			updated_at = $3
		WHERE id = $4
	`
	result, err := r.getDB().ExecContext(ctx, query, newRefreshToken, newExpiry, time.Now(), id)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			if pgErr.Constraint == "sessions_refresh_token_key" {
				return interfaces.ErrDuplicateRefreshToken
			}
		}
		return fmt.Errorf("update refresh token failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrSessionNotFound
	}
	return nil
}

// Delete removes a session.
func (r *sessionRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM sessions WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete session failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrSessionNotFound
	}
	return nil
}

// DeleteByRefreshToken removes a session by its refresh token.
func (r *sessionRepo) DeleteByRefreshToken(ctx context.Context, refreshToken string) error {
	query := `DELETE FROM sessions WHERE refresh_token = $1`
	result, err := r.getDB().ExecContext(ctx, query, refreshToken)
	if err != nil {
		return fmt.Errorf("delete session by refresh token failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrSessionNotFound
	}
	return nil
}

// DeleteByUserID removes all sessions for a user.
func (r *sessionRepo) DeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM sessions WHERE user_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete sessions by user failed: %w", err)
	}
	return nil
}

// ======================================================================
// Existence Checks
// ======================================================================

// Exists checks if a session exists.
func (r *sessionRepo) Exists(ctx context.Context, id string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, id)
	if err != nil {
		return false, fmt.Errorf("check session existence failed: %w", err)
	}
	return exists, nil
}

// ExistsByRefreshToken checks if a refresh token exists.
func (r *sessionRepo) ExistsByRefreshToken(ctx context.Context, refreshToken string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM sessions WHERE refresh_token = $1)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, refreshToken)
	if err != nil {
		return false, fmt.Errorf("check refresh token existence failed: %w", err)
	}
	return exists, nil
}

// IsValidSession checks if a session is valid (exists and not expired).
func (r *sessionRepo) IsValidSession(ctx context.Context, id string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM sessions
			WHERE id = $1 AND expires_at > NOW()
		)
	`
	var valid bool
	err := r.getDB().GetContext(ctx, &valid, query, id)
	if err != nil {
		return false, fmt.Errorf("check session validity failed: %w", err)
	}
	return valid, nil
}

// IsValidRefreshToken checks if a refresh token is valid.
func (r *sessionRepo) IsValidRefreshToken(ctx context.Context, refreshToken string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM sessions
			WHERE refresh_token = $1 AND expires_at > NOW()
		)
	`
	var valid bool
	err := r.getDB().GetContext(ctx, &valid, query, refreshToken)
	if err != nil {
		return false, fmt.Errorf("check refresh token validity failed: %w", err)
	}
	return valid, nil
}

// ======================================================================
// Active/Expired Sessions
// ======================================================================

// GetActiveSessions returns active sessions for a user.
func (r *sessionRepo) GetActiveSessions(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1 AND expires_at > NOW()
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get active sessions failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetExpiredSessions returns expired sessions for a user.
func (r *sessionRepo) GetExpiredSessions(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1 AND expires_at <= NOW()
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get expired sessions failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetActiveSessionsAll returns all active sessions globally (for admin).
func (r *sessionRepo) GetActiveSessionsAll(ctx context.Context, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE expires_at > NOW()
	`
	if cursor != "" {
		query += ` AND id > $1`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{}
	argIdx := 1
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 2
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get active sessions all failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetExpiredSessionsAll returns all expired sessions globally.
func (r *sessionRepo) GetExpiredSessionsAll(ctx context.Context, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE expires_at <= NOW()
	`
	if cursor != "" {
		query += ` AND id > $1`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{}
	argIdx := 1
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 2
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get expired sessions all failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetSessionsByIP returns sessions from a specific IP.
func (r *sessionRepo) GetSessionsByIP(ctx context.Context, ip string, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE ip = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{ip}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get sessions by IP failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetSessionsByUserAgent returns sessions with a specific user agent.
func (r *sessionRepo) GetSessionsByUserAgent(ctx context.Context, userAgent string, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE user_agent = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userAgent}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get sessions by user agent failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountByUserID returns total sessions for a user.
func (r *sessionRepo) CountByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE user_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count sessions by user failed: %w", err)
	}
	return count, nil
}

// CountActiveByUserID returns active sessions count for a user.
func (r *sessionRepo) CountActiveByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND expires_at > NOW()`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count active sessions by user failed: %w", err)
	}
	return count, nil
}

// CountExpiredByUserID returns expired sessions count for a user.
func (r *sessionRepo) CountExpiredByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND expires_at <= NOW()`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count expired sessions by user failed: %w", err)
	}
	return count, nil
}

// CountTotal returns total sessions in the system.
func (r *sessionRepo) CountTotal(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count total sessions failed: %w", err)
	}
	return count, nil
}

// CountActive returns total active sessions in the system.
func (r *sessionRepo) CountActive(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE expires_at > NOW()`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count active sessions failed: %w", err)
	}
	return count, nil
}

// CountExpired returns total expired sessions in the system.
func (r *sessionRepo) CountExpired(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE expires_at <= NOW()`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count expired sessions failed: %w", err)
	}
	return count, nil
}

// CountByDateRange returns session count within a date range.
func (r *sessionRepo) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE created_at >= $1 AND created_at <= $2`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, start, end)
	if err != nil {
		return 0, fmt.Errorf("count sessions by date range failed: %w", err)
	}
	return count, nil
}

// CountByDateRangeForUser returns session count for a user within a date range.
func (r *sessionRepo) CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, start, end)
	if err != nil {
		return 0, fmt.Errorf("count sessions by date range for user failed: %w", err)
	}
	return count, nil
}

// CountUniqueUsersInDateRange returns unique users with sessions in a date range.
func (r *sessionRepo) CountUniqueUsersInDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= $1 AND created_at <= $2`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, start, end)
	if err != nil {
		return 0, fmt.Errorf("count unique users in date range failed: %w", err)
	}
	return count, nil
}

// ======================================================================
// Expiry Management
// ======================================================================

// ExtendExpiry extends the expiry of a session.
func (r *sessionRepo) ExtendExpiry(ctx context.Context, id string, newExpiry time.Time) error {
	query := `UPDATE sessions SET expires_at = $1, updated_at = $2 WHERE id = $3`
	result, err := r.getDB().ExecContext(ctx, query, newExpiry, time.Now(), id)
	if err != nil {
		return fmt.Errorf("extend expiry failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrSessionNotFound
	}
	return nil
}

// Revoke revokes a session (sets it as inactive/expired).
func (r *sessionRepo) Revoke(ctx context.Context, id string) error {
	query := `UPDATE sessions SET expires_at = $1, updated_at = $2 WHERE id = $3`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), time.Now(), id)
	if err != nil {
		return fmt.Errorf("revoke session failed: %w", err)
	}
	return nil
}

// RevokeAll revokes all sessions for a user.
func (r *sessionRepo) RevokeAll(ctx context.Context, userID string) error {
	query := `UPDATE sessions SET expires_at = $1, updated_at = $2 WHERE user_id = $3 AND expires_at > NOW()`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), time.Now(), userID)
	if err != nil {
		return fmt.Errorf("revoke all sessions failed: %w", err)
	}
	return nil
}

// RevokeAllExcept revokes all sessions for a user except the given one.
func (r *sessionRepo) RevokeAllExcept(ctx context.Context, userID, excludeSessionID string) error {
	query := `
		UPDATE sessions SET expires_at = $1, updated_at = $2
		WHERE user_id = $3 AND id != $4 AND expires_at > NOW()
	`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), time.Now(), userID, excludeSessionID)
	if err != nil {
		return fmt.Errorf("revoke all except failed: %w", err)
	}
	return nil
}

// RevokeByRefreshToken revokes a session by its refresh token.
func (r *sessionRepo) RevokeByRefreshToken(ctx context.Context, refreshToken string) error {
	query := `UPDATE sessions SET expires_at = $1, updated_at = $2 WHERE refresh_token = $3`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), time.Now(), refreshToken)
	if err != nil {
		return fmt.Errorf("revoke by refresh token failed: %w", err)
	}
	return nil
}

// ======================================================================
// Cleanup Operations
// ======================================================================

// CleanupExpired removes all expired sessions.
func (r *sessionRepo) CleanupExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM sessions WHERE expires_at <= NOW()`
	result, err := r.getDB().ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired sessions failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// CleanupExpiredForUser removes expired sessions for a specific user.
func (r *sessionRepo) CleanupExpiredForUser(ctx context.Context, userID string) (int64, error) {
	query := `DELETE FROM sessions WHERE user_id = $1 AND expires_at <= NOW()`
	result, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired sessions for user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// CleanupOlderThan removes sessions older than a date.
func (r *sessionRepo) CleanupOlderThan(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM sessions WHERE created_at < $1`
	result, err := r.getDB().ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("cleanup older than failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// CleanupOlderThanForUser removes sessions older than a date for a user.
func (r *sessionRepo) CleanupOlderThanForUser(ctx context.Context, userID string, before time.Time) (int64, error) {
	query := `DELETE FROM sessions WHERE user_id = $1 AND created_at < $2`
	result, err := r.getDB().ExecContext(ctx, query, userID, before)
	if err != nil {
		return 0, fmt.Errorf("cleanup older than for user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// ======================================================================
// Advanced Queries
// ======================================================================

// GetLatestSession returns the most recent session for a user.
func (r *sessionRepo) GetLatestSession(ctx context.Context, userID string) (*entities.Session, error) {
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var session entities.Session
	err := r.getDB().GetContext(ctx, &session, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrSessionNotFound
		}
		return nil, fmt.Errorf("get latest session failed: %w", err)
	}
	return &session, nil
}

// GetOldestSession returns the oldest session for a user.
func (r *sessionRepo) GetOldestSession(ctx context.Context, userID string) (*entities.Session, error) {
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`
	var session entities.Session
	err := r.getDB().GetContext(ctx, &session, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrSessionNotFound
		}
		return nil, fmt.Errorf("get oldest session failed: %w", err)
	}
	return &session, nil
}

// GetSessionsByDateRange returns sessions within a date range.
func (r *sessionRepo) GetSessionsByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE created_at >= $1 AND created_at <= $2
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{start, end}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get sessions by date range failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetSessionsForUserByDateRange returns sessions for a user within a date range.
func (r *sessionRepo) GetSessionsForUserByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
	`
	if cursor != "" {
		query += ` AND id > $4`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, start, end}
	argIdx := 4
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 5
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get sessions for user by date range failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetSessionsByUserAndIP returns sessions for a user from a specific IP.
func (r *sessionRepo) GetSessionsByUserAndIP(ctx context.Context, userID, ip string, cursor string, limit int) ([]*entities.Session, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1 AND ip = $2
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, ip}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get sessions by user and IP failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetSessionsWithMetadata returns sessions with specific metadata.
func (r *sessionRepo) GetSessionsWithMetadata(ctx context.Context, key, value string, cursor string, limit int) ([]*entities.Session, string, error) {
	// This is a simplified version; in real scenario, we'd use JSONB queries
	// For PostgreSQL, we'd use `metadata->>$1 = $2`
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM sessions
		WHERE metadata->>$1 = $2
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{key, value}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get sessions with metadata failed: %w", err)
	}
	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// ======================================================================
// Bulk Operations
// ======================================================================

// BulkCreate inserts multiple sessions in a single transaction.
func (r *sessionRepo) BulkCreate(ctx context.Context, sessions []*entities.Session) error {
	if len(sessions) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO sessions (
			id, user_id, refresh_token, user_agent, ip,
			expires_at, created_at, updated_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range sessions {
		metadataJSON, err := json.Marshal(s.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata failed: %w", err)
		}
		_, err = stmt.ExecContext(ctx,
			s.ID, s.UserID, s.RefreshToken, s.UserAgent, s.IP,
			s.ExpiresAt, s.CreatedAt, s.UpdatedAt, metadataJSON,
		)
		if err != nil {
			return fmt.Errorf("bulk create session failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple sessions in a single transaction.
func (r *sessionRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM sessions WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete sessions failed: %w", err)
	}
	return nil
}

// BulkDeleteByUserIDs removes all sessions for multiple users.
func (r *sessionRepo) BulkDeleteByUserIDs(ctx context.Context, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM sessions WHERE user_id IN (?)`, userIDs)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete by user IDs failed: %w", err)
	}
	return nil
}

// BulkRevoke revokes multiple sessions.
func (r *sessionRepo) BulkRevoke(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE sessions SET expires_at = $1, updated_at = $2 WHERE id IN (?)`, time.Now(), time.Now(), ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk revoke sessions failed: %w", err)
	}
	return nil
}

// BulkExtendExpiry extends expiry for multiple sessions.
func (r *sessionRepo) BulkExtendExpiry(ctx context.Context, ids []string, newExpiry time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE sessions SET expires_at = $1, updated_at = $2 WHERE id IN (?)`, newExpiry, time.Now(), ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk extend expiry failed: %w", err)
	}
	return nil
}

// BulkDeleteExpired removes all expired sessions in bulk.
func (r *sessionRepo) BulkDeleteExpired(ctx context.Context) (int64, error) {
	return r.CleanupExpired(ctx)
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetSessionStats returns aggregated session statistics.
func (r *sessionRepo) GetSessionStats(ctx context.Context) (*interfaces.SessionStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_sessions,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active_sessions,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired_sessions,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT ip) as unique_ips,
			COUNT(DISTINCT user_agent) as unique_user_agents,
			AVG(EXTRACT(EPOCH FROM (expires_at - created_at))) as avg_session_duration,
			MAX(expires_at - created_at) as max_session_duration,
			MIN(expires_at - created_at) as min_session_duration,
			MAX(created_at) as last_session_created,
			MAX(expires_at) as last_session_expired
		FROM sessions
	`
	var stats interfaces.SessionStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get session stats failed: %w", err)
	}
	// Handle NULL values
	if stats.AvgSessionDuration == 0 {
		stats.AvgSessionDuration = 0
	}
	return &stats, nil
}

// GetUserSessionStats returns session statistics for a specific user.
func (r *sessionRepo) GetUserSessionStats(ctx context.Context, userID string) (*interfaces.SessionStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_sessions,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active_sessions,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired_sessions,
			1 as unique_users,
			COUNT(DISTINCT ip) as unique_ips,
			COUNT(DISTINCT user_agent) as unique_user_agents,
			AVG(EXTRACT(EPOCH FROM (expires_at - created_at))) as avg_session_duration,
			MAX(expires_at - created_at) as max_session_duration,
			MIN(expires_at - created_at) as min_session_duration,
			MAX(created_at) as last_session_created,
			MAX(expires_at) as last_session_expired
		FROM sessions
		WHERE user_id = $1
	`
	var stats interfaces.SessionStats
	err := r.getDB().GetContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user session stats failed: %w", err)
	}
	return &stats, nil
}

// GetDailySessionStats returns daily session counts for a date range.
func (r *sessionRepo) GetDailySessionStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailySessionCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT ip) as unique_ips
		FROM sessions
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailySessionCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily session stats failed: %w", err)
	}
	return results, nil
}

// GetDailySessionStatsForUser returns daily session counts for a user.
func (r *sessionRepo) GetDailySessionStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*interfaces.DailySessionCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired,
			1 as unique_users,
			COUNT(DISTINCT ip) as unique_ips
		FROM sessions
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailySessionCount
	err := r.getDB().SelectContext(ctx, &results, query, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily session stats for user failed: %w", err)
	}
	return results, nil
}

// GetDeviceStats returns session statistics by user agent.
func (r *sessionRepo) GetDeviceStats(ctx context.Context, userID string) ([]*interfaces.DeviceStat, error) {
	query := `
		SELECT 
			user_agent,
			COUNT(*) as count,
			MAX(created_at) as last_used
		FROM sessions
		WHERE user_id = $1
		GROUP BY user_agent
		ORDER BY count DESC
	`
	var stats []*interfaces.DeviceStat
	err := r.getDB().SelectContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get device stats failed: %w", err)
	}
	return stats, nil
}

// GetLocationStats returns session statistics by IP (geo-location based).
func (r *sessionRepo) GetLocationStats(ctx context.Context, userID string) ([]*interfaces.LocationStat, error) {
	query := `
		SELECT 
			ip,
			COUNT(*) as count,
			MAX(created_at) as last_used
		FROM sessions
		WHERE user_id = $1
		GROUP BY ip
		ORDER BY count DESC
	`
	var stats []*interfaces.LocationStat
	err := r.getDB().SelectContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get location stats failed: %w", err)
	}
	// In production, we would resolve IP to geo-location, but for now just IP
	return stats, nil
}

// GetAverageSessionDuration calculates average session duration for a user.
func (r *sessionRepo) GetAverageSessionDuration(ctx context.Context, userID string) (float64, error) {
	query := `
		SELECT AVG(EXTRACT(EPOCH FROM (expires_at - created_at)))
		FROM sessions
		WHERE user_id = $1
	`
	var avg float64
	err := r.getDB().GetContext(ctx, &avg, query, userID)
	if err != nil {
		return 0, fmt.Errorf("get average session duration failed: %w", err)
	}
	return avg, nil
}

// GetSessionRetentionRate calculates session retention rate.
func (r *sessionRepo) GetSessionRetentionRate(ctx context.Context, days int) (float64, error) {
	startDate := time.Now().AddDate(0, 0, -days)
	// Count users who had at least one session in the period
	var activeUsers int64
	err := r.getDB().GetContext(ctx, &activeUsers,
		`SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= $1`, startDate)
	if err != nil {
		return 0, fmt.Errorf("get active users failed: %w", err)
	}
	// Count users who had at least one session in the period and another after that
	var retainedUsers int64
	err = r.getDB().GetContext(ctx, &retainedUsers,
		`SELECT COUNT(DISTINCT user_id)
		 FROM sessions s1
		 WHERE s1.created_at >= $1
		 AND EXISTS (
		   SELECT 1 FROM sessions s2
		   WHERE s2.user_id = s1.user_id
		   AND s2.created_at > s1.created_at
		   AND s2.created_at <= NOW()
		 )`, startDate)
	if err != nil {
		return 0, fmt.Errorf("get retained users failed: %w", err)
	}
	if activeUsers == 0 {
		return 0, nil
	}
	return float64(retainedUsers) / float64(activeUsers) * 100, nil
}

// ======================================================================
// Health
// ======================================================================

func (r *sessionRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *sessionRepo) Close() error {
	return nil
}

func (r *sessionRepo) GetRawDB() interface{} {
	return r.db
}