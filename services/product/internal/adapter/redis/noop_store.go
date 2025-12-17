package redis

import (
	"context"
	"time"
)

// NoopIdempotencyStore provides no idempotency guarantees. Use only when Redis is unavailable.
type NoopIdempotencyStore struct{}

func NewNoopIdempotencyStore() *NoopIdempotencyStore {
	return &NoopIdempotencyStore{}
}

func (s *NoopIdempotencyStore) Get(ctx context.Context, key string) (string, error) {
	return "", ErrKeyNotFound
}

func (s *NoopIdempotencyStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return true, nil
}

func (s *NoopIdempotencyStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

func (s *NoopIdempotencyStore) Del(ctx context.Context, key string) error {
	return nil
}
