package jwtutil

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateToken_valid(t *testing.T) {
	secret := "test-secret"
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "42",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	sub, err := ValidateToken(tok, secret)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if sub != "42" {
		t.Fatalf("sub = %q, want 42", sub)
	}
}

func TestValidateToken_expired(t *testing.T) {
	secret := "test-secret"
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "42",
		"exp": time.Now().Add(-time.Hour).Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateToken(tok, secret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_wrongSecret(t *testing.T) {
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "42",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateToken(tok, "b")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidateToken_invalidSub(t *testing.T) {
	secret := "test-secret"
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": float64(42),
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ValidateToken(tok, secret)
	if err == nil {
		t.Fatal("expected error for non-string sub")
	}
}
