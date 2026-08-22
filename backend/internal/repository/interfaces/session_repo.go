// backend/internal/repository/interfaces/session_repo.go
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
	ErrSessionNotFound        = errors.New("session not found")
	ErrSessionExpired         = errors.New("session has expired")
	ErrInvalidSessionID       = errors.New("invalid session ID")
	ErrInvalidUserID          = errors.New("invalid user ID")
	ErrInvalidRefreshToken    = errors.New("invalid refresh token")
	ErrDuplicateRefreshToken  = errors.New("duplicate refresh token")
	ErrSessionRevoked         = errors.New("session has been revoked")
	ErrSessionLimitExceeded   = errors.New("maximum active sessions exceeded")
	ErrRefreshTokenExpired    = errors.New("refresh token has expired")
	ErrRefreshTokenRevoked    = errors.New("refresh token has been revoked")
)

// ======================================================================
// SessionFilter
// ======================================================================

// SessionFilter defines filtering options for session queries.
type SessionFilter struct {
	UserID      *string
	IP          *string
	UserAgent   *string
	IsActive    *bool
	IsExpired   *bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	ExpiresFrom *time.Time
	ExpiresTo   *time.Time
}

// HasCriteria checks if any filter criteria are set.
func (f *SessionFilter) HasCriteria() bool {
	return f.UserID != nil || f.IP != nil || f.UserAgent != nil ||
		f.IsActive != nil || f.IsExpired != nil || f.CreatedFrom != nil ||
		f.CreatedTo != nil || f.ExpiresFrom != nil || f.ExpiresTo != nil
}

// ======================================================================
// SessionPagination
// ======================================================================

// SessionSortField defines sortable fields for sessions.
type SessionSortField string

const (
	SortSessionByCreatedAt SessionSortField = "created_at"
	SortSessionByExpiresAt SessionSortField = "expires_at"
	SortSessionByUpdatedAt SessionSortField = "updated_at"
)

// SessionSortOrder defines sort order.
type SessionSortOrder string

const (
	SessionSortAsc  SessionSortOrder = "ASC"
	SessionSortDesc SessionSortOrder = "DESC"
)

// SessionPagination holds pagination options for sessions.
type SessionPagination struct {
	Cursor string            `json:"cursor"`
	Limit  int               `json:"limit"`
	SortBy SessionSortField  `json:"sort_by"`
	Order  SessionSortOrder  `json:"order"`
}

// DefaultSessionPagination returns default pagination options.
func DefaultSessionPagination() *SessionPagination {
	return &SessionPagination{
		Limit:  20,
		SortBy: SortSessionByCreatedAt,
		Order:  SessionSortDesc,
	}
}

// Validate checks pagination parameters.
func (p *SessionPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// SessionStats
// ======================================================================

// SessionStats represents aggregated session statistics.
type SessionStats struct {
	TotalSessions       int64     `json:"total_sessions"`
	ActiveSessions      int64     `json:"active_sessions"`
	ExpiredSessions     int64     `json:"expired_sessions"`
	UniqueUsers         int64     `json:"unique_users"`
	UniqueIPs           int64     `json:"unique_ips"`
	UniqueUserAgents    int64     `json:"unique_user_agents"`
	AvgSessionDuration  float64   `json:"avg_session_duration"` // in seconds
	MaxSessionDuration  float64   `json:"max_session_duration"`
	MinSessionDuration  float64   `json:"min_session_duration"`
	LastSessionCreated  time.Time `json:"last_session_created"`
	LastSessionExpired  time.Time `json:"last_session_expired"`
	MostActiveUserID    string    `json:"most_active_user_id"`
	MostActiveUserSessions int64  `json:"most_active_user_sessions"`
}

// ======================================================================
// DailySessionCount
// ======================================================================

// DailySessionCount represents daily session counts.
type DailySessionCount struct {
	Date         time.Time `json:"date"`
	Total        int64     `json:"total"`
	Active       int64     `json:"active"`
	Expired      int64     `json:"expired"`
	UniqueUsers  int64     `json:"unique_users"`
	UniqueIPs    int64     `json:"unique_ips"`
}

// ======================================================================
// SessionRepository Interface
// ======================================================================

// SessionRepository defines the interface for session data persistence.
type SessionRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a new session.
	Create(ctx context.Context, session *entities.Session) error

	// GetByID retrieves a session by its ID.
	GetByID(ctx context.Context, id string) (*entities.Session, error)

	// GetByRefreshToken retrieves a session by its refresh token.
	GetByRefreshToken(ctx context.Context, refreshToken string) (*entities.Session, error)

	// GetByUserID retrieves all sessions for a user.
	GetByUserID(ctx context.Context, userID string) ([]*entities.Session, error)

	// Update updates a session (e.g., refresh token rotation).
	Update(ctx context.Context, session *entities.Session) error

	// UpdateRefreshToken updates the refresh token for a session.
	UpdateRefreshToken(ctx context.Context, id, newRefreshToken string, newExpiry time.Time) error

	// Delete removes a session.
	Delete(ctx context.Context, id string) error

	// DeleteByRefreshToken removes a session by its refresh token.
	DeleteByRefreshToken(ctx context.Context, refreshToken string) error

	// DeleteByUserID removes all sessions for a user.
	DeleteByUserID(ctx context.Context, userID string) error

	// --------------------------------------------------------------------
	// Existence Checks
	// --------------------------------------------------------------------

	// Exists checks if a session exists.
	Exists(ctx context.Context, id string) (bool, error)

	// ExistsByRefreshToken checks if a refresh token exists.
	ExistsByRefreshToken(ctx context.Context, refreshToken string) (bool, error)

	// IsValidSession checks if a session is valid (exists and not expired).
	IsValidSession(ctx context.Context, id string) (bool, error)

	// IsValidRefreshToken checks if a refresh token is valid.
	IsValidRefreshToken(ctx context.Context, refreshToken string) (bool, error)

	// --------------------------------------------------------------------
	// Active/Expired Sessions
	// --------------------------------------------------------------------

	// GetActiveSessions returns active sessions for a user.
	GetActiveSessions(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Session, string, error)

	// GetExpiredSessions returns expired sessions for a user.
	GetExpiredSessions(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Session, string, error)

	// GetActiveSessionsAll returns all active sessions globally (for admin).
	GetActiveSessionsAll(ctx context.Context, cursor string, limit int) ([]*entities.Session, string, error)

	// GetExpiredSessionsAll returns all expired sessions globally.
	GetExpiredSessionsAll(ctx context.Context, cursor string, limit int) ([]*entities.Session, string, error)

	// GetSessionsByIP returns sessions from a specific IP.
	GetSessionsByIP(ctx context.Context, ip string, cursor string, limit int) ([]*entities.Session, string, error)

	// GetSessionsByUserAgent returns sessions with a specific user agent.
	GetSessionsByUserAgent(ctx context.Context, userAgent string, cursor string, limit int) ([]*entities.Session, string, error)

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountByUserID returns total sessions for a user.
	CountByUserID(ctx context.Context, userID string) (int64, error)

	// CountActiveByUserID returns active sessions count for a user.
	CountActiveByUserID(ctx context.Context, userID string) (int64, error)

	// CountExpiredByUserID returns expired sessions count for a user.
	CountExpiredByUserID(ctx context.Context, userID string) (int64, error)

	// CountTotal returns total sessions in the system.
	CountTotal(ctx context.Context) (int64, error)

	// CountActive returns total active sessions in the system.
	CountActive(ctx context.Context) (int64, error)

	// CountExpired returns total expired sessions in the system.
	CountExpired(ctx context.Context) (int64, error)

	// CountByDateRange returns session count within a date range.
	CountByDateRange(ctx context.Context, start, end time.Time) (int64, error)

	// CountByDateRangeForUser returns session count for a user within a date range.
	CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error)

	// CountUniqueUsersInDateRange returns unique users with sessions in a date range.
	CountUniqueUsersInDateRange(ctx context.Context, start, end time.Time) (int64, error)

	// --------------------------------------------------------------------
	// Expiry Management
	// --------------------------------------------------------------------

	// ExtendExpiry extends the expiry of a session.
	ExtendExpiry(ctx context.Context, id string, newExpiry time.Time) error

	// Revoke revokes a session (sets it as inactive/expired).
	Revoke(ctx context.Context, id string) error

	// RevokeAll revokes all sessions for a user.
	RevokeAll(ctx context.Context, userID string) error

	// RevokeAllExcept revokes all sessions for a user except the given one.
	RevokeAllExcept(ctx context.Context, userID, excludeSessionID string) error

	// RevokeByRefreshToken revokes a session by its refresh token.
	RevokeByRefreshToken(ctx context.Context, refreshToken string) error

	// --------------------------------------------------------------------
	// Cleanup Operations
	// --------------------------------------------------------------------

	// CleanupExpired removes all expired sessions.
	CleanupExpired(ctx context.Context) (int64, error)

	// CleanupExpiredForUser removes expired sessions for a specific user.
	CleanupExpiredForUser(ctx context.Context, userID string) (int64, error)

	// CleanupOlderThan removes sessions older than a date.
	CleanupOlderThan(ctx context.Context, before time.Time) (int64, error)

	// CleanupOlderThanForUser removes sessions older than a date for a user.
	CleanupOlderThanForUser(ctx context.Context, userID string, before time.Time) (int64, error)

	// --------------------------------------------------------------------
	// Advanced Queries
	// --------------------------------------------------------------------

	// GetLatestSession returns the most recent session for a user.
	GetLatestSession(ctx context.Context, userID string) (*entities.Session, error)

	// GetOldestSession returns the oldest session for a user.
	GetOldestSession(ctx context.Context, userID string) (*entities.Session, error)

	// GetSessionsByDateRange returns sessions within a date range.
	GetSessionsByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Session, string, error)

	// GetSessionsForUserByDateRange returns sessions for a user within a date range.
	GetSessionsForUserByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Session, string, error)

	// GetSessionsByUserAndIP returns sessions for a user from a specific IP.
	GetSessionsByUserAndIP(ctx context.Context, userID, ip string, cursor string, limit int) ([]*entities.Session, string, error)

	// GetSessionsWithMetadata returns sessions with specific metadata.
	GetSessionsWithMetadata(ctx context.Context, key, value string, cursor string, limit int) ([]*entities.Session, string, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple sessions in a single transaction.
	BulkCreate(ctx context.Context, sessions []*entities.Session) error

	// BulkDelete removes multiple sessions in a single transaction.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkDeleteByUserID removes all sessions for multiple users.
	BulkDeleteByUserIDs(ctx context.Context, userIDs []string) error

	// BulkRevoke revokes multiple sessions.
	BulkRevoke(ctx context.Context, ids []string) error

	// BulkExtendExpiry extends expiry for multiple sessions.
	BulkExtendExpiry(ctx context.Context, ids []string, newExpiry time.Time) error

	// BulkDeleteExpired removes all expired sessions in bulk.
	BulkDeleteExpired(ctx context.Context) (int64, error)

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetSessionStats returns aggregated session statistics.
	GetSessionStats(ctx context.Context) (*SessionStats, error)

	// GetUserSessionStats returns session statistics for a specific user.
	GetUserSessionStats(ctx context.Context, userID string) (*SessionStats, error)

	// GetDailySessionStats returns daily session counts for a date range.
	GetDailySessionStats(ctx context.Context, start, end time.Time) ([]*DailySessionCount, error)

	// GetDailySessionStatsForUser returns daily session counts for a user.
	GetDailySessionStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailySessionCount, error)

	// GetDeviceStats returns session statistics by user agent.
	GetDeviceStats(ctx context.Context, userID string) ([]*DeviceStat, error)

	// GetLocationStats returns session statistics by IP (geo-location based).
	GetLocationStats(ctx context.Context, userID string) ([]*LocationStat, error)

	// GetAverageSessionDuration calculates average session duration for a user.
	GetAverageSessionDuration(ctx context.Context, userID string) (float64, error)

	// GetSessionRetentionRate calculates session retention rate.
	GetSessionRetentionRate(ctx context.Context, days int) (float64, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) SessionRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo SessionRepository) error) error

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

// DeviceStat represents device statistics (by user agent).
type DeviceStat struct {
	UserAgent string    `json:"user_agent"`
	Count     int64     `json:"count"`
	LastUsed  time.Time `json:"last_used"`
}

// LocationStat represents location statistics (by IP).
type LocationStat struct {
	IP        string    `json:"ip"`
	Country   string    `json:"country"`
	Region    string    `json:"region"`
	City      string    `json:"city"`
	Count     int64     `json:"count"`
	LastUsed  time.Time `json:"last_used"`
}

// ======================================================================
// Helper Functions
// ======================================================================

// IsSessionNotFound checks if an error indicates a session was not found.
func IsSessionNotFound(err error) bool {
	return errors.Is(err, ErrSessionNotFound)
}

// IsSessionExpired checks if an error indicates a session expired.
func IsSessionExpired(err error) bool {
	return errors.Is(err, ErrSessionExpired) || errors.Is(err, ErrRefreshTokenExpired)
}

// IsSessionInvalid checks if an error indicates an invalid session.
func IsSessionInvalid(err error) bool {
	return errors.Is(err, ErrInvalidSessionID) ||
		errors.Is(err, ErrInvalidUserID) ||
		errors.Is(err, ErrInvalidRefreshToken) ||
		errors.Is(err, ErrDuplicateRefreshToken)
}

// IsSessionError checks if an error is session-related.
func IsSessionError(err error) bool {
	return errors.Is(err, ErrSessionNotFound) ||
		errors.Is(err, ErrSessionExpired) ||
		errors.Is(err, ErrSessionRevoked) ||
		errors.Is(err, ErrInvalidSessionID) ||
		errors.Is(err, ErrInvalidUserID) ||
		errors.Is(err, ErrInvalidRefreshToken) ||
		errors.Is(err, ErrDuplicateRefreshToken)
}

// ======================================================================
// Mock Session Repository (for testing)
// ======================================================================

// MockSessionRepository is a mock implementation for testing.
type MockSessionRepository struct {
	Sessions   map[string]*entities.Session
	UserSessions map[string][]string // userID -> session IDs
	Error      error
	NextCursor string
}

// NewMockSessionRepo creates a new mock repository.
func NewMockSessionRepo() SessionRepository {
	return &MockSessionRepository{
		Sessions:     make(map[string]*entities.Session),
		UserSessions: make(map[string][]string),
	}
}

// Create mock implementation.
func (m *MockSessionRepository) Create(ctx context.Context, session *entities.Session) error {
	if m.Error != nil {
		return m.Error
	}
	// Check for duplicate refresh token
	for _, s := range m.Sessions {
		if s.RefreshToken == session.RefreshToken {
			return ErrDuplicateRefreshToken
		}
	}
	m.Sessions[session.ID] = session
	if m.UserSessions[session.UserID] == nil {
		m.UserSessions[session.UserID] = []string{}
	}
	m.UserSessions[session.UserID] = append(m.UserSessions[session.UserID], session.ID)
	return nil
}

// GetByID mock implementation.
func (m *MockSessionRepository) GetByID(ctx context.Context, id string) (*entities.Session, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if session, ok := m.Sessions[id]; ok {
		return session, nil
	}
	return nil, ErrSessionNotFound
}

// GetByRefreshToken mock implementation.
func (m *MockSessionRepository) GetByRefreshToken(ctx context.Context, refreshToken string) (*entities.Session, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	for _, session := range m.Sessions {
		if session.RefreshToken == refreshToken {
			return session, nil
		}
	}
	return nil, ErrSessionNotFound
}

// GetByUserID mock implementation.
func (m *MockSessionRepository) GetByUserID(ctx context.Context, userID string) ([]*entities.Session, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if ids, ok := m.UserSessions[userID]; ok {
		var sessions []*entities.Session
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok {
				sessions = append(sessions, s)
			}
		}
		return sessions, nil
	}
	return []*entities.Session{}, nil
}

// Update mock implementation.
func (m *MockSessionRepository) Update(ctx context.Context, session *entities.Session) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Sessions[session.ID]; !ok {
		return ErrSessionNotFound
	}
	m.Sessions[session.ID] = session
	return nil
}

// UpdateRefreshToken mock implementation.
func (m *MockSessionRepository) UpdateRefreshToken(ctx context.Context, id, newRefreshToken string, newExpiry time.Time) error {
	if m.Error != nil {
		return m.Error
	}
	if session, ok := m.Sessions[id]; ok {
		session.RefreshToken = newRefreshToken
		session.ExpiresAt = newExpiry
		return nil
	}
	return ErrSessionNotFound
}

// Delete mock implementation.
func (m *MockSessionRepository) Delete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if session, ok := m.Sessions[id]; ok {
		delete(m.Sessions, id)
		// Remove from user list
		if userSessions, ok := m.UserSessions[session.UserID]; ok {
			for i, sid := range userSessions {
				if sid == id {
					m.UserSessions[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
					break
				}
			}
		}
		return nil
	}
	return ErrSessionNotFound
}

// DeleteByRefreshToken mock implementation.
func (m *MockSessionRepository) DeleteByRefreshToken(ctx context.Context, refreshToken string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, session := range m.Sessions {
		if session.RefreshToken == refreshToken {
			return m.Delete(ctx, id)
		}
	}
	return ErrSessionNotFound
}

// DeleteByUserID mock implementation.
func (m *MockSessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			delete(m.Sessions, id)
		}
		delete(m.UserSessions, userID)
	}
	return nil
}

// Exists mock implementation.
func (m *MockSessionRepository) Exists(ctx context.Context, id string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	_, ok := m.Sessions[id]
	return ok, nil
}

// ExistsByRefreshToken mock implementation.
func (m *MockSessionRepository) ExistsByRefreshToken(ctx context.Context, refreshToken string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	for _, session := range m.Sessions {
		if session.RefreshToken == refreshToken {
			return true, nil
		}
	}
	return false, nil
}

// IsValidSession mock implementation.
func (m *MockSessionRepository) IsValidSession(ctx context.Context, id string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	if session, ok := m.Sessions[id]; ok {
		return session.ExpiresAt.After(time.Now()), nil
	}
	return false, nil
}

// IsValidRefreshToken mock implementation.
func (m *MockSessionRepository) IsValidRefreshToken(ctx context.Context, refreshToken string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	for _, session := range m.Sessions {
		if session.RefreshToken == refreshToken && session.ExpiresAt.After(time.Now()) {
			return true, nil
		}
	}
	return false, nil
}

// GetActiveSessions mock implementation.
func (m *MockSessionRepository) GetActiveSessions(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	now := time.Now()
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok && s.ExpiresAt.After(now) {
				sessions = append(sessions, s)
			}
		}
	}
	return sessions, "", nil
}

// GetExpiredSessions mock implementation.
func (m *MockSessionRepository) GetExpiredSessions(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	now := time.Now()
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok && s.ExpiresAt.Before(now) {
				sessions = append(sessions, s)
			}
		}
	}
	return sessions, "", nil
}

// GetActiveSessionsAll mock implementation.
func (m *MockSessionRepository) GetActiveSessionsAll(ctx context.Context, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	now := time.Now()
	for _, s := range m.Sessions {
		if s.ExpiresAt.After(now) {
			sessions = append(sessions, s)
		}
	}
	return sessions, "", nil
}

// GetExpiredSessionsAll mock implementation.
func (m *MockSessionRepository) GetExpiredSessionsAll(ctx context.Context, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	now := time.Now()
	for _, s := range m.Sessions {
		if s.ExpiresAt.Before(now) {
			sessions = append(sessions, s)
		}
	}
	return sessions, "", nil
}

// GetSessionsByIP mock implementation.
func (m *MockSessionRepository) GetSessionsByIP(ctx context.Context, ip string, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	for _, s := range m.Sessions {
		if s.IP == ip {
			sessions = append(sessions, s)
		}
	}
	return sessions, "", nil
}

// GetSessionsByUserAgent mock implementation.
func (m *MockSessionRepository) GetSessionsByUserAgent(ctx context.Context, userAgent string, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	for _, s := range m.Sessions {
		if s.UserAgent == userAgent {
			sessions = append(sessions, s)
		}
	}
	return sessions, "", nil
}

// CountByUserID mock implementation.
func (m *MockSessionRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if ids, ok := m.UserSessions[userID]; ok {
		return int64(len(ids)), nil
	}
	return 0, nil
}

// CountActiveByUserID mock implementation.
func (m *MockSessionRepository) CountActiveByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	now := time.Now()
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok && s.ExpiresAt.After(now) {
				count++
			}
		}
	}
	return count, nil
}

// CountExpiredByUserID mock implementation.
func (m *MockSessionRepository) CountExpiredByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	now := time.Now()
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok && s.ExpiresAt.Before(now) {
				count++
			}
		}
	}
	return count, nil
}

// CountTotal mock implementation.
func (m *MockSessionRepository) CountTotal(ctx context.Context) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return int64(len(m.Sessions)), nil
}

// CountActive mock implementation.
func (m *MockSessionRepository) CountActive(ctx context.Context) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	now := time.Now()
	for _, s := range m.Sessions {
		if s.ExpiresAt.After(now) {
			count++
		}
	}
	return count, nil
}

// CountExpired mock implementation.
func (m *MockSessionRepository) CountExpired(ctx context.Context) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	now := time.Now()
	for _, s := range m.Sessions {
		if s.ExpiresAt.Before(now) {
			count++
		}
	}
	return count, nil
}

// CountByDateRange mock implementation.
func (m *MockSessionRepository) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, s := range m.Sessions {
		if s.CreatedAt.After(start) && s.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// CountByDateRangeForUser mock implementation.
func (m *MockSessionRepository) CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok && s.CreatedAt.After(start) && s.CreatedAt.Before(end) {
				count++
			}
		}
	}
	return count, nil
}

// CountUniqueUsersInDateRange mock implementation.
func (m *MockSessionRepository) CountUniqueUsersInDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	users := make(map[string]bool)
	for _, s := range m.Sessions {
		if s.CreatedAt.After(start) && s.CreatedAt.Before(end) {
			users[s.UserID] = true
		}
	}
	return int64(len(users)), nil
}

// ExtendExpiry mock implementation.
func (m *MockSessionRepository) ExtendExpiry(ctx context.Context, id string, newExpiry time.Time) error {
	if m.Error != nil {
		return m.Error
	}
	if session, ok := m.Sessions[id]; ok {
		session.ExpiresAt = newExpiry
		return nil
	}
	return ErrSessionNotFound
}

// Revoke mock implementation.
func (m *MockSessionRepository) Revoke(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if session, ok := m.Sessions[id]; ok {
		// Set expiry to now to revoke
		session.ExpiresAt = time.Now()
		return nil
	}
	return ErrSessionNotFound
}

// RevokeAll mock implementation.
func (m *MockSessionRepository) RevokeAll(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if ids, ok := m.UserSessions[userID]; ok {
		now := time.Now()
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok {
				s.ExpiresAt = now
			}
		}
	}
	return nil
}

// RevokeAllExcept mock implementation.
func (m *MockSessionRepository) RevokeAllExcept(ctx context.Context, userID, excludeSessionID string) error {
	if m.Error != nil {
		return m.Error
	}
	if ids, ok := m.UserSessions[userID]; ok {
		now := time.Now()
		for _, id := range ids {
			if id != excludeSessionID {
				if s, ok := m.Sessions[id]; ok {
					s.ExpiresAt = now
				}
			}
		}
	}
	return nil
}

// RevokeByRefreshToken mock implementation.
func (m *MockSessionRepository) RevokeByRefreshToken(ctx context.Context, refreshToken string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, s := range m.Sessions {
		if s.RefreshToken == refreshToken {
			s.ExpiresAt = time.Now()
			return nil
		}
	}
	return ErrSessionNotFound
}

// CleanupExpired mock implementation.
func (m *MockSessionRepository) CleanupExpired(ctx context.Context) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	now := time.Now()
	for id, s := range m.Sessions {
		if s.ExpiresAt.Before(now) {
			delete(m.Sessions, id)
			// Remove from user list
			if userSessions, ok := m.UserSessions[s.UserID]; ok {
				for i, sid := range userSessions {
					if sid == id {
						m.UserSessions[s.UserID] = append(userSessions[:i], userSessions[i+1:]...)
						break
					}
				}
			}
			count++
		}
	}
	return count, nil
}

// CleanupExpiredForUser mock implementation.
func (m *MockSessionRepository) CleanupExpiredForUser(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	now := time.Now()
	if ids, ok := m.UserSessions[userID]; ok {
		var remaining []string
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok {
				if s.ExpiresAt.Before(now) {
					delete(m.Sessions, id)
					count++
				} else {
					remaining = append(remaining, id)
				}
			}
		}
		m.UserSessions[userID] = remaining
	}
	return count, nil
}

// CleanupOlderThan mock implementation.
func (m *MockSessionRepository) CleanupOlderThan(ctx context.Context, before time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for id, s := range m.Sessions {
		if s.CreatedAt.Before(before) {
			delete(m.Sessions, id)
			if userSessions, ok := m.UserSessions[s.UserID]; ok {
				for i, sid := range userSessions {
					if sid == id {
						m.UserSessions[s.UserID] = append(userSessions[:i], userSessions[i+1:]...)
						break
					}
				}
			}
			count++
		}
	}
	return count, nil
}

// CleanupOlderThanForUser mock implementation.
func (m *MockSessionRepository) CleanupOlderThanForUser(ctx context.Context, userID string, before time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	if ids, ok := m.UserSessions[userID]; ok {
		var remaining []string
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok {
				if s.CreatedAt.Before(before) {
					delete(m.Sessions, id)
					count++
				} else {
					remaining = append(remaining, id)
				}
			}
		}
		m.UserSessions[userID] = remaining
	}
	return count, nil
}

// GetLatestSession mock implementation.
func (m *MockSessionRepository) GetLatestSession(ctx context.Context, userID string) (*entities.Session, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var latest *entities.Session
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok {
				if latest == nil || s.CreatedAt.After(latest.CreatedAt) {
					latest = s
				}
			}
		}
	}
	if latest == nil {
		return nil, ErrSessionNotFound
	}
	return latest, nil
}

// GetOldestSession mock implementation.
func (m *MockSessionRepository) GetOldestSession(ctx context.Context, userID string) (*entities.Session, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var oldest *entities.Session
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok {
				if oldest == nil || s.CreatedAt.Before(oldest.CreatedAt) {
					oldest = s
				}
			}
		}
	}
	if oldest == nil {
		return nil, ErrSessionNotFound
	}
	return oldest, nil
}

// GetSessionsByDateRange mock implementation.
func (m *MockSessionRepository) GetSessionsByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	for _, s := range m.Sessions {
		if s.CreatedAt.After(start) && s.CreatedAt.Before(end) {
			sessions = append(sessions, s)
		}
	}
	return sessions, "", nil
}

// GetSessionsForUserByDateRange mock implementation.
func (m *MockSessionRepository) GetSessionsForUserByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok && s.CreatedAt.After(start) && s.CreatedAt.Before(end) {
				sessions = append(sessions, s)
			}
		}
	}
	return sessions, "", nil
}

// GetSessionsByUserAndIP mock implementation.
func (m *MockSessionRepository) GetSessionsByUserAndIP(ctx context.Context, userID, ip string, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	if ids, ok := m.UserSessions[userID]; ok {
		for _, id := range ids {
			if s, ok := m.Sessions[id]; ok && s.IP == ip {
				sessions = append(sessions, s)
			}
		}
	}
	return sessions, "", nil
}

// GetSessionsWithMetadata mock implementation.
func (m *MockSessionRepository) GetSessionsWithMetadata(ctx context.Context, key, value string, cursor string, limit int) ([]*entities.Session, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var sessions []*entities.Session
	for _, s := range m.Sessions {
		if s.Metadata != nil {
			if v, ok := s.Metadata[key]; ok && v == value {
				sessions = append(sessions, s)
			}
		}
	}
	return sessions, "", nil
}

// BulkCreate mock implementation.
func (m *MockSessionRepository) BulkCreate(ctx context.Context, sessions []*entities.Session) error {
	if m.Error != nil {
		return m.Error
	}
	for _, s := range sessions {
		_ = m.Create(ctx, s)
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockSessionRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.Delete(ctx, id)
	}
	return nil
}

// BulkDeleteByUserIDs mock implementation.
func (m *MockSessionRepository) BulkDeleteByUserIDs(ctx context.Context, userIDs []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, uid := range userIDs {
		_ = m.DeleteByUserID(ctx, uid)
	}
	return nil
}

// BulkRevoke mock implementation.
func (m *MockSessionRepository) BulkRevoke(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.Revoke(ctx, id)
	}
	return nil
}

// BulkExtendExpiry mock implementation.
func (m *MockSessionRepository) BulkExtendExpiry(ctx context.Context, ids []string, newExpiry time.Time) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.ExtendExpiry(ctx, id, newExpiry)
	}
	return nil
}

// BulkDeleteExpired mock implementation.
func (m *MockSessionRepository) BulkDeleteExpired(ctx context.Context) (int64, error) {
	return m.CleanupExpired(ctx)
}

// GetSessionStats mock implementation.
func (m *MockSessionRepository) GetSessionStats(ctx context.Context) (*SessionStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	now := time.Now()
	stats := &SessionStats{
		TotalSessions: int64(len(m.Sessions)),
	}
	for _, s := range m.Sessions {
		if s.ExpiresAt.After(now) {
			stats.ActiveSessions++
		} else {
			stats.ExpiredSessions++
		}
	}
	return stats, nil
}

// GetUserSessionStats mock implementation.
func (m *MockSessionRepository) GetUserSessionStats(ctx context.Context, userID string) (*SessionStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	sessions, _ := m.GetByUserID(ctx, userID)
	stats := &SessionStats{
		TotalSessions: int64(len(sessions)),
	}
	now := time.Now()
	for _, s := range sessions {
		if s.ExpiresAt.After(now) {
			stats.ActiveSessions++
		} else {
			stats.ExpiredSessions++
		}
	}
	return stats, nil
}

// GetDailySessionStats mock implementation.
func (m *MockSessionRepository) GetDailySessionStats(ctx context.Context, start, end time.Time) ([]*DailySessionCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailySessionCount{}, nil
}

// GetDailySessionStatsForUser mock implementation.
func (m *MockSessionRepository) GetDailySessionStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailySessionCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailySessionCount{}, nil
}

// GetDeviceStats mock implementation.
func (m *MockSessionRepository) GetDeviceStats(ctx context.Context, userID string) ([]*DeviceStat, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DeviceStat{}, nil
}

// GetLocationStats mock implementation.
func (m *MockSessionRepository) GetLocationStats(ctx context.Context, userID string) ([]*LocationStat, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*LocationStat{}, nil
}

// GetAverageSessionDuration mock implementation.
func (m *MockSessionRepository) GetAverageSessionDuration(ctx context.Context, userID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// GetSessionRetentionRate mock implementation.
func (m *MockSessionRepository) GetSessionRetentionRate(ctx context.Context, days int) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// WithTransaction mock implementation.
func (m *MockSessionRepository) WithTransaction(ctx context.Context, tx *sql.Tx) SessionRepository {
	return m
}

// Transaction mock implementation.
func (m *MockSessionRepository) Transaction(ctx context.Context, fn func(txRepo SessionRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockSessionRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockSessionRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockSessionRepository) GetRawDB() interface{} {
	return nil
}