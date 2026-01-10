package handlers

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type mockShareOGService struct {
	services.CardServiceInterface
	GetSharedCardFunc func(ctx context.Context, token string) (*models.SharedCard, error)
}

func (m *mockShareOGService) GetSharedCardByToken(ctx context.Context, token string) (*models.SharedCard, error) {
	return m.GetSharedCardFunc(ctx, token)
}

func TestShareOGImageHandler_Serve_PNGAndETag(t *testing.T) {
	token := strings.Repeat("c", 64)
	title := "Card"
	shared := &models.SharedCard{
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
	}

	h := NewShareOGImageHandler(&mockShareOGService{
		GetSharedCardFunc: func(ctx context.Context, got string) (*models.SharedCard, error) {
			if got != token {
				t.Fatalf("expected token %q, got %q", token, got)
			}
			return shared, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/og/share/"+token+".png", nil)
	req.SetPathValue("token", token+".png")
	rr := httptest.NewRecorder()
	h.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected content-type image/png, got %q", ct)
	}
	if etag := rr.Header().Get("ETag"); etag == "" {
		t.Fatalf("expected etag to be set")
	}
	if _, err := png.Decode(bytes.NewReader(rr.Body.Bytes())); err != nil {
		t.Fatalf("expected response body to be a valid PNG: %v", err)
	}

	etag := rr.Header().Get("ETag")
	req2 := httptest.NewRequest(http.MethodGet, "/og/share/"+token+".png", nil)
	req2.SetPathValue("token", token+".png")
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	h.Serve(rr2, req2)
	if rr2.Code != http.StatusNotModified {
		t.Fatalf("expected status 304, got %d", rr2.Code)
	}
}
