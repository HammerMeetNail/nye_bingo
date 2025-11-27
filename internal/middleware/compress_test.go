package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompress_GzipWhenAccepted(t *testing.T) {
	compress := NewCompress()

	responseBody := "This is a test response that should be compressed"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(responseBody))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	compress.Apply(handler).ServeHTTP(rr, req)

	// Check Content-Encoding header
	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("expected Content-Encoding: gzip, got %q", got)
	}

	// Check Vary header
	if got := rr.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("expected Vary: Accept-Encoding, got %q", got)
	}

	// Decompress and verify content
	gzReader, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer func() { _ = gzReader.Close() }()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	if string(decompressed) != responseBody {
		t.Errorf("expected %q, got %q", responseBody, string(decompressed))
	}
}

func TestCompress_NoGzipWhenNotAccepted(t *testing.T) {
	compress := NewCompress()

	responseBody := "This is a test response"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(responseBody))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	// No Accept-Encoding header

	rr := httptest.NewRecorder()
	compress.Apply(handler).ServeHTTP(rr, req)

	// Check no Content-Encoding header
	if got := rr.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("expected no Content-Encoding, got %q", got)
	}

	// Body should be uncompressed
	if got := rr.Body.String(); got != responseBody {
		t.Errorf("expected %q, got %q", responseBody, got)
	}
}

func TestCompress_SkipPreCompressedFiles(t *testing.T) {
	compress := NewCompress()

	preCompressedPaths := []string{
		"/static/image.jpg",
		"/static/image.jpeg",
		"/static/image.png",
		"/static/image.gif",
		"/static/image.webp",
		"/static/icon.ico",
		"/static/video.mp4",
		"/static/video.webm",
		"/static/audio.mp3",
		"/static/audio.ogg",
		"/static/archive.zip",
		"/static/archive.gz",
		"/static/archive.br",
		"/static/archive.zst",
		"/static/font.woff",
		"/static/font.woff2",
	}

	for _, path := range preCompressedPaths {
		t.Run(path, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("content"))
			})

			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept-Encoding", "gzip")

			rr := httptest.NewRecorder()
			compress.Apply(handler).ServeHTTP(rr, req)

			// Should not be compressed
			if got := rr.Header().Get("Content-Encoding"); got != "" {
				t.Errorf("path %s should not be compressed, got Content-Encoding: %q", path, got)
			}
		})
	}
}

func TestCompress_CompressTextPaths(t *testing.T) {
	compress := NewCompress()

	textPaths := []string{
		"/api/test",
		"/static/script.js",
		"/static/style.css",
		"/",
		"/about",
	}

	for _, path := range textPaths {
		t.Run(path, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("content"))
			})

			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept-Encoding", "gzip")

			rr := httptest.NewRecorder()
			compress.Apply(handler).ServeHTTP(rr, req)

			// Should be compressed
			if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
				t.Errorf("path %s should be compressed, got Content-Encoding: %q", path, got)
			}
		})
	}
}

func TestCompress_CaseInsensitiveExtension(t *testing.T) {
	compress := NewCompress()

	// Test uppercase extensions
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	})

	uppercasePaths := []string{
		"/static/image.JPG",
		"/static/image.PNG",
		"/static/font.WOFF2",
	}

	for _, path := range uppercasePaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept-Encoding", "gzip")

			rr := httptest.NewRecorder()
			compress.Apply(handler).ServeHTTP(rr, req)

			// Should not be compressed (case insensitive check)
			if got := rr.Header().Get("Content-Encoding"); got != "" {
				t.Errorf("path %s should not be compressed, got Content-Encoding: %q", path, got)
			}
		})
	}
}

func TestIsPreCompressedPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/static/image.jpg", true},
		{"/static/image.JPEG", true},
		{"/static/font.woff", true},
		{"/static/font.woff2", true},
		{"/static/script.js", false},
		{"/static/style.css", false},
		{"/api/users", false},
		{"/", false},
		{"/static/image.jpgx", false}, // Not a real extension
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isPreCompressedPath(tt.path)
			if got != tt.expected {
				t.Errorf("isPreCompressedPath(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestCompress_GzipDeflateAccepted(t *testing.T) {
	compress := NewCompress()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	rr := httptest.NewRecorder()
	compress.Apply(handler).ServeHTTP(rr, req)

	// Should still use gzip
	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("expected Content-Encoding: gzip, got %q", got)
	}
}

func TestCompress_VaryHeader(t *testing.T) {
	compress := NewCompress()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("test content"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	compress.Apply(handler).ServeHTTP(rr, req)

	// Vary header should be set
	if got := rr.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("expected Vary: Accept-Encoding, got %q", got)
	}
}
