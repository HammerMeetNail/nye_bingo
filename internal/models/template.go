package models

import (
	"time"

	"github.com/google/uuid"
)

type CardTemplate struct {
	ID                      uuid.UUID `json:"id"`
	UserID                  uuid.UUID `json:"user_id"`
	Name                    string    `json:"name"`
	Category                *string   `json:"category,omitempty"`
	GridSize                int       `json:"grid_size"`
	HeaderText              string    `json:"header_text"`
	HasFreeSpace            bool      `json:"has_free_space"`
	DefaultVisibleToFriends bool      `json:"default_visible_to_friends"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type CardTemplateItem struct {
	ID         uuid.UUID `json:"id"`
	TemplateID uuid.UUID `json:"template_id"`
	SortOrder  int       `json:"sort_order"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type TemplateWithItems struct {
	Template CardTemplate       `json:"template"`
	Items    []CardTemplateItem `json:"items"`
}

type CreateTemplateParams struct {
	Name                    string
	Category                *string
	GridSize                int
	HeaderText              string
	HasFreeSpace            bool
	DefaultVisibleToFriends bool
	Items                   []string
}

type UpdateTemplateParams struct {
	Name                    string
	Category                *string
	GridSize                int
	HeaderText              string
	HasFreeSpace            bool
	DefaultVisibleToFriends bool
}

type TemplateCreateCardParams struct {
	Year             int
	Title            *string
	Category         *string
	ShuffleLayout    bool
	VisibleToFriends *bool
}

type RolloverParams struct {
	Year          int
	CarryOver     string
	ShuffleLayout bool
	Title         *string
}

