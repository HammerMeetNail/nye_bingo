package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HammerMeetNail/yearofbingo/internal/httpx"
)

func TestRateLimitEmailKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"TEST@Example.COM"}`))
	req = httpx.WithBodyBytes(req, []byte(`{"email":"TEST@Example.COM"}`))

	if got := RateLimitEmailKey(req); got != "test@example.com" {
		t.Fatalf("expected normalized email, got %q", got)
	}
}
