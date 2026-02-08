package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/HammerMeetNail/yearofbingo/internal/logging"
)

func TestLogErrorPreservesReservedErrorField(t *testing.T) {
	original := logging.Default
	buf := &bytes.Buffer{}
	logging.Default = logging.New().SetOutput(buf).SetLevel(logging.LevelDebug)
	t.Cleanup(func() {
		logging.Default = original
	})

	logError("boom happened", errors.New("boom"), map[string]interface{}{
		"error":   "override-attempt",
		"user_id": "u1",
	})

	var entry logging.LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v", err)
	}
	if entry.Fields["error"] != "boom" {
		t.Fatalf("expected reserved error field to match err.Error(), got %v", entry.Fields["error"])
	}
	if entry.Fields["user_id"] != "u1" {
		t.Fatalf("expected extra fields to be included, got %v", entry.Fields["user_id"])
	}
}

func TestLogErrorNoOutputWhenErrNil(t *testing.T) {
	original := logging.Default
	buf := &bytes.Buffer{}
	logging.Default = logging.New().SetOutput(buf).SetLevel(logging.LevelDebug)
	t.Cleanup(func() {
		logging.Default = original
	})

	logError("ignored", nil, map[string]interface{}{"key": "value"})

	if buf.Len() != 0 {
		t.Fatalf("expected no log output when err is nil")
	}
}
