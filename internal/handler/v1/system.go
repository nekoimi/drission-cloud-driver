package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
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
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListDrivers returns all registered drivers.
func (h *SystemHandler) ListDrivers(c *gin.Context) {
	platforms := h.registry.ListPlatforms()
	c.JSON(http.StatusOK, gin.H{
		"drivers": platforms,
	})
}
