package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type CardHandler struct {
	cardService *services.CardService
}

func NewCardHandler(cardService *services.CardService) *CardHandler {
	return &CardHandler{cardService: cardService}
}

type CreateCardRequest struct {
	Year     int     `json:"year"`
	Category *string `json:"category,omitempty"`
	Title    *string `json:"title,omitempty"`
}

type UpdateCardMetaRequest struct {
	Category *string `json:"category,omitempty"`
	Title    *string `json:"title,omitempty"`
}

type CategoryInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CategoriesResponse struct {
	Categories []CategoryInfo `json:"categories"`
}

type AddItemRequest struct {
	Content  string `json:"content"`
	Position *int   `json:"position,omitempty"`
}

type UpdateItemRequest struct {
	Content  *string `json:"content,omitempty"`
	Position *int    `json:"position,omitempty"`
}

type CompleteItemRequest struct {
	Notes    *string `json:"notes,omitempty"`
	ProofURL *string `json:"proof_url,omitempty"`
}

type UpdateNotesRequest struct {
	Notes    *string `json:"notes,omitempty"`
	ProofURL *string `json:"proof_url,omitempty"`
}

// ImportCardRequest represents a request to import an anonymous card
type ImportCardRequest struct {
	Year     int              `json:"year"`
	Title    *string          `json:"title,omitempty"`
	Category *string          `json:"category,omitempty"`
	Items    []ImportCardItem `json:"items"`
	Finalize bool             `json:"finalize"`
}

type ImportCardItem struct {
	Position int    `json:"position"`
	Content  string `json:"content"`
}

// ImportCardResponse includes conflict info when a card already exists
type ImportCardResponse struct {
	Card         *models.BingoCard `json:"card,omitempty"`
	Error        string            `json:"error,omitempty"`
	Message      string            `json:"message,omitempty"`
	ExistingCard *ExistingCardInfo `json:"existing_card,omitempty"`
}

type ExistingCardInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	Year        int    `json:"year"`
	ItemCount   int    `json:"item_count"`
	IsFinalized bool   `json:"is_finalized"`
}

type CardResponse struct {
	Card    *models.BingoCard   `json:"card,omitempty"`
	Cards   []*models.BingoCard `json:"cards,omitempty"`
	Item    *models.BingoItem   `json:"item,omitempty"`
	Stats   *models.CardStats   `json:"stats,omitempty"`
	Message string              `json:"message,omitempty"`
}

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

	card, err := h.cardService.Create(r.Context(), models.CreateCardParams{
		UserID:   user.ID,
		Year:     req.Year,
		Category: req.Category,
		Title:    req.Title,
	})
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
	if err != nil {
		log.Printf("Error creating card: %v", err)
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
		log.Printf("Error listing cards: %v", err)
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
		log.Printf("Error getting card: %v", err)
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
		log.Printf("Error deleting card: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Message: "Card deleted"})
}

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
		writeError(w, http.StatusBadRequest, "Card already has 24 items")
		return
	}
	if errors.Is(err, services.ErrPositionOccupied) {
		writeError(w, http.StatusBadRequest, "Position is already occupied")
		return
	}
	if errors.Is(err, services.ErrInvalidPosition) {
		writeError(w, http.StatusBadRequest, "Invalid position")
		return
	}
	if err != nil {
		log.Printf("Error adding item: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CardResponse{Item: item})
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
		writeError(w, http.StatusBadRequest, "Position is already occupied")
		return
	}
	if errors.Is(err, services.ErrInvalidPosition) {
		writeError(w, http.StatusBadRequest, "Invalid position")
		return
	}
	if err != nil {
		log.Printf("Error updating item: %v", err)
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
		log.Printf("Error removing item: %v", err)
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
		log.Printf("Error shuffling card: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Card: card})
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

	card, err := h.cardService.Finalize(r.Context(), user.ID, cardID)
	if errors.Is(err, services.ErrCardNotFound) {
		writeError(w, http.StatusNotFound, "Card not found")
		return
	}
	if errors.Is(err, services.ErrNotCardOwner) {
		writeError(w, http.StatusForbidden, "Access denied")
		return
	}
	if err != nil {
		log.Printf("Error finalizing card: %v", err)
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
		log.Printf("Error completing item: %v", err)
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
		log.Printf("Error uncompleting item: %v", err)
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
		log.Printf("Error updating notes: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Item: item})
}

func parseCardID(r *http.Request) (uuid.UUID, error) {
	// Extract card ID from path: /api/cards/{id}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "cards" && i+1 < len(parts) {
			return uuid.Parse(parts[i+1])
		}
	}
	return uuid.Nil, errors.New("card ID not found in path")
}

func parsePosition(r *http.Request) (int, error) {
	// Extract position from path: /api/cards/{id}/items/{pos}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "items" && i+1 < len(parts) {
			return strconv.Atoi(parts[i+1])
		}
	}
	return 0, errors.New("position not found in path")
}

func (h *CardHandler) Archive(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	cards, err := h.cardService.GetArchive(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error getting archive: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if cards == nil {
		cards = []*models.BingoCard{}
	}

	writeJSON(w, http.StatusOK, CardResponse{Cards: cards})
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
		log.Printf("Error getting stats: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Stats: stats})
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
		log.Printf("Error updating card meta: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, CardResponse{Card: card})
}

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
		log.Printf("Error listing current cards for export: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Get archived cards
	archivedCards, err := h.cardService.GetArchive(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error listing archived cards for export: %v", err)
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

	// Validate year
	currentYear := time.Now().Year()
	if req.Year < 2020 || req.Year > currentYear+1 {
		writeError(w, http.StatusBadRequest, "Year must be between 2020 and next year")
		return
	}

	// Validate items count
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "At least one item is required")
		return
	}
	if len(req.Items) > 24 {
		writeError(w, http.StatusBadRequest, "Cannot import more than 24 items")
		return
	}

	// If finalizing, must have exactly 24 items
	if req.Finalize && len(req.Items) != 24 {
		writeError(w, http.StatusBadRequest, "Card must have exactly 24 items to finalize")
		return
	}

	// Check for existing card for this year/title
	existingCard, err := h.cardService.CheckForConflict(r.Context(), user.ID, req.Year, req.Title)
	if err != nil && !errors.Is(err, services.ErrCardNotFound) {
		log.Printf("Error checking for conflict: %v", err)
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
		UserID:   user.ID,
		Year:     req.Year,
		Title:    req.Title,
		Category: req.Category,
		Items:    items,
		Finalize: req.Finalize,
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
	if err != nil {
		log.Printf("Error importing card: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, CardResponse{Card: card})
}
