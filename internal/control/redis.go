package control

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisControl struct {
	client    *redis.Client
	queueName string
}

func NewRedisPublisher(url, queueName string) (*RedisControl, error) {
	return newRedisControl(url, queueName)
}

func NewRedisSubscriber(url, queueName string) (*RedisControl, error) {
	return newRedisControl(url, queueName)
}

func newRedisControl(url, queueName string) (*RedisControl, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return &RedisControl{client: client, queueName: queueName}, nil
}

func (r *RedisControl) PublishJobCancel(ctx context.Context, nodeID string, signal CancelSignal) error {
	payload, err := signal.Marshal()
	if err != nil {
		return err
	}
	channel := fmt.Sprintf("%s:control:%s", r.queueName, nodeID)
	return r.client.Publish(ctx, channel, payload).Err()
}

func (r *RedisControl) ConsumeJobCancels(ctx context.Context, nodeID string, handler func(context.Context, CancelSignal) error) error {
	channel := fmt.Sprintf("%s:control:%s", r.queueName, nodeID)
	pubsub := r.client.Subscribe(ctx, channel)
	defer pubsub.Close()

	msgCh := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgCh:
			if !ok {
				return fmt.Errorf("redis control subscription closed")
			}
			signal, err := UnmarshalCancelSignal([]byte(msg.Payload))
			if err != nil {
				continue
			}
			if err := handler(ctx, signal); err != nil {
				continue
			}
		}
	}
}

func (r *RedisControl) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
