package driver

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
)

// systemHandler handles system-related endpoints.
type systemHandler struct {
	registry *drivers.Registry
	logger   *zap.Logger
}

func newSystemHandler(registry *drivers.Registry, logger *zap.Logger) *systemHandler {
	return &systemHandler{
		registry: registry,
		logger:   logger,
	}
}

func (h *systemHandler) ListDrivers(c *gin.Context) (any, error) {
	platforms := h.registry.ListPlatforms()
	return gin.H{"drivers": platforms}, nil
}

// driverHandler handles driver API endpoints.
type driverHandler struct {
	registry      *drivers.Registry
	browserMgr    *browser.Manager
	offlineStore  offline.Store
	idempotencyMu sync.Mutex
	logger        *zap.Logger
}

func newDriverHandler(registry *drivers.Registry, browserMgr *browser.Manager, offlineStore offline.Store, logger *zap.Logger) *driverHandler {
	return &driverHandler{
		registry:     registry,
		browserMgr:   browserMgr,
		offlineStore: offlineStore,
		logger:       logger,
	}
}

func (h *driverHandler) getDriver(c *gin.Context) (drivers.Driver, string, error) {
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

func (h *driverHandler) GetCapabilities(c *gin.Context) (any, error) {
	platform := c.Param("platform")
	driver, err := h.registry.Get(platform, h.browserMgr)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDriverNotFound, err)
	}

	capabilities := driver.Capabilities()
	return gin.H{
		"platform":         platform,
		"offline_download": capabilities.OfflineDownload,
		"fs_list":          capabilities.FileManage,
		"fs_search":        capabilities.Search,
		"media_url":        capabilities.DirectLink,
		"capabilities":     capabilities,
	}, nil
}

func (h *driverHandler) AddOfflineTask(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	var req drivers.AddTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errcode.NewWithDetail(errcode.Validation, err.Error())
	}
	req.ClientTaskID = strings.TrimSpace(req.ClientTaskID)

	if req.ClientTaskID != "" && h.offlineStore != nil {
		h.idempotencyMu.Lock()
		defer h.idempotencyMu.Unlock()

		if record, ok := h.offlineStore.GetByClientTask(driver.Platform(), profileID, req.ClientTaskID); ok {
			task := h.refreshOfflineTask(c, driver, profileID, record)
			return task, nil
		}
	}

	task, err := driver.AddOfflineTask(c.Request.Context(), profileID, &req)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	h.putOfflineTask(driver.Platform(), profileID, req, *task)
	return task, nil
}

func (h *driverHandler) GetOfflineTask(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	taskID := c.Param("id")
	task, err := driver.QueryOfflineTask(c.Request.Context(), profileID, taskID)
	if err != nil {
		if stored, ok := h.getStoredOfflineTask(driver.Platform(), profileID, taskID); ok {
			h.logger.Warn("query offline task failed, returning stored record",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			return &stored, nil
		}
		return nil, errcode.Wrap(errcode.ErrTaskNotFound, err)
	}

	h.updateStoredOfflineTask(driver.Platform(), profileID, task)
	return task, nil
}

func (h *driverHandler) RemoveOfflineTask(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	taskID := c.Param("id")
	if err := driver.RemoveOfflineTask(c.Request.Context(), profileID, taskID); err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	h.markStoredOfflineTaskCanceled(driver.Platform(), profileID, taskID)
	return gin.H{"task_id": taskID, "deleted": true}, nil
}

func (h *driverHandler) ListOfflineTasks(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	taskList, err := driver.ListOfflineTasks(c.Request.Context(), profileID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	h.updateStoredOfflineTasks(driver.Platform(), profileID, taskList.Items)
	return taskList, nil
}

func (h *driverHandler) refreshOfflineTask(c *gin.Context, driver drivers.Driver, profileID string, record offline.OfflineTaskRecord) *drivers.OfflineTask {
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

func (h *driverHandler) putOfflineTask(platform, profileID string, req drivers.AddTaskRequest, task drivers.OfflineTask) {
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

func (h *driverHandler) updateStoredOfflineTask(platform, profileID string, task *drivers.OfflineTask) {
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

func (h *driverHandler) getStoredOfflineTask(platform, profileID, taskID string) (drivers.OfflineTask, bool) {
	if h.offlineStore == nil {
		return drivers.OfflineTask{}, false
	}

	record, ok := h.offlineStore.Get(taskID)
	if !ok || record.Platform != platform || record.ProfileID != profileID {
		return drivers.OfflineTask{}, false
	}

	return record.Task, true
}

func (h *driverHandler) updateStoredOfflineTasks(platform, profileID string, tasks []drivers.OfflineTask) {
	for i := range tasks {
		h.updateStoredOfflineTask(platform, profileID, &tasks[i])
	}
}

func (h *driverHandler) markStoredOfflineTaskCanceled(platform, profileID, taskID string) {
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

func (h *driverHandler) Mkdir(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	var req struct {
		Path       string `json:"path"`
		ParentPath string `json:"parent_path"`
		Name       string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errcode.NewWithDetail(errcode.Validation, err.Error())
	}

	parentPath, name, err := parseMkdirRequest(req.Path, req.ParentPath, req.Name)
	if err != nil {
		return nil, errcode.NewWithDetail(errcode.BadRequest, err.Error())
	}

	if err := driver.Mkdir(c.Request.Context(), profileID, parentPath, name); err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	return gin.H{"status": "created"}, nil
}

func (h *driverHandler) Remove(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errcode.NewWithDetail(errcode.Validation, err.Error())
	}

	if err := driver.Remove(c.Request.Context(), profileID, req.Path); err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	return gin.H{"status": "removed"}, nil
}

func (h *driverHandler) Move(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	var req struct {
		Src string `json:"src" binding:"required"`
		Dst string `json:"dst" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errcode.NewWithDetail(errcode.Validation, err.Error())
	}

	if err := driver.Move(c.Request.Context(), profileID, req.Src, req.Dst); err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	return gin.H{"status": "moved"}, nil
}

func (h *driverHandler) Rename(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	var req struct {
		Path    string `json:"path" binding:"required"`
		NewName string `json:"new_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errcode.NewWithDetail(errcode.Validation, err.Error())
	}

	if err := driver.Rename(c.Request.Context(), profileID, req.Path, req.NewName); err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	return gin.H{"status": "renamed"}, nil
}

func (h *driverHandler) List(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	dirPath := c.Query("path")
	files, err := driver.List(c.Request.Context(), profileID, dirPath)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	return gin.H{"files": files}, nil
}

func (h *driverHandler) Search(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	keyword := c.Query("keyword")
	if keyword == "" {
		return nil, errcode.NewWithDetail(errcode.BadRequest, "keyword is required")
	}

	files, err := driver.Search(c.Request.Context(), profileID, keyword)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	return gin.H{"files": files}, nil
}

func (h *driverHandler) DirName2CID(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	tester, ok := driver.(drivers.DirName2CIDTester)
	if !ok {
		return nil, errcode.NewWithDetail(errcode.BadRequest, "dirname2cid is not supported by this driver")
	}

	remotePath := strings.TrimSpace(c.Query("path"))
	if remotePath == "" {
		return nil, errcode.NewWithDetail(errcode.BadRequest, "path is required")
	}

	result, err := tester.DirName2CID(c.Request.Context(), profileID, remotePath)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	return result, nil
}

func (h *driverHandler) GetDownloadURL(c *gin.Context) (any, error) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		return nil, err
	}

	fileID := strings.TrimSpace(c.Query("file_id"))
	filePath := strings.TrimSpace(c.Query("path"))
	if fileID == "" && filePath == "" {
		return nil, errcode.NewWithDetail(errcode.BadRequest, "file_id or path is required")
	}

	var url string
	if fileID != "" {
		url, err = driver.GetDownloadURLByID(c.Request.Context(), profileID, fileID)
	} else {
		url, err = driver.GetDownloadURL(c.Request.Context(), profileID, filePath)
	}
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, err)
	}

	return gin.H{"url": url}, nil
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
