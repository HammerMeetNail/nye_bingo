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

func TestFriendInviteService_CreateInvite_Success(t *testing.T) {
	inviterID := uuid.New()
	inviteID := uuid.New()
	now := time.Now()

	var gotArgs []any
	callCount := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if strings.Contains(sql, "COUNT") {
				return rowFromValues(0)
			}
			if strings.Contains(sql, "INSERT INTO friend_invites") {
				gotArgs = args
				return rowFromValues(inviteID, inviterID, &now, nil, nil, nil, now)
			}
			t.Fatalf("unexpected sql: %q", sql)
			return rowFromValues()
		},
	}

	svc := NewFriendInviteService(db)
	invite, token, err := svc.CreateInvite(context.Background(), inviterID, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 query calls, got %d", callCount)
	}
	if token == "" {
		t.Fatal("expected invite token")
	}
	if invite.ID != inviteID || invite.InviterUserID != inviterID {
		t.Fatalf("unexpected invite: %+v", invite)
	}
	if len(gotArgs) != 3 {
		t.Fatalf("expected 3 args, got %d", len(gotArgs))
	}
	if gotArgs[0] != inviterID {
		t.Fatalf("expected inviterID arg, got %v", gotArgs[0])
	}
	if gotArgs[1] == "" {
		t.Fatal("expected token hash arg")
	}
	if gotArgs[2] == nil {
		t.Fatal("expected expires_at arg")
	}
}

func TestFriendInviteService_CreateInvite_InvalidExpiry(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			t.Fatal("expected no database calls for invalid expiry")
			return rowFromValues()
		},
	}

	svc := NewFriendInviteService(db)
	_, _, err := svc.CreateInvite(context.Background(), uuid.New(), InviteExpiryMaxDays+1)
	if !errors.Is(err, ErrInviteExpiryOutOfRange) {
		t.Fatalf("expected ErrInviteExpiryOutOfRange, got %v", err)
	}
}

func TestFriendInviteService_CreateInvite_LimitReached(t *testing.T) {
	inviterID := uuid.New()
	callCount := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			callCount++
			if !strings.Contains(sql, "COUNT") {
				t.Fatalf("unexpected sql: %q", sql)
			}
			return rowFromValues(InviteMaxActive)
		},
	}

	svc := NewFriendInviteService(db)
	_, _, err := svc.CreateInvite(context.Background(), inviterID, 7)
	if !errors.Is(err, ErrInviteLimitReached) {
		t.Fatalf("expected ErrInviteLimitReached, got %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 query call, got %d", callCount)
	}
}

func TestFriendInviteService_ListInvites_ReturnsRows(t *testing.T) {
	inviterID := uuid.New()
	inviteID := uuid.New()
	now := time.Now()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{inviteID, inviterID, &now, nil, nil, nil, now},
			}}, nil
		},
	}

	svc := NewFriendInviteService(db)
	invites, err := svc.ListInvites(context.Background(), inviterID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(invites) != 1 {
		t.Fatalf("expected 1 invite, got %d", len(invites))
	}
	if invites[0].ID != inviteID {
		t.Fatalf("unexpected invite: %+v", invites[0])
	}
}

func TestFriendInviteService_RevokeInvite_NotFound(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}
	svc := NewFriendInviteService(db)
	if err := svc.RevokeInvite(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("expected ErrInviteNotFound, got %v", err)
	}
}

func TestFriendInviteService_AcceptInvite_NotFound(t *testing.T) {
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}
	svc := NewFriendInviteService(db)
	_, err := svc.AcceptInvite(context.Background(), uuid.New(), "token")
	if !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("expected ErrInviteNotFound, got %v", err)
	}
}

func TestFriendInviteService_AcceptInvite_Self(t *testing.T) {
	userID := uuid.New()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(uuid.New(), userID, "self")
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}
	svc := NewFriendInviteService(db)
	_, err := svc.AcceptInvite(context.Background(), userID, "token")
	if !errors.Is(err, ErrCannotFriendSelf) {
		t.Fatalf("expected ErrCannotFriendSelf, got %v", err)
	}
}

func TestFriendInviteService_AcceptInvite_Blocked(t *testing.T) {
	userID := uuid.New()
	inviterID := uuid.New()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM friend_invites") {
				return rowFromValues(uuid.New(), inviterID, "inviter")
			}
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				return rowFromValues(args[0])
			}
			if strings.Contains(sql, "FROM user_blocks") {
				return rowFromValues(true)
			}
			t.Fatalf("unexpected sql: %q", sql)
			return rowFromValues()
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}
	svc := NewFriendInviteService(db)
	_, err := svc.AcceptInvite(context.Background(), userID, "token")
	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf("expected ErrUserBlocked, got %v", err)
	}
}

func TestFriendInviteService_AcceptInvite_Success(t *testing.T) {
	userID := uuid.New()
	inviterID := uuid.New()
	inviteID := uuid.New()
	friendshipID := uuid.New()
	var execCalls int
	var notified bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM friend_invites") {
				return rowFromValues(inviteID, inviterID, "inviter")
			}
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				return rowFromValues(args[0])
			}
			if strings.Contains(sql, "FROM user_blocks") {
				return rowFromValues(false)
			}
			if strings.Contains(sql, "FROM friendships") {
				return rowFromValues(false)
			}
			if strings.Contains(sql, "INSERT INTO friendships") {
				return rowFromValues(friendshipID)
			}
			t.Fatalf("unexpected sql: %q", sql)
			return rowFromValues()
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalls++
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}
	svc := NewFriendInviteService(db)
	svc.SetNotificationService(&stubNotificationService{
		NotifyFriendRequestAcceptedFunc: func(ctx context.Context, recipientID, actorID, gotFriendshipID uuid.UUID) error {
			notified = true
			if recipientID != inviterID || actorID != userID || gotFriendshipID != friendshipID {
				t.Fatalf("unexpected notification args: %v %v %v", recipientID, actorID, gotFriendshipID)
			}
			return nil
		},
	})
	inviter, err := svc.AcceptInvite(context.Background(), userID, "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inviter.ID != inviterID || inviter.Username != "inviter" {
		t.Fatalf("unexpected inviter: %+v", inviter)
	}
	if execCalls != 1 {
		t.Fatalf("expected 1 exec call, got %d", execCalls)
	}
	if !notified {
		t.Fatal("expected notification")
	}
}

func TestFriendInviteService_AcceptInvite_LockError(t *testing.T) {
	userID := uuid.New()
	inviterID := uuid.New()

	var rolledBack bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM friend_invites") {
				return rowFromValues(uuid.New(), inviterID, "inviter")
			}
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				return fakeRow{scanFunc: func(dest ...any) error { return errors.New("boom") }}
			}
			t.Fatalf("unexpected sql: %q", sql)
			return rowFromValues()
		},
		RollbackFunc: func(ctx context.Context) error {
			rolledBack = true
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}
	svc := NewFriendInviteService(db)
	_, err := svc.AcceptInvite(context.Background(), userID, "token")
	if err == nil {
		t.Fatal("expected error")
	}
	if !rolledBack {
		t.Fatal("expected rollback")
	}
}
