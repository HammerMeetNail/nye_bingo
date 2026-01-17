package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type mockSharePublicService struct {
	services.CardServiceInterface
	GetSharedCardFunc func(ctx context.Context, token string) (*models.SharedCard, error)
}

func (m *mockSharePublicService) GetSharedCardByToken(ctx context.Context, token string) (*models.SharedCard, error) {
	return m.GetSharedCardFunc(ctx, token)
}

func TestSharePublicHandler_Serve_Found(t *testing.T) {
	token := strings.Repeat("a", 64)
	title := "My Card"
	handler, err := NewSharePublicHandler("../../web/templates", &mockSharePublicService{
		GetSharedCardFunc: func(ctx context.Context, got string) (*models.SharedCard, error) {
			if got != token {
				t.Fatalf("expected token %q, got %q", token, got)
			}
			return &models.SharedCard{
				Card: models.PublicBingoCard{
					Year:         2026,
					Title:        &title,
					GridSize:     5,
					HasFreeSpace: true,
					FreeSpacePos: ptrInt(12),
					IsFinalized:  true,
				},
				Items: []models.PublicBingoItem{
					{Position: 0, Content: "A", IsCompleted: true},
					{Position: 1, Content: "B", IsCompleted: false},
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/s/"+token, nil)
	req.Host = "example.com"
	req.SetPathValue("token", token)
	rr := httptest.NewRecorder()

	handler.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !containsAll(body, []string{
		`property="og:title"`,
		`content="My Card"`,
		`/og/share/` + token + `.png?v=`,
		`http-equiv="refresh"`,
		`/share/` + token,
	}) {
		t.Fatalf("expected og tags and redirect meta to be present")
	}
}

func TestSharePublicHandler_Serve_NotFound(t *testing.T) {
	token := strings.Repeat("b", 64)
	handler, err := NewSharePublicHandler("../../web/templates", &mockSharePublicService{
		GetSharedCardFunc: func(ctx context.Context, got string) (*models.SharedCard, error) {
			return nil, services.ErrShareNotFound
		},
	})
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/s/"+token, nil)
	req.Host = "example.com"
	req.SetPathValue("token", token)
	rr := httptest.NewRecorder()

	handler.Serve(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !containsAll(body, []string{`Share Link Not Found`, `/share/` + token}) {
		t.Fatalf("expected not found content and redirect meta to be present")
	}
}
