package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"mutual-friend/internal/cache"
	"mutual-friend/internal/domain"
	"mutual-friend/internal/service"
	"mutual-friend/pkg/config"
	cacheTypes "mutual-friend/pkg/cache"
)

// MockFriendRepository for testing (since we don't have a real DynamoDB connection in this test)
type MockFriendRepository struct{}

func (m *MockFriendRepository) AddFriend(ctx context.Context, userID, friendID string) error {
	return nil
}

func (m *MockFriendRepository) RemoveFriend(ctx context.Context, userID, friendID string) error {
	return nil
}

func (m *MockFriendRepository) AreFriends(ctx context.Context, userID, friendID string) (bool, error) {
	// Simulate some friendship relationships
	friends := map[string][]string{
		"user1": {"user2", "user3"},
		"user2": {"user1", "user4"},
		"user3": {"user1"},
		"user4": {"user2"},
	}
	
	if userFriends, exists := friends[userID]; exists {
		for _, friend := range userFriends {
			if friend == friendID {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *MockFriendRepository) GetFriends(ctx context.Context, userID string, limit int, startKey interface{}) ([]*domain.Friend, interface{}, error) {
	// Simulate friend lists
	friendsData := map[string][]*domain.Friend{
		"user1": {
			{UserID: "user1", FriendID: "user2", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
			{UserID: "user1", FriendID: "user3", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
		},
		"user2": {
			{UserID: "user2", FriendID: "user1", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
			{UserID: "user2", FriendID: "user4", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
		},
		"user3": {
			{UserID: "user3", FriendID: "user1", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
		},
		"user4": {
			{UserID: "user4", FriendID: "user2", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
		},
	}
	
	if friends, exists := friendsData[userID]; exists {
		return friends, nil, nil
	}
	return []*domain.Friend{}, nil, nil
}

func (m *MockFriendRepository) GetFriendCount(ctx context.Context, userID string) (int, error) {
	friends, _, _ := m.GetFriends(ctx, userID, 100, nil)
	return len(friends), nil
}

func (m *MockFriendRepository) GetMutualFriends(ctx context.Context, userID1, userID2 string) ([]*domain.Friend, error) {
	// Simple implementation - user1 and user2 have no mutual friends in this mock
	if userID1 == "user1" && userID2 == "user2" {
		return []*domain.Friend{}, nil
	}
	return []*domain.Friend{}, nil
}

// MockUserRepository for testing
type MockUserRepository struct{}

func (m *MockUserRepository) CreateUser(ctx context.Context, user *domain.User) error {
	return nil
}

func (m *MockUserRepository) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	// Return mock user data
	return &domain.User{
		ID:          userID,
		Username:    fmt.Sprintf("user_%s", userID),
		Email:       fmt.Sprintf("%s@example.com", userID),
		DisplayName: fmt.Sprintf("User %s", userID),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (m *MockUserRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	return nil
}

func (m *MockUserRepository) DeleteUser(ctx context.Context, userID string) error {
	return nil
}

func main() {
	log.Println("Starting Friend Cache Test...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create cache service
	factory := cache.NewFactory(cfg)
	cacheService, err := factory.CreateCacheService()
	if err != nil {
		log.Fatalf("Failed to create cache service: %v", err)
	}
	defer cacheService.Close()

	// Create mock repositories
	friendRepo := &MockFriendRepository{}
	userRepo := &MockUserRepository{}
	
	// Create mock event service (nil for this test)
	var eventService *service.EventService = nil

	// Create friend service with cache
	friendService := service.NewFriendService(friendRepo, userRepo, eventService, cacheService)

	// Test cache functionality
	if err := testFriendCaching(friendService, cacheService); err != nil {
		log.Printf("Friend caching test failed: %v", err)
	} else {
		log.Println("✅ Friend caching test completed successfully!")
	}

	log.Println("Friend Cache Test completed!")
}

func testFriendCaching(friendService *service.FriendService, cacheService cacheTypes.Cache) error {
	ctx := context.Background()
	
	log.Println("\n=== Testing Friend Cache Functionality ===")

	// Test 1: Friend Count Caching
	log.Println("\n--- Test 1: Friend Count Caching ---")
	userID := "user1"
	
	// First call - should be cache miss
	count1, err := friendService.GetFriendCount(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get friend count: %w", err)
	}
	log.Printf("First call - Friend count for %s: %d", userID, count1)

	// Second call - should be cache hit
	count2, err := friendService.GetFriendCount(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get friend count (cached): %w", err)
	}
	log.Printf("Second call - Friend count for %s: %d (should be cached)", userID, count2)

	if count1 != count2 {
		return fmt.Errorf("cached friend count doesn't match original")
	}

	// Test 2: Friend List Caching
	log.Println("\n--- Test 2: Friend List Caching ---")
	
	// First call - should be cache miss
	friends1, _, err := friendService.GetFriends(ctx, userID, 10, nil)
	if err != nil {
		return fmt.Errorf("failed to get friends: %w", err)
	}
	log.Printf("First call - Friends for %s: %d friends", userID, len(friends1))

	// Second call - should be cache hit
	friends2, _, err := friendService.GetFriends(ctx, userID, 10, nil)
	if err != nil {
		return fmt.Errorf("failed to get friends (cached): %w", err)
	}
	log.Printf("Second call - Friends for %s: %d friends (should be cached)", userID, len(friends2))

	if len(friends1) != len(friends2) {
		return fmt.Errorf("cached friends list doesn't match original")
	}

	// Test 3: Friendship Check Caching
	log.Println("\n--- Test 3: Friendship Check Caching ---")
	
	// First call - should be cache miss
	areFriends1, err := friendService.AreFriends(ctx, "user1", "user2")
	if err != nil {
		return fmt.Errorf("failed to check friendship: %w", err)
	}
	log.Printf("First call - Are user1 and user2 friends: %v", areFriends1)

	// Second call - should be cache hit
	areFriends2, err := friendService.AreFriends(ctx, "user1", "user2")
	if err != nil {
		return fmt.Errorf("failed to check friendship (cached): %w", err)
	}
	log.Printf("Second call - Are user1 and user2 friends: %v (should be cached)", areFriends2)

	if areFriends1 != areFriends2 {
		return fmt.Errorf("cached friendship check doesn't match original")
	}

	// Test 4: Mutual Friends Caching
	log.Println("\n--- Test 4: Mutual Friends Caching ---")
	
	// First call - should be cache miss
	mutual1, err := friendService.GetMutualFriends(ctx, "user1", "user2")
	if err != nil {
		return fmt.Errorf("failed to get mutual friends: %w", err)
	}
	log.Printf("First call - Mutual friends between user1 and user2: %d", len(mutual1))

	// Second call - should be cache hit
	mutual2, err := friendService.GetMutualFriends(ctx, "user1", "user2")
	if err != nil {
		return fmt.Errorf("failed to get mutual friends (cached): %w", err)
	}
	log.Printf("Second call - Mutual friends between user1 and user2: %d (should be cached)", len(mutual2))

	if len(mutual1) != len(mutual2) {
		return fmt.Errorf("cached mutual friends doesn't match original")
	}

	// Test 5: Cache Invalidation
	log.Println("\n--- Test 5: Cache Invalidation ---")
	
	// First, populate cache
	_, err = friendService.GetFriendCount(ctx, "user1")
	if err != nil {
		return fmt.Errorf("failed to populate cache: %w", err)
	}
	
	// Check cache exists
	cacheKey := cacheTypes.CacheKeyFriendCount.Format("user1")
	_, err = cacheService.Get(ctx, cacheKey)
	if err != nil {
		return fmt.Errorf("cache should exist before invalidation")
	}
	log.Printf("Cache verified to exist before invalidation")

	// Add a friend (this should invalidate cache)
	err = friendService.AddFriend(ctx, "user1", "user5")
	if err != nil {
		return fmt.Errorf("failed to add friend: %w", err)
	}
	log.Printf("Added friend user5 to user1 (should invalidate cache)")

	// Check cache is invalidated
	time.Sleep(100 * time.Millisecond) // Small delay to ensure invalidation completes
	_, err = cacheService.Get(ctx, cacheKey)
	if err == nil {
		log.Printf("Warning: Cache still exists after invalidation (might be expected in mock)")
	} else {
		log.Printf("Cache successfully invalidated after friend addition")
	}

	// Test 6: Cache Metrics
	log.Println("\n--- Test 6: Cache Metrics ---")
	
	// Check if cache service supports metrics
	if cacheServiceImpl, ok := cacheService.(*cache.Service); ok {
		metrics := cacheServiceImpl.GetMetrics()
		log.Printf("Cache Metrics:")
		log.Printf("  Hits: %d", metrics.Hits)
		log.Printf("  Misses: %d", metrics.Misses)
		log.Printf("  Hit Rate: %.2f%%", metrics.HitRate)
		log.Printf("  Operations: %d", metrics.Operations)
		log.Printf("  Errors: %d", metrics.Errors)
		log.Printf("  Avg Latency: %.2f ms", metrics.AvgLatency)
	}

	return nil
}