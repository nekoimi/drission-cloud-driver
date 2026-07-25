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
		SyncError:    "sync\x00error",
		CleanupError: "cleanup\x00error",
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
		got.SyncError,
		got.CleanupError,
	}
	for _, value := range values {
		if hasNUL(value) {
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

func TestPostgresClientKeyIsStableAndContainsNoNUL(t *testing.T) {
	got := postgresClientKey("115", "profile-a", "client-1")
	if got == "" {
		t.Fatal("postgresClientKey() is empty")
	}
	if hasNUL(got) {
		t.Fatalf("postgresClientKey() contains NUL: %q", got)
	}
	if len(got) != sha256HexLength {
		t.Fatalf("postgresClientKey() length = %d, want %d", len(got), sha256HexLength)
	}
	if again := postgresClientKey("115", "profile-a", "client-1"); again != got {
		t.Fatalf("postgresClientKey() is not stable: %q != %q", got, again)
	}
	if other := postgresClientKey("115", "profile-a", "client-2"); other == got {
		t.Fatalf("different client task IDs produced the same key: %q", got)
	}
	if empty := postgresClientKey("115", "profile-a", ""); empty != "" {
		t.Fatalf("empty client task ID produced key %q", empty)
	}
}

const sha256HexLength = 64

func hasNUL(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] == 0 {
			return true
		}
	}
	return false
}
