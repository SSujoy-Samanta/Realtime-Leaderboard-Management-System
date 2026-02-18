package database

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/SSujoy-Samanta/leaderboard-backend/internal/config"
)

var RedisClient *redis.Client
var Ctx = context.Background()

// ConnectRedis initializes Redis connection
func ConnectRedis(cfg *config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: 20,
	})

	// Test connection
	if err := client.Ping(Ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("✅ Redis connected successfully")

	RedisClient = client
	return client, nil
}

// CloseRedis closes Redis connection
func CloseRedis() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}

// Redis key constants
const (
	LeaderboardKey     = "leaderboard:global"
	UserCacheKey       = "user:cache:%d" // user:cache:123
	UsernamePrefixKey  = "prefix:%s"     // prefix:rahul
	RankCacheKey       = "rank:cache:%d" // rank:cache:123
	ScoreUpdateChannel = "score:updates"
)