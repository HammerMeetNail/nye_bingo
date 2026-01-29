package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HammerMeetNail/yearofbingo/internal/httpx"
)

func TestTrustedProxyHeaders_TrustedRequest(t *testing.T) {
	// Create checker that trusts loopback addresses (default behavior)
	checker, err := httpx.NewTrustedProxyChecker(nil)
	if err != nil {
		t.Fatalf("NewTrustedProxyChecker() error = %v", err)
	}

	mw := NewTrustedProxyHeaders(checker)

	var wasTrusted bool
	handler := mw.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wasTrusted = httpx.TrustedForwardedHeaders(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345" // Loopback = trusted
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !wasTrusted {
		t.Error("Expected request from loopback to be marked as trusted")
	}
}

func TestTrustedProxyHeaders_UntrustedRequest(t *testing.T) {
	// Create checker with specific CIDR that won't match
	checker, err := httpx.NewTrustedProxyChecker([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("NewTrustedProxyChecker() error = %v", err)
	}

	mw := NewTrustedProxyHeaders(checker)

	var wasTrusted bool
	handler := mw.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wasTrusted = httpx.TrustedForwardedHeaders(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "8.8.8.8:12345" // Public IP = not trusted
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if wasTrusted {
		t.Error("Expected request from public IP to NOT be marked as trusted")
	}
}

func TestTrustedProxyHeaders_NilChecker(t *testing.T) {
	mw := NewTrustedProxyHeaders(nil)

	var wasTrusted bool
	handler := mw.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wasTrusted = httpx.TrustedForwardedHeaders(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if wasTrusted {
		t.Error("Expected nil checker to result in untrusted request")
	}
}

func TestTrustedProxyHeaders_ExplicitCIDRMatch(t *testing.T) {
	// Create checker with specific CIDR
	checker, err := httpx.NewTrustedProxyChecker([]string{"192.168.1.0/24"})
	if err != nil {
		t.Fatalf("NewTrustedProxyChecker() error = %v", err)
	}

	mw := NewTrustedProxyHeaders(checker)

	var wasTrusted bool
	handler := mw.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wasTrusted = httpx.TrustedForwardedHeaders(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:8080" // Matches CIDR
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !wasTrusted {
		t.Error("Expected request from matching CIDR to be marked as trusted")
	}
}

func TestTrustedProxyHeaders_PrivateIPDefaultTrusted(t *testing.T) {
	// Default behavior (no CIDRs) trusts private IPs
	checker, err := httpx.NewTrustedProxyChecker(nil)
	if err != nil {
		t.Fatalf("NewTrustedProxyChecker() error = %v", err)
	}

	mw := NewTrustedProxyHeaders(checker)

	tests := []struct {
		name       string
		remoteAddr string
		expected   bool
	}{
		{"Private 10.x", "10.0.0.1:8080", true},
		{"Private 172.16.x", "172.16.0.1:8080", true},
		{"Private 192.168.x", "192.168.1.1:8080", true},
		{"Loopback", "127.0.0.1:8080", true},
		{"Public IP", "8.8.8.8:8080", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wasTrusted bool
			handler := mw.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wasTrusted = httpx.TrustedForwardedHeaders(r)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if wasTrusted != tt.expected {
				t.Errorf("Expected trusted=%v for %s, got %v", tt.expected, tt.remoteAddr, wasTrusted)
			}
		})
	}
}

func TestTrustedProxyHeaders_RequestPassesThrough(t *testing.T) {
	checker, err := httpx.NewTrustedProxyChecker(nil)
	if err != nil {
		t.Fatalf("NewTrustedProxyChecker() error = %v", err)
	}

	mw := NewTrustedProxyHeaders(checker)

	called := false
	handler := mw.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("Expected next handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
