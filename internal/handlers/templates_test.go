package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
)

type mockTemplateService struct {
	ListFunc                   func(ctx context.Context, userID uuid.UUID) ([]models.CardTemplate, error)
	GetFunc                    func(ctx context.Context, userID, templateID uuid.UUID) (*models.TemplateWithItems, error)
	CreateFromCardFunc         func(ctx context.Context, userID, cardID uuid.UUID, name string) (*models.TemplateWithItems, error)
	CreateFunc                 func(ctx context.Context, userID uuid.UUID, params models.CreateTemplateParams) (*models.TemplateWithItems, error)
	UpdateFunc                 func(ctx context.Context, userID, templateID uuid.UUID, params models.UpdateTemplateParams) (*models.TemplateWithItems, error)
	ReplaceItemsFunc           func(ctx context.Context, userID, templateID uuid.UUID, items []string) (*models.TemplateWithItems, error)
	DeleteFunc                 func(ctx context.Context, userID, templateID uuid.UUID) error
	CreateCardFromTemplateFunc func(ctx context.Context, userID, templateID uuid.UUID, params models.TemplateCreateCardParams) (*models.BingoCard, error)
	RolloverCardFunc           func(ctx context.Context, userID, cardID uuid.UUID, params models.RolloverParams) (*models.BingoCard, error)
}

func (m *mockTemplateService) List(ctx context.Context, userID uuid.UUID) ([]models.CardTemplate, error) {
	if m.ListFunc == nil {
		return nil, nil
	}
	return m.ListFunc(ctx, userID)
}
func (m *mockTemplateService) Get(ctx context.Context, userID, templateID uuid.UUID) (*models.TemplateWithItems, error) {
	if m.GetFunc == nil {
		return nil, nil
	}
	return m.GetFunc(ctx, userID, templateID)
}
func (m *mockTemplateService) CreateFromCard(ctx context.Context, userID, cardID uuid.UUID, name string) (*models.TemplateWithItems, error) {
	if m.CreateFromCardFunc == nil {
		return nil, nil
	}
	return m.CreateFromCardFunc(ctx, userID, cardID, name)
}
func (m *mockTemplateService) Create(ctx context.Context, userID uuid.UUID, params models.CreateTemplateParams) (*models.TemplateWithItems, error) {
	if m.CreateFunc == nil {
		return nil, nil
	}
	return m.CreateFunc(ctx, userID, params)
}
func (m *mockTemplateService) Update(ctx context.Context, userID, templateID uuid.UUID, params models.UpdateTemplateParams) (*models.TemplateWithItems, error) {
	if m.UpdateFunc == nil {
		return nil, nil
	}
	return m.UpdateFunc(ctx, userID, templateID, params)
}
func (m *mockTemplateService) ReplaceItems(ctx context.Context, userID, templateID uuid.UUID, items []string) (*models.TemplateWithItems, error) {
	if m.ReplaceItemsFunc == nil {
		return nil, nil
	}
	return m.ReplaceItemsFunc(ctx, userID, templateID, items)
}
func (m *mockTemplateService) Delete(ctx context.Context, userID, templateID uuid.UUID) error {
	if m.DeleteFunc == nil {
		return nil
	}
	return m.DeleteFunc(ctx, userID, templateID)
}
func (m *mockTemplateService) CreateCardFromTemplate(ctx context.Context, userID, templateID uuid.UUID, params models.TemplateCreateCardParams) (*models.BingoCard, error) {
	if m.CreateCardFromTemplateFunc == nil {
		return nil, nil
	}
	return m.CreateCardFromTemplateFunc(ctx, userID, templateID, params)
}
func (m *mockTemplateService) RolloverCard(ctx context.Context, userID, cardID uuid.UUID, params models.RolloverParams) (*models.BingoCard, error) {
	if m.RolloverCardFunc == nil {
		return nil, nil
	}
	return m.RolloverCardFunc(ctx, userID, cardID, params)
}

func TestTemplateHandler_CreateTemplate_RequiresPremium(t *testing.T) {
	handler := NewTemplateHandler(&mockTemplateService{
		CreateFunc: func(ctx context.Context, userID uuid.UUID, params models.CreateTemplateParams) (*models.TemplateWithItems, error) {
			t.Fatalf("service should not be called for non-premium user")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/templates", bytes.NewBufferString(`{"name":"Test"}`))
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "free"}))
	rr := httptest.NewRecorder()

	handler.CreateTemplate(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestTemplateHandler_ListTemplates_RequiresAuth(t *testing.T) {
	handler := NewTemplateHandler(&mockTemplateService{})
	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	rr := httptest.NewRecorder()

	handler.ListTemplates(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestTemplateHandler_ListTemplates_RequiresPremium(t *testing.T) {
	handler := NewTemplateHandler(&mockTemplateService{
		ListFunc: func(ctx context.Context, userID uuid.UUID) ([]models.CardTemplate, error) {
			t.Fatalf("service should not be called for non-premium user")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "free"}))
	rr := httptest.NewRecorder()

	handler.ListTemplates(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestTemplateHandler_ListTemplates_GlobalFeatureDisabled(t *testing.T) {
	prev := billing.GlobalFeatureSwitches()
	billing.SetGlobalFeatureSwitches(billing.FeatureEntitlements{
		Templates:         false,
		EditAfterFinalize: true,
	})
	t.Cleanup(func() {
		billing.SetGlobalFeatureSwitches(prev)
	})

	handler := NewTemplateHandler(&mockTemplateService{
		ListFunc: func(ctx context.Context, userID uuid.UUID) ([]models.CardTemplate, error) {
			t.Fatalf("service should not be called when templates feature is globally disabled")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.ListTemplates(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestTemplateHandler_CreateCardFromTemplate_ConflictReturns409(t *testing.T) {
	templateID := uuid.New()
	handler := NewTemplateHandler(&mockTemplateService{
		CreateCardFromTemplateFunc: func(ctx context.Context, userID, templateID uuid.UUID, params models.TemplateCreateCardParams) (*models.BingoCard, error) {
			return nil, &services.CardConflictError{
				Year:           2026,
				Title:          "My 2026 Goals",
				SuggestedTitle: "My 2026 Goals (Copy)",
			}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/templates/"+templateID.String()+"/create-card", bytes.NewBufferString(`{"year":2026}`))
	req.SetPathValue("id", templateID.String())
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.CreateCardFromTemplate(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, rr.Code)
	}

	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if out["suggested_title"] != "My 2026 Goals (Copy)" {
		t.Fatalf("expected suggested_title, got %#v", out["suggested_title"])
	}
}

func TestTemplateHandler_GetTemplate_NotFoundReturns404(t *testing.T) {
	templateID := uuid.New()
	handler := NewTemplateHandler(&mockTemplateService{
		GetFunc: func(ctx context.Context, userID, templateID uuid.UUID) (*models.TemplateWithItems, error) {
			return nil, services.ErrTemplateNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/templates/"+templateID.String(), nil)
	req.SetPathValue("id", templateID.String())
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.GetTemplate(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestTemplateHandler_GetTemplate_RequiresPremium(t *testing.T) {
	templateID := uuid.New()
	handler := NewTemplateHandler(&mockTemplateService{
		GetFunc: func(ctx context.Context, userID, templateID uuid.UUID) (*models.TemplateWithItems, error) {
			t.Fatalf("service should not be called for non-premium user")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/templates/"+templateID.String(), nil)
	req.SetPathValue("id", templateID.String())
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "free"}))
	rr := httptest.NewRecorder()

	handler.GetTemplate(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestTemplateHandler_CreateTemplate_FromCard_InvalidFromCardIDReturns400(t *testing.T) {
	handler := NewTemplateHandler(&mockTemplateService{
		CreateFromCardFunc: func(ctx context.Context, userID, cardID uuid.UUID, name string) (*models.TemplateWithItems, error) {
			t.Fatalf("service should not be called for invalid from_card_id")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/templates", bytes.NewBufferString(`{"from_card_id":"not-a-uuid","name":"Test"}`))
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.CreateTemplate(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTemplateHandler_CreateTemplate_FromCard_NotOwnerReturns403(t *testing.T) {
	handler := NewTemplateHandler(&mockTemplateService{
		CreateFromCardFunc: func(ctx context.Context, userID, cardID uuid.UUID, name string) (*models.TemplateWithItems, error) {
			return nil, services.ErrNotCardOwner
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/templates", bytes.NewBufferString(fmt.Sprintf(`{"from_card_id":"%s","name":"Test"}`, uuid.New().String())))
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.CreateTemplate(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestTemplateHandler_UpdateTemplate_Success(t *testing.T) {
	templateID := uuid.New()
	called := false
	handler := NewTemplateHandler(&mockTemplateService{
		UpdateFunc: func(ctx context.Context, userID, templateID uuid.UUID, params models.UpdateTemplateParams) (*models.TemplateWithItems, error) {
			called = true
			return &models.TemplateWithItems{
				Template: models.CardTemplate{ID: templateID, UserID: userID, Name: params.Name, GridSize: params.GridSize, HeaderText: params.HeaderText, HasFreeSpace: params.HasFreeSpace},
				Items:    []models.CardTemplateItem{},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/api/templates/"+templateID.String(), bytes.NewBufferString(`{"name":"Renamed"}`))
	req.SetPathValue("id", templateID.String())
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.UpdateTemplate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	if !called {
		t.Fatalf("expected service to be called")
	}
}

func TestTemplateHandler_CreateCardFromTemplate_InvalidYearReturns400(t *testing.T) {
	templateID := uuid.New()
	handler := NewTemplateHandler(&mockTemplateService{
		CreateCardFromTemplateFunc: func(ctx context.Context, userID, templateID uuid.UUID, params models.TemplateCreateCardParams) (*models.BingoCard, error) {
			t.Fatalf("service should not be called for invalid year")
			return nil, nil
		},
	})

	year := time.Now().Year() + 2
	req := httptest.NewRequest(http.MethodPost, "/api/templates/"+templateID.String()+"/create-card", bytes.NewBufferString(fmt.Sprintf(`{"year":%d}`, year)))
	req.SetPathValue("id", templateID.String())
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.CreateCardFromTemplate(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTemplateHandler_RolloverCard_InvalidCarryOverReturns400(t *testing.T) {
	cardID := uuid.New()
	handler := NewTemplateHandler(&mockTemplateService{
		RolloverCardFunc: func(ctx context.Context, userID, cardID uuid.UUID, params models.RolloverParams) (*models.BingoCard, error) {
			return nil, services.ErrInvalidCarryOverValue
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/rollover", bytes.NewBufferString(`{"year":2026,"carry_over":"nope"}`))
	req.SetPathValue("id", cardID.String())
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.RolloverCard(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestTemplateHandler_decodeStrictJSON_RejectsTrailingTokens(t *testing.T) {
	handler := NewTemplateHandler(&mockTemplateService{
		CreateFunc: func(ctx context.Context, userID uuid.UUID, params models.CreateTemplateParams) (*models.TemplateWithItems, error) {
			t.Fatalf("service should not be called for invalid request body")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/templates", bytes.NewBufferString(`{"name":"Test"} {"oops":1}`))
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "premium"}))
	rr := httptest.NewRecorder()

	handler.CreateTemplate(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
