package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/HammerMeetNail/yearofbingo/internal/services/ai"
)

type AIHandler struct {
	service *ai.Service
}

func NewAIHandler(service *ai.Service) *AIHandler {
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

	if req.Category == "" {
		writeError(w, http.StatusBadRequest, "Category is required")
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
