package browser

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
)

type profileHandler struct {
	browserMgr *browser.Manager
	logger     *zap.Logger
}

func newProfileHandler(browserMgr *browser.Manager, logger *zap.Logger) *profileHandler {
	return &profileHandler{
		browserMgr: browserMgr,
		logger:     logger,
	}
}

func (h *profileHandler) ListProfiles(c *gin.Context) (any, error) {
	profiles, err := h.browserMgr.ListProfiles(c.Request.Context())
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}
	return gin.H{"profiles": profiles}, nil
}

func (h *profileHandler) GetProfile(c *gin.Context) (any, error) {
	profileID := c.Param("id")
	profile, err := h.browserMgr.GetProfile(c.Request.Context(), profileID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrProfileNotFound, err)
	}
	return profile, nil
}

func (h *profileHandler) StartProfile(c *gin.Context) (any, error) {
	profileID := c.Param("id")
	if err := h.browserMgr.StartProfile(c.Request.Context(), profileID); err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}
	return gin.H{"status": "started"}, nil
}

func (h *profileHandler) StopProfile(c *gin.Context) (any, error) {
	profileID := c.Param("id")
	if err := h.browserMgr.StopProfile(c.Request.Context(), profileID); err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}
	return gin.H{"status": "stopped"}, nil
}
