package pan115

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SheltonZhu/115driver/pkg/driver"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers/base"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
)

const platform115 = "115"
const maxOfflineTaskExpandDepth = 20
const dirIDCacheTimeFormat = time.RFC3339Nano

// Driver implements the drivers.Driver interface for 115 cloud storage.
type Driver struct {
	base.Base
	clients            map[string]*driver.Pan115Client // profileID -> client
	dirIDCache         map[string]string               // profileID + path -> dirID
	dirIDCacheDB       *sql.DB
	dirIDCacheDBDriver string
	mu                 sync.RWMutex
}

// NewFactory creates a new 115 driver factory.
func NewFactory() drivers.Factory {
	return NewFactoryWithDirIDCacheDSN("")
}

// NewFactoryWithDirIDCacheDSN creates a 115 driver factory with a persistent
// directory-id cache.
func NewFactoryWithDirIDCacheDSN(cacheDSN string) drivers.Factory {
	return func(browserMgr *browser.Manager, logger *zap.Logger) (drivers.Driver, error) {
		d := &Driver{
			Base: base.Base{
				Platform_: platform115,
				Capabilities_: drivers.DriverCapabilities{
					OfflineDownload: true,
					FileManage:      true,
					Search:          true,
					DirectLink:      true,
				},
				BrowserMgr: browserMgr,
				Logger:     logger,
			},
			clients:    make(map[string]*driver.Pan115Client),
			dirIDCache: make(map[string]string),
		}
		if err := d.openDirIDCache(cacheDSN); err != nil {
			return nil, err
		}
		return d, nil
	}
}

// Close releases resources held by the driver.
func (d *Driver) Close() error {
	if d == nil || d.dirIDCacheDB == nil {
		return nil
	}
	return d.dirIDCacheDB.Close()
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

	// Read cookies through a short-lived browser session. If no cookies are
	// found, the browser manager opens 115 once and retries.
	cookieStr, err := d.BrowserMgr.GetCookieStringFor(ctx, profileID, browser.CookieRequest{
		Domain:  "115.com",
		WakeURL: "https://115.com/",
	})
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrCDPConnection, fmt.Errorf("get cookie from browser: %w", err))
	}

	if cookieStr == "" {
		return nil, errcode.NewWithDetail(errcode.ErrProfileNotLoggedIn, "no cookie found for 115.com, please login to 115 in the browser first")
	}

	// Parse cookie
	cr := &driver.Credential{}
	if err := cr.FromCookie(cookieStr); err != nil {
		return nil, errcode.Wrap(errcode.ErrProfileNotLoggedIn, fmt.Errorf("parse 115 cookie: %w", err))
	}

	// Create client
	client = driver.New(driver.UA(driver.UA115Browser), driver.WithDebug(), driver.WithTrace()).ImportCredential(cr)

	// Verify login
	if err := client.LoginCheck(); err != nil {
		return nil, errcode.Wrap(errcode.ErrProfileNotLoggedIn, fmt.Errorf("115 login check failed: %w", err))
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
		saveDirID, err = d.ensureDir(ctx, profileID, client, req.SavePath)
		if err != nil {
			return nil, err
		}
	}
	saveDir := d.dirInfoFromPath(saveDirID, req.SavePath)

	hashes, err := client.AddOfflineTaskURIs([]string{req.URL}, saveDirID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, fmt.Errorf("add offline task: %w", err))
	}

	if len(hashes) == 0 {
		return nil, errcode.NewWithDetail(errcode.ErrPlatformState, "no task created")
	}

	return &drivers.OfflineTask{
		TaskID:         drivers.BuildTaskID(platform115, hashes[0]),
		ProviderTaskID: hashes[0],
		Status:         drivers.TaskPending,
		SavePath:       req.SavePath,
		SaveDir:        saveDir,
	}, nil
}

// QueryOfflineTask queries the status of an offline download task.
func (d *Driver) QueryOfflineTask(ctx context.Context, profileID string, taskID string) (*drivers.OfflineTask, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}
	providerTaskID := drivers.ProviderTaskID(platform115, taskID)

	for page := int64(1); ; page++ {
		resp, err := client.ListOfflineTask(page)
		if err != nil {
			return nil, errcode.Wrap(errcode.ErrOperationFailed, fmt.Errorf("list offline tasks page %d: %w", page, err))
		}

		for _, task := range resp.Tasks {
			if task != nil && strings.EqualFold(task.InfoHash, providerTaskID) {
				d.Logger.Info("refreshed 115 offline task",
					zap.String("task_id", taskID),
					zap.Int("provider_status", task.Status),
					zap.Float64("provider_percent", task.Percent),
					zap.String("provider_file_id", task.FileId),
					zap.Int64("page", page),
				)
				return d.toOfflineTask(client, task), nil
			}
		}

		if page >= resp.PageCount || resp.PageCount <= 0 {
			break
		}
	}

	return nil, errcode.NewWithDetail(errcode.ErrTaskNotFound, fmt.Sprintf("task not found: %s", taskID))
}

// RemoveOfflineTask removes an offline download task.
func (d *Driver) RemoveOfflineTask(ctx context.Context, profileID string, taskID string) error {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return err
	}

	providerTaskID := drivers.ProviderTaskID(platform115, taskID)
	if err := client.DeleteOfflineTasks([]string{providerTaskID}, false); err != nil {
		return errcode.Wrap(errcode.ErrOperationFailed, fmt.Errorf("delete offline task %s: %w", taskID, err))
	}
	return nil
}

// ListOfflineTasks lists all offline download tasks.
func (d *Driver) ListOfflineTasks(ctx context.Context, profileID string) (*drivers.OfflineTaskList, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	tasks := make([]drivers.OfflineTask, 0)
	total := 0
	for page := int64(1); ; page++ {
		resp, err := client.ListOfflineTask(page)
		if err != nil {
			return nil, errcode.Wrap(errcode.ErrOperationFailed, fmt.Errorf("list offline tasks page %d: %w", page, err))
		}
		if resp.Total > 0 {
			total = int(resp.Total)
		}
		for _, task := range resp.Tasks {
			tasks = append(tasks, *d.toOfflineTask(client, task))
		}
		if page >= resp.PageCount || resp.PageCount <= 0 {
			break
		}
	}
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

	_, err = d.ensureDir(ctx, profileID, client, joinRemotePath(parentPath, name))
	return err
}

// Remove removes a file or directory.
func (d *Driver) Remove(ctx context.Context, profileID string, path string) error {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return err
	}

	fileID, _, err := d.resolvePath(profileID, client, path)
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

	fileID, _, err := d.resolvePath(profileID, client, src)
	if err != nil {
		return err
	}
	if fileID == "0" {
		return fmt.Errorf("cannot move root directory")
	}

	dstDirID, err := d.resolveDirID(profileID, client, dst)
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

	fileID, _, err := d.resolvePath(profileID, client, path)
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

	dirID, err := d.resolveDirID(profileID, client, dirPath)
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
			ID:     f.FileID,
			FileID: f.FileID,
			Name:   f.Name,
			Size:   f.Size,
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

	file, err := d.resolveFile(profileID, client, path)
	if err != nil {
		return "", err
	}
	if file.IsDirectory {
		return "", fmt.Errorf("cannot get download URL for directory: %s", path)
	}
	if file.PickCode == "" {
		return "", fmt.Errorf("file has no pickcode: %s", path)
	}

	return d.getDownloadURLForFile(client, file, path)
}

// GetDownloadURLByID returns the download URL for a file ID.
func (d *Driver) GetDownloadURLByID(ctx context.Context, profileID string, fileID string) (string, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return "", err
	}

	file, err := client.GetFile(fileID)
	if err != nil {
		return "", fmt.Errorf("get file %s: %w", fileID, err)
	}
	if file == nil || file.GetID() == "" {
		return "", fmt.Errorf("file not found: %s", fileID)
	}
	if file.IsDirectory {
		return "", fmt.Errorf("cannot get download URL for directory: %s", fileID)
	}
	if file.PickCode == "" {
		return "", fmt.Errorf("file has no pickcode: %s", fileID)
	}

	return d.getDownloadURLForFile(client, file, fileID)
}

func (d *Driver) getDownloadURLForFile(client *driver.Pan115Client, file *driver.File, label string) (string, error) {
	info, err := client.Download(file.PickCode)
	if err != nil {
		return "", fmt.Errorf("get download URL for %s: %w", label, err)
	}
	if info.Url.Url == "" {
		return "", fmt.Errorf("empty download URL for %s", label)
	}

	return info.Url.Url, nil
}

func (d *Driver) resolveDirID(profileID string, client *driver.Pan115Client, remotePath string) (string, error) {
	cleaned := cleanRemotePath(remotePath)
	if cleaned == "" {
		return "0", nil
	}

	if dirID, ok := d.getCachedDirID(profileID, cleaned); ok {
		return dirID, nil
	}

	resp, err := client.DirName2CID(cleaned)
	if err != nil {
		if dirID, fallbackErr := d.resolveDirIDByList(client, cleaned); fallbackErr == nil {
			d.setCachedDirID(profileID, cleaned, dirID)
			return dirID, nil
		}
		return "", fmt.Errorf("resolve dir path %s: %w", remotePath, err)
	}
	if string(resp.CategoryID) == "0" {
		if dirID, fallbackErr := d.resolveDirIDByList(client, cleaned); fallbackErr == nil {
			d.setCachedDirID(profileID, cleaned, dirID)
			return dirID, nil
		}
		return "", fmt.Errorf("directory not found: %s", remotePath)
	}

	dirID := string(resp.CategoryID)
	d.setCachedDirID(profileID, cleaned, dirID)
	return dirID, nil
}

func (d *Driver) resolveDirIDByList(client *driver.Pan115Client, remotePath string) (string, error) {
	cleaned := cleanRemotePath(remotePath)
	if cleaned == "" {
		return "0", nil
	}

	parentID := "0"
	parts := strings.Split(cleaned, "/")
	for i, part := range parts {
		files, err := client.List(parentID)
		if err != nil {
			currentPath := strings.Join(parts[:i+1], "/")
			return "", fmt.Errorf("list parent directory for %s: %w", currentPath, err)
		}

		found := false
		for _, f := range *files {
			if f.IsDirectory && f.Name == part {
				parentID = f.GetID()
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("directory not found: %s", remotePath)
		}
	}

	return parentID, nil
}

func (d *Driver) openDirIDCache(dsn string) error {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil
	}

	driverName := dirIDCacheDriverName(dsn)
	if driverName == "sqlite" {
		if err := ensureDirIDCacheSQLiteDir(dsn); err != nil {
			return err
		}
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("open pan115 dir id cache: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping pan115 dir id cache: %w", err)
	}

	if driverName == "sqlite" {
		if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
			_ = db.Close()
			return fmt.Errorf("set pan115 dir id cache journal mode: %w", err)
		}
		if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
			_ = db.Close()
			return fmt.Errorf("set pan115 dir id cache busy timeout: %w", err)
		}
	}

	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS pan115_dir_id_cache (
    profile_id TEXT NOT NULL,
    path TEXT NOT NULL,
    dir_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (profile_id, path)
);
`); err != nil {
		_ = db.Close()
		return fmt.Errorf("migrate pan115 dir id cache: %w", err)
	}

	d.dirIDCacheDB = db
	d.dirIDCacheDBDriver = driverName
	return nil
}

func (d *Driver) getCachedDirID(profileID string, remotePath string) (string, bool) {
	key := dirIDCacheKey(profileID, remotePath)
	d.mu.RLock()
	dirID, ok := d.dirIDCache[key]
	d.mu.RUnlock()
	if ok {
		return dirID, true
	}

	cleaned := cleanRemotePath(remotePath)
	if d.dirIDCacheDB == nil || strings.TrimSpace(profileID) == "" || cleaned == "" {
		return "", false
	}

	row := d.dirIDCacheDB.QueryRow(`
SELECT dir_id
FROM pan115_dir_id_cache
WHERE profile_id = `+d.dirIDCachePlaceholder(1)+` AND path = `+d.dirIDCachePlaceholder(2)+`
`, profileID, cleaned)
	if err := row.Scan(&dirID); err != nil {
		return "", false
	}

	d.setCachedDirIDMemory(profileID, cleaned, dirID)
	return dirID, true
}

func (d *Driver) setCachedDirID(profileID string, remotePath string, dirID string) {
	cleaned := cleanRemotePath(remotePath)
	if strings.TrimSpace(profileID) == "" || cleaned == "" || dirID == "" {
		return
	}

	d.setCachedDirIDMemory(profileID, cleaned, dirID)

	if d.dirIDCacheDB == nil {
		return
	}

	now := time.Now().Format(dirIDCacheTimeFormat)
	query := `
INSERT INTO pan115_dir_id_cache (
    profile_id, path, dir_id, created_at, updated_at
) VALUES (` + d.dirIDCachePlaceholder(1) + `, ` + d.dirIDCachePlaceholder(2) + `, ` + d.dirIDCachePlaceholder(3) + `, ` + d.dirIDCachePlaceholder(4) + `, ` + d.dirIDCachePlaceholder(5) + `)
ON CONFLICT(profile_id, path) DO UPDATE SET
    dir_id = excluded.dir_id,
    updated_at = excluded.updated_at
`
	_, err := d.dirIDCacheDB.Exec(query, profileID, cleaned, dirID, now, now)
	if err != nil && d.Logger != nil {
		d.Logger.Warn("save pan115 dir id cache failed",
			zap.String("profile", profileID),
			zap.String("path", cleaned),
			zap.String("dir_id", dirID),
			zap.Error(err),
		)
	}
}

func (d *Driver) setCachedDirIDMemory(profileID string, remotePath string, dirID string) {
	cleaned := cleanRemotePath(remotePath)
	if strings.TrimSpace(profileID) == "" || cleaned == "" || dirID == "" {
		return
	}

	d.mu.Lock()
	if d.dirIDCache == nil {
		d.dirIDCache = make(map[string]string)
	}
	d.dirIDCache[dirIDCacheKey(profileID, cleaned)] = dirID
	d.mu.Unlock()
}

func dirIDCacheKey(profileID string, remotePath string) string {
	return profileID + "\x00" + cleanRemotePath(remotePath)
}

func (d *Driver) dirIDCachePlaceholder(n int) string {
	if d.dirIDCacheDBDriver == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func dirIDCacheDriverName(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if strings.HasPrefix(dsn, "postgres://") ||
		strings.HasPrefix(dsn, "postgresql://") ||
		strings.Contains(dsn, "host=") {
		return "postgres"
	}
	return "sqlite"
}

func ensureDirIDCacheSQLiteDir(dsn string) error {
	if dsn == ":memory:" || strings.HasPrefix(dsn, "file:") {
		return nil
	}

	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create pan115 dir id cache directory: %w", err)
	}
	return nil
}

// DirName2CID directly calls 115's files/getid endpoint for debugging.
func (d *Driver) DirName2CID(ctx context.Context, profileID string, remotePath string) (*drivers.DirName2CIDResult, error) {
	client, err := d.getClient(ctx, profileID)
	if err != nil {
		return nil, err
	}

	cleaned := cleanRemotePath(remotePath)
	resp, err := client.DirName2CID(cleaned)
	if err != nil {
		return nil, fmt.Errorf("dirname2cid %s: %w", cleaned, err)
	}

	return &drivers.DirName2CIDResult{
		Path:       remotePath,
		Cleaned:    cleaned,
		CategoryID: string(resp.CategoryID),
		IsPrivate:  string(resp.IsPrivate),
	}, nil
}

func (d *Driver) ensureDir(ctx context.Context, profileID string, client *driver.Pan115Client, remotePath string) (string, error) {
	_ = ctx

	cleaned := cleanRemotePath(remotePath)
	if cleaned == "" {
		return "0", nil
	}

	if dirID, err := d.resolveDirID(profileID, client, cleaned); err == nil {
		return dirID, nil
	}

	parentID := "0"
	parts := strings.Split(cleaned, "/")
	for i, part := range parts {
		currentPath := strings.Join(parts[:i+1], "/")
		if dirID, err := d.resolveDirID(profileID, client, currentPath); err == nil {
			parentID = dirID
			continue
		}

		dirID, err := client.Mkdir(parentID, part)
		if err != nil {
			if isTargetAlreadyExists(err) {
				if dirID, resolveErr := d.resolveDirID(profileID, client, currentPath); resolveErr == nil {
					parentID = dirID
					continue
				}
			}
			return "", fmt.Errorf("create dir %s: %w", currentPath, err)
		}
		d.setCachedDirID(profileID, currentPath, dirID)
		parentID = dirID
	}

	return parentID, nil
}

func isTargetAlreadyExists(err error) bool {
	return errors.Is(err, driver.ErrExist)
}

func (d *Driver) resolvePath(profileID string, client *driver.Pan115Client, remotePath string) (string, bool, error) {
	if remotePath == "" || remotePath == "/" {
		return "0", true, nil
	}

	if dirID, err := d.resolveDirID(profileID, client, remotePath); err == nil && dirID != "" && dirID != "0" {
		return dirID, true, nil
	}

	file, err := d.resolveFile(profileID, client, remotePath)
	if err != nil {
		return "", false, err
	}
	return file.GetID(), file.IsDirectory, nil
}

func (d *Driver) resolveFile(profileID string, client *driver.Pan115Client, remotePath string) (*driver.File, error) {
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
		dirID, err := d.resolveDirID(profileID, client, parentPath)
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
		ID:           f.GetID(),
		FileID:       f.GetID(),
		ParentID:     f.ParentID,
		Name:         f.Name,
		Path:         filePath,
		RelativePath: cleanRemotePath(filePath),
		IsDir:        f.IsDirectory,
		Size:         f.Size,
		CreatedAt:    f.CreateTime,
		UpdatedAt:    f.UpdateTime,
		Extra: map[string]any{
			"pick_code": f.PickCode,
			"sha1":      f.Sha1,
			"thumb_url": f.ThumbURL,
		},
	}
}

func (d *Driver) toOfflineTask(client *driver.Pan115Client, task *driver.OfflineTask) *drivers.OfflineTask {
	result := toOfflineTask(task)
	if task == nil {
		return result
	}

	if task.DirId != "" {
		result.SaveDir = d.dirInfoByID(client, task.DirId, "")
	}
	if result.Status != drivers.TaskCompleted {
		return result
	}

	result.Files = d.locateOfflineTaskFiles(client, task)
	return result
}

func toOfflineTask(task *driver.OfflineTask) *drivers.OfflineTask {
	if task == nil {
		return &drivers.OfflineTask{Status: drivers.TaskUnknown}
	}

	result := &drivers.OfflineTask{
		TaskID:         drivers.BuildTaskID(platform115, task.InfoHash),
		ProviderTaskID: task.InfoHash,
		ProviderStatus: task.Status,
		Status:         mapOfflineStatus(task),
		Name:           task.Name,
		Progress:       task.Percent / 100.0,
		CreatedAt:      unixTime(task.AddTime),
		UpdatedAt:      unixTime(task.UpdateTime),
	}

	return result
}

func unixTime(timestamp int64) time.Time {
	if timestamp <= 0 {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

func (d *Driver) locateOfflineTaskFiles(client *driver.Pan115Client, task *driver.OfflineTask) []drivers.FileInfo {
	if task == nil {
		return nil
	}

	if task.FileId != "" {
		if file, err := client.GetFile(task.FileId); err == nil && file != nil && file.GetID() != "" {
			result := d.expandOfflineTaskEntry(client, *file, "", 0)
			if len(result) > 0 {
				return result
			}
		}
	}

	if task.DirId != "" {
		if files, err := client.List(task.DirId); err == nil && files != nil {
			matched := make([]driver.File, 0, len(*files))
			for _, file := range *files {
				if matchesOfflineTaskFile(task, file) {
					matched = append(matched, file)
				}
			}
			result := d.expandOfflineTaskEntries(client, matched, "")
			if len(result) > 0 {
				return result
			}
		}
	}

	keyword := offlineTaskSearchKeyword(task)
	if keyword == "" {
		return nil
	}

	searchResult, err := client.Search(&driver.SearchOption{
		SearchValue: keyword,
		Limit:       100,
	})
	if err != nil || searchResult == nil {
		return nil
	}

	result := make([]drivers.FileInfo, 0, len(searchResult.Files))
	for _, file := range searchResult.Files {
		if matchesOfflineTaskFile(task, file) {
			result = append(result, d.expandOfflineTaskEntry(client, file, "", 0)...)
		}
	}

	return result
}

func (d *Driver) expandOfflineTaskEntries(client *driver.Pan115Client, entries []driver.File, basePath string) []drivers.FileInfo {
	result := make([]drivers.FileInfo, 0, len(entries))
	for _, entry := range entries {
		result = append(result, d.expandOfflineTaskEntry(client, entry, basePath, 0)...)
	}
	return result
}

func (d *Driver) expandOfflineTaskEntry(client *driver.Pan115Client, entry driver.File, basePath string, depth int) []drivers.FileInfo {
	entryPath := joinRemotePath(basePath, entry.Name)
	if !entry.IsDirectory {
		return []drivers.FileInfo{toFileInfo(entry, entryPath)}
	}
	if entry.GetID() == "" || depth >= maxOfflineTaskExpandDepth {
		return nil
	}

	files, err := client.List(entry.GetID())
	if err != nil || files == nil {
		return nil
	}

	result := make([]drivers.FileInfo, 0, len(*files))
	for _, child := range *files {
		result = append(result, d.expandOfflineTaskEntry(client, child, entryPath, depth+1)...)
	}
	return result
}

func (d *Driver) dirInfoFromPath(dirID, remotePath string) *drivers.FileInfo {
	cleaned := cleanRemotePath(remotePath)
	if cleaned == "" {
		return &drivers.FileInfo{
			ID:     "0",
			FileID: "0",
			Name:   "/",
			Path:   "/",
			IsDir:  true,
		}
	}

	return &drivers.FileInfo{
		ID:           dirID,
		FileID:       dirID,
		Name:         path.Base(cleaned),
		Path:         "/" + cleaned,
		RelativePath: cleaned,
		IsDir:        true,
	}
}

func (d *Driver) dirInfoByID(client *driver.Pan115Client, dirID string, fallbackPath string) *drivers.FileInfo {
	if dirID == "" || dirID == "0" {
		return d.dirInfoFromPath("0", fallbackPath)
	}

	if stat, err := client.Stat(dirID); err == nil && stat != nil {
		remotePath := dirStatPath(stat)
		if remotePath == "" {
			remotePath = fallbackPath
		}
		info := d.dirInfoFromPath(dirID, remotePath)
		info.Name = stat.Name
		info.CreatedAt = stat.CreateTime
		info.UpdatedAt = stat.UpdateTime
		if len(stat.Parents) > 0 {
			info.ParentID = stat.Parents[len(stat.Parents)-1].ID
		}
		return info
	}

	if file, err := client.GetFile(dirID); err == nil && file != nil && file.GetID() != "" {
		info := toFileInfo(*file, fallbackPath)
		if info.Path == "" {
			info.Path = "/" + cleanRemotePath(file.Name)
		}
		info.RelativePath = cleanRemotePath(info.Path)
		info.IsDir = true
		return &info
	}

	return d.dirInfoFromPath(dirID, fallbackPath)
}

func dirStatPath(stat *driver.FileStatInfo) string {
	if stat == nil {
		return ""
	}

	parts := make([]string, 0, len(stat.Parents)+1)
	for _, parent := range stat.Parents {
		if parent == nil || parent.Name == "" {
			continue
		}
		parts = append(parts, parent.Name)
	}
	if stat.Name != "" {
		parts = append(parts, stat.Name)
	}
	if len(parts) == 0 {
		return ""
	}
	return "/" + path.Join(parts...)
}

func matchesOfflineTaskFile(task *driver.OfflineTask, file driver.File) bool {
	if task == nil {
		return false
	}
	if task.FileId != "" && file.GetID() == task.FileId {
		return true
	}
	if task.Name != "" && strings.EqualFold(file.Name, task.Name) {
		return true
	}
	return false
}

func offlineTaskSearchKeyword(task *driver.OfflineTask) string {
	if task == nil {
		return ""
	}
	if strings.TrimSpace(task.Name) != "" {
		return strings.TrimSpace(task.Name)
	}
	return strings.TrimSpace(task.InfoHash)
}

func mapOfflineStatus(task *driver.OfflineTask) drivers.TaskStatus {
	switch {
	case task.IsTodo():
		return drivers.TaskPending
	case task.IsDone() || (task.Percent >= 100 && strings.TrimSpace(task.FileId) != ""):
		return drivers.TaskCompleted
	case task.IsRunning():
		return drivers.TaskRunning
	case task.IsFailed():
		return drivers.TaskFailed
	default:
		return drivers.TaskUnknown
	}
}
