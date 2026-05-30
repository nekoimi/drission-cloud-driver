package pan115

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/SheltonZhu/115driver/pkg/driver"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers/base"
)

// Driver implements the drivers.Driver interface for 115 cloud storage.
type Driver struct {
	base.Base
	clients map[string]*driver.Pan115Client // profileID -> client
	mu      sync.RWMutex
}

// NewFactory creates a new 115 driver factory.
func NewFactory() drivers.Factory {
	return func(browserMgr *browser.Manager, logger *zap.Logger) (drivers.Driver, error) {
		return &Driver{
			Base: base.Base{
				Platform_: "115",
				Capabilities_: drivers.DriverCapabilities{
					OfflineDownload: true,
					FileManage:      true,
					Search:          true,
					DirectLink:      true,
				},
				BrowserMgr: browserMgr,
				Logger:     logger,
			},
			clients: make(map[string]*driver.Pan115Client),
		}, nil
	}
}

// getClient returns a 115 client for the given profile, creating it if necessary.
func (d *Driver) getClient(ctx context.Context, profileID string) (*driver.Pan115Client, error) {
	d.mu.RLock()
	client, ok := d.clients[profileID]
	d.mu.RUnlock()

	if ok {
		return client, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Double check after acquiring write lock
	if client, ok := d.clients[profileID]; ok {
		return client, nil
	}

	// Get browser connection
	conn, err := d.BrowserMgr.GetConnection(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("get browser connection: %w", err)
	}

	// Get cookie from browser
	cookieStr, err := conn.GetCookieString(ctx, "115.com")
	if err != nil {
		return nil, fmt.Errorf("get cookie from browser: %w", err)
	}

	if cookieStr == "" {
		return nil, fmt.Errorf("no cookie found for 115.com, please login to 115 in the browser first")
	}

	// Parse cookie
	cr := &driver.Credential{}
	if err := cr.FromCookie(cookieStr); err != nil {
		return nil, fmt.Errorf("parse 115 cookie: %w", err)
	}

	// Create client
	client = driver.Default().ImportCredential(cr)

	// Verify login
	if err := client.LoginCheck(); err != nil {
		return nil, fmt.Errorf("115 login check failed: %w", err)
	}

	d.clients[profileID] = client
	d.Logger.Info("created 115 client for profile", zap.String("profile", profileID))

	return client, nil
}

// AddOfflineTask adds an offline download task.
func (d *Driver) AddOfflineTask(ctx context.Context, profileID string, req *drivers.AddTaskRequest) (*drivers.OfflineTask, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	saveDirID := "0"
	if strings.TrimSpace(req.SavePath) != "" {
		var err error
		saveDirID, err = d.resolveDirID(client, req.SavePath)
		if err != nil {
			return nil, err
		}
	}

	hashes, err := client.AddOfflineTaskURIs([]string{req.URL}, saveDirID)
	if err != nil {
		return nil, fmt.Errorf("add offline task: %w", err)
	}

	if len(hashes) == 0 {
		return nil, fmt.Errorf("no task created")
	}

	return &drivers.OfflineTask{
		TaskID:         hashes[0],
		ProviderTaskID: hashes[0],
		Status:         drivers.TaskPending,
		SavePath:       req.SavePath,
	}, nil
}

// QueryOfflineTask queries the status of an offline download task.
func (d *Driver) QueryOfflineTask(ctx context.Context, profileID string, taskID string) (*drivers.OfflineTask, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListOfflineTask(1)
	if err != nil {
		return nil, fmt.Errorf("list offline tasks: %w", err)
	}

	for _, task := range resp.Tasks {
		if task.InfoHash == taskID {
			return toOfflineTask(task), nil
		}
	}

	return nil, fmt.Errorf("task not found: %s", taskID)
}

// RemoveOfflineTask removes an offline download task.
func (d *Driver) RemoveOfflineTask(ctx context.Context, profileID string, taskID string) error {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return err
	}

	return client.DeleteOfflineTasks([]string{taskID}, false)
}

// ListOfflineTasks lists all offline download tasks.
func (d *Driver) ListOfflineTasks(ctx context.Context, profileID string) (*drivers.OfflineTaskList, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListOfflineTask(1)
	if err != nil {
		return nil, fmt.Errorf("list offline tasks: %w", err)
	}

	tasks := make([]drivers.OfflineTask, len(resp.Tasks))
	for i, task := range resp.Tasks {
		tasks[i] = *toOfflineTask(task)
	}

	total := int(resp.Total)
	if total == 0 {
		total = len(tasks)
	}

	return &drivers.OfflineTaskList{
		Items: tasks,
		Total: total,
	}, nil
}

// Mkdir creates a new directory.
func (d *Driver) Mkdir(ctx context.Context, profileID string, parentPath string, name string) error {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return err
	}

	dirID, err := d.resolveDirID(client, parentPath)
	if err != nil {
		return err
	}

	_, err = client.Mkdir(dirID, name)
	return err
}

// Remove removes a file or directory.
func (d *Driver) Remove(ctx context.Context, profileID string, path string) error {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return err
	}

	fileID, _, err := d.resolvePath(client, path)
	if err != nil {
		return err
	}
	if fileID == "0" {
		return fmt.Errorf("cannot remove root directory")
	}

	if err := client.Delete(fileID); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// Move moves a file or directory.
func (d *Driver) Move(ctx context.Context, profileID string, src string, dst string) error {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return err
	}

	fileID, _, err := d.resolvePath(client, src)
	if err != nil {
		return err
	}
	if fileID == "0" {
		return fmt.Errorf("cannot move root directory")
	}

	dstDirID, err := d.resolveDirID(client, dst)
	if err != nil {
		return err
	}

	if err := client.Move(dstDirID, fileID); err != nil {
		return fmt.Errorf("move %s to %s: %w", src, dst, err)
	}
	return nil
}

// Rename renames a file or directory.
func (d *Driver) Rename(ctx context.Context, profileID string, path string, newName string) error {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return err
	}

	fileID, _, err := d.resolvePath(client, path)
	if err != nil {
		return err
	}
	if fileID == "0" {
		return fmt.Errorf("cannot rename root directory")
	}

	if err := client.Rename(fileID, newName); err != nil {
		return fmt.Errorf("rename %s to %s: %w", path, newName, err)
	}
	return nil
}

// List lists files and directories in a directory.
func (d *Driver) List(ctx context.Context, profileID string, dirPath string) ([]drivers.FileInfo, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	dirID, err := d.resolveDirID(client, dirPath)
	if err != nil {
		return nil, err
	}

	files, err := client.List(dirID)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	result := make([]drivers.FileInfo, len(*files))
	for i, f := range *files {
		result[i] = toFileInfo(f, joinRemotePath(dirPath, f.Name))
	}

	return result, nil
}

// Search searches for files.
func (d *Driver) Search(ctx context.Context, profileID string, keyword string) ([]drivers.FileInfo, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	result, err := client.Search(&driver.SearchOption{
		SearchValue: keyword,
		Limit:       100,
	})
	if err != nil {
		return nil, fmt.Errorf("search files: %w", err)
	}

	files := make([]drivers.FileInfo, len(result.Files))
	for i, f := range result.Files {
		files[i] = drivers.FileInfo{
			ID:   f.FileID,
			Name: f.Name,
			Size: f.Size,
		}
	}

	return files, nil
}

// GetDownloadURL returns the download URL for a file.
func (d *Driver) GetDownloadURL(ctx context.Context, profileID string, path string) (string, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return "", err
	}

	file, err := d.resolveFile(client, path)
	if err != nil {
		return "", err
	}
	if file.IsDirectory {
		return "", fmt.Errorf("cannot get download URL for directory: %s", path)
	}
	if file.PickCode == "" {
		return "", fmt.Errorf("file has no pickcode: %s", path)
	}

	info, err := client.Download(file.PickCode)
	if err != nil {
		return "", fmt.Errorf("get download URL for %s: %w", path, err)
	}
	if info.Url.Url == "" {
		return "", fmt.Errorf("empty download URL for %s", path)
	}

	return info.Url.Url, nil
}

func (d *Driver) resolveDirID(client *driver.Pan115Client, remotePath string) (string, error) {
	cleaned := cleanRemotePath(remotePath)
	if cleaned == "" {
		return "0", nil
	}

	resp, err := client.DirName2CID(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve dir path %s: %w", remotePath, err)
	}
	if string(resp.CategoryID) == "0" {
		return "", fmt.Errorf("directory not found: %s", remotePath)
	}

	return string(resp.CategoryID), nil
}

func (d *Driver) resolvePath(client *driver.Pan115Client, remotePath string) (string, bool, error) {
	if remotePath == "" || remotePath == "/" {
		return "0", true, nil
	}

	if dirID, err := d.resolveDirID(client, remotePath); err == nil && dirID != "" && dirID != "0" {
		return dirID, true, nil
	}

	file, err := d.resolveFile(client, remotePath)
	if err != nil {
		return "", false, err
	}
	return file.GetID(), file.IsDirectory, nil
}

func (d *Driver) resolveFile(client *driver.Pan115Client, remotePath string) (*driver.File, error) {
	cleaned := cleanRemotePath(remotePath)
	if cleaned == "" {
		return nil, fmt.Errorf("file path is required")
	}

	parentPath := path.Dir(cleaned)
	fileName := path.Base(cleaned)
	if fileName == "." || fileName == "/" || fileName == "" {
		return nil, fmt.Errorf("file path is required")
	}

	parentID := "0"
	if parentPath != "." && parentPath != "/" {
		dirID, err := d.resolveDirID(client, parentPath)
		if err != nil {
			return nil, err
		}
		parentID = dirID
	}

	files, err := client.List(parentID)
	if err != nil {
		return nil, fmt.Errorf("list parent directory %s: %w", parentPath, err)
	}

	for _, f := range *files {
		if f.Name == fileName {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("path not found: %s", remotePath)
}

func cleanRemotePath(remotePath string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(remotePath))
	if cleaned == "/" || cleaned == "." {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func joinRemotePath(dirPath string, name string) string {
	if dirPath == "" || dirPath == "/" {
		return "/" + name
	}
	return path.Join("/", dirPath, name)
}

func toFileInfo(f driver.File, filePath string) drivers.FileInfo {
	return drivers.FileInfo{
		ID:        f.GetID(),
		Name:      f.Name,
		Path:      filePath,
		IsDir:     f.IsDirectory,
		Size:      f.Size,
		CreatedAt: f.CreateTime,
		UpdatedAt: f.UpdateTime,
		Extra: map[string]any{
			"pick_code": f.PickCode,
			"sha1":      f.Sha1,
			"thumb_url": f.ThumbURL,
		},
	}
}

func toOfflineTask(task *driver.OfflineTask) *drivers.OfflineTask {
	if task == nil {
		return &drivers.OfflineTask{Status: drivers.TaskUnknown}
	}

	result := &drivers.OfflineTask{
		TaskID:         task.InfoHash,
		ProviderTaskID: task.InfoHash,
		Status:         mapOfflineStatus(task),
		Name:           task.Name,
		Progress:       task.Percent / 100.0,
	}

	return result
}

func mapOfflineStatus(task *driver.OfflineTask) drivers.TaskStatus {
	switch {
	case task.IsTodo():
		return drivers.TaskPending
	case task.IsRunning():
		return drivers.TaskRunning
	case task.IsDone():
		return drivers.TaskCompleted
	case task.IsFailed():
		return drivers.TaskFailed
	default:
		return drivers.TaskUnknown
	}
}
