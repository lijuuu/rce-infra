package app

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"agent-svc/app/clients"
	"agent-svc/app/handlers"
	"agent-svc/app/services"
	"agent-svc/storage/postgres"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	postgresdriver "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// App represents the application
type App struct {
	Config         *Config
	Storage        clients.StorageAdapter
	JWTService     *services.JWTService
	CommandService *services.CommandService
	LogService     *services.LogService
	Router         *gin.Engine
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

	// Run migrations using golang-migrate
	if err := runMigrations(connString); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize services
	jwtService := services.NewJWTService(cfg.JWTSecret, cfg.JWTExpirationSec)
	commandService := services.NewCommandService(store)
	logService := services.NewLogService(store)

	// Initialize HTTP handlers
	agentHandler := handlers.NewAgentHandler(jwtService, store)
	commandHandler := handlers.NewCommandHandler(commandService, logService, jwtService, store)

	// Setup HTTP router
	router := gin.Default()

	// Configure CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	setupRoutes(router, agentHandler, commandHandler)

	// Start cleanup job
	go startCleanupJob(store, cfg.LogRetentionDays)

	app := &App{
		Config:         cfg,
		Storage:        store,
		JWTService:     jwtService,
		CommandService: commandService,
		LogService:     logService,
		Router:         router,
	}

	return app, nil
}

// runMigrations runs database migrations using golang-migrate
func runMigrations(connString string) error {
	// Parse connection string and create database connection for migrate
	// golang-migrate expects database/sql driver, so we use pgx stdlib adapter
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	// Create migrate driver instance
	driver, err := postgresdriver.WithInstance(db, &postgresdriver.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migrate driver: %w", err)
	}

	// Use file:// source for migrations
	migrationDir := "storage/postgres/migrations"
	sourceURL := fmt.Sprintf("file://%s", migrationDir)

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Run migrations up
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			// No new migrations to run - this is fine
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
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
		v1.GET("/agents", agentHandler.ListNodes) // Admin API - list all nodes

		// Command endpoints
		v1.POST("/commands/submit", commandHandler.SubmitCommand)           // Admin API
		v1.GET("/commands", commandHandler.ListCommands)                    // Admin API - list commands
		v1.GET("/commands/next", commandHandler.GetNextCommand)             // Node-agent
		v1.POST("/commands/logs", commandHandler.PushCommandLogs)           // Node-agent
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
