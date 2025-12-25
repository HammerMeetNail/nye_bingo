package handlers

import (
	"bytes"
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

func TestApiTokenHandler_Create_Unauthenticated(t *testing.T) {
	handler := NewApiTokenHandler(&mockApiTokenService{})

	req := httptest.NewRequest(http.MethodPost, "/api/tokens", nil)
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestApiTokenHandler_Create_Validation(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	handler := NewApiTokenHandler(&mockApiTokenService{})

	tests := []struct {
		name string
		body any
		want int
	}{
		{"invalid json", "not-json", http.StatusBadRequest},
		{"missing name", CreateApiTokenRequest{Name: "", Scope: models.ScopeRead, ExpiresInDays: 7}, http.StatusBadRequest},
		{"invalid scope", CreateApiTokenRequest{Name: "test", Scope: "nope", ExpiresInDays: 7}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyBytes []byte
			switch v := tt.body.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(v)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/tokens", bytes.NewBuffer(bodyBytes))
			req = req.WithContext(SetUserInContext(req.Context(), user))
			rr := httptest.NewRecorder()

			handler.Create(rr, req)

			if rr.Code != tt.want {
				t.Fatalf("expected status %d, got %d", tt.want, rr.Code)
			}
		})
	}
}

func TestApiTokenHandler_Create_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}

	tokenID := uuid.New()
	mockSvc := &mockApiTokenService{
		CreateFunc: func(ctx context.Context, userID uuid.UUID, name string, scope models.ApiTokenScope, expiresInDays int) (*models.ApiToken, string, error) {
			if userID != user.ID {
				t.Fatalf("unexpected user id: %s", userID)
			}
			if name != "My Token" {
				t.Fatalf("unexpected name: %q", name)
			}
			if scope != models.ScopeRead {
				t.Fatalf("unexpected scope: %s", scope)
			}
			if expiresInDays != 30 {
				t.Fatalf("unexpected expires_in_days: %d", expiresInDays)
			}

			return &models.ApiToken{
				ID:          tokenID,
				UserID:      user.ID,
				Name:        name,
				TokenPrefix: "yob_abcd",
				Scope:       scope,
			}, "yob_secret", nil
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	bodyBytes, _ := json.Marshal(CreateApiTokenRequest{Name: "My Token", Scope: models.ScopeRead, ExpiresInDays: 30})
	req := httptest.NewRequest(http.MethodPost, "/api/tokens", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	var resp CreateApiTokenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Token == nil || resp.Token.ID != tokenID {
		t.Fatalf("expected token metadata")
	}
	if resp.RawToken != "yob_secret" {
		t.Fatalf("expected raw token to be returned once")
	}
	if resp.Warning == "" {
		t.Fatalf("expected warning to be set")
	}
}

func TestApiTokenHandler_Create_Error(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockSvc := &mockApiTokenService{
		CreateFunc: func(ctx context.Context, userID uuid.UUID, name string, scope models.ApiTokenScope, expiresInDays int) (*models.ApiToken, string, error) {
			return nil, "", errors.New("create error")
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	bodyBytes, _ := json.Marshal(CreateApiTokenRequest{Name: "My Token", Scope: models.ScopeRead, ExpiresInDays: 7})
	req := httptest.NewRequest(http.MethodPost, "/api/tokens", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestApiTokenHandler_List_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockSvc := &mockApiTokenService{
		ListFunc: func(ctx context.Context, userID uuid.UUID) ([]models.ApiToken, error) {
			if userID != user.ID {
				t.Fatalf("unexpected user id: %s", userID)
			}
			return []models.ApiToken{{ID: uuid.New(), UserID: user.ID, Name: "t1", Scope: models.ScopeRead}}, nil
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp ListApiTokensResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(resp.Tokens))
	}
}

func TestApiTokenHandler_List_Unauthenticated(t *testing.T) {
	handler := NewApiTokenHandler(&mockApiTokenService{})

	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestApiTokenHandler_List_NilBecomesEmpty(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockSvc := &mockApiTokenService{
		ListFunc: func(ctx context.Context, userID uuid.UUID) ([]models.ApiToken, error) {
			return nil, nil
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp ListApiTokensResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Tokens == nil {
		t.Fatalf("expected tokens to be an empty slice, got nil")
	}
	if len(resp.Tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(resp.Tokens))
	}
}

func TestApiTokenHandler_List_Error(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockSvc := &mockApiTokenService{
		ListFunc: func(ctx context.Context, userID uuid.UUID) ([]models.ApiToken, error) {
			return nil, errors.New("boom")
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.List(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestApiTokenHandler_Delete_InvalidPath(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	handler := NewApiTokenHandler(&mockApiTokenService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestApiTokenHandler_Delete_InvalidTokenID(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	handler := NewApiTokenHandler(&mockApiTokenService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens/not-a-uuid", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestApiTokenHandler_Delete_NotFound(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockSvc := &mockApiTokenService{
		DeleteFunc: func(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error {
			return services.ErrTokenNotFound
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens/"+uuid.New().String(), nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestApiTokenHandler_Delete_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockSvc := &mockApiTokenService{
		DeleteFunc: func(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error {
			return nil
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens/"+uuid.New().String(), nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestApiTokenHandler_Delete_Error(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockSvc := &mockApiTokenService{
		DeleteFunc: func(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error {
			return errors.New("delete error")
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens/"+uuid.New().String(), nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestApiTokenHandler_Delete_Unauthenticated(t *testing.T) {
	handler := NewApiTokenHandler(&mockApiTokenService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()

	handler.Delete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestApiTokenHandler_DeleteAll_Error(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	mockSvc := &mockApiTokenService{
		DeleteAllFunc: func(ctx context.Context, userID uuid.UUID) error {
			return errors.New("boom")
		},
	}
	handler := NewApiTokenHandler(mockSvc)

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens", nil)
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.DeleteAll(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}
}

func TestApiTokenHandler_DeleteAll_Success(t *testing.T) {
	service := &mockApiTokenService{
		DeleteAllFunc: func(ctx context.Context, userID uuid.UUID) error {
			return nil
		},
	}
	handler := NewApiTokenHandler(service)

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.DeleteAll(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestApiTokenHandler_DeleteAll_Unauthenticated(t *testing.T) {
	handler := NewApiTokenHandler(&mockApiTokenService{})

	req := httptest.NewRequest(http.MethodDelete, "/api/tokens", nil)
	rr := httptest.NewRecorder()

	handler.DeleteAll(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}
