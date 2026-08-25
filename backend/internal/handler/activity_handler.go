// backend/internal/worker/analytics_worker.go
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// Event types
	EventTypeTweetCreated   = "tweet_created"
	EventTypeTweetViewed    = "tweet_viewed"
	EventTypeTweetLiked     = "tweet_liked"
	EventTypeTweetRetweeted = "tweet_retweeted"
	EventTypeTweetReplied   = "tweet_replied"
	EventTypeUserFollowed   = "user_followed"
	EventTypeUserUnfollowed = "user_unfollowed"
	EventTypeUserLoggedIn   = "user_logged_in"
	EventTypeUserLoggedOut  = "user_logged_out"
	EventTypeSearchQuery    = "search_query"
	EventTypeProfileViewed  = "profile_viewed"
	EventTypeLinkClicked    = "link_clicked"

	// Aggregation intervals
	AggregationHourly  = "hourly"
	AggregationDaily   = "daily"
	AggregationWeekly  = "weekly"
	AggregationMonthly = "monthly"

	// Redis keys
	RedisKeyEventQueue     = "analytics:events:queue"
	RedisKeyProcessingLock = "analytics:events:lock"
	RedisKeyMetricsPrefix  = "analytics:metrics:"
	RedisKeyReportsPrefix  = "analytics:reports:"
)

// ======================================================================
// AnalyticsWorker
// ======================================================================

// AnalyticsWorker handles processing of analytics events.
type AnalyticsWorker struct {
	redisAdapter  adapter.RedisAdapter
	tweetRepo     interfaces.TweetRepository
	userRepo      interfaces.UserRepository
	followRepo    interfaces.FollowRepository
	likeRepo      interfaces.LikeRepository
	retweetRepo   interfaces.RetweetRepository
	notificationRepo interfaces.NotificationRepository
	log           *logrus.Entry
	mu            sync.RWMutex
	stopCh        chan struct{}
	wg            sync.WaitGroup
	isRunning     bool
	config        *AnalyticsWorkerConfig
	metricsStore  *MetricsStore
}

// AnalyticsWorkerConfig holds configuration for the analytics worker.
type AnalyticsWorkerConfig struct {
	// Processing settings
	BatchSize           int           `json:"batch_size"`
	ProcessingInterval  time.Duration `json:"processing_interval"`
	AggregationInterval time.Duration `json:"aggregation_interval"`
	MaxRetries          int           `json:"max_retries"`
	RetryDelay          time.Duration `json:"retry_delay"`

	// Report generation
	ReportGenerationHour int `json:"report_generation_hour"` // 0-23
	ReportGenerationDay  int `json:"report_generation_day"`  // 0-6 (Sunday=0)

	// Metrics retention
	MetricsRetentionDays int `json:"metrics_retention_days"`

	// Batch processing
	BatchTimeout time.Duration `json:"batch_timeout"`
}

// DefaultAnalyticsWorkerConfig returns sensible defaults.
func DefaultAnalyticsWorkerConfig() *AnalyticsWorkerConfig {
	return &AnalyticsWorkerConfig{
		BatchSize:           100,
		ProcessingInterval:  5 * time.Second,
		AggregationInterval: 1 * time.Minute,
		MaxRetries:          3,
		RetryDelay:          5 * time.Second,
		ReportGenerationHour: 2,
		ReportGenerationDay:  0,
		MetricsRetentionDays: 90,
		BatchTimeout:         30 * time.Second,
	}
}

// MetricsStore holds in-memory metrics.
type MetricsStore struct {
	mu              sync.RWMutex
	TweetCount      int64
	LikeCount       int64
	RetweetCount    int64
	ReplyCount      int64
	FollowCount     int64
	UnfollowCount   int64
	ViewCount       int64
	SearchCount     int64
	ProfileViewCount int64
	LastUpdated     time.Time
}

// NewAnalyticsWorker creates a new analytics worker.
func NewAnalyticsWorker(
	redisAdapter adapter.RedisAdapter,
	tweetRepo interfaces.TweetRepository,
	userRepo interfaces.UserRepository,
	followRepo interfaces.FollowRepository,
	likeRepo interfaces.LikeRepository,
	retweetRepo interfaces.RetweetRepository,
	notificationRepo interfaces.NotificationRepository,
) *AnalyticsWorker {
	return &AnalyticsWorker{
		redisAdapter:     redisAdapter,
		tweetRepo:        tweetRepo,
		userRepo:         userRepo,
		followRepo:       followRepo,
		likeRepo:         likeRepo,
		retweetRepo:      retweetRepo,
		notificationRepo: notificationRepo,
		log:              logger.WithField("worker", "analytics"),
		config:           DefaultAnalyticsWorkerConfig(),
		metricsStore:     &MetricsStore{},
		stopCh:           make(chan struct{}),
	}
}

// ======================================================================
// Worker Lifecycle
// ======================================================================

// Start starts the analytics worker.
func (w *AnalyticsWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.isRunning {
		return nil
	}

	w.isRunning = true
	w.stopCh = make(chan struct{})
	w.log.Info("Starting analytics worker")

	// Start event processing
	w.wg.Add(1)
	go w.processEvents(ctx)

	// Start aggregation
	w.wg.Add(1)
	go w.aggregateMetrics(ctx)

	// Start report generation
	w.wg.Add(1)
	go w.generateReports(ctx)

	// Start cleanup
	w.wg.Add(1)
	go w.cleanupOldMetrics(ctx)

	return nil
}

// Stop stops the analytics worker.
func (w *AnalyticsWorker) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isRunning {
		return nil
	}

	w.log.Info("Stopping analytics worker")
	w.isRunning = false
	close(w.stopCh)
	w.wg.Wait()
	return nil
}

// IsRunning returns whether the worker is running.
func (w *AnalyticsWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// ======================================================================
= Event Processing
// ======================================================================

// processEvents processes events from the queue.
func (w *AnalyticsWorker) processEvents(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.ProcessingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			w.log.Info("Event processor stopped")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

// processBatch processes a batch of events.
func (w *AnalyticsWorker) processBatch(ctx context.Context) {
	// Acquire lock for distributed processing
	if w.redisAdapter != nil {
		locked, _, err := w.redisAdapter.Lock(ctx, RedisKeyProcessingLock, 30*time.Second)
		if err != nil || !locked {
			return
		}
		defer func() {
			_ = w.redisAdapter.Unlock(ctx, RedisKeyProcessingLock, "")
		}()
	}

	// Get events from queue
	events, err := w.getEvents(ctx, w.config.BatchSize)
	if err != nil {
		w.log.WithError(err).Error("Failed to get events from queue")
		return
	}

	if len(events) == 0 {
		return
	}

	w.log.WithField("count", len(events)).Debug("Processing events")

	// Process events
	for _, event := range events {
		if err := w.processEvent(ctx, event); err != nil {
			w.log.WithError(err).WithField("event_id", event.ID).Error("Failed to process event")
			// Requeue for retry
			if event.RetryCount < w.config.MaxRetries {
				event.RetryCount++
				_ = w.requeueEvent(ctx, event)
			}
		}
	}

	// Remove processed events from queue
	_ = w.removeEvents(ctx, events)
}

// getEvents gets events from the queue.
func (w *AnalyticsWorker) getEvents(ctx context.Context, limit int) ([]*AnalyticsEvent, error) {
	if w.redisAdapter == nil {
		return nil, nil
	}

	results, err := w.redisAdapter.LRange(ctx, RedisKeyEventQueue, 0, int64(limit-1))
	if err != nil {
		return nil, err
	}

	var events []*AnalyticsEvent
	for _, data := range results {
		var event AnalyticsEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			w.log.WithError(err).Warn("Failed to unmarshal event")
			continue
		}
		events = append(events, &event)
	}
	return events, nil
}

// removeEvents removes events from the queue.
func (w *AnalyticsWorker) removeEvents(ctx context.Context, events []*AnalyticsEvent) error {
	if w.redisAdapter == nil || len(events) == 0 {
		return nil
	}

	// Remove by count
	_, err := w.redisAdapter.LPop(ctx, RedisKeyEventQueue)
	return err
}

// requeueEvent requeues an event for retry.
func (w *AnalyticsWorker) requeueEvent(ctx context.Context, event *AnalyticsEvent) error {
	if w.redisAdapter == nil {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return w.redisAdapter.RPush(ctx, RedisKeyEventQueue, string(data))
}

// ======================================================================
= Event Processing Logic
// ======================================================================

// processEvent processes a single event.
func (w *AnalyticsWorker) processEvent(ctx context.Context, event *AnalyticsEvent) error {
	switch event.Type {
	case EventTypeTweetCreated:
		return w.processTweetCreated(ctx, event)
	case EventTypeTweetViewed:
		return w.processTweetViewed(ctx, event)
	case EventTypeTweetLiked:
		return w.processTweetLiked(ctx, event)
	case EventTypeTweetRetweeted:
		return w.processTweetRetweeted(ctx, event)
	case EventTypeTweetReplied:
		return w.processTweetReplied(ctx, event)
	case EventTypeUserFollowed:
		return w.processUserFollowed(ctx, event)
	case EventTypeUserUnfollowed:
		return w.processUserUnfollowed(ctx, event)
	case EventTypeUserLoggedIn:
		return w.processUserLoggedIn(ctx, event)
	case EventTypeUserLoggedOut:
		return w.processUserLoggedOut(ctx, event)
	case EventTypeSearchQuery:
		return w.processSearchQuery(ctx, event)
	case EventTypeProfileViewed:
		return w.processProfileViewed(ctx, event)
	case EventTypeLinkClicked:
		return w.processLinkClicked(ctx, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
}

// processTweetCreated processes a tweet created event.
func (w *AnalyticsWorker) processTweetCreated(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.TweetCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	// Update daily tweet count
	_ = w.updateDailyMetric(ctx, "tweets", event.Timestamp)
	return nil
}

// processTweetViewed processes a tweet viewed event.
func (w *AnalyticsWorker) processTweetViewed(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.ViewCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	// Update tweet view count in repository
	if event.TweetID != "" {
		_ = w.tweetRepo.IncrementViewCount(ctx, event.TweetID)
	}
	_ = w.updateDailyMetric(ctx, "views", event.Timestamp)
	return nil
}

// processTweetLiked processes a tweet liked event.
func (w *AnalyticsWorker) processTweetLiked(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.LikeCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	_ = w.updateDailyMetric(ctx, "likes", event.Timestamp)
	return nil
}

// processTweetRetweeted processes a tweet retweeted event.
func (w *AnalyticsWorker) processTweetRetweeted(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.RetweetCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	_ = w.updateDailyMetric(ctx, "retweets", event.Timestamp)
	return nil
}

// processTweetReplied processes a tweet replied event.
func (w *AnalyticsWorker) processTweetReplied(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.ReplyCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	_ = w.updateDailyMetric(ctx, "replies", event.Timestamp)
	return nil
}

// processUserFollowed processes a user followed event.
func (w *AnalyticsWorker) processUserFollowed(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.FollowCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	_ = w.updateDailyMetric(ctx, "follows", event.Timestamp)
	return nil
}

// processUserUnfollowed processes a user unfollowed event.
func (w *AnalyticsWorker) processUserUnfollowed(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.UnfollowCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	_ = w.updateDailyMetric(ctx, "unfollows", event.Timestamp)
	return nil
}

// processUserLoggedIn processes a user logged in event.
func (w *AnalyticsWorker) processUserLoggedIn(ctx context.Context, event *AnalyticsEvent) error {
	_ = w.updateDailyMetric(ctx, "logins", event.Timestamp)
	return nil
}

// processUserLoggedOut processes a user logged out event.
func (w *AnalyticsWorker) processUserLoggedOut(ctx context.Context, event *AnalyticsEvent) error {
	_ = w.updateDailyMetric(ctx, "logouts", event.Timestamp)
	return nil
}

// processSearchQuery processes a search query event.
func (w *AnalyticsWorker) processSearchQuery(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.SearchCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	_ = w.updateDailyMetric(ctx, "searches", event.Timestamp)

	// Record popular search terms
	if event.Metadata != nil {
		if query, ok := event.Metadata["query"].(string); ok && query != "" {
			_ = w.recordSearchTerm(ctx, query, event.Timestamp)
		}
	}
	return nil
}

// processProfileViewed processes a profile viewed event.
func (w *AnalyticsWorker) processProfileViewed(ctx context.Context, event *AnalyticsEvent) error {
	w.metricsStore.mu.Lock()
	w.metricsStore.ProfileViewCount++
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()

	_ = w.updateDailyMetric(ctx, "profile_views", event.Timestamp)
	return nil
}

// processLinkClicked processes a link clicked event.
func (w *AnalyticsWorker) processLinkClicked(ctx context.Context, event *AnalyticsEvent) error {
	_ = w.updateDailyMetric(ctx, "link_clicks", event.Timestamp)
	return nil
}

// ======================================================================
= Metric Aggregation
// ======================================================================

// aggregateMetrics aggregates metrics periodically.
func (w *AnalyticsWorker) aggregateMetrics(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.AggregationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			w.log.Info("Metric aggregator stopped")
			return
		case <-ticker.C:
			w.doAggregation(ctx)
		}
	}
}

// doAggregation performs metric aggregation.
func (w *AnalyticsWorker) doAggregation(ctx context.Context) {
	if w.redisAdapter == nil {
		return
	}

	now := time.Now()
	hourKey := fmt.Sprintf("%shourly:%s", RedisKeyMetricsPrefix, now.Format("2006-01-02-15"))
	dayKey := fmt.Sprintf("%sdaily:%s", RedisKeyMetricsPrefix, now.Format("2006-01-02"))
	weekKey := fmt.Sprintf("%sweekly:%s", RedisKeyMetricsPrefix, now.Format("2006-W02"))
	monthKey := fmt.Sprintf("%smonthly:%s", RedisKeyMetricsPrefix, now.Format("2006-01"))

	w.metricsStore.mu.RLock()
	metrics := map[string]interface{}{
		"tweets":         w.metricsStore.TweetCount,
		"likes":          w.metricsStore.LikeCount,
		"retweets":       w.metricsStore.RetweetCount,
		"replies":        w.metricsStore.ReplyCount,
		"follows":        w.metricsStore.FollowCount,
		"unfollows":      w.metricsStore.UnfollowCount,
		"views":          w.metricsStore.ViewCount,
		"searches":       w.metricsStore.SearchCount,
		"profile_views":  w.metricsStore.ProfileViewCount,
	}
	w.metricsStore.mu.RUnlock()

	// Store hourly metrics
	_ = w.redisAdapter.HSet(ctx, hourKey, metrics)
	_ = w.redisAdapter.Expire(ctx, hourKey, 48*time.Hour)

	// Increment daily metrics
	_ = w.redisAdapter.HIncrBy(ctx, dayKey, "tweets", metrics["tweets"].(int64))
	_ = w.redisAdapter.HIncrBy(ctx, dayKey, "likes", metrics["likes"].(int64))
	_ = w.redisAdapter.HIncrBy(ctx, dayKey, "retweets", metrics["retweets"].(int64))
	_ = w.redisAdapter.HIncrBy(ctx, dayKey, "replies", metrics["replies"].(int64))
	_ = w.redisAdapter.HIncrBy(ctx, dayKey, "follows", metrics["follows"].(int64))
	_ = w.redisAdapter.HIncrBy(ctx, dayKey, "views", metrics["views"].(int64))
	_ = w.redisAdapter.Expire(ctx, dayKey, 30*24*time.Hour)

	// Reset counters
	w.metricsStore.mu.Lock()
	w.metricsStore.TweetCount = 0
	w.metricsStore.LikeCount = 0
	w.metricsStore.RetweetCount = 0
	w.metricsStore.ReplyCount = 0
	w.metricsStore.FollowCount = 0
	w.metricsStore.UnfollowCount = 0
	w.metricsStore.ViewCount = 0
	w.metricsStore.SearchCount = 0
	w.metricsStore.ProfileViewCount = 0
	w.metricsStore.LastUpdated = time.Now()
	w.metricsStore.mu.Unlock()
}

// updateDailyMetric updates a daily metric.
func (w *AnalyticsWorker) updateDailyMetric(ctx context.Context, key string, timestamp time.Time) error {
	if w.redisAdapter == nil {
		return nil
	}
	dayKey := fmt.Sprintf("%sdaily:%s", RedisKeyMetricsPrefix, timestamp.Format("2006-01-02"))
	return w.redisAdapter.HIncrBy(ctx, dayKey, key, 1)
}

// recordSearchTerm records a search term.
func (w *AnalyticsWorker) recordSearchTerm(ctx context.Context, query string, timestamp time.Time) error {
	if w.redisAdapter == nil {
		return nil
	}
	key := fmt.Sprintf("%ssearches:terms", RedisKeyMetricsPrefix)
	return w.redisAdapter.ZIncrBy(ctx, key, 1, query)
}

// ======================================================================
= Report Generation
// ======================================================================

// generateReports generates analytics reports periodically.
func (w *AnalyticsWorker) generateReports(ctx context.Context) {
	defer w.wg.Done()

	// Generate daily report at configured hour
	// For now, we run every 6 hours
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			w.log.Info("Report generator stopped")
			return
		case <-ticker.C:
			w.generateDailyReport(ctx)
			w.generateWeeklyReport(ctx)
			w.generateMonthlyReport(ctx)
		}
	}
}

// generateDailyReport generates a daily report.
func (w *AnalyticsWorker) generateDailyReport(ctx context.Context) {
	date := time.Now().AddDate(0, 0, -1)
	reportKey := fmt.Sprintf("%sdaily_report:%s", RedisKeyReportsPrefix, date.Format("2006-01-02"))

	// Get daily metrics
	metrics, err := w.getDailyMetrics(ctx, date)
	if err != nil {
		w.log.WithError(err).Error("Failed to get daily metrics")
		return
	}

	report := &AnalyticsReport{
		ID:          uuid.New().String(),
		Type:        "daily",
		Date:        date,
		GeneratedAt: time.Now(),
		Metrics:     metrics,
	}

	data, err := json.Marshal(report)
	if err != nil {
		w.log.WithError(err).Error("Failed to marshal report")
		return
	}

	if w.redisAdapter != nil {
		_ = w.redisAdapter.Set(ctx, reportKey, string(data), 30*24*time.Hour)
	}

	w.log.WithField("date", date.Format("2006-01-02")).Info("Generated daily report")
}

// generateWeeklyReport generates a weekly report.
func (w *AnalyticsWorker) generateWeeklyReport(ctx context.Context) {
	date := time.Now().AddDate(0, 0, -7)
	reportKey := fmt.Sprintf("%sweekly_report:%s", RedisKeyReportsPrefix, date.Format("2006-W02"))

	// Get weekly metrics (sum of daily metrics)
	metrics := make(map[string]int64)
	for i := 0; i < 7; i++ {
		day := date.AddDate(0, 0, i)
		dayMetrics, err := w.getDailyMetrics(ctx, day)
		if err != nil {
			continue
		}
		for k, v := range dayMetrics {
			metrics[k] += v
		}
	}

	report := &AnalyticsReport{
		ID:          uuid.New().String(),
		Type:        "weekly",
		Date:        date,
		GeneratedAt: time.Now(),
		Metrics:     metrics,
	}

	data, err := json.Marshal(report)
	if err != nil {
		w.log.WithError(err).Error("Failed to marshal report")
		return
	}

	if w.redisAdapter != nil {
		_ = w.redisAdapter.Set(ctx, reportKey, string(data), 30*24*time.Hour)
	}

	w.log.WithField("week", date.Format("2006-W02")).Info("Generated weekly report")
}

// generateMonthlyReport generates a monthly report.
func (w *AnalyticsWorker) generateMonthlyReport(ctx context.Context) {
	date := time.Now().AddDate(0, -1, 0)
	reportKey := fmt.Sprintf("%smonthly_report:%s", RedisKeyReportsPrefix, date.Format("2006-01"))

	// Get monthly metrics
	metrics := make(map[string]int64)
	days := daysInMonth(date)
	for i := 0; i < days; i++ {
		day := time.Date(date.Year(), date.Month(), i+1, 0, 0, 0, 0, date.Location())
		dayMetrics, err := w.getDailyMetrics(ctx, day)
		if err != nil {
			continue
		}
		for k, v := range dayMetrics {
			metrics[k] += v
		}
	}

	report := &AnalyticsReport{
		ID:          uuid.New().String(),
		Type:        "monthly",
		Date:        date,
		GeneratedAt: time.Now(),
		Metrics:     metrics,
	}

	data, err := json.Marshal(report)
	if err != nil {
		w.log.WithError(err).Error("Failed to marshal report")
		return
	}

	if w.redisAdapter != nil {
		_ = w.redisAdapter.Set(ctx, reportKey, string(data), 30*24*time.Hour)
	}

	w.log.WithField("month", date.Format("2006-01")).Info("Generated monthly report")
}

// getDailyMetrics gets daily metrics from Redis.
func (w *AnalyticsWorker) getDailyMetrics(ctx context.Context, date time.Time) (map[string]int64, error) {
	if w.redisAdapter == nil {
		return map[string]int64{}, nil
	}
	key := fmt.Sprintf("%sdaily:%s", RedisKeyMetricsPrefix, date.Format("2006-01-02"))
	return w.redisAdapter.HGetAll(ctx, key)
}

// daysInMonth returns the number of days in a month.
func daysInMonth(date time.Time) int {
	nextMonth := time.Date(date.Year(), date.Month()+1, 1, 0, 0, 0, 0, date.Location())
	return nextMonth.AddDate(0, 0, -1).Day()
}

// ======================================================================
= Cleanup
// ======================================================================

// cleanupOldMetrics cleans up old metrics data.
func (w *AnalyticsWorker) cleanupOldMetrics(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			w.log.Info("Cleanup stopped")
			return
		case <-ticker.C:
			w.doCleanup(ctx)
		}
	}
}

// doCleanup performs cleanup of old metrics.
func (w *AnalyticsWorker) doCleanup(ctx context.Context) {
	if w.redisAdapter == nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -w.config.MetricsRetentionDays)

	// Scan and delete old keys
	pattern := fmt.Sprintf("%sdaily:*", RedisKeyMetricsPrefix)
	iter := w.redisAdapter.Scan(ctx, 0, pattern, 100)

	for {
		keys, nextCursor, err := iter.Next()
		if err != nil {
			break
		}
		for _, key := range keys {
			// Parse date from key
			dateStr := strings.TrimPrefix(key, fmt.Sprintf("%sdaily:", RedisKeyMetricsPrefix))
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				if t.Before(cutoff) {
					_ = w.redisAdapter.Delete(ctx, key)
				}
			}
		}
		if nextCursor == 0 {
			break
		}
	}
}

// ======================================================================
= Event Publishing
// ======================================================================

// PublishEvent publishes an event to the analytics queue.
func (w *AnalyticsWorker) PublishEvent(ctx context.Context, eventType string, userID string, tweetID string, metadata map[string]interface{}) error {
	if w.redisAdapter == nil {
		return nil
	}

	event := &AnalyticsEvent{
		ID:         uuid.New().String(),
		Type:       eventType,
		UserID:     userID,
		TweetID:    tweetID,
		Timestamp:  time.Now(),
		Metadata:   metadata,
		RetryCount: 0,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return w.redisAdapter.RPush(ctx, RedisKeyEventQueue, string(data))
}

// ======================================================================
= Types
// ======================================================================

// AnalyticsEvent represents an analytics event.
type AnalyticsEvent struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	UserID     string                 `json:"user_id"`
	TweetID    string                 `json:"tweet_id"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata"`
	RetryCount int                    `json:"retry_count"`
}

// AnalyticsReport represents an analytics report.
type AnalyticsReport struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Date        time.Time         `json:"date"`
	GeneratedAt time.Time         `json:"generated_at"`
	Metrics     map[string]int64  `json:"metrics"`
}