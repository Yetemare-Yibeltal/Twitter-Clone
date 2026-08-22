// backend/internal/middleware/ratelimit.go
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/pkg/logger"
)

// Common rate limit errors.
var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrInvalidLimit      = errors.New("invalid rate limit configuration")
	ErrStoreNotAvailable = errors.New("rate limit store not available")
)

// RateLimitAlgorithm defines the algorithm to use.
type RateLimitAlgorithm string

const (
	AlgorithmFixedWindow  RateLimitAlgorithm = "fixed_window"
	AlgorithmSlidingWindow RateLimitAlgorithm = "sliding_window"
	AlgorithmTokenBucket  RateLimitAlgorithm = "token_bucket"
	AlgorithmLeakyBucket  RateLimitAlgorithm = "leaky_bucket"
)

// RateLimitConfig holds the configuration for rate limiting.
type RateLimitConfig struct {
	// Global defaults
	DefaultLimit    int                    `json:"default_limit"`
	DefaultWindow   time.Duration          `json:"default_window"`
	DefaultBurst    int                    `json:"default_burst"`
	Algorithm       RateLimitAlgorithm     `json:"algorithm"`
	
	// Per-route overrides
	Routes          map[string]*RouteLimit `json:"routes"`
	
	// Store configuration
	RedisAdapter    adapter.RedisAdapter   `json:"-"`
	InMemoryStore   *InMemoryStore         `json:"-"`
	
	// Whitelist/blacklist
	WhitelistIPs    []string               `json:"whitelist_ips"`
	BlacklistIPs    []string               `json:"blacklist_ips"`
	
	// Headers
	IncludeHeaders  bool                   `json:"include_headers"`
	
	// Metrics
	EnableMetrics   bool                   `json:"enable_metrics"`
	
	// Fallback behavior
	FallbackToMemory bool                 `json:"fallback_to_memory"`
}

// RouteLimit defines per-route limits.
type RouteLimit struct {
	Limit   int           `json:"limit"`
	Window  time.Duration `json:"window"`
	Burst   int           `json:"burst"`
	Method  string        `json:"method"`
}

// RateLimiter is the main rate limiter interface.
type RateLimiter interface {
	// Allow checks if a request is allowed.
	Allow(ctx context.Context, key string) (*RateLimitResult, error)
	// AllowWithOptions checks with specific limits.
	AllowWithOptions(ctx context.Context, key string, limit int, window time.Duration, burst int) (*RateLimitResult, error)
	// Reset resets the limit for a key.
	Reset(ctx context.Context, key string) error
	// GetStats returns current statistics.
	GetStats(ctx context.Context, key string) (*RateLimitStats, error)
	// Close cleans up resources.
	Close() error
}

// RateLimitResult contains the result of an allow check.
type RateLimitResult struct {
	Allowed   bool          `json:"allowed"`
	Limit     int           `json:"limit"`
	Remaining int           `json:"remaining"`
	ResetAt   time.Time     `json:"reset_at"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
	Current   int           `json:"current"`
}

// RateLimitStats contains statistics for a key.
type RateLimitStats struct {
	Key        string    `json:"key"`
	TotalHits  int64     `json:"total_hits"`
	TotalBlocked int64   `json:"total_blocked"`
	Current    int       `json:"current"`
	Limit      int       `json:"limit"`
	ResetAt    time.Time `json:"reset_at"`
	Window     time.Duration `json:"window"`
}

// InMemoryStore provides a local rate limit store (fallback).
type InMemoryStore struct {
	mu          sync.RWMutex
	limits      map[string]*inMemoryLimit
	cleanupTick time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// inMemoryLimit represents a single rate limit entry in memory.
type inMemoryLimit struct {
	count     int
	window    time.Duration
	resetAt   time.Time
	burst     int
	tokens    int
	lastRefill time.Time
	mu        sync.Mutex
	algorithm RateLimitAlgorithm
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore(cleanupInterval time.Duration) *InMemoryStore {
	store := &InMemoryStore{
		limits:      make(map[string]*inMemoryLimit),
		cleanupTick: cleanupInterval,
		stopCh:      make(chan struct{}),
	}
	store.startCleanup()
	return store
}

// startCleanup runs a background goroutine to clean up expired entries.
func (s *InMemoryStore) startCleanup() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.cleanupTick)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.cleanup()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// cleanup removes expired entries.
func (s *InMemoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, entry := range s.limits {
		entry.mu.Lock()
		if entry.resetAt.Before(now) {
			delete(s.limits, key)
		}
		entry.mu.Unlock()
	}
}

// Close stops the cleanup goroutine.
func (s *InMemoryStore) Close() {
	close(s.stopCh)
	s.wg.Wait()
}

// getOrCreate returns an existing limit entry or creates a new one.
func (s *InMemoryStore) getOrCreate(key string, limit int, window time.Duration, burst int, algorithm RateLimitAlgorithm) *inMemoryLimit {
	s.mu.RLock()
	entry, exists := s.limits[key]
	s.mu.RUnlock()
	if exists {
		entry.mu.Lock()
		// Check if window has changed
		if entry.window != window || entry.burst != burst {
			entry.window = window
			entry.burst = burst
			entry.resetAt = time.Now().Add(window)
			entry.count = 0
			entry.tokens = burst
			entry.lastRefill = time.Now()
		}
		entry.mu.Unlock()
		return entry
	}
	// Create new entry
	s.mu.Lock()
	defer s.mu.Unlock()
	entry = &inMemoryLimit{
		window:    window,
		resetAt:   time.Now().Add(window),
		burst:     burst,
		tokens:    burst,
		lastRefill: time.Now(),
		algorithm: algorithm,
	}
	s.limits[key] = entry
	return entry
}

// Allow checks if a request is allowed using in-memory store.
func (s *InMemoryStore) Allow(key string, limit int, window time.Duration, burst int, algorithm RateLimitAlgorithm) (int, int, time.Time, bool) {
	entry := s.getOrCreate(key, limit, window, burst, algorithm)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	now := time.Now()
	switch algorithm {
	case AlgorithmFixedWindow:
		// Check if window has expired
		if now.After(entry.resetAt) {
			entry.count = 0
			entry.resetAt = now.Add(window)
		}
		if entry.count >= limit {
			return entry.count, limit, entry.resetAt, false
		}
		entry.count++
		return entry.count, limit - entry.count, entry.resetAt, true
	case AlgorithmSlidingWindow:
		// Simplified sliding window: use fixed window with smaller granularity
		if now.After(entry.resetAt) {
			entry.count = 0
			entry.resetAt = now.Add(window)
		}
		if entry.count >= limit {
			return entry.count, limit, entry.resetAt, false
		}
		entry.count++
		return entry.count, limit - entry.count, entry.resetAt, true
	case AlgorithmTokenBucket:
		// Refill tokens
		elapsed := now.Sub(entry.lastRefill)
		refillRate := float64(limit) / float64(window.Seconds())
		newTokens := int(elapsed.Seconds() * refillRate)
		if newTokens > 0 {
			entry.tokens = min(entry.tokens+newTokens, burst)
			entry.lastRefill = now
		}
		if entry.tokens <= 0 {
			return 0, 0, now.Add(time.Duration(float64(window.Seconds())/float64(limit)) * time.Second), false
		}
		entry.tokens--
		return entry.tokens, entry.tokens, entry.resetAt, true
	case AlgorithmLeakyBucket:
		// Simplified: leaky bucket with fixed rate
		if now.After(entry.resetAt) {
			entry.count = 0
			entry.resetAt = now.Add(window)
		}
		leakRate := float64(limit) / float64(window.Seconds())
		leaked := int(elapsed.Seconds() * leakRate)
		if leaked > 0 {
			entry.count = max(0, entry.count-leaked)
			entry.lastRefill = now
		}
		if entry.count >= limit {
			return entry.count, limit, entry.resetAt, false
		}
		entry.count++
		return entry.count, limit - entry.count, entry.resetAt, true
	default:
		return 0, 0, now, false
	}
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// redisRateLimiter implements RateLimiter using Redis.
type redisRateLimiter struct {
	redis     *redis.Client
	config    RateLimitConfig
	log       *logrus.Entry
	inMemory  *InMemoryStore
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter.
func NewRedisRateLimiter(redisClient *redis.Client, config RateLimitConfig) RateLimiter {
	limiter := &redisRateLimiter{
		redis: redisClient,
		config: config,
		log:    logger.WithField("component", "redis_ratelimiter"),
	}
	if config.FallbackToMemory {
		limiter.inMemory = NewInMemoryStore(5 * time.Minute)
	}
	return limiter
}

// Allow implements RateLimiter.Allow.
func (r *redisRateLimiter) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	limit := r.config.DefaultLimit
	window := r.config.DefaultWindow
	burst := r.config.DefaultBurst
	return r.AllowWithOptions(ctx, key, limit, window, burst)
}

// AllowWithOptions implements RateLimiter.AllowWithOptions.
func (r *redisRateLimiter) AllowWithOptions(ctx context.Context, key string, limit int, window time.Duration, burst int) (*RateLimitResult, error) {
	// Check blacklist
	if r.isIPBlacklisted(key) {
		return &RateLimitResult{
			Allowed:   false,
			Limit:     limit,
			Remaining: 0,
			RetryAfter: 24 * time.Hour,
		}, nil
	}
	// Check whitelist
	if r.isIPWhitelisted(key) {
		return &RateLimitResult{
			Allowed:   true,
			Limit:     limit,
			Remaining: limit,
		}, nil
	}
	// Try Redis first
	if r.redis != nil {
		result, err := r.allowRedis(ctx, key, limit, window, burst)
		if err == nil {
			return result, nil
		}
		r.log.WithError(err).Warn("Redis rate limit failed, falling back to memory")
	}
	// Fallback to memory
	if r.inMemory != nil {
		return r.allowMemory(key, limit, window, burst)
	}
	return nil, ErrStoreNotAvailable
}

// allowRedis uses Lua scripts for atomic rate limiting in Redis.
func (r *redisRateLimiter) allowRedis(ctx context.Context, key string, limit int, window time.Duration, burst int) (*RateLimitResult, error) {
	var script string
	var result []interface{}
	var err error
	switch r.config.Algorithm {
	case AlgorithmFixedWindow:
		script = `
			local key = KEYS[1]
			local limit = tonumber(ARGV[1])
			local window = tonumber(ARGV[2])
			local now = tonumber(ARGV[3])
			local count = redis.call("incr", key)
			if count == 1 then
				redis.call("expire", key, math.ceil(window))
			end
			local ttl = redis.call("ttl", key)
			if count <= limit then
				return {count, limit - count, ttl, 1}
			else
				return {count, 0, ttl, 0}
			end
		`
		result, err = r.redis.Eval(ctx, script, []string{key}, limit, window.Seconds(), time.Now().Unix()).Result().([]interface{})
	case AlgorithmTokenBucket:
		script = `
			local key = KEYS[1]
			local limit = tonumber(ARGV[1])
			local window = tonumber(ARGV[2])
			local burst = tonumber(ARGV[3])
			local now = tonumber(ARGV[4])
			local tokens_key = key .. ":tokens"
			local last_refill_key = key .. ":last_refill"
			local tokens = redis.call("get", tokens_key)
			local last_refill = redis.call("get", last_refill_key)
			if tokens == false then
				tokens = burst
				last_refill = now
				redis.call("set", tokens_key, tokens)
				redis.call("set", last_refill_key, last_refill)
				redis.call("expire", tokens_key, math.ceil(window))
				redis.call("expire", last_refill_key, math.ceil(window))
			else
				tokens = tonumber(tokens)
				last_refill = tonumber(last_refill)
				local rate = limit / window
				local elapsed = now - last_refill
				local new_tokens = math.min(burst, tokens + elapsed * rate)
				tokens = new_tokens
				redis.call("set", tokens_key, tokens)
				redis.call("set", last_refill_key, now)
				redis.call("expire", tokens_key, math.ceil(window))
				redis.call("expire", last_refill_key, math.ceil(window))
			end
			if tokens >= 1 then
				redis.call("decr", tokens_key)
				return {tokens - 1, tokens - 1, 0, 1}
			else
				local wait_time = (1 - tokens) / (limit / window)
				return {0, 0, wait_time, 0}
			end
		`
		result, err = r.redis.Eval(ctx, script, []string{key}, limit, window.Seconds(), burst, time.Now().Unix()).Result().([]interface{})
	default:
		// Default to fixed window
		return r.allowRedis(ctx, key, limit, window, burst)
	}
	if err != nil {
		return nil, err
	}
	if len(result) < 4 {
		return nil, errors.New("invalid result from Redis")
	}
	current := int(result[0].(int64))
	remaining := int(result[1].(int64))
	ttl := int64(result[2].(int64))
	allowed := int(result[3].(int64)) == 1
	resetAt := time.Now().Add(time.Duration(ttl) * time.Second)
	return &RateLimitResult{
		Allowed:   allowed,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
		Current:   current,
	}, nil
}

// allowMemory uses the in-memory store.
func (r *redisRateLimiter) allowMemory(key string, limit int, window time.Duration, burst int) (*RateLimitResult, error) {
	current, remaining, resetAt, allowed := r.inMemory.Allow(key, limit, window, burst, r.config.Algorithm)
	return &RateLimitResult{
		Allowed:   allowed,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
		Current:   current,
	}, nil
}

// Reset implements RateLimiter.Reset.
func (r *redisRateLimiter) Reset(ctx context.Context, key string) error {
	if r.redis != nil {
		return r.redis.Del(ctx, key).Err()
	}
	if r.inMemory != nil {
		r.inMemory.mu.Lock()
		defer r.inMemory.mu.Unlock()
		delete(r.inMemory.limits, key)
		return nil
	}
	return ErrStoreNotAvailable
}

// GetStats implements RateLimiter.GetStats.
func (r *redisRateLimiter) GetStats(ctx context.Context, key string) (*RateLimitStats, error) {
	if r.redis != nil {
		val, err := r.redis.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		ttl, err := r.redis.TTL(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		return &RateLimitStats{
			Key:     key,
			Current: int(val),
			ResetAt: time.Now().Add(ttl),
		}, nil
	}
	return nil, ErrStoreNotAvailable
}

// Close implements RateLimiter.Close.
func (r *redisRateLimiter) Close() error {
	if r.inMemory != nil {
		r.inMemory.Close()
	}
	return nil
}

// isIPBlacklisted checks if an IP is blacklisted.
func (r *redisRateLimiter) isIPBlacklisted(key string) bool {
	for _, ip := range r.config.BlacklistIPs {
		if strings.Contains(key, ip) {
			return true
		}
	}
	return false
}

// isIPWhitelisted checks if an IP is whitelisted.
func (r *redisRateLimiter) isIPWhitelisted(key string) bool {
	for _, ip := range r.config.WhitelistIPs {
		if strings.Contains(key, ip) {
			return true
		}
	}
	return false
}

// RateLimitMiddleware returns an HTTP middleware that applies rate limiting.
func RateLimitMiddleware(limiter RateLimiter, config RateLimitConfig) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Determine the key
			key := getRateLimitKey(r)
			// Check if route has specific limits
			limit := config.DefaultLimit
			window := config.DefaultWindow
			burst := config.DefaultBurst
			// Check route overrides
			if config.Routes != nil {
				routeKey := getRouteKey(r)
				if routeLimit, exists := config.Routes[routeKey]; exists {
					if routeLimit.Limit > 0 {
						limit = routeLimit.Limit
					}
					if routeLimit.Window > 0 {
						window = routeLimit.Window
					}
					if routeLimit.Burst > 0 {
						burst = routeLimit.Burst
					}
				}
			}
			// Check if method-specific limit exists
			if config.Routes != nil {
				methodKey := getRouteKey(r) + ":" + r.Method
				if routeLimit, exists := config.Routes[methodKey]; exists {
					if routeLimit.Limit > 0 {
						limit = routeLimit.Limit
					}
					if routeLimit.Window > 0 {
						window = routeLimit.Window
					}
					if routeLimit.Burst > 0 {
						burst = routeLimit.Burst
					}
				}
			}
			// Check if user is authenticated (use user ID for key)
			if userID, err := GetUserID(r.Context()); err == nil {
				key = "user:" + userID + ":" + r.URL.Path
			}
			// Apply rate limit
			result, err := limiter.AllowWithOptions(r.Context(), key, limit, window, burst)
			if err != nil {
				// Log error and allow request (fail open)
				logger.WithError(err).Error("Rate limiter error, allowing request")
				next.ServeHTTP(w, r)
				return
			}
			// Set headers
			if config.IncludeHeaders {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
				w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", result.ResetAt.Unix()))
			}
			if !result.Allowed {
				if result.RetryAfter > 0 {
					w.Header().Set("Retry-After", fmt.Sprintf("%d", int(result.RetryAfter.Seconds())))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "rate_limit_exceeded",
					"message": "Too many requests. Please try again later.",
					"retry_after": int(result.RetryAfter.Seconds()),
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// getRateLimitKey generates a key for rate limiting.
func getRateLimitKey(r *http.Request) string {
	// Try to get client IP
	ip := getClientIP(r)
	// Try to get authenticated user
	if userID, err := GetUserID(r.Context()); err == nil {
		return fmt.Sprintf("user:%s:%s:%s", userID, r.Method, r.URL.Path)
	}
	// Fallback to IP
	return fmt.Sprintf("ip:%s:%s:%s", ip, r.Method, r.URL.Path)
}

// getRouteKey generates a route key.
func getRouteKey(r *http.Request) string {
	// Use the route pattern if available
	if route := mux.CurrentRoute(r); route != nil {
		if pattern, err := route.GetPathTemplate(); err == nil {
			return pattern
		}
	}
	return r.URL.Path
}

// getClientIP extracts the client IP from the request.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// Check X-Real-IP header
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}