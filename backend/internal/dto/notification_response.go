// backend/internal/dto/notification_response.go
package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ======================================================================
// Validation Constants
// ======================================================================

const (
	MaxNotificationLimit     = 100
	DefaultNotificationLimit = 20
	MaxGroupLimit            = 50
	DefaultGroupLimit        = 10
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrNotificationIDRequired = errors.New("notification ID is required")
	ErrUserIDRequired         = errors.New("user ID is required")
	ErrInvalidNotificationType = errors.New("invalid notification type")
	ErrInvalidLimit           = errors.New("limit must be between 1 and 100")
	ErrInvalidCursor          = errors.New("invalid cursor format")
	ErrNotificationNotFound   = errors.New("notification not found")
	ErrNotificationsRequired  = errors.New("at least one notification ID is required")
	ErrNotificationAlreadyRead = errors.New("notification already read")
)

// ======================================================================
// Notification Types
// ======================================================================

// NotificationType represents the type of notification.
type NotificationType string

const (
	NotifTypeLike      NotificationType = "like"
	NotifTypeRetweet   NotificationType = "retweet"
	NotifTypeFollow    NotificationType = "follow"
	NotifTypeReply     NotificationType = "reply"
	NotifTypeMention   NotificationType = "mention"
	NotifTypeQuote     NotificationType = "quote"
	NotifTypeMessage   NotificationType = "message"
	NotifTypeJoin      NotificationType = "join"
	NotifTypeLeave     NotificationType = "leave"
	NotifTypeCommunity NotificationType = "community"
	NotifTypeSystem    NotificationType = "system"
)

// ValidNotificationTypes returns all valid notification types.
func ValidNotificationTypes() []NotificationType {
	return []NotificationType{
		NotifTypeLike,
		NotifTypeRetweet,
		NotifTypeFollow,
		NotifTypeReply,
		NotifTypeMention,
		NotifTypeQuote,
		NotifTypeMessage,
		NotifTypeJoin,
		NotifTypeLeave,
		NotifTypeCommunity,
		NotifTypeSystem,
	}
}

// IsValid checks if a notification type is valid.
func (t NotificationType) IsValid() bool {
	for _, typ := range ValidNotificationTypes() {
		if t == typ {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (t NotificationType) String() string {
	return string(t)
}

// GetDisplayName returns a human-readable name for the type.
func (t NotificationType) GetDisplayName() string {
	names := map[NotificationType]string{
		NotifTypeLike:      "Liked your tweet",
		NotifTypeRetweet:   "Retweeted your tweet",
		NotifTypeFollow:    "Followed you",
		NotifTypeReply:     "Replied to your tweet",
		NotifTypeMention:   "Mentioned you",
		NotifTypeQuote:     "Quoted your tweet",
		NotifTypeMessage:   "Sent you a message",
		NotifTypeJoin:      "Joined the community",
		NotifTypeLeave:     "Left the community",
		NotifTypeCommunity: "Community update",
		NotifTypeSystem:    "System notification",
	}
	if name, ok := names[t]; ok {
		return name
	}
	return string(t)
}

// GetPriority returns the priority level of the notification type.
func (t NotificationType) GetPriority() int {
	priorities := map[NotificationType]int{
		NotifTypeMessage:   3,
		NotifTypeFollow:    2,
		NotifTypeReply:     2,
		NotifTypeMention:   2,
		NotifTypeLike:      1,
		NotifTypeRetweet:   1,
		NotifTypeQuote:     1,
		NotifTypeCommunity: 1,
		NotifTypeSystem:    2,
		NotifTypeJoin:      0,
		NotifTypeLeave:     0,
	}
	if p, ok := priorities[t]; ok {
		return p
	}
	return 0
}

// ======================================================================
// Request DTOs
// ======================================================================

// GetNotificationsRequest represents the request to list notifications.
type GetNotificationsRequest struct {
	UserID     string `json:"user_id,omitempty"`
	Type       string `json:"type,omitempty"`
	ReadStatus string `json:"read_status,omitempty"` // "all", "read", "unread"
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	SortOrder  string `json:"sort_order,omitempty"`
	FromDate   string `json:"from_date,omitempty"`
	ToDate     string `json:"to_date,omitempty"`
}

// Validate validates the get notifications request.
func (r *GetNotificationsRequest) Validate() error {
	if r.Type != "" && !NotificationType(r.Type).IsValid() {
		return ErrInvalidNotificationType
	}
	if r.ReadStatus != "" && r.ReadStatus != "all" && r.ReadStatus != "read" && r.ReadStatus != "unread" {
		return errors.New("invalid read status")
	}
	if r.Limit < 0 || r.Limit > MaxNotificationLimit {
		return ErrInvalidLimit
	}
	if r.SortBy != "" {
		allowed := map[string]bool{
			"created_at": true, "read_at": true, "type": true,
		}
		if !allowed[r.SortBy] {
			return errors.New("invalid sort field")
		}
	}
	if r.SortOrder != "" && r.SortOrder != "asc" && r.SortOrder != "desc" {
		return errors.New("invalid sort order")
	}
	if r.FromDate != "" && r.ToDate != "" {
		from, err := time.Parse(time.RFC3339, r.FromDate)
		if err != nil {
			return errors.New("invalid from_date format")
		}
		to, err := time.Parse(time.RFC3339, r.ToDate)
		if err != nil {
			return errors.New("invalid to_date format")
		}
		if from.After(to) {
			return errors.New("from_date must be before to_date")
		}
	}
	return nil
}

// Sanitize sanitizes the get notifications request.
func (r *GetNotificationsRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Type = strings.TrimSpace(r.Type)
	r.ReadStatus = strings.TrimSpace(r.ReadStatus)
	r.Cursor = strings.TrimSpace(r.Cursor)
	r.FromDate = strings.TrimSpace(r.FromDate)
	r.ToDate = strings.TrimSpace(r.ToDate)
	if r.Limit < 1 {
		r.Limit = DefaultNotificationLimit
	}
	if r.Limit > MaxNotificationLimit {
		r.Limit = MaxNotificationLimit
	}
	if r.SortBy != "" {
		r.SortBy = strings.ToLower(strings.TrimSpace(r.SortBy))
	}
	if r.SortOrder != "" {
		r.SortOrder = strings.ToLower(strings.TrimSpace(r.SortOrder))
	}
}

// MarkNotificationReadRequest represents the request to mark notifications as read.
type MarkNotificationReadRequest struct {
	NotificationIDs []string `json:"notification_ids,omitempty"`
	All             bool     `json:"all,omitempty"`
	UserID          string   `json:"user_id,omitempty"`
	Type            string   `json:"type,omitempty"` // mark all of a specific type as read
}

// Validate validates the mark read request.
func (r *MarkNotificationReadRequest) Validate() error {
	if len(r.NotificationIDs) == 0 && !r.All && r.Type == "" {
		return errors.New("either notification_ids, all, or type must be provided")
	}
	if r.All && len(r.NotificationIDs) > 0 {
		return errors.New("cannot specify both all and notification_ids")
	}
	if r.Type != "" && !NotificationType(r.Type).IsValid() {
		return ErrInvalidNotificationType
	}
	for _, id := range r.NotificationIDs {
		if strings.TrimSpace(id) == "" {
			return ErrNotificationIDRequired
		}
	}
	return nil
}

// Sanitize sanitizes the mark read request.
func (r *MarkNotificationReadRequest) Sanitize() {
	cleaned := make([]string, 0, len(r.NotificationIDs))
	for _, id := range r.NotificationIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.NotificationIDs = cleaned
	r.UserID = strings.TrimSpace(r.UserID)
	r.Type = strings.TrimSpace(r.Type)
}

// DeleteNotificationRequest represents the request to delete notifications.
type DeleteNotificationRequest struct {
	NotificationIDs []string `json:"notification_ids,omitempty"`
	All             bool     `json:"all,omitempty"`
	UserID          string   `json:"user_id,omitempty"`
	Type            string   `json:"type,omitempty"`
	BeforeDate      string   `json:"before_date,omitempty"`
}

// Validate validates the delete notifications request.
func (r *DeleteNotificationRequest) Validate() error {
	if len(r.NotificationIDs) == 0 && !r.All && r.Type == "" && r.BeforeDate == "" {
		return errors.New("at least one filter must be provided")
	}
	if r.All && len(r.NotificationIDs) > 0 {
		return errors.New("cannot specify both all and notification_ids")
	}
	if r.Type != "" && !NotificationType(r.Type).IsValid() {
		return ErrInvalidNotificationType
	}
	if r.BeforeDate != "" {
		_, err := time.Parse(time.RFC3339, r.BeforeDate)
		if err != nil {
			return errors.New("invalid before_date format")
		}
	}
	for _, id := range r.NotificationIDs {
		if strings.TrimSpace(id) == "" {
			return ErrNotificationIDRequired
		}
	}
	return nil
}

// Sanitize sanitizes the delete notifications request.
func (r *DeleteNotificationRequest) Sanitize() {
	cleaned := make([]string, 0, len(r.NotificationIDs))
	for _, id := range r.NotificationIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.NotificationIDs = cleaned
	r.UserID = strings.TrimSpace(r.UserID)
	r.Type = strings.TrimSpace(r.Type)
	r.BeforeDate = strings.TrimSpace(r.BeforeDate)
}

// GetGroupedNotificationsRequest represents the request to get grouped notifications.
type GetGroupedNotificationsRequest struct {
	UserID   string `json:"user_id,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
	Types    []string `json:"types,omitempty"`
}

// Validate validates the get grouped notifications request.
func (r *GetGroupedNotificationsRequest) Validate() error {
	if r.Limit < 0 || r.Limit > MaxGroupLimit {
		return errors.New("limit must be between 1 and 50")
	}
	for _, t := range r.Types {
		if !NotificationType(t).IsValid() {
			return ErrInvalidNotificationType
		}
	}
	return nil
}

// Sanitize sanitizes the get grouped notifications request.
func (r *GetGroupedNotificationsRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultGroupLimit
	}
	if r.Limit > MaxGroupLimit {
		r.Limit = MaxGroupLimit
	}
	cleaned := make([]string, 0, len(r.Types))
	for _, t := range r.Types {
		if trimmed := strings.TrimSpace(t); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.Types = cleaned
}

// ======================================================================
// Response DTOs
// ======================================================================

// NotificationResponse represents a notification in responses.
type NotificationResponse struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	FromUserID  string                 `json:"from_user_id"`
	Type        string                 `json:"type"`
	TypeDisplay string                 `json:"type_display"`
	ReferenceID string                 `json:"reference_id,omitempty"`
	Read        bool                   `json:"read"`
	ReadAt      *time.Time             `json:"read_at,omitempty"`
	Priority    int                    `json:"priority"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	FromUser    *MinimalUserResponse   `json:"from_user,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// NotificationListResponse represents a paginated list of notifications.
type NotificationListResponse struct {
	Data       []NotificationResponse `json:"data"`
	Total      int64                  `json:"total"`
	UnreadCount int64                 `json:"unread_count"`
	NextCursor string                 `json:"next_cursor"`
	HasMore    bool                   `json:"has_more"`
	Limit      int                    `json:"limit"`
}

// GroupedNotificationResponse represents a group of similar notifications.
type GroupedNotificationResponse struct {
	Type         string    `json:"type"`
	TypeDisplay  string    `json:"type_display"`
	ReferenceID  string    `json:"reference_id,omitempty"`
	Count        int64     `json:"count"`
	LatestAt     time.Time `json:"latest_at"`
	FromUserIDs  []string  `json:"from_user_ids,omitempty"`
	FromUsers    []MinimalUserResponse `json:"from_users,omitempty"`
	IsRead       bool      `json:"is_read"`
	ReadCount    int64     `json:"read_count"`
	UnreadCount  int64     `json:"unread_count"`
	LatestID     string    `json:"latest_id"`
}

// GroupedNotificationListResponse represents a paginated list of grouped notifications.
type GroupedNotificationListResponse struct {
	Data       []GroupedNotificationResponse `json:"data"`
	Total      int64                         `json:"total"`
	NextCursor string                        `json:"next_cursor"`
	HasMore    bool                          `json:"has_more"`
	Limit      int                           `json:"limit"`
}

// NotificationStatsResponse represents notification statistics.
type NotificationStatsResponse struct {
	Total        int64            `json:"total"`
	Unread       int64            `json:"unread"`
	Read         int64            `json:"read"`
	UniqueUsers  int64            `json:"unique_users"`
	UniqueSenders int64           `json:"unique_senders"`
	Latest       time.Time        `json:"latest"`
	Earliest     time.Time        `json:"earliest"`
	TypeStats    map[string]int64 `json:"type_stats"`
	DailyStats   []DailyNotificationCount `json:"daily_stats,omitempty"`
}

// DailyNotificationCount represents daily notification counts.
type DailyNotificationCount struct {
	Date        time.Time `json:"date"`
	Total       int64     `json:"total"`
	Unread      int64     `json:"unread"`
	Read        int64     `json:"read"`
	UniqueUsers int64     `json:"unique_users"`
}

// UserNotificationStatsResponse represents notification stats for a user.
type UserNotificationStatsResponse struct {
	UserID     string                     `json:"user_id"`
	Total      int64                      `json:"total"`
	Unread     int64                      `json:"unread"`
	Read       int64                      `json:"read"`
	TypeStats  []NotificationTypeStat     `json:"type_stats"`
	LatestAt   time.Time                  `json:"latest_at"`
}

// NotificationTypeStat represents notification statistics by type.
type NotificationTypeStat struct {
	Type        string    `json:"type"`
	TypeDisplay string    `json:"type_display"`
	Count       int64     `json:"count"`
	UnreadCount int64     `json:"unread_count"`
	ReadCount   int64     `json:"read_count"`
	LatestAt    time.Time `json:"latest_at"`
}

// UnreadCountResponse represents unread notification count.
type UnreadCountResponse struct {
	UserID string `json:"user_id"`
	Count  int64  `json:"count"`
}

// MarkReadResponse represents the response after marking notifications as read.
type MarkReadResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	MarkedCount  int64  `json:"marked_count"`
	UnreadRemaining int64 `json:"unread_remaining"`
}

// ======================================================================
// Builder Methods for NotificationResponse
// ======================================================================

// NewNotificationResponse creates a new notification response.
func NewNotificationResponse(id, userID, fromUserID, notifType, referenceID string, read bool) *NotificationResponse {
	typ := NotificationType(notifType)
	return &NotificationResponse{
		ID:          id,
		UserID:      userID,
		FromUserID:  fromUserID,
		Type:        notifType,
		TypeDisplay: typ.GetDisplayName(),
		ReferenceID: referenceID,
		Read:        read,
		Priority:    typ.GetPriority(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

// WithReadAt sets the read at time.
func (r *NotificationResponse) WithReadAt(t time.Time) *NotificationResponse {
	r.ReadAt = &t
	return r
}

// WithMetadata sets the metadata.
func (r *NotificationResponse) WithMetadata(metadata map[string]interface{}) *NotificationResponse {
	r.Metadata = metadata
	return r
}

// WithFromUser sets the from user.
func (r *NotificationResponse) WithFromUser(user *MinimalUserResponse) *NotificationResponse {
	r.FromUser = user
	return r
}

// WithCreatedAt sets the created at time.
func (r *NotificationResponse) WithCreatedAt(t time.Time) *NotificationResponse {
	r.CreatedAt = t
	return r
}

// WithUpdatedAt sets the updated at time.
func (r *NotificationResponse) WithUpdatedAt(t time.Time) *NotificationResponse {
	r.UpdatedAt = t
	return r
}

// ======================================================================
// Builder Methods for NotificationListResponse
// ======================================================================

// NewNotificationListResponse creates a new notification list response.
func NewNotificationListResponse() *NotificationListResponse {
	return &NotificationListResponse{
		Data:  []NotificationResponse{},
		Total: 0,
	}
}

// Add adds a notification to the response.
func (r *NotificationListResponse) Add(notification NotificationResponse) {
	r.Data = append(r.Data, notification)
}

// WithTotal sets the total count.
func (r *NotificationListResponse) WithTotal(total int64) *NotificationListResponse {
	r.Total = total
	return r
}

// WithUnreadCount sets the unread count.
func (r *NotificationListResponse) WithUnreadCount(count int64) *NotificationListResponse {
	r.UnreadCount = count
	return r
}

// WithNextCursor sets the next cursor.
func (r *NotificationListResponse) WithNextCursor(cursor string) *NotificationListResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithLimit sets the limit.
func (r *NotificationListResponse) WithLimit(limit int) *NotificationListResponse {
	r.Limit = limit
	return r
}

// ======================================================================
// Builder Methods for GroupedNotificationResponse
// ======================================================================

// NewGroupedNotificationResponse creates a new grouped notification response.
func NewGroupedNotificationResponse(notifType, referenceID string, count int64, latestAt time.Time) *GroupedNotificationResponse {
	typ := NotificationType(notifType)
	return &GroupedNotificationResponse{
		Type:        notifType,
		TypeDisplay: typ.GetDisplayName(),
		ReferenceID: referenceID,
		Count:       count,
		LatestAt:    latestAt,
		FromUserIDs: []string{},
		FromUsers:   []MinimalUserResponse{},
	}
}

// WithFromUsers sets the from users.
func (r *GroupedNotificationResponse) WithFromUsers(users []MinimalUserResponse) *GroupedNotificationResponse {
	r.FromUsers = users
	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	r.FromUserIDs = ids
	return r
}

// WithReadStatus sets the read status.
func (r *GroupedNotificationResponse) WithReadStatus(read bool) *GroupedNotificationResponse {
	r.IsRead = read
	return r
}

// WithReadCount sets the read count.
func (r *GroupedNotificationResponse) WithReadCount(count int64) *GroupedNotificationResponse {
	r.ReadCount = count
	return r
}

// WithUnreadCount sets the unread count.
func (r *GroupedNotificationResponse) WithUnreadCount(count int64) *GroupedNotificationResponse {
	r.UnreadCount = count
	return r
}

// WithLatestID sets the latest ID.
func (r *GroupedNotificationResponse) WithLatestID(id string) *GroupedNotificationResponse {
	r.LatestID = id
	return r
}

// ======================================================================
// Builder Methods for NotificationStatsResponse
// ======================================================================

// NewNotificationStatsResponse creates a new notification stats response.
func NewNotificationStatsResponse() *NotificationStatsResponse {
	return &NotificationStatsResponse{
		TypeStats: make(map[string]int64),
		DailyStats: []DailyNotificationCount{},
	}
}

// WithTotal sets the total count.
func (r *NotificationStatsResponse) WithTotal(total int64) *NotificationStatsResponse {
	r.Total = total
	return r
}

// WithUnread sets the unread count.
func (r *NotificationStatsResponse) WithUnread(unread int64) *NotificationStatsResponse {
	r.Unread = unread
	return r
}

// WithRead sets the read count.
func (r *NotificationStatsResponse) WithRead(read int64) *NotificationStatsResponse {
	r.Read = read
	return r
}

// WithUniqueUsers sets the unique users count.
func (r *NotificationStatsResponse) WithUniqueUsers(count int64) *NotificationStatsResponse {
	r.UniqueUsers = count
	return r
}

// WithUniqueSenders sets the unique senders count.
func (r *NotificationStatsResponse) WithUniqueSenders(count int64) *NotificationStatsResponse {
	r.UniqueSenders = count
	return r
}

// WithLatest sets the latest timestamp.
func (r *NotificationStatsResponse) WithLatest(t time.Time) *NotificationStatsResponse {
	r.Latest = t
	return r
}

// WithEarliest sets the earliest timestamp.
func (r *NotificationStatsResponse) WithEarliest(t time.Time) *NotificationStatsResponse {
	r.Earliest = t
	return r
}

// AddTypeStat adds a type statistic.
func (r *NotificationStatsResponse) AddTypeStat(notifType string, count int64) {
	r.TypeStats[notifType] = count
}

// WithDailyStats sets the daily stats.
func (r *NotificationStatsResponse) WithDailyStats(stats []DailyNotificationCount) *NotificationStatsResponse {
	r.DailyStats = stats
	return r
}

// ======================================================================
// Conversion Helpers
// ======================================================================

// ToNotificationResponse converts notification data to response.
func ToNotificationResponse(id, userID, fromUserID, notifType, referenceID string, read bool, createdAt, updatedAt time.Time) NotificationResponse {
	typ := NotificationType(notifType)
	return NotificationResponse{
		ID:          id,
		UserID:      userID,
		FromUserID:  fromUserID,
		Type:        notifType,
		TypeDisplay: typ.GetDisplayName(),
		ReferenceID: referenceID,
		Read:        read,
		Priority:    typ.GetPriority(),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

// ToDailyNotificationCount creates a daily count.
func ToDailyNotificationCount(date time.Time, total, unread, read, uniqueUsers int64) DailyNotificationCount {
	return DailyNotificationCount{
		Date:        date,
		Total:       total,
		Unread:      unread,
		Read:        read,
		UniqueUsers: uniqueUsers,
	}
}

// ToNotificationTypeStat creates a type stat.
func ToNotificationTypeStat(notifType string, count, unreadCount, readCount int64, latestAt time.Time) NotificationTypeStat {
	typ := NotificationType(notifType)
	return NotificationTypeStat{
		Type:        notifType,
		TypeDisplay: typ.GetDisplayName(),
		Count:       count,
		UnreadCount: unreadCount,
		ReadCount:   readCount,
		LatestAt:    latestAt,
	}
}

// ======================================================================
= JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *NotificationResponse) MarshalJSON() ([]byte, error) {
	type Alias NotificationResponse
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(r),
		Type:  r.Type,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *NotificationResponse) UnmarshalJSON(data []byte) error {
	type Alias NotificationResponse
	aux := &struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Type != "" {
		r.Type = aux.Type
	}
	return nil
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestNotificationResponse creates a test notification response.
func NewTestNotificationResponse() *NotificationResponse {
	resp := NewNotificationResponse(
		"notif1", "user1", "user2", "like", "tweet1", false,
	)
	resp.WithMetadata(map[string]interface{}{
		"tweet_content": "Hello world",
	})
	return resp
}

// NewTestNotificationListResponse creates a test notification list response.
func NewTestNotificationListResponse() *NotificationListResponse {
	list := NewNotificationListResponse()
	list.Add(*NewTestNotificationResponse())
	list.WithTotal(1).WithUnreadCount(1).WithNextCursor("cursor123").WithLimit(20)
	return list
}

// NewTestGroupedNotificationResponse creates a test grouped notification response.
func NewTestGroupedNotificationResponse() *GroupedNotificationResponse {
	resp := NewGroupedNotificationResponse("like", "tweet1", 5, time.Now().UTC())
	resp.WithFromUsers([]MinimalUserResponse{
		{ID: "user1", Username: "john_doe", FullName: "John Doe"},
		{ID: "user2", Username: "jane_smith", FullName: "Jane Smith"},
	})
	resp.WithReadStatus(false).WithReadCount(2).WithUnreadCount(3)
	return resp
}

// NewTestNotificationStatsResponse creates a test stats response.
func NewTestNotificationStatsResponse() *NotificationStatsResponse {
	stats := NewNotificationStatsResponse()
	stats.WithTotal(100).WithUnread(30).WithRead(70)
	stats.WithUniqueUsers(50).WithUniqueSenders(40)
	stats.WithLatest(time.Now().UTC()).WithEarliest(time.Now().Add(-7*24*time.Hour))
	stats.AddTypeStat("like", 40)
	stats.AddTypeStat("follow", 30)
	stats.AddTypeStat("reply", 30)
	return stats
}

// ======================================================================
= API Documentation Constants
// ======================================================================

const (
	APITagNotifications = "Notifications"
)