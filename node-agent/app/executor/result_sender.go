package executor

import (
	"context"

	"node-agent/app/clients"
)

// ResultSender sends command results via HTTP
type ResultSender struct {
	httpClient *clients.HTTPClient
}

// NewResultSender creates a new HTTP result sender
func NewResultSender(httpClient *clients.HTTPClient) *ResultSender {
	return &ResultSender{
		httpClient: httpClient,
	}
}

// PushLogs sends log chunks via HTTP
func (s *ResultSender) PushLogs(ctx context.Context, commandID string, chunks []map[string]interface{}) ([]int64, error) {
	return s.httpClient.PushLogs(ctx, commandID, chunks)
}

// UpdateStatus sends command status update via HTTP
func (s *ResultSender) UpdateStatus(ctx context.Context, commandID, status string, exitCode int32, errorMsg string) error {
	return s.httpClient.UpdateCommandStatus(ctx, commandID, status, exitCode, errorMsg)
}
