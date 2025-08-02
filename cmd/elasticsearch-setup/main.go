package main

import (
	"context"
	"fmt"
	"log"

	"mutual-friend/internal/repository"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/elasticsearch"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("🔍 Setting up Elasticsearch for User Search...")
	fmt.Println("================================================")

	// Initialize Elasticsearch client
	esConfig := elasticsearch.Config{
		Addresses: []string{cfg.Elasticsearch.URL},
	}
	esClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		log.Fatalf("Failed to create Elasticsearch client: %v", err)
	}

	ctx := context.Background()

	// Create users index with mapping
	userMapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"user_id": map[string]interface{}{
					"type": "keyword",
				},
				"username": map[string]interface{}{
					"type": "text",
					"analyzer": "standard",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type": "keyword",
						},
						"suggest": map[string]interface{}{
							"type": "completion",
						},
					},
				},
				"display_name": map[string]interface{}{
					"type": "text",
					"analyzer": "standard",
				},
				"email": map[string]interface{}{
					"type": "text",
					"analyzer": "standard",
				},
				"bio": map[string]interface{}{
					"type": "text",
					"analyzer": "standard",
				},
				"interests": map[string]interface{}{
					"type": "keyword",
				},
				"social_metrics": map[string]interface{}{
					"properties": map[string]interface{}{
						"friend_count": map[string]interface{}{
							"type": "integer",
						},
						"mutual_friend_count": map[string]interface{}{
							"type": "integer",
						},
						"popularity_score": map[string]interface{}{
							"type": "float",
						},
						"activity_score": map[string]interface{}{
							"type": "float",
						},
						"response_rate": map[string]interface{}{
							"type": "float",
						},
					},
				},
				"location": map[string]interface{}{
					"properties": map[string]interface{}{
						"city": map[string]interface{}{
							"type": "keyword",
						},
						"state": map[string]interface{}{
							"type": "keyword",
						},
					},
				},
				"privacy_settings": map[string]interface{}{
					"properties": map[string]interface{}{
						"searchable": map[string]interface{}{
							"type": "boolean",
						},
					},
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"created_at": map[string]interface{}{
					"type": "date",
				},
				"updated_at": map[string]interface{}{
					"type": "date",
				},
				"last_active": map[string]interface{}{
					"type": "date",
				},
			},
		},
	}

	// Create users index
	fmt.Println("Creating users index...")
	err = esClient.CreateIndex(ctx, "users", userMapping)
	if err != nil {
		log.Printf("Warning: Failed to create index (might already exist): %v", err)
	} else {
		fmt.Println("✅ Users index created successfully")
	}

	// Initialize DynamoDB to get user data
	fmt.Println("Fetching user data from DynamoDB...")
	dynamoClient, err := repository.NewDynamoDBClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client: %v", err)
	}

	userRepo := repository.NewUserRepository(dynamoClient)
	friendRepo := repository.NewFriendRepository(dynamoClient)

	// User IDs from setup
	userIDs := []string{
		"2c1db4fa-4bfd-4b3f-8ba4-5b1e6af13cef", // alice
		"3044887a-2e71-4c9d-b9e6-455f1ca3d51b", // bob  
		"e8f80bb6-328f-4063-afb9-ce9724c3520b", // charlie
		"e8aed391-d481-4a63-80ac-c6e5edbfcb01", // diana
		"edb7acd0-5690-4fa1-b11d-7cbe2a9b8ac4", // eve
	}

	// Index each user
	fmt.Println("Indexing user data...")
	for _, userID := range userIDs {
		user, err := userRepo.GetUser(ctx, userID)
		if err != nil {
			log.Printf("Failed to get user %s: %v", userID, err)
			continue
		}

		// Get friend count
		friendCount, err := friendRepo.GetFriendCount(ctx, userID)
		if err != nil {
			log.Printf("Failed to get friend count for %s: %v", userID, err)
			friendCount = 0
		}

		// Create Elasticsearch document
		esDoc := map[string]interface{}{
			"user_id":      user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"email":        user.Email,
			"bio":          fmt.Sprintf("Hello, I'm %s! I love technology and making new friends.", user.DisplayName),
			"interests":    []string{"technology", "programming", "music", "travel"},
			"social_metrics": map[string]interface{}{
				"friend_count":        friendCount,
				"mutual_friend_count": 0, // Will be calculated dynamically
				"popularity_score":    float64(friendCount) * 1.5,
				"activity_score":      85.5,
				"response_rate":       0.92,
			},
			"location": map[string]interface{}{
				"city":  "Seoul",
				"state": "Seoul",
			},
			"privacy_settings": map[string]interface{}{
				"searchable": true,
			},
			"status":      "active",
			"created_at":  user.CreatedAt,
			"updated_at":  user.UpdatedAt,
			"last_active": user.UpdatedAt,
		}

		// Index the document
		err = esClient.IndexDocument(ctx, "users", user.ID, esDoc)
		if err != nil {
			log.Printf("Failed to index user %s: %v", userID, err)
		} else {
			fmt.Printf("✅ Indexed user: %s (%s)\n", user.Username, user.ID)
		}
	}

	fmt.Println("================================================")
	fmt.Println("🎉 Elasticsearch setup complete!")
	fmt.Println("📊 Users indexed and ready for search testing")
}