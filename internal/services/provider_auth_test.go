package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestProviderAuth_LinkOrFind_InvalidClaims(t *testing.T) {
	service := NewProviderAuthService(&fakeDB{})

	_, err := service.LinkOrFindUserFromProvider(context.Background(), IdentityClaims{})
	if !errors.Is(err, ErrInvalidProviderClaims) {
		t.Fatalf("expected ErrInvalidProviderClaims, got %v", err)
	}
}

func TestProviderAuth_LinkOrFind_UnverifiedEmail(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	service := NewProviderAuthService(db)

	_, err := service.LinkOrFindUserFromProvider(context.Background(), IdentityClaims{
		Provider:      ProviderGoogle,
		Subject:       "sub",
		Email:         "test@example.com",
		EmailVerified: false,
	})
	if !errors.Is(err, ErrProviderEmailUnverified) {
		t.Fatalf("expected ErrProviderEmailUnverified, got %v", err)
	}
}

func TestProviderAuth_LinkOrFind_Pending(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	service := NewProviderAuthService(db)

	result, err := service.LinkOrFindUserFromProvider(context.Background(), IdentityClaims{
		Provider:      ProviderGoogle,
		Subject:       "sub",
		Email:         "Test@Example.com",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pending == nil {
		t.Fatalf("expected pending result")
	}
	if result.Pending.Email != "test@example.com" {
		t.Fatalf("expected normalized email, got %q", result.Pending.Email)
	}
}

func TestProviderAuth_CreateUserFromProviderPending_Success(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	username := "newuser"
	email := "test@example.com"
	var committed bool

	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			switch {
			case strings.Contains(sql, "SELECT EXISTS") && strings.Contains(sql, "email"):
				return rowFromValues(false)
			case strings.Contains(sql, "SELECT EXISTS") && strings.Contains(sql, "LOWER(username)"):
				return rowFromValues(false)
			case strings.Contains(sql, "INSERT INTO users"):
				return rowFromValues(
					userID,
					email,
					nil,
					username,
					true,
					nil,
					0,
					true,
					now,
					now,
				)
			default:
				return fakeRow{scanFunc: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			committed = true
			return nil
		},
	}

	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}

	service := NewProviderAuthService(db)
	user, err := service.CreateUserFromProviderPending(context.Background(), PendingProviderUser{
		Provider: ProviderGoogle,
		Subject:  "sub",
		Email:    email,
	}, username, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !committed {
		t.Fatalf("expected transaction commit")
	}
	if user.ID != userID {
		t.Fatalf("expected user id %v, got %v", userID, user.ID)
	}
}
