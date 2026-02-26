package queue

import (
	"context"
	"time"
)

// Message represents a queue message with ack semantics.
type Message interface {
	JobID() string
	Ack() error
	Nack(requeue bool) error
}

// Consumer pops messages for workers.
type Consumer interface {
	Pop(ctx context.Context, timeout time.Duration) (Message, error)
	Delay(ctx context.Context, jobID string, retryAt time.Time) error
	RequeueReady(ctx context.Context, batch int) (int, error)
	Close() error
}

// Producer publishes jobs to the queue.
type Producer interface {
	Publish(ctx context.Context, jobID string, gpuCount int) error
	Close() error
}
