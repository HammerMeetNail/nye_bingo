package handlers

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPageHandler_IndexAndErrors(t *testing.T) {
	handler, err := NewPageHandler("../../web/templates", PageOAuthConfig{})
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
	_, err := NewPageHandler(filepath.Join(os.TempDir(), "nope"), PageOAuthConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPageHandler_Index_TemplateError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "404.html"), []byte("not found"), 0o644); err != nil {
		t.Fatalf("write 404: %v", err)
	}
	handler, err := NewPageHandler(dir, PageOAuthConfig{})
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

func TestResolveBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		host    string
		tls     bool
		headers map[string]string
		want    string
	}{
		{
			name: "direct request uses host + http",
			url:  "http://example.com/",
			want: "http://example.com",
		},
		{
			name: "tls sets https",
			url:  "http://example.com/",
			tls:  true,
			want: "https://example.com",
		},
		{
			name: "x-forwarded-proto overrides scheme",
			url:  "http://example.com/",
			headers: map[string]string{
				"X-Forwarded-Proto": "https",
			},
			want: "https://example.com",
		},
		{
			name: "x-forwarded-proto uses first value",
			url:  "http://example.com/",
			headers: map[string]string{
				"X-Forwarded-Proto": "https, http",
			},
			want: "https://example.com",
		},
		{
			name: "invalid x-forwarded-proto is ignored",
			url:  "http://example.com/",
			headers: map[string]string{
				"X-Forwarded-Proto": "ftp",
			},
			want: "http://example.com",
		},
		{
			name: "x-forwarded-host overrides host",
			url:  "http://example.com/",
			headers: map[string]string{
				"X-Forwarded-Host": "proxy.example.com",
			},
			want: "http://proxy.example.com",
		},
		{
			name: "x-forwarded-host uses first value",
			url:  "http://example.com/",
			headers: map[string]string{
				"X-Forwarded-Host": "proxy.example.com, evil.example.com",
			},
			want: "http://proxy.example.com",
		},
		{
			name: "malformed forwarded host is ignored",
			url:  "http://example.com/",
			headers: map[string]string{
				"X-Forwarded-Host": "evil.example.com/path",
			},
			want: "http://example.com",
		},
		{
			name: "host with port is preserved",
			url:  "http://localhost:8080/",
			want: "http://localhost:8080",
		},
		{
			name: "forwarded host with port is preserved",
			url:  "http://example.com/",
			headers: map[string]string{
				"X-Forwarded-Host": "example.com:8443",
			},
			want: "http://example.com:8443",
		},
		{
			name: "invalid host falls back to localhost",
			url:  "http://example.com/",
			host: "evil.example.com/path",
			want: "http://localhost",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			if tc.host != "" {
				req.Host = tc.host
			}
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			got := resolveBaseURL(req)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
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
