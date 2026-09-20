package rabbit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

const QueueProcess = "video.process"
const QueueDLQ = "video.process.dlq"

type Client struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

type ProcessMessage struct {
	JobID         uuid.UUID `json:"job_id"`
	CorrelationID string    `json:"correlation_id"`
}

func Dial(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := ch.QueueDeclare(QueueDLQ, true, false, false, false, nil); err != nil {
		return nil, err
	}
	args := amqp.Table{"x-dead-letter-exchange": "", "x-dead-letter-routing-key": QueueDLQ}
	if _, err := ch.QueueDeclare(QueueProcess, true, false, false, false, args); err != nil {
		return nil, err
	}
	return &Client{conn: conn, ch: ch}, nil
}

func DialRetry(url string, attempts int, delay time.Duration) (*Client, error) {
	var last error
	for i := 0; i < attempts; i++ {
		c, err := Dial(url)
		if err == nil {
			return c, nil
		}
		last = err
		time.Sleep(delay)
	}
	return nil, last
}

func (c *Client) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *Client) PublishProcess(ctx context.Context, jobID uuid.UUID, correlationID string) error {
	body, err := json.Marshal(ProcessMessage{JobID: jobID, CorrelationID: correlationID})
	if err != nil {
		return err
	}
	return c.ch.PublishWithContext(ctx, "", QueueProcess, false, false, amqp.Publishing{
		ContentType:   "application/json",
		DeliveryMode:  amqp.Persistent,
		CorrelationId: correlationID,
		Body:          body,
	})
}

func (c *Client) Consume() (<-chan amqp.Delivery, error) {
	if err := c.ch.Qos(1, 0, false); err != nil {
		return nil, err
	}
	return c.ch.Consume(QueueProcess, "", false, false, false, false, nil)
}

func Parse(d amqp.Delivery) (ProcessMessage, error) {
	var m ProcessMessage
	if err := json.Unmarshal(d.Body, &m); err != nil {
		return m, fmt.Errorf("parse message: %w", err)
	}
	return m, nil
}
