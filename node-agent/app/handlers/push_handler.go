package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"node-agent/app/executor"
)

// PushHandler handles pushing logs and status to agent-svc
type PushHandler struct {
	agentSvcURL string
	jwtToken    string
	httpClient  *http.Client
}

// NewPushHandler creates a new push handler
func NewPushHandler(agentSvcURL, jwtToken string) *PushHandler {
	return &PushHandler{
		agentSvcURL: agentSvcURL,
		jwtToken:    jwtToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PushLogs pushes log chunks to agent-svc
func (h *PushHandler) PushLogs(ctx context.Context, commandID string, chunks []executor.Chunk) ([]int64, error) {
	if len(chunks) == 0 {
		return []int64{}, nil
	}

	chunkReqs := make([]map[string]interface{}, len(chunks))
	for i, chunk := range chunks {
		chunkReqs[i] = map[string]interface{}{
			"offset": chunk.Offset,
			"stream": chunk.Stream,
			"data":   chunk.Data,
		}
	}

	payload := map[string]interface{}{
		"command_id": commandID,
		"chunks":     chunkReqs,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.agentSvcURL+"/v1/commands/logs", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.jwtToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AckedOffsets []int64 `json:"acked_offsets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.AckedOffsets, nil
}

// PushStatus pushes command status to agent-svc
func (h *PushHandler) PushStatus(ctx context.Context, commandID, status string, exitCode *int, errorMsg string) error {
	payload := map[string]interface{}{
		"command_id": commandID,
		"status":     status,
	}
	if exitCode != nil {
		payload["exit_code"] = *exitCode
	}
	if errorMsg != "" {
		payload["error_msg"] = errorMsg
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.agentSvcURL+"/v1/commands/status", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.jwtToken)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
