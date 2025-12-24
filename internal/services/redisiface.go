package services

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient narrows redis operations used by services.
type RedisClient interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisAdapter wraps *redis.Client to satisfy RedisClient.
type RedisAdapter struct {
	client *redis.Client
}

// NewRedisAdapter builds a RedisClient adapter around a redis client.
func NewRedisAdapter(client *redis.Client) *RedisAdapter {
	return &RedisAdapter{client: client}
}

func (r *RedisAdapter) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisAdapter) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisAdapter) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

func (r *RedisAdapter) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}
