package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
