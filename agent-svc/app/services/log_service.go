package services

import (
	"context"
	"fmt"

	"agent-svc/app/clients"
	"agent-svc/app/domains"

	"github.com/google/uuid"
)

// LogService handles log operations
type LogService struct {
	storage clients.StorageAdapter
}

// NewLogService creates a new log service
func NewLogService(storage clients.StorageAdapter) *LogService {
	return &LogService{storage: storage}
}

// PushCommandLogs pushes command execution log chunks for a command
func (s *LogService) PushCommandLogs(ctx context.Context, commandID uuid.UUID, nodeID string, chunks []domains.CommandLog) ([]int64, error) {
	// Verify command belongs to node
	cmd, err := s.storage.GetCommandByID(ctx, commandID)
	if err != nil {
		return nil, fmt.Errorf("failed to get command: %w", err)
	}
	if cmd == nil {
		return nil, fmt.Errorf("command not found")
	}
	if cmd.NodeID != nodeID {
		return nil, fmt.Errorf("command does not belong to node")
	}

	// Validate chunks
	for _, chunk := range chunks {
		if chunk.Stream != "stdout" && chunk.Stream != "stderr" {
			return nil, fmt.Errorf("invalid stream: %s", chunk.Stream)
		}
		if chunk.ChunkIndex < 0 {
			return nil, fmt.Errorf("invalid chunk_index: %d", chunk.ChunkIndex)
		}
		if chunk.Data == "" {
			return nil, fmt.Errorf("empty data in chunk")
		}
		if chunk.Encoding == "" {
			chunk.Encoding = "utf-8"
		}
	}

	// Insert chunks with idempotency
	ackedChunkIndexes, err := s.storage.InsertLogChunks(ctx, commandID, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to insert log chunks: %w", err)
	}

	return ackedChunkIndexes, nil
}

// GetCommandLogs retrieves logs for a command
// If afterChunkIndex is provided, only returns logs with chunk_index > afterChunkIndex
func (s *LogService) GetCommandLogs(ctx context.Context, commandID uuid.UUID, afterChunkIndex *int64) ([]domains.CommandLog, error) {
	return s.storage.GetCommandLogs(ctx, commandID, afterChunkIndex)
}
