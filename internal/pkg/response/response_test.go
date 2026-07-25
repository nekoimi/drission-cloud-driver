package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestSuccessWritesUnifiedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Success(c, gin.H{"status": "ok"})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != errcode.OK.Value || got.Message != errcode.OK.Message {
		t.Fatalf("response = %+v, want success code/message", got)
	}
	if got.Data == nil {
		t.Fatalf("data is nil, want payload")
	}
}

func TestErrorWithMsgWritesStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	ErrorWithMsg(c, http.StatusBadRequest, errcode.BadRequest, "bad input")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var got APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Code != errcode.BadRequest.Value {
		t.Fatalf("code = %d, want %d", got.Code, errcode.BadRequest.Value)
	}
	if got.Message != "bad input" {
		t.Fatalf("message = %q, want %q", got.Message, "bad input")
	}
}

func TestHTTPStatusFromCode(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{name: "bad request", code: errcode.BadRequest.Value, want: http.StatusBadRequest},
		{name: "invalid request", code: errcode.ErrInvalidRequest.Value, want: http.StatusBadRequest},
		{name: "profile not logged in", code: errcode.ErrProfileNotLoggedIn.Value, want: http.StatusUnauthorized},
		{name: "not found", code: errcode.NotFound.Value, want: http.StatusNotFound},
		{name: "offline task not found", code: errcode.ErrTaskNotFound.Value, want: http.StatusNotFound},
		{name: "idempotent conflict", code: errcode.ErrIdempotentConflict.Value, want: http.StatusConflict},
		{name: "too many requests", code: errcode.TooManyReq.Value, want: http.StatusTooManyRequests},
		{name: "internal", code: errcode.Internal.Value, want: http.StatusInternalServerError},
		{name: "platform state", code: errcode.ErrPlatformState.Value, want: http.StatusInternalServerError},
		{name: "unknown", code: 1, want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := httpStatusFromCode(tt.code); got != tt.want {
				t.Fatalf("httpStatusFromCode(%d) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

func TestHandleLogsServerAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("X-Request-ID", "request-123")
		c.Next()
	})
	router.DELETE("/drivers/115/fs/remove", Handle(func(c *gin.Context) (any, error) {
		return nil, errcode.Wrap(errcode.ErrOperationFailed, errors.New("remove /test: api rejected request"))
	}, logger))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/drivers/115/fs/remove", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if logs.Len() != 1 {
		t.Fatalf("log entries = %d, want 1", logs.Len())
	}

	entry := logs.All()[0]
	if entry.Message != "handler error" {
		t.Fatalf("message = %q, want %q", entry.Message, "handler error")
	}
	fields := entry.ContextMap()
	if fields["code"] != int64(errcode.ErrOperationFailed.Value) {
		t.Fatalf("code field = %v, want %d", fields["code"], errcode.ErrOperationFailed.Value)
	}
	if fields["request_id"] != "request-123" {
		t.Fatalf("request_id field = %v, want %q", fields["request_id"], "request-123")
	}
	if fields["error"] != "[50012] driver operation failed: remove /test: api rejected request" {
		t.Fatalf("error field = %v, want wrapped operation error", fields["error"])
	}
}
