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
	delayKey  string
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
		delayKey:  queueName + ":delay",
	}, nil
}

// Publish adds a job ID to the queue
func (q *RedisQueue) Publish(ctx context.Context, jobID string, _ int) error {
	return q.client.LPush(ctx, q.queueName, jobID).Err()
}

// Pop removes and returns a job message from the queue (blocking)
func (q *RedisQueue) Pop(ctx context.Context, timeout time.Duration) (Message, error) {
	result, err := q.client.BRPop(ctx, timeout, q.queueName).Result()
	if err == redis.Nil {
		return nil, nil // Timeout, no items
	}
	if err != nil {
		return nil, err
	}

	// BRPop returns [key, value]
	if len(result) < 2 {
		return nil, fmt.Errorf("unexpected BRPOP result")
	}

	return &redisMessage{queue: q, jobID: result[1]}, nil
}

// Delay schedules a job for requeue at a future time.
func (q *RedisQueue) Delay(ctx context.Context, jobID string, retryAt time.Time) error {
	return q.client.ZAdd(ctx, q.delayKey, redis.Z{
		Score:  float64(retryAt.UnixMilli()),
		Member: jobID,
	}).Err()
}

// RequeueReady moves ready delayed jobs back into the main queue.
func (q *RedisQueue) RequeueReady(ctx context.Context, batch int) (int, error) {
	if batch <= 0 {
		batch = 100
	}

	now := time.Now().UnixMilli()
	jobIDs, err := q.client.ZRangeByScore(ctx, q.delayKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    fmt.Sprintf("%d", now),
		Offset: 0,
		Count:  int64(batch),
	}).Result()
	if err != nil {
		return 0, err
	}
	if len(jobIDs) == 0 {
		return 0, nil
	}

	pipe := q.client.Pipeline()
	for _, id := range jobIDs {
		pipe.LPush(ctx, q.queueName, id)
		pipe.ZRem(ctx, q.delayKey, id)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return len(jobIDs), nil
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

type redisMessage struct {
	queue *RedisQueue
	jobID string
}

func (m *redisMessage) JobID() string {
	return m.jobID
}

func (m *redisMessage) Ack() error {
	return nil
}

func (m *redisMessage) Nack(requeue bool) error {
	if !requeue {
		return nil
	}
	return nil
}
