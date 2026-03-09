package control

import "fmt"

func NewPublisher(queueType, queueURL, queueExchange, queueName string) (Publisher, error) {
	switch queueType {
	case "rabbitmq":
		return NewRabbitPublisher(queueURL, queueExchange, queueName)
	case "redis":
		return NewRedisPublisher(queueURL, queueName)
	default:
		return nil, fmt.Errorf("unsupported queue type: %s", queueType)
	}
}

func NewSubscriber(queueType, queueURL, queueExchange, queueName string) (Subscriber, error) {
	switch queueType {
	case "rabbitmq":
		return NewRabbitSubscriber(queueURL, queueExchange, queueName)
	case "redis":
		return NewRedisSubscriber(queueURL, queueName)
	default:
		return nil, fmt.Errorf("unsupported queue type: %s", queueType)
	}
}
