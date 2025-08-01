package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"mutual-friend/internal/repository"
	"mutual-friend/internal/service"
	"mutual-friend/pkg/config"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("=== RabbitMQ Event Publishing Test ===\n")

	// Create DynamoDB client
	dynamoClient, err := repository.NewDynamoDBClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client: %v", err)
	}

	// Create repositories
	userRepo := repository.NewUserRepository(dynamoClient)
	friendRepo := repository.NewFriendRepository(dynamoClient)

	// Create event service
	eventService, err := service.NewEventService(cfg)
	if err != nil {
		log.Fatalf("Failed to create event service: %v", err)
	}

	// Initialize event service (setup exchanges and queues)
	fmt.Println("📡 Initializing RabbitMQ exchanges and queues...")
	if err := eventService.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize event service: %v", err)
	}
	fmt.Println("✅ RabbitMQ setup completed\n")

	// Create friend service
	friendService := service.NewFriendService(friendRepo, userRepo, eventService)

	// Test scenarios
	fmt.Println("🧪 Running test scenarios...\n")

	// Scenario 1: Add friend relationship
	fmt.Println("1️⃣ Test: Adding friend relationship")
	userID1 := "f85d8354-c4e9-4e69-b80b-56a168c2933f" // Alice
	userID2 := "afd231e3-db12-4944-a213-55c7955ef0ac" // Bob

	// Check if they are already friends
	areFriends, err := friendService.AreFriends(ctx, userID1, userID2)
	if err != nil {
		log.Printf("Error checking friendship: %v", err)
	} else if areFriends {
		fmt.Printf("   ℹ️  %s and %s are already friends\n", userID1[:8], userID2[:8])
		
		// Remove friendship first for testing
		fmt.Println("   🔄 Removing existing friendship for testing...")
		if err := friendService.RemoveFriend(ctx, userID1, userID2); err != nil {
			log.Printf("   ⚠️  Failed to remove friendship: %v\n", err)
		} else {
			fmt.Println("   ✅ Friendship removed successfully")
			time.Sleep(1 * time.Second) // Wait for event processing
		}
	}

	// Add friendship
	fmt.Println("   ➕ Adding friend relationship...")
	if err := friendService.AddFriend(ctx, userID1, userID2); err != nil {
		log.Printf("   ⚠️  Failed to add friendship: %v\n", err)
	} else {
		fmt.Println("   ✅ Friend relationship added successfully")
		fmt.Println("   📨 FRIEND_ADDED events published")
	}

	time.Sleep(2 * time.Second) // Wait for event processing

	// Scenario 2: Test friend count
	fmt.Println("\n2️⃣ Test: Getting friend counts")
	count1, err := friendService.GetFriendCount(ctx, userID1)
	if err != nil {
		log.Printf("   ⚠️  Failed to get friend count for user1: %v\n", err)
	} else {
		fmt.Printf("   📊 User %s has %d friends\n", userID1[:8], count1)
	}

	count2, err := friendService.GetFriendCount(ctx, userID2)
	if err != nil {
		log.Printf("   ⚠️  Failed to get friend count for user2: %v\n", err)
	} else {
		fmt.Printf("   📊 User %s has %d friends\n", userID2[:8], count2)
	}

	// Scenario 3: Test mutual friends
	fmt.Println("\n3️⃣ Test: Finding mutual friends")
	userID3 := "37051378-2bc5-4e48-92a0-2f3b273cbfd8" // Diana
	mutualFriends, err := friendService.GetMutualFriends(ctx, userID1, userID3)
	if err != nil {
		log.Printf("   ⚠️  Failed to get mutual friends: %v\n", err)
	} else {
		fmt.Printf("   🤝 User %s and %s have %d mutual friends\n", 
			userID1[:8], userID3[:8], len(mutualFriends))
		for _, mutual := range mutualFriends {
			fmt.Printf("      - %s\n", mutual.FriendID[:8])
		}
	}

	// Scenario 4: Remove friend relationship
	fmt.Println("\n4️⃣ Test: Removing friend relationship")
	fmt.Println("   ➖ Removing friend relationship...")
	if err := friendService.RemoveFriend(ctx, userID1, userID2); err != nil {
		log.Printf("   ⚠️  Failed to remove friendship: %v\n", err)
	} else {
		fmt.Println("   ✅ Friend relationship removed successfully")
		fmt.Println("   📨 FRIEND_REMOVED events published")
	}

	time.Sleep(2 * time.Second) // Wait for event processing

	// Scenario 5: Test event service health
	fmt.Println("\n5️⃣ Test: Event service health check")
	if eventService.IsHealthy() {
		fmt.Println("   ✅ Event service is healthy")
	} else {
		fmt.Println("   ⚠️  Event service is not healthy")
	}

	// Final status
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🎉 Event publishing test completed!")
	fmt.Println("📋 Check RabbitMQ Management UI at http://localhost:15672")
	fmt.Println("   Username: admin")
	fmt.Println("   Password: admin123")
	fmt.Println(strings.Repeat("=", 50))

	// Cleanup
	if err := eventService.Close(); err != nil {
		log.Printf("Failed to close event service: %v", err)
	}
}