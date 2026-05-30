package v1

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
)

// SystemHandler handles system-related endpoints.
type SystemHandler struct {
	registry *drivers.Registry
	logger   *zap.Logger
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(registry *drivers.Registry, logger *zap.Logger) *SystemHandler {
	return &SystemHandler{
		registry: registry,
		logger:   logger,
	}
}

// Health returns the health status.
func (h *SystemHandler) Health(c *gin.Context) {
	response.Success(c, gin.H{"status": "ok"})
}

// ListDrivers returns all registered drivers.
func (h *SystemHandler) ListDrivers(c *gin.Context) {
	platforms := h.registry.ListPlatforms()
	response.Success(c, gin.H{
		"drivers": platforms,
	})
}
