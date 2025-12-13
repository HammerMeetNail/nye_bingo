package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
)

type AIService interface {
	GenerateGoals(ctx context.Context, userID uuid.UUID, prompt ai.GoalPrompt) ([]string, ai.UsageStats, error)
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

func (h *AIHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
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

		switch err {
		case ai.ErrSafetyViolation:
			status = http.StatusBadRequest
			msg = "We couldn't generate safe goals for that topic. Please try rephrasing."
		case ai.ErrRateLimitExceeded:
			status = http.StatusTooManyRequests
			msg = "AI provider rate limit exceeded."
		case ai.ErrAIProviderUnavailable:
			status = http.StatusServiceUnavailable
			msg = "The AI service is currently down. Please try again later."
		}

		writeError(w, status, msg)
		return
	}

	response := map[string]interface{}{
		"goals": goals,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
