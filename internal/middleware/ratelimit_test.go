package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Middleware_NilRedis(t *testing.T) {
	// Test that middleware fails open (allows request) when Redis is nil
	limiter := NewRateLimiter(nil, 10, time.Hour, "test:", func(r *http.Request) string {
		return "test-key"
	})

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", rr.Code)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name:     "X-Forwarded-For Single",
			headers:  map[string]string{"X-Forwarded-For": "10.0.0.1"},
			remote:   "192.168.1.1:1234",
			expected: "10.0.0.1",
		},
		{
			name:     "X-Forwarded-For Multiple",
			headers:  map[string]string{"X-Forwarded-For": "10.0.0.1, 10.0.0.2"},
			remote:   "192.168.1.1:1234",
			expected: "10.0.0.1",
		},
		{
			name:     "X-Real-IP",
			headers:  map[string]string{"X-Real-IP": "10.0.0.2"},
			remote:   "192.168.1.1:1234",
			expected: "10.0.0.2",
		},
		{
			name:     "XFF Preference over X-Real-IP",
			headers:  map[string]string{"X-Forwarded-For": "10.0.0.1", "X-Real-IP": "10.0.0.2"},
			remote:   "192.168.1.1:1234",
			expected: "10.0.0.1",
		},
		{
			name:     "No Headers",
			headers:  map[string]string{},
			remote:   "192.168.1.1:1234",
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remote

			ip := GetClientIP(req)
			if ip != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, ip)
			}
		})
	}
}

// Note: Full integration testing of RateLimiter requires a running Redis instance
// or a mock that implements the go-redis interface, which is not trivial without
// external libraries like redismock.
