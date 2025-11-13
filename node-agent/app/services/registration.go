package services

import (
	"context"
	"fmt"
	"log"

	"node-agent/app/clients"
	"node-agent/app/identity"
	"node-agent/app/utils"
)

// RegistrationService handles node registration and re-registration
type RegistrationService struct {
	agentSvcURL string
	identityMgr *identity.Manager
}

// NewRegistrationService creates a new registration service
func NewRegistrationService(agentSvcURL string, identityMgr *identity.Manager) *RegistrationService {
	return &RegistrationService{
		agentSvcURL: agentSvcURL,
		identityMgr: identityMgr,
	}
}

// Register registers a new node (generates new node_id)
func (r *RegistrationService) Register(ctx context.Context) (*identity.Identity, string, error) {
	// Collect metadata
	collector := identity.NewCollector()
	metadata, err := collector.Collect()
	if err != nil {
		return nil, "", fmt.Errorf("failed to collect metadata: %w", err)
	}

	// Generate new node ID
	nodeID := utils.GenerateUUID()
	log.Printf("registering new node with node_id: %s", nodeID)

	attrs := map[string]interface{}{
		"os_name":        metadata.OSName,
		"os_version":     metadata.OSVersion,
		"arch":           metadata.Arch,
		"kernel_version": metadata.KernelVersion,
		"hostname":       metadata.Hostname,
		"ip_address":     metadata.IPAddress,
		"cpu_cores":      metadata.CPUCores,
		"memory_mb":      metadata.MemoryMB,
		"disk_gb":        metadata.DiskGB,
	}

	// Register with agent-svc
	httpClient := clients.NewHTTPClient(r.agentSvcURL, "")
	agentClient := NewAgentClient(httpClient)

	token, err := agentClient.RegisterAgent(ctx, nodeID, attrs)
	if err != nil {
		return nil, "", fmt.Errorf("failed to register: %w", err)
	}

	// Create identity
	ident := &identity.Identity{
		NodeID:   nodeID,
		JWTToken: token,
		Metadata: attrs,
	}

	// Save identity
	if err := r.identityMgr.Save(ident); err != nil {
		return nil, "", fmt.Errorf("failed to save identity: %w", err)
	}

	log.Printf("registered node: %s", nodeID)
	return ident, token, nil
}

// ReRegister re-registers an existing node using identity from file
func (r *RegistrationService) ReRegister(ctx context.Context) (*identity.Identity, string, error) {
	// Load existing identity
	ident, err := r.identityMgr.Load()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load identity: %w", err)
	}
	if ident == nil {
		return nil, "", fmt.Errorf("identity not found, cannot re-register")
	}

	log.Printf("re-registering node with existing node_id: %s", ident.NodeID)

	// Use existing node_id and metadata
	attrs := ident.Metadata
	if attrs == nil {
		attrs = make(map[string]interface{})
	}

	// Register with agent-svc
	httpClient := clients.NewHTTPClient(r.agentSvcURL, "")
	agentClient := NewAgentClient(httpClient)

	token, err := agentClient.RegisterAgent(ctx, ident.NodeID, attrs)
	if err != nil {
		return nil, "", fmt.Errorf("failed to re-register: %w", err)
	}

	// Update token in identity
	ident.JWTToken = token
	if err := r.identityMgr.Save(ident); err != nil {
		return nil, "", fmt.Errorf("failed to save identity: %w", err)
	}

	log.Printf("re-registered node: %s", ident.NodeID)
	return ident, token, nil
}
