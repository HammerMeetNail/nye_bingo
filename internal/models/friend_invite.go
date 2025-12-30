package models

import (
	"time"

	"github.com/google/uuid"
)

type FriendInvite struct {
	ID               uuid.UUID  `json:"id"`
	InviterUserID    uuid.UUID  `json:"inviter_user_id"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	AcceptedByUserID *uuid.UUID `json:"accepted_by_user_id,omitempty"`
	AcceptedAt       *time.Time `json:"accepted_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}
