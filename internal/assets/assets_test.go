package assets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestLoadAndGet(t *testing.T) {
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, "web", "static", "dist")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}

	manifestPath := filepath.Join(manifestDir, "manifest.json")
	content := map[string]string{
		"css/styles.css": "css/styles.abcd1234.css",
		"js/app.js":      "js/app.9876.js",
	}
	data, _ := json.Marshal(content)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	m := NewManifest(dir)
	if err := m.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if got := m.GetCSS(); got != "/static/"+content["css/styles.css"] {
		t.Fatalf("unexpected css path: %s", got)
	}
	if got := m.GetAppJS(); got != "/static/"+content["js/app.js"] {
		t.Fatalf("unexpected app js path: %s", got)
	}
	if got := m.Get("missing.js"); got != "/static/missing.js" {
		t.Fatalf("expected fallback path, got %s", got)
	}
}

func TestManifestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest(dir)
	if err := m.Load(); err != nil {
		t.Fatalf("expected missing manifest to be handled, got %v", err)
	}

	if got := m.GetAPIJS(); got != "/static/js/api.js" {
		t.Fatalf("expected fallback path for api.js, got %s", got)
	}
}
