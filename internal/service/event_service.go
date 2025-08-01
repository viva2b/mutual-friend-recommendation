package service

import (
	"context"
	"fmt"
	"log"

	"mutual-friend/internal/domain"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/events"
	"mutual-friend/pkg/rabbitmq"
)

// EventService handles event publishing for the application
type EventService struct {
	publisher *events.Publisher
	config    *config.Config
}

// NewEventService creates a new event service
func NewEventService(cfg *config.Config) (*EventService, error) {
	// Create RabbitMQ client
	rabbitConfig := &rabbitmq.Config{
		Host:     cfg.RabbitMQ.Host,
		Port:     cfg.RabbitMQ.Port,
		Username: cfg.RabbitMQ.Username,
		Password: cfg.RabbitMQ.Password,
		Vhost:    cfg.RabbitMQ.Vhost,
	}

	client, err := rabbitmq.NewClient(rabbitConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create RabbitMQ client: %w", err)
	}

	// Create publisher
	publisherConfig := &events.PublisherConfig{
		ExchangeName:  "friend-events",
		RetryAttempts: 3,
		RetryDelay:    cfg.RabbitMQ.RetryDelay,
	}

	publisher := events.NewPublisher(client, publisherConfig)

	service := &EventService{
		publisher: publisher,
		config:    cfg,
	}

	return service, nil
}

// Initialize sets up the exchange and queues
func (s *EventService) Initialize(ctx context.Context) error {
	return s.publisher.SetupExchangeAndQueues(ctx)
}

// PublishFriendAdded publishes a friend added event
func (s *EventService) PublishFriendAdded(ctx context.Context, userID, friendID string) error {
	event := events.NewFriendAddedEvent(userID, friendID, "mutual-friend-service")
	
	// Add additional metadata
	event.Metadata = map[string]interface{}{
		"service_version": s.config.App.Version,
		"environment":     s.config.App.Environment,
	}

	log.Printf("Publishing FRIEND_ADDED event: user=%s, friend=%s", userID, friendID)
	return s.publisher.PublishFriendEvent(ctx, event)
}

// PublishFriendRemoved publishes a friend removed event
func (s *EventService) PublishFriendRemoved(ctx context.Context, userID, friendID string) error {
	event := events.NewFriendRemovedEvent(userID, friendID, "mutual-friend-service")
	
	// Add additional metadata
	event.Metadata = map[string]interface{}{
		"service_version": s.config.App.Version,
		"environment":     s.config.App.Environment,
	}

	log.Printf("Publishing FRIEND_REMOVED event: user=%s, friend=%s", userID, friendID)
	return s.publisher.PublishFriendEvent(ctx, event)
}

// PublishUserCreated publishes a user created event
func (s *EventService) PublishUserCreated(ctx context.Context, user *domain.User) error {
	event := events.NewUserCreatedEvent(
		user.ID,
		user.Username,
		user.Email,
		user.DisplayName,
		"mutual-friend-service",
	)
	
	// Add additional metadata
	event.Metadata = map[string]interface{}{
		"service_version": s.config.App.Version,
		"environment":     s.config.App.Environment,
	}

	log.Printf("Publishing USER_CREATED event: user=%s, email=%s", user.ID, user.Email)
	return s.publisher.PublishUserEvent(ctx, event)
}

// PublishRecommendationGenerated publishes a recommendation generated event
func (s *EventService) PublishRecommendationGenerated(ctx context.Context, userID, recommendedUserID string, mutualFriends []string, score float64) error {
	event := events.NewRecommendationGeneratedEvent(
		userID,
		recommendedUserID,
		mutualFriends,
		score,
		"mutual-friend-service",
	)
	
	// Add additional metadata
	event.Metadata = map[string]interface{}{
		"service_version": s.config.App.Version,
		"environment":     s.config.App.Environment,
	}

	log.Printf("Publishing RECOMMENDATION_GENERATED event: user=%s, recommended=%s, score=%.2f", 
		userID, recommendedUserID, score)
	return s.publisher.PublishRecommendationEvent(ctx, event)
}

// PublishBatchEvents publishes multiple events in batch
func (s *EventService) PublishBatchEvents(ctx context.Context, eventsBatch []events.Event) error {
	log.Printf("Publishing batch of %d events", len(eventsBatch))
	return s.publisher.PublishBatch(ctx, eventsBatch)
}

// Close closes the event service
func (s *EventService) Close() error {
	log.Println("Closing event service...")
	if s.publisher != nil {
		return s.publisher.Close()
	}
	return nil
}

// IsHealthy checks if the event service is healthy
func (s *EventService) IsHealthy() bool {
	return s.publisher != nil && s.publisher.IsHealthy()
}