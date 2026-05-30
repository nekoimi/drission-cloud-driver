package drivers

import "testing"

func TestBuildTaskID(t *testing.T) {
	if got := BuildTaskID("115", "abc"); got != "115:abc" {
		t.Fatalf("BuildTaskID() = %q, want %q", got, "115:abc")
	}
	if got := BuildTaskID(" 115 ", " abc "); got != "115:abc" {
		t.Fatalf("BuildTaskID() trims inputs = %q, want %q", got, "115:abc")
	}
	if got := BuildTaskID("", "abc"); got != "abc" {
		t.Fatalf("BuildTaskID() with empty platform = %q, want %q", got, "abc")
	}
}

func TestParseTaskID(t *testing.T) {
	platform, providerTaskID, ok := ParseTaskID("115:abc")
	if !ok || platform != "115" || providerTaskID != "abc" {
		t.Fatalf("ParseTaskID() = %q, %q, %v; want 115, abc, true", platform, providerTaskID, ok)
	}

	platform, providerTaskID, ok = ParseTaskID("abc")
	if ok || platform != "" || providerTaskID != "abc" {
		t.Fatalf("ParseTaskID() legacy = %q, %q, %v; want empty, abc, false", platform, providerTaskID, ok)
	}

	_, _, ok = ParseTaskID("115:")
	if ok {
		t.Fatalf("ParseTaskID() accepted empty provider id")
	}
}

func TestProviderTaskID(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		taskID   string
		want     string
	}{
		{name: "unified", platform: "115", taskID: "115:abc", want: "abc"},
		{name: "legacy", platform: "115", taskID: "abc", want: "abc"},
		{name: "case insensitive platform", platform: "115", taskID: "115:abc", want: "abc"},
		{name: "different platform", platform: "115", taskID: "pikpak:abc", want: "pikpak:abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProviderTaskID(tt.platform, tt.taskID); got != tt.want {
				t.Fatalf("ProviderTaskID() = %q, want %q", got, tt.want)
			}
		})
	}
}
