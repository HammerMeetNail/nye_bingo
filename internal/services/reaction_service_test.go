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

type fakeFriendChecker struct {
	isFriend bool
	err      error
	calls    int
}

func (f *fakeFriendChecker) IsFriend(ctx context.Context, userID, otherUserID uuid.UUID) (bool, error) {
	f.calls++
	return f.isFriend, f.err
}

func TestReactionService_AddReaction_InvalidEmoji(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			t.Fatal("unexpected db call for invalid emoji")
			return nil
		},
	}
	friend := &fakeFriendChecker{}

	service := NewReactionService(db, friend)
	_, err := service.AddReaction(context.Background(), uuid.New(), uuid.New(), "not-an-emoji")
	if !errors.Is(err, ErrInvalidEmoji) {
		t.Fatalf("expected ErrInvalidEmoji, got %v", err)
	}
}

func TestReactionService_AddReaction_ItemNotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	friend := &fakeFriendChecker{}

	service := NewReactionService(db, friend)
	_, err := service.AddReaction(context.Background(), uuid.New(), uuid.New(), "🎉")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
	if friend.calls != 0 {
		t.Fatalf("expected no friend checks, got %d", friend.calls)
	}
}

func TestReactionService_AddReaction_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("query error")
			}}
		},
	}
	friend := &fakeFriendChecker{}

	service := NewReactionService(db, friend)
	_, err := service.AddReaction(context.Background(), uuid.New(), uuid.New(), "🎉")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_AddReaction_CannotReactToOwnItem(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(userID, true)
		},
	}
	friend := &fakeFriendChecker{}

	service := NewReactionService(db, friend)
	_, err := service.AddReaction(context.Background(), userID, uuid.New(), "🎉")
	if !errors.Is(err, ErrCannotReactToOwn) {
		t.Fatalf("expected ErrCannotReactToOwn, got %v", err)
	}
	if friend.calls != 0 {
		t.Fatalf("expected no friend checks, got %d", friend.calls)
	}
}

func TestReactionService_AddReaction_ItemNotCompleted(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(uuid.New(), false)
		},
	}
	friend := &fakeFriendChecker{}

	service := NewReactionService(db, friend)
	_, err := service.AddReaction(context.Background(), userID, uuid.New(), "🎉")
	if !errors.Is(err, ErrItemNotCompleted) {
		t.Fatalf("expected ErrItemNotCompleted, got %v", err)
	}
	if friend.calls != 0 {
		t.Fatalf("expected no friend checks, got %d", friend.calls)
	}
}

func TestReactionService_AddReaction_NotFriend(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(uuid.New(), true)
		},
	}
	friend := &fakeFriendChecker{isFriend: false}

	service := NewReactionService(db, friend)
	_, err := service.AddReaction(context.Background(), uuid.New(), uuid.New(), "🎉")
	if !errors.Is(err, ErrNotFriend) {
		t.Fatalf("expected ErrNotFriend, got %v", err)
	}
	if friend.calls != 1 {
		t.Fatalf("expected friend check, got %d", friend.calls)
	}
}

func TestReactionService_AddReaction_FriendCheckError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(uuid.New(), true)
		},
	}
	friend := &fakeFriendChecker{err: errors.New("friend error")}

	service := NewReactionService(db, friend)
	_, err := service.AddReaction(context.Background(), uuid.New(), uuid.New(), "🎉")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_AddReaction_InsertError(t *testing.T) {
	userID := uuid.New()
	itemID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM bingo_items") {
				return rowFromValues(uuid.New(), true)
			}
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("insert error")
			}}
		},
	}
	friend := &fakeFriendChecker{isFriend: true}

	service := NewReactionService(db, friend)
	_, err := service.AddReaction(context.Background(), userID, itemID, "🎉")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_AddReaction_Success(t *testing.T) {
	userID := uuid.New()
	itemID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM bingo_items") {
				return rowFromValues(uuid.New(), true)
			}
			return rowFromValues(uuid.New(), itemID, userID, "🎉", time.Now())
		},
	}
	friend := &fakeFriendChecker{isFriend: true}

	service := NewReactionService(db, friend)
	reaction, err := service.AddReaction(context.Background(), userID, itemID, "🎉")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reaction.ItemID != itemID {
		t.Fatalf("expected item %v, got %v", itemID, reaction.ItemID)
	}
}

func TestReactionService_RemoveReaction_NotFound(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	err := service.RemoveReaction(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrReactionNotFound) {
		t.Fatalf("expected ErrReactionNotFound, got %v", err)
	}
}

func TestReactionService_RemoveReaction_ExecError(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("exec error")
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	err := service.RemoveReaction(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_RemoveReaction_Success(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	if err := service.RemoveReaction(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReactionService_GetUserReactionForItem_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	reaction, err := service.GetUserReactionForItem(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reaction != nil {
		t.Fatal("expected nil reaction for missing row")
	}
}

func TestReactionService_GetUserReactionForItem_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("query error")
			}}
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	_, err := service.GetUserReactionForItem(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_GetUserReactionForItem_Success(t *testing.T) {
	userID := uuid.New()
	itemID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(uuid.New(), itemID, userID, "🎉", time.Now())
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	reaction, err := service.GetUserReactionForItem(context.Background(), userID, itemID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reaction == nil || reaction.ItemID != itemID {
		t.Fatalf("expected reaction for %v", itemID)
	}
}

func TestReactionService_GetReactionsForItem_Empty(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	reactions, err := service.GetReactionsForItem(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reactions) != 0 {
		t.Fatalf("expected 0 reactions, got %d", len(reactions))
	}
}

func TestReactionService_GetReactionsForItem_ReturnsRows(t *testing.T) {
	itemID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{uuid.New(), itemID, uuid.New(), "🎉", time.Now(), "alice"},
			}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	reactions, err := service.GetReactionsForItem(context.Background(), itemID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reactions) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(reactions))
	}
}

func TestReactionService_GetReactionsForItem_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("query error")
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	_, err := service.GetReactionsForItem(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_GetReactionsForItem_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	_, err := service.GetReactionsForItem(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_GetReactionSummaryForItem_Empty(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	summaries, err := service.GetReactionSummaryForItem(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 summaries, got %d", len(summaries))
	}
}

func TestReactionService_GetReactionSummaryForItem_ReturnsRows(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{"🎉", int64(2)},
				{"👍", int64(1)},
			}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	summaries, err := service.GetReactionSummaryForItem(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
}

func TestReactionService_GetReactionSummaryForItem_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("query error")
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	_, err := service.GetReactionSummaryForItem(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_GetReactionSummaryForItem_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-emoji"}}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	_, err := service.GetReactionSummaryForItem(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_GetReactionsForCard_Empty(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	reactions, err := service.GetReactionsForCard(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reactions) != 0 {
		t.Fatalf("expected empty map, got %d", len(reactions))
	}
}

func TestReactionService_GetReactionsForCard_ReturnsRows(t *testing.T) {
	cardID := uuid.New()
	itemID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{uuid.New(), itemID, uuid.New(), "🎉", time.Now(), "alice"},
			}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	reactions, err := service.GetReactionsForCard(context.Background(), cardID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reactions[itemID]) != 1 {
		t.Fatalf("expected 1 reaction, got %d", len(reactions[itemID]))
	}
}

func TestReactionService_GetReactionsForCard_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("query error")
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	_, err := service.GetReactionsForCard(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_GetReactionsForCard_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	service := NewReactionService(db, &fakeFriendChecker{})
	_, err := service.GetReactionsForCard(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}
