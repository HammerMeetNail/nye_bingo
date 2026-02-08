package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCardService_CheckForConflict_QuerySelection(t *testing.T) {
	userID := uuid.New()

	t.Run("nil title uses null-title query", func(t *testing.T) {
		var gotSQL string
		var gotArgs []any

		db := &fakeDB{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				gotSQL = sql
				gotArgs = args
				return fakeRow{scanFunc: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		}

		svc := NewCardService(db)
		_, err := svc.CheckForConflict(context.Background(), userID, 2026, nil)
		if !errors.Is(err, ErrCardNotFound) {
			t.Fatalf("expected ErrCardNotFound, got %v", err)
		}
		if !strings.Contains(gotSQL, "title IS NULL") {
			t.Fatalf("expected null-title query, got %q", gotSQL)
		}
		if len(gotArgs) != 2 {
			t.Fatalf("expected 2 args, got %d", len(gotArgs))
		}
	})

	t.Run("non-empty title uses exact-title query", func(t *testing.T) {
		var gotSQL string
		var gotArgs []any

		db := &fakeDB{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				gotSQL = sql
				gotArgs = args
				return fakeRow{scanFunc: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		}

		svc := NewCardService(db)
		title := "My Card"
		_, err := svc.CheckForConflict(context.Background(), userID, 2026, &title)
		if !errors.Is(err, ErrCardNotFound) {
			t.Fatalf("expected ErrCardNotFound, got %v", err)
		}
		if !strings.Contains(gotSQL, "title = $3") {
			t.Fatalf("expected titled query, got %q", gotSQL)
		}
		if len(gotArgs) != 3 {
			t.Fatalf("expected 3 args, got %d", len(gotArgs))
		}
	})
}

func TestCardService_SuggestEditTitle_FallbackAndError(t *testing.T) {
	userID := uuid.New()

	t.Run("blank base title falls back to year default", func(t *testing.T) {
		db := &fakeDB{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		}

		svc := NewCardService(db)
		got, err := svc.suggestEditTitle(context.Background(), userID, 2026, "   ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "2026 Bingo Card (Edit)" {
			t.Fatalf("expected default year-based title, got %q", got)
		}
	})

	t.Run("query errors are wrapped", func(t *testing.T) {
		db := &fakeDB{
			QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
				return fakeRow{scanFunc: func(dest ...any) error {
					return errors.New("boom")
				}}
			},
		}

		svc := NewCardService(db)
		_, err := svc.suggestEditTitle(context.Background(), userID, 2026, "My Card")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "checking suggested title conflict") {
			t.Fatalf("expected wrapped conflict message, got %v", err)
		}
	})
}

func TestCardService_NotifyHelpers_HandleNilAndErrors(t *testing.T) {
	svc := NewCardService(&fakeDB{})
	userID := uuid.New()
	cardID := uuid.New()

	// Nil notification service should be a no-op.
	svc.notifyFriendsNewCard(context.Background(), userID, cardID)
	svc.notifyFriendsBingo(context.Background(), userID, cardID, 2)

	var newCardCalls int
	var bingoCalls int

	svc.SetNotificationService(&stubNotificationService{
		NotifyFriendsNewCardFunc: func(ctx context.Context, actorID, gotCardID uuid.UUID) error {
			newCardCalls++
			return errors.New("notify new card failed")
		},
		NotifyFriendsBingoFunc: func(ctx context.Context, actorID, gotCardID uuid.UUID, bingoCount int) error {
			bingoCalls++
			return errors.New("notify bingo failed")
		},
	})

	// Errors from the notification service should not propagate/panic.
	svc.notifyFriendsNewCard(context.Background(), userID, cardID)
	svc.notifyFriendsBingo(context.Background(), userID, cardID, 3)

	if newCardCalls != 1 {
		t.Fatalf("expected 1 new-card notification call, got %d", newCardCalls)
	}
	if bingoCalls != 1 {
		t.Fatalf("expected 1 bingo notification call, got %d", bingoCalls)
	}
}
