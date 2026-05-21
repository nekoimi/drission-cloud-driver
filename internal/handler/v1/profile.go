package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
)

// ProfileHandler handles browser profile endpoints.
type ProfileHandler struct {
	browserMgr *browser.Manager
	logger     *zap.Logger
}

// NewProfileHandler creates a new profile handler.
func NewProfileHandler(browserMgr *browser.Manager, logger *zap.Logger) *ProfileHandler {
	return &ProfileHandler{
		browserMgr: browserMgr,
		logger:     logger,
	}
}

// ListProfiles returns all browser profiles.
func (h *ProfileHandler) ListProfiles(c *gin.Context) {
	profiles, err := h.browserMgr.ListProfiles(c.Request.Context())
	if err != nil {
		h.logger.Error("list profiles failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list profiles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"profiles": profiles})
}

// GetProfile returns a specific browser profile.
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	// TODO: implement get profile
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// StartProfile starts a browser profile.
func (h *ProfileHandler) StartProfile(c *gin.Context) {
	// TODO: implement start profile
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// StopProfile stops a browser profile.
func (h *ProfileHandler) StopProfile(c *gin.Context) {
	// TODO: implement stop profile
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
