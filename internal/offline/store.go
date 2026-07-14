package offline

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
)

var ErrTaskIDRequired = errors.New("offline task id is required")

// OfflineTaskRecord stores the local metadata needed to make offline task
// submission idempotent inside the current process.
type OfflineTaskRecord struct {
	Platform     string
	ProfileID    string
	ClientTaskID string
	URL          string
	Category     string
	SavePath     string
	Metadata     map[string]string
	Task         drivers.OfflineTask
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Store keeps offline task records.
type Store interface {
	Put(task OfflineTaskRecord) error
	Get(taskID string) (OfflineTaskRecord, bool)
	GetByClientTask(platform, profileID, clientTaskID string) (OfflineTaskRecord, bool)
	Update(task OfflineTaskRecord) error
}

// MemoryStore is a process-local offline task repository.
type MemoryStore struct {
	mu       sync.RWMutex
	byTask   map[string]OfflineTaskRecord
	byClient map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byTask:   make(map[string]OfflineTaskRecord),
		byClient: make(map[string]string),
	}
}

func (s *MemoryStore) Put(task OfflineTaskRecord) error {
	if strings.TrimSpace(task.Task.TaskID) == "" {
		return ErrTaskIDRequired
	}

	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()

	s.removeIndexesLocked(task.Task.TaskID)
	s.putLocked(task)
	return nil
}

func (s *MemoryStore) Get(taskID string) (OfflineTaskRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.byTask[taskID]
	return cloneRecord(task), ok
}

func (s *MemoryStore) GetByClientTask(platform, profileID, clientTaskID string) (OfflineTaskRecord, bool) {
	key := clientKey(platform, profileID, clientTaskID)
	if key == "" {
		return OfflineTaskRecord{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	taskID, ok := s.byClient[key]
	if !ok {
		return OfflineTaskRecord{}, false
	}

	task, ok := s.byTask[taskID]
	return cloneRecord(task), ok
}

func (s *MemoryStore) Update(task OfflineTaskRecord) error {
	if strings.TrimSpace(task.Task.TaskID) == "" {
		return ErrTaskIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.byTask[task.Task.TaskID]; ok {
		task.CreatedAt = existing.CreatedAt
		s.removeIndexesLocked(task.Task.TaskID)
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	task.UpdatedAt = time.Now()

	s.putLocked(task)
	return nil
}

func (s *MemoryStore) putLocked(task OfflineTaskRecord) {
	task = cloneRecord(task)
	s.byTask[task.Task.TaskID] = task

	key := clientKey(task.Platform, task.ProfileID, task.ClientTaskID)
	if key != "" {
		s.byClient[key] = task.Task.TaskID
	}
}

func (s *MemoryStore) removeIndexesLocked(taskID string) {
	for key, indexedTaskID := range s.byClient {
		if indexedTaskID == taskID {
			delete(s.byClient, key)
		}
	}
}

func clientKey(platform, profileID, clientTaskID string) string {
	platform = strings.TrimSpace(platform)
	profileID = strings.TrimSpace(profileID)
	clientTaskID = strings.TrimSpace(clientTaskID)
	if platform == "" || profileID == "" || clientTaskID == "" {
		return ""
	}

	return platform + "\x00" + profileID + "\x00" + clientTaskID
}

func cloneRecord(task OfflineTaskRecord) OfflineTaskRecord {
	if task.Metadata != nil {
		metadata := make(map[string]string, len(task.Metadata))
		for key, value := range task.Metadata {
			metadata[key] = value
		}
		task.Metadata = metadata
	}
	if task.Task.SaveDir != nil {
		saveDir := cloneFileInfo(*task.Task.SaveDir)
		task.Task.SaveDir = &saveDir
	}
	if task.Task.Files != nil {
		files := make([]drivers.FileInfo, len(task.Task.Files))
		for i := range task.Task.Files {
			files[i] = cloneFileInfo(task.Task.Files[i])
		}
		task.Task.Files = files
	}
	return task
}

func cloneFileInfo(file drivers.FileInfo) drivers.FileInfo {
	if file.Extra != nil {
		extra := make(map[string]any, len(file.Extra))
		for key, value := range file.Extra {
			extra[key] = value
		}
		file.Extra = extra
	}
	return file
}
