// backend/internal/middleware/logger.go
package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// RequestIDKey is the context key for request ID.
	RequestIDKey contextKey = "request_id"
	
	// RequestStartKey is the context key for request start time.
	RequestStartKey contextKey = "request_start"
	
	// DefaultRequestIDHeader is the header used for request ID propagation.
	DefaultRequestIDHeader = "X-Request-ID"
)

// ======================================================================
// Sensitive Data Redaction
// ======================================================================

// SensitiveFields lists fields that should be redacted.
var SensitiveFields = map[string]bool{
	"password":   true,
	"token":      true,
	"secret":     true,
	"api_key":    true,
	"api_secret": true,
	"access_token": true,
	"refresh_token": true,
	"authorization": true,
	"cookie":     true,
	"credit_card": true,
	"card_number": true,
	"cvv":        true,
	"ssn":        true,
}

// SensitiveHeaders lists headers that should be redacted.
var SensitiveHeaders = map[string]bool{
	"authorization": true,
	"cookie":        true,
	"x-api-key":     true,
	"x-access-token": true,
	"x-refresh-token": true,
}

// redactPatterns is a list of regex patterns for redaction.
var redactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`([Aa]uth(?:orization)?)[:=]\s*[^\s,]+`),
	regexp.MustCompile(`([Bb]earer)\s+[^\s]+`),
	regexp.MustCompile(`([Tt]oken)[:=]\s*[^\s,]+`),
	regexp.MustCompile(`([Ss]ecret)[:=]\s*[^\s,]+`),
	regexp.MustCompile(`([Pp]assword)[:=]\s*[^\s,]+`),
	regexp.MustCompile(`([Aa]pi[_-]?[Kk]ey)[:=]\s*[^\s,]+`),
}

// RedactString redacts sensitive information from a string.
func RedactString(input string) string {
	if input == "" {
		return ""
	}
	result := input
	for _, pattern := range redactPatterns {
		result = pattern.ReplaceAllString(result, "$1=[REDACTED]")
	}
	return result
}

// RedactMap redacts sensitive fields from a map.
func RedactMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	result := make(map[string]interface{})
	for key, value := range data {
		if SensitiveFields[strings.ToLower(key)] {
			result[key] = "[REDACTED]"
		} else {
			result[key] = value
		}
	}
	return result
}

// ======================================================================
// Logger Middleware Configuration
// ======================================================================

// LoggerConfig holds configuration for the logging middleware.
type LoggerConfig struct {
	// LogLevel is the minimum log level to log.
	LogLevel logrus.Level `json:"log_level"`
	
	// LogHeaders determines if headers are logged.
	LogHeaders bool `json:"log_headers"`
	
	// LogBody determines if request/response bodies are logged.
	LogBody bool `json:"log_body"`
	
	// LogQuery determines if query parameters are logged.
	LogQuery bool `json:"log_query"`
	
	// MaxBodySize is the maximum body size to log.
	MaxBodySize int64 `json:"max_body_size"`
	
	// RequestIDHeader is the header to use for request ID.
	RequestIDHeader string `json:"request_id_header"`
	
	// GenerateRequestID determines if a request ID should be generated.
	GenerateRequestID bool `json:"generate_request_id"`
	
	// IncludeIP determines if client IP is logged.
	IncludeIP bool `json:"include_ip"`
	
	// IncludeUserAgent determines if user agent is logged.
	IncludeUserAgent bool `json:"include_user_agent"`
	
	// IncludeLatency determines if request latency is logged.
	IncludeLatency bool `json:"include_latency"`
	
	// IncludeMemory determines if memory usage is logged.
	IncludeMemory bool `json:"include_memory"`
	
	// IncludeGoroutine determines if goroutine count is logged.
	IncludeGoroutine bool `json:"include_goroutine"`
	
	// StructuredOutput determines if logs are JSON formatted.
	StructuredOutput bool `json:"structured_output"`
	
	// Output is the output destination (default: stdout).
	Output io.Writer `json:"-"`
	
	// ExcludePaths are paths to exclude from logging.
	ExcludePaths []string `json:"exclude_paths"`
	
	// ExcludePatterns are regex patterns to exclude from logging.
	ExcludePatterns []*regexp.Regexp `json:"-"`
}

// DefaultLoggerConfig returns sensible defaults.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		LogLevel:        logrus.InfoLevel,
		LogHeaders:      false,
		LogBody:         false,
		LogQuery:        true,
		MaxBodySize:     1024 * 1024, // 1MB
		RequestIDHeader: DefaultRequestIDHeader,
		GenerateRequestID: true,
		IncludeIP:       true,
		IncludeUserAgent: true,
		IncludeLatency:  true,
		IncludeMemory:   false,
		IncludeGoroutine: false,
		StructuredOutput: false,
		Output:          os.Stdout,
		ExcludePaths:    []string{"/health", "/ready", "/live", "/metrics"},
	}
}

// ======================================================================
= Logger Middleware
// ======================================================================

// LoggerMiddleware is the main logger middleware struct.
type LoggerMiddleware struct {
	config   LoggerConfig
	log      *logrus.Entry
	mu       sync.RWMutex
	excludes map[string]bool
}

// NewLoggerMiddleware creates a new logger middleware.
func NewLoggerMiddleware(cfg LoggerConfig) *LoggerMiddleware {
	// Build exclude map
	excludes := make(map[string]bool)
	for _, path := range cfg.ExcludePaths {
		excludes[path] = true
	}
	
	// Setup logging
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	
	logger := logrus.New()
	logger.SetOutput(cfg.Output)
	logger.SetLevel(cfg.LogLevel)
	if cfg.StructuredOutput {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
	
	return &LoggerMiddleware{
		config:   cfg,
		log:      logger.WithField("component", "http_logger"),
		excludes: excludes,
	}
}

// Middleware returns the HTTP middleware handler.
func (lm *LoggerMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if path should be excluded
		if lm.shouldExclude(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		
		// Start timer
		startTime := time.Now()
		
		// Generate or propagate request ID
		requestID := lm.getRequestID(r)
		
		// Create request context with ID
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		ctx = context.WithValue(ctx, RequestStartKey, startTime)
		r = r.WithContext(ctx)
		
		// Wrap response writer to capture status and size
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:    http.StatusOK,
		}
		
		// Log request (if enabled)
		logEntry := lm.buildRequestLogEntry(r)
		logEntry = logEntry.WithField("request_id", requestID)
		
		// Log body if enabled
		if lm.config.LogBody && r.ContentLength > 0 && r.ContentLength <= lm.config.MaxBodySize {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil {
				// Restore body for downstream
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				logEntry = logEntry.WithField("request_body", lm.sanitizeBody(string(bodyBytes)))
			}
		}
		
		// Log before request (debug level)
		logEntry.Debug("Request received")
		
		// Process request
		next.ServeHTTP(wrapped, r)
		
		// Calculate latency
		latency := time.Since(startTime)
		
		// Build response log entry
		respLogEntry := lm.buildResponseLogEntry(wrapped, r, latency)
		respLogEntry = respLogEntry.WithFields(logrus.Fields{
			"request_id": requestID,
			"latency_ms": latency.Milliseconds(),
		})
		
		// Add memory stats if enabled
		if lm.config.IncludeMemory {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			respLogEntry = respLogEntry.WithFields(logrus.Fields{
				"memory_alloc":   memStats.Alloc,
				"memory_sys":     memStats.Sys,
				"memory_gc":      memStats.NumGC,
				"goroutines":     runtime.NumGoroutine(),
			})
		}
		
		// Log based on status code
		status := wrapped.statusCode
		if status >= 500 {
			respLogEntry.Error("Request completed with error")
		} else if status >= 400 {
			respLogEntry.Warn("Request completed with client error")
		} else {
			respLogEntry.Info("Request completed")
		}
	})
}

// ======================================================================
= Response Writer Wrapper
// ======================================================================

// responseWriter wraps http.ResponseWriter to capture status and size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size.
func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += int64(size)
	return size, err
}

// Status returns the status code.
func (rw *responseWriter) Status() int {
	return rw.statusCode
}

// Size returns the response size.
func (rw *responseWriter) Size() int64 {
	return rw.size
}

// ======================================================================
= Request ID Handling
// ======================================================================

// getRequestID gets or generates a request ID.
func (lm *LoggerMiddleware) getRequestID(r *http.Request) string {
	// Try to get from header
	header := lm.config.RequestIDHeader
	if header == "" {
		header = DefaultRequestIDHeader
	}
	
	if id := r.Header.Get(header); id != "" {
		return id
	}
	
	// Try to get from context
	if id, ok := r.Context().Value(RequestIDKey).(string); ok && id != "" {
		return id
	}
	
	// Generate new ID if configured
	if lm.config.GenerateRequestID {
		return generateRequestID()
	}
	
	return ""
}

// generateRequestID generates a unique request ID.
func generateRequestID() string {
	// Try UUID first
	if id, err := uuid.NewRandom(); err == nil {
		return id.String()
	}
	
	// Fallback to hex
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	
	// Ultimate fallback
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
}

// ======================================================================
= Log Entry Builders
// ======================================================================

// buildRequestLogEntry builds a log entry for the request.
func (lm *LoggerMiddleware) buildRequestLogEntry(r *http.Request) *logrus.Entry {
	fields := logrus.Fields{
		"method":     r.Method,
		"path":       r.URL.Path,
		"protocol":   r.Proto,
		"remote_addr": r.RemoteAddr,
	}
	
	// Add IP if enabled
	if lm.config.IncludeIP {
		ip := getClientIP(r)
		if ip != "" {
			fields["client_ip"] = ip
		}
	}
	
	// Add User-Agent if enabled
	if lm.config.IncludeUserAgent {
		if ua := r.UserAgent(); ua != "" {
			fields["user_agent"] = ua
		}
	}
	
	// Add query if enabled
	if lm.config.LogQuery && len(r.URL.Query()) > 0 {
		fields["query"] = redactMap(r.URL.Query())
	}
	
	// Add headers if enabled
	if lm.config.LogHeaders {
		fields["headers"] = lm.redactHeaders(r.Header)
	}
	
	// Add content length
	if r.ContentLength > 0 {
		fields["content_length"] = r.ContentLength
	}
	
	// Add host
	if host := r.Host; host != "" {
		fields["host"] = host
	}
	
	return lm.log.WithFields(fields)
}

// buildResponseLogEntry builds a log entry for the response.
func (lm *LoggerMiddleware) buildResponseLogEntry(w *responseWriter, r *http.Request, latency time.Duration) *logrus.Entry {
	fields := logrus.Fields{
		"status":      w.statusCode,
		"response_size": w.size,
		"method":      r.Method,
		"path":        r.URL.Path,
	}
	
	if lm.config.IncludeLatency {
		fields["latency_ms"] = latency.Milliseconds()
	}
	
	return lm.log.WithFields(fields)
}

// ======================================================================
= Exclude Checking
// ======================================================================

// shouldExclude checks if a path should be excluded from logging.
func (lm *LoggerMiddleware) shouldExclude(path string) bool {
	if lm.excludes[path] {
		return true
	}
	
	// Check regex patterns
	for _, pattern := range lm.config.ExcludePatterns {
		if pattern.MatchString(path) {
			return true
		}
	}
	
	return false
}

// ======================================================================
= IP Detection
// ======================================================================

// getClientIP extracts the real client IP address.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	
	// Check CF-Connecting-IP (Cloudflare)
	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		return cfIP
	}
	
	// Check True-Client-IP
	if tcIP := r.Header.Get("True-Client-IP"); tcIP != "" {
		return tcIP
	}
	
	// Fallback to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// ======================================================================
= Redaction Helpers
// ======================================================================

// redactHeaders redacts sensitive headers.
func (lm *LoggerMiddleware) redactHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if SensitiveHeaders[strings.ToLower(key)] {
			result[key] = "[REDACTED]"
		} else {
			if len(values) == 1 {
				result[key] = values[0]
			} else {
				result[key] = strings.Join(values, ", ")
			}
		}
	}
	return result
}

// sanitizeBody redacts sensitive data from a body string.
func (lm *LoggerMiddleware) sanitizeBody(body string) string {
	// Try to parse as JSON
	var data interface{}
	if err := json.Unmarshal([]byte(body), &data); err == nil {
		// Recursively redact
		redacted := redactJSON(data)
		if redactedJSON, err := json.Marshal(redacted); err == nil {
			return string(redactedJSON)
		}
	}
	
	// Fallback to string redaction
	return RedactString(body)
}

// redactJSON recursively redacts sensitive data in JSON.
func redactJSON(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			if SensitiveFields[strings.ToLower(key)] {
				result[key] = "[REDACTED]"
			} else {
				result[key] = redactJSON(value)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, value := range v {
			result[i] = redactJSON(value)
		}
		return result
	default:
		return data
	}
}

// redactMap redacts sensitive fields in a map.
func redactMap(data map[string][]string) map[string][]string {
	if data == nil {
		return nil
	}
	result := make(map[string][]string)
	for key, values := range data {
		if SensitiveFields[strings.ToLower(key)] {
			result[key] = []string{"[REDACTED]"}
		} else {
			result[key] = values
		}
	}
	return result
}

// ======================================================================
= Custom Fields
// ======================================================================

// WithField adds a field to the logger context.
func (lm *LoggerMiddleware) WithField(key string, value interface{}) *LoggerMiddleware {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.log = lm.log.WithField(key, value)
	return lm
}

// WithFields adds multiple fields to the logger context.
func (lm *LoggerMiddleware) WithFields(fields logrus.Fields) *LoggerMiddleware {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.log = lm.log.WithFields(fields)
	return lm
}

// ======================================================================
= Context Helpers
// ======================================================================

// GetRequestID extracts request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetRequestStart extracts request start time from context.
func GetRequestStart(ctx context.Context) time.Time {
	if t, ok := ctx.Value(RequestStartKey).(time.Time); ok {
		return t
	}
	return time.Time{}
}

// ======================================================================
= Response Body Logging
// ======================================================================

// LogResponseBody is a helper to log response bodies (use with caution).
type LoggingResponseWriter struct {
	http.ResponseWriter
	buf    *bytes.Buffer
	status int
}

// NewLoggingResponseWriter creates a new response writer that captures the body.
func NewLoggingResponseWriter(w http.ResponseWriter) *LoggingResponseWriter {
	return &LoggingResponseWriter{
		ResponseWriter: w,
		buf:           &bytes.Buffer{},
		status:        http.StatusOK,
	}
}

// WriteHeader captures the status code.
func (lrw *LoggingResponseWriter) WriteHeader(status int) {
	lrw.status = status
	lrw.ResponseWriter.WriteHeader(status)
}

// Write captures the response body.
func (lrw *LoggingResponseWriter) Write(b []byte) (int, error) {
	lrw.buf.Write(b)
	return lrw.ResponseWriter.Write(b)
}

// Body returns the captured response body.
func (lrw *LoggingResponseWriter) Body() string {
	return lrw.buf.String()
}

// Status returns the status code.
func (lrw *LoggingResponseWriter) Status() int {
	return lrw.status
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck checks the health of the logger middleware.
func (lm *LoggerMiddleware) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"component": "logger_middleware",
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"config": map[string]interface{}{
			"log_level":    lm.config.LogLevel.String(),
			"log_headers":  lm.config.LogHeaders,
			"log_body":     lm.config.LogBody,
			"log_query":    lm.config.LogQuery,
			"max_body_size": lm.config.MaxBodySize,
			"exclude_paths": lm.config.ExcludePaths,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ======================================================================
= Convenience Functions
// ======================================================================

// Logger returns a ready-to-use logger middleware with default config.
func Logger() mux.MiddlewareFunc {
	mw := NewLoggerMiddleware(DefaultLoggerConfig())
	return mw.Middleware
}

// LoggerWithConfig returns a middleware with custom config.
func LoggerWithConfig(cfg LoggerConfig) mux.MiddlewareFunc {
	mw := NewLoggerMiddleware(cfg)
	return mw.Middleware
}

// ======================================================================
= Exclude Paths Helper
// ======================================================================

// ExcludePaths adds paths to the exclusion list.
func (lm *LoggerMiddleware) ExcludePaths(paths ...string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	for _, path := range paths {
		lm.excludes[path] = true
	}
}

// ExcludePatterns adds regex patterns to the exclusion list.
func (lm *LoggerMiddleware) ExcludePatterns(patterns ...*regexp.Regexp) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.config.ExcludePatterns = append(lm.config.ExcludePatterns, patterns...)
}

// ======================================================================
= Set Log Level
// ======================================================================

// SetLogLevel changes the log level dynamically.
func (lm *LoggerMiddleware) SetLogLevel(level logrus.Level) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.log.Logger.SetLevel(level)
	lm.config.LogLevel = level
}

// GetLogLevel returns the current log level.
func (lm *LoggerMiddleware) GetLogLevel() logrus.Level {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.config.LogLevel
}

// ======================================================================
= Test Helpers
// ======================================================================

// MockLoggerConfig returns a config suitable for testing.
func MockLoggerConfig() LoggerConfig {
	cfg := DefaultLoggerConfig()
	cfg.LogLevel = logrus.DebugLevel
	cfg.LogBody = true
	cfg.LogHeaders = true
	cfg.StructuredOutput = false
	return cfg
}

// TestLogger returns a logger middleware with mock config.
func TestLogger() mux.MiddlewareFunc {
	mw := NewLoggerMiddleware(MockLoggerConfig())
	return mw.Middleware
}

// ======================================================================
= Additional Helpers
// ======================================================================

// WithRequestID creates a context with a request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}