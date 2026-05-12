package client

import (
	"errors"
	"net/http"
	"testing"
)

func TestParseAPIError_Envelope(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"not_found_error","message":"agent not found"}}`)
	err := parseAPIError(http.StatusNotFound, "req_123", body)

	if err.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", err.StatusCode)
	}
	if err.Type != "not_found_error" {
		t.Errorf("Type = %q, want not_found_error", err.Type)
	}
	if err.Message != "agent not found" {
		t.Errorf("Message = %q, want %q", err.Message, "agent not found")
	}
	if err.RequestID != "req_123" {
		t.Errorf("RequestID = %q, want req_123", err.RequestID)
	}
}

func TestParseAPIError_NonJSONBody(t *testing.T) {
	body := []byte("upstream went sideways")
	err := parseAPIError(http.StatusBadGateway, "", body)

	if err.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d", err.StatusCode)
	}
	if err.Message != "upstream went sideways" {
		t.Errorf("Message = %q, want fallback to raw body", err.Message)
	}
}

func TestIsNotFound(t *testing.T) {
	notFound := &APIError{StatusCode: http.StatusNotFound}
	other := &APIError{StatusCode: http.StatusInternalServerError}
	wrapped := errors.New("plain")

	if !IsNotFound(notFound) {
		t.Error("IsNotFound(404) = false")
	}
	if IsNotFound(other) {
		t.Error("IsNotFound(500) = true")
	}
	if IsNotFound(wrapped) {
		t.Error("IsNotFound(plain) = true")
	}
	if IsNotFound(nil) {
		t.Error("IsNotFound(nil) = true")
	}
}

func TestIsConflict(t *testing.T) {
	conflict := &APIError{StatusCode: http.StatusConflict}
	other := &APIError{StatusCode: http.StatusBadRequest}
	if !IsConflict(conflict) {
		t.Error("IsConflict(409) = false")
	}
	if IsConflict(other) {
		t.Error("IsConflict(400) = true")
	}
}

func TestAPIError_String(t *testing.T) {
	e := &APIError{
		StatusCode: 401,
		Type:       "authentication_error",
		Message:    "invalid api key",
		RequestID:  "req_abc",
	}
	got := e.Error()
	if got == "" {
		t.Fatal("Error() returned empty")
	}
	// Just verify all useful fields show up.
	for _, want := range []string{"401", "authentication_error", "invalid api key", "req_abc"} {
		if !contains(got, want) {
			t.Errorf("Error() = %q, missing %q", got, want)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && stringIndex(haystack, needle) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
