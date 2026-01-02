package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func TestCardService_Finalize_NotifiesFriendsWhenVisible(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	createdAt := time.Now()
	updatedAt := time.Now()

	cardRow := []any{
		cardID,
		userID,
		2024,
		nil,
		nil,
		2,
		"BI",
		false,
		nil,
		true,
		false,
		true,
		false,
		createdAt,
		updatedAt,
	}

	items := []models.BingoItem{
		{ID: uuid.New(), CardID: cardID, Position: 0, Content: "A"},
		{ID: uuid.New(), CardID: cardID, Position: 1, Content: "B"},
		{ID: uuid.New(), CardID: cardID, Position: 2, Content: "C"},
		{ID: uuid.New(), CardID: cardID, Position: 3, Content: "D"},
	}

	var notified bool
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM bingo_cards") {
				return rowFromValues(cardRow...)
			}
			t.Fatalf("unexpected query: %q", sql)
			return rowFromValues()
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if strings.Contains(sql, "FROM bingo_items") {
				rows := make([][]any, 0, len(items))
				for _, item := range items {
					rows = append(rows, []any{item.ID, item.CardID, item.Position, item.Content, item.IsCompleted, item.CompletedAt, item.Notes, item.ProofURL, time.Now()})
				}
				return &fakeRows{rows: rows}, nil
			}
			return &fakeRows{}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE bingo_cards SET is_finalized") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			return fakeCommandTag{}, nil
		},
	}

	svc := NewCardService(db)
	svc.SetNotificationService(&stubNotificationService{
		NotifyFriendsNewCardFunc: func(ctx context.Context, actorID, gotCardID uuid.UUID) error {
			notified = true
			if actorID != userID || gotCardID != cardID {
				t.Fatalf("unexpected notification args: %v %v", actorID, gotCardID)
			}
			return nil
		},
	})

	_, err := svc.Finalize(context.Background(), userID, cardID, &FinalizeParams{VisibleToFriends: boolPtr(true)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !notified {
		t.Fatal("expected notification")
	}
}

func TestCardService_CompleteItem_NotifiesBingo(t *testing.T) {
	userID := uuid.New()
	cardID := uuid.New()
	createdAt := time.Now()
	updatedAt := time.Now()

	cardRow := []any{
		cardID,
		userID,
		2024,
		nil,
		nil,
		2,
		"BI",
		false,
		nil,
		true,
		true,
		true,
		false,
		createdAt,
		updatedAt,
	}

	items := []models.BingoItem{
		{ID: uuid.New(), CardID: cardID, Position: 0, Content: "A", IsCompleted: true},
		{ID: uuid.New(), CardID: cardID, Position: 1, Content: "B", IsCompleted: false},
		{ID: uuid.New(), CardID: cardID, Position: 2, Content: "C", IsCompleted: false},
		{ID: uuid.New(), CardID: cardID, Position: 3, Content: "D", IsCompleted: false},
	}

	var notified bool
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM bingo_cards") {
				return rowFromValues(cardRow...)
			}
			t.Fatalf("unexpected query: %q", sql)
			return rowFromValues()
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			if strings.Contains(sql, "FROM bingo_items") {
				rows := make([][]any, 0, len(items))
				for _, item := range items {
					rows = append(rows, []any{item.ID, item.CardID, item.Position, item.Content, item.IsCompleted, item.CompletedAt, item.Notes, item.ProofURL, time.Now()})
				}
				return &fakeRows{rows: rows}, nil
			}
			return &fakeRows{}, nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "UPDATE bingo_items") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			return fakeCommandTag{}, nil
		},
	}

	svc := NewCardService(db)
	svc.SetNotificationService(&stubNotificationService{
		NotifyFriendsBingoFunc: func(ctx context.Context, actorID, gotCardID uuid.UUID, bingoCount int) error {
			notified = true
			if actorID != userID || gotCardID != cardID {
				t.Fatalf("unexpected notification args: %v %v", actorID, gotCardID)
			}
			if bingoCount == 0 {
				t.Fatal("expected bingo count")
			}
			return nil
		},
	})

	_, err := svc.CompleteItem(context.Background(), userID, cardID, 1, models.CompleteItemParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !notified {
		t.Fatal("expected bingo notification")
	}
}
