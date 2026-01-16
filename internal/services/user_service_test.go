package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestUserService_Create_EmailExists(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(true)
		},
	}

	service := NewUserService(db)
	_, err := service.Create(context.Background(), models.CreateUserParams{
		Email:        "exists@example.com",
		PasswordHash: stringPtr("hash"),
		Username:     "user",
		Searchable:   true,
	})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestUserService_Create_EmailCheckError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	service := NewUserService(db)
	_, err := service.Create(context.Background(), models.CreateUserParams{
		Email:        "test@example.com",
		PasswordHash: stringPtr("hash"),
		Username:     "user",
		Searchable:   true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_Create_UsernameExists(t *testing.T) {
	call := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			call++
			switch call {
			case 1:
				return rowFromValues(false)
			case 2:
				return rowFromValues(true)
			default:
				return rowFromValues(false)
			}
		},
	}

	service := NewUserService(db)
	_, err := service.Create(context.Background(), models.CreateUserParams{
		Email:        "new@example.com",
		PasswordHash: stringPtr("hash"),
		Username:     "exists",
		Searchable:   true,
	})
	if !errors.Is(err, ErrUsernameAlreadyExists) {
		t.Fatalf("expected ErrUsernameAlreadyExists, got %v", err)
	}
}

func TestUserService_Create_UsernameCheckError(t *testing.T) {
	call := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			call++
			if call == 1 {
				return rowFromValues(false)
			}
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	service := NewUserService(db)
	_, err := service.Create(context.Background(), models.CreateUserParams{
		Email:        "test@example.com",
		PasswordHash: stringPtr("hash"),
		Username:     "user",
		Searchable:   true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_Create_InsertError(t *testing.T) {
	call := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			call++
			switch call {
			case 1:
				return rowFromValues(false)
			case 2:
				return rowFromValues(false)
			default:
				return fakeRow{scanFunc: func(dest ...any) error {
					return errors.New("boom")
				}}
			}
		},
	}

	service := NewUserService(db)
	_, err := service.Create(context.Background(), models.CreateUserParams{
		Email:        "test@example.com",
		PasswordHash: stringPtr("hash"),
		Username:     "user",
		Searchable:   true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	service := NewUserService(db)
	_, err := service.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_GetByID_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	service := NewUserService(db)
	_, err := service.GetByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_GetByID_FiltersDeleted(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if !strings.Contains(sql, "deleted_at IS NULL") {
				t.Fatalf("expected deleted_at filter in query, got %q", sql)
			}
			return rowFromValues(uuid.New(), "test@example.com", stringPtr("hash"), "user", false, nil, 0, true, time.Now(), time.Now())
		},
	}

	service := NewUserService(db)
	_, err := service.GetByID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserService_UpdatePassword_NotFound(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}

	service := NewUserService(db)
	err := service.UpdatePassword(context.Background(), uuid.New(), "hash")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_UpdatePassword_ExecError(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	service := NewUserService(db)
	err := service.UpdatePassword(context.Background(), uuid.New(), "hash")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_GetByEmail_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	service := NewUserService(db)
	_, err := service.GetByEmail(context.Background(), "missing@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_GetByEmail_FiltersDeleted(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if !strings.Contains(sql, "deleted_at IS NULL") {
				t.Fatalf("expected deleted_at filter in query, got %q", sql)
			}
			return rowFromValues(uuid.New(), "test@example.com", stringPtr("hash"), "user", false, nil, 0, true, time.Now(), time.Now())
		},
	}

	service := NewUserService(db)
	_, err := service.GetByEmail(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserService_GetByEmail_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	service := NewUserService(db)
	_, err := service.GetByEmail(context.Background(), "test@example.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_UpdateSearchable_NotFound(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}

	service := NewUserService(db)
	err := service.UpdateSearchable(context.Background(), uuid.New(), true)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_UpdateSearchable_ExecError(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	service := NewUserService(db)
	err := service.UpdateSearchable(context.Background(), uuid.New(), true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_Create_Success(t *testing.T) {
	call := 0
	now := time.Now()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			call++
			switch call {
			case 1:
				return rowFromValues(false)
			case 2:
				return rowFromValues(false)
			default:
				return rowFromValues(
					userID,
					"test@example.com",
					stringPtr("hash"),
					"user",
					false,
					nil,
					0,
					true,
					now,
					now,
				)
			}
		},
	}

	service := NewUserService(db)
	user, err := service.Create(context.Background(), models.CreateUserParams{
		Email:        "test@example.com",
		PasswordHash: stringPtr("hash"),
		Username:     "user",
		Searchable:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != userID {
		t.Fatalf("expected user id %v, got %v", userID, user.ID)
	}
}

func TestUserService_GetByID_Success(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(
				userID,
				"test@example.com",
				stringPtr("hash"),
				"user",
				false,
				nil,
				0,
				true,
				now,
				now,
			)
		},
	}

	service := NewUserService(db)
	user, err := service.GetByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != userID {
		t.Fatalf("expected user id %v, got %v", userID, user.ID)
	}
}

func TestUserService_GetByEmail_Success(t *testing.T) {
	now := time.Now()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(
				userID,
				"test@example.com",
				stringPtr("hash"),
				"user",
				true,
				nil,
				2,
				false,
				now,
				now,
			)
		},
	}

	service := NewUserService(db)
	user, err := service.GetByEmail(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != userID {
		t.Fatalf("expected user id %v, got %v", userID, user.ID)
	}
}

func TestUserService_UpdatePassword_Success(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	service := NewUserService(db)
	if err := service.UpdatePassword(context.Background(), uuid.New(), "hash"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserService_MarkEmailVerified_Success(t *testing.T) {
	called := false
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			called = true
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	service := NewUserService(db)
	if err := service.MarkEmailVerified(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected update to be executed")
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestUserService_MarkEmailVerified_Error(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	service := NewUserService(db)
	if err := service.MarkEmailVerified(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_UpdateSearchable_Success(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	service := NewUserService(db)
	if err := service.UpdateSearchable(context.Background(), uuid.New(), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
