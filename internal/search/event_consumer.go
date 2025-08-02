package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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
	err := ec.rabbitmqClient.DeclareExchange("friend-events", "topic", true, false)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}
	
	err = ec.rabbitmqClient.DeclareExchange("user-events", "topic", true, false)
	if err != nil {
		return fmt.Errorf("failed to declare user-events exchange: %w", err)
	}
	
	// Declare queue
	err = ec.rabbitmqClient.DeclareQueue(ec.queueName, true, false, false)
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
		err = ec.rabbitmqClient.BindQueue(ec.queueName, binding.exchange, binding.routingKey)
		if err != nil {
			return fmt.Errorf("failed to bind queue to %s with key %s: %w", 
				binding.exchange, binding.routingKey, err)
		}
	}
	
	log.Printf("Successfully setup search index queue with bindings")
	return nil
}

// processMessages processes incoming messages with enhanced error handling
func (ec *EventConsumer) processMessages(ctx context.Context, deliveries <-chan rabbitmq.Delivery) {
	maxRetries := 3
	retryDelay := time.Second * 2
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("Search index event consumer stopped")
			return
		case delivery := <-deliveries:
			err := ec.processMessageWithRetry(ctx, delivery, maxRetries, retryDelay)
			if err != nil {
				log.Printf("Failed to process message after %d retries: %v", maxRetries, err)
				// Send to dead letter queue or log for manual inspection
				ec.handleFailedMessage(delivery, err)
				delivery.Nack(false, false) // Don't requeue after max retries
			} else {
				// Ack message
				delivery.Ack(false)
			}
		}
	}
}

// processMessageWithRetry processes a message with retry logic
func (ec *EventConsumer) processMessageWithRetry(ctx context.Context, delivery rabbitmq.Delivery, maxRetries int, retryDelay time.Duration) error {
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay * time.Duration(attempt)):
				// Continue with retry
			}
		}
		
		err := ec.processMessage(ctx, delivery)
		if err == nil {
			if attempt > 0 {
				log.Printf("Message processed successfully on attempt %d", attempt+1)
			}
			return nil
		}
		
		lastErr = err
		log.Printf("Message processing failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
		
		// Check if error is retryable
		if !ec.isRetryableError(err) {
			log.Printf("Non-retryable error encountered: %v", err)
			return err
		}
	}
	
	return fmt.Errorf("message processing failed after %d attempts: %w", maxRetries, lastErr)
}

// isRetryableError determines if an error is worth retrying
func (ec *EventConsumer) isRetryableError(err error) bool {
	errStr := err.Error()
	
	// Network related errors are usually retryable
	retryableErrors := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"service unavailable",
		"too many requests",
	}
	
	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}
	
	// JSON unmarshaling errors are usually not retryable
	if strings.Contains(errStr, "unmarshal") || strings.Contains(errStr, "json") {
		return false
	}
	
	// Default to retryable for unknown errors
	return true
}

// handleFailedMessage handles messages that failed after all retries
func (ec *EventConsumer) handleFailedMessage(delivery rabbitmq.Delivery, err error) {
	// Log the failed message for manual inspection
	log.Printf("FAILED MESSAGE - Routing Key: %s, Body: %s, Error: %v", 
		delivery.RoutingKey, string(delivery.Body), err)
	
	// In a production system, you might want to:
	// 1. Send to a dead letter queue
	// 2. Store in a database for later processing
	// 3. Send alerts to monitoring systems
	// 4. Write to a file for manual inspection
	
	// For now, we'll just log it
	failureData := map[string]interface{}{
		"timestamp":   time.Now(),
		"routing_key": delivery.RoutingKey,
		"message":     string(delivery.Body),
		"error":       err.Error(),
		"headers":     delivery.Headers,
	}
	
	// This could be enhanced to write to a failure log file or database
	log.Printf("FAILURE_LOG: %+v", failureData)
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
		event.Type, event.UserID, event.FriendID)
	
	// Update social metrics for both users involved
	switch event.Type {
	case events.EventTypeFriendAdded:
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
		
	case events.EventTypeFriendRemoved:
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
	
	log.Printf("Processing user event: %s for user %s", event.Type, event.UserID)
	
	switch event.Type {
	case events.EventTypeUserCreated:
		// Index new user
		if event.Username != "" || event.DisplayName != "" {
			// Convert event to user document for indexing
			userDoc := map[string]interface{}{
				"user_id":      event.UserID,
				"username":     event.Username,
				"display_name": event.DisplayName,
				"email":        event.Email,
				"social_metrics": map[string]interface{}{
					"friend_count":       0,
					"mutual_friend_count": 0,
				},
				"created_at": event.Timestamp,
				"updated_at": event.Timestamp,
			}
			
			// Index the user document
			if err := ec.userIndexer.IndexUserFromEvent(ctx, event.UserID, userDoc); err != nil {
				return fmt.Errorf("failed to index new user %s: %w", event.UserID, err)
			}
			
			log.Printf("Successfully indexed new user: %s", event.UserID)
		}
		
	case events.EventTypeUserUpdated:
		// Update existing user
		if event.Username != "" || event.DisplayName != "" {
			updateDoc := map[string]interface{}{
				"doc": map[string]interface{}{
					"username":     event.Username,
					"display_name": event.DisplayName,
					"email":        event.Email,
					"updated_at":   event.Timestamp,
				},
			}
			
			// Update the user document
			if err := ec.userIndexer.UpdateUserFromEvent(ctx, event.UserID, updateDoc); err != nil {
				return fmt.Errorf("failed to update user %s: %w", event.UserID, err)
			}
			
			log.Printf("Successfully updated user: %s", event.UserID)
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