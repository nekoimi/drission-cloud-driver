package offline

import (
	"testing"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
)

func TestMemoryStorePutAndGet(t *testing.T) {
	store := NewMemoryStore()
	record := OfflineTaskRecord{
		Platform:     "115",
		ProfileID:    "profile-a",
		ClientTaskID: "client-1",
		Metadata:     map[string]string{"source": "test"},
		Task: drivers.OfflineTask{
			TaskID: "115:abc",
			Status: drivers.TaskPending,
			SaveDir: &drivers.FileInfo{
				ID:    "dir-1",
				Name:  "downloads",
				Path:  "/downloads",
				IsDir: true,
				Extra: map[string]any{
					"provider": "115",
				},
			},
			Files: []drivers.FileInfo{{
				ID:    "file-1",
				Name:  "movie.mp4",
				Extra: map[string]any{"pick_code": "pc"},
			}},
		},
	}

	if err := store.Put(record); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, ok := store.Get("115:abc")
	if !ok {
		t.Fatalf("Get() ok = false, want true")
	}
	if got.Task.TaskID != "115:abc" {
		t.Fatalf("TaskID = %q, want %q", got.Task.TaskID, "115:abc")
	}
	if got.Metadata["source"] != "test" {
		t.Fatalf("metadata source = %q, want test", got.Metadata["source"])
	}

	got.Metadata["source"] = "changed"
	got.Task.SaveDir.Name = "changed"
	got.Task.SaveDir.Extra["provider"] = "changed"
	got.Task.Files[0].Name = "changed.mp4"
	got.Task.Files[0].Extra["pick_code"] = "changed"

	again, _ := store.Get("115:abc")
	if again.Metadata["source"] != "test" {
		t.Fatalf("stored metadata was mutated through returned record")
	}
	if again.Task.SaveDir.Name != "downloads" || again.Task.SaveDir.Extra["provider"] != "115" {
		t.Fatalf("stored save dir was mutated through returned record")
	}
	if again.Task.Files[0].Name != "movie.mp4" {
		t.Fatalf("stored files were mutated through returned record")
	}
	if again.Task.Files[0].Extra["pick_code"] != "pc" {
		t.Fatalf("stored file extra was mutated through returned record")
	}
}

func TestMemoryStoreGetByClientTask(t *testing.T) {
	store := NewMemoryStore()
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

	got, ok := store.GetByClientTask("115", "profile-a", "client-1")
	if !ok {
		t.Fatalf("GetByClientTask() ok = false, want true")
	}
	if got.Task.TaskID != "115:abc" {
		t.Fatalf("TaskID = %q, want %q", got.Task.TaskID, "115:abc")
	}
}

func TestMemoryStoreUpdateReindexesClientTask(t *testing.T) {
	store := NewMemoryStore()
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
}

func TestMemoryStoreRejectsEmptyTaskID(t *testing.T) {
	store := NewMemoryStore()

	if err := store.Put(OfflineTaskRecord{}); err != ErrTaskIDRequired {
		t.Fatalf("Put() error = %v, want %v", err, ErrTaskIDRequired)
	}
}
