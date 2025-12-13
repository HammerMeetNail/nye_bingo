package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
)

type AIService interface {
	GenerateGoals(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error)
	ConsumeUnverifiedFreeGeneration(ctx context.Context, userID uuid.UUID) (int, error)
}

type AIHandler struct {
	service AIService
}

func NewAIHandler(service AIService) *AIHandler {
	return &AIHandler{service: service}
}

type GenerateRequest struct {
	Category   string `json:"category"`
	Focus      string `json:"focus"`
	Difficulty string `json:"difficulty"`
	Budget     string `json:"budget"`
	Context    string `json:"context"`
}

type GenerateResponse struct {
	Goals         []string `json:"goals"`
	FreeRemaining *int     `json:"free_remaining,omitempty"`
}

type GenerateErrorResponse struct {
	Error         string `json:"error"`
	FreeRemaining *int   `json:"free_remaining,omitempty"`
}

func (h *AIHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Category == "" || req.Difficulty == "" || req.Budget == "" {
		writeError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	// Input Validation
	validCategories := map[string]bool{"hobbies": true, "health": true, "career": true, "social": true, "travel": true, "mix": true}
	if !validCategories[req.Category] {
		writeError(w, http.StatusBadRequest, "Invalid category")
		return
	}

	validDifficulties := map[string]bool{"easy": true, "medium": true, "hard": true}
	if !validDifficulties[req.Difficulty] {
		writeError(w, http.StatusBadRequest, "Invalid difficulty")
		return
	}

	validBudgets := map[string]bool{"free": true, "low": true, "medium": true, "high": true}
	if !validBudgets[req.Budget] {
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

	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var freeRemaining *int
	if !user.EmailVerified {
		remaining, err := h.service.ConsumeUnverifiedFreeGeneration(r.Context(), user.ID)
		if err != nil {
			switch {
			case errors.Is(err, ai.ErrEmailVerificationRequired):
				zero := 0
				writeJSON(w, http.StatusForbidden, GenerateErrorResponse{
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
		freeRemaining = &remaining
	}

	prompt := ai.GoalPrompt{
		Category:   req.Category,
		Focus:      req.Focus,
		Difficulty: req.Difficulty,
		Budget:     req.Budget,
		Context:    req.Context,
	}

	goals, _, err := h.service.GenerateGoals(r.Context(), user.ID, prompt)
	if err != nil {
		status := http.StatusInternalServerError
		msg := "An unexpected error occurred."

		switch {
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

		writeJSON(w, status, GenerateErrorResponse{
			Error:         msg,
			FreeRemaining: freeRemaining,
		})
		return
	}

	writeJSON(w, http.StatusOK, GenerateResponse{
		Goals:         goals,
		FreeRemaining: freeRemaining,
	})
}
