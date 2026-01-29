package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBodySize_AllowsSmallBody(t *testing.T) {
	m := NewMaxBodySize(10)
	h := m.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		_, _ = w.Write([]byte(string(b)))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/anything", bytes.NewBufferString("12345"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "12345" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestMaxBodySize_RejectsLargeBody(t *testing.T) {
	m := NewMaxBodySize(5)
	h := m.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run for oversized body")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/anything", bytes.NewBufferString("123456"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Request body too large") {
		t.Fatalf("expected error message, got %q", rr.Body.String())
	}
}

func TestMaxBodySize_DoesNotApplyOutsideAPI(t *testing.T) {
	m := NewMaxBodySize(5)
	called := false
	h := m.Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/r/unsubscribe", bytes.NewBufferString("123456"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected handler to run")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
