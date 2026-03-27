package cache

import (
	"context"
	"encoding/json"
	"time"
)

func TryGetString(ctx context.Context, c Cache, key string) (string, bool) {
	if c == nil {
		return "", false
	}
	v, err := c.Get(ctx, key)
	if err != nil {
		return "", false
	}
	return v, true
}

func SetString(ctx context.Context, c Cache, key string, value string, ttl time.Duration) {
	if c == nil {
		return
	}
	_ = c.Set(ctx, key, value, ttl)
}

func SetJSONWithJitter(ctx context.Context, c Cache, key string, value interface{}, ttl time.Duration) {
	if c == nil {
		return
	}
	_ = c.SetWithJitter(ctx, key, value, ttl)
}

func DelKey(ctx context.Context, c Cache, key string) {
	if c == nil {
		return
	}
	_ = c.Del(ctx, key)
}

func MarshalJSON(v interface{}) (string, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	return string(b), true
}
