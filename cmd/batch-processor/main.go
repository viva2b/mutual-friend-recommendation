package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mutual-friend/internal/batch"
	"mutual-friend/internal/cache"
	"mutual-friend/internal/repository"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/rabbitmq"
	"mutual-friend/pkg/redis"
)

func main() {
	// Create logger
	logger := log.New(os.Stdout, "[BATCH-PROCESSOR] ", log.LstdFlags|log.Lshortfile)
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}
	
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Initialize DynamoDB client
	dynamoClient, err := repository.NewDynamoDBClient(cfg.DynamoDB)
	if err != nil {
		logger.Fatalf("Failed to create DynamoDB client: %v", err)
	}
	
	// Initialize repositories
	friendRepo := repository.NewFriendRepository(dynamoClient)
	
	// Initialize Redis client
	redisClient, err := redis.NewClient(cfg.Redis)
	if err != nil {
		logger.Fatalf("Failed to create Redis client: %v", err)
	}
	defer redisClient.Close()
	
	// Initialize cache service
	cacheService := cache.NewRedisService(redisClient)
	
	// Initialize RabbitMQ client
	rabbitClient, err := rabbitmq.NewClient(cfg.RabbitMQ)
	if err != nil {
		logger.Fatalf("Failed to create RabbitMQ client: %v", err)
	}
	defer rabbitClient.Close()
	
	// Create consumer configuration
	consumerConfig := batch.ConsumerConfig{
		QueueName:      "friend-events-batch",
		ExchangeName:   "friend-events",
		RoutingKey:     "friend.*",
		ConsumerTag:    "batch-processor",
		WorkerCount:    10,
		PrefetchCount:  100,
		ReconnectDelay: 5 * time.Second,
	}
	
	// Create batch consumer
	consumer := batch.NewBatchConsumer(
		consumerConfig,
		rabbitClient,
		friendRepo,
		cacheService,
		logger,
	)
	
	// Start the consumer
	if err := consumer.Start(ctx); err != nil {
		logger.Fatalf("Failed to start consumer: %v", err)
	}
	
	// Start metrics reporting
	go reportMetrics(consumer, logger)
	
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	logger.Println("Batch processor is running. Press Ctrl+C to stop.")
	
	// Wait for shutdown signal
	<-sigChan
	
	logger.Println("Received shutdown signal, stopping gracefully...")
	
	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	// Stop the consumer
	if err := consumer.Stop(shutdownCtx); err != nil {
		logger.Printf("Error stopping consumer: %v", err)
	}
	
	// Final metrics and limitation analysis
	finalMetrics := consumer.GetMetrics()
	logger.Printf("Final Metrics: %+v", finalMetrics)
	
	limitations := consumer.AnalyzeLimitations()
	logger.Printf("Limitation Analysis:")
	logger.Printf("  - Nack Cascade: %+v", limitations.NackCascadeIssues)
	logger.Printf("  - Goroutine Overhead: %+v", limitations.GoroutineOverhead)
	logger.Printf("  - Memory Usage: %+v", limitations.MemoryUsageAnalysis)
	logger.Printf("  - Throughput: %+v", limitations.ThroughputAnalysis)
	
	logger.Println("Batch processor stopped successfully")
}

// reportMetrics periodically reports processor metrics
func reportMetrics(consumer *batch.BatchConsumer, logger *log.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		metrics := consumer.GetMetrics()
		logger.Printf("=== Processor Metrics ===")
		logger.Printf("Events Ingested: %d", metrics.EventsIngested)
		logger.Printf("Events Dropped: %d", metrics.EventsDropped)
		logger.Printf("Batches Created: %d", metrics.BatchesCreated)
		logger.Printf("Batches Completed: %d", metrics.BatchesCompleted)
		logger.Printf("Users Processed: %d", metrics.UsersProcessed)
		logger.Printf("Processing Errors: %d", metrics.ProcessingErrors)
		logger.Printf("Retry Attempts: %d", metrics.RetryAttempts)
		logger.Printf("Avg Batch Processing Time: %v", metrics.AvgBatchProcessingTime)
		logger.Printf("Goroutine Count: %d", metrics.GoroutineCount)
		logger.Printf("Memory Usage: %.2f MB", metrics.MemoryUsageMB)
		logger.Printf("Error Rate: %.2f%%", metrics.ErrorRate*100)
		logger.Printf("Retry Rate: %.2f%%", metrics.RetryRate*100)
		logger.Printf("========================")
		
		// Check for issues
		if metrics.ErrorRate > 0.05 {
			logger.Printf("WARNING: High error rate detected: %.2f%%", metrics.ErrorRate*100)
		}
		if metrics.GoroutineCount > 100 {
			logger.Printf("WARNING: High goroutine count: %d", metrics.GoroutineCount)
		}
		if metrics.MemoryUsageMB > 500 {
			logger.Printf("WARNING: High memory usage: %.2f MB", metrics.MemoryUsageMB)
		}
	}
}