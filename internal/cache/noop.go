package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type noopCache struct{}

func NewNoopCache() Cache {
	return &noopCache{}
}

func (c *noopCache) Get(ctx context.Context, key string) (string, error) {
	return "", redis.Nil
}

func (c *noopCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

func (c *noopCache) Del(ctx context.Context, key string) error {
	return nil
}

func (c *noopCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	return 0, nil
}

func (c *noopCache) SetWithJitter(context.Context, string, interface{}, time.Duration) error {
	return nil
}
