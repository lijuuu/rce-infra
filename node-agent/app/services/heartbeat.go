package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"node-agent/app/clients"
)

// HeartbeatService handles heartbeat via HTTP
type HeartbeatService struct {
	agentClient         *AgentClient
	nodeID              string
	interval            time.Duration
	registrationService *RegistrationService
	httpClient          *clients.HTTPClient
}

// NewHeartbeatService creates a new HTTP heartbeat service
func NewHeartbeatService(agentClient *AgentClient, nodeID string, intervalSec int, registrationService *RegistrationService, httpClient *clients.HTTPClient) *HeartbeatService {
	return &HeartbeatService{
		agentClient:         agentClient,
		nodeID:              nodeID,
		interval:            time.Duration(intervalSec) * time.Second,
		registrationService: registrationService,
		httpClient:          httpClient,
	}
}

// Start starts the heartbeat loop
func (h *HeartbeatService) Start(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Send initial heartbeat
	h.sendHeartbeat(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.sendHeartbeat(ctx)
		}
	}
}

// sendHeartbeat sends a heartbeat request via HTTP
func (h *HeartbeatService) sendHeartbeat(ctx context.Context) {
	if err := h.agentClient.Heartbeat(ctx, h.nodeID); err != nil {
		// Check if error indicates node not found (404 status code)
		if isNodeNotFoundError(err) {
			log.Printf("heartbeat failed: node not registered. Re-registering...")
			if err := h.reRegister(ctx); err != nil {
				log.Printf("re-registration failed: %v", err)
			}
		} else {
			// Log other errors but don't fail - heartbeat is best effort
			fmt.Printf("heartbeat failed: %v\n", err)
		}
	}
}

// isNodeNotFoundError checks if the error indicates the node is not registered (404 status code)
func isNodeNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's an HTTPError with 404 status code
	if httpErr, ok := err.(*clients.HTTPError); ok {
		return httpErr.GetStatusCode() == 404
	}

	// Fallback: check error string for backward compatibility
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "404") || strings.Contains(errStr, "not found")
}

// reRegister re-registers the node and updates tokens
func (h *HeartbeatService) reRegister(ctx context.Context) error {
	// Re-register using registration service
	ident, token, err := h.registrationService.ReRegister(ctx)
	if err != nil {
		return err
	}

	// Update HTTP client token
	h.httpClient.UpdateToken(token)

	// Update agent client's HTTP client token
	h.agentClient.GetHTTPClient().UpdateToken(token)

	// Update nodeID if it changed (shouldn't happen, but be safe)
	h.nodeID = ident.NodeID

	log.Printf("re-registered node: %s", ident.NodeID)
	return nil
}
