package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	cacheService "mutual-friend/internal/cache"
	"mutual-friend/internal/domain"
	"mutual-friend/pkg/config"
	cacheTypes "mutual-friend/pkg/cache"
)

func main() {
	log.Println("Starting Cache Integration Test...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create cache service
	factory := cacheService.NewFactory(cfg)
	cacheImpl, err := factory.CreateCacheService()
	if err != nil {
		log.Fatalf("Failed to create cache service: %v", err)
	}
	defer cacheImpl.Close()

	// Test cache integration patterns
	if err := testCacheIntegration(cacheImpl); err != nil {
		log.Printf("Cache integration test failed: %v", err)
	} else {
		log.Println("✅ Cache integration test completed successfully!")
	}

	log.Println("Cache Integration Test completed!")
}

func testCacheIntegration(cacheService cacheTypes.Cache) error {
	ctx := context.Background()
	
	log.Println("\n=== Testing Cache Integration Patterns ===")

	// Test 1: Friend List Caching Pattern
	if err := testFriendListCaching(ctx, cacheService); err != nil {
		return fmt.Errorf("friend list caching test failed: %w", err)
	}

	// Test 2: Friend Count Caching Pattern
	if err := testFriendCountCaching(ctx, cacheService); err != nil {
		return fmt.Errorf("friend count caching test failed: %w", err)
	}

	// Test 3: Friendship Status Caching Pattern
	if err := testFriendshipCaching(ctx, cacheService); err != nil {
		return fmt.Errorf("friendship caching test failed: %w", err)
	}

	// Test 4: Cache Invalidation Pattern
	if err := testCacheInvalidation(ctx, cacheService); err != nil {
		return fmt.Errorf("cache invalidation test failed: %w", err)
	}

	// Test 5: Performance Measurement
	if err := testCachePerformance(ctx, cacheService); err != nil {
		return fmt.Errorf("cache performance test failed: %w", err)
	}

	return nil
}

func testFriendListCaching(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test 1: Friend List Caching Pattern ---")

	userID := "user123"
	cacheKey := cacheTypes.CacheKeyUserFriends.Format(userID)
	
	// Create mock friend list
	friends := []*domain.Friend{
		{UserID: userID, FriendID: "friend1", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
		{UserID: userID, FriendID: "friend2", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
		{UserID: userID, FriendID: "friend3", Status: domain.FriendStatusAccepted, CreatedAt: time.Now()},
	}

	// Cache the friend list
	friendsJSON, err := json.Marshal(friends)
	if err != nil {
		return fmt.Errorf("failed to marshal friends: %w", err)
	}

	ttl := cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyUserFriends)
	err = cache.Set(ctx, cacheKey, string(friendsJSON), ttl)
	if err != nil {
		return fmt.Errorf("failed to cache friends: %w", err)
	}
	log.Printf("✅ Cached friends list for user %s", userID)

	// Retrieve from cache
	cachedData, err := cache.Get(ctx, cacheKey)
	if err != nil {
		return fmt.Errorf("failed to get cached friends: %w", err)
	}

	var cachedFriends []*domain.Friend
	err = json.Unmarshal([]byte(cachedData), &cachedFriends)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cached friends: %w", err)
	}

	if len(cachedFriends) != len(friends) {
		return fmt.Errorf("cached friends count mismatch: expected %d, got %d", len(friends), len(cachedFriends))
	}
	log.Printf("✅ Retrieved %d friends from cache", len(cachedFriends))

	// Check TTL
	ttlRemaining, err := cache.GetTTL(ctx, cacheKey)
	if err != nil {
		return fmt.Errorf("failed to get TTL: %w", err)
	}
	log.Printf("✅ Cache TTL remaining: %v", ttlRemaining)

	return nil
}

func testFriendCountCaching(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test 2: Friend Count Caching Pattern ---")

	userID := "user456"
	cacheKey := cacheTypes.CacheKeyFriendCount.Format(userID)
	
	// Simulate friend count
	friendCount := 42

	// Cache the friend count
	countJSON, err := json.Marshal(friendCount)
	if err != nil {
		return fmt.Errorf("failed to marshal friend count: %w", err)
	}

	ttl := cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyFriendCount)
	err = cache.Set(ctx, cacheKey, string(countJSON), ttl)
	if err != nil {
		return fmt.Errorf("failed to cache friend count: %w", err)
	}
	log.Printf("✅ Cached friend count %d for user %s", friendCount, userID)

	// Retrieve from cache
	cachedData, err := cache.Get(ctx, cacheKey)
	if err != nil {
		return fmt.Errorf("failed to get cached friend count: %w", err)
	}

	var cachedCount int
	err = json.Unmarshal([]byte(cachedData), &cachedCount)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cached friend count: %w", err)
	}

	if cachedCount != friendCount {
		return fmt.Errorf("cached friend count mismatch: expected %d, got %d", friendCount, cachedCount)
	}
	log.Printf("✅ Retrieved friend count %d from cache", cachedCount)

	return nil
}

func testFriendshipCaching(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test 3: Friendship Status Caching Pattern ---")

	userID1 := "user789"
	userID2 := "user101"
	cacheKey := cacheTypes.CacheKeyFriendship.Format(userID1, userID2)
	
	// Simulate friendship status
	areFriends := true

	// Cache the friendship status
	statusJSON, err := json.Marshal(areFriends)
	if err != nil {
		return fmt.Errorf("failed to marshal friendship status: %w", err)
	}

	ttl := cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyFriendship)
	err = cache.Set(ctx, cacheKey, string(statusJSON), ttl)
	if err != nil {
		return fmt.Errorf("failed to cache friendship status: %w", err)
	}
	log.Printf("✅ Cached friendship status %v for %s <-> %s", areFriends, userID1, userID2)

	// Retrieve from cache
	cachedData, err := cache.Get(ctx, cacheKey)
	if err != nil {
		return fmt.Errorf("failed to get cached friendship status: %w", err)
	}

	var cachedStatus bool
	err = json.Unmarshal([]byte(cachedData), &cachedStatus)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cached friendship status: %w", err)
	}

	if cachedStatus != areFriends {
		return fmt.Errorf("cached friendship status mismatch: expected %v, got %v", areFriends, cachedStatus)
	}
	log.Printf("✅ Retrieved friendship status %v from cache", cachedStatus)

	return nil
}

func testCacheInvalidation(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test 4: Cache Invalidation Pattern ---")

	userID := "user999"
	friendKeys := []string{
		cacheTypes.CacheKeyUserFriends.Format(userID),
		cacheTypes.CacheKeyFriendCount.Format(userID),
	}

	// Set some cache entries
	for i, key := range friendKeys {
		value := fmt.Sprintf("test_value_%d", i)
		err := cache.Set(ctx, key, value, 5*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to set cache entry %s: %w", key, err)
		}
		log.Printf("✅ Set cache entry: %s", key)
	}

	// Verify cache entries exist
	for _, key := range friendKeys {
		_, err := cache.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("cache entry %s should exist: %w", key, err)
		}
	}
	log.Printf("✅ Verified all cache entries exist")

	// Invalidate cache entries (simulate friend relationship change)
	deletedCount, err := cache.Delete(ctx, friendKeys...)
	if err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}
	log.Printf("✅ Invalidated %d cache entries", deletedCount)

	// Verify cache entries are gone
	for _, key := range friendKeys {
		_, err := cache.Get(ctx, key)
		if err == nil {
			return fmt.Errorf("cache entry %s should be invalidated", key)
		}
	}
	log.Printf("✅ Verified all cache entries are invalidated")

	return nil
}

func testCachePerformance(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test 5: Cache Performance Measurement ---")

	// Test cache write performance
	writeStart := time.Now()
	numWrites := 100
	
	for i := 0; i < numWrites; i++ {
		key := fmt.Sprintf("perf:test:%d", i)
		value := fmt.Sprintf("performance_test_value_%d", i)
		err := cache.Set(ctx, key, value, 1*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to write cache entry %d: %w", i, err)
		}
	}
	
	writeElapsed := time.Since(writeStart)
	writeAvg := writeElapsed / time.Duration(numWrites)
	log.Printf("✅ Write performance: %d writes in %v (avg: %v per write)", numWrites, writeElapsed, writeAvg)

	// Test cache read performance
	readStart := time.Now()
	numReads := 100
	
	for i := 0; i < numReads; i++ {
		key := fmt.Sprintf("perf:test:%d", i)
		_, err := cache.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to read cache entry %d: %w", i, err)
		}
	}
	
	readElapsed := time.Since(readStart)
	readAvg := readElapsed / time.Duration(numReads)
	log.Printf("✅ Read performance: %d reads in %v (avg: %v per read)", numReads, readElapsed, readAvg)

	// Test cache metrics (if available)
	if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
		metrics := cacheServiceImpl.GetMetrics()
		log.Printf("✅ Cache Metrics:")
		log.Printf("   Hits: %d", metrics.Hits)
		log.Printf("   Misses: %d", metrics.Misses)
		log.Printf("   Hit Rate: %.2f%%", metrics.HitRate)
		log.Printf("   Operations: %d", metrics.Operations)
		log.Printf("   Errors: %d", metrics.Errors)
		log.Printf("   Avg Latency: %.2f ms", metrics.AvgLatency)
		
		// Performance assessment
		if metrics.AvgLatency > 10.0 {
			log.Printf("⚠️  Warning: Average latency is high (%.2f ms)", metrics.AvgLatency)
		} else {
			log.Printf("✅ Average latency is good (%.2f ms)", metrics.AvgLatency)
		}
		
		if metrics.HitRate > 50.0 {
			log.Printf("✅ Good hit rate (%.2f%%)", metrics.HitRate)
		} else if metrics.HitRate > 0 {
			log.Printf("⚠️  Hit rate could be improved (%.2f%%)", metrics.HitRate)
		}
	}

	// Cleanup performance test entries
	var keysToDelete []string
	for i := 0; i < numWrites; i++ {
		keysToDelete = append(keysToDelete, fmt.Sprintf("perf:test:%d", i))
	}
	_, err := cache.Delete(ctx, keysToDelete...)
	if err != nil {
		log.Printf("Warning: Failed to cleanup performance test entries: %v", err)
	}

	return nil
}