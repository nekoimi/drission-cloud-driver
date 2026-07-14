package offline

import (
	"path/filepath"
	"testing"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
)

func TestSQLiteStorePersistsRecords(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "offline.db")

	store, err := NewSQLiteStore(dsn)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}

	record := OfflineTaskRecord{
		Platform:     "115",
		ProfileID:    "profile-a",
		ClientTaskID: "client-1",
		URL:          "magnet:?xt=urn:btih:abc",
		Category:     "movie",
		SavePath:     "/downloads",
		Metadata:     map[string]string{"source": "test"},
		Task: drivers.OfflineTask{
			TaskID:         "115:abc",
			ProviderTaskID: "abc",
			Status:         drivers.TaskPending,
			Name:           "movie",
			Progress:       12.5,
			SavePath:       "/downloads",
			SaveDir: &drivers.FileInfo{
				ID:    "dir-1",
				Name:  "downloads",
				Path:  "/downloads",
				IsDir: true,
			},
			Files: []drivers.FileInfo{{ID: "file-1", Name: "movie.mp4"}},
		},
	}

	if err := store.Put(record); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := NewSQLiteStore(dsn)
	if err != nil {
		t.Fatalf("reopen NewSQLiteStore() error = %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	got, ok := reopened.GetByClientTask("115", "profile-a", "client-1")
	if !ok {
		t.Fatalf("GetByClientTask() ok = false, want true")
	}
	if got.Task.TaskID != "115:abc" {
		t.Fatalf("TaskID = %q, want %q", got.Task.TaskID, "115:abc")
	}
	if got.URL != record.URL || got.Category != record.Category || got.SavePath != record.SavePath {
		t.Fatalf("record = %+v, want submitted metadata preserved", got)
	}
	if got.Metadata["source"] != "test" {
		t.Fatalf("metadata source = %q, want test", got.Metadata["source"])
	}
	if got.Task.SaveDir == nil || got.Task.SaveDir.ID != "dir-1" {
		t.Fatalf("save dir = %#v, want persisted dir", got.Task.SaveDir)
	}
	if len(got.Task.Files) != 1 || got.Task.Files[0].ID != "file-1" {
		t.Fatalf("files = %#v, want persisted file", got.Task.Files)
	}
}

func TestSQLiteStoreUpdate(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "offline.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	record := OfflineTaskRecord{
		Platform:     "115",
		ProfileID:    "profile-a",
		ClientTaskID: "client-1",
		Task: drivers.OfflineTask{
			TaskID: "115:abc",
			Status: drivers.TaskPending,
		},
	}
	if err := store.Put(record); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	record.ClientTaskID = "client-2"
	record.Task.Status = drivers.TaskCompleted
	if err := store.Update(record); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if _, ok := store.GetByClientTask("115", "profile-a", "client-1"); ok {
		t.Fatalf("old client task index still exists")
	}

	got, ok := store.GetByClientTask("115", "profile-a", "client-2")
	if !ok {
		t.Fatalf("new client task index missing")
	}
	if got.Task.Status != drivers.TaskCompleted {
		t.Fatalf("status = %q, want %q", got.Task.Status, drivers.TaskCompleted)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not stored: %+v", got)
	}
}
