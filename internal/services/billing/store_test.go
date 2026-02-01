package billing

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

type fakeCommandTag struct {
	rowsAffected int64
}

func (f fakeCommandTag) RowsAffected() int64 {
	return f.rowsAffected
}

type fakeRow struct {
	scanFunc func(dest ...any) error
}

func (f fakeRow) Scan(dest ...any) error {
	if f.scanFunc == nil {
		return fmt.Errorf("scanFunc not set")
	}
	return f.scanFunc(dest...)
}

type fakeDB struct {
	ExecFunc     func(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) services.Row
	BeginFunc    func(ctx context.Context) (services.Tx, error)
}

func (f *fakeDB) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	if f.ExecFunc != nil {
		return f.ExecFunc(ctx, sql, args...)
	}
	return fakeCommandTag{}, nil
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...any) (services.Rows, error) {
	return nil, nil
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...any) services.Row {
	if f.QueryRowFunc != nil {
		return f.QueryRowFunc(ctx, sql, args...)
	}
	return fakeRow{scanFunc: func(dest ...any) error {
		return fmt.Errorf("queryRowFunc not set")
	}}
}

func (f *fakeDB) Begin(ctx context.Context) (services.Tx, error) {
	if f.BeginFunc != nil {
		return f.BeginFunc(ctx)
	}
	return nil, fmt.Errorf("beginFunc not set")
}

type fakeTx struct {
	ExecFunc     func(ctx context.Context, sql string, args ...any) (services.CommandTag, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) services.Row
	CommitFunc   func(ctx context.Context) error
	RollbackFunc func(ctx context.Context) error
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
	if f.ExecFunc != nil {
		return f.ExecFunc(ctx, sql, args...)
	}
	return fakeCommandTag{}, nil
}

func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (services.Rows, error) {
	return nil, nil
}

func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) services.Row {
	if f.QueryRowFunc != nil {
		return f.QueryRowFunc(ctx, sql, args...)
	}
	return fakeRow{scanFunc: func(dest ...any) error {
		return fmt.Errorf("queryRowFunc not set")
	}}
}

func (f *fakeTx) Commit(ctx context.Context) error {
	if f.CommitFunc != nil {
		return f.CommitFunc(ctx)
	}
	return nil
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	if f.RollbackFunc != nil {
		return f.RollbackFunc(ctx)
	}
	return nil
}

func rowFromValues(values ...any) services.Row {
	return fakeRow{scanFunc: func(dest ...any) error {
		return assignRow(dest, values)
	}}
}

func assignRow(dest []any, values []any) error {
	if len(dest) != len(values) {
		return fmt.Errorf("scan dest mismatch: got %d want %d", len(dest), len(values))
	}
	for i, value := range values {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr || dv.IsNil() {
			return fmt.Errorf("dest %d not pointer", i)
		}
		if value == nil {
			dv.Elem().Set(reflect.Zero(dv.Elem().Type()))
			continue
		}
		vv := reflect.ValueOf(value)
		if vv.Type().AssignableTo(dv.Elem().Type()) {
			dv.Elem().Set(vv)
			continue
		}
		if vv.Type().ConvertibleTo(dv.Elem().Type()) {
			dv.Elem().Set(vv.Convert(dv.Elem().Type()))
			continue
		}
		return fmt.Errorf("cannot assign %T to %s", value, dv.Elem().Type())
	}
	return nil
}

func TestStore_GetStripeCustomerID_ReturnsValue(t *testing.T) {
	ctx := context.Background()
	value := "cus_123"
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues(&value)
		},
	}
	store := NewStore(db)

	got, err := store.GetStripeCustomerID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != value {
		t.Fatalf("expected %q, got %v", value, got)
	}
}

func TestStore_GetStripeCustomerID_NoRows(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	store := NewStore(db)

	_, err := store.GetStripeCustomerID(ctx, uuid.New())
	if !errors.Is(err, ErrBillingUserNotFound) {
		t.Fatalf("expected ErrBillingUserNotFound, got %v", err)
	}
}

func TestStore_EnsureStripeCustomerID_Existing(t *testing.T) {
	ctx := context.Background()
	value := "cus_existing"
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues(&value)
		},
	}
	store := NewStore(db)
	created := false

	got, err := store.EnsureStripeCustomerID(ctx, uuid.New(), func(ctx context.Context) (string, error) {
		created = true
		return "cus_new", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Fatal("expected createFn not called")
	}
	if got != value {
		t.Fatalf("expected %q, got %q", value, got)
	}
}

func TestStore_EnsureStripeCustomerID_CreateUpdatesRow(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues((*string)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	store := NewStore(db)

	got, err := store.EnsureStripeCustomerID(ctx, uuid.New(), func(ctx context.Context) (string, error) {
		return "cus_new", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cus_new" {
		t.Fatalf("expected cus_new, got %q", got)
	}
}

func TestStore_EnsureStripeCustomerID_ConcurrentCreateUsesExisting(t *testing.T) {
	ctx := context.Background()
	existing := "cus_existing"
	calls := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			calls++
			if calls == 1 {
				return rowFromValues((*string)(nil))
			}
			return rowFromValues(&existing)
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}
	store := NewStore(db)

	got, err := store.EnsureStripeCustomerID(ctx, uuid.New(), func(ctx context.Context) (string, error) {
		return "cus_new", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != existing {
		t.Fatalf("expected %q, got %q", existing, got)
	}
}

func TestStore_EnsureStripeCustomerID_ConcurrentCreateMissing(t *testing.T) {
	ctx := context.Background()
	calls := 0
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			calls++
			return rowFromValues((*string)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 0}, nil
		},
	}
	store := NewStore(db)

	_, err := store.EnsureStripeCustomerID(ctx, uuid.New(), func(ctx context.Context) (string, error) {
		return "cus_new", nil
	})
	if err == nil || !strings.Contains(err.Error(), "stripe customer id not set after create") {
		t.Fatalf("expected missing create error, got %v", err)
	}
}

func TestStore_SetStripeIDs_Error(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{}, fmt.Errorf("boom")
		},
	}
	store := NewStore(db)

	err := store.SetStripeIDs(ctx, uuid.New(), "cus", "sub", db)
	if err == nil || !strings.Contains(err.Error(), "set stripe ids") {
		t.Fatalf("expected set stripe ids error, got %v", err)
	}
}

func TestStore_WithWebhookEvent_AlreadyProcessed(t *testing.T) {
	ctx := context.Background()
	processedAt := time.Now().UTC()
	var commitCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues(&processedAt)
		},
		CommitFunc: func(ctx context.Context) error {
			commitCalls++
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	called := false
	already, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_1"}, func(ctx context.Context, tx services.Tx) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !already {
		t.Fatal("expected already processed")
	}
	if called {
		t.Fatal("expected handler not called")
	}
	if commitCalls != 1 {
		t.Fatalf("expected commit, got %d", commitCalls)
	}
}

func TestStore_WithWebhookEvent_HandlerError(t *testing.T) {
	ctx := context.Background()
	var commitCalls int
	var execCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues((*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execCalls++
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			commitCalls++
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	wantErr := errors.New("handler failed")
	already, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_2"}, func(ctx context.Context, tx services.Tx) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected handler error, got %v", err)
	}
	if already {
		t.Fatal("expected not already processed")
	}
	if commitCalls != 1 {
		t.Fatalf("expected commit, got %d", commitCalls)
	}
	if execCalls == 0 {
		t.Fatal("expected error update exec")
	}
}

func TestStore_WithWebhookEvent_InsertError(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{}, fmt.Errorf("insert failed")
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	_, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_1"}, func(ctx context.Context, tx services.Tx) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "insert webhook event") {
		t.Fatalf("expected insert webhook event error, got %v", err)
	}
}

func TestStore_WithWebhookEvent_Success(t *testing.T) {
	ctx := context.Background()
	var commitCalls int
	var execCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues((*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execCalls++
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			commitCalls++
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	already, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_3"}, func(ctx context.Context, tx services.Tx) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if already {
		t.Fatal("expected not already processed")
	}
	if commitCalls != 1 {
		t.Fatalf("expected commit, got %d", commitCalls)
	}
	if execCalls < 2 {
		t.Fatalf("expected exec calls, got %d", execCalls)
	}
}

func TestStore_FindUserIDByStripeCustomerID_NoRows(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	store := NewStore(db)

	_, err := store.FindUserIDByStripeCustomerID(ctx, "cus", db)
	if !errors.Is(err, ErrBillingUserNotFound) {
		t.Fatalf("expected ErrBillingUserNotFound, got %v", err)
	}
}

func TestStore_GrantLifetime_Error(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{}, fmt.Errorf("boom")
		},
	}
	db := &fakeDB{}
	store := NewStore(db)

	err := store.GrantLifetime(ctx, uuid.New(), "cus", tx)
	if err == nil || !strings.Contains(err.Error(), "grant lifetime") {
		t.Fatalf("expected grant lifetime error, got %v", err)
	}
}

func TestStore_ResetToFree_Error(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{}, fmt.Errorf("boom")
		},
	}
	db := &fakeDB{}
	store := NewStore(db)

	err := store.ResetToFree(ctx, uuid.New(), tx)
	if err == nil || !strings.Contains(err.Error(), "reset billing") {
		t.Fatalf("expected reset billing error, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_NoRows(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", time.Now())
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_AlreadyRedeemed(t *testing.T) {
	ctx := context.Background()
	redeemed := time.Now().UTC()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			codeID := uuid.New()
			return rowFromValues(codeID, (*int)(nil), (*time.Time)(nil), &redeemed)
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", time.Now())
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_Expired(t *testing.T) {
	ctx := context.Background()
	expired := time.Now().UTC().Add(-time.Hour)
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			codeID := uuid.New()
			return rowFromValues(codeID, (*int)(nil), &expired, (*time.Time)(nil))
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", time.Now())
	if !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_Success(t *testing.T) {
	ctx := context.Background()
	duration := 30
	now := time.Now().UTC()
	var execCalls int
	var commitCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			codeID := uuid.New()
			return rowFromValues(codeID, &duration, (*time.Time)(nil), (*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execCalls++
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			commitCalls++
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if execCalls < 2 {
		t.Fatalf("expected exec calls, got %d", execCalls)
	}
	if commitCalls != 1 {
		t.Fatalf("expected commit, got %d", commitCalls)
	}
}

func TestStore_RedeemPremiumCode_UpdateError(t *testing.T) {
	ctx := context.Background()
	var execCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			codeID := uuid.New()
			return rowFromValues(codeID, (*int)(nil), (*time.Time)(nil), (*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execCalls++
			return fakeCommandTag{}, fmt.Errorf("update failed")
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", time.Now())
	if err == nil || !strings.Contains(err.Error(), "mark code redeemed") {
		t.Fatalf("expected mark code redeemed error, got %v", err)
	}
	if execCalls != 1 {
		t.Fatalf("expected 1 exec call, got %d", execCalls)
	}
}

func TestStore_GetStripeCustomerID_DBError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return fmt.Errorf("db connection failed")
			}}
		},
	}
	store := NewStore(db)

	_, err := store.GetStripeCustomerID(ctx, uuid.New())
	if err == nil || !strings.Contains(err.Error(), "get stripe customer id") {
		t.Fatalf("expected get stripe customer id error, got %v", err)
	}
}

func TestStore_EnsureStripeCustomerID_GetError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return fmt.Errorf("db connection failed")
			}}
		},
	}
	store := NewStore(db)

	_, err := store.EnsureStripeCustomerID(ctx, uuid.New(), func(ctx context.Context) (string, error) {
		return "cus_new", nil
	})
	if err == nil || !strings.Contains(err.Error(), "get stripe customer id") {
		t.Fatalf("expected get error, got %v", err)
	}
}

func TestStore_EnsureStripeCustomerID_CreateError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues((*string)(nil))
		},
	}
	store := NewStore(db)

	_, err := store.EnsureStripeCustomerID(ctx, uuid.New(), func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("create customer failed")
	})
	if err == nil || !strings.Contains(err.Error(), "create customer failed") {
		t.Fatalf("expected create error, got %v", err)
	}
}

func TestStore_EnsureStripeCustomerID_ExecError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues((*string)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{}, fmt.Errorf("exec failed")
		},
	}
	store := NewStore(db)

	_, err := store.EnsureStripeCustomerID(ctx, uuid.New(), func(ctx context.Context) (string, error) {
		return "cus_new", nil
	})
	if err == nil || !strings.Contains(err.Error(), "set stripe customer id") {
		t.Fatalf("expected set error, got %v", err)
	}
}

func TestStore_SetStripeIDs_Success(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	store := NewStore(db)

	err := store.SetStripeIDs(ctx, uuid.New(), "cus", "sub", db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStore_FindUserIDByStripeCustomerID_Success(t *testing.T) {
	ctx := context.Background()
	expectedID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues(expectedID)
		},
	}
	store := NewStore(db)

	got, err := store.FindUserIDByStripeCustomerID(ctx, "cus", db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expectedID {
		t.Fatalf("expected %s, got %s", expectedID, got)
	}
}

func TestStore_FindUserIDByStripeCustomerID_DBError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return fmt.Errorf("db error")
			}}
		},
	}
	store := NewStore(db)

	_, err := store.FindUserIDByStripeCustomerID(ctx, "cus", db)
	if err == nil || !strings.Contains(err.Error(), "find user by customer id") {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestStore_FindUserIDByStripeSubscriptionID_Success(t *testing.T) {
	ctx := context.Background()
	expectedID := uuid.New()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues(expectedID)
		},
	}
	store := NewStore(db)

	got, err := store.FindUserIDByStripeSubscriptionID(ctx, "sub", db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expectedID {
		t.Fatalf("expected %s, got %s", expectedID, got)
	}
}

func TestStore_FindUserIDByStripeSubscriptionID_NoRows(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	store := NewStore(db)

	_, err := store.FindUserIDByStripeSubscriptionID(ctx, "sub", db)
	if !errors.Is(err, ErrBillingUserNotFound) {
		t.Fatalf("expected ErrBillingUserNotFound, got %v", err)
	}
}

func TestStore_FindUserIDByStripeSubscriptionID_DBError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return fmt.Errorf("db error")
			}}
		},
	}
	store := NewStore(db)

	_, err := store.FindUserIDByStripeSubscriptionID(ctx, "sub", db)
	if err == nil || !strings.Contains(err.Error(), "find user by subscription id") {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestStore_GrantLifetime_Success(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	db := &fakeDB{}
	store := NewStore(db)

	err := store.GrantLifetime(ctx, uuid.New(), "cus", tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStore_SetSubscriptionState_Success(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	db := &fakeDB{}
	store := NewStore(db)

	err := store.SetSubscriptionState(ctx, uuid.New(), "cus", "sub", "active", time.Now(), false, tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStore_SetSubscriptionState_Error(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{}, fmt.Errorf("boom")
		},
	}
	db := &fakeDB{}
	store := NewStore(db)

	err := store.SetSubscriptionState(ctx, uuid.New(), "cus", "sub", "active", time.Now(), false, tx)
	if err == nil || !strings.Contains(err.Error(), "set subscription state") {
		t.Fatalf("expected set subscription state error, got %v", err)
	}
}

func TestStore_ResetToFree_Success(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	db := &fakeDB{}
	store := NewStore(db)

	err := store.ResetToFree(ctx, uuid.New(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStore_WithWebhookEvent_BeginError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return nil, fmt.Errorf("begin failed")
		},
	}
	store := NewStore(db)

	_, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_1"}, func(ctx context.Context, tx services.Tx) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "begin webhook tx") {
		t.Fatalf("expected begin error, got %v", err)
	}
}

func TestStore_WithWebhookEvent_LockError(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return fmt.Errorf("lock failed")
			}}
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	_, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_lock"}, func(ctx context.Context, tx services.Tx) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "lock webhook event row") {
		t.Fatalf("expected lock error, got %v", err)
	}
}

func TestStore_WithWebhookEvent_CommitNoOpError(t *testing.T) {
	ctx := context.Background()
	processedAt := time.Now().UTC()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues(&processedAt)
		},
		CommitFunc: func(ctx context.Context) error {
			return fmt.Errorf("commit failed")
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	_, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_commit"}, func(ctx context.Context, tx services.Tx) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "commit webhook no-op") {
		t.Fatalf("expected commit error, got %v", err)
	}
}

func TestStore_WithWebhookEvent_CommitErrorAfterHandlerError(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues((*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			return fmt.Errorf("commit failed")
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	_, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_err"}, func(ctx context.Context, tx services.Tx) error {
		return fmt.Errorf("handler error")
	})
	if err == nil || !strings.Contains(err.Error(), "commit webhook error") {
		t.Fatalf("expected commit error, got %v", err)
	}
}

func TestStore_WithWebhookEvent_MarkProcessedError(t *testing.T) {
	ctx := context.Background()
	var execCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues((*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execCalls++
			if execCalls == 2 {
				return fakeCommandTag{}, fmt.Errorf("mark processed failed")
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	_, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_mark"}, func(ctx context.Context, tx services.Tx) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "mark webhook processed") {
		t.Fatalf("expected mark processed error, got %v", err)
	}
}

func TestStore_WithWebhookEvent_FinalCommitError(t *testing.T) {
	ctx := context.Background()
	var execCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return rowFromValues((*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execCalls++
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			return fmt.Errorf("final commit failed")
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	_, err := store.WithWebhookEvent(ctx, WebhookEventMeta{StripeEventID: "evt_final"}, func(ctx context.Context, tx services.Tx) error {
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "commit webhook processed") {
		t.Fatalf("expected final commit error, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_BeginError(t *testing.T) {
	ctx := context.Background()
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return nil, fmt.Errorf("begin failed")
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", time.Now())
	if err == nil || !strings.Contains(err.Error(), "begin redeem tx") {
		t.Fatalf("expected begin error, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_QueryError(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			return fakeRow{scanFunc: func(dest ...any) error {
				return fmt.Errorf("query failed")
			}}
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", time.Now())
	if err == nil || !strings.Contains(err.Error(), "load premium code") {
		t.Fatalf("expected load premium code error, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_UserUpdateError(t *testing.T) {
	ctx := context.Background()
	var execCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			codeID := uuid.New()
			return rowFromValues(codeID, (*int)(nil), (*time.Time)(nil), (*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execCalls++
			if execCalls == 2 {
				return fakeCommandTag{}, fmt.Errorf("user update failed")
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", time.Now())
	if err == nil || !strings.Contains(err.Error(), "apply premium code entitlement") {
		t.Fatalf("expected apply entitlement error, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_CommitError(t *testing.T) {
	ctx := context.Background()
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			codeID := uuid.New()
			return rowFromValues(codeID, (*int)(nil), (*time.Time)(nil), (*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			return fmt.Errorf("commit failed")
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", time.Now())
	if err == nil || !strings.Contains(err.Error(), "commit redeem tx") {
		t.Fatalf("expected commit error, got %v", err)
	}
}

func TestStore_RedeemPremiumCode_LifetimeNoDuration(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	var gotPeriodEnd any
	var execCalls int
	tx := &fakeTx{
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) services.Row {
			codeID := uuid.New()
			// duration_days is nil (lifetime code)
			return rowFromValues(codeID, (*int)(nil), (*time.Time)(nil), (*time.Time)(nil))
		},
		ExecFunc: func(ctx context.Context, sql string, args ...any) (services.CommandTag, error) {
			execCalls++
			// Capture the period_end arg from the user update query (second exec)
			if execCalls == 2 && len(args) >= 2 {
				gotPeriodEnd = args[1]
			}
			return fakeCommandTag{rowsAffected: 1}, nil
		},
		CommitFunc: func(ctx context.Context) error {
			return nil
		},
	}
	db := &fakeDB{
		BeginFunc: func(ctx context.Context) (services.Tx, error) {
			return tx, nil
		},
	}
	store := NewStore(db)

	err := store.RedeemPremiumCode(ctx, uuid.New(), "hash", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// For lifetime codes, periodEnd should be nil
	// Check that gotPeriodEnd is (*time.Time)(nil), not a non-nil *time.Time
	if pe, ok := gotPeriodEnd.(*time.Time); !ok || pe != nil {
		t.Fatalf("expected nil *time.Time period end for lifetime, got %T = %v", gotPeriodEnd, gotPeriodEnd)
	}
}
