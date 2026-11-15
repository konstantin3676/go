package cached

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func readJSON[T any](ctx context.Context, client *redis.Client, key, name string) (T, bool, error) {
	var value T
	if client == nil {
		return value, false, nil
	}
	encoded, err := client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return value, false, nil
	}
	if err != nil {
		return value, false, fmt.Errorf("read %s cache: %w", name, err)
	}
	if err := json.Unmarshal([]byte(encoded), &value); err != nil {
		return value, false, fmt.Errorf("decode %s cache: %w", name, err)
	}
	return value, true, nil
}

func writeJSON[T any](ctx context.Context, client *redis.Client, key, name string, value T, ttl time.Duration) error {
	if client == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s cache: %w", name, err)
	}
	if err := client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("write %s cache: %w", name, err)
	}
	return nil
}
