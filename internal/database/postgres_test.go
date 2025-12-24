package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewPostgresDB_ParseError(t *testing.T) {
	origParse := parsePGConfig
	t.Cleanup(func() { parsePGConfig = origParse })
	parsePGConfig = func(dsn string) (*pgxpool.Config, error) {
		return nil, errors.New("bad dsn")
	}

	_, err := NewPostgresDB("bad")
	if err == nil || err.Error() == "" {
		t.Fatal("expected parse error")
	}
}

func TestNewPostgresDB_PingError(t *testing.T) {
	origParse := parsePGConfig
	origNew := newPGPool
	origPing := pingPGPool
	origClose := closePGPool
	t.Cleanup(func() {
		parsePGConfig = origParse
		newPGPool = origNew
		pingPGPool = origPing
		closePGPool = origClose
	})

	cfg := &pgxpool.Config{}
	parsePGConfig = func(dsn string) (*pgxpool.Config, error) {
		return cfg, nil
	}
	pool := &pgxpool.Pool{}
	newPGPool = func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
		return pool, nil
	}
	pingPGPool = func(ctx context.Context, pool *pgxpool.Pool) error {
		return errors.New("ping failed")
	}
	closePGPool = func(pool *pgxpool.Pool) {}

	_, err := NewPostgresDB("dsn")
	if err == nil || err.Error() == "" {
		t.Fatal("expected ping error")
	}
}

func TestNewPostgresDB_SuccessConfigValues(t *testing.T) {
	origParse := parsePGConfig
	origNew := newPGPool
	origPing := pingPGPool
	origClose := closePGPool
	t.Cleanup(func() {
		parsePGConfig = origParse
		newPGPool = origNew
		pingPGPool = origPing
		closePGPool = origClose
	})

	cfg := &pgxpool.Config{}
	parsePGConfig = func(dsn string) (*pgxpool.Config, error) {
		return cfg, nil
	}
	pool := &pgxpool.Pool{}
	newPGPool = func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
		return pool, nil
	}
	pingPGPool = func(ctx context.Context, pool *pgxpool.Pool) error {
		return nil
	}
	closePGPool = func(pool *pgxpool.Pool) {}

	db, err := NewPostgresDB("dsn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db.Pool != pool {
		t.Fatal("expected returned pool to match stubbed pool")
	}
	if cfg.MaxConns != 25 {
		t.Fatalf("expected MaxConns 25, got %d", cfg.MaxConns)
	}
	if cfg.MinConns != 5 {
		t.Fatalf("expected MinConns 5, got %d", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Fatalf("expected MaxConnLifetime 1h, got %v", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 30*time.Minute {
		t.Fatalf("expected MaxConnIdleTime 30m, got %v", cfg.MaxConnIdleTime)
	}
	if cfg.HealthCheckPeriod != time.Minute {
		t.Fatalf("expected HealthCheckPeriod 1m, got %v", cfg.HealthCheckPeriod)
	}
}
