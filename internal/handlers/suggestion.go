package handlers

import (
	"log"
	"net/http"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type SuggestionHandler struct {
	suggestionService *services.SuggestionService
}

func NewSuggestionHandler(suggestionService *services.SuggestionService) *SuggestionHandler {
	return &SuggestionHandler{suggestionService: suggestionService}
}

type SuggestionsResponse struct {
	Categories  []string                         `json:"categories,omitempty"`
	Suggestions []*models.Suggestion             `json:"suggestions,omitempty"`
	Grouped     []services.SuggestionsByCategory `json:"grouped,omitempty"`
}

func (h *SuggestionHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	// Check if grouped parameter is set
	grouped := r.URL.Query().Get("grouped") == "true"
	category := r.URL.Query().Get("category")

	if grouped {
		groupedSuggestions, err := h.suggestionService.GetGroupedByCategory(r.Context())
		if err != nil {
			log.Printf("Error getting grouped suggestions: %v", err)
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		writeJSON(w, http.StatusOK, SuggestionsResponse{Grouped: groupedSuggestions})
		return
	}

	if category != "" {
		suggestions, err := h.suggestionService.GetByCategory(r.Context(), category)
		if err != nil {
			log.Printf("Error getting suggestions by category: %v", err)
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		writeJSON(w, http.StatusOK, SuggestionsResponse{Suggestions: suggestions})
		return
	}

	suggestions, err := h.suggestionService.GetAll(r.Context())
	if err != nil {
		log.Printf("Error getting suggestions: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, SuggestionsResponse{Suggestions: suggestions})
}

func (h *SuggestionHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.suggestionService.GetCategories(r.Context())
	if err != nil {
		log.Printf("Error getting categories: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, SuggestionsResponse{Categories: categories})
}
