package driver

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
	"github.com/nekoimi/drission-cloud-driver/internal/config"
	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
	"github.com/nekoimi/drission-cloud-driver/internal/offline"
)

type syncTestDriver struct {
	*idempotentDriver
	tasks       []drivers.OfflineTask
	removeCalls int
}

type failingUpdateStore struct {
	offline.Store
}

func (s *failingUpdateStore) Update(record offline.OfflineTaskRecord) error {
	if record.Task.Status == drivers.TaskCompleted {
		return context.DeadlineExceeded
	}
	return s.Store.Update(record)
}

func (d *syncTestDriver) ListOfflineTasks(context.Context, string) (*drivers.OfflineTaskList, error) {
	return &drivers.OfflineTaskList{Items: d.tasks, Total: len(d.tasks)}, nil
}

func (d *syncTestDriver) RemoveOfflineTask(context.Context, string, string) error {
	d.removeCalls++
	return nil
}

func TestOfflineSyncerPersistsCompletionBeforeRemoteCleanup(t *testing.T) {
	logger := zap.NewNop()
	store := offline.NewMemoryStore()
	if err := store.Put(offline.OfflineTaskRecord{
		Platform:  "115",
		ProfileID: "profile-a",
		SavePath:  "/downloads",
		Task: drivers.OfflineTask{
			TaskID:         "115:abc",
			ProviderTaskID: "abc",
			Status:         drivers.TaskRunning,
		},
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	fake := &syncTestDriver{
		idempotentDriver: &idempotentDriver{},
		tasks: []drivers.OfflineTask{{
			TaskID:         "115:abc",
			ProviderTaskID: "abc",
			ProviderStatus: 2,
			Status:         drivers.TaskCompleted,
			Progress:       1,
			Files:          []drivers.FileInfo{{ID: "file-1", Name: "movie.mp4"}},
		}},
	}
	registry := drivers.NewRegistry(logger)
	registry.Register("115", func(_ *browser.Manager, _ *zap.Logger) (drivers.Driver, error) {
		return fake, nil
	})

	syncer := newOfflineSyncer(store, registry, nil, config.OfflineSyncConfig{
		Enabled:             true,
		CleanupCompleted:    true,
		CleanupGraceSeconds: 0,
	}, logger)
	syncer.syncOnce(context.Background())

	got, ok := store.Get("115:abc")
	if !ok {
		t.Fatal("synchronized task not found")
	}
	if got.Task.Status != drivers.TaskCompleted {
		t.Fatalf("status = %q, want completed", got.Task.Status)
	}
	if len(got.Task.Files) != 1 || got.Task.Files[0].ID != "file-1" {
		t.Fatalf("files = %#v, want completed file snapshot", got.Task.Files)
	}
	if got.CompletedAt.IsZero() {
		t.Fatal("CompletedAt is zero")
	}
	if got.RemoteCleanedAt.IsZero() {
		t.Fatal("RemoteCleanedAt is zero")
	}
	if fake.removeCalls != 1 {
		t.Fatalf("remove calls = %d, want 1", fake.removeCalls)
	}
}

func TestOfflineSyncerDoesNotCleanupWhenCompletionPersistenceFails(t *testing.T) {
	logger := zap.NewNop()
	baseStore := offline.NewMemoryStore()
	if err := baseStore.Put(offline.OfflineTaskRecord{
		Platform:  "115",
		ProfileID: "profile-a",
		Task: drivers.OfflineTask{
			TaskID:         "115:abc",
			ProviderTaskID: "abc",
			Status:         drivers.TaskRunning,
		},
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	store := &failingUpdateStore{Store: baseStore}

	fake := &syncTestDriver{
		idempotentDriver: &idempotentDriver{},
		tasks: []drivers.OfflineTask{{
			TaskID:         "115:abc",
			ProviderTaskID: "abc",
			Status:         drivers.TaskCompleted,
			Files:          []drivers.FileInfo{{ID: "file-1"}},
		}},
	}
	registry := drivers.NewRegistry(logger)
	registry.Register("115", func(_ *browser.Manager, _ *zap.Logger) (drivers.Driver, error) {
		return fake, nil
	})

	syncer := newOfflineSyncer(store, registry, nil, config.OfflineSyncConfig{
		Enabled:          true,
		CleanupCompleted: true,
	}, logger)
	syncer.syncOnce(context.Background())

	if fake.removeCalls != 0 {
		t.Fatalf("remove calls = %d, want 0 after persistence failure", fake.removeCalls)
	}
	got, _ := baseStore.Get("115:abc")
	if got.Task.Status != drivers.TaskRunning {
		t.Fatalf("stored status = %q, want running after failed update", got.Task.Status)
	}
}
