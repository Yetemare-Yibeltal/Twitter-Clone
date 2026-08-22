// backend/internal/repository/postgres/community_pg.go
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

// communityRepo is the PostgreSQL implementation of CommunityRepository.
type communityRepo struct {
	db  *sqlx.DB
	tx  *sqlx.Tx
	log *logrus.Entry
}

// NewCommunityRepository creates a new PostgreSQL community repository.
func NewCommunityRepository(db *sqlx.DB) interfaces.CommunityRepository {
	return &communityRepo{
		db:  db,
		log: logger.WithField("repository", "community_pg"),
	}
}

// WithTransaction returns a new repository using the given transaction.
func (r *communityRepo) WithTransaction(ctx context.Context, tx *sql.Tx) interfaces.CommunityRepository {
	sqlxTx := sqlx.NewTx(tx, r.db.DriverName())
	return &communityRepo{
		db:  r.db,
		tx:  sqlxTx,
		log: r.log.WithField("transaction", true),
	}
}

// Transaction executes a function within a transaction.
func (r *communityRepo) Transaction(ctx context.Context, fn func(txRepo interfaces.CommunityRepository) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}
	txRepo := &communityRepo{
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
func (r *communityRepo) getDB() sqlx.ExtContext {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// ======================================================================
// Basic Community CRUD
// ======================================================================

// Create inserts a new community.
func (r *communityRepo) Create(ctx context.Context, community *entities.Community) error {
	query := `
		INSERT INTO communities (
			id, name, slug, description, avatar_url, banner_url,
			created_by, is_private, member_count, post_count,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.getDB().ExecContext(ctx, query,
		community.ID, community.Name, community.Slug, community.Description,
		community.AvatarURL, community.BannerURL, community.CreatedBy,
		community.IsPrivate, community.MemberCount, community.PostCount,
		community.CreatedAt, community.UpdatedAt,
	)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			if pgErr.Constraint == "communities_slug_key" {
				return interfaces.ErrDuplicateSlug
			}
		}
		return fmt.Errorf("create community failed: %w", err)
	}
	return nil
}

// GetByID retrieves a community by its ID.
func (r *communityRepo) GetByID(ctx context.Context, id string) (*entities.Community, error) {
	query := `SELECT * FROM communities WHERE id = $1 AND deleted_at IS NULL`
	var community entities.Community
	err := r.getDB().GetContext(ctx, &community, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrCommunityNotFound
		}
		return nil, fmt.Errorf("get community by ID failed: %w", err)
	}
	return &community, nil
}

// GetBySlug retrieves a community by its slug.
func (r *communityRepo) GetBySlug(ctx context.Context, slug string) (*entities.Community, error) {
	query := `SELECT * FROM communities WHERE slug = $1 AND deleted_at IS NULL`
	var community entities.Community
	err := r.getDB().GetContext(ctx, &community, query, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, interfaces.ErrCommunityNotFound
		}
		return nil, fmt.Errorf("get community by slug failed: %w", err)
	}
	return &community, nil
}

// Update updates a community.
func (r *communityRepo) Update(ctx context.Context, community *entities.Community) error {
	query := `
		UPDATE communities SET
			name = $1,
			description = $2,
			avatar_url = $3,
			banner_url = $4,
			is_private = $5,
			updated_at = $6
		WHERE id = $7 AND deleted_at IS NULL
	`
	result, err := r.getDB().ExecContext(ctx, query,
		community.Name, community.Description, community.AvatarURL,
		community.BannerURL, community.IsPrivate, time.Now(), community.ID,
	)
	if err != nil {
		return fmt.Errorf("update community failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrCommunityNotFound
	}
	return nil
}

// SoftDelete marks a community as deleted.
func (r *communityRepo) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE communities SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL`
	_, err := r.getDB().ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("soft delete community failed: %w", err)
	}
	return nil
}

// HardDelete permanently removes a community.
func (r *communityRepo) HardDelete(ctx context.Context, id string) error {
	query := `DELETE FROM communities WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("hard delete community failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrCommunityNotFound
	}
	return nil
}

// ======================================================================
= List and Search
// ======================================================================

// List returns communities with pagination and filtering.
func (r *communityRepo) List(ctx context.Context, filter *interfaces.CommunityFilter, pagination *interfaces.PaginationOptions) ([]*entities.Community, int64, error) {
	whereClauses := []string{"deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if filter != nil {
		if filter.Name != nil && *filter.Name != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE $%d", argIdx))
			args = append(args, "%"+*filter.Name+"%")
			argIdx++
		}
		if filter.CreatedBy != nil && *filter.CreatedBy != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("created_by = $%d", argIdx))
			args = append(args, *filter.CreatedBy)
			argIdx++
		}
		if filter.IsPrivate != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("is_private = $%d", argIdx))
			args = append(args, *filter.IsPrivate)
			argIdx++
		}
		if filter.MinMembers != nil && *filter.MinMembers > 0 {
			whereClauses = append(whereClauses, fmt.Sprintf("member_count >= $%d", argIdx))
			args = append(args, *filter.MinMembers)
			argIdx++
		}
		if filter.MaxMembers != nil && *filter.MaxMembers > 0 {
			whereClauses = append(whereClauses, fmt.Sprintf("member_count <= $%d", argIdx))
			args = append(args, *filter.MaxMembers)
			argIdx++
		}
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM communities %s", whereSQL)
	var total int64
	err := r.getDB().GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("count communities failed: %w", err)
	}

	// Set defaults
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

	allowedSort := map[string]bool{
		"created_at": true, "updated_at": true, "name": true,
		"member_count": true, "post_count": true,
	}
	if !allowedSort[sortBy] {
		sortBy = "created_at"
	}
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT * FROM communities %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, sortBy, order, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var communities []*entities.Community
	err = r.getDB().SelectContext(ctx, &communities, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list communities failed: %w", err)
	}
	return communities, total, nil
}

// Search performs full-text search on communities.
func (r *communityRepo) Search(ctx context.Context, query string, pagination *interfaces.PaginationOptions) ([]*entities.Community, int64, error) {
	if query == "" {
		return r.List(ctx, nil, pagination)
	}
	whereSQL := `
		WHERE deleted_at IS NULL
		AND (name ILIKE $1 OR description ILIKE $2)
	`
	args := []interface{}{"%"+query+"%", "%"+query+"%"}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM communities %s", whereSQL)
	var total int64
	err := r.getDB().GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search communities count failed: %w", err)
	}

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
	}

	querySQL := fmt.Sprintf(`
		SELECT * FROM communities %s
		ORDER BY %s %s
		LIMIT $3 OFFSET $4
	`, whereSQL, sortBy, order)
	args = append(args, limit, offset)

	var communities []*entities.Community
	err = r.getDB().SelectContext(ctx, &communities, querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search communities failed: %w", err)
	}
	return communities, total, nil
}

// ======================================================================
// Count Operations
// ======================================================================

// CountTotal returns total number of communities.
func (r *communityRepo) CountTotal(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM communities WHERE deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("count total communities failed: %w", err)
	}
	return count, nil
}

// CountByUser returns number of communities created by a user.
func (r *communityRepo) CountByUser(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM communities WHERE created_by = $1 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count communities by user failed: %w", err)
	}
	return count, nil
}

// ======================================================================
= Membership Management
// ======================================================================

// AddMember adds a user to a community with a role.
func (r *communityRepo) AddMember(ctx context.Context, communityID, userID string, role string) error {
	query := `
		INSERT INTO community_members (community_id, user_id, role, joined_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.getDB().ExecContext(ctx, query, communityID, userID, role, time.Now(), time.Now())
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return interfaces.ErrMemberAlreadyExists
		}
		return fmt.Errorf("add community member failed: %w", err)
	}
	// Increment member count
	_, _ = r.getDB().ExecContext(ctx, `UPDATE communities SET member_count = member_count + 1 WHERE id = $1`, communityID)
	return nil
}

// RemoveMember removes a user from a community.
func (r *communityRepo) RemoveMember(ctx context.Context, communityID, userID string) error {
	query := `DELETE FROM community_members WHERE community_id = $1 AND user_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, communityID, userID)
	if err != nil {
		return fmt.Errorf("remove community member failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrMemberNotFound
	}
	// Decrement member count
	_, _ = r.getDB().ExecContext(ctx, `UPDATE communities SET member_count = GREATEST(member_count - 1, 0) WHERE id = $1`, communityID)
	return nil
}

// UpdateMemberRole updates the role of a community member.
func (r *communityRepo) UpdateMemberRole(ctx context.Context, communityID, userID, newRole string) error {
	query := `
		UPDATE community_members
		SET role = $1, updated_at = $2
		WHERE community_id = $3 AND user_id = $4
	`
	result, err := r.getDB().ExecContext(ctx, query, newRole, time.Now(), communityID, userID)
	if err != nil {
		return fmt.Errorf("update member role failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrMemberNotFound
	}
	return nil
}

// GetMemberRole returns the role of a member in a community.
func (r *communityRepo) GetMemberRole(ctx context.Context, communityID, userID string) (string, error) {
	query := `SELECT role FROM community_members WHERE community_id = $1 AND user_id = $2`
	var role string
	err := r.getDB().GetContext(ctx, &role, query, communityID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", interfaces.ErrMemberNotFound
		}
		return "", fmt.Errorf("get member role failed: %w", err)
	}
	return role, nil
}

// IsMember checks if a user is a member of a community.
func (r *communityRepo) IsMember(ctx context.Context, communityID, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, communityID, userID)
	if err != nil {
		return false, fmt.Errorf("check is member failed: %w", err)
	}
	return exists, nil
}

// ======================================================================
= Member Listing
// ======================================================================

// GetMembers returns members of a community with pagination and role filter.
func (r *communityRepo) GetMembers(ctx context.Context, communityID string, role string, cursor string, limit int) ([]*entities.CommunityMember, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM community_members
		WHERE community_id = $1
	`
	args := []interface{}{communityID}
	argIdx := 2
	if role != "" {
		query += ` AND role = $2`
		args = append(args, role)
		argIdx = 3
	}
	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}
	query += fmt.Sprintf(` ORDER BY joined_at DESC, id DESC LIMIT $%d`, argIdx)
	args = append(args, limit)

	query = r.getDB().Rebind(query)
	var members []*entities.CommunityMember
	err := r.getDB().SelectContext(ctx, &members, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get members failed: %w", err)
	}
	var nextCursor string
	if len(members) == limit {
		nextCursor = members[len(members)-1].ID
	}
	return members, nextCursor, nil
}

// GetMemberUserIDs returns all user IDs of members.
func (r *communityRepo) GetMemberUserIDs(ctx context.Context, communityID string) ([]string, error) {
	query := `SELECT user_id FROM community_members WHERE community_id = $1`
	var ids []string
	err := r.getDB().SelectContext(ctx, &ids, query, communityID)
	if err != nil {
		return nil, fmt.Errorf("get member user IDs failed: %w", err)
	}
	return ids, nil
}

// GetUserCommunities returns communities a user belongs to.
func (r *communityRepo) GetUserCommunities(ctx context.Context, userID string, cursor string, limit int) ([]*entities.Community, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT c.*
		FROM communities c
		JOIN community_members cm ON c.id = cm.community_id
		WHERE cm.user_id = $1 AND c.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND c.id > $2`
	}
	query += ` ORDER BY cm.joined_at DESC, c.id DESC LIMIT $?`

	args := []interface{}{userID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var communities []*entities.Community
	err := r.getDB().SelectContext(ctx, &communities, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get user communities failed: %w", err)
	}
	var nextCursor string
	if len(communities) == limit {
		nextCursor = communities[len(communities)-1].ID
	}
	return communities, nextCursor, nil
}

// ======================================================================
= Moderation
// ======================================================================

// BanUser bans a user from a community.
func (r *communityRepo) BanUser(ctx context.Context, communityID, userID, reason string) error {
	// First remove from members if present
	_ = r.RemoveMember(ctx, communityID, userID)
	// Insert into bans
	query := `
		INSERT INTO community_bans (community_id, user_id, reason, banned_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.getDB().ExecContext(ctx, query, communityID, userID, reason, time.Now())
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return interfaces.ErrUserAlreadyBanned
		}
		return fmt.Errorf("ban user failed: %w", err)
	}
	return nil
}

// UnbanUser removes a ban from a user.
func (r *communityRepo) UnbanUser(ctx context.Context, communityID, userID string) error {
	query := `DELETE FROM community_bans WHERE community_id = $1 AND user_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, communityID, userID)
	if err != nil {
		return fmt.Errorf("unban user failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrBanNotFound
	}
	return nil
}

// IsBanned checks if a user is banned from a community.
func (r *communityRepo) IsBanned(ctx context.Context, communityID, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM community_bans WHERE community_id = $1 AND user_id = $2)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, communityID, userID)
	if err != nil {
		return false, fmt.Errorf("check banned failed: %w", err)
	}
	return exists, nil
}

// GetBannedUsers returns banned users for a community.
func (r *communityRepo) GetBannedUsers(ctx context.Context, communityID string, cursor string, limit int) ([]*entities.CommunityBan, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT * FROM community_bans
		WHERE community_id = $1
	`
	if cursor != "" {
		query += ` AND id > $2`
	}
	query += ` ORDER BY banned_at DESC, id DESC LIMIT $?`

	args := []interface{}{communityID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var bans []*entities.CommunityBan
	err := r.getDB().SelectContext(ctx, &bans, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get banned users failed: %w", err)
	}
	var nextCursor string
	if len(bans) == limit {
		nextCursor = bans[len(bans)-1].ID
	}
	return bans, nextCursor, nil
}

// ======================================================================
= Community Posts
// ======================================================================

// AddPost adds a tweet to a community (as a post).
func (r *communityRepo) AddPost(ctx context.Context, communityID, tweetID string) error {
	query := `
		INSERT INTO community_posts (community_id, tweet_id, created_at)
		VALUES ($1, $2, $3)
	`
	_, err := r.getDB().ExecContext(ctx, query, communityID, tweetID, time.Now())
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return interfaces.ErrPostAlreadyExists
		}
		return fmt.Errorf("add community post failed: %w", err)
	}
	// Increment post count
	_, _ = r.getDB().ExecContext(ctx, `UPDATE communities SET post_count = post_count + 1 WHERE id = $1`, communityID)
	return nil
}

// RemovePost removes a post from a community.
func (r *communityRepo) RemovePost(ctx context.Context, communityID, tweetID string) error {
	query := `DELETE FROM community_posts WHERE community_id = $1 AND tweet_id = $2`
	result, err := r.getDB().ExecContext(ctx, query, communityID, tweetID)
	if err != nil {
		return fmt.Errorf("remove community post failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrPostNotFound
	}
	// Decrement post count
	_, _ = r.getDB().ExecContext(ctx, `UPDATE communities SET post_count = GREATEST(post_count - 1, 0) WHERE id = $1`, communityID)
	return nil
}

// GetPosts returns posts (tweets) in a community.
func (r *communityRepo) GetPosts(ctx context.Context, communityID string, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT t.*
		FROM community_posts cp
		JOIN tweets t ON cp.tweet_id = t.id
		WHERE cp.community_id = $1 AND t.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND t.id > $2`
	}
	query += ` ORDER BY cp.created_at DESC, t.id DESC LIMIT $?`

	args := []interface{}{communityID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get community posts failed: %w", err)
	}
	var nextCursor string
	if len(tweets) == limit {
		nextCursor = tweets[len(tweets)-1].ID
	}
	return tweets, nextCursor, nil
}

// ======================================================================
= Stats and Analytics
// ======================================================================

// GetCommunityStats returns aggregated community statistics.
func (r *communityRepo) GetCommunityStats(ctx context.Context) (*CommunityStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_communities,
			SUM(member_count) as total_members,
			SUM(post_count) as total_posts,
			AVG(member_count) as avg_members,
			MAX(member_count) as max_members,
			MIN(member_count) as min_members,
			COUNT(CASE WHEN is_private = true THEN 1 END) as private_count,
			COUNT(CASE WHEN is_private = false THEN 1 END) as public_count
		FROM communities
		WHERE deleted_at IS NULL
	`
	var stats CommunityStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get community stats failed: %w", err)
	}
	return &stats, nil
}

// CommunityStats represents aggregated community statistics.
type CommunityStats struct {
	TotalCommunities int64   `db:"total_communities"`
	TotalMembers     int64   `db:"total_members"`
	TotalPosts       int64   `db:"total_posts"`
	AvgMembers       float64 `db:"avg_members"`
	MaxMembers       int64   `db:"max_members"`
	MinMembers       int64   `db:"min_members"`
	PrivateCount     int64   `db:"private_count"`
	PublicCount      int64   `db:"public_count"`
}

// GetDailyCommunityStats returns daily creation counts.
func (r *communityRepo) GetDailyCommunityStats(ctx context.Context, start, end time.Time) ([]*DailyCommunityCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			SUM(member_count) as new_members,
			SUM(post_count) as new_posts
		FROM communities
		WHERE created_at >= $1 AND created_at <= $2 AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*DailyCommunityCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily community stats failed: %w", err)
	}
	return results, nil
}

// DailyCommunityCount represents daily community statistics.
type DailyCommunityCount struct {
	Date        time.Time `db:"date"`
	Total       int64     `db:"total"`
	NewMembers  int64     `db:"new_members"`
	NewPosts    int64     `db:"new_posts"`
}

// ======================================================================
= Bulk Operations
// ======================================================================

// BulkCreate inserts multiple communities in a transaction.
func (r *communityRepo) BulkCreate(ctx context.Context, communities []*entities.Community) error {
	if len(communities) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO communities (
			id, name, slug, description, avatar_url, banner_url,
			created_by, is_private, member_count, post_count,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range communities {
		_, err = stmt.ExecContext(ctx,
			c.ID, c.Name, c.Slug, c.Description, c.AvatarURL, c.BannerURL,
			c.CreatedBy, c.IsPrivate, c.MemberCount, c.PostCount,
			c.CreatedAt, c.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("bulk create community failed: %w", err)
		}
	}
	return tx.Commit()
}

// BulkDelete removes multiple communities.
func (r *communityRepo) BulkDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM communities WHERE id IN (?)`, ids)
	if err != nil {
		return err
	}
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk delete communities failed: %w", err)
	}
	return nil
}

// ======================================================================
= Health
// ======================================================================

func (r *communityRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *communityRepo) Close() error {
	return nil
}

func (r *communityRepo) GetRawDB() interface{} {
	return r.db
}