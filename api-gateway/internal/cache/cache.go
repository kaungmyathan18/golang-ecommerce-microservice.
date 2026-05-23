package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/config"

	"github.com/redis/go-redis/v9"
)

// Client wraps Redis for caching.
type Client struct {
	rdb *redis.Client
}

func New(cfg config.CacheConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) Redis() *redis.Client {
	return c.rdb
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}
