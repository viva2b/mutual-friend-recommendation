package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"mutual-friend/internal/cache"
	"mutual-friend/pkg/config"
	cacheTypes "mutual-friend/pkg/cache"
)

func main() {
	log.Println("Starting Redis cache test...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create cache factory
	factory := cache.NewFactory(cfg)

	// Test Redis client directly
	if err := testRedisClient(factory); err != nil {
		log.Printf("Redis client test failed: %v", err)
	}

	// Test cache service
	if err := testCacheService(factory); err != nil {
		log.Printf("Cache service test failed: %v", err)
	}

	// Test cache manager
	if err := testCacheManager(factory); err != nil {
		log.Printf("Cache manager test failed: %v", err)
	}

	log.Println("Redis cache test completed!")
}

func testRedisClient(factory *cache.Factory) error {
	log.Println("\n=== Testing Redis Client ===")

	client, err := factory.CreateRedisClient()
	if err != nil {
		return fmt.Errorf("failed to create Redis client: %w", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test basic operations
	testKey := "test:redis:client"
	testValue := "Hello, Redis!"

	// Test Set
	log.Printf("Setting key '%s' with value '%s'", testKey, testValue)
	err = client.Set(ctx, testKey, testValue, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	// Test Get
	log.Printf("Getting key '%s'", testKey)
	value, err := client.Get(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}
	log.Printf("Retrieved value: %s", value)

	if value != testValue {
		return fmt.Errorf("value mismatch: expected %s, got %s", testValue, value)
	}

	// Test TTL
	ttl, err := client.TTL(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to get TTL: %w", err)
	}
	log.Printf("TTL for key '%s': %v", testKey, ttl)

	// Test Exists
	exists, err := client.Exists(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to check existence: %w", err)
	}
	log.Printf("Key '%s' exists: %d", testKey, exists)

	// Test Delete
	deleted, err := client.Delete(ctx, testKey)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	log.Printf("Deleted %d keys", deleted)

	// Verify deletion
	_, err = client.Get(ctx, testKey)
	if err == nil {
		return fmt.Errorf("key should have been deleted")
	}
	log.Printf("Confirmed key was deleted")

	log.Println("✅ Redis client test passed!")
	return nil
}

func testCacheService(factory *cache.Factory) error {
	log.Println("\n=== Testing Cache Service ===")

	cacheService, err := factory.CreateCacheService()
	if err != nil {
		return fmt.Errorf("failed to create cache service: %w", err)
	}
	defer cacheService.Close()

	ctx := context.Background()

	// Test JSON caching
	testKey := "test:cache:service"
	testData := map[string]interface{}{
		"user_id":   "12345",
		"username":  "testuser",
		"friends":   []string{"friend1", "friend2", "friend3"},
		"timestamp": time.Now().Unix(),
	}

	// Test SetJSON
	log.Printf("Setting JSON data for key '%s'", testKey)
	if service, ok := cacheService.(*cache.Service); ok {
		err = service.SetJSON(ctx, testKey, testData, 60*time.Second)
		if err != nil {
			return fmt.Errorf("failed to set JSON: %w", err)
		}
	}

	// Test GetJSON
	log.Printf("Getting JSON data for key '%s'", testKey)
	var retrievedData map[string]interface{}
	if service, ok := cacheService.(*cache.Service); ok {
		err = service.GetJSON(ctx, testKey, &retrievedData)
		if err != nil {
			return fmt.Errorf("failed to get JSON: %w", err)
		}
	}

	log.Printf("Retrieved JSON data: %+v", retrievedData)

	// Test cache metrics
	if service, ok := cacheService.(*cache.Service); ok {
		metrics := service.GetMetrics()
		log.Printf("Cache metrics: Hits=%d, Misses=%d, HitRate=%.2f%%, Operations=%d",
			metrics.Hits, metrics.Misses, metrics.HitRate, metrics.Operations)
	}

	// Test health check
	if !cacheService.IsHealthy() {
		return fmt.Errorf("cache service is not healthy")
	}
	log.Printf("Cache service is healthy")

	// Test cache keys with TTL
	friendsKey := cacheTypes.CacheKeyUserFriends.Format("user123")
	log.Printf("Testing cache key pattern: %s", friendsKey)

	err = cacheService.Set(ctx, friendsKey, []string{"friend1", "friend2"}, cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyUserFriends))
	if err != nil {
		return fmt.Errorf("failed to set friends list: %w", err)
	}

	ttl, err := cacheService.GetTTL(ctx, friendsKey)
	if err != nil {
		return fmt.Errorf("failed to get TTL: %w", err)
	}
	log.Printf("TTL for friends list: %v", ttl)

	log.Println("✅ Cache service test passed!")
	return nil
}

func testCacheManager(factory *cache.Factory) error {
	log.Println("\n=== Testing Cache Manager ===")

	manager, err := factory.CreateCacheManager()
	if err != nil {
		return fmt.Errorf("failed to create cache manager: %w", err)
	}
	defer manager.Close()

	// Test health check
	err = manager.HealthCheck()
	if err != nil {
		return fmt.Errorf("cache manager health check failed: %w", err)
	}
	log.Printf("Cache manager health check passed")

	// Test metrics
	metrics := manager.GetMetrics()
	log.Printf("Manager metrics: %+v", metrics)

	// Test GetOrSet functionality
	ctx := context.Background()
	cacheImpl := manager.GetCache()

	testKey := "test:cache:get_or_set"
	if service, ok := cacheImpl.(*cache.Service); ok {
		value, err := service.GetOrSet(ctx, testKey, 30*time.Second, func() (interface{}, error) {
			log.Printf("Cache miss - generating value for key '%s'", testKey)
			return fmt.Sprintf("Generated at %s", time.Now().Format(time.RFC3339)), nil
		})
		if err != nil {
			return fmt.Errorf("failed GetOrSet: %w", err)
		}
		log.Printf("GetOrSet result: %s", value)

		// Test cache hit
		value2, err := service.GetOrSet(ctx, testKey, 30*time.Second, func() (interface{}, error) {
			return "This should not be called", nil
		})
		if err != nil {
			return fmt.Errorf("failed GetOrSet (cache hit): %w", err)
		}
		log.Printf("GetOrSet cache hit result: %s", value2)

		if value != value2 {
			return fmt.Errorf("cache hit should return same value")
		}
	}

	log.Println("✅ Cache manager test passed!")
	return nil
}