// backend/internal/repository/postgres/follow_pg.go
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

	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// followRepo is the PostgreSQL implementation of FollowRepository.
type followRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewFollowRepository creates a new PostgreSQL follow repository.
func NewFollowRepository(db *sqlx.DB) interfaces.FollowRepository {
	return &followRepo{
		db:  db,
		log: logger.WithField("repository", "follow_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *followRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.FollowRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &followRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *followRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.FollowRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &followRepo{
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
func (r *followRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic CRUD
// ======================================================================

// Create inserts a new follow relationship.
func (r *followRepo) Create(ctx context.Context, follow *interfaces.Follow) error {
	query := `
		INSERT INTO follows (id, follower_id, followee_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		follow.ID, follow.FollowerID, follow.FolloweeID,
		follow.Status, follow.CreatedAt, follow.UpdatedAt,
	)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return interfaces.ErrAlreadyFollowing
		}
		return fmt.Errorf("create follow failed: %w", err)
	}
	return nil
}

// GetByID retrieves a follow by its ID.
func (r *followRepo) GetByID(ctx context.Context, id string) (*interfaces.Follow, error) {
	query := `SELECT * FROM follows WHERE id = $1 AND deleted_at IS NULL`
	var follow interfaces.Follow
	err := r.getDB().GetContext(ctx, &follow, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrFollowNotFound
		}
		return nil, fmt.Errorf("get follow by ID failed: %w", err)
	}
	return &follow, nil
}

// GetByFollowerAndFollowee retrieves a follow relationship.
func (r *followRepo) GetByFollowerAndFollowee(ctx context.Context, followerID, followeeID string) (*interfaces.Follow, error) {
	query := `SELECT * FROM follows WHERE follower_id = $1 AND followee_id = $2 AND deleted_at IS NULL`
	var follow interfaces.Follow
	err := r.getDB().GetContext(ctx, &follow, query, followerID, followeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrFollowNotFound
		}
		return nil, fmt.Errorf("get follow by follower and followee failed: %w", err)
	}
	return &follow, nil
}

// Delete removes a follow relationship.
func (r *followRepo) Delete(ctx context.Context, followerID, followeeID string) error {
	query := `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("delete follow failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrFollowNotFound
	}
	return nil
}

// DeleteByID removes a follow by ID.
func (r *followRepo) DeleteByID(ctx context.Context, id string) error {
	query := `DELETE FROM follows WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete follow by ID failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrFollowNotFound
	}
	return nil
}

// UpdateStatus updates the status of a follow relationship.
func (r *followRepo) UpdateStatus(ctx context.Context, id string, status interfaces.FollowStatus) error {
	query := `UPDATE follows SET status = $1, updated_at = $2 WHERE id = $3`
	result, err := r.getDB().ExecContext(ctx, query, string(status), time.Now(), id)
	if err != nil {
		return fmt.Errorf("update follow status failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrFollowNotFound
	}
	return nil
}

// ======================================================================
// Existence Checks
// ======================================================================

// Exists checks if a follow relationship exists.
func (r *followRepo) Exists(ctx context.Context, followerID, followeeID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2 AND deleted_at IS NULL)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, followerID, followeeID)
	if err != nil {
		return false, fmt.Errorf("check follow existence failed: %w", err)
	}
	return exists, nil
}

// ExistsWithStatus checks if a follow relationship exists with a specific status.
func (r *followRepo) ExistsWithStatus(ctx context.Context, followerID, followeeID string, status interfaces.FollowStatus) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM follows
			WHERE follower_id = $1 AND followee_id = $2 AND status = $3 AND deleted_at IS NULL
		)
	`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, followerID, followeeID, string(status))
	if err != nil {
		return false, fmt.Errorf("check follow existence with status failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountFollowers returns the number of followers for a user.
func (r *followRepo) CountFollowers(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM follows WHERE followee_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count followers failed: %w", err)
	}
	return count, nil
}

// CountFollowersWithStatus returns the number of followers with a specific status.
func (r *followRepo) CountFollowersWithStatus(ctx context.Context, userID string, status interfaces.FollowStatus) (int64, error) {
	query := `SELECT COUNT(*) FROM follows WHERE followee_id = $1 AND status = $2 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, string(status))
	if err != nil {
		return 0, fmt.Errorf("count followers with status failed: %w", err)
	}
	return count, nil
}

// CountFollowing returns the number of users a user is following.
func (r *followRepo) CountFollowing(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM follows WHERE follower_id = $1 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count following failed: %w", err)
	}
	return count, nil
}

// CountFollowingWithStatus returns the number of following with a specific status.
func (r *followRepo) CountFollowingWithStatus(ctx context.Context, userID string, status interfaces.FollowStatus) (int64, error) {
	query := `SELECT COUNT(*) FROM follows WHERE follower_id = $1 AND status = $2 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, string(status))
	if err != nil {
		return 0, fmt.Errorf("count following with status failed: %w", err)
	}
	return count, nil
}

// CountMutual returns the number of mutual follows between two users.
func (r *followRepo) CountMutual(ctx context.Context, userID1, userID2 string) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM follows f1
		JOIN follows f2 ON f1.followee_id = f2.follower_id
		WHERE f1.follower_id = $1
		  AND f2.followee_id = $2
		  AND f1.deleted_at IS NULL
		  AND f2.deleted_at IS NULL
	`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID1, userID2)
	if err != nil {
		return 0, fmt.Errorf("count mutual follows failed: %w", err)
	}
	return count, nil
}

// CountPendingRequests returns the number of pending follow requests.
func (r *followRepo) CountPendingRequests(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM follows WHERE followee_id = $1 AND status = 'pending' AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count pending requests failed: %w", err)
	}
	return count, nil
}

// CountPendingRequestsFromUser returns pending requests from a specific user.
func (r *followRepo) CountPendingRequestsFromUser(ctx context.Context, userID, fromUserID string) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM follows
		WHERE followee_id = $1 AND follower_id = $2 AND status = 'pending' AND deleted_at IS NULL
	`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID, fromUserID)
	if err != nil {
		return 0, fmt.Errorf("count pending requests from user failed: %w", err)
	}
	return count, nil
}

// ======================================================================
// List Operations
// ======================================================================

// GetFollowers returns the list of followers for a user with pagination.
func (r *followRepo) GetFollowers(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE followee_id = $1 AND deleted_at IS NULL
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

	var follows []*interfaces.Follow
	err := r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get followers failed: %w", err)
	}
	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// GetFollowersWithStatus returns followers with a specific status.
func (r *followRepo) GetFollowersWithStatus(ctx context.Context, userID string, status interfaces.FollowStatus, cursor string, limit int) ([]*interfaces.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE followee_id = $1 AND status = $2 AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, string(status)}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var follows []*interfaces.Follow
	err := r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get followers with status failed: %w", err)
	}
	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// GetFollowing returns the list of users a user is following with pagination.
func (r *followRepo) GetFollowing(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE follower_id = $1 AND deleted_at IS NULL
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

	var follows []*interfaces.Follow
	err := r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get following failed: %w", err)
	}
	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// GetFollowingWithStatus returns following with a specific status.
func (r *followRepo) GetFollowingWithStatus(ctx context.Context, userID string, status interfaces.FollowStatus, cursor string, limit int) ([]*interfaces.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE follower_id = $1 AND status = $2 AND deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND id > $3`
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT $?`

	args := []interface{}{userID, string(status)}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var follows []*interfaces.Follow
	err := r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get following with status failed: %w", err)
	}
	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// GetPendingRequests returns pending follow requests for a user.
func (r *followRepo) GetPendingRequests(ctx context.Context, userID string, cursor string, limit int) ([]*interfaces.Follow, string, error) {
	return r.GetFollowersWithStatus(ctx, userID, interfaces.FollowStatusPending, cursor, limit)
}

// GetFollowerIDs returns all follower IDs for a user (no pagination).
func (r *followRepo) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT follower_id FROM follows WHERE followee_id = $1 AND deleted_at IS NULL`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get follower IDs failed: %w", err)
	}
	return ids, nil
}

// GetFollowingIDs returns all following IDs for a user (no pagination).
func (r *followRepo) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT followee_id FROM follows WHERE follower_id = $1 AND deleted_at IS NULL`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get following IDs failed: %w", err)
	}
	return ids, nil
}

// ======================================================================
// Mutual Follows
// ======================================================================

// AreMutual checks if two users follow each other.
func (r *followRepo) AreMutual(ctx context.Context, userID1, userID2 string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM follows f1
			JOIN follows f2 ON f1.followee_id = f2.follower_id
			WHERE f1.follower_id = $1 AND f2.followee_id = $2
			  AND f1.deleted_at IS NULL AND f2.deleted_at IS NULL
		)
	`
	var mutual bool
	err := r.getDB().GetContext(ctx, &mutual, query, userID1, userID2)
	if err != nil {
		return false, fmt.Errorf("check mutual follow failed: %w", err)
	}
	return mutual, nil
}

// GetMutualFollows returns the list of mutual follows between two users.
func (r *followRepo) GetMutualFollows(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT f1.followee_id as user_id
		FROM follows f1
		JOIN follows f2 ON f1.followee_id = f2.follower_id
		WHERE f1.follower_id = $1 AND f2.followee_id = $2
		  AND f1.deleted_at IS NULL AND f2.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND f1.followee_id > $3`
	}
	query += ` ORDER BY f1.created_at DESC, f1.followee_id DESC LIMIT $?`

	args := []interface{}{userID1, userID2}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get mutual follows failed: %w", err)
	}
	var nextCursor string
	if len(ids) == limit {
		nextCursor = ids[len(ids)-1]
	}
	return ids, nextCursor, nil
}

// GetMutualFollowsDetailed returns detailed mutual follow information.
func (r *followRepo) GetMutualFollowsDetailed(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]*interfaces.Follow, string, error) {
	ids, nextCursor, err := r.GetMutualFollows(ctx, userID1, userID2, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	if len(ids) == 0 {
		return []*interfaces.Follow{}, "", nil
	}
	query, args, err := sqlx.In(`SELECT * FROM follows WHERE followee_id IN (?) AND deleted_at IS NULL`, ids)
	if err != nil {
		return nil, "", fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var follows []*interfaces.Follow
	err = r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get mutual follows detailed failed: %w", err)
	}
	return follows, nextCursor, nil
}

// GetMutualCountsForUsers returns mutual counts for multiple users.
func (r *followRepo) GetMutualCountsForUsers(ctx context.Context, userID string, targetUserIDs []string) (map[string]int64, error) {
	if len(targetUserIDs) == 0 {
		return map[string]int64{}, nil
	}
	// This is a more complex query; we'll do it in a loop for simplicity
	counts := make(map[string]int64)
	for _, targetID := range targetUserIDs {
		count, err := r.CountMutual(ctx, userID, targetID)
		if err != nil {
			return nil, err
		}
		counts[targetID] = count
	}
	return counts, nil
}

// ======================================================================
// Follow Recommendations
// ======================================================================

// GetFollowRecommendations returns suggested users to follow.
func (r *followRepo) GetFollowRecommendations(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT DISTINCT f2.followee_id
		FROM follows f1
		JOIN follows f2 ON f1.followee_id = f2.follower_id
		WHERE f1.follower_id = $1
		  AND f2.followee_id != $1
		  AND f2.followee_id NOT IN (
		    SELECT followee_id FROM follows WHERE follower_id = $1 AND deleted_at IS NULL
		  )
		  AND f1.deleted_at IS NULL AND f2.deleted_at IS NULL
		ORDER BY RANDOM()
		LIMIT $2
	`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get follow recommendations failed: %w", err)
	}
	return ids, nil
}

// GetFollowRecommendationsWithScore returns recommendations with scores.
func (r *followRepo) GetFollowRecommendationsWithScore(ctx context.Context, userID string, limit int) ([]*interfaces.FollowRecommendation, error) {
	ids, err := r.GetFollowRecommendations(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	var recs []*interfaces.FollowRecommendation
	for _, id := range ids {
		user, err := r.getUserByID(ctx, id)
		if err != nil {
			continue
		}
		mutual, _ := r.CountMutual(ctx, userID, id)
		followerCount, _ := r.CountFollowers(ctx, id)
		recs = append(recs, &interfaces.FollowRecommendation{
			UserID:       id,
			Username:     user.Username,
			FullName:     user.FullName,
			AvatarURL:    user.AvatarURL,
			MutualCount:  mutual,
			FollowerCount: followerCount,
			Score:        float64(mutual)*2 + float64(followerCount)*0.5,
		})
	}
	return recs, nil
}

// GetPeopleAlsoFollow returns users also followed by followers.
func (r *followRepo) GetPeopleAlsoFollow(ctx context.Context, userID string, limit int) ([]string, error) {
	return r.GetFollowRecommendations(ctx, userID, limit)
}

// GetPopularUsers returns users with most followers (discovery).
func (r *followRepo) GetPopularUsers(ctx context.Context, limit int, excludeUserID string) ([]string, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT followee_id, COUNT(*) as cnt
		FROM follows
		WHERE deleted_at IS NULL
		GROUP BY followee_id
		ORDER BY cnt DESC
		LIMIT $1
	`
	var results []struct {
		FolloweeID string `db:"followee_id"`
		Count      int64  `db:"cnt"`
	}
	err := r.getDB().SelectContext(ctx, &results, query, limit+1)
	if err != nil {
		return nil, fmt.Errorf("get popular users failed: %w", err)
	}
	var ids []string
	for _, r := range results {
		if r.FolloweeID != excludeUserID && len(ids) < limit {
			ids = append(ids, r.FolloweeID)
		}
	}
	return ids, nil
}

// ======================================================================
// Bulk Operations
// ======================================================================

// BulkCreate inserts multiple follows in a single transaction.
func (r *followRepo) BulkCreate(ctx context.Context, follows []*interfaces.Follow) error {
	if len(follows) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO follows (id, follower_id, followee_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range follows {
		_, err := stmt.ExecContext(ctx,
			f.ID, f.FollowerID, f.FolloweeID, f.Status, f.CreatedAt, f.UpdatedAt)
		if err != nil {
			return fmt.Errorf("bulk create follow failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple follows in a single transaction.
func (r *followRepo) BulkDelete(ctx context.Context, followerIDs, followeeIDs []string) error {
	if len(followerIDs) == 0 || len(followeeIDs) == 0 {
		return nil
	}
	if len(followerIDs) != len(followeeIDs) {
		return errors.New("followerIDs and followeeIDs must have same length")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range followerIDs {
		_, err := stmt.ExecContext(ctx, followerIDs[i], followeeIDs[i])
		if err != nil {
			return fmt.Errorf("bulk delete follow failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDeleteByUserID removes all follows where the user is involved.
func (r *followRepo) BulkDeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM follows WHERE follower_id = $1 OR followee_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("bulk delete by user failed: %w", err)
	}
	return nil
}

// BulkDeleteByFollowerID removes all follows by a specific follower.
func (r *followRepo) BulkDeleteByFollowerID(ctx context.Context, followerID string) error {
	query := `DELETE FROM follows WHERE follower_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, followerID)
	if err != nil {
		return fmt.Errorf("bulk delete by follower failed: %w", err)
	}
	return nil
}

// BulkDeleteByFolloweeID removes all follows to a specific followee.
func (r *followRepo) BulkDeleteByFolloweeID(ctx context.Context, followeeID string) error {
	query := `DELETE FROM follows WHERE followee_id = $1`
	_, err := r.getDB().ExecContext(ctx, query, followeeID)
	if err != nil {
		return fmt.Errorf("bulk delete by followee failed: %w", err)
	}
	return nil
}

// BulkUpdateStatus updates status for multiple follows.
func (r *followRepo) BulkUpdateStatus(ctx context.Context, ids []string, status interfaces.FollowStatus) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`UPDATE follows SET status = ?, updated_at = ? WHERE id IN (?)`,
		string(status), time.Now(), ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk update status failed: %w", err)
	}
	return nil
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetFollowStats returns aggregated follow statistics.
func (r *followRepo) GetFollowStats(ctx context.Context) (*interfaces.FollowStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_follows,
			COUNT(DISTINCT follower_id) as unique_followers,
			COUNT(DISTINCT followee_id) as unique_followees,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_follows,
			SUM(CASE WHEN status = 'accepted' THEN 1 ELSE 0 END) as accepted_follows,
			SUM(CASE WHEN status = 'rejected' THEN 1 ELSE 0 END) as rejected_follows,
			SUM(CASE WHEN status = 'blocked' THEN 1 ELSE 0 END) as blocked_follows,
			MAX(created_at) as last_follow,
			MIN(created_at) as first_follow
		FROM follows
		WHERE deleted_at IS NULL
	`
	var stats interfaces.FollowStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get follow stats failed: %w", err)
	}
	return &stats, nil
}

// GetUserFollowStats returns follow statistics for a specific user.
func (r *followRepo) GetUserFollowStats(ctx context.Context, userID string) (*interfaces.FollowStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_follows,
			COUNT(DISTINCT follower_id) as unique_followers,
			COUNT(DISTINCT followee_id) as unique_followees,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_follows,
			SUM(CASE WHEN status = 'accepted' THEN 1 ELSE 0 END) as accepted_follows,
			SUM(CASE WHEN status = 'rejected' THEN 1 ELSE 0 END) as rejected_follows,
			SUM(CASE WHEN status = 'blocked' THEN 1 ELSE 0 END) as blocked_follows,
			MAX(created_at) as last_follow,
			MIN(created_at) as first_follow
		FROM follows
		WHERE (follower_id = $1 OR followee_id = $1) AND deleted_at IS NULL
	`
	var stats interfaces.FollowStats
	err := r.getDB().GetContext(ctx, &stats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user follow stats failed: %w", err)
	}
	return &stats, nil
}

// GetDailyFollowStats returns daily follow counts for a date range.
func (r *followRepo) GetDailyFollowStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailyFollowCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			COUNT(DISTINCT follower_id) as new_followers,
			COUNT(DISTINCT followee_id) as new_followees,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_count,
			SUM(CASE WHEN status = 'accepted' THEN 1 ELSE 0 END) as accepted_count
		FROM follows
		WHERE created_at >= $1 AND created_at <= $2 AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyFollowCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily follow stats failed: %w", err)
	}
	return results, nil
}

// GetFollowGrowthRate calculates follow growth rate over a period.
func (r *followRepo) GetFollowGrowthRate(ctx context.Context, userID string, days int) (float64, error) {
	startDate := time.Now().AddDate(0, 0, -days)
	var startCount, endCount int64
	err := r.getDB().GetContext(ctx, &startCount,
		`SELECT COUNT(*) FROM follows WHERE (follower_id = $1 OR followee_id = $1) AND created_at <= $2 AND deleted_at IS NULL`,
		userID, startDate)
	if err != nil {
		return 0, fmt.Errorf("get start count failed: %w", err)
	}
	err = r.getDB().GetContext(ctx, &endCount,
		`SELECT COUNT(*) FROM follows WHERE (follower_id = $1 OR followee_id = $1) AND deleted_at IS NULL`,
		userID)
	if err != nil {
		return 0, fmt.Errorf("get end count failed: %w", err)
	}
	if startCount == 0 {
		return float64(endCount), nil
	}
	return (float64(endCount-startCount) / float64(startCount)) * 100, nil
}

// GetTopFollowers returns users with the most followers (global).
func (r *followRepo) GetTopFollowers(ctx context.Context, limit int) ([]*interfaces.User, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT u.id, u.username, u.full_name, u.avatar_url,
		       COUNT(f.follower_id) as follower_count
		FROM users u
		JOIN follows f ON u.id = f.followee_id
		WHERE f.deleted_at IS NULL AND u.deleted_at IS NULL
		GROUP BY u.id
		ORDER BY follower_count DESC
		LIMIT $1
	`
	var users []*interfaces.User
	err := r.getDB().SelectContext(ctx, &users, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get top followers failed: %w", err)
	}
	return users, nil
}

// GetTopFollowees returns users followed by the most people.
func (r *followRepo) GetTopFollowees(ctx context.Context, limit int) ([]*interfaces.User, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT u.id, u.username, u.full_name, u.avatar_url,
		       COUNT(f.follower_id) as follower_count
		FROM users u
		JOIN follows f ON u.id = f.followee_id
		WHERE f.deleted_at IS NULL AND u.deleted_at IS NULL
		GROUP BY u.id
		ORDER BY follower_count DESC
		LIMIT $1
	`
	var users []*interfaces.User
	err := r.getDB().SelectContext(ctx, &users, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get top followees failed: %w", err)
	}
	return users, nil
}

// ======================================================================
// Advanced Queries
// ======================================================================

// GetFollowingIntersection returns users followed by both users.
func (r *followRepo) GetFollowingIntersection(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT f1.followee_id
		FROM follows f1
		JOIN follows f2 ON f1.followee_id = f2.followee_id
		WHERE f1.follower_id = $1 AND f2.follower_id = $2
		  AND f1.deleted_at IS NULL AND f2.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND f1.followee_id > $3`
	}
	query += ` ORDER BY f1.followee_id ASC LIMIT $?`

	args := []interface{}{userID1, userID2}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get following intersection failed: %w", err)
	}
	var nextCursor string
	if len(ids) == limit {
		nextCursor = ids[len(ids)-1]
	}
	return ids, nextCursor, nil
}

// GetFollowerIntersection returns users who follow both users.
func (r *followRepo) GetFollowerIntersection(ctx context.Context, userID1, userID2 string, cursor string, limit int) ([]string, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT f1.follower_id
		FROM follows f1
		JOIN follows f2 ON f1.follower_id = f2.follower_id
		WHERE f1.followee_id = $1 AND f2.followee_id = $2
		  AND f1.deleted_at IS NULL AND f2.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND f1.follower_id > $3`
	}
	query += ` ORDER BY f1.follower_id ASC LIMIT $?`

	args := []interface{}{userID1, userID2}
	argIdx := 3
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 4
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get follower intersection failed: %w", err)
	}
	var nextCursor string
	if len(ids) == limit {
		nextCursor = ids[len(ids)-1]
	}
	return ids, nextCursor, nil
}

// GetFollowPaths returns the follow path between two users (graph traversal).
func (r *followRepo) GetFollowPaths(ctx context.Context, userID1, userID2 string, maxDepth int) ([][]string, error) {
	// This is a more complex graph query; for simplicity, return empty for now
	// In production, you could use recursive CTEs
	return [][]string{}, nil
}

// GetFollowersByDateRange returns followers within a date range.
func (r *followRepo) GetFollowersByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*interfaces.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE followee_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL
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

	var follows []*interfaces.Follow
	err := r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get followers by date range failed: %w", err)
	}
	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// GetFollowingByDateRange returns following within a date range.
func (r *followRepo) GetFollowingByDateRange(ctx context.Context, userID string, start, end time.Time, cursor string, limit int) ([]*interfaces.Follow, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM follows
		WHERE follower_id = $1 AND created_at >= $2 AND created_at <= $3 AND deleted_at IS NULL
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

	var follows []*interfaces.Follow
	err := r.getDB().SelectContext(ctx, &follows, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get following by date range failed: %w", err)
	}
	var nextCursor string
	if len(follows) == limit {
		nextCursor = follows[len(follows)-1].ID
	}
	return follows, nextCursor, nil
}

// ======================================================================
// Health and Cleanup
// ======================================================================

// CleanupExpired removes expired or stale follow requests.
func (r *followRepo) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM follows WHERE status = 'pending' AND created_at < $1`
	result, err := r.getDB().ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired follows failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// Ping checks database connectivity.
func (r *followRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close releases any resources.
func (r *followRepo) Close() error {
	return nil
}

// GetRawDB returns the underlying database connection.
func (r *followRepo) GetRawDB() interface{} {
	return r.db
}

// ======================================================================
// Helper Functions
// ======================================================================

// getUserByID retrieves a user by ID (minimal fields).
func (r *followRepo) getUserByID(ctx context.Context, userID string) (*interfaces.User, error) {
	query := `SELECT id, username, full_name, avatar_url FROM users WHERE id = $1 AND deleted_at IS NULL`
	var user interfaces.User
	err := r.getDB().GetContext(ctx, &user, query, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user failed: %w", err)
	}
	return &user, nil
}