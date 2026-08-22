// backend/internal/middleware/ratelimit.go
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	// Default limits (requests per minute)
	DefaultGlobalLimit   = 100
	DefaultUserLimit     = 200
	DefaultIPLimit       = 50
	DefaultWebSocketLimit = 30

	// Header names
	HeaderRateLimitLimit     = "X-RateLimit-Limit"
	HeaderRateLimitRemaining = "X-RateLimit-Remaining"
	HeaderRateLimitReset     = "X-RateLimit-Reset"
	HeaderRetryAfter         = "Retry-After"
)

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrRedisUnavailable  = errors.New("redis unavailable")
)

// ======================================================================
// Algorithms
// ======================================================================

type RateLimitAlgorithm string

const (
	AlgorithmTokenBucket    RateLimitAlgorithm = "token_bucket"
	AlgorithmSlidingWindow  RateLimitAlgorithm = "sliding_window"
	AlgorithmFixedWindow    RateLimitAlgorithm = "fixed_window"
)

// ======================================================================
= Configuration
// ======================================================================

// RateLimitConfig holds all rate‑limiting configuration.
type RateLimitConfig struct {
	// Global defaults
	DefaultLimit   int           `json:"default_limit"`
	DefaultWindow  time.Duration `json:"default_window"`
	Algorithm      RateLimitAlgorithm `json:"algorithm"`
	
	// Per‑type overrides
	IPLimit        int           `json:"ip_limit"`
	UserLimit      int           `json:"user_limit"`
	WebSocketLimit int           `json:"websocket_limit"`
	
	// Redis settings
	RedisAdapter   adapter.RedisAdapter `json:"-"`
	UseRedis       bool          `json:"use_redis"`
	RedisPrefix    string        `json:"redis_prefix"`
	
	// Whitelist/blacklist
	WhitelistIPs   []string      `json:"whitelist_ips"`
	BlacklistIPs   []string      `json:"blacklist_ips"`
	WhitelistUserIDs []string    `json:"whitelist_user_ids"`
	
	// Exempted paths
	ExemptPaths    []string      `json:"exempt_paths"`
	ExemptMethods  []string      `json:"exempt_methods"`
	
	// Failure behavior when Redis fails
	FallbackToLocal bool         `json:"fallback_to_local"`
	
	// Cleanup interval for local store
	CleanupInterval time.Duration `json:"cleanup_interval"`
	
	// Metrics
	EnableMetrics  bool          `json:"enable_metrics"`
	MetricsPrefix  string        `json:"metrics_prefix"`
}

// DefaultRateLimitConfig returns sensible defaults.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		DefaultLimit:    DefaultGlobalLimit,
		DefaultWindow:   1 * time.Minute,
		Algorithm:       AlgorithmTokenBucket,
		IPLimit:         DefaultIPLimit,
		UserLimit:       DefaultUserLimit,
		WebSocketLimit:  DefaultWebSocketLimit,
		UseRedis:        false,
		RedisPrefix:     "ratelimit:",
		FallbackToLocal: true,
		CleanupInterval: 10 * time.Minute,
		EnableMetrics:   false,
	}
}

// ======================================================================
// Rate Limiter Interface
// ======================================================================

// RateLimiter defines the interface for rate limiting implementations.
type RateLimiter interface {
	// Allow checks if a request is allowed for the given identifier.
	// Returns (allowed, remaining, resetTime, error).
	Allow(ctx context.Context, identifier string, limit int, window time.Duration) (bool, int, time.Time, error)
	
	// Close cleans up resources.
	Close() error
}

// ======================================================================
= Token Bucket Implementation (Local)
// ======================================================================

type tokenBucket struct {
	mu      sync.RWMutex
	buckets map[string]*bucketState
	config  RateLimitConfig
	log     *logrus.Entry
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

type bucketState struct {
	tokens      float64
	lastRefill  time.Time
	limit       int
	window      time.Duration
}

// NewLocalTokenBucket creates a token‑bucket limiter (in‑memory).
func NewLocalTokenBucket(cfg RateLimitConfig) RateLimiter {
	tb := &tokenBucket{
		buckets: make(map[string]*bucketState),
		config:  cfg,
		log:     logger.WithField("limiter", "token_bucket"),
		stopCh:  make(chan struct{}),
	}
	// Start cleanup goroutine
	if cfg.CleanupInterval > 0 {
		tb.wg.Add(1)
		go tb.cleanup()
	}
	return tb
}

func (tb *tokenBucket) Allow(ctx context.Context, identifier string, limit int, window time.Duration) (bool, int, time.Time, error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	state, exists := tb.buckets[identifier]
	now := time.Now()
	
	if !exists {
		// Create new bucket
		state = &bucketState{
			tokens:     float64(limit),
			lastRefill: now,
			limit:      limit,
			window:     window,
		}
		tb.buckets[identifier] = state
	} else {
		// Refill based on elapsed time
		elapsed := now.Sub(state.lastRefill)
		if elapsed > 0 {
			refillRate := float64(state.limit) / state.window.Seconds()
			state.tokens += elapsed.Seconds() * refillRate
			if state.tokens > float64(state.limit) {
				state.tokens = float64(state.limit)
			}
			state.lastRefill = now
		}
	}
	
	// Check if enough tokens
	if state.tokens >= 1.0 {
		state.tokens -= 1.0
		remaining := int(state.tokens)
		// Reset time
		resetTime := state.lastRefill.Add(state.window)
		tb.log.WithFields(logrus.Fields{
			"identifier": identifier,
			"remaining":  remaining,
			"limit":      limit,
		}).Debug("Request allowed")
		return true, remaining, resetTime, nil
	}
	
	// Not enough tokens
	resetTime := state.lastRefill.Add(state.window)
	tb.log.WithFields(logrus.Fields{
		"identifier": identifier,
		"limit":      limit,
	}).Warn("Rate limit exceeded")
	return false, 0, resetTime, nil
}

// cleanup removes expired buckets to prevent memory leak.
func (tb *tokenBucket) cleanup() {
	defer tb.wg.Done()
	ticker := time.NewTicker(tb.config.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-tb.stopCh:
			return
		case <-ticker.C:
			tb.mu.Lock()
			now := time.Now()
			for key, state := range tb.buckets {
				// Remove if the bucket hasn't been used for > 2 windows
				if now.Sub(state.lastRefill) > 2*state.window {
					delete(tb.buckets, key)
				}
			}
			tb.mu.Unlock()
			tb.log.Debug("Cleaned up expired buckets")
		}
	}
}

func (tb *tokenBucket) Close() error {
	close(tb.stopCh)
	tb.wg.Wait()
	return nil
}

// ======================================================================
= Sliding Window with Redis
// ======================================================================

type redisSlidingWindow struct {
	redis  adapter.RedisAdapter
	prefix string
	log    *logrus.Entry
	cfg    RateLimitConfig
}

// NewRedisSlidingWindow creates a sliding‑window counter limiter using Redis.
func NewRedisSlidingWindow(redis adapter.RedisAdapter, prefix string, cfg RateLimitConfig) RateLimiter {
	return &redisSlidingWindow{
		redis:  redis,
		prefix: prefix,
		log:    logger.WithField("limiter", "redis_sliding_window"),
		cfg:    cfg,
	}
}

func (rw *redisSlidingWindow) Allow(ctx context.Context, identifier string, limit int, window time.Duration) (bool, int, time.Time, error) {
	key := rw.prefix + identifier
	now := time.Now().UnixMilli()
	windowMs := window.Milliseconds()
	
	// Lua script for sliding window
	script := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		-- Remove old entries
		redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
		
		-- Get current count
		local count = redis.call('ZCARD', key)
		
		if count < limit then
			-- Add current request
			redis.call('ZADD', key, now, now)
			redis.call('EXPIRE', key, math.ceil(window/1000) + 1)
			return {count + 1, limit - (count + 1)}
		else
			-- Get oldest timestamp to compute reset time
			local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
			local reset_time = 0
			if #oldest > 0 then
				reset_time = tonumber(oldest[2]) + window
			end
			return {count, 0, reset_time}
		end
	`
	
	result, err := rw.redis.Eval(ctx, script, []string{key}, limit, windowMs, now)
	if err != nil {
		rw.log.WithError(err).Warn("Redis sliding window failed, falling back to local")
		if rw.cfg.FallbackToLocal {
			// Fallback to local token bucket
			local := NewLocalTokenBucket(rw.cfg)
			return local.Allow(ctx, identifier, limit, window)
		}
		return false, 0, time.Time{}, err
	}
	
	values, ok := result.([]interface{})
	if !ok || len(values) < 2 {
		return false, 0, time.Time{}, errors.New("unexpected result format")
	}
	
	allowed := values[0].(int64) > 0
	remaining := values[1].(int64)
	if remaining < 0 {
		remaining = 0
	}
	resetTime := time.Unix(0, 0)
	if len(values) > 2 {
		if resetMs, ok := values[2].(int64); ok && resetMs > 0 {
			resetTime = time.UnixMilli(resetMs)
		}
	}
	
	return allowed, int(remaining), resetTime, nil
}

func (rw *redisSlidingWindow) Close() error {
	return nil
}

// ======================================================================
= Fixed Window with Redis
// ======================================================================

type redisFixedWindow struct {
	redis  adapter.RedisAdapter
	prefix string
	log    *logrus.Entry
	cfg    RateLimitConfig
}

// NewRedisFixedWindow creates a fixed‑window counter limiter using Redis.
func NewRedisFixedWindow(redis adapter.RedisAdapter, prefix string, cfg RateLimitConfig) RateLimiter {
	return &redisFixedWindow{
		redis:  redis,
		prefix: prefix,
		log:    logger.WithField("limiter", "redis_fixed_window"),
		cfg:    cfg,
	}
}

func (fw *redisFixedWindow) Allow(ctx context.Context, identifier string, limit int, window time.Duration) (bool, int, time.Time, error) {
	key := fw.prefix + identifier
	now := time.Now()
	windowSec := int64(window.Seconds())
	// Current window key (by timestamp)
	windowKey := fmt.Sprintf("%s:%d", key, now.Unix()/windowSec)
	
	// Use atomic INCR and EXPIRE
	count, err := fw.redis.Incr(ctx, windowKey)
	if err != nil {
		fw.log.WithError(err).Warn("Redis incr failed, falling back")
		if fw.cfg.FallbackToLocal {
			local := NewLocalTokenBucket(fw.cfg)
			return local.Allow(ctx, identifier, limit, window)
		}
		return false, 0, time.Time{}, err
	}
	
	if count == 1 {
		// Set expiry
		_ = fw.redis.Expire(ctx, windowKey, window)
	}
	
	if count <= int64(limit) {
		remaining := int(limit) - int(count)
		resetTime := time.Unix(now.Unix()/windowSec*windowSec+windowSec, 0)
		return true, remaining, resetTime, nil
	}
	
	// Exceeded
	// Get TTL for reset header
	ttl, _ := fw.redis.TTL(ctx, windowKey)
	resetTime := time.Now().Add(ttl)
	return false, 0, resetTime, nil
}

func (fw *redisFixedWindow) Close() error {
	return nil
}

// ======================================================================
= Middleware Factory
// ======================================================================

// RateLimitMiddleware is the main HTTP middleware.
type RateLimitMiddleware struct {
	limiter    RateLimiter
	config     RateLimitConfig
	log        *logrus.Entry
	mu         sync.RWMutex
	exemptMap  map[string]bool // path -> exempt (compiled)
	exemptMethods map[string]bool
}

// NewRateLimitMiddleware creates a new rate‑limiting middleware.
func NewRateLimitMiddleware(cfg RateLimitConfig) (*RateLimitMiddleware, error) {
	var limiter RateLimiter
	
	if cfg.UseRedis && cfg.RedisAdapter != nil {
		switch cfg.Algorithm {
		case AlgorithmSlidingWindow:
			limiter = NewRedisSlidingWindow(cfg.RedisAdapter, cfg.RedisPrefix, cfg)
		case AlgorithmFixedWindow:
			limiter = NewRedisFixedWindow(cfg.RedisAdapter, cfg.RedisPrefix, cfg)
		default: // token bucket also works with Redis? We'll use sliding.
			limiter = NewRedisSlidingWindow(cfg.RedisAdapter, cfg.RedisPrefix, cfg)
		}
	} else {
		limiter = NewLocalTokenBucket(cfg)
	}
	
	// Build exempt maps
	exemptMap := make(map[string]bool)
	for _, path := range cfg.ExemptPaths {
		exemptMap[path] = true
	}
	exemptMethods := make(map[string]bool)
	for _, method := range cfg.ExemptMethods {
		exemptMethods[strings.ToUpper(method)] = true
	}
	
	return &RateLimitMiddleware{
		limiter:    limiter,
		config:     cfg,
		log:        logger.WithField("middleware", "rate_limit"),
		exemptMap:  exemptMap,
		exemptMethods: exemptMethods,
	}, nil
}

// Middleware returns the HTTP middleware handler.
func (rl *RateLimitMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Check exemption by path
		path := r.URL.Path
		if rl.exemptMap[path] {
			rl.log.WithField("path", path).Debug("Path exempt from rate limiting")
			next.ServeHTTP(w, r)
			return
		}
		
		// Check exemption by method
		if rl.exemptMethods[r.Method] {
			rl.log.WithField("method", r.Method).Debug("Method exempt from rate limiting")
			next.ServeHTTP(w, r)
			return
		}
		
		// Determine identifier
		identifier, limit, window := rl.getLimits(r)
		
		// Check whitelist
		if rl.isWhitelisted(identifier) {
			rl.log.WithField("identifier", identifier).Debug("Identifier whitelisted")
			next.ServeHTTP(w, r)
			return
		}
		
		// Check blacklist
		if rl.isBlacklisted(identifier) {
			rl.log.WithField("identifier", identifier).Warn("Identifier blacklisted")
			rl.sendError(w, http.StatusForbidden, "Access denied")
			return
		}
		
		// Perform rate limit check
		allowed, remaining, resetTime, err := rl.limiter.Allow(ctx, identifier, limit, window)
		if err != nil {
			rl.log.WithError(err).Error("Rate limiter failed")
			// On error, we may allow or deny. We'll allow to avoid blocking due to internal error.
			next.ServeHTTP(w, r)
			return
		}
		
		// Set headers
		w.Header().Set(HeaderRateLimitLimit, strconv.Itoa(limit))
		w.Header().Set(HeaderRateLimitRemaining, strconv.Itoa(remaining))
		if !resetTime.IsZero() {
			w.Header().Set(HeaderRateLimitReset, strconv.FormatInt(resetTime.Unix(), 10))
		}
		
		if !allowed {
			// Set Retry-After
			if !resetTime.IsZero() {
				retryAfter := int(time.Until(resetTime).Seconds())
				if retryAfter < 0 {
					retryAfter = 1
				}
				w.Header().Set(HeaderRetryAfter, strconv.Itoa(retryAfter))
			}
			rl.sendError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// ======================================================================
= Identifier and Limit Resolution
// ======================================================================

// getLimits determines the appropriate identifier, limit, and window.
func (rl *RateLimitMiddleware) getLimits(r *http.Request) (identifier string, limit int, window time.Duration) {
	// Defaults
	identifier = ""
	limit = rl.config.DefaultLimit
	window = rl.config.DefaultWindow
	
	// Try to get user ID from context (if authenticated)
	userID, err := GetUserID(r.Context())
	if err == nil && userID != "" {
		identifier = "user:" + userID
		if rl.config.UserLimit > 0 {
			limit = rl.config.UserLimit
		}
		return
	}
	
	// Fallback to IP
	ip := r.RemoteAddr
	// Handle X-Forwarded-For
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			ip = strings.TrimSpace(ips[0])
		}
	}
	identifier = "ip:" + ip
	if rl.config.IPLimit > 0 {
		limit = rl.config.IPLimit
	}
	return
}

// isWhitelisted checks if the identifier is in the whitelist.
func (rl *RateLimitMiddleware) isWhitelisted(identifier string) bool {
	if identifier == "" {
		return false
	}
	// Check if it's a user whitelist
	if strings.HasPrefix(identifier, "user:") {
		userID := strings.TrimPrefix(identifier, "user:")
		for _, w := range rl.config.WhitelistUserIDs {
			if w == userID {
				return true
			}
		}
	}
	// IP whitelist
	if strings.HasPrefix(identifier, "ip:") {
		ip := strings.TrimPrefix(identifier, "ip:")
		for _, w := range rl.config.WhitelistIPs {
			if w == ip {
				return true
			}
		}
	}
	return false
}

// isBlacklisted checks if the identifier is in the blacklist.
func (rl *RateLimitMiddleware) isBlacklisted(identifier string) bool {
	if identifier == "" {
		return false
	}
	if strings.HasPrefix(identifier, "ip:") {
		ip := strings.TrimPrefix(identifier, "ip:")
		for _, b := range rl.config.BlacklistIPs {
			if b == ip {
				return true
			}
		}
	}
	return false
}

// ======================================================================
= Response Helpers
// ======================================================================

func (rl *RateLimitMiddleware) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"error":   message,
		"status":  status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(resp)
}

// ======================================================================
= WebSocket Rate Limiting
// ======================================================================

// RateLimitWebSocket checks rate limit for a WebSocket connection.
func (rl *RateLimitMiddleware) RateLimitWebSocket(ctx context.Context, identifier string) (bool, int, time.Time, error) {
	limit := rl.config.WebSocketLimit
	if limit == 0 {
		limit = DefaultWebSocketLimit
	}
	window := rl.config.DefaultWindow
	return rl.limiter.Allow(ctx, identifier, limit, window)
}

// ======================================================================
= Reset / Cleanup
// ======================================================================

// Reset clears all rate limit data (useful for testing).
func (rl *RateLimitMiddleware) Reset() {
	if closer, ok := rl.limiter.(interface{ Reset() }); ok {
		closer.Reset()
	}
}

// Close releases resources.
func (rl *RateLimitMiddleware) Close() error {
	return rl.limiter.Close()
}

// ======================================================================
= Prometheus Metrics Integration (optional)
// ======================================================================

// MetricsCollector can be injected to record metrics.
type MetricsCollector interface {
	IncRateLimited(identifier string)
	IncAllowed(identifier string)
	ObserveRemaining(identifier string, remaining int)
}

var metricCollector MetricsCollector

// SetMetricsCollector sets the global metrics collector.
func SetMetricsCollector(mc MetricsCollector) {
	metricCollector = mc
}

// ======================================================================
= Middleware Convenience Functions
// ======================================================================

// RateLimit is a convenience function to create middleware with defaults.
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	mw, err := NewRateLimitMiddleware(cfg)
	if err != nil {
		panic(err)
	}
	return mw.Middleware
}

// Global rate limit instance (for simplicity).
var defaultMiddleware *RateLimitMiddleware

// InitDefaultRateLimit initializes the default rate limiter.
func InitDefaultRateLimit(cfg RateLimitConfig) error {
	mw, err := NewRateLimitMiddleware(cfg)
	if err != nil {
		return err
	}
	defaultMiddleware = mw
	return nil
}

// DefaultMiddleware returns the global rate limiter.
func DefaultMiddleware() *RateLimitMiddleware {
	if defaultMiddleware == nil {
		panic("rate limit middleware not initialized")
	}
	return defaultMiddleware
}

// ======================================================================
= Test Helpers
// ======================================================================

// MockRateLimiter implements RateLimiter for testing.
type MockRateLimiter struct {
	AllowFunc func(ctx context.Context, identifier string, limit int, window time.Duration) (bool, int, time.Time, error)
}

func (m *MockRateLimiter) Allow(ctx context.Context, identifier string, limit int, window time.Duration) (bool, int, time.Time, error) {
	if m.AllowFunc != nil {
		return m.AllowFunc(ctx, identifier, limit, window)
	}
	return true, limit, time.Now().Add(window), nil
}

func (m *MockRateLimiter) Close() error { return nil }

// Reset for token bucket (implemented by tokenBucket)
func (tb *tokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.buckets = make(map[string]*bucketState)
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck checks the health of the rate limiter dependencies.
func (rl *RateLimitMiddleware) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"component": "rate_limit_middleware",
		"status":    "ok",
		"algorithm": rl.config.Algorithm,
		"use_redis": rl.config.UseRedis,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if rl.config.UseRedis && rl.config.RedisAdapter != nil {
		if err := rl.config.RedisAdapter.Ping(r.Context()); err != nil {
			status["status"] = "degraded"
			status["redis"] = "unavailable"
			status["error"] = err.Error()
		} else {
			status["redis"] = "available"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if status["status"] == "ok" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(status)
}