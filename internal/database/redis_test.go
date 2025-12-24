package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewRedisDB_PingError(t *testing.T) {
	origNew := newRedisClient
	origPing := redisPing
	t.Cleanup(func() {
		newRedisClient = origNew
		redisPing = origPing
	})

	newRedisClient = func(opts *redis.Options) *redis.Client {
		return &redis.Client{}
	}
	redisPing = func(ctx context.Context, client *redis.Client) error {
		return errors.New("ping failed")
	}

	_, err := NewRedisDB("localhost:6379", "pass", 2)
	if err == nil || err.Error() == "" {
		t.Fatal("expected ping error")
	}
}

func TestNewRedisDB_SetsOptions(t *testing.T) {
	origNew := newRedisClient
	origPing := redisPing
	t.Cleanup(func() {
		newRedisClient = origNew
		redisPing = origPing
	})

	var got redis.Options
	newRedisClient = func(opts *redis.Options) *redis.Client {
		got = *opts
		return &redis.Client{}
	}
	redisPing = func(ctx context.Context, client *redis.Client) error {
		return nil
	}

	db, err := NewRedisDB("localhost:6379", "pass", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db.Client == nil {
		t.Fatal("expected client")
	}
	if got.Addr != "localhost:6379" {
		t.Fatalf("expected addr, got %s", got.Addr)
	}
	if got.Password != "pass" {
		t.Fatalf("expected password, got %s", got.Password)
	}
	if got.DB != 2 {
		t.Fatalf("expected db 2, got %d", got.DB)
	}
	if got.DialTimeout != 5*time.Second {
		t.Fatalf("expected DialTimeout 5s, got %v", got.DialTimeout)
	}
	if got.ReadTimeout != 3*time.Second {
		t.Fatalf("expected ReadTimeout 3s, got %v", got.ReadTimeout)
	}
	if got.WriteTimeout != 3*time.Second {
		t.Fatalf("expected WriteTimeout 3s, got %v", got.WriteTimeout)
	}
	if got.PoolSize != 10 {
		t.Fatalf("expected PoolSize 10, got %d", got.PoolSize)
	}
	if got.MinIdleConns != 3 {
		t.Fatalf("expected MinIdleConns 3, got %d", got.MinIdleConns)
	}
}

func TestRedisDB_HealthError(t *testing.T) {
	origPing := redisPing
	t.Cleanup(func() { redisPing = origPing })
	redisPing = func(ctx context.Context, client *redis.Client) error {
		return errors.New("health failed")
	}

	db := &RedisDB{Client: &redis.Client{}}
	if err := db.Health(context.Background()); err == nil {
		t.Fatal("expected health error")
	}
}

func TestRedisDB_CloseNil(t *testing.T) {
	db := &RedisDB{}
	if err := db.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}
