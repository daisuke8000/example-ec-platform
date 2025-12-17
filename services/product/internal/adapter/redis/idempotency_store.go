package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrKeyNotFound = errors.New("key not found")

type IdempotencyStore struct {
	client *redis.Client
	prefix string
}

func NewIdempotencyStore(client *redis.Client, prefix string) *IdempotencyStore {
	if prefix == "" {
		prefix = "product:idempotency:"
	}
	return &IdempotencyStore{
		client: client,
		prefix: prefix,
	}
}

func (s *IdempotencyStore) Get(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, s.prefix+key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrKeyNotFound
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (s *IdempotencyStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, s.prefix+key, value, ttl).Result()
}

func (s *IdempotencyStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.client.Set(ctx, s.prefix+key, value, ttl).Err()
}

func (s *IdempotencyStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.prefix+key).Err()
}
