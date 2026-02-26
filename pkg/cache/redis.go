package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds the configuration for connecting to a Redis instance.
// Zero values for pool fields will use sensible defaults.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int

	PoolSize     int // default: 10
	MinIdleConns int // default: 5
	MaxRetries   int // default: 3
}

func (c *RedisConfig) applyDefaults() {
	if c.PoolSize <= 0 {
		c.PoolSize = 10
	}
	if c.MinIdleConns <= 0 {
		c.MinIdleConns = 5
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
}

func NewRedisClient(cfg RedisConfig) (*redis.Client, error) {
	cfg.applyDefaults()

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("unable to connect redis: %w", err)
	}

	return client, nil
}
