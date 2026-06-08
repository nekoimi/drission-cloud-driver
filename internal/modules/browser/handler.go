package browser

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
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

func (h *profileHandler) ListProfiles(c *gin.Context) {
	profiles, err := h.browserMgr.ListProfiles(c.Request.Context())
	if err != nil {
		h.logger.Error("list profiles failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"profiles": profiles})
}

func (h *profileHandler) GetProfile(c *gin.Context) {
	profileID := c.Param("id")
	profile, err := h.browserMgr.GetProfile(c.Request.Context(), profileID)
	if err != nil {
		h.logger.Error("get profile failed", zap.String("profile", profileID), zap.Error(err))
		notFound(c, errcode.ErrProfileNotFound, err)
		return
	}

	response.Success(c, profile)
}

func (h *profileHandler) StartProfile(c *gin.Context) {
	profileID := c.Param("id")
	if err := h.browserMgr.StartProfile(c.Request.Context(), profileID); err != nil {
		h.logger.Error("start profile failed", zap.String("profile", profileID), zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "started"})
}

func (h *profileHandler) StopProfile(c *gin.Context) {
	profileID := c.Param("id")
	if err := h.browserMgr.StopProfile(c.Request.Context(), profileID); err != nil {
		h.logger.Error("stop profile failed", zap.String("profile", profileID), zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "stopped"})
}

func notFound(c *gin.Context, code *errcode.Code, err error) {
	response.ErrorWithMsg(c, http.StatusNotFound, code, err.Error())
}

func operationFailed(c *gin.Context, err error) {
	if appErr, ok := response.IsAppError(err); ok {
		response.AppErr(c, appErr)
		return
	}

	response.ErrorWithMsg(c, http.StatusInternalServerError, errcode.ErrOperationFailed, err.Error())
}
