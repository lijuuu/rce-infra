package services

import (
	"fmt"

	"agent-svc/app/clients"
	"agent-svc/storage/postgres"
)

// StorageFactory creates storage adapters
type StorageFactory struct{}

// NewStorageFactory creates a new storage factory
func NewStorageFactory() *StorageFactory {
	return &StorageFactory{}
}

// CreatePostgresStore creates a Postgres store
func (f *StorageFactory) CreatePostgresStore(connString string) (clients.StorageAdapter, error) {
	store, err := postgres.NewStore(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres store: %w", err)
	}
	return store, nil
}
