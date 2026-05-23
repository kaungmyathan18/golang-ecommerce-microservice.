package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const ExchangeName = "ecommerce.events"

type Client struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func Connect(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}
	if err := ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("exchange declare: %w", err)
	}
	return &Client{conn: conn, ch: ch}, nil
}

func (c *Client) Channel() *amqp.Channel { return c.ch }

func (c *Client) Close() error {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) DeclareQueue(name, routingKey string) (amqp.Queue, error) {
	q, err := c.ch.QueueDeclare(name, true, false, false, false, nil)
	if err != nil {
		return amqp.Queue{}, err
	}
	if err := c.ch.QueueBind(q.Name, routingKey, ExchangeName, false, nil); err != nil {
		return amqp.Queue{}, err
	}
	return q, nil
}
