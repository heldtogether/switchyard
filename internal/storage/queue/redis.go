package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisQueue implements a job queue using Redis
type RedisQueue struct {
	client    *redis.Client
	queueName string
}

// NewRedis creates a new Redis queue
func NewRedis(url, queueName string) (*RedisQueue, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisQueue{
		client:    client,
		queueName: queueName,
	}, nil
}

// Push adds a job ID to the queue
func (q *RedisQueue) Push(ctx context.Context, jobID string) error {
	return q.client.LPush(ctx, q.queueName, jobID).Err()
}

// Pop removes and returns a job ID from the queue (blocking)
func (q *RedisQueue) Pop(ctx context.Context, timeout time.Duration) (string, error) {
	result, err := q.client.BRPop(ctx, timeout, q.queueName).Result()
	if err == redis.Nil {
		return "", nil // Timeout, no items
	}
	if err != nil {
		return "", err
	}

	// BRPop returns [key, value]
	if len(result) < 2 {
		return "", fmt.Errorf("unexpected BRPOP result")
	}

	return result[1], nil
}

// Length returns the queue length
func (q *RedisQueue) Length(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, q.queueName).Result()
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

// Ping checks if Redis is responsive
func (q *RedisQueue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}
