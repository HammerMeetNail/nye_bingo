package services

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisAdapter_MethodsReturnErrorsWhenUnavailable(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 50 * time.Millisecond,
		ReadTimeout: 50 * time.Millisecond,
	})
	defer func() { _ = client.Close() }()

	adapter := NewRedisAdapter(client)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := adapter.Set(ctx, "k", "v", 10*time.Second); err == nil {
		t.Fatal("expected Set to return error when redis unavailable")
	}
	if _, err := adapter.Get(ctx, "k"); err == nil {
		t.Fatal("expected Get to return error when redis unavailable")
	}
	if err := adapter.Expire(ctx, "k", time.Second); err == nil {
		t.Fatal("expected Expire to return error when redis unavailable")
	}
	if err := adapter.Del(ctx, "k"); err == nil {
		t.Fatal("expected Del to return error when redis unavailable")
	}
}
