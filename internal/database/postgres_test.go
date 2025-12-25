package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewPostgresDB_ParseError(t *testing.T) {
	origParse := parsePGConfig
	t.Cleanup(func() { parsePGConfig = origParse })
	parseErr := errors.New("bad dsn")
	parsePGConfig = func(dsn string) (*pgxpool.Config, error) {
		return nil, parseErr
	}

	_, err := NewPostgresDB("bad")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !errors.Is(err, parseErr) {
		t.Fatalf("expected parse error to wrap %v, got %v", parseErr, err)
	}
	if !strings.Contains(err.Error(), "parsing database config") {
		t.Fatalf("expected parse error message context, got %q", err.Error())
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
	pingErr := errors.New("ping failed")
	pingPGPool = func(ctx context.Context, pool *pgxpool.Pool) error {
		return pingErr
	}
	closePGPool = func(pool *pgxpool.Pool) {}

	_, err := NewPostgresDB("dsn")
	if err == nil {
		t.Fatal("expected ping error")
	}
	if !errors.Is(err, pingErr) {
		t.Fatalf("expected ping error to wrap %v, got %v", pingErr, err)
	}
	if !strings.Contains(err.Error(), "pinging database") {
		t.Fatalf("expected ping error message context, got %q", err.Error())
	}
}

func TestNewPostgresDB_NewPoolError(t *testing.T) {
	origParse := parsePGConfig
	origNew := newPGPool
	t.Cleanup(func() {
		parsePGConfig = origParse
		newPGPool = origNew
	})

	parsePGConfig = func(dsn string) (*pgxpool.Config, error) {
		return &pgxpool.Config{}, nil
	}
	newErr := errors.New("new pool error")
	newPGPool = func(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
		return nil, newErr
	}

	_, err := NewPostgresDB("dsn")
	if err == nil {
		t.Fatal("expected new pool error")
	}
	if !errors.Is(err, newErr) {
		t.Fatalf("expected pool error to wrap %v, got %v", newErr, err)
	}
	if !strings.Contains(err.Error(), "creating connection pool") {
		t.Fatalf("expected pool error message context, got %q", err.Error())
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

func TestPostgresDB_Close_CallsPoolClose(t *testing.T) {
	origClose := closePGPool
	t.Cleanup(func() { closePGPool = origClose })

	called := false
	closePGPool = func(pool *pgxpool.Pool) {
		called = true
	}

	db := &PostgresDB{Pool: &pgxpool.Pool{}}
	db.Close()

	if !called {
		t.Fatal("expected closePGPool to be called")
	}
}
