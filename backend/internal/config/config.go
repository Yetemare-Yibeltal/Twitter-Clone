// backend/internal/config/config.go
package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	// Server
	Port int `json:"port"`

	// Database
	DatabaseURL       string        `json:"database_url"`
	DBMaxOpenConns    int           `json:"db_max_open_conns"`
	DBMaxIdleConns    int           `json:"db_max_idle_conns"`
	DBConnMaxLifetime time.Duration `json:"db_conn_max_lifetime"`

	// Redis
	RedisAddr     string `json:"redis_addr"`
	RedisPassword string `json:"redis_password"`
	RedisDB       int    `json:"redis_db"`

	// JWT
	JWTSecret          string        `json:"jwt_secret"`
	JWTExpiry          time.Duration `json:"jwt_expiry"`
	RefreshTokenExpiry time.Duration `json:"refresh_token_expiry"`

	// CORS
	AllowedOrigins []string `json:"allowed_origins"`

	// Rate limiting
	RateLimitPerMinute int `json:"rate_limit_per_minute"`

	// Logging
	LogLevel string `json:"log_level"`

	// Email (SMTP)
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	SMTPUser string `json:"smtp_user"`
	SMTPPass string `json:"smtp_pass"`
	SMTPFrom string `json:"smtp_from"`

	// Storage (Cloudinary / S3)
	StorageProvider string            `json:"storage_provider"`
	StorageConfig   map[string]string `json:"storage_config"`

	// Tweet constraints
	TweetCharLimit int `json:"tweet_char_limit"`
}

// Load reads environment variables and returns a Config instance.
func Load() (*Config, error) {
	_ = godotenv.Load() // ignore if .env missing

	cfg := &Config{
		Port:               getEnvAsInt("PORT", 8080),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		DBMaxOpenConns:     getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:     getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime:  getEnvAsDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            getEnvAsInt("REDIS_DB", 0),
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiry:          getEnvAsDuration("JWT_EXPIRY", 15*time.Minute),
		RefreshTokenExpiry: getEnvAsDuration("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),
		AllowedOrigins:     getEnvAsSlice("ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		RateLimitPerMinute: getEnvAsInt("RATE_LIMIT_PER_MINUTE", 100),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		SMTPHost:           getEnv("SMTP_HOST", ""),
		SMTPPort:           getEnvAsInt("SMTP_PORT", 587),
		SMTPUser:           getEnv("SMTP_USER", ""),
		SMTPPass:           getEnv("SMTP_PASS", ""),
		SMTPFrom:           getEnv("SMTP_FROM", "noreply@twitter-clone.local"),
		StorageProvider:    getEnv("STORAGE_PROVIDER", "local"),
		StorageConfig:      parseStorageConfig(),
		TweetCharLimit:     getEnvAsInt("TWEET_CHAR_LIMIT", 280),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "change-me-in-production" {
		log.Println("WARNING: JWT_SECRET is default – change it for production")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvAsSlice(key string, fallback []string) []string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	return fallback
}

func parseStorageConfig() map[string]string {
	cfg := make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "STORAGE_") {
			kv := strings.SplitN(env, "=", 2)
			if len(kv) == 2 {
				key := strings.TrimPrefix(kv[0], "STORAGE_")
				cfg[key] = kv[1]
			}
		}
	}
	return cfg
}