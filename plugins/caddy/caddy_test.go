package caddy

import (
	"net/http"
	"testing"
)

func TestParseSameSite(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"lax", "Lax"},
		{"strict", "Strict"},
		{"none", "None"},
		{"default", "Default"},
		{"", "None"},
		{"LAX", "Lax"},
		{"  strict  ", "Strict"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := parseSameSite(tc.input)
			if result.String() != tc.expected {
				t.Errorf("parseSameSite(%q) = %q, want %q", tc.input, result.String(), tc.expected)
			}
		})
	}
}

func TestKeyFromHex(t *testing.T) {
	// Valid 32-byte hex seed
	validHex := "0000000000000000000000000000000000000000000000000000000000000000"
	priv, err := keyFromHex(validHex)
	if err != nil {
		t.Fatalf("keyFromHex(valid) returned error: %v", err)
	}
	if priv == nil {
		t.Fatal("keyFromHex(valid) returned nil key")
	}

	// Invalid hex
	_, err = keyFromHex("zzz")
	if err == nil {
		t.Fatal("keyFromHex(invalid hex) should return error")
	}

	// Wrong length
	_, err = keyFromHex("00")
	if err == nil {
		t.Fatal("keyFromHex(too short) should return error")
	}
}

func TestContextualNext(t *testing.T) {
	cn := contextualNext{}
	// contextualNext with no next handler in context should not panic
	req, _ := http.NewRequest("GET", "/", nil)
	cn.ServeHTTP(nil, req)
}
