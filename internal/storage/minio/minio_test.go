package minio

import "testing"

func TestSanitizeMinIOFolder(t *testing.T) {
	got, err := sanitizeMinIOFolder("uploads/docs")
	if err != nil || got != "uploads/docs" {
		t.Fatalf("got %q err %v", got, err)
	}
	_, err = sanitizeMinIOFolder("uploads/../secret")
	if err == nil {
		t.Fatal("expected error for traversal")
	}
}
