// backend/internal/dto/admin_request.go
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
	MaxReasonLength    = 500
	MaxNoteLength      = 1000
	MaxFilterValueLen  = 100
	MaxActionLength    = 50
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrUserIDRequired       = errors.New("user ID is required")
	ErrReportIDRequired     = errors.New("report ID is required")
	ErrTweetIDRequired      = errors.New("tweet ID is required")
	ErrActionRequired       = errors.New("action is required")
	ErrReasonRequired       = errors.New("reason is required")
	ErrReasonTooLong        = fmt.Errorf("reason exceeds maximum length of %d characters", MaxReasonLength)
	ErrNoteTooLong          = fmt.Errorf("note exceeds maximum length of %d characters", MaxNoteLength)
	ErrInvalidAction        = errors.New("invalid action")
	ErrInvalidSortField     = errors.New("invalid sort field")
	ErrInvalidSortOrder     = errors.New("invalid sort order")
	ErrInvalidDateRange     = errors.New("invalid date range")
	ErrInvalidUserStatus    = errors.New("invalid user status")
	ErrInvalidReportStatus  = errors.New("invalid report status")
	ErrInvalidSeverity      = errors.New("invalid severity")
	ErrInvalidTargetType    = errors.New("invalid target type")
)

// ======================================================================
// Admin Action Types
// ======================================================================

// AdminAction represents possible admin actions.
type AdminAction string

const (
	ActionSuspendUser   AdminAction = "suspend_user"
	ActionUnsuspendUser AdminAction = "unsuspend_user"
	ActionDeleteUser    AdminAction = "delete_user"
	ActionRestoreUser   AdminAction = "restore_user"
	ActionVerifyUser    AdminAction = "verify_user"
	ActionUnverifyUser  AdminAction = "unverify_user"
	ActionDeleteTweet   AdminAction = "delete_tweet"
	ActionRestoreTweet  AdminAction = "restore_tweet"
	ActionResolveReport AdminAction = "resolve_report"
	ActionDismissReport AdminAction = "dismiss_report"
	ActionEscalateReport AdminAction = "escalate_report"
	ActionReopenReport  AdminAction = "reopen_report"
	ActionLockAccount   AdminAction = "lock_account"
	ActionUnlockAccount AdminAction = "unlock_account"
)

// ValidAdminActions returns all valid admin actions.
func ValidAdminActions() []AdminAction {
	return []AdminAction{
		ActionSuspendUser,
		ActionUnsuspendUser,
		ActionDeleteUser,
		ActionRestoreUser,
		ActionVerifyUser,
		ActionUnverifyUser,
		ActionDeleteTweet,
		ActionRestoreTweet,
		ActionResolveReport,
		ActionDismissReport,
		ActionEscalateReport,
		ActionReopenReport,
		ActionLockAccount,
		ActionUnlockAccount,
	}
}

// IsValid checks if an admin action is valid.
func (a AdminAction) IsValid() bool {
	for _, action := range ValidAdminActions() {
		if a == action {
			return true
		}
	}
	return false
}

// String returns the string representation.
func (a AdminAction) String() string {
	return string(a)
}

// ======================================================================
// User Management DTOs
// ======================================================================

// AdminUserFilterRequest represents user filter options for admin.
type AdminUserFilterRequest struct {
	Username    *string    `json:"username,omitempty"`
	Email       *string    `json:"email,omitempty"`
	FullName    *string    `json:"full_name,omitempty"`
	Role        *string    `json:"role,omitempty"` // "user", "moderator", "admin"
	Status      *string    `json:"status,omitempty"` // "active", "inactive", "suspended", "deleted"
	IsVerified  *bool      `json:"is_verified,omitempty"`
	IsSuspended *bool      `json:"is_suspended,omitempty"`
	JoinedFrom  *time.Time `json:"joined_from,omitempty"`
	JoinedTo    *time.Time `json:"joined_to,omitempty"`
	Search      *string    `json:"search,omitempty"` // full-text search
	SortBy      string     `json:"sort_by,omitempty"`
	SortOrder   string     `json:"sort_order,omitempty"` // "asc", "desc"
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

// Validate validates the filter request.
func (f *AdminUserFilterRequest) Validate() error {
	if f.Role != nil {
		validRoles := map[string]bool{"user": true, "moderator": true, "admin": true}
		if !validRoles[*f.Role] {
			return ErrInvalidUserStatus
		}
	}
	if f.Status != nil {
		validStatus := map[string]bool{"active": true, "inactive": true, "suspended": true, "deleted": true}
		if !validStatus[*f.Status] {
			return ErrInvalidUserStatus
		}
	}
	if f.JoinedFrom != nil && f.JoinedTo != nil && f.JoinedFrom.After(*f.JoinedTo) {
		return ErrInvalidDateRange
	}
	if f.SortBy != "" {
		allowed := map[string]bool{
			"username": true, "email": true, "full_name": true,
			"role": true, "status": true, "is_verified": true,
			"created_at": true, "updated_at": true,
		}
		if !allowed[f.SortBy] {
			return ErrInvalidSortField
		}
	}
	if f.SortOrder != "" && f.SortOrder != "asc" && f.SortOrder != "desc" {
		return ErrInvalidSortOrder
	}
	if f.Limit < 0 || f.Limit > 100 {
		return errors.New("limit must be between 0 and 100")
	}
	if f.Offset < 0 {
		return errors.New("offset must be non-negative")
	}
	return nil
}

// Sanitize sanitizes the filter request.
func (f *AdminUserFilterRequest) Sanitize() {
	if f.Username != nil {
		trimmed := strings.TrimSpace(*f.Username)
		f.Username = &trimmed
	}
	if f.Email != nil {
		trimmed := strings.TrimSpace(*f.Email)
		f.Email = &trimmed
	}
	if f.FullName != nil {
		trimmed := strings.TrimSpace(*f.FullName)
		f.FullName = &trimmed
	}
	if f.Search != nil {
		trimmed := strings.TrimSpace(*f.Search)
		f.Search = &trimmed
	}
	if f.SortBy != "" {
		f.SortBy = strings.ToLower(strings.TrimSpace(f.SortBy))
	}
	if f.SortOrder != "" {
		f.SortOrder = strings.ToLower(strings.TrimSpace(f.SortOrder))
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

// AdminUserUpdateRequest represents user update by admin.
type AdminUserUpdateRequest struct {
	ID          string  `json:"id" binding:"required"`
	Username    *string `json:"username,omitempty"`
	Email       *string `json:"email,omitempty"`
	FullName    *string `json:"full_name,omitempty"`
	Bio         *string `json:"bio,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	BannerURL   *string `json:"banner_url,omitempty"`
	Location    *string `json:"location,omitempty"`
	Website     *string `json:"website,omitempty"`
	Role        *string `json:"role,omitempty"`
	Status      *string `json:"status,omitempty"`
	IsVerified  *bool   `json:"is_verified,omitempty"`
	IsPrivate   *bool   `json:"is_private,omitempty"`
}

// Validate validates the user update request.
func (r *AdminUserUpdateRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrUserIDRequired
	}
	if r.Role != nil {
		validRoles := map[string]bool{"user": true, "moderator": true, "admin": true}
		if !validRoles[*r.Role] {
			return errors.New("invalid role")
		}
	}
	if r.Status != nil {
		validStatus := map[string]bool{"active": true, "inactive": true, "suspended": true, "deleted": true}
		if !validStatus[*r.Status] {
			return ErrInvalidUserStatus
		}
	}
	if r.Bio != nil && len(*r.Bio) > MaxBioLength {
		return ErrBioTooLong
	}
	return nil
}

// Sanitize sanitizes the update request.
func (r *AdminUserUpdateRequest) Sanitize() {
	r.ID = strings.TrimSpace(r.ID)
	if r.Username != nil {
		trimmed := strings.TrimSpace(*r.Username)
		r.Username = &trimmed
	}
	if r.Email != nil {
		trimmed := strings.TrimSpace(*r.Email)
		r.Email = &trimmed
	}
	if r.FullName != nil {
		trimmed := strings.TrimSpace(*r.FullName)
		r.FullName = &trimmed
	}
	if r.Bio != nil {
		trimmed := strings.TrimSpace(*r.Bio)
		r.Bio = &trimmed
	}
	if r.AvatarURL != nil {
		trimmed := strings.TrimSpace(*r.AvatarURL)
		r.AvatarURL = &trimmed
	}
	if r.BannerURL != nil {
		trimmed := strings.TrimSpace(*r.BannerURL)
		r.BannerURL = &trimmed
	}
	if r.Location != nil {
		trimmed := strings.TrimSpace(*r.Location)
		r.Location = &trimmed
	}
	if r.Website != nil {
		trimmed := strings.TrimSpace(*r.Website)
		r.Website = &trimmed
	}
}

// AdminUserActionRequest represents an action on a user.
type AdminUserActionRequest struct {
	UserID    string `json:"user_id" binding:"required"`
	Action    string `json:"action" binding:"required"`
	Reason    string `json:"reason,omitempty"`
	Note      string `json:"note,omitempty"`
	Duration  int    `json:"duration,omitempty"` // duration in days for suspension
}

// Validate validates the user action request.
func (r *AdminUserActionRequest) Validate() error {
	if strings.TrimSpace(r.UserID) == "" {
		return ErrUserIDRequired
	}
	if strings.TrimSpace(r.Action) == "" {
		return ErrActionRequired
	}
	action := AdminAction(r.Action)
	if !action.IsValid() {
		return ErrInvalidAction
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	if len(r.Note) > MaxNoteLength {
		return ErrNoteTooLong
	}
	if action == ActionSuspendUser && r.Duration <= 0 {
		return errors.New("duration is required for suspension")
	}
	return nil
}

// Sanitize sanitizes the action request.
func (r *AdminUserActionRequest) Sanitize() {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Action = strings.TrimSpace(r.Action)
	r.Reason = strings.TrimSpace(r.Reason)
	r.Note = strings.TrimSpace(r.Note)
}

// ======================================================================
// Moderation / Report DTOs
// ======================================================================

// AdminReportFilterRequest represents report filter options.
type AdminReportFilterRequest struct {
	TargetID    *string    `json:"target_id,omitempty"`
	TargetType  *string    `json:"target_type,omitempty"` // "tweet", "user", "community", "message"
	ReporterID  *string    `json:"reporter_id,omitempty"`
	Status      *string    `json:"status,omitempty"` // "pending", "under_review", "resolved", "dismissed", "escalated", "closed"
	Severity    *string    `json:"severity,omitempty"` // "low", "medium", "high", "critical"
	Type        *string    `json:"type,omitempty"` // "spam", "harassment", etc.
	ReviewerID  *string    `json:"reviewer_id,omitempty"`
	CreatedFrom *time.Time `json:"created_from,omitempty"`
	CreatedTo   *time.Time `json:"created_to,omitempty"`
	SortBy      string     `json:"sort_by,omitempty"`
	SortOrder   string     `json:"sort_order,omitempty"`
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

// Validate validates the report filter request.
func (f *AdminReportFilterRequest) Validate() error {
	if f.Status != nil {
		validStatus := map[string]bool{
			"pending": true, "under_review": true, "resolved": true,
			"dismissed": true, "escalated": true, "closed": true,
		}
		if !validStatus[*f.Status] {
			return ErrInvalidReportStatus
		}
	}
	if f.Severity != nil {
		validSeverity := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
		if !validSeverity[*f.Severity] {
			return ErrInvalidSeverity
		}
	}
	if f.TargetType != nil {
		validTarget := map[string]bool{"tweet": true, "user": true, "community": true, "message": true}
		if !validTarget[*f.TargetType] {
			return ErrInvalidTargetType
		}
	}
	if f.CreatedFrom != nil && f.CreatedTo != nil && f.CreatedFrom.After(*f.CreatedTo) {
		return ErrInvalidDateRange
	}
	if f.SortBy != "" {
		allowed := map[string]bool{
			"created_at": true, "updated_at": true, "resolved_at": true,
			"severity": true, "status": true, "type": true,
		}
		if !allowed[f.SortBy] {
			return ErrInvalidSortField
		}
	}
	if f.SortOrder != "" && f.SortOrder != "asc" && f.SortOrder != "desc" {
		return ErrInvalidSortOrder
	}
	if f.Limit < 0 || f.Limit > 100 {
		return errors.New("limit must be between 0 and 100")
	}
	if f.Offset < 0 {
		return errors.New("offset must be non-negative")
	}
	return nil
}

// Sanitize sanitizes the filter request.
func (f *AdminReportFilterRequest) Sanitize() {
	if f.TargetID != nil {
		trimmed := strings.TrimSpace(*f.TargetID)
		f.TargetID = &trimmed
	}
	if f.ReporterID != nil {
		trimmed := strings.TrimSpace(*f.ReporterID)
		f.ReporterID = &trimmed
	}
	if f.ReviewerID != nil {
		trimmed := strings.TrimSpace(*f.ReviewerID)
		f.ReviewerID = &trimmed
	}
	if f.SortBy != "" {
		f.SortBy = strings.ToLower(strings.TrimSpace(f.SortBy))
	}
	if f.SortOrder != "" {
		f.SortOrder = strings.ToLower(strings.TrimSpace(f.SortOrder))
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

// AdminReportActionRequest represents an action on a report.
type AdminReportActionRequest struct {
	ReportID   string `json:"report_id" binding:"required"`
	Action     string `json:"action" binding:"required"` // "resolve", "dismiss", "escalate", "reopen"
	ReviewerID string `json:"reviewer_id,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// Validate validates the report action request.
func (r *AdminReportActionRequest) Validate() error {
	if strings.TrimSpace(r.ReportID) == "" {
		return ErrReportIDRequired
	}
	if strings.TrimSpace(r.Action) == "" {
		return ErrActionRequired
	}
	validActions := map[string]bool{"resolve": true, "dismiss": true, "escalate": true, "reopen": true}
	if !validActions[r.Action] {
		return ErrInvalidAction
	}
	if len(r.Notes) > MaxNoteLength {
		return ErrNoteTooLong
	}
	return nil
}

// Sanitize sanitizes the action request.
func (r *AdminReportActionRequest) Sanitize() {
	r.ReportID = strings.TrimSpace(r.ReportID)
	r.Action = strings.TrimSpace(r.Action)
	r.ReviewerID = strings.TrimSpace(r.ReviewerID)
	r.Notes = strings.TrimSpace(r.Notes)
}

// ======================================================================
// Tweet Moderation DTOs
// ======================================================================

// AdminTweetFilterRequest represents tweet filter options for admin.
type AdminTweetFilterRequest struct {
	UserID      *string    `json:"user_id,omitempty"`
	Content     *string    `json:"content,omitempty"`
	HasMedia    *bool      `json:"has_media,omitempty"`
	IsPoll      *bool      `json:"is_poll,omitempty"`
	IsReply     *bool      `json:"is_reply,omitempty"`
	IsRetweet   *bool      `json:"is_retweet,omitempty"`
	Status      *string    `json:"status,omitempty"` // "active", "deleted", "all"
	CreatedFrom *time.Time `json:"created_from,omitempty"`
	CreatedTo   *time.Time `json:"created_to,omitempty"`
	MinLikes    *int64     `json:"min_likes,omitempty"`
	MinRetweets *int64     `json:"min_retweets,omitempty"`
	SortBy      string     `json:"sort_by,omitempty"`
	SortOrder   string     `json:"sort_order,omitempty"`
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

// Validate validates the tweet filter request.
func (f *AdminTweetFilterRequest) Validate() error {
	if f.Status != nil && *f.Status != "" {
		validStatus := map[string]bool{"active": true, "deleted": true, "all": true}
		if !validStatus[*f.Status] {
			return errors.New("invalid status")
		}
	}
	if f.CreatedFrom != nil && f.CreatedTo != nil && f.CreatedFrom.After(*f.CreatedTo) {
		return ErrInvalidDateRange
	}
	if f.SortBy != "" {
		allowed := map[string]bool{
			"created_at": true, "updated_at": true, "likes": true,
			"retweets": true, "replies": true,
		}
		if !allowed[f.SortBy] {
			return ErrInvalidSortField
		}
	}
	if f.SortOrder != "" && f.SortOrder != "asc" && f.SortOrder != "desc" {
		return ErrInvalidSortOrder
	}
	if f.Limit < 0 || f.Limit > 100 {
		return errors.New("limit must be between 0 and 100")
	}
	if f.Offset < 0 {
		return errors.New("offset must be non-negative")
	}
	return nil
}

// Sanitize sanitizes the filter request.
func (f *AdminTweetFilterRequest) Sanitize() {
	if f.UserID != nil {
		trimmed := strings.TrimSpace(*f.UserID)
		f.UserID = &trimmed
	}
	if f.Content != nil {
		trimmed := strings.TrimSpace(*f.Content)
		f.Content = &trimmed
	}
	if f.SortBy != "" {
		f.SortBy = strings.ToLower(strings.TrimSpace(f.SortBy))
	}
	if f.SortOrder != "" {
		f.SortOrder = strings.ToLower(strings.TrimSpace(f.SortOrder))
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

// AdminTweetActionRequest represents an action on a tweet.
type AdminTweetActionRequest struct {
	TweetID string `json:"tweet_id" binding:"required"`
	Action  string `json:"action" binding:"required"` // "delete", "restore"
	Reason  string `json:"reason,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Validate validates the tweet action request.
func (r *AdminTweetActionRequest) Validate() error {
	if strings.TrimSpace(r.TweetID) == "" {
		return ErrTweetIDRequired
	}
	if strings.TrimSpace(r.Action) == "" {
		return ErrActionRequired
	}
	if r.Action != "delete" && r.Action != "restore" {
		return ErrInvalidAction
	}
	if len(r.Reason) > MaxReasonLength {
		return ErrReasonTooLong
	}
	if len(r.Note) > MaxNoteLength {
		return ErrNoteTooLong
	}
	return nil
}

// Sanitize sanitizes the action request.
func (r *AdminTweetActionRequest) Sanitize() {
	r.TweetID = strings.TrimSpace(r.TweetID)
	r.Action = strings.TrimSpace(r.Action)
	r.Reason = strings.TrimSpace(r.Reason)
	r.Note = strings.TrimSpace(r.Note)
}

// ======================================================================
// System Settings DTOs
// ======================================================================

// AdminSystemSettingsRequest represents system settings update.
type AdminSystemSettingsRequest struct {
	SiteName        *string `json:"site_name,omitempty"`
	SiteDescription *string `json:"site_description,omitempty"`
	MaxTweetLength  *int    `json:"max_tweet_length,omitempty"`
	MaxMediaCount   *int    `json:"max_media_count,omitempty"`
	MaxImageSizeMB  *int    `json:"max_image_size_mb,omitempty"`
	MaxVideoSizeMB  *int    `json:"max_video_size_mb,omitempty"`
	AllowRegistration *bool `json:"allow_registration,omitempty"`
	RequireEmailVerification *bool `json:"require_email_verification,omitempty"`
	DefaultLanguage *string `json:"default_language,omitempty"`
	DefaultTheme    *string `json:"default_theme,omitempty"`
	MaintenanceMode *bool   `json:"maintenance_mode,omitempty"`
	MaintenanceMessage *string `json:"maintenance_message,omitempty"`
}

// Validate validates the settings request.
func (r *AdminSystemSettingsRequest) Validate() error {
	if r.MaxTweetLength != nil && (*r.MaxTweetLength < 1 || *r.MaxTweetLength > 1000) {
		return errors.New("max tweet length must be between 1 and 1000")
	}
	if r.MaxMediaCount != nil && (*r.MaxMediaCount < 0 || *r.MaxMediaCount > 20) {
		return errors.New("max media count must be between 0 and 20")
	}
	if r.MaxImageSizeMB != nil && (*r.MaxImageSizeMB < 1 || *r.MaxImageSizeMB > 100) {
		return errors.New("max image size must be between 1 and 100 MB")
	}
	if r.MaxVideoSizeMB != nil && (*r.MaxVideoSizeMB < 1 || *r.MaxVideoSizeMB > 500) {
		return errors.New("max video size must be between 1 and 500 MB")
	}
	return nil
}

// Sanitize sanitizes the settings request.
func (r *AdminSystemSettingsRequest) Sanitize() {
	if r.SiteName != nil {
		trimmed := strings.TrimSpace(*r.SiteName)
		r.SiteName = &trimmed
	}
	if r.SiteDescription != nil {
		trimmed := strings.TrimSpace(*r.SiteDescription)
		r.SiteDescription = &trimmed
	}
	if r.DefaultLanguage != nil {
		trimmed := strings.TrimSpace(*r.DefaultLanguage)
		r.DefaultLanguage = &trimmed
	}
	if r.DefaultTheme != nil {
		trimmed := strings.TrimSpace(*r.DefaultTheme)
		r.DefaultTheme = &trimmed
	}
	if r.MaintenanceMessage != nil {
		trimmed := strings.TrimSpace(*r.MaintenanceMessage)
		r.MaintenanceMessage = &trimmed
	}
}

// ======================================================================
= Analytics DTOs
// ======================================================================

// AdminAnalyticsRequest represents analytics query parameters.
type AdminAnalyticsRequest struct {
	Metric      string     `json:"metric" binding:"required"` // "users", "tweets", "likes", "retweets", "engagement", "growth"
	Period      string     `json:"period,omitempty"` // "hour", "day", "week", "month", "year"
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	GroupBy     string     `json:"group_by,omitempty"` // "day", "week", "month"
	Limit       int        `json:"limit,omitempty"`
}

// Validate validates the analytics request.
func (r *AdminAnalyticsRequest) Validate() error {
	if strings.TrimSpace(r.Metric) == "" {
		return errors.New("metric is required")
	}
	validMetrics := map[string]bool{
		"users": true, "tweets": true, "likes": true,
		"retweets": true, "engagement": true, "growth": true,
		"signups": true, "logins": true, "reports": true,
	}
	if !validMetrics[r.Metric] {
		return errors.New("invalid metric")
	}
	if r.Period != "" {
		validPeriods := map[string]bool{"hour": true, "day": true, "week": true, "month": true, "year": true}
		if !validPeriods[r.Period] {
			return errors.New("invalid period")
		}
	}
	if r.StartDate != nil && r.EndDate != nil && r.StartDate.After(*r.EndDate) {
		return ErrInvalidDateRange
	}
	if r.GroupBy != "" {
		validGroup := map[string]bool{"day": true, "week": true, "month": true}
		if !validGroup[r.GroupBy] {
			return errors.New("invalid group_by")
		}
	}
	if r.Limit < 0 || r.Limit > 1000 {
		return errors.New("limit must be between 0 and 1000")
	}
	return nil
}

// Sanitize sanitizes the analytics request.
func (r *AdminAnalyticsRequest) Sanitize() {
	r.Metric = strings.TrimSpace(r.Metric)
	r.Period = strings.TrimSpace(r.Period)
	if r.GroupBy != "" {
		r.GroupBy = strings.ToLower(strings.TrimSpace(r.GroupBy))
	}
	if r.Limit < 1 {
		r.Limit = 100
	}
	if r.Limit > 1000 {
		r.Limit = 1000
	}
}

// ======================================================================
= Audit Log DTOs
// ======================================================================

// AdminAuditLogFilterRequest represents audit log filter options.
type AdminAuditLogFilterRequest struct {
	UserID      *string    `json:"user_id,omitempty"`
	Action      *string    `json:"action,omitempty"`
	Resource    *string    `json:"resource,omitempty"`
	IP          *string    `json:"ip,omitempty"`
	Status      *string    `json:"status,omitempty"` // "success", "failure"
	CreatedFrom *time.Time `json:"created_from,omitempty"`
	CreatedTo   *time.Time `json:"created_to,omitempty"`
	SortBy      string     `json:"sort_by,omitempty"`
	SortOrder   string     `json:"sort_order,omitempty"`
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

// Validate validates the audit log filter request.
func (f *AdminAuditLogFilterRequest) Validate() error {
	if f.Status != nil && *f.Status != "" {
		if *f.Status != "success" && *f.Status != "failure" {
			return errors.New("invalid status")
		}
	}
	if f.CreatedFrom != nil && f.CreatedTo != nil && f.CreatedFrom.After(*f.CreatedTo) {
		return ErrInvalidDateRange
	}
	if f.SortBy != "" {
		allowed := map[string]bool{
			"created_at": true, "action": true, "user_id": true,
			"resource": true, "status": true,
		}
		if !allowed[f.SortBy] {
			return ErrInvalidSortField
		}
	}
	if f.SortOrder != "" && f.SortOrder != "asc" && f.SortOrder != "desc" {
		return ErrInvalidSortOrder
	}
	if f.Limit < 0 || f.Limit > 1000 {
		return errors.New("limit must be between 0 and 1000")
	}
	if f.Offset < 0 {
		return errors.New("offset must be non-negative")
	}
	return nil
}

// Sanitize sanitizes the filter request.
func (f *AdminAuditLogFilterRequest) Sanitize() {
	if f.UserID != nil {
		trimmed := strings.TrimSpace(*f.UserID)
		f.UserID = &trimmed
	}
	if f.Action != nil {
		trimmed := strings.TrimSpace(*f.Action)
		f.Action = &trimmed
	}
	if f.Resource != nil {
		trimmed := strings.TrimSpace(*f.Resource)
		f.Resource = &trimmed
	}
	if f.IP != nil {
		trimmed := strings.TrimSpace(*f.IP)
		f.IP = &trimmed
	}
	if f.SortBy != "" {
		f.SortBy = strings.ToLower(strings.TrimSpace(f.SortBy))
	}
	if f.SortOrder != "" {
		f.SortOrder = strings.ToLower(strings.TrimSpace(f.SortOrder))
	}
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 1000 {
		f.Limit = 1000
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
}

// ======================================================================
= Admin Response DTOs
// ======================================================================

// AdminUserResponse represents admin user detail response.
type AdminUserResponse struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	FullName    string     `json:"full_name"`
	Bio         string     `json:"bio,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	BannerURL   string     `json:"banner_url,omitempty"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	IsVerified  bool       `json:"is_verified"`
	IsPrivate   bool       `json:"is_private"`
	JoinedAt    time.Time  `json:"joined_at"`
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
	TweetCount  int64      `json:"tweet_count"`
	FollowerCount int64    `json:"follower_count"`
	FollowingCount int64   `json:"following_count"`
}

// AdminReportResponse represents admin report detail response.
type AdminReportResponse struct {
	ID          string                 `json:"id"`
	ReporterID  string                 `json:"reporter_id"`
	TargetID    string                 `json:"target_id"`
	TargetType  string                 `json:"target_type"`
	Type        string                 `json:"type"`
	Reason      string                 `json:"reason"`
	Description string                 `json:"description,omitempty"`
	Status      string                 `json:"status"`
	Severity    string                 `json:"severity"`
	ReviewerID  string                 `json:"reviewer_id,omitempty"`
	ReviewNotes string                 `json:"review_notes,omitempty"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AdminDashboardStats represents dashboard statistics.
type AdminDashboardStats struct {
	TotalUsers      int64     `json:"total_users"`
	ActiveUsers     int64     `json:"active_users"`
	SuspendedUsers  int64     `json:"suspended_users"`
	TotalTweets     int64     `json:"total_tweets"`
	TotalLikes      int64     `json:"total_likes"`
	TotalRetweets   int64     `json:"total_retweets"`
	TotalReports    int64     `json:"total_reports"`
	PendingReports  int64     `json:"pending_reports"`
	NewUsersToday   int64     `json:"new_users_today"`
	NewTweetsToday  int64     `json:"new_tweets_today"`
	EngagementRate  float64   `json:"engagement_rate"`
	ServerUptime    float64   `json:"server_uptime"`
	LastUpdated     time.Time `json:"last_updated"`
}

// ======================================================================
= Builder Methods for Testing
// ======================================================================

// NewAdminUserFilterRequest creates a default filter request.
func NewAdminUserFilterRequest() *AdminUserFilterRequest {
	return &AdminUserFilterRequest{
		Limit:  20,
		Offset: 0,
	}
}

// NewAdminReportFilterRequest creates a default report filter request.
func NewAdminReportFilterRequest() *AdminReportFilterRequest {
	return &AdminReportFilterRequest{
		Limit:  20,
		Offset: 0,
	}
}

// NewAdminTweetFilterRequest creates a default tweet filter request.
func NewAdminTweetFilterRequest() *AdminTweetFilterRequest {
	return &AdminTweetFilterRequest{
		Limit:  20,
		Offset: 0,
	}
}

// NewAdminAnalyticsRequest creates a default analytics request.
func NewAdminAnalyticsRequest(metric string) *AdminAnalyticsRequest {
	return &AdminAnalyticsRequest{
		Metric: metric,
		Limit:  100,
	}
}

// ======================================================================
= JSON Serialization Helpers
// ======================================================================

// MarshalJSON ensures proper JSON serialization.
func (r *AdminUserFilterRequest) MarshalJSON() ([]byte, error) {
	type Alias AdminUserFilterRequest
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

// MarshalJSON ensures proper JSON serialization.
func (r *AdminReportFilterRequest) MarshalJSON() ([]byte, error) {
	type Alias AdminReportFilterRequest
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

// ======================================================================
= Test Helpers
// ======================================================================

// MustValidate panics if validation fails.
func MustValidate(req interface{ Validate() error }) {
	if err := req.Validate(); err != nil {
		panic(err)
	}
}

// MustSanitize panics if sanitize fails (no-op).
func MustSanitize(req interface{ Sanitize() }) {
	req.Sanitize()
}