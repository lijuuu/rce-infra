package services

import (
	"context"
	"fmt"
	"time"

	"node-agent/app/storage"
	"node-agent/app/utils"
)

// OfflineBufferService handles uploading pending chunks when online
type OfflineBufferService struct {
	storage      *storage.Store
	resultSender ResultSenderInterface
	interval     time.Duration
}

// ResultSenderInterface defines the interface for sending results
type ResultSenderInterface interface {
	PushLogs(ctx context.Context, commandID string, chunks []interface{}) ([]int64, error)
}

// NewOfflineBufferService creates a new offline buffer service
func NewOfflineBufferService(store *storage.Store, resultSender ResultSenderInterface, intervalSec int) *OfflineBufferService {
	return &OfflineBufferService{
		storage:      store,
		resultSender: resultSender,
		interval:     time.Duration(intervalSec) * time.Second,
	}
}

// Start starts the offline buffer uploader
func (o *OfflineBufferService) Start(ctx context.Context) {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.uploadPendingChunks(ctx)
		}
	}
}

// uploadPendingChunks uploads pending chunks for all commands
func (o *OfflineBufferService) uploadPendingChunks(ctx context.Context) {
	// Get all commands with pending chunks
	// For simplicity, we'll process commands one by one
	// In production, you might want to batch by command_id

	// This is a simplified version - in practice you'd query for distinct command_ids
	// and process each one
}

// UploadChunksForCommand uploads pending chunks for a specific command
func (o *OfflineBufferService) UploadChunksForCommand(ctx context.Context, commandID string) error {
	chunks, err := o.storage.GetPendingChunks(ctx, commandID)
	if err != nil {
		return fmt.Errorf("failed to get pending chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil
	}

	// Convert to executor.Chunk format
	executorChunks := make([]interface{}, len(chunks))
	for i, chunk := range chunks {
		executorChunks[i] = map[string]interface{}{
			"offset": chunk.Offset,
			"stream": chunk.Stream,
			"data":   chunk.Data,
		}
	}

	// Retry with backoff
	err = utils.RetryWithBackoff(5, 1*time.Second, 30*time.Second, func() error {
		ackedOffsets, err := o.resultSender.PushLogs(ctx, commandID, executorChunks)
		if err != nil {
			return err
		}

		// Mark chunks as acked
		if err := o.storage.MarkChunksAcked(ctx, commandID, ackedOffsets); err != nil {
			return fmt.Errorf("failed to mark chunks acked: %w", err)
		}

		// Increment retries for non-acked chunks
		allOffsets := make([]int64, len(chunks))
		for i, chunk := range chunks {
			allOffsets[i] = chunk.Offset
		}
		ackedMap := make(map[int64]bool)
		for _, offset := range ackedOffsets {
			ackedMap[offset] = true
		}

		failedOffsets := []int64{}
		for _, offset := range allOffsets {
			if !ackedMap[offset] {
				failedOffsets = append(failedOffsets, offset)
			}
		}

		if len(failedOffsets) > 0 {
			if err := o.storage.IncrementChunkRetries(ctx, commandID, failedOffsets); err != nil {
				return fmt.Errorf("failed to increment retries: %w", err)
			}
		}

		return nil
	})

	return err
}
