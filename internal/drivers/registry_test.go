package drivers

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/nekoimi/drission-cloud-driver/internal/browser"
)

type fakeDriver struct{}

func (fakeDriver) Platform() string {
	return "fake"
}

func (fakeDriver) Capabilities() DriverCapabilities {
	return DriverCapabilities{Search: true}
}

func (fakeDriver) AddOfflineTask(context.Context, string, *AddTaskRequest) (*OfflineTask, error) {
	return nil, nil
}

func (fakeDriver) QueryOfflineTask(context.Context, string, string) (*OfflineTask, error) {
	return nil, nil
}

func (fakeDriver) RemoveOfflineTask(context.Context, string, string) error {
	return nil
}

func (fakeDriver) ListOfflineTasks(context.Context, string) (*OfflineTaskList, error) {
	return nil, nil
}

func (fakeDriver) Mkdir(context.Context, string, string, string) error {
	return nil
}

func (fakeDriver) Remove(context.Context, string, string) error {
	return nil
}

func (fakeDriver) Move(context.Context, string, string, string) error {
	return nil
}

func (fakeDriver) Rename(context.Context, string, string, string) error {
	return nil
}

func (fakeDriver) List(context.Context, string, string) ([]FileInfo, error) {
	return nil, nil
}

func (fakeDriver) Search(context.Context, string, string) ([]FileInfo, error) {
	return nil, nil
}

func (fakeDriver) GetDownloadURL(context.Context, string, string) (string, error) {
	return "", nil
}

func TestRegistryRegisterGetAndCache(t *testing.T) {
	registry := NewRegistry(zap.NewNop())
	createCount := 0
	registry.Register("fake", func(*browser.Manager, *zap.Logger) (Driver, error) {
		createCount++
		return fakeDriver{}, nil
	})

	first, err := registry.Get("fake", nil)
	if err != nil {
		t.Fatalf("first Get() error = %v", err)
	}
	second, err := registry.Get("fake", nil)
	if err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if first != second {
		t.Fatalf("Get() did not return cached driver")
	}
	if createCount != 1 {
		t.Fatalf("factory called %d times, want 1", createCount)
	}
}

func TestRegistryUnknownPlatform(t *testing.T) {
	registry := NewRegistry(zap.NewNop())

	if _, err := registry.Get("missing", nil); err == nil {
		t.Fatalf("Get() error = nil, want error")
	}
}

func TestRegistryListPlatforms(t *testing.T) {
	registry := NewRegistry(zap.NewNop())
	registry.Register("fake", func(*browser.Manager, *zap.Logger) (Driver, error) {
		return fakeDriver{}, nil
	})

	platforms := registry.ListPlatforms()
	if len(platforms) != 1 || platforms[0] != "fake" {
		t.Fatalf("ListPlatforms() = %#v, want [fake]", platforms)
	}
}
