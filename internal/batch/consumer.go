package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/rabbitmq/amqp091-go"
	
	"mutual-friend/internal/cache"
	"mutual-friend/internal/repository"
	"mutual-friend/pkg/events"
	"mutual-friend/pkg/rabbitmq"
)

// ConsumerConfig contains configuration for the batch consumer
type ConsumerConfig struct {
	QueueName       string
	ExchangeName    string
	RoutingKey      string
	ConsumerTag     string
	WorkerCount     int
	PrefetchCount   int
	ReconnectDelay  time.Duration
}

// DefaultConsumerConfig returns default consumer configuration
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		QueueName:      "friend-events-batch",
		ExchangeName:   "friend-events",
		RoutingKey:     "friend.*",
		ConsumerTag:    "batch-processor",
		WorkerCount:    10,
		PrefetchCount:  100,
		ReconnectDelay: 5 * time.Second,
	}
}

// BatchConsumer consumes events from RabbitMQ and feeds them to the batch processor
type BatchConsumer struct {
	config          ConsumerConfig
	rabbitClient    *rabbitmq.Client
	batchProcessor  *BatchProcessor
	logger          *log.Logger
	shutdownChan    chan struct{}
}

// NewBatchConsumer creates a new batch consumer
func NewBatchConsumer(
	config ConsumerConfig,
	rabbitClient *rabbitmq.Client,
	friendRepo *repository.FriendRepository,
	cacheService cache.Service,
	logger *log.Logger,
) *BatchConsumer {
	processorConfig := DefaultProcessorConfig()
	batchProcessor := NewBatchProcessor(processorConfig, friendRepo, cacheService, logger)
	
	return &BatchConsumer{
		config:         config,
		rabbitClient:   rabbitClient,
		batchProcessor: batchProcessor,
		logger:         logger,
		shutdownChan:   make(chan struct{}),
	}
}

// Start begins consuming messages and processing them in batches
func (bc *BatchConsumer) Start(ctx context.Context) error {
	// Start the batch processor
	if err := bc.batchProcessor.Start(ctx); err != nil {
		return fmt.Errorf("failed to start batch processor: %w", err)
	}
	
	// Setup RabbitMQ consumer
	if err := bc.setupConsumer(); err != nil {
		return fmt.Errorf("failed to setup consumer: %w", err)
	}
	
	// Start consuming messages
	go bc.consumeLoop(ctx)
	
	bc.logger.Println("Batch consumer started successfully")
	return nil
}

// setupConsumer sets up the RabbitMQ consumer
func (bc *BatchConsumer) setupConsumer() error {
	channel := bc.rabbitClient.GetChannel()
	
	// Declare exchange
	err := channel.ExchangeDeclare(
		bc.config.ExchangeName,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}
	
	// Declare queue
	queue, err := channel.QueueDeclare(
		bc.config.QueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp091.Table{
			"x-message-ttl": 3600000, // 1 hour TTL
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}
	
	// Bind queue to exchange
	err = channel.QueueBind(
		queue.Name,
		bc.config.RoutingKey,
		bc.config.ExchangeName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}
	
	// Set QoS
	err = channel.Qos(
		bc.config.PrefetchCount, // prefetch count
		0,                        // prefetch size
		false,                    // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}
	
	return nil
}

// consumeLoop continuously consumes messages from RabbitMQ
func (bc *BatchConsumer) consumeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			bc.logger.Println("Consumer loop stopped by context")
			return
		case <-bc.shutdownChan:
			bc.logger.Println("Consumer loop stopped by shutdown signal")
			return
		default:
			if err := bc.consume(ctx); err != nil {
				bc.logger.Printf("Consumer error: %v, reconnecting in %v", err, bc.config.ReconnectDelay)
				time.Sleep(bc.config.ReconnectDelay)
				
				// Try to reconnect
				if err := bc.rabbitClient.Reconnect(); err != nil {
					bc.logger.Printf("Failed to reconnect: %v", err)
				}
			}
		}
	}
}

// consume starts consuming messages from the queue
func (bc *BatchConsumer) consume(ctx context.Context) error {
	channel := bc.rabbitClient.GetChannel()
	
	deliveries, err := channel.Consume(
		bc.config.QueueName,
		bc.config.ConsumerTag,
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}
	
	bc.logger.Printf("Consumer registered, waiting for messages...")
	
	for {
		select {
		case <-ctx.Done():
			return nil
			
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			
			// Process the delivery
			if err := bc.processDelivery(ctx, delivery); err != nil {
				bc.logger.Printf("Failed to process delivery: %v", err)
				
				// Nack the message with requeue
				// NOTE: This is one of the limitations mentioned in the portfolio
				// A single message failure causes the entire batch to be reprocessed
				if nackErr := delivery.Nack(false, true); nackErr != nil {
					bc.logger.Printf("Failed to nack message: %v", nackErr)
				}
			} else {
				// Ack the message
				if ackErr := delivery.Ack(false); ackErr != nil {
					bc.logger.Printf("Failed to ack message: %v", ackErr)
				}
			}
		}
	}
}

// processDelivery processes a single message delivery
func (bc *BatchConsumer) processDelivery(ctx context.Context, delivery amqp091.Delivery) error {
	// Parse the event based on routing key
	var event *events.FriendEvent
	
	switch delivery.RoutingKey {
	case "friend.added", "friend.removed":
		var friendEvent events.FriendEvent
		if err := json.Unmarshal(delivery.Body, &friendEvent); err != nil {
			return fmt.Errorf("failed to unmarshal friend event: %w", err)
		}
		event = &friendEvent
		
	default:
		bc.logger.Printf("Unknown routing key: %s, skipping message", delivery.RoutingKey)
		return nil
	}
	
	// Send event to batch processor
	if err := bc.batchProcessor.ProcessEvent(event); err != nil {
		return fmt.Errorf("failed to process event: %w", err)
	}
	
	return nil
}

// Stop gracefully shuts down the consumer
func (bc *BatchConsumer) Stop(ctx context.Context) error {
	bc.logger.Println("Stopping batch consumer...")
	
	// Stop consuming new messages
	close(bc.shutdownChan)
	
	// Stop the batch processor
	if err := bc.batchProcessor.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop batch processor: %w", err)
	}
	
	// Cancel consumer on RabbitMQ
	channel := bc.rabbitClient.GetChannel()
	if err := channel.Cancel(bc.config.ConsumerTag, false); err != nil {
		return fmt.Errorf("failed to cancel consumer: %w", err)
	}
	
	bc.logger.Println("Batch consumer stopped successfully")
	return nil
}

// GetMetrics returns the batch processor metrics
func (bc *BatchConsumer) GetMetrics() ProcessorMetricsSnapshot {
	return bc.batchProcessor.GetMetrics()
}

// AnalyzeLimitations returns analysis of known limitations
func (bc *BatchConsumer) AnalyzeLimitations() LimitationAnalysis {
	return bc.batchProcessor.metrics.AnalyzeLimitations()
}