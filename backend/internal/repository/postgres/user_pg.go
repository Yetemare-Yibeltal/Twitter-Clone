// backend/internal/repository/postgres/user_pg.go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/domain/valueobjects"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// userRepo is the PostgreSQL implementation of UserRepository.
type userRepo struct {
	db     *sqlx.DB
	tx     *sqlx.Tx // optional transaction
	log    *logrus.Entry
	cached bool // could be extended with caching
}

// NewUserRepository creates a new PostgreSQL user repository.
func NewUserRepository(db *sqlx.DB) interfaces.UserRepository {
	return &userRepo{
		db:  db,
		log: logger.WithField("repository", "user_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *userRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.UserRepository {
	// Convert *sql.Tx to *sqlx.Tx
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &userRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *userRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.UserRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &userRepo{
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

// getDB returns the current DB connection (either the transaction or the main DB).
func (r *userRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ============ CRUD operations ============

func (r *userRepo) Create(ctx context.Context, user *entities.User) error {
	query := `
		INSERT INTO users (
			id, username, email, password_hash, full_name, bio, avatar_url,
			is_verified, is_suspended, is_active, role, last_active,
			tweet_count, follower_count, following_count, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		user.ID, user.Username, user.Email, user.PasswordHash, user.FullName,
		user.Bio, user.AvatarURL, user.IsVerified, user.IsSuspended,
		user.IsActive, user.Role, user.LastActive, user.TweetCount,
		user.FollowerCount, user.FollowingCount, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			switch pgErr.Code {
			case "23505": // unique violation
				if strings.Contains(pgErr.Message, "username") {
					return interfaces.ErrDuplicateUsername
				}
				if strings.Contains(pgErr.Message, "email") {
					return interfaces.ErrDuplicateEmail
				}
				return interfaces.ErrUserAlreadyExists
			}
		}
		return fmt.Errorf("create user failed: %w", err)
	}
	return nil
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*entities.User, error) {
	query := `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`
	var user entities.User
	err := r.getDB().GetContext(ctx, &user, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by ID failed: %w", err)
	}
	return &user, nil
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (*entities.User, error) {
	query := `SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL`
	var user entities.User
	err := r.getDB().GetContext(ctx, &user, query, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by username failed: %w", err)
	}
	return &user, nil
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*entities.User, error) {
	query := `SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL`
	var user entities.User
	err := r.getDB().GetContext(ctx, &user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email failed: %w", err)
	}
	return &user, nil
}

func (r *userRepo) GetByUsernameOrEmail(ctx context.Context, identifier string) (*entities.User, error) {
	query := `SELECT * FROM users WHERE (username = $1 OR email = $2) AND deleted_at IS NULL`
	var user entities.User
	err := r.getDB().GetContext(ctx, &user, query, identifier, identifier)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by username or email failed: %w", err)
	}
	return &user, nil
}

func (r *userRepo) GetByIDs(ctx context.Context, ids []string) ([]*entities.User, error) {
	if len(ids) == 0 {
		return []*entities.User{}, nil
	}
	query, args, err := sqlx.In(`SELECT * FROM users WHERE id IN (?) AND deleted_at IS NULL`, ids)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var users []*entities.User
	err = r.getDB().SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get users by IDs failed: %w", err)
	}
	return users, nil
}

func (r *userRepo) Update(ctx context.Context, user *entities.User) error {
	query := `
		UPDATE users SET
			username = $1, email = $2, password_hash = $3, full_name = $4,
			bio = $5, avatar_url = $6, is_verified = $7, is_suspended = $8,
			is_active = $9, role = $10, last_active = $11,
			tweet_count = $12, follower_count = $13, following_count = $14,
			updated_at = $15
		WHERE id = $16 AND deleted_at IS NULL
	`
	result, err := r.getDB().ExecContext(ctx, query,
		user.Username, user.Email, user.PasswordHash, user.FullName,
		user.Bio, user.AvatarURL, user.IsVerified, user.IsSuspended,
		user.IsActive, user.Role, user.LastActive, user.TweetCount,
		user.FollowerCount, user.FollowingCount, time.Now(), user.ID,
	)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			if strings.Contains(pgErr.Message, "username") {
				return interfaces.ErrDuplicateUsername
			}
			if strings.Contains(pgErr.Message, "email") {
				return interfaces.ErrDuplicateEmail
			}
		}
		return fmt.Errorf("update user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	// Add updated_at automatically
	fields["updated_at"] = time.Now()
	var setParts []string
	var args []interface{}
	i := 1
	for k, v := range fields {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, v)
		i++
	}
	query := fmt.Sprintf(`UPDATE users SET %s WHERE id = $%d AND deleted_at IS NULL`,
		strings.Join(setParts, ", "), i)
	args = append(args, id)
	result, err := r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update fields failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL`
	now := time.Now()
	result, err := r.getDB().ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("soft delete failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) Restore(ctx context.Context, id string) error {
	query := `UPDATE users SET deleted_at = NULL WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("restore user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrUserNotFound
	}
	return nil
}

// ============ Listing, filtering, search ============

func (r *userRepo) List(ctx context.Context, filter *interfaces.UserFilter, pagination *interfaces.PaginationOptions) ([]*entities.User, int64, error) {
	var whereClauses []string
	var args []interface{}
	argIdx := 1

	// Base condition: not soft-deleted
	whereClauses = append(whereClauses, "deleted_at IS NULL")

	if filter != nil {
		if filter.Username != nil && *filter.Username != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("username ILIKE $%d", argIdx))
			args = append(args, "%"+*filter.Username+"%")
			argIdx++
		}
		if filter.Email != nil && *filter.Email != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("email ILIKE $%d", argIdx))
			args = append(args, "%"+*filter.Email+"%")
			argIdx++
		}
		if filter.FullName != nil && *filter.FullName != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("full_name ILIKE $%d", argIdx))
			args = append(args, "%"+*filter.FullName+"%")
			argIdx++
		}
		if filter.Bio != nil && *filter.Bio != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("bio ILIKE $%d", argIdx))
			args = append(args, "%"+*filter.Bio+"%")
			argIdx++
		}
		if filter.IsActive != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("is_active = $%d", argIdx))
			args = append(args, *filter.IsActive)
			argIdx++
		}
		if filter.IsVerified != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("is_verified = $%d", argIdx))
			args = append(args, *filter.IsVerified)
			argIdx++
		}
		if filter.IsSuspended != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("is_suspended = $%d", argIdx))
			args = append(args, *filter.IsSuspended)
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
		if filter.Role != nil && *filter.Role != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("role = $%d", argIdx))
			args = append(args, *filter.Role)
			argIdx++
		}
		if filter.Search != nil && *filter.Search != "" {
			// Full‑text search using tsvector (assuming GIN index)
			whereClauses = append(whereClauses, fmt.Sprintf(`to_tsvector('english', username || ' ' || email || ' ' || full_name || ' ' || COALESCE(bio,'')) @@ plainto_tsquery('english', $%d)`, argIdx))
			args = append(args, *filter.Search)
			argIdx++
		}
	}

	// Build WHERE clause
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereSQL)
	var total int64
	err := r.getDB().GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("count users failed: %w", err)
	}

	// Set pagination defaults
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

	// Validate sort column (prevent SQL injection)
	allowedSort := map[string]bool{
		"created_at": true, "updated_at": true, "username": true,
		"full_name": true, "email": true, "tweet_count": true,
		"follower_count": true,
	}
	if !allowedSort[sortBy] {
		sortBy = "created_at"
	}
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	// Final query with order and pagination
	query := fmt.Sprintf("SELECT * FROM users %s ORDER BY %s %s LIMIT $%d OFFSET $%d", whereSQL, sortBy, order, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var users []*entities.User
	err = r.getDB().SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users failed: %w", err)
	}
	return users, total, nil
}

func (r *userRepo) Count(ctx context.Context, filter *interfaces.UserFilter) (int64, error) {
	// Reuse list's filter logic but only count
	_, total, err := r.List(ctx, filter, &interfaces.PaginationOptions{Limit: 1})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (r *userRepo) Search(ctx context.Context, query string, pagination *interfaces.PaginationOptions) ([]*entities.User, int64, error) {
	filter := &interfaces.UserFilter{
		Search: &query,
	}
	return r.List(ctx, filter, pagination)
}

// ============ Existence checks ============

func (r *userRepo) Exists(ctx context.Context, id string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, id)
	if err != nil {
		return false, fmt.Errorf("exists check failed: %w", err)
	}
	return exists, nil
}

func (r *userRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, username)
	if err != nil {
		return false, fmt.Errorf("exists by username failed: %w", err)
	}
	return exists, nil
}

func (r *userRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, email)
	if err != nil {
		return false, fmt.Errorf("exists by email failed: %w", err)
	}
	return exists, nil
}

// ============ Stats ============

func (r *userRepo) GetStats(ctx context.Context, userID string) (*interfaces.UserStats, error) {
	query := `
		SELECT 
			u.id AS user_id,
			u.tweet_count,
			u.follower_count,
			u.following_count,
			COUNT(DISTINCT l.id) AS total_likes,
			COUNT(DISTINCT rt.id) AS total_retweets,
			COUNT(DISTINCT r.id) AS total_replies,
			COUNT(DISTINCT b.id) AS total_bookmarks,
			u.created_at AS joined_at,
			u.last_active
		FROM users u
		LEFT JOIN tweets t ON t.user_id = u.id AND t.deleted_at IS NULL
		LEFT JOIN likes l ON l.tweet_id = t.id
		LEFT JOIN retweets rt ON rt.tweet_id = t.id
		LEFT JOIN tweets r ON r.parent_tweet_id = t.id AND r.deleted_at IS NULL
		LEFT JOIN bookmarks b ON b.tweet_id = t.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
		GROUP BY u.id
	`
	var stats interfaces.UserStats
	err := r.getDB().GetContext(ctx, &stats, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrUserNotFound
		}
		return nil, fmt.Errorf("get stats failed: %w", err)
	}
	return &stats, nil
}

func (r *userRepo) GetStatsForUsers(ctx context.Context, userIDs []string) (map[string]*interfaces.UserStats, error) {
	if len(userIDs) == 0 {
		return map[string]*interfaces.UserStats{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT 
			u.id AS user_id,
			u.tweet_count,
			u.follower_count,
			u.following_count,
			COUNT(DISTINCT l.id) AS total_likes,
			COUNT(DISTINCT rt.id) AS total_retweets,
			COUNT(DISTINCT r.id) AS total_replies,
			COUNT(DISTINCT b.id) AS total_bookmarks,
			u.created_at AS joined_at,
			u.last_active
		FROM users u
		LEFT JOIN tweets t ON t.user_id = u.id AND t.deleted_at IS NULL
		LEFT JOIN likes l ON l.tweet_id = t.id
		LEFT JOIN retweets rt ON rt.tweet_id = t.id
		LEFT JOIN tweets r ON r.parent_tweet_id = t.id AND r.deleted_at IS NULL
		LEFT JOIN bookmarks b ON b.tweet_id = t.id
		WHERE u.id IN (?) AND u.deleted_at IS NULL
		GROUP BY u.id
	`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var statsList []interfaces.UserStats
	err = r.getDB().SelectContext(ctx, &statsList, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get stats for users failed: %w", err)
	}
	result := make(map[string]*interfaces.UserStats)
	for i := range statsList {
		result[statsList[i].UserID] = &statsList[i]
	}
	return result, nil
}

// ============ Activity logging ============

func (r *userRepo) RecordActivity(ctx context.Context, activity *interfaces.UserActivity) error {
	query := `
		INSERT INTO user_activities (id, user_id, activity_type, reference_id, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	metadataJSON, err := json.Marshal(activity.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	_, err = r.getDB().ExecContext(ctx, query,
		activity.ID, activity.UserID, activity.ActivityType,
		activity.ReferenceID, metadataJSON, activity.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("record activity failed: %w", err)
	}
	return nil
}

func (r *userRepo) GetUserActivities(ctx context.Context, userID string, limit, offset int) ([]*interfaces.UserActivity, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	// Count total
	countQuery := `SELECT COUNT(*) FROM user_activities WHERE user_id = $1`
	var total int64
	err := r.getDB().GetContext(ctx, &total, countQuery, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("count activities failed: %w", err)
	}
	// Select with limit/offset
	query := `
		SELECT id, user_id, activity_type, reference_id, metadata, created_at
		FROM user_activities
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	var activities []*interfaces.UserActivity
	err = r.getDB().SelectContext(ctx, &activities, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get activities failed: %w", err)
	}
	// Unmarshal metadata
	for _, act := range activities {
		if act.Metadata == nil {
			act.Metadata = make(map[string]interface{})
		}
	}
	return activities, total, nil
}

// ============ Atomic counters ============

func (r *userRepo) UpdateLastActive(ctx context.Context, userID string) error {
	query := `UPDATE users SET last_active = $1 WHERE id = $2 AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), userID)
	return err
}

func (r *userRepo) IncrementTweetCount(ctx context.Context, userID string) error {
	query := `UPDATE users SET tweet_count = tweet_count + 1 WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) DecrementTweetCount(ctx context.Context, userID string) error {
	query := `UPDATE users SET tweet_count = GREATEST(tweet_count - 1, 0) WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	return err
}

func (r *userRepo) IncrementFollowerCount(ctx context.Context, userID string) error {
	query := `UPDATE users SET follower_count = follower_count + 1 WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	return err
}

func (r *userRepo) DecrementFollowerCount(ctx context.Context, userID string) error {
	query := `UPDATE users SET follower_count = GREATEST(follower_count - 1, 0) WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	return err
}

func (r *userRepo) IncrementFollowingCount(ctx context.Context, userID string) error {
	query := `UPDATE users SET following_count = following_count + 1 WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	return err
}

func (r *userRepo) DecrementFollowingCount(ctx context.Context, userID string) error {
	query := `UPDATE users SET following_count = GREATEST(following_count - 1, 0) WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	return err
}

// ============ Admin operations ============

func (r *userRepo) UpdateVerificationStatus(ctx context.Context, userID string, verified bool) error {
	query := `UPDATE users SET is_verified = $1 WHERE id = $2 AND deleted_at IS NULL`
	result, err := r.getDB().ExecContext(ctx, query, verified, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) UpdateSuspensionStatus(ctx context.Context, userID string, suspended bool, reason string) error {
	query := `UPDATE users SET is_suspended = $1, suspended_reason = $2 WHERE id = $3 AND deleted_at IS NULL`
	result, err := r.getDB().ExecContext(ctx, query, suspended, reason, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrUserNotFound
	}
	return nil
}

// ============ Bulk operations ============

func (r *userRepo) BulkCreate(ctx context.Context, users []*entities.User) error {
	if len(users) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO users (id, username, email, password_hash, full_name, bio, avatar_url,
			is_verified, is_suspended, is_active, role, last_active,
			tweet_count, follower_count, following_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, u := range users {
		_, err := stmt.ExecContext(ctx,
			u.ID, u.Username, u.Email, u.PasswordHash, u.FullName,
			u.Bio, u.AvatarURL, u.IsVerified, u.IsSuspended,
			u.IsActive, u.Role, u.LastActive, u.TweetCount,
			u.FollowerCount, u.FollowingCount, u.CreatedAt, u.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("bulk create insert failed: %w", err)
		}
	}
	return tx.Commit()
}

func (r *userRepo) BulkUpdate(ctx context.Context, users []*entities.User) error {
	if len(users) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		UPDATE users SET
			username = $1, email = $2, password_hash = $3, full_name = $4,
			bio = $5, avatar_url = $6, is_verified = $7, is_suspended = $8,
			is_active = $9, role = $10, last_active = $11,
			tweet_count = $12, follower_count = $13, following_count = $14,
			updated_at = $15
		WHERE id = $16 AND deleted_at IS NULL
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, u := range users {
		_, err := stmt.ExecContext(ctx,
			u.Username, u.Email, u.PasswordHash, u.FullName,
			u.Bio, u.AvatarURL, u.IsVerified, u.IsSuspended,
			u.IsActive, u.Role, u.LastActive, u.TweetCount,
			u.FollowerCount, u.FollowingCount, time.Now(), u.ID,
		)
		if err != nil {
			return fmt.Errorf("bulk update failed: %w", err)
		}
	}
	return tx.Commit()
}

func (r *userRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM users WHERE id IN (?) AND deleted_at IS NULL`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	return err
}

// ============ Specialized queries ============

func (r *userRepo) GetRecentlyJoined(ctx context.Context, duration time.Duration, limit int) ([]*entities.User, error) {
	since := time.Now().Add(-duration)
	query := `SELECT * FROM users WHERE created_at >= $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2`
	var users []*entities.User
	err := r.getDB().SelectContext(ctx, &users, query, since, limit)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepo) GetActiveUsers(ctx context.Context, duration time.Duration, limit int) ([]*entities.User, error) {
	since := time.Now().Add(-duration)
	query := `SELECT * FROM users WHERE last_active >= $1 AND deleted_at IS NULL AND is_active = true ORDER BY last_active DESC LIMIT $2`
	var users []*entities.User
	err := r.getDB().SelectContext(ctx, &users, query, since, limit)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepo) GetTopUsersByFollowers(ctx context.Context, limit int) ([]*entities.User, error) {
	query := `SELECT * FROM users WHERE deleted_at IS NULL ORDER BY follower_count DESC LIMIT $1`
	var users []*entities.User
	err := r.getDB().SelectContext(ctx, &users, query, limit)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepo) GetTopUsersByTweets(ctx context.Context, limit int) ([]*entities.User, error) {
	query := `SELECT * FROM users WHERE deleted_at IS NULL ORDER BY tweet_count DESC LIMIT $1`
	var users []*entities.User
	err := r.getDB().SelectContext(ctx, &users, query, limit)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userRepo) GetUsersWithRole(ctx context.Context, role string, pagination *interfaces.PaginationOptions) ([]*entities.User, int64, error) {
	filter := &interfaces.UserFilter{Role: &role}
	return r.List(ctx, filter, pagination)
}

// ============ Health and utilities ============

func (r *userRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *userRepo) Close() error {
	// For sqlx, closing is handled by the main DB; we don't close here.
	return nil
}

func (r *userRepo) GetRawDB() interface{} {
	return r.db
}