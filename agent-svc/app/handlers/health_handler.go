package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health handles health check
func (h *HealthHandler) Health(c *gin.Context) {
	respondJSON(c, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// Ready handles readiness check
func (h *HealthHandler) Ready(c *gin.Context) {
	respondJSON(c, http.StatusOK, map[string]string{
		"status": "ready",
	})
}
