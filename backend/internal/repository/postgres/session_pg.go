// backend/internal/repository/postgres/session_pg.go
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
// Basic Session Operations
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
			// Check if it's a duplicate refresh token
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

// Update updates a session (e.g., refresh token rotation, expiry extension).
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

// ======================================================================
// User Session Queries
// ======================================================================

// GetByUserID returns all active sessions for a user.
func (r *sessionRepo) GetByUserID(ctx context.Context, userID string) ([]*entities.Session, error) {
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1 AND expires_at > NOW()
		ORDER BY created_at DESC
	`
	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get sessions by user failed: %w", err)
	}
	return sessions, nil
}

// GetActiveByUserID returns active sessions for a user with pagination.
func (r *sessionRepo) GetActiveByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Session, string, error) {
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
	argIndex := 2
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get active sessions by user failed: %w", err)
	}

	var nextCursor string
	if len(sessions) == limit {
		nextCursor = sessions[len(sessions)-1].ID
	}
	return sessions, nextCursor, nil
}

// GetExpiredByUserID returns expired sessions for a user.
func (r *sessionRepo) GetExpiredByUserID(ctx context.Context, userID string) ([]*entities.Session, error) {
	query := `
		SELECT * FROM sessions
		WHERE user_id = $1 AND expires_at <= NOW()
		ORDER BY expires_at DESC
	`
	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get expired sessions by user failed: %w", err)
	}
	return sessions, nil
}

// GetLatestByUserID returns the most recent session for a user.
func (r *sessionRepo) GetLatestByUserID(ctx context.Context, userID string) (*entities.Session, error) {
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
			return nil, nil
		}
		return nil, fmt.Errorf("get latest session by user failed: %w", err)
	}
	return &session, nil
}

// ======================================================================
= Count Operations
// ======================================================================

// CountActiveByUserID returns the number of active sessions for a user.
func (r *sessionRepo) CountActiveByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND expires_at > NOW()`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count active sessions by user failed: %w", err)
	}
	return count, nil
}

// CountTotalByUserID returns the total number of sessions for a user.
func (r *sessionRepo) CountTotalByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE user_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count total sessions by user failed: %w", err)
	}
	return count, nil
}

// CountTotal returns the total number of sessions in the system.
func (r *sessionRepo) CountTotal(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count total sessions failed: %w", err)
	}
	return count, nil
}

// CountActive returns the number of active sessions in the system.
func (r *sessionRepo) CountActive(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE expires_at > NOW()`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count active sessions failed: %w", err)
	}
	return count, nil
}

// CountExpired returns the number of expired sessions in the system.
func (r *sessionRepo) CountExpired(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM sessions WHERE expires_at <= NOW()`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count expired sessions failed: %w", err)
	}
	return count, nil
}

// ======================================================================
= Expiry and Cleanup
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
		return 0, fmt.Errorf("cleanup sessions older than date failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// ExtendExpiry extends the expiry of a session.
func (r *sessionRepo) ExtendExpiry(ctx context.Context, id string, newExpiry time.Time) error {
	query := `UPDATE sessions SET expires_at = $1, updated_at = $2 WHERE id = $3`
	result, err := r.getDB().ExecContext(ctx, query, newExpiry, time.Now(), id)
	if err != nil {
		return fmt.Errorf("extend session expiry failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrSessionNotFound
	}
	return nil
}

// ======================================================================
= Bulk Operations
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

// BulkDeleteByUserID removes all sessions for a user.
func (r *sessionRepo) BulkDeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM sessions WHERE user_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("bulk delete sessions by user failed: %w", err)
	}
	return nil
}

// BulkDeleteExpired removes all expired sessions in bulk.
func (r *sessionRepo) BulkDeleteExpired(ctx context.Context) (int64, error) {
	return r.CleanupExpired(ctx)
}

// ======================================================================
= Advanced Queries
// ======================================================================

// GetSessionsByIP returns all sessions from a specific IP.
func (r *sessionRepo) GetSessionsByIP(ctx context.Context, ip string) ([]*entities.Session, error) {
	query := `SELECT * FROM sessions WHERE ip = $1 ORDER BY created_at DESC`
	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, ip)
	if err != nil {
		return nil, fmt.Errorf("get sessions by IP failed: %w", err)
	}
	return sessions, nil
}

// GetSessionsByUserAgent returns all sessions with a specific user agent.
func (r *sessionRepo) GetSessionsByUserAgent(ctx context.Context, userAgent string) ([]*entities.Session, error) {
	query := `SELECT * FROM sessions WHERE user_agent = $1 ORDER BY created_at DESC`
	var sessions []*entities.Session
	err := r.getDB().SelectContext(ctx, &sessions, query, userAgent)
	if err != nil {
		return nil, fmt.Errorf("get sessions by user agent failed: %w", err)
	}
	return sessions, nil
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
	argIndex := 3
	if cursor != "" {
		args = append(args, cursor)
		argIndex = 4
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

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetSessionStats returns aggregated session statistics.
func (r *sessionRepo) GetSessionStats(ctx context.Context) (*SessionStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(DISTINCT user_id) as unique_users,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired,
			MAX(created_at) as latest_session,
			MIN(created_at) as earliest_session,
			AVG(EXTRACT(EPOCH FROM (expires_at - created_at))) as avg_lifetime_seconds,
			COUNT(DISTINCT ip) as unique_ips,
			COUNT(DISTINCT user_agent) as unique_user_agents
		FROM sessions
	`
	var stats SessionStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get session stats failed: %w", err)
	}
	return &stats, nil
}

// SessionStats represents aggregated session statistics.
type SessionStats struct {
	Total              int64     `db:"total"`
	UniqueUsers        int64     `db:"unique_users"`
	Active             int64     `db:"active"`
	Expired            int64     `db:"expired"`
	LatestSession      time.Time `db:"latest_session"`
	EarliestSession    time.Time `db:"earliest_session"`
	AvgLifetimeSeconds float64   `db:"avg_lifetime_seconds"`
	UniqueIPs          int64     `db:"unique_ips"`
	UniqueUserAgents   int64     `db:"unique_user_agents"`
}

// GetDailySessions returns daily session creation counts.
func (r *sessionRepo) GetDailySessions(ctx context.Context, start, end time.Time) ([]*DailySessionCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT ip) as unique_ips
		FROM sessions
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailySessionCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily sessions failed: %w", err)
	}
	return results, nil
}

// DailySessionCount represents daily session counts.
type DailySessionCount struct {
	Date       time.Time `db:"date"`
	Total      int64     `db:"total"`
	UniqueUsers int64    `db:"unique_users"`
	UniqueIPs  int64     `db:"unique_ips"`
}

// GetUserSessionStats returns session statistics for a specific user.
func (r *sessionRepo) GetUserSessionStats(ctx context.Context, userID string) (*UserSessionStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as active,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired,
			MAX(created_at) as latest_session,
			MIN(created_at) as earliest_session,
			COUNT(DISTINCT ip) as unique_ips,
			COUNT(DISTINCT user_agent) as unique_user_agents
		FROM sessions
		WHERE user_id = $1
	`
	var stats UserSessionStats
	err := r.getDB().GetContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user session stats failed: %w", err)
	}
	return &stats, nil
}

// UserSessionStats represents session statistics for a user.
type UserSessionStats struct {
	Total            int64     `db:"total"`
	Active           int64     `db:"active"`
	Expired          int64     `db:"expired"`
	LatestSession    time.Time `db:"latest_session"`
	EarliestSession  time.Time `db:"earliest_session"`
	UniqueIPs        int64     `db:"unique_ips"`
	UniqueUserAgents int64     `db:"unique_user_agents"`
}

// GetSessionDeviceStats returns session statistics by user agent.
func (r *sessionRepo) GetSessionDeviceStats(ctx context.Context, userID string) ([]*DeviceStat, error) {
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
	var stats []*DeviceStat
	err := r.getDB().SelectContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get device stats failed: %w", err)
	}
	return stats, nil
}

// DeviceStat represents device statistics.
type DeviceStat struct {
	UserAgent string    `db:"user_agent"`
	Count     int64     `db:"count"`
	LastUsed  time.Time `db:"last_used"`
}

// ======================================================================
= Session Validation
// ======================================================================

// IsValidSession checks if a session exists and is active.
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

// IsValidRefreshToken checks if a refresh token is valid and active.
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
= Health
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