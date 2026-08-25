// backend/internal/domain/valueobjects/media.go
package valueobjects

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ======================================================================
// Constants
// ======================================================================

const (
	MaxMediaCount          = 4
	MaxMediaURLSize        = 2048
	MaxImageFileSize       = 10 * 1024 * 1024  // 10MB
	MaxVideoFileSize       = 50 * 1024 * 1024  // 50MB
	MaxGifFileSize         = 15 * 1024 * 1024  // 15MB
	MaxAudioFileSize       = 20 * 1024 * 1024  // 20MB
	MaxMediaWidth          = 4096
	MaxMediaHeight         = 4096
	MinMediaWidth          = 1
	MinMediaHeight         = 1
)

// MediaType represents the type of media.
type MediaType string

const (
	MediaTypeImage    MediaType = "image"
	MediaTypeVideo    MediaType = "video"
	MediaTypeGif      MediaType = "gif"
	MediaTypeAudio    MediaType = "audio"
	MediaTypeDocument MediaType = "document"
	MediaTypeUnknown  MediaType = "unknown"
)

// ValidMediaTypes returns all valid media types.
func ValidMediaTypes() []MediaType {
	return []MediaType{
		MediaTypeImage,
		MediaTypeVideo,
		MediaTypeGif,
		MediaTypeAudio,
		MediaTypeDocument,
	}
}

// IsValid checks if a media type is valid.
func (m MediaType) IsValid() bool {
	for _, typ := range ValidMediaTypes() {
		if m == typ {
			return true
		}
	}
	return false
}

// String returns the string representation of the media type.
func (m MediaType) String() string {
	return string(m)
}

// ImageFormat represents image formats.
type ImageFormat string

const (
	FormatJPEG ImageFormat = "jpeg"
	FormatPNG  ImageFormat = "png"
	FormatGIF  ImageFormat = "gif"
	FormatWebP ImageFormat = "webp"
	FormatSVG  ImageFormat = "svg"
	FormatBMP  ImageFormat = "bmp"
	FormatTIFF ImageFormat = "tiff"
	FormatHEIC ImageFormat = "heic"
)

// ValidImageFormats returns all valid image formats.
func ValidImageFormats() []ImageFormat {
	return []ImageFormat{
		FormatJPEG,
		FormatPNG,
		FormatGIF,
		FormatWebP,
		FormatSVG,
		FormatBMP,
		FormatTIFF,
		FormatHEIC,
	}
}

// IsValid checks if an image format is valid.
func (f ImageFormat) IsValid() bool {
	for _, format := range ValidImageFormats() {
		if f == format {
			return true
		}
	}
	return false
}

// String returns the string representation of the image format.
func (f ImageFormat) String() string {
	return string(f)
}

// VideoFormat represents video formats.
type VideoFormat string

const (
	FormatMP4  VideoFormat = "mp4"
	FormatWebM VideoFormat = "webm"
	FormatAVI  VideoFormat = "avi"
	FormatMOV  VideoFormat = "mov"
	FormatWMV  VideoFormat = "wmv"
	FormatFLV  VideoFormat = "flv"
	FormatMKV  VideoFormat = "mkv"
)

// ValidVideoFormats returns all valid video formats.
func ValidVideoFormats() []VideoFormat {
	return []VideoFormat{
		FormatMP4,
		FormatWebM,
		FormatAVI,
		FormatMOV,
		FormatWMV,
		FormatFLV,
		FormatMKV,
	}
}

// IsValid checks if a video format is valid.
func (f VideoFormat) IsValid() bool {
	for _, format := range ValidVideoFormats() {
		if f == format {
			return true
		}
	}
	return false
}

// ======================================================================
// Errors
// ======================================================================

var (
	ErrMediaURLEmpty           = errors.New("media URL cannot be empty")
	ErrMediaURLInvalid         = errors.New("invalid media URL")
	ErrMediaURLTooLong         = fmt.Errorf("media URL exceeds maximum length of %d characters", MaxMediaURLSize)
	ErrMediaURLUnsupported     = errors.New("unsupported media URL")
	ErrMediaTypeUnknown        = errors.New("unknown media type")
	ErrMediaTypeNotSupported   = errors.New("media type not supported")
	ErrMediaFileTooLarge       = errors.New("media file exceeds maximum size")
	ErrMediaCountExceeded      = fmt.Errorf("maximum %d media files allowed", MaxMediaCount)
	ErrMediaDimensionInvalid   = errors.New("invalid media dimensions")
	ErrMediaWidthExceeded      = fmt.Errorf("media width exceeds maximum of %d pixels", MaxMediaWidth)
	ErrMediaHeightExceeded     = fmt.Errorf("media height exceeds maximum of %d pixels", MaxMediaHeight)
	ErrMediaWidthTooSmall      = fmt.Errorf("media width is less than minimum of %d pixel", MinMediaWidth)
	ErrMediaHeightTooSmall     = fmt.Errorf("media height is less than minimum of %d pixel", MinMediaHeight)
	ErrMediaFormatUnsupported  = errors.New("media format not supported")
	ErrMediaDurationInvalid    = errors.New("invalid media duration")
	ErrMediaDurationTooLong    = errors.New("media duration exceeds maximum")
	ErrMediaMetadataInvalid    = errors.New("invalid media metadata")
	ErrMediaThumbnailFailed    = errors.New("thumbnail generation failed")
	ErrMediaDuplicatesDetected = errors.New("duplicate media URLs detected")
	ErrMediaEmptyList          = errors.New("media list cannot be empty")
)

// ======================================================================
// Media Entity
// ======================================================================

// Media represents a media attachment value object.
type Media struct {
	URL         string            `json:"url"`
	Type        MediaType         `json:"type"`
	Format      string            `json:"format,omitempty"`
	Width       int               `json:"width,omitempty"`
	Height      int               `json:"height,omitempty"`
	Duration    int               `json:"duration,omitempty"` // seconds
	FileSize    int64             `json:"file_size,omitempty"`
	Filename    string            `json:"filename,omitempty"`
	ThumbnailURL string           `json:"thumbnail_url,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ======================================================================
// Factory Methods
// ======================================================================

// NewMedia creates a new media value object with validation.
func NewMedia(urlStr string, fileSize int64) (*Media, error) {
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return nil, ErrMediaURLEmpty
	}
	if len(urlStr) > MaxMediaURLSize {
		return nil, ErrMediaURLTooLong
	}
	if !isValidMediaURL(urlStr) {
		return nil, ErrMediaURLInvalid
	}
	mediaType := detectMediaType(urlStr)
	if mediaType == MediaTypeUnknown {
		return nil, ErrMediaTypeUnknown
	}
	if !mediaType.IsValid() {
		return nil, ErrMediaTypeNotSupported
	}
	if fileSize > 0 {
		if err := validateFileSize(mediaType, fileSize); err != nil {
			return nil, err
		}
	}
	filename := extractFilename(urlStr)
	format := detectFormat(urlStr, mediaType)
	// Validate format
	if mediaType == MediaTypeImage {
		imgFormat := ImageFormat(format)
		if !imgFormat.IsValid() {
			return nil, ErrMediaFormatUnsupported
		}
	}
	if mediaType == MediaTypeVideo {
		vidFormat := VideoFormat(format)
		if !vidFormat.IsValid() {
			return nil, ErrMediaFormatUnsupported
		}
	}
	return &Media{
		URL:      urlStr,
		Type:     mediaType,
		Format:   format,
		FileSize: fileSize,
		Filename: filename,
		Metadata: make(map[string]string),
	}, nil
}

// NewMediaWithDimensions creates a media with dimensions.
func NewMediaWithDimensions(urlStr string, fileSize int64, width, height int) (*Media, error) {
	media, err := NewMedia(urlStr, fileSize)
	if err != nil {
		return nil, err
	}
	if err := validateDimensions(width, height); err != nil {
		return nil, err
	}
	media.Width = width
	media.Height = height
	return media, nil
}

// NewMediaWithThumbnail creates a media with thumbnail.
func NewMediaWithThumbnail(urlStr string, fileSize int64, thumbnailURL string) (*Media, error) {
	media, err := NewMedia(urlStr, fileSize)
	if err != nil {
		return nil, err
	}
	if thumbnailURL != "" && !isValidMediaURL(thumbnailURL) {
		return nil, ErrMediaURLInvalid
	}
	media.ThumbnailURL = thumbnailURL
	return media, nil
}

// NewMediaWithDuration creates a media with duration (for videos/audio).
func NewMediaWithDuration(urlStr string, fileSize int64, duration int) (*Media, error) {
	media, err := NewMedia(urlStr, fileSize)
	if err != nil {
		return nil, err
	}
	if duration < 0 {
		return nil, ErrMediaDurationInvalid
	}
	if duration > 3600 { // Max 1 hour
		return nil, ErrMediaDurationTooLong
	}
	media.Duration = duration
	return media, nil
}

// MustNewMedia creates a media and panics on error.
func MustNewMedia(urlStr string, fileSize int64) *Media {
	media, err := NewMedia(urlStr, fileSize)
	if err != nil {
		panic(err)
	}
	return media
}

// ======================================================================
// Validation
// ======================================================================

// validateFileSize checks if the file size is within limits.
func validateFileSize(mediaType MediaType, fileSize int64) error {
	switch mediaType {
	case MediaTypeImage:
		if fileSize > MaxImageFileSize {
			return fmt.Errorf("image size %d bytes exceeds maximum of %d bytes", fileSize, MaxImageFileSize)
		}
	case MediaTypeVideo:
		if fileSize > MaxVideoFileSize {
			return fmt.Errorf("video size %d bytes exceeds maximum of %d bytes", fileSize, MaxVideoFileSize)
		}
	case MediaTypeGif:
		if fileSize > MaxGifFileSize {
			return fmt.Errorf("GIF size %d bytes exceeds maximum of %d bytes", fileSize, MaxGifFileSize)
		}
	case MediaTypeAudio:
		if fileSize > MaxAudioFileSize {
			return fmt.Errorf("audio size %d bytes exceeds maximum of %d bytes", fileSize, MaxAudioFileSize)
		}
	}
	return nil
}

// validateDimensions checks if dimensions are valid.
func validateDimensions(width, height int) error {
	if width < MinMediaWidth || height < MinMediaHeight {
		return ErrMediaDimensionInvalid
	}
	if width > MaxMediaWidth {
		return ErrMediaWidthExceeded
	}
	if height > MaxMediaHeight {
		return ErrMediaHeightExceeded
	}
	return nil
}

// Validate validates the media.
func (m *Media) Validate() error {
	if m.URL == "" {
		return ErrMediaURLEmpty
	}
	if len(m.URL) > MaxMediaURLSize {
		return ErrMediaURLTooLong
	}
	if !isValidMediaURL(m.URL) {
		return ErrMediaURLInvalid
	}
	if !m.Type.IsValid() {
		return ErrMediaTypeNotSupported
	}
	if m.FileSize > 0 {
		if err := validateFileSize(m.Type, m.FileSize); err != nil {
			return err
		}
	}
	if m.Width > 0 || m.Height > 0 {
		if err := validateDimensions(m.Width, m.Height); err != nil {
			return err
		}
	}
	if m.Duration > 0 {
		if m.Duration < 0 {
			return ErrMediaDurationInvalid
		}
		if m.Duration > 3600 {
			return ErrMediaDurationTooLong
		}
	}
	if m.ThumbnailURL != "" && !isValidMediaURL(m.ThumbnailURL) {
		return ErrMediaURLInvalid
	}
	return nil
}

// ======================================================================
// Getters
// ======================================================================

// GetURL returns the media URL.
func (m *Media) GetURL() string {
	return m.URL
}

// GetType returns the media type.
func (m *Media) GetType() MediaType {
	return m.Type
}

// GetFormat returns the media format.
func (m *Media) GetFormat() string {
	return m.Format
}

// GetWidth returns the media width.
func (m *Media) GetWidth() int {
	return m.Width
}

// GetHeight returns the media height.
func (m *Media) GetHeight() int {
	return m.Height
}

// GetDuration returns the media duration.
func (m *Media) GetDuration() int {
	return m.Duration
}

// GetFileSize returns the file size.
func (m *Media) GetFileSize() int64 {
	return m.FileSize
}

// GetFilename returns the filename.
func (m *Media) GetFilename() string {
	return m.Filename
}

// GetThumbnailURL returns the thumbnail URL.
func (m *Media) GetThumbnailURL() string {
	return m.ThumbnailURL
}

// GetMetadata returns the media metadata.
func (m *Media) GetMetadata() map[string]string {
	return m.Metadata
}

// ======================================================================
// Utility Methods
// ======================================================================

// IsImage returns true if the media is an image.
func (m *Media) IsImage() bool {
	return m.Type == MediaTypeImage
}

// IsVideo returns true if the media is a video.
func (m *Media) IsVideo() bool {
	return m.Type == MediaTypeVideo
}

// IsGif returns true if the media is a GIF.
func (m *Media) IsGif() bool {
	return m.Type == MediaTypeGif
}

// IsAudio returns true if the media is audio.
func (m *Media) IsAudio() bool {
	return m.Type == MediaTypeAudio
}

// IsDocument returns true if the media is a document.
func (m *Media) IsDocument() bool {
	return m.Type == MediaTypeDocument
}

// HasThumbnail returns true if the media has a thumbnail.
func (m *Media) HasThumbnail() bool {
	return m.ThumbnailURL != ""
}

// HasDimensions returns true if dimensions are set.
func (m *Media) HasDimensions() bool {
	return m.Width > 0 && m.Height > 0
}

// HasDuration returns true if duration is set.
func (m *Media) HasDuration() bool {
	return m.Duration > 0
}

// SetMetadata sets a metadata key-value pair.
func (m *Media) SetMetadata(key, value string) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
}

// GetMetadataValue returns a metadata value by key.
func (m *Media) GetMetadataValue(key string) (string, bool) {
	val, ok := m.Metadata[key]
	return val, ok
}

// String returns a string representation.
func (m *Media) String() string {
	return fmt.Sprintf("Media{url:%s, type:%s, format:%s, size:%d}", m.URL, m.Type, m.Format, m.FileSize)
}

// Clone creates a deep copy of the media.
func (m *Media) Clone() *Media {
	clone := &Media{
		URL:          m.URL,
		Type:         m.Type,
		Format:       m.Format,
		Width:        m.Width,
		Height:       m.Height,
		Duration:     m.Duration,
		FileSize:     m.FileSize,
		Filename:     m.Filename,
		ThumbnailURL: m.ThumbnailURL,
		Metadata:     make(map[string]string),
	}
	for k, v := range m.Metadata {
		clone.Metadata[k] = v
	}
	return clone
}

// ======================================================================
= Helper Functions
// ======================================================================

// isValidMediaURL checks if a URL is valid for media.
func isValidMediaURL(urlStr string) bool {
	parsed, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return true
}

// detectMediaType detects the media type from URL.
func detectMediaType(urlStr string) MediaType {
	lower := strings.ToLower(urlStr)
	// Check extensions
	ext := filepath.Ext(lower)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".svg", ".bmp", ".tiff", ".heic":
		return MediaTypeImage
	case ".mp4", ".webm", ".avi", ".mov", ".wmv", ".flv", ".mkv":
		return MediaTypeVideo
	case ".gif":
		return MediaTypeGif
	case ".mp3", ".wav", ".aac", ".ogg", ".flac", ".m4a":
		return MediaTypeAudio
	case ".pdf", ".doc", ".docx", ".txt", ".rtf":
		return MediaTypeDocument
	}
	// Check content-type in URL (if present as parameter)
	if strings.Contains(lower, "image/") {
		return MediaTypeImage
	}
	if strings.Contains(lower, "video/") {
		return MediaTypeVideo
	}
	if strings.Contains(lower, "audio/") {
		return MediaTypeAudio
	}
	return MediaTypeUnknown
}

// detectFormat detects the media format from URL.
func detectFormat(urlStr string, mediaType MediaType) string {
	lower := strings.ToLower(urlStr)
	ext := filepath.Ext(lower)
	if ext == "" {
		return ""
	}
	ext = strings.TrimPrefix(ext, ".")
	switch mediaType {
	case MediaTypeImage:
		switch ext {
		case "jpg", "jpeg":
			return "jpeg"
		case "png":
			return "png"
		case "gif":
			return "gif"
		case "webp":
			return "webp"
		case "svg":
			return "svg"
		case "bmp":
			return "bmp"
		case "tiff":
			return "tiff"
		case "heic":
			return "heic"
		}
	case MediaTypeVideo:
		switch ext {
		case "mp4":
			return "mp4"
		case "webm":
			return "webm"
		case "avi":
			return "avi"
		case "mov":
			return "mov"
		case "wmv":
			return "wmv"
		case "flv":
			return "flv"
		case "mkv":
			return "mkv"
		}
	case MediaTypeAudio:
		switch ext {
		case "mp3":
			return "mp3"
		case "wav":
			return "wav"
		case "aac":
			return "aac"
		case "ogg":
			return "ogg"
		case "flac":
			return "flac"
		case "m4a":
			return "m4a"
		}
	}
	return ext
}

// extractFilename extracts filename from URL.
func extractFilename(urlStr string) string {
	path := urlStr
	if strings.Contains(path, "?") {
		path = strings.Split(path, "?")[0]
	}
	return filepath.Base(path)
}

// ======================================================================
= Media Collection
// ======================================================================

// MediaCollection represents a collection of media items.
type MediaCollection struct {
	items []*Media
}

// NewMediaCollection creates a new media collection.
func NewMediaCollection() *MediaCollection {
	return &MediaCollection{
		items: []*Media{},
	}
}

// Add adds a media item to the collection.
func (c *MediaCollection) Add(media *Media) error {
	if len(c.items) >= MaxMediaCount {
		return ErrMediaCountExceeded
	}
	if media == nil {
		return errors.New("media cannot be nil")
	}
	// Check for duplicates
	for _, existing := range c.items {
		if existing.URL == media.URL {
			return ErrMediaDuplicatesDetected
		}
	}
	c.items = append(c.items, media)
	return nil
}

// AddMultiple adds multiple media items to the collection.
func (c *MediaCollection) AddMultiple(medias []*Media) error {
	if len(c.items)+len(medias) > MaxMediaCount {
		return ErrMediaCountExceeded
	}
	for _, m := range medias {
		if err := c.Add(m); err != nil {
			return err
		}
	}
	return nil
}

// Remove removes a media item by index.
func (c *MediaCollection) Remove(index int) error {
	if index < 0 || index >= len(c.items) {
		return errors.New("index out of range")
	}
	c.items = append(c.items[:index], c.items[index+1:]...)
	return nil
}

// Get returns a media item by index.
func (c *MediaCollection) Get(index int) (*Media, error) {
	if index < 0 || index >= len(c.items) {
		return nil, errors.New("index out of range")
	}
	return c.items[index], nil
}

// Count returns the number of media items.
func (c *MediaCollection) Count() int {
	return len(c.items)
}

// IsEmpty returns true if the collection is empty.
func (c *MediaCollection) IsEmpty() bool {
	return len(c.items) == 0
}

// IsFull returns true if the collection is full.
func (c *MediaCollection) IsFull() bool {
	return len(c.items) >= MaxMediaCount
}

// RemainingCount returns the number of media items that can still be added.
func (c *MediaCollection) RemainingCount() int {
	return MaxMediaCount - len(c.items)
}

// Items returns all media items.
func (c *MediaCollection) Items() []*Media {
	return c.items
}

// Images returns only image media items.
func (c *MediaCollection) Images() []*Media {
	result := []*Media{}
	for _, m := range c.items {
		if m.IsImage() {
			result = append(result, m)
		}
	}
	return result
}

// Videos returns only video media items.
func (c *MediaCollection) Videos() []*Media {
	result := []*Media{}
	for _, m := range c.items {
		if m.IsVideo() {
			result = append(result, m)
		}
	}
	return result
}

// Gifs returns only GIF media items.
func (c *MediaCollection) Gifs() []*Media {
	result := []*Media{}
	for _, m := range c.items {
		if m.IsGif() {
			result = append(result, m)
		}
	}
	return result
}

// Audio returns only audio media items.
func (c *MediaCollection) Audio() []*Media {
	result := []*Media{}
	for _, m := range c.items {
		if m.IsAudio() {
			result = append(result, m)
		}
	}
	return result
}

// HasImages returns true if the collection has images.
func (c *MediaCollection) HasImages() bool {
	return len(c.Images()) > 0
}

// HasVideos returns true if the collection has videos.
func (c *MediaCollection) HasVideos() bool {
	return len(c.Videos()) > 0
}

// HasGifs returns true if the collection has GIFs.
func (c *MediaCollection) HasGifs() bool {
	return len(c.Gifs()) > 0
}

// HasAudio returns true if the collection has audio.
func (c *MediaCollection) HasAudio() bool {
	return len(c.Audio()) > 0
}

// URLs returns all media URLs.
func (c *MediaCollection) URLs() []string {
	urls := make([]string, len(c.items))
	for i, m := range c.items {
		urls[i] = m.URL
	}
	return urls
}

// Valid validates all media items in the collection.
func (c *MediaCollection) Valid() error {
	if c.IsEmpty() {
		return ErrMediaEmptyList
	}
	for _, m := range c.items {
		if err := m.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ======================================================================
= MediaStatistics
// ======================================================================

// MediaStats represents media statistics.
type MediaStats struct {
	TotalCount    int                `json:"total_count"`
	ImageCount    int                `json:"image_count"`
	VideoCount    int                `json:"video_count"`
	GifCount      int                `json:"gif_count"`
	AudioCount    int                `json:"audio_count"`
	TotalSize     int64              `json:"total_size"`
	MaxSize       int64              `json:"max_size"`
	MinSize       int64              `json:"min_size"`
	AvgSize       int64              `json:"avg_size"`
	Formats       map[string]int     `json:"formats"`
	HasThumbnails int                `json:"has_thumbnails"`
	Types         map[string]int     `json:"types"`
}

// GetStats returns statistics for the media collection.
func (c *MediaCollection) GetStats() *MediaStats {
	if c.IsEmpty() {
		return &MediaStats{
			Formats: make(map[string]int),
			Types:   make(map[string]int),
		}
	}
	stats := &MediaStats{
		TotalCount: c.Count(),
		Formats:    make(map[string]int),
		Types:      make(map[string]int),
	}
	var totalSize int64
	var maxSize int64 = -1
	var minSize int64 = -1
	for _, m := range c.items {
		// Count by type
		switch m.Type {
		case MediaTypeImage:
			stats.ImageCount++
		case MediaTypeVideo:
			stats.VideoCount++
		case MediaTypeGif:
			stats.GifCount++
		case MediaTypeAudio:
			stats.AudioCount++
		}
		stats.Types[string(m.Type)]++
		// Count by format
		if m.Format != "" {
			stats.Formats[m.Format]++
		}
		// Size stats
		if m.FileSize > 0 {
			totalSize += m.FileSize
			if m.FileSize > maxSize {
				maxSize = m.FileSize
			}
			if minSize == -1 || m.FileSize < minSize {
				minSize = m.FileSize
			}
		}
		if m.HasThumbnail() {
			stats.HasThumbnails++
		}
	}
	stats.TotalSize = totalSize
	stats.MaxSize = maxSize
	stats.MinSize = minSize
	if c.Count() > 0 {
		stats.AvgSize = totalSize / int64(c.Count())
	}
	return stats
}

// ======================================================================
= Builder Pattern
// ======================================================================

// MediaBuilder helps construct media for testing.
type MediaBuilder struct {
	media *Media
}

// NewMediaBuilder creates a new media builder.
func NewMediaBuilder() *MediaBuilder {
	return &MediaBuilder{
		media: &Media{
			URL:      "",
			Type:     MediaTypeImage,
			Format:   "",
			Width:    0,
			Height:   0,
			Duration: 0,
			FileSize: 0,
			Filename: "",
			Metadata: make(map[string]string),
		},
	}
}

// WithURL sets the URL.
func (b *MediaBuilder) WithURL(url string) *MediaBuilder {
	b.media.URL = url
	return b
}

// WithType sets the media type.
func (b *MediaBuilder) WithType(mediaType MediaType) *MediaBuilder {
	b.media.Type = mediaType
	return b
}

// WithFormat sets the format.
func (b *MediaBuilder) WithFormat(format string) *MediaBuilder {
	b.media.Format = format
	return b
}

// WithDimensions sets the dimensions.
func (b *MediaBuilder) WithDimensions(width, height int) *MediaBuilder {
	b.media.Width = width
	b.media.Height = height
	return b
}

// WithDuration sets the duration.
func (b *MediaBuilder) WithDuration(duration int) *MediaBuilder {
	b.media.Duration = duration
	return b
}

// WithFileSize sets the file size.
func (b *MediaBuilder) WithFileSize(size int64) *MediaBuilder {
	b.media.FileSize = size
	return b
}

// WithFilename sets the filename.
func (b *MediaBuilder) WithFilename(filename string) *MediaBuilder {
	b.media.Filename = filename
	return b
}

// WithThumbnail sets the thumbnail URL.
func (b *MediaBuilder) WithThumbnail(url string) *MediaBuilder {
	b.media.ThumbnailURL = url
	return b
}

// WithMetadata sets metadata.
func (b *MediaBuilder) WithMetadata(key, value string) *MediaBuilder {
	if b.media.Metadata == nil {
		b.media.Metadata = make(map[string]string)
	}
	b.media.Metadata[key] = value
	return b
}

// Build validates and returns the media.
func (b *MediaBuilder) Build() (*Media, error) {
	if err := b.media.Validate(); err != nil {
		return nil, err
	}
	return b.media, nil
}

// MustBuild builds without error (panics on error).
func (b *MediaBuilder) MustBuild() *Media {
	m, err := b.Build()
	if err != nil {
		panic(err)
	}
	return m
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestMedia1 = MustNewMedia("https://example.com/image.jpg", 1024*1024)
	TestMedia2 = MustNewMedia("https://example.com/video.mp4", 5*1024*1024)
	TestMedia3 = MustNewMedia("https://example.com/audio.mp3", 2*1024*1024)
)

// MustNewTestImage creates a test image media.
func MustNewTestImage(url string) *Media {
	return MustNewMedia(url, 1024*1024)
}

// MustNewTestVideo creates a test video media.
func MustNewTestVideo(url string) *Media {
	return MustNewMedia(url, 5*1024*1024)
}

// MustNewTestGif creates a test GIF media.
func MustNewTestGif(url string) *Media {
	m := MustNewMedia(url, 2*1024*1024)
	m.Type = MediaTypeGif
	m.Format = "gif"
	return m
}

// MustNewTestAudio creates a test audio media.
func MustNewTestAudio(url string) *Media {
	m := MustNewMedia(url, 2*1024*1024)
	m.Type = MediaTypeAudio
	m.Format = "mp3"
	return m
}

// MustNewTestMediaWithDimensions creates a test media with dimensions.
func MustNewTestMediaWithDimensions(url string, width, height int) *Media {
	m, err := NewMediaWithDimensions(url, 1024*1024, width, height)
	if err != nil {
		panic(err)
	}
	return m
}

// MustNewTestMediaWithThumbnail creates a test media with thumbnail.
func MustNewTestMediaWithThumbnail(url, thumbURL string) *Media {
	m, err := NewMediaWithThumbnail(url, 1024*1024, thumbURL)
	if err != nil {
		panic(err)
	}
	return m
}

// ======================================================================
= JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (m *Media) MarshalJSON() ([]byte, error) {
	type Alias Media
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(m),
		Type:  string(m.Type),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (m *Media) UnmarshalJSON(data []byte) error {
	type Alias Media
	aux := &struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Type != "" {
		m.Type = MediaType(aux.Type)
	}
	return nil
}

// Value implements driver.Valuer for JSONB storage.
func (m Media) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements sql.Scanner for JSONB retrieval.
func (m *Media) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("unsupported type for Media: %T", value)
	}
	return json.Unmarshal(bytes, m)
}