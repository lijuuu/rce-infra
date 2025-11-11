package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"node-agent/app/clients"
	"node-agent/app/executor"
	"node-agent/app/storage"
)

// RuntimeService is the main runtime loop for command execution
type RuntimeService struct {
	storage       *storage.Store
	executor      *executor.Executor
	chunkSize     int
	chunkInterval int
	resultSender  *executor.ResultSender
	offlineBuffer *OfflineBufferService
	httpClient    *clients.HTTPClient
	nodeID        string
	checkInterval time.Duration
}

// NewRuntimeService creates a new runtime service
func NewRuntimeService(
	store *storage.Store,
	executor *executor.Executor,
	chunkSize, chunkInterval int,
	resultSender *executor.ResultSender,
	offlineBuffer *OfflineBufferService,
	httpClient *clients.HTTPClient,
	nodeID string,
	checkIntervalSec int,
) *RuntimeService {
	return &RuntimeService{
		storage:       store,
		executor:      executor,
		chunkSize:     chunkSize,
		chunkInterval: chunkInterval,
		resultSender:  resultSender,
		offlineBuffer: offlineBuffer,
		httpClient:    httpClient,
		nodeID:        nodeID,
		checkInterval: time.Duration(checkIntervalSec) * time.Second,
	}
}

// Start starts the main runtime loop
func (r *RuntimeService) Start(ctx context.Context) {
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	// Process any queued commands on startup
	r.processQueuedCommands(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Request commands from agent-svc via Kong (HTTP)
			r.requestCommands(ctx)
			// Process queued commands
			r.processQueuedCommands(ctx)
		}
	}
}

// requestCommands requests commands from agent-svc via Kong (HTTP)
func (r *RuntimeService) requestCommands(ctx context.Context) {
	// Request command via HTTP
	cmdResp, err := r.httpClient.PollCommands(ctx, r.nodeID, 5) // 5 second wait
	if err != nil {
		// No command available or error - continue
		return
	}

	if cmdResp == nil || cmdResp["command_id"] == nil {
		return
	}

	commandID, ok := cmdResp["command_id"].(string)
	if !ok || commandID == "" {
		return
	}

	commandType, ok := cmdResp["command_type"].(string)
	if !ok {
		return
	}

	payloadJSON, ok := cmdResp["payload_json"].(string)
	if !ok {
		// Try as object
		if payload, ok := cmdResp["payload"].(map[string]interface{}); ok {
			jsonBytes, _ := json.Marshal(payload)
			payloadJSON = string(jsonBytes)
		} else {
			return
		}
	}

	// Save command locally
	if err := r.storage.SaveCommand(ctx, commandID, commandType, payloadJSON); err != nil {
		fmt.Printf("failed to save command: %v\n", err)
	}
}

// processQueuedCommands processes queued commands from local storage
func (r *RuntimeService) processQueuedCommands(ctx context.Context) {
	for {
		cmd, err := r.storage.GetNextQueuedCommand(ctx)
		if err != nil {
			fmt.Printf("failed to get queued command: %v\n", err)
			return
		}
		if cmd == nil {
			return
		}

		// Update status to running
		r.storage.UpdateCommandStatus(ctx, cmd.CommandID, "running", nil, nil)

		// Execute command
		r.executeCommand(ctx, cmd)
	}
}

// executeCommand executes a command
func (r *RuntimeService) executeCommand(ctx context.Context, cmd *storage.LocalCommand) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
		r.handleCommandError(ctx, cmd.CommandID, fmt.Sprintf("invalid payload: %v", err))
		return
	}

	// Handle different command types
	switch cmd.CommandType {
	case "RunCommand":
		r.executeRunCommand(ctx, cmd.CommandID, payload)
	case "UpdateAgent":
		r.executeUpdateAgent(ctx, cmd.CommandID, payload)
	case "UpdatePackage":
		r.executeUpdatePackage(ctx, cmd.CommandID, payload)
	default:
		r.handleCommandError(ctx, cmd.CommandID, fmt.Sprintf("unknown command type: %s", cmd.CommandType))
	}
}

// executeRunCommand executes a RunCommand
func (r *RuntimeService) executeRunCommand(ctx context.Context, commandID string, payload map[string]interface{}) {
	cmdStr, ok := payload["cmd"].(string)
	if !ok {
		r.handleCommandError(ctx, commandID, "cmd field is required")
		return
	}

	timeoutSec := 120
	if ts, ok := payload["timeout_sec"].(float64); ok {
		timeoutSec = int(ts)
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// Create command
	command := exec.CommandContext(execCtx, "sh", "-c", cmdStr)

	// Get stdout and stderr pipes
	stdout, err := command.StdoutPipe()
	if err != nil {
		r.handleCommandError(ctx, commandID, fmt.Sprintf("failed to create stdout pipe: %v", err))
		return
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		r.handleCommandError(ctx, commandID, fmt.Sprintf("failed to create stderr pipe: %v", err))
		return
	}

	// Start command
	if err := command.Start(); err != nil {
		r.handleCommandError(ctx, commandID, fmt.Sprintf("failed to start command: %v", err))
		return
	}

	// Create new chunker for this command
	chunker := executor.NewChunker(r.chunkSize, r.chunkInterval)
	chunkChan := chunker.StartChunking(execCtx, stdout, stderr)

	// Save chunks as they arrive
	go func() {
		for chunk := range chunkChan {
			r.storage.SaveLogChunk(execCtx, commandID, chunk.Offset, chunk.Stream, chunk.Data)

			// Try to send immediately (best effort)
			chunkMap := map[string]interface{}{
				"offset": chunk.Offset,
				"stream": chunk.Stream,
				"data":   chunk.Data,
			}
			ackedOffsets, err := r.resultSender.PushLogs(execCtx, commandID, []map[string]interface{}{chunkMap})
			if err == nil && len(ackedOffsets) > 0 {
				r.storage.MarkChunksAcked(execCtx, commandID, ackedOffsets)
			}
		}
	}()

	// Wait for command to complete
	err = command.Wait()
	chunker.FinalFlush()

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Upload any remaining pending chunks
	r.offlineBuffer.UploadChunksForCommand(context.Background(), commandID)

	// Update status
	status := "success"
	errorMsg := ""
	if exitCode != 0 {
		status = "failed"
		errorMsg = fmt.Sprintf("command exited with code %d", exitCode)
	}

	r.storage.UpdateCommandStatus(ctx, commandID, status, &exitCode, &errorMsg)

	// Update status via HTTP
	exitCodeInt32 := int32(exitCode)
	r.resultSender.UpdateStatus(ctx, commandID, status, exitCodeInt32, errorMsg)
}

// executeUpdateAgent handles UpdateAgent command (placeholder)
func (r *RuntimeService) executeUpdateAgent(ctx context.Context, commandID string, payload map[string]interface{}) {
	// Implementation would download and update agent binary
	exitCode := 0
	r.storage.UpdateCommandStatus(ctx, commandID, "success", &exitCode, nil)
	r.resultSender.UpdateStatus(ctx, commandID, "success", int32(exitCode), "")
}

// executeUpdatePackage handles UpdatePackage command (placeholder)
func (r *RuntimeService) executeUpdatePackage(ctx context.Context, commandID string, payload map[string]interface{}) {
	// Implementation would use package manager to install/remove/upgrade packages
	exitCode := 0
	r.storage.UpdateCommandStatus(ctx, commandID, "success", &exitCode, nil)
	r.resultSender.UpdateStatus(ctx, commandID, "success", int32(exitCode), "")
}

// handleCommandError handles command execution errors
func (r *RuntimeService) handleCommandError(ctx context.Context, commandID, errorMsg string) {
	exitCode := -1
	r.storage.UpdateCommandStatus(ctx, commandID, "failed", &exitCode, &errorMsg)
	r.resultSender.UpdateStatus(ctx, commandID, "failed", int32(exitCode), errorMsg)
}
