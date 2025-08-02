package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mutual-friend/internal/search"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/elasticsearch"
	"mutual-friend/pkg/events"
	"mutual-friend/pkg/rabbitmq"
)

func main() {
	log.Println("Starting Event Consumer Test...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Elasticsearch client
	esClient, err := elasticsearch.NewClient(cfg.Elasticsearch)
	if err != nil {
		log.Fatalf("Failed to create Elasticsearch client: %v", err)
	}

	// Initialize RabbitMQ client
	rabbitmqClient, err := rabbitmq.NewClient(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ client: %v", err)
	}
	defer rabbitmqClient.Close()

	// Create user indexer
	userIndexer := search.NewUserIndexer(esClient)

	// Create event consumer
	eventConsumer := search.NewEventConsumer(rabbitmqClient, userIndexer)

	// Start the consumer
	if err := eventConsumer.Start(ctx); err != nil {
		log.Fatalf("Failed to start event consumer: %v", err)
	}

	// Test event publishing
	go func() {
		if err := runTests(ctx, cfg); err != nil {
			log.Printf("Test failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Event consumer is running. Press Ctrl+C to stop...")
	<-sigChan

	log.Println("Shutting down event consumer...")
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	log.Println("Event consumer stopped")
}

func runTests(ctx context.Context, cfg *config.Config) error {
	log.Println("Starting event tests...")

	// Create a test publisher
	rabbitmqClient, err := rabbitmq.NewClient(cfg.RabbitMQ)
	if err != nil {
		return fmt.Errorf("failed to create RabbitMQ client for testing: %w", err)
	}
	defer rabbitmqClient.Close()

	publisher := events.NewPublisher(rabbitmqClient, &events.PublisherConfig{
		ExchangeName:  "friend-events",
		RetryAttempts: 3,
		RetryDelay:    time.Second * 2,
	})

	// Setup exchange and queues
	if err := publisher.SetupExchangeAndQueues(ctx); err != nil {
		return fmt.Errorf("failed to setup exchange and queues: %w", err)
	}

	// Wait a moment for consumer to be ready
	time.Sleep(2 * time.Second)

	// Test 1: Publish user created event
	log.Println("Test 1: Publishing user created event...")
	userEvent := events.NewUserCreatedEvent(
		"test-user-123",
		"testuser",
		"test@example.com",
		"Test User",
		"test-consumer",
	)

	if err := publisher.PublishUserEvent(ctx, userEvent); err != nil {
		return fmt.Errorf("failed to publish user created event: %w", err)
	}
	log.Println("✓ User created event published successfully")

	// Test 2: Publish friend added event
	log.Println("Test 2: Publishing friend added event...")
	friendEvent := events.NewFriendAddedEvent(
		"test-user-123",
		"test-friend-456",
		"test-consumer",
	)

	if err := publisher.PublishFriendEvent(ctx, friendEvent); err != nil {
		return fmt.Errorf("failed to publish friend added event: %w", err)
	}
	log.Println("✓ Friend added event published successfully")

	// Test 3: Publish user updated event
	log.Println("Test 3: Publishing user updated event...")
	userUpdateEvent := events.NewUserCreatedEvent(
		"test-user-123",
		"testuser_updated",
		"test_updated@example.com",
		"Test User Updated",
		"test-consumer",
	)
	userUpdateEvent.Type = events.EventTypeUserUpdated

	if err := publisher.PublishUserEvent(ctx, userUpdateEvent); err != nil {
		return fmt.Errorf("failed to publish user updated event: %w", err)
	}
	log.Println("✓ User updated event published successfully")

	// Test 4: Publish friend removed event
	log.Println("Test 4: Publishing friend removed event...")
	friendRemovedEvent := events.NewFriendRemovedEvent(
		"test-user-123",
		"test-friend-456",
		"test-consumer",
	)

	if err := publisher.PublishFriendEvent(ctx, friendRemovedEvent); err != nil {
		return fmt.Errorf("failed to publish friend removed event: %w", err)
	}
	log.Println("✓ Friend removed event published successfully")

	// Test 5: Publish invalid event (to test error handling)
	log.Println("Test 5: Publishing invalid event to test error handling...")
	invalidMessage := []byte(`{"invalid": "json", "missing_fields": true}`)
	
	if err := rabbitmqClient.Publish(ctx, "friend-events", "friend.added", invalidMessage, nil); err != nil {
		return fmt.Errorf("failed to publish invalid event: %w", err)
	}
	log.Println("✓ Invalid event published successfully (should trigger error handling)")

	// Wait for events to be processed
	log.Println("Waiting 10 seconds for events to be processed...")
	time.Sleep(10 * time.Second)

	log.Println("All tests completed successfully!")
	return nil
}