package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

func (h *CardHandler) AddItem(w http.ResponseWriter, r *http.Request) {
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

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "Content is required")
		return
	}
	if len(req.Content) > 500 {
		writeError(w, http.StatusBadRequest, "Content must be 500 characters or less")
		return
	}

	item, err := h.cardService.AddItem(r.Context(), user.ID, models.AddItemParams{
		CardID:   cardID,
		Content:  req.Content,
		Position: req.Position,
	})
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrCardFinalized) {
		writeError(w, http.StatusBadRequest, "Card is finalized and cannot be modified")
		return
	}
	if errors.Is(err, services.ErrCardFull) {
		writeError(w, http.StatusBadRequest, "Card is full")
		return
	}
	if errors.Is(err, services.ErrPositionOccupied) {
		writeError(w, http.StatusConflict, "Position is already occupied")
		return
	}
	if errors.Is(err, services.ErrInvalidPosition) {
		writeError(w, http.StatusBadRequest, "Invalid position")
		return
	}
	if err != nil {
		logError("Error adding item", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CardResponse{Item: item})
}

type UpdateCardConfigRequest struct {
	HeaderText   *string `json:"header_text,omitempty"`
	HasFreeSpace *bool   `json:"has_free_space,omitempty"`
}

func (h *CardHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateCardConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.HeaderText != nil {
		trimmed := strings.TrimSpace(*req.HeaderText)
		req.HeaderText = &trimmed
	}

	card, err := h.cardService.UpdateConfig(r.Context(), user.ID, cardID, models.UpdateCardConfigParams{
		HeaderText:   req.HeaderText,
		HasFreeSpace: req.HasFreeSpace,
	})
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrCardFinalized) {
		writeError(w, http.StatusBadRequest, "Card is finalized and cannot be modified")
		return
	}
	if errors.Is(err, services.ErrInvalidHeaderText) {
		writeError(w, http.StatusBadRequest, "Invalid header text")
		return
	}
	if errors.Is(err, services.ErrNoSpaceForFree) {
		writeError(w, http.StatusBadRequest, "Your card is full. Remove an item to add or move the FREE space.")
		return
	}
	if err != nil {
		logError("Error updating card config", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Card: card})
}

type CloneCardRequest struct {
	Year         *int    `json:"year,omitempty"`
	Title        *string `json:"title,omitempty"`
	Category     *string `json:"category,omitempty"`
	GridSize     int     `json:"grid_size,omitempty"`
	HeaderText   *string `json:"header_text,omitempty"`
	HasFreeSpace *bool   `json:"has_free_space,omitempty"`
}

type EditFinalizedCardRequest struct {
	Title         *string `json:"title,omitempty"`
	ShuffleLayout *bool   `json:"shuffle_layout,omitempty"`
	ResetProgress *bool   `json:"reset_progress,omitempty"`
}

func (h *CardHandler) Clone(w http.ResponseWriter, r *http.Request) {
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

	var req CloneCardRequest
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
	if req.HeaderText != nil {
		trimmed := strings.TrimSpace(*req.HeaderText)
		req.HeaderText = &trimmed
	}

	header := ""
	if req.HeaderText != nil {
		header = *req.HeaderText
	}

	result, err := h.cardService.Clone(r.Context(), user.ID, cardID, services.CloneParams{
		Year:         req.Year,
		Title:        req.Title,
		Category:     req.Category,
		GridSize:     req.GridSize,
		HeaderText:   header,
		HasFreeSpace: req.HasFreeSpace,
	})
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
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
	if errors.Is(err, services.ErrCardTitleExists) {
		writeError(w, http.StatusConflict, "You already have a card with this title for this year")
		return
	}
	if errors.Is(err, services.ErrCardAlreadyExists) {
		writeError(w, http.StatusConflict, "You already have a card for this year. Give your new card a unique title.")
		return
	}
	if err != nil {
		logError("Error cloning card", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	message := "Card cloned"
	if result.TruncatedItemCount > 0 {
		message = "Card cloned (some items were not copied because the new grid is smaller)"
	}
	writeJSON(w, http.StatusCreated, CardResponse{Card: result.Card, Message: message})
}

func (h *CardHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content != nil {
		*req.Content = strings.TrimSpace(*req.Content)
		if *req.Content == "" {
			writeError(w, http.StatusBadRequest, "Content cannot be empty")
			return
		}
		if len(*req.Content) > 500 {
			writeError(w, http.StatusBadRequest, "Content must be 500 characters or less")
			return
		}
	}

	item, err := h.cardService.UpdateItem(r.Context(), user.ID, cardID, position, models.UpdateItemParams{
		Content:  req.Content,
		Position: req.Position,
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
	if errors.Is(err, services.ErrCardFinalized) {
		writeError(w, http.StatusBadRequest, "Card is finalized and cannot be modified")
		return
	}
	if errors.Is(err, services.ErrPositionOccupied) {
		writeError(w, http.StatusConflict, "Position is already occupied")
		return
	}
	if errors.Is(err, services.ErrInvalidPosition) {
		writeError(w, http.StatusBadRequest, "Invalid position")
		return
	}
	if err != nil {
		logError("Error updating item", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Item: item})
}

func (h *CardHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
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

	err = h.cardService.RemoveItem(r.Context(), user.ID, cardID, position)
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
	if errors.Is(err, services.ErrCardFinalized) {
		writeError(w, http.StatusBadRequest, "Card is finalized and cannot be modified")
		return
	}
	if err != nil {
		logError("Error removing item", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Message: "Item removed"})
}

func (h *CardHandler) Shuffle(w http.ResponseWriter, r *http.Request) {
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

	card, err := h.cardService.Shuffle(r.Context(), user.ID, cardID)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if errors.Is(err, services.ErrCardFinalized) {
		writeError(w, http.StatusBadRequest, "Card is finalized and cannot be modified")
		return
	}
	if err != nil {
		logError("Error shuffling card", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Card: card})
}

type SwapRequest struct {
	Position1 int `json:"position1"`
	Position2 int `json:"position2"`
}

func (h *CardHandler) SwapItems(w http.ResponseWriter, r *http.Request) {
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

	var req SwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err = h.cardService.SwapItems(r.Context(), user.ID, cardID, req.Position1, req.Position2)
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
	if errors.Is(err, services.ErrCardFinalized) {
		writeError(w, http.StatusBadRequest, "Card is finalized and cannot be modified")
		return
	}
	if errors.Is(err, services.ErrInvalidPosition) {
		writeError(w, http.StatusBadRequest, "Invalid position")
		return
	}
	if errors.Is(err, services.ErrNoSpaceForFree) {
		writeError(w, http.StatusBadRequest, "Your card is full. Remove an item to move the FREE space.")
		return
	}
	if err != nil {
		logError("Error swapping items", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Message: "Items swapped"})
}

func (h *CardHandler) UpdateMeta(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateCardMetaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Trim title if provided
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		req.Title = &trimmed
	}

	card, err := h.cardService.UpdateMeta(r.Context(), user.ID, cardID, models.UpdateCardMetaParams{
		Category: req.Category,
		Title:    req.Title,
	})
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
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
	if err != nil {
		logError("Error updating card meta", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Card: card})
}

func (h *CardHandler) UpdateVisibility(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateVisibilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	card, err := h.cardService.UpdateVisibility(r.Context(), user.ID, cardID, req.VisibleToFriends)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if err != nil {
		logError("Error updating visibility", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Card: card})
}
