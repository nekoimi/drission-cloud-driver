package drivers

import (
	"encoding/json"
	"testing"
)

func TestTaskStatusValues(t *testing.T) {
	tests := map[TaskStatus]string{
		TaskPending:   "pending",
		TaskRunning:   "running",
		TaskCompleted: "completed",
		TaskFailed:    "failed",
		TaskCanceled:  "canceled",
		TaskUnknown:   "unknown",
	}

	for status, want := range tests {
		if string(status) != want {
			t.Fatalf("status = %q, want %q", status, want)
		}
	}
}

func TestAddTaskRequestUnmarshal(t *testing.T) {
	body := []byte(`{
		"url": "magnet:?xt=urn:btih:abc",
		"category": "JavDB",
		"save_path": "/get-magnet/JavDB",
		"client_task_id": "task-1",
		"metadata": {"origin": "JavDB"}
	}`)

	var req AddTaskRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal AddTaskRequest: %v", err)
	}

	if req.URL == "" || req.Category != "JavDB" || req.SavePath != "/get-magnet/JavDB" {
		t.Fatalf("request fields not decoded: %+v", req)
	}
	if req.ClientTaskID != "task-1" {
		t.Fatalf("client_task_id = %q, want task-1", req.ClientTaskID)
	}
	if req.Metadata["origin"] != "JavDB" {
		t.Fatalf("metadata origin = %q, want JavDB", req.Metadata["origin"])
	}
}

func TestOfflineTaskListJSONShape(t *testing.T) {
	list := OfflineTaskList{
		Items: []OfflineTask{{
			TaskID:         "task-id",
			ProviderTaskID: "provider-id",
			Status:         TaskPending,
			Name:           "example",
			SavePath:       "/get-magnet",
		}},
		Total: 1,
	}

	bytes, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshal OfflineTaskList: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(bytes, &got); err != nil {
		t.Fatalf("decode OfflineTaskList JSON: %v", err)
	}
	if _, ok := got["items"]; !ok {
		t.Fatalf("items field missing in %s", string(bytes))
	}
	if _, ok := got["total"]; !ok {
		t.Fatalf("total field missing in %s", string(bytes))
	}
	if _, ok := got["tasks"]; ok {
		t.Fatalf("legacy tasks field should not be present in %s", string(bytes))
	}
}
