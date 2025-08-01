package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client represents a RabbitMQ client
type Client struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	url        string
	mu         sync.RWMutex
	closed     bool
	exchanges  map[string]bool
	queues     map[string]bool
}

// Config holds RabbitMQ configuration
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	Vhost    string
}

// NewClient creates a new RabbitMQ client
func NewClient(config *Config) (*Client, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Vhost,
	)

	client := &Client{
		url:       url,
		exchanges: make(map[string]bool),
		queues:    make(map[string]bool),
	}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	return client, nil
}

// connect establishes connection to RabbitMQ
func (c *Client) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	c.conn, err = amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Enable publisher confirms
	if err := c.channel.Confirm(false); err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("failed to enable publisher confirms: %w", err)
	}

	c.closed = false
	return nil
}

// DeclareExchange declares an exchange
func (c *Client) DeclareExchange(name, kind string, durable, autoDelete bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	if c.exchanges[name] {
		return nil // Already declared
	}

	err := c.channel.ExchangeDeclare(
		name,       // name
		kind,       // type
		durable,    // durable
		autoDelete, // auto-deleted
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", name, err)
	}

	c.exchanges[name] = true
	log.Printf("Exchange '%s' declared successfully", name)
	return nil
}

// DeclareQueue declares a queue
func (c *Client) DeclareQueue(name string, durable, autoDelete, exclusive bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	if c.queues[name] {
		return nil // Already declared
	}

	_, err := c.channel.QueueDeclare(
		name,       // name
		durable,    // durable
		autoDelete, // delete when unused
		exclusive,  // exclusive
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", name, err)
	}

	c.queues[name] = true
	log.Printf("Queue '%s' declared successfully", name)
	return nil
}

// BindQueue binds a queue to an exchange
func (c *Client) BindQueue(queueName, exchangeName, routingKey string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	err := c.channel.QueueBind(
		queueName,    // queue name
		routingKey,   // routing key
		exchangeName, // exchange
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", queueName, exchangeName, err)
	}

	log.Printf("Queue '%s' bound to exchange '%s' with routing key '%s'", queueName, exchangeName, routingKey)
	return nil
}

// Publish publishes a message to an exchange
func (c *Client) Publish(ctx context.Context, exchange, routingKey string, body []byte, headers map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		Headers:      headers,
		Timestamp:    time.Now(),
		DeliveryMode: amqp.Persistent, // Make message persistent
	}

	// Set message ID if not provided
	if publishing.MessageId == "" {
		publishing.MessageId = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	err := c.channel.PublishWithContext(
		ctx,
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		publishing,
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for publisher confirmation
	select {
	case confirm := <-c.channel.NotifyPublish(make(chan amqp.Confirmation, 1)):
		if !confirm.Ack {
			return fmt.Errorf("message publishing was not acknowledged")
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for publish confirmation")
	}

	return nil
}

// Consume starts consuming messages from a queue
func (c *Client) Consume(queueName, consumerTag string) (<-chan amqp.Delivery, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	msgs, err := c.channel.Consume(
		queueName,   // queue
		consumerTag, // consumer
		false,       // auto-ack (we'll ack manually)
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register consumer: %w", err)
	}

	return msgs, nil
}

// Close closes the RabbitMQ connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			log.Printf("Error closing channel: %v", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Printf("Error closing connection: %v", err)
		}
	}

	return nil
}

// IsConnected checks if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return !c.closed && c.conn != nil && !c.conn.IsClosed()
}

// Reconnect attempts to reconnect to RabbitMQ
func (c *Client) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed && c.conn != nil && !c.conn.IsClosed() {
		return nil // Already connected
	}

	// Close existing connections
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}

	// Reset state
	c.exchanges = make(map[string]bool)
	c.queues = make(map[string]bool)

	// Reconnect
	return c.connect()
}