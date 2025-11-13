package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"node-agent/app/executor"
	"node-agent/app/storage"
)

// RuntimeService is the main runtime loop for command execution
type RuntimeService struct {
	storage           *storage.Store
	chunkSize         int
	chunkInterval     int
	agentClient       *AgentClient
	chunkStorageRetry *ChunkStorageRetryService
	nodeID            string
	checkInterval     time.Duration
	commandChan       chan *storage.LocalCommand
	workerCount       int
}

// NewRuntimeService creates a new runtime service
func NewRuntimeService(
	store *storage.Store,
	chunkSize, chunkInterval int,
	agentClient *AgentClient,
	chunkStorageRetry *ChunkStorageRetryService,
	nodeID string,
	checkIntervalSec int,
	workerCount int,
	channelSize int,
) *RuntimeService {
	return &RuntimeService{
		storage:           store,
		chunkSize:         chunkSize,
		chunkInterval:     chunkInterval,
		agentClient:       agentClient,
		chunkStorageRetry: chunkStorageRetry,
		nodeID:            nodeID,
		checkInterval:     time.Duration(checkIntervalSec) * time.Second,
		commandChan:       make(chan *storage.LocalCommand, channelSize),
		workerCount:       workerCount,
	}
}

// Start starts the main runtime loop
func (r *RuntimeService) Start(ctx context.Context) {
	for i := 0; i < r.workerCount; i++ {
		go r.worker(ctx, i)
	}

	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	r.enqueueQueuedCommands(ctx)

	for {
		select {
		case <-ctx.Done():
			close(r.commandChan)
			return
		case <-ticker.C:
			r.requestCommands(ctx)
			r.enqueueQueuedCommands(ctx)
		}
	}
}

// worker processes commands from the channel
func (r *RuntimeService) worker(ctx context.Context, workerID int) {
	for cmd := range r.commandChan {
		r.agentClient.UpdateCommandStatus(ctx, cmd.CommandID, "running", 0, "")
		r.executeCommand(ctx, cmd)
	}
}

// requestCommands requests commands from agent-svc
func (r *RuntimeService) requestCommands(ctx context.Context) {
	cmdResps, err := r.agentClient.PollCommands(ctx, r.nodeID, 5)
	if err != nil {
		return
	}

	if len(cmdResps) == 0 {
		return
	}

	// Process all returned commands
	for _, cmdResp := range cmdResps {
		if cmdResp == nil || cmdResp["command_id"] == nil {
			continue
		}

		commandID, ok := cmdResp["command_id"].(string)
		if !ok || commandID == "" {
			continue
		}

		isFinished, err := r.storage.IsCommandFinished(ctx, commandID)
		if err != nil {
			fmt.Printf("failed to check command status: %v\n", err)
			continue
		}
		if isFinished {
			fmt.Printf("command %s already executed, skipping\n", commandID)
			continue
		}

		commandType, ok := cmdResp["command_type"].(string)
		if !ok {
			continue
		}

		payloadJSON, ok := cmdResp["payload_json"].(string)
		if !ok {
			if payload, ok := cmdResp["payload"].(map[string]interface{}); ok {
				jsonBytes, _ := json.Marshal(payload)
				payloadJSON = string(jsonBytes)
			} else {
				continue
			}
		}

		if err := r.storage.SaveCommandWithStatus(ctx, commandID, commandType, payloadJSON, "running"); err != nil {
			fmt.Printf("failed to save command: %v\n", err)
			continue
		}

		cmd := &storage.LocalCommand{
			CommandID:   commandID,
			CommandType: commandType,
			Payload:     payloadJSON,
			Status:      "running",
		}

		select {
		case r.commandChan <- cmd:
		case <-ctx.Done():
			return
		default:
			r.storage.UpdateCommandStatus(ctx, commandID, "queued", nil, nil)
		}
	}
}

// enqueueQueuedCommands enqueues queued commands from local storage
func (r *RuntimeService) enqueueQueuedCommands(ctx context.Context) {
	for {
		cmd, err := r.storage.GetNextQueuedCommand(ctx)
		if err != nil {
			fmt.Printf("failed to get queued command: %v\n", err)
			return
		}
		if cmd == nil {
			return
		}

		select {
		case r.commandChan <- cmd:
		case <-ctx.Done():
			return
		default:
			r.storage.UpdateCommandStatus(ctx, cmd.CommandID, "queued", nil, nil)
			return
		}
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
	// case "UpdateAgent":
	// 	r.executeUpdateAgent(ctx, cmd.CommandID, payload)
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

	timeoutSec := 300
	if ts, ok := payload["timeout_sec"].(float64); ok && ts > 0 {
		timeoutSec = int(ts)
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	command := exec.CommandContext(execCtx, "sh", "-c", cmdStr)
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

	if err := command.Start(); err != nil {
		r.handleCommandError(ctx, commandID, fmt.Sprintf("failed to start command: %v", err))
		return
	}

	chunker := executor.NewChunker(r.chunkSize, r.chunkInterval)
	chunkChan := chunker.StartChunking(execCtx, stdout, stderr)

	go func() {
		for chunk := range chunkChan {
			r.storage.SaveLogChunk(execCtx, commandID, chunk.ChunkIndex, chunk.Stream, chunk.Data)

			chunkMap := map[string]interface{}{
				"chunk_index": chunk.ChunkIndex,
				"stream":      chunk.Stream,
				"data":        chunk.Data,
				"is_final":    chunk.IsFinal,
			}
			ackedChunkIndexes, err := r.agentClient.PushCommandLogs(execCtx, commandID, []map[string]interface{}{chunkMap})
			if err == nil && len(ackedChunkIndexes) > 0 {
				r.storage.MarkChunksAcked(execCtx, commandID, ackedChunkIndexes)
			}
		}
	}()

	err = command.Wait()
	chunker.FinalFlush()

	exitCode := 0
	status := "success"
	errorMsg := ""

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			status = "timeout"
			exitCode = -1
			errorMsg = fmt.Sprintf("command execution timed out after %d seconds", timeoutSec)
		} else if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
			status = "failed"
			errorMsg = fmt.Sprintf("command exited with code %d", exitCode)
		} else {
			exitCode = -1
			status = "failed"
			errorMsg = fmt.Sprintf("command execution failed: %v", err)
		}
	}

	r.chunkStorageRetry.UploadChunksForCommand(context.Background(), commandID, true)
	r.storage.UpdateCommandStatus(ctx, commandID, status, &exitCode, &errorMsg)

	exitCodeInt32 := int32(exitCode)
	r.agentClient.UpdateCommandStatus(ctx, commandID, status, exitCodeInt32, errorMsg)
}

// handleCommandError handles command execution errors
func (r *RuntimeService) handleCommandError(ctx context.Context, commandID, errorMsg string) {
	exitCode := -1
	r.storage.UpdateCommandStatus(ctx, commandID, "failed", &exitCode, &errorMsg)
	r.agentClient.UpdateCommandStatus(ctx, commandID, "failed", int32(exitCode), errorMsg)
}
