// backend/cmd/api/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/config"
	"twitter-clone/backend/internal/handler"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/repository/postgres"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/internal/utils"
	"twitter-clone/backend/pkg/logger"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize logger
	logger.Init(cfg.LogLevel)
	log := logger.Get()

	// 3. Connect to PostgreSQL
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	// 4. Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// 5. Initialize adapters (external services)
	emailAdapter := adapter.NewEmailAdapter(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
	storageAdapter, err := adapter.NewStorageAdapter(cfg.StorageProvider, cfg.StorageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// 6. Initialize repositories
	userRepo := postgres.NewUserRepository(db)
	tweetRepo := postgres.NewTweetRepository(db)
	followRepo := postgres.NewFollowRepository(db)
	likeRepo := postgres.NewLikeRepository(db)
	retweetRepo := postgres.NewRetweetRepository(db)
	notificationRepo := postgres.NewNotificationRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	bookmarkRepo := postgres.NewBookmarkRepository(db)
	communityRepo := postgres.NewCommunityRepository(db)
	messageRepo := postgres.NewMessageRepository(db)
	pollRepo := postgres.NewPollRepository(db)
	reportRepo := postgres.NewReportRepository(db)

	// 7. Initialize services
	authService := service.NewAuthService(userRepo, sessionRepo, cfg.JWTSecret, cfg.JWTExpiry, cfg.RefreshTokenExpiry, emailAdapter)
	userService := service.NewUserService(userRepo, storageAdapter)
	tweetService := service.NewTweetService(tweetRepo, userRepo, notificationRepo, redisClient, cfg.TweetCharLimit)
	followService := service.NewFollowService(followRepo, userRepo, notificationRepo)
	likeService := service.NewLikeService(likeRepo, tweetRepo, notificationRepo)
	retweetService := service.NewRetweetService(retweetRepo, tweetRepo, notificationRepo)
	notificationService := service.NewNotificationService(notificationRepo, redisClient)
	bookmarkService := service.NewBookmarkService(bookmarkRepo, tweetRepo)
	communityService := service.NewCommunityService(communityRepo, userRepo)
	dmService := service.NewDMService(messageRepo, userRepo, redisClient)
	pollService := service.NewPollService(pollRepo, tweetRepo)
	reportService := service.NewReportService(reportRepo, tweetRepo, userRepo)
	searchService := service.NewSearchService(tweetRepo, userRepo)
	feedService := service.NewFeedService(tweetRepo, followRepo, redisClient)
	spaceService := service.NewSpaceService(redisClient) // placeholder for audio spaces

	// 8. Initialize WebSocket hub
	hub := utils.NewHub()
	go hub.Run()

	// 9. Set up HTTP router
	r := mux.NewRouter()

	// 9a. Middlewares (order matters)
	r.Use(middleware.Logger(log))
	r.Use(middleware.Recovery(log))
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.RateLimiter(redisClient, cfg.RateLimitPerMinute))

	// 9b. Public routes
	authHandler := handler.NewAuthHandler(authService)
	r.HandleFunc("/api/auth/register", authHandler.Register).Methods("POST")
	r.HandleFunc("/api/auth/login", authHandler.Login).Methods("POST")
	r.HandleFunc("/api/auth/refresh", authHandler.RefreshToken).Methods("POST")
	r.HandleFunc("/api/auth/logout", authHandler.Logout).Methods("POST")

	// 9c. Protected routes (JWT required)
	protected := r.PathPrefix("/api").Subrouter()
	protected.Use(middleware.Auth(cfg.JWTSecret, userRepo))

	userHandler := handler.NewUserHandler(userService)
	protected.HandleFunc("/users/{id}", userHandler.GetProfile).Methods("GET")
	protected.HandleFunc("/users/{id}", userHandler.UpdateProfile).Methods("PUT")
	protected.HandleFunc("/users/{id}/follow", followService.FollowHandler).Methods("POST") // simplified; use handler wrapper
	protected.HandleFunc("/users/{id}/unfollow", followService.UnfollowHandler).Methods("POST")
	protected.HandleFunc("/users/{id}/followers", followService.FollowersHandler).Methods("GET")
	protected.HandleFunc("/users/{id}/following", followService.FollowingHandler).Methods("GET")

	tweetHandler := handler.NewTweetHandler(tweetService)
	protected.HandleFunc("/tweets", tweetHandler.CreateTweet).Methods("POST")
	protected.HandleFunc("/tweets/{id}", tweetHandler.GetTweet).Methods("GET")
	protected.HandleFunc("/tweets/{id}", tweetHandler.DeleteTweet).Methods("DELETE")
	protected.HandleFunc("/tweets/feed", feedService.GetFeedHandler).Methods("GET")
	protected.HandleFunc("/tweets/{id}/like", likeHandler.ToggleLike).Methods("POST")
	protected.HandleFunc("/tweets/{id}/retweet", retweetHandler.ToggleRetweet).Methods("POST")
	protected.HandleFunc("/tweets/{id}/replies", tweetHandler.GetReplies).Methods("GET")

	// 9d. WebSocket endpoint
	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Upgrade to WebSocket with auth check
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true }, // refine in production
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		// Register connection with hub (pass userID from JWT query param)
		userID := r.URL.Query().Get("userId")
		hub.Register(conn, userID)
	})

	// 10. Start HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 11. Graceful shutdown
	go func() {
		log.Infof("Server starting on port %d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server shutdown error: %v", err)
	}
	log.Info("Server exited gracefully")
}