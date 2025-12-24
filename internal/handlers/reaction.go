package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type ReactionHandler struct {
	reactionService services.ReactionServiceInterface
}

func NewReactionHandler(reactionService services.ReactionServiceInterface) *ReactionHandler {
	return &ReactionHandler{reactionService: reactionService}
}

type AddReactionRequest struct {
	Emoji string `json:"emoji"`
}

type ReactionResponse struct {
	Reaction  *models.Reaction          `json:"reaction,omitempty"`
	Reactions []models.ReactionWithUser `json:"reactions,omitempty"`
	Summary   []models.ReactionSummary  `json:"summary,omitempty"`
	Message   string                    `json:"message,omitempty"`
}

type AllowedEmojisResponse struct {
	Emojis []string `json:"emojis"`
}

func (h *ReactionHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	itemID, err := parseItemID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}

	var req AddReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	reaction, err := h.reactionService.AddReaction(r.Context(), user.ID, itemID, req.Emoji)
	if errors.Is(err, services.ErrInvalidEmoji) {
		writeError(w, http.StatusBadRequest, "Invalid emoji")
		return
	}
	if errors.Is(err, services.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "Item not found")
		return
	}
	if errors.Is(err, services.ErrCannotReactToOwn) {
		writeError(w, http.StatusBadRequest, "Cannot react to your own items")
		return
	}
	if errors.Is(err, services.ErrItemNotCompleted) {
		writeError(w, http.StatusBadRequest, "Can only react to completed items")
		return
	}
	if errors.Is(err, services.ErrNotFriend) {
		writeError(w, http.StatusForbidden, "You must be friends to react")
		return
	}
	if err != nil {
		log.Printf("Error adding reaction: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReactionResponse{Reaction: reaction})
}

func (h *ReactionHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	itemID, err := parseItemID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}

	err = h.reactionService.RemoveReaction(r.Context(), user.ID, itemID)
	if errors.Is(err, services.ErrReactionNotFound) {
		writeError(w, http.StatusNotFound, "Reaction not found")
		return
	}
	if err != nil {
		log.Printf("Error removing reaction: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReactionResponse{Message: "Reaction removed"})
}

func (h *ReactionHandler) GetReactions(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	itemID, err := parseItemID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}

	reactions, err := h.reactionService.GetReactionsForItem(r.Context(), itemID)
	if err != nil {
		log.Printf("Error getting reactions: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	summary, err := h.reactionService.GetReactionSummaryForItem(r.Context(), itemID)
	if err != nil {
		log.Printf("Error getting reaction summary: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ReactionResponse{
		Reactions: reactions,
		Summary:   summary,
	})
}

func (h *ReactionHandler) GetAllowedEmojis(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, AllowedEmojisResponse{Emojis: models.AllowedEmojis})
}

func parseItemID(r *http.Request) (uuid.UUID, error) {
	path := r.URL.Path
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "items" && i+1 < len(parts) {
			return uuid.Parse(parts[i+1])
		}
	}
	return uuid.Nil, errors.New("item ID not found in path")
}
