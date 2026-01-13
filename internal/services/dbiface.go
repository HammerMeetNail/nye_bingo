package services

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Row abstracts pgx.Row for testability.
type Row interface {
	Scan(dest ...any) error
}

// Rows abstracts pgx.Rows for testability.
type Rows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...any) error
}

// CommandTag abstracts the result of Exec calls.
type CommandTag interface {
	RowsAffected() int64
}

// DBConn provides the minimum query surface for services.
type DBConn interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// Tx mirrors the transaction methods used by services.
type Tx interface {
	DBConn
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DB adds transaction support on top of DBConn.
type DB interface {
	DBConn
	Begin(ctx context.Context) (Tx, error)
}

type pgxPoolLike interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// PoolAdapter wraps *pgxpool.Pool to satisfy DB.
type PoolAdapter struct {
	pool pgxPoolLike
}

// NewPoolAdapter builds a DB adapter around a pgx pool.
func NewPoolAdapter(pool *pgxpool.Pool) *PoolAdapter {
	return &PoolAdapter{pool: pool}
}

func (p *PoolAdapter) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	tag, err := p.pool.Exec(ctx, sql, args...)
	return commandTagAdapter{tag: tag}, err
}

func (p *PoolAdapter) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return rowsAdapter{rows: rows}, nil
}

func (p *PoolAdapter) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

func (p *PoolAdapter) Begin(ctx context.Context) (Tx, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &txAdapter{tx: tx}, nil
}

type txAdapter struct {
	tx pgx.Tx
}

func (t *txAdapter) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	tag, err := t.tx.Exec(ctx, sql, args...)
	return commandTagAdapter{tag: tag}, err
}

func (t *txAdapter) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return rowsAdapter{rows: rows}, nil
}

func (t *txAdapter) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t *txAdapter) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *txAdapter) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

type rowsAdapter struct {
	rows pgx.Rows
}

func (r rowsAdapter) Close() {
	r.rows.Close()
}

func (r rowsAdapter) Err() error {
	return r.rows.Err()
}

func (r rowsAdapter) Next() bool {
	return r.rows.Next()
}

func (r rowsAdapter) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

type commandTagAdapter struct {
	tag pgconn.CommandTag
}

func (c commandTagAdapter) RowsAffected() int64 {
	return c.tag.RowsAffected()
}
