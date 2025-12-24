package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLoggerLevelsAndFields(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New().SetOutput(buf).SetLevel(LevelDebug)

	logger.WithField("req", "123").Info("hello")
	if buf.Len() == 0 {
		t.Fatalf("expected log output")
	}

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}
	if entry.Level != "INFO" || entry.Message != "hello" {
		t.Fatalf("unexpected log entry: %+v", entry)
	}
	if entry.Fields["req"] != "123" {
		t.Fatalf("expected field to propagate")
	}
}

func TestLoggerFiltersByLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := New().SetOutput(buf).SetLevel(LevelError)

	logger.Info("ignored")
	if buf.Len() != 0 {
		t.Fatalf("expected info to be filtered")
	}

	logger.Error("visible", map[string]interface{}{"k": "v"})
	if !strings.Contains(buf.String(), "visible") {
		t.Fatalf("expected error log to be written")
	}
}

func TestDefaultLoggerHelpers(t *testing.T) {
	buf := &bytes.Buffer{}
	Default = New().SetOutput(buf).SetLevel(LevelDebug)

	Info("info")
	Warn("warn")
	Error("error")

	output := buf.String()
	if !strings.Contains(output, "info") || !strings.Contains(output, "warn") || !strings.Contains(output, "error") {
		t.Fatalf("expected default logger helper output, got %s", output)
	}
}
