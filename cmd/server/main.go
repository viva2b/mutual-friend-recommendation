package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mutual-friend/internal/api"
	"mutual-friend/internal/repository"
	"mutual-friend/internal/service"
	"mutual-friend/pkg/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting %s v%s in %s mode", 
		cfg.App.Name, cfg.App.Version, cfg.App.Environment)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize DynamoDB client
	log.Println("Initializing DynamoDB client...")
	dynamoClient, err := repository.NewDynamoDBClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(dynamoClient)
	friendRepo := repository.NewFriendRepository(dynamoClient)

	// Initialize event service
	log.Println("Initializing event service...")
	eventService, err := service.NewEventService(cfg)
	if err != nil {
		log.Fatalf("Failed to create event service: %v", err)
	}

	// Initialize event service (setup exchanges and queues)
	if err := eventService.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize event service: %v", err)
	}

	// Initialize friend service
	friendService := service.NewFriendService(friendRepo, userRepo, eventService)

	// Initialize gRPC server
	log.Println("Initializing gRPC server...")
	grpcServer, err := api.NewGRPCServer(cfg, friendService, eventService)
	if err != nil {
		log.Fatalf("Failed to create gRPC server: %v", err)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal...")

	// Graceful shutdown
	grpcServer.Stop()
	
	// Close event service
	if err := eventService.Close(); err != nil {
		log.Printf("Error closing event service: %v", err)
	}

	log.Println("Server shutdown complete")
}