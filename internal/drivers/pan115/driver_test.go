package pan115

import (
	"testing"

	pan115driver "github.com/SheltonZhu/115driver/pkg/driver"

	"github.com/nekoimi/drission-cloud-driver/internal/drivers"
)

func TestMapOfflineStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   drivers.TaskStatus
	}{
		{name: "pending", status: 0, want: drivers.TaskPending},
		{name: "running", status: 1, want: drivers.TaskRunning},
		{name: "completed", status: 2, want: drivers.TaskCompleted},
		{name: "failed", status: -1, want: drivers.TaskFailed},
		{name: "unknown", status: 99, want: drivers.TaskUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &pan115driver.OfflineTask{Status: tt.status}
			if got := mapOfflineStatus(task); got != tt.want {
				t.Fatalf("mapOfflineStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToOfflineTaskBuildsUnifiedTaskID(t *testing.T) {
	task := &pan115driver.OfflineTask{
		InfoHash: "abc",
		Name:     "example",
		Status:   1,
		Percent:  42,
	}

	got := toOfflineTask(task)
	if got.TaskID != "115:abc" {
		t.Fatalf("TaskID = %q, want %q", got.TaskID, "115:abc")
	}
	if got.ProviderTaskID != "abc" {
		t.Fatalf("ProviderTaskID = %q, want %q", got.ProviderTaskID, "abc")
	}
	if got.Status != drivers.TaskRunning {
		t.Fatalf("Status = %q, want %q", got.Status, drivers.TaskRunning)
	}
	if got.Progress != 0.42 {
		t.Fatalf("Progress = %v, want 0.42", got.Progress)
	}
}
