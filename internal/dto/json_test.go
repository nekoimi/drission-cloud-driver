package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserResponse_IDJSONIsString(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(UserResponse{
		ID:       "1234567890123456789",
		Username: "u",
		Email:    "e@e.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"id":"1234567890123456789"`) {
		t.Fatalf("marshal = %s, want id as JSON string", b)
	}
}

func TestUserInfo_IDJSONIsString(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(UserInfo{
		ID:       "1234567890123456789",
		Username: "u",
		Email:    "e@e.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"id":"1234567890123456789"`) {
		t.Fatalf("marshal = %s, want id as JSON string", b)
	}
}
