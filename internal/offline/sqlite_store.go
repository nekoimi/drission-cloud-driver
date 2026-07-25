package offline

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
)

const sqliteTimeFormat = time.RFC3339Nano

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("sqlite offline store dsn is required")
	}
	if err := ensureSQLiteDir(dsn); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite offline store: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) Put(task OfflineTaskRecord) error {
	if strings.TrimSpace(task.Task.TaskID) == "" {
		return ErrTaskIDRequired
	}

	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	return s.save(task)
}

func (s *SQLiteStore) Get(taskID string) (OfflineTaskRecord, bool) {
	row := s.db.QueryRow(`
SELECT task_id, platform, profile_id, client_task_id, url, category, save_path,
       save_dir_json, metadata_json, provider_task_id, status, name, progress, error_code,
       error_message, files_json, provider_status, task_created_at, task_updated_at,
       last_synced_at, sync_attempts, sync_error, completed_at, remote_cleaned_at,
       cleanup_attempts, cleanup_error, created_at, updated_at
FROM offline_tasks
WHERE task_id = ?
`, taskID)

	record, err := scanRecord(row)
	return record, err == nil
}

func (s *SQLiteStore) GetByClientTask(platform, profileID, clientTaskID string) (OfflineTaskRecord, bool) {
	key := clientKey(platform, profileID, clientTaskID)
	if key == "" {
		return OfflineTaskRecord{}, false
	}

	row := s.db.QueryRow(`
SELECT task_id, platform, profile_id, client_task_id, url, category, save_path,
       save_dir_json, metadata_json, provider_task_id, status, name, progress, error_code,
       error_message, files_json, provider_status, task_created_at, task_updated_at,
       last_synced_at, sync_attempts, sync_error, completed_at, remote_cleaned_at,
       cleanup_attempts, cleanup_error, created_at, updated_at
FROM offline_tasks
WHERE client_key = ?
`, key)

	record, err := scanRecord(row)
	return record, err == nil
}

func (s *SQLiteStore) List(platform, profileID string) ([]OfflineTaskRecord, error) {
	rows, err := s.db.Query(`
SELECT task_id, platform, profile_id, client_task_id, url, category, save_path,
       save_dir_json, metadata_json, provider_task_id, status, name, progress, error_code,
       error_message, files_json, provider_status, task_created_at, task_updated_at,
       last_synced_at, sync_attempts, sync_error, completed_at, remote_cleaned_at,
       cleanup_attempts, cleanup_error, created_at, updated_at
FROM offline_tasks
WHERE (? = '' OR platform = ?) AND (? = '' OR profile_id = ?)
ORDER BY created_at DESC
`, platform, platform, profileID, profileID)
	if err != nil {
		return nil, fmt.Errorf("list sqlite offline tasks: %w", err)
	}
	defer rows.Close()

	records := make([]OfflineTaskRecord, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sqlite offline task: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite offline tasks: %w", err)
	}
	return records, nil
}

func (s *SQLiteStore) Update(task OfflineTaskRecord) error {
	if strings.TrimSpace(task.Task.TaskID) == "" {
		return ErrTaskIDRequired
	}

	if existing, ok := s.Get(task.Task.TaskID); ok {
		task.CreatedAt = existing.CreatedAt
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	task.UpdatedAt = time.Now()

	return s.save(task)
}

func (s *SQLiteStore) migrate() error {
	if _, err := s.db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		return fmt.Errorf("set sqlite journal mode: %w", err)
	}
	if _, err := s.db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}

	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS offline_tasks (
    task_id TEXT PRIMARY KEY,
    platform TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    client_task_id TEXT NOT NULL DEFAULT '',
    client_key TEXT,
    url TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    save_path TEXT NOT NULL DEFAULT '',
    save_dir_json TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    provider_task_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    progress REAL NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    files_json TEXT NOT NULL DEFAULT '[]',
    provider_status INTEGER NOT NULL DEFAULT 0,
    task_created_at TEXT NOT NULL DEFAULT '',
    task_updated_at TEXT NOT NULL DEFAULT '',
    last_synced_at TEXT NOT NULL DEFAULT '',
    sync_attempts INTEGER NOT NULL DEFAULT 0,
    sync_error TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    remote_cleaned_at TEXT NOT NULL DEFAULT '',
    cleanup_attempts INTEGER NOT NULL DEFAULT 0,
    cleanup_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_offline_tasks_client_key
    ON offline_tasks(client_key)
    WHERE client_key IS NOT NULL;
`)
	if err != nil {
		return fmt.Errorf("migrate sqlite offline store: %w", err)
	}
	for _, migration := range []string{
		`ALTER TABLE offline_tasks ADD COLUMN save_dir_json TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE offline_tasks ADD COLUMN provider_status INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE offline_tasks ADD COLUMN last_synced_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE offline_tasks ADD COLUMN sync_attempts INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE offline_tasks ADD COLUMN sync_error TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE offline_tasks ADD COLUMN completed_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE offline_tasks ADD COLUMN remote_cleaned_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE offline_tasks ADD COLUMN cleanup_attempts INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE offline_tasks ADD COLUMN cleanup_error TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.Exec(migration); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("migrate sqlite offline store columns: %w", err)
		}
	}

	return nil
}

func (s *SQLiteStore) save(task OfflineTaskRecord) error {
	task = cloneRecord(task)

	metadataJSON, err := marshalJSON(task.Metadata, map[string]string{})
	if err != nil {
		return fmt.Errorf("marshal offline task metadata: %w", err)
	}
	saveDirJSON, err := marshalJSON(task.Task.SaveDir, (*drivers.FileInfo)(nil))
	if err != nil {
		return fmt.Errorf("marshal offline task save dir: %w", err)
	}
	filesJSON, err := marshalJSON(task.Task.Files, []drivers.FileInfo{})
	if err != nil {
		return fmt.Errorf("marshal offline task files: %w", err)
	}

	clientKeyValue := sql.NullString{}
	if key := clientKey(task.Platform, task.ProfileID, task.ClientTaskID); key != "" {
		clientKeyValue = sql.NullString{String: key, Valid: true}
	}

	_, err = s.db.Exec(`
INSERT INTO offline_tasks (
    task_id, platform, profile_id, client_task_id, client_key, url, category,
    save_path, save_dir_json, metadata_json, provider_task_id, status, name, progress,
    error_code, error_message, files_json, task_created_at, task_updated_at,
    provider_status, last_synced_at, sync_attempts, sync_error, completed_at,
    remote_cleaned_at, cleanup_attempts, cleanup_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(task_id) DO UPDATE SET
    platform = excluded.platform,
    profile_id = excluded.profile_id,
    client_task_id = excluded.client_task_id,
    client_key = excluded.client_key,
    url = excluded.url,
    category = excluded.category,
    save_path = excluded.save_path,
    save_dir_json = excluded.save_dir_json,
    metadata_json = excluded.metadata_json,
    provider_task_id = excluded.provider_task_id,
    status = excluded.status,
    name = excluded.name,
    progress = excluded.progress,
    error_code = excluded.error_code,
    error_message = excluded.error_message,
    files_json = excluded.files_json,
    provider_status = excluded.provider_status,
    task_created_at = excluded.task_created_at,
    task_updated_at = excluded.task_updated_at,
    last_synced_at = excluded.last_synced_at,
    sync_attempts = excluded.sync_attempts,
    sync_error = excluded.sync_error,
    completed_at = excluded.completed_at,
    remote_cleaned_at = excluded.remote_cleaned_at,
    cleanup_attempts = excluded.cleanup_attempts,
    cleanup_error = excluded.cleanup_error,
    updated_at = excluded.updated_at
`,
		task.Task.TaskID,
		task.Platform,
		task.ProfileID,
		task.ClientTaskID,
		clientKeyValue,
		task.URL,
		task.Category,
		task.SavePath,
		string(saveDirJSON),
		string(metadataJSON),
		task.Task.ProviderTaskID,
		string(task.Task.Status),
		task.Task.Name,
		task.Task.Progress,
		task.Task.ErrorCode,
		task.Task.ErrorMessage,
		string(filesJSON),
		formatTime(task.Task.CreatedAt),
		formatTime(task.Task.UpdatedAt),
		task.Task.ProviderStatus,
		formatTime(task.LastSyncedAt),
		task.SyncAttempts,
		task.SyncError,
		formatTime(task.CompletedAt),
		formatTime(task.RemoteCleanedAt),
		task.CleanupAttempts,
		task.CleanupError,
		formatTime(task.CreatedAt),
		formatTime(task.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("save sqlite offline task: %w", err)
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row rowScanner) (OfflineTaskRecord, error) {
	var record OfflineTaskRecord
	var saveDirJSON string
	var metadataJSON string
	var filesJSON string
	var status string
	var taskCreatedAt string
	var taskUpdatedAt string
	var lastSyncedAt string
	var completedAt string
	var remoteCleanedAt string
	var createdAt string
	var updatedAt string

	err := row.Scan(
		&record.Task.TaskID,
		&record.Platform,
		&record.ProfileID,
		&record.ClientTaskID,
		&record.URL,
		&record.Category,
		&record.SavePath,
		&saveDirJSON,
		&metadataJSON,
		&record.Task.ProviderTaskID,
		&status,
		&record.Task.Name,
		&record.Task.Progress,
		&record.Task.ErrorCode,
		&record.Task.ErrorMessage,
		&filesJSON,
		&record.Task.ProviderStatus,
		&taskCreatedAt,
		&taskUpdatedAt,
		&lastSyncedAt,
		&record.SyncAttempts,
		&record.SyncError,
		&completedAt,
		&remoteCleanedAt,
		&record.CleanupAttempts,
		&record.CleanupError,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return OfflineTaskRecord{}, sql.ErrNoRows
	}
	if err != nil {
		return OfflineTaskRecord{}, err
	}

	record.Task.Status = drivers.TaskStatus(status)
	record.Task.SavePath = record.SavePath
	record.Task.CreatedAt = parseTime(taskCreatedAt)
	record.Task.UpdatedAt = parseTime(taskUpdatedAt)
	record.LastSyncedAt = parseTime(lastSyncedAt)
	record.CompletedAt = parseTime(completedAt)
	record.RemoteCleanedAt = parseTime(remoteCleanedAt)
	record.CreatedAt = parseTime(createdAt)
	record.UpdatedAt = parseTime(updatedAt)

	if metadataJSON != "" {
		_ = json.Unmarshal([]byte(metadataJSON), &record.Metadata)
	}
	if saveDirJSON != "" && saveDirJSON != "null" {
		_ = json.Unmarshal([]byte(saveDirJSON), &record.Task.SaveDir)
	}
	if filesJSON != "" {
		_ = json.Unmarshal([]byte(filesJSON), &record.Task.Files)
	}

	return cloneRecord(record), nil
}

func ensureSQLiteDir(dsn string) error {
	if dsn == ":memory:" || strings.HasPrefix(dsn, "file:") {
		return nil
	}

	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite offline store directory: %w", err)
	}
	return nil
}

func marshalJSON[T any](value T, fallback T) ([]byte, error) {
	out, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if string(out) == "null" {
		return json.Marshal(fallback)
	}
	return out, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(sqliteTimeFormat)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(sqliteTimeFormat, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
