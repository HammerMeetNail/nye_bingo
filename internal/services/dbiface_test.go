package services

import (
	"context"
	"fmt"
	"reflect"
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

type fakeRows struct {
	rows   [][]any
	idx    int
	err    error
	closed bool
}

func (f *fakeRows) Close() {
	f.closed = true
}

func (f *fakeRows) Err() error {
	return f.err
}

func (f *fakeRows) Next() bool {
	if f.idx >= len(f.rows) {
		return false
	}
	f.idx++
	return true
}

func (f *fakeRows) Scan(dest ...any) error {
	if f.idx == 0 || f.idx > len(f.rows) {
		return fmt.Errorf("scan called without active row")
	}
	return assignRow(dest, f.rows[f.idx-1])
}

type fakeDB struct {
	ExecFunc     func(ctx context.Context, sql string, args ...any) (CommandTag, error)
	QueryFunc    func(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) Row
	BeginFunc    func(ctx context.Context) (Tx, error)
}

func (f *fakeDB) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	if f.ExecFunc != nil {
		return f.ExecFunc(ctx, sql, args...)
	}
	return fakeCommandTag{}, nil
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	if f.QueryFunc != nil {
		return f.QueryFunc(ctx, sql, args...)
	}
	return &fakeRows{}, nil
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...any) Row {
	if f.QueryRowFunc != nil {
		return f.QueryRowFunc(ctx, sql, args...)
	}
	return fakeRow{scanFunc: func(dest ...any) error {
		return fmt.Errorf("queryRowFunc not set")
	}}
}

func (f *fakeDB) Begin(ctx context.Context) (Tx, error) {
	if f.BeginFunc != nil {
		return f.BeginFunc(ctx)
	}
	return nil, fmt.Errorf("beginFunc not set")
}

type fakeTx struct {
	ExecFunc     func(ctx context.Context, sql string, args ...any) (CommandTag, error)
	QueryFunc    func(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) Row
	CommitFunc   func(ctx context.Context) error
	RollbackFunc func(ctx context.Context) error
}

func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	if f.ExecFunc != nil {
		return f.ExecFunc(ctx, sql, args...)
	}
	return fakeCommandTag{}, nil
}

func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	if f.QueryFunc != nil {
		return f.QueryFunc(ctx, sql, args...)
	}
	return &fakeRows{}, nil
}

func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) Row {
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

func rowFromValues(values ...any) Row {
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
