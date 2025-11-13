package dto

// RegisterRequest represents node registration request
type RegisterRequest struct {
	NodeID string                 `json:"node_id" validate:"required"`
	Attrs  map[string]interface{} `json:"attrs,omitempty"`
}

// HeartbeatRequest represents heartbeat request
type HeartbeatRequest struct {
	NodeID string `json:"node_id" validate:"required"`
}

// SubmitCommandRequest represents command submission request (one-to-one)
type SubmitCommandRequest struct {
	CommandType string                 `json:"command_type" validate:"required"`
	NodeID      string                 `json:"node_id" validate:"required"`
	Payload     map[string]interface{} `json:"payload" validate:"required"`
}

// PushCommandLogsRequest represents command execution log chunk push request
type PushCommandLogsRequest struct {
	CommandID string            `json:"command_id" validate:"required"`
	Chunks    []LogChunkRequest `json:"chunks" validate:"required"`
}

// LogChunkRequest represents a single log chunk
type LogChunkRequest struct {
	ChunkIndex int64  `json:"chunk_index" validate:"required,min=0"`
	Stream     string `json:"stream" validate:"required,oneof=stdout stderr"`
	Data       string `json:"data" validate:"required"`
	IsFinal    bool   `json:"is_final,omitempty"` // true if this is the final chunk (work is done)
}

// CommandStatusRequest represents command status update
type CommandStatusRequest struct {
	CommandID string `json:"command_id" validate:"required"`
	Status    string `json:"status" validate:"required,oneof=queued running success failed timeout"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

// PollCommandRequest represents command polling request (query params)
type PollCommandRequest struct {
	NodeID string
	Wait   int // seconds
}
