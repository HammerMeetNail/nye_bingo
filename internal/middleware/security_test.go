package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders_Apply(t *testing.T) {
	tests := []struct {
		name     string
		secure   bool
		expected map[string]string
	}{
		{
			name:   "non-secure mode",
			secure: false,
			expected: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
				"X-XSS-Protection":       "1; mode=block",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
			},
		},
		{
			name:   "secure mode",
			secure: true,
			expected: map[string]string{
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff",
				"X-XSS-Protection":          "1; mode=block",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Permissions-Policy":        "geolocation=(), microphone=(), camera=()",
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sec := NewSecurityHeaders(tt.secure)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			sec.Apply(handler).ServeHTTP(rr, req)

			for header, expected := range tt.expected {
				got := rr.Header().Get(header)
				if got != expected {
					t.Errorf("header %s: expected %q, got %q", header, expected, got)
				}
			}
		})
	}
}

func TestSecurityHeaders_NoHSTSInNonSecureMode(t *testing.T) {
	sec := NewSecurityHeaders(false)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	sec.Apply(handler).ServeHTTP(rr, req)

	if hsts := rr.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Errorf("HSTS should not be set in non-secure mode, got %q", hsts)
	}
}

func TestSecurityHeaders_CSP(t *testing.T) {
	sec := NewSecurityHeaders(false)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	sec.Apply(handler).ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("CSP header not set")
	}

	// Check required directives
	requiredDirectives := []string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"font-src 'self'",
		"img-src 'self'",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing directive: %s", directive)
		}
	}

	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Error("CSP script-src should not include unsafe-inline")
	}
	if strings.Contains(csp, "unsafe-hashes") {
		t.Error("CSP should not include unsafe-hashes")
	}
}

func TestSecurityHeaders_HandlerCalled(t *testing.T) {
	sec := NewSecurityHeaders(false)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	sec.Apply(handler).ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called")
	}
}

func TestSecurityHeaders_AllMethods(t *testing.T) {
	sec := NewSecurityHeaders(false)
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()

			sec.Apply(handler).ServeHTTP(rr, req)

			// Check that headers are set regardless of method
			if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
				t.Errorf("X-Frame-Options not set for %s", method)
			}
		})
	}
}
