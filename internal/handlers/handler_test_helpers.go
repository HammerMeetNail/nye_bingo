package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func assertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if rr.Code != status {
		t.Fatalf("expected status %d, got %d", status, rr.Code)
	}
	if ct := rr.Result().Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected content type application/json, got %q", ct)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response.Error != message {
		t.Fatalf("expected error %q, got %q", message, response.Error)
	}
}
