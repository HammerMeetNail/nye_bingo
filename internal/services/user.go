package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already taken")
)

type UserService struct {
	db DBConn
}

func NewUserService(db DBConn) *UserService {
	return &UserService{db: db}
}

func (s *UserService) Create(ctx context.Context, params models.CreateUserParams) (*models.User, error) {
	// Check if email already exists
	var exists bool
	err := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", params.Email).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("checking email existence: %w", err)
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	// Check if username already exists (case-insensitive)
	err = s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = LOWER($1))", params.Username).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("checking username existence: %w", err)
	}
	if exists {
		return nil, ErrUsernameAlreadyExists
	}

	user := &models.User{}
	err = s.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, username, email_verified, searchable)
		 VALUES ($1, $2, $3, false, $4)
		 RETURNING id, email, password_hash, username, email_verified, email_verified_at, ai_free_generations_used, searchable, created_at, updated_at`,
		params.Email, params.PasswordHash, params.Username, params.Searchable,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Username, &user.EmailVerified, &user.EmailVerifiedAt, &user.AIFreeGenerationsUsed, &user.Searchable, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(ctx,
		`SELECT id, email, password_hash, username, email_verified, email_verified_at, ai_free_generations_used, searchable, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Username, &user.EmailVerified, &user.EmailVerifiedAt, &user.AIFreeGenerationsUsed, &user.Searchable, &user.CreatedAt, &user.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}

	return user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	err := s.db.QueryRow(ctx,
		`SELECT id, email, password_hash, username, email_verified, email_verified_at, ai_free_generations_used, searchable, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Username, &user.EmailVerified, &user.EmailVerifiedAt, &user.AIFreeGenerationsUsed, &user.Searchable, &user.CreatedAt, &user.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}

	return user, nil
}

func (s *UserService) UpdatePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string) error {
	result, err := s.db.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		newPasswordHash, userID,
	)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (s *UserService) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET email_verified = true, email_verified_at = NOW() WHERE id = $1 AND email_verified = false`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("marking email verified: %w", err)
	}
	return nil
}

func (s *UserService) UpdateSearchable(ctx context.Context, userID uuid.UUID, searchable bool) error {
	result, err := s.db.Exec(ctx,
		`UPDATE users SET searchable = $1, updated_at = NOW() WHERE id = $2`,
		searchable, userID,
	)
	if err != nil {
		return fmt.Errorf("updating searchable: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}
