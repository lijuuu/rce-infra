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
)

// Bootstrap initializes and starts the node agent
func Bootstrap() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store, err := storage.NewStore(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	identityMgr := identity.NewManager(cfg.IdentityPath)
	registrationService := services.NewRegistrationService(cfg.AgentSvcURL, identityMgr)

	ident, err := identityMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}

	if ident == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		var lastErr error
		for i := 0; i < 10; i++ {
			ident, _, err = registrationService.Register(ctx)
			if err == nil {
				break
			}
			lastErr = err
			log.Printf("registration attempt %d failed: %v, retrying in 3 seconds...", i+1, err)
			time.Sleep(3 * time.Second)
		}

		if err != nil {
			return fmt.Errorf("failed to register after retries: %w", lastErr)
		}
	}

	httpClient := clients.NewHTTPClient(cfg.AgentSvcURL, ident.JWTToken)
	agentClient := services.NewAgentClient(httpClient)
	chunkStorageRetry := services.NewChunkStorageRetryService(store, agentClient, 2)

	runtimeService := services.NewRuntimeService(
		store,
		cfg.ChunkSize,
		cfg.ChunkIntervalSec,
		agentClient,
		chunkStorageRetry,
		ident.NodeID,
		5,
		cfg.WorkerCount,
		cfg.ChannelSize,
	)

	heartbeatService := services.NewHeartbeatService(
		agentClient,
		ident.NodeID,
		cfg.HeartbeatIntervalSec,
		registrationService,
		httpClient,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go heartbeatService.Start(ctx)
	go chunkStorageRetry.Start(ctx)
	go runtimeService.Start(ctx)
	go startCleanupJob(ctx, store)

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
			store.CleanupAckedChunks(ctx, 15)
			store.CleanupCompletedCommands(ctx, 24)
		}
	}
}
