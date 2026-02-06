package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/handlers"
)

func TestRegisterTemplateRoutes(t *testing.T) {
	mux := http.NewServeMux()
	templatesHandler := handlers.NewTemplateHandler(nil)
	identity := func(h http.Handler) http.Handler { return h }

	registerTemplateRoutes(mux, templatesHandler, identity, identity)

	id := uuid.New().String()
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/templates"},
		{http.MethodGet, "/api/templates/" + id},
		{http.MethodPost, "/api/templates"},
		{http.MethodPut, "/api/templates/" + id},
		{http.MethodPut, "/api/templates/" + id + "/items"},
		{http.MethodDelete, "/api/templates/" + id},
		{http.MethodPost, "/api/templates/" + id + "/create-card"},
		{http.MethodPost, "/api/cards/" + id + "/rollover"},
	}

	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Fatalf("expected route registered for %s %s", c.method, c.path)
		}
	}
}
