package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"agent-svc/app/clients"
	"agent-svc/app/domains"
	"agent-svc/app/dto"
	"agent-svc/app/services"
	"agent-svc/app/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CommandHandler handles command-related endpoints
type CommandHandler struct {
	commandService *services.CommandService
	logService     *services.LogService
	jwtService     *services.JWTService
	storage        clients.StorageAdapter
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(
	commandService *services.CommandService,
	logService *services.LogService,
	jwtService *services.JWTService,
	storage clients.StorageAdapter,
) *CommandHandler {
	return &CommandHandler{
		commandService: commandService,
		logService:     logService,
		jwtService:     jwtService,
		storage:        storage,
	}
}

// SubmitCommand handles command submission
func (h *CommandHandler) SubmitCommand(c *gin.Context) {
	var req dto.SubmitCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		respondError(c, http.StatusBadRequest, "validation failed", map[string]string{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	commandID, err := h.commandService.SubmitCommand(ctx, req.CommandType, req.NodeID, req.Payload)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	respondJSON(c, http.StatusCreated, dto.SubmitCommandResponse{
		CommandID: commandID.String(),
	})
}

// GetNextCommand handles command polling
func (h *CommandHandler) GetNextCommand(c *gin.Context) {
	nodeID := h.getNodeIDFromToken(c)
	if nodeID == "" {
		respondError(c, http.StatusUnauthorized, "invalid token", nil)
		return
	}

	waitSeconds := 30
	if waitStr := c.Query("wait"); waitStr != "" {
		if w, err := strconv.Atoi(waitStr); err == nil && w > 0 && w <= 60 {
			waitSeconds = w
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(waitSeconds)*time.Second)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			respondJSON(c, http.StatusOK, dto.CommandsResponse{Commands: []dto.CommandResponse{}})
			return
		case <-ticker.C:
			cmds, err := h.commandService.GetNextCommand(ctx, nodeID)
			if err != nil {
				respondError(c, http.StatusInternalServerError, "failed to get command", nil)
				return
			}
			if len(cmds) > 0 {
				commandResponses := make([]dto.CommandResponse, len(cmds))
				for i, cmd := range cmds {
					commandResponses[i] = dto.CommandResponse{
						CommandID:   cmd.CommandID.String(),
						CommandType: cmd.CommandType,
						Payload:     cmd.Payload,
					}
				}
				respondJSON(c, http.StatusOK, dto.CommandsResponse{
					Commands: commandResponses,
				})
				return
			}
		}
	}
}

// PushCommandLogs handles command execution log chunk push
func (h *CommandHandler) PushCommandLogs(c *gin.Context) {
	nodeID := h.getNodeIDFromToken(c)
	if nodeID == "" {
		respondError(c, http.StatusUnauthorized, "invalid token", nil)
		return
	}

	var req dto.PushCommandLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		respondError(c, http.StatusBadRequest, "validation failed", map[string]string{"error": err.Error()})
		return
	}

	commandID, err := uuid.Parse(req.CommandID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid command_id", nil)
		return
	}

	chunks := make([]domains.CommandLog, len(req.Chunks))
	for i, chunkReq := range req.Chunks {
		chunks[i] = domains.CommandLog{
			CommandID:  req.CommandID,
			ChunkIndex: chunkReq.ChunkIndex,
			Stream:     chunkReq.Stream,
			Data:       chunkReq.Data,
			Encoding:   "utf-8",
			IsFinal:    chunkReq.IsFinal,
		}
	}

	ctx := c.Request.Context()
	ackedChunkIndexes, err := h.logService.PushCommandLogs(ctx, commandID, nodeID, chunks)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	respondJSON(c, http.StatusCreated, dto.PushCommandLogsResponse{
		AckedOffsets: ackedChunkIndexes,
	})
}

// UpdateCommandStatus handles command status update
func (h *CommandHandler) UpdateCommandStatus(c *gin.Context) {
	nodeID := h.getNodeIDFromToken(c)
	if nodeID == "" {
		respondError(c, http.StatusUnauthorized, "invalid token", nil)
		return
	}

	var req dto.CommandStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		respondError(c, http.StatusBadRequest, "validation failed", map[string]string{"error": err.Error()})
		return
	}

	commandID, err := uuid.Parse(req.CommandID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid command_id", nil)
		return
	}

	ctx := c.Request.Context()
	var errorMsg *string
	if req.ErrorMsg != "" {
		errorMsg = &req.ErrorMsg
	}

	if err := h.commandService.UpdateCommandStatus(ctx, commandID, nodeID, req.Status, req.ExitCode, errorMsg); err != nil {
		respondError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	respondJSON(c, http.StatusOK, dto.CommandStatusResponse{OK: true})
}

// GetCommandLogs handles fetching logs for a command
func (h *CommandHandler) GetCommandLogs(c *gin.Context) {
	commandIDStr := c.Param("command_id")
	if commandIDStr == "" {
		respondError(c, http.StatusBadRequest, "command_id is required", nil)
		return
	}

	commandID, err := uuid.Parse(commandIDStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid command_id", nil)
		return
	}

	var afterChunkIndex *int64
	if chunkIndexStr := c.Query("after_chunk_index"); chunkIndexStr != "" {
		if chunkIndex, err := strconv.ParseInt(chunkIndexStr, 10, 64); err == nil && chunkIndex >= 0 {
			afterChunkIndex = &chunkIndex
		}
	}

	ctx := c.Request.Context()
	logs, err := h.logService.GetCommandLogs(ctx, commandID, afterChunkIndex)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	logResponses := make([]dto.LogChunkResponse, len(logs))
	for i, log := range logs {
		logResponses[i] = dto.LogChunkResponse{
			ChunkIndex: log.ChunkIndex,
			Stream:     log.Stream,
			Data:       log.Data,
			IsFinal:    log.IsFinal,
		}
	}

	respondJSON(c, http.StatusOK, dto.GetLogsResponse{
		CommandID: commandIDStr,
		Logs:      logResponses,
	})
}

// getNodeIDFromToken extracts node ID from JWT token
func (h *CommandHandler) getNodeIDFromToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return ""
	}

	token := authHeader[7:]
	nodeID, err := h.jwtService.ValidateToken(token)
	if err != nil {
		return ""
	}

	return nodeID
}

// ListCommands handles listing commands (optionally filtered by node_id)
func (h *CommandHandler) ListCommands(c *gin.Context) {
	var nodeID *string
	if nodeIDStr := c.Query("node_id"); nodeIDStr != "" {
		nodeID = &nodeIDStr
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	ctx := c.Request.Context()
	commands, err := h.storage.ListCommands(ctx, nodeID, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list commands", nil)
		return
	}

	commandResponses := make([]dto.CommandDetailResponse, len(commands))
	for i, cmd := range commands {
		commandResponses[i] = dto.CommandDetailResponse{
			CommandID:   cmd.CommandID.String(),
			NodeID:      cmd.NodeID,
			CommandType: cmd.CommandType,
			Payload:     cmd.Payload,
			Status:      cmd.Status,
			ExitCode:    cmd.ExitCode,
			ErrorMsg:    cmd.ErrorMsg,
			CreatedAt:   cmd.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   cmd.UpdatedAt.Format(time.RFC3339),
		}
	}

	respondJSON(c, http.StatusOK, dto.ListCommandsResponse{Commands: commandResponses})
}

// DeleteQueuedCommands handles deletion of queued commands
func (h *CommandHandler) DeleteQueuedCommands(c *gin.Context) {
	var nodeID *string
	if nodeIDStr := c.Query("node_id"); nodeIDStr != "" {
		nodeID = &nodeIDStr
	}

	ctx := c.Request.Context()
	count, err := h.commandService.DeleteQueuedCommands(ctx, nodeID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	respondJSON(c, http.StatusOK, dto.DeleteQueuedCommandsResponse{
		DeletedCount: count,
	})
}
