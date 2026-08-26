// backend/tests/unit/auth_test.go
package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/internal/service"
)

// ======================================================================
// Mocks
// ======================================================================

// MockUserRepository implements interfaces.UserRepository.
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *entities.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*entities.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.User), args.Error(1)
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*entities.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.User), args.Error(1)
}

func (m *MockUserRepository) GetByUsernameOrEmail(ctx context.Context, identifier string) (*entities.User, error) {
	args := m.Called(ctx, identifier)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.User), args.Error(1)
}

func (m *MockUserRepository) GetByIDs(ctx context.Context, ids []string) ([]*entities.User, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *entities.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	args := m.Called(ctx, id, fields)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) SoftDelete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) Restore(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) List(ctx context.Context, filter *interfaces.UserFilter, pagination *interfaces.PaginationOptions) ([]*entities.User, int64, error) {
	args := m.Called(ctx, filter, pagination)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*entities.User), args.Get(1).(int64), args.Error(2)
}

func (m *MockUserRepository) Count(ctx context.Context, filter *interfaces.UserFilter) (int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockUserRepository) Search(ctx context.Context, query string, pagination *interfaces.PaginationOptions) ([]*entities.User, int64, error) {
	args := m.Called(ctx, query, pagination)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*entities.User), args.Get(1).(int64), args.Error(2)
}

func (m *MockUserRepository) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	args := m.Called(ctx, username)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) GetStats(ctx context.Context, userID string) (*interfaces.UserStats, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.UserStats), args.Error(1)
}

func (m *MockUserRepository) GetStatsForUsers(ctx context.Context, userIDs []string) (map[string]*interfaces.UserStats, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*interfaces.UserStats), args.Error(1)
}

func (m *MockUserRepository) RecordActivity(ctx context.Context, activity *interfaces.UserActivity) error {
	args := m.Called(ctx, activity)
	return args.Error(0)
}

func (m *MockUserRepository) GetUserActivities(ctx context.Context, userID string, limit, offset int) ([]*interfaces.UserActivity, int64, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*interfaces.UserActivity), args.Get(1).(int64), args.Error(2)
}

func (m *MockUserRepository) UpdateLastActive(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) IncrementTweetCount(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) DecrementTweetCount(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) IncrementFollowerCount(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) DecrementFollowerCount(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) IncrementFollowingCount(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) DecrementFollowingCount(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateVerificationStatus(ctx context.Context, userID string, verified bool) error {
	args := m.Called(ctx, userID, verified)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateSuspensionStatus(ctx context.Context, userID string, suspended bool, reason string) error {
	args := m.Called(ctx, userID, suspended, reason)
	return args.Error(0)
}

func (m *MockUserRepository) BulkCreate(ctx context.Context, users []*entities.User) error {
	args := m.Called(ctx, users)
	return args.Error(0)
}

func (m *MockUserRepository) BulkUpdate(ctx context.Context, users []*entities.User) error {
	args := m.Called(ctx, users)
	return args.Error(0)
}

func (m *MockUserRepository) BulkDelete(ctx context.Context, ids []string) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockUserRepository) GetRecentlyJoined(ctx context.Context, duration time.Duration, limit int) ([]*entities.User, error) {
	args := m.Called(ctx, duration, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.User), args.Error(1)
}

func (m *MockUserRepository) GetActiveUsers(ctx context.Context, duration time.Duration, limit int) ([]*entities.User, error) {
	args := m.Called(ctx, duration, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.User), args.Error(1)
}

func (m *MockUserRepository) GetTopUsersByFollowers(ctx context.Context, limit int) ([]*entities.User, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.User), args.Error(1)
}

func (m *MockUserRepository) GetTopUsersByTweets(ctx context.Context, limit int) ([]*entities.User, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entities.User), args.Error(1)
}

func (m *MockUserRepository) GetUsersWithRole(ctx context.Context, role string, pagination *interfaces.PaginationOptions) ([]*entities.User, int64, error) {
	args := m.Called(ctx, role, pagination)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*entities.User), args.Get(1).(int64), args.Error(2)
}

func (m *MockUserRepository) Transaction(ctx context.Context, fn func(txRepo interfaces.UserRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockUserRepository) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.UserRepository {
	args := m.Called(ctx, tx)
	return args.Get(0).(interfaces.UserRepository)
}

func (m *MockUserRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockUserRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockUserRepository) GetRawDB() interface{} {
	args := m.Called()
	return args.Get(0)
}

// MockSessionRepository implements interfaces.SessionRepository.
type MockSessionRepository struct {
	mock.Mock
}

func (m *MockSessionRepository) Create(ctx context.Context, session *interfaces.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockSessionRepository) GetByID(ctx context.Context, id string) (*interfaces.Session, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.Session), args.Error(1)
}

func (m *MockSessionRepository) GetByRefreshToken(ctx context.Context, refreshToken string) (*interfaces.Session, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.Session), args.Error(1)
}

func (m *MockSessionRepository) GetByUserID(ctx context.Context, userID string) ([]*interfaces.Session, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.Session), args.Error(1)
}

func (m *MockSessionRepository) Update(ctx context.Context, session *interfaces.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockSessionRepository) UpdateRefreshToken(ctx context.Context, id, newRefreshToken string, newExpiry time.Time) error {
	args := m.Called(ctx, id, newRefreshToken, newExpiry)
	return args.Error(0)
}

func (m *MockSessionRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSessionRepository) DeleteByRefreshToken(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

func (m *MockSessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockSessionRepository) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockSessionRepository) ExistsByRefreshToken(ctx context.Context, refreshToken string) (bool, error) {
	args := m.Called(ctx, refreshToken)
	return args.Bool(0), args.Error(1)
}

func (m *MockSessionRepository) IsValidSession(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockSessionRepository) IsValidRefreshToken(ctx context.Context, refreshToken string) (bool, error) {
	args := m.Called(ctx, refreshToken)
	return args.Bool(0), args.Error(1)
}

func (m *MockSessionRepository) GetActiveSessions(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) GetExpiredSessions(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) GetActiveSessionsAll(ctx context.Context, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) GetExpiredSessionsAll(ctx context.Context, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) GetSessionsByIP(ctx context.Context, ip string, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, ip, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) GetSessionsByUserAgent(ctx context.Context, userAgent string, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, userAgent, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CountActiveByUserID(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CountExpiredByUserID(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CountTotal(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CountActive(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CountExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CountByDateRangeForUser(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	args := m.Called(ctx, userID, start, end)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CountUniqueUsersInDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) ExtendExpiry(ctx context.Context, id string, newExpiry time.Time) error {
	args := m.Called(ctx, id, newExpiry)
	return args.Error(0)
}

func (m *MockSessionRepository) Revoke(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSessionRepository) RevokeAll(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockSessionRepository) RevokeAllExcept(ctx context.Context, userID, excludeSessionID string) error {
	args := m.Called(ctx, userID, excludeSessionID)
	return args.Error(0)
}

func (m *MockSessionRepository) RevokeByRefreshToken(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

func (m *MockSessionRepository) CleanupExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CleanupExpiredForUser(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CleanupOlderThan(ctx context.Context, before time.Time) (int64, error) {
	args := m.Called(ctx, before)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) CleanupOlderThanForUser(ctx context.Context, userID string, before time.Time) (int64, error) {
	args := m.Called(ctx, userID, before)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) GetLatestSession(ctx context.Context, userID string) (*interfaces.Session, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.Session), args.Error(1)
}

func (m *MockSessionRepository) GetOldestSession(ctx context.Context, userID string) (*interfaces.Session, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.Session), args.Error(1)
}

func (m *MockSessionRepository) GetSessionsByDateRange(ctx context.Context, start, end time.Time, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, start, end, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) GetSessionsForUserByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, userID, start, end, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) GetSessionsByUserAndIP(ctx context.Context, userID, ip string, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, userID, ip, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) GetSessionsWithMetadata(ctx context.Context, key, value string, cursor string, limit int) ([]*interfaces.Session, string, error) {
	args := m.Called(ctx, key, value, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Session), args.Get(1).(string), args.Error(2)
}

func (m *MockSessionRepository) BulkCreate(ctx context.Context, sessions []*interfaces.Session) error {
	args := m.Called(ctx, sessions)
	return args.Error(0)
}

func (m *MockSessionRepository) BulkDelete(ctx context.Context, ids []string) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockSessionRepository) BulkDeleteByUserIDs(ctx context.Context, userIDs []string) error {
	args := m.Called(ctx, userIDs)
	return args.Error(0)
}

func (m *MockSessionRepository) BulkRevoke(ctx context.Context, ids []string) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockSessionRepository) BulkExtendExpiry(ctx context.Context, ids []string, newExpiry time.Time) error {
	args := m.Called(ctx, ids, newExpiry)
	return args.Error(0)
}

func (m *MockSessionRepository) BulkDeleteExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSessionRepository) GetSessionStats(ctx context.Context) (*interfaces.SessionStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.SessionStats), args.Error(1)
}

func (m *MockSessionRepository) GetUserSessionStats(ctx context.Context, userID string) (*interfaces.SessionStats, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.SessionStats), args.Error(1)
}

func (m *MockSessionRepository) GetDailySessionStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailySessionCount, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.DailySessionCount), args.Error(1)
}

func (m *MockSessionRepository) GetDailySessionStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*interfaces.DailySessionCount, error) {
	args := m.Called(ctx, userID, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.DailySessionCount), args.Error(1)
}

func (m *MockSessionRepository) GetDeviceStats(ctx context.Context, userID string) ([]*interfaces.DeviceStat, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.DeviceStat), args.Error(1)
}

func (m *MockSessionRepository) GetLocationStats(ctx context.Context, userID string) ([]*interfaces.LocationStat, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.LocationStat), args.Error(1)
}

func (m *MockSessionRepository) GetAverageSessionDuration(ctx context.Context, userID string) (float64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockSessionRepository) GetSessionRetentionRate(ctx context.Context, days int) (float64, error) {
	args := m.Called(ctx, days)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockSessionRepository) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.SessionRepository {
	args := m.Called(ctx, tx)
	return args.Get(0).(interfaces.SessionRepository)
}

func (m *MockSessionRepository) Transaction(ctx context.Context, fn func(txRepo interfaces.SessionRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockSessionRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSessionRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSessionRepository) GetRawDB() interface{} {
	args := m.Called()
	return args.Get(0)
}

// MockNotificationRepository implements interfaces.NotificationRepository.
type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) Create(ctx context.Context, notification *interfaces.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) GetByID(ctx context.Context, id string) (*interfaces.Notification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.Notification), args.Error(1)
}

func (m *MockNotificationRepository) GetByUserAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	args := m.Called(ctx, userID, notificationType, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Notification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GetByReferenceID(ctx context.Context, referenceID string) ([]*interfaces.Notification, error) {
	args := m.Called(ctx, referenceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.Notification), args.Error(1)
}

func (m *MockNotificationRepository) Update(ctx context.Context, notification *interfaces.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationRepository) DeleteByUserAndReference(ctx context.Context, userID, referenceID string) error {
	args := m.Called(ctx, userID, referenceID)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkAsRead(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkMultipleAsRead(ctx context.Context, ids []string) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkAsUnread(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkAllAsUnread(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockNotificationRepository) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockNotificationRepository) ExistsByUserAndReference(ctx context.Context, userID, referenceID string) (bool, error) {
	args := m.Called(ctx, userID, referenceID)
	return args.Bool(0), args.Error(1)
}

func (m *MockNotificationRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) CountUnread(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) CountRead(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) CountByType(ctx context.Context, userID, notificationType string) (int64, error) {
	args := m.Called(ctx, userID, notificationType)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) CountByDateRange(ctx context.Context, userID string, start, end time.Time) (int64, error) {
	args := m.Called(ctx, userID, start, end)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) CountByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockNotificationRepository) CountUnreadByUserIDs(ctx context.Context, userIDs []string) (map[string]int64, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockNotificationRepository) GetByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Notification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GetUnreadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Notification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GetReadByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Notification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GetByFromUserID(ctx context.Context, fromUserID string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	args := m.Called(ctx, fromUserID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Notification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GetRecentByUserID(ctx context.Context, userID string, limit int) ([]*interfaces.Notification, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.Notification), args.Error(1)
}

func (m *MockNotificationRepository) GetByUserIDAndType(ctx context.Context, userID, notificationType string, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	args := m.Called(ctx, userID, notificationType, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Notification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GroupByType(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.GroupedNotification, string, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.GroupedNotification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GroupByReference(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.GroupedNotification, string, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.GroupedNotification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GroupByTypeAndReference(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.GroupedNotification, string, error) {
	args := m.Called(ctx, userID, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.GroupedNotification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GetGroupedCount(ctx context.Context, userID string) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) GetNotificationsByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	args := m.Called(ctx, userID, start, end, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Notification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GetUnreadByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*interfaces.Notification, string, error) {
	args := m.Called(ctx, userID, start, end, cursor, limit)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).([]*interfaces.Notification), args.Get(1).(string), args.Error(2)
}

func (m *MockNotificationRepository) GetNotificationsByReferenceIDAndType(ctx context.Context, referenceID, notificationType string) ([]*interfaces.Notification, error) {
	args := m.Called(ctx, referenceID, notificationType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.Notification), args.Error(1)
}

func (m *MockNotificationRepository) GetNotificationsByMultipleReferences(ctx context.Context, referenceIDs []string) ([]*interfaces.Notification, error) {
	args := m.Called(ctx, referenceIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.Notification), args.Error(1)
}

func (m *MockNotificationRepository) BulkCreate(ctx context.Context, notifications []*interfaces.Notification) error {
	args := m.Called(ctx, notifications)
	return args.Error(0)
}

func (m *MockNotificationRepository) BulkDelete(ctx context.Context, ids []string) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockNotificationRepository) BulkDeleteByUserID(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockNotificationRepository) BulkDeleteByType(ctx context.Context, userID, notificationType string) error {
	args := m.Called(ctx, userID, notificationType)
	return args.Error(0)
}

func (m *MockNotificationRepository) BulkDeleteByReference(ctx context.Context, referenceID string) error {
	args := m.Called(ctx, referenceID)
	return args.Error(0)
}

func (m *MockNotificationRepository) BulkDeleteOlderThan(ctx context.Context, userID string, before time.Time) error {
	args := m.Called(ctx, userID, before)
	return args.Error(0)
}

func (m *MockNotificationRepository) BulkDeleteOlderThanAll(ctx context.Context, before time.Time) error {
	args := m.Called(ctx, before)
	return args.Error(0)
}

func (m *MockNotificationRepository) BulkMarkAsRead(ctx context.Context, ids []string) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockNotificationRepository) BulkMarkAsUnread(ctx context.Context, ids []string) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockNotificationRepository) GetNotificationStats(ctx context.Context) (*interfaces.NotificationStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.NotificationStats), args.Error(1)
}

func (m *MockNotificationRepository) GetUserNotificationStats(ctx context.Context, userID string) (*interfaces.NotificationStats, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.NotificationStats), args.Error(1)
}

func (m *MockNotificationRepository) GetDailyNotificationStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailyNotificationCount, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.DailyNotificationCount), args.Error(1)
}

func (m *MockNotificationRepository) GetDailyNotificationStatsForUser(ctx context.Context, userID string, start, end time.Time) ([]*interfaces.DailyNotificationCount, error) {
	args := m.Called(ctx, userID, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.DailyNotificationCount), args.Error(1)
}

func (m *MockNotificationRepository) GetNotificationTypeStats(ctx context.Context, userID string) ([]*interfaces.NotificationTypeStat, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.NotificationTypeStat), args.Error(1)
}

func (m *MockNotificationRepository) GetNotificationTrends(ctx context.Context, userID string, days int) ([]*interfaces.TrendData, error) {
	args := m.Called(ctx, userID, days)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.TrendData), args.Error(1)
}

func (m *MockNotificationRepository) GetAverageResponseTime(ctx context.Context, userID string) (float64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockNotificationRepository) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.NotificationRepository {
	args := m.Called(ctx, tx)
	return args.Get(0).(interfaces.NotificationRepository)
}

func (m *MockNotificationRepository) Transaction(ctx context.Context, fn func(txRepo interfaces.NotificationRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockNotificationRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockNotificationRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNotificationRepository) GetRawDB() interface{} {
	args := m.Called()
	return args.Get(0)
}

// MockEmailAdapter implements adapter.EmailAdapter.
type MockEmailAdapter struct {
	mock.Mock
}

func (m *MockEmailAdapter) Send(ctx context.Context, msg *adapter.EmailMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockEmailAdapter) SendTemplate(ctx context.Context, templateName string, data map[string]interface{}, to []string, subject string) error {
	args := m.Called(ctx, templateName, data, to, subject)
	return args.Error(0)
}

func (m *MockEmailAdapter) SendBatch(ctx context.Context, msgs []*adapter.EmailMessage) error {
	args := m.Called(ctx, msgs)
	return args.Error(0)
}

func (m *MockEmailAdapter) Queue(ctx context.Context, msg *adapter.EmailMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockEmailAdapter) SetProvider(provider adapter.EmailProvider) {
	m.Called(provider)
}

func (m *MockEmailAdapter) GetProvider() adapter.EmailProvider {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(adapter.EmailProvider)
}

func (m *MockEmailAdapter) Close() error {
	args := m.Called()
	return args.Error(0)
}

// ======================================================================
// Tests
// ======================================================================

func TestAuthService_Register(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		req           *dto.RegisterRequest
		mockSetup     func(*MockUserRepository, *MockEmailAdapter)
		expectError   bool
		expectedError error
	}{
		{
			name: "successful registration",
			req: &dto.RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "Test@1234",
				FullName: "Test User",
			},
			mockSetup: func(m *MockUserRepository, e *MockEmailAdapter) {
				m.On("ExistsByUsername", mock.Anything, "testuser").Return(false, nil)
				m.On("ExistsByEmail", mock.Anything, "test@example.com").Return(false, nil)
				m.On("Create", mock.Anything, mock.Anything).Return(nil)
				m.On("GetByID", mock.Anything, mock.Anything).Return(&entities.User{
					ID:        "user123",
					Username:  "testuser",
					Email:     "test@example.com",
					FullName:  "Test User",
					IsActive:  true,
				}, nil)
			},
			expectError: false,
		},
		{
			name: "duplicate username",
			req: &dto.RegisterRequest{
				Username: "existinguser",
				Email:    "test@example.com",
				Password: "Test@1234",
				FullName: "Test User",
			},
			mockSetup: func(m *MockUserRepository, e *MockEmailAdapter) {
				m.On("ExistsByUsername", mock.Anything, "existinguser").Return(true, nil)
			},
			expectError:   true,
			expectedError: service.ErrDuplicateUsername,
		},
		{
			name: "invalid email",
			req: &dto.RegisterRequest{
				Username: "testuser",
				Email:    "invalid-email",
				Password: "Test@1234",
				FullName: "Test User",
			},
			mockSetup:   func(m *MockUserRepository, e *MockEmailAdapter) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(MockUserRepository)
			sessionRepo := new(MockSessionRepository)
			notifRepo := new(MockNotificationRepository)
			emailAdapter := new(MockEmailAdapter)

			tt.mockSetup(userRepo, emailAdapter)

			authService := service.NewAuthService(
				userRepo,
				sessionRepo,
				notifRepo,
				emailAdapter,
				nil, // redisAdapter
				"test-secret",
				15*time.Minute,
				7*24*time.Hour,
				5,
				15*time.Minute,
			)

			_, err := authService.Register(ctx, tt.req)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.Equal(t, tt.expectedError, err)
				}
			} else {
				assert.NoError(t, err)
			}

			userRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("Test@1234"), bcrypt.DefaultCost)

	tests := []struct {
		name          string
		req           *dto.LoginRequest
		mockSetup     func(*MockUserRepository, *MockSessionRepository)
		expectError   bool
		expectedError error
	}{
		{
			name: "successful login",
			req: &dto.LoginRequest{
				Identifier: "testuser",
				Password:   "Test@1234",
			},
			mockSetup: func(m *MockUserRepository, s *MockSessionRepository) {
				m.On("GetByUsernameOrEmail", mock.Anything, "testuser").Return(&entities.User{
					ID:           "user123",
					Username:     "testuser",
					Email:        "test@example.com",
					PasswordHash: string(hashedPassword),
					FullName:     "Test User",
					IsActive:     true,
					IsSuspended:  false,
				}, nil)
				m.On("UpdateLastActive", mock.Anything, "user123").Return(nil)
				s.On("Create", mock.Anything, mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name: "invalid credentials",
			req: &dto.LoginRequest{
				Identifier: "testuser",
				Password:   "wrongpassword",
			},
			mockSetup: func(m *MockUserRepository, s *MockSessionRepository) {
				m.On("GetByUsernameOrEmail", mock.Anything, "testuser").Return(&entities.User{
					ID:           "user123",
					Username:     "testuser",
					Email:        "test@example.com",
					PasswordHash: string(hashedPassword),
					FullName:     "Test User",
					IsActive:     true,
					IsSuspended:  false,
				}, nil)
			},
			expectError:   true,
			expectedError: service.ErrInvalidCredentials,
		},
		{
			name: "user not found",
			req: &dto.LoginRequest{
				Identifier: "nonexistent",
				Password:   "Test@1234",
			},
			mockSetup: func(m *MockUserRepository, s *MockSessionRepository) {
				m.On("GetByUsernameOrEmail", mock.Anything, "nonexistent").Return(nil, interfaces.ErrUserNotFound)
			},
			expectError:   true,
			expectedError: service.ErrInvalidCredentials,
		},
		{
			name: "suspended user",
			req: &dto.LoginRequest{
				Identifier: "suspendeduser",
				Password:   "Test@1234",
			},
			mockSetup: func(m *MockUserRepository, s *MockSessionRepository) {
				m.On("GetByUsernameOrEmail", mock.Anything, "suspendeduser").Return(&entities.User{
					ID:           "user456",
					Username:     "suspendeduser",
					Email:        "suspended@example.com",
					PasswordHash: string(hashedPassword),
					FullName:     "Suspended User",
					IsActive:     false,
					IsSuspended:  true,
				}, nil)
			},
			expectError:   true,
			expectedError: service.ErrUserSuspended,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(MockUserRepository)
			sessionRepo := new(MockSessionRepository)
			notifRepo := new(MockNotificationRepository)
			emailAdapter := new(MockEmailAdapter)

			tt.mockSetup(userRepo, sessionRepo)

			authService := service.NewAuthService(
				userRepo,
				sessionRepo,
				notifRepo,
				emailAdapter,
				nil,
				"test-secret",
				15*time.Minute,
				7*24*time.Hour,
				5,
				15*time.Minute,
			)

			_, err := authService.Login(ctx, tt.req)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.Equal(t, tt.expectedError, err)
				}
			} else {
				assert.NoError(t, err)
			}

			userRepo.AssertExpectations(t)
			sessionRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		refreshToken  string
		mockSetup     func(*MockSessionRepository, *MockUserRepository)
		expectError   bool
		expectedError error
	}{
		{
			name:         "successful refresh",
			refreshToken: "valid_refresh_token",
			mockSetup: func(s *MockSessionRepository, u *MockUserRepository) {
				s.On("GetByRefreshToken", mock.Anything, "valid_refresh_token").Return(&interfaces.Session{
					ID:         "session1",
					UserID:     "user123",
					ExpiresAt:  time.Now().Add(7 * 24 * time.Hour),
				}, nil)
				u.On("GetByID", mock.Anything, "user123").Return(&entities.User{
					ID:        "user123",
					Username:  "testuser",
					IsActive:  true,
					IsSuspended: false,
				}, nil)
				s.On("Update", mock.Anything, mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name:         "expired refresh token",
			refreshToken: "expired_token",
			mockSetup: func(s *MockSessionRepository, u *MockUserRepository) {
				s.On("GetByRefreshToken", mock.Anything, "expired_token").Return(&interfaces.Session{
					ID:         "session2",
					UserID:     "user123",
					ExpiresAt:  time.Now().Add(-1 * time.Hour),
				}, nil)
			},
			expectError:   true,
			expectedError: service.ErrTokenExpired,
		},
		{
			name:         "invalid refresh token",
			refreshToken: "invalid_token",
			mockSetup: func(s *MockSessionRepository, u *MockUserRepository) {
				s.On("GetByRefreshToken", mock.Anything, "invalid_token").Return(nil, interfaces.ErrSessionNotFound)
			},
			expectError:   true,
			expectedError: service.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := new(MockSessionRepository)
			userRepo := new(MockUserRepository)
			notifRepo := new(MockNotificationRepository)
			emailAdapter := new(MockEmailAdapter)

			tt.mockSetup(sessionRepo, userRepo)

			authService := service.NewAuthService(
				userRepo,
				sessionRepo,
				notifRepo,
				emailAdapter,
				nil,
				"test-secret",
				15*time.Minute,
				7*24*time.Hour,
				5,
				15*time.Minute,
			)

			_, err := authService.RefreshToken(ctx, tt.refreshToken)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.Equal(t, tt.expectedError, err)
				}
			} else {
				assert.NoError(t, err)
			}

			sessionRepo.AssertExpectations(t)
			userRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_Logout(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		refreshToken  string
		mockSetup     func(*MockSessionRepository)
		expectError   bool
	}{
		{
			name:         "successful logout",
			refreshToken: "valid_token",
			mockSetup: func(s *MockSessionRepository) {
				s.On("GetByRefreshToken", mock.Anything, "valid_token").Return(&interfaces.Session{
					ID: "session1",
				}, nil)
				s.On("Delete", mock.Anything, "session1").Return(nil)
			},
			expectError: false,
		},
		{
			name:         "logout with invalid token",
			refreshToken: "invalid_token",
			mockSetup: func(s *MockSessionRepository) {
				s.On("GetByRefreshToken", mock.Anything, "invalid_token").Return(nil, interfaces.ErrSessionNotFound)
			},
			expectError: false, // Should not error on invalid token
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionRepo := new(MockSessionRepository)
			userRepo := new(MockUserRepository)
			notifRepo := new(MockNotificationRepository)
			emailAdapter := new(MockEmailAdapter)

			tt.mockSetup(sessionRepo)

			authService := service.NewAuthService(
				userRepo,
				sessionRepo,
				notifRepo,
				emailAdapter,
				nil,
				"test-secret",
				15*time.Minute,
				7*24*time.Hour,
				5,
				15*time.Minute,
			)

			err := authService.Logout(ctx, tt.refreshToken)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			sessionRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_ChangePassword(t *testing.T) {
	ctx := context.Background()
	oldPassword := "Test@1234"
	newPassword := "NewTest@5678"
	hashedOld, _ := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)

	tests := []struct {
		name          string
		userID        string
		oldPass       string
		newPass       string
		mockSetup     func(*MockUserRepository)
		expectError   bool
		expectedError error
	}{
		{
			name:    "successful password change",
			userID:  "user123",
			oldPass: oldPassword,
			newPass: newPassword,
			mockSetup: func(m *MockUserRepository) {
				m.On("GetByID", mock.Anything, "user123").Return(&entities.User{
					ID:           "user123",
					Username:     "testuser",
					PasswordHash: string(hashedOld),
					IsActive:     true,
					IsSuspended:  false,
				}, nil)
				m.On("Update", mock.Anything, mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name:    "incorrect old password",
			userID:  "user123",
			oldPass: "wrongpassword",
			newPass: newPassword,
			mockSetup: func(m *MockUserRepository) {
				m.On("GetByID", mock.Anything, "user123").Return(&entities.User{
					ID:           "user123",
					Username:     "testuser",
					PasswordHash: string(hashedOld),
					IsActive:     true,
					IsSuspended:  false,
				}, nil)
			},
			expectError:   true,
			expectedError: service.ErrInvalidCredentials,
		},
		{
			name:    "weak new password",
			userID:  "user123",
			oldPass: oldPassword,
			newPass: "weak",
			mockSetup: func(m *MockUserRepository) {
				m.On("GetByID", mock.Anything, "user123").Return(&entities.User{
					ID:           "user123",
					Username:     "testuser",
					PasswordHash: string(hashedOld),
					IsActive:     true,
					IsSuspended:  false,
				}, nil)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := new(MockUserRepository)
			sessionRepo := new(MockSessionRepository)
			notifRepo := new(MockNotificationRepository)
			emailAdapter := new(MockEmailAdapter)

			tt.mockSetup(userRepo)

			authService := service.NewAuthService(
				userRepo,
				sessionRepo,
				notifRepo,
				emailAdapter,
				nil,
				"test-secret",
				15*time.Minute,
				7*24*time.Hour,
				5,
				15*time.Minute,
			)

			err := authService.ChangePassword(ctx, tt.userID, tt.oldPass, tt.newPass)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.Equal(t, tt.expectedError, err)
				}
			} else {
				assert.NoError(t, err)
			}

			userRepo.AssertExpectations(t)
		})
	}
}