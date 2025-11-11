package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"node-agent/app/storage"
)

// PullHandler handles command pulling from agent-svc
type PullHandler struct {
	agentSvcURL string
	jwtToken    string
	nodeID      string
	storage     *storage.Store
	httpClient  *http.Client
}

// NewPullHandler creates a new pull handler
func NewPullHandler(agentSvcURL, jwtToken, nodeID string, store *storage.Store) *PullHandler {
	return &PullHandler{
		agentSvcURL: agentSvcURL,
		jwtToken:    jwtToken,
		nodeID:      nodeID,
		storage:     store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PullNextCommand pulls the next command from agent-svc
func (h *PullHandler) PullNextCommand(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/commands/next?node_id=%s&wait=5", h.agentSvcURL, h.nodeID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+h.jwtToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var cmdResp struct {
		CommandID   string                 `json:"command_id"`
		CommandType string                 `json:"command_type"`
		Payload     map[string]interface{} `json:"payload"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cmdResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if cmdResp.CommandID == "" {
		return nil // No command available
	}

	// Save command locally
	payloadJSON, _ := json.Marshal(cmdResp.Payload)
	if err := h.storage.SaveCommand(ctx, cmdResp.CommandID, cmdResp.CommandType, string(payloadJSON)); err != nil {
		return fmt.Errorf("failed to save command: %w", err)
	}

	return nil
}
