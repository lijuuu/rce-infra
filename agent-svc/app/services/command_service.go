package services

import (
	"context"
	"fmt"

	"agent-svc/app/clients"
	"agent-svc/app/domains"
	"agent-svc/app/utils"

	"github.com/google/uuid"
)

// CommandService handles command operations
type CommandService struct {
	storage clients.StorageAdapter
}

// NewCommandService creates a new command service
func NewCommandService(storage clients.StorageAdapter) *CommandService {
	return &CommandService{storage: storage}
}

// SubmitCommand submits a command to a single node (one-to-one)
func (s *CommandService) SubmitCommand(ctx context.Context, commandType string, nodeID string, payload map[string]interface{}) (uuid.UUID, error) {
	// Validate payload against command type
	if err := utils.ValidateCommandPayload(commandType, payload); err != nil {
		return uuid.Nil, fmt.Errorf("payload validation failed: %w", err)
	}

	// Verify node exists
	node, err := s.storage.GetNode(ctx, nodeID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get node %s: %w", nodeID, err)
	}
	if node == nil {
		return uuid.Nil, fmt.Errorf("node %s not found", nodeID)
	}
	if node.Disabled {
		return uuid.Nil, fmt.Errorf("node %s is disabled", nodeID)
	}

	commandID, err := s.storage.CreateCommand(ctx, nodeID, commandType, payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create command for node %s: %w", nodeID, err)
	}

	return commandID, nil
}

// GetNextCommand retrieves up to 5 queued commands for a node
func (s *CommandService) GetNextCommand(ctx context.Context, nodeID string) ([]*domains.NodeCommand, error) {
	return s.storage.GetNextCommand(ctx, nodeID)
}

// UpdateCommandStatus updates the status of a command
func (s *CommandService) UpdateCommandStatus(ctx context.Context, commandID uuid.UUID, nodeID string, status string, exitCode *int, errorMsg *string) error {
	// Verify command belongs to node
	cmd, err := s.storage.GetCommandByID(ctx, commandID)
	if err != nil {
		return fmt.Errorf("failed to get command: %w", err)
	}
	if cmd == nil {
		return fmt.Errorf("command not found")
	}
	if cmd.NodeID != nodeID {
		return fmt.Errorf("command does not belong to node")
	}

	return s.storage.UpdateCommandStatus(ctx, commandID, status, exitCode, errorMsg)
}

// DeleteQueuedCommands deletes all queued commands, optionally filtered by nodeID
func (s *CommandService) DeleteQueuedCommands(ctx context.Context, nodeID *string) (int, error) {
	return s.storage.DeleteQueuedCommands(ctx, nodeID)
}
