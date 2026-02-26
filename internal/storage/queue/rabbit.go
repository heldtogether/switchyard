package queue

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitQueue provides publish/consume with RabbitMQ.
type RabbitQueue struct {
	conn          *amqp.Connection
	ch            *amqp.Channel
	exchange      string
	delayExchange string
	queueName     string
	deliveries    <-chan amqp.Delivery
}

// NewRabbitPublisher creates a publisher for RabbitMQ.
func NewRabbitPublisher(url, exchange string) (*RabbitQueue, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &RabbitQueue{
		conn:     conn,
		ch:       ch,
		exchange: exchange,
	}, nil
}

// NewRabbitProducer is an alias for NewRabbitPublisher.
func NewRabbitProducer(url, exchange string) (*RabbitQueue, error) {
	return NewRabbitPublisher(url, exchange)
}

// NewRabbitConsumer creates a consumer queue bound to gpu.* routing keys.
func NewRabbitConsumer(url, exchange, delayExchange, queueName string, gpuCount int, prefetch int, taskTimeout time.Duration, maxPriority int) (*RabbitQueue, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}
	if err := ch.ExchangeDeclare(delayExchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare delay exchange: %w", err)
	}

	queueArgs := amqp.Table{
		"x-dead-letter-exchange": delayExchange,
	}
	if maxPriority > 0 {
		queueArgs["x-max-priority"] = int32(maxPriority)
	}

	q, err := ch.QueueDeclare(queueName, true, false, false, false, queueArgs)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	delayQueueName := fmt.Sprintf("%s.delay", queueName)
	delayArgs := amqp.Table{
		"x-message-ttl":          int32(taskTimeout.Milliseconds()),
		"x-dead-letter-exchange": exchange,
	}
	delayQueue, err := ch.QueueDeclare(delayQueueName, true, false, false, false, delayArgs)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare delay queue: %w", err)
	}

	if err := ch.QueueBind(delayQueue.Name, "gpu.*", delayExchange, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to bind delay queue: %w", err)
	}

	for i := 0; i <= gpuCount; i++ {
		routingKey := fmt.Sprintf("gpu.%d", i)
		if err := ch.QueueBind(q.Name, routingKey, exchange, false, nil); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("failed to bind queue: %w", err)
		}
	}

	if prefetch > 0 {
		if err := ch.Qos(prefetch, 0, false); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("failed to set QoS: %w", err)
		}
	}

	deliveries, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to start consuming: %w", err)
	}

	return &RabbitQueue{
		conn:          conn,
		ch:            ch,
		exchange:      exchange,
		delayExchange: delayExchange,
		queueName:     q.Name,
		deliveries:    deliveries,
	}, nil
}

// Publish sends a job to the exchange with gpu routing key.
func (q *RabbitQueue) Publish(ctx context.Context, jobID string, gpuCount int) error {
	routingKey := fmt.Sprintf("gpu.%d", gpuCount)
	return q.ch.PublishWithContext(ctx, q.exchange, routingKey, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "text/plain",
		Body:         []byte(jobID),
	})
}

// Pop returns the next delivery as a Message.
func (q *RabbitQueue) Pop(ctx context.Context, _ time.Duration) (Message, error) {
	if q.deliveries == nil {
		return nil, fmt.Errorf("rabbit queue is not a consumer")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case d, ok := <-q.deliveries:
		if !ok {
			return nil, fmt.Errorf("rabbit deliveries channel closed")
		}
		return &rabbitMessage{delivery: d}, nil
	}
}

// Delay is not used for RabbitMQ (handled by DLX).
func (q *RabbitQueue) Delay(ctx context.Context, jobID string, retryAt time.Time) error {
	_ = ctx
	_ = jobID
	_ = retryAt
	return nil
}

// RequeueReady is a no-op for RabbitMQ (handled by DLX).
func (q *RabbitQueue) RequeueReady(ctx context.Context, batch int) (int, error) {
	_ = ctx
	_ = batch
	return 0, nil
}

// Close closes the channel and connection.
func (q *RabbitQueue) Close() error {
	if q.ch != nil {
		_ = q.ch.Close()
	}
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}

type rabbitMessage struct {
	delivery amqp.Delivery
}

func (m *rabbitMessage) JobID() string {
	return string(m.delivery.Body)
}

func (m *rabbitMessage) Ack() error {
	return m.delivery.Ack(false)
}

func (m *rabbitMessage) Nack(requeue bool) error {
	return m.delivery.Nack(false, requeue)
}
