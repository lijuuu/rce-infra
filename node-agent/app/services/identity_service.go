package services

import (
	"context"
	"fmt"

	"node-agent/app/identity"
)

// IdentityService handles node identity and JWT refresh
type IdentityService struct {
	identityMgr *identity.Manager
	agentSvcURL string
	jwtToken    string
	nodeID      string
}

// NewIdentityService creates a new identity service
func NewIdentityService(identityMgr *identity.Manager, agentSvcURL string) *IdentityService {
	return &IdentityService{
		identityMgr: identityMgr,
		agentSvcURL: agentSvcURL,
	}
}

// Initialize loads or creates identity
func (s *IdentityService) Initialize() error {
	ident, err := s.identityMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}

	if ident == nil {
		return fmt.Errorf("identity not found, registration required")
	}

	s.nodeID = ident.NodeID
	s.jwtToken = ident.JWTToken

	return nil
}

// GetNodeID returns the node ID
func (s *IdentityService) GetNodeID() string {
	return s.nodeID
}

// GetJWTToken returns the JWT token
func (s *IdentityService) GetJWTToken() string {
	return s.jwtToken
}

// RefreshToken refreshes the JWT token (if needed)
func (s *IdentityService) RefreshToken(ctx context.Context) error {
	// In a production implementation, this would:
	// 1. Check token expiration
	// 2. Call agent-svc to refresh token
	// 3. Update identity file
	// For now, tokens are long-lived and refreshed on registration
	return nil
}

// UpdateToken updates the stored JWT token
func (s *IdentityService) UpdateToken(token string) error {
	if err := s.identityMgr.UpdateToken(token); err != nil {
		return fmt.Errorf("failed to update token: %w", err)
	}
	s.jwtToken = token
	return nil
}
