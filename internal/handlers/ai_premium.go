package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
)

var validAssistModes = map[string]bool{
	"breakdown":  true,
	"next_step":  true,
	"obstacles":  true,
	"schedule":   true,
	"ideas":      true,
	"motivation": true,
}

type PremiumStatusResponse struct {
	Limit     int       `json:"limit"`
	Used      int       `json:"used"`
	Remaining int       `json:"remaining"`
	ResetsAt  time.Time `json:"resets_at"`
}

type AssistRequest struct {
	CardID   string `json:"card_id"`
	Position *int   `json:"position"`
	Mode     string `json:"mode"`
	Notes    string `json:"notes"`
}

type AssistResponse struct {
	Reply                 string    `json:"reply"`
	EnhancementsRemaining int       `json:"enhancements_remaining"`
	ResetsAt              time.Time `json:"resets_at"`
}

type RegenerateRequest struct {
	Category     string   `json:"category"`
	Focus        string   `json:"focus"`
	Difficulty   string   `json:"difficulty"`
	Budget       string   `json:"budget"`
	Context      string   `json:"context"`
	Existing     []string `json:"existing_goals"`
	ReplaceIndex *int     `json:"replace_index"`
}

type RegenerateResponse struct {
	Goal                  string    `json:"goal"`
	EnhancementsRemaining int       `json:"enhancements_remaining"`
	ResetsAt              time.Time `json:"resets_at"`
}

type FillEmptyRequest struct {
	CardID     string `json:"card_id"`
	Category   string `json:"category"`
	Focus      string `json:"focus"`
	Difficulty string `json:"difficulty"`
	Budget     string `json:"budget"`
	Context    string `json:"context"`
}

type FillEmptyResponse struct {
	Card                  any       `json:"card"`
	EnhancementsRemaining int       `json:"enhancements_remaining"`
	ResetsAt              time.Time `json:"resets_at"`
}

func (h *AIHandler) PremiumStatus(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureAIEnhancements) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	limit, used, remaining, resetsAt, err := h.service.GetPremiumEnhancementsStatus(r.Context(), user.ID, time.Now())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "AI usage tracking is temporarily unavailable. Please try again later.")
		return
	}

	writeJSON(w, http.StatusOK, PremiumStatusResponse{
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
		ResetsAt:  resetsAt,
	})
}

func (h *AIHandler) Assist(w http.ResponseWriter, r *http.Request) {
	var req AssistRequest
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CardID == "" || req.Mode == "" || req.Position == nil {
		writeError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	if len(req.Notes) > 500 {
		writeError(w, http.StatusBadRequest, "Notes is too long (max 500 chars)")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if !validAssistModes[mode] {
		writeError(w, http.StatusBadRequest, "Invalid mode")
		return
	}
	cardID, err := uuid.Parse(req.CardID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card_id")
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureAIEnhancements) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	freeConsumed, premiumReserved, premiumReservedAt, remaining, resetsAt, ok := h.consumePremiumAllowanceOrWriteError(w, r, user.ID, user.EmailVerified)
	if !ok {
		return
	}

	reply, _, err := h.service.AssistCardGoal(r.Context(), user.ID, cardID, *req.Position, mode, req.Notes)
	if err != nil {
		h.refundPremiumOnFailure(r.Context(), user.ID, freeConsumed, premiumReserved, premiumReservedAt)
		h.writePremiumAIError(w, err, resetsAt)
		return
	}

	writeJSON(w, http.StatusOK, AssistResponse{
		Reply:                 reply,
		EnhancementsRemaining: remaining,
		ResetsAt:              resetsAt,
	})
}

func (h *AIHandler) Regenerate(w http.ResponseWriter, r *http.Request) {
	var req RegenerateRequest
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Category == "" || req.Difficulty == "" || req.Budget == "" || req.ReplaceIndex == nil {
		writeError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	if !isValidAICategory(req.Category) {
		writeError(w, http.StatusBadRequest, "Invalid category")
		return
	}
	if !isValidAIDifficulty(req.Difficulty) {
		writeError(w, http.StatusBadRequest, "Invalid difficulty")
		return
	}
	if !isValidAIBudget(req.Budget) {
		writeError(w, http.StatusBadRequest, "Invalid budget")
		return
	}
	if len(req.Focus) > 100 {
		writeError(w, http.StatusBadRequest, "Focus is too long (max 100 chars)")
		return
	}
	if len(req.Context) > 500 {
		writeError(w, http.StatusBadRequest, "Context is too long (max 500 chars)")
		return
	}
	if len(req.Existing) == 0 || len(req.Existing) > 24 {
		writeError(w, http.StatusBadRequest, "existing_goals must contain between 1 and 24 goals")
		return
	}
	replaceIndex := *req.ReplaceIndex
	if replaceIndex < 0 || replaceIndex >= len(req.Existing) {
		writeError(w, http.StatusBadRequest, "replace_index is out of range")
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureAIEnhancements) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	freeConsumed, premiumReserved, premiumReservedAt, remaining, resetsAt, ok := h.consumePremiumAllowanceOrWriteError(w, r, user.ID, user.EmailVerified)
	if !ok {
		return
	}

	goal, _, err := h.service.RegenerateGoal(r.Context(), user.ID, ai.GoalPrompt{
		Category:   req.Category,
		Focus:      req.Focus,
		Difficulty: req.Difficulty,
		Budget:     req.Budget,
		Context:    req.Context,
	}, req.Existing, replaceIndex)
	if err != nil {
		h.refundPremiumOnFailure(r.Context(), user.ID, freeConsumed, premiumReserved, premiumReservedAt)
		h.writePremiumAIError(w, err, resetsAt)
		return
	}

	writeJSON(w, http.StatusOK, RegenerateResponse{
		Goal:                  goal,
		EnhancementsRemaining: remaining,
		ResetsAt:              resetsAt,
	})
}

func (h *AIHandler) FillEmpty(w http.ResponseWriter, r *http.Request) {
	var req FillEmptyRequest
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.CardID == "" || req.Category == "" || req.Difficulty == "" || req.Budget == "" {
		writeError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	if !isValidAICategory(req.Category) {
		writeError(w, http.StatusBadRequest, "Invalid category")
		return
	}
	if !isValidAIDifficulty(req.Difficulty) {
		writeError(w, http.StatusBadRequest, "Invalid difficulty")
		return
	}
	if !isValidAIBudget(req.Budget) {
		writeError(w, http.StatusBadRequest, "Invalid budget")
		return
	}
	if len(req.Focus) > 100 {
		writeError(w, http.StatusBadRequest, "Focus is too long (max 100 chars)")
		return
	}
	if len(req.Context) > 500 {
		writeError(w, http.StatusBadRequest, "Context is too long (max 500 chars)")
		return
	}

	cardID, err := uuid.Parse(req.CardID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card_id")
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureAIEnhancements) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	freeConsumed, premiumReserved, premiumReservedAt, remaining, resetsAt, ok := h.consumePremiumAllowanceOrWriteError(w, r, user.ID, user.EmailVerified)
	if !ok {
		return
	}

	card, _, err := h.service.FillEmptyOnCard(r.Context(), user.ID, cardID, ai.GoalPrompt{
		Category:   req.Category,
		Focus:      req.Focus,
		Difficulty: req.Difficulty,
		Budget:     req.Budget,
		Context:    req.Context,
	})
	if err != nil {
		h.refundPremiumOnFailure(r.Context(), user.ID, freeConsumed, premiumReserved, premiumReservedAt)
		h.writePremiumAIError(w, err, resetsAt)
		return
	}

	writeJSON(w, http.StatusOK, FillEmptyResponse{
		Card:                  card,
		EnhancementsRemaining: remaining,
		ResetsAt:              resetsAt,
	})
}

func (h *AIHandler) consumePremiumAllowanceOrWriteError(w http.ResponseWriter, r *http.Request, userID uuid.UUID, emailVerified bool) (bool, bool, time.Time, int, time.Time, bool) {
	freeConsumed := false
	if !emailVerified {
		if _, err := h.service.ConsumeUnverifiedFreeGeneration(r.Context(), userID); err != nil {
			switch {
			case errors.Is(err, ai.ErrEmailVerificationRequired):
				zero := 0
				writeJSON(w, http.StatusForbidden, GenerateErrorResponse{
					Error:         "You've used your 5 free AI generations. Verify your email to keep using AI.",
					FreeRemaining: &zero,
				})
			default:
				writeError(w, http.StatusServiceUnavailable, "AI usage tracking is temporarily unavailable. Please try again later.")
			}
			return false, false, time.Time{}, 0, time.Time{}, false
		}
		freeConsumed = true
	}

	reserveAt := time.Now()
	remaining, resetsAt, err := h.service.ReservePremiumEnhancement(r.Context(), userID, reserveAt)
	if err != nil {
		if freeConsumed {
			refundCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = h.service.RefundUnverifiedFreeGeneration(refundCtx, userID)
		}
		switch {
		case errors.Is(err, ai.ErrPremiumEnhancementsExhausted):
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":     "Premium AI enhancements limit reached for this month",
				"resets_at": resetsAt,
			})
		default:
			writeError(w, http.StatusServiceUnavailable, "AI usage tracking is temporarily unavailable. Please try again later.")
		}
		return false, false, time.Time{}, 0, time.Time{}, false
	}

	return freeConsumed, true, reserveAt, remaining, resetsAt, true
}

func (h *AIHandler) refundPremiumOnFailure(ctx context.Context, userID uuid.UUID, freeConsumed, premiumReserved bool, premiumReservedAt time.Time) {
	refundCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if premiumReserved {
		refundAt := premiumReservedAt
		if refundAt.IsZero() {
			refundAt = time.Now()
		}
		_ = h.service.RefundPremiumEnhancement(refundCtx, userID, refundAt)
	}
	if freeConsumed {
		_, _ = h.service.RefundUnverifiedFreeGeneration(refundCtx, userID)
	}
}

func (h *AIHandler) writePremiumAIError(w http.ResponseWriter, err error, resetsAt time.Time) {
	switch {
	case errors.Is(err, ai.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "Invalid AI request.")
	case errors.Is(err, services.ErrInvalidPosition):
		writeError(w, http.StatusBadRequest, "Invalid position")
	case errors.Is(err, services.ErrCardFinalized):
		writeError(w, http.StatusBadRequest, "Card must be a draft")
	case errors.Is(err, services.ErrCardFull):
		writeError(w, http.StatusBadRequest, "Card has no empty positions")
	case errors.Is(err, services.ErrCardNotFound):
		writeError(w, http.StatusNotFound, "Card not found")
	case errors.Is(err, services.ErrItemNotFound):
		writeError(w, http.StatusNotFound, "Goal not found")
	case errors.Is(err, ai.ErrSafetyViolation):
		writeError(w, http.StatusBadRequest, "We couldn't generate safe goals for that topic. Please try rephrasing.")
	case errors.Is(err, ai.ErrRateLimitExceeded):
		writeError(w, http.StatusTooManyRequests, "AI provider rate limit exceeded.")
	case errors.Is(err, ai.ErrPremiumEnhancementsExhausted):
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":     "Premium AI enhancements limit reached for this month",
			"resets_at": resetsAt,
		})
	case errors.Is(err, ai.ErrAINotConfigured):
		writeError(w, http.StatusServiceUnavailable, "AI is not configured on this server. Please try again later.")
	case errors.Is(err, ai.ErrAIProviderUnavailable):
		writeError(w, http.StatusServiceUnavailable, "The AI service is currently down. Please try again later.")
	case errors.Is(err, ai.ErrAIUsageTrackingUnavailable):
		writeError(w, http.StatusServiceUnavailable, "AI usage tracking is temporarily unavailable. Please try again later.")
	default:
		writeError(w, http.StatusInternalServerError, "An unexpected error occurred.")
	}
}

func isValidAICategory(category string) bool {
	switch category {
	case "hobbies", "health", "career", "social", "travel", "mix":
		return true
	default:
		return false
	}
}

func isValidAIDifficulty(difficulty string) bool {
	switch difficulty {
	case "easy", "medium", "hard":
		return true
	default:
		return false
	}
}

func isValidAIBudget(budget string) bool {
	switch budget {
	case "free", "low", "medium", "high":
		return true
	default:
		return false
	}
}
