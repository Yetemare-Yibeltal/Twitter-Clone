// backend/pkg/logger/logger.go
package logger

import (
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	log  *logrus.Logger
	once sync.Once
)

// Init sets up the global logger with the given log level.
// It should be called once at application startup.
func Init(level string) {
	once.Do(func() {
		log = logrus.New()

		// Set output (stdout)
		log.SetOutput(os.Stdout)

		// Set formatter: JSON in production, text in development
		if os.Getenv("ENV") == "production" {
			log.SetFormatter(&logrus.JSONFormatter{
				TimestampFormat: "2006-01-02T15:04:05Z07:00",
			})
		} else {
			log.SetFormatter(&logrus.TextFormatter{
				FullTimestamp: true,
			})
		}

		// Parse log level
		lvl, err := logrus.ParseLevel(level)
		if err != nil {
			lvl = logrus.InfoLevel
		}
		log.SetLevel(lvl)
	})
}

// Get returns the global logger instance.
// Init must be called before using this.
func Get() *logrus.Logger {
	if log == nil {
		// Fallback: initialise with default info level
		Init("info")
	}
	return log
}

// SetOutput allows redirecting logs to a custom writer (e.g., for testing).
func SetOutput(w io.Writer) {
	Get().SetOutput(w)
}

// WithField adds a single field to the log entry.
func WithField(key string, value interface{}) *logrus.Entry {
	return Get().WithField(key, value)
}

// WithFields adds multiple fields to the log entry.
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Get().WithFields(fields)
}

// WithError adds an error field to the log entry.
func WithError(err error) *logrus.Entry {
	return Get().WithError(err)
}

// Debug logs a message at debug level.
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

// Debugf logs a formatted message at debug level.
func Debugf(format string, args ...interface{}) {
	Get().Debugf(format, args...)
}

// Info logs a message at info level.
func Info(args ...interface{}) {
	Get().Info(args...)
}

// Infof logs a formatted message at info level.
func Infof(format string, args ...interface{}) {
	Get().Infof(format, args...)
}

// Warn logs a message at warn level.
func Warn(args ...interface{}) {
	Get().Warn(args...)
}

// Warnf logs a formatted message at warn level.
func Warnf(format string, args ...interface{}) {
	Get().Warnf(format, args...)
}

// Error logs a message at error level.
func Error(args ...interface{}) {
	Get().Error(args...)
}

// Errorf logs a formatted message at error level.
func Errorf(format string, args ...interface{}) {
	Get().Errorf(format, args...)
}

// Fatal logs a message at fatal level and calls os.Exit(1).
func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

// Fatalf logs a formatted message at fatal level and calls os.Exit(1).
func Fatalf(format string, args ...interface{}) {
	Get().Fatalf(format, args...)
}

// Panic logs a message at panic level and panics.
func Panic(args ...interface{}) {
	Get().Panic(args...)
}

// Panicf logs a formatted message at panic level and panics.
func Panicf(format string, args ...interface{}) {
	Get().Panicf(format, args...)
}