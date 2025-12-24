package services

import (
	"context"
	"errors"
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
		PasswordHash: "hash",
		Username:     "user",
		Searchable:   true,
	})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
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
		PasswordHash: "hash",
		Username:     "exists",
		Searchable:   true,
	})
	if !errors.Is(err, ErrUsernameAlreadyExists) {
		t.Fatalf("expected ErrUsernameAlreadyExists, got %v", err)
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
					"hash",
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
		PasswordHash: "hash",
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
				"hash",
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
