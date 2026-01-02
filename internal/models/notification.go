package models

import (
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	NotificationTypeFriendRequestReceived NotificationType = "friend_request_received"
	NotificationTypeFriendRequestAccepted NotificationType = "friend_request_accepted"
	NotificationTypeFriendBingo           NotificationType = "friend_bingo"
	NotificationTypeFriendNewCard         NotificationType = "friend_new_card"
)

type Notification struct {
	ID             uuid.UUID        `json:"id"`
	UserID         uuid.UUID        `json:"user_id"`
	Type           NotificationType `json:"type"`
	ActorUserID    *uuid.UUID       `json:"actor_user_id,omitempty"`
	ActorUsername  *string          `json:"actor_username,omitempty"`
	FriendshipID   *uuid.UUID       `json:"friendship_id,omitempty"`
	CardID         *uuid.UUID       `json:"card_id,omitempty"`
	CardTitle      *string          `json:"card_title,omitempty"`
	CardYear       *int             `json:"card_year,omitempty"`
	BingoCount     *int             `json:"bingo_count,omitempty"`
	InAppDelivered bool             `json:"in_app_delivered"`
	EmailDelivered bool             `json:"email_delivered"`
	EmailSentAt    *time.Time       `json:"email_sent_at,omitempty"`
	ReadAt         *time.Time       `json:"read_at,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

type NotificationSettings struct {
	UserID                     uuid.UUID `json:"user_id"`
	InAppEnabled               bool      `json:"in_app_enabled"`
	InAppFriendRequestReceived bool      `json:"in_app_friend_request_received"`
	InAppFriendRequestAccepted bool      `json:"in_app_friend_request_accepted"`
	InAppFriendBingo           bool      `json:"in_app_friend_bingo"`
	InAppFriendNewCard         bool      `json:"in_app_friend_new_card"`
	EmailEnabled               bool      `json:"email_enabled"`
	EmailFriendRequestReceived bool      `json:"email_friend_request_received"`
	EmailFriendRequestAccepted bool      `json:"email_friend_request_accepted"`
	EmailFriendBingo           bool      `json:"email_friend_bingo"`
	EmailFriendNewCard         bool      `json:"email_friend_new_card"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type NotificationSettingsPatch struct {
	InAppEnabled               *bool `json:"in_app_enabled,omitempty"`
	InAppFriendRequestReceived *bool `json:"in_app_friend_request_received,omitempty"`
	InAppFriendRequestAccepted *bool `json:"in_app_friend_request_accepted,omitempty"`
	InAppFriendBingo           *bool `json:"in_app_friend_bingo,omitempty"`
	InAppFriendNewCard         *bool `json:"in_app_friend_new_card,omitempty"`
	EmailEnabled               *bool `json:"email_enabled,omitempty"`
	EmailFriendRequestReceived *bool `json:"email_friend_request_received,omitempty"`
	EmailFriendRequestAccepted *bool `json:"email_friend_request_accepted,omitempty"`
	EmailFriendBingo           *bool `json:"email_friend_bingo,omitempty"`
	EmailFriendNewCard         *bool `json:"email_friend_new_card,omitempty"`
}
