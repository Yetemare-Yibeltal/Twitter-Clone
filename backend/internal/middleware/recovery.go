// backend/internal/middleware/recovery.go
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// DefaultRecoveryMessage is the default error message sent to clients.
	DefaultRecoveryMessage = "Internal Server Error"
	
	// MaxStackTraceSize is the maximum size of stack trace to capture.
	MaxStackTraceSize = 10 * 1024 // 10KB
)

// ======================================================================
= Panic Types
// ======================================================================

// PanicType represents the type of panic.
type PanicType string

const (
	PanicTypeUnknown     PanicType = "unknown"
	PanicTypeRuntime     PanicType = "runtime"
	PanicTypeMemory      PanicType = "out_of_memory"
	PanicTypeDeadlock    PanicType = "deadlock"
	PanicTypeConcurrent  PanicType = "concurrent"
	PanicTypeCustom      PanicType = "custom"
)

// ======================================================================
= Configuration
// ======================================================================

// RecoveryConfig holds configuration for the recovery middleware.
type RecoveryConfig struct {
	// Enabled determines if recovery is enabled.
	Enabled bool `json:"enabled"`
	
	// LogStack determines if stack trace is logged.
	LogStack bool `json:"log_stack"`
	
	// LogPanic determines if panic details are logged.
	LogPanic bool `json:"log_panic"`
	
	// SendSentry determines if panics are sent to Sentry.
	SendSentry bool `json:"send_sentry"`
	
	// SentryDSN is the Sentry DSN for error reporting.
	SentryDSN string `json:"sentry_dsn"`
	
	// SentryEnvironment is the Sentry environment.
	SentryEnvironment string `json:"sentry_environment"`
	
	// SentryRelease is the Sentry release version.
	SentryRelease string `json:"sentry_release"`
	
	// IncludeRequest determines if request details are included in logs.
	IncludeRequest bool `json:"include_request"`
	
	// IncludeHeaders determines if request headers are included.
	IncludeHeaders bool `json:"include_headers"`
	
	// IncludeBody determines if request body is included.
	IncludeBody bool `json:"include_body"`
	
	// MaxBodySize is the maximum body size to include.
	MaxBodySize int64 `json:"max_body_size"`
	
	// ResponseMessage is the message sent to the client.
	ResponseMessage string `json:"response_message"`
	
	// ResponseStatusCode is the HTTP status code sent to the client.
	ResponseStatusCode int `json:"response_status_code"`
	
	// CustomRecoveryHandler is a custom recovery function.
	CustomRecoveryHandler func(ctx context.Context, r *http.Request, panicVal interface{}, stack []byte) `json:"-"`
	
	// ExcludePaths are paths to exclude from recovery.
	ExcludePaths []string `json:"exclude_paths"`
}

// DefaultRecoveryConfig returns sensible defaults.
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		Enabled:          true,
		LogStack:         true,
		LogPanic:         true,
		SendSentry:       false,
		IncludeRequest:   true,
		IncludeHeaders:   false,
		IncludeBody:      false,
		MaxBodySize:      1024 * 1024, // 1MB
		ResponseMessage:  DefaultRecoveryMessage,
		ResponseStatusCode: http.StatusInternalServerError,
		ExcludePaths:     []string{"/health", "/ready", "/live", "/metrics"},
	}
}

// ======================================================================
= Recovery Middleware
// ======================================================================

// RecoveryMiddleware is the main recovery middleware struct.
type RecoveryMiddleware struct {
	config   RecoveryConfig
	log      *logrus.Entry
	mu       sync.RWMutex
	excludes map[string]bool
}

// NewRecoveryMiddleware creates a new recovery middleware.
func NewRecoveryMiddleware(cfg RecoveryConfig) *RecoveryMiddleware {
	// Build exclude map
	excludes := make(map[string]bool)
	for _, path := range cfg.ExcludePaths {
		excludes[path] = true
	}
	
	return &RecoveryMiddleware{
		config:   cfg,
		log:      logger.WithField("middleware", "recovery"),
		excludes: excludes,
	}
}

// Middleware returns the HTTP middleware handler.
func (rm *RecoveryMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if path should be excluded
		if rm.shouldExclude(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		
		// Get request ID from context
		requestID := GetRequestID(r.Context())
		
		// Defer recovery
		defer func() {
			if err := recover(); err != nil {
				rm.handlePanic(w, r, err, requestID)
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// ======================================================================
= Panic Handling
// ======================================================================

// handlePanic handles a panic and recovers.
func (rm *RecoveryMiddleware) handlePanic(w http.ResponseWriter, r *http.Request, panicVal interface{}, requestID string) {
	// Get stack trace
	stack := debug.Stack()
	
	// Truncate stack if too large
	if len(stack) > MaxStackTraceSize {
		stack = stack[:MaxStackTraceSize]
	}
	
	// Determine panic type
	panicType := rm.determinePanicType(panicVal, stack)
	
	// Build log entry
	logEntry := rm.buildPanicLogEntry(r, panicVal, stack, panicType, requestID)
	
	// Log the panic
	if rm.config.LogPanic {
		logEntry.Error("Panic recovered")
	}
	
	// Log stack trace if enabled
	if rm.config.LogStack {
		logEntry.WithField("stack", string(stack)).Error("Stack trace")
	}
	
	// Send to Sentry if enabled
	if rm.config.SendSentry {
		rm.sendToSentry(r, panicVal, stack, requestID)
	}
	
	// Call custom handler if provided
	if rm.config.CustomRecoveryHandler != nil {
		rm.config.CustomRecoveryHandler(r.Context(), r, panicVal, stack)
	}
	
	// Send response
	rm.sendRecoveryResponse(w, r)
}

// ======================================================================
= Panic Type Detection
// ======================================================================

// determinePanicType determines the type of panic.
func (rm *RecoveryMiddleware) determinePanicType(panicVal interface{}, stack []byte) PanicType {
	// Check if it's a runtime error
	if err, ok := panicVal.(runtime.Error); ok {
		errStr := err.Error()
		if strings.Contains(errStr, "out of memory") {
			return PanicTypeMemory
		}
		if strings.Contains(errStr, "deadlock") {
			return PanicTypeDeadlock
		}
		if strings.Contains(errStr, "concurrent") {
			return PanicTypeConcurrent
		}
		return PanicTypeRuntime
	}
	
	// Check if it's a string
	if s, ok := panicVal.(string); ok {
		if strings.Contains(s, "memory") {
			return PanicTypeMemory
		}
	}
	
	// Check stack for patterns
	stackStr := string(stack)
	if strings.Contains(stackStr, "OutOfMemory") {
		return PanicTypeMemory
	}
	if strings.Contains(stackStr, "Deadlock") {
		return PanicTypeDeadlock
	}
	if strings.Contains(stackStr, "DataRace") {
		return PanicTypeConcurrent
	}
	
	return PanicTypeUnknown
}

// ======================================================================
= Log Entry Building
// ======================================================================

// buildPanicLogEntry builds a log entry for the panic.
func (rm *RecoveryMiddleware) buildPanicLogEntry(r *http.Request, panicVal interface{}, stack []byte, panicType PanicType, requestID string) *logrus.Entry {
	fields := logrus.Fields{
		"panic_type":   panicType,
		"panic_value":  fmt.Sprintf("%v", panicVal),
		"request_id":   requestID,
		"method":       r.Method,
		"path":         r.URL.Path,
		"remote_addr":  r.RemoteAddr,
	}
	
	// Add IP if available
	if ip := getClientIP(r); ip != "" {
		fields["client_ip"] = ip
	}
	
	// Add User-Agent if available
	if ua := r.UserAgent(); ua != "" {
		fields["user_agent"] = ua
	}
	
	// Add headers if enabled
	if rm.config.IncludeHeaders {
		fields["headers"] = rm.redactHeaders(r.Header)
	}
	
	// Add body if enabled
	if rm.config.IncludeBody && r.ContentLength > 0 && r.ContentLength <= rm.config.MaxBodySize {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			// Restore body for downstream
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			fields["request_body"] = rm.sanitizeBody(string(bodyBytes))
		}
	}
	
	// Add goroutine info
	fields["goroutine_id"] = getGoroutineID()
	fields["goroutine_count"] = runtime.NumGoroutine()
	
	// Add memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	fields["memory_alloc"] = memStats.Alloc
	fields["memory_sys"] = memStats.Sys
	fields["memory_gc"] = memStats.NumGC
	
	return rm.log.WithFields(fields)
}

// ======================================================================
= Response Sending
// ======================================================================

// sendRecoveryResponse sends a response to the client.
func (rm *RecoveryMiddleware) sendRecoveryResponse(w http.ResponseWriter, r *http.Request) {
	statusCode := rm.config.ResponseStatusCode
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}
	
	message := rm.config.ResponseMessage
	if message == "" {
		message = DefaultRecoveryMessage
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	resp := map[string]interface{}{
		"error":     message,
		"status":    statusCode,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	// Add request ID if available
	if requestID := GetRequestID(r.Context()); requestID != "" {
		resp["request_id"] = requestID
	}
	
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		rm.log.WithError(err).Error("Failed to encode recovery response")
		http.Error(w, message, statusCode)
	}
}

// ======================================================================
= Sentry Integration
// ======================================================================

// sendToSentry sends panic details to Sentry.
func (rm *RecoveryMiddleware) sendToSentry(r *http.Request, panicVal interface{}, stack []byte, requestID string) {
	if rm.config.SentryDSN == "" {
		return
	}
	
	// Use a goroutine to avoid blocking the request
	go func() {
		defer func() {
			if err := recover(); err != nil {
				// Don't panic while sending to Sentry
				rm.log.WithField("sentry_panic", err).Error("Sentry integration panicked")
			}
		}()
		
		// This is a placeholder for Sentry integration
		// In production, you would use the Sentry SDK:
		// sentry.Init(sentry.ClientOptions{
		//     DSN: rm.config.SentryDSN,
		//     Environment: rm.config.SentryEnvironment,
		//     Release: rm.config.SentryRelease,
		// })
		// defer sentry.Flush(2 * time.Second)
		// 
		// hub := sentry.CurrentHub()
		// event := sentry.NewEvent()
		// event.Message = fmt.Sprintf("Panic recovered: %v", panicVal)
		// event.Extra = map[string]interface{}{
		//     "request_id": requestID,
		//     "method": r.Method,
		//     "path": r.URL.Path,
		//     "remote_addr": r.RemoteAddr,
		// }
		// event.Exception = []sentry.Exception{
		//     {
		//         Type:  "panic",
		//         Value: fmt.Sprintf("%v", panicVal),
		//         Stacktrace: sentry.ExtractStacktrace(debug.Stack()),
		//     },
		// }
		// hub.CaptureEvent(event)
		
		rm.log.WithFields(logrus.Fields{
			"dsn":        rm.config.SentryDSN,
			"environment": rm.config.SentryEnvironment,
			"release":    rm.config.SentryRelease,
		}).Debug("Sentry integration would send panic")
	}()
}

// ======================================================================
= Exclude Checking
// ======================================================================

// shouldExclude checks if a path should be excluded from recovery.
func (rm *RecoveryMiddleware) shouldExclude(path string) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.excludes[path]
}

// ======================================================================
= Configuration Management
// ======================================================================

// SetExcludePaths sets the exclude paths.
func (rm *RecoveryMiddleware) SetExcludePaths(paths ...string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.excludes = make(map[string]bool)
	for _, path := range paths {
		rm.excludes[path] = true
	}
}

// AddExcludePath adds a path to the exclude list.
func (rm *RecoveryMiddleware) AddExcludePath(path string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.excludes[path] = true
}

// RemoveExcludePath removes a path from the exclude list.
func (rm *RecoveryMiddleware) RemoveExcludePath(path string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.excludes, path)
}

// SetResponseMessage sets the response message.
func (rm *RecoveryMiddleware) SetResponseMessage(message string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.config.ResponseMessage = message
}

// SetResponseStatusCode sets the response status code.
func (rm *RecoveryMiddleware) SetResponseStatusCode(statusCode int) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.config.ResponseStatusCode = statusCode
}

// ======================================================================
= Helper Functions
// ======================================================================

// getGoroutineID returns the current goroutine ID.
func getGoroutineID() uint64 {
	// This is a hack to get goroutine ID from the stack
	buf := make([]byte, 64)
	n := runtime.Stack(buf, false)
	buf = buf[:n]
	// Parse: "goroutine 123 [running]:"
	fields := strings.Fields(string(buf))
	if len(fields) >= 2 {
		var id uint64
		fmt.Sscanf(fields[1], "%d", &id)
		return id
	}
	return 0
}

// redactHeaders redacts sensitive headers.
func (rm *RecoveryMiddleware) redactHeaders(headers http.Header) map[string]string {
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
func (rm *RecoveryMiddleware) sanitizeBody(body string) string {
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

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck checks the health of the recovery middleware.
func (rm *RecoveryMiddleware) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"component": "recovery_middleware",
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"config": map[string]interface{}{
			"enabled":    rm.config.Enabled,
			"log_stack":  rm.config.LogStack,
			"log_panic":  rm.config.LogPanic,
			"send_sentry": rm.config.SendSentry,
			"exclude_paths": rm.config.ExcludePaths,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ======================================================================
= Convenience Functions
// ======================================================================

// Recovery returns a ready-to-use recovery middleware with default config.
func Recovery() mux.MiddlewareFunc {
	mw := NewRecoveryMiddleware(DefaultRecoveryConfig())
	return mw.Middleware
}

// RecoveryWithConfig returns a middleware with custom config.
func RecoveryWithConfig(cfg RecoveryConfig) mux.MiddlewareFunc {
	mw := NewRecoveryMiddleware(cfg)
	return mw.Middleware
}

// ======================================================================
= Custom Recovery Handler
// ======================================================================

// CustomRecoveryHandler is a function type for custom recovery handling.
type CustomRecoveryHandler func(ctx context.Context, r *http.Request, panicVal interface{}, stack []byte)

// SetCustomRecoveryHandler sets a custom recovery handler.
func (rm *RecoveryMiddleware) SetCustomRecoveryHandler(handler CustomRecoveryHandler) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.config.CustomRecoveryHandler = handler
}

// ======================================================================
= Panic Simulation (for testing)
// ======================================================================

// SimulatePanic simulates a panic for testing purposes.
func (rm *RecoveryMiddleware) SimulatePanic(w http.ResponseWriter, r *http.Request) {
	panic("simulated panic for testing")
}

// SimulateMemoryPanic simulates an out-of-memory panic.
func (rm *RecoveryMiddleware) SimulateMemoryPanic(w http.ResponseWriter, r *http.Request) {
	// Force memory panic by allocating a huge slice
	_ = make([]byte, 10<<30) // 10GB
}

// ======================================================================
= Test Helpers
// ======================================================================

// MockRecoveryConfig returns a config suitable for testing.
func MockRecoveryConfig() RecoveryConfig {
	cfg := DefaultRecoveryConfig()
	cfg.LogStack = true
	cfg.LogPanic = true
	cfg.IncludeRequest = true
	cfg.IncludeHeaders = true
	cfg.IncludeBody = true
	cfg.SendSentry = false
	return cfg
}

// TestRecovery returns a recovery middleware with mock config.
func TestRecovery() mux.MiddlewareFunc {
	mw := NewRecoveryMiddleware(MockRecoveryConfig())
	return mw.Middleware
}

// ======================================================================
= Recovery Statistics
// ======================================================================

// RecoveryStats holds statistics about panics.
type RecoveryStats struct {
	TotalPanics    int64            `json:"total_panics"`
	PanicsByType   map[PanicType]int64 `json:"panics_by_type"`
	LastPanic      time.Time        `json:"last_panic"`
	LastPanicType  PanicType        `json:"last_panic_type"`
	LastPanicPath  string           `json:"last_panic_path"`
}

// GetStats returns statistics about panics.
func (rm *RecoveryMiddleware) GetStats() RecoveryStats {
	// This would require metrics integration
	// For now, return empty stats
	return RecoveryStats{
		TotalPanics:  0,
		PanicsByType: make(map[PanicType]int64),
	}
}

// ======================================================================
= Panic Classification
// ======================================================================

// IsOutOfMemoryPanic checks if the panic is an out-of-memory error.
func IsOutOfMemoryPanic(panicVal interface{}) bool {
	if err, ok := panicVal.(runtime.Error); ok {
		return strings.Contains(err.Error(), "out of memory")
	}
	if s, ok := panicVal.(string); ok {
		return strings.Contains(s, "out of memory") || strings.Contains(s, "OutOfMemory")
	}
	return false
}

// IsDeadlockPanic checks if the panic is a deadlock.
func IsDeadlockPanic(panicVal interface{}) bool {
	if err, ok := panicVal.(runtime.Error); ok {
		return strings.Contains(err.Error(), "deadlock")
	}
	if s, ok := panicVal.(string); ok {
		return strings.Contains(s, "deadlock")
	}
	return false
}

// IsConcurrentPanic checks if the panic is a concurrent access error.
func IsConcurrentPanic(panicVal interface{}) bool {
	if err, ok := panicVal.(runtime.Error); ok {
		return strings.Contains(err.Error(), "concurrent") || strings.Contains(err.Error(), "race")
	}
	if s, ok := panicVal.(string); ok {
		return strings.Contains(s, "concurrent") || strings.Contains(s, "race")
	}
	return false
}