package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

const (
	bcryptCost = 12
	// sessionDuration is the sliding TTL refreshed on each request.
	sessionDuration = 30 * 24 * time.Hour // 30 days
	// maxSessionLifetime is an absolute cap regardless of activity. A session is
	// rejected once it is older than this even if its sliding TTL keeps refreshing.
	maxSessionLifetime = 90 * 24 * time.Hour // 90 days
	sessionKeyPrefix   = "session:"
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

func (s *AuthService) VerifyPassword(hash *string, password string) bool {
	if hash == nil || *hash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password))
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

	now := time.Now()
	expiresAt := now.Add(sessionDuration)

	// Store in Redis for fast lookups. The value embeds the creation time so we can
	// enforce an absolute max lifetime and a per-user invalidation cutoff without a
	// per-user session index (Redis exposes no enumeration primitive here).
	redisKey := sessionKeyPrefix + tokenHash
	err = s.redis.Set(ctx, redisKey, encodeSessionValue(userID, now), sessionDuration)
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
	value, err := s.redis.Get(ctx, redisKey)
	if err == nil {
		userID, createdAt, hasCreatedAt, perr := decodeSessionValue(value)
		if perr != nil {
			return nil, perr
		}

		// Absolute lifetime cap (only enforceable when the value carries a creation time).
		if hasCreatedAt && time.Since(createdAt) > maxSessionLifetime {
			_ = s.redis.Del(ctx, redisKey)
			return nil, ErrSessionExpired
		}

		user, err := s.getUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}

		// Reject sessions that predate a password change/reset invalidation cutoff.
		// Legacy values without a creation time are treated as predating the cutoff.
		if user.SessionsInvalidatedAt != nil && (!hasCreatedAt || !createdAt.After(*user.SessionsInvalidatedAt)) {
			_ = s.redis.Del(ctx, redisKey)
			return nil, ErrSessionNotFound
		}

		// Valid: extend the sliding session TTL.
		_ = s.redis.Expire(ctx, redisKey, sessionDuration)
		return user, nil
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

	if time.Now().After(session.ExpiresAt) || time.Since(session.CreatedAt) > maxSessionLifetime {
		// Clean up expired session
		_, _ = s.db.Exec(ctx, "DELETE FROM sessions WHERE id = $1", session.ID)
		return nil, ErrSessionExpired
	}

	user, err := s.getUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	// Reject sessions that predate a password change/reset invalidation cutoff.
	if user.SessionsInvalidatedAt != nil && !session.CreatedAt.After(*user.SessionsInvalidatedAt) {
		_, _ = s.db.Exec(ctx, "DELETE FROM sessions WHERE id = $1", session.ID)
		return nil, ErrSessionNotFound
	}

	return user, nil
}

// encodeSessionValue serializes the Redis session value as "<userID>|<unixCreatedAt>".
func encodeSessionValue(userID uuid.UUID, createdAt time.Time) string {
	return userID.String() + "|" + strconv.FormatInt(createdAt.Unix(), 10)
}

// decodeSessionValue parses a Redis session value. Legacy values are a bare user
// ID with no creation time, in which case hasCreatedAt is false.
func decodeSessionValue(value string) (userID uuid.UUID, createdAt time.Time, hasCreatedAt bool, err error) {
	idStr, tsStr, found := strings.Cut(value, "|")
	if found {
		if ts, perr := strconv.ParseInt(tsStr, 10, 64); perr == nil {
			createdAt = time.Unix(ts, 0)
			hasCreatedAt = true
		}
	}
	id, perr := uuid.Parse(idStr)
	if perr != nil {
		return uuid.UUID{}, time.Time{}, false, fmt.Errorf("parsing user id: %w", perr)
	}
	return id, createdAt, hasCreatedAt, nil
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
	// Stamp an invalidation cutoff so that all existing sessions (including
	// Redis-only sessions that have no row in the sessions table) are rejected on
	// their next validation. This is the primary revocation mechanism.
	if _, err := s.db.Exec(ctx, "UPDATE users SET sessions_invalidated_at = NOW() WHERE id = $1", userID); err != nil {
		return fmt.Errorf("invalidating user sessions: %w", err)
	}

	// Best-effort cleanup of any PG-fallback sessions and their Redis mirrors.
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
		`SELECT id, email, password_hash, username, email_verified, email_verified_at, ai_free_generations_used, searchable,
		        stripe_customer_id, stripe_subscription_id, billing_plan, billing_source, billing_status,
		        billing_current_period_end, billing_cancel_at_period_end, billing_updated_at, sessions_invalidated_at,
		        created_at, updated_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Username, &user.EmailVerified, &user.EmailVerifiedAt, &user.AIFreeGenerationsUsed, &user.Searchable,
		&user.StripeCustomerID, &user.StripeSubscriptionID, &user.BillingPlan, &user.BillingSource, &user.BillingStatus,
		&user.BillingCurrentPeriodEnd, &user.BillingCancelAtPeriodEnd, &user.BillingUpdatedAt, &user.SessionsInvalidatedAt,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	return user, nil
}
