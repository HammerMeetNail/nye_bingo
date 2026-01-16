package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

var (
	ErrInvalidProviderClaims  = errors.New("invalid provider claims")
	ErrProviderEmailUnverified = errors.New("provider email not verified")
	ErrProviderIdentityExists  = errors.New("provider identity already linked")
	ErrInvalidProviderPending  = errors.New("invalid provider pending record")
	ErrInvalidUsername         = errors.New("invalid username")
)

type PendingProviderUser struct {
	Provider Provider
	Subject  string
	Email    string
}

type ProviderLinkResult struct {
	User    *models.User
	Pending *PendingProviderUser
}

type ProviderAuthService struct {
	db DB
}

func NewProviderAuthService(db DB) *ProviderAuthService {
	return &ProviderAuthService{db: db}
}

func (s *ProviderAuthService) LinkOrFindUserFromProvider(ctx context.Context, claims IdentityClaims) (*ProviderLinkResult, error) {
	provider := strings.TrimSpace(string(claims.Provider))
	subject := strings.TrimSpace(claims.Subject)
	if provider == "" || subject == "" {
		return nil, ErrInvalidProviderClaims
	}

	linkedUser, err := s.getUserByProviderSubject(ctx, claims.Provider, subject, s.db)
	if err == nil {
		return &ProviderLinkResult{User: linkedUser}, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	email := normalizeEmail(claims.Email)
	if email == "" || !claims.EmailVerified {
		return nil, ErrProviderEmailUnverified
	}

	user, err := s.getUserByEmail(ctx, email, s.db)
	if err == nil {
		if err := s.linkIdentity(ctx, user.ID, claims.Provider, subject, email); err != nil {
			if errors.Is(err, ErrProviderIdentityExists) {
				existing, lookupErr := s.getUserByProviderSubject(ctx, claims.Provider, subject, s.db)
				if lookupErr == nil {
					return &ProviderLinkResult{User: existing}, nil
				}
			}
			return nil, err
		}

		updatedUser, err := s.getUserByID(ctx, user.ID, s.db)
		if err != nil {
			return nil, err
		}
		return &ProviderLinkResult{User: updatedUser}, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	return &ProviderLinkResult{
		Pending: &PendingProviderUser{
			Provider: claims.Provider,
			Subject:  subject,
			Email:    email,
		},
	}, nil
}

func (s *ProviderAuthService) CreateUserFromProviderPending(ctx context.Context, pending PendingProviderUser, username string, searchable bool) (*models.User, error) {
	if strings.TrimSpace(string(pending.Provider)) == "" || strings.TrimSpace(pending.Subject) == "" {
		return nil, ErrInvalidProviderPending
	}
	email := normalizeEmail(pending.Email)
	if email == "" {
		return nil, ErrInvalidProviderPending
	}

	username = strings.TrimSpace(username)
	if len(username) < 2 || len(username) > 100 {
		return nil, ErrInvalidUsername
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

	if exists, err := s.userExistsByEmail(ctx, email, tx); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrEmailAlreadyExists
	}

	if exists, err := s.userExistsByUsername(ctx, username, tx); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrUsernameAlreadyExists
	}

	user := &models.User{}
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, username, email_verified, email_verified_at, searchable)
		 VALUES ($1, $2, $3, true, NOW(), $4)
		 RETURNING id, email, password_hash, username, email_verified, email_verified_at, ai_free_generations_used, searchable, created_at, updated_at`,
		email, nil, username, searchable,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Username, &user.EmailVerified, &user.EmailVerifiedAt, &user.AIFreeGenerationsUsed, &user.Searchable, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, s.resolveUserInsertConflict(ctx, email, username, tx)
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO user_identities (user_id, provider, subject, email_at_link_time)
		 VALUES ($1, $2, $3, $4)`,
		user.ID, pending.Provider, pending.Subject, email,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrProviderIdentityExists
		}
		return nil, fmt.Errorf("linking user identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return user, nil
}

func (s *ProviderAuthService) linkIdentity(ctx context.Context, userID uuid.UUID, provider Provider, subject, email string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Rollback is a no-op after commit

	if _, err := tx.Exec(ctx,
		`INSERT INTO user_identities (user_id, provider, subject, email_at_link_time)
		 VALUES ($1, $2, $3, $4)`,
		userID, provider, subject, email,
	); err != nil {
		if isUniqueViolation(err) {
			return ErrProviderIdentityExists
		}
		return fmt.Errorf("inserting user identity: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE users
		 SET email_verified = true,
		     email_verified_at = COALESCE(email_verified_at, NOW()),
		     updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	); err != nil {
		return fmt.Errorf("marking email verified: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (s *ProviderAuthService) getUserByProviderSubject(ctx context.Context, provider Provider, subject string, db DBConn) (*models.User, error) {
	user := &models.User{}
	err := db.QueryRow(ctx,
		`SELECT u.id, u.email, u.password_hash, u.username, u.email_verified, u.email_verified_at, u.ai_free_generations_used, u.searchable, u.created_at, u.updated_at
		 FROM user_identities ui
		 JOIN users u ON u.id = ui.user_id
		 WHERE ui.provider = $1 AND ui.subject = $2 AND u.deleted_at IS NULL`,
		provider, subject,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Username, &user.EmailVerified, &user.EmailVerifiedAt, &user.AIFreeGenerationsUsed, &user.Searchable, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by provider subject: %w", err)
	}
	return user, nil
}

func (s *ProviderAuthService) getUserByEmail(ctx context.Context, email string, db DBConn) (*models.User, error) {
	user := &models.User{}
	err := db.QueryRow(ctx,
		`SELECT id, email, password_hash, username, email_verified, email_verified_at, ai_free_generations_used, searchable, created_at, updated_at
		 FROM users WHERE email = $1 AND deleted_at IS NULL`,
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

func (s *ProviderAuthService) getUserByID(ctx context.Context, userID uuid.UUID, db DBConn) (*models.User, error) {
	user := &models.User{}
	err := db.QueryRow(ctx,
		`SELECT id, email, password_hash, username, email_verified, email_verified_at, ai_free_generations_used, searchable, created_at, updated_at
		 FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Username, &user.EmailVerified, &user.EmailVerifiedAt, &user.AIFreeGenerationsUsed, &user.Searchable, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return user, nil
}

func (s *ProviderAuthService) userExistsByEmail(ctx context.Context, email string, db DBConn) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)",
		email,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking email existence: %w", err)
	}
	return exists, nil
}

func (s *ProviderAuthService) userExistsByUsername(ctx context.Context, username string, db DBConn) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = LOWER($1) AND deleted_at IS NULL)",
		username,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking username existence: %w", err)
	}
	return exists, nil
}

func (s *ProviderAuthService) resolveUserInsertConflict(ctx context.Context, email, username string, db DBConn) error {
	if exists, err := s.userExistsByEmail(ctx, email, db); err != nil {
		return err
	} else if exists {
		return ErrEmailAlreadyExists
	}
	if exists, err := s.userExistsByUsername(ctx, username, db); err != nil {
		return err
	} else if exists {
		return ErrUsernameAlreadyExists
	}
	return ErrInvalidProviderClaims
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
