// backend/internal/pkg/logger/logger.go
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ======================================================================
// Constants
// ======================================================================

const (
	// Log levels
	LevelPanic = "panic"
	LevelFatal = "fatal"
	LevelError = "error"
	LevelWarn  = "warn"
	LevelInfo  = "info"
	LevelDebug = "debug"
	LevelTrace = "trace"

	// Default values
	DefaultLogLevel   = LevelInfo
	DefaultLogFormat  = "text"
	DefaultMaxSize    = 100 // MB
	DefaultMaxBackups = 5
	DefaultMaxAge     = 30 // days
)

// ======================================================================
// Configuration
// ======================================================================

// Config holds logger configuration.
type Config struct {
	Level      string `json:"level"`
	Format     string `json:"format"` // "text", "json", "pretty"
	Output     string `json:"output"` // "stdout", "stderr", "file"
	FilePath   string `json:"file_path"`
	MaxSize    int    `json:"max_size"`    // MB
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`     // days
	Compress   bool   `json:"compress"`
	Caller     bool   `json:"caller"`
	Timestamp  bool   `json:"timestamp"`
	AddSource  bool   `json:"add_source"`
	Env        string `json:"env"`
	Service    string `json:"service"`
	Version    string `json:"version"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Level:      DefaultLogLevel,
		Format:     DefaultLogFormat,
		Output:     "stdout",
		MaxSize:    DefaultMaxSize,
		MaxBackups: DefaultMaxBackups,
		MaxAge:     DefaultMaxAge,
		Compress:   true,
		Caller:     true,
		Timestamp:  true,
		AddSource:  false,
	}
}

// ======================================================================
= Logger
// ======================================================================

// Logger wraps logrus.Logger with additional features.
type Logger struct {
	*logrus.Logger
	config   Config
	fields   logrus.Fields
	mu       sync.RWMutex
	entries  chan *logrus.Entry
	stopCh   chan struct{}
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	initialized bool
}

// ======================================================================
= Fields and Entry
// ======================================================================

// Fields represents log fields.
type Fields map[string]interface{}

// Entry represents a log entry.
type Entry struct {
	*logrus.Entry
	logger *Logger
}

// ======================================================================
= New Logger
// ======================================================================

// New creates a new logger instance.
func New(cfg Config) (*Logger, error) {
	log := &Logger{
		Logger: logrus.New(),
		config: cfg,
		fields: make(logrus.Fields),
		entries: make(chan *logrus.Entry, 1000),
		stopCh:  make(chan struct{}),
	}
	// Set output
	if err := log.setOutput(cfg.Output, cfg.FilePath); err != nil {
		return nil, fmt.Errorf("failed to set output: %w", err)
	}
	// Set level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)
	// Set formatter
	log.setFormatter(cfg)
	// Set caller
	if cfg.Caller {
		log.SetReportCaller(true)
	}
	// Add default fields
	if cfg.Service != "" {
		log.WithField("service", cfg.Service)
	}
	if cfg.Env != "" {
		log.WithField("env", cfg.Env)
	}
	if cfg.Version != "" {
		log.WithField("version", cfg.Version)
	}
	// Start async log processor
	if cfg.Output == "file" {
		ctx, cancel := context.WithCancel(context.Background())
		log.ctx = ctx
		log.cancel = cancel
		log.wg.Add(1)
		go log.processEntries()
	}
	log.initialized = true
	return log, nil
}

// MustNew creates a new logger or panics.
func MustNew(cfg Config) *Logger {
	log, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return log
}

// ======================================================================
= Output Configuration
// ======================================================================

// setOutput configures the logger output.
func (l *Logger) setOutput(output, filePath string) error {
	switch output {
	case "stdout":
		l.SetOutput(os.Stdout)
	case "stderr":
		l.SetOutput(os.Stderr)
	case "file":
		if filePath == "" {
			return fmt.Errorf("file path is required for file output")
		}
		// Create directory if not exists
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
		rotator := &lumberjack.Logger{
			Filename:   filePath,
			MaxSize:    l.config.MaxSize,
			MaxBackups: l.config.MaxBackups,
			MaxAge:     l.config.MaxAge,
			Compress:   l.config.Compress,
		}
		l.SetOutput(rotator)
	default:
		return fmt.Errorf("unsupported output: %s", output)
	}
	return nil
}

// ======================================================================
= Formatter Configuration
// ======================================================================

// setFormatter configures the log formatter.
func (l *Logger) setFormatter(cfg Config) {
	switch cfg.Format {
	case "json":
		l.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "caller",
			},
		})
	case "pretty":
		l.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
			ForceColors:     true,
			DisableColors:   false,
		})
	default: // text
		l.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
			ForceColors:     false,
			DisableColors:   true,
		})
	}
}

// ======================================================================
= Async Processing
// ======================================================================

// processEntries processes log entries asynchronously.
func (l *Logger) processEntries() {
	defer l.wg.Done()
	for {
		select {
		case entry := <-l.entries:
			// Write entry to underlying logger
			entry.Log()
		case <-l.stopCh:
			return
		case <-l.ctx.Done():
			return
		}
	}
}

// ======================================================================
= Log Methods
// ======================================================================

// Debug logs a debug message.
func (l *Logger) Debug(args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Debug(args...)
	}
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Debugf(format, args...)
	}
}

// Info logs an info message.
func (l *Logger) Info(args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Info(args...)
	}
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Infof(format, args...)
	}
}

// Warn logs a warning message.
func (l *Logger) Warn(args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Warn(args...)
	}
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Warnf(format, args...)
	}
}

// Error logs an error message.
func (l *Logger) Error(args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Error(args...)
	}
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Errorf(format, args...)
	}
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Fatal(args...)
	}
}

// Fatalf logs a formatted fatal message and exits.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Fatalf(format, args...)
	}
}

// Panic logs a panic message and panics.
func (l *Logger) Panic(args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Panic(args...)
	}
}

// Panicf logs a formatted panic message and panics.
func (l *Logger) Panicf(format string, args ...interface{}) {
	if l.initialized {
		l.WithFields(l.fields).Panicf(format, args...)
	}
}

// ======================================================================
= With Fields
// ======================================================================

// WithField adds a single field to the logger.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	clone := l.clone()
	clone.fields[key] = value
	return clone
}

// WithFields adds multiple fields to the logger.
func (l *Logger) WithFields(fields Fields) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	clone := l.clone()
	for k, v := range fields {
		clone.fields[k] = v
	}
	return clone
}

// WithError adds an error field.
func (l *Logger) WithError(err error) *Logger {
	return l.WithField("error", err.Error())
}

// WithContext adds context values as fields.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	fields := Fields{}
	// Extract common context values
	if requestID, ok := ctx.Value("request_id").(string); ok {
		fields["request_id"] = requestID
	}
	if userID, ok := ctx.Value("user_id").(string); ok {
		fields["user_id"] = userID
	}
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		fields["trace_id"] = traceID
	}
	if spanID, ok := ctx.Value("span_id").(string); ok {
		fields["span_id"] = spanID
	}
	if len(fields) > 0 {
		return l.WithFields(fields)
	}
	return l
}

// ======================================================================
= Entry Methods
// ======================================================================

// WithFieldEntry returns an entry with a single field.
func (l *Logger) WithFieldEntry(key string, value interface{}) *Entry {
	return &Entry{
		Entry:  l.WithFields(l.fields).WithField(key, value),
		logger: l,
	}
}

// WithFieldsEntry returns an entry with multiple fields.
func (l *Logger) WithFieldsEntry(fields Fields) *Entry {
	entry := l.WithFields(l.fields)
	for k, v := range fields {
		entry = entry.WithField(k, v)
	}
	return &Entry{
		Entry:  entry,
		logger: l,
	}
}

// Entry log methods.
func (e *Entry) Debug(args ...interface{}) { e.Entry.Debug(args...) }
func (e *Entry) Debugf(format string, args ...interface{}) { e.Entry.Debugf(format, args...) }
func (e *Entry) Info(args ...interface{}) { e.Entry.Info(args...) }
func (e *Entry) Infof(format string, args ...interface{}) { e.Entry.Infof(format, args...) }
func (e *Entry) Warn(args ...interface{}) { e.Entry.Warn(args...) }
func (e *Entry) Warnf(format string, args ...interface{}) { e.Entry.Warnf(format, args...) }
func (e *Entry) Error(args ...interface{}) { e.Entry.Error(args...) }
func (e *Entry) Errorf(format string, args ...interface{}) { e.Entry.Errorf(format, args...) }
func (e *Entry) Fatal(args ...interface{}) { e.Entry.Fatal(args...) }
func (e *Entry) Fatalf(format string, args ...interface{}) { e.Entry.Fatalf(format, args...) }
func (e *Entry) Panic(args ...interface{}) { e.Entry.Panic(args...) }
func (e *Entry) Panicf(format string, args ...interface{}) { e.Entry.Panicf(format, args...) }

// ======================================================================
= Helper Methods
// ======================================================================

// clone creates a shallow clone of the logger.
func (l *Logger) clone() *Logger {
	clone := &Logger{
		Logger:      l.Logger,
		config:      l.config,
		fields:      make(logrus.Fields),
		initialized: l.initialized,
	}
	for k, v := range l.fields {
		clone.fields[k] = v
	}
	return clone
}

// SetLevel sets the log level.
func (l *Logger) SetLevel(level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	l.Logger.SetLevel(lvl)
	return nil
}

// GetLevel returns the current log level.
func (l *Logger) GetLevel() string {
	return l.Logger.GetLevel().String()
}

// SetOutputWriter sets a custom output writer.
func (l *Logger) SetOutputWriter(writer io.Writer) {
	l.Logger.SetOutput(writer)
}

// ======================================================================
= Close and Cleanup
// ======================================================================

// Close closes the logger and flushes pending logs.
func (l *Logger) Close() error {
	if l.cancel != nil {
		l.cancel()
	}
	close(l.stopCh)
	l.wg.Wait()
	return nil
}

// Flush forces a flush of pending logs.
func (l *Logger) Flush() {
	// Logrus doesn't have a flush method; this is a no-op for compatibility.
}

// ======================================================================
= Global Logger
// ======================================================================

var (
	defaultLogger *Logger
	once          sync.Once
)

// Init initializes the global logger.
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		defaultLogger, err = New(cfg)
	})
	return err
}

// Get returns the global logger.
func Get() *Logger {
	if defaultLogger == nil {
		// Initialize with defaults
		_ = Init(DefaultConfig())
	}
	return defaultLogger
}

// ======================================================================
= Global Log Functions
// ======================================================================

// Debug logs a debug message using the global logger.
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

// Debugf logs a formatted debug message.
func Debugf(format string, args ...interface{}) {
	Get().Debugf(format, args...)
}

// Info logs an info message.
func Info(args ...interface{}) {
	Get().Info(args...)
}

// Infof logs a formatted info message.
func Infof(format string, args ...interface{}) {
	Get().Infof(format, args...)
}

// Warn logs a warning message.
func Warn(args ...interface{}) {
	Get().Warn(args...)
}

// Warnf logs a formatted warning message.
func Warnf(format string, args ...interface{}) {
	Get().Warnf(format, args...)
}

// Error logs an error message.
func Error(args ...interface{}) {
	Get().Error(args...)
}

// Errorf logs a formatted error message.
func Errorf(format string, args ...interface{}) {
	Get().Errorf(format, args...)
}

// Fatal logs a fatal message and exits.
func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits.
func Fatalf(format string, args ...interface{}) {
	Get().Fatalf(format, args...)
}

// Panic logs a panic message and panics.
func Panic(args ...interface{}) {
	Get().Panic(args...)
}

// Panicf logs a formatted panic message and panics.
func Panicf(format string, args ...interface{}) {
	Get().Panicf(format, args...)
}

// ======================================================================
= Global Field Functions
// ======================================================================

// WithField returns a logger with a single field.
func WithField(key string, value interface{}) *Logger {
	return Get().WithField(key, value)
}

// WithFields returns a logger with multiple fields.
func WithFields(fields Fields) *Logger {
	return Get().WithFields(fields)
}

// WithError returns a logger with an error field.
func WithError(err error) *Logger {
	return Get().WithError(err)
}

// WithContext returns a logger with context fields.
func WithContext(ctx context.Context) *Logger {
	return Get().WithContext(ctx)
}

// ======================================================================
= Environment Setup
// ======================================================================

// SetupFromEnv configures the logger from environment variables.
func SetupFromEnv() {
	cfg := DefaultConfig()
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.Format = format
	}
	if output := os.Getenv("LOG_OUTPUT"); output != "" {
		cfg.Output = output
	}
	if filePath := os.Getenv("LOG_FILE"); filePath != "" {
		cfg.FilePath = filePath
	}
	if service := os.Getenv("SERVICE_NAME"); service != "" {
		cfg.Service = service
	}
	if env := os.Getenv("ENV"); env != "" {
		cfg.Env = env
	}
	if version := os.Getenv("APP_VERSION"); version != "" {
		cfg.Version = version
	}
	_ = Init(cfg)
}

// ======================================================================
= Test Helpers
// ======================================================================

// NewTestLogger creates a logger for testing.
func NewTestLogger() *Logger {
	cfg := DefaultConfig()
	cfg.Level = LevelDebug
	cfg.Output = "stdout"
	cfg.Format = "text"
	log, _ := New(cfg)
	return log
}

// DiscardLogger creates a logger that discards all output.
func DiscardLogger() *Logger {
	cfg := DefaultConfig()
	cfg.Level = LevelPanic
	cfg.Output = "stdout"
	log, _ := New(cfg)
	log.SetOutput(io.Discard)
	return log
}