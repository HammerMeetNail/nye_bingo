package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
)

func withAIEnhancementsEnabled(t *testing.T, enabled bool) {
	t.Helper()
	prev := billing.GlobalFeatureSwitches()
	next := prev
	next.AIEnhancements = enabled
	billing.SetGlobalFeatureSwitches(next)
	t.Cleanup(func() {
		billing.SetGlobalFeatureSwitches(prev)
	})
}

func premiumUser() *models.User {
	return &models.User{
		ID:            uuid.New(),
		Email:         "u@example.com",
		EmailVerified: true,
		BillingPlan:   "premium",
	}
}

func TestAIPremiumStatus_PremiumRequired(t *testing.T) {
	withAIEnhancementsEnabled(t, true)
	mockService := &MockAIService{}
	handler := NewAIHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/premium/status", nil)
	req = req.WithContext(SetUserInContext(req.Context(), &models.User{ID: uuid.New(), BillingPlan: "free"}))
	w := httptest.NewRecorder()

	handler.PremiumStatus(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestAIPremiumStatus_Success(t *testing.T) {
	withAIEnhancementsEnabled(t, true)
	mockService := &MockAIService{
		GetPremiumStatusFunc: func(_ context.Context, _ uuid.UUID, _ time.Time) (int, int, int, time.Time, error) {
			return 100, 20, 80, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), nil
		},
	}
	handler := NewAIHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/premium/status", nil)
	req = req.WithContext(SetUserInContext(req.Context(), premiumUser()))
	w := httptest.NewRecorder()

	handler.PremiumStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body PremiumStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Limit != 100 || body.Used != 20 || body.Remaining != 80 {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestAIAssistPremium_Success(t *testing.T) {
	withAIEnhancementsEnabled(t, true)
	reset := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	mockService := &MockAIService{
		ReservePremiumFunc: func(_ context.Context, _ uuid.UUID, _ time.Time) (int, time.Time, error) {
			return 72, reset, nil
		},
		AssistCardGoalFunc: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ int, _ string, _ string) (string, ai.UsageStats, error) {
			return "Do this next: schedule 20 minutes tonight.", ai.UsageStats{}, nil
		},
	}
	handler := NewAIHandler(mockService)

	cardID := uuid.New()
	body := map[string]any{
		"card_id":  cardID.String(),
		"position": 7,
		"mode":     "next_step",
		"notes":    "Only weekends",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/assist", bytes.NewBuffer(raw))
	req = req.WithContext(SetUserInContext(req.Context(), premiumUser()))
	w := httptest.NewRecorder()

	handler.Assist(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if mockService.ReservePremiumCalls != 1 || mockService.AssistCalls != 1 {
		t.Fatalf("expected reserve and assist once, got reserve=%d assist=%d", mockService.ReservePremiumCalls, mockService.AssistCalls)
	}
}

func TestAIAssistPremium_InvalidMode(t *testing.T) {
	withAIEnhancementsEnabled(t, true)
	mockService := &MockAIService{}
	handler := NewAIHandler(mockService)

	body := map[string]any{
		"card_id":  uuid.New().String(),
		"position": 1,
		"mode":     "unknown",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/assist", bytes.NewBuffer(raw))
	req = req.WithContext(SetUserInContext(req.Context(), premiumUser()))
	w := httptest.NewRecorder()

	handler.Assist(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if mockService.ReservePremiumCalls != 0 {
		t.Fatalf("reserve should not be called on invalid mode")
	}
}

func TestAIAssistPremium_RequiresPositionField(t *testing.T) {
	withAIEnhancementsEnabled(t, true)
	mockService := &MockAIService{}
	handler := NewAIHandler(mockService)

	body := map[string]any{
		"card_id": uuid.New().String(),
		"mode":    "next_step",
		"notes":   "only evenings",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/assist", bytes.NewBuffer(raw))
	req = req.WithContext(SetUserInContext(req.Context(), premiumUser()))
	w := httptest.NewRecorder()

	handler.Assist(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if mockService.ReservePremiumCalls != 0 {
		t.Fatalf("reserve should not be called when position is missing")
	}
	if mockService.AssistCalls != 0 {
		t.Fatalf("assist should not be called when position is missing")
	}
}

func TestAIRegeneratePremium_Exhausted(t *testing.T) {
	withAIEnhancementsEnabled(t, true)
	reset := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	mockService := &MockAIService{
		ReservePremiumFunc: func(_ context.Context, _ uuid.UUID, _ time.Time) (int, time.Time, error) {
			return 0, reset, ai.ErrPremiumEnhancementsExhausted
		},
	}
	handler := NewAIHandler(mockService)

	body := map[string]any{
		"category":       "mix",
		"difficulty":     "medium",
		"budget":         "free",
		"existing_goals": []string{"Goal 1", "Goal 2"},
		"replace_index":  0,
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/regenerate", bytes.NewBuffer(raw))
	req = req.WithContext(SetUserInContext(req.Context(), premiumUser()))
	w := httptest.NewRecorder()

	handler.Regenerate(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestAIRegeneratePremium_RequiresReplaceIndexField(t *testing.T) {
	withAIEnhancementsEnabled(t, true)
	mockService := &MockAIService{}
	handler := NewAIHandler(mockService)

	body := map[string]any{
		"category":       "mix",
		"difficulty":     "medium",
		"budget":         "free",
		"existing_goals": []string{"Goal 1", "Goal 2"},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/regenerate", bytes.NewBuffer(raw))
	req = req.WithContext(SetUserInContext(req.Context(), premiumUser()))
	w := httptest.NewRecorder()

	handler.Regenerate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if mockService.ReservePremiumCalls != 0 {
		t.Fatalf("reserve should not be called when replace_index is missing")
	}
	if mockService.RegenerateCalls != 0 {
		t.Fatalf("regenerate should not be called when replace_index is missing")
	}
}

func TestAIFillEmptyPremium_RefundsOnFailure(t *testing.T) {
	withAIEnhancementsEnabled(t, true)
	var reserveAt time.Time
	var refundAt time.Time
	mockService := &MockAIService{
		ReservePremiumFunc: func(_ context.Context, _ uuid.UUID, now time.Time) (int, time.Time, error) {
			reserveAt = now
			return 50, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), nil
		},
		RefundPremiumFunc: func(_ context.Context, _ uuid.UUID, now time.Time) error {
			refundAt = now
			return nil
		},
		FillEmptyOnCardFunc: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ ai.GoalPrompt) (*models.BingoCard, ai.UsageStats, error) {
			return nil, ai.UsageStats{}, ai.ErrAIProviderUnavailable
		},
	}
	handler := NewAIHandler(mockService)

	body := map[string]any{
		"card_id":    uuid.New().String(),
		"category":   "mix",
		"difficulty": "medium",
		"budget":     "free",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/fill-empty", bytes.NewBuffer(raw))
	req = req.WithContext(SetUserInContext(req.Context(), premiumUser()))
	w := httptest.NewRecorder()

	handler.FillEmpty(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	if mockService.RefundPremiumCalls != 1 {
		t.Fatalf("expected refund call, got %d", mockService.RefundPremiumCalls)
	}
	if reserveAt.IsZero() || refundAt.IsZero() {
		t.Fatalf("expected reserve/refund timestamps to be captured")
	}
	if !reserveAt.Equal(refundAt) {
		t.Fatalf("expected refund time %s to match reserve time %s", refundAt, reserveAt)
	}
}
