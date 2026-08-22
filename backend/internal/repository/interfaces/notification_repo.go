// backend/internal/repository/interfaces/notification_repo.go
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
	ErrNotificationNotFound    = errors.New("notification not found")
	ErrNotificationAlreadyRead = errors.New("notification already read")
	ErrInvalidNotificationID   = errors.New("invalid notification ID")
	ErrInvalidUserID           = errors.New("invalid user ID")
	ErrInvalidNotificationType = errors.New("invalid notification type")
	ErrNotificationExpired     = errors.New("notification has expired")
	ErrNotificationLimitExceeded = errors.New("notification limit exceeded")
)

// ======================================================================
// NotificationFilter
// ======================================================================

// NotificationFilter defines filtering options for notification queries.
type NotificationFilter struct {
	UserID      *string
	FromUserID  *string
	Type        *string
	Read        *bool
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	ReferenceID *string
}

// HasCriteria checks if any filter criteria are set.
func (f *NotificationFilter) HasCriteria() bool {
	return f.UserID != nil || f.FromUserID != nil || f.Type != nil ||
		f.Read != nil || f.CreatedFrom != nil || f.CreatedTo != nil ||
		f.ReferenceID != nil
}

// ======================================================================
// NotificationPagination
// ======================================================================

// NotificationSortField defines sortable fields for notifications.
type NotificationSortField string

const (
	SortNotificationByCreatedAt NotificationSortField = "created_at"
	SortNotificationByReadAt    NotificationSortField = "read_at"
)

// NotificationSortOrder defines sort order.
type NotificationSortOrder string

const (
	NotificationSortAsc  NotificationSortOrder = "ASC"
	NotificationSortDesc NotificationSortOrder = "DESC"
)

// NotificationPagination holds pagination options for notifications.
type NotificationPagination struct {
	Cursor string                 `json:"cursor"`
	Limit  int                    `json:"limit"`
	SortBy NotificationSortField  `json:"sort_by"`
	Order  NotificationSortOrder  `json:"order"`
}

// DefaultNotificationPagination returns default pagination options.
func DefaultNotificationPagination() *NotificationPagination {
	return &NotificationPagination{
		Limit:  20,
		SortBy: SortNotificationByCreatedAt,
		Order:  NotificationSortDesc,
	}
}

// Validate checks pagination parameters.
func (p *NotificationPagination) Validate() error {
	if p.Limit < 1 || p.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	return nil
}

// ======================================================================
// NotificationStats
// ======================================================================

// NotificationStats represents aggregated notification statistics.
type NotificationStats struct {
	Total           int64     `json:"total"`
	Unread          int64     `json:"unread"`
	Read            int64     `json:"read"`
	UniqueUsers     int64     `json:"unique_users"`
	UniqueSenders   int64     `json:"unique_senders"`
	LastNotification time.Time `json:"last_notification"`
	FirstNotification time.Time `json:"first_notification"`
	AveragePerUser  float64   `json:"average_per_user"`
	// By type
	TypeStats map[string]int64 `json:"type_stats"`
}

// ======================================================================
// DailyNotificationCount
// ======================================================================

// DailyNotificationCount represents daily notification counts.
type DailyNotificationCount struct {
	Date        time.Time `json:"date"`
	Total       int64     `json:"total"`
	Unread      int64     `json:"unread"`
	Read        int64     `json:"read"`
	UniqueUsers int64     `json:"unique_users"`
}

// ======================================================================
// GroupedNotification
// ======================================================================

// GroupedNotification represents a group of similar notifications.
type GroupedNotification struct {
	Type         string    `json:"type"`
	ReferenceID  string    `json:"reference_id"`
	Count        int64     `json:"count"`
	LatestAt     time.Time `json:"latest_at"`
	LatestID     string    `json:"latest_id"`
	ReadCount    int64     `json:"read_count"`
	UnreadCount  int64     `json:"unread_count"`
	FromUserIDs  []string  `json:"from_user_ids"`
}

// ======================================================================
// NotificationRepository Interface
// ======================================================================

// NotificationRepository defines the interface for notification data persistence.
type NotificationRepository interface {
	// --------------------------------------------------------------------
	// Basic CRUD
	// --------------------------------------------------------------------

	// Create creates a new notification.
	Create(ctx context.Context, notification *entities.Notification) error

	// GetByID retrieves a notification by its ID.
	GetByID(ctx context.Context, id string) (*entities.Notification, error)

	// GetByUserAndType retrieves notifications by user and type.
	GetByUserAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*entities.Notification, string, error)

	// GetByReferenceID retrieves notifications by reference ID.
	GetByReferenceID(ctx context.Context, referenceID string) ([]*entities.Notification, error)

	// Update updates a notification (e.g., read status).
	Update(ctx context.Context, notification *entities.Notification) error

	// Delete removes a notification.
	Delete(ctx context.Context, id string) error

	// DeleteByUserAndReference removes notifications by user and reference.
	DeleteByUserAndReference(ctx context.Context, userID, referenceID string) error

	// --------------------------------------------------------------------
	// Read Status Operations
	// --------------------------------------------------------------------

	// MarkAsRead marks a notification as read.
	MarkAsRead(ctx context.Context, id string) error

	// MarkAllAsRead marks all notifications for a user as read.
	MarkAllAsRead(ctx context.Context, userID string) error

	// MarkMultipleAsRead marks multiple notifications as read.
	MarkMultipleAsRead(ctx context.Context, ids []string) error

	// MarkAsUnread marks a notification as unread.
	MarkAsUnread(ctx context.Context, id string) error

	// MarkAllAsUnread marks all notifications for a user as unread.
	MarkAllAsUnread(ctx context.Context, userID string) error

	// --------------------------------------------------------------------
	// Existence Checks
	// --------------------------------------------------------------------

	// Exists checks if a notification exists.
	Exists(ctx context.Context, id string) (bool, error)

	// ExistsByUserAndReference checks if a notification exists for a user and reference.
	ExistsByUserAndReference(ctx context.Context, userID, referenceID string) (bool, error)

	// --------------------------------------------------------------------
	// Count Operations
	// --------------------------------------------------------------------

	// CountByUserID returns total notifications for a user.
	CountByUserID(ctx context.Context, userID string) (int64, error)

	// CountUnread returns total unread notifications for a user.
	CountUnread(ctx context.Context, userID string) (int64, error)

	// CountRead returns total read notifications for a user.
	CountRead(ctx context.Context, userID string) (int64, error)

	// CountByType returns notifications count by type for a user.
	CountByType(ctx context.Context, userID, notificationType string) (int64, error)

	// CountByDateRange returns notifications count within a date range.
	CountByDateRange(ctx context.Context, userID string, start, end time.Time) (int64, error)

	// CountByUserIDs returns notification counts for multiple users.
	CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error)

	// CountUnreadByUserIDs returns unread notification counts for multiple users.
	CountUnreadByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error)

	// --------------------------------------------------------------------
	// List Operations
	// --------------------------------------------------------------------

	// GetByUserID returns notifications for a user with pagination.
	GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Notification, string, error)

	// GetUnreadByUserID returns unread notifications for a user.
	GetUnreadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Notification, string, error)

	// GetReadByUserID returns read notifications for a user.
	GetReadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Notification, string, error)

	// GetByFromUserID returns notifications from a specific sender.
	GetByFromUserID(ctx context.Context, fromUserID string, cursor string, limit int) ([]*entities.Notification, string, error)

	// GetRecentByUserID returns recent notifications for a user.
	GetRecentByUserID(ctx context.Context, userID string, limit int) ([]*entities.Notification, error)

	// GetByUserIDAndType returns notifications by user and type.
	GetByUserIDAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*entities.Notification, string, error)

	// --------------------------------------------------------------------
	// Grouped Notifications
	// --------------------------------------------------------------------

	// GroupByType groups notifications by type.
	GroupByType(ctx context.Context, userID string, cursor string, limit int) ([]*GroupedNotification, string, error)

	// GroupByReference groups notifications by reference ID.
	GroupByReference(ctx context.Context, userID string, cursor string, limit int) ([]*GroupedNotification, string, error)

	// GroupByTypeAndReference groups notifications by type and reference.
	GroupByTypeAndReference(ctx context.Context, userID string, cursor string, limit int) ([]*GroupedNotification, string, error)

	// GetGroupedCount returns count of grouped notifications.
	GetGroupedCount(ctx context.Context, userID string) (int64, error)

	// --------------------------------------------------------------------
	// Advanced Queries
	// --------------------------------------------------------------------

	// GetNotificationsByDateRange returns notifications within a date range.
	GetNotificationsByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Notification, string, error)

	// GetUnreadByDateRange returns unread notifications within a date range.
	GetUnreadByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Notification, string, error)

	// GetNotificationsByReferenceIDAndType returns notifications by reference and type.
	GetNotificationsByReferenceIDAndType(ctx context.Context, referenceID, notificationType string) ([]*entities.Notification, error)

	// GetNotificationsByMultipleReferences returns notifications for multiple references.
	GetNotificationsByMultipleReferences(ctx context.Context, referenceIDs []string) ([]*entities.Notification, error)

	// --------------------------------------------------------------------
	// Bulk Operations
	// --------------------------------------------------------------------

	// BulkCreate inserts multiple notifications in a single transaction.
	BulkCreate(ctx context.Context, notifications []*entities.Notification) error

	// BulkDelete removes multiple notifications in a single transaction.
	BulkDelete(ctx context.Context, ids []string) error

	// BulkDeleteByUserID removes all notifications for a user.
	BulkDeleteByUserID(ctx context.Context, userID string) error

	// BulkDeleteByType removes notifications by type for a user.
	BulkDeleteByType(ctx context.Context, userID, notificationType string) error

	// BulkDeleteByReference removes notifications by reference ID.
	BulkDeleteByReference(ctx context.Context, referenceID string) error

	// BulkDeleteOlderThan removes notifications older than a date.
	BulkDeleteOlderThan(ctx context.Context, userID string, before time.Time) error

	// BulkDeleteOlderThanAll removes all notifications older than a date.
	BulkDeleteOlderThanAll(ctx context.Context, before time.Time) error

	// BulkMarkAsRead marks multiple notifications as read.
	BulkMarkAsRead(ctx context.Context, ids []string) error

	// BulkMarkAsUnread marks multiple notifications as unread.
	BulkMarkAsUnread(ctx context.Context, ids []string) error

	// --------------------------------------------------------------------
	// Stats and Analytics
	// --------------------------------------------------------------------

	// GetNotificationStats returns aggregated notification statistics.
	GetNotificationStats(ctx context.Context) (*NotificationStats, error)

	// GetUserNotificationStats returns notification stats for a specific user.
	GetUserNotificationStats(ctx context.Context, userID string) (*NotificationStats, error)

	// GetDailyNotificationStats returns daily notification counts for a date range.
	GetDailyNotificationStats(ctx context.Context, start, end time.Time) ([]*DailyNotificationCount, error)

	// GetDailyNotificationStatsForUser returns daily notification counts for a user.
	GetDailyNotificationStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyNotificationCount, error)

	// GetNotificationTypeStats returns notification stats by type.
	GetNotificationTypeStats(ctx context.Context, userID string) ([]*NotificationTypeStat, error)

	// GetNotificationTrends returns notification trends over time.
	GetNotificationTrends(ctx context.Context, userID string, days int) ([]*TrendData, error)

	// GetAverageResponseTime calculates average time between notification and read.
	GetAverageResponseTime(ctx context.Context, userID string) (float64, error)

	// --------------------------------------------------------------------
	// Transaction Support
	// --------------------------------------------------------------------

	// WithTransaction returns a new repository using the given transaction.
	WithTransaction(ctx context.Context, tx *sql.Tx) NotificationRepository

	// Transaction executes a function within a database transaction.
	Transaction(ctx context.Context, fn func(txRepo NotificationRepository) error) error

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

// NotificationTypeStat represents notification statistics by type.
type NotificationTypeStat struct {
	Type        string    `json:"type"`
	Count       int64     `json:"count"`
	UnreadCount int64     `json:"unread_count"`
	ReadCount   int64     `json:"read_count"`
	LatestAt    time.Time `json:"latest_at"`
}

// TrendData represents trend data over time.
type TrendData struct {
	Date  time.Time `json:"date"`
	Value int64     `json:"value"`
}

// ======================================================================
// Helper Functions
// ======================================================================

// IsNotificationNotFound checks if an error indicates a notification was not found.
func IsNotificationNotFound(err error) bool {
	return errors.Is(err, ErrNotificationNotFound)
}

// IsNotificationAlreadyRead checks if an error indicates already read.
func IsNotificationAlreadyRead(err error) bool {
	return errors.Is(err, ErrNotificationAlreadyRead)
}

// IsNotificationError checks if an error is notification-related.
func IsNotificationError(err error) bool {
	return errors.Is(err, ErrNotificationNotFound) ||
		errors.Is(err, ErrNotificationAlreadyRead) ||
		errors.Is(err, ErrInvalidNotificationID) ||
		errors.Is(err, ErrInvalidUserID) ||
		errors.Is(err, ErrInvalidNotificationType)
}

// ======================================================================
// Mock Notification Repository (for testing)
// ======================================================================

// MockNotificationRepository is a mock implementation for testing.
type MockNotificationRepository struct {
	Notifications map[string]*entities.Notification
	UserNotifications map[string][]string // userID -> notification IDs
	Error        error
	NextCursor   string
}

// NewMockNotificationRepo creates a new mock repository.
func NewMockNotificationRepo() NotificationRepository {
	return &MockNotificationRepository{
		Notifications: make(map[string]*entities.Notification),
		UserNotifications: make(map[string][]string),
	}
}

// Create mock implementation.
func (m *MockNotificationRepository) Create(ctx context.Context, notification *entities.Notification) error {
	if m.Error != nil {
		return m.Error
	}
	m.Notifications[notification.ID] = notification
	if m.UserNotifications[notification.UserID] == nil {
		m.UserNotifications[notification.UserID] = []string{}
	}
	m.UserNotifications[notification.UserID] = append(m.UserNotifications[notification.UserID], notification.ID)
	return nil
}

// GetByID mock implementation.
func (m *MockNotificationRepository) GetByID(ctx context.Context, id string) (*entities.Notification, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if notification, ok := m.Notifications[id]; ok {
		return notification, nil
	}
	return nil, ErrNotificationNotFound
}

// GetByUserAndType mock implementation.
func (m *MockNotificationRepository) GetByUserAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var notifications []*entities.Notification
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && n.Type == notificationType {
			notifications = append(notifications, n)
		}
	}
	return notifications, "", nil
}

// GetByReferenceID mock implementation.
func (m *MockNotificationRepository) GetByReferenceID(ctx context.Context, referenceID string) ([]*entities.Notification, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var notifications []*entities.Notification
	for _, n := range m.Notifications {
		if n.ReferenceID == referenceID {
			notifications = append(notifications, n)
		}
	}
	return notifications, nil
}

// Update mock implementation.
func (m *MockNotificationRepository) Update(ctx context.Context, notification *entities.Notification) error {
	if m.Error != nil {
		return m.Error
	}
	if _, ok := m.Notifications[notification.ID]; !ok {
		return ErrNotificationNotFound
	}
	m.Notifications[notification.ID] = notification
	return nil
}

// Delete mock implementation.
func (m *MockNotificationRepository) Delete(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if notification, ok := m.Notifications[id]; ok {
		delete(m.Notifications, id)
		// Remove from user list
		if userNotifs, ok := m.UserNotifications[notification.UserID]; ok {
			for i, nid := range userNotifs {
				if nid == id {
					m.UserNotifications[notification.UserID] = append(userNotifs[:i], userNotifs[i+1:]...)
					break
				}
			}
		}
		return nil
	}
	return ErrNotificationNotFound
}

// DeleteByUserAndReference mock implementation.
func (m *MockNotificationRepository) DeleteByUserAndReference(ctx context.Context, userID, referenceID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, n := range m.Notifications {
		if n.UserID == userID && n.ReferenceID == referenceID {
			delete(m.Notifications, id)
			if userNotifs, ok := m.UserNotifications[userID]; ok {
				for i, nid := range userNotifs {
					if nid == id {
						m.UserNotifications[userID] = append(userNotifs[:i], userNotifs[i+1:]...)
						break
					}
				}
			}
		}
	}
	return nil
}

// MarkAsRead mock implementation.
func (m *MockNotificationRepository) MarkAsRead(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if notification, ok := m.Notifications[id]; ok {
		notification.Read = true
		now := time.Now()
		notification.ReadAt = &now
		return nil
	}
	return ErrNotificationNotFound
}

// MarkAllAsRead mock implementation.
func (m *MockNotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok {
			n.Read = true
			now := time.Now()
			n.ReadAt = &now
		}
	}
	return nil
}

// MarkMultipleAsRead mock implementation.
func (m *MockNotificationRepository) MarkMultipleAsRead(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.MarkAsRead(ctx, id)
	}
	return nil
}

// MarkAsUnread mock implementation.
func (m *MockNotificationRepository) MarkAsUnread(ctx context.Context, id string) error {
	if m.Error != nil {
		return m.Error
	}
	if notification, ok := m.Notifications[id]; ok {
		notification.Read = false
		notification.ReadAt = nil
		return nil
	}
	return ErrNotificationNotFound
}

// MarkAllAsUnread mock implementation.
func (m *MockNotificationRepository) MarkAllAsUnread(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok {
			n.Read = false
			n.ReadAt = nil
		}
	}
	return nil
}

// Exists mock implementation.
func (m *MockNotificationRepository) Exists(ctx context.Context, id string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	_, ok := m.Notifications[id]
	return ok, nil
}

// ExistsByUserAndReference mock implementation.
func (m *MockNotificationRepository) ExistsByUserAndReference(ctx context.Context, userID, referenceID string) (bool, error) {
	if m.Error != nil {
		return false, m.Error
	}
	for _, n := range m.Notifications {
		if n.UserID == userID && n.ReferenceID == referenceID {
			return true, nil
		}
	}
	return false, nil
}

// CountByUserID mock implementation.
func (m *MockNotificationRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if userNotifs, ok := m.UserNotifications[userID]; ok {
		return int64(len(userNotifs)), nil
	}
	return 0, nil
}

// CountUnread mock implementation.
func (m *MockNotificationRepository) CountUnread(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && !n.Read {
			count++
		}
	}
	return count, nil
}

// CountRead mock implementation.
func (m *MockNotificationRepository) CountRead(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && n.Read {
			count++
		}
	}
	return count, nil
}

// CountByType mock implementation.
func (m *MockNotificationRepository) CountByType(ctx context.Context, userID, notificationType string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && n.Type == notificationType {
			count++
		}
	}
	return count, nil
}

// CountByDateRange mock implementation.
func (m *MockNotificationRepository) CountByDateRange(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count := int64(0)
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && n.CreatedAt.After(start) && n.CreatedAt.Before(end) {
			count++
		}
	}
	return count, nil
}

// CountByUserIDs mock implementation.
func (m *MockNotificationRepository) CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, uid := range userIDs {
		count, _ := m.CountByUserID(ctx, uid)
		result[uid] = count
	}
	return result, nil
}

// CountUnreadByUserIDs mock implementation.
func (m *MockNotificationRepository) CountUnreadByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	result := make(map[string]int64)
	for _, uid := range userIDs {
		count, _ := m.CountUnread(ctx, uid)
		result[uid] = count
	}
	return result, nil
}

// GetByUserID mock implementation.
func (m *MockNotificationRepository) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var notifications []*entities.Notification
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok {
			notifications = append(notifications, n)
		}
	}
	return notifications, "", nil
}

// GetUnreadByUserID mock implementation.
func (m *MockNotificationRepository) GetUnreadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var notifications []*entities.Notification
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && !n.Read {
			notifications = append(notifications, n)
		}
	}
	return notifications, "", nil
}

// GetReadByUserID mock implementation.
func (m *MockNotificationRepository) GetReadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var notifications []*entities.Notification
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && n.Read {
			notifications = append(notifications, n)
		}
	}
	return notifications, "", nil
}

// GetByFromUserID mock implementation.
func (m *MockNotificationRepository) GetByFromUserID(ctx context.Context, fromUserID string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var notifications []*entities.Notification
	for _, n := range m.Notifications {
		if n.FromUserID == fromUserID {
			notifications = append(notifications, n)
		}
	}
	return notifications, "", nil
}

// GetRecentByUserID mock implementation.
func (m *MockNotificationRepository) GetRecentByUserID(ctx context.Context, userID string, limit int) ([]*entities.Notification, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var notifications []*entities.Notification
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok {
			notifications = append(notifications, n)
		}
	}
	return notifications, nil
}

// GetByUserIDAndType mock implementation.
func (m *MockNotificationRepository) GetByUserIDAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*entities.Notification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var notifications []*entities.Notification
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && n.Type == notificationType {
			notifications = append(notifications, n)
		}
	}
	return notifications, "", nil
}

// GroupByType mock implementation.
func (m *MockNotificationRepository) GroupByType(ctx context.Context, userID string, cursor string, limit int) ([]*GroupedNotification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*GroupedNotification{}, "", nil
}

// GroupByReference mock implementation.
func (m *MockNotificationRepository) GroupByReference(ctx context.Context, userID string, cursor string, limit int) ([]*GroupedNotification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*GroupedNotification{}, "", nil
}

// GroupByTypeAndReference mock implementation.
func (m *MockNotificationRepository) GroupByTypeAndReference(ctx context.Context, userID string, cursor string, limit int) ([]*GroupedNotification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	return []*GroupedNotification{}, "", nil
}

// GetGroupedCount mock implementation.
func (m *MockNotificationRepository) GetGroupedCount(ctx context.Context, userID string) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	count, _ := m.CountByUserID(ctx, userID)
	return count, nil
}

// GetNotificationsByDateRange mock implementation.
func (m *MockNotificationRepository) GetNotificationsByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Notification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var notifications []*entities.Notification
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && n.CreatedAt.After(start) && n.CreatedAt.Before(end) {
			notifications = append(notifications, n)
		}
	}
	return notifications, "", nil
}

// GetUnreadByDateRange mock implementation.
func (m *MockNotificationRepository) GetUnreadByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*entities.Notification, string, error) {
	if m.Error != nil {
		return nil, "", m.Error
	}
	var notifications []*entities.Notification
	for _, id := range m.UserNotifications[userID] {
		if n, ok := m.Notifications[id]; ok && !n.Read && n.CreatedAt.After(start) && n.CreatedAt.Before(end) {
			notifications = append(notifications, n)
		}
	}
	return notifications, "", nil
}

// GetNotificationsByReferenceIDAndType mock implementation.
func (m *MockNotificationRepository) GetNotificationsByReferenceIDAndType(ctx context.Context, referenceID, notificationType string) ([]*entities.Notification, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var notifications []*entities.Notification
	for _, n := range m.Notifications {
		if n.ReferenceID == referenceID && n.Type == notificationType {
			notifications = append(notifications, n)
		}
	}
	return notifications, nil
}

// GetNotificationsByMultipleReferences mock implementation.
func (m *MockNotificationRepository) GetNotificationsByMultipleReferences(ctx context.Context, referenceIDs []string) ([]*entities.Notification, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	var notifications []*entities.Notification
	for _, n := range m.Notifications {
		for _, ref := range referenceIDs {
			if n.ReferenceID == ref {
				notifications = append(notifications, n)
				break
			}
		}
	}
	return notifications, nil
}

// BulkCreate mock implementation.
func (m *MockNotificationRepository) BulkCreate(ctx context.Context, notifications []*entities.Notification) error {
	if m.Error != nil {
		return m.Error
	}
	for _, n := range notifications {
		_ = m.Create(ctx, n)
	}
	return nil
}

// BulkDelete mock implementation.
func (m *MockNotificationRepository) BulkDelete(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.Delete(ctx, id)
	}
	return nil
}

// BulkDeleteByUserID mock implementation.
func (m *MockNotificationRepository) BulkDeleteByUserID(ctx context.Context, userID string) error {
	if m.Error != nil {
		return m.Error
	}
	if userNotifs, ok := m.UserNotifications[userID]; ok {
		for _, id := range userNotifs {
			delete(m.Notifications, id)
		}
		delete(m.UserNotifications, userID)
	}
	return nil
}

// BulkDeleteByType mock implementation.
func (m *MockNotificationRepository) BulkDeleteByType(ctx context.Context, userID, notificationType string) error {
	if m.Error != nil {
		return m.Error
	}
	if userNotifs, ok := m.UserNotifications[userID]; ok {
		var remaining []string
		for _, id := range userNotifs {
			if n, ok := m.Notifications[id]; ok && n.Type == notificationType {
				delete(m.Notifications, id)
			} else {
				remaining = append(remaining, id)
			}
		}
		m.UserNotifications[userID] = remaining
	}
	return nil
}

// BulkDeleteByReference mock implementation.
func (m *MockNotificationRepository) BulkDeleteByReference(ctx context.Context, referenceID string) error {
	if m.Error != nil {
		return m.Error
	}
	for id, n := range m.Notifications {
		if n.ReferenceID == referenceID {
			delete(m.Notifications, id)
			if userNotifs, ok := m.UserNotifications[n.UserID]; ok {
				var remaining []string
				for _, nid := range userNotifs {
					if nid != id {
						remaining = append(remaining, nid)
					}
				}
				m.UserNotifications[n.UserID] = remaining
			}
		}
	}
	return nil
}

// BulkDeleteOlderThan mock implementation.
func (m *MockNotificationRepository) BulkDeleteOlderThan(ctx context.Context, userID string, before time.Time) error {
	if m.Error != nil {
		return m.Error
	}
	if userNotifs, ok := m.UserNotifications[userID]; ok {
		var remaining []string
		for _, id := range userNotifs {
			if n, ok := m.Notifications[id]; ok && n.CreatedAt.Before(before) {
				delete(m.Notifications, id)
			} else {
				remaining = append(remaining, id)
			}
		}
		m.UserNotifications[userID] = remaining
	}
	return nil
}

// BulkDeleteOlderThanAll mock implementation.
func (m *MockNotificationRepository) BulkDeleteOlderThanAll(ctx context.Context, before time.Time) error {
	if m.Error != nil {
		return m.Error
	}
	for id, n := range m.Notifications {
		if n.CreatedAt.Before(before) {
			delete(m.Notifications, id)
		}
	}
	return nil
}

// BulkMarkAsRead mock implementation.
func (m *MockNotificationRepository) BulkMarkAsRead(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.MarkAsRead(ctx, id)
	}
	return nil
}

// BulkMarkAsUnread mock implementation.
func (m *MockNotificationRepository) BulkMarkAsUnread(ctx context.Context, ids []string) error {
	if m.Error != nil {
		return m.Error
	}
	for _, id := range ids {
		_ = m.MarkAsUnread(ctx, id)
	}
	return nil
}

// GetNotificationStats mock implementation.
func (m *MockNotificationRepository) GetNotificationStats(ctx context.Context) (*NotificationStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &NotificationStats{
		Total: int64(len(m.Notifications)),
	}, nil
}

// GetUserNotificationStats mock implementation.
func (m *MockNotificationRepository) GetUserNotificationStats(ctx context.Context, userID string) (*NotificationStats, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	count, _ := m.CountByUserID(ctx, userID)
	unread, _ := m.CountUnread(ctx, userID)
	return &NotificationStats{
		Total:  count,
		Unread: unread,
		Read:   count - unread,
	}, nil
}

// GetDailyNotificationStats mock implementation.
func (m *MockNotificationRepository) GetDailyNotificationStats(ctx context.Context, start, end time.Time) ([]*DailyNotificationCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyNotificationCount{}, nil
}

// GetDailyNotificationStatsForUser mock implementation.
func (m *MockNotificationRepository) GetDailyNotificationStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*DailyNotificationCount, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*DailyNotificationCount{}, nil
}

// GetNotificationTypeStats mock implementation.
func (m *MockNotificationRepository) GetNotificationTypeStats(ctx context.Context, userID string) ([]*NotificationTypeStat, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*NotificationTypeStat{}, nil
}

// GetNotificationTrends mock implementation.
func (m *MockNotificationRepository) GetNotificationTrends(ctx context.Context, userID string, days int) ([]*TrendData, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return []*TrendData{}, nil
}

// GetAverageResponseTime mock implementation.
func (m *MockNotificationRepository) GetAverageResponseTime(ctx context.Context, userID string) (float64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	return 0.0, nil
}

// WithTransaction mock implementation.
func (m *MockNotificationRepository) WithTransaction(ctx context.Context, tx *sql.Tx) NotificationRepository {
	return m
}

// Transaction mock implementation.
func (m *MockNotificationRepository) Transaction(ctx context.Context, fn func(txRepo NotificationRepository) error) error {
	if m.Error != nil {
		return m.Error
	}
	return fn(m)
}

// Ping mock implementation.
func (m *MockNotificationRepository) Ping(ctx context.Context) error {
	return m.Error
}

// Close mock implementation.
func (m *MockNotificationRepository) Close() error {
	return nil
}

// GetRawDB mock implementation.
func (m *MockNotificationRepository) GetRawDB() interface{} {
	return nil
}