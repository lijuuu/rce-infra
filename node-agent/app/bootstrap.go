package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"node-agent/app/clients"
	"node-agent/app/identity"
	"node-agent/app/services"
	"node-agent/app/storage"
	"node-agent/app/utils"
)

// Bootstrap initializes and starts the node agent
func Bootstrap() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize storage
	store, err := storage.NewStore(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Initialize identity manager
	identityMgr := identity.NewManager(cfg.IdentityPath)

	// Load or register identity
	ident, err := identityMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}

	if ident == nil {
		// Need to register
		log.Println("identity not found, registering with agent-svc...")

		// Collect metadata
		collector := identity.NewCollector()
		metadata, err := collector.Collect()
		if err != nil {
			return fmt.Errorf("failed to collect metadata: %w", err)
		}

		// Generate unique node ID using UUID
		nodeID := utils.GenerateUUID()
		log.Printf("generated unique node ID: %s", nodeID)

		// Register with agent-svc via HTTP
		httpClient := clients.NewHTTPClient(cfg.AgentSvcURL, "")
		agentClient := services.NewAgentClient(httpClient)

		attrs := map[string]interface{}{
			"os_name":        metadata.OSName,
			"os_version":     metadata.OSVersion,
			"arch":           metadata.Arch,
			"kernel_version": metadata.KernelVersion,
			"hostname":       metadata.Hostname,
			"ip_address":     metadata.IPAddress,
			"cpu_cores":      metadata.CPUCores,
			"memory_mb":      metadata.MemoryMB,
			"disk_gb":        metadata.DiskGB,
		}

		token, err := agentClient.RegisterAgent(context.Background(), nodeID, attrs)
		if err != nil {
			return fmt.Errorf("failed to register: %w", err)
		}

		// Save identity
		ident = &identity.Identity{
			NodeID:   nodeID,
			JWTToken: token,
			Metadata: map[string]interface{}{
				"os_name":        metadata.OSName,
				"os_version":     metadata.OSVersion,
				"arch":           metadata.Arch,
				"kernel_version": metadata.KernelVersion,
				"hostname":       metadata.Hostname,
				"ip_address":     metadata.IPAddress,
				"cpu_cores":      metadata.CPUCores,
				"memory_mb":      metadata.MemoryMB,
				"disk_gb":        metadata.DiskGB,
			},
		}

		if err := identityMgr.Save(ident); err != nil {
			return fmt.Errorf("failed to save identity: %w", err)
		}

		log.Printf("registered as node: %s", nodeID)
	}

	// Initialize HTTP client and agent client
	httpClient := clients.NewHTTPClient(cfg.AgentSvcURL, ident.JWTToken)
	agentClient := services.NewAgentClient(httpClient)

	// Create chunk storage retry service - run every 2 seconds for real-time priority
	chunkStorageRetry := services.NewChunkStorageRetryService(store, agentClient, 2)

	runtimeService := services.NewRuntimeService(
		store,
		cfg.ChunkSize,
		cfg.ChunkIntervalSec,
		agentClient,
		chunkStorageRetry,
		ident.NodeID,
		5, // check every 5 seconds
		cfg.WorkerCount,
		cfg.ChannelSize,
	)

	heartbeatService := services.NewHeartbeatService(
		agentClient,
		ident.NodeID,
		cfg.HeartbeatIntervalSec,
	)

	// Start services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go heartbeatService.Start(ctx)
	go chunkStorageRetry.Start(ctx)
	go runtimeService.Start(ctx)

	// Start cleanup job
	go startCleanupJob(ctx, store)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("node agent started (node_id: %s)", ident.NodeID)
	<-sigChan
	log.Println("shutting down...")

	return nil
}

// startCleanupJob runs periodic cleanup
func startCleanupJob(ctx context.Context, store *storage.Store) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Cleanup acked chunks older than 15 minutes
			store.CleanupAckedChunks(ctx, 15)
			// Cleanup completed commands older than 24 hours
			store.CleanupCompletedCommands(ctx, 24)
		}
	}
}
