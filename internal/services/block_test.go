package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestBlockService_Block_Self(t *testing.T) {
	svc := &BlockService{}
	id := uuid.New()
	if err := svc.Block(context.Background(), id, id); !errors.Is(err, ErrCannotBlockSelf) {
		t.Fatalf("expected ErrCannotBlockSelf, got %v", err)
	}
}

func TestBlockService_Block_BeginError(t *testing.T) {
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return nil, errors.New("boom")
		},
	}
	svc := NewBlockService(db)
	if err := svc.Block(context.Background(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error")
	}
}

func TestBlockService_Block_AlreadyBlocked(t *testing.T) {
	var rolledBack bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				return rowFromValues(args[0])
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues()
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if !strings.Contains(sql, "INSERT INTO user_blocks") {
				t.Fatalf("unexpected sql: %q", sql)
			}
			return fakeCommandTag{rowsAffected: 0}, nil
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
	svc := NewBlockService(db)
	if err := svc.Block(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrBlockExists) {
		t.Fatalf("expected ErrBlockExists, got %v", err)
	}
	if !rolledBack {
		t.Fatal("expected rollback on duplicate block")
	}
}

func TestBlockService_Block_UserNotFound(t *testing.T) {
	blockedID := uuid.New()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				if args[0] == blockedID {
					return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
				}
				return rowFromValues(args[0])
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues()
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("expected to fail before insert")
			return fakeCommandTag{}, &pgconn.PgError{Code: "23503"}
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}
	svc := NewBlockService(db)
	if err := svc.Block(context.Background(), uuid.New(), blockedID); !errors.Is(err, ErrBlockedUserNotFound) {
		t.Fatalf("expected ErrBlockedUserNotFound, got %v", err)
	}
}

func TestBlockService_Block_UserNotFound_FKViolationFallback(t *testing.T) {
	var rolledBack bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				return rowFromValues(args[0])
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues()
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if !strings.Contains(sql, "INSERT INTO user_blocks") {
				t.Fatalf("unexpected exec sql: %q", sql)
			}
			return fakeCommandTag{}, &pgconn.PgError{Code: "23503"}
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
	svc := NewBlockService(db)
	if err := svc.Block(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrBlockedUserNotFound) {
		t.Fatalf("expected ErrBlockedUserNotFound, got %v", err)
	}
	if !rolledBack {
		t.Fatal("expected rollback")
	}
}

func TestBlockService_Block_Success(t *testing.T) {
	var execCalls int
	var committed bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				return rowFromValues(args[0])
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues()
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			execCalls++
			switch execCalls {
			case 1:
				if !strings.Contains(sql, "INSERT INTO user_blocks") {
					t.Fatalf("unexpected insert sql: %q", sql)
				}
				return fakeCommandTag{rowsAffected: 1}, nil
			case 2:
				if !strings.Contains(sql, "DELETE FROM friendships") {
					t.Fatalf("unexpected delete sql: %q", sql)
				}
				return fakeCommandTag{rowsAffected: 1}, nil
			default:
				t.Fatalf("unexpected exec call %d", execCalls)
				return fakeCommandTag{}, nil
			}
		},
		CommitFunc: func(ctx context.Context) error {
			committed = true
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}

	svc := NewBlockService(db)
	if err := svc.Block(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !committed {
		t.Fatal("expected commit")
	}
}

func TestBlockService_Block_DeleteError(t *testing.T) {
	var rolledBack bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				return rowFromValues(args[0])
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues()
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO user_blocks") {
				return fakeCommandTag{rowsAffected: 1}, nil
			}
			if strings.Contains(sql, "DELETE FROM friendships") {
				return fakeCommandTag{}, errors.New("boom")
			}
			t.Fatalf("unexpected sql: %q", sql)
			return fakeCommandTag{}, nil
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
	svc := NewBlockService(db)
	if err := svc.Block(context.Background(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error")
	}
	if !rolledBack {
		t.Fatal("expected rollback on delete error")
	}
}

func TestBlockService_Block_LockError(t *testing.T) {
	var rolledBack bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error { return errors.New("boom") }}
		},
		RollbackFunc: func(ctx context.Context) error {
			rolledBack = true
			return nil
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			t.Fatal("expected lock failure before exec")
			return fakeCommandTag{}, nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (Tx, error) {
			return tx, nil
		},
	}
	svc := NewBlockService(db)
	if err := svc.Block(context.Background(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error")
	}
	if !rolledBack {
		t.Fatal("expected rollback")
	}
}

func TestBlockService_Block_CommitError(t *testing.T) {
	var rolledBack bool
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if strings.Contains(sql, "FROM users") && strings.Contains(sql, "FOR UPDATE") {
				return rowFromValues(args[0])
			}
			t.Fatalf("unexpected query sql: %q", sql)
			return rowFromValues()
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			return errors.New("boom")
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
	svc := NewBlockService(db)
	if err := svc.Block(context.Background(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("expected error")
	}
	if !rolledBack {
		t.Fatal("expected rollback on commit error")
	}
}

func TestBlockService_Unblock_Success(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	svc := NewBlockService(db)
	if err := svc.Unblock(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBlockService_Unblock_NotFound(t *testing.T) {
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}
	svc := NewBlockService(db)
	if err := svc.Unblock(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("expected ErrBlockNotFound, got %v", err)
	}
}

func TestBlockService_IsBlocked(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return rowFromValues(true)
		},
	}
	svc := NewBlockService(db)
	blocked, err := svc.IsBlocked(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatal("expected blocked to be true")
	}
}

func TestBlockService_ListBlocked_ReturnsRows(t *testing.T) {
	userID := uuid.New()
	blockedID := uuid.New()
	now := time.Now()
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{
				{blockedID, "blocked", now},
			}}, nil
		},
	}
	svc := NewBlockService(db)
	blocked, err := svc.ListBlocked(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 1 {
		t.Fatalf("expected 1 blocked user, got %d", len(blocked))
	}
	if blocked[0].ID != blockedID || blocked[0].Username != "blocked" {
		t.Fatalf("unexpected blocked user: %+v", blocked[0])
	}
	if blocked[0].BlockedAt != now {
		t.Fatalf("unexpected blocked at: %v", blocked[0].BlockedAt)
	}
}

func TestBlockService_ListBlocked_ScanError(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{{"bad-id"}}}, nil
		},
	}
	svc := NewBlockService(db)
	_, err := svc.ListBlocked(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBlockService_ListBlocked_Empty(t *testing.T) {
	db := &fakeDB{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &fakeRows{rows: [][]any{}}, nil
		},
	}
	svc := NewBlockService(db)
	blocked, err := svc.ListBlocked(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 0 {
		t.Fatalf("expected empty list, got %d", len(blocked))
	}
}
