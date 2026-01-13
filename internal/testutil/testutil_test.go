package testutil

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestNewTestRequestWithJSON(t *testing.T) {
	req := NewTestRequestWithJSON(t, http.MethodPost, "/path", map[string]string{"ok": "yes"})
	if ct := req.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected content type json, got %q", ct)
	}
}

func TestParseJSONResponse(t *testing.T) {
	body := []byte(`{"ok":true}`)
	got := ParseJSONResponse(t, body)
	if got["ok"] != true {
		t.Fatalf("expected ok=true, got %v", got["ok"])
	}
}

func TestNewTestRequest(t *testing.T) {
	req := NewTestRequest(http.MethodPost, "/path", bytes.NewBufferString("{}"))
	if req.Method != http.MethodPost {
		t.Fatalf("expected method POST, got %s", req.Method)
	}
}

func TestAssertStatusCode(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteHeader(http.StatusCreated)
	AssertStatusCode(t, rr, http.StatusCreated)
}

func TestAssertJSONContains(t *testing.T) {
	body := []byte(`{"ok":"yes"}`)
	AssertJSONContains(t, body, "ok", "yes")
}

func TestRandomHelpers(t *testing.T) {
	if RandomUUID() == uuid.Nil {
		t.Fatal("expected non-nil uuid")
	}
	if RandomEmail() == "" {
		t.Fatal("expected email")
	}
}

func TestAssertHelpers_RunWithoutErrors(t *testing.T) {
	AssertEqual(t, 1, 1, "equal")
	AssertNotEqual(t, 1, 2, "not equal")
	AssertNil(t, nil, "nil")
	AssertNotNil(t, 1, "not nil")
	AssertTrue(t, true, "true")
	AssertFalse(t, false, "false")
	AssertNoError(t, nil, "no error")
	AssertError(t, errors.New("boom"), "has error")
	AssertContains(t, "hello world", "world", "contains")
}
