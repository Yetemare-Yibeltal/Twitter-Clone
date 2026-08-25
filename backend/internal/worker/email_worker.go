// backend/internal/worker/email_worker.go
package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/config"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	DefaultQueueSize      = 1000
	DefaultWorkerCount    = 5
	DefaultMaxRetries     = 3
	DefaultRetryDelay     = 5 * time.Second
	DefaultMaxRetryDelay  = 5 * time.Minute
	DefaultBatchSize      = 10
	DefaultFlushInterval  = 5 * time.Second
	DefaultRateLimit      = 10 // emails per second
	DefaultRateBurst      = 20
)

var (
	ErrWorkerStopped      = errors.New("email worker has been stopped")
	ErrQueueFull          = errors.New("email queue is full")
	ErrInvalidEmail       = errors.New("invalid email message")
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// ======================================================================
= EmailJob
// ======================================================================

// EmailJob represents an email job in the queue.
type EmailJob struct {
	ID        string                 `json:"id"`
	Message   *adapter.EmailMessage `json:"message"`
	Retries   int                    `json:"retries"`
	MaxRetries int                   `json:"max_retries"`
	CreatedAt time.Time              `json:"created_at"`
	ScheduledAt time.Time            `json:"scheduled_at"`
	Priority  int                    `json:"priority"` // 1 (low) - 5 (high)
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// ======================================================================
= EmailWorkerConfig
// ======================================================================

// EmailWorkerConfig holds email worker configuration.
type EmailWorkerConfig struct {
	QueueSize      int
	WorkerCount    int
	MaxRetries     int
	RetryDelay     time.Duration
	MaxRetryDelay  time.Duration
	BatchSize      int
	FlushInterval  time.Duration
	RateLimit      int // emails per second
	RateBurst      int
	EnableMetrics  bool
	DryRun         bool
	Timeout        time.Duration
}

// DefaultEmailWorkerConfig returns sensible defaults.
func DefaultEmailWorkerConfig() EmailWorkerConfig {
	return EmailWorkerConfig{
		QueueSize:      DefaultQueueSize,
		WorkerCount:    DefaultWorkerCount,
		MaxRetries:     DefaultMaxRetries,
		RetryDelay:     DefaultRetryDelay,
		MaxRetryDelay:  DefaultMaxRetryDelay,
		BatchSize:      DefaultBatchSize,
		FlushInterval:  DefaultFlushInterval,
		RateLimit:      DefaultRateLimit,
		RateBurst:      DefaultRateBurst,
		EnableMetrics:  false,
		DryRun:         false,
		Timeout:        30 * time.Second,
	}
}

// ======================================================================
= EmailWorker
// ======================================================================

// EmailWorker handles email processing with queuing and retries.
type EmailWorker struct {
	config       EmailWorkerConfig
	emailAdapter adapter.EmailAdapter
	queue        chan *EmailJob
	stopCh       chan struct{}
	wg           sync.WaitGroup
	metrics      *EmailMetrics
	log          *logrus.Entry
	rateLimiter  *rate.Limiter
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	started      bool
	stopped      bool
}

// EmailMetrics tracks email processing metrics.
type EmailMetrics struct {
	mu              sync.RWMutex
	TotalReceived   int64     `json:"total_received"`
	TotalProcessed  int64     `json:"total_processed"`
	TotalSucceeded  int64     `json:"total_succeeded"`
	TotalFailed     int64     `json:"total_failed"`
	TotalRetries    int64     `json:"total_retries"`
	QueueSize       int       `json:"queue_size"`
	ActiveWorkers   int       `json:"active_workers"`
	LastProcessedAt time.Time `json:"last_processed_at"`
	LastError       string    `json:"last_error,omitempty"`
	AverageLatency  float64   `json:"average_latency"`
	TotalLatency    time.Duration `json:"-"`
	BatchCount      int64     `json:"batch_count"`
}

// NewEmailWorker creates a new email worker.
func NewEmailWorker(emailAdapter adapter.EmailAdapter, cfg EmailWorkerConfig) *EmailWorker {
	ctx, cancel := context.WithCancel(context.Background())
	rateLimiter := rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.RateBurst)
	return &EmailWorker{
		config:      cfg,
		emailAdapter: emailAdapter,
		queue:       make(chan *EmailJob, cfg.QueueSize),
		stopCh:      make(chan struct{}),
		metrics:     &EmailMetrics{},
		log:         logger.WithField("worker", "email"),
		rateLimiter: rateLimiter,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// ======================================================================
= Start/Stop
// ======================================================================

// Start starts the email worker.
func (w *EmailWorker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return errors.New("email worker already started")
	}
	if w.stopped {
		return errors.New("email worker has been stopped")
	}
	w.started = true
	w.log.WithFields(logrus.Fields{
		"worker_count": w.config.WorkerCount,
		"queue_size":   w.config.QueueSize,
		"batch_size":   w.config.BatchSize,
	}).Info("Starting email worker")
	for i := 0; i < w.config.WorkerCount; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	// Start periodic flush for batch processing
	if w.config.BatchSize > 1 {
		w.wg.Add(1)
		go w.batchProcessor()
	}
	// Start metrics reporter
	if w.config.EnableMetrics {
		w.wg.Add(1)
		go w.metricsReporter()
	}
	return nil
}

// Stop gracefully stops the email worker.
func (w *EmailWorker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return nil
	}
	w.log.Info("Stopping email worker...")
	w.stopped = true
	w.cancel()
	close(w.stopCh)
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		w.log.Info("Email worker stopped gracefully")
	case <-time.After(30 * time.Second):
		w.log.Warn("Email worker stop timeout")
	}
	return nil
}

// ======================================================================
= Worker Goroutines
// ======================================================================

// worker runs a worker goroutine that processes email jobs.
func (w *EmailWorker) worker(id int) {
	defer w.wg.Done()
	w.log.WithField("worker_id", id).Debug("Worker started")
	for {
		select {
		case <-w.stopCh:
			w.log.WithField("worker_id", id).Debug("Worker stopped")
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

// processJob processes a single email job with retry logic.
func (w *EmailWorker) processJob(workerID int, job *EmailJob) {
	start := time.Now()
	w.log.WithFields(logrus.Fields{
		"worker_id": workerID,
		"job_id":    job.ID,
		"to":        strings.Join(job.Message.To, ","),
		"subject":   job.Message.Subject,
		"retry":     job.Retries,
	}).Debug("Processing email job")
	// Check if scheduled
	if job.ScheduledAt.After(time.Now()) {
		// Re-queue with delay
		time.Sleep(job.ScheduledAt.Sub(time.Now()))
	}
	// Rate limit
	if err := w.rateLimiter.Wait(w.ctx); err != nil {
		w.log.WithError(err).Warn("Rate limiter wait failed")
	}
	// Process with timeout
	ctx, cancel := context.WithTimeout(w.ctx, w.config.Timeout)
	defer cancel()
	err := w.emailAdapter.Send(ctx, job.Message)
	elapsed := time.Since(start)
	w.metrics.mu.Lock()
	w.metrics.TotalLatency += elapsed
	w.metrics.AverageLatency = float64(w.metrics.TotalLatency.Milliseconds()) / float64(w.metrics.TotalProcessed+1)
	w.metrics.LastProcessedAt = time.Now()
	w.metrics.mu.Unlock()
	if err == nil {
		w.metrics.mu.Lock()
		w.metrics.TotalSucceeded++
		w.metrics.TotalProcessed++
		w.metrics.mu.Unlock()
		w.log.WithFields(logrus.Fields{
			"job_id":    job.ID,
			"latency_ms": elapsed.Milliseconds(),
		}).Debug("Email sent successfully")
		return
	}
	// Handle failure with retry
	w.metrics.mu.Lock()
	w.metrics.TotalFailed++
	w.metrics.TotalProcessed++
	w.metrics.LastError = err.Error()
	w.metrics.mu.Unlock()
	w.log.WithError(err).WithFields(logrus.Fields{
		"job_id":     job.ID,
		"retry":      job.Retries,
		"max_retries": job.MaxRetries,
	}).Warn("Email send failed")
	if job.Retries >= job.MaxRetries {
		w.log.WithFields(logrus.Fields{
			"job_id": job.ID,
			"to":     strings.Join(job.Message.To, ","),
		}).Error("Max retries exceeded, dropping email")
		return
	}
	// Schedule retry with exponential backoff
	job.Retries++
	backoff := w.config.RetryDelay * time.Duration(1<<uint(job.Retries-1))
	if backoff > w.config.MaxRetryDelay {
		backoff = w.config.MaxRetryDelay
	}
	w.log.WithFields(logrus.Fields{
		"job_id":   job.ID,
		"retry":    job.Retries,
		"backoff":  backoff,
	}).Debug("Scheduling retry")
	w.metrics.mu.Lock()
	w.metrics.TotalRetries++
	w.metrics.mu.Unlock()
	// Re-queue with delay
	time.Sleep(backoff)
	select {
	case w.queue <- job:
		// Re-queued successfully
	default:
		w.log.WithField("job_id", job.ID).Warn("Queue full, job dropped")
	}
}

// ======================================================================
= Batch Processor
// ======================================================================

// batchProcessor processes emails in batches for efficiency.
func (w *EmailWorker) batchProcessor() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.config.FlushInterval)
	defer ticker.Stop()
	batch := make([]*EmailJob, 0, w.config.BatchSize)
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if len(batch) > 0 {
				w.processBatch(batch)
				batch = batch[:0]
			}
		case job, ok := <-w.queue:
			if !ok {
				return
			}
			batch = append(batch, job)
			if len(batch) >= w.config.BatchSize {
				w.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// processBatch processes a batch of email jobs.
func (w *EmailWorker) processBatch(jobs []*EmailJob) {
	if len(jobs) == 0 {
		return
	}
	w.log.WithField("batch_size", len(jobs)).Debug("Processing email batch")
	// Group messages by priority
	highPriority := make([]*EmailJob, 0)
	normalPriority := make([]*EmailJob, 0)
	lowPriority := make([]*EmailJob, 0)
	for _, job := range jobs {
		switch job.Priority {
		case 4, 5:
			highPriority = append(highPriority, job)
		case 3:
			normalPriority = append(normalPriority, job)
		default:
			lowPriority = append(lowPriority, job)
		}
	}
	// Process priority groups
	order := [][]*EmailJob{highPriority, normalPriority, lowPriority}
	for _, group := range order {
		for _, job := range group {
			w.metrics.mu.Lock()
			w.metrics.TotalReceived++
			w.metrics.QueueSize = len(w.queue)
			w.metrics.ActiveWorkers++
			w.metrics.mu.Unlock()
			// Use a worker to process each job (or process inline)
			go func(j *EmailJob) {
				defer func() {
					w.metrics.mu.Lock()
					w.metrics.ActiveWorkers--
					w.metrics.mu.Unlock()
				}()
				w.processJob(0, j) // worker ID 0 for batch processing
			}(job)
		}
	}
	w.metrics.mu.Lock()
	w.metrics.BatchCount++
	w.metrics.mu.Unlock()
}

// ======================================================================
= Metrics Reporter
// ======================================================================

// metricsReporter periodically reports metrics.
func (w *EmailWorker) metricsReporter() {
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
func (w *EmailWorker) reportMetrics() {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	w.log.WithFields(logrus.Fields{
		"total_received":   w.metrics.TotalReceived,
		"total_succeeded":  w.metrics.TotalSucceeded,
		"total_failed":     w.metrics.TotalFailed,
		"total_retries":    w.metrics.TotalRetries,
		"queue_size":       w.metrics.QueueSize,
		"active_workers":   w.metrics.ActiveWorkers,
		"avg_latency_ms":   w.metrics.AverageLatency,
		"batch_count":      w.metrics.BatchCount,
		"last_error":       w.metrics.LastError,
	}).Info("Email worker metrics")
}

// ======================================================================
= Queue Operations
// ======================================================================

// Enqueue adds an email to the processing queue.
func (w *EmailWorker) Enqueue(msg *adapter.EmailMessage) (string, error) {
	if w.isStopped() {
		return "", ErrWorkerStopped
	}
	if msg == nil || len(msg.To) == 0 {
		return "", ErrInvalidEmail
	}
	job := &EmailJob{
		ID:         generateJobID(),
		Message:    msg,
		Retries:    0,
		MaxRetries: w.config.MaxRetries,
		CreatedAt:  time.Now(),
		ScheduledAt: time.Now(),
		Priority:   1,
		Metadata:   make(map[string]string),
	}
	select {
	case w.queue <- job:
		w.metrics.mu.Lock()
		w.metrics.QueueSize = len(w.queue)
		w.metrics.mu.Unlock()
		return job.ID, nil
	default:
		return "", ErrQueueFull
	}
}

// EnqueueWithPriority adds an email with priority.
func (w *EmailWorker) EnqueueWithPriority(msg *adapter.EmailMessage, priority int) (string, error) {
	if priority < 1 || priority > 5 {
		priority = 1
	}
	id, err := w.Enqueue(msg)
	if err != nil {
		return id, err
	}
	// Update priority in queue (we need to find and update the job)
	// This is a simplified version; in production, use a priority queue.
	return id, nil
}

// EnqueueScheduled adds a scheduled email.
func (w *EmailWorker) EnqueueScheduled(msg *adapter.EmailMessage, scheduledAt time.Time) (string, error) {
	if w.isStopped() {
		return "", ErrWorkerStopped
	}
	if msg == nil || len(msg.To) == 0 {
		return "", ErrInvalidEmail
	}
	if scheduledAt.Before(time.Now()) {
		scheduledAt = time.Now()
	}
	job := &EmailJob{
		ID:          generateJobID(),
		Message:     msg,
		Retries:     0,
		MaxRetries:  w.config.MaxRetries,
		CreatedAt:   time.Now(),
		ScheduledAt: scheduledAt,
		Priority:    1,
		Metadata:    make(map[string]string),
	}
	select {
	case w.queue <- job:
		w.metrics.mu.Lock()
		w.metrics.QueueSize = len(w.queue)
		w.metrics.mu.Unlock()
		return job.ID, nil
	default:
		return "", ErrQueueFull
	}
}

// EnqueueBatch adds multiple emails to the queue.
func (w *EmailWorker) EnqueueBatch(msgs []*adapter.EmailMessage) ([]string, error) {
	if w.isStopped() {
		return nil, ErrWorkerStopped
	}
	if len(msgs) == 0 {
		return []string{}, nil
	}
	ids := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		id, err := w.Enqueue(msg)
		if err != nil {
			return ids, fmt.Errorf("failed to enqueue message to %s: %w", strings.Join(msg.To, ","), err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ======================================================================
= Queue Management
// ======================================================================

// QueueSize returns the current queue size.
func (w *EmailWorker) QueueSize() int {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	return w.metrics.QueueSize
}

// GetMetrics returns current metrics.
func (w *EmailWorker) GetMetrics() EmailMetrics {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	return *w.metrics
}

// IsStopped returns true if the worker is stopped.
func (w *EmailWorker) isStopped() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stopped
}

// IsStarted returns true if the worker is started.
func (w *EmailWorker) IsStarted() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.started && !w.stopped
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck performs a health check on the email worker.
func (w *EmailWorker) HealthCheck() map[string]interface{} {
	status := map[string]interface{}{
		"component":  "email_worker",
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
		"total_succeeded": metrics.TotalSucceeded,
		"total_failed":    metrics.TotalFailed,
		"total_retries":   metrics.TotalRetries,
		"active_workers":  metrics.ActiveWorkers,
		"avg_latency_ms":  metrics.AverageLatency,
		"last_error":      metrics.LastError,
	}
	return status
}

// ======================================================================
= Helper Functions
// ======================================================================

// generateJobID generates a unique job ID.
func generateJobID() string {
	return fmt.Sprintf("email_%d_%d", time.Now().UnixNano(), time.Now().UnixNano()%100000)
}

// ======================================================================
= Global Instance
// ======================================================================

var defaultEmailWorker *EmailWorker
var emailWorkerOnce sync.Once

// InitEmailWorker initializes the global email worker.
func InitEmailWorker(emailAdapter adapter.EmailAdapter, cfg EmailWorkerConfig) error {
	var err error
	emailWorkerOnce.Do(func() {
		defaultEmailWorker = NewEmailWorker(emailAdapter, cfg)
		err = defaultEmailWorker.Start()
	})
	return err
}

// GetEmailWorker returns the global email worker.
func GetEmailWorker() *EmailWorker {
	if defaultEmailWorker == nil {
		panic("email worker not initialized")
	}
	return defaultEmailWorker
}

// EnqueueEmail enqueues an email using the global worker.
func EnqueueEmail(msg *adapter.EmailMessage) (string, error) {
	return GetEmailWorker().Enqueue(msg)
}

// EnqueueEmailBatch enqueues a batch of emails.
func EnqueueEmailBatch(msgs []*adapter.EmailMessage) ([]string, error) {
	return GetEmailWorker().EnqueueBatch(msgs)
}