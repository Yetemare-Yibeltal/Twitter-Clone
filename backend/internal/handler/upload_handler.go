// backend/internal/handler/upload_handler.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/pkg/logger"
)

// UploadHandler handles all upload-related HTTP endpoints.
type UploadHandler struct {
	storageAdapter adapter.StorageAdapter
	mediaService   service.MediaService
	userService    service.UserService
	config         *UploadConfig
	log            *logrus.Entry
}

// UploadConfig holds upload configuration.
type UploadConfig struct {
	MaxFileSize         int64    `json:"max_file_size"`
	AllowedImageTypes   []string `json:"allowed_image_types"`
	AllowedVideoTypes   []string `json:"allowed_video_types"`
	MaxImageWidth       int      `json:"max_image_width"`
	MaxImageHeight      int      `json:"max_image_height"`
	MaxVideoDuration    int      `json:"max_video_duration"`
	MaxVideoSize        int64    `json:"max_video_size"`
	UploadPath          string   `json:"upload_path"`
	AvatarPath          string   `json:"avatar_path"`
	BannerPath          string   `json:"banner_path"`
	TweetMediaPath      string   `json:"tweet_media_path"`
	CommunityAvatarPath string   `json:"community_avatar_path"`
	ThumbnailSize       int      `json:"thumbnail_size"`
	ThumbnailQuality    int      `json:"thumbnail_quality"`
	EnableCDN           bool     `json:"enable_cdn"`
	CDNURL              string   `json:"cdn_url"`
}

// DefaultUploadConfig returns default upload configuration.
func DefaultUploadConfig() *UploadConfig {
	return &UploadConfig{
		MaxFileSize:         10 * 1024 * 1024,
		AllowedImageTypes:   []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml"},
		AllowedVideoTypes:   []string{"video/mp4", "video/quicktime", "video/webm", "video/avi"},
		MaxImageWidth:       4096,
		MaxImageHeight:      4096,
		MaxVideoDuration:    120,
		MaxVideoSize:        50 * 1024 * 1024,
		UploadPath:          "uploads",
		AvatarPath:          "uploads/avatars",
		BannerPath:          "uploads/banners",
		TweetMediaPath:      "uploads/tweets",
		CommunityAvatarPath: "uploads/communities",
		ThumbnailSize:       150,
		ThumbnailQuality:    85,
		EnableCDN:           false,
	}
}

// NewUploadHandler creates a new upload handler.
func NewUploadHandler(
	storageAdapter adapter.StorageAdapter,
	mediaService service.MediaService,
	userService service.UserService,
) *UploadHandler {
	return &UploadHandler{
		storageAdapter: storageAdapter,
		mediaService:   mediaService,
		userService:    userService,
		config:         DefaultUploadConfig(),
		log:            logger.WithField("handler", "upload"),
	}
}

// ======================================================================
// Upload Avatar
// ======================================================================

// UploadAvatar handles uploading a user avatar.
// @Summary Upload avatar
// @Description Uploads a profile avatar image for the authenticated user
// @Tags uploads
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Avatar image file"
// @Success 200 {object} dto.UploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 413 {object} dto.ErrorResponse
// @Failure 415 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/upload/avatar [post]
func (h *UploadHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := r.ParseMultipartForm(h.config.MaxFileSize); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse form", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "File is required", nil)
		return
	}
	defer file.Close()

	if err := h.validateImageFile(header); err != nil {
		h.handleUploadError(w, err)
		return
	}

	filename := h.generateFilename(header.Filename, "avatar")
	uploadPath := filepath.Join(h.config.AvatarPath, userID, filename)

	result, err := h.storageAdapter.Upload(r.Context(), file, uploadPath, &adapter.UploadOptions{
		Public:      true,
		ContentType: header.Header.Get("Content-Type"),
		Metadata: map[string]string{
			"user_id":     userID,
			"upload_type": "avatar",
			"filename":    header.Filename,
		},
	})
	if err != nil {
		h.handleUploadError(w, err)
		return
	}

	// Generate thumbnail
	thumbnailURL, _ := h.generateThumbnail(r.Context(), uploadPath)

	// Update user profile with avatar URL
	avatarURL := h.getMediaURL(uploadPath, "original")
	if h.config.EnableCDN && h.config.CDNURL != "" {
		avatarURL = fmt.Sprintf("%s/%s", h.config.CDNURL, uploadPath)
	}

	// Save to media service
	media := &service.Media{
		ID:            uuid.New().String(),
		Filename:      header.Filename,
		Path:          uploadPath,
		ThumbnailPath: thumbnailURL,
		Size:          header.Size,
		ContentType:   header.Header.Get("Content-Type"),
		UploadedBy:    userID,
		IsPrivate:     false,
		CreatedAt:     time.Now(),
	}
	_ = h.mediaService.CreateMedia(r.Context(), media)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"avatar_url":    avatarURL,
		"thumbnail_url": thumbnailURL,
		"filename":      header.Filename,
		"file_size":     header.Size,
		"content_type":  header.Header.Get("Content-Type"),
		"upload_id":     result.PublicID,
		"user_id":       userID,
	})
}

// ======================================================================
// Upload Banner
// ======================================================================

// UploadBanner handles uploading a profile banner.
// @Summary Upload banner
// @Description Uploads a profile banner image for the authenticated user
// @Tags uploads
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Banner image file"
// @Success 200 {object} dto.UploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 413 {object} dto.ErrorResponse
// @Failure 415 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/upload/banner [post]
func (h *UploadHandler) UploadBanner(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := r.ParseMultipartForm(h.config.MaxFileSize); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse form", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "File is required", nil)
		return
	}
	defer file.Close()

	if err := h.validateImageFile(header); err != nil {
		h.handleUploadError(w, err)
		return
	}

	filename := h.generateFilename(header.Filename, "banner")
	uploadPath := filepath.Join(h.config.BannerPath, userID, filename)

	result, err := h.storageAdapter.Upload(r.Context(), file, uploadPath, &adapter.UploadOptions{
		Public:      true,
		ContentType: header.Header.Get("Content-Type"),
		Metadata: map[string]string{
			"user_id":     userID,
			"upload_type": "banner",
			"filename":    header.Filename,
		},
	})
	if err != nil {
		h.handleUploadError(w, err)
		return
	}

	bannerURL := h.getMediaURL(uploadPath, "original")
	if h.config.EnableCDN && h.config.CDNURL != "" {
		bannerURL = fmt.Sprintf("%s/%s", h.config.CDNURL, uploadPath)
	}

	// Save to media service
	media := &service.Media{
		ID:          uuid.New().String(),
		Filename:    header.Filename,
		Path:        uploadPath,
		Size:        header.Size,
		ContentType: header.Header.Get("Content-Type"),
		UploadedBy:  userID,
		IsPrivate:   false,
		CreatedAt:   time.Now(),
	}
	_ = h.mediaService.CreateMedia(r.Context(), media)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"banner_url":   bannerURL,
		"filename":     header.Filename,
		"file_size":    header.Size,
		"content_type": header.Header.Get("Content-Type"),
		"upload_id":    result.PublicID,
		"user_id":      userID,
	})
}

// ======================================================================
// Upload Tweet Media
// ======================================================================

// UploadTweetMedia handles uploading media for a tweet.
// @Summary Upload tweet media
// @Description Uploads media (image/video) for a tweet
// @Tags uploads
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Media file"
// @Param type query string false "Media type (image, video) default image"
// @Param tweet_id query string false "Tweet ID (optional)"
// @Success 200 {object} dto.UploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 413 {object} dto.ErrorResponse
// @Failure 415 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/upload/tweet [post]
func (h *UploadHandler) UploadTweetMedia(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	mediaType := r.URL.Query().Get("type")
	if mediaType == "" {
		mediaType = "image"
	}
	tweetID := r.URL.Query().Get("tweet_id")

	maxSize := h.config.MaxFileSize
	if mediaType == "video" {
		maxSize = h.config.MaxVideoSize
	}

	if err := r.ParseMultipartForm(maxSize); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse form", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "File is required", nil)
		return
	}
	defer file.Close()

	if mediaType == "video" {
		if err := h.validateVideoFile(header); err != nil {
			h.handleUploadError(w, err)
			return
		}
	} else {
		if err := h.validateImageFile(header); err != nil {
			h.handleUploadError(w, err)
			return
		}
	}

	filename := h.generateFilename(header.Filename, mediaType)
	uploadPath := filepath.Join(h.config.TweetMediaPath, userID, filename)

	result, err := h.storageAdapter.Upload(r.Context(), file, uploadPath, &adapter.UploadOptions{
		Public:      true,
		ContentType: header.Header.Get("Content-Type"),
		Metadata: map[string]string{
			"user_id":     userID,
			"upload_type": "tweet_media",
			"media_type":  mediaType,
			"filename":    header.Filename,
			"tweet_id":    tweetID,
		},
	})
	if err != nil {
		h.handleUploadError(w, err)
		return
	}

	mediaURL := h.getMediaURL(uploadPath, "original")
	if h.config.EnableCDN && h.config.CDNURL != "" {
		mediaURL = fmt.Sprintf("%s/%s", h.config.CDNURL, uploadPath)
	}

	// Generate thumbnail for images
	var thumbnailURL string
	if mediaType == "image" {
		thumbnailURL, _ = h.generateThumbnail(r.Context(), uploadPath)
	}

	// Save to media service
	media := &service.Media{
		ID:            uuid.New().String(),
		Filename:      header.Filename,
		Path:          uploadPath,
		ThumbnailPath: thumbnailURL,
		Size:          header.Size,
		ContentType:   header.Header.Get("Content-Type"),
		UploadedBy:    userID,
		IsPrivate:     false,
		CreatedAt:     time.Now(),
		Tags:          []string{mediaType},
	}
	_ = h.mediaService.CreateMedia(r.Context(), media)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"media_url":     mediaURL,
		"thumbnail_url": thumbnailURL,
		"media_type":    mediaType,
		"filename":      header.Filename,
		"file_size":     header.Size,
		"content_type":  header.Header.Get("Content-Type"),
		"upload_id":     result.PublicID,
		"user_id":       userID,
		"tweet_id":      tweetID,
	})
}

// ======================================================================
= Multi-file Upload
// ======================================================================

// UploadMultiple handles uploading multiple files at once.
// @Summary Upload multiple files
// @Description Uploads multiple files (max 5)
// @Tags uploads
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param files formData file true "Multiple files"
// @Param type query string false "Upload type (avatar, banner, tweet)"
// @Success 200 {object} dto.MultiUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 413 {object} dto.ErrorResponse
// @Failure 415 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/upload/multiple [post]
func (h *UploadHandler) UploadMultiple(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	uploadType := r.URL.Query().Get("type")
	if uploadType == "" {
		uploadType = "tweet"
	}

	if err := r.ParseMultipartForm(h.config.MaxVideoSize * 5); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse form", nil)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		h.sendError(w, http.StatusBadRequest, "No files uploaded", nil)
		return
	}
	if len(files) > 5 {
		h.sendError(w, http.StatusBadRequest, "Maximum 5 files allowed", nil)
		return
	}

	results := make([]map[string]interface{}, 0, len(files))
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		if err := h.validateImageFile(fileHeader); err != nil {
			continue
		}

		filename := h.generateFilename(fileHeader.Filename, uploadType)
		uploadPath := filepath.Join(h.config.TweetMediaPath, userID, filename)

		result, err := h.storageAdapter.Upload(r.Context(), file, uploadPath, &adapter.UploadOptions{
			Public:      true,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Metadata: map[string]string{
				"user_id":     userID,
				"upload_type": uploadType,
				"filename":    fileHeader.Filename,
			},
		})
		if err != nil {
			continue
		}

		mediaURL := h.getMediaURL(uploadPath, "original")
		if h.config.EnableCDN && h.config.CDNURL != "" {
			mediaURL = fmt.Sprintf("%s/%s", h.config.CDNURL, uploadPath)
		}

		results = append(results, map[string]interface{}{
			"media_url":    mediaURL,
			"filename":     fileHeader.Filename,
			"file_size":    fileHeader.Size,
			"content_type": fileHeader.Header.Get("Content-Type"),
			"upload_id":    result.PublicID,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"files":       results,
		"total":       len(results),
		"upload_type": uploadType,
		"user_id":     userID,
	})
}

// ======================================================================
= Get Upload URL (Pre-signed)
// ======================================================================

// GetUploadURL handles generating a pre-signed upload URL.
// @Summary Get upload URL
// @Description Generates a pre-signed URL for direct upload
// @Tags uploads
// @Security BearerAuth
// @Produce json
// @Param filename query string true "Filename"
// @Param type query string true "Upload type (avatar, banner, tweet)"
// @Param expiry query int false "URL expiry in minutes (default 15)"
// @Success 200 {object} dto.UploadURLResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/upload/url [get]
func (h *UploadHandler) GetUploadURL(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		h.sendError(w, http.StatusBadRequest, "Filename is required", nil)
		return
	}
	uploadType := r.URL.Query().Get("type")
	if uploadType == "" {
		uploadType = "tweet"
	}
	expiryMinutes, err := strconv.Atoi(r.URL.Query().Get("expiry"))
	if err != nil || expiryMinutes < 1 || expiryMinutes > 60 {
		expiryMinutes = 15
	}

	filename = h.generateFilename(filename, uploadType)
	var uploadPath string
	switch uploadType {
	case "avatar":
		uploadPath = filepath.Join(h.config.AvatarPath, userID, filename)
	case "banner":
		uploadPath = filepath.Join(h.config.BannerPath, userID, filename)
	default:
		uploadPath = filepath.Join(h.config.TweetMediaPath, userID, filename)
	}

	url, err := h.storageAdapter.GetURL(r.Context(), uploadPath, time.Duration(expiryMinutes)*time.Minute)
	if err != nil {
		h.handleUploadError(w, err)
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"upload_url":   url,
		"upload_path":  uploadPath,
		"filename":     filename,
		"expires_in":   expiryMinutes,
		"method":       "PUT",
		"content_type": "application/octet-stream",
	})
}

// ======================================================================
= Delete Upload
// ======================================================================

// DeleteUpload handles deleting an uploaded file.
// @Summary Delete upload
// @Description Deletes an uploaded file (owner or admin)
// @Tags uploads
// @Security BearerAuth
// @Param id path string true "Media ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/upload/{id} [delete]
func (h *UploadHandler) DeleteUpload(w http.ResponseWriter, r *http.Request) {
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

	media, err := h.mediaService.GetMediaByID(r.Context(), mediaID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get media")
		return
	}

	// Check ownership
	if media.UploadedBy != userID {
		role, _ := middleware.GetUserRole(r.Context())
		if role != "admin" {
			h.sendError(w, http.StatusForbidden, "Access denied", nil)
			return
		}
	}

	// Delete from storage
	if err := h.storageAdapter.Delete(r.Context(), media.Path); err != nil {
		h.handleUploadError(w, err)
		return
	}
	if media.ThumbnailPath != "" {
		_ = h.storageAdapter.Delete(r.Context(), media.ThumbnailPath)
	}

	// Delete from database
	if err := h.mediaService.DeleteMedia(r.Context(), mediaID); err != nil {
		h.handleUploadError(w, err)
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "File deleted successfully",
	})
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListUploads handles admin listing of all uploads.
// @Summary Admin list uploads
// @Description Lists all uploaded files for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param user_id query string false "Filter by user ID"
// @Param type query string false "Filter by upload type"
// @Success 200 {object} dto.UploadListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/uploads [get]
func (h *UploadHandler) AdminListUploads(w http.ResponseWriter, r *http.Request) {
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
	mediaType := r.URL.Query().Get("type")

	mediaItems, nextCursor, total, err := h.mediaService.AdminListMedia(r.Context(), cursor, limit, userID, mediaType, "", time.Time{}, time.Time{})
	if err != nil {
		h.handleServiceError(w, err, "Failed to list uploads")
		return
	}

	responses := make([]*dto.UploadAdminResponse, 0, len(mediaItems))
	for _, m := range mediaItems {
		user, _ := h.userService.GetUserByID(r.Context(), m.UploadedBy)
		responses = append(responses, &dto.UploadAdminResponse{
			ID:          m.ID,
			Filename:    m.Filename,
			Path:        m.Path,
			Size:        m.Size,
			ContentType: m.ContentType,
			UploadedBy:  m.UploadedBy,
			UploaderUsername: func() string {
				if user != nil {
					return user.Username
				}
				return ""
			}(),
			UploadedAt: m.CreatedAt,
			Tags:       m.Tags,
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

// AdminDeleteUpload handles admin deletion of an upload.
// @Summary Admin delete upload
// @Description Deletes an uploaded file (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Media ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/uploads/{id} [delete]
func (h *UploadHandler) AdminDeleteUpload(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	mediaID := vars["id"]
	if mediaID == "" {
		h.sendError(w, http.StatusBadRequest, "Media ID required", nil)
		return
	}

	if err := h.mediaService.AdminDeleteMedia(r.Context(), mediaID); err != nil {
		h.handleServiceError(w, err, "Failed to delete upload")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Upload deleted successfully",
	})
}

// AdminGetUploadStats handles retrieving global upload statistics.
// @Summary Admin get upload stats
// @Description Retrieves global upload statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.UploadStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/uploads/stats [get]
func (h *UploadHandler) AdminGetUploadStats(w http.ResponseWriter, r *http.Request) {
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
		h.handleServiceError(w, err, "Failed to get upload stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// validateImageFile validates an image file.
func (h *UploadHandler) validateImageFile(header *multipart.FileHeader) error {
	contentType := header.Header.Get("Content-Type")
	allowed := false
	for _, t := range h.config.AllowedImageTypes {
		if contentType == t {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrInvalidImageType
	}
	if header.Size > h.config.MaxFileSize {
		return ErrFileTooLarge
	}
	return nil
}

// validateVideoFile validates a video file.
func (h *UploadHandler) validateVideoFile(header *multipart.FileHeader) error {
	contentType := header.Header.Get("Content-Type")
	allowed := false
	for _, t := range h.config.AllowedVideoTypes {
		if contentType == t {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrInvalidVideoType
	}
	if header.Size > h.config.MaxVideoSize {
		return ErrVideoTooLarge
	}
	return nil
}

// generateFilename generates a unique filename.
func (h *UploadHandler) generateFilename(originalName, prefix string) string {
	ext := filepath.Ext(originalName)
	timestamp := time.Now().UnixNano()
	uuidStr := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%d_%s%s", prefix, timestamp, uuidStr, ext)
}

// generateThumbnail generates a thumbnail for an image.
func (h *UploadHandler) generateThumbnail(ctx context.Context, uploadPath string) (string, error) {
	thumbnailPath := strings.Replace(uploadPath, ".", "_thumb.", -1)
	if err := h.storageAdapter.GenerateThumbnail(ctx, uploadPath, thumbnailPath, h.config.ThumbnailSize, h.config.ThumbnailQuality); err != nil {
		return "", err
	}
	return h.getMediaURL(thumbnailPath, "thumbnail"), nil
}

// getMediaURL returns the URL for a media file.
func (h *UploadHandler) getMediaURL(path, size string) string {
	if h.config.EnableCDN && h.config.CDNURL != "" {
		return fmt.Sprintf("%s/%s?size=%s", h.config.CDNURL, path, size)
	}
	return fmt.Sprintf("/media/%s?size=%s", path, size)
}

// handleServiceError maps service errors to HTTP responses.
func (h *UploadHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrMediaNotFound):
		h.sendError(w, http.StatusNotFound, "Media not found", nil)
	case errors.Is(err, service.ErrMediaAccessDenied):
		h.sendError(w, http.StatusForbidden, "Access denied", nil)
	case errors.Is(err, service.ErrMediaInUse):
		h.sendError(w, http.StatusConflict, "Media is in use and cannot be deleted", nil)
	default:
		h.log.WithError(err).Error(defaultMsg)
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}

// handleUploadError maps upload errors to HTTP responses.
func (h *UploadHandler) handleUploadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrFileTooLarge):
		h.sendError(w, http.StatusRequestEntityTooLarge, "File too large", nil)
	case errors.Is(err, ErrVideoTooLarge):
		h.sendError(w, http.StatusRequestEntityTooLarge, "Video too large", nil)
	case errors.Is(err, ErrInvalidImageType):
		h.sendError(w, http.StatusUnsupportedMediaType, "Invalid image type", nil)
	case errors.Is(err, ErrInvalidVideoType):
		h.sendError(w, http.StatusUnsupportedMediaType, "Invalid video type", nil)
	case errors.Is(err, ErrFileNotFound):
		h.sendError(w, http.StatusNotFound, "File not found", nil)
	case errors.Is(err, ErrStorageError):
		h.sendError(w, http.StatusInternalServerError, "Storage error", nil)
	case errors.Is(err, ErrUploadFailed):
		h.sendError(w, http.StatusInternalServerError, "Upload failed", nil)
	default:
		h.log.WithError(err).Error("Upload error")
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}

// ======================================================================
= Errors
// ======================================================================

var (
	ErrFileTooLarge     = errors.New("file size exceeds maximum allowed")
	ErrVideoTooLarge    = errors.New("video size exceeds maximum allowed")
	ErrInvalidImageType = errors.New("invalid image type")
	ErrInvalidVideoType = errors.New("invalid video type")
	ErrUploadFailed     = errors.New("upload failed")
	ErrFileNotFound     = errors.New("file not found")
	ErrStorageError     = errors.New("storage error")
)

// ======================================================================
= Helper Response Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *UploadHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *UploadHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *UploadHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck returns the health status of the upload handler.
func (h *UploadHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "upload_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}