package pan115

import (
	"context"
	"fmt"
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
func (d *Driver) AddOfflineTask(ctx context.Context, profileID string, req *drivers.AddTaskRequest) (*drivers.TaskStatus, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	hashes, err := client.AddOfflineTaskURIs([]string{req.URL}, "0")
	if err != nil {
		return nil, fmt.Errorf("add offline task: %w", err)
	}

	if len(hashes) == 0 {
		return nil, fmt.Errorf("no task created")
	}

	return &drivers.TaskStatus{
		TaskID: hashes[0],
		Status: "pending",
	}, nil
}

// QueryOfflineTask queries the status of an offline download task.
func (d *Driver) QueryOfflineTask(ctx context.Context, profileID string, taskID string) (*drivers.TaskStatus, error) {
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
			return &drivers.TaskStatus{
				TaskID:   task.InfoHash,
				Status:   mapOfflineStatus(task),
				Progress: task.Percent / 100.0,
				FileName: task.Name,
				FileSize: task.Size,
			}, nil
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
func (d *Driver) ListOfflineTasks(ctx context.Context, profileID string) ([]drivers.TaskStatus, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListOfflineTask(1)
	if err != nil {
		return nil, fmt.Errorf("list offline tasks: %w", err)
	}

	tasks := make([]drivers.TaskStatus, len(resp.Tasks))
	for i, task := range resp.Tasks {
		tasks[i] = drivers.TaskStatus{
			TaskID:   task.InfoHash,
			Status:   mapOfflineStatus(task),
			Progress: task.Percent / 100.0,
			FileName: task.Name,
			FileSize: task.Size,
		}
	}

	return tasks, nil
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
	// TODO: implement remove
	return fmt.Errorf("not implemented")
}

// Move moves a file or directory.
func (d *Driver) Move(ctx context.Context, profileID string, src string, dst string) error {
	// TODO: implement move
	return fmt.Errorf("not implemented")
}

// Rename renames a file or directory.
func (d *Driver) Rename(ctx context.Context, profileID string, path string, newName string) error {
	// TODO: implement rename
	return fmt.Errorf("not implemented")
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
		result[i] = drivers.FileInfo{
			ID:        f.GetID(),
			Name:      f.Name,
			Path:      dirPath + "/" + f.Name,
			IsDir:     f.IsDirectory,
			Size:      f.Size,
			CreatedAt: f.CreateTime,
			UpdatedAt: f.UpdateTime,
		}
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
	// TODO: implement get download URL
	return "", fmt.Errorf("not implemented")
}

func (d *Driver) resolveDirID(client *driver.Pan115Client, path string) (string, error) {
	if path == "" || path == "/" {
		return "0", nil
	}

	resp, err := client.DirName2CID(path)
	if err != nil {
		return "", fmt.Errorf("resolve dir path %s: %w", path, err)
	}

	return string(resp.CategoryID), nil
}

func mapOfflineStatus(task *driver.OfflineTask) string {
	switch {
	case task.IsTodo():
		return "pending"
	case task.IsRunning():
		return "downloading"
	case task.IsDone():
		return "completed"
	case task.IsFailed():
		return "failed"
	default:
		return "unknown"
	}
}
