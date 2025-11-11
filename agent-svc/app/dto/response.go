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

// PushLogsResponse represents log push response
type PushLogsResponse struct {
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

// GetLogsResponse represents log retrieval response
type GetLogsResponse struct {
	CommandID string             `json:"command_id"`
	Logs      []LogChunkResponse `json:"logs"`
}

// LogChunkResponse represents a log chunk in response
type LogChunkResponse struct {
	Offset int64  `json:"offset"`
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string            `json:"error"`
	Details map[string]string `json:"details,omitempty"`
}
