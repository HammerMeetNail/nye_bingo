package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ReminderSettings stores user-level reminder preferences.
type ReminderSettings struct {
	UserID        uuid.UUID `json:"user_id"`
	EmailEnabled  bool      `json:"email_enabled"`
	DailyEmailCap int       `json:"daily_email_cap"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ReminderSettingsPatch allows partial updates to reminder settings.
type ReminderSettingsPatch struct {
	EmailEnabled *bool `json:"email_enabled,omitempty"`
}

// CardCheckinReminder stores a per-card reminder schedule.
type CardCheckinReminder struct {
	ID                     uuid.UUID       `json:"id"`
	UserID                 uuid.UUID       `json:"user_id"`
	CardID                 uuid.UUID       `json:"card_id"`
	Enabled                bool            `json:"enabled"`
	Frequency              string          `json:"frequency"`
	Schedule               json.RawMessage `json:"schedule"`
	IncludeImage           bool            `json:"include_image"`
	IncludeRecommendations bool            `json:"include_recommendations"`
	NextSendAt             *time.Time      `json:"next_send_at,omitempty"`
	LastSentAt             *time.Time      `json:"last_sent_at,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

// CardCheckinScheduleInput is the payload for upserting a card check-in reminder.
type CardCheckinScheduleInput struct {
	Frequency              string                     `json:"frequency"`
	Schedule               CardCheckinSchedulePayload `json:"schedule"`
	IncludeImage           *bool                      `json:"include_image,omitempty"`
	IncludeRecommendations *bool                      `json:"include_recommendations,omitempty"`
}

// CardCheckinSchedulePayload describes the monthly schedule payload.
type CardCheckinSchedulePayload struct {
	DayOfMonth int    `json:"day_of_month"`
	Time       string `json:"time"`
}

// CardCheckinSummary joins card metadata with an optional reminder.
type CardCheckinSummary struct {
	CardID       uuid.UUID            `json:"card_id"`
	CardTitle    *string              `json:"card_title,omitempty"`
	CardYear     int                  `json:"card_year"`
	IsFinalized  bool                 `json:"is_finalized"`
	IsArchived   bool                 `json:"is_archived"`
	Checkin      *CardCheckinReminder `json:"checkin,omitempty"`
	HasFreeSpace bool                 `json:"has_free_space"`
	GridSize     int                  `json:"grid_size"`
}

// GoalReminder stores reminder scheduling for a specific goal.
type GoalReminder struct {
	ID         uuid.UUID       `json:"id"`
	UserID     uuid.UUID       `json:"user_id"`
	CardID     uuid.UUID       `json:"card_id"`
	ItemID     uuid.UUID       `json:"item_id"`
	Enabled    bool            `json:"enabled"`
	Kind       string          `json:"kind"`
	Schedule   json.RawMessage `json:"schedule"`
	NextSendAt *time.Time      `json:"next_send_at,omitempty"`
	LastSentAt *time.Time      `json:"last_sent_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// GoalReminderInput is the payload for upserting a goal reminder.
type GoalReminderInput struct {
	ItemID   uuid.UUID                 `json:"item_id"`
	Kind     string                    `json:"kind"`
	Schedule GoalReminderScheduleInput `json:"schedule"`
}

// GoalReminderScheduleInput defines goal reminder scheduling fields.
type GoalReminderScheduleInput struct {
	SendAt string `json:"send_at"`
}

// GoalReminderSummary joins reminder data with card/item context for the UI.
type GoalReminderSummary struct {
	ID         uuid.UUID       `json:"id"`
	CardID     uuid.UUID       `json:"card_id"`
	ItemID     uuid.UUID       `json:"item_id"`
	Kind       string          `json:"kind"`
	Schedule   json.RawMessage `json:"schedule"`
	NextSendAt *time.Time      `json:"next_send_at,omitempty"`
	LastSentAt *time.Time      `json:"last_sent_at,omitempty"`
	CardTitle  *string         `json:"card_title,omitempty"`
	CardYear   int             `json:"card_year"`
	ItemText   string          `json:"item_text"`
}

// ReminderImageToken grants short-lived access to reminder card images.
type ReminderImageToken struct {
	Token           string     `json:"token"`
	UserID          uuid.UUID  `json:"user_id"`
	CardID          uuid.UUID  `json:"card_id"`
	ShowCompletions bool       `json:"show_completions"`
	ExpiresAt       time.Time  `json:"expires_at"`
	CreatedAt       time.Time  `json:"created_at"`
	LastAccessedAt  *time.Time `json:"last_accessed_at,omitempty"`
	AccessCount     int        `json:"access_count"`
}

// ReminderUnsubscribeToken is a stored token for one-click unsubscribe.
type ReminderUnsubscribeToken struct {
	Token     string     `json:"token"`
	UserID    uuid.UUID  `json:"user_id"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}
