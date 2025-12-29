package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
)

type GuideRequest struct {
	Mode        string   `json:"mode"`
	CurrentGoal string   `json:"current_goal"`
	Hint        string   `json:"hint"`
	Count       int      `json:"count"`
	Avoid       []string `json:"avoid"`
}

type GuideResponse struct {
	Goals         []string `json:"goals"`
	FreeRemaining *int     `json:"free_remaining,omitempty"`
}

type GuideErrorResponse struct {
	Error         string `json:"error"`
	FreeRemaining *int   `json:"free_remaining,omitempty"`
}

func (h *AIHandler) Guide(w http.ResponseWriter, r *http.Request) {
	var req GuideRequest
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		writeError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	if mode != "refine" && mode != "new" {
		writeError(w, http.StatusBadRequest, "Invalid mode")
		return
	}

	currentGoal := strings.TrimSpace(req.CurrentGoal)
	hint := strings.TrimSpace(req.Hint)
	if mode == "refine" && currentGoal == "" {
		writeError(w, http.StatusBadRequest, "Current goal is required")
		return
	}
	if len(currentGoal) > 500 {
		writeError(w, http.StatusBadRequest, "Current goal is too long (max 500 chars)")
		return
	}
	if len(hint) > 500 {
		writeError(w, http.StatusBadRequest, "Hint is too long (max 500 chars)")
		return
	}

	count := req.Count
	if count == 0 {
		if mode == "refine" {
			count = 3
		} else {
			count = 5
		}
	}
	if count < 1 || count > 5 {
		writeError(w, http.StatusBadRequest, "Count must be between 1 and 5")
		return
	}

	avoid := normalizeGuideAvoidList(req.Avoid)

	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var freeRemaining *int
	freeRemainingValue := 0
	consumedFree := false
	if !user.EmailVerified {
		remaining, err := h.service.ConsumeUnverifiedFreeGeneration(r.Context(), user.ID)
		if err != nil {
			switch {
			case errors.Is(err, ai.ErrEmailVerificationRequired):
				zero := 0
				writeJSON(w, http.StatusForbidden, GuideErrorResponse{
					Error:         "You've used your 5 free AI generations. Verify your email to keep using AI.",
					FreeRemaining: &zero,
				})
				return
			case errors.Is(err, ai.ErrAIUsageTrackingUnavailable):
				writeError(w, http.StatusServiceUnavailable, "AI usage tracking is temporarily unavailable. Please try again later.")
				return
			default:
				writeError(w, http.StatusServiceUnavailable, "AI usage tracking is temporarily unavailable. Please try again later.")
				return
			}
		}
		freeRemainingValue = remaining
		freeRemaining = &freeRemainingValue
		consumedFree = true
	}

	prompt := ai.GuidePrompt{
		Mode:        mode,
		CurrentGoal: currentGoal,
		Hint:        hint,
		Count:       count,
		Avoid:       avoid,
	}

	goals, _, err := h.service.GenerateGuideGoals(r.Context(), user.ID, prompt)
	if err != nil {
		status := http.StatusInternalServerError
		msg := "An unexpected error occurred."

		switch {
		case errors.Is(err, ai.ErrInvalidInput):
			status = http.StatusBadRequest
			msg = "Invalid AI request."
		case errors.Is(err, ai.ErrSafetyViolation):
			status = http.StatusBadRequest
			msg = "We couldn't generate safe goals for that topic. Please try rephrasing."
		case errors.Is(err, ai.ErrRateLimitExceeded):
			status = http.StatusTooManyRequests
			msg = "AI provider rate limit exceeded."
		case errors.Is(err, ai.ErrAINotConfigured):
			status = http.StatusServiceUnavailable
			msg = "AI is not configured on this server. Please try again later."
		case errors.Is(err, ai.ErrAIProviderUnavailable):
			status = http.StatusServiceUnavailable
			msg = "The AI service is currently down. Please try again later."
		}

		if consumedFree && (errors.Is(err, ai.ErrAIProviderUnavailable) || errors.Is(err, ai.ErrAINotConfigured) || errors.Is(err, ai.ErrRateLimitExceeded)) {
			refundCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if refunded, refundErr := h.service.RefundUnverifiedFreeGeneration(refundCtx, user.ID); refundErr == nil && refunded && freeRemaining != nil {
				freeRemainingValue++
			}
		}

		writeJSON(w, status, GuideErrorResponse{
			Error:         msg,
			FreeRemaining: freeRemaining,
		})
		return
	}

	writeJSON(w, http.StatusOK, GuideResponse{
		Goals:         goals,
		FreeRemaining: freeRemaining,
	})
}

func normalizeGuideAvoidList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		clean := strings.TrimSpace(item)
		if clean == "" {
			continue
		}
		clean = truncateGuideRunes(clean, 100)
		if clean == "" {
			continue
		}
		out = append(out, clean)
		if len(out) >= 24 {
			break
		}
	}
	return out
}

func truncateGuideRunes(input string, max int) string {
	if max <= 0 {
		return ""
	}
	if len([]rune(input)) <= max {
		return input
	}
	return string([]rune(input)[:max])
}
