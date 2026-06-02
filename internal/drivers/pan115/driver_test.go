package pan115

import (
	"fmt"
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

func TestIsTargetAlreadyExists(t *testing.T) {
	if !isTargetAlreadyExists(fmt.Errorf("create dir path: %w", pan115driver.ErrExist)) {
		t.Fatalf("isTargetAlreadyExists() = false, want true")
	}
	if isTargetAlreadyExists(fmt.Errorf("create dir path: %w", pan115driver.ErrNotExist)) {
		t.Fatalf("isTargetAlreadyExists() = true, want false")
	}
}

func TestCleanRemotePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "root", in: "/", want: ""},
		{name: "empty", in: "", want: ""},
		{name: "nested", in: "/get-magnet/JavDB/", want: "get-magnet/JavDB"},
		{name: "cleans dot segments", in: "/get-magnet/./JavDB", want: "get-magnet/JavDB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanRemotePath(tt.in); got != tt.want {
				t.Fatalf("cleanRemotePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinRemotePath(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		base string
		want string
	}{
		{name: "root", dir: "/", base: "JavDB", want: "/JavDB"},
		{name: "empty root", dir: "", base: "JavDB", want: "/JavDB"},
		{name: "nested", dir: "/get-magnet", base: "JavDB", want: "/get-magnet/JavDB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinRemotePath(tt.dir, tt.base); got != tt.want {
				t.Fatalf("joinRemotePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchesOfflineTaskFile(t *testing.T) {
	task := &pan115driver.OfflineTask{
		FileId: "file-1",
		Name:   "Example.mp4",
	}

	tests := []struct {
		name string
		file pan115driver.File
		want bool
	}{
		{name: "matches id", file: pan115driver.File{FileID: "file-1", Name: "Other.mp4"}, want: true},
		{name: "matches name case insensitive", file: pan115driver.File{FileID: "file-2", Name: "example.mp4"}, want: true},
		{name: "no match", file: pan115driver.File{FileID: "file-2", Name: "Other.mp4"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesOfflineTaskFile(task, tt.file); got != tt.want {
				t.Fatalf("matchesOfflineTaskFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOfflineTaskSearchKeyword(t *testing.T) {
	tests := []struct {
		name string
		task *pan115driver.OfflineTask
		want string
	}{
		{name: "nil", task: nil, want: ""},
		{name: "name first", task: &pan115driver.OfflineTask{Name: " Example ", InfoHash: "hash"}, want: "Example"},
		{name: "hash fallback", task: &pan115driver.OfflineTask{InfoHash: " hash "}, want: "hash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := offlineTaskSearchKeyword(tt.task); got != tt.want {
				t.Fatalf("offlineTaskSearchKeyword() = %q, want %q", got, tt.want)
			}
		})
	}
}
