package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func TestSuggestionHandler_GetAll_Grouped(t *testing.T) {
	mockSvc := &mockSuggestionService{
		GetGroupedByCategoryFunc: func(ctx context.Context) ([]services.SuggestionsByCategory, error) {
			return []services.SuggestionsByCategory{
				{Category: "Health", Suggestions: []*models.Suggestion{{ID: uuid.New(), Category: "Health", Content: "Walk"}}},
			}, nil
		},
	}
	handler := NewSuggestionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions?grouped=true", nil)
	rr := httptest.NewRecorder()

	handler.GetAll(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp SuggestionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Grouped) != 1 {
		t.Fatalf("expected grouped results")
	}
}

func TestSuggestionHandler_GetAll_ByCategory(t *testing.T) {
	mockSvc := &mockSuggestionService{
		GetByCategoryFunc: func(ctx context.Context, category string) ([]*models.Suggestion, error) {
			if category != "Health" {
				t.Fatalf("unexpected category: %q", category)
			}
			return []*models.Suggestion{{ID: uuid.New(), Category: category, Content: "Walk"}}, nil
		},
	}
	handler := NewSuggestionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions?category=Health", nil)
	rr := httptest.NewRecorder()

	handler.GetAll(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp SuggestionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Suggestions) != 1 {
		t.Fatalf("expected suggestions results")
	}
}

func TestSuggestionHandler_GetAll_ByCategoryError(t *testing.T) {
	mockSvc := &mockSuggestionService{
		GetByCategoryFunc: func(ctx context.Context, category string) ([]*models.Suggestion, error) {
			return nil, errors.New("boom")
		},
	}
	handler := NewSuggestionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions?category=Health", nil)
	rr := httptest.NewRecorder()

	handler.GetAll(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestSuggestionHandler_GetAll_All(t *testing.T) {
	mockSvc := &mockSuggestionService{
		GetAllFunc: func(ctx context.Context) ([]*models.Suggestion, error) {
			return []*models.Suggestion{{ID: uuid.New(), Category: "Health", Content: "Walk"}}, nil
		},
	}
	handler := NewSuggestionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions", nil)
	rr := httptest.NewRecorder()

	handler.GetAll(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestSuggestionHandler_GetAll_GroupedError(t *testing.T) {
	mockSvc := &mockSuggestionService{
		GetGroupedByCategoryFunc: func(ctx context.Context) ([]services.SuggestionsByCategory, error) {
			return nil, errors.New("boom")
		},
	}
	handler := NewSuggestionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions?grouped=true", nil)
	rr := httptest.NewRecorder()

	handler.GetAll(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestSuggestionHandler_GetAll_Error(t *testing.T) {
	mockSvc := &mockSuggestionService{
		GetAllFunc: func(ctx context.Context) ([]*models.Suggestion, error) {
			return nil, errors.New("boom")
		},
	}
	handler := NewSuggestionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions", nil)
	rr := httptest.NewRecorder()

	handler.GetAll(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestSuggestionHandler_GetCategories_Success(t *testing.T) {
	mockSvc := &mockSuggestionService{
		GetCategoriesFunc: func(ctx context.Context) ([]string, error) {
			return []string{"Health", "Career"}, nil
		},
	}
	handler := NewSuggestionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions/categories", nil)
	rr := httptest.NewRecorder()

	handler.GetCategories(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp SuggestionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Categories) != 2 {
		t.Fatalf("expected categories")
	}
}

func TestSuggestionHandler_GetCategories_Error(t *testing.T) {
	mockSvc := &mockSuggestionService{
		GetCategoriesFunc: func(ctx context.Context) ([]string, error) {
			return nil, errors.New("boom")
		},
	}
	handler := NewSuggestionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/suggestions/categories", nil)
	rr := httptest.NewRecorder()

	handler.GetCategories(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}
