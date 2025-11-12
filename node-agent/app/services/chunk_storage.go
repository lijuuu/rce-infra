package services

import (
	"context"
	"fmt"
	"time"

	"node-agent/app/storage"
	"node-agent/app/utils"
)

// ChunkStorageRetryService handles uploading pending chunks when online
type ChunkStorageRetryService struct {
	storage     *storage.Store
	agentClient *AgentClient
	interval    time.Duration
}

// NewChunkStorageRetryService creates a new chunk storage retry service
func NewChunkStorageRetryService(store *storage.Store, agentClient *AgentClient, intervalSec int) *ChunkStorageRetryService {
	return &ChunkStorageRetryService{
		storage:     store,
		agentClient: agentClient,
		interval:    time.Duration(intervalSec) * time.Second,
	}
}

// Start starts the chunk storage retry uploader
func (c *ChunkStorageRetryService) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get all commands with pending chunks and upload them
			commandIDs, err := c.storage.GetCommandsWithPendingChunks(ctx)
			if err != nil {
				fmt.Printf("failed to get commands with pending chunks: %v\n", err)
				continue
			}
			for _, commandID := range commandIDs {
				// Regular retry uploads are not final (command may still be running)
				if err := c.UploadChunksForCommand(ctx, commandID, false); err != nil {
					fmt.Printf("failed to upload chunks for command %s: %v\n", commandID, err)
				}
			}
		}
	}
}

// UploadChunksForCommand uploads pending chunks for a specific command
// isFinal should be true when the command has completed and these are the final chunks
func (c *ChunkStorageRetryService) UploadChunksForCommand(ctx context.Context, commandID string, isFinal bool) error {
	chunks, err := c.storage.GetPendingChunks(ctx, commandID)
	if err != nil {
		return fmt.Errorf("failed to get pending chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil
	}

	// Convert to map format for AgentClient
	chunkMaps := make([]map[string]interface{}, len(chunks))
	for i, chunk := range chunks {
		chunkMaps[i] = map[string]interface{}{
			"chunk_index": chunk.ChunkIndex,
			"stream":      chunk.Stream,
			"data":        chunk.Data,
			"is_final":    isFinal,
		}
	}

	// Retry with backoff
	err = utils.RetryWithBackoff(5, 1*time.Second, 30*time.Second, func() error {
		ackedChunkIndexes, err := c.agentClient.PushCommandLogs(ctx, commandID, chunkMaps)
		if err != nil {
			return err
		}

		// Mark chunks as acked
		if err := c.storage.MarkChunksAcked(ctx, commandID, ackedChunkIndexes); err != nil {
			return fmt.Errorf("failed to mark chunks acked: %w", err)
		}

		// Increment retries for non-acked chunks
		allChunkIndexes := make([]int64, len(chunks))
		for i, chunk := range chunks {
			allChunkIndexes[i] = chunk.ChunkIndex
		}
		ackedMap := make(map[int64]bool)
		for _, chunkIndex := range ackedChunkIndexes {
			ackedMap[chunkIndex] = true
		}

		failedChunkIndexes := []int64{}
		for _, chunkIndex := range allChunkIndexes {
			if !ackedMap[chunkIndex] {
				failedChunkIndexes = append(failedChunkIndexes, chunkIndex)
			}
		}

		if len(failedChunkIndexes) > 0 {
			if err := c.storage.IncrementChunkRetries(ctx, commandID, failedChunkIndexes); err != nil {
				return fmt.Errorf("failed to increment retries: %w", err)
			}
		}

		return nil
	})

	return err
}
