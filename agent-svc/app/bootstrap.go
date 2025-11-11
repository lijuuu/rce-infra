package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-svc/app/clients"
	"agent-svc/app/handlers"
	"agent-svc/app/services"
	"agent-svc/storage/postgres"

	"github.com/gin-gonic/gin"
)

// App represents the application
type App struct {
	Config          *Config
	Storage         clients.StorageAdapter
	JWTService      *services.JWTService
	CommandService  *services.CommandService
	LogService      *services.LogService
	MetadataService *services.MetadataService
	Router          *gin.Engine
}

// Bootstrap initializes the application
func Bootstrap() (*App, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Build connection string
	connString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	// Initialize storage
	store, err := postgres.NewStore(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Run migrations
	migrations, err := loadMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	if err := store.RunMigrations(migrations); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize services
	jwtService := services.NewJWTService(cfg.JWTSecret, cfg.JWTExpirationSec)
	commandService := services.NewCommandService(store)
	logService := services.NewLogService(store)
	metadataService := services.NewMetadataService(store)

	// Initialize HTTP handlers
	agentHandler := handlers.NewAgentHandler(jwtService, store)
	commandHandler := handlers.NewCommandHandler(commandService, logService, jwtService, store)

	// Setup HTTP router
	router := gin.Default()
	setupRoutes(router, agentHandler, commandHandler)

	// Start cleanup job
	go startCleanupJob(store, cfg.LogRetentionDays)

	app := &App{
		Config:          cfg,
		Storage:         store,
		JWTService:      jwtService,
		CommandService:  commandService,
		LogService:      logService,
		MetadataService: metadataService,
		Router:          router,
	}

	return app, nil
}

// loadMigrations loads SQL migration files
func loadMigrations() ([]string, error) {
	migrationDir := "storage/postgres/migrations"
	migrations := []string{}

	files, err := os.ReadDir(migrationDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".sql" {
			path := filepath.Join(migrationDir, file.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read migration %s: %w", path, err)
			}
			migrations = append(migrations, string(content))
		}
	}

	return migrations, nil
}

// setupRoutes configures HTTP routes
func setupRoutes(router *gin.Engine, agentHandler *handlers.AgentHandler, commandHandler *handlers.CommandHandler) {
	// Health endpoints
	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// API v1 routes (for node-agent via Kong and admin APIs)
	v1 := router.Group("/v1")
	{
		// Agent endpoints (node-agent uses these)
		v1.POST("/agents/register", agentHandler.Register)
		v1.POST("/agents/heartbeat", agentHandler.Heartbeat)

		// Command endpoints
		v1.POST("/commands/submit", commandHandler.SubmitCommand)           // Admin API
		v1.GET("/commands/next", commandHandler.GetNextCommand)             // Node-agent
		v1.POST("/commands/logs", commandHandler.PushLogs)                  // Node-agent
		v1.POST("/commands/status", commandHandler.UpdateCommandStatus)     // Node-agent
		v1.GET("/commands/:command_id/logs", commandHandler.GetCommandLogs) // Admin API - fetch logs
	}
}

// startCleanupJob runs periodic cleanup of old logs
func startCleanupJob(storage clients.StorageAdapter, retentionDays int) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := storage.CleanupOldLogs(ctx, retentionDays); err != nil {
			fmt.Printf("cleanup job failed: %v\n", err)
		}
		cancel()
	}
}
