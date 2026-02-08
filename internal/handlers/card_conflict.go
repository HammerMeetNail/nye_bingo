package handlers

import "github.com/HammerMeetNail/yearofbingo/internal/models"

// ImportCardResponse includes conflict info when a card already exists.
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
