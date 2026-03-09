package control

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitControl struct {
	conn      *amqp.Connection
	ch        *amqp.Channel
	exchange  string
	queueName string
}

func NewRabbitPublisher(url, baseExchange, baseQueueName string) (*RabbitControl, error) {
	return newRabbitControl(url, baseExchange, baseQueueName)
}

func NewRabbitSubscriber(url, baseExchange, baseQueueName string) (*RabbitControl, error) {
	return newRabbitControl(url, baseExchange, baseQueueName)
}

func newRabbitControl(url, baseExchange, baseQueueName string) (*RabbitControl, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbit channel: %w", err)
	}

	exchange := baseExchange + ".control"
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare control exchange: %w", err)
	}

	return &RabbitControl{conn: conn, ch: ch, exchange: exchange, queueName: baseQueueName}, nil
}

func (r *RabbitControl) PublishJobCancel(ctx context.Context, nodeID string, signal CancelSignal) error {
	payload, err := signal.Marshal()
	if err != nil {
		return err
	}
	routingKey := fmt.Sprintf("cancel.%s", nodeID)
	return r.ch.PublishWithContext(ctx, r.exchange, routingKey, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         payload,
	})
}

func (r *RabbitControl) ConsumeJobCancels(ctx context.Context, nodeID string, handler func(context.Context, CancelSignal) error) error {
	queueName := fmt.Sprintf("%s.control.%s", r.queueName, nodeID)
	q, err := r.ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare control queue: %w", err)
	}

	routingKey := fmt.Sprintf("cancel.%s", nodeID)
	if err := r.ch.QueueBind(q.Name, routingKey, r.exchange, false, nil); err != nil {
		return fmt.Errorf("bind control queue: %w", err)
	}

	deliveries, err := r.ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume control queue: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("control delivery channel closed")
			}
			signal, err := UnmarshalCancelSignal(d.Body)
			if err != nil {
				_ = d.Ack(false)
				continue
			}
			if err := handler(ctx, signal); err != nil {
				_ = d.Nack(false, true)
				continue
			}
			_ = d.Ack(false)
		}
	}
}

func (r *RabbitControl) Close() error {
	if r.ch != nil {
		_ = r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
