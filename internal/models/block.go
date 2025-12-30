package models

import (
	"time"

	"github.com/google/uuid"
)

type BlockedUser struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	BlockedAt time.Time `json:"blocked_at"`
}
