package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/offline"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
)

func TestParseMkdirRequestWithPath(t *testing.T) {
	parentPath, name, err := parseMkdirRequest("/get-magnet/JavDB/2026-05-30", "", "")
	if err != nil {
		t.Fatalf("parseMkdirRequest() error = %v", err)
	}
	if parentPath != "/get-magnet/JavDB" {
		t.Fatalf("parentPath = %q, want %q", parentPath, "/get-magnet/JavDB")
	}
	if name != "2026-05-30" {
		t.Fatalf("name = %q, want %q", name, "2026-05-30")
	}
}

func TestParseMkdirRequestWithParentAndName(t *testing.T) {
	parentPath, name, err := parseMkdirRequest("", "/get-magnet/JavDB", "2026-05-30")
	if err != nil {
		t.Fatalf("parseMkdirRequest() error = %v", err)
	}
	if parentPath != "/get-magnet/JavDB" {
		t.Fatalf("parentPath = %q, want %q", parentPath, "/get-magnet/JavDB")
	}
	if name != "2026-05-30" {
		t.Fatalf("name = %q, want %q", name, "2026-05-30")
	}
}

func TestParseMkdirRequestRejectsRootPath(t *testing.T) {
	if _, _, err := parseMkdirRequest("/", "", ""); err == nil {
		t.Fatalf("parseMkdirRequest() error = nil, want error")
	}
}

func TestParseMkdirRequestRequiresNameForLegacyFormat(t *testing.T) {
	if _, _, err := parseMkdirRequest("", "/get-magnet", ""); err == nil {
		t.Fatalf("parseMkdirRequest() error = nil, want error")
	}
}

func TestHandleWrapsAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := zap.NewNop()

	fn := func(c *gin.Context) (any, error) {
		return nil, errcode.NewWithDetail(errcode.ErrInvalidRequest, "missing field")
	}

	router := gin.New()
	router.GET("/test", response.Handle(fn, log))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var got response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != errcode.ErrInvalidRequest.Value {
		t.Fatalf("code = %d, want %d", got.Code, errcode.ErrInvalidRequest.Value)
	}
}

func TestHandleWrapsGenericError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := zap.NewNop()

	fn := func(c *gin.Context) (any, error) {
		return nil, errors.New("something broke")
	}

	router := gin.New()
	router.GET("/test", response.Handle(fn, log))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var got response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != errcode.Internal.Value {
		t.Fatalf("code = %d, want %d", got.Code, errcode.Internal.Value)
	}
}

func TestAddOfflineTaskIsIdempotentByClientTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := zap.NewNop()

	fake := &idempotentDriver{}
	registry := drivers.NewRegistry(log)
	registry.Register("115", func(*browser.Manager, *zap.Logger) (drivers.Driver, error) {
		return fake, nil
	})

	handler := newDriverHandler(registry, nil, offline.NewMemoryStore(), log)
	router := gin.New()
	router.POST("/drivers/:platform/offline/add", response.Handle(handler.AddOfflineTask, log))

	body := []byte(`{"url":"magnet:?xt=urn:btih:abc","client_task_id":"client-1","save_path":"/downloads"}`)

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/drivers/115/offline/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Profile-ID", "profile-a")
	router.ServeHTTP(first, req)

	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, body = %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/drivers/115/offline/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Profile-ID", "profile-a")
	router.ServeHTTP(second, req)

	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, body = %s", second.Code, second.Body.String())
	}
	if fake.addCalls != 1 {
		t.Fatalf("AddOfflineTask calls = %d, want 1", fake.addCalls)
	}
	if fake.queryCalls != 0 {
		t.Fatalf("QueryOfflineTask calls = %d, want 0", fake.queryCalls)
	}

	var got struct {
		Data drivers.OfflineTask `json:"data"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if got.Data.TaskID != "115:abc" {
		t.Fatalf("task_id = %q, want %q", got.Data.TaskID, "115:abc")
	}
	if got.Data.Status != drivers.TaskPending {
		t.Fatalf("status = %q, want %q", got.Data.Status, drivers.TaskPending)
	}
	if fake.queryCalls != 0 {
		t.Fatalf("provider query calls = %d, want 0", fake.queryCalls)
	}
}

func TestGetOfflineTaskReadsStoredRecordWithoutProviderQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := zap.NewNop()

	store := offline.NewMemoryStore()
	if err := store.Put(offline.OfflineTaskRecord{
		Platform:  "115",
		ProfileID: "profile-a",
		Task: drivers.OfflineTask{
			TaskID: "115:abc",
			Status: drivers.TaskCompleted,
		},
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	fake := &idempotentDriver{queryErr: errors.New("platform task missing")}
	registry := drivers.NewRegistry(log)
	registry.Register("115", func(*browser.Manager, *zap.Logger) (drivers.Driver, error) {
		return fake, nil
	})

	handler := newDriverHandler(registry, nil, store, log)
	router := gin.New()
	router.GET("/drivers/:platform/offline/tasks/:id", response.Handle(handler.GetOfflineTask, log))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/drivers/115/offline/tasks/115:abc", nil)
	req.Header.Set("X-Profile-ID", "profile-a")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var got struct {
		Data drivers.OfflineTask `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.Status != drivers.TaskCompleted {
		t.Fatalf("status = %q, want %q", got.Data.Status, drivers.TaskCompleted)
	}
	if fake.queryCalls != 0 {
		t.Fatalf("provider query calls = %d, want 0", fake.queryCalls)
	}
}

func TestNormalizeStoredOfflineTaskPrefixesSavePath(t *testing.T) {
	record := offline.OfflineTaskRecord{
		SavePath: "/get-magnet/JavDB/2026-06-02",
	}
	task := &drivers.OfflineTask{
		Status: drivers.TaskCompleted,
		Files: []drivers.FileInfo{{
			ID:     "file-1",
			FileID: "file-1",
			Name:   "ADN-776-C.mp4",
			Path:   "/ADN-776-C/ADN-776-C.mp4",
			Size:   123,
		}},
	}

	normalizeStoredOfflineTask(&record, task)

	if task.SavePath != record.SavePath {
		t.Fatalf("SavePath = %q, want %q", task.SavePath, record.SavePath)
	}
	wantPath := "/get-magnet/JavDB/2026-06-02/ADN-776-C/ADN-776-C.mp4"
	if task.Files[0].Path != wantPath {
		t.Fatalf("file path = %q, want %q", task.Files[0].Path, wantPath)
	}
	if task.Files[0].RelativePath != "ADN-776-C/ADN-776-C.mp4" {
		t.Fatalf("relative path = %q, want task-local path", task.Files[0].RelativePath)
	}
	if task.Files[0].IsDir {
		t.Fatalf("normalized file should be a leaf file")
	}
	if task.SaveDir == nil || task.SaveDir.Path != record.SavePath {
		t.Fatalf("save dir = %+v, want path %q", task.SaveDir, record.SavePath)
	}
}

func TestNormalizeStoredOfflineTaskWarnsOnCompletedEmptyFiles(t *testing.T) {
	record := offline.OfflineTaskRecord{SavePath: "/downloads"}
	task := &drivers.OfflineTask{
		Status: drivers.TaskCompleted,
		Files:  []drivers.FileInfo{},
	}

	normalizeStoredOfflineTask(&record, task)

	if len(task.Warnings) == 0 {
		t.Fatalf("warnings is empty, want completed task warning")
	}
}

type idempotentDriver struct {
	addCalls   int
	queryCalls int
	queryErr   error
}

func (d *idempotentDriver) Platform() string {
	return "115"
}

func (d *idempotentDriver) Capabilities() drivers.DriverCapabilities {
	return drivers.DriverCapabilities{OfflineDownload: true}
}

func (d *idempotentDriver) AddOfflineTask(context.Context, string, *drivers.AddTaskRequest) (*drivers.OfflineTask, error) {
	d.addCalls++
	return &drivers.OfflineTask{
		TaskID:         "115:abc",
		ProviderTaskID: "abc",
		Status:         drivers.TaskPending,
		SavePath:       "/downloads",
	}, nil
}

func (d *idempotentDriver) QueryOfflineTask(context.Context, string, string) (*drivers.OfflineTask, error) {
	d.queryCalls++
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	return &drivers.OfflineTask{
		TaskID:         "115:abc",
		ProviderTaskID: "abc",
		Status:         drivers.TaskRunning,
		SavePath:       "/downloads",
	}, nil
}

func (d *idempotentDriver) RemoveOfflineTask(context.Context, string, string) error {
	return nil
}

func (d *idempotentDriver) ListOfflineTasks(context.Context, string) (*drivers.OfflineTaskList, error) {
	return nil, nil
}

func (d *idempotentDriver) Mkdir(context.Context, string, string, string) error {
	return nil
}

func (d *idempotentDriver) Remove(context.Context, string, string) error {
	return nil
}

func (d *idempotentDriver) Move(context.Context, string, string, string) error {
	return nil
}

func (d *idempotentDriver) Rename(context.Context, string, string, string) error {
	return nil
}

func (d *idempotentDriver) List(context.Context, string, string) ([]drivers.FileInfo, error) {
	return nil, nil
}

func (d *idempotentDriver) Search(context.Context, string, string) ([]drivers.FileInfo, error) {
	return nil, nil
}

func (d *idempotentDriver) GetDownloadURL(context.Context, string, string) (string, error) {
	return "", nil
}

func (d *idempotentDriver) GetDownloadURLByID(context.Context, string, string) (string, error) {
	return "", nil
}
