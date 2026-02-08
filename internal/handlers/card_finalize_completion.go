package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
	"github.com/HammerMeetNail/yearofbingo/internal/services/billing"
)

func (h *CardHandler) EditFinalized(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if !billing.HasFeature(user, time.Now(), billing.FeatureEditAfterFinalize) {
		writeError(w, http.StatusForbidden, "Premium required")
		return
	}

	cardID, err := parseCardID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	req := EditFinalizedCardRequest{}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		req.Title = &trimmed
	}

	params := services.EditFinalizedCardParams{
		Title:         req.Title,
		ShuffleLayout: req.ShuffleLayout != nil && *req.ShuffleLayout,
		ResetProgress: true,
	}
	if req.ResetProgress != nil {
		params.ResetProgress = *req.ResetProgress
	}

	card, err := h.cardService.EditFinalized(r.Context(), user.ID, cardID, params)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrCardNotFinalized) {
		writeError(w, http.StatusBadRequest, "Card must be finalized first")
		return
	}
	if errors.Is(err, services.ErrTitleTooLong) {
		writeError(w, http.StatusBadRequest, "Title must be 100 characters or less")
		return
	}

	var conflict *services.CardConflictError
	if errors.As(err, &conflict) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(CardConflictResponse{
			Error:          "Card conflict",
			Conflict:       CardConflictInfo{Year: conflict.Year, Title: conflict.Title},
			SuggestedTitle: conflict.SuggestedTitle,
		})
		return
	}

	if errors.Is(err, services.ErrCardTitleExists) || errors.Is(err, services.ErrCardAlreadyExists) {
		writeError(w, http.StatusConflict, "You already have a card with this title for this year")
		return
	}

	if err != nil {
		logError("Error creating editable draft from finalized card", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CardResponse{Card: card})
}

func (h *CardHandler) Finalize(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardID, err := parseCardID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	// Parse optional request body for visibility setting
	var req FinalizeRequest
	var params *services.FinalizeParams
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		params = &services.FinalizeParams{
			VisibleToFriends: req.VisibleToFriends,
		}
	}

	card, err := h.cardService.Finalize(r.Context(), user.ID, cardID, params)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if err != nil {
		logError("Error finalizing card", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Card: card})
}

func (h *CardHandler) CompleteItem(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardID, err := parseCardID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	position, err := parsePosition(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid position")
		return
	}

	var req CompleteItemRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	item, err := h.cardService.CompleteItem(r.Context(), user.ID, cardID, position, models.CompleteItemParams{
		Notes:    req.Notes,
		ProofURL: req.ProofURL,
	})
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "Item not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrCardNotFinalized) {
		writeError(w, http.StatusBadRequest, "Card must be finalized first")
		return
	}
	if err != nil {
		logError("Error completing item", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Item: item})
}

func (h *CardHandler) UncompleteItem(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardID, err := parseCardID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	position, err := parsePosition(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid position")
		return
	}

	item, err := h.cardService.UncompleteItem(r.Context(), user.ID, cardID, position)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "Item not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrCardNotFinalized) {
		writeError(w, http.StatusBadRequest, "Card must be finalized first")
		return
	}
	if err != nil {
		logError("Error uncompleting item", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Item: item})
}

func (h *CardHandler) UpdateNotes(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cardID, err := parseCardID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid card ID")
		return
	}

	position, err := parsePosition(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid position")
		return
	}

	var req UpdateNotesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	item, err := h.cardService.UpdateItemNotes(r.Context(), user.ID, cardID, position, req.Notes, req.ProofURL)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, "Item not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if err != nil {
		logError("Error updating notes", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Item: item})
}
