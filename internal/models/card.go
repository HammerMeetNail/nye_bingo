package models

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MinGridSize = 2
	MaxGridSize = 5
)

func IsValidGridSize(n int) bool {
	return n >= MinGridSize && n <= MaxGridSize
}

func DefaultHeaderText(gridSize int) string {
	if !IsValidGridSize(gridSize) {
		gridSize = MaxGridSize
	}
	base := "BINGO"
	if gridSize >= len(base) {
		return base
	}
	return base[:gridSize]
}

func NormalizeHeaderText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	return s
}

func ValidateHeaderText(headerText string, gridSize int) error {
	if !IsValidGridSize(gridSize) {
		return fmt.Errorf("invalid grid size")
	}
	headerText = NormalizeHeaderText(headerText)
	n := utf8.RuneCountInString(headerText)
	if n < 1 || n > gridSize {
		return fmt.Errorf("header must be between 1 and %d characters", gridSize)
	}
	return nil
}

type BingoCard struct {
	ID               uuid.UUID   `json:"id"`
	UserID           uuid.UUID   `json:"user_id"`
	Year             int         `json:"year"`
	Category         *string     `json:"category,omitempty"`
	Title            *string     `json:"title,omitempty"`
	GridSize         int         `json:"grid_size"`
	HeaderText       string      `json:"header_text"`
	HasFreeSpace     bool        `json:"has_free_space"`
	FreeSpacePos     *int        `json:"free_space_position,omitempty"`
	IsActive         bool        `json:"is_active"`
	IsFinalized      bool        `json:"is_finalized"`
	VisibleToFriends bool        `json:"visible_to_friends"`
	IsArchived       bool        `json:"is_archived"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	Items            []BingoItem `json:"items,omitempty"`
}

func (c *BingoCard) TotalSquares() int {
	if !IsValidGridSize(c.GridSize) {
		return MaxGridSize * MaxGridSize
	}
	return c.GridSize * c.GridSize
}

func (c *BingoCard) Capacity() int {
	total := c.TotalSquares()
	if c.HasFreeSpace {
		return total - 1
	}
	return total
}

func (c *BingoCard) HasFreePositionSet() bool {
	return c.HasFreeSpace && c.FreeSpacePos != nil
}

func (c *BingoCard) IsPositionInRange(pos int) bool {
	return pos >= 0 && pos < c.TotalSquares()
}

func (c *BingoCard) IsFreeSpacePosition(pos int) bool {
	return c.HasFreePositionSet() && pos == *c.FreeSpacePos
}

func (c *BingoCard) IsValidItemPosition(pos int) bool {
	return c.IsPositionInRange(pos) && !c.IsFreeSpacePosition(pos)
}

func (c BingoCard) DefaultFreeSpacePosition() int {
	if !IsValidGridSize(c.GridSize) {
		return (MaxGridSize * MaxGridSize) / 2
	}
	total := c.GridSize * c.GridSize
	if c.GridSize%2 == 1 {
		return total / 2
	}
	return rand.Intn(total)
}

// DisplayName returns a human-readable name for the card
func (c *BingoCard) DisplayName() string {
	if c.Title != nil && *c.Title != "" {
		return *c.Title
	}
	return fmt.Sprintf("%d Bingo Card", c.Year)
}

// ValidCategories defines the allowed card categories
var ValidCategories = []string{
	"personal",     // Personal Growth
	"health",       // Health & Fitness
	"food",         // Food & Dining
	"travel",       // Travel & Adventure
	"hobbies",      // Hobbies & Creativity
	"social",       // Social & Relationships
	"professional", // Professional & Career
	"fun",          // Fun & Silly
}

// CategoryNames maps category IDs to display names
var CategoryNames = map[string]string{
	"personal":     "Personal Growth",
	"health":       "Health & Fitness",
	"food":         "Food & Dining",
	"travel":       "Travel & Adventure",
	"hobbies":      "Hobbies & Creativity",
	"social":       "Social & Relationships",
	"professional": "Professional & Career",
	"fun":          "Fun & Silly",
}

// IsValidCategory checks if a category string is valid
func IsValidCategory(category string) bool {
	for _, c := range ValidCategories {
		if c == category {
			return true
		}
	}
	return false
}

type BingoItem struct {
	ID          uuid.UUID  `json:"id"`
	CardID      uuid.UUID  `json:"card_id"`
	Position    int        `json:"position"`
	Content     string     `json:"content"`
	IsCompleted bool       `json:"is_completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
	ProofURL    *string    `json:"proof_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type CreateCardParams struct {
	UserID   uuid.UUID
	Year     int
	Category *string
	Title    *string
	GridSize int
	Header   string
	HasFree  bool
}

type UpdateCardMetaParams struct {
	Category *string
	Title    *string
}

type UpdateCardConfigParams struct {
	HeaderText   *string
	HasFreeSpace *bool
}

type AddItemParams struct {
	CardID   uuid.UUID
	Content  string
	Position *int // Optional; if nil, assign randomly
}

type UpdateItemParams struct {
	Content  *string
	Position *int
}

type CompleteItemParams struct {
	Notes    *string
	ProofURL *string
}

// CardStats contains statistics for a bingo card
type CardStats struct {
	CardID          uuid.UUID  `json:"card_id"`
	Year            int        `json:"year"`
	TotalItems      int        `json:"total_items"`
	CompletedItems  int        `json:"completed_items"`
	CompletionRate  float64    `json:"completion_rate"`
	BingosAchieved  int        `json:"bingos_achieved"`
	FirstCompletion *time.Time `json:"first_completion,omitempty"`
	LastCompletion  *time.Time `json:"last_completion,omitempty"`
}

// ImportCardParams contains parameters for importing an anonymous card
type ImportCardParams struct {
	UserID           uuid.UUID
	Year             int
	Title            *string
	Category         *string
	Items            []ImportItem
	Finalize         bool
	VisibleToFriends *bool // Optional; defaults to true if nil
	GridSize         int
	HeaderText       string
	HasFreeSpace     bool
	FreeSpacePos     *int
}

// ImportItem represents a single item to import
type ImportItem struct {
	Position int
	Content  string
}
