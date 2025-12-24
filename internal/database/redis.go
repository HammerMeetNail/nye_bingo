package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisDB struct {
	Client *redis.Client
}

var (
	newRedisClient = redis.NewClient
	redisPing      = func(ctx context.Context, client *redis.Client) error {
		return client.Ping(ctx).Err()
	}
)

func NewRedisDB(addr, password string, db int) (*RedisDB, error) {
	client := newRedisClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisPing(ctx, client); err != nil {
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	return &RedisDB{Client: client}, nil
}

func (r *RedisDB) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

func (r *RedisDB) Health(ctx context.Context) error {
	return redisPing(ctx, r.Client)
}
