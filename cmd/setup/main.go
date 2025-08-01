package main

import (
	"context"
	"fmt"
	"log"

	"mutual-friend/internal/domain"
	"mutual-friend/internal/repository"
	"mutual-friend/pkg/config"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create DynamoDB client
	dynamoClient, err := repository.NewDynamoDBClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client: %v", err)
	}

	// Test connection
	fmt.Println("Testing DynamoDB connection...")
	if err := dynamoClient.TestConnection(ctx); err != nil {
		log.Fatalf("Failed to connect to DynamoDB: %v", err)
	}
	fmt.Println("✅ DynamoDB connection successful")

	// Create table
	fmt.Println("Creating DynamoDB table...")
	if err := dynamoClient.CreateTable(ctx); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	fmt.Println("✅ DynamoDB table created successfully")

	// Create repositories
	userRepo := repository.NewUserRepository(dynamoClient)
	friendRepo := repository.NewFriendRepository(dynamoClient)

	// Create sample users
	fmt.Println("Creating sample users...")
	users := []*domain.User{
		domain.NewUser("alice", "alice@example.com", "Alice Johnson"),
		domain.NewUser("bob", "bob@example.com", "Bob Smith"),
		domain.NewUser("charlie", "charlie@example.com", "Charlie Brown"),
		domain.NewUser("diana", "diana@example.com", "Diana Prince"),
		domain.NewUser("eve", "eve@example.com", "Eve Wilson"),
	}

	for _, user := range users {
		if err := userRepo.CreateUser(ctx, user); err != nil {
			log.Printf("Failed to create user %s: %v", user.Username, err)
			continue
		}
		fmt.Printf("✅ Created user: %s (%s)\n", user.Username, user.ID)
	}

	// Create sample friendships
	fmt.Println("Creating sample friendships...")
	friendships := []struct {
		user1, user2 int
	}{
		{0, 1}, // Alice - Bob
		{0, 2}, // Alice - Charlie
		{1, 2}, // Bob - Charlie
		{1, 3}, // Bob - Diana
		{2, 4}, // Charlie - Eve
		{3, 4}, // Diana - Eve
	}

	for _, friendship := range friendships {
		user1 := users[friendship.user1]
		user2 := users[friendship.user2]
		
		if err := friendRepo.AddFriend(ctx, user1.ID, user2.ID); err != nil {
			log.Printf("Failed to create friendship %s - %s: %v", user1.Username, user2.Username, err)
			continue
		}
		fmt.Printf("✅ Created friendship: %s - %s\n", user1.Username, user2.Username)
	}

	// Verify data
	fmt.Println("\nVerifying created data...")
	
	// List all users
	allUsers, err := userRepo.ListUsers(ctx, 10)
	if err != nil {
		log.Printf("Failed to list users: %v", err)
	} else {
		fmt.Printf("✅ Total users created: %d\n", len(allUsers))
		for _, user := range allUsers {
			fmt.Printf("  - %s (%s): %s\n", user.Username, user.ID, user.Email)
		}
	}

	// Check friend counts
	fmt.Println("\nFriend counts:")
	for _, user := range users {
		count, err := friendRepo.GetFriendCount(ctx, user.ID)
		if err != nil {
			log.Printf("Failed to get friend count for %s: %v", user.Username, err)
			continue
		}
		fmt.Printf("  - %s: %d friends\n", user.Username, count)
	}

	// Test mutual friends
	fmt.Println("\nTesting mutual friends:")
	if len(users) >= 3 {
		mutualFriends, err := friendRepo.GetMutualFriends(ctx, users[0].ID, users[1].ID)
		if err != nil {
			log.Printf("Failed to get mutual friends: %v", err)
		} else {
			fmt.Printf("✅ Mutual friends between %s and %s: %d\n", 
				users[0].Username, users[1].Username, len(mutualFriends))
			for _, friend := range mutualFriends {
				// Find username by ID
				for _, u := range users {
					if u.ID == friend.FriendID {
						fmt.Printf("  - %s\n", u.Username)
						break
					}
				}
			}
		}
	}

	fmt.Println("\n🎉 Setup completed successfully!")
}