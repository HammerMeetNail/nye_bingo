package services

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCardService_CreateOrRotateShare_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	ownerID := uuid.New()
	callCount := 0

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if callCount > 1 {
				t.Fatalf("unexpected query after ownership check: %s", sql)
			}
			return rowFromValues(ownerID, true)
		},
	}

	svc := NewCardService(db)
	_, err := svc.CreateOrRotateShare(context.Background(), userID, cardID, nil)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_CreateOrRotateShare_NotFinalized(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	callCount := 0

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if callCount > 1 {
				t.Fatalf("unexpected query after finalize check: %s", sql)
			}
			return rowFromValues(userID, false)
		},
	}

	svc := NewCardService(db)
	_, err := svc.CreateOrRotateShare(context.Background(), userID, cardID, nil)
	if !errors.Is(err, ErrCardNotFinalized) {
		t.Fatalf("expected ErrCardNotFinalized, got %v", err)
	}
}

func TestCardService_CreateOrRotateShare_Success_NoExpiry(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	createdAt := time.Now().Add(-time.Minute)
	callCount := 0
	var gotExpiresAt *time.Time
	var gotToken string

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if callCount == 1 {
				return rowFromValues(userID, true)
			}
			if !strings.Contains(sql, "INSERT INTO bingo_card_shares") {
				t.Fatalf("unexpected query for share insert: %s", sql)
			}
			if len(args) < 3 {
				t.Fatalf("expected share insert args, got %d", len(args))
			}
			gotToken, _ = args[1].(string)
			if args[2] != nil {
				gotExpiresAt, _ = args[2].(*time.Time)
			}
			return rowFromValues(cardID, gotToken, createdAt, (*time.Time)(nil), (*time.Time)(nil), 0)
		},
	}

	svc := NewCardService(db)
	share, err := svc.CreateOrRotateShare(context.Background(), userID, cardID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if share.CardID != cardID {
		t.Fatalf("expected card ID %v, got %v", cardID, share.CardID)
	}
	if gotToken == "" {
		t.Fatal("expected token to be generated")
	}
	if gotExpiresAt != nil {
		t.Fatalf("expected expires_at to be nil, got %v", gotExpiresAt)
	}
	if len(share.Token) != 64 {
		t.Fatalf("expected token length 64, got %d", len(share.Token))
	}
	if _, err := hex.DecodeString(share.Token); err != nil {
		t.Fatalf("expected hex token, got %q", share.Token)
	}
}

func TestCardService_CreateOrRotateShare_Success_WithExpiry(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	callCount := 0
	var gotExpiresAt *time.Time

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if callCount == 1 {
				return rowFromValues(userID, true)
			}
			if len(args) < 3 {
				t.Fatalf("expected share insert args, got %d", len(args))
			}
			if args[2] != nil {
				gotExpiresAt, _ = args[2].(*time.Time)
			}
			return rowFromValues(cardID, args[1], time.Now(), &expiresAt, (*time.Time)(nil), 0)
		},
	}

	svc := NewCardService(db)
	share, err := svc.CreateOrRotateShare(context.Background(), userID, cardID, &expiresAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if share.ExpiresAt == nil {
		t.Fatal("expected expires_at to be set")
	}
	if gotExpiresAt == nil {
		t.Fatal("expected expires_at arg to be set")
	}
}

func TestCardService_GetShareStatus_NotOwner(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	ownerID := uuid.New()
	callCount := 0

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if callCount > 1 {
				t.Fatalf("unexpected query after ownership check: %s", sql)
			}
			return rowFromValues(ownerID, false)
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetShareStatus(context.Background(), userID, cardID)
	if !errors.Is(err, ErrNotCardOwner) {
		t.Fatalf("expected ErrNotCardOwner, got %v", err)
	}
}

func TestCardService_GetShareStatus_NotFound(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	callCount := 0

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if callCount == 1 {
				return rowFromValues(userID, true)
			}
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetShareStatus(context.Background(), userID, cardID)
	if !errors.Is(err, ErrShareNotFound) {
		t.Fatalf("expected ErrShareNotFound, got %v", err)
	}
}

func TestCardService_GetSharedCardByToken_Success(t *testing.T) {
	cardID := uuid.New()
	token := "deadbeef"
	year := 2025
	gridSize := 5
	header := "BINGO"
	hasFree := true
	freePos := 12
	expiresAt := (*time.Time)(nil)
	callCount := 0
	var touchCalled bool

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if !strings.Contains(sql, "FROM bingo_card_shares") {
				t.Fatalf("unexpected query for share lookup: %s", sql)
			}
			return rowFromValues(cardID, year, (*string)(nil), (*string)(nil), gridSize, header, hasFree, &freePos, true, expiresAt)
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if !strings.Contains(sql, "FROM bingo_items") {
				t.Fatalf("unexpected query for items: %s", sql)
			}
			return &fakeRows{rows: [][]any{
				{0, "Goal A", false},
				{1, "Goal B", true},
			}}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE bingo_card_shares") {
				touchCalled = true
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewCardService(db)
	shared, err := svc.GetSharedCardByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shared.Card.ID != cardID {
		t.Fatalf("expected card ID %v, got %v", cardID, shared.Card.ID)
	}
	if shared.Card.GridSize != gridSize {
		t.Fatalf("expected grid size %d, got %d", gridSize, shared.Card.GridSize)
	}
	if len(shared.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(shared.Items))
	}
	if shared.Items[1].IsCompleted != true {
		t.Fatal("expected completion state to be true for item 2")
	}
	if !touchCalled {
		t.Fatal("expected share access to be recorded")
	}
}

func TestCardService_GetSharedCardByToken_Expired(t *testing.T) {
	cardID := uuid.New()
	token := "deadbeef"
	expired := time.Now().Add(-1 * time.Hour)

	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(cardID, 2025, (*string)(nil), (*string)(nil), 5, "BINGO", true, (*int)(nil), true, &expired)
		},
	}

	svc := NewCardService(db)
	_, err := svc.GetSharedCardByToken(context.Background(), token)
	if !errors.Is(err, ErrShareNotFound) {
		t.Fatalf("expected ErrShareNotFound, got %v", err)
	}
}
