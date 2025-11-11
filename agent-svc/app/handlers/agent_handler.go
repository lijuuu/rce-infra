package handlers

import (
	"net/http"

	"agent-svc/app/clients"
	"agent-svc/app/dto"
	"agent-svc/app/services"
	"agent-svc/app/utils"

	"github.com/gin-gonic/gin"
)

// respondJSON sends a JSON response
func respondJSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

// respondError sends an error response
func respondError(c *gin.Context, status int, message string, details map[string]string) {
	c.JSON(status, dto.ErrorResponse{
		Error:   message,
		Details: details,
	})
}

// AgentHandler handles agent-related endpoints
type AgentHandler struct {
	jwtService *services.JWTService
	storage    clients.StorageAdapter
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(jwtService *services.JWTService, storage clients.StorageAdapter) *AgentHandler {
	return &AgentHandler{
		jwtService: jwtService,
		storage:    storage,
	}
}

// Register handles node registration
func (h *AgentHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		respondError(c, http.StatusBadRequest, "validation failed", map[string]string{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	var publicKey *string
	if req.PublicKey != "" {
		publicKey = &req.PublicKey
	}

	attrs := req.Attrs
	if attrs == nil {
		attrs = make(map[string]interface{})
	}

	if err := h.storage.RegisterNode(ctx, req.NodeID, publicKey, attrs); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to register node", nil)
		return
	}

	token, err := h.jwtService.GenerateToken(req.NodeID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to generate token", nil)
		return
	}

	respondJSON(c, http.StatusOK, dto.RegisterResponse{
		Token:     token,
		NodeID:    req.NodeID,
		ExpiresIn: 86400,
	})
}

// Heartbeat handles node heartbeat
func (h *AgentHandler) Heartbeat(c *gin.Context) {
	var req dto.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body", nil)
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		respondError(c, http.StatusBadRequest, "validation failed", map[string]string{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	if err := h.storage.UpdateNodeLastSeen(ctx, req.NodeID); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update heartbeat", nil)
		return
	}

	respondJSON(c, http.StatusOK, dto.HeartbeatResponse{OK: true})
}
