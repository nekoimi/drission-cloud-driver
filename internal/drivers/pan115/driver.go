package pan115

import (
	"context"
	"fmt"

	"github.com/SheltonZhu/115driver/pkg/driver"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers/base"
)

// Driver implements the drivers.Driver interface for 115 cloud storage.
type Driver struct {
	base.Base
	client *driver.Pan115Client
}

// NewFactory creates a new 115 driver factory.
func NewFactory(cookie string) drivers.Factory {
	return func(browserMgr *browser.Manager, logger *zap.Logger) (drivers.Driver, error) {
		cr := &driver.Credential{}
		if err := cr.FromCookie(cookie); err != nil {
			return nil, fmt.Errorf("parse 115 cookie: %w", err)
		}

		client := driver.Default().ImportCredential(cr)

		// Verify login
		if err := client.LoginCheck(); err != nil {
			return nil, fmt.Errorf("115 login check failed: %w", err)
		}

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
			client: client,
		}, nil
	}
}

// AddOfflineTask adds an offline download task.
func (d *Driver) AddOfflineTask(ctx context.Context, profileID string, req *drivers.AddTaskRequest) (*drivers.TaskStatus, error) {
	hashes, err := d.client.AddOfflineTaskURIs([]string{req.URL}, "0")
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
	resp, err := d.client.ListOfflineTask(1)
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
	return d.client.DeleteOfflineTasks([]string{taskID}, false)
}

// ListOfflineTasks lists all offline download tasks.
func (d *Driver) ListOfflineTasks(ctx context.Context, profileID string) ([]drivers.TaskStatus, error) {
	resp, err := d.client.ListOfflineTask(1)
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
	dirID, err := d.resolveDirID(parentPath)
	if err != nil {
		return err
	}

	_, err = d.client.Mkdir(dirID, name)
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
	dirID, err := d.resolveDirID(dirPath)
	if err != nil {
		return nil, err
	}

	files, err := d.client.List(dirID)
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
	result, err := d.client.Search(&driver.SearchOption{
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

func (d *Driver) resolveDirID(path string) (string, error) {
	if path == "" || path == "/" {
		return "0", nil
	}

	resp, err := d.client.DirName2CID(path)
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
