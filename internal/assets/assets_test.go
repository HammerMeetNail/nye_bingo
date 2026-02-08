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
		"css/styles.css":          "css/styles.abcd1234.css",
		"js/app.js":               "js/app.9876.js",
		"js/app-core.js":          "js/app-core.1111.js",
		"js/app-actions.js":       "js/app-actions.2222.js",
		"js/app-modals.js":        "js/app-modals.3333.js",
		"js/app-notifications.js": "js/app-notifications.4444.js",
		"js/app-reminders.js":     "js/app-reminders.5555.js",
		"js/app-friends.js":       "js/app-friends.6666.js",
		"js/app-billing.js":       "js/app-billing.7777.js",
		"js/app-templates.js":     "js/app-templates.8888.js",
		"js/app-ai.js":            "js/app-ai.9999.js",
		"js/app-auth.js":          "js/app-auth.aaaa.js",
		"js/app-cards.js":         "js/app-cards.bbbb.js",
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
	if got := m.GetAppAuthJS(); got != "/static/"+content["js/app-auth.js"] {
		t.Fatalf("unexpected app auth js path: %s", got)
	}
	modulePaths := m.GetAppModuleJSPaths()
	if len(modulePaths) != 11 {
		t.Fatalf("expected 11 app module paths, got %d", len(modulePaths))
	}
	if modulePaths[0] != "/static/"+content["js/app-core.js"] {
		t.Fatalf("unexpected first module path: %s", modulePaths[0])
	}
	if modulePaths[len(modulePaths)-1] != "/static/"+content["js/app-cards.js"] {
		t.Fatalf("unexpected last module path: %s", modulePaths[len(modulePaths)-1])
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
	modulePaths := m.GetAppModuleJSPaths()
	if len(modulePaths) != 11 {
		t.Fatalf("expected fallback module paths, got %d", len(modulePaths))
	}
	if modulePaths[0] != "/static/js/app-core.js" {
		t.Fatalf("expected fallback app-core path, got %s", modulePaths[0])
	}
	if modulePaths[len(modulePaths)-1] != "/static/js/app-cards.js" {
		t.Fatalf("expected fallback app-cards path, got %s", modulePaths[len(modulePaths)-1])
	}
}

func TestManifestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, "web", "static", "dist")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("failed to create manifest dir: %v", err)
	}

	manifestPath := filepath.Join(manifestDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	m := NewManifest(dir)
	if err := m.Load(); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
