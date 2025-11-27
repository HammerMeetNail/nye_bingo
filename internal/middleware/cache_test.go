package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCacheControl_APIEndpoints(t *testing.T) {
	cache := NewCacheControl()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	apiPaths := []string{
		"/api/users",
		"/api/cards",
		"/api/auth/me",
		"/api/friends/123",
	}

	for _, path := range apiPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()

			cache.Apply(handler).ServeHTTP(rr, req)

			cacheControl := rr.Header().Get("Cache-Control")
			if cacheControl != "no-store, no-cache, must-revalidate" {
				t.Errorf("API path %s: expected no-store cache, got %q", path, cacheControl)
			}

			pragma := rr.Header().Get("Pragma")
			if pragma != "no-cache" {
				t.Errorf("API path %s: expected Pragma: no-cache, got %q", path, pragma)
			}
		})
	}
}

func TestCacheControl_StaticAssets(t *testing.T) {
	cache := NewCacheControl()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		path          string
		expectedCache string
		isImmutable   bool
	}{
		{"/static/js/app.js", "public, max-age=86400, must-revalidate", false},
		{"/static/css/styles.css", "public, max-age=86400, must-revalidate", false},
		{"/static/fonts/font.woff2", "public, max-age=31536000, immutable", true},
		{"/static/images/logo.png", "public, max-age=31536000, immutable", true},
		{"/static/images/icon.jpg", "public, max-age=31536000, immutable", true},
		{"/static/fonts/font.woff", "public, max-age=31536000, immutable", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			cache.Apply(handler).ServeHTTP(rr, req)

			cacheControl := rr.Header().Get("Cache-Control")
			if cacheControl != tt.expectedCache {
				t.Errorf("path %s: expected %q, got %q", tt.path, tt.expectedCache, cacheControl)
			}
		})
	}
}

func TestCacheControl_RootPage(t *testing.T) {
	cache := NewCacheControl()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("path_/", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		cache.Apply(handler).ServeHTTP(rr, req)

		cacheControl := rr.Header().Get("Cache-Control")
		if cacheControl != "no-cache, must-revalidate" {
			t.Errorf("root path: expected no-cache, must-revalidate, got %q", cacheControl)
		}
	})

	t.Run("path_empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = ""
		rr := httptest.NewRecorder()

		cache.Apply(handler).ServeHTTP(rr, req)

		cacheControl := rr.Header().Get("Cache-Control")
		if cacheControl != "no-cache, must-revalidate" {
			t.Errorf("empty path: expected no-cache, must-revalidate, got %q", cacheControl)
		}
	})
}

func TestCacheControl_DefaultPath(t *testing.T) {
	cache := NewCacheControl()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Paths that don't match other patterns
	paths := []string{
		"/unknown",
		"/some/random/path",
		"/favicon.ico",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()

			cache.Apply(handler).ServeHTTP(rr, req)

			cacheControl := rr.Header().Get("Cache-Control")
			if cacheControl != "no-store" {
				t.Errorf("default path %s: expected no-store, got %q", path, cacheControl)
			}
		})
	}
}

func TestCacheControl_HandlerCalled(t *testing.T) {
	cache := NewCacheControl()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()

	cache.Apply(handler).ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called")
	}
}

func TestCacheControl_AllMethods(t *testing.T) {
	cache := NewCacheControl()
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(method, "/api/test", nil)
			rr := httptest.NewRecorder()

			cache.Apply(handler).ServeHTTP(rr, req)

			// Cache headers should be set regardless of method
			if got := rr.Header().Get("Cache-Control"); got == "" {
				t.Errorf("Cache-Control not set for %s", method)
			}
		})
	}
}

func TestIsImmutableAsset(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// Fonts
		{"/static/fonts/font.woff", true},
		{"/static/fonts/font.woff2", true},
		{"/static/fonts/font.ttf", true},
		{"/static/fonts/font.otf", true},
		{"/static/fonts/font.eot", true},
		// Images
		{"/static/images/photo.jpg", true},
		{"/static/images/photo.jpeg", true},
		{"/static/images/icon.png", true},
		{"/static/images/logo.gif", true},
		{"/static/images/hero.webp", true},
		{"/static/images/favicon.ico", true},
		{"/static/images/logo.svg", true},
		// Media
		{"/static/video.mp4", true},
		{"/static/video.webm", true},
		{"/static/audio.mp3", true},
		{"/static/audio.ogg", true},
		// Non-immutable
		{"/static/js/app.js", false},
		{"/static/css/styles.css", false},
		{"/static/data.json", false},
		{"/api/users", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isImmutableAsset(tt.path)
			if got != tt.expected {
				t.Errorf("isImmutableAsset(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestSetStaticCacheHeaders(t *testing.T) {
	cache := NewCacheControl()

	tests := []struct {
		path     string
		expected string
	}{
		{"/static/js/app.js", "public, max-age=86400, must-revalidate"},
		{"/static/css/styles.css", "public, max-age=86400, must-revalidate"},
		{"/static/fonts/font.woff2", "public, max-age=31536000, immutable"},
		{"/static/images/logo.png", "public, max-age=31536000, immutable"},
		{"/static/unknown.txt", "public, max-age=3600"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			cache.setStaticCacheHeaders(w, tt.path)

			got := w.Header().Get("Cache-Control")
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
