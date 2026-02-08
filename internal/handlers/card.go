package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type CardHandler struct {
	cardService services.CardServiceInterface
}

func NewCardHandler(cardService services.CardServiceInterface) *CardHandler {
	return &CardHandler{cardService: cardService}
}

type CreateCardRequest struct {
	Year         int     `json:"year"`
	Category     *string `json:"category,omitempty"`
	Title        *string `json:"title,omitempty"`
	GridSize     *int    `json:"grid_size,omitempty"`
	HeaderText   *string `json:"header_text,omitempty"`
	HasFreeSpace *bool   `json:"has_free_space,omitempty"`
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

type FinalizeRequest struct {
	VisibleToFriends *bool `json:"visible_to_friends,omitempty"`
}

type UpdateVisibilityRequest struct {
	VisibleToFriends bool `json:"visible_to_friends"`
}

type BulkUpdateVisibilityRequest struct {
	CardIDs          []string `json:"card_ids"`
	VisibleToFriends bool     `json:"visible_to_friends"`
}

type BulkUpdateVisibilityResponse struct {
	UpdatedCount int `json:"updated_count"`
}

type BulkDeleteRequest struct {
	CardIDs []string `json:"card_ids"`
}

type BulkDeleteResponse struct {
	DeletedCount int `json:"deleted_count"`
}

type BulkUpdateArchiveRequest struct {
	CardIDs    []string `json:"card_ids"`
	IsArchived bool     `json:"is_archived"`
}

type BulkUpdateArchiveResponse struct {
	UpdatedCount int `json:"updated_count"`
}

// ImportCardRequest represents a request to import an anonymous card
type ImportCardRequest struct {
	Year              int              `json:"year"`
	Title             *string          `json:"title,omitempty"`
	Category          *string          `json:"category,omitempty"`
	GridSize          int              `json:"grid_size,omitempty"`
	HeaderText        string           `json:"header_text,omitempty"`
	HasFreeSpace      *bool            `json:"has_free_space,omitempty"`
	FreeSpacePosition *int             `json:"free_space_position,omitempty"`
	Items             []ImportCardItem `json:"items"`
	Finalize          bool             `json:"finalize"`
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
