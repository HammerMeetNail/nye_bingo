package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func (h *CardHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories := make([]CategoryInfo, len(models.ValidCategories))
	for i, cat := range models.ValidCategories {
		categories[i] = CategoryInfo{
			ID:   cat,
			Name: models.CategoryNames[cat],
		}
	}
	writeJSON(w, http.StatusOK, CategoriesResponse{Categories: categories})
}

func (h *CardHandler) ListExportable(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Get current year cards
	currentCards, err := h.cardService.ListByUser(r.Context(), user.ID)
	if err != nil {
		logError("Error listing current cards for export", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Get archived cards
	archivedCards, err := h.cardService.GetArchive(r.Context(), user.ID)
	if err != nil {
		logError("Error listing archived cards for export", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Combine all cards
	allCards := make([]*models.BingoCard, 0, len(currentCards)+len(archivedCards))
	allCards = append(allCards, currentCards...)
	allCards = append(allCards, archivedCards...)

	if allCards == nil {
		allCards = []*models.BingoCard{}
	}

	writeJSON(w, http.StatusOK, CardResponse{Cards: allCards})
}

// Import imports an anonymous card, creating the card and all items in one transaction
func (h *CardHandler) Import(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req ImportCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.Title = normalizeOptionalString(req.Title)
	req.Category = normalizeOptionalString(req.Category)

	// Validate year
	currentYear := time.Now().Year()
	if req.Year < 2020 || req.Year > currentYear+1 {
		writeError(w, http.StatusBadRequest, "Year must be between 2020 and next year")
		return
	}

	gridSize := req.GridSize
	if gridSize == 0 {
		gridSize = models.MaxGridSize
	}
	if !models.IsValidGridSize(gridSize) {
		writeError(w, http.StatusBadRequest, "Grid size must be 2, 3, 4, or 5")
		return
	}

	hasFreeSpace := true
	if req.HasFreeSpace != nil {
		hasFreeSpace = *req.HasFreeSpace
	}

	headerText := req.HeaderText
	if headerText == "" {
		headerText = models.DefaultHeaderText(gridSize)
	}
	headerText = models.NormalizeHeaderText(headerText)
	if err := models.ValidateHeaderText(headerText, gridSize); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate items count
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "At least one item is required")
		return
	}

	totalSquares := gridSize * gridSize
	maxItems := totalSquares
	if hasFreeSpace {
		maxItems = totalSquares - 1
	}

	if len(req.Items) > maxItems {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Cannot import more than %d items for a %dx%d card", maxItems, gridSize, gridSize))
		return
	}

	if req.Finalize && len(req.Items) != maxItems {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Card must have exactly %d items to finalize", maxItems))
		return
	}

	// Check for existing card for this year/title
	existingCard, err := h.cardService.CheckForConflict(r.Context(), user.ID, req.Year, req.Title)
	if err != nil && !errors.Is(err, services.ErrCardNotFound) {
		logError("Error checking for conflict", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if existingCard != nil {
		// Return conflict response
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

	// Convert request items to models
	items := make([]models.ImportItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = models.ImportItem{
			Position: item.Position,
			Content:  item.Content,
		}
	}

	// Import the card
	card, err := h.cardService.Import(r.Context(), models.ImportCardParams{
		UserID:       user.ID,
		Year:         req.Year,
		Title:        req.Title,
		Category:     req.Category,
		Items:        items,
		Finalize:     req.Finalize,
		GridSize:     gridSize,
		HeaderText:   headerText,
		HasFreeSpace: hasFreeSpace,
		FreeSpacePos: req.FreeSpacePosition,
	})
	if errors.Is(err, services.ErrInvalidCategory) {
		writeError(w, http.StatusBadRequest, "Invalid category")
		return
	}
	if errors.Is(err, services.ErrTitleTooLong) {
		writeError(w, http.StatusBadRequest, "Title must be 100 characters or less")
		return
	}
	if errors.Is(err, services.ErrInvalidPosition) {
		writeError(w, http.StatusBadRequest, "Invalid item position")
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
		logError("Error importing card", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CardResponse{Card: card})
}
