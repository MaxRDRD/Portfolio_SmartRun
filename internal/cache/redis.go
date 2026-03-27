package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker/v2"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	SetWithJitter(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error
}

type redisCache struct {
	client  *redis.Client
	breaker *gobreaker.CircuitBreaker[any]
}

func NewRedisCache(redisURL string) (Cache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	client := redis.NewClient(opt)

	// Проверка подключения
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	breaker := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        "redis-cache",
		Interval:    60 * time.Second,
		Timeout:     5 * time.Second,
		MaxRequests: 10,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		IsSuccessful: func(err error) bool {
			return err == nil || errors.Is(err, redis.Nil)
		},
	})

	return &redisCache{client: client, breaker: breaker}, nil
}

func (c *redisCache) Get(ctx context.Context, key string) (string, error) {
	result, err := c.breaker.Execute(func() (any, error) {
		val, getErr := c.client.Get(ctx, key).Result()
		if getErr != nil {
			return nil, getErr
		}
		return val, nil
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (c *redisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	_, err := c.breaker.Execute(func() (any, error) {
		return nil, c.client.Set(ctx, key, value, ttl).Err()
	})
	return err
}

func (c *redisCache) Del(ctx context.Context, key string) error {
	_, err := c.breaker.Execute(func() (any, error) {
		return nil, c.client.Del(ctx, key).Err()
	})
	return err
}

func (c *redisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	result, err := c.breaker.Execute(func() (any, error) {
		exists, existsErr := c.client.Exists(ctx, keys...).Result()
		if existsErr != nil {
			return nil, existsErr
		}
		return exists, nil
	})
	if err != nil {
		return 0, err
	}
	return result.(int64), nil
}

func (c *redisCache) SetWithJitter(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	if baseTTL <= 0 {
		return fmt.Errorf("baseTTL must be > 0")
	}

	// Jitter in range [-10%, +10%] to avoid synchronized expiry spikes.
	maxJitter := int64(baseTTL / 10)
	if maxJitter <= 0 {
		maxJitter = int64(time.Second)
	}
	delta := rand.Int64N(2*maxJitter+1) - maxJitter
	ttl := baseTTL + time.Duration(delta)
	if ttl < time.Second {
		ttl = time.Second
	}

	data, err := json.Marshal(value) // или используй свой формат
	if err != nil {
		return err
	}

	_, err = c.breaker.Execute(func() (any, error) {
		return nil, c.client.Set(ctx, key, data, ttl).Err()
	})
	return err
}
