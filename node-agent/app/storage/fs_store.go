package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FSStore handles file system storage for JSON files
type FSStore struct {
	basePath string
}

// NewFSStore creates a new file system store
func NewFSStore(basePath string) (*FSStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &FSStore{basePath: basePath}, nil
}

// SaveJSON saves data as JSON to a file
func (s *FSStore) SaveJSON(filename string, data interface{}) error {
	path := filepath.Join(s.basePath, filename)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// LoadJSON loads JSON data from a file
func (s *FSStore) LoadJSON(filename string, data interface{}) error {
	path := filepath.Join(s.basePath, filename)

	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, return nil (caller can check)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(jsonData, data); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// Exists checks if a file exists
func (s *FSStore) Exists(filename string) bool {
	path := filepath.Join(s.basePath, filename)
	_, err := os.Stat(path)
	return err == nil
}

// Delete deletes a file
func (s *FSStore) Delete(filename string) error {
	path := filepath.Join(s.basePath, filename)
	return os.Remove(path)
}
