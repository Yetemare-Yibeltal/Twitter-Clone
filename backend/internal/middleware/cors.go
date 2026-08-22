// backend/internal/middleware/cors.go
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
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
	// CORS headers
	HeaderOrigin          = "Origin"
	HeaderAccessControlRequestMethod    = "Access-Control-Request-Method"
	HeaderAccessControlRequestHeaders   = "Access-Control-Request-Headers"
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlExposeHeaders    = "Access-Control-Expose-Headers"
	HeaderAccessControlMaxAge           = "Access-Control-Max-Age"
	HeaderVary                          = "Vary"

	// Default values
	DefaultMaxAge = 86400 // 24 hours
)

// ======================================================================
= Configuration
// ======================================================================

// CORSConfig holds all CORS configuration.
type CORSConfig struct {
	// AllowedOrigins is a list of allowed origins.
	// Use "*" for any origin, or specific URLs.
	AllowedOrigins []string `json:"allowed_origins"`
	
	// AllowedOriginPatterns is a list of regex patterns for allowed origins.
	AllowedOriginPatterns []string `json:"allowed_origin_patterns"`
	
	// AllowedMethods is a list of allowed HTTP methods.
	AllowedMethods []string `json:"allowed_methods"`
	
	// AllowedHeaders is a list of allowed request headers.
	AllowedHeaders []string `json:"allowed_headers"`
	
	// ExposedHeaders is a list of headers exposed to the client.
	ExposedHeaders []string `json:"exposed_headers"`
	
	// AllowCredentials indicates if credentials (cookies, auth) are allowed.
	AllowCredentials bool `json:"allow_credentials"`
	
	// MaxAge is the maximum age (in seconds) for preflight cache.
	MaxAge int `json:"max_age"`
	
	// OptionsPassthrough allows passing OPTIONS requests through.
	OptionsPassthrough bool `json:"options_passthrough"`
	
	// Debug enables detailed logging.
	Debug bool `json:"debug"`
	
	// AllowAllOrigins is a shortcut for "*".
	AllowAllOrigins bool `json:"allow_all_origins"`
	
	// AllowSubdomains allows subdomains of allowed origins.
	AllowSubdomains bool `json:"allow_subdomains"`
	
	// AllowWildcard allows wildcard patterns like *.example.com.
	AllowWildcard bool `json:"allow_wildcard"`
	
	// SkipValidation skips origin validation (for testing).
	SkipValidation bool `json:"skip_validation"`
}

// DefaultCORSConfig returns sensible defaults.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:     []string{"http://localhost:3000", "https://localhost:3000"},
		AllowedOriginPatterns: []string{},
		AllowedMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"},
		AllowedHeaders:     []string{"Accept", "Accept-Language", "Content-Type", "Authorization", "X-Requested-With", "X-Access-Token"},
		ExposedHeaders:     []string{"Content-Length", "Content-Type", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials:   true,
		MaxAge:             DefaultMaxAge,
		OptionsPassthrough: false,
		Debug:              false,
		AllowAllOrigins:    false,
		AllowSubdomains:    true,
		AllowWildcard:      true,
		SkipValidation:     false,
	}
}

// ======================================================================
= CORS Middleware
// ======================================================================

// CORSMiddleware is the main CORS middleware struct.
type CORSMiddleware struct {
	config    CORSConfig
	log       *logrus.Entry
	mu        sync.RWMutex
	patterns  []*regexp.Regexp
	cache     map[string]bool // origin -> allowed
	cacheLock sync.RWMutex
}

// NewCORSMiddleware creates a new CORS middleware.
func NewCORSMiddleware(cfg CORSConfig) (*CORSMiddleware, error) {
	// Compile regex patterns
	patterns := make([]*regexp.Regexp, 0, len(cfg.AllowedOriginPatterns))
	for _, pattern := range cfg.AllowedOriginPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid origin pattern %s: %w", pattern, err)
		}
		patterns = append(patterns, re)
	}
	
	// Validate config
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	
	return &CORSMiddleware{
		config:   cfg,
		log:      logger.WithField("middleware", "cors"),
		patterns: patterns,
		cache:    make(map[string]bool),
	}, nil
}

// validateConfig validates CORS configuration.
func validateConfig(cfg CORSConfig) error {
	if cfg.AllowAllOrigins && len(cfg.AllowedOrigins) > 0 {
		return errors.New("cannot have AllowAllOrigins true with specific AllowedOrigins")
	}
	if cfg.AllowAllOrigins && len(cfg.AllowedOriginPatterns) > 0 {
		return errors.New("cannot have AllowAllOrigins true with specific AllowedOriginPatterns")
	}
	for _, origin := range cfg.AllowedOrigins {
		if origin == "*" && cfg.AllowCredentials {
			// This is valid with credentials? Actually no, but we allow it.
			// Log a warning.
			// In practice, "*" with credentials is not allowed by browsers.
		}
	}
	return nil
}

// Middleware returns the HTTP middleware handler.
func (c *CORSMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle preflight
		if r.Method == http.MethodOptions {
			c.log.WithFields(logrus.Fields{
				"origin": r.Header.Get(HeaderOrigin),
				"method": r.Header.Get(HeaderAccessControlRequestMethod),
				"headers": r.Header.Get(HeaderAccessControlRequestHeaders),
			}).Debug("Preflight request received")
			
			if c.handlePreflight(w, r) {
				if c.config.OptionsPassthrough {
					next.ServeHTTP(w, r)
				}
				return
			}
			// If not handled, continue
		}
		
		// Process actual request
		origin := r.Header.Get(HeaderOrigin)
		if origin == "" {
			// No origin header, treat as same-origin
			next.ServeHTTP(w, r)
			return
		}
		
		// Check if origin is allowed
		if !c.isOriginAllowed(origin) {
			c.log.WithField("origin", origin).Warn("Origin not allowed")
			http.Error(w, "CORS: origin not allowed", http.StatusForbidden)
			return
		}
		
		// Set CORS headers
		c.setCORSHeaders(w, origin)
		
		// Set Vary header
		w.Header().Add(HeaderVary, HeaderOrigin)
		
		next.ServeHTTP(w, r)
	})
}

// ======================================================================
= Preflight Handling
// ======================================================================

// handlePreflight handles OPTIONS preflight requests.
// Returns true if the preflight was handled.
func (c *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get(HeaderOrigin)
	if origin == "" {
		return false
	}
	
	// Check if origin is allowed
	if !c.isOriginAllowed(origin) {
		c.log.WithField("origin", origin).Warn("Preflight origin not allowed")
		w.WriteHeader(http.StatusForbidden)
		return true
	}
	
	// Check request method
	requestMethod := r.Header.Get(HeaderAccessControlRequestMethod)
	if requestMethod != "" {
		if !c.isMethodAllowed(requestMethod) {
			c.log.WithField("method", requestMethod).Warn("Preflight method not allowed")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return true
		}
	}
	
	// Check request headers
	requestHeaders := r.Header.Get(HeaderAccessControlRequestHeaders)
	if requestHeaders != "" {
		if !c.areHeadersAllowed(requestHeaders) {
			c.log.WithField("headers", requestHeaders).Warn("Preflight headers not allowed")
			w.WriteHeader(http.StatusForbidden)
			return true
		}
	}
	
	// Set CORS headers
	c.setCORSHeaders(w, origin)
	
	// Set allowed methods
	if requestMethod != "" {
		w.Header().Set(HeaderAccessControlAllowMethods, strings.Join(c.config.AllowedMethods, ", "))
	}
	
	// Set allowed headers
	if requestHeaders != "" {
		w.Header().Set(HeaderAccessControlAllowHeaders, strings.Join(c.config.AllowedHeaders, ", "))
	}
	
	// Set max age
	if c.config.MaxAge > 0 {
		w.Header().Set(HeaderAccessControlMaxAge, fmt.Sprintf("%d", c.config.MaxAge))
	}
	
	// Set Vary header
	w.Header().Add(HeaderVary, HeaderOrigin)
	w.Header().Add(HeaderVary, HeaderAccessControlRequestMethod)
	w.Header().Add(HeaderVary, HeaderAccessControlRequestHeaders)
	
	w.WriteHeader(http.StatusNoContent)
	return true
}

// ======================================================================
= Origin Validation
// ======================================================================

// isOriginAllowed checks if an origin is allowed.
func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	if c.config.SkipValidation {
		return true
	}
	
	if c.config.AllowAllOrigins {
		return true
	}
	
	// Check cache
	c.cacheLock.RLock()
	if allowed, exists := c.cache[origin]; exists {
		c.cacheLock.RUnlock()
		return allowed
	}
	c.cacheLock.RUnlock()
	
	// Check exact matches
	for _, allowed := range c.config.AllowedOrigins {
		if allowed == "*" {
			c.cacheLock.Lock()
			c.cache[origin] = true
			c.cacheLock.Unlock()
			return true
		}
		if strings.EqualFold(origin, allowed) {
			c.cacheLock.Lock()
			c.cache[origin] = true
			c.cacheLock.Unlock()
			return true
		}
	}
	
	// Check subdomain matching
	if c.config.AllowSubdomains {
		for _, allowed := range c.config.AllowedOrigins {
			if isSubdomain(origin, allowed) {
				c.cacheLock.Lock()
				c.cache[origin] = true
				c.cacheLock.Unlock()
				return true
			}
		}
	}
	
	// Check wildcard matching
	if c.config.AllowWildcard {
		for _, allowed := range c.config.AllowedOrigins {
			if matchWildcard(origin, allowed) {
				c.cacheLock.Lock()
				c.cache[origin] = true
				c.cacheLock.Unlock()
				return true
			}
		}
	}
	
	// Check regex patterns
	for _, pattern := range c.patterns {
		if pattern.MatchString(origin) {
			c.cacheLock.Lock()
			c.cache[origin] = true
			c.cacheLock.Unlock()
			return true
		}
	}
	
	// Not allowed
	c.cacheLock.Lock()
	c.cache[origin] = false
	c.cacheLock.Unlock()
	return false
}

// isSubdomain checks if origin is a subdomain of allowed.
func isSubdomain(origin, allowed string) bool {
	// Remove protocol
	origin = removeProtocol(origin)
	allowed = removeProtocol(allowed)
	
	if origin == allowed {
		return true
	}
	
	// Check if allowed is a subdomain of origin (or vice versa)
	if strings.HasSuffix(origin, "."+allowed) {
		return true
	}
	if strings.HasSuffix(allowed, "."+origin) {
		return true
	}
	return false
}

// matchWildcard checks if origin matches a wildcard pattern.
func matchWildcard(origin, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return false
	}
	// Simple wildcard: *.example.com
	origin = removeProtocol(origin)
	pattern = removeProtocol(pattern)
	
	// Check if pattern starts with *.
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // .example.com
		return strings.HasSuffix(origin, suffix)
	}
	
	// Check if pattern ends with .*
	if strings.HasSuffix(pattern, ".*") {
		prefix := pattern[:len(pattern)-2]
		return strings.HasPrefix(origin, prefix)
	}
	
	// Check for * in the middle
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(origin, parts[0]) && strings.HasSuffix(origin, parts[1])
		}
	}
	return false
}

// removeProtocol removes http:// or https:// from a URL.
func removeProtocol(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	return url
}

// isMethodAllowed checks if a method is allowed.
func (c *CORSMiddleware) isMethodAllowed(method string) bool {
	method = strings.ToUpper(method)
	for _, allowed := range c.config.AllowedMethods {
		if strings.EqualFold(allowed, method) {
			return true
		}
	}
	return false
}

// areHeadersAllowed checks if request headers are allowed.
func (c *CORSMiddleware) areHeadersAllowed(headers string) bool {
	requested := strings.Split(headers, ",")
	for _, header := range requested {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		allowed := false
		for _, allowedHeader := range c.config.AllowedHeaders {
			if strings.EqualFold(allowedHeader, header) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}

// ======================================================================
= Response Headers
// ======================================================================

// setCORSHeaders sets CORS response headers.
func (c *CORSMiddleware) setCORSHeaders(w http.ResponseWriter, origin string) {
	// Set Access-Control-Allow-Origin
	if c.config.AllowAllOrigins {
		w.Header().Set(HeaderAccessControlAllowOrigin, "*")
	} else {
		w.Header().Set(HeaderAccessControlAllowOrigin, origin)
	}
	
	// Set Access-Control-Allow-Credentials
	if c.config.AllowCredentials {
		w.Header().Set(HeaderAccessControlAllowCredentials, "true")
	}
	
	// Set Access-Control-Expose-Headers
	if len(c.config.ExposedHeaders) > 0 {
		w.Header().Set(HeaderAccessControlExposeHeaders, strings.Join(c.config.ExposedHeaders, ", "))
	}
}

// ======================================================================
= Additional Helpers
// ======================================================================

// GetCORSConfig returns the current CORS configuration.
func (c *CORSMiddleware) GetCORSConfig() CORSConfig {
	return c.config
}

// UpdateAllowedOrigins updates the allowed origins list.
func (c *CORSMiddleware) UpdateAllowedOrigins(origins []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.AllowedOrigins = origins
	// Clear cache
	c.cacheLock.Lock()
	c.cache = make(map[string]bool)
	c.cacheLock.Unlock()
	c.log.WithField("origins", origins).Info("Updated allowed origins")
}

// AddAllowedOrigin adds a single origin to the allowlist.
func (c *CORSMiddleware) AddAllowedOrigin(origin string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.AllowedOrigins = append(c.config.AllowedOrigins, origin)
	c.cacheLock.Lock()
	c.cache = make(map[string]bool)
	c.cacheLock.Unlock()
	c.log.WithField("origin", origin).Info("Added allowed origin")
}

// RemoveAllowedOrigin removes an origin from the allowlist.
func (c *CORSMiddleware) RemoveAllowedOrigin(origin string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	newOrigins := make([]string, 0, len(c.config.AllowedOrigins))
	for _, o := range c.config.AllowedOrigins {
		if o != origin {
			newOrigins = append(newOrigins, o)
		}
	}
	c.config.AllowedOrigins = newOrigins
	c.cacheLock.Lock()
	c.cache = make(map[string]bool)
	c.cacheLock.Unlock()
	c.log.WithField("origin", origin).Info("Removed allowed origin")
}

// ClearCache clears the origin validation cache.
func (c *CORSMiddleware) ClearCache() {
	c.cacheLock.Lock()
	c.cache = make(map[string]bool)
	c.cacheLock.Unlock()
}

// ======================================================================
= Security Headers (Optional)
// ======================================================================

// SecurityHeaders adds additional security headers.
// This can be combined with CORS middleware.
type SecurityHeaders struct {
	STS           string // Strict-Transport-Security
	CSP           string // Content-Security-Policy
	XFrameOptions string
	XSSProtection string
	NoSniff       bool
}

// DefaultSecurityHeaders returns sensible defaults.
func DefaultSecurityHeaders() SecurityHeaders {
	return SecurityHeaders{
		STS:           "max-age=31536000; includeSubDomains",
		CSP:           "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self' wss:;",
		XFrameOptions: "DENY",
		XSSProtection: "1; mode=block",
		NoSniff:       true,
	}
}

// SecurityMiddleware adds security headers to responses.
func SecurityMiddleware(headers SecurityHeaders) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set security headers
			if headers.STS != "" {
				w.Header().Set("Strict-Transport-Security", headers.STS)
			}
			if headers.CSP != "" {
				w.Header().Set("Content-Security-Policy", headers.CSP)
			}
			if headers.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", headers.XFrameOptions)
			}
			if headers.XSSProtection != "" {
				w.Header().Set("X-XSS-Protection", headers.XSSProtection)
			}
			if headers.NoSniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}
			// Also set Referrer-Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Set Permissions-Policy
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			
			next.ServeHTTP(w, r)
		})
	}
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck checks the health of the CORS middleware.
func (c *CORSMiddleware) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"component": "cors_middleware",
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"config": map[string]interface{}{
			"allow_all_origins":  c.config.AllowAllOrigins,
			"allowed_origins":    c.config.AllowedOrigins,
			"allowed_methods":    c.config.AllowedMethods,
			"allow_credentials":  c.config.AllowCredentials,
			"max_age":           c.config.MaxAge,
		},
		"cache_size": len(c.cache),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ======================================================================
= Convenience Function
// ======================================================================

// CORS returns a ready-to-use CORS middleware with default config.
func CORS() mux.MiddlewareFunc {
	mw, err := NewCORSMiddleware(DefaultCORSConfig())
	if err != nil {
		panic(err)
	}
	return mw.Middleware
}

// CORSWithConfig returns a middleware with custom config.
func CORSWithConfig(cfg CORSConfig) mux.MiddlewareFunc {
	mw, err := NewCORSMiddleware(cfg)
	if err != nil {
		panic(err)
	}
	return mw.Middleware
}

// ======================================================================
= Test Helpers
// ======================================================================

// MockCORSConfig returns a config suitable for testing.
func MockCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:     []string{"*"},
		AllowedMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:     []string{"Content-Type", "Authorization"},
		ExposedHeaders:     []string{"Content-Length"},
		AllowCredentials:   false,
		MaxAge:             3600,
		OptionsPassthrough: false,
		Debug:              true,
		AllowAllOrigins:    true,
		SkipValidation:     true,
	}
}

// TestCORS returns a CORS middleware with mock config.
func TestCORS() mux.MiddlewareFunc {
	mw, _ := NewCORSMiddleware(MockCORSConfig())
	return mw.Middleware
}

// ======================================================================
= Middleware Chain Helper
// ======================================================================

// ChainCORS applies both CORS and Security headers together.
func ChainCORS(cfg CORSConfig, secHeaders SecurityHeaders) mux.MiddlewareFunc {
	corsMw, err := NewCORSMiddleware(cfg)
	if err != nil {
		panic(err)
	}
	secMw := SecurityMiddleware(secHeaders)
	
	return func(next http.Handler) http.Handler {
		// Apply security first, then CORS
		handler := secMw(next)
		return corsMw.Middleware(handler)
	}
}