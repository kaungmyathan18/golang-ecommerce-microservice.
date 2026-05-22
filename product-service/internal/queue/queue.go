package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/config"

	"github.com/redis/go-redis/v9"
)

// Client is a minimal queue facade over Redis lists.
type Client struct {
	rdb    *redis.Client
	prefix string
}

func New(cfg config.QueueConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &Client{rdb: rdb, prefix: cfg.Prefix}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) key(name string) string {
	return c.prefix + ":queue:" + name
}

func (c *Client) Enqueue(ctx context.Context, queueName, payload string) error {
	return c.rdb.RPush(ctx, c.key(queueName), payload).Err()
}

func (c *Client) Dequeue(ctx context.Context, queueName string, timeout time.Duration) (string, error) {
	res, err := c.rdb.BLPop(ctx, timeout, c.key(queueName)).Result()
	if err != nil {
		return "", err
	}
	if len(res) < 2 {
		return "", nil
	}
	return res[1], nil
}
