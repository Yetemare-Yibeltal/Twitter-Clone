// backend/internal/service/notification_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	MaxNotificationsLimit = 100
	DefaultNotificationsLimit = 20
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrNotificationAlreadyRead = errors.New("notification already read")
	ErrInvalidNotificationType = errors.New("invalid notification type")
	ErrNotificationUserMismatch = errors.New("notification does not belong to user")
)

// ======================================================================
= NotificationService Interface
// ======================================================================

// NotificationService defines the notification service interface.
type NotificationService interface {
	// Create creates a new notification.
	Create(ctx context.Context, notification *entities.Notification) error
	
	// GetByID retrieves a notification by ID.
	GetByID(ctx context.Context, id string) (*entities.Notification, error)
	
	// GetByUserID retrieves notifications for a user with pagination.
	GetByUserID(ctx context.Context, userID, cursor string, limit int) ([]*entities.Notification, string, int64, error)
	
	// GetUnreadByUserID retrieves unread notifications for a user.
	GetUnreadByUserID(ctx context.Context, userID, cursor string, limit int) ([]*entities.Notification, string, int64, error)
	
	// MarkAsRead marks a notification as read.
	MarkAsRead(ctx context.Context, userID, notificationID string) error
	
	// MarkAllAsRead marks all notifications for a user as read.
	MarkAllAsRead(ctx context.Context, userID string) error
	
	// MarkMultipleAsRead marks multiple notifications as read.
	MarkMultipleAsRead(ctx context.Context, userID string, ids []string) error
	
	// GetUnreadCount returns the number of unread notifications for a user.
	GetUnreadCount(ctx context.Context, userID string) (int64, error)
	
	// Delete deletes a notification.
	Delete(ctx context.Context, userID, notificationID string) error
	
	// DeleteAll deletes all notifications for a user.
	DeleteAll(ctx context.Context, userID string) error
	
	// GetGroupedNotifications returns grouped notifications for a user.
	GetGroupedNotifications(ctx context.Context, userID, cursor string, limit int) ([]*dto.GroupedNotificationResponse, string, int64, error)
	
	// GetNotificationStats returns notification statistics.
	GetNotificationStats(ctx context.Context) (*dto.NotificationStatsResponse, error)
	
	// GetUserNotificationStats returns notification stats for a user.
	GetUserNotificationStats(ctx context.Context, userID string) (*dto.UserNotificationStatsResponse, error)
}

// ======================================================================
= NotificationService Implementation
// ======================================================================

// notificationService implements NotificationService.
type notificationService struct {
	notificationRepo interfaces.NotificationRepository
	userRepo         interfaces.UserRepository
	redisAdapter     adapter.RedisAdapter
	wsHub            *adapter.WebSocketHub
	log              *logrus.Entry
}

// NewNotificationService creates a new notification service.
func NewNotificationService(
	notificationRepo interfaces.NotificationRepository,
	userRepo interfaces.UserRepository,
	redisAdapter adapter.RedisAdapter,
	wsHub *adapter.WebSocketHub,
) NotificationService {
	return &notificationService{
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
		redisAdapter:     redisAdapter,
		wsHub:            wsHub,
		log:              logger.WithField("service", "notification"),
	}
}

// ======================================================================
// Create Notification
// ======================================================================

// Create creates a new notification.
func (s *notificationService) Create(ctx context.Context, notification *entities.Notification) error {
	// Validate notification type
	validTypes := map[string]bool{
		"like": true, "retweet": true, "follow": true, "reply": true,
		"mention": true, "quote": true, "message": true,
	}
	if !validTypes[notification.Type] {
		return ErrInvalidNotificationType
	}
	
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, notification.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	
	// If from_user_id is provided, verify exists
	if notification.FromUserID != "" {
		_, err := s.userRepo.GetByID(ctx, notification.FromUserID)
		if err != nil {
			return fmt.Errorf("failed to get from user: %w", err)
		}
	}
	
	// Generate ID if not set
	if notification.ID == "" {
		notification.ID = uuid.New().String()
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}
	
	// Save notification
	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	
	// Invalidate unread count cache
	_ = s.invalidateUnreadCache(ctx, notification.UserID)
	_ = s.invalidateListCache(ctx, notification.UserID)
	
	// Send real-time notification via WebSocket
	if s.wsHub != nil {
		s.wsHub.SendNotification(notification.UserID, notification)
	}
	
	s.log.WithFields(logrus.Fields{
		"user_id":     notification.UserID,
		"from_user_id": notification.FromUserID,
		"type":        notification.Type,
		"reference_id": notification.ReferenceID,
	}).Info("Notification created")
	
	return nil
}

// ======================================================================
// Get Notification
// ======================================================================

// GetByID retrieves a notification by ID.
func (s *notificationService) GetByID(ctx context.Context, id string) (*entities.Notification, error) {
	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, interfaces.ErrNotificationNotFound) {
			return nil, ErrNotificationNotFound
		}
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	return notification, nil
}

// ======================================================================
// Get Notifications by User
// ======================================================================

// GetByUserID retrieves notifications for a user with pagination.
func (s *notificationService) GetByUserID(ctx context.Context, userID, cursor string, limit int) ([]*entities.Notification, string, int64, error) {
	if limit < 1 || limit > MaxNotificationsLimit {
		limit = DefaultNotificationsLimit
	}
	
	// Check cache for first page
	if cursor == "" {
		cacheKey := fmt.Sprintf("notifications:%s:%d", userID, limit)
		if s.redisAdapter != nil {
			var cached struct {
				Notifications []*entities.Notification `json:"notifications"`
				NextCursor    string                    `json:"next_cursor"`
				Total         int64                     `json:"total"`
			}
			if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
				s.log.WithField("user_id", userID).Debug("Notifications served from cache")
				return cached.Notifications, cached.NextCursor, cached.Total, nil
			}
		}
	}
	
	// Get from repository
	notifications, nextCursor, err := s.notificationRepo.GetByUserID(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get notifications: %w", err)
	}
	
	// Get total count
	total, err := s.notificationRepo.CountByUserID(ctx, userID)
	if err != nil {
		total = int64(len(notifications))
	}
	
	// Cache first page for 30 seconds
	if cursor == "" && s.redisAdapter != nil {
		cacheData := struct {
			Notifications []*entities.Notification `json:"notifications"`
			NextCursor    string                    `json:"next_cursor"`
			Total         int64                     `json:"total"`
		}{
			Notifications: notifications,
			NextCursor:    nextCursor,
			Total:         total,
		}
		cacheKey := fmt.Sprintf("notifications:%s:%d", userID, limit)
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 30*time.Second)
	}
	
	return notifications, nextCursor, total, nil
}

// ======================================================================
// Get Unread Notifications
// ======================================================================

// GetUnreadByUserID retrieves unread notifications for a user.
func (s *notificationService) GetUnreadByUserID(ctx context.Context, userID, cursor string, limit int) ([]*entities.Notification, string, int64, error) {
	if limit < 1 || limit > MaxNotificationsLimit {
		limit = DefaultNotificationsLimit
	}
	
	// Check cache for first page
	if cursor == "" {
		cacheKey := fmt.Sprintf("unread_notifications:%s:%d", userID, limit)
		if s.redisAdapter != nil {
			var cached struct {
				Notifications []*entities.Notification `json:"notifications"`
				NextCursor    string                    `json:"next_cursor"`
				Total         int64                     `json:"total"`
			}
			if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
				return cached.Notifications, cached.NextCursor, cached.Total, nil
			}
		}
	}
	
	// Get from repository
	notifications, nextCursor, err := s.notificationRepo.GetUnreadByUserID(ctx, userID, cursor, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get unread notifications: %w", err)
	}
	
	// Get total count
	total, err := s.notificationRepo.CountUnread(ctx, userID)
	if err != nil {
		total = int64(len(notifications))
	}
	
	// Cache first page for 15 seconds (more frequent updates)
	if cursor == "" && s.redisAdapter != nil {
		cacheData := struct {
			Notifications []*entities.Notification `json:"notifications"`
			NextCursor    string                    `json:"next_cursor"`
			Total         int64                     `json:"total"`
		}{
			Notifications: notifications,
			NextCursor:    nextCursor,
			Total:         total,
		}
		cacheKey := fmt.Sprintf("unread_notifications:%s:%d", userID, limit)
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 15*time.Second)
	}
	
	return notifications, nextCursor, total, nil
}

// ======================================================================
= Mark as Read
// ======================================================================

// MarkAsRead marks a notification as read.
func (s *notificationService) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	// Get notification
	notification, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		if errors.Is(err, interfaces.ErrNotificationNotFound) {
			return ErrNotificationNotFound
		}
		return fmt.Errorf("failed to get notification: %w", err)
	}
	
	// Verify ownership
	if notification.UserID != userID {
		return ErrNotificationUserMismatch
	}
	
	// Check if already read
	if notification.Read {
		return ErrNotificationAlreadyRead
	}
	
	// Mark as read
	if err := s.notificationRepo.MarkAsRead(ctx, notificationID); err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}
	
	// Invalidate caches
	_ = s.invalidateUnreadCache(ctx, userID)
	_ = s.invalidateListCache(ctx, userID)
	_ = s.invalidateGroupedCache(ctx, userID)
	
	return nil
}

// ======================================================================
= Mark All as Read
// ======================================================================

// MarkAllAsRead marks all notifications for a user as read.
func (s *notificationService) MarkAllAsRead(ctx context.Context, userID string) error {
	if err := s.notificationRepo.MarkAllAsRead(ctx, userID); err != nil {
		return fmt.Errorf("failed to mark all as read: %w", err)
	}
	
	// Invalidate caches
	_ = s.invalidateUnreadCache(ctx, userID)
	_ = s.invalidateListCache(ctx, userID)
	_ = s.invalidateGroupedCache(ctx, userID)
	
	s.log.WithField("user_id", userID).Info("All notifications marked as read")
	return nil
}

// ======================================================================
= Mark Multiple as Read
// ======================================================================

// MarkMultipleAsRead marks multiple notifications as read.
func (s *notificationService) MarkMultipleAsRead(ctx context.Context, userID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	
	// Verify ownership for each notification
	for _, id := range ids {
		notification, err := s.notificationRepo.GetByID(ctx, id)
		if err != nil {
			continue
		}
		if notification.UserID != userID {
			return fmt.Errorf("notification %s does not belong to user", id)
		}
	}
	
	if err := s.notificationRepo.MarkMultipleAsRead(ctx, ids); err != nil {
		return fmt.Errorf("failed to mark multiple as read: %w", err)
	}
	
	// Invalidate caches
	_ = s.invalidateUnreadCache(ctx, userID)
	_ = s.invalidateListCache(ctx, userID)
	_ = s.invalidateGroupedCache(ctx, userID)
	
	return nil
}

// ======================================================================
= Get Unread Count
// ======================================================================

// GetUnreadCount returns the number of unread notifications for a user.
func (s *notificationService) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	// Check cache
	cacheKey := fmt.Sprintf("unread_count:%s", userID)
	if s.redisAdapter != nil {
		var count int64
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &count); err == nil {
			return count, nil
		}
	}
	
	count, err := s.notificationRepo.CountUnread(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	
	// Cache for 10 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, count, 10*time.Second)
	}
	
	return count, nil
}

// ======================================================================
= Delete
// ======================================================================

// Delete deletes a notification.
func (s *notificationService) Delete(ctx context.Context, userID, notificationID string) error {
	// Get notification
	notification, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		if errors.Is(err, interfaces.ErrNotificationNotFound) {
			return ErrNotificationNotFound
		}
		return fmt.Errorf("failed to get notification: %w", err)
	}
	
	// Verify ownership
	if notification.UserID != userID {
		return ErrNotificationUserMismatch
	}
	
	if err := s.notificationRepo.Delete(ctx, notificationID); err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}
	
	// Invalidate caches
	_ = s.invalidateUnreadCache(ctx, userID)
	_ = s.invalidateListCache(ctx, userID)
	_ = s.invalidateGroupedCache(ctx, userID)
	
	return nil
}

// ======================================================================
= Delete All
// ======================================================================

// DeleteAll deletes all notifications for a user.
func (s *notificationService) DeleteAll(ctx context.Context, userID string) error {
	if err := s.notificationRepo.BulkDeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete all notifications: %w", err)
	}
	
	// Invalidate caches
	_ = s.invalidateUnreadCache(ctx, userID)
	_ = s.invalidateListCache(ctx, userID)
	_ = s.invalidateGroupedCache(ctx, userID)
	
	s.log.WithField("user_id", userID).Info("All notifications deleted")
	return nil
}

// ======================================================================
= Get Grouped Notifications
// ======================================================================

// GetGroupedNotifications returns grouped notifications for a user.
func (s *notificationService) GetGroupedNotifications(ctx context.Context, userID, cursor string, limit int) ([]*dto.GroupedNotificationResponse, string, int64, error) {
	if limit < 1 || limit > MaxNotificationsLimit {
		limit = DefaultNotificationsLimit
	}
	
	// Check cache for first page
	if cursor == "" {
		cacheKey := fmt.Sprintf("grouped_notifications:%s:%d", userID, limit)
		if s.redisAdapter != nil {
			var cached struct {
				Groups    []*dto.GroupedNotificationResponse `json:"groups"`
				NextCursor string                            `json:"next_cursor"`
				Total      int64                             `json:"total"`
			}
			if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
				return cached.Groups, cached.NextCursor, cached.Total, nil
			}
		}
	}
	
	// Get grouped notifications from repository
	groups, err := s.notificationRepo.GroupNotifications(ctx, userID, limit)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get grouped notifications: %w", err)
	}
	
	// Build response
	responses := make([]*dto.GroupedNotificationResponse, 0, len(groups))
	for _, group := range groups {
		// Get count of notifications in this group
		count, err := s.notificationRepo.CountByType(ctx, userID, group.Type)
		if err != nil {
			count = 1
		}
		
		// Get from user info
		fromUser, _ := s.userRepo.GetByID(ctx, group.FromUserID)
		
		responses = append(responses, &dto.GroupedNotificationResponse{
			Type:        group.Type,
			ReferenceID: group.ReferenceID,
			Count:       count,
			LatestAt:    group.CreatedAt,
			FromUserID:  group.FromUserID,
			FromUsername: func() string {
				if fromUser != nil {
					return fromUser.Username
				}
				return ""
			}(),
			FromAvatarURL: func() string {
				if fromUser != nil {
					return fromUser.AvatarURL
				}
				return ""
			}(),
			IsRead: group.Read,
		})
	}
	
	// Get total count
	total, err := s.notificationRepo.CountByUserID(ctx, userID)
	if err != nil {
		total = int64(len(responses))
	}
	
	// Cache for 30 seconds
	if cursor == "" && s.redisAdapter != nil && len(responses) > 0 {
		cacheData := struct {
			Groups    []*dto.GroupedNotificationResponse `json:"groups"`
			NextCursor string                            `json:"next_cursor"`
			Total      int64                             `json:"total"`
		}{
			Groups:    responses,
			NextCursor: "",
			Total:     total,
		}
		cacheKey := fmt.Sprintf("grouped_notifications:%s:%d", userID, limit)
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, cacheData, 30*time.Second)
	}
	
	return responses, "", total, nil
}

// ======================================================================
= Get Notification Stats
// ======================================================================

// GetNotificationStats returns notification statistics.
func (s *notificationService) GetNotificationStats(ctx context.Context) (*dto.NotificationStatsResponse, error) {
	stats, err := s.notificationRepo.GetNotificationStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification stats: %w", err)
	}
	
	// Get daily stats for last 7 days
	end := time.Now()
	start := end.AddDate(0, 0, -7)
	dailyStats, err := s.notificationRepo.GetDailyNotifications(ctx, start, end)
	if err != nil {
		dailyStats = []*interfaces.DailyNotificationCount{}
	}
	
	return &dto.NotificationStatsResponse{
		Total:        stats.Total,
		UniqueUsers:  stats.UniqueUsers,
		UniqueSenders: stats.UniqueSenders,
		ReadCount:    stats.ReadCount,
		UnreadCount:  stats.UnreadCount,
		Latest:       stats.Latest,
		Earliest:     stats.Earliest,
		DailyStats:   dailyStats,
	}, nil
}

// ======================================================================
= Get User Notification Stats
// ======================================================================

// GetUserNotificationStats returns notification stats for a user.
func (s *notificationService) GetUserNotificationStats(ctx context.Context, userID string) (*dto.UserNotificationStatsResponse, error) {
	total, err := s.notificationRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count notifications: %w", err)
	}
	
	unread, err := s.notificationRepo.CountUnread(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count unread: %w", err)
	}
	
	// Get type distribution
	typeStats, err := s.notificationRepo.GetNotificationTypeStats(ctx, userID)
	if err != nil {
		typeStats = []*interfaces.NotificationTypeStat{}
	}
	
	// Get latest notification
	latest, err := s.notificationRepo.GetRecentNotifications(ctx, userID, 1)
	if err != nil || len(latest) == 0 {
		latest = []*entities.Notification{}
	}
	
	return &dto.UserNotificationStatsResponse{
		UserID:       userID,
		Total:        total,
		Unread:       unread,
		Read:         total - unread,
		TypeStats:    typeStats,
		LatestAt: func() time.Time {
			if len(latest) > 0 {
				return latest[0].CreatedAt
			}
			return time.Time{}
		}(),
	}, nil
}

// ======================================================================
= Cache Invalidation Helpers
// ======================================================================

// invalidateUnreadCache invalidates unread count cache.
func (s *notificationService) invalidateUnreadCache(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	cacheKey := fmt.Sprintf("unread_count:%s", userID)
	return s.redisAdapter.Delete(ctx, cacheKey)
}

// invalidateListCache invalidates notification list cache.
func (s *notificationService) invalidateListCache(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	pattern := fmt.Sprintf("notifications:%s:*", userID)
	iter := s.redisAdapter.Scan(ctx, 0, pattern, 100)
	var keys []string
	for {
		keysBatch, nextCursor, err := iter.Next()
		if err != nil {
			break
		}
		keys = append(keys, keysBatch...)
		if nextCursor == 0 {
			break
		}
	}
	if len(keys) > 0 {
		return s.redisAdapter.Delete(ctx, keys...)
	}
	// Also unread notifications list
	unreadPattern := fmt.Sprintf("unread_notifications:%s:*", userID)
	iter = s.redisAdapter.Scan(ctx, 0, unreadPattern, 100)
	keys = []string{}
	for {
		keysBatch, nextCursor, err := iter.Next()
		if err != nil {
			break
		}
		keys = append(keys, keysBatch...)
		if nextCursor == 0 {
			break
		}
	}
	if len(keys) > 0 {
		return s.redisAdapter.Delete(ctx, keys...)
	}
	return nil
}

// invalidateGroupedCache invalidates grouped notifications cache.
func (s *notificationService) invalidateGroupedCache(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	pattern := fmt.Sprintf("grouped_notifications:%s:*", userID)
	iter := s.redisAdapter.Scan(ctx, 0, pattern, 100)
	var keys []string
	for {
		keysBatch, nextCursor, err := iter.Next()
		if err != nil {
			break
		}
		keys = append(keys, keysBatch...)
		if nextCursor == 0 {
			break
		}
	}
	if len(keys) > 0 {
		return s.redisAdapter.Delete(ctx, keys...)
	}
	return nil
}