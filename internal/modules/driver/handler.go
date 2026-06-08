package driver

import (
	"fmt"
	"net/http"
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

func (h *systemHandler) ListDrivers(c *gin.Context) {
	platforms := h.registry.ListPlatforms()
	response.Success(c, gin.H{
		"drivers": platforms,
	})
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

func (h *driverHandler) GetCapabilities(c *gin.Context) {
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

func (h *driverHandler) AddOfflineTask(c *gin.Context) {
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

func (h *driverHandler) GetOfflineTask(c *gin.Context) {
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

func (h *driverHandler) RemoveOfflineTask(c *gin.Context) {
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

func (h *driverHandler) ListOfflineTasks(c *gin.Context) {
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

func (h *driverHandler) Mkdir(c *gin.Context) {
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

func (h *driverHandler) Remove(c *gin.Context) {
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

func (h *driverHandler) Move(c *gin.Context) {
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

func (h *driverHandler) Rename(c *gin.Context) {
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

func (h *driverHandler) List(c *gin.Context) {
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

func (h *driverHandler) Search(c *gin.Context) {
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

func (h *driverHandler) DirName2CID(c *gin.Context) {
	driver, profileID, err := h.getDriver(c)
	if err != nil {
		appError(c, err)
		return
	}

	tester, ok := driver.(drivers.DirName2CIDTester)
	if !ok {
		badRequest(c, "dirname2cid is not supported by this driver")
		return
	}

	remotePath := strings.TrimSpace(c.Query("path"))
	if remotePath == "" {
		badRequest(c, "path is required")
		return
	}

	result, err := tester.DirName2CID(c.Request.Context(), profileID, remotePath)
	if err != nil {
		h.logger.Error("dirname2cid failed", zap.Error(err))
		operationFailed(c, err)
		return
	}

	response.Success(c, result)
}

func (h *driverHandler) GetDownloadURL(c *gin.Context) {
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

func badRequest(c *gin.Context, msg string) {
	response.ErrorWithMsg(c, http.StatusBadRequest, errcode.ErrInvalidRequest, msg)
}

func validationError(c *gin.Context, err error) {
	response.ValidationError(c, err.Error())
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

func appError(c *gin.Context, err error) {
	if appErr, ok := response.IsAppError(err); ok {
		response.AppErr(c, appErr)
		return
	}

	operationFailed(c, err)
}
