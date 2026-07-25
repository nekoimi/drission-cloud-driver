package drivers

import (
	"encoding/json"
	"time"
)

// FileInfo represents a file or directory in cloud storage.
type FileInfo struct {
	ID           string         `json:"id"`
	FileID       string         `json:"file_id,omitempty"`
	ParentID     string         `json:"parent_id,omitempty"`
	Name         string         `json:"name"`
	Path         string         `json:"path"`
	RelativePath string         `json:"relative_path,omitempty"`
	IsDir        bool           `json:"is_dir"`
	Size         int64          `json:"size"`
	MimeType     string         `json:"mime_type,omitempty"`
	CreatedAt    time.Time      `json:"created_at,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
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
	ProviderStatus int        `json:"-"`
	Status         TaskStatus `json:"status"`
	Name           string     `json:"name,omitempty"`
	Progress       float64    `json:"progress,omitempty"`
	SavePath       string     `json:"save_path,omitempty"`
	SaveDir        *FileInfo  `json:"save_dir,omitempty"`
	ErrorCode      string     `json:"error_code,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	Files          []FileInfo `json:"files"`
	Warnings       []string   `json:"warnings,omitempty"`
	CreatedAt      time.Time  `json:"created_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
}

func (t OfflineTask) MarshalJSON() ([]byte, error) {
	type alias OfflineTask
	if t.Files == nil {
		t.Files = []FileInfo{}
	}
	return json.Marshal(alias(t))
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

// DirName2CIDResult is the raw-ish result returned by a provider path-to-dir-id
// lookup endpoint.
type DirName2CIDResult struct {
	Path       string `json:"path"`
	Cleaned    string `json:"cleaned"`
	CategoryID string `json:"category_id"`
	IsPrivate  string `json:"is_private,omitempty"`
}
