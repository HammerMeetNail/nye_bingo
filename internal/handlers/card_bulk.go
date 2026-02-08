package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func (h *CardHandler) Archive(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cards, err := h.cardService.GetArchive(r.Context(), user.ID)
	if err != nil {
		logError("Error getting archive", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if cards == nil {
		cards = []*models.BingoCard{}
	}

	writeJSON(w, http.StatusOK, CardResponse{Cards: cards})
}

func (h *CardHandler) BulkUpdateVisibility(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req BulkUpdateVisibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.CardIDs) == 0 {
		writeError(w, http.StatusBadRequest, "At least one card ID is required")
		return
	}

	// Parse card IDs
	cardIDs := make([]uuid.UUID, 0, len(req.CardIDs))
	for _, idStr := range req.CardIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid card ID: "+idStr)
			return
		}
		cardIDs = append(cardIDs, id)
	}

	count, err := h.cardService.BulkUpdateVisibility(r.Context(), user.ID, cardIDs, req.VisibleToFriends)
	if err != nil {
		logError("Error bulk updating visibility", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, BulkUpdateVisibilityResponse{UpdatedCount: count})
}

func (h *CardHandler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req BulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.CardIDs) == 0 {
		writeError(w, http.StatusBadRequest, "At least one card ID is required")
		return
	}

	// Parse card IDs
	cardIDs := make([]uuid.UUID, 0, len(req.CardIDs))
	for _, idStr := range req.CardIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid card ID: "+idStr)
			return
		}
		cardIDs = append(cardIDs, id)
	}

	count, err := h.cardService.BulkDelete(r.Context(), user.ID, cardIDs)
	if err != nil {
		logError("Error bulk deleting cards", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, BulkDeleteResponse{DeletedCount: count})
}

func (h *CardHandler) BulkUpdateArchive(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req BulkUpdateArchiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.CardIDs) == 0 {
		writeError(w, http.StatusBadRequest, "At least one card ID is required")
		return
	}

	// Parse card IDs
	cardIDs := make([]uuid.UUID, 0, len(req.CardIDs))
	for _, idStr := range req.CardIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid card ID: "+idStr)
			return
		}
		cardIDs = append(cardIDs, id)
	}

	count, err := h.cardService.BulkUpdateArchive(r.Context(), user.ID, cardIDs, req.IsArchived)
	if err != nil {
		logError("Error bulk updating archive status", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, BulkUpdateArchiveResponse{UpdatedCount: count})
}
