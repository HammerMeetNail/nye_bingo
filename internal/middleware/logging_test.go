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

	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar&token=secret123&code=oauthcode&state=oauthstate", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry logging.LogEntry
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}
	if entry.Level != logging.LevelError.String() {
		t.Fatalf("expected ERROR level, got %s", entry.Level)
	}
	if _, ok := entry.Fields["query"]; ok {
		t.Fatalf("did not expect raw query string to be logged, got %v", entry.Fields["query"])
	}
	if v, ok := entry.Fields["query_present"]; !ok || v != true {
		t.Fatalf("expected query_present=true, got %v", entry.Fields["query_present"])
	}
	logLine := buf.String()
	for _, secret := range []string{"foo=bar", "secret123", "oauthcode", "oauthstate"} {
		if bytes.Contains([]byte(logLine), []byte(secret)) {
			t.Fatalf("log output must not contain query values; found %q in %q", secret, logLine)
		}
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
	if _, ok := entry.Fields["query_present"]; ok {
		t.Fatal("did not expect query_present field for empty query string")
	}
}
