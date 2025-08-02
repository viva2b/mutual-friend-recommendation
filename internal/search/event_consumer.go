package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"mutual-friend/pkg/events"
	"mutual-friend/pkg/rabbitmq"
)

// EventConsumer consumes events and updates Elasticsearch indexes
type EventConsumer struct {
	rabbitmqClient *rabbitmq.Client
	userIndexer    *UserIndexer
	queueName      string
	consumerTag    string
}

// NewEventConsumer creates a new EventConsumer
func NewEventConsumer(rabbitmqClient *rabbitmq.Client, userIndexer *UserIndexer) *EventConsumer {
	return &EventConsumer{
		rabbitmqClient: rabbitmqClient,
		userIndexer:    userIndexer,
		queueName:      "search-index-queue",
		consumerTag:    "search-indexer",
	}
}

// Start starts consuming events
func (ec *EventConsumer) Start(ctx context.Context) error {
	log.Printf("Starting search index event consumer...")
	
	// Setup queue and bindings
	if err := ec.setupQueue(ctx); err != nil {
		return fmt.Errorf("failed to setup queue: %w", err)
	}
	
	// Start consuming messages
	deliveries, err := ec.rabbitmqClient.Consume(ec.queueName, ec.consumerTag, false)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}
	
	// Process messages in a goroutine
	go ec.processMessages(ctx, deliveries)
	
	log.Printf("Search index event consumer started on queue: %s", ec.queueName)
	return nil
}

// setupQueue creates the queue and bindings for search index updates
func (ec *EventConsumer) setupQueue(ctx context.Context) error {
	// Declare exchange if not exists
	err := ec.rabbitmqClient.DeclareExchange("friend-events", "topic", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}
	
	err = ec.rabbitmqClient.DeclareExchange("user-events", "topic", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare user-events exchange: %w", err)
	}
	
	// Declare queue
	_, err = ec.rabbitmqClient.DeclareQueue(ec.queueName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}
	
	// Bind queue to exchanges with routing keys
	bindings := []struct {
		exchange   string
		routingKey string
	}{
		{"friend-events", "friend.added"},
		{"friend-events", "friend.removed"},
		{"user-events", "user.created"},
		{"user-events", "user.updated"},
	}
	
	for _, binding := range bindings {
		err = ec.rabbitmqClient.BindQueue(ec.queueName, binding.routingKey, binding.exchange, false, nil)
		if err != nil {
			return fmt.Errorf("failed to bind queue to %s with key %s: %w", 
				binding.exchange, binding.routingKey, err)
		}
	}
	
	log.Printf("Successfully setup search index queue with bindings")
	return nil
}

// processMessages processes incoming messages
func (ec *EventConsumer) processMessages(ctx context.Context, deliveries <-chan rabbitmq.Delivery) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Search index event consumer stopped")
			return
		case delivery := <-deliveries:
			if err := ec.processMessage(ctx, delivery); err != nil {
				log.Printf("Failed to process message: %v", err)
				// Nack message to retry later
				delivery.Nack(false, true)
			} else {
				// Ack message
				delivery.Ack(false)
			}
		}
	}
}

// processMessage processes a single message
func (ec *EventConsumer) processMessage(ctx context.Context, delivery rabbitmq.Delivery) error {
	// Parse message based on routing key
	routingKey := delivery.RoutingKey
	
	switch routingKey {
	case "friend.added", "friend.removed":
		return ec.processFriendEvent(ctx, delivery)
	case "user.created", "user.updated":
		return ec.processUserEvent(ctx, delivery)
	default:
		log.Printf("Unknown routing key: %s", routingKey)
		return nil // Don't retry unknown messages
	}
}

// processFriendEvent processes friend-related events
func (ec *EventConsumer) processFriendEvent(ctx context.Context, delivery rabbitmq.Delivery) error {
	var event events.FriendEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal friend event: %w", err)
	}
	
	log.Printf("Processing friend event: %s for users %s <-> %s", 
		event.EventType, event.UserID, event.FriendID)
	
	// Update social metrics for both users involved
	switch event.EventType {
	case events.EventFriendAdded:
		// Increment friend count for both users
		if err := ec.updateFriendCount(ctx, event.UserID, 1); err != nil {
			log.Printf("Failed to update friend count for user %s: %v", event.UserID, err)
		}
		if err := ec.updateFriendCount(ctx, event.FriendID, 1); err != nil {
			log.Printf("Failed to update friend count for user %s: %v", event.FriendID, err)
		}
		
		// Update mutual friend counts (simplified - in real implementation, 
		// this would involve more complex calculation)
		if err := ec.updateMutualFriendCounts(ctx, event.UserID, event.FriendID); err != nil {
			log.Printf("Failed to update mutual friend counts: %v", err)
		}
		
	case events.EventFriendRemoved:
		// Decrement friend count for both users
		if err := ec.updateFriendCount(ctx, event.UserID, -1); err != nil {
			log.Printf("Failed to update friend count for user %s: %v", event.UserID, err)
		}
		if err := ec.updateFriendCount(ctx, event.FriendID, -1); err != nil {
			log.Printf("Failed to update friend count for user %s: %v", event.FriendID, err)
		}
		
		// Update mutual friend counts
		if err := ec.updateMutualFriendCounts(ctx, event.UserID, event.FriendID); err != nil {
			log.Printf("Failed to update mutual friend counts: %v", err)
		}
	}
	
	return nil
}

// processUserEvent processes user-related events
func (ec *EventConsumer) processUserEvent(ctx context.Context, delivery rabbitmq.Delivery) error {
	var event events.UserEvent
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		return fmt.Errorf("failed to unmarshal user event: %w", err)
	}
	
	log.Printf("Processing user event: %s for user %s", event.EventType, event.UserID)
	
	switch event.EventType {
	case events.EventUserCreated:
		// Index new user
		if event.UserData != nil {
			// In a real implementation, we would convert the event data to a User struct
			// For now, we'll just log it
			log.Printf("New user created: %s", event.UserID)
		}
		
	case events.EventUserUpdated:
		// Update existing user
		if event.UserData != nil {
			log.Printf("User updated: %s", event.UserID)
			// In a real implementation, we would update the user document
		}
	}
	
	return nil
}

// updateFriendCount updates the friend count for a user
func (ec *EventConsumer) updateFriendCount(ctx context.Context, userID string, delta int) error {
	// This is a simplified implementation
	// In a real system, we would:
	// 1. Get current friend count from database
	// 2. Update the count
	// 3. Update Elasticsearch with new metrics
	
	// For now, we'll create a simple update
	updateScript := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "if (ctx._source.social_metrics == null) { ctx._source.social_metrics = [:] } if (ctx._source.social_metrics.friend_count == null) { ctx._source.social_metrics.friend_count = 0 } ctx._source.social_metrics.friend_count += params.delta; ctx._source.updated_at = params.timestamp",
			"params": map[string]interface{}{
				"delta":     delta,
				"timestamp": time.Now(),
			},
		},
		"upsert": map[string]interface{}{
			"social_metrics": map[string]interface{}{
				"friend_count": max(0, delta), // Don't go below 0
			},
			"updated_at": time.Now(),
		},
	}
	
	err := ec.userIndexer.esClient.UpdateDocument(ctx, "users", userID, updateScript)
	if err != nil {
		return fmt.Errorf("failed to update friend count for user %s: %w", userID, err)
	}
	
	return nil
}

// updateMutualFriendCounts updates mutual friend counts for affected users
func (ec *EventConsumer) updateMutualFriendCounts(ctx context.Context, userA, userB string) error {
	// This is a placeholder for mutual friend count calculation
	// In a real implementation, this would:
	// 1. Query the database for mutual friends between userA and userB
	// 2. Update the mutual_friend_count for relevant users in Elasticsearch
	
	log.Printf("Updating mutual friend counts for users %s and %s", userA, userB)
	
	// For now, just update both users with a placeholder calculation
	for _, userID := range []string{userA, userB} {
		updateScript := map[string]interface{}{
			"script": map[string]interface{}{
				"source": "if (ctx._source.social_metrics == null) { ctx._source.social_metrics = [:] } ctx._source.social_metrics.mutual_friend_count = params.count; ctx._source.updated_at = params.timestamp",
				"params": map[string]interface{}{
					"count":     1, // Simplified - would be calculated
					"timestamp": time.Now(),
				},
			},
		}
		
		err := ec.userIndexer.esClient.UpdateDocument(ctx, "users", userID, updateScript)
		if err != nil {
			log.Printf("Failed to update mutual friend count for user %s: %v", userID, err)
		}
	}
	
	return nil
}

// Stop stops the event consumer
func (ec *EventConsumer) Stop() error {
	log.Printf("Stopping search index event consumer...")
	return ec.rabbitmqClient.Close()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}