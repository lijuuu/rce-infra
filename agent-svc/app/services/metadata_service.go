package services

import (
	"context"

	"agent-svc/app/clients"
	"agent-svc/app/domains"
)

// MetadataService handles metadata operations
type MetadataService struct {
	storage clients.StorageAdapter
}

// NewMetadataService creates a new metadata service
func NewMetadataService(storage clients.StorageAdapter) *MetadataService {
	return &MetadataService{storage: storage}
}

// UpdateMetadata updates agent metadata for a node
func (s *MetadataService) UpdateMetadata(ctx context.Context, nodeID string, metadata *domains.AgentMetadata) error {
	return s.storage.UpdateAgentMetadata(ctx, nodeID, metadata)
}
