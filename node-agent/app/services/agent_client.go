package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"node-agent/app/clients"
)

// AgentClient provides high-level API methods for agent-svc
type AgentClient struct {
	httpClient *clients.HTTPClient
}

// GetHTTPClient returns the underlying HTTP client (for token updates)
func (c *AgentClient) GetHTTPClient() *clients.HTTPClient {
	return c.httpClient
}

// NewAgentClient creates a new agent client
func NewAgentClient(httpClient *clients.HTTPClient) *AgentClient {
	return &AgentClient{
		httpClient: httpClient,
	}
}

// RegisterAgent registers the node via HTTP
func (c *AgentClient) RegisterAgent(ctx context.Context, nodeID string, attrs map[string]interface{}) (string, error) {
	result, err := c.httpClient.DoRequest(ctx, "POST", "/v1/agents/register", map[string]interface{}{
		"node_id": nodeID,
		"attrs":   attrs,
	}, func(resp *http.Response) (interface{}, error) {
		var result struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return result.Token, nil
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// Heartbeat sends a heartbeat via HTTP
func (c *AgentClient) Heartbeat(ctx context.Context, nodeID string) error {
	_, err := c.httpClient.DoRequest(ctx, "POST", "/v1/agents/heartbeat", map[string]interface{}{
		"node_id": nodeID,
	}, func(resp *http.Response) (interface{}, error) {
		return nil, nil
	})
	return err
}

// PollCommands polls for commands
func (c *AgentClient) PollCommands(ctx context.Context, nodeID string, maxWaitSeconds int) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("/v1/commands/next?node_id=%s&wait=%d", nodeID, maxWaitSeconds)
	result, err := c.httpClient.DoRequest(ctx, "GET", path, nil, func(resp *http.Response) (interface{}, error) {
		var cmdResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&cmdResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if commands, ok := cmdResp["commands"].([]interface{}); ok {
			if len(commands) == 0 {
				return nil, nil
			}
			cmdMaps := make([]map[string]interface{}, len(commands))
			for i, cmd := range commands {
				if cmdMap, ok := cmd.(map[string]interface{}); ok {
					cmdMaps[i] = cmdMap
				} else {
					return nil, fmt.Errorf("invalid command format")
				}
			}
			return cmdMaps, nil
		}

		if cmdResp["command_id"] == nil || cmdResp["command_id"] == "" {
			return nil, nil
		}
		return []map[string]interface{}{cmdResp}, nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.([]map[string]interface{}), nil
}

// PushCommandLogs pushes command execution log chunks via HTTP
func (c *AgentClient) PushCommandLogs(ctx context.Context, commandID string, chunks []map[string]interface{}) ([]int64, error) {
	if len(chunks) == 0 {
		return []int64{}, nil
	}

	result, err := c.httpClient.DoRequest(ctx, "POST", "/v1/commands/logs", map[string]interface{}{
		"command_id": commandID,
		"chunks":     chunks,
	}, func(resp *http.Response) (interface{}, error) {
		var result struct {
			AckedOffsets []int64 `json:"acked_offsets"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return result.AckedOffsets, nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]int64), nil
}

// UpdateCommandStatus updates command status via HTTP
func (c *AgentClient) UpdateCommandStatus(ctx context.Context, commandID, status string, exitCode int32, errorMsg string) error {
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

	_, err := c.httpClient.DoRequest(ctx, "POST", "/v1/commands/status", payload, func(resp *http.Response) (interface{}, error) {
		return nil, nil
	})
	return err
}
