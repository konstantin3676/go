package ledger

import (
	"context"
	"fmt"

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
