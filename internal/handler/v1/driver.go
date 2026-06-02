package v1

import (
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/offline"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
)

// DriverHandler handles driver API endpoints.
type DriverHandler struct {
	registry      *drivers.Registry
	browserMgr    *browser.Manager
	offlineStore  offline.Store
	idempotencyMu sync.Mutex
	logger        *zap.Logger
}

// NewDriverHandler creates a new driver handler.
func NewDriverHandler(registry *drivers.Registry, browserMgr *browser.Manager, offlineStore offline.Store, logger *zap.Logger) *DriverHandler {
	return &DriverHandler{
		registry:     registry,
		browserMgr:   browserMgr,
		offlineStore: offlineStore,
		logger:       logger,
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

	capabilities := driver.Capabilities()
	response.Success(c, gin.H{
		"platform":         platform,
		"offline_download": capabilities.OfflineDownload,
		"fs_list":          capabilities.FileManage,
		"fs_search":        capabilities.Search,
		"media_url":        capabilities.DirectLink,
		"capabilities":     capabilities,
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
	req.ClientTaskID = strings.TrimSpace(req.ClientTaskID)

	if req.ClientTaskID != "" && h.offlineStore != nil {
		h.idempotencyMu.Lock()
		defer h.idempotencyMu.Unlock()

		if record, ok := h.offlineStore.GetByClientTask(driver.Platform(), profileID, req.ClientTaskID); ok {
			task := h.refreshOfflineTask(c, driver, profileID, record)
			response.Success(c, task)
			return
		}
	}

	task, err := driver.AddOfflineTask(c.Request.Context(), profileID, &req)
	if err != nil {
		h.logger.Error("add offline task failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	h.putOfflineTask(driver.Platform(), profileID, req, *task)
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
		if stored, ok := h.getStoredOfflineTask(driver.Platform(), profileID, taskID); ok {
			h.logger.Warn("query offline task failed, returning stored record",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			response.Success(c, &stored)
			return
		}
		h.logger.Error("query offline task failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	h.updateStoredOfflineTask(driver.Platform(), profileID, task)
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

	h.markStoredOfflineTaskCanceled(driver.Platform(), profileID, taskID)
	response.Success(c, gin.H{
		"task_id": taskID,
		"deleted": true,
	})
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

	h.updateStoredOfflineTasks(driver.Platform(), profileID, taskList.Items)
	response.Success(c, taskList)
}

func (h *DriverHandler) refreshOfflineTask(c *gin.Context, driver drivers.Driver, profileID string, record offline.OfflineTaskRecord) *drivers.OfflineTask {
	task, err := driver.QueryOfflineTask(c.Request.Context(), profileID, record.Task.TaskID)
	if err != nil {
		h.logger.Warn("refresh idempotent offline task failed",
			zap.String("task_id", record.Task.TaskID),
			zap.Error(err),
		)
		return &record.Task
	}

	normalizeStoredOfflineTask(&record, task)
	record.Task = *task
	if err := h.offlineStore.Update(record); err != nil {
		h.logger.Warn("update offline task store failed",
			zap.String("task_id", task.TaskID),
			zap.Error(err),
		)
	}
	return task
}

func (h *DriverHandler) putOfflineTask(platform, profileID string, req drivers.AddTaskRequest, task drivers.OfflineTask) {
	if h.offlineStore == nil {
		return
	}

	if err := h.offlineStore.Put(offline.OfflineTaskRecord{
		Platform:     platform,
		ProfileID:    profileID,
		ClientTaskID: req.ClientTaskID,
		URL:          req.URL,
		Category:     req.Category,
		SavePath:     req.SavePath,
		Metadata:     req.Metadata,
		Task:         task,
	}); err != nil {
		h.logger.Warn("put offline task store failed",
			zap.String("task_id", task.TaskID),
			zap.Error(err),
		)
	}
}

func (h *DriverHandler) updateStoredOfflineTask(platform, profileID string, task *drivers.OfflineTask) {
	if h.offlineStore == nil {
		return
	}

	record, ok := h.offlineStore.Get(task.TaskID)
	if !ok {
		return
	}
	if record.Platform != platform || record.ProfileID != profileID {
		return
	}

	normalizeStoredOfflineTask(&record, task)
	record.Task = *task
	if err := h.offlineStore.Update(record); err != nil {
		h.logger.Warn("update offline task store failed",
			zap.String("task_id", task.TaskID),
			zap.Error(err),
		)
	}
}

func normalizeStoredOfflineTask(record *offline.OfflineTaskRecord, task *drivers.OfflineTask) {
	if record == nil || task == nil {
		return
	}

	if task.SavePath == "" {
		task.SavePath = record.SavePath
	}
	if task.SavePath != "" {
		for i := range task.Files {
			task.Files[i].Path = prefixOfflineFilePath(task.SavePath, task.Files[i].Path)
		}
	}
	if task.Status == drivers.TaskCompleted && len(task.Files) == 0 {
		appendOfflineTaskWarning(task, "completed task has no consumable leaf files")
	}
}

func appendOfflineTaskWarning(task *drivers.OfflineTask, warning string) {
	for _, existing := range task.Warnings {
		if existing == warning {
			return
		}
	}
	task.Warnings = append(task.Warnings, warning)
}

func prefixOfflineFilePath(savePath, filePath string) string {
	savePath = strings.TrimSpace(savePath)
	filePath = strings.TrimSpace(filePath)
	if savePath == "" || filePath == "" {
		return filePath
	}

	savePath = "/" + strings.Trim(strings.ReplaceAll(savePath, "\\", "/"), "/")
	filePath = "/" + strings.Trim(strings.ReplaceAll(filePath, "\\", "/"), "/")
	if strings.EqualFold(filePath, savePath) || strings.HasPrefix(strings.ToLower(filePath), strings.ToLower(savePath)+"/") {
		return filePath
	}
	return savePath + filePath
}

func (h *DriverHandler) getStoredOfflineTask(platform, profileID, taskID string) (drivers.OfflineTask, bool) {
	if h.offlineStore == nil {
		return drivers.OfflineTask{}, false
	}

	record, ok := h.offlineStore.Get(taskID)
	if !ok || record.Platform != platform || record.ProfileID != profileID {
		return drivers.OfflineTask{}, false
	}

	return record.Task, true
}

func (h *DriverHandler) updateStoredOfflineTasks(platform, profileID string, tasks []drivers.OfflineTask) {
	for i := range tasks {
		h.updateStoredOfflineTask(platform, profileID, &tasks[i])
	}
}

func (h *DriverHandler) markStoredOfflineTaskCanceled(platform, profileID, taskID string) {
	if h.offlineStore == nil {
		return
	}

	record, ok := h.offlineStore.Get(taskID)
	if !ok || record.Platform != platform || record.ProfileID != profileID {
		return
	}

	record.Task.Status = drivers.TaskCanceled
	if err := h.offlineStore.Update(record); err != nil {
		h.logger.Warn("mark offline task canceled failed",
			zap.String("task_id", taskID),
			zap.Error(err),
		)
	}
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

	fileID := strings.TrimSpace(c.Query("file_id"))
	filePath := strings.TrimSpace(c.Query("path"))
	if fileID == "" && filePath == "" {
		badRequest(c, "file_id or path is required")
		return
	}

	var url string
	if fileID != "" {
		url, err = driver.GetDownloadURLByID(c.Request.Context(), profileID, fileID)
	} else {
		url, err = driver.GetDownloadURL(c.Request.Context(), profileID, filePath)
	}
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
