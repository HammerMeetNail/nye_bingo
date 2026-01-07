package models

import (
	"time"

	"github.com/google/uuid"
)

type CardShare struct {
	CardID         uuid.UUID  `json:"card_id"`
	Token          string     `json:"token"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	AccessCount    int        `json:"access_count"`
}

type PublicBingoCard struct {
	ID           uuid.UUID `json:"id"`
	Year         int       `json:"year"`
	Category     *string   `json:"category,omitempty"`
	Title        *string   `json:"title,omitempty"`
	GridSize     int       `json:"grid_size"`
	HeaderText   string    `json:"header_text"`
	HasFreeSpace bool      `json:"has_free_space"`
	FreeSpacePos *int      `json:"free_space_position,omitempty"`
	IsFinalized  bool      `json:"is_finalized"`
}

type PublicBingoItem struct {
	Position    int    `json:"position"`
	Content     string `json:"content"`
	IsCompleted bool   `json:"is_completed"`
}

type SharedCard struct {
	Card  PublicBingoCard   `json:"card"`
	Items []PublicBingoItem `json:"items"`
}
