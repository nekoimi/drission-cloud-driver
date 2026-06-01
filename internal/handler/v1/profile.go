package v1

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
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
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"profiles": profiles})
}

// GetProfile returns a specific browser profile.
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	profileID := c.Param("id")
	profile, err := h.browserMgr.GetProfile(c.Request.Context(), profileID)
	if err != nil {
		h.logger.Error("get profile failed", zap.String("profile", profileID), zap.Error(err))
		notFound(c, errcode.ErrProfileNotFound, err)
		return
	}

	response.Success(c, profile)
}

// StartProfile starts a browser profile.
func (h *ProfileHandler) StartProfile(c *gin.Context) {
	profileID := c.Param("id")
	if err := h.browserMgr.StartProfile(c.Request.Context(), profileID); err != nil {
		h.logger.Error("start profile failed", zap.String("profile", profileID), zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "started"})
}

// StopProfile stops a browser profile.
func (h *ProfileHandler) StopProfile(c *gin.Context) {
	profileID := c.Param("id")
	if err := h.browserMgr.StopProfile(c.Request.Context(), profileID); err != nil {
		h.logger.Error("stop profile failed", zap.String("profile", profileID), zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "stopped"})
}
