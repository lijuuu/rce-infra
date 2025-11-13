package dto

// RegisterResponse represents registration response
type RegisterResponse struct {
	Token     string `json:"token"`
	NodeID    string `json:"node_id"`
	ExpiresIn int64  `json:"expires_in"`
}

// HeartbeatResponse represents heartbeat response
type HeartbeatResponse struct {
	OK bool `json:"ok"`
}

// SubmitCommandResponse represents command submission response
type SubmitCommandResponse struct {
	CommandID string `json:"command_id"`
}

// PushCommandLogsResponse represents command execution log push response
type PushCommandLogsResponse struct {
	AckedOffsets []int64 `json:"acked_offsets"`
}

// CommandStatusResponse represents status update response
type CommandStatusResponse struct {
	OK bool `json:"ok"`
}

// CommandResponse represents a command for polling
type CommandResponse struct {
	CommandID   string                 `json:"command_id"`
	CommandType string                 `json:"command_type"`
	Payload     map[string]interface{} `json:"payload"`
}

// CommandsResponse represents multiple commands for polling
type CommandsResponse struct {
	Commands []CommandResponse `json:"commands"`
}

// GetLogsResponse represents log retrieval response
type GetLogsResponse struct {
	CommandID string             `json:"command_id"`
	Logs      []LogChunkResponse `json:"logs"`
}

// LogChunkResponse represents a log chunk in response
type LogChunkResponse struct {
	ChunkIndex int64  `json:"chunk_index"`
	Stream     string `json:"stream"`
	Data       string `json:"data"`
	IsFinal    bool   `json:"is_final,omitempty"` // true if this is the final chunk (work is done)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string            `json:"error"`
	Details map[string]string `json:"details,omitempty"`
}

// ListNodesResponse represents list of nodes response
type ListNodesResponse struct {
	Nodes []NodeResponse `json:"nodes"`
}

// NodeResponse represents a node in API response
type NodeResponse struct {
	NodeID     string                 `json:"node_id"`
	Attrs      map[string]interface{} `json:"attrs"`
	LastSeenAt string                 `json:"last_seen_at"`
	Disabled   bool                   `json:"disabled"`
	IsHealthy  bool                   `json:"is_healthy"` // true if last_seen_at is within last 2 minutes
}

// ListCommandsResponse represents list of commands response
type ListCommandsResponse struct {
	Commands []CommandDetailResponse `json:"commands"`
}

// CommandDetailResponse represents a command detail in API response
type CommandDetailResponse struct {
	CommandID   string                 `json:"command_id"`
	NodeID      string                 `json:"node_id"`
	CommandType string                 `json:"command_type"`
	Payload     map[string]interface{} `json:"payload"`
	Status      string                 `json:"status"`
	ExitCode    *int                   `json:"exit_code,omitempty"`
	ErrorMsg    *string                `json:"error_msg,omitempty"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// DeleteQueuedCommandsResponse represents the response for deleting queued commands
type DeleteQueuedCommandsResponse struct {
	DeletedCount int `json:"deleted_count"`
}
