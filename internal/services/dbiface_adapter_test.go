package services

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCommandTagAdapter_RowsAffected(t *testing.T) {
	tag := pgconn.NewCommandTag("UPDATE 12")
	got := commandTagAdapter{tag: tag}.RowsAffected()
	if got != 12 {
		t.Fatalf("expected RowsAffected 12, got %d", got)
	}
}

type fakePgxRow struct {
	ScanFunc func(dest ...any) error
}

func (f fakePgxRow) Scan(dest ...any) error {
	if f.ScanFunc != nil {
		return f.ScanFunc(dest...)
	}
	return errors.New("ScanFunc not set")
}

type fakePgxRows struct {
	rows [][]any
	idx  int
	err  error
}

func (f *fakePgxRows) Close()                        {}
func (f *fakePgxRows) Err() error                    { return f.err }
func (f *fakePgxRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (f *fakePgxRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (f *fakePgxRows) Next() bool {
	if f.idx >= len(f.rows) {
		return false
	}
	f.idx++
	return true
}
func (f *fakePgxRows) Scan(dest ...any) error {
	if f.idx == 0 || f.idx > len(f.rows) {
		return errors.New("scan called without active row")
	}
	return assignRow(dest, f.rows[f.idx-1])
}
func (f *fakePgxRows) Values() ([]any, error) { return nil, errors.New("not implemented") }
func (f *fakePgxRows) RawValues() [][]byte    { return nil }
func (f *fakePgxRows) Conn() *pgx.Conn        { return nil }

type fakePgxTx struct {
	BeginFunc    func(ctx context.Context) (pgx.Tx, error)
	CommitFunc   func(ctx context.Context) error
	RollbackFunc func(ctx context.Context) error
	ExecFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (f *fakePgxTx) Begin(ctx context.Context) (pgx.Tx, error) {
	if f.BeginFunc != nil {
		return f.BeginFunc(ctx)
	}
	return f, nil
}
func (f *fakePgxTx) Commit(ctx context.Context) error {
	if f.CommitFunc != nil {
		return f.CommitFunc(ctx)
	}
	return nil
}
func (f *fakePgxTx) Rollback(ctx context.Context) error {
	if f.RollbackFunc != nil {
		return f.RollbackFunc(ctx)
	}
	return nil
}
func (f *fakePgxTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *fakePgxTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}
func (f *fakePgxTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (f *fakePgxTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}
func (f *fakePgxTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.ExecFunc != nil {
		return f.ExecFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("UPDATE 0"), nil
}
func (f *fakePgxTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if f.QueryFunc != nil {
		return f.QueryFunc(ctx, sql, args...)
	}
	return &fakePgxRows{}, nil
}
func (f *fakePgxTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.QueryRowFunc != nil {
		return f.QueryRowFunc(ctx, sql, args...)
	}
	return fakePgxRow{}
}
func (f *fakePgxTx) Conn() *pgx.Conn { return nil }

type fakePgxPool struct {
	ExecFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	BeginFunc    func(ctx context.Context) (pgx.Tx, error)
}

func (f *fakePgxPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return f.ExecFunc(ctx, sql, args...)
}
func (f *fakePgxPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return f.QueryFunc(ctx, sql, args...)
}
func (f *fakePgxPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f.QueryRowFunc(ctx, sql, args...)
}
func (f *fakePgxPool) Begin(ctx context.Context) (pgx.Tx, error) {
	return f.BeginFunc(ctx)
}

func TestPoolAdapter_WrapsPoolAndTxTypes(t *testing.T) {
	ctx := context.Background()
	pgxRows := &fakePgxRows{rows: [][]any{{"ok"}}}

	pool := &fakePgxPool{
		ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 12"), nil
		},
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return pgxRows, nil
		},
		QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return fakePgxRow{ScanFunc: func(dest ...any) error {
				return assignRow(dest, []any{"row"})
			}}
		},
		BeginFunc: func(ctx context.Context) (pgx.Tx, error) {
			return &fakePgxTx{
				ExecFunc: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("DELETE 2"), nil
				},
				QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return &fakePgxRows{rows: [][]any{{"tx"}}}, nil
				},
				QueryRowFunc: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return fakePgxRow{ScanFunc: func(dest ...any) error {
						return assignRow(dest, []any{"txrow"})
					}}
				},
				CommitFunc:   func(ctx context.Context) error { return nil },
				RollbackFunc: func(ctx context.Context) error { return nil },
			}, nil
		},
	}

	adapter := &PoolAdapter{pool: pool}

	tag, err := adapter.Exec(ctx, "UPDATE x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.RowsAffected() != 12 {
		t.Fatalf("expected rows affected 12, got %d", tag.RowsAffected())
	}

	rows, err := adapter.Query(ctx, "SELECT x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rows.Next() {
		t.Fatal("expected Next to be true")
	}
	var got string
	if err := rows.Scan(&got); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected ok, got %q", got)
	}
	rows.Close()
	_ = rows.Err()

	row := adapter.QueryRow(ctx, "SELECT y")
	var rowValue string
	if err := row.Scan(&rowValue); err != nil {
		t.Fatalf("scan row: %v", err)
	}
	if rowValue != "row" {
		t.Fatalf("expected row, got %q", rowValue)
	}

	tx, err := adapter.Begin(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	txTag, err := tx.Exec(ctx, "DELETE x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txTag.RowsAffected() != 2 {
		t.Fatalf("expected tx rows affected 2, got %d", txTag.RowsAffected())
	}

	txRows, err := tx.Query(ctx, "SELECT z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !txRows.Next() {
		t.Fatal("expected txRows.Next to be true")
	}
	var txValue string
	if err := txRows.Scan(&txValue); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if txValue != "tx" {
		t.Fatalf("expected tx, got %q", txValue)
	}
	txRows.Close()
	_ = txRows.Err()

	txRow := tx.QueryRow(ctx, "SELECT a")
	var txRowValue string
	if err := txRow.Scan(&txRowValue); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if txRowValue != "txrow" {
		t.Fatalf("expected txrow, got %q", txRowValue)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("unexpected commit error: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("unexpected rollback error: %v", err)
	}
}

func TestNewPoolAdapter_CanBeConstructed(t *testing.T) {
	adapter := NewPoolAdapter(nil)
	if adapter == nil {
		t.Fatal("expected adapter")
	}
}
