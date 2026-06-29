package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP_TrustedWithXForwardedFor(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		remoteAddr string
		expected   string
	}{
		{"Single IP", "203.0.113.1", "10.0.0.1:8080", "203.0.113.1"},
		// Right-most entry is the one appended by the nearest trusted proxy.
		{"Multiple IPs", "10.0.0.2, 10.0.0.3, 203.0.113.1", "10.0.0.1:8080", "203.0.113.1"},
		{"IP with spaces", "  203.0.113.1  ", "10.0.0.1:8080", "203.0.113.1"},
		{"Multiple IPs with spaces", "  10.0.0.2  ,  203.0.113.1  ", "10.0.0.1:8080", "203.0.113.1"},
		// A client-supplied (spoofed) left-most entry must be ignored.
		{"Spoofed left-most ignored", "1.2.3.4, 203.0.113.1", "10.0.0.1:8080", "203.0.113.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Forwarded-For", tt.xff)
			req = WithTrustedForwardedHeaders(req, true)

			if got := ClientIP(req); got != tt.expected {
				t.Errorf("ClientIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClientIP_TrustedWithXRealIP(t *testing.T) {
	tests := []struct {
		name       string
		xri        string
		remoteAddr string
		expected   string
	}{
		{"Single IP", "203.0.113.1", "10.0.0.1:8080", "203.0.113.1"},
		{"IP with spaces", "  203.0.113.1  ", "10.0.0.1:8080", "203.0.113.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set("X-Real-IP", tt.xri)
			req = WithTrustedForwardedHeaders(req, true)

			if got := ClientIP(req); got != tt.expected {
				t.Errorf("ClientIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClientIP_TrustedPrefersXForwardedFor(t *testing.T) {
	// When both headers are present, X-Forwarded-For takes precedence
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "203.0.113.2")
	req = WithTrustedForwardedHeaders(req, true)

	if got := ClientIP(req); got != "203.0.113.1" {
		t.Errorf("ClientIP() = %q, want %q (X-Forwarded-For should take precedence)", got, "203.0.113.1")
	}
}

func TestClientIP_TrustedPrefersCFConnectingIP(t *testing.T) {
	// CF-Connecting-IP wins over X-Forwarded-For and X-Real-IP.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	req.Header.Set("CF-Connecting-IP", "203.0.113.9")
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.1")
	req.Header.Set("X-Real-IP", "203.0.113.2")
	req = WithTrustedForwardedHeaders(req, true)

	if got := ClientIP(req); got != "203.0.113.9" {
		t.Errorf("ClientIP() = %q, want %q (CF-Connecting-IP should win)", got, "203.0.113.9")
	}
}

func TestClientIP_UntrustedIgnoresCFConnectingIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	req.Header.Set("CF-Connecting-IP", "203.0.113.9")
	// Not trusted: header must be ignored.

	if got := ClientIP(req); got != "10.0.0.1" {
		t.Errorf("ClientIP() = %q, want %q (CF-Connecting-IP must be ignored when untrusted)", got, "10.0.0.1")
	}
}

func TestClientIP_TrustedNoHeaders(t *testing.T) {
	// Trusted request but no forwarding headers; should fall back to RemoteAddr
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	req = WithTrustedForwardedHeaders(req, true)

	if got := ClientIP(req); got != "10.0.0.1" {
		t.Errorf("ClientIP() = %q, want %q", got, "10.0.0.1")
	}
}

func TestClientIP_Untrusted(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xri        string
		remoteAddr string
		expected   string
	}{
		{"Ignores X-Forwarded-For", "203.0.113.1", "", "10.0.0.1:8080", "10.0.0.1"},
		{"Ignores X-Real-IP", "", "203.0.113.2", "10.0.0.1:8080", "10.0.0.1"},
		{"Ignores both headers", "203.0.113.1", "203.0.113.2", "10.0.0.1:8080", "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			// Not setting trusted headers (defaults to untrusted)

			if got := ClientIP(req); got != tt.expected {
				t.Errorf("ClientIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClientIP_RemoteAddrParsing(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{"IPv4 with port", "192.168.1.1:8080", "192.168.1.1"},
		{"IPv4 without port", "192.168.1.1", "192.168.1.1"},
		{"IPv6 with port", "[2001:db8::1]:443", "2001:db8::1"},
		{"IPv6 without port", "2001:db8::1", "2001:db8::1"},
		{"Loopback with port", "127.0.0.1:12345", "127.0.0.1"},
		{"IPv6 loopback with port", "[::1]:8080", "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			// Untrusted request, so it should use RemoteAddr

			if got := ClientIP(req); got != tt.expected {
				t.Errorf("ClientIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClientIP_ExplicitlyUntrusted(t *testing.T) {
	// Explicitly marked as untrusted
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req = WithTrustedForwardedHeaders(req, false)

	if got := ClientIP(req); got != "10.0.0.1" {
		t.Errorf("ClientIP() = %q, want %q (should ignore X-Forwarded-For when untrusted)", got, "10.0.0.1")
	}
}
