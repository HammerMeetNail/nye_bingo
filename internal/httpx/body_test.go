package httpx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithBodyBytes(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{"Empty body", []byte{}},
		{"Simple body", []byte("hello world")},
		{"JSON body", []byte(`{"key": "value"}`)},
		{"Binary data", []byte{0x00, 0x01, 0x02, 0xFF}},
		{"Large body", bytes.Repeat([]byte("a"), 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req = WithBodyBytes(req, tt.body)

			got, ok := BodyBytes(req)
			if !ok {
				t.Fatal("BodyBytes() returned ok=false, want ok=true")
			}
			if !bytes.Equal(got, tt.body) {
				t.Errorf("BodyBytes() = %v, want %v", got, tt.body)
			}
		})
	}
}

func TestBodyBytes_NotSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	got, ok := BodyBytes(req)
	if ok {
		t.Error("BodyBytes() returned ok=true on fresh request, want ok=false")
	}
	if got != nil {
		t.Errorf("BodyBytes() = %v, want nil", got)
	}
}

func TestBodyBytes_NilBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = WithBodyBytes(req, nil)

	got, ok := BodyBytes(req)
	if !ok {
		t.Fatal("BodyBytes() returned ok=false, want ok=true (nil is a valid []byte)")
	}
	if got != nil {
		t.Errorf("BodyBytes() = %v, want nil", got)
	}
}

func TestWithBodyBytes_PreservesOtherContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	// Set trusted headers first
	req = WithTrustedForwardedHeaders(req, true)

	// Then set body bytes
	body := []byte("test body")
	req = WithBodyBytes(req, body)

	// Both should be retrievable
	if !TrustedForwardedHeaders(req) {
		t.Error("TrustedForwardedHeaders() = false after WithBodyBytes, want true")
	}

	got, ok := BodyBytes(req)
	if !ok {
		t.Fatal("BodyBytes() returned ok=false, want ok=true")
	}
	if !bytes.Equal(got, body) {
		t.Errorf("BodyBytes() = %v, want %v", got, body)
	}
}
