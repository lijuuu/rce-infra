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

// HTTPClient is a basic HTTP client wrapper
type HTTPClient struct {
	baseURL    string
	jwtToken   string
	httpClient *http.Client
}

// UpdateToken updates the JWT token
func (c *HTTPClient) UpdateToken(token string) {
	c.jwtToken = token
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

// DoRequest performs an HTTP request and handles the response
func (c *HTTPClient) DoRequest(ctx context.Context, method, path string, payload interface{}, handler func(*http.Response) (interface{}, error)) (interface{}, error) {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwtToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	expectedStatus := http.StatusOK
	if method == "POST" && path == "/v1/commands/logs" {
		expectedStatus = http.StatusCreated
	}

	if resp.StatusCode != expectedStatus {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{
			Code:    resp.StatusCode,
			Message: string(bodyBytes),
		}
	}

	return handler(resp)
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	Code    int
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("server returned %d: %s", e.Code, e.Message)
}

// GetStatusCode returns the HTTP status code
func (e *HTTPError) GetStatusCode() int {
	return e.Code
}
