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

// SubmitCommand handles command submission (admin endpoint - one-to-one only)
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

// GetNextCommand handles command polling (long polling)
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

	// Poll with timeout
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout - return empty response
			respondJSON(c, http.StatusOK, dto.CommandResponse{})
			return
		case <-ticker.C:
			cmd, err := h.commandService.GetNextCommand(ctx, nodeID)
			if err != nil {
				respondError(c, http.StatusInternalServerError, "failed to get command", nil)
				return
			}
			if cmd != nil {
				respondJSON(c, http.StatusOK, dto.CommandResponse{
					CommandID:   cmd.CommandID.String(),
					CommandType: cmd.CommandType,
					Payload:     cmd.Payload,
				})
				return
			}
		}
	}
}

// PushLogs handles log chunk push
func (h *CommandHandler) PushLogs(c *gin.Context) {
	nodeID := h.getNodeIDFromToken(c)
	if nodeID == "" {
		respondError(c, http.StatusUnauthorized, "invalid token", nil)
		return
	}

	var req dto.PushLogsRequest
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
			CommandID: req.CommandID,
			Offset:    chunkReq.Offset,
			Stream:    chunkReq.Stream,
			Data:      chunkReq.Data,
			Encoding:  "utf-8",
		}
	}

	ctx := c.Request.Context()
	ackedOffsets, err := h.logService.PushLogs(ctx, commandID, nodeID, chunks)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	respondJSON(c, http.StatusCreated, dto.PushLogsResponse{
		AckedOffsets: ackedOffsets,
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

// GetCommandLogs handles fetching logs for a command (with optional offset filtering)
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

	// Optional offset parameter - fetch logs after this offset
	var afterOffset *int64
	if offsetStr := c.Query("after_offset"); offsetStr != "" {
		if offset, err := strconv.ParseInt(offsetStr, 10, 64); err == nil && offset >= 0 {
			afterOffset = &offset
		}
	}

	ctx := c.Request.Context()
	logs, err := h.logService.GetCommandLogs(ctx, commandID, afterOffset)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// Convert to response format
	logResponses := make([]dto.LogChunkResponse, len(logs))
	for i, log := range logs {
		logResponses[i] = dto.LogChunkResponse{
			Offset: log.Offset,
			Stream: log.Stream,
			Data:   log.Data,
		}
	}

	respondJSON(c, http.StatusOK, dto.GetLogsResponse{
		CommandID: commandIDStr,
		Logs:      logResponses,
	})
}

// getNodeIDFromToken extracts node ID from JWT token in Authorization header
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
