package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Identity represents node identity
type Identity struct {
	NodeID   string                 `json:"node_id"`
	JWTToken string                 `json:"jwt_token"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Manager handles identity file operations
type Manager struct {
	identityPath string
}

// NewManager creates a new identity manager
func NewManager(identityPath string) *Manager {
	return &Manager{identityPath: identityPath}
}

// Load loads identity from file
func (m *Manager) Load() (*Identity, error) {
	data, err := os.ReadFile(m.identityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read identity file: %w", err)
	}

	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return nil, fmt.Errorf("failed to unmarshal identity: %w", err)
	}

	return &identity, nil
}

// Save saves identity to file
func (m *Manager) Save(identity *Identity) error {
	// Ensure directory exists
	dir := filepath.Dir(m.identityPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal identity: %w", err)
	}

	if err := os.WriteFile(m.identityPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write identity file: %w", err)
	}

	return nil
}

// UpdateToken updates the JWT token in identity
func (m *Manager) UpdateToken(token string) error {
	identity, err := m.Load()
	if err != nil {
		return err
	}
	if identity == nil {
		return fmt.Errorf("identity not found")
	}

	identity.JWTToken = token
	return m.Save(identity)
}
