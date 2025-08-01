package events

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mutual-friend/pkg/rabbitmq"
)

// Publisher handles event publishing to RabbitMQ
type Publisher struct {
	client         *rabbitmq.Client
	exchangeName   string
	retryAttempts  int
	retryDelay     time.Duration
	mu             sync.RWMutex
}

// PublisherConfig holds publisher configuration
type PublisherConfig struct {
	ExchangeName  string
	RetryAttempts int
	RetryDelay    time.Duration
}

// NewPublisher creates a new event publisher
func NewPublisher(client *rabbitmq.Client, config *PublisherConfig) *Publisher {
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second * 2
	}

	return &Publisher{
		client:        client,
		exchangeName:  config.ExchangeName,
		retryAttempts: config.RetryAttempts,
		retryDelay:    config.RetryDelay,
	}
}

// SetupExchangeAndQueues sets up the exchange and queues for events
func (p *Publisher) SetupExchangeAndQueues(ctx context.Context) error {
	// Declare main exchange
	if err := p.client.DeclareExchange(p.exchangeName, "topic", true, false); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queues and bind them to exchange
	queues := map[string][]string{
		"friend-events":         {"FRIEND_ADDED.*", "FRIEND_REMOVED.*"},
		"user-events":          {"USER_CREATED.*", "USER_UPDATED.*", "USER_DELETED.*"},
		"recommendation-events": {"RECOMMENDATION_GENERATED.*", "RECOMMENDATION_UPDATED.*"},
		"elasticsearch-sync":   {"USER_CREATED.*", "USER_UPDATED.*", "FRIEND_ADDED.*", "FRIEND_REMOVED.*"},
		"analytics":           {"#"}, // Catch all events for analytics
	}

	for queueName, routingKeys := range queues {
		// Declare queue
		if err := p.client.DeclareQueue(queueName, true, false, false); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
		}

		// Bind queue to exchange with routing keys
		for _, routingKey := range routingKeys {
			if err := p.client.BindQueue(queueName, p.exchangeName, routingKey); err != nil {
				return fmt.Errorf("failed to bind queue %s with routing key %s: %w", queueName, routingKey, err)
			}
		}
	}

	log.Printf("Exchange '%s' and queues setup completed", p.exchangeName)
	return nil
}

// PublishFriendEvent publishes a friend event
func (p *Publisher) PublishFriendEvent(ctx context.Context, event *FriendEvent) error {
	return p.publishEventWithRetry(ctx, event, event.GetRoutingKey())
}

// PublishUserEvent publishes a user event
func (p *Publisher) PublishUserEvent(ctx context.Context, event *UserEvent) error {
	return p.publishEventWithRetry(ctx, event, event.GetRoutingKey())
}

// PublishRecommendationEvent publishes a recommendation event
func (p *Publisher) PublishRecommendationEvent(ctx context.Context, event *RecommendationEvent) error {
	return p.publishEventWithRetry(ctx, event, event.GetRoutingKey())
}

// Event interface for type assertion
type Event interface {
	ToJSON() ([]byte, error)
	GetRoutingKey() string
}

// publishEventWithRetry publishes an event with retry logic
func (p *Publisher) publishEventWithRetry(ctx context.Context, event Event, routingKey string) error {
	var lastErr error

	for attempt := 0; attempt <= p.retryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.retryDelay * time.Duration(attempt)):
				// Continue with retry
			}
		}

		// Convert event to JSON
		body, err := event.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		// Create headers
		headers := map[string]interface{}{
			"attempt": attempt + 1,
			"source":  "mutual-friend-service",
		}

		// Attempt to publish
		err = p.client.Publish(ctx, p.exchangeName, routingKey, body, headers)
		if err == nil {
			log.Printf("Event published successfully: type=%s, routing_key=%s, attempt=%d", 
				routingKey, routingKey, attempt+1)
			return nil
		}

		lastErr = err
		log.Printf("Failed to publish event (attempt %d/%d): %v", 
			attempt+1, p.retryAttempts+1, err)
	}

	return fmt.Errorf("failed to publish event after %d attempts: %w", p.retryAttempts+1, lastErr)
}

// PublishBatch publishes multiple events in batch
func (p *Publisher) PublishBatch(ctx context.Context, events []Event) error {
	results := make(chan error, len(events))
	var wg sync.WaitGroup

	// Publish events concurrently
	for _, event := range events {
		wg.Add(1)
		go func(e Event) {
			defer wg.Done()
			
			var err error
			switch v := e.(type) {
			case *FriendEvent:
				err = p.PublishFriendEvent(ctx, v)
			case *UserEvent:
				err = p.PublishUserEvent(ctx, v)
			case *RecommendationEvent:
				err = p.PublishRecommendationEvent(ctx, v)
			default:
				err = fmt.Errorf("unsupported event type: %T", e)
			}
			
			results <- err
		}(event)
	}

	// Wait for all publications to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Check results
	var errors []error
	for err := range results {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch publish failed with %d errors: %v", len(errors), errors[0])
	}

	log.Printf("Batch published %d events successfully", len(events))
	return nil
}

// Close closes the publisher
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// IsHealthy checks if the publisher is healthy
func (p *Publisher) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.client != nil && p.client.IsConnected()
}