package offline

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
)

type PostgresStore struct {
	db *sql.DB
}

const offlineSyncAdvisoryLockID int64 = 0x4452495353494F4E

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("postgres offline store dsn is required")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres offline store: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres offline store: %w", err)
	}

	store := &PostgresStore{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) WithSyncCycleLock(ctx context.Context, fn func() error) (bool, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("acquire postgres sync connection: %w", err)
	}
	defer conn.Close()

	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, offlineSyncAdvisoryLockID).Scan(&locked); err != nil {
		return false, fmt.Errorf("acquire offline sync advisory lock: %w", err)
	}
	if !locked {
		return false, nil
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, offlineSyncAdvisoryLockID)
	}()

	return true, fn()
}

func (s *PostgresStore) Put(task OfflineTaskRecord) error {
	task = sanitizePostgresRecord(task)
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

func (s *PostgresStore) Get(taskID string) (OfflineTaskRecord, bool) {
	row := s.db.QueryRow(`
SELECT task_id, platform, profile_id, client_task_id, url, category, save_path,
       save_dir_json, metadata_json, provider_task_id, status, name, progress, error_code,
       error_message, files_json, provider_status, task_created_at, task_updated_at,
       last_synced_at, sync_attempts, sync_error, completed_at, remote_cleaned_at,
       cleanup_attempts, cleanup_error, created_at, updated_at
FROM offline_tasks
WHERE task_id = $1
`, taskID)

	record, err := scanRecord(row)
	return record, err == nil
}

func (s *PostgresStore) GetByClientTask(platform, profileID, clientTaskID string) (OfflineTaskRecord, bool) {
	key := postgresClientKey(platform, profileID, clientTaskID)
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
WHERE client_key = $1
`, key)

	record, err := scanRecord(row)
	return record, err == nil
}

func (s *PostgresStore) List(platform, profileID string) ([]OfflineTaskRecord, error) {
	rows, err := s.db.Query(`
SELECT task_id, platform, profile_id, client_task_id, url, category, save_path,
       save_dir_json, metadata_json, provider_task_id, status, name, progress, error_code,
       error_message, files_json, provider_status, task_created_at, task_updated_at,
       last_synced_at, sync_attempts, sync_error, completed_at, remote_cleaned_at,
       cleanup_attempts, cleanup_error, created_at, updated_at
FROM offline_tasks
WHERE ($1 = '' OR platform = $1) AND ($2 = '' OR profile_id = $2)
ORDER BY created_at DESC
`, platform, profileID)
	if err != nil {
		return nil, fmt.Errorf("list postgres offline tasks: %w", err)
	}
	defer rows.Close()

	records := make([]OfflineTaskRecord, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan postgres offline task: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres offline tasks: %w", err)
	}
	return records, nil
}

func (s *PostgresStore) Update(task OfflineTaskRecord) error {
	task = sanitizePostgresRecord(task)
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

func (s *PostgresStore) migrate() error {
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
    progress DOUBLE PRECISION NOT NULL DEFAULT 0,
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
ALTER TABLE offline_tasks
    ADD COLUMN IF NOT EXISTS save_dir_json TEXT NOT NULL DEFAULT '';
ALTER TABLE offline_tasks
    ADD COLUMN IF NOT EXISTS provider_status INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_synced_at TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sync_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sync_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS completed_at TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS remote_cleaned_at TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cleanup_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cleanup_error TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_offline_tasks_client_key
    ON offline_tasks(client_key)
    WHERE client_key IS NOT NULL;
`)
	if err != nil {
		return fmt.Errorf("migrate postgres offline store: %w", err)
	}

	return nil
}

func (s *PostgresStore) save(task OfflineTaskRecord) error {
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
	if key := postgresClientKey(task.Platform, task.ProfileID, task.ClientTaskID); key != "" {
		clientKeyValue = sql.NullString{String: key, Valid: true}
	}

	_, err = s.db.Exec(`
INSERT INTO offline_tasks (
    task_id, platform, profile_id, client_task_id, client_key, url, category,
    save_path, save_dir_json, metadata_json, provider_task_id, status, name, progress,
    error_code, error_message, files_json, task_created_at, task_updated_at,
    provider_status, last_synced_at, sync_attempts, sync_error, completed_at,
    remote_cleaned_at, cleanup_attempts, cleanup_error, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
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
		return fmt.Errorf("save postgres offline task: %w", err)
	}

	return nil
}

// The in-memory store separates the client-key components with NUL bytes.
// PostgreSQL TEXT cannot store NUL, so persist a stable digest of that
// collision-safe representation instead.
func postgresClientKey(platform, profileID, clientTaskID string) string {
	key := clientKey(platform, profileID, clientTaskID)
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
}

// PostgreSQL text values cannot contain the NUL byte. Provider data and
// user-supplied metadata can occasionally contain it, so strip it at the
// persistence boundary instead of allowing an otherwise successful remote
// task submission to be lost locally.
func sanitizePostgresRecord(task OfflineTaskRecord) OfflineTaskRecord {
	clean := func(value string) string {
		return strings.ReplaceAll(value, "\x00", "")
	}

	task.Platform = clean(task.Platform)
	task.ProfileID = clean(task.ProfileID)
	task.ClientTaskID = clean(task.ClientTaskID)
	task.URL = clean(task.URL)
	task.Category = clean(task.Category)
	task.SavePath = clean(task.SavePath)
	task.Task.TaskID = clean(task.Task.TaskID)
	task.Task.ProviderTaskID = clean(task.Task.ProviderTaskID)
	task.Task.Status = drivers.TaskStatus(clean(string(task.Task.Status)))
	task.Task.Name = clean(task.Task.Name)
	task.Task.SavePath = clean(task.Task.SavePath)
	task.Task.ErrorCode = clean(task.Task.ErrorCode)
	task.Task.ErrorMessage = clean(task.Task.ErrorMessage)
	task.SyncError = clean(task.SyncError)
	task.CleanupError = clean(task.CleanupError)

	return task
}
