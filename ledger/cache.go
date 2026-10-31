package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func InitRedis(ctx context.Context) (*redis.Client, error) {
	if redisClient != nil {
		return redisClient, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     envOrDefault("REDIS_ADDR", "localhost:6379"),
		Password: envOrDefault("REDIS_PASSWORD", ""),
		DB:       redisDBNumber(),
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}

	redisClient = client
	return redisClient, nil
}

func SetRedisClient(client *redis.Client) {
	redisClient = client
}

func CloseRedis() error {
	if redisClient == nil {
		return nil
	}
	err := redisClient.Close()
	redisClient = nil
	return err
}

func readReportCache(ctx context.Context, key string) ([]ReportSummary, bool, error) {
	if redisClient == nil {
		return nil, false, nil
	}

	encoded, err := redisClient.Get(ctx, key).Result()
	if err == nil {
		var cached []ReportSummary
		if err := json.Unmarshal([]byte(encoded), &cached); err != nil {
			return nil, false, fmt.Errorf("decode report cache: %w", err)
		}
		return cached, true, nil
	}
	if err != redis.Nil {
		return nil, false, fmt.Errorf("read report cache: %w", err)
	}
	return nil, false, nil
}

func writeReportCache(ctx context.Context, key string, report []ReportSummary) error {
	if redisClient == nil {
		return nil
	}
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("encode report cache: %w", err)
	}
	if err := redisClient.Set(ctx, key, data, 30*time.Second).Err(); err != nil {
		return fmt.Errorf("write report cache: %w", err)
	}
	return nil
}

func invalidateBudgetsCache(ctx context.Context) error {
	if redisClient == nil {
		return nil
	}
	if err := redisClient.Del(ctx, "budgets:all").Err(); err != nil {
		return fmt.Errorf("invalidate budgets cache: %w", err)
	}
	return nil
}
