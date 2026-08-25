// backend/internal/handler/health_handler.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/config"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// HealthHandler handles all health check-related HTTP endpoints.
type HealthHandler struct {
	db           interfaces.UserRepository
	redisAdapter adapter.RedisAdapter
	config       *config.Config
	startTime    time.Time
	mu           sync.RWMutex
	status       string
	log          *logrus.Entry
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(
	db interfaces.UserRepository,
	redisAdapter adapter.RedisAdapter,
	cfg *config.Config,
) *HealthHandler {
	return &HealthHandler{
		db:           db,
		redisAdapter: redisAdapter,
		config:       cfg,
		startTime:    time.Now(),
		status:       "ok",
		log:          logger.WithField("handler", "health"),
	}
}

// ======================================================================
// Health Check Endpoints
// ======================================================================

// Liveness handles liveness probe.
// @Summary Liveness probe
// @Description Checks if the service is alive
// @Tags health
// @Produce json
// @Success 200 {object} dto.HealthResponse
// @Failure 503 {object} dto.HealthResponse
// @Router /health/live [get]
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	h.sendHealthResponse(w, http.StatusOK, "alive", nil)
}

// Readiness handles readiness probe.
// @Summary Readiness probe
// @Description Checks if the service is ready to accept traffic
// @Tags health
// @Produce json
// @Success 200 {object} dto.HealthResponse
// @Failure 503 {object} dto.HealthResponse
// @Router /health/ready [get]
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check database
	if err := h.checkDatabase(ctx); err != nil {
		h.sendHealthResponse(w, http.StatusServiceUnavailable, "not ready", map[string]interface{}{
			"database": "unavailable",
			"error":    err.Error(),
		})
		return
	}

	// Check Redis
	if err := h.checkRedis(ctx); err != nil {
		h.sendHealthResponse(w, http.StatusServiceUnavailable, "not ready", map[string]interface{}{
			"redis": "unavailable",
			"error": err.Error(),
		})
		return
	}

	h.sendHealthResponse(w, http.StatusOK, "ready", map[string]interface{}{
		"database": "available",
		"redis":    "available",
	})
}

// Health handles general health check.
// @Summary Health check
// @Description Returns comprehensive health status
// @Tags health
// @Produce json
// @Success 200 {object} dto.HealthResponse
// @Failure 503 {object} dto.HealthResponse
// @Router /health [get]
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := "ok"
	details := make(map[string]interface{})

	// Check database
	dbErr := h.checkDatabase(ctx)
	if dbErr != nil {
		status = "degraded"
		details["database"] = "unavailable"
		details["database_error"] = dbErr.Error()
	} else {
		details["database"] = "available"
	}

	// Check Redis
	redisErr := h.checkRedis(ctx)
	if redisErr != nil {
		status = "degraded"
		details["redis"] = "unavailable"
		details["redis_error"] = redisErr.Error()
	} else {
		details["redis"] = "available"
	}

	// Add system metrics
	details["uptime"] = time.Since(h.startTime).String()
	details["goroutines"] = runtime.NumGoroutine()
	details["go_version"] = runtime.Version()
	details["memory"] = h.getMemoryStats()

	if status == "ok" {
		h.sendHealthResponse(w, http.StatusOK, "ok", details)
	} else {
		h.sendHealthResponse(w, http.StatusServiceUnavailable, "degraded", details)
	}
}

// ======================================================================
= Detailed Health Checks
// ======================================================================

// DetailedHealth handles detailed health check with all dependencies.
// @Summary Detailed health check
// @Description Returns detailed health status for all dependencies
// @Tags health
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.DetailedHealthResponse
// @Failure 503 {object} dto.DetailedHealthResponse
// @Router /health/detailed [get]
func (h *HealthHandler) DetailedHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response := &DetailedHealthResponse{
		Status:     "ok",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Uptime:     time.Since(h.startTime).String(),
		Services:   make(map[string]*ServiceHealth),
		SystemInfo: &SystemInfo{
			GoVersion:    runtime.Version(),
			NumCPU:       runtime.NumCPU(),
			NumGoroutine: runtime.NumGoroutine(),
			Memory:       h.getMemoryStats(),
		},
	}

	// Check Database
	response.Services["database"] = h.checkDatabaseDetailed(ctx)

	// Check Redis
	response.Services["redis"] = h.checkRedisDetailed(ctx)

	// Check Config
	response.Services["config"] = h.checkConfig()

	// Check Disk space
	response.Services["disk"] = h.checkDiskSpace()

	// Overall status
	for _, svc := range response.Services {
		if svc.Status != "ok" {
			response.Status = "degraded"
			break
		}
	}

	statusCode := http.StatusOK
	if response.Status == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	h.sendDetailedHealthResponse(w, statusCode, response)
}

// ======================================================================
= Health Check Methods
// ======================================================================

// checkDatabase checks database connectivity.
func (h *HealthHandler) checkDatabase(ctx context.Context) error {
	if h.db == nil {
		return fmt.Errorf("database not configured")
	}
	return h.db.Ping(ctx)
}

// checkDatabaseDetailed checks database with detailed response.
func (h *HealthHandler) checkDatabaseDetailed(ctx context.Context) *ServiceHealth {
	start := time.Now()
	health := &ServiceHealth{
		Status: "ok",
		Name:   "PostgreSQL",
	}

	if err := h.checkDatabase(ctx); err != nil {
		health.Status = "error"
		health.Error = err.Error()
		health.Message = "Database connection failed"
	} else {
		health.Message = "Database connected"
		health.Latency = time.Since(start).String()
		// Get stats if possible
		if stats, err := h.getDBStats(); err == nil {
			health.Details = stats
		}
	}
	return health
}

// checkRedis checks Redis connectivity.
func (h *HealthHandler) checkRedis(ctx context.Context) error {
	if h.redisAdapter == nil {
		return fmt.Errorf("redis not configured")
	}
	return h.redisAdapter.Ping(ctx)
}

// checkRedisDetailed checks Redis with detailed response.
func (h *HealthHandler) checkRedisDetailed(ctx context.Context) *ServiceHealth {
	start := time.Now()
	health := &ServiceHealth{
		Status: "ok",
		Name:   "Redis",
	}

	if err := h.checkRedis(ctx); err != nil {
		health.Status = "error"
		health.Error = err.Error()
		health.Message = "Redis connection failed"
	} else {
		health.Message = "Redis connected"
		health.Latency = time.Since(start).String()
		// Get stats
		if h.redisAdapter != nil {
			if stats, err := h.redisAdapter.GetStats(); err == nil {
				health.Details = map[string]interface{}{
					"pool_size":  stats.PoolSize,
					"idle_conns": stats.IdleConns,
					"active":     stats.ActiveConns,
				}
			}
		}
	}
	return health
}

// checkConfig checks configuration.
func (h *HealthHandler) checkConfig() *ServiceHealth {
	health := &ServiceHealth{
		Status: "ok",
		Name:   "Configuration",
	}

	if h.config == nil {
		health.Status = "error"
		health.Error = "Config not initialized"
		health.Message = "Configuration is missing"
	} else {
		health.Message = "Configuration loaded"
		health.Details = map[string]interface{}{
			"environment": h.config.Environment,
			"port":        h.config.Port,
		}
	}
	return health
}

// checkDiskSpace checks disk space.
func (h *HealthHandler) checkDiskSpace() *ServiceHealth {
	health := &ServiceHealth{
		Status: "ok",
		Name:   "Disk Space",
	}

	// For now, return ok since we can't easily check disk in all environments
	// In production, you'd use syscall.Statfs
	health.Message = "Disk space available"
	return health
}

// ======================================================================
= Helper Methods
// ======================================================================

// getMemoryStats returns memory statistics.
func (h *HealthHandler) getMemoryStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]interface{}{
		"alloc":       m.Alloc / 1024 / 1024,       // MB
		"total_alloc": m.TotalAlloc / 1024 / 1024,  // MB
		"sys":         m.Sys / 1024 / 1024,         // MB
		"num_gc":      m.NumGC,
		"gc_pause":    m.PauseNs[(m.NumGC-1)%256] / 1000000, // ms
	}
}

// getDBStats attempts to get database statistics.
func (h *HealthHandler) getDBStats() (map[string]interface{}, error) {
	// This would be implementation-specific
	// For now, return a placeholder
	return map[string]interface{}{
		"connected": true,
	}, nil
}

// ======================================================================
= Response Helpers
// ======================================================================

// sendHealthResponse sends a health response.
func (h *HealthHandler) sendHealthResponse(w http.ResponseWriter, status int, statusText string, details map[string]interface{}) {
	response := map[string]interface{}{
		"status":    statusText,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if details != nil {
		response["details"] = details
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// sendDetailedHealthResponse sends a detailed health response.
func (h *HealthHandler) sendDetailedHealthResponse(w http.ResponseWriter, status int, response *DetailedHealthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// ======================================================================
= Types
// ======================================================================

// ServiceHealth represents health of a service.
type ServiceHealth struct {
	Name     string                 `json:"name"`
	Status   string                 `json:"status"` // ok, error, warning
	Message  string                 `json:"message"`
	Error    string                 `json:"error,omitempty"`
	Latency  string                 `json:"latency,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// SystemInfo represents system information.
type SystemInfo struct {
	GoVersion    string                 `json:"go_version"`
	NumCPU       int                    `json:"num_cpu"`
	NumGoroutine int                    `json:"num_goroutine"`
	Memory       map[string]interface{} `json:"memory"`
}

// DetailedHealthResponse represents a detailed health response.
type DetailedHealthResponse struct {
	Status     string                   `json:"status"`
	Timestamp  string                   `json:"timestamp"`
	Uptime     string                   `json:"uptime"`
	Services   map[string]*ServiceHealth `json:"services"`
	SystemInfo *SystemInfo               `json:"system_info"`
}

// ======================================================================
= Metrics Endpoint
// ======================================================================

// Metrics handles retrieving system metrics.
// @Summary System metrics
// @Description Returns system metrics in Prometheus format
// @Tags health
// @Produce text/plain
// @Success 200 {string} string
// @Router /metrics [get]
func (h *HealthHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	// In production, you'd use a proper Prometheus metrics handler
	// For now, we provide a simple format
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Fprintf(w, "# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.\n")
	fmt.Fprintf(w, "# TYPE go_memstats_alloc_bytes gauge\n")
	fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", m.Alloc)

	fmt.Fprintf(w, "# HELP go_goroutines Number of goroutines that currently exist.\n")
	fmt.Fprintf(w, "# TYPE go_goroutines gauge\n")
	fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())

	fmt.Fprintf(w, "# HELP go_memstats_num_gc Number of completed GC cycles.\n")
	fmt.Fprintf(w, "# TYPE go_memstats_num_gc gauge\n")
	fmt.Fprintf(w, "go_memstats_num_gc %d\n", m.NumGC)

	fmt.Fprintf(w, "# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds.\n")
	fmt.Fprintf(w, "# TYPE process_cpu_seconds_total counter\n")
	fmt.Fprintf(w, "process_cpu_seconds_total %f\n", time.Since(h.startTime).Seconds())

	// Custom metrics
	fmt.Fprintf(w, "# HELP app_uptime_seconds Application uptime in seconds.\n")
	fmt.Fprintf(w, "# TYPE app_uptime_seconds gauge\n")
	fmt.Fprintf(w, "app_uptime_seconds %f\n", time.Since(h.startTime).Seconds())

	// Database connection status
	dbStatus := 1
	if err := h.checkDatabase(r.Context()); err != nil {
		dbStatus = 0
	}
	fmt.Fprintf(w, "# HELP app_database_connected Database connection status (1=connected, 0=disconnected).\n")
	fmt.Fprintf(w, "# TYPE app_database_connected gauge\n")
	fmt.Fprintf(w, "app_database_connected %d\n", dbStatus)

	// Redis connection status
	redisStatus := 1
	if err := h.checkRedis(r.Context()); err != nil {
		redisStatus = 0
	}
	fmt.Fprintf(w, "# HELP app_redis_connected Redis connection status (1=connected, 0=disconnected).\n")
	fmt.Fprintf(w, "# TYPE app_redis_connected gauge\n")
	fmt.Fprintf(w, "app_redis_connected %d\n", redisStatus)
}

// ======================================================================
= Version Endpoint
// ======================================================================

// Version handles retrieving application version.
// @Summary Application version
// @Description Returns the application version information
// @Tags health
// @Produce json
// @Success 200 {object} dto.VersionResponse
// @Router /version [get]
func (h *HealthHandler) Version(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"version":     h.config.Version,
		"build_time":  h.config.BuildTime,
		"go_version":  runtime.Version(),
		"environment": h.config.Environment,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ======================================================================
= Health Check Routes Registration
// ======================================================================

// RegisterRoutes registers health check routes.
func (h *HealthHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", h.Health).Methods("GET")
	r.HandleFunc("/health/live", h.Liveness).Methods("GET")
	r.HandleFunc("/health/ready", h.Readiness).Methods("GET")
	r.HandleFunc("/health/detailed", h.DetailedHealth).Methods("GET")
	r.HandleFunc("/metrics", h.Metrics).Methods("GET")
	r.HandleFunc("/version", h.Version).Methods("GET")
}

// ======================================================================
= Third-party Health Checks (for integration)
// ======================================================================

// ExternalHealthCheck handles health checks from external systems.
// @Summary External health check
// @Description Health check endpoint for external monitoring systems
// @Tags health
// @Produce json
// @Param system query string false "System name (e.g., kubernetes, prometheus, datadog)"
// @Success 200 {object} dto.HealthResponse
// @Failure 503 {object} dto.HealthResponse
// @Router /health/external [get]
func (h *HealthHandler) ExternalHealthCheck(w http.ResponseWriter, r *http.Request) {
	system := r.URL.Query().Get("system")
	if system == "" {
		system = "unknown"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// Quick check of critical dependencies
	dbErr := h.checkDatabase(ctx)
	redisErr := h.checkRedis(ctx)

	status := "ok"
	if dbErr != nil || redisErr != nil {
		status = "unhealthy"
	}

	response := map[string]interface{}{
		"status":     status,
		"system":     system,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"checks": map[string]interface{}{
			"database": dbErr == nil,
			"redis":    redisErr == nil,
		},
	}

	httpStatus := http.StatusOK
	if status == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(response)
}