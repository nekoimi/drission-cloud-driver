package v1

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestBadRequestUsesInvalidRequestCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	badRequest(c, "missing field")

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

func TestOperationFailedPassesThroughAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	operationFailed(c, errcode.NewWithDetail(errcode.ErrTaskNotFound, "task missing"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}

	var got response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != errcode.ErrTaskNotFound.Value {
		t.Fatalf("code = %d, want %d", got.Code, errcode.ErrTaskNotFound.Value)
	}
}

func TestAddOfflineTaskIsIdempotentByClientTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fake := &idempotentDriver{}
	registry := drivers.NewRegistry(zap.NewNop())
	registry.Register("115", func(*browser.Manager, *zap.Logger) (drivers.Driver, error) {
		return fake, nil
	})

	handler := NewDriverHandler(registry, nil, offline.NewMemoryStore(), zap.NewNop())
	router := gin.New()
	router.POST("/drivers/:platform/offline/add", handler.AddOfflineTask)

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
	if fake.queryCalls != 1 {
		t.Fatalf("QueryOfflineTask calls = %d, want 1", fake.queryCalls)
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
	if got.Data.Status != drivers.TaskRunning {
		t.Fatalf("status = %q, want %q", got.Data.Status, drivers.TaskRunning)
	}
}

type idempotentDriver struct {
	addCalls   int
	queryCalls int
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
