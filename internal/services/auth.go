package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

const (
	bcryptCost       = 12
	sessionDuration  = 30 * 24 * time.Hour // 30 days
	sessionKeyPrefix = "session:"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExpired     = errors.New("session expired")
	ErrPasswordTooLong    = errors.New("password exceeds bcrypt limit")
)

type AuthService struct {
	db    DBConn
	redis RedisClient
}

func NewAuthService(db DBConn, redis RedisClient) *AuthService {
	return &AuthService{
		db:    db,
		redis: redis,
	}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	if len([]byte(password)) > 72 {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

func (s *AuthService) VerifyPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *AuthService) GenerateSessionToken() (token string, hash string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generating random bytes: %w", err)
	}

	token = hex.EncodeToString(bytes)
	hashBytes := sha256.Sum256([]byte(token))
	hash = hex.EncodeToString(hashBytes[:])

	return token, hash, nil
}

func (s *AuthService) hashToken(token string) string {
	hashBytes := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hashBytes[:])
}

func (s *AuthService) CreateSession(ctx context.Context, userID uuid.UUID) (token string, err error) {
	token, tokenHash, err := s.GenerateSessionToken()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(sessionDuration)

	// Store in Redis for fast lookups
	redisKey := sessionKeyPrefix + tokenHash
	err = s.redis.Set(ctx, redisKey, userID.String(), sessionDuration)
	if err != nil {
		// Fall back to PostgreSQL if Redis fails
		_, err = s.db.Exec(ctx,
			`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
			userID, tokenHash, expiresAt,
		)
		if err != nil {
			return "", fmt.Errorf("creating session in database: %w", err)
		}
	}

	return token, nil
}

func (s *AuthService) ValidateSession(ctx context.Context, token string) (*models.User, error) {
	tokenHash := s.hashToken(token)

	// Try Redis first
	redisKey := sessionKeyPrefix + tokenHash
	userIDStr, err := s.redis.Get(ctx, redisKey)
	if err == nil {
		// Found in Redis, extend session
		_ = s.redis.Expire(ctx, redisKey, sessionDuration)

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil, fmt.Errorf("parsing user id: %w", err)
		}

		return s.getUserByID(ctx, userID)
	}

	// Fall back to PostgreSQL
	var session models.Session
	err = s.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at
		 FROM sessions WHERE token_hash = $1`,
		tokenHash,
	).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.ExpiresAt, &session.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying session: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		// Clean up expired session
		_, _ = s.db.Exec(ctx, "DELETE FROM sessions WHERE id = $1", session.ID)
		return nil, ErrSessionExpired
	}

	return s.getUserByID(ctx, session.UserID)
}

func (s *AuthService) DeleteSession(ctx context.Context, token string) error {
	tokenHash := s.hashToken(token)

	// Delete from Redis
	redisKey := sessionKeyPrefix + tokenHash
	_ = s.redis.Del(ctx, redisKey)

	// Delete from PostgreSQL
	_, err := s.db.Exec(ctx, "DELETE FROM sessions WHERE token_hash = $1", tokenHash)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	return nil
}

func (s *AuthService) DeleteAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	// Get all session hashes for this user from PostgreSQL
	rows, err := s.db.Query(ctx, "SELECT token_hash FROM sessions WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("querying user sessions: %w", err)
	}
	defer rows.Close()

	var tokenHashes []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return fmt.Errorf("scanning token hash: %w", err)
		}
		tokenHashes = append(tokenHashes, hash)
	}

	// Delete from Redis
	for _, hash := range tokenHashes {
		_ = s.redis.Del(ctx, sessionKeyPrefix+hash)
	}

	// Delete from PostgreSQL
	_, err = s.db.Exec(ctx, "DELETE FROM sessions WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("deleting user sessions: %w", err)
	}

	return nil
}

func (s *AuthService) getUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
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
		return nil, fmt.Errorf("getting user: %w", err)
	}

	return user, nil
}
