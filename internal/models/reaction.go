package models

import (
	"time"

	"github.com/google/uuid"
)

var AllowedEmojis = []string{"🎉", "👏", "🔥", "❤️", "⭐"}

type Reaction struct {
	ID        uuid.UUID `json:"id"`
	ItemID    uuid.UUID `json:"item_id"`
	UserID    uuid.UUID `json:"user_id"`
	Emoji     string    `json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

type ReactionWithUser struct {
	Reaction
	UserUsername string `json:"user_username"`
}

type ReactionSummary struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}
