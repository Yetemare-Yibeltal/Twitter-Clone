// backend/internal/handler/media_handler.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/pkg/logger"
)

// MediaHandler handles all media-related HTTP endpoints.
type MediaHandler struct {
	storageAdapter adapter.StorageAdapter
	mediaService   service.MediaService
	tweetService   service.TweetService
	userService    service.UserService
	config         *MediaConfig
	log            *logrus.Entry
}

// MediaConfig holds media configuration.
type MediaConfig struct {
	ServePath          string        `json:"serve_path"`
	MaxImageSize       int64         `json:"max_image_size"`
	MaxVideoSize       int64         `json:"max_video_size"`
	ThumbnailSize      int           `json:"thumbnail_size"`
	CacheControl       string        `json:"cache_control"`
	AllowedImageTypes  []string      `json:"allowed_image_types"`
	AllowedVideoTypes  []string      `json:"allowed_video_types"`
	ThumbnailQuality   int           `json:"thumbnail_quality"`
	EnableCDN          bool          `json:"enable_cdn"`
	CDNURL             string        `json:"cdn_url"`
	DefaultThumbnail   string        `json:"default_thumbnail"`
	MediaTTL           time.Duration `json:"media_ttl"`
}

// DefaultMediaConfig returns default media configuration.
func DefaultMediaConfig() *MediaConfig {
	return &MediaConfig{
		ServePath:         "/media",
		MaxImageSize:      10 * 1024 * 1024,
		MaxVideoSize:      50 * 1024 * 1024,
		ThumbnailSize:     150,
		CacheControl:      "public, max-age=31536000, immutable",
		AllowedImageTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml"},
		AllowedVideoTypes: []string{"video/mp4", "video/quicktime", "video/webm", "video/avi"},
		ThumbnailQuality:  85,
		EnableCDN:         false,
		MediaTTL:          7 * 24 * time.Hour,
	}
}

// NewMediaHandler creates a new media handler.
func NewMediaHandler(
	storageAdapter adapter.StorageAdapter,
	mediaService service.MediaService,
	tweetService service.TweetService,
	userService service.UserService,
) *MediaHandler {
	return &MediaHandler{
		storageAdapter: storageAdapter,
		mediaService:   mediaService,
		tweetService:   tweetService,
		userService:    userService,
		config:         DefaultMediaConfig(),
		log:            logger.WithField("handler", "media"),
	}
}

// ======================================================================
// Serve Media
// ======================================================================

// ServeMedia handles serving a media file.
// @Summary Serve media
// @Description Serves a media file by ID or path
// @Tags media
// @Produce image/jpeg, image/png, video/mp4, etc.
// @Param id path string true "Media ID or path"
// @Param size query string false "Size variant (original, small, medium, large, thumbnail)"
// @Success 200 {file} file
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /media/{id} [get]
func (h *MediaHandler) ServeMedia(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mediaID := vars["id"]
	if mediaID == "" {
		h.sendError(w, http.StatusBadRequest, "Media ID required", nil)
		return
	}

	size := r.URL.Query().Get("size")
	if size == "" {
		size = "original"
	}

	// Get media metadata
	media, err := h.mediaService.GetMediaByID(r.Context(), mediaID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get media")
		return
	}

	// Check if user can access this media
	if !h.canAccessMedia(r.Context(), media) {
		h.sendError(w, http.StatusForbidden, "Access denied", nil)
		return
	}

	// Determine which file to serve
	filePath := media.Path
	if size != "original" && size != "" {
		// Check if thumbnail exists
		if media.ThumbnailPath != "" {
			filePath = media.ThumbnailPath
		}
	}

	// Set cache headers
	if h.config.CacheControl != "" {
		w.Header().Set("Cache-Control", h.config.CacheControl)
	}

	// If CDN is enabled, redirect to CDN URL
	if h.config.EnableCDN && h.config.CDNURL != "" {
		cdnURL := fmt.Sprintf("%s/%s", h.config.CDNURL, filePath)
		http.Redirect(w, r, cdnURL, http.StatusFound)
		return
	}

	// Stream the file
	if err := h.storageAdapter.Download(r.Context(), filePath, w); err != nil {
		h.handleServiceError(w, err, "Failed to serve media")
		return
	}
}

// ======================================================================
= Get Media Metadata
// ======================================================================

// GetMediaMetadata handles retrieving media metadata.
// @Summary Get media metadata
// @Description Retrieves metadata for a media file
// @Tags media
// @Security BearerAuth
// @Produce json
// @Param id path string true "Media ID"
// @Success 200 {object} dto.MediaMetadataResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/media/{id}/metadata [get]
func (h *MediaHandler) GetMediaMetadata(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	mediaID := vars["id"]
	if mediaID == "" {
		h.sendError(w, http.StatusBadRequest, "Media ID required", nil)
		return
	}

	// Get media
	media, err := h.mediaService.GetMediaByID(r.Context(), mediaID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get media")
		return
	}

	// Check ownership
	if !h.isMediaOwner(r.Context(), media, userID) {
		role, _ := middleware.GetUserRole(r.Context())
		if role != "admin" {
			h.sendError(w, http.StatusForbidden, "Access denied", nil)
			return
		}
	}

	// Build response
	response := &dto.MediaMetadataResponse{
		ID:          media.ID,
		Filename:    media.Filename,
		Path:        media.Path,
		ThumbnailPath: media.ThumbnailPath,
		Size:        media.Size,
		ContentType: media.ContentType,
		Width:       media.Width,
		Height:      media.Height,
		Duration:    media.Duration,
		UploadedBy:  media.UploadedBy,
		UploadedAt:  media.CreatedAt,
		Tags:        media.Tags,
		IsPrivate:   media.IsPrivate,
	}

	h.sendSuccess(w, http.StatusOK, response)
}

// ======================================================================
= Delete Media
// ======================================================================

// DeleteMedia handles deleting a media file.
// @Summary Delete media
// @Description Deletes a media file (owner or admin only)
// @Tags media
// @Security BearerAuth
// @Param id path string true "Media ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/media/{id} [delete]
func (h *MediaHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	mediaID := vars["id"]
	if mediaID == "" {
		h.sendError(w, http.StatusBadRequest, "Media ID required", nil)
		return
	}

	// Get media
	media, err := h.mediaService.GetMediaByID(r.Context(), mediaID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get media")
		return
	}

	// Check ownership
	if !h.isMediaOwner(r.Context(), media, userID) {
		role, _ := middleware.GetUserRole(r.Context())
		if role != "admin" {
			h.sendError(w, http.StatusForbidden, "Access denied", nil)
			return
		}
	}

	// Delete from storage
	if err := h.storageAdapter.Delete(r.Context(), media.Path); err != nil {
		h.handleServiceError(w, err, "Failed to delete media file")
		return
	}
	if media.ThumbnailPath != "" {
		_ = h.storageAdapter.Delete(r.Context(), media.ThumbnailPath)
	}

	// Delete from database
	if err := h.mediaService.DeleteMedia(r.Context(), mediaID); err != nil {
		h.handleServiceError(w, err, "Failed to delete media record")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Media deleted successfully",
	})
}

// ======================================================================
= Bulk Delete Media
// ======================================================================

// BulkDeleteMedia handles deleting multiple media files.
// @Summary Bulk delete media
// @Description Deletes multiple media files (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.BulkDeleteMediaRequest true "Media IDs"
// @Success 200 {object} dto.BulkDeleteResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/media/bulk-delete [post]
func (h *MediaHandler) BulkDeleteMedia(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	var req dto.BulkDeleteMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	results, err := h.mediaService.BulkDeleteMedia(r.Context(), req.MediaIDs)
	if err != nil {
		h.handleServiceError(w, err, "Failed to delete media")
		return
	}

	h.sendSuccess(w, http.StatusOK, results)
}

// ======================================================================
= Generate Thumbnail
// ======================================================================

// GenerateThumbnail handles generating a thumbnail for a media file.
// @Summary Generate thumbnail
// @Description Generates a thumbnail for a media file
// @Tags media
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.GenerateThumbnailRequest true "Generation options"
// @Success 200 {object} dto.ThumbnailResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/media/thumbnail [post]
func (h *MediaHandler) GenerateThumbnail(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.GenerateThumbnailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Get media
	media, err := h.mediaService.GetMediaByID(r.Context(), req.MediaID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get media")
		return
	}

	// Check ownership
	if !h.isMediaOwner(r.Context(), media, userID) {
		role, _ := middleware.GetUserRole(r.Context())
		if role != "admin" {
			h.sendError(w, http.StatusForbidden, "Access denied", nil)
			return
		}
	}

	// Generate thumbnail
	thumbnailPath, err := h.storageAdapter.GenerateThumbnail(r.Context(), media.Path, h.config.ThumbnailSize, h.config.ThumbnailQuality)
	if err != nil {
		h.handleServiceError(w, err, "Failed to generate thumbnail")
		return
	}

	// Update media record
	media.ThumbnailPath = thumbnailPath
	if err := h.mediaService.UpdateMedia(r.Context(), media); err != nil {
		h.handleServiceError(w, err, "Failed to update media record")
		return
	}

	// Get thumbnail URL
	thumbnailURL := h.getMediaURL(thumbnailPath, "thumbnail")

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"message":        "Thumbnail generated successfully",
		"thumbnail_path": thumbnailPath,
		"thumbnail_url":  thumbnailURL,
	})
}

// ======================================================================
= Get Media Usage
// ======================================================================

// GetMediaUsage handles retrieving media usage statistics.
// @Summary Get media usage
// @Description Retrieves media usage statistics for a user or globally
// @Tags media
// @Security BearerAuth
// @Produce json
// @Param user_id query string false "User ID (admin only for others)"
// @Success 200 {object} dto.MediaUsageResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/media/usage [get]
func (h *MediaHandler) GetMediaUsage(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	targetUserID := r.URL.Query().Get("user_id")
	role, _ := middleware.GetUserRole(r.Context())

	// If targetUserID is specified, only admin can access
	if targetUserID != "" && targetUserID != userID && role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}
	if targetUserID == "" {
		targetUserID = userID
	}

	usage, err := h.mediaService.GetMediaUsage(r.Context(), targetUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get media usage")
		return
	}

	h.sendSuccess(w, http.StatusOK, usage)
}

// ======================================================================
= Admin List Media
// ======================================================================

// AdminListMedia handles admin listing of all media.
// @Summary Admin list media
// @Description Lists all media with pagination and filters (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param user_id query string false "Filter by user ID"
// @Param media_type query string false "Filter by media type (image, video, audio)"
// @Param search query string false "Search by filename"
// @Param from_date query string false "Filter from date (YYYY-MM-DD)"
// @Param to_date query string false "Filter to date (YYYY-MM-DD)"
// @Success 200 {object} dto.MediaListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/media [get]
func (h *MediaHandler) AdminListMedia(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	userID := r.URL.Query().Get("user_id")
	mediaType := r.URL.Query().Get("media_type")
	search := r.URL.Query().Get("search")
	fromDate := r.URL.Query().Get("from_date")
	toDate := r.URL.Query().Get("to_date")

	var from, to time.Time
	if fromDate != "" {
		from, _ = time.Parse("2006-01-02", fromDate)
	}
	if toDate != "" {
		to, _ = time.Parse("2006-01-02", toDate)
	}

	mediaItems, nextCursor, total, err := h.mediaService.AdminListMedia(r.Context(), cursor, limit, userID, mediaType, search, from, to)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list media")
		return
	}

	// Build response
	responses := make([]*dto.MediaAdminResponse, 0, len(mediaItems))
	for _, m := range mediaItems {
		user, _ := h.userService.GetUserByID(r.Context(), m.UploadedBy)
		responses = append(responses, &dto.MediaAdminResponse{
			ID:          m.ID,
			Filename:    m.Filename,
			Path:        m.Path,
			ThumbnailPath: m.ThumbnailPath,
			Size:        m.Size,
			ContentType: m.ContentType,
			Width:       m.Width,
			Height:      m.Height,
			Duration:    m.Duration,
			UploadedBy:  m.UploadedBy,
			UploaderUsername: func() string {
				if user != nil {
					return user.Username
				}
				return ""
			}(),
			UploadedAt:  m.CreatedAt,
			Tags:        m.Tags,
			IsPrivate:   m.IsPrivate,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        responses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Admin Get Media Stats
// ======================================================================

// AdminGetMediaStats handles retrieving global media statistics.
// @Summary Admin get media stats
// @Description Retrieves global media statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.MediaStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/media/stats [get]
func (h *MediaHandler) AdminGetMediaStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	stats, err := h.mediaService.AdminGetMediaStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get media stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// canAccessMedia checks if the current user can access a media file.
func (h *MediaHandler) canAccessMedia(ctx context.Context, media *service.Media) bool {
	if !media.IsPrivate {
		return true
	}
	userID, _ := middleware.GetUserID(ctx)
	if userID == "" {
		return false
	}
	if media.UploadedBy == userID {
		return true
	}
	role, _ := middleware.GetUserRole(ctx)
	return role == "admin"
}

// isMediaOwner checks if the user owns the media.
func (h *MediaHandler) isMediaOwner(ctx context.Context, media *service.Media, userID string) bool {
	return media.UploadedBy == userID
}

// getMediaURL returns the URL for a media file.
func (h *MediaHandler) getMediaURL(path string, size string) string {
	if h.config.EnableCDN && h.config.CDNURL != "" {
		return fmt.Sprintf("%s/%s?size=%s", h.config.CDNURL, path, size)
	}
	return fmt.Sprintf("%s/%s?size=%s", h.config.ServePath, path, size)
}

// ======================================================================
= Helper Response Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *MediaHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *MediaHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := dto.ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
		Details: details,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.WithError(err).Error("Failed to encode error response")
	}
}

// sendValidationError handles validation errors.
func (h *MediaHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *MediaHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrMediaNotFound):
		h.sendError(w, http.StatusNotFound, "Media not found", nil)
	case errors.Is(err, service.ErrMediaAccessDenied):
		h.sendError(w, http.StatusForbidden, "Access denied", nil)
	case errors.Is(err, service.ErrMediaAlreadyDeleted):
		h.sendError(w, http.StatusBadRequest, "Media already deleted", nil)
	case errors.Is(err, service.ErrMediaInvalidType):
		h.sendError(w, http.StatusBadRequest, "Invalid media type", nil)
	case errors.Is(err, service.ErrMediaTooLarge):
		h.sendError(w, http.StatusRequestEntityTooLarge, "Media too large", nil)
	case errors.Is(err, service.ErrThumbnailGenerationFailed):
		h.sendError(w, http.StatusInternalServerError, "Failed to generate thumbnail", nil)
	case errors.Is(err, service.ErrMediaInUse):
		h.sendError(w, http.StatusConflict, "Media is in use and cannot be deleted", nil)
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrInvalidMediaID):
		h.sendError(w, http.StatusBadRequest, "Invalid media ID", nil)
	case errors.Is(err, context.Canceled):
		h.sendError(w, http.StatusRequestTimeout, "Request cancelled", nil)
	case errors.Is(err, context.DeadlineExceeded):
		h.sendError(w, http.StatusGatewayTimeout, "Request timed out", nil)
	default:
		h.log.WithError(err).Error(defaultMsg)
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck returns the health status of the media handler.
func (h *MediaHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "media_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}