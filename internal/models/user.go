package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	Username        string     `json:"username"`
	EmailVerified   bool       `json:"email_verified"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	Searchable      bool       `json:"searchable"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type CreateUserParams struct {
	Email        string
	PasswordHash string
	Username     string
	Searchable   bool
}
