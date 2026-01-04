package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func TestReminderPublicHandler_UnsubscribeConfirm_MissingToken(t *testing.T) {
	handler := NewReminderPublicHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodGet, "/r/unsubscribe", nil)
	rr := httptest.NewRecorder()

	handler.UnsubscribeConfirm(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Missing token")
}

func TestReminderPublicHandler_UnsubscribeConfirm_RendersFormAndEscapesToken(t *testing.T) {
	handler := NewReminderPublicHandler(&mockReminderService{})
	token := `"><img src=x onerror=alert(1)>`
	req := httptest.NewRequest(http.MethodGet, "/r/unsubscribe?token="+url.QueryEscape(token), nil)
	rr := httptest.NewRecorder()

	handler.UnsubscribeConfirm(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Result().Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected content type text/html, got %q", ct)
	}
	if cache := rr.Result().Header.Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", cache)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `<form method="POST" action="/r/unsubscribe">`) {
		t.Fatalf("expected unsubscribe form, got %q", body)
	}
	if strings.Contains(body, "<img") {
		t.Fatalf("expected token to be escaped, got %q", body)
	}
	if !strings.Contains(body, `value="&#34;&gt;&lt;img src=x onerror=alert(1)&gt;"`) {
		t.Fatalf("expected escaped token, got %q", body)
	}
}

func TestReminderPublicHandler_UnsubscribeSubmit_MissingToken(t *testing.T) {
	handler := NewReminderPublicHandler(&mockReminderService{})
	req := httptest.NewRequest(http.MethodPost, "/r/unsubscribe", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.UnsubscribeSubmit(rr, req)
	assertErrorResponse(t, rr, http.StatusBadRequest, "Missing token")
}

func TestReminderPublicHandler_UnsubscribeSubmit_ExpiredToken(t *testing.T) {
	handler := NewReminderPublicHandler(&mockReminderService{
		UnsubscribeByTokenFunc: func(ctx context.Context, token string) (bool, error) {
			return false, services.ErrReminderNotFound
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/r/unsubscribe", strings.NewReader("token=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.UnsubscribeSubmit(rr, req)
	assertErrorResponse(t, rr, http.StatusNotFound, "Unsubscribe link expired")
}

func TestReminderPublicHandler_UnsubscribeSubmit_Success(t *testing.T) {
	var gotToken string
	handler := NewReminderPublicHandler(&mockReminderService{
		UnsubscribeByTokenFunc: func(ctx context.Context, token string) (bool, error) {
			gotToken = token
			return false, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/r/unsubscribe", strings.NewReader("token=abc123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.UnsubscribeSubmit(rr, req)

	if gotToken != "abc123" {
		t.Fatalf("expected token abc123, got %q", gotToken)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Result().Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected content type text/html, got %q", ct)
	}
	if cache := rr.Result().Header.Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", cache)
	}
	if !strings.Contains(rr.Body.String(), "Reminders disabled") {
		t.Fatalf("expected success message, got %q", rr.Body.String())
	}
}

func TestReminderPublicHandler_UnsubscribeSubmit_AlreadyDisabled(t *testing.T) {
	handler := NewReminderPublicHandler(&mockReminderService{
		UnsubscribeByTokenFunc: func(ctx context.Context, token string) (bool, error) {
			return true, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/r/unsubscribe", strings.NewReader("token=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler.UnsubscribeSubmit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "already disabled") {
		t.Fatalf("expected already disabled message, got %q", rr.Body.String())
	}
}
