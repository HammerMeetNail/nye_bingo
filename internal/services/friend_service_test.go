package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
)

func friendshipRowValues(id, userID, friendID uuid.UUID, status models.FriendshipStatus) []any {
	return []any{id, userID, friendID, status, time.Now()}
}

func TestFriendService_SearchUsers_ShortQuery(t *testing.T) {
	svc := &FriendService{}
	results, err := svc.SearchUsers(context.Background(), uuid.New(), " a ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func TestFriendService_SearchUsers_ReturnsRows(t *testing.T) {
	userID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{userID, "alice"}}}, nil
		},
	}

	svc := NewFriendService(db)
	results, err := svc.SearchUsers(context.Background(), uuid.New(), "al")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != userID || results[0].Username != "alice" {
		t.Fatalf("unexpected result: %+v", results[0])
	}
}

func TestFriendService_SendRequest_Self(t *testing.T) {
	svc := &FriendService{}
	userID := uuid.New()
	_, err := svc.SendRequest(context.Background(), userID, userID)
	if !errors.Is(err, ErrCannotFriendSelf) {
		t.Fatalf("expected ErrCannotFriendSelf, got %v", err)
	}
}

func TestFriendService_SendRequest_AlreadyExists(t *testing.T) {
	calls := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			calls++
			return rowFromValues(true)
		},
	}

	svc := NewFriendService(db)
	_, err := svc.SendRequest(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrFriendshipExists) {
		t.Fatalf("expected ErrFriendshipExists, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected single existence check, got %d", calls)
	}
}

func TestFriendService_AcceptRequest_NotRecipient(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), uuid.New(), models.FriendshipStatusPending)...)
		},
	}

	svc := NewFriendService(db)
	_, err := svc.AcceptRequest(context.Background(), userID, friendshipID)
	if !errors.Is(err, ErrNotFriendshipRecipient) {
		t.Fatalf("expected ErrNotFriendshipRecipient, got %v", err)
	}
}

func TestFriendService_AcceptRequest_NotPending(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), userID, models.FriendshipStatusAccepted)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("unexpected exec on non-pending friendship")
			return fakeCommandTag{}, nil
		},
	}

	svc := NewFriendService(db)
	_, err := svc.AcceptRequest(context.Background(), userID, friendshipID)
	if !errors.Is(err, ErrFriendshipNotPending) {
		t.Fatalf("expected ErrFriendshipNotPending, got %v", err)
	}
}

func TestFriendService_CancelRequest_NotSender(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), uuid.New(), models.FriendshipStatusPending)...)
		},
	}

	svc := NewFriendService(db)
	err := svc.CancelRequest(context.Background(), userID, friendshipID)
	if !errors.Is(err, ErrFriendshipNotFound) {
		t.Fatalf("expected ErrFriendshipNotFound, got %v", err)
	}
}

func TestFriendService_RejectRequest_NotRecipient(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), uuid.New(), models.FriendshipStatusPending)...)
		},
	}

	svc := NewFriendService(db)
	err := svc.RejectRequest(context.Background(), userID, friendshipID)
	if !errors.Is(err, ErrNotFriendshipRecipient) {
		t.Fatalf("expected ErrNotFriendshipRecipient, got %v", err)
	}
}

func TestFriendService_RemoveFriend_NotParticipant(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), uuid.New(), models.FriendshipStatusAccepted)...)
		},
	}

	svc := NewFriendService(db)
	err := svc.RemoveFriend(context.Background(), userID, friendshipID)
	if !errors.Is(err, ErrFriendshipNotFound) {
		t.Fatalf("expected ErrFriendshipNotFound, got %v", err)
	}
}

func TestFriendService_IsFriend_True(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(true)
		},
	}

	svc := NewFriendService(db)
	ok, err := svc.IsFriend(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected friendship to be true")
	}
}

func TestFriendService_GetFriendUserID_NotParticipant(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), uuid.New(), models.FriendshipStatusAccepted)...)
		},
	}

	svc := NewFriendService(db)
	_, err := svc.GetFriendUserID(context.Background(), userID, friendshipID)
	if !errors.Is(err, ErrFriendshipNotFound) {
		t.Fatalf("expected ErrFriendshipNotFound, got %v", err)
	}
}

func TestFriendService_ListFriends_Empty(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewFriendService(db)
	friends, err := svc.ListFriends(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(friends) != 0 {
		t.Fatalf("expected 0 friends, got %d", len(friends))
	}
}

func TestFriendService_ListPendingRequests_Empty(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewFriendService(db)
	requests, err := svc.ListPendingRequests(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("expected 0 requests, got %d", len(requests))
	}
}

func TestFriendService_ListSentRequests_Empty(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
	}

	svc := NewFriendService(db)
	requests, err := svc.ListSentRequests(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("expected 0 requests, got %d", len(requests))
	}
}

func TestFriendService_GetFriendUserID_ReturnsOther(t *testing.T) {
	friendshipID := uuid.New()
	currentUser := uuid.New()
	otherUser := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, currentUser, otherUser, models.FriendshipStatusAccepted)...)
		},
	}

	svc := NewFriendService(db)
	other, err := svc.GetFriendUserID(context.Background(), currentUser, friendshipID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if other != otherUser {
		t.Fatalf("expected other user %v, got %v", otherUser, other)
	}
}
