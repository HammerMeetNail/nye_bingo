package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type mockCardShareService struct {
	services.CardServiceInterface
	CreateOrRotateShareFunc func(ctx context.Context, userID, cardID uuid.UUID, expiresAt *time.Time) (*models.CardShare, error)
	GetShareStatusFunc      func(ctx context.Context, userID, cardID uuid.UUID) (*models.CardShare, error)
	RevokeShareFunc         func(ctx context.Context, userID, cardID uuid.UUID) error
	GetSharedCardFunc       func(ctx context.Context, token string) (*models.SharedCard, error)
}

func (m *mockCardShareService) CreateOrRotateShare(ctx context.Context, userID, cardID uuid.UUID, expiresAt *time.Time) (*models.CardShare, error) {
	return m.CreateOrRotateShareFunc(ctx, userID, cardID, expiresAt)
}

func (m *mockCardShareService) GetShareStatus(ctx context.Context, userID, cardID uuid.UUID) (*models.CardShare, error) {
	return m.GetShareStatusFunc(ctx, userID, cardID)
}

func (m *mockCardShareService) RevokeShare(ctx context.Context, userID, cardID uuid.UUID) error {
	return m.RevokeShareFunc(ctx, userID, cardID)
}

func (m *mockCardShareService) GetSharedCardByToken(ctx context.Context, token string) (*models.SharedCard, error) {
	return m.GetSharedCardFunc(ctx, token)
}

func TestCardShare_Create_Unauthorized(t *testing.T) {
	handler := NewCardHandler(&mockCardShareService{})
	req := httptest.NewRequest(http.MethodPost, "/api/cards/123/share", nil)
	req.SetPathValue("id", uuid.New().String())
	rr := httptest.NewRecorder()

	handler.CreateShare(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestCardShare_Create_NotFinalized(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()
	handler := NewCardHandler(&mockCardShareService{
		CreateOrRotateShareFunc: func(ctx context.Context, userID, cardID uuid.UUID, expiresAt *time.Time) (*models.CardShare, error) {
			return nil, services.ErrCardNotFinalized
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/share", nil)
	req.SetPathValue("id", cardID.String())
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.CreateShare(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestCardShare_Create_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()
	now := time.Now()
	share := &models.CardShare{
		CardID:    cardID,
		Token:     "deadbeef",
		CreatedAt: now,
	}

	handler := NewCardHandler(&mockCardShareService{
		CreateOrRotateShareFunc: func(ctx context.Context, userID, cardID uuid.UUID, expiresAt *time.Time) (*models.CardShare, error) {
			return share, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/cards/"+cardID.String()+"/share", nil)
	req.SetPathValue("id", cardID.String())
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.CreateShare(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}

	var response ShareStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !response.Enabled || response.URL == "" {
		t.Fatalf("expected enabled share with token, got %+v", response)
	}
	if response.URL != share.Token {
		t.Fatalf("expected token %q, got %q", share.Token, response.URL)
	}
}

func TestCardShare_Status_NotFound(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()

	handler := NewCardHandler(&mockCardShareService{
		GetShareStatusFunc: func(ctx context.Context, userID, cardID uuid.UUID) (*models.CardShare, error) {
			return nil, services.ErrShareNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cards/"+cardID.String()+"/share", nil)
	req.SetPathValue("id", cardID.String())
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.GetShareStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var response ShareStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response.Enabled {
		t.Fatal("expected enabled false for missing share")
	}
}

func TestCardShare_Status_Success(t *testing.T) {
	user := &models.User{ID: uuid.New()}
	cardID := uuid.New()
	now := time.Now()
	share := &models.CardShare{
		CardID:    cardID,
		Token:     "deadbeef",
		CreatedAt: now,
	}

	handler := NewCardHandler(&mockCardShareService{
		GetShareStatusFunc: func(ctx context.Context, userID, cardID uuid.UUID) (*models.CardShare, error) {
			return share, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cards/"+cardID.String()+"/share", nil)
	req.SetPathValue("id", cardID.String())
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	handler.GetShareStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var response ShareStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !response.Enabled || response.Expired {
		t.Fatalf("expected enabled share without expiration, got %+v", response)
	}
	if response.URL != share.Token {
		t.Fatalf("expected token %q, got %q", share.Token, response.URL)
	}
}

func TestCardShare_Public_NotFound(t *testing.T) {
	handler := NewCardHandler(&mockCardShareService{
		GetSharedCardFunc: func(ctx context.Context, token string) (*models.SharedCard, error) {
			return nil, services.ErrShareNotFound
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/share/deadbeef", nil)
	req.SetPathValue("token", "deadbeef")
	rr := httptest.NewRecorder()

	handler.GetSharedCard(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestCardShare_Public_SetsNoStore(t *testing.T) {
	handler := NewCardHandler(&mockCardShareService{
		GetSharedCardFunc: func(ctx context.Context, token string) (*models.SharedCard, error) {
			return &models.SharedCard{
				Card: models.PublicBingoCard{ID: uuid.New(), Year: 2025, GridSize: 5, HeaderText: "BINGO", HasFreeSpace: true, IsFinalized: true},
				Items: []models.PublicBingoItem{
					{Position: 0, Content: "Goal", IsCompleted: false},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/share/deadbeef", nil)
	req.SetPathValue("token", "deadbeef")
	rr := httptest.NewRecorder()

	handler.GetSharedCard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", rr.Header().Get("Cache-Control"))
	}
	if rr.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("expected X-Robots-Tag noindex, got %q", rr.Header().Get("X-Robots-Tag"))
	}
}
