// backend/internal/worker/media_worker.go
package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/sirupsen/logrus"
	"golang.org/x/image/draw"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	DefaultMediaQueueSize   = 500
	DefaultMediaWorkerCount = 4
	DefaultMediaBatchSize   = 10
	DefaultMediaFlushInterval = 5 * time.Second
	MaxImageSize            = 10 * 1024 * 1024 // 10MB
	MaxVideoSize            = 50 * 1024 * 1024 // 50MB
	ThumbnailSize           = 150
	MediumThumbnailSize     = 400
	LargeThumbnailSize      = 800
)

var (
	ErrMediaWorkerStopped = errors.New("media worker has been stopped")
	ErrMediaQueueFull     = errors.New("media queue is full")
	ErrInvalidMediaJob    = errors.New("invalid media job")
	ErrMediaTypeUnsupported = errors.New("unsupported media type")
	ErrMediaProcessingFailed = errors.New("media processing failed")
	ErrMediaFileTooLarge  = errors.New("media file too large")
	ErrMediaFormatInvalid = errors.New("invalid media format")
)

// ======================================================================
= MediaJob Types
// ======================================================================

// MediaJobType represents the type of media job.
type MediaJobType string

const (
	JobProcessImage MediaJobType = "process_image"
	JobProcessVideo MediaJobType = "process_video"
	JobGenerateThumbnail MediaJobType = "generate_thumbnail"
	JobOptimizeImage MediaJobType = "optimize_image"
	JobConvertFormat MediaJobType = "convert_format"
)

// ======================================================================
= MediaJob
// ======================================================================

// MediaJob represents a media processing job.
type MediaJob struct {
	ID          string                 `json:"id"`
	Type        MediaJobType           `json:"type"`
	UserID      string                 `json:"user_id"`
	TweetID     string                 `json:"tweet_id"`
	FileName    string                 `json:"file_name"`
	FileData    []byte                 `json:"-"`
	FileURL     string                 `json:"file_url,omitempty"`
	ContentType string                 `json:"content_type"`
	Options     MediaOptions           `json:"options"`
	Retries     int                    `json:"retries"`
	MaxRetries  int                    `json:"max_retries"`
	CreatedAt   time.Time              `json:"created_at"`
	Priority    int                    `json:"priority"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// MediaOptions represents media processing options.
type MediaOptions struct {
	Width            int      `json:"width"`
	Height           int      `json:"height"`
	Quality          int      `json:"quality"`
	Format           string   `json:"format"`
	Crop             string   `json:"crop"`
	ResizeMode       string   `json:"resize_mode"` // "fit", "fill", "crop"
	GenerateThumbnail bool    `json:"generate_thumbnail"`
	ThumbnailSizes   []int    `json:"thumbnail_sizes"`
	Optimize         bool     `json:"optimize"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// ======================================================================
= MediaResult
// ======================================================================

// MediaResult represents the result of media processing.
type MediaResult struct {
	OriginalURL    string            `json:"original_url"`
	ProcessedURL   string            `json:"processed_url"`
	ThumbnailURLs  map[string]string `json:"thumbnail_urls"` // size -> url
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	Size           int64             `json:"size"`
	Format         string            `json:"format"`
	Metadata       map[string]interface{} `json:"metadata"`
	ProcessingTime time.Duration     `json:"processing_time"`
}

// ======================================================================
= MediaWorkerConfig
// ======================================================================

// MediaWorkerConfig holds media worker configuration.
type MediaWorkerConfig struct {
	QueueSize       int
	WorkerCount     int
	BatchSize       int
	FlushInterval   time.Duration
	MaxImageSize    int64
	MaxVideoSize    int64
	ThumbnailSizes  []int
	AllowedImageTypes []string
	AllowedVideoTypes []string
	EnableOptimization bool
	EnableMetrics    bool
	TempDir          string
	Timeout          time.Duration
}

// DefaultMediaWorkerConfig returns sensible defaults.
func DefaultMediaWorkerConfig() MediaWorkerConfig {
	return MediaWorkerConfig{
		QueueSize:        DefaultMediaQueueSize,
		WorkerCount:      DefaultMediaWorkerCount,
		BatchSize:        DefaultMediaBatchSize,
		FlushInterval:    DefaultMediaFlushInterval,
		MaxImageSize:     MaxImageSize,
		MaxVideoSize:     MaxVideoSize,
		ThumbnailSizes:   []int{ThumbnailSize, MediumThumbnailSize, LargeThumbnailSize},
		AllowedImageTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
		AllowedVideoTypes: []string{"video/mp4", "video/quicktime", "video/webm"},
		EnableOptimization: true,
		EnableMetrics:    true,
		TempDir:          "/tmp/media",
		Timeout:          30 * time.Second,
	}
}

// ======================================================================
= MediaWorker
// ======================================================================

// MediaWorker handles media processing.
type MediaWorker struct {
	config         MediaWorkerConfig
	storageAdapter adapter.StorageAdapter
	queue          chan *MediaJob
	stopCh         chan struct{}
	wg             sync.WaitGroup
	metrics        *MediaMetrics
	log            *logrus.Entry
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.RWMutex
	started        bool
	stopped        bool
	batch          []*MediaJob
	batchMu        sync.Mutex
}

// MediaMetrics tracks media processing metrics.
type MediaMetrics struct {
	mu               sync.RWMutex
	TotalReceived    int64     `json:"total_received"`
	TotalProcessed   int64     `json:"total_processed"`
	TotalSucceeded   int64     `json:"total_succeeded"`
	TotalFailed      int64     `json:"total_failed"`
	QueueSize        int       `json:"queue_size"`
	ActiveWorkers    int       `json:"active_workers"`
	LastProcessedAt  time.Time `json:"last_processed_at"`
	LastError        string    `json:"last_error,omitempty"`
	AvgProcessingMs  float64   `json:"avg_processing_ms"`
	ProcessedImages  int64     `json:"processed_images"`
	ProcessedVideos  int64     `json:"processed_videos"`
	ThumbnailsGenerated int64  `json:"thumbnails_generated"`
}

// NewMediaWorker creates a new media worker.
func NewMediaWorker(
	storageAdapter adapter.StorageAdapter,
	cfg MediaWorkerConfig,
) *MediaWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &MediaWorker{
		config:         cfg,
		storageAdapter: storageAdapter,
		queue:          make(chan *MediaJob, cfg.QueueSize),
		stopCh:         make(chan struct{}),
		metrics:        &MediaMetrics{},
		log:            logger.WithField("worker", "media"),
		ctx:            ctx,
		cancel:         cancel,
		batch:          make([]*MediaJob, 0, cfg.BatchSize),
	}
}

// ======================================================================
// Start/Stop
// ======================================================================

// Start starts the media worker.
func (w *MediaWorker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return errors.New("media worker already started")
	}
	if w.stopped {
		return errors.New("media worker has been stopped")
	}
	w.started = true
	w.log.WithFields(logrus.Fields{
		"worker_count": w.config.WorkerCount,
		"queue_size":   w.config.QueueSize,
		"batch_size":   w.config.BatchSize,
	}).Info("Starting media worker")
	for i := 0; i < w.config.WorkerCount; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	// Start flush goroutine
	w.wg.Add(1)
	go w.flushLoop()
	// Start metrics reporter
	if w.config.EnableMetrics {
		w.wg.Add(1)
		go w.metricsReporter()
	}
	return nil
}

// Stop gracefully stops the media worker.
func (w *MediaWorker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return nil
	}
	w.log.Info("Stopping media worker...")
	w.stopped = true
	w.cancel()
	close(w.stopCh)
	// Flush remaining batch
	w.flushBatch()
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		w.log.Info("Media worker stopped gracefully")
	case <-time.After(30 * time.Second):
		w.log.Warn("Media worker stop timeout")
	}
	return nil
}

// ======================================================================
= Worker Goroutines
// ======================================================================

// worker runs a worker goroutine that processes media jobs.
func (w *MediaWorker) worker(id int) {
	defer w.wg.Done()
	w.log.WithField("worker_id", id).Debug("Media worker started")
	for {
		select {
		case <-w.stopCh:
			w.log.WithField("worker_id", id).Debug("Media worker stopped")
			return
		case <-w.ctx.Done():
			return
		default:
		}
		select {
		case job, ok := <-w.queue:
			if !ok {
				return
			}
			w.metrics.mu.Lock()
			w.metrics.TotalReceived++
			w.metrics.QueueSize = len(w.queue)
			w.metrics.ActiveWorkers++
			w.metrics.mu.Unlock()
			w.processJob(id, job)
			w.metrics.mu.Lock()
			w.metrics.ActiveWorkers--
			w.metrics.mu.Unlock()
		case <-w.stopCh:
			return
		case <-w.ctx.Done():
			return
		}
	}
}

// ======================================================================
= Process Job
// ======================================================================

// processJob processes a single media job.
func (w *MediaWorker) processJob(workerID int, job *MediaJob) {
	start := time.Now()
	w.log.WithFields(logrus.Fields{
		"worker_id": workerID,
		"job_id":    job.ID,
		"type":      job.Type,
		"file":      job.FileName,
		"user_id":   job.UserID,
	}).Debug("Processing media job")
	// Validate job
	if err := w.validateJob(job); err != nil {
		w.metrics.mu.Lock()
		w.metrics.TotalFailed++
		w.metrics.LastError = err.Error()
		w.metrics.mu.Unlock()
		w.log.WithError(err).WithField("job_id", job.ID).Warn("Invalid job dropped")
		return
	}
	// Process based on type
	var result *MediaResult
	var err error
	switch job.Type {
	case JobProcessImage:
		result, err = w.processImage(job)
	case JobProcessVideo:
		result, err = w.processVideo(job)
	case JobGenerateThumbnail:
		result, err = w.generateThumbnail(job)
	case JobOptimizeImage:
		result, err = w.optimizeImage(job)
	case JobConvertFormat:
		result, err = w.convertFormat(job)
	default:
		err = fmt.Errorf("unsupported job type: %s", job.Type)
	}
	elapsed := time.Since(start)
	if err != nil {
		w.metrics.mu.Lock()
		w.metrics.TotalFailed++
		w.metrics.LastError = err.Error()
		w.metrics.mu.Unlock()
		w.log.WithError(err).WithFields(logrus.Fields{
			"job_id":    job.ID,
			"latency_ms": elapsed.Milliseconds(),
		}).Warn("Media processing failed")
		// Retry logic
		if job.Retries < job.MaxRetries {
			job.Retries++
			w.log.WithFields(logrus.Fields{
				"job_id":  job.ID,
				"retry":   job.Retries,
				"max":     job.MaxRetries,
			}).Debug("Retrying media job")
			time.Sleep(time.Duration(job.Retries) * time.Second)
			select {
			case w.queue <- job:
			default:
				w.log.WithField("job_id", job.ID).Warn("Queue full, retry dropped")
			}
		}
		return
	}
	w.metrics.mu.Lock()
	w.metrics.TotalSucceeded++
	w.metrics.TotalProcessed++
	w.metrics.LastProcessedAt = time.Now()
	w.metrics.AvgProcessingMs = (w.metrics.AvgProcessingMs*float64(w.metrics.TotalProcessed-1) + float64(elapsed.Milliseconds())) / float64(w.metrics.TotalProcessed)
	if job.Type == JobProcessImage {
		w.metrics.ProcessedImages++
	} else if job.Type == JobProcessVideo {
		w.metrics.ProcessedVideos++
	}
	if result != nil && len(result.ThumbnailURLs) > 0 {
		w.metrics.ThumbnailsGenerated += int64(len(result.ThumbnailURLs))
	}
	w.metrics.mu.Unlock()
	w.log.WithFields(logrus.Fields{
		"job_id":     job.ID,
		"result_url": result.ProcessedURL,
		"latency_ms": elapsed.Milliseconds(),
	}).Debug("Media processed successfully")
}

// ======================================================================
= Media Processing Functions
// ======================================================================

// processImage processes an image job.
func (w *MediaWorker) processImage(job *MediaJob) (*MediaResult, error) {
	if job.FileData == nil && job.FileURL == "" {
		return nil, errors.New("no image data provided")
	}
	var img image.Image
	var format string
	var err error
	// Read image data
	if job.FileData != nil {
		img, format, err = image.Decode(bytes.NewReader(job.FileData))
	} else {
		// Download from URL
		data, err := w.downloadFile(job.FileURL)
		if err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}
		img, format, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	// Apply transformations
	bounds := img.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()
	width := job.Options.Width
	height := job.Options.Height
	if width > 0 || height > 0 {
		img = w.resizeImage(img, width, height, job.Options.ResizeMode)
	}
	// Generate thumbnails
	thumbnailURLs := make(map[string]string)
	if job.Options.GenerateThumbnail {
		sizes := job.Options.ThumbnailSizes
		if len(sizes) == 0 {
			sizes = w.config.ThumbnailSizes
		}
		for _, size := range sizes {
			thumb := w.resizeImage(img, size, size, "fill")
			thumbData, err := w.encodeImage(thumb, job.Options.Format)
			if err != nil {
				continue
			}
			thumbName := fmt.Sprintf("thumb_%d_%s", size, job.FileName)
			url, err := w.uploadFile(thumbData, thumbName, job.ContentType)
			if err != nil {
				continue
			}
			thumbnailURLs[fmt.Sprintf("%d", size)] = url
		}
	}
	// Encode final image
	processedData, err := w.encodeImage(img, job.Options.Format)
	if err != nil {
		return nil, fmt.Errorf("encode failed: %w", err)
	}
	processedName := fmt.Sprintf("processed_%s", job.FileName)
	processedURL, err := w.uploadFile(processedData, processedName, job.ContentType)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	return &MediaResult{
		OriginalURL:    job.FileURL,
		ProcessedURL:   processedURL,
		ThumbnailURLs:  thumbnailURLs,
		Width:          img.Bounds().Dx(),
		Height:         img.Bounds().Dy(),
		Size:           int64(len(processedData)),
		Format:         format,
		ProcessingTime: 0,
	}, nil
}

// processVideo processes a video job.
func (w *MediaWorker) processVideo(job *MediaJob) (*MediaResult, error) {
	// For now, we just validate and upload
	if job.FileData == nil && job.FileURL == "" {
		return nil, errors.New("no video data provided")
	}
	if len(job.FileData) > int(w.config.MaxVideoSize) {
		return nil, ErrMediaFileTooLarge
	}
	// Generate thumbnail from video (first frame) - simplified
	// In production, use ffmpeg or similar
	thumbnailURLs := make(map[string]string)
	if job.Options.GenerateThumbnail {
		// Placeholder: In production, extract first frame and upload
	}
	uploadURL := job.FileURL
	if job.FileData != nil {
		url, err := w.uploadFile(job.FileData, job.FileName, job.ContentType)
		if err != nil {
			return nil, err
		}
		uploadURL = url
	}
	return &MediaResult{
		OriginalURL:   job.FileURL,
		ProcessedURL:  uploadURL,
		ThumbnailURLs: thumbnailURLs,
		Size:          int64(len(job.FileData)),
		Format:        "video",
	}, nil
}

// generateThumbnail generates a thumbnail from media.
func (w *MediaWorker) generateThumbnail(job *MediaJob) (*MediaResult, error) {
	// Similar to processImage but only generates thumbnails
	return w.processImage(job)
}

// optimizeImage optimizes an image.
func (w *MediaWorker) optimizeImage(job *MediaJob) (*MediaResult, error) {
	if job.FileData == nil {
		return nil, errors.New("no image data provided")
	}
	img, format, err := image.Decode(bytes.NewReader(job.FileData))
	if err != nil {
		return nil, err
	}
	// Optimize quality
	quality := job.Options.Quality
	if quality == 0 {
		quality = 85
	}
	var processedData []byte
	switch format {
	case "jpeg", "jpg":
		buf := &bytes.Buffer{}
		err = jpeg.Encode(buf, img, &jpeg.Options{Quality: quality})
		processedData = buf.Bytes()
	case "png":
		buf := &bytes.Buffer{}
		err = png.Encode(buf, img)
		processedData = buf.Bytes()
	default:
		// Use existing format
		processedData = job.FileData
	}
	if err != nil {
		return nil, fmt.Errorf("optimize failed: %w", err)
	}
	uploadedURL, err := w.uploadFile(processedData, "optimized_"+job.FileName, job.ContentType)
	if err != nil {
		return nil, err
	}
	return &MediaResult{
		ProcessedURL: uploadedURL,
		Size:         int64(len(processedData)),
		Format:       format,
	}, nil
}

// convertFormat converts media format.
func (w *MediaWorker) convertFormat(job *MediaJob) (*MediaResult, error) {
	if job.FileData == nil {
		return nil, errors.New("no data provided")
	}
	targetFormat := job.Options.Format
	if targetFormat == "" {
		targetFormat = "jpeg"
	}
	img, _, err := image.Decode(bytes.NewReader(job.FileData))
	if err != nil {
		return nil, err
	}
	processedData, err := w.encodeImage(img, targetFormat)
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(job.FileName)
	newName := strings.TrimSuffix(job.FileName, ext) + "." + targetFormat
	uploadedURL, err := w.uploadFile(processedData, newName, "image/"+targetFormat)
	if err != nil {
		return nil, err
	}
	return &MediaResult{
		ProcessedURL: uploadedURL,
		Size:         int64(len(processedData)),
		Format:       targetFormat,
	}, nil
}

// ======================================================================
= Image Processing Helpers
// ======================================================================

// resizeImage resizes an image.
func (w *MediaWorker) resizeImage(img image.Image, width, height int, mode string) image.Image {
	bounds := img.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()
	if width == 0 && height == 0 {
		return img
	}
	if width == 0 {
		width = (origW * height) / origH
	}
	if height == 0 {
		height = (origH * width) / origW
	}
	dst := imaging.New(width, height, image.Transparent)
	switch mode {
	case "fill":
		// Fill the entire canvas, cropping if needed
		scaleX := float64(width) / float64(origW)
		scaleY := float64(height) / float64(origH)
		scale := scaleX
		if scaleY > scaleX {
			scale = scaleY
		}
		newW := int(float64(origW) * scale)
		newH := int(float64(origH) * scale)
		resized := imaging.Resize(img, newW, newH, imaging.Lanczos)
		// Crop to center
		srcBounds := resized.Bounds()
		srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
		offsetX := (srcW - width) / 2
		offsetY := (srcH - height) / 2
		return imaging.Crop(resized, image.Rect(offsetX, offsetY, offsetX+width, offsetY+height))
	case "fit":
		return imaging.Fit(img, width, height, imaging.Lanczos)
	case "crop":
		return imaging.Thumbnail(img, width, height, imaging.Lanczos)
	default:
		return imaging.Resize(img, width, height, imaging.Lanczos)
	}
}

// encodeImage encodes an image to the specified format.
func (w *MediaWorker) encodeImage(img image.Image, format string) ([]byte, error) {
	buf := &bytes.Buffer{}
	switch format {
	case "jpeg", "jpg":
		err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90})
		return buf.Bytes(), err
	case "png":
		err := png.Encode(buf, img)
		return buf.Bytes(), err
	case "gif":
		err := gif.Encode(buf, img, &gif.Options{NumColors: 256})
		return buf.Bytes(), err
	default:
		// Default to PNG
		err := png.Encode(buf, img)
		return buf.Bytes(), err
	}
}

// ======================================================================
= Upload Helper
// ======================================================================

// uploadFile uploads file data to storage.
func (w *MediaWorker) uploadFile(data []byte, name, contentType string) (string, error) {
	reader := bytes.NewReader(data)
	opts := &adapter.UploadOptions{
		ContentType: contentType,
		Public:      true,
	}
	result, err := w.storageAdapter.Upload(w.ctx, reader, name, opts)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// downloadFile downloads a file from a URL.
func (w *MediaWorker) downloadFile(url string) ([]byte, error) {
	// Simplified: In production, use HTTP client
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// ======================================================================
= Job Validation
// ======================================================================

// validateJob validates a media job.
func (w *MediaWorker) validateJob(job *MediaJob) error {
	if job == nil {
		return ErrInvalidMediaJob
	}
	if job.ID == "" {
		return errors.New("job ID is required")
	}
	if job.Type == "" {
		return errors.New("job type is required")
	}
	if job.FileName == "" && job.FileData == nil && job.FileURL == "" {
		return errors.New("no file provided")
	}
	if job.FileData != nil {
		// Validate size based on content type
		if strings.HasPrefix(job.ContentType, "image/") && int64(len(job.FileData)) > w.config.MaxImageSize {
			return ErrMediaFileTooLarge
		}
		if strings.HasPrefix(job.ContentType, "video/") && int64(len(job.FileData)) > w.config.MaxVideoSize {
			return ErrMediaFileTooLarge
		}
		// Validate content type
		if strings.HasPrefix(job.ContentType, "image/") {
			allowed := false
			for _, allowedType := range w.config.AllowedImageTypes {
				if job.ContentType == allowedType {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("image type not allowed: %s", job.ContentType)
			}
		}
		if strings.HasPrefix(job.ContentType, "video/") {
			allowed := false
			for _, allowedType := range w.config.AllowedVideoTypes {
				if job.ContentType == allowedType {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("video type not allowed: %s", job.ContentType)
			}
		}
	}
	return nil
}

// ======================================================================
= Batch Processing
// ======================================================================

// flushLoop periodically flushes the batch.
func (w *MediaWorker) flushLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.config.FlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.batchMu.Lock()
			if len(w.batch) > 0 {
				w.flushBatchLocked()
			}
			w.batchMu.Unlock()
		}
	}
}

// flushBatch flushes the current batch.
func (w *MediaWorker) flushBatch() {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	if len(w.batch) > 0 {
		w.flushBatchLocked()
	}
}

// flushBatchLocked flushes the batch (must be called with lock held).
func (w *MediaWorker) flushBatchLocked() {
	if len(w.batch) == 0 {
		return
	}
	w.log.WithField("batch_size", len(w.batch)).Debug("Flushing media batch")
	w.batch = make([]*MediaJob, 0, w.config.BatchSize)
}

// ======================================================================
= Metrics Reporter
// ======================================================================

// metricsReporter periodically reports metrics.
func (w *MediaWorker) metricsReporter() {
	defer w.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.reportMetrics()
		}
	}
}

// reportMetrics logs current metrics.
func (w *MediaWorker) reportMetrics() {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	w.log.WithFields(logrus.Fields{
		"total_received":     w.metrics.TotalReceived,
		"total_succeeded":    w.metrics.TotalSucceeded,
		"total_failed":       w.metrics.TotalFailed,
		"queue_size":         w.metrics.QueueSize,
		"active_workers":     w.metrics.ActiveWorkers,
		"avg_processing_ms":  w.metrics.AvgProcessingMs,
		"processed_images":   w.metrics.ProcessedImages,
		"processed_videos":   w.metrics.ProcessedVideos,
		"thumbnails_generated": w.metrics.ThumbnailsGenerated,
	}).Info("Media worker metrics")
}

// ======================================================================
= Queue Operations
// ======================================================================

// ProcessMedia enqueues a media job.
func (w *MediaWorker) ProcessMedia(job *MediaJob) error {
	if w.isStopped() {
		return ErrMediaWorkerStopped
	}
	if err := w.validateJob(job); err != nil {
		return err
	}
	if job.MaxRetries == 0 {
		job.MaxRetries = 3
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	select {
	case w.queue <- job:
		return nil
	default:
		return ErrMediaQueueFull
	}
}

// ProcessImage enqueues an image processing job.
func (w *MediaWorker) ProcessImage(userID, tweetID, fileName string, data []byte, opts MediaOptions) (string, error) {
	job := &MediaJob{
		ID:          generateMediaID(),
		Type:        JobProcessImage,
		UserID:      userID,
		TweetID:     tweetID,
		FileName:    fileName,
		FileData:    data,
		ContentType: "image/jpeg",
		Options:     opts,
		MaxRetries:  3,
		CreatedAt:   time.Now(),
	}
	if err := w.ProcessMedia(job); err != nil {
		return "", err
	}
	return job.ID, nil
}

// GenerateThumbnail enqueues a thumbnail generation job.
func (w *MediaWorker) GenerateThumbnail(userID, fileName string, data []byte, sizes []int) (string, error) {
	opts := MediaOptions{
		GenerateThumbnail: true,
		ThumbnailSizes:    sizes,
	}
	return w.ProcessImage(userID, "", fileName, data, opts)
}

// ======================================================================
= Queue Management
// ======================================================================

// QueueSize returns the current queue size.
func (w *MediaWorker) QueueSize() int {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	return w.metrics.QueueSize
}

// GetMetrics returns current metrics.
func (w *MediaWorker) GetMetrics() MediaMetrics {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	return *w.metrics
}

// IsStopped returns true if the worker is stopped.
func (w *MediaWorker) isStopped() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stopped
}

// IsStarted returns true if the worker is started.
func (w *MediaWorker) IsStarted() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.started && !w.stopped
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck performs a health check on the media worker.
func (w *MediaWorker) HealthCheck() map[string]interface{} {
	status := map[string]interface{}{
		"component":  "media_worker",
		"started":    w.IsStarted(),
		"queue_size": w.QueueSize(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	if w.IsStarted() {
		status["status"] = "ok"
	} else {
		status["status"] = "stopped"
	}
	metrics := w.GetMetrics()
	status["metrics"] = map[string]interface{}{
		"total_received":     metrics.TotalReceived,
		"total_succeeded":    metrics.TotalSucceeded,
		"total_failed":       metrics.TotalFailed,
		"active_workers":     metrics.ActiveWorkers,
		"avg_processing_ms":  metrics.AvgProcessingMs,
		"processed_images":   metrics.ProcessedImages,
		"processed_videos":   metrics.ProcessedVideos,
		"thumbnails_generated": metrics.ThumbnailsGenerated,
	}
	return status
}

// ======================================================================
= Helper Functions
// ======================================================================

// generateMediaID generates a unique media job ID.
func generateMediaID() string {
	return fmt.Sprintf("media_%d_%d", time.Now().UnixNano(), time.Now().UnixNano()%100000)
}

// ======================================================================
= Global Instance
// ======================================================================

var defaultMediaWorker *MediaWorker
var mediaWorkerOnce sync.Once

// InitMediaWorker initializes the global media worker.
func InitMediaWorker(
	storageAdapter adapter.StorageAdapter,
	cfg MediaWorkerConfig,
) error {
	var err error
	mediaWorkerOnce.Do(func() {
		defaultMediaWorker = NewMediaWorker(storageAdapter, cfg)
		err = defaultMediaWorker.Start()
	})
	return err
}

// GetMediaWorker returns the global media worker.
func GetMediaWorker() *MediaWorker {
	if defaultMediaWorker == nil {
		panic("media worker not initialized")
	}
	return defaultMediaWorker
}

// ProcessMediaJob processes a media job using the global worker.
func ProcessMediaJob(job *MediaJob) error {
	return GetMediaWorker().ProcessMedia(job)
}