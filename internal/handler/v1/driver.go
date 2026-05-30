package v1

import (
	"fmt"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
)

// DriverHandler handles driver API endpoints.
type DriverHandler struct {
	registry   *drivers.Registry
	browserMgr *browser.Manager
	logger     *zap.Logger
}

// NewDriverHandler creates a new driver handler.
func NewDriverHandler(registry *drivers.Registry, browserMgr *browser.Manager, logger *zap.Logger) *DriverHandler {
	return &DriverHandler{
		registry:   registry,
		browserMgr: browserMgr,
		logger:     logger,
	}
}

// getDriver extracts the platform from the URL and returns the driver.
func (h *DriverHandler) getDriver(c *gin.Context) (drivers.Driver, string, error) {
	platform := c.Param("platform")
	profileID := c.GetHeader("X-Profile-ID")
	if profileID == "" {
		profileID = c.Query("profile_id")
	}
	if profileID == "" {
		return nil, "", errcode.NewWithDetail(errcode.BadRequest, "profile id is required")
	}

	driver, err := h.registry.Get(platform, h.browserMgr)
	if err != nil {
		return nil, "", errcode.Wrap(errcode.ErrDriverNotFound, err)
	}

	return driver, profileID, nil
}

// GetCapabilities returns the capabilities of a driver.
func (h *DriverHandler) GetCapabilities(c *gin.Context) {
	platform := c.Param("platform")
	driver, err := h.registry.Get(platform, h.browserMgr)
	if err != nil {
		notFound(c, errcode.ErrDriverNotFound, err)
		return
	}

	response.Success(c, gin.H{
		"platform":     platform,
		"capabilities": driver.Capabilities(),
	})
}

// AddOfflineTask adds an offline download task.
func (h *DriverHandler) AddOfflineTask(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	var req drivers.AddTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationError(c, err)
		return
	}

	task, err := driver.AddOfflineTask(c.Request.Context(), profileID, &req)
	if err != nil {
		h.logger.Error("add offline task failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, task)
}

// GetOfflineTask returns the status of an offline download task.
func (h *DriverHandler) GetOfflineTask(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	taskID := c.Param("id")
	task, err := driver.QueryOfflineTask(c.Request.Context(), profileID, taskID)
	if err != nil {
		h.logger.Error("query offline task failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, task)
}

// RemoveOfflineTask removes an offline download task.
func (h *DriverHandler) RemoveOfflineTask(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	taskID := c.Param("id")
	if err := driver.RemoveOfflineTask(c.Request.Context(), profileID, taskID); err != nil {
		h.logger.Error("remove offline task failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "removed"})
}

// ListOfflineTasks lists all offline download tasks.
func (h *DriverHandler) ListOfflineTasks(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	taskList, err := driver.ListOfflineTasks(c.Request.Context(), profileID)
	if err != nil {
		h.logger.Error("list offline tasks failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, taskList)
}

// Mkdir creates a new directory.
func (h *DriverHandler) Mkdir(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	var req struct {
		Path       string `json:"path"`
		ParentPath string `json:"parent_path"`
		Name       string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		validationError(c, err)
		return
	}

	parentPath, name, err := parseMkdirRequest(req.Path, req.ParentPath, req.Name)
	if err != nil {
		badRequest(c, err.Error())
		return
	}

	if err := driver.Mkdir(c.Request.Context(), profileID, parentPath, name); err != nil {
		h.logger.Error("mkdir failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "created"})
}

// Remove removes a file or directory.
func (h *DriverHandler) Remove(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		validationError(c, err)
		return
	}

	if err := driver.Remove(c.Request.Context(), profileID, req.Path); err != nil {
		h.logger.Error("remove failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "removed"})
}

// Move moves a file or directory.
func (h *DriverHandler) Move(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	var req struct {
		Src string `json:"src" binding:"required"`
		Dst string `json:"dst" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		validationError(c, err)
		return
	}

	if err := driver.Move(c.Request.Context(), profileID, req.Src, req.Dst); err != nil {
		h.logger.Error("move failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "moved"})
}

// Rename renames a file or directory.
func (h *DriverHandler) Rename(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	var req struct {
		Path    string `json:"path" binding:"required"`
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		validationError(c, err)
		return
	}

	if err := driver.Rename(c.Request.Context(), profileID, req.Path, req.NewName); err != nil {
		h.logger.Error("rename failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"status": "renamed"})
}

// List lists files and directories.
func (h *DriverHandler) List(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	dirPath := c.Query("path")
	files, err := driver.List(c.Request.Context(), profileID, dirPath)
	if err != nil {
		h.logger.Error("list failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"files": files})
}

// Search searches for files.
func (h *DriverHandler) Search(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	keyword := c.Query("keyword")
	if keyword == "" {
		badRequest(c, "keyword is required")
		return
	}

	files, err := driver.Search(c.Request.Context(), profileID, keyword)
	if err != nil {
		h.logger.Error("search failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"files": files})
}

// GetDownloadURL returns the download URL for a file.
func (h *DriverHandler) GetDownloadURL(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	path := c.Query("path")
	if path == "" {
		badRequest(c, "path is required")
		return
	}

	url, err := driver.GetDownloadURL(c.Request.Context(), profileID, path)
	if err != nil {
		h.logger.Error("get download URL failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, gin.H{"url": url})
}

func parseMkdirRequest(requestPath, parentPath, name string) (string, string, error) {
	requestPath = strings.TrimSpace(requestPath)
	parentPath = strings.TrimSpace(parentPath)
	name = strings.TrimSpace(name)

	if requestPath != "" {
		cleaned := path.Clean("/" + requestPath)
		if cleaned == "/" || cleaned == "." {
			return "", "", fmt.Errorf("path must point to a directory below root")
		}

		return path.Dir(cleaned), path.Base(cleaned), nil
	}

	if name == "" {
		return "", "", fmt.Errorf("name is required")
	}
	if parentPath == "" {
		parentPath = "/"
	}

	return parentPath, name, nil
}
