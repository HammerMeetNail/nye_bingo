package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestApiTokenService_Delete_NotFound(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}

	svc := NewApiTokenService(db)
	err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestApiTokenService_ValidateToken_Expired(t *testing.T) {
	expired := time.Now().Add(-1 * time.Hour)
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(
				uuid.New(),
				uuid.New(),
				"token-name",
				"yob_",
				"cards:read",
				&expired,
				nil,
				time.Now().Add(-2*time.Hour),
			)
		},
	}

	svc := NewApiTokenService(db)
	_, err := svc.ValidateToken(context.Background(), "plain-token")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestApiTokenService_ValidateToken_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	svc := NewApiTokenService(db)
	_, err := svc.ValidateToken(context.Background(), "plain-token")
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestApiTokenService_UpdateLastUsed(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewApiTokenService(db)
	if err := svc.UpdateLastUsed(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
