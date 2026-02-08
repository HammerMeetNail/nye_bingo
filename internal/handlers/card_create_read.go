package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func (h *CardHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req CreateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.Title = normalizeOptionalString(req.Title)
	req.Category = normalizeOptionalString(req.Category)

	// Default to current year if not specified
	if req.Year == 0 {
		req.Year = time.Now().Year()
	}

	// Validate year (reasonable range: 2020 to next year)
	currentYear := time.Now().Year()
	if req.Year < 2020 || req.Year > currentYear+1 {
		writeError(w, http.StatusBadRequest, "Year must be between 2020 and next year")
		return
	}

	gridSize := models.MaxGridSize
	if req.GridSize != nil {
		gridSize = *req.GridSize
	}
	if !models.IsValidGridSize(gridSize) {
		writeError(w, http.StatusBadRequest, "Grid size must be 2, 3, 4, or 5")
		return
	}

	hasFreeSpace := true
	if req.HasFreeSpace != nil {
		hasFreeSpace = *req.HasFreeSpace
	}

	headerText := models.DefaultHeaderText(gridSize)
	if req.HeaderText != nil {
		headerText = *req.HeaderText
	}
	headerText = models.NormalizeHeaderText(headerText)
	if err := models.ValidateHeaderText(headerText, gridSize); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check for existing card for this year/title before attempting create
	existingCard, err := h.cardService.CheckForConflict(r.Context(), user.ID, req.Year, req.Title)
	if err != nil && !errors.Is(err, services.ErrCardNotFound) {
		logError("Error checking for conflict", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if existingCard != nil {
		// Return conflict response with existing card info
		title := ""
		if existingCard.Title != nil {
			title = *existingCard.Title
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(ImportCardResponse{
			Error:   "card_exists",
			Message: "You already have a card for this year",
			ExistingCard: &ExistingCardInfo{
				ID:          existingCard.ID.String(),
				Title:       title,
				Year:        existingCard.Year,
				ItemCount:   len(existingCard.Items),
				IsFinalized: existingCard.IsFinalized,
			},
		})
		return
	}

	card, err := h.cardService.Create(r.Context(), models.CreateCardParams{
		UserID:   user.ID,
		Year:     req.Year,
		Category: req.Category,
		Title:    req.Title,
		GridSize: gridSize,
		Header:   headerText,
		HasFree:  hasFreeSpace,
	})
	// These errors shouldn't happen since we checked above, but handle gracefully
	if errors.Is(err, services.ErrCardAlreadyExists) {
		writeError(w, http.StatusConflict, "You already have a card for this year. Give your new card a unique title.")
		return
	}
	if errors.Is(err, services.ErrCardTitleExists) {
		writeError(w, http.StatusConflict, "You already have a card with this title for this year")
		return
	}
	if errors.Is(err, services.ErrInvalidCategory) {
		writeError(w, http.StatusBadRequest, "Invalid category")
		return
	}
	if errors.Is(err, services.ErrTitleTooLong) {
		writeError(w, http.StatusBadRequest, "Title must be 100 characters or less")
		return
	}
	if errors.Is(err, services.ErrInvalidGridSize) {
		writeError(w, http.StatusBadRequest, "Grid size must be 2, 3, 4, or 5")
		return
	}
	if errors.Is(err, services.ErrInvalidHeaderText) {
		writeError(w, http.StatusBadRequest, "Invalid header text")
		return
	}
	if err != nil {
		logError("Error creating card", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CardResponse{Card: card})
}

func (h *CardHandler) List(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cards, err := h.cardService.ListByUser(r.Context(), user.ID)
	if err != nil {
		logError("Error listing cards", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if cards == nil {
		cards = []*models.BingoCard{}
	}

	writeJSON(w, http.StatusOK, CardResponse{Cards: cards})
}

func (h *CardHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	card, err := h.cardService.GetByID(r.Context(), cardID)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if err != nil {
		logError("Error getting card", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Only owner can view their own card (friends view handled separately)
	if card.UserID != user.ID {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Card: card})
}

func (h *CardHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	err = h.cardService.Delete(r.Context(), user.ID, cardID)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if err != nil {
		logError("Error deleting card", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Message: "Card deleted"})
}

func (h *CardHandler) Stats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.cardService.GetStats(r.Context(), user.ID, cardID)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if err != nil {
		logError("Error getting stats", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Stats: stats})
}
