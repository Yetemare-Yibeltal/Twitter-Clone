// backend/internal/adapter/redis.go
package adapter

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/pkg/logger"
)

// RedisAdapter is the main interface for Redis operations.
type RedisAdapter interface {
	// Basic key-value operations
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	GetBytes(ctx context.Context, key string) ([]byte, error)
	GetJSON(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	Persist(ctx context.Context, key string) error
	
	// Cache patterns
	CacheGet(ctx context.Context, key string, dest interface{}, loader func() (interface{}, error), ttl time.Duration) error
	CacheSet(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	CacheDelete(ctx context.Context, key string) error
	CacheIncrement(ctx context.Context, key string, delta int64) (int64, error)

	// Hash operations
	HSet(ctx context.Context, key string, fields map[string]interface{}) error
	HGet(ctx context.Context, key, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, fields ...string) error
	HExists(ctx context.Context, key, field string) (bool, error)
	HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error)
	HKeys(ctx context.Context, key string) ([]string, error)
	HVals(ctx context.Context, key string) ([]string, error)

	// Set operations
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SRem(ctx context.Context, key string, members ...interface{}) error
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)
	SMembers(ctx context.Context, key string) ([]string, error)
	SCard(ctx context.Context, key string) (int64, error)

	// Sorted set operations
	ZAdd(ctx context.Context, key string, score float64, member interface{}) error
	ZRem(ctx context.Context, key string, members ...interface{}) error
	ZScore(ctx context.Context, key string, member interface{}) (float64, error)
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	ZRangeByScore(ctx context.Context, key string, min, max string, offset, limit int64) ([]string, error)
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	ZRank(ctx context.Context, key string, member interface{}) (int64, error)
	ZRevRank(ctx context.Context, key string, member interface{}) (int64, error)
	ZIncrBy(ctx context.Context, key string, incr float64, member interface{}) (float64, error)
	ZCard(ctx context.Context, key string) (int64, error)

	// List operations
	LPush(ctx context.Context, key string, values ...interface{}) error
	RPush(ctx context.Context, key string, values ...interface{}) error
	LPop(ctx context.Context, key string) (string, error)
	RPop(ctx context.Context, key string) (string, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LLen(ctx context.Context, key string) (int64, error)
	LRem(ctx context.Context, key string, count int64, value interface{}) error

	// Pub/Sub
	Publish(ctx context.Context, channel string, message interface{}) error
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub

	// Distributed locking
	Lock(ctx context.Context, key string, ttl time.Duration) (bool, string, error)
	Unlock(ctx context.Context, key, lockToken string) error
	WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error

	// Rate limiting
	RateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, int64, time.Duration, error)
	RateLimitWithTokenBucket(ctx context.Context, key string, rate int, burst int, capacity int) (bool, time.Duration, error)

	// Other utilities
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, delta int64) (int64, error)
	GetSet(ctx context.Context, key string, value interface{}) (string, error)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error)
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)

	// Health and metrics
	Ping(ctx context.Context) error
	Info(ctx context.Context) (string, error)
	GetStats() RedisStats
	Close() error
}

// RedisStats holds connection statistics.
type RedisStats struct {
	PoolSize     int
	IdleConns    int
	ActiveConns  int
	TotalConnections int64
	FailedConnections int64
	LastError    string
}

// redisAdapter implements RedisAdapter.
type redisAdapter struct {
	client     *redis.Client
	cluster    *redis.ClusterClient // not used; for future
	config     RedisConfig
	log        *logrus.Entry
	mu         sync.RWMutex
	stats      RedisStats
	closed     bool
}

// RedisConfig holds configuration.
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	Timeout      time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
	IdleTimeout  time.Duration
}

// DefaultRedisConfig returns sensible defaults.
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
		Timeout:      5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  5 * time.Minute,
	}
}

// NewRedisAdapter creates a new Redis adapter.
func NewRedisAdapter(cfg RedisConfig) (RedisAdapter, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	adapter := &redisAdapter{
		client: client,
		config: cfg,
		log:    logger.WithField("component", "redis_adapter"),
	}
	// Start stats collector
	go adapter.collectStats()
	return adapter, nil
}

// collectStats updates stats periodically.
func (a *redisAdapter) collectStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.mu.Lock()
		poolStats := a.client.PoolStats()
		a.stats.PoolSize = poolStats.TotalConns
		a.stats.IdleConns = poolStats.IdleConns
		a.stats.ActiveConns = poolStats.ActiveConns
		a.stats.TotalConnections = poolStats.Hits + poolStats.Misses
		a.stats.FailedConnections = poolStats.Timeouts
		a.mu.Unlock()
	}
}

// GetStats returns current stats.
func (a *redisAdapter) GetStats() RedisStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stats
}

// Close closes the Redis connection.
func (a *redisAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	return a.client.Close()
}

// ============ Basic key-value ============

func (a *redisAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return a.client.Set(ctx, key, value, ttl).Err()
}

func (a *redisAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}

func (a *redisAdapter) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return a.client.Get(ctx, key).Bytes()
}

func (a *redisAdapter) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := a.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (a *redisAdapter) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return a.client.Del(ctx, keys...).Err()
}

func (a *redisAdapter) Exists(ctx context.Context, keys ...string) (int64, error) {
	return a.client.Exists(ctx, keys...).Result()
}

func (a *redisAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.client.Expire(ctx, key, ttl).Err()
}

func (a *redisAdapter) TTL(ctx context.Context, key string) (time.Duration, error) {
	return a.client.TTL(ctx, key).Result()
}

func (a *redisAdapter) Persist(ctx context.Context, key string) error {
	return a.client.Persist(ctx, key).Err()
}

// ============ Cache patterns ============

func (a *redisAdapter) CacheGet(ctx context.Context, key string, dest interface{}, loader func() (interface{}, error), ttl time.Duration) error {
	// Try to get from cache
	val, err := a.GetJSON(ctx, key, dest)
	if err == nil {
		return nil
	}
	if err != nil && err != redis.Nil {
		// If error is not cache miss, log and proceed to load
		a.log.WithError(err).WithField("key", key).Warn("cache get error, loading from source")
	}
	// Load data
	data, err := loader()
	if err != nil {
		return err
	}
	// Store in cache
	if err := a.CacheSet(ctx, key, data, ttl); err != nil {
		a.log.WithError(err).WithField("key", key).Warn("failed to set cache")
	}
	// Unmarshal into dest
	switch v := data.(type) {
	case []byte:
		return json.Unmarshal(v, dest)
	case string:
		return json.Unmarshal([]byte(v), dest)
	default:
		// Assume data is already the desired type
		// This is risky; better to use JSON
	}
	return nil
}

func (a *redisAdapter) CacheSet(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Try to marshal to JSON
	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		var err error
		data, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal cache value: %w", err)
		}
	}
	return a.client.Set(ctx, key, data, ttl).Err()
}

func (a *redisAdapter) CacheDelete(ctx context.Context, key string) error {
	return a.Delete(ctx, key)
}

func (a *redisAdapter) CacheIncrement(ctx context.Context, key string, delta int64) (int64, error) {
	return a.client.IncrBy(ctx, key, delta).Result()
}

// ============ Hash operations ============

func (a *redisAdapter) HSet(ctx context.Context, key string, fields map[string]interface{}) error {
	return a.client.HSet(ctx, key, fields).Err()
}

func (a *redisAdapter) HGet(ctx context.Context, key, field string) (string, error) {
	return a.client.HGet(ctx, key, field).Result()
}

func (a *redisAdapter) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return a.client.HGetAll(ctx, key).Result()
}

func (a *redisAdapter) HDel(ctx context.Context, key string, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	return a.client.HDel(ctx, key, fields...).Err()
}

func (a *redisAdapter) HExists(ctx context.Context, key, field string) (bool, error) {
	return a.client.HExists(ctx, key, field).Result()
}

func (a *redisAdapter) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	return a.client.HIncrBy(ctx, key, field, incr).Result()
}

func (a *redisAdapter) HKeys(ctx context.Context, key string) ([]string, error) {
	return a.client.HKeys(ctx, key).Result()
}

func (a *redisAdapter) HVals(ctx context.Context, key string) ([]string, error) {
	return a.client.HVals(ctx, key).Result()
}

// ============ Set operations ============

func (a *redisAdapter) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return a.client.SAdd(ctx, key, members...).Err()
}

func (a *redisAdapter) SRem(ctx context.Context, key string, members ...interface{}) error {
	return a.client.SRem(ctx, key, members...).Err()
}

func (a *redisAdapter) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return a.client.SIsMember(ctx, key, member).Result()
}

func (a *redisAdapter) SMembers(ctx context.Context, key string) ([]string, error) {
	return a.client.SMembers(ctx, key).Result()
}

func (a *redisAdapter) SCard(ctx context.Context, key string) (int64, error) {
	return a.client.SCard(ctx, key).Result()
}

// ============ Sorted set operations ============

func (a *redisAdapter) ZAdd(ctx context.Context, key string, score float64, member interface{}) error {
	return a.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

func (a *redisAdapter) ZRem(ctx context.Context, key string, members ...interface{}) error {
	return a.client.ZRem(ctx, key, members...).Err()
}

func (a *redisAdapter) ZScore(ctx context.Context, key string, member interface{}) (float64, error) {
	return a.client.ZScore(ctx, key, member).Result()
}

func (a *redisAdapter) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return a.client.ZRange(ctx, key, start, stop).Result()
}

func (a *redisAdapter) ZRangeByScore(ctx context.Context, key string, min, max string, offset, limit int64) ([]string, error) {
	opt := &redis.ZRangeBy{
		Min:    min,
		Max:    max,
		Offset: offset,
		Count:  limit,
	}
	return a.client.ZRangeByScore(ctx, key, opt).Result()
}

func (a *redisAdapter) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return a.client.ZRevRange(ctx, key, start, stop).Result()
}

func (a *redisAdapter) ZRank(ctx context.Context, key string, member interface{}) (int64, error) {
	return a.client.ZRank(ctx, key, member).Result()
}

func (a *redisAdapter) ZRevRank(ctx context.Context, key string, member interface{}) (int64, error) {
	return a.client.ZRevRank(ctx, key, member).Result()
}

func (a *redisAdapter) ZIncrBy(ctx context.Context, key string, incr float64, member interface{}) (float64, error) {
	return a.client.ZIncrBy(ctx, key, incr, member).Result()
}

func (a *redisAdapter) ZCard(ctx context.Context, key string) (int64, error) {
	return a.client.ZCard(ctx, key).Result()
}

// ============ List operations ============

func (a *redisAdapter) LPush(ctx context.Context, key string, values ...interface{}) error {
	return a.client.LPush(ctx, key, values...).Err()
}

func (a *redisAdapter) RPush(ctx context.Context, key string, values ...interface{}) error {
	return a.client.RPush(ctx, key, values...).Err()
}

func (a *redisAdapter) LPop(ctx context.Context, key string) (string, error) {
	return a.client.LPop(ctx, key).Result()
}

func (a *redisAdapter) RPop(ctx context.Context, key string) (string, error) {
	return a.client.RPop(ctx, key).Result()
}

func (a *redisAdapter) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return a.client.LRange(ctx, key, start, stop).Result()
}

func (a *redisAdapter) LLen(ctx context.Context, key string) (int64, error) {
	return a.client.LLen(ctx, key).Result()
}

func (a *redisAdapter) LRem(ctx context.Context, key string, count int64, value interface{}) error {
	return a.client.LRem(ctx, key, count, value).Err()
}

// ============ Pub/Sub ============

func (a *redisAdapter) Publish(ctx context.Context, channel string, message interface{}) error {
	return a.client.Publish(ctx, channel, message).Err()
}

func (a *redisAdapter) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return a.client.Subscribe(ctx, channels...)
}

func (a *redisAdapter) PSubscribe(ctx context.Context, patterns ...string) *redis.PubSub {
	return a.client.PSubscribe(ctx, patterns...)
}

// ============ Distributed locking ============

// Lock attempts to acquire a lock with TTL. Returns lock token if successful.
func (a *redisAdapter) Lock(ctx context.Context, key string, ttl time.Duration) (bool, string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return false, "", err
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)
	// Use SET NX with TTL
	ok, err := a.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "", nil
	}
	return true, token, nil
}

// Unlock releases the lock if token matches.
func (a *redisAdapter) Unlock(ctx context.Context, key, lockToken string) error {
	// Lua script to atomically check and delete
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	result, err := a.client.Eval(ctx, script, []string{key}, lockToken).Result()
	if err != nil {
		return err
	}
	if result.(int64) == 0 {
		return fmt.Errorf("lock not held or token mismatch")
	}
	return nil
}

// WithLock executes a function under a lock, retrying if needed.
func (a *redisAdapter) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	var token string
	var ok bool
	var err error
	for {
		ok, token, err = a.Lock(ctx, key, ttl)
		if err != nil {
			return err
		}
		if ok {
			break
		}
		// Wait and retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer func() {
		if unlockErr := a.Unlock(context.Background(), key, token); unlockErr != nil {
			a.log.WithError(unlockErr).WithField("key", key).Warn("failed to release lock")
		}
	}()
	return fn()
}

// ============ Rate limiting ============

// RateLimit implements a sliding window rate limiter (fixed window with Lua).
func (a *redisAdapter) RateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, int64, time.Duration, error) {
	now := time.Now().UnixMilli()
	windowMs := window.Milliseconds()
	// Lua script for sliding window (simplified: fixed window with counters)
	script := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local current_time = tonumber(ARGV[3])
		
		local count_key = key .. ":count"
		local expiry_key = key .. ":expiry"
		
		local current_count = redis.call("get", count_key)
		if current_count == false then
			redis.call("set", count_key, 1)
			redis.call("expire", count_key, math.ceil(window/1000))
			return {1, limit - 1, 0}
		end
		
		current_count = tonumber(current_count)
		if current_count < limit then
			redis.call("incr", count_key)
			return {current_count + 1, limit - (current_count + 1), 0}
		else
			local ttl = redis.call("ttl", count_key)
			return {current_count, 0, ttl}
		end
	`
	result, err := a.client.Eval(ctx, script, []string{key}, limit, windowMs, now).Result()
	if err != nil {
		return false, 0, 0, err
	}
	values := result.([]interface{})
	current := values[0].(int64)
	remaining := values[1].(int64)
	ttlSec := values[2].(int64)
	retryAfter := time.Duration(ttlSec) * time.Second
	if remaining < 0 {
		remaining = 0
	}
	allowed := current <= int64(limit)
	return allowed, remaining, retryAfter, nil
}

// RateLimitWithTokenBucket implements token bucket algorithm.
func (a *redisAdapter) RateLimitWithTokenBucket(ctx context.Context, key string, rate int, burst int, capacity int) (bool, time.Duration, error) {
	// Lua script for token bucket
	script := `
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local burst = tonumber(ARGV[2])
		local capacity = tonumber(ARGV[3])
		local now = tonumber(ARGV[4])
		
		local last_refill_key = key .. ":last_refill"
		local tokens_key = key .. ":tokens"
		
		local last_refill = redis.call("get", last_refill_key)
		local tokens = redis.call("get", tokens_key)
		
		if last_refill == false then
			last_refill = now
			tokens = capacity
			redis.call("set", last_refill_key, last_refill)
			redis.call("set", tokens_key, tokens)
			redis.call("expire", last_refill_key, 3600)
			redis.call("expire", tokens_key, 3600)
		else
			last_refill = tonumber(last_refill)
			tokens = tonumber(tokens)
			local elapsed = (now - last_refill) / 1000
			local new_tokens = math.min(capacity, tokens + elapsed * rate)
			tokens = new_tokens
			redis.call("set", last_refill_key, now)
			redis.call("set", tokens_key, tokens)
			redis.call("expire", last_refill_key, 3600)
			redis.call("expire", tokens_key, 3600)
		end
		
		if tokens >= 1 then
			redis.call("decr", tokens_key)
			return {1, tokens - 1}
		else
			local wait_time = math.ceil((1 - tokens) / rate * 1000)
			return {0, wait_time}
		end
	`
	result, err := a.client.Eval(ctx, script, []string{key}, rate, burst, capacity, time.Now().UnixMilli()).Result()
	if err != nil {
		return false, 0, err
	}
	values := result.([]interface{})
	allowed := values[0].(int64) == 1
	waitMs := values[1].(int64)
	return allowed, time.Duration(waitMs) * time.Millisecond, nil
}

// ============ Other utilities ============

func (a *redisAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return a.client.Incr(ctx, key).Result()
}

func (a *redisAdapter) Decr(ctx context.Context, key string) (int64, error) {
	return a.client.Decr(ctx, key).Result()
}

func (a *redisAdapter) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return a.client.IncrBy(ctx, key, delta).Result()
}

func (a *redisAdapter) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	return a.client.GetSet(ctx, key, value).Result()
}

func (a *redisAdapter) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return a.client.SetNX(ctx, key, value, ttl).Result()
}

func (a *redisAdapter) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	return a.client.Scan(ctx, cursor, match, count).Result()
}

func (a *redisAdapter) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return a.client.Eval(ctx, script, keys, args...).Result()
}

// ============ Health ============

func (a *redisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

func (a *redisAdapter) Info(ctx context.Context) (string, error) {
	return a.client.Info(ctx).Result()
}

// ============ Additional helper for default instance ============

var defaultRedisAdapter RedisAdapter
var redisOnce sync.Once

// InitRedis initializes the global Redis adapter.
func InitRedis(cfg RedisConfig) error {
	var err error
	redisOnce.Do(func() {
		defaultRedisAdapter, err = NewRedisAdapter(cfg)
	})
	return err
}

// GetRedis returns the global Redis adapter.
func GetRedis() RedisAdapter {
	if defaultRedisAdapter == nil {
		panic("redis adapter not initialized")
	}
	return defaultRedisAdapter
}