package models

import (
	"time"

	"github.com/google/uuid"
)

type FriendshipStatus string

const (
	FriendshipStatusPending  FriendshipStatus = "pending"
	FriendshipStatusAccepted FriendshipStatus = "accepted"
	FriendshipStatusRejected FriendshipStatus = "rejected"
)

type Friendship struct {
	ID        uuid.UUID        `json:"id"`
	UserID    uuid.UUID        `json:"user_id"`
	FriendID  uuid.UUID        `json:"friend_id"`
	Status    FriendshipStatus `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
}

type FriendWithUser struct {
	Friendship
	FriendDisplayName string `json:"friend_display_name"`
	FriendEmail       string `json:"friend_email,omitempty"`
}

type FriendRequest struct {
	Friendship
	RequesterDisplayName string `json:"requester_display_name"`
	RequesterEmail       string `json:"requester_email,omitempty"`
}

type UserSearchResult struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}
