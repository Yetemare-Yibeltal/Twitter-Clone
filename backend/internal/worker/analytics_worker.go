// backend/internal/worker/analytics_worker.go
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	DefaultAnalyticsQueueSize   = 10000
	DefaultAnalyticsWorkerCount = 3
	DefaultAnalyticsBatchSize   = 100
	DefaultAnalyticsFlushInterval = 10 * time.Second
	DefaultAggregationInterval  = 5 * time.Minute
	DefaultRetentionDays        = 90
)

var (
	ErrAnalyticsWorkerStopped = errors.New("analytics worker has been stopped")
	ErrAnalyticsQueueFull     = errors.New("analytics queue is full")
	ErrInvalidAnalyticsEvent  = errors.New("invalid analytics event")
)

// ======================================================================
= AnalyticsEvent Types
// ======================================================================

// AnalyticsEventType represents the type of analytics event.
type AnalyticsEventType string

const (
	EventTweetCreate    AnalyticsEventType = "tweet_create"
	EventTweetDelete    AnalyticsEventType = "tweet_delete"
	EventTweetView      AnalyticsEventType = "tweet_view"
	EventLike           AnalyticsEventType = "like"
	EventUnlike         AnalyticsEventType = "unlike"
	EventRetweet        AnalyticsEventType = "retweet"
	EventUnretweet      AnalyticsEventType = "unretweet"
	EventFollow         AnalyticsEventType = "follow"
	EventUnfollow       AnalyticsEventType = "unfollow"
	EventSearch         AnalyticsEventType = "search"
	EventLogin          AnalyticsEventType = "login"
	EventLogout         AnalyticsEventType = "logout"
	EventSignup         AnalyticsEventType = "signup"
	EventProfileView    AnalyticsEventType = "profile_view"
	EventNotification   AnalyticsEventType = "notification"
	EventMessage        AnalyticsEventType = "message"
	EventCommunityJoin  AnalyticsEventType = "community_join"
	EventCommunityLeave AnalyticsEventType = "community_leave"
	EventPollVote       AnalyticsEventType = "poll_vote"
)

// ======================================================================
= AnalyticsEvent
// ======================================================================

// AnalyticsEvent represents a single analytics event.
type AnalyticsEvent struct {
	ID          string                 `json:"id"`
	Type        AnalyticsEventType     `json:"type"`
	UserID      string                 `json:"user_id,omitempty"`
	TargetID    string                 `json:"target_id,omitempty"`
	TargetType  string                 `json:"target_type,omitempty"`
	IP          string                 `json:"ip,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Referrer    string                 `json:"referrer,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	DeviceType  string                 `json:"device_type,omitempty"`
	OS          string                 `json:"os,omitempty"`
	Browser     string                 `json:"browser,omitempty"`
	Location    string                 `json:"location,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ======================================================================
= AggregatedStats
// ======================================================================

// AggregatedStats represents aggregated analytics statistics.
type AggregatedStats struct {
	Date          time.Time `json:"date"`
	TotalTweets   int64     `json:"total_tweets"`
	TotalLikes    int64     `json:"total_likes"`
	TotalRetweets int64     `json:"total_retweets"`
	TotalFollows  int64     `json:"total_follows"`
	TotalSearches int64     `json:"total_searches"`
	TotalLogins   int64     `json:"total_logins"`
	TotalSignups  int64     `json:"total_signups"`
	ActiveUsers   int64     `json:"active_users"`
	NewUsers      int64     `json:"new_users"`
	TotalMessages int64     `json:"total_messages"`
	EngagementRate float64  `json:"engagement_rate"`
}

// ======================================================================
= AnalyticsWorkerConfig
// ======================================================================

// AnalyticsWorkerConfig holds analytics worker configuration.
type AnalyticsWorkerConfig struct {
	QueueSize         int
	WorkerCount       int
	BatchSize         int
	FlushInterval     time.Duration
	AggregationInterval time.Duration
	RetentionDays     int
	EnableMetrics     bool
	EnableGeoLocation bool
	EnableDeviceTracking bool
}

// DefaultAnalyticsWorkerConfig returns sensible defaults.
func DefaultAnalyticsWorkerConfig() AnalyticsWorkerConfig {
	return AnalyticsWorkerConfig{
		QueueSize:         DefaultAnalyticsQueueSize,
		WorkerCount:       DefaultAnalyticsWorkerCount,
		BatchSize:         DefaultAnalyticsBatchSize,
		FlushInterval:     DefaultAnalyticsFlushInterval,
		AggregationInterval: DefaultAggregationInterval,
		RetentionDays:     DefaultRetentionDays,
		EnableMetrics:     true,
		EnableGeoLocation: false,
		EnableDeviceTracking: true,
	}
}

// ======================================================================
= AnalyticsWorker
// ======================================================================

// AnalyticsWorker handles analytics event processing.
type AnalyticsWorker struct {
	config          AnalyticsWorkerConfig
	eventRepo       interfaces.EventRepository
	statsRepo       interfaces.StatsRepository
	queue           chan *AnalyticsEvent
	stopCh          chan struct{}
	wg              sync.WaitGroup
	metrics         *AnalyticsMetrics
	log             *logrus.Entry
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex
	started         bool
	stopped         bool
	batch           []*AnalyticsEvent
	batchMu         sync.Mutex
	lastAggregation time.Time
}

// AnalyticsMetrics tracks analytics processing metrics.
type AnalyticsMetrics struct {
	mu              sync.RWMutex
	TotalReceived   int64     `json:"total_received"`
	TotalProcessed  int64     `json:"total_processed"`
	TotalFailed     int64     `json:"total_failed"`
	QueueSize       int       `json:"queue_size"`
	ActiveWorkers   int       `json:"active_workers"`
	LastProcessedAt time.Time `json:"last_processed_at"`
	LastError       string    `json:"last_error,omitempty"`
	EventsByType    map[AnalyticsEventType]int64 `json:"events_by_type"`
	AggregatedStats *AggregatedStats `json:"aggregated_stats"`
}

// NewAnalyticsWorker creates a new analytics worker.
func NewAnalyticsWorker(
	eventRepo interfaces.EventRepository,
	statsRepo interfaces.StatsRepository,
	cfg AnalyticsWorkerConfig,
) *AnalyticsWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &AnalyticsWorker{
		config:          cfg,
		eventRepo:       eventRepo,
		statsRepo:       statsRepo,
		queue:           make(chan *AnalyticsEvent, cfg.QueueSize),
		stopCh:          make(chan struct{}),
		metrics:         &AnalyticsMetrics{EventsByType: make(map[AnalyticsEventType]int64)},
		log:             logger.WithField("worker", "analytics"),
		ctx:             ctx,
		cancel:          cancel,
		batch:           make([]*AnalyticsEvent, 0, cfg.BatchSize),
		lastAggregation: time.Now(),
	}
}

// ======================================================================
= Start/Stop
// ======================================================================

// Start starts the analytics worker.
func (w *AnalyticsWorker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return errors.New("analytics worker already started")
	}
	if w.stopped {
		return errors.New("analytics worker has been stopped")
	}
	w.started = true
	w.log.WithFields(logrus.Fields{
		"worker_count": w.config.WorkerCount,
		"queue_size":   w.config.QueueSize,
		"batch_size":   w.config.BatchSize,
	}).Info("Starting analytics worker")
	for i := 0; i < w.config.WorkerCount; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
	// Start flush goroutine
	w.wg.Add(1)
	go w.flushLoop()
	// Start aggregation goroutine
	w.wg.Add(1)
	go w.aggregationLoop()
	// Start metrics reporter
	if w.config.EnableMetrics {
		w.wg.Add(1)
		go w.metricsReporter()
	}
	return nil
}

// Stop gracefully stops the analytics worker.
func (w *AnalyticsWorker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return nil
	}
	w.log.Info("Stopping analytics worker...")
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
		w.log.Info("Analytics worker stopped gracefully")
	case <-time.After(30 * time.Second):
		w.log.Warn("Analytics worker stop timeout")
	}
	return nil
}

// ======================================================================
= Worker Goroutines
// ======================================================================

// worker runs a worker goroutine that processes analytics events.
func (w *AnalyticsWorker) worker(id int) {
	defer w.wg.Done()
	w.log.WithField("worker_id", id).Debug("Analytics worker started")
	for {
		select {
		case <-w.stopCh:
			w.log.WithField("worker_id", id).Debug("Analytics worker stopped")
			return
		case <-w.ctx.Done():
			return
		default:
		}
		select {
		case event, ok := <-w.queue:
			if !ok {
				return
			}
			w.metrics.mu.Lock()
			w.metrics.TotalReceived++
			w.metrics.QueueSize = len(w.queue)
			w.metrics.ActiveWorkers++
			w.metrics.mu.Unlock()
			w.processEvent(id, event)
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
= Process Event
// ======================================================================

// processEvent processes a single analytics event.
func (w *AnalyticsWorker) processEvent(workerID int, event *AnalyticsEvent) {
	w.log.WithFields(logrus.Fields{
		"worker_id": workerID,
		"event_id":  event.ID,
		"type":      event.Type,
		"user_id":   event.UserID,
	}).Debug("Processing analytics event")
	// Validate event
	if err := w.validateEvent(event); err != nil {
		w.metrics.mu.Lock()
		w.metrics.TotalFailed++
		w.metrics.LastError = err.Error()
		w.metrics.mu.Unlock()
		w.log.WithError(err).WithField("event_id", event.ID).Warn("Invalid event dropped")
		return
	}
	// Add to batch
	w.batchMu.Lock()
	w.batch = append(w.batch, event)
	if len(w.batch) >= w.config.BatchSize {
		w.flushBatchLocked()
	}
	w.batchMu.Unlock()
	w.metrics.mu.Lock()
	w.metrics.TotalProcessed++
	w.metrics.LastProcessedAt = time.Now()
	w.metrics.EventsByType[event.Type]++
	w.metrics.mu.Unlock()
}

// ======================================================================
= Batch Processing
// ======================================================================

// flushLoop periodically flushes the batch.
func (w *AnalyticsWorker) flushLoop() {
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

// flushBatch flushes the current batch to storage.
func (w *AnalyticsWorker) flushBatch() {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	if len(w.batch) > 0 {
		w.flushBatchLocked()
	}
}

// flushBatchLocked flushes the batch (must be called with lock held).
func (w *AnalyticsWorker) flushBatchLocked() {
	if len(w.batch) == 0 {
		return
	}
	batch := w.batch
	w.batch = make([]*AnalyticsEvent, 0, w.config.BatchSize)
	w.log.WithField("batch_size", len(batch)).Debug("Flushing analytics batch")
	// Store events in repository
	if err := w.eventRepo.BulkCreate(w.ctx, batch); err != nil {
		w.log.WithError(err).Error("Failed to store analytics batch")
		w.metrics.mu.Lock()
		w.metrics.TotalFailed += int64(len(batch))
		w.metrics.LastError = err.Error()
		w.metrics.mu.Unlock()
		// Re-queue events
		for _, event := range batch {
			select {
			case w.queue <- event:
			default:
				w.log.WithField("event_id", event.ID).Warn("Failed to re-queue event")
			}
		}
		return
	}
	w.log.WithField("batch_size", len(batch)).Debug("Analytics batch stored successfully")
}

// ======================================================================
= Aggregation
// ======================================================================

// aggregationLoop performs periodic aggregation.
func (w *AnalyticsWorker) aggregationLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.config.AggregationInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.runAggregation()
		}
	}
}

// runAggregation runs the aggregation process.
func (w *AnalyticsWorker) runAggregation() {
	w.log.Debug("Running analytics aggregation")
	now := time.Now()
	// Aggregate events since last aggregation
	since := w.lastAggregation
	w.lastAggregation = now
	// Get events from repository
	events, err := w.eventRepo.GetByDateRange(w.ctx, since, now, 0, 10000)
	if err != nil {
		w.log.WithError(err).Warn("Failed to get events for aggregation")
		return
	}
	if len(events) == 0 {
		return
	}
	// Aggregate stats
	stats := w.aggregateEvents(events)
	// Store aggregated stats
	if err := w.statsRepo.Create(w.ctx, stats); err != nil {
		w.log.WithError(err).Warn("Failed to store aggregated stats")
		return
	}
	w.metrics.mu.Lock()
	w.metrics.AggregatedStats = stats
	w.metrics.mu.Unlock()
	w.log.WithField("date", stats.Date).Debug("Aggregation completed")
}

// aggregateEvents aggregates a list of events.
func (w *AnalyticsWorker) aggregateEvents(events []*AnalyticsEvent) *AggregatedStats {
	stats := &AggregatedStats{
		Date: time.Now().Truncate(24 * time.Hour),
	}
	userSet := make(map[string]bool)
	newUserSet := make(map[string]bool)
	for _, event := range events {
		switch event.Type {
		case EventTweetCreate:
			stats.TotalTweets++
		case EventLike:
			stats.TotalLikes++
		case EventRetweet:
			stats.TotalRetweets++
		case EventFollow:
			stats.TotalFollows++
		case EventSearch:
			stats.TotalSearches++
		case EventLogin:
			stats.TotalLogins++
		case EventSignup:
			stats.TotalSignups++
			newUserSet[event.UserID] = true
		case EventMessage:
			stats.TotalMessages++
		}
		if event.UserID != "" {
			userSet[event.UserID] = true
		}
	}
	stats.ActiveUsers = int64(len(userSet))
	stats.NewUsers = int64(len(newUserSet))
	// Calculate engagement rate
	if stats.ActiveUsers > 0 {
		stats.EngagementRate = float64(stats.TotalLikes+stats.TotalRetweets) / float64(stats.ActiveUsers)
	}
	return stats
}

// ======================================================================
= Event Validation
// ======================================================================

// validateEvent validates an analytics event.
func (w *AnalyticsWorker) validateEvent(event *AnalyticsEvent) error {
	if event == nil {
		return ErrInvalidAnalyticsEvent
	}
	if event.ID == "" {
		return errors.New("event ID is required")
	}
	if event.Type == "" {
		return errors.New("event type is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	// Validate event type
	validTypes := map[AnalyticsEventType]bool{
		EventTweetCreate: true, EventTweetDelete: true, EventTweetView: true,
		EventLike: true, EventUnlike: true, EventRetweet: true,
		EventUnretweet: true, EventFollow: true, EventUnfollow: true,
		EventSearch: true, EventLogin: true, EventLogout: true,
		EventSignup: true, EventProfileView: true, EventNotification: true,
		EventMessage: true, EventCommunityJoin: true, EventCommunityLeave: true,
		EventPollVote: true,
	}
	if !validTypes[event.Type] {
		return fmt.Errorf("invalid event type: %s", event.Type)
	}
	return nil
}

// ======================================================================
= Metrics Reporter
// ======================================================================

// metricsReporter periodically reports metrics.
func (w *AnalyticsWorker) metricsReporter() {
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
func (w *AnalyticsWorker) reportMetrics() {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	w.log.WithFields(logrus.Fields{
		"total_received":  w.metrics.TotalReceived,
		"total_processed": w.metrics.TotalProcessed,
		"total_failed":    w.metrics.TotalFailed,
		"queue_size":      w.metrics.QueueSize,
		"active_workers":  w.metrics.ActiveWorkers,
		"events_by_type":  w.metrics.EventsByType,
	}).Info("Analytics worker metrics")
}

// ======================================================================
= Queue Operations
// ======================================================================

// RecordEvent records a single analytics event.
func (w *AnalyticsWorker) RecordEvent(event *AnalyticsEvent) error {
	if w.isStopped() {
		return ErrAnalyticsWorkerStopped
	}
	if err := w.validateEvent(event); err != nil {
		return err
	}
	select {
	case w.queue <- event:
		return nil
	default:
		return ErrAnalyticsQueueFull
	}
}

// RecordEventType records an event by type with data.
func (w *AnalyticsWorker) RecordEventType(eventType AnalyticsEventType, userID, targetID string, data map[string]interface{}) error {
	event := &AnalyticsEvent{
		ID:        generateAnalyticsID(),
		Type:      eventType,
		UserID:    userID,
		TargetID:  targetID,
		Data:      data,
		Timestamp: time.Now(),
		CreatedAt: time.Now(),
	}
	return w.RecordEvent(event)
}

// RecordTweetView records a tweet view event.
func (w *AnalyticsWorker) RecordTweetView(userID, tweetID string, ip, userAgent string) error {
	return w.RecordEventType(EventTweetView, userID, tweetID, map[string]interface{}{
		"ip":         ip,
		"user_agent": userAgent,
	})
}

// RecordSearch records a search event.
func (w *AnalyticsWorker) RecordSearch(userID, query string, resultCount int) error {
	return w.RecordEventType(EventSearch, userID, "", map[string]interface{}{
		"query":        query,
		"result_count": resultCount,
	})
}

// RecordLogin records a login event.
func (w *AnalyticsWorker) RecordLogin(userID, ip, userAgent string) error {
	return w.RecordEventType(EventLogin, userID, "", map[string]interface{}{
		"ip":         ip,
		"user_agent": userAgent,
	})
}

// RecordSignup records a signup event.
func (w *AnalyticsWorker) RecordSignup(userID, ip, userAgent string) error {
	return w.RecordEventType(EventSignup, userID, "", map[string]interface{}{
		"ip":         ip,
		"user_agent": userAgent,
	})
}

// RecordBatch records multiple events.
func (w *AnalyticsWorker) RecordBatch(events []*AnalyticsEvent) error {
	if w.isStopped() {
		return ErrAnalyticsWorkerStopped
	}
	for _, event := range events {
		if err := w.validateEvent(event); err != nil {
			return err
		}
	}
	for _, event := range events {
		select {
		case w.queue <- event:
		default:
			return ErrAnalyticsQueueFull
		}
	}
	return nil
}

// ======================================================================
= Queue Management
// ======================================================================

// QueueSize returns the current queue size.
func (w *AnalyticsWorker) QueueSize() int {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	return w.metrics.QueueSize
}

// GetMetrics returns current metrics.
func (w *AnalyticsWorker) GetMetrics() AnalyticsMetrics {
	w.metrics.mu.RLock()
	defer w.metrics.mu.RUnlock()
	return *w.metrics
}

// IsStopped returns true if the worker is stopped.
func (w *AnalyticsWorker) isStopped() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stopped
}

// IsStarted returns true if the worker is started.
func (w *AnalyticsWorker) IsStarted() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.started && !w.stopped
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck performs a health check on the analytics worker.
func (w *AnalyticsWorker) HealthCheck() map[string]interface{} {
	status := map[string]interface{}{
		"component":  "analytics_worker",
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
		"total_received":  metrics.TotalReceived,
		"total_processed": metrics.TotalProcessed,
		"total_failed":    metrics.TotalFailed,
		"active_workers":  metrics.ActiveWorkers,
		"events_by_type":  metrics.EventsByType,
	}
	if metrics.AggregatedStats != nil {
		status["aggregated_stats"] = metrics.AggregatedStats
	}
	return status
}

// ======================================================================
= Cleanup
// ======================================================================

// Cleanup removes old analytics data.
func (w *AnalyticsWorker) Cleanup(ctx context.Context) error {
	retentionDate := time.Now().AddDate(0, 0, -w.config.RetentionDays)
	w.log.WithField("retention_date", retentionDate).Info("Cleaning up old analytics data")
	if err := w.eventRepo.DeleteOlderThan(ctx, retentionDate); err != nil {
		return fmt.Errorf("failed to cleanup events: %w", err)
	}
	if err := w.statsRepo.DeleteOlderThan(ctx, retentionDate); err != nil {
		return fmt.Errorf("failed to cleanup stats: %w", err)
	}
	return nil
}

// ======================================================================
= Helper Functions
// ======================================================================

// generateAnalyticsID generates a unique analytics event ID.
func generateAnalyticsID() string {
	return fmt.Sprintf("analytics_%d_%d", time.Now().UnixNano(), time.Now().UnixNano()%100000)
}

// ======================================================================
= Repository Interfaces (for dependency injection)
// ======================================================================

// EventRepository defines the interface for analytics event storage.
type EventRepository interface {
	BulkCreate(ctx context.Context, events []*AnalyticsEvent) error
	GetByDateRange(ctx context.Context, start, end time.Time, offset, limit int) ([]*AnalyticsEvent, error)
	DeleteOlderThan(ctx context.Context, before time.Time) error
}

// StatsRepository defines the interface for aggregated stats storage.
type StatsRepository interface {
	Create(ctx context.Context, stats *AggregatedStats) error
	GetByDateRange(ctx context.Context, start, end time.Time) ([]*AggregatedStats, error)
	DeleteOlderThan(ctx context.Context, before time.Time) error
}

// ======================================================================
= Global Instance
// ======================================================================

var defaultAnalyticsWorker *AnalyticsWorker
var analyticsWorkerOnce sync.Once

// InitAnalyticsWorker initializes the global analytics worker.
func InitAnalyticsWorker(
	eventRepo EventRepository,
	statsRepo StatsRepository,
	cfg AnalyticsWorkerConfig,
) error {
	var err error
	analyticsWorkerOnce.Do(func() {
		defaultAnalyticsWorker = NewAnalyticsWorker(eventRepo, statsRepo, cfg)
		err = defaultAnalyticsWorker.Start()
	})
	return err
}

// GetAnalyticsWorker returns the global analytics worker.
func GetAnalyticsWorker() *AnalyticsWorker {
	if defaultAnalyticsWorker == nil {
		panic("analytics worker not initialized")
	}
	return defaultAnalyticsWorker
}

// RecordAnalyticsEvent records an event using the global worker.
func RecordAnalyticsEvent(event *AnalyticsEvent) error {
	return GetAnalyticsWorker().RecordEvent(event)
}