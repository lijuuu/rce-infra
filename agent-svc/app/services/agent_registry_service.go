package services

import (
	"context"

	"agent-svc/app/clients"
	"agent-svc/app/domains"
)

// AgentRegistryService handles agent registry operations
type AgentRegistryService struct {
	storage clients.StorageAdapter
}

// NewAgentRegistryService creates a new agent registry service
func NewAgentRegistryService(storage clients.StorageAdapter) *AgentRegistryService {
	return &AgentRegistryService{
		storage: storage,
	}
}

// RegisterNode registers a new node
func (s *AgentRegistryService) RegisterNode(ctx context.Context, nodeID string, publicKey *string, attrs map[string]interface{}) error {
	return s.storage.RegisterNode(ctx, nodeID, publicKey, attrs)
}

// UpdateNodeLastSeen updates the last seen timestamp
func (s *AgentRegistryService) UpdateNodeLastSeen(ctx context.Context, nodeID string) error {
	return s.storage.UpdateNodeLastSeen(ctx, nodeID)
}

// GetNode retrieves a node by ID
func (s *AgentRegistryService) GetNode(ctx context.Context, nodeID string) (*domains.Node, error) {
	return s.storage.GetNode(ctx, nodeID)
}

// UpdateNodeMetadata updates node metadata
func (s *AgentRegistryService) UpdateNodeMetadata(ctx context.Context, nodeID string, metadata *domains.AgentMetadata) error {
	return s.storage.UpdateAgentMetadata(ctx, nodeID, metadata)
}
