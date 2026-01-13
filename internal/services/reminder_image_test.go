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

func TestReminderService_RenderImageByToken_SuccessAndExpiry(t *testing.T) {
	now := time.Date(2026, time.January, 10, 8, 0, 0, 0, time.UTC)
	userID := uuid.New()
	cardID := uuid.New()
	token := "tok"

	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if sql == "UPDATE reminder_image_tokens SET last_accessed_at = NOW(), access_count = access_count + 1 WHERE token = $1" {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if !strings.Contains(sql, "FROM bingo_items") {
				return &fakeRows{rows: [][]any{}}, nil
			}
			return &fakeRows{rows: [][]any{}}, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			switch {
			case strings.Contains(sql, "FROM reminder_image_tokens"):
				expiresAt := now.Add(time.Hour)
				createdAt := now.Add(-time.Hour)
				var lastAccessedAt *time.Time
				return rowFromValues(token, userID, cardID, true, expiresAt, createdAt, lastAccessedAt, 0)
			case strings.Contains(sql, "FROM bingo_cards WHERE id"):
				title := "Card"
				return rowFromValues(
					cardID,
					userID,
					2025,
					nil,
					&title,
					2,
					"BI",
					false,
					nil,
					true,
					true,
					false,
					false,
					now.Add(-2*time.Hour),
					now.Add(-time.Hour),
				)
			default:
				return fakeRow{scanFunc: func(dest ...any) error { return errors.New("unexpected query") }}
			}
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	svc.now = func() time.Time { return now }

	pngBytes, err := svc.RenderImageByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pngBytes) < 4 || pngBytes[0] != 0x89 || pngBytes[1] != 0x50 || pngBytes[2] != 0x4e || pngBytes[3] != 0x47 {
		t.Fatalf("expected png header, got %v", pngBytes[:minInt(len(pngBytes), 8)])
	}

	// Expired tokens return ErrReminderNotFound.
	db.QueryRowFunc = func(ctx context.Context, sql string, args ...any) Row {
		if strings.Contains(sql, "FROM reminder_image_tokens") {
			expiresAt := now.Add(-time.Minute)
			createdAt := now.Add(-time.Hour)
			return rowFromValues(token, userID, cardID, false, expiresAt, createdAt, (*time.Time)(nil), 0)
		}
		return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
	}
	_, err = svc.RenderImageByToken(context.Background(), token)
	if !errors.Is(err, ErrReminderNotFound) {
		t.Fatalf("expected ErrReminderNotFound, got %v", err)
	}
}

func TestReminderService_CreateImageToken_CreatesWhenMissing(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()

	execCalls := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO reminder_image_tokens") {
				execCalls++
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	token, err := svc.createImageToken(context.Background(), userID, cardID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if execCalls == 0 {
		t.Fatal("expected insert to be executed")
	}
	if len(token) < 10 {
		t.Fatalf("expected token to be non-empty, got %q", token)
	}
}

func TestReminderService_UnsubscribeByToken_AlreadyUsed(t *testing.T) {
	now := time.Date(2026, time.January, 10, 8, 0, 0, 0, time.UTC)
	userID := uuid.New()
	usedAt := now.Add(-time.Hour)

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM reminder_unsubscribe_tokens") {
				return rowFromValues(userID, now.Add(time.Hour), &usedAt)
			}
			return rowFromValues(false)
		},
	}

	svc := NewReminderService(db, nil, "http://example.com")
	svc.now = func() time.Time { return now }

	alreadyDisabled, err := svc.UnsubscribeByToken(context.Background(), "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !alreadyDisabled {
		t.Fatal("expected alreadyDisabled=true for used token")
	}
}
