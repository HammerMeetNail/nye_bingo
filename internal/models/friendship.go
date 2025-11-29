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
	FriendUsername string `json:"friend_username"`
	FriendEmail    string `json:"friend_email,omitempty"`
}

type FriendRequest struct {
	Friendship
	RequesterUsername string `json:"requester_username"`
	RequesterEmail    string `json:"requester_email,omitempty"`
}

type UserSearchResult struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
}
