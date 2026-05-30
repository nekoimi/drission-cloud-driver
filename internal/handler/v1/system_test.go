package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/response"
)

func TestSystemHandlerHealthUsesUnifiedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSystemHandler(drivers.NewRegistry(zap.NewNop()), zap.NewNop())

	router := gin.New()
	router.GET("/health", handler.Health)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != errcode.OK.Value || got.Message != errcode.OK.Message {
		t.Fatalf("response = %+v, want success envelope", got)
	}
	if got.Data == nil {
		t.Fatalf("data is nil, want health payload")
	}
}
