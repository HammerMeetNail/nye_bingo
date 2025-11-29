package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	GridSize      = 5
	TotalSquares  = GridSize * GridSize // 25
	FreeSpacePos  = 12                  // Center position (0-indexed)
	ItemsRequired = TotalSquares - 1    // 24 items (excluding free space)
)

type BingoCard struct {
	ID               uuid.UUID   `json:"id"`
	UserID           uuid.UUID   `json:"user_id"`
	Year             int         `json:"year"`
	Category         *string     `json:"category,omitempty"`
	Title            *string     `json:"title,omitempty"`
	IsActive         bool        `json:"is_active"`
	IsFinalized      bool        `json:"is_finalized"`
	VisibleToFriends bool        `json:"visible_to_friends"`
	IsArchived       bool        `json:"is_archived"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	Items            []BingoItem `json:"items,omitempty"`
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
}

type UpdateCardMetaParams struct {
	Category *string
	Title    *string
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
}

// ImportItem represents a single item to import
type ImportItem struct {
	Position int
	Content  string
}
