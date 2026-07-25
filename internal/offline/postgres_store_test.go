package offline

import (
	"testing"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
)

func TestSanitizePostgresRecordRemovesNULBytes(t *testing.T) {
	record := OfflineTaskRecord{
		Platform:     "1\x0015",
		ProfileID:    "profile\x00-a",
		ClientTaskID: "client\x00-1",
		URL:          "magnet:\x00?xt=urn:btih:abc",
		Category:     "mov\x00ie",
		SavePath:     "/down\x00loads",
		Task: drivers.OfflineTask{
			TaskID:         "115:\x00abc",
			ProviderTaskID: "a\x00bc",
			Status:         drivers.TaskStatus("pend\x00ing"),
			Name:           "mov\x00ie",
			SavePath:       "/down\x00loads",
			ErrorCode:      "E\x001",
			ErrorMessage:   "bad\x00value",
		},
	}

	got := sanitizePostgresRecord(record)

	values := []string{
		got.Platform,
		got.ProfileID,
		got.ClientTaskID,
		got.URL,
		got.Category,
		got.SavePath,
		got.Task.TaskID,
		got.Task.ProviderTaskID,
		string(got.Task.Status),
		got.Task.Name,
		got.Task.SavePath,
		got.Task.ErrorCode,
		got.Task.ErrorMessage,
	}
	for _, value := range values {
		if containsNUL := len(value) != len([]byte(value)) || hasNUL(value); containsNUL {
			t.Fatalf("sanitized value still contains NUL: %q", value)
		}
	}

	if got.Task.TaskID != "115:abc" {
		t.Fatalf("TaskID = %q, want %q", got.Task.TaskID, "115:abc")
	}
	if got.URL != "magnet:?xt=urn:btih:abc" {
		t.Fatalf("URL = %q, want NUL removed", got.URL)
	}
}

func hasNUL(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] == 0 {
			return true
		}
	}
	return false
}
