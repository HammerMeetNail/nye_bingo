package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPageHandler_IndexAndErrors(t *testing.T) {
	handler, err := NewPageHandler("../../web/templates")
	if err != nil {
		t.Fatalf("failed to create page handler: %v", err)
	}

	t.Run("index", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handler.Index(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
		if ct := rr.Header().Get("Content-Type"); ct == "" {
			t.Fatalf("expected content-type to be set")
		}
		body := rr.Body.String()
		if body == "" {
			t.Fatalf("expected response body to be set")
		}
		if !containsAll(body, []string{`property="og:image"`, `/og/default.png`, `name="twitter:card"`}) {
			t.Fatalf("expected OpenGraph/Twitter meta tags to be present")
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nope", nil)
		rr := httptest.NewRecorder()

		handler.NotFound(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rr.Code)
		}
	})

	t.Run("internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/err", nil)
		rr := httptest.NewRecorder()

		handler.InternalError(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rr.Code)
		}
	})
}

func TestPageHandler_NewPageHandler_InvalidDir(t *testing.T) {
	_, err := NewPageHandler(filepath.Join(os.TempDir(), "nope"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPageHandler_Index_TemplateError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "404.html"), []byte("not found"), 0o644); err != nil {
		t.Fatalf("write 404: %v", err)
	}
	handler, err := NewPageHandler(dir)
	if err != nil {
		t.Fatalf("failed to create page handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.Index(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func containsAll(s string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(s, needle) {
			return false
		}
	}
	return true
}
