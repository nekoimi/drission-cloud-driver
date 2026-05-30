package v1

import "testing"

func TestParseMkdirRequestWithPath(t *testing.T) {
	parentPath, name, err := parseMkdirRequest("/get-magnet/JavDB/2026-05-30", "", "")
	if err != nil {
		t.Fatalf("parseMkdirRequest() error = %v", err)
	}
	if parentPath != "/get-magnet/JavDB" {
		t.Fatalf("parentPath = %q, want %q", parentPath, "/get-magnet/JavDB")
	}
	if name != "2026-05-30" {
		t.Fatalf("name = %q, want %q", name, "2026-05-30")
	}
}

func TestParseMkdirRequestWithParentAndName(t *testing.T) {
	parentPath, name, err := parseMkdirRequest("", "/get-magnet/JavDB", "2026-05-30")
	if err != nil {
		t.Fatalf("parseMkdirRequest() error = %v", err)
	}
	if parentPath != "/get-magnet/JavDB" {
		t.Fatalf("parentPath = %q, want %q", parentPath, "/get-magnet/JavDB")
	}
	if name != "2026-05-30" {
		t.Fatalf("name = %q, want %q", name, "2026-05-30")
	}
}

func TestParseMkdirRequestRejectsRootPath(t *testing.T) {
	if _, _, err := parseMkdirRequest("/", "", ""); err == nil {
		t.Fatalf("parseMkdirRequest() error = nil, want error")
	}
}

func TestParseMkdirRequestRequiresNameForLegacyFormat(t *testing.T) {
	if _, _, err := parseMkdirRequest("", "/get-magnet", ""); err == nil {
		t.Fatalf("parseMkdirRequest() error = nil, want error")
	}
}
