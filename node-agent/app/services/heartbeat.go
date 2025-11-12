package services

import (
	"context"
	"fmt"
	"time"
)

// HeartbeatService handles heartbeat via HTTP
type HeartbeatService struct {
	agentClient *AgentClient
	nodeID      string
	interval    time.Duration
}

// NewHeartbeatService creates a new HTTP heartbeat service
func NewHeartbeatService(agentClient *AgentClient, nodeID string, intervalSec int) *HeartbeatService {
	return &HeartbeatService{
		agentClient: agentClient,
		nodeID:      nodeID,
		interval:    time.Duration(intervalSec) * time.Second,
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
		// Log error but don't fail - heartbeat is best effort
		fmt.Printf("heartbeat failed: %v\n", err)
	}
}
