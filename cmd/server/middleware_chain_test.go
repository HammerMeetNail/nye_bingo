package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestBuildMiddlewareChain_Order(t *testing.T) {
	order := make([]string, 0, 9)
	wrap := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	base := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "handler")
	})

	handler := buildMiddlewareChain(base, &middlewareChain{
		authenticate:             wrap("authenticate"),
		applyMaxBodySize:         wrap("max-body"),
		protectCSRF:              wrap("csrf"),
		applyCacheControl:        wrap("cache"),
		applyCompress:            wrap("compress"),
		applySecurityHeaders:     wrap("security"),
		applyRequestLogger:       wrap("request-logger"),
		applyTrustedProxyHeaders: wrap("trusted-proxy"),
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	want := []string{
		"trusted-proxy",
		"request-logger",
		"security",
		"compress",
		"cache",
		"csrf",
		"max-body",
		"authenticate",
		"handler",
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("middleware order mismatch: got=%v want=%v", order, want)
	}
}
