package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrTokenNotFound = errors.New("api token not found")
)

type ApiTokenService struct {
	db DBConn
}

func NewApiTokenService(db DBConn) *ApiTokenService {
	return &ApiTokenService{db: db}
}

func (s *ApiTokenService) Create(ctx context.Context, userID uuid.UUID, name string, scope models.ApiTokenScope, expiresInDays int) (*models.ApiToken, string, error) {
	// Generate token: 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, "", fmt.Errorf("generating random bytes: %w", err)
	}

	// yob_ + base64(bytes)
	tokenSuffix := base64.RawURLEncoding.EncodeToString(bytes)
	plainToken := "yob_" + tokenSuffix

	// Prefix is "yob_" + first 4 chars of suffix
	tokenPrefix := "yob_"
	if len(tokenSuffix) >= 4 {
		tokenPrefix += tokenSuffix[:4]
	} else {
		tokenPrefix += tokenSuffix
	}

	// Hash token
	hashBytes := sha256.Sum256([]byte(plainToken))
	tokenHash := hex.EncodeToString(hashBytes[:])

	// Calculate expiration
	var expiresAt *time.Time
	if expiresInDays > 0 {
		t := time.Now().Add(time.Duration(expiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}

	apiToken := &models.ApiToken{}
	err := s.db.QueryRow(ctx,
		`INSERT INTO api_tokens (user_id, name, token_hash, token_prefix, scope, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, name, token_prefix, scope, expires_at, last_used_at, created_at`,
		userID, name, tokenHash, tokenPrefix, scope, expiresAt,
	).Scan(&apiToken.ID, &apiToken.UserID, &apiToken.Name, &apiToken.TokenPrefix, &apiToken.Scope, &apiToken.ExpiresAt, &apiToken.LastUsedAt, &apiToken.CreatedAt)

	if err != nil {
		return nil, "", fmt.Errorf("inserting api token: %w", err)
	}

	return apiToken, plainToken, nil
}

func (s *ApiTokenService) List(ctx context.Context, userID uuid.UUID) ([]models.ApiToken, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, name, token_prefix, scope, expires_at, last_used_at, created_at
		 FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []models.ApiToken
	for rows.Next() {
		var t models.ApiToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenPrefix, &t.Scope, &t.ExpiresAt, &t.LastUsedAt, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning api token: %w", err)
		}
		tokens = append(tokens, t)
	}

	return tokens, nil
}

func (s *ApiTokenService) Delete(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID) error {
	result, err := s.db.Exec(ctx,
		"DELETE FROM api_tokens WHERE id = $1 AND user_id = $2",
		tokenID, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting api token: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func (s *ApiTokenService) DeleteAll(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		"DELETE FROM api_tokens WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return fmt.Errorf("deleting all api tokens: %w", err)
	}
	return nil
}

func (s *ApiTokenService) ValidateToken(ctx context.Context, plainToken string) (*models.ApiToken, error) {
	// Hash token
	hashBytes := sha256.Sum256([]byte(plainToken))
	tokenHash := hex.EncodeToString(hashBytes[:])

	apiToken := &models.ApiToken{}
	err := s.db.QueryRow(ctx,
		`SELECT id, user_id, name, token_prefix, scope, expires_at, last_used_at, created_at
		 FROM api_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&apiToken.ID, &apiToken.UserID, &apiToken.Name, &apiToken.TokenPrefix, &apiToken.Scope, &apiToken.ExpiresAt, &apiToken.LastUsedAt, &apiToken.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying api token: %w", err)
	}

	// Check expiration
	if apiToken.ExpiresAt != nil && time.Now().After(*apiToken.ExpiresAt) {
		return nil, ErrTokenNotFound
	}

	return apiToken, nil
}

func (s *ApiTokenService) UpdateLastUsed(ctx context.Context, tokenID uuid.UUID) error {
	_, err := s.db.Exec(ctx, "UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1", tokenID)
	return err
}
