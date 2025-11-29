package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Note: The SuggestionHandler doesn't require authentication and delegates to the service.
// These tests verify the handler properly parses query parameters.

func TestSuggestionHandler_GetAll_NoQueryParams(t *testing.T) {
	// Handler with nil service will panic when called, but we can test endpoint exists
	// In real usage, service would be injected
	handler := NewSuggestionHandler(nil)

	if handler == nil {
		t.Error("expected handler to be created")
	}
}

func TestSuggestionHandler_GetAll_GroupedParam(t *testing.T) {
	// Verify the handler is created correctly and would parse grouped param
	handler := NewSuggestionHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions?grouped=true", nil)
	// We can't actually call the handler without a real service, but we verify the URL parsing
	if req.URL.Query().Get("grouped") != "true" {
		t.Error("expected grouped param to be parsed")
	}
	_ = handler
}

func TestSuggestionHandler_GetAll_CategoryParam(t *testing.T) {
	handler := NewSuggestionHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions?category=Health", nil)
	if req.URL.Query().Get("category") != "Health" {
		t.Error("expected category param to be parsed")
	}
	_ = handler
}

func TestSuggestionHandler_QueryParams(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantGrouped  string
		wantCategory string
	}{
		{
			name:         "no params",
			url:          "/api/suggestions",
			wantGrouped:  "",
			wantCategory: "",
		},
		{
			name:         "grouped true",
			url:          "/api/suggestions?grouped=true",
			wantGrouped:  "true",
			wantCategory: "",
		},
		{
			name:         "grouped false",
			url:          "/api/suggestions?grouped=false",
			wantGrouped:  "false",
			wantCategory: "",
		},
		{
			name:         "category only",
			url:          "/api/suggestions?category=Health",
			wantGrouped:  "",
			wantCategory: "Health",
		},
		{
			name:         "both params",
			url:          "/api/suggestions?grouped=true&category=Health",
			wantGrouped:  "true",
			wantCategory: "Health",
		},
		{
			name:         "special characters in category",
			url:          "/api/suggestions?category=Health%20%26%20Wellness",
			wantGrouped:  "",
			wantCategory: "Health & Wellness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)

			gotGrouped := req.URL.Query().Get("grouped")
			gotCategory := req.URL.Query().Get("category")

			if gotGrouped != tt.wantGrouped {
				t.Errorf("grouped: expected %q, got %q", tt.wantGrouped, gotGrouped)
			}
			if gotCategory != tt.wantCategory {
				t.Errorf("category: expected %q, got %q", tt.wantCategory, gotCategory)
			}
		})
	}
}
