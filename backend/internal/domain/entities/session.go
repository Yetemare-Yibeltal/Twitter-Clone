// backend/internal/domain/entities/session.go
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

// SessionStatus represents the status of a session.
type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "active"
	SessionStatusExpired  SessionStatus = "expired"
	SessionStatusRevoked  SessionStatus = "revoked"
	SessionStatusInactive SessionStatus = "inactive"
)

// ValidSessionStatuses returns all valid session statuses.
func ValidSessionStatuses() []SessionStatus {
	return []SessionStatus{
		SessionStatusActive,
		SessionStatusExpired,
		SessionStatusRevoked,
		SessionStatusInactive,
	}
}

// IsValid checks if a session status is valid.
func (s SessionStatus) IsValid() bool {
	for _, status := range ValidSessionStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

// String returns the string representation of the status.
func (s SessionStatus) String() string {
	return string(s)
}

// SessionType represents the type of session.
type SessionType string

const (
	SessionTypeWeb      SessionType = "web"
	SessionTypeMobile   SessionType = "mobile"
	SessionTypeAPI      SessionType = "api"
	SessionTypeAdmin    SessionType = "admin"
	SessionTypeOAuth    SessionType = "oauth"
)

// ValidSessionTypes returns all valid session types.
func ValidSessionTypes() []SessionType {
	return []SessionType{
		SessionTypeWeb,
		SessionTypeMobile,
		SessionTypeAPI,
		SessionTypeAdmin,
		SessionTypeOAuth,
	}
}

// IsValid checks if a session type is valid.
func (t SessionType) IsValid() bool {
	for _, typ := range ValidSessionTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation of the type.
func (t SessionType) String() string {
	return string(t)
}

// ======================================================================
// Errors
// ======================================================================

var (
	ErrSessionIDEmpty         = errors.New("session ID cannot be empty")
	ErrUserIDEmpty            = errors.New("user ID cannot be empty")
	ErrRefreshTokenEmpty      = errors.New("refresh token cannot be empty")
	ErrInvalidSessionStatus   = errors.New("invalid session status")
	ErrInvalidSessionType     = errors.New("invalid session type")
	ErrSessionAlreadyActive   = errors.New("session already active")
	ErrSessionAlreadyExpired  = errors.New("session already expired")
	ErrSessionAlreadyRevoked  = errors.New("session already revoked")
	ErrSessionAlreadyDeleted  = errors.New("session already deleted")
	ErrSessionNotDeleted      = errors.New("session is not deleted")
	ErrSessionNotFound        = errors.New("session not found")
	ErrSessionExpired         = errors.New("session has expired")
	ErrSessionRevoked         = errors.New("session has been revoked")
	ErrRefreshTokenTooShort   = errors.New("refresh token too short")
	ErrRefreshTokenTooLong    = errors.New("refresh token too long")
	ErrExpiryTimeInvalid      = errors.New("expiry time is invalid")
	ErrExpiryTimeInPast       = errors.New("expiry time cannot be in the past")
)

// ======================================================================
// Session Entity
// ======================================================================

// Session represents a user session in the system.
type Session struct {
	ID           string          `db:"id" json:"id"`
	UserID       string          `db:"user_id" json:"user_id"`
	RefreshToken string          `db:"refresh_token" json:"refresh_token"`
	Status       SessionStatus   `db:"status" json:"status"`
	Type         SessionType     `db:"type" json:"type"`
	UserAgent    string          `db:"user_agent" json:"user_agent,omitempty"`
	IP           string          `db:"ip" json:"ip,omitempty"`
	DeviceID     string          `db:"device_id" json:"device_id,omitempty"`
	DeviceName   string          `db:"device_name" json:"device_name,omitempty"`
	OS           string          `db:"os" json:"os,omitempty"`
	Browser      string          `db:"browser" json:"browser,omitempty"`
	Location     string          `db:"location" json:"location,omitempty"`
	ExpiresAt    time.Time       `db:"expires_at" json:"expires_at"`
	LastActiveAt *time.Time      `db:"last_active_at" json:"last_active_at,omitempty"`
	Metadata     SessionMetadata `db:"metadata" json:"metadata,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
	DeletedAt    *time.Time      `db:"deleted_at" json:"deleted_at,omitempty"`
}

// SessionMetadata holds optional session metadata.
type SessionMetadata struct {
	LoginMethod   string            `json:"login_method,omitempty"` // "password", "oauth", "sso"
	AuthProvider  string            `json:"auth_provider,omitempty"`
	SessionToken  string            `json:"session_token,omitempty"`
	CustomData    map[string]string `json:"custom_data,omitempty"`
	SecurityFlags map[string]bool   `json:"security_flags,omitempty"`
}

// Value implements driver.Valuer for JSON storage.
func (m SessionMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements sql.Scanner for JSON retrieval.
func (m *SessionMetadata) Scan(value interface{}) error {
	if value == nil {
		*m = SessionMetadata{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for SessionMetadata: %T", value)
	}
	return json.Unmarshal(bytes, m)
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewSession creates a new session with default values.
func NewSession(userID, refreshToken string, expiresAt time.Time, sessionType SessionType) (*Session, error) {
	if !sessionType.IsValid() {
		sessionType = SessionTypeWeb
	}
	if expiresAt.IsZero() {
		return nil, ErrExpiryTimeInvalid
	}
	if expiresAt.Before(time.Now()) {
		return nil, ErrExpiryTimeInPast
	}
	s := &Session{
		ID:           uuid.New().String(),
		UserID:       userID,
		RefreshToken: refreshToken,
		Status:       SessionStatusActive,
		Type:         sessionType,
		ExpiresAt:    expiresAt,
		Metadata:     SessionMetadata{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// NewSessionWithMetadata creates a session with metadata.
func NewSessionWithMetadata(userID, refreshToken string, expiresAt time.Time, sessionType SessionType, metadata SessionMetadata) (*Session, error) {
	s, err := NewSession(userID, refreshToken, expiresAt, sessionType)
	if err != nil {
		return nil, err
	}
	s.Metadata = metadata
	return s, nil
}

// MustNewSession creates a new session and panics on error.
func MustNewSession(userID, refreshToken string, expiresAt time.Time, sessionType SessionType) *Session {
	s, err := NewSession(userID, refreshToken, expiresAt, sessionType)
	if err != nil {
		panic(err)
	}
	return s
}

// ======================================================================
// Validation
// ======================================================================

// Validate performs comprehensive validation.
func (s *Session) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return ErrSessionIDEmpty
	}
	if strings.TrimSpace(s.UserID) == "" {
		return ErrUserIDEmpty
	}
	refreshTokenTrimmed := strings.TrimSpace(s.RefreshToken)
	if refreshTokenTrimmed == "" {
		return ErrRefreshTokenEmpty
	}
	if len(refreshTokenTrimmed) < 16 {
		return ErrRefreshTokenTooShort
	}
	if len(refreshTokenTrimmed) > 256 {
		return ErrRefreshTokenTooLong
	}
	s.RefreshToken = refreshTokenTrimmed
	if !s.Status.IsValid() {
		return ErrInvalidSessionStatus
	}
	if !s.Type.IsValid() {
		return ErrInvalidSessionType
	}
	if s.ExpiresAt.IsZero() {
		return ErrExpiryTimeInvalid
	}
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	if s.Status == SessionStatusExpired && s.ExpiresAt.After(time.Now()) {
		return errors.New("expired status with future expiry")
	}
	if s.Status == SessionStatusActive && s.ExpiresAt.Before(time.Now()) {
		return errors.New("active status with past expiry")
	}
	return nil
}

// ======================================================================
// Status Management
// ======================================================================

// SetStatus sets the session status.
func (s *Session) SetStatus(status SessionStatus) error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	if !status.IsValid() {
		return ErrInvalidSessionStatus
	}
	if s.Status == status {
		return nil
	}
	// Check status transitions
	if s.Status == SessionStatusRevoked && status != SessionStatusRevoked {
		return errors.New("cannot change status of revoked session")
	}
	if s.Status == SessionStatusExpired && status != SessionStatusExpired {
		return errors.New("cannot change status of expired session")
	}
	s.Status = status
	s.UpdatedAt = time.Now()
	return nil
}

// Activate activates the session.
func (s *Session) Activate() error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	if s.Status == SessionStatusActive {
		return ErrSessionAlreadyActive
	}
	if s.ExpiresAt.Before(time.Now()) {
		return errors.New("cannot activate expired session")
	}
	s.Status = SessionStatusActive
	s.UpdatedAt = time.Now()
	return nil
}

// Expire expires the session.
func (s *Session) Expire() error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	if s.Status == SessionStatusExpired {
		return ErrSessionAlreadyExpired
	}
	if s.Status == SessionStatusRevoked {
		return errors.New("cannot expire revoked session")
	}
	s.Status = SessionStatusExpired
	s.ExpiresAt = time.Now()
	s.UpdatedAt = time.Now()
	return nil
}

// Revoke revokes the session.
func (s *Session) Revoke() error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	if s.Status == SessionStatusRevoked {
		return ErrSessionAlreadyRevoked
	}
	s.Status = SessionStatusRevoked
	s.UpdatedAt = time.Now()
	return nil
}

// IsActive checks if the session is active.
func (s *Session) IsActive() bool {
	return s.Status == SessionStatusActive && s.ExpiresAt.After(time.Now()) && s.DeletedAt == nil
}

// IsExpired checks if the session is expired.
func (s *Session) IsExpired() bool {
	return s.Status == SessionStatusExpired || s.ExpiresAt.Before(time.Now())
}

// IsRevoked checks if the session is revoked.
func (s *Session) IsRevoked() bool {
	return s.Status == SessionStatusRevoked
}

// IsValid checks if the session is valid (active and not expired).
func (s *Session) IsValid() bool {
	return s.IsActive()
}

// ======================================================================
= Expiry Management
// ======================================================================

// SetExpiry sets the session expiry time.
func (s *Session) SetExpiry(expiresAt time.Time) error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	if s.Status == SessionStatusRevoked {
		return errors.New("cannot modify expiry of revoked session")
	}
	if expiresAt.IsZero() {
		return ErrExpiryTimeInvalid
	}
	if expiresAt.Before(time.Now()) {
		return ErrExpiryTimeInPast
	}
	s.ExpiresAt = expiresAt
	s.UpdatedAt = time.Now()
	return nil
}

// ExtendExpiry extends the session expiry by a duration.
func (s *Session) ExtendExpiry(duration time.Duration) error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	if s.Status == SessionStatusRevoked {
		return errors.New("cannot extend expiry of revoked session")
	}
	if duration <= 0 {
		return errors.New("duration must be positive")
	}
	s.ExpiresAt = s.ExpiresAt.Add(duration)
	s.UpdatedAt = time.Now()
	return nil
}

// TimeUntilExpiry returns the time until the session expires.
func (s *Session) TimeUntilExpiry() time.Duration {
	if s.ExpiresAt.IsZero() {
		return 0
	}
	if s.ExpiresAt.Before(time.Now()) {
		return -time.Since(s.ExpiresAt)
	}
	return time.Until(s.ExpiresAt)
}

// IsExpiringSoon checks if the session is expiring within the given duration.
func (s *Session) IsExpiringSoon(duration time.Duration) bool {
	return s.TimeUntilExpiry() <= duration
}

// ======================================================================
= Token Management
// ======================================================================

// UpdateRefreshToken updates the refresh token.
func (s *Session) UpdateRefreshToken(newToken string) error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	if s.Status == SessionStatusRevoked {
		return errors.New("cannot update token of revoked session")
	}
	newToken = strings.TrimSpace(newToken)
	if newToken == "" {
		return ErrRefreshTokenEmpty
	}
	if len(newToken) < 16 {
		return ErrRefreshTokenTooShort
	}
	if len(newToken) > 256 {
		return ErrRefreshTokenTooLong
	}
	s.RefreshToken = newToken
	s.UpdatedAt = time.Now()
	return nil
}

// RotateRefreshToken rotates the refresh token (generates a new one).
func (s *Session) RotateRefreshToken(newToken string) error {
	return s.UpdateRefreshToken(newToken)
}

// ======================================================================
// Device Information
// ======================================================================

// SetDeviceInfo sets device information.
func (s *Session) SetDeviceInfo(deviceID, deviceName, os, browser string) error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	s.DeviceID = strings.TrimSpace(deviceID)
	s.DeviceName = strings.TrimSpace(deviceName)
	s.OS = strings.TrimSpace(os)
	s.Browser = strings.TrimSpace(browser)
	s.UpdatedAt = time.Now()
	return nil
}

// SetIP sets the IP address.
func (s *Session) SetIP(ip string) error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	s.IP = strings.TrimSpace(ip)
	s.UpdatedAt = time.Now()
	return nil
}

// SetUserAgent sets the user agent.
func (s *Session) SetUserAgent(userAgent string) error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	s.UserAgent = strings.TrimSpace(userAgent)
	s.UpdatedAt = time.Now()
	return nil
}

// SetLocation sets the location.
func (s *Session) SetLocation(location string) error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	s.Location = strings.TrimSpace(location)
	s.UpdatedAt = time.Now()
	return nil
}

// ======================================================================
= Activity Management
// ======================================================================

// UpdateLastActive updates the last active timestamp.
func (s *Session) UpdateLastActive() {
	if s.DeletedAt == nil {
		now := time.Now()
		s.LastActiveAt = &now
		s.UpdatedAt = now
	}
}

// GetLastActive returns the last active time or zero time.
func (s *Session) GetLastActive() time.Time {
	if s.LastActiveAt == nil {
		return time.Time{}
	}
	return *s.LastActiveAt
}

// ======================================================================
= Deletion Operations
// ======================================================================

// SoftDelete marks the session as deleted.
func (s *Session) SoftDelete() error {
	if s.DeletedAt != nil {
		return ErrSessionAlreadyDeleted
	}
	now := time.Now()
	s.DeletedAt = &now
	s.UpdatedAt = now
	return nil
}

// Restore restores a soft-deleted session.
func (s *Session) Restore() error {
	if s.DeletedAt == nil {
		return ErrSessionNotDeleted
	}
	s.DeletedAt = nil
	s.UpdatedAt = time.Now()
	return nil
}

// IsDeleted checks if the session is deleted.
func (s *Session) IsDeleted() bool {
	return s.DeletedAt != nil
}

// ======================================================================
= Helper Methods
// ======================================================================

// IsUser checks if the session belongs to a specific user.
func (s *Session) IsUser(userID string) bool {
	return s.UserID == userID
}

// IsType checks if the session is of a specific type.
func (s *Session) IsType(sessionType SessionType) bool {
	return s.Type == sessionType
}

// IsWeb checks if the session is a web session.
func (s *Session) IsWeb() bool {
	return s.Type == SessionTypeWeb
}

// IsMobile checks if the session is a mobile session.
func (s *Session) IsMobile() bool {
	return s.Type == SessionTypeMobile
}

// IsAPI checks if the session is an API session.
func (s *Session) IsAPI() bool {
	return s.Type == SessionTypeAPI
}

// IsAdmin checks if the session is an admin session.
func (s *Session) IsAdmin() bool {
	return s.Type == SessionTypeAdmin
}

// GetDeviceDisplay returns a display name for the device.
func (s *Session) GetDeviceDisplay() string {
	if s.DeviceName != "" {
		return s.DeviceName
	}
	if s.OS != "" && s.Browser != "" {
		return s.OS + " - " + s.Browser
	}
	if s.OS != "" {
		return s.OS
	}
	if s.Browser != "" {
		return s.Browser
	}
	return "Unknown Device"
}

// GetLocationDisplay returns a display name for the location.
func (s *Session) GetLocationDisplay() string {
	if s.Location != "" {
		return s.Location
	}
	if s.IP != "" {
		return s.IP
	}
	return "Unknown Location"
}

// String returns a human-readable representation.
func (s *Session) String() string {
	return fmt.Sprintf("Session{ID:%s, user:%s, type:%s, status:%s, expires:%v, created:%v}",
		s.ID, s.UserID, s.Type, s.Status, s.ExpiresAt, s.CreatedAt)
}

// Clone returns a deep copy of the session.
func (s *Session) Clone() *Session {
	clone := *s
	if s.LastActiveAt != nil {
		t := *s.LastActiveAt
		clone.LastActiveAt = &t
	}
	if s.DeletedAt != nil {
		t := *s.DeletedAt
		clone.DeletedAt = &t
	}
	if s.Metadata.CustomData != nil {
		clone.Metadata.CustomData = make(map[string]string)
		for k, v := range s.Metadata.CustomData {
			clone.Metadata.CustomData[k] = v
		}
	}
	if s.Metadata.SecurityFlags != nil {
		clone.Metadata.SecurityFlags = make(map[string]bool)
		for k, v := range s.Metadata.SecurityFlags {
			clone.Metadata.SecurityFlags[k] = v
		}
	}
	return &clone
}

// Equals checks if two sessions are the same by ID.
func (s *Session) Equals(other *Session) bool {
	return s.ID == other.ID
}

// IsEmpty returns true if the session is zero value.
func (s *Session) IsEmpty() bool {
	return s.ID == "" && s.UserID == "" && s.RefreshToken == ""
}

// ======================================================================
= Database Value Handling
// ======================================================================

// Value implements driver.Valuer for JSONB storage.
func (s Session) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (s *Session) Scan(value interface{}) error {
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
		return fmt.Errorf("unsupported type for Session: %T", value)
	}
	return json.Unmarshal(bytes, s)
}

// ======================================================================
// JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (s *Session) MarshalJSON() ([]byte, error) {
	type Alias Session
	return json.Marshal(&struct {
		*Alias
		Status      string `json:"status"`
		Type        string `json:"type"`
		IsValid     bool   `json:"is_valid"`
		IsExpiring  bool   `json:"is_expiring"`
		DeviceDisplay string `json:"device_display"`
	}{
		Alias:        (*Alias)(s),
		Status:       string(s.Status),
		Type:         string(s.Type),
		IsValid:      s.IsValid(),
		IsExpiring:   s.IsExpiringSoon(1 * time.Hour),
		DeviceDisplay: s.GetDeviceDisplay(),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (s *Session) UnmarshalJSON(data []byte) error {
	type Alias Session
	aux := &struct {
		*Alias
		Status string `json:"status"`
		Type   string `json:"type"`
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Status != "" {
		s.Status = SessionStatus(aux.Status)
	}
	if aux.Type != "" {
		s.Type = SessionType(aux.Type)
	}
	return nil
}

// ======================================================================
// Session Group (for batch operations)
// ======================================================================

// SessionGroup represents a group of sessions.
type SessionGroup struct {
	Sessions []*Session `json:"sessions"`
	Total    int64      `json:"total"`
}

// NewSessionGroup creates a new session group.
func NewSessionGroup() *SessionGroup {
	return &SessionGroup{
		Sessions: []*Session{},
		Total:    0,
	}
}

// Add adds a session to the group.
func (g *SessionGroup) Add(s *Session) {
	g.Sessions = append(g.Sessions, s)
	g.Total++
}

// Contains checks if a session is in the group.
func (g *SessionGroup) Contains(id string) bool {
	for _, s := range g.Sessions {
		if s.ID == id {
			return true
		}
	}
	return false
}

// FilterByStatus returns sessions with a specific status.
func (g *SessionGroup) FilterByStatus(status SessionStatus) []*Session {
	result := []*Session{}
	for _, s := range g.Sessions {
		if s.Status == status {
			result = append(result, s)
		}
	}
	return result
}

// FilterByUser returns sessions for a specific user.
func (g *SessionGroup) FilterByUser(userID string) []*Session {
	result := []*Session{}
	for _, s := range g.Sessions {
		if s.UserID == userID {
			result = append(result, s)
		}
	}
	return result
}

// FilterByType returns sessions of a specific type.
func (g *SessionGroup) FilterByType(sessionType SessionType) []*Session {
	result := []*Session{}
	for _, s := range g.Sessions {
		if s.Type == sessionType {
			result = append(result, s)
		}
	}
	return result
}

// FilterActive returns active sessions.
func (g *SessionGroup) FilterActive() []*Session {
	return g.FilterByStatus(SessionStatusActive)
}

// FilterExpired returns expired sessions.
func (g *SessionGroup) FilterExpired() []*Session {
	return g.FilterByStatus(SessionStatusExpired)
}

// FilterRevoked returns revoked sessions.
func (g *SessionGroup) FilterRevoked() []*Session {
	return g.FilterByStatus(SessionStatusRevoked)
}

// FilterValid returns valid sessions (active and not expired).
func (g *SessionGroup) FilterValid() []*Session {
	result := []*Session{}
	for _, s := range g.Sessions {
		if s.IsValid() {
			result = append(result, s)
		}
	}
	return result
}

// GetByDeviceID returns sessions by device ID.
func (g *SessionGroup) GetByDeviceID(deviceID string) []*Session {
	result := []*Session{}
	for _, s := range g.Sessions {
		if s.DeviceID == deviceID {
			result = append(result, s)
		}
	}
	return result
}

// ======================================================================
// Session Statistics
// ======================================================================

// SessionStats represents session statistics.
type SessionStats struct {
	TotalSessions   int64            `json:"total_sessions"`
	ActiveSessions  int64            `json:"active_sessions"`
	ExpiredSessions int64            `json:"expired_sessions"`
	RevokedSessions int64            `json:"revoked_sessions"`
	StatusStats     map[string]int64 `json:"status_stats"`
	TypeStats       map[string]int64 `json:"type_stats"`
	UniqueUsers     int64            `json:"unique_users"`
	UniqueDevices   int64            `json:"unique_devices"`
	AvgDuration     float64          `json:"avg_duration_hours"`
}

// CalculateStats calculates statistics from a session group.
func (g *SessionGroup) CalculateStats() *SessionStats {
	stats := &SessionStats{
		TotalSessions: int64(len(g.Sessions)),
		StatusStats:   make(map[string]int64),
		TypeStats:     make(map[string]int64),
	}
	users := make(map[string]bool)
	devices := make(map[string]bool)
	var totalDuration float64
	var durationCount int64

	for _, s := range g.Sessions {
		users[s.UserID] = true
		if s.DeviceID != "" {
			devices[s.DeviceID] = true
		}
		stats.StatusStats[string(s.Status)]++
		stats.TypeStats[string(s.Type)]++

		switch s.Status {
		case SessionStatusActive:
			stats.ActiveSessions++
		case SessionStatusExpired:
			stats.ExpiredSessions++
		case SessionStatusRevoked:
			stats.RevokedSessions++
		}

		// Calculate duration for expired/revoked sessions
		if s.Status == SessionStatusExpired || s.Status == SessionStatusRevoked {
			duration := s.CreatedAt.Sub(s.ExpiresAt)
			if duration > 0 {
				totalDuration += duration.Hours()
				durationCount++
			}
		}
	}
	stats.UniqueUsers = int64(len(users))
	stats.UniqueDevices = int64(len(devices))
	if durationCount > 0 {
		stats.AvgDuration = totalDuration / float64(durationCount)
	}
	return stats
}

// ======================================================================
= Builder Pattern (for tests)
// ======================================================================

// SessionBuilder helps construct sessions for testing.
type SessionBuilder struct {
	session *Session
}

// NewSessionBuilder creates a new session builder.
func NewSessionBuilder() *SessionBuilder {
	return &SessionBuilder{
		session: &Session{
			ID:           uuid.New().String(),
			UserID:       "",
			RefreshToken: "",
			Status:       SessionStatusActive,
			Type:         SessionTypeWeb,
			ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}
}

// WithID sets the ID.
func (b *SessionBuilder) WithID(id string) *SessionBuilder {
	b.session.ID = id
	return b
}

// WithUserID sets the user ID.
func (b *SessionBuilder) WithUserID(userID string) *SessionBuilder {
	b.session.UserID = userID
	return b
}

// WithRefreshToken sets the refresh token.
func (b *SessionBuilder) WithRefreshToken(token string) *SessionBuilder {
	b.session.RefreshToken = token
	return b
}

// WithStatus sets the status.
func (b *SessionBuilder) WithStatus(status SessionStatus) *SessionBuilder {
	b.session.Status = status
	return b
}

// WithType sets the session type.
func (b *SessionBuilder) WithType(sessionType SessionType) *SessionBuilder {
	b.session.Type = sessionType
	return b
}

// WithUserAgent sets the user agent.
func (b *SessionBuilder) WithUserAgent(userAgent string) *SessionBuilder {
	b.session.UserAgent = userAgent
	return b
}

// WithIP sets the IP.
func (b *SessionBuilder) WithIP(ip string) *SessionBuilder {
	b.session.IP = ip
	return b
}

// WithDeviceInfo sets device information.
func (b *SessionBuilder) WithDeviceInfo(deviceID, deviceName, os, browser string) *SessionBuilder {
	b.session.DeviceID = deviceID
	b.session.DeviceName = deviceName
	b.session.OS = os
	b.session.Browser = browser
	return b
}

// WithExpiresAt sets the expiry time.
func (b *SessionBuilder) WithExpiresAt(t time.Time) *SessionBuilder {
	b.session.ExpiresAt = t
	return b
}

// WithLastActive sets the last active timestamp.
func (b *SessionBuilder) WithLastActive(t time.Time) *SessionBuilder {
	b.session.LastActiveAt = &t
	return b
}

// WithMetadata sets metadata.
func (b *SessionBuilder) WithMetadata(metadata SessionMetadata) *SessionBuilder {
	b.session.Metadata = metadata
	return b
}

// WithCreatedAt sets the creation time.
func (b *SessionBuilder) WithCreatedAt(t time.Time) *SessionBuilder {
	b.session.CreatedAt = t
	b.session.UpdatedAt = t
	return b
}

// WithDeleted sets the deleted timestamp.
func (b *SessionBuilder) WithDeleted(t time.Time) *SessionBuilder {
	b.session.DeletedAt = &t
	return b
}

// Build validates and returns the session.
func (b *SessionBuilder) Build() (*Session, error) {
	if err := b.session.Validate(); err != nil {
		return nil, err
	}
	return b.session, nil
}

// MustBuild builds without error (panics on error).
func (b *SessionBuilder) MustBuild() *Session {
	s, err := b.Build()
	if err != nil {
		panic(err)
	}
	return s
}

// ======================================================================
// Test Helpers
// ======================================================================

var (
	TestSession1 = MustNewSession("user1", "refresh_token_1", time.Now().Add(7*24*time.Hour), SessionTypeWeb)
	TestSession2 = MustNewSession("user2", "refresh_token_2", time.Now().Add(30*24*time.Hour), SessionTypeMobile)
	TestSession3 = MustNewSession("user1", "refresh_token_3", time.Now().Add(24*time.Hour), SessionTypeAPI)
)

// MustNewSessionWithMetadata creates a session with metadata and panics.
func MustNewSessionWithMetadata(userID, refreshToken string, expiresAt time.Time, sessionType SessionType, metadata SessionMetadata) *Session {
	s, err := NewSessionWithMetadata(userID, refreshToken, expiresAt, sessionType, metadata)
	if err != nil {
		panic(err)
	}
	return s
}

// MustNewExpiredSession creates an expired session for testing.
func MustNewExpiredSession(userID, refreshToken string, sessionType SessionType) *Session {
	s := MustNewSession(userID, refreshToken, time.Now().Add(-1*time.Hour), sessionType)
	_ = s.Expire()
	return s
}

// MustNewRevokedSession creates a revoked session for testing.
func MustNewRevokedSession(userID, refreshToken string, sessionType SessionType) *Session {
	s := MustNewSession(userID, refreshToken, time.Now().Add(7*24*time.Hour), sessionType)
	_ = s.Revoke()
	return s
}

// MustNewDeletedSession creates a deleted session for testing.
func MustNewDeletedSession(userID, refreshToken string, sessionType SessionType) *Session {
	s := MustNewSession(userID, refreshToken, time.Now().Add(7*24*time.Hour), sessionType)
	_ = s.SoftDelete()
	return s
}