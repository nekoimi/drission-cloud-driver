package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nekoimi/drission-cloud-driver/internal/pkg/errcode"
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
		{name: "not found", code: errcode.NotFound.Value, want: http.StatusNotFound},
		{name: "too many requests", code: errcode.TooManyReq.Value, want: http.StatusTooManyRequests},
		{name: "internal", code: errcode.Internal.Value, want: http.StatusInternalServerError},
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
