package clients

import (
	"context"

	"agent-svc/app/domains"

	"github.com/google/uuid"
)

// StorageAdapter defines the interface for storage operations
type StorageAdapter interface {
	RegisterNode(ctx context.Context, nodeID string, publicKey *string, attrs map[string]interface{}) error
	UpdateNodeLastSeen(ctx context.Context, nodeID string) error
	GetNode(ctx context.Context, nodeID string) (*domains.Node, error)
	CreateCommand(ctx context.Context, nodeID, commandType string, payload map[string]interface{}) (uuid.UUID, error)
	GetNextCommand(ctx context.Context, nodeID string) (*domains.NodeCommand, error)
	UpdateCommandStatus(ctx context.Context, commandID uuid.UUID, status string, exitCode *int, errorMsg *string) error
	GetCommandByID(ctx context.Context, commandID uuid.UUID) (*domains.NodeCommand, error)
	InsertLogChunks(ctx context.Context, commandID uuid.UUID, chunks []domains.CommandLog) ([]int64, error)
	GetCommandLogs(ctx context.Context, commandID uuid.UUID, afterOffset *int64) ([]domains.CommandLog, error)
	UpdateAgentMetadata(ctx context.Context, nodeID string, metadata *domains.AgentMetadata) error
	CleanupOldLogs(ctx context.Context, retentionDays int) error
}
