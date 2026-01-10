package handlers

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOGImageHandler_Default(t *testing.T) {
	h := NewOGImageHandler()

	req := httptest.NewRequest(http.MethodGet, "/og/default.png", nil)
	rr := httptest.NewRecorder()

	h.Default(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected content-type image/png, got %q", ct)
	}
	if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "public") {
		t.Fatalf("expected cache-control to be public, got %q", cc)
	}

	if _, err := png.Decode(bytes.NewReader(rr.Body.Bytes())); err != nil {
		t.Fatalf("expected response body to be a valid PNG: %v", err)
	}
}

