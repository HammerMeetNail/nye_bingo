package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
)

func TestRequestLogger_LogsErrorWithQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New().SetOutput(&buf).SetLevel(logging.LevelDebug)

	rl := NewRequestLogger(logger)
	handler := rl.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry logging.LogEntry
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}
	if entry.Level != logging.LevelError.String() {
		t.Fatalf("expected ERROR level, got %s", entry.Level)
	}
	if entry.Fields["query"] != "foo=bar" {
		t.Fatalf("expected query field, got %v", entry.Fields["query"])
	}
}

func TestRequestLogger_LogsWarnWithoutQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New().SetOutput(&buf).SetLevel(logging.LevelDebug)

	rl := NewRequestLogger(logger)
	handler := rl.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry logging.LogEntry
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}
	if entry.Level != logging.LevelWarn.String() {
		t.Fatalf("expected WARN level, got %s", entry.Level)
	}
	if _, ok := entry.Fields["query"]; ok {
		t.Fatal("did not expect query field for empty query string")
	}
}
