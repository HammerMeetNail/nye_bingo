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

func TestFriendService_SearchUsers_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("boom")
		},
	}

	svc := NewFriendService(db)
	_, err := svc.SearchUsers(context.Background(), uuid.New(), "al")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendService_SearchUsers_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	svc := NewFriendService(db)
	_, err := svc.SearchUsers(context.Background(), uuid.New(), "al")
	if err == nil {
		t.Fatal("expected error")
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
	userID := uuid.New()
	friendID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			calls++
			if !strings.Contains(sql, "SELECT EXISTS") || !strings.Contains(sql, "FROM friendships") {
				t.Fatalf("unexpected existence sql: %q", sql)
			}
			if len(args) != 2 || args[0] != userID || args[1] != friendID {
				t.Fatalf("unexpected existence args: %v", args)
			}
			return rowFromValues(true)
		},
	}

	svc := NewFriendService(db)
	_, err := svc.SendRequest(context.Background(), userID, friendID)
	if !errors.Is(err, ErrFriendshipExists) {
		t.Fatalf("expected ErrFriendshipExists, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected single existence check, got %d", calls)
	}
}

func TestFriendService_SendRequest_ExistenceError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	svc := NewFriendService(db)
	_, err := svc.SendRequest(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendService_SendRequest_Success(t *testing.T) {
	userID := uuid.New()
	friendID := uuid.New()
	friendshipID := uuid.New()
	call := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			call++
			if call == 1 {
				if !strings.Contains(sql, "SELECT EXISTS") || !strings.Contains(sql, "FROM friendships") {
					t.Fatalf("unexpected existence sql: %q", sql)
				}
				if len(args) != 2 || args[0] != userID || args[1] != friendID {
					t.Fatalf("unexpected existence args: %v", args)
				}
				return rowFromValues(false)
			}
			if !strings.Contains(sql, "INSERT INTO friendships") || !strings.Contains(sql, "RETURNING id, user_id, friend_id, status, created_at") {
				t.Fatalf("unexpected insert sql: %q", sql)
			}
			if len(args) != 2 || args[0] != userID || args[1] != friendID {
				t.Fatalf("unexpected insert args: %v", args)
			}
			return rowFromValues(friendshipRowValues(friendshipID, userID, friendID, models.FriendshipStatusPending)...)
		},
	}

	svc := NewFriendService(db)
	friendship, err := svc.SendRequest(context.Background(), userID, friendID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if friendship.ID != friendshipID {
		t.Fatalf("expected friendship %v, got %v", friendshipID, friendship.ID)
	}
}

func TestFriendService_SendRequest_InsertError(t *testing.T) {
	userID := uuid.New()
	friendID := uuid.New()
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

	svc := NewFriendService(db)
	_, err := svc.SendRequest(context.Background(), userID, friendID)
	if err == nil {
		t.Fatal("expected error")
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

func TestFriendService_AcceptRequest_Success(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), userID, models.FriendshipStatusPending)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if !strings.Contains(sql, "UPDATE friendships SET status = 'accepted'") || !strings.Contains(sql, "WHERE id = $1") {
				t.Fatalf("unexpected accept sql: %q", sql)
			}
			if len(args) != 1 || args[0] != friendshipID {
				t.Fatalf("unexpected accept args: %v", args)
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewFriendService(db)
	friendship, err := svc.AcceptRequest(context.Background(), userID, friendshipID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if friendship.Status != models.FriendshipStatusAccepted {
		t.Fatalf("expected accepted status, got %s", friendship.Status)
	}
}

func TestFriendService_AcceptRequest_ExecError(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), userID, models.FriendshipStatusPending)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	svc := NewFriendService(db)
	_, err := svc.AcceptRequest(context.Background(), userID, friendshipID)
	if err == nil {
		t.Fatal("expected error")
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

func TestFriendService_CancelRequest_Success(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, userID, uuid.New(), models.FriendshipStatusPending)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewFriendService(db)
	if err := svc.CancelRequest(context.Background(), userID, friendshipID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFriendService_CancelRequest_ExecError(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, userID, uuid.New(), models.FriendshipStatusPending)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	svc := NewFriendService(db)
	err := svc.CancelRequest(context.Background(), userID, friendshipID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendService_CancelRequest_NotPending(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, userID, uuid.New(), models.FriendshipStatusAccepted)...)
		},
	}

	svc := NewFriendService(db)
	err := svc.CancelRequest(context.Background(), userID, friendshipID)
	if !errors.Is(err, ErrFriendshipNotPending) {
		t.Fatalf("expected ErrFriendshipNotPending, got %v", err)
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

func TestFriendService_RejectRequest_Success(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), userID, models.FriendshipStatusPending)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewFriendService(db)
	if err := svc.RejectRequest(context.Background(), userID, friendshipID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFriendService_RejectRequest_ExecError(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), userID, models.FriendshipStatusPending)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	svc := NewFriendService(db)
	err := svc.RejectRequest(context.Background(), userID, friendshipID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendService_RejectRequest_NotPending(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, uuid.New(), userID, models.FriendshipStatusAccepted)...)
		},
	}

	svc := NewFriendService(db)
	err := svc.RejectRequest(context.Background(), userID, friendshipID)
	if !errors.Is(err, ErrFriendshipNotPending) {
		t.Fatalf("expected ErrFriendshipNotPending, got %v", err)
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

func TestFriendService_RemoveFriend_Success(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, userID, uuid.New(), models.FriendshipStatusAccepted)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}

	svc := NewFriendService(db)
	if err := svc.RemoveFriend(context.Background(), userID, friendshipID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFriendService_RemoveFriend_ExecError(t *testing.T) {
	friendshipID := uuid.New()
	userID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, userID, uuid.New(), models.FriendshipStatusAccepted)...)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{}, errors.New("boom")
		},
	}

	svc := NewFriendService(db)
	err := svc.RemoveFriend(context.Background(), userID, friendshipID)
	if err == nil {
		t.Fatal("expected error")
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

func TestFriendService_IsFriend_False(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(false)
		},
	}

	svc := NewFriendService(db)
	ok, err := svc.IsFriend(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected friendship to be false")
	}
}

func TestFriendService_IsFriend_Error(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return errors.New("boom")
			}}
		},
	}

	svc := NewFriendService(db)
	_, err := svc.IsFriend(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
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

func TestFriendService_ListFriends_ReturnsRows(t *testing.T) {
	userID := uuid.New()
	friendshipID := uuid.New()
	friendID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{friendshipID, userID, friendID, models.FriendshipStatusAccepted, time.Now(), "friend", "friend@example.com"},
			}}, nil
		},
	}

	svc := NewFriendService(db)
	friends, err := svc.ListFriends(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(friends) != 1 {
		t.Fatalf("expected 1 friend, got %d", len(friends))
	}
}

func TestFriendService_ListFriends_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("boom")
		},
	}

	svc := NewFriendService(db)
	_, err := svc.ListFriends(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendService_ListFriends_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	svc := NewFriendService(db)
	_, err := svc.ListFriends(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
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

func TestFriendService_ListPendingRequests_ReturnsRows(t *testing.T) {
	userID := uuid.New()
	friendshipID := uuid.New()
	friendID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{friendshipID, friendID, userID, models.FriendshipStatusPending, time.Now(), "sender", "sender@example.com"},
			}}, nil
		},
	}

	svc := NewFriendService(db)
	requests, err := svc.ListPendingRequests(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
}

func TestFriendService_ListPendingRequests_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("boom")
		},
	}

	svc := NewFriendService(db)
	_, err := svc.ListPendingRequests(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendService_ListPendingRequests_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	svc := NewFriendService(db)
	_, err := svc.ListPendingRequests(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
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

func TestFriendService_ListSentRequests_ReturnsRows(t *testing.T) {
	userID := uuid.New()
	friendshipID := uuid.New()
	friendID := uuid.New()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{friendshipID, userID, friendID, models.FriendshipStatusPending, time.Now(), "friend", "friend@example.com"},
			}}, nil
		},
	}

	svc := NewFriendService(db)
	requests, err := svc.ListSentRequests(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
}

func TestFriendService_ListSentRequests_QueryError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return nil, errors.New("boom")
		},
	}

	svc := NewFriendService(db)
	_, err := svc.ListSentRequests(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFriendService_ListSentRequests_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}

	svc := NewFriendService(db)
	_, err := svc.ListSentRequests(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
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

func TestFriendService_GetFriendUserID_NotAccepted(t *testing.T) {
	friendshipID := uuid.New()
	currentUser := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(friendshipRowValues(friendshipID, currentUser, uuid.New(), models.FriendshipStatusPending)...)
		},
	}

	svc := NewFriendService(db)
	_, err := svc.GetFriendUserID(context.Background(), currentUser, friendshipID)
	if !errors.Is(err, ErrNotFriend) {
		t.Fatalf("expected ErrNotFriend, got %v", err)
	}
}

func TestFriendService_GetByID_NotFound(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	svc := NewFriendService(db)
	_, err := svc.getByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrFriendshipNotFound) {
		t.Fatalf("expected ErrFriendshipNotFound, got %v", err)
	}
}
