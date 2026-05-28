package drivers

import "time"

// FileInfo represents a file or directory in cloud storage.
type FileInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IsDir     bool      `json:"is_dir"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mime_type,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// TaskStatus represents the status of an offline download task.
type TaskStatus struct {
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`    // pending, downloading, completed, failed
	Progress  float64   `json:"progress"`  // 0.0 ~ 1.0
	FileSize  int64     `json:"file_size,omitempty"`
	FileName  string    `json:"file_name,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AddTaskRequest is the request to add an offline download task.
type AddTaskRequest struct {
	URL      string `json:"url" binding:"required"`
	SavePath string `json:"save_path"`
}

// DriverCapabilities describes what a driver can do.
type DriverCapabilities struct {
	OfflineDownload bool `json:"offline_download"`
	FileManage      bool `json:"file_manage"`
	Search          bool `json:"search"`
	DirectLink      bool `json:"direct_link"`
	MediaInfo       bool `json:"media_info"`
}
