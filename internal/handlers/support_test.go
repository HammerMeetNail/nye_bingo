package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

type fakeRateLimitStore struct {
	incrCount   int64
	incrErr     error
	expireCalls int
}

func (s *fakeRateLimitStore) Incr(ctx context.Context, key string) (int64, error) {
	if s.incrErr != nil {
		return 0, s.incrErr
	}
	s.incrCount++
	return s.incrCount, nil
}

func (s *fakeRateLimitStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	s.expireCalls++
	return nil
}

func TestSupportHandler_checkRateLimit(t *testing.T) {
	t.Run("no store", func(t *testing.T) {
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: nil}
		req := httptest.NewRequest(http.MethodPost, "/api/support", nil)
		if ok := h.checkRateLimit(req, "1.2.3.4"); !ok {
			t.Fatalf("expected allowed")
		}
	})

	t.Run("store error allows", func(t *testing.T) {
		store := &fakeRateLimitStore{incrErr: errors.New("boom")}
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: store}
		req := httptest.NewRequest(http.MethodPost, "/api/support", nil)
		if ok := h.checkRateLimit(req, "1.2.3.4"); !ok {
			t.Fatalf("expected allowed")
		}
	})

	t.Run("enforces limit and sets expiry once", func(t *testing.T) {
		store := &fakeRateLimitStore{}
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: store}
		req := httptest.NewRequest(http.MethodPost, "/api/support", nil)

		for i := 0; i < supportRateLimitMax; i++ {
			if ok := h.checkRateLimit(req, "1.2.3.4"); !ok {
				t.Fatalf("expected allowed on attempt %d", i+1)
			}
		}

		if ok := h.checkRateLimit(req, "1.2.3.4"); ok {
			t.Fatalf("expected rate limit to reject after max")
		}
		if store.expireCalls != 1 {
			t.Fatalf("expected expire to be called once, got %d", store.expireCalls)
		}
	})
}

func TestSupportHandler_getClientIP(t *testing.T) {
	t.Run("x-forwarded-for", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/support", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1, 203.0.113.2")
		if got := getClientIP(req); got != "203.0.113.1" {
			t.Fatalf("expected first ip, got %q", got)
		}
	})

	t.Run("x-real-ip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/support", nil)
		req.Header.Set("X-Real-IP", "203.0.113.9")
		if got := getClientIP(req); got != "203.0.113.9" {
			t.Fatalf("expected x-real-ip, got %q", got)
		}
	})
}

func TestSupportHandler_Submit_Success(t *testing.T) {
	var called bool
	mockEmail := &mockEmailService{
		SendSupportEmailFunc: func(ctx context.Context, fromEmail, category, message string, userID string) error {
			called = true
			if fromEmail != "test@example.com" {
				t.Fatalf("unexpected from email: %q", fromEmail)
			}
			if category != "Bug Report" {
				t.Fatalf("unexpected category: %q", category)
			}
			if message != "This is a valid message." {
				t.Fatalf("unexpected message: %q", message)
			}
			if userID == "" {
				t.Fatalf("expected user id")
			}
			return nil
		},
	}

	h := &SupportHandler{
		emailService: mockEmail,
		rateLimiter:  nil,
	}

	user := &models.User{ID: uuid.New()}
	bodyBytes, _ := json.Marshal(SupportRequest{
		Email:    "TEST@EXAMPLE.COM",
		Category: "Bug Report",
		Message:  "This is a valid message.",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/support", bytes.NewBuffer(bodyBytes))
	req = req.WithContext(SetUserInContext(req.Context(), user))
	rr := httptest.NewRecorder()

	h.Submit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !called {
		t.Fatalf("expected support email to be sent")
	}
}

func TestSupportHandler_Submit_ValidationAndErrors(t *testing.T) {
	t.Run("rate limited", func(t *testing.T) {
		store := &fakeRateLimitStore{incrCount: supportRateLimitMax}
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: store}

		bodyBytes, _ := json.Marshal(SupportRequest{
			Email:    "test@example.com",
			Category: "Bug Report",
			Message:  "This is a valid message.",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/support", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rr.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: nil}
		req := httptest.NewRequest(http.MethodPost, "/api/support", bytes.NewBufferString("{"))
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: nil}
		bodyBytes, _ := json.Marshal(SupportRequest{Email: "nope", Category: "Bug Report", Message: "This is a valid message."})
		req := httptest.NewRequest(http.MethodPost, "/api/support", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("invalid category", func(t *testing.T) {
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: nil}
		bodyBytes, _ := json.Marshal(SupportRequest{Email: "test@example.com", Category: "Nope", Message: "This is a valid message."})
		req := httptest.NewRequest(http.MethodPost, "/api/support", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("message too short", func(t *testing.T) {
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: nil}
		bodyBytes, _ := json.Marshal(SupportRequest{Email: "test@example.com", Category: "Bug Report", Message: "short"})
		req := httptest.NewRequest(http.MethodPost, "/api/support", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("message too long", func(t *testing.T) {
		h := &SupportHandler{emailService: &mockEmailService{}, rateLimiter: nil}
		bodyBytes, _ := json.Marshal(SupportRequest{Email: "test@example.com", Category: "Bug Report", Message: string(bytes.Repeat([]byte("a"), 5001))})
		req := httptest.NewRequest(http.MethodPost, "/api/support", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("email service error", func(t *testing.T) {
		h := &SupportHandler{
			emailService: &mockEmailService{SendSupportEmailFunc: func(ctx context.Context, fromEmail, category, message string, userID string) error {
				return errors.New("boom")
			}},
			rateLimiter: nil,
		}

		bodyBytes, _ := json.Marshal(SupportRequest{Email: "test@example.com", Category: "Bug Report", Message: "This is a valid message."})
		req := httptest.NewRequest(http.MethodPost, "/api/support", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()

		h.Submit(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})
}
