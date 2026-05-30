package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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
