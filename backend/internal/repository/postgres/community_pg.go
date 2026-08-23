// backend/internal/repository/postgres/community_pg.go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

// GetByIDs retrieves multiple communities by their IDs.
func (r *communityRepo) GetByIDs(ctx context.Context, ids []string) ([]*entities.Community, error) {
	if len(ids) == 0 {
		return []*entities.Community{}, nil
	}
	query, args, err := sqlx.In(`SELECT * FROM communities WHERE id IN (?) AND deleted_at IS NULL`, ids)
	if err != nil {
		return nil, fmt.Errorf("build IN query failed: %w", err)
	}
	query = r.getDB().Rebind(query)
	var communities []*entities.Community
	err = r.getDB().SelectContext(ctx, &communities, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get communities by IDs failed: %w", err)
	}
	return communities, nil
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

// Restore restores a soft-deleted community.
func (r *communityRepo) Restore(ctx context.Context, id string) error {
	query := `UPDATE communities SET deleted_at = NULL WHERE id = $1`
	result, err := r.getDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("restore community failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrCommunityNotFound
	}
	return nil
}

// ======================================================================
// List and Search
// ======================================================================

// List returns communities with filtering and pagination.
func (r *communityRepo) List(ctx context.Context, filter *interfaces.CommunityFilter, pagination *interfaces.CommunityPagination) ([]*entities.Community, int64, error) {
	whereClauses := []string{"deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if filter != nil {
		if filter.Name != nil && *filter.Name != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE $%d", argIdx))
			args = append(args, "%"+*filter.Name+"%")
			argIdx++
		}
		if filter.Slug != nil && *filter.Slug != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("slug = $%d", argIdx))
			args = append(args, *filter.Slug)
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
		if filter.MinPosts != nil && *filter.MinPosts > 0 {
			whereClauses = append(whereClauses, fmt.Sprintf("post_count >= $%d", argIdx))
			args = append(args, *filter.MinPosts)
			argIdx++
		}
		if filter.MaxPosts != nil && *filter.MaxPosts > 0 {
			whereClauses = append(whereClauses, fmt.Sprintf("post_count <= $%d", argIdx))
			args = append(args, *filter.MaxPosts)
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
		if filter.Search != nil && *filter.Search != "" {
			whereClauses = append(whereClauses, fmt.Sprintf(`(name ILIKE $%d OR description ILIKE $%d)`, argIdx, argIdx+1))
			args = append(args, "%"+*filter.Search+"%", "%"+*filter.Search+"%")
			argIdx += 2
		}
	}

	// Build WHERE clause
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
		if pagination.Cursor != "" {
			// For cursor-based pagination, we use the cursor as a reference
			// For simplicity, we use offset-based pagination with cursor as a marker
			// In production, implement proper cursor pagination
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
func (r *communityRepo) Search(ctx context.Context, query string, pagination *interfaces.CommunityPagination) ([]*entities.Community, int64, error) {
	if query == "" {
		return r.List(ctx, nil, pagination)
	}
	filter := &interfaces.CommunityFilter{Search: &query}
	return r.List(ctx, filter, pagination)
}

// GetByUserID returns communities created by a user.
func (r *communityRepo) GetByUserID(ctx context.Context, userID string, pagination *interfaces.CommunityPagination) ([]*entities.Community, int64, error) {
	filter := &interfaces.CommunityFilter{CreatedBy: &userID}
	return r.List(ctx, filter, pagination)
}

// GetByMemberID returns communities a user is a member of.
func (r *communityRepo) GetByMemberID(ctx context.Context, userID string, pagination *interfaces.CommunityPagination) ([]*entities.Community, int64, error) {
	if pagination == nil {
		pagination = interfaces.DefaultCommunityPagination()
	}
	limit := pagination.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := 0
	sortBy := "c.created_at"
	order := "DESC"
	if pagination.SortBy != "" {
		sortBy = string(pagination.SortBy)
	}
	if pagination.Order != "" {
		order = string(pagination.Order)
	}

	query := `
		SELECT c.*
		FROM communities c
		INNER JOIN community_members cm ON c.id = cm.community_id
		WHERE cm.user_id = $1 AND c.deleted_at IS NULL
		ORDER BY ` + sortBy + ` ` + order + `
		LIMIT $2 OFFSET $3
	`
	var communities []*entities.Community
	err := r.getDB().SelectContext(ctx, &communities, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get communities by member failed: %w", err)
	}

	// Count total
	countQuery := `
		SELECT COUNT(*) FROM communities c
		INNER JOIN community_members cm ON c.id = cm.community_id
		WHERE cm.user_id = $1 AND c.deleted_at IS NULL
	`
	var total int64
	err = r.getDB().GetContext(ctx, &total, countQuery, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("count communities by member failed: %w", err)
	}
	return communities, total, nil
}

// GetByAdminID returns communities where a user is an admin.
func (r *communityRepo) GetByAdminID(ctx context.Context, userID string, pagination *interfaces.CommunityPagination) ([]*entities.Community, int64, error) {
	if pagination == nil {
		pagination = interfaces.DefaultCommunityPagination()
	}
	limit := pagination.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := 0
	sortBy := "c.created_at"
	order := "DESC"
	if pagination.SortBy != "" {
		sortBy = string(pagination.SortBy)
	}
	if pagination.Order != "" {
		order = string(pagination.Order)
	}

	query := `
		SELECT c.*
		FROM communities c
		INNER JOIN community_members cm ON c.id = cm.community_id
		WHERE cm.user_id = $1 
		  AND c.deleted_at IS NULL
		  AND cm.role IN ('admin', 'owner')
		ORDER BY ` + sortBy + ` ` + order + `
		LIMIT $2 OFFSET $3
	`
	var communities []*entities.Community
	err := r.getDB().SelectContext(ctx, &communities, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get communities by admin failed: %w", err)
	}

	// Count total
	countQuery := `
		SELECT COUNT(*) FROM communities c
		INNER JOIN community_members cm ON c.id = cm.community_id
		WHERE cm.user_id = $1 AND c.deleted_at IS NULL AND cm.role IN ('admin', 'owner')
	`
	var total int64
	err = r.getDB().GetContext(ctx, &total, countQuery, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("count communities by admin failed: %w", err)
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

// CountByUserID returns number of communities created by a user.
func (r *communityRepo) CountByUserID(ctx context.Context, userID string) (int64, error) {
	query := `SELECT COUNT(*) FROM communities WHERE created_by = $1 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count communities by user failed: %w", err)
	}
	return count, nil
}

// CountByMemberID returns number of communities a user is a member of.
func (r *communityRepo) CountByMemberID(ctx context.Context, userID string) (int64, error) {
	query := `
		SELECT COUNT(*) FROM community_members 
		WHERE user_id = $1
	`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("count communities by member failed: %w", err)
	}
	return count, nil
}

// CountByDateRange returns community count within a date range.
func (r *communityRepo) CountByDateRange(ctx context.Context, start, end time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM communities WHERE created_at >= $1 AND created_at <= $2 AND deleted_at IS NULL`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, start, end)
	if err != nil {
		return 0, fmt.Errorf("count communities by date range failed: %w", err)
	}
	return count, nil
}

// ======================================================================
// Membership Management
// ======================================================================

// AddMember adds a user to a community with a role.
func (r *communityRepo) AddMember(ctx context.Context, communityID, userID string, role string) error {
	// Start transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if community exists
	var exists bool
	err = tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM communities WHERE id = $1 AND deleted_at IS NULL)`, communityID)
	if err != nil {
		return fmt.Errorf("check community existence failed: %w", err)
	}
	if !exists {
		return interfaces.ErrCommunityNotFound
	}

	// Check if already a member
	err = tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2)`, communityID, userID)
	if err != nil {
		return fmt.Errorf("check member existence failed: %w", err)
	}
	if exists {
		return interfaces.ErrMemberAlreadyExists
	}

	// Insert member
	query := `
		INSERT INTO community_members (community_id, user_id, role, joined_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = tx.ExecContext(ctx, query, communityID, userID, role, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("add member failed: %w", err)
	}

	// Increment member count
	_, err = tx.ExecContext(ctx, `UPDATE communities SET member_count = member_count + 1 WHERE id = $1`, communityID)
	if err != nil {
		return fmt.Errorf("increment member count failed: %w", err)
	}

	return tx.Commit()
}

// RemoveMember removes a user from a community.
func (r *communityRepo) RemoveMember(ctx context.Context, communityID, userID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if member exists
	var role string
	err = tx.GetContext(ctx, &role, `SELECT role FROM community_members WHERE community_id = $1 AND user_id = $2`, communityID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return interfaces.ErrMemberNotFound
		}
		return fmt.Errorf("get member role failed: %w", err)
	}

	// Can't remove owner
	if role == "owner" {
		return interfaces.ErrCannotRemoveOwner
	}

	// Delete member
	_, err = tx.ExecContext(ctx, `DELETE FROM community_members WHERE community_id = $1 AND user_id = $2`, communityID, userID)
	if err != nil {
		return fmt.Errorf("remove member failed: %w", err)
	}

	// Decrement member count
	_, err = tx.ExecContext(ctx, `UPDATE communities SET member_count = GREATEST(member_count - 1, 0) WHERE id = $1`, communityID)
	if err != nil {
		return fmt.Errorf("decrement member count failed: %w", err)
	}

	return tx.Commit()
}

// UpdateMemberRole updates the role of a community member.
func (r *communityRepo) UpdateMemberRole(ctx context.Context, communityID, userID, newRole string) error {
	// Check if member exists
	var currentRole string
	err := r.getDB().GetContext(ctx, &currentRole, `SELECT role FROM community_members WHERE community_id = $1 AND user_id = $2`, communityID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return interfaces.ErrMemberNotFound
		}
		return fmt.Errorf("get member role failed: %w", err)
	}

	// Can't demote owner
	if currentRole == "owner" && newRole != "owner" {
		return interfaces.ErrCannotDemoteOwner
	}

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

// IsAdmin checks if a user is an admin of a community.
func (r *communityRepo) IsAdmin(ctx context.Context, communityID, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2 AND role IN ('admin', 'owner'))`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, communityID, userID)
	if err != nil {
		return false, fmt.Errorf("check is admin failed: %w", err)
	}
	return exists, nil
}

// IsModerator checks if a user is a moderator of a community.
func (r *communityRepo) IsModerator(ctx context.Context, communityID, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2 AND role IN ('moderator', 'admin', 'owner'))`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, communityID, userID)
	if err != nil {
		return false, fmt.Errorf("check is moderator failed: %w", err)
	}
	return exists, nil
}

// GetMemberCount returns the number of members in a community.
func (r *communityRepo) GetMemberCount(ctx context.Context, communityID string) (int64, error) {
	query := `SELECT COUNT(*) FROM community_members WHERE community_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, communityID)
	if err != nil {
		return 0, fmt.Errorf("get member count failed: %w", err)
	}
	return count, nil
}

// GetMembers returns members of a community with pagination and role filter.
func (r *communityRepo) GetMembers(ctx context.Context, communityID string, role string, cursor string, limit int) ([]*interfaces.CommunityMember, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT cm.user_id, cm.role, cm.joined_at,
		       u.username, u.full_name, u.avatar_url,
		       u.is_active
		FROM community_members cm
		INNER JOIN users u ON cm.user_id = u.id
		WHERE cm.community_id = $1
	`
	args := []interface{}{communityID}
	argIdx := 2

	if role != "" {
		query += ` AND cm.role = $2`
		args = append(args, role)
		argIdx = 3
	}

	if cursor != "" {
		query += fmt.Sprintf(` AND cm.user_id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY cm.joined_at DESC, cm.user_id DESC LIMIT $%d`, argIdx)
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var members []*interfaces.CommunityMember
	err := r.getDB().SelectContext(ctx, &members, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get members failed: %w", err)
	}

	var nextCursor string
	if len(members) == limit {
		nextCursor = members[len(members)-1].UserID
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
// Moderation - Bans
// ======================================================================

// BanUser bans a user from a community.
func (r *communityRepo) BanUser(ctx context.Context, communityID, userID, reason string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if already banned
	var exists bool
	err = tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM community_bans WHERE community_id = $1 AND user_id = $2)`, communityID, userID)
	if err != nil {
		return fmt.Errorf("check ban existence failed: %w", err)
	}
	if exists {
		return interfaces.ErrUserAlreadyBanned
	}

	// Remove from members if present
	_, _ = tx.ExecContext(ctx, `DELETE FROM community_members WHERE community_id = $1 AND user_id = $2`, communityID, userID)

	// Insert ban
	query := `
		INSERT INTO community_bans (community_id, user_id, reason, banned_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.ExecContext(ctx, query, communityID, userID, reason, time.Now())
	if err != nil {
		return fmt.Errorf("ban user failed: %w", err)
	}

	// Update member count if member was removed
	_, _ = tx.ExecContext(ctx, `UPDATE communities SET member_count = GREATEST(member_count - 1, 0) WHERE id = $1 AND member_count > 0`, communityID)

	return tx.Commit()
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
func (r *communityRepo) GetBannedUsers(ctx context.Context, communityID string, cursor string, limit int) ([]*interfaces.CommunityBan, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT cb.user_id, cb.reason, cb.banned_at,
		       u.username, u.full_name, u.avatar_url
		FROM community_bans cb
		INNER JOIN users u ON cb.user_id = u.id
		WHERE cb.community_id = $1
	`
	if cursor != "" {
		query += ` AND cb.user_id > $2`
	}
	query += ` ORDER BY cb.banned_at DESC, cb.user_id DESC LIMIT $?`

	args := []interface{}{communityID}
	argIdx := 2
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 3
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var bans []*interfaces.CommunityBan
	err := r.getDB().SelectContext(ctx, &bans, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get banned users failed: %w", err)
	}
	var nextCursor string
	if len(bans) == limit {
		nextCursor = bans[len(bans)-1].UserID
	}
	return bans, nextCursor, nil
}

// GetBanReason returns the reason a user was banned.
func (r *communityRepo) GetBanReason(ctx context.Context, communityID, userID string) (string, error) {
	query := `SELECT reason FROM community_bans WHERE community_id = $1 AND user_id = $2`
	var reason string
	err := r.getDB().GetContext(ctx, &reason, query, communityID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", interfaces.ErrBanNotFound
		}
		return "", fmt.Errorf("get ban reason failed: %w", err)
	}
	return reason, nil
}

// ======================================================================
// Community Posts
// ======================================================================

// AddPost adds a tweet to a community as a post.
func (r *communityRepo) AddPost(ctx context.Context, communityID, tweetID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if community exists
	var exists bool
	err = tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM communities WHERE id = $1 AND deleted_at IS NULL)`, communityID)
	if err != nil {
		return fmt.Errorf("check community existence failed: %w", err)
	}
	if !exists {
		return interfaces.ErrCommunityNotFound
	}

	// Check if post already exists
	err = tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM community_posts WHERE community_id = $1 AND tweet_id = $2)`, communityID, tweetID)
	if err != nil {
		return fmt.Errorf("check post existence failed: %w", err)
	}
	if exists {
		return interfaces.ErrPostAlreadyExists
	}

	// Insert post
	query := `
		INSERT INTO community_posts (community_id, tweet_id, created_at)
		VALUES ($1, $2, $3)
	`
	_, err = tx.ExecContext(ctx, query, communityID, tweetID, time.Now())
	if err != nil {
		return fmt.Errorf("add community post failed: %w", err)
	}

	// Increment post count
	_, err = tx.ExecContext(ctx, `UPDATE communities SET post_count = post_count + 1 WHERE id = $1`, communityID)
	if err != nil {
		return fmt.Errorf("increment post count failed: %w", err)
	}

	return tx.Commit()
}

// RemovePost removes a post from a community.
func (r *communityRepo) RemovePost(ctx context.Context, communityID, tweetID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete post
	result, err := tx.ExecContext(ctx, `DELETE FROM community_posts WHERE community_id = $1 AND tweet_id = $2`, communityID, tweetID)
	if err != nil {
		return fmt.Errorf("remove community post failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return interfaces.ErrPostNotFound
	}

	// Decrement post count
	_, err = tx.ExecContext(ctx, `UPDATE communities SET post_count = GREATEST(post_count - 1, 0) WHERE id = $1`, communityID)
	if err != nil {
		return fmt.Errorf("decrement post count failed: %w", err)
	}

	return tx.Commit()
}

// GetPosts returns posts (tweets) in a community with pagination.
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

// GetPostCount returns the number of posts in a community.
func (r *communityRepo) GetPostCount(ctx context.Context, communityID string) (int64, error) {
	query := `SELECT COUNT(*) FROM community_posts WHERE community_id = $1`
	var count int64
	err := r.getDB().GetContext(ctx, &count, query, communityID)
	if err != nil {
		return 0, fmt.Errorf("get post count failed: %w", err)
	}
	return count, nil
}

// GetPostByTweetID checks if a tweet is posted in a community.
func (r *communityRepo) GetPostByTweetID(ctx context.Context, communityID, tweetID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM community_posts WHERE community_id = $1 AND tweet_id = $2)`
	var exists bool
	err := r.getDB().GetContext(ctx, &exists, query, communityID, tweetID)
	if err != nil {
		return false, fmt.Errorf("check post existence failed: %w", err)
	}
	return exists, nil
}

// GetPostsByDateRange returns posts within a date range.
func (r *communityRepo) GetPostsByDateRange(ctx context.Context, communityID string, start, end time.Time, cursor string, limit int) ([]*entities.Tweet, string, error) {
	if limit < 1 {
		limit = 20
	}
	query := `
		SELECT t.*
		FROM community_posts cp
		JOIN tweets t ON cp.tweet_id = t.id
		WHERE cp.community_id = $1 
		  AND cp.created_at >= $2 AND cp.created_at <= $3
		  AND t.deleted_at IS NULL
	`
	if cursor != "" {
		query += ` AND t.id > $4`
	}
	query += ` ORDER BY cp.created_at DESC, t.id DESC LIMIT $?`

	args := []interface{}{communityID, start, end}
	argIdx := 4
	if cursor != "" {
		args = append(args, cursor)
		argIdx = 5
	}
	args = append(args, limit)
	query = r.getDB().Rebind(query)

	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("get posts by date range failed: %w", err)
	}
	var nextCursor string
	if len(tweets) == limit {
		nextCursor = tweets[len(tweets)-1].ID
	}
	return tweets, nextCursor, nil
}

// GetTopPosts returns the most popular posts in a community.
func (r *communityRepo) GetTopPosts(ctx context.Context, communityID string, limit int, since time.Time) ([]*entities.Tweet, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT t.*
		FROM community_posts cp
		JOIN tweets t ON cp.tweet_id = t.id
		LEFT JOIN likes l ON t.id = l.tweet_id
		WHERE cp.community_id = $1 
		  AND cp.created_at >= $2
		  AND t.deleted_at IS NULL
		GROUP BY t.id
		ORDER BY COUNT(l.id) DESC, t.created_at DESC
		LIMIT $3
	`
	var tweets []*entities.Tweet
	err := r.getDB().SelectContext(ctx, &tweets, query, communityID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get top posts failed: %w", err)
	}
	return tweets, nil
}

// ======================================================================
// Role-based Permissions
// ======================================================================

// GetCommunityRoles returns all roles and their permissions.
func (r *communityRepo) GetCommunityRoles(ctx context.Context) ([]*interfaces.CommunityRole, error) {
	query := `SELECT * FROM community_roles ORDER BY name`
	var roles []*interfaces.CommunityRole
	err := r.getDB().SelectContext(ctx, &roles, query)
	if err != nil {
		return nil, fmt.Errorf("get community roles failed: %w", err)
	}
	return roles, nil
}

// GetUserPermissions returns permissions for a user in a community.
func (r *communityRepo) GetUserPermissions(ctx context.Context, communityID, userID string) (*interfaces.CommunityPermissions, error) {
	// Get user role
	role, err := r.GetMemberRole(ctx, communityID, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrMemberNotFound) {
			return &interfaces.CommunityPermissions{}, nil
		}
		return nil, err
	}

	perms := &interfaces.CommunityPermissions{}
	switch role {
	case "owner", "admin":
		perms.CanPost = true
		perms.CanComment = true
		perms.CanVote = true
		perms.CanModerate = true
		perms.CanManage = true
		perms.CanInvite = true
		perms.CanBan = true
		perms.CanPin = true
		perms.CanDelete = true
		perms.IsAdmin = true
		perms.IsModerator = true
	case "moderator":
		perms.CanPost = true
		perms.CanComment = true
		perms.CanVote = true
		perms.CanModerate = true
		perms.CanInvite = true
		perms.CanPin = true
		perms.CanDelete = true
		perms.IsModerator = true
	default:
		perms.CanPost = true
		perms.CanComment = true
		perms.CanVote = true
	}
	return perms, nil
}

// SetCustomRole sets custom role permissions for a community.
func (r *communityRepo) SetCustomRole(ctx context.Context, communityID, role string, permissions []string) error {
	permsJSON, err := json.Marshal(permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions failed: %w", err)
	}
	query := `
		INSERT INTO community_roles (community_id, name, permissions, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (community_id, name) DO UPDATE SET
			permissions = $3, updated_at = $5
	`
	_, err = r.getDB().ExecContext(ctx, query, communityID, role, permsJSON, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("set custom role failed: %w", err)
	}
	return nil
}

// ======================================================================
// Advanced Queries
// ======================================================================

// GetTrendingCommunities returns trending communities.
func (r *communityRepo) GetTrendingCommunities(ctx context.Context, limit int, since time.Time) ([]*entities.Community, error) {
	if limit < 1 {
		limit = 10
	}
	query := `
		SELECT c.*
		FROM communities c
		LEFT JOIN community_posts cp ON c.id = cp.community_id AND cp.created_at >= $1
		WHERE c.deleted_at IS NULL
		GROUP BY c.id
		ORDER BY COUNT(cp.tweet_id) DESC, c.member_count DESC
		LIMIT $2
	`
	var communities []*entities.Community
	err := r.getDB().SelectContext(ctx, &communities, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("get trending communities failed: %w", err)
	}
	return communities, nil
}

// GetSimilarCommunities returns similar communities.
func (r *communityRepo) GetSimilarCommunities(ctx context.Context, communityID string, limit int) ([]*entities.Community, error) {
	if limit < 1 {
		limit = 10
	}
	// Find communities with similar tags or member overlap
	query := `
		SELECT c2.*
		FROM communities c1
		JOIN community_members cm1 ON c1.id = cm1.community_id
		JOIN community_members cm2 ON cm1.user_id = cm2.user_id
		JOIN communities c2 ON cm2.community_id = c2.id
		WHERE c1.id = $1 
		  AND c2.id != $1
		  AND c2.deleted_at IS NULL
		GROUP BY c2.id
		ORDER BY COUNT(DISTINCT cm2.user_id) DESC
		LIMIT $2
	`
	var communities []*entities.Community
	err := r.getDB().SelectContext(ctx, &communities, query, communityID, limit)
	if err != nil {
		return nil, fmt.Errorf("get similar communities failed: %w", err)
	}
	return communities, nil
}

// GetRecommendations returns recommended communities for a user.
func (r *communityRepo) GetRecommendations(ctx context.Context, userID string, limit int) ([]*entities.Community, error) {
	if limit < 1 {
		limit = 10
	}
	// Find communities that followers of the user are in, but user is not
	query := `
		SELECT c.*
		FROM communities c
		JOIN community_members cm ON c.id = cm.community_id
		WHERE c.deleted_at IS NULL
		  AND c.is_private = false
		  AND c.id NOT IN (
		    SELECT community_id FROM community_members WHERE user_id = $1
		  )
		  AND cm.user_id IN (
		    SELECT followee_id FROM follows WHERE follower_id = $1
		  )
		GROUP BY c.id
		ORDER BY COUNT(DISTINCT cm.user_id) DESC
		LIMIT $2
	`
	var communities []*entities.Community
	err := r.getDB().SelectContext(ctx, &communities, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get recommendations failed: %w", err)
	}
	return communities, nil
}

// GetCommunitiesByTags returns communities with specific tags.
func (r *communityRepo) GetCommunitiesByTags(ctx context.Context, tags []string, pagination *interfaces.CommunityPagination) ([]*entities.Community, int64, error) {
	if len(tags) == 0 {
		return r.List(ctx, nil, pagination)
	}
	// Assuming tags are stored as a text array or JSON
	// For simplicity, we'll search in description
	searchTerm := strings.Join(tags, " ")
	filter := &interfaces.CommunityFilter{Search: &searchTerm}
	return r.List(ctx, filter, pagination)
}

// GetActivitySummary returns community activity summary.
func (r *communityRepo) GetActivitySummary(ctx context.Context, communityID string) (*interfaces.CommunityActivitySummary, error) {
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	var summary interfaces.CommunityActivitySummary

	// New members in last 7 days
	err := r.getDB().GetContext(ctx, &summary.NewMembers,
		`SELECT COUNT(*) FROM community_members WHERE community_id = $1 AND joined_at >= $2`,
		communityID, weekAgo)
	if err != nil {
		return nil, fmt.Errorf("get new members failed: %w", err)
	}

	// New posts in last 7 days
	err = r.getDB().GetContext(ctx, &summary.NewPosts,
		`SELECT COUNT(*) FROM community_posts WHERE community_id = $1 AND created_at >= $2`,
		communityID, weekAgo)
	if err != nil {
		return nil, fmt.Errorf("get new posts failed: %w", err)
	}

	// Active members (posted or commented in last 7 days)
	err = r.getDB().GetContext(ctx, &summary.ActiveMembers,
		`SELECT COUNT(DISTINCT user_id) FROM community_posts WHERE community_id = $1 AND created_at >= $2`,
		communityID, weekAgo)
	if err != nil {
		return nil, fmt.Errorf("get active members failed: %w", err)
	}

	// Engagement rate (active members / total members)
	var totalMembers int64
	err = r.getDB().GetContext(ctx, &totalMembers,
		`SELECT COUNT(*) FROM community_members WHERE community_id = $1`,
		communityID)
	if err != nil {
		return nil, fmt.Errorf("get total members failed: %w", err)
	}
	if totalMembers > 0 {
		summary.EngagementRate = float64(summary.ActiveMembers) / float64(totalMembers) * 100
	}

	// Last activity
	err = r.getDB().GetContext(ctx, &summary.LastActivity,
		`SELECT MAX(created_at) FROM community_posts WHERE community_id = $1`,
		communityID)
	if err != nil {
		return nil, fmt.Errorf("get last activity failed: %w", err)
	}

	// Top post
	var topPost struct {
		TweetID string `db:"tweet_id"`
		Likes   int64  `db:"likes"`
	}
	err = r.getDB().GetContext(ctx, &topPost,
		`SELECT cp.tweet_id, COUNT(l.id) as likes
		FROM community_posts cp
		JOIN tweets t ON cp.tweet_id = t.id
		LEFT JOIN likes l ON t.id = l.tweet_id
		WHERE cp.community_id = $1 AND t.deleted_at IS NULL
		GROUP BY cp.tweet_id
		ORDER BY likes DESC, cp.created_at DESC
		LIMIT 1`,
		communityID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get top post failed: %w", err)
	}
	summary.TopPostID = topPost.TweetID
	summary.TopPostLikes = topPost.Likes

	return &summary, nil
}

// ======================================================================
// Bulk Operations
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
		_, err := stmt.ExecContext(ctx,
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

// BulkAddMembers adds multiple members to a community.
func (r *communityRepo) BulkAddMembers(ctx context.Context, communityID string, userIDs []string, role string) error {
	if len(userIDs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check community exists
	var exists bool
	err = tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM communities WHERE id = $1)`, communityID)
	if err != nil {
		return err
	}
	if !exists {
		return interfaces.ErrCommunityNotFound
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO community_members (community_id, user_id, role, joined_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (community_id, user_id) DO UPDATE SET role = $3, updated_at = $5
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	added := 0
	for _, uid := range userIDs {
		_, err := stmt.ExecContext(ctx, communityID, uid, role, time.Now(), time.Now())
		if err != nil {
			continue
		}
		added++
	}

	// Update member count
	_, err = tx.ExecContext(ctx, `UPDATE communities SET member_count = member_count + $1 WHERE id = $2`, added, communityID)
	if err != nil {
		return fmt.Errorf("update member count failed: %w", err)
	}
	return tx.Commit()
}

// BulkRemoveMembers removes multiple members from a community.
func (r *communityRepo) BulkRemoveMembers(ctx context.Context, communityID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query, args, err := sqlx.In(`
		DELETE FROM community_members WHERE community_id = $1 AND user_id IN (?)
	`, userIDs)
	if err != nil {
		return err
	}
	args = append([]interface{}{communityID}, args...)
	query = r.getDB().Rebind(query)

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk remove members failed: %w", err)
	}
	removed, _ := result.RowsAffected()

	_, err = tx.ExecContext(ctx, `UPDATE communities SET member_count = GREATEST(member_count - $1, 0) WHERE id = $2`, removed, communityID)
	if err != nil {
		return fmt.Errorf("update member count failed: %w", err)
	}
	return tx.Commit()
}

// BulkUpdateRoles updates roles for multiple members.
func (r *communityRepo) BulkUpdateRoles(ctx context.Context, communityID string, updates map[string]string) error {
	if len(updates) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE community_members SET role = $1, updated_at = $2
		WHERE community_id = $3 AND user_id = $4
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for userID, role := range updates {
		_, err := stmt.ExecContext(ctx, role, time.Now(), communityID, userID)
		if err != nil {
			return fmt.Errorf("update role for user %s failed: %w", userID, err)
		}
	}
	return tx.Commit()
}

// BulkBanUsers bans multiple users from a community.
func (r *communityRepo) BulkBanUsers(ctx context.Context, communityID string, userIDs []string, reason string) error {
	if len(userIDs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO community_bans (community_id, user_id, reason, banned_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (community_id, user_id) DO UPDATE SET reason = $3, banned_at = $4
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Remove from members first
	for _, uid := range userIDs {
		_, _ = tx.ExecContext(ctx, `DELETE FROM community_members WHERE community_id = $1 AND user_id = $2`, communityID, uid)
		_, err := stmt.ExecContext(ctx, communityID, uid, reason, time.Now())
		if err != nil {
			return fmt.Errorf("ban user %s failed: %w", uid, err)
		}
	}

	// Update member count
	_, err = tx.ExecContext(ctx, `UPDATE communities SET member_count = GREATEST(member_count - $1, 0) WHERE id = $2`, int64(len(userIDs)), communityID)
	if err != nil {
		return fmt.Errorf("update member count failed: %w", err)
	}
	return tx.Commit()
}

// BulkUnbanUsers unbans multiple users from a community.
func (r *communityRepo) BulkUnbanUsers(ctx context.Context, communityID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM community_bans WHERE community_id = $1 AND user_id IN (?)`, userIDs)
	if err != nil {
		return err
	}
	args = append([]interface{}{communityID}, args...)
	query = r.getDB().Rebind(query)
	_, err = r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk unban users failed: %w", err)
	}
	return nil
}

// BulkAddPosts adds multiple posts to a community.
func (r *communityRepo) BulkAddPosts(ctx context.Context, communityID string, tweetIDs []string) error {
	if len(tweetIDs) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO community_posts (community_id, tweet_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (community_id, tweet_id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	added := 0
	for _, tid := range tweetIDs {
		result, err := stmt.ExecContext(ctx, communityID, tid, time.Now())
		if err != nil {
			continue
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			added++
		}
	}

	_, err = tx.ExecContext(ctx, `UPDATE communities SET post_count = post_count + $1 WHERE id = $2`, added, communityID)
	if err != nil {
		return fmt.Errorf("update post count failed: %w", err)
	}
	return tx.Commit()
}

// BulkRemovePosts removes multiple posts from a community.
func (r *communityRepo) BulkRemovePosts(ctx context.Context, communityID string, tweetIDs []string) error {
	if len(tweetIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In(`DELETE FROM community_posts WHERE community_id = $1 AND tweet_id IN (?)`, tweetIDs)
	if err != nil {
		return err
	}
	args = append([]interface{}{communityID}, args...)
	query = r.getDB().Rebind(query)

	result, err := r.getDB().ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bulk remove posts failed: %w", err)
	}
	removed, _ := result.RowsAffected()

	_, err = r.getDB().ExecContext(ctx, `UPDATE communities SET post_count = GREATEST(post_count - $1, 0) WHERE id = $2`, removed, communityID)
	if err != nil {
		return fmt.Errorf("update post count failed: %w", err)
	}
	return nil
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetCommunityStats returns aggregated community statistics.
func (r *communityRepo) GetCommunityStats(ctx context.Context) (*interfaces.CommunityStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_communities,
			SUM(CASE WHEN is_private = true THEN 1 ELSE 0 END) as private_communities,
			SUM(CASE WHEN is_private = false THEN 1 ELSE 0 END) as public_communities,
			SUM(member_count) as total_members,
			SUM(post_count) as total_posts,
			AVG(member_count) as average_members,
			AVG(post_count) as average_posts,
			MAX(member_count) as max_members,
			MIN(member_count) as min_members,
			MAX(created_at) as last_community_created
		FROM communities
		WHERE deleted_at IS NULL
	`
	var stats interfaces.CommunityStats
	err := r.getDB().GetContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("get community stats failed: %w", err)
	}
	return &stats, nil
}

// GetUserCommunityStats returns community stats for a specific user.
func (r *communityRepo) GetUserCommunityStats(ctx context.Context, userID string) (*interfaces.CommunityStats, error) {
	stats, err := r.GetCommunityStats(ctx)
	if err != nil {
		return nil, err
	}
	// Add user-specific stats
	var userStats struct {
		CreatedCount int64 `db:"created_count"`
		JoinedCount  int64 `db:"joined_count"`
	}
	err = r.getDB().GetContext(ctx, &userStats,
		`SELECT 
			(SELECT COUNT(*) FROM communities WHERE created_by = $1 AND deleted_at IS NULL) as created_count,
			(SELECT COUNT(*) FROM community_members WHERE user_id = $1) as joined_count`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("get user community stats failed: %w", err)
	}
	// Add fields to stats
	stats.TotalCommunities = userStats.CreatedCount
	return stats, nil
}

// GetDailyCommunityStats returns daily community counts for a date range.
func (r *communityRepo) GetDailyCommunityStats(ctx context.Context, start, end time.Time) ([]*interfaces.DailyCommunityCount, error) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as total,
			SUM(CASE WHEN is_private = true THEN 1 ELSE 0 END) as private,
			SUM(CASE WHEN is_private = false THEN 1 ELSE 0 END) as public,
			0 as new_members,
			0 as new_posts
		FROM communities
		WHERE created_at >= $1 AND created_at <= $2 AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`
	var results []*interfaces.DailyCommunityCount
	err := r.getDB().SelectContext(ctx, &results, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get daily community stats failed: %w", err)
	}
	// Fill in member and post counts from separate queries
	for i := range results {
		var members, posts int64
		_ = r.getDB().GetContext(ctx, &members,
			`SELECT COUNT(*) FROM community_members WHERE DATE(joined_at) = $1`,
			results[i].Date)
		_ = r.getDB().GetContext(ctx, &posts,
			`SELECT COUNT(*) FROM community_posts WHERE DATE(created_at) = $1`,
			results[i].Date)
		results[i].NewMembers = members
		results[i].NewPosts = posts
	}
	return results, nil
}

// GetCommunityGrowthRate calculates community growth rate over a period.
func (r *communityRepo) GetCommunityGrowthRate(ctx context.Context, communityID string, days int) (float64, error) {
	startDate := time.Now().AddDate(0, 0, -days)
	var startCount, endCount int64

	err := r.getDB().GetContext(ctx, &startCount,
		`SELECT COUNT(*) FROM community_members WHERE community_id = $1 AND joined_at <= $2`,
		communityID, startDate)
	if err != nil {
		return 0, fmt.Errorf("get start count failed: %w", err)
	}

	err = r.getDB().GetContext(ctx, &endCount,
		`SELECT COUNT(*) FROM community_members WHERE community_id = $1`,
		communityID)
	if err != nil {
		return 0, fmt.Errorf("get end count failed: %w", err)
	}

	if startCount == 0 {
		return float64(endCount), nil
	}
	return (float64(endCount-startCount) / float64(startCount)) * 100, nil
}

// GetTopCommunities returns the top communities by member count or post count.
func (r *communityRepo) GetTopCommunities(ctx context.Context, sortBy string, limit int) ([]*entities.Community, error) {
	if limit < 1 {
		limit = 10
	}
	var orderBy string
	switch sortBy {
	case "members", "member_count":
		orderBy = "member_count DESC"
	case "posts", "post_count":
		orderBy = "post_count DESC"
	default:
		orderBy = "member_count DESC"
	}
	query := fmt.Sprintf(`
		SELECT * FROM communities
		WHERE deleted_at IS NULL
		ORDER BY %s
		LIMIT $1
	`, orderBy)
	var communities []*entities.Community
	err := r.getDB().SelectContext(ctx, &communities, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get top communities failed: %w", err)
	}
	return communities, nil
}

// ======================================================================
// Health
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