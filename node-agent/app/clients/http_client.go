package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient is the HTTP client for agent-svc via Kong
type HTTPClient struct {
	baseURL    string
	jwtToken   string
	httpClient *http.Client
}

// NewHTTPClient creates a new HTTP client
func NewHTTPClient(baseURL string, jwtToken string) *HTTPClient {
	return &HTTPClient{
		baseURL:  baseURL,
		jwtToken: jwtToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegisterAgent registers the node via HTTP
func (c *HTTPClient) RegisterAgent(ctx context.Context, nodeID string, attrs map[string]interface{}) (string, error) {
	payload := map[string]interface{}{
		"node_id": nodeID,
		"attrs":   attrs,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/agents/register", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("registration failed: %d %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Token, nil
}

// Heartbeat sends a heartbeat via HTTP
func (c *HTTPClient) Heartbeat(ctx context.Context, nodeID string) error {
	payload := map[string]interface{}{
		"node_id": nodeID,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/agents/heartbeat", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.jwtToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed: %d %s", resp.StatusCode, string(body))
	}

	return nil
}

// PollCommands polls for commands via HTTP
func (c *HTTPClient) PollCommands(ctx context.Context, nodeID string, maxWaitSeconds int) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/commands/next?node_id=%s&wait=%d", c.baseURL, nodeID, maxWaitSeconds)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.jwtToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var cmdResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&cmdResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if cmdResp["command_id"] == nil || cmdResp["command_id"] == "" {
		return nil, nil // No command available
	}

	return cmdResp, nil
}

// PushLogs pushes log chunks via HTTP
func (c *HTTPClient) PushLogs(ctx context.Context, commandID string, chunks []map[string]interface{}) ([]int64, error) {
	if len(chunks) == 0 {
		return []int64{}, nil
	}

	payload := map[string]interface{}{
		"command_id": commandID,
		"chunks":     chunks,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/commands/logs", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.jwtToken)

	resp, err := c.httpClient.Do(req)
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

// UpdateCommandStatus updates command status via HTTP
func (c *HTTPClient) UpdateCommandStatus(ctx context.Context, commandID, status string, exitCode int32, errorMsg string) error {
	payload := map[string]interface{}{
		"command_id": commandID,
		"status":     status,
	}
	if exitCode != 0 {
		payload["exit_code"] = exitCode
	}
	if errorMsg != "" {
		payload["error_msg"] = errorMsg
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/commands/status", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.jwtToken)

	resp, err := c.httpClient.Do(req)
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
