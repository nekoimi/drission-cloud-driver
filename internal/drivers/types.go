package drivers

import "time"

// FileInfo represents a file or directory in cloud storage.
type FileInfo struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	IsDir     bool           `json:"is_dir"`
	Size      int64          `json:"size"`
	MimeType  string         `json:"mime_type,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// AddTaskRequest is the request to add an offline download task.
type AddTaskRequest struct {
	URL          string            `json:"url" binding:"required"`
	Category     string            `json:"category,omitempty"`
	SavePath     string            `json:"save_path,omitempty"`
	ClientTaskID string            `json:"client_task_id,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// TaskStatus is the normalized status of an offline download task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCanceled  TaskStatus = "canceled"
	TaskUnknown   TaskStatus = "unknown"
)

// OfflineTask represents a normalized offline download task.
type OfflineTask struct {
	TaskID         string     `json:"task_id"`
	ProviderTaskID string     `json:"provider_task_id,omitempty"`
	Status         TaskStatus `json:"status"`
	Name           string     `json:"name,omitempty"`
	Progress       float64    `json:"progress,omitempty"`
	SavePath       string     `json:"save_path,omitempty"`
	ErrorCode      string     `json:"error_code,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	Files          []FileInfo `json:"files,omitempty"`
	CreatedAt      time.Time  `json:"created_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
}

// OfflineTaskList represents a paged list of offline download tasks.
type OfflineTaskList struct {
	Items []OfflineTask `json:"items"`
	Total int           `json:"total"`
}

// DriverCapabilities describes what a driver can do.
type DriverCapabilities struct {
	OfflineDownload bool `json:"offline_download"`
	FileManage      bool `json:"file_manage"`
	Search          bool `json:"search"`
	DirectLink      bool `json:"direct_link"`
	MediaInfo       bool `json:"media_info"`
}
