package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestLockUserPairForUpdate_LocksInStableOrder(t *testing.T) {
	low := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	high := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	var got []uuid.UUID
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			if !strings.Contains(sql, "FROM users") || !strings.Contains(sql, "FOR UPDATE") {
				t.Fatalf("unexpected sql: %q", sql)
			}
			got = append(got, args[0].(uuid.UUID))
			return rowFromValues(args[0])
		},
	}

	if err := lockUserPairForUpdate(context.Background(), db, high, low); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != low || got[1] != high {
		t.Fatalf("unexpected lock order: %+v", got)
	}
}

func TestLockUserPairForUpdate_SameUserLocksOnce(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	var calls int
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			calls++
			return rowFromValues(args[0])
		},
	}

	if err := lockUserPairForUpdate(context.Background(), db, id, id); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 lock call, got %d", calls)
	}
}

func TestLockUserPairForUpdate_PropagatesNoRows(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	var calls int
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			calls++
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}

	if err := lockUserPairForUpdate(context.Background(), db, id, uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestLockUserPairForUpdate_NoRowsOnSecondLock(t *testing.T) {
	low := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	high := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

	var calls int
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			calls++
			id := args[0].(uuid.UUID)
			if id == high {
				return fakeRow{scanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
			}
			return rowFromValues(id)
		},
	}

	if err := lockUserPairForUpdate(context.Background(), db, low, high); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestLockUserForUpdate_WrapsUnexpectedError(t *testing.T) {
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) Row {
			return fakeRow{scanFunc: func(dest ...any) error { return errors.New("boom") }}
		},
	}

	err := lockUserForUpdate(context.Background(), db, uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "lock user") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}
