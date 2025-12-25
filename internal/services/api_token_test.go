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

func TestApiTokenService_ValidateToken_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	svc := NewApiTokenService(db)
	_, err := svc.ValidateToken(context.Background(), "plain-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApiTokenService_UpdateLastUsed(t *testing.T) {
	tokenID := uuid.New()
	var gotSQL string
	var gotArgs []any
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewApiTokenService(db)
	if err := svc.UpdateLastUsed(context.Background(), tokenID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotSQL, "UPDATE api_tokens") || !strings.Contains(gotSQL, "WHERE id = $1") {
		t.Fatalf("unexpected update sql: %q", gotSQL)
	}
	if len(gotArgs) != 1 || gotArgs[0] != tokenID {
		t.Fatalf("unexpected update args: %v", gotArgs)
	}
}

func TestApiTokenService_Create_Success(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(
				uuid.New(),
				userID,
				"name",
				"yob_test",
				"cards:read",
				(*time.Time)(nil),
				(*time.Time)(nil),
				now,
			)
		},
	}

	svc := NewApiTokenService(db)
	token, plain, err := svc.Create(context.Background(), userID, "name", "cards:read", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain == "" || token == nil {
		t.Fatal("expected token and plain value")
	}
	if !strings.HasPrefix(plain, "yob_") {
		t.Fatalf("expected token prefix yob_, got %q", plain)
	}
}

func TestApiTokenService_Create_WithExpiry(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if args[5] == nil {
				t.Fatal("expected expires_at to be set")
			}
			return rowFromValues(
				uuid.New(),
				userID,
				"name",
				"yob_test",
				"cards:read",
				args[5],
				(*time.Time)(nil),
				time.Now(),
			)
		},
	}

	svc := NewApiTokenService(db)
	_, _, err := svc.Create(context.Background(), userID, "name", "cards:read", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApiTokenService_List_Success(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{uuid.New(), userID, "A", "yob_a", "cards:read", (*time.Time)(nil), (*time.Time)(nil), time.Now()},
				{uuid.New(), userID, "B", "yob_b", "cards:read", (*time.Time)(nil), (*time.Time)(nil), time.Now()},
			}}, nil
		},
	}

	svc := NewApiTokenService(db)
	tokens, err := svc.List(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestApiTokenService_List_Error(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("boom")
		},
	}

	svc := NewApiTokenService(db)
	_, err := svc.List(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApiTokenService_Delete_Success(t *testing.T) {
	userID := uuid.New()
	tokenID := uuid.New()
	var gotSQL string
	var gotArgs []any
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewApiTokenService(db)
	if err := svc.Delete(context.Background(), userID, tokenID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotSQL, "DELETE FROM api_tokens") || !strings.Contains(gotSQL, "WHERE id = $1 AND user_id = $2") {
		t.Fatalf("unexpected delete sql: %q", gotSQL)
	}
	if len(gotArgs) != 2 || gotArgs[0] != tokenID || gotArgs[1] != userID {
		t.Fatalf("unexpected delete args: %v", gotArgs)
	}
}

func TestApiTokenService_DeleteAll_Success(t *testing.T) {
	userID := uuid.New()
	var gotSQL string
	var gotArgs []any
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return fakeCommandTag{rowsAffected: 2}, nil
		},
	}

	svc := NewApiTokenService(db)
	if err := svc.DeleteAll(context.Background(), userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotSQL, "DELETE FROM api_tokens") || !strings.Contains(gotSQL, "WHERE user_id = $1") {
		t.Fatalf("unexpected delete all sql: %q", gotSQL)
	}
	if len(gotArgs) != 1 || gotArgs[0] != userID {
		t.Fatalf("unexpected delete all args: %v", gotArgs)
	}
}

func TestApiTokenService_DeleteAll_Error(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	svc := NewApiTokenService(db)
	if err := svc.DeleteAll(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected error")
	}
}

func TestApiTokenService_ValidateToken_Success(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(
				uuid.New(),
				userID,
				"token-name",
				"yob_",
				"cards:read",
				(*time.Time)(nil),
				(*time.Time)(nil),
				now,
			)
		},
	}

	svc := NewApiTokenService(db)
	token, err := svc.ValidateToken(context.Background(), "plain-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.UserID != userID {
		t.Fatalf("expected user %v, got %v", userID, token.UserID)
	}
}
