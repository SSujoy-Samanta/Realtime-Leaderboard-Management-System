package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/config"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/database"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/handler"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/middleware"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/repository"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/service"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/websocket"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Set Gin mode
	gin.SetMode(cfg.Server.GinMode)

	// Connect to PostgreSQL
	db, err := database.ConnectPostgres(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer database.CloseDB()

	// if !config.IsProduction() {
	// 	log.Println("🧱 Running AutoMigrate (non-production)")
	// 	if err := database.AutoMigrate(db); err != nil {
	// 		log.Fatalf("Failed to run migrations: %v", err)
	// 	}
	// } else {
	// 	log.Println("🚫 Skipping AutoMigrate in production")
	// }

	// Connect to Redis
	redisClient, err := database.ConnectRedis(&cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer database.CloseRedis()

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	scoreUpdateRepo := repository.NewScoreUpdateRepository(db)
	leaderboardRepo := repository.NewLeaderboardRepository(redisClient)

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Initialize Redis Pub/Sub service (handles multi-server broadcasting)
	pubSubService := service.NewPubSubService(redisClient)
	// Subscribe to Redis channel and broadcast to local WebSocket clients
	pubSubService.Start(func(payload *models.ScoreUpdatePayload) {
		// When ANY server publishes, this server receives it
		// and broadcasts to ITS WebSocket clients
		hub.BroadcastScoreUpdate(payload)
		log.Printf("📨 Received broadcast: User %d, rank %+d", 
			payload.UserID, payload.RankDelta)
	})
	defer pubSubService.Stop()

	// Initialize DB sync service (Redis queue-based, async PostgreSQL writes)
	dbSyncService := service.NewDBSyncService(redisClient, db)
	dbSyncService.Start()
	defer dbSyncService.Stop()

	// Initialize services
	leaderboardSvc := service.NewLeaderboardService(userRepo, leaderboardRepo, scoreUpdateRepo, dbSyncService, pubSubService)
	searchSvc := service.NewSearchService(userRepo, leaderboardRepo, leaderboardSvc)
	simulatorSvc := service.NewSimulatorService(leaderboardSvc, userRepo)

	// Initialize handlers
	leaderboardHandler := handler.NewLeaderboardHandler(leaderboardSvc)
	searchHandler := handler.NewSearchHandler(searchSvc)
	wsHandler := handler.NewWebSocketHandler(hub)

	// Setup router
	router := setupRouter(leaderboardHandler, searchHandler, wsHandler)

	// Start score simulator
	simulatorSvc.Start()
	defer simulatorSvc.Stop()

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("🚀 Server starting on port %s", cfg.Server.Port)
		log.Printf("📊 Leaderboard API: http://localhost:%s/api/leaderboard", cfg.Server.Port)
		log.Printf("🔍 Search API: http://localhost:%s/api/search?q=user", cfg.Server.Port)
		log.Printf("🌐 WebSocket: ws://localhost:%s/ws", cfg.Server.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server stopped")
}

func setupRouter(
	leaderboardHandler *handler.LeaderboardHandler,
	searchHandler *handler.SearchHandler,
	wsHandler *handler.WebSocketHandler,
) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.CORSMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// API routes
	api := router.Group("/api")
	{
		// Leaderboard routes
		api.GET("/leaderboard", leaderboardHandler.GetLeaderboard)
		api.GET("/leaderboard/stats", leaderboardHandler.GetStats)
		api.GET("/leaderboard/user/:user_id/rank", leaderboardHandler.GetUserRank)
		api.PUT("/leaderboard/user/:user_id/score", leaderboardHandler.UpdateUserScore)

		// Search routes
		api.GET("/search", searchHandler.SearchUsers)

		// WebSocket stats
		api.GET("/ws/stats", wsHandler.GetConnectionStats)
	}

	// WebSocket endpoint
	router.GET("/ws", wsHandler.HandleWebSocket)

	return router
}
