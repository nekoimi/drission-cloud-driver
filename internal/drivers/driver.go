package drivers

import "context"

// Driver is the interface that all cloud storage drivers must implement.
type Driver interface {
	// Platform returns the platform identifier (e.g., "115", "pikpak", "quark").
	Platform() string

	// Capabilities returns what this driver can do.
	Capabilities() DriverCapabilities

	// Offline download operations
	AddOfflineTask(ctx context.Context, profileID string, req *AddTaskRequest) (*TaskStatus, error)
	QueryOfflineTask(ctx context.Context, profileID string, taskID string) (*TaskStatus, error)
	RemoveOfflineTask(ctx context.Context, profileID string, taskID string) error
	ListOfflineTasks(ctx context.Context, profileID string) ([]TaskStatus, error)

	// File system operations
	Mkdir(ctx context.Context, profileID string, parentPath string, name string) error
	Remove(ctx context.Context, profileID string, path string) error
	Move(ctx context.Context, profileID string, src string, dst string) error
	Rename(ctx context.Context, profileID string, path string, newName string) error
	List(ctx context.Context, profileID string, dirPath string) ([]FileInfo, error)
	Search(ctx context.Context, profileID string, keyword string) ([]FileInfo, error)

	// Media operations
	GetDownloadURL(ctx context.Context, profileID string, path string) (string, error)
}
