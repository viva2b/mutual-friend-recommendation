package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	cacheService "mutual-friend/internal/cache"
	"mutual-friend/internal/domain"
	"mutual-friend/pkg/config"
	cacheTypes "mutual-friend/pkg/cache"
)

type PerformanceResult struct {
	Operation    string        `json:"operation"`
	TotalOps     int           `json:"total_ops"`
	Duration     time.Duration `json:"duration"`
	AvgLatency   time.Duration `json:"avg_latency"`
	MinLatency   time.Duration `json:"min_latency"`
	MaxLatency   time.Duration `json:"max_latency"`
	Throughput   float64       `json:"throughput"` // ops/sec
	SuccessRate  float64       `json:"success_rate"`
	MemoryUsage  uint64        `json:"memory_usage"` // bytes
	Errors       int           `json:"errors"`
}

type LoadTestConfig struct {
	Concurrency     int           `json:"concurrency"`
	TotalRequests   int           `json:"total_requests"`
	RequestTimeout  time.Duration `json:"request_timeout"`
	DataSize        int           `json:"data_size"` // bytes
	TTL             time.Duration `json:"ttl"`
}

func main() {
	log.Println("Starting Comprehensive Cache Performance Test (Quiet Mode)...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create cache service with quiet Redis client
	factory := cacheService.NewFactory(cfg)
	cacheImpl, err := factory.CreateCacheService()
	if err != nil {
		log.Fatalf("Failed to create cache service: %v", err)
	}
	defer cacheImpl.Close()

	// Initialize performance tests
	if err := runComprehensivePerformanceTests(cacheImpl); err != nil {
		log.Printf("Performance tests failed: %v", err)
	} else {
		log.Println("✅ Comprehensive cache performance tests completed successfully!")
	}

	log.Println("Cache Performance Test completed!")
}

func runComprehensivePerformanceTests(cache cacheTypes.Cache) error {
	ctx := context.Background()
	
	log.Println("\n=== Comprehensive Cache Performance Analysis ===")

	// Test Suite 1: Basic Operations Performance
	if err := testBasicOperationsPerformance(ctx, cache); err != nil {
		return fmt.Errorf("basic operations performance test failed: %w", err)
	}

	// Test Suite 2: Concurrent Load Testing
	if err := testConcurrentLoadPerformance(ctx, cache); err != nil {
		return fmt.Errorf("concurrent load performance test failed: %w", err)
	}

	// Test Suite 3: Data Size Impact Analysis
	if err := testDataSizeImpact(ctx, cache); err != nil {
		return fmt.Errorf("data size impact test failed: %w", err)
	}

	// Test Suite 4: TTL Performance Impact
	if err := testTTLPerformanceImpact(ctx, cache); err != nil {
		return fmt.Errorf("TTL performance impact test failed: %w", err)
	}

	// Test Suite 5: Cache Hit/Miss Ratio Analysis
	if err := testCacheHitMissRatio(ctx, cache); err != nil {
		return fmt.Errorf("cache hit/miss ratio test failed: %w", err)
	}

	// Test Suite 6: Memory Usage Analysis
	if err := testMemoryUsage(ctx, cache); err != nil {
		return fmt.Errorf("memory usage test failed: %w", err)
	}

	// Test Suite 7: Friend Service Integration Performance
	if err := testFriendServiceIntegrationPerformance(ctx, cache); err != nil {
		return fmt.Errorf("friend service integration performance test failed: %w", err)
	}

	return nil
}

func testBasicOperationsPerformance(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test Suite 1: Basic Operations Performance ---")

	operations := []struct {
		name string
		test func() error
	}{
		{
			name: "SET_PERFORMANCE",
			test: func() error {
				return benchmarkSetOperations(ctx, cache, 500)
			},
		},
		{
			name: "GET_PERFORMANCE",
			test: func() error {
				return benchmarkGetOperations(ctx, cache, 500)
			},
		},
		{
			name: "DELETE_PERFORMANCE", 
			test: func() error {
				return benchmarkDeleteOperations(ctx, cache, 500)
			},
		},
	}

	for _, op := range operations {
		log.Printf("\n  Running %s...", op.name)
		start := time.Now()
		err := op.test()
		elapsed := time.Since(start)
		
		if err != nil {
			log.Printf("  ❌ %s failed: %v", op.name, err)
			return err
		}
		log.Printf("  ✅ %s completed in %v", op.name, elapsed)
	}

	return nil
}

func benchmarkSetOperations(ctx context.Context, cache cacheTypes.Cache, count int) error {
	var totalLatency time.Duration
	var minLatency, maxLatency time.Duration
	errors := 0

	for i := 0; i < count; i++ {
		key := fmt.Sprintf("perf:set:%d", i)
		value := fmt.Sprintf("performance_test_value_%d_with_some_extra_data", i)
		
		start := time.Now()
		
		// Use the cache service SetJSON method instead of Set to reduce logging
		if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
			err := cacheServiceImpl.SetJSON(ctx, key, value, 5*time.Minute)
			latency := time.Since(start)
			
			if err != nil {
				errors++
				continue
			}
			
			totalLatency += latency
			if i == 0 || latency < minLatency {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
		} else {
			// Fallback to normal Set method
			err := cache.Set(ctx, key, value, 5*time.Minute)
			latency := time.Since(start)
			
			if err != nil {
				errors++
				continue
			}
			
			totalLatency += latency
			if i == 0 || latency < minLatency {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
		}
	}

	successCount := count - errors
	avgLatency := totalLatency / time.Duration(successCount)
	successRate := float64(successCount) / float64(count) * 100

	log.Printf("    SET Operations: %d total, %.1f%% success rate", count, successRate)
	log.Printf("    Latency: avg=%v, min=%v, max=%v", avgLatency, minLatency, maxLatency)
	
	return nil
}

func benchmarkGetOperations(ctx context.Context, cache cacheTypes.Cache, count int) error {
	// First, populate cache with test data (quietly)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("perf:get:%d", i)
		value := fmt.Sprintf("test_value_%d", i)
		if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
			cacheServiceImpl.SetJSON(ctx, key, value, 5*time.Minute)
		} else {
			cache.Set(ctx, key, value, 5*time.Minute)
		}
	}

	var totalLatency time.Duration
	var minLatency, maxLatency time.Duration
	errors := 0

	for i := 0; i < count; i++ {
		key := fmt.Sprintf("perf:get:%d", i)
		
		start := time.Now()
		_, err := cache.Get(ctx, key)
		latency := time.Since(start)
		
		if err != nil {
			errors++
			continue
		}

		totalLatency += latency
		if i == 0 || latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}
	}

	successCount := count - errors
	avgLatency := totalLatency / time.Duration(successCount)
	successRate := float64(successCount) / float64(count) * 100

	log.Printf("    GET Operations: %d total, %.1f%% success rate", count, successRate)
	log.Printf("    Latency: avg=%v, min=%v, max=%v", avgLatency, minLatency, maxLatency)
	
	return nil
}

func benchmarkDeleteOperations(ctx context.Context, cache cacheTypes.Cache, count int) error {
	// First, populate cache with test data (quietly)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("perf:delete:%d", i)
		value := fmt.Sprintf("test_value_%d", i)
		if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
			cacheServiceImpl.SetJSON(ctx, key, value, 5*time.Minute)
		} else {
			cache.Set(ctx, key, value, 5*time.Minute)
		}
	}

	var totalLatency time.Duration
	var minLatency, maxLatency time.Duration
	errors := 0

	for i := 0; i < count; i++ {
		key := fmt.Sprintf("perf:delete:%d", i)
		
		start := time.Now()
		_, err := cache.Delete(ctx, key)
		latency := time.Since(start)
		
		if err != nil {
			errors++
			continue
		}

		totalLatency += latency
		if i == 0 || latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}
	}

	successCount := count - errors
	avgLatency := totalLatency / time.Duration(successCount)
	successRate := float64(successCount) / float64(count) * 100

	log.Printf("    DELETE Operations: %d total, %.1f%% success rate", count, successRate)
	log.Printf("    Latency: avg=%v, min=%v, max=%v", avgLatency, minLatency, maxLatency)
	
	return nil
}

func testConcurrentLoadPerformance(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test Suite 2: Concurrent Load Testing ---")

	testConfigs := []LoadTestConfig{
		{Concurrency: 10, TotalRequests: 500, RequestTimeout: 1 * time.Second, DataSize: 100, TTL: 5 * time.Minute},
		{Concurrency: 50, TotalRequests: 2500, RequestTimeout: 1 * time.Second, DataSize: 100, TTL: 5 * time.Minute},
		{Concurrency: 100, TotalRequests: 5000, RequestTimeout: 1 * time.Second, DataSize: 100, TTL: 5 * time.Minute},
	}

	for _, config := range testConfigs {
		log.Printf("\n  Testing concurrent load: %d goroutines, %d requests", config.Concurrency, config.TotalRequests)
		
		result, err := runConcurrentLoadTest(ctx, cache, config)
		if err != nil {
			return fmt.Errorf("concurrent load test failed: %w", err)
		}
		
		log.Printf("    Throughput: %.1f ops/sec", result.Throughput)
		log.Printf("    Success Rate: %.1f%%", result.SuccessRate)
		log.Printf("    Avg Latency: %v", result.AvgLatency)
		log.Printf("    Memory Usage: %.2f MB", float64(result.MemoryUsage)/1024/1024)
	}

	return nil
}

func runConcurrentLoadTest(ctx context.Context, cache cacheTypes.Cache, config LoadTestConfig) (*PerformanceResult, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	requestsPerWorker := config.TotalRequests / config.Concurrency
	results := make([]time.Duration, 0, config.TotalRequests)
	errors := 0
	
	// Memory usage before test
	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)
	
	start := time.Now()
	
	// Launch concurrent workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerWorker; j++ {
				key := fmt.Sprintf("load:test:%d:%d", workerID, j)
				value := generateTestData(config.DataSize)
				
				opStart := time.Now()
				// Use quiet cache operations
				var err error
				if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
					err = cacheServiceImpl.SetJSON(ctx, key, value, config.TTL)
				} else {
					err = cache.Set(ctx, key, value, config.TTL)
				}
				latency := time.Since(opStart)
				
				mu.Lock()
				if err != nil {
					errors++
				} else {
					results = append(results, latency)
				}
				mu.Unlock()
			}
		}(i)
	}
	
	wg.Wait()
	totalDuration := time.Since(start)
	
	// Memory usage after test
	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)
	
	// Calculate statistics
	successCount := len(results)
	successRate := float64(successCount) / float64(config.TotalRequests) * 100
	throughput := float64(successCount) / totalDuration.Seconds()
	
	var totalLatency time.Duration
	var minLatency, maxLatency time.Duration
	
	if len(results) > 0 {
		minLatency = results[0]
		maxLatency = results[0]
		
		for _, latency := range results {
			totalLatency += latency
			if latency < minLatency {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
		}
	}
	
	avgLatency := totalLatency / time.Duration(len(results))
	memoryUsage := memAfter.Alloc - memBefore.Alloc
	
	return &PerformanceResult{
		Operation:   "CONCURRENT_LOAD",
		TotalOps:    config.TotalRequests,
		Duration:    totalDuration,
		AvgLatency:  avgLatency,
		MinLatency:  minLatency,
		MaxLatency:  maxLatency,
		Throughput:  throughput,
		SuccessRate: successRate,
		MemoryUsage: memoryUsage,
		Errors:      errors,
	}, nil
}

func testDataSizeImpact(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test Suite 3: Data Size Impact Analysis ---")

	dataSizes := []int{100, 1024, 10240, 102400} // 100B, 1KB, 10KB, 100KB
	operationCount := 50

	for _, size := range dataSizes {
		log.Printf("\n  Testing data size: %d bytes", size)
		
		// Test SET performance with different data sizes
		setLatencies := make([]time.Duration, 0, operationCount)
		getLatencies := make([]time.Duration, 0, operationCount)
		
		testData := generateTestData(size)
		
		// Benchmark SET operations
		for i := 0; i < operationCount; i++ {
			key := fmt.Sprintf("size:test:set:%d:%d", size, i)
			
			start := time.Now()
			var err error
			if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
				err = cacheServiceImpl.SetJSON(ctx, key, testData, 5*time.Minute)
			} else {
				err = cache.Set(ctx, key, testData, 5*time.Minute)
			}
			latency := time.Since(start)
			
			if err == nil {
				setLatencies = append(setLatencies, latency)
			}
		}
		
		// Benchmark GET operations
		for i := 0; i < operationCount; i++ {
			key := fmt.Sprintf("size:test:get:%d:%d", size, i)
			
			// First set the data (quietly)
			if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
				cacheServiceImpl.SetJSON(ctx, key, testData, 5*time.Minute)
			} else {
				cache.Set(ctx, key, testData, 5*time.Minute)
			}
			
			start := time.Now()
			_, err := cache.Get(ctx, key)
			latency := time.Since(start)
			
			if err == nil {
				getLatencies = append(getLatencies, latency)
			}
		}
		
		// Calculate averages
		setAvg := calculateAverageLatency(setLatencies)
		getAvg := calculateAverageLatency(getLatencies)
		
		log.Printf("    SET avg latency: %v (%d ops)", setAvg, len(setLatencies))
		log.Printf("    GET avg latency: %v (%d ops)", getAvg, len(getLatencies))
		log.Printf("    Data overhead: SET=%.2fμs/byte, GET=%.2fμs/byte", 
			float64(setAvg.Microseconds())/float64(size),
			float64(getAvg.Microseconds())/float64(size))
	}

	return nil
}

func testTTLPerformanceImpact(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test Suite 4: TTL Performance Impact Analysis ---")

	ttlValues := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		24 * time.Hour,
	}

	operationCount := 50
	testData := generateTestData(1024) // 1KB test data

	for _, ttl := range ttlValues {
		log.Printf("\n  Testing TTL: %v", ttl)
		
		var totalLatency time.Duration
		successCount := 0
		
		for i := 0; i < operationCount; i++ {
			key := fmt.Sprintf("ttl:test:%v:%d", ttl, i)
			
			start := time.Now()
			var err error
			if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
				err = cacheServiceImpl.SetJSON(ctx, key, testData, ttl)
			} else {
				err = cache.Set(ctx, key, testData, ttl)
			}
			latency := time.Since(start)
			
			if err == nil {
				totalLatency += latency
				successCount++
			}
		}
		
		if successCount > 0 {
			avgLatency := totalLatency / time.Duration(successCount)
			log.Printf("    Avg latency: %v (%d successful ops)", avgLatency, successCount)
		}
	}

	// Test TTL expiration behavior
	log.Printf("\n  Testing TTL expiration behavior...")
	shortTTL := 2 * time.Second
	key := "ttl:expiration:test"
	value := "test_value"
	
	// Set with short TTL
	var err error
	if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
		err = cacheServiceImpl.SetJSON(ctx, key, value, shortTTL)
	} else {
		err = cache.Set(ctx, key, value, shortTTL)
	}
	if err != nil {
		return fmt.Errorf("failed to set key with short TTL: %w", err)
	}
	
	// Immediate read - should succeed
	_, err = cache.Get(ctx, key)
	if err != nil {
		log.Printf("    ⚠️  Immediate read failed: %v", err)
	} else {
		log.Printf("    ✅ Immediate read succeeded")
	}
	
	// Wait for expiration
	time.Sleep(shortTTL + 500*time.Millisecond)
	
	// Read after expiration - should fail
	_, err = cache.Get(ctx, key)
	if err != nil {
		log.Printf("    ✅ Read after expiration correctly failed: %v", err)
	} else {
		log.Printf("    ⚠️  Read after expiration unexpectedly succeeded")
	}

	return nil
}

func testCacheHitMissRatio(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test Suite 5: Cache Hit/Miss Ratio Analysis ---")

	// Populate cache with known data
	populateCount := 50
	testKeys := make([]string, populateCount)
	
	for i := 0; i < populateCount; i++ {
		key := fmt.Sprintf("hit:miss:test:%d", i)
		value := fmt.Sprintf("test_value_%d", i)
		testKeys[i] = key
		
		if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
			err := cacheServiceImpl.SetJSON(ctx, key, value, 5*time.Minute)
			if err != nil {
				return fmt.Errorf("failed to populate cache: %w", err)
			}
		} else {
			err := cache.Set(ctx, key, value, 5*time.Minute)
			if err != nil {
				return fmt.Errorf("failed to populate cache: %w", err)
			}
		}
	}
	
	log.Printf("  Populated cache with %d entries", populateCount)

	// Test different hit/miss scenarios
	scenarios := []struct {
		name        string
		hitRatio    float64 // 0.0 to 1.0
		totalReads  int
	}{
		{"High Hit Ratio", 0.9, 500},
		{"Medium Hit Ratio", 0.5, 500},
		{"Low Hit Ratio", 0.1, 500},
	}

	for _, scenario := range scenarios {
		log.Printf("\n  Scenario: %s (target hit ratio: %.1f%%)", scenario.name, scenario.hitRatio*100)
		
		hits := 0
		misses := 0
		var hitLatencyTotal, missLatencyTotal time.Duration
		
		for i := 0; i < scenario.totalReads; i++ {
			var key string
			
			// Decide whether this should be a hit or miss based on target ratio
			if float64(i)/float64(scenario.totalReads) < scenario.hitRatio {
				// Should be a hit - use existing key
				key = testKeys[i%len(testKeys)]
			} else {
				// Should be a miss - use non-existent key
				key = fmt.Sprintf("nonexistent:key:%d", i)
			}
			
			start := time.Now()
			_, err := cache.Get(ctx, key)
			latency := time.Since(start)
			
			if err == nil {
				hits++
				hitLatencyTotal += latency
			} else {
				misses++
				missLatencyTotal += latency
			}
		}
		
		actualHitRatio := float64(hits) / float64(scenario.totalReads)
		avgHitLatency := time.Duration(0)
		avgMissLatency := time.Duration(0)
		
		if hits > 0 {
			avgHitLatency = hitLatencyTotal / time.Duration(hits)
		}
		if misses > 0 {
			avgMissLatency = missLatencyTotal / time.Duration(misses)
		}
		
		log.Printf("    Actual hit ratio: %.1f%% (%d hits, %d misses)", actualHitRatio*100, hits, misses)
		log.Printf("    Avg hit latency: %v", avgHitLatency)
		log.Printf("    Avg miss latency: %v", avgMissLatency)
		
		if hits > 0 && misses > 0 {
			latencyDiff := avgMissLatency - avgHitLatency
			log.Printf("    Miss penalty: %v (%.1fx slower)", latencyDiff, float64(avgMissLatency)/float64(avgHitLatency))
		}
	}

	return nil
}

func testMemoryUsage(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test Suite 6: Memory Usage Analysis ---")

	// Memory usage before operations
	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)
	
	log.Printf("  Initial memory usage: %.2f MB", float64(memBefore.Alloc)/1024/1024)

	// Test memory usage with different data sizes and quantities
	testCases := []struct {
		entryCount int
		entrySize  int
		name       string
	}{
		{500, 100, "Small entries"},
		{250, 1024, "Medium entries"},
		{50, 10240, "Large entries"},
	}

	for _, testCase := range testCases {
		log.Printf("\n  Testing %s: %d entries of %d bytes each", testCase.name, testCase.entryCount, testCase.entrySize)
		
		var memBeforeTest runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memBeforeTest)
		
		// Populate cache
		testData := generateTestData(testCase.entrySize)
		for i := 0; i < testCase.entryCount; i++ {
			key := fmt.Sprintf("memory:test:%s:%d", testCase.name, i)
			if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
				err := cacheServiceImpl.SetJSON(ctx, key, testData, 10*time.Minute)
				if err != nil {
					log.Printf("    Warning: Failed to set entry %d: %v", i, err)
				}
			} else {
				err := cache.Set(ctx, key, testData, 10*time.Minute)
				if err != nil {
					log.Printf("    Warning: Failed to set entry %d: %v", i, err)
				}
			}
		}
		
		var memAfterTest runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memAfterTest)
		
		memoryIncrease := memAfterTest.Alloc - memBeforeTest.Alloc
		expectedDataSize := uint64(testCase.entryCount * testCase.entrySize)
		overhead := float64(memoryIncrease) / float64(expectedDataSize)
		
		log.Printf("    Memory increase: %.2f MB", float64(memoryIncrease)/1024/1024)
		log.Printf("    Expected data size: %.2f MB", float64(expectedDataSize)/1024/1024)
		log.Printf("    Memory overhead: %.1fx", overhead)
		
		// Clean up
		for i := 0; i < testCase.entryCount; i++ {
			key := fmt.Sprintf("memory:test:%s:%d", testCase.name, i)
			cache.Delete(ctx, key)
		}
	}

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)
	
	log.Printf("\n  Final memory usage: %.2f MB", float64(memAfter.Alloc)/1024/1024)
	log.Printf("  Net memory change: %.2f MB", float64(int64(memAfter.Alloc)-int64(memBefore.Alloc))/1024/1024)

	return nil
}

func testFriendServiceIntegrationPerformance(ctx context.Context, cache cacheTypes.Cache) error {
	log.Println("\n--- Test Suite 7: Friend Service Integration Performance ---")

	// Simulate friend service cache patterns
	userIDs := []string{"user1", "user2", "user3", "user4", "user5"}
	
	// Test friend list caching patterns
	log.Printf("\n  Testing friend list caching patterns...")
	
	for _, userID := range userIDs {
		// Simulate friend list data
		friends := make([]*domain.Friend, 10)
		for i := 0; i < 10; i++ {
			friends[i] = &domain.Friend{
				UserID:    userID,
				FriendID:  fmt.Sprintf("friend%d", i),
				Status:    domain.FriendStatusAccepted,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		}
		
		// Cache friend list
		cacheKey := cacheTypes.CacheKeyUserFriends.Format(userID)
		friendsJSON, _ := json.Marshal(friends)
		ttl := cacheTypes.GetDefaultTTL(cacheTypes.CacheKeyUserFriends)
		
		start := time.Now()
		var err error
		if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
			err = cacheServiceImpl.SetJSON(ctx, cacheKey, string(friendsJSON), ttl)
		} else {
			err = cache.Set(ctx, cacheKey, string(friendsJSON), ttl)
		}
		setLatency := time.Since(start)
		
		if err != nil {
			log.Printf("    ❌ Failed to cache friends for %s: %v", userID, err)
			continue
		}
		
		// Read friend list from cache
		start = time.Now()
		cachedData, err := cache.Get(ctx, cacheKey)
		getLatency := time.Since(start)
		
		if err != nil {
			log.Printf("    ❌ Failed to read cached friends for %s: %v", userID, err)
			continue
		}
		
		var cachedFriends []*domain.Friend
		err = json.Unmarshal([]byte(cachedData), &cachedFriends)
		if err != nil {
			log.Printf("    ❌ Failed to unmarshal cached friends for %s: %v", userID, err)
			continue
		}
		
		log.Printf("    ✅ %s: SET=%v, GET=%v, friends=%d", userID, setLatency, getLatency, len(cachedFriends))
	}

	// Test cache invalidation patterns
	log.Printf("\n  Testing cache invalidation patterns...")
	
	userID := "user1"
	keysToInvalidate := []string{
		cacheTypes.CacheKeyUserFriends.Format(userID),
		cacheTypes.CacheKeyFriendCount.Format(userID),
		cacheTypes.CacheKeyFriendship.Format(userID, "user2"),
		cacheTypes.CacheKeyMutualFriends.Format(userID, "user3"),
	}
	
	// First populate all cache keys
	for _, key := range keysToInvalidate {
		if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
			err := cacheServiceImpl.SetJSON(ctx, key, "test_value", 5*time.Minute)
			if err != nil {
				log.Printf("    Warning: Failed to populate cache key %s: %v", key, err)
			}
		} else {
			err := cache.Set(ctx, key, "test_value", 5*time.Minute)
			if err != nil {
				log.Printf("    Warning: Failed to populate cache key %s: %v", key, err)
			}
		}
	}
	
	// Test bulk invalidation performance
	start := time.Now()
	deletedCount, err := cache.Delete(ctx, keysToInvalidate...)
	invalidationLatency := time.Since(start)
	
	if err != nil {
		log.Printf("    ❌ Cache invalidation failed: %v", err)
	} else {
		log.Printf("    ✅ Invalidated %d keys in %v", deletedCount, invalidationLatency)
	}

	// Test cache metrics if available
	log.Printf("\n  Cache performance summary...")
	if cacheServiceImpl, ok := cache.(*cacheService.Service); ok {
		metrics := cacheServiceImpl.GetMetrics()
		log.Printf("    Total operations: %d", metrics.Operations)
		log.Printf("    Cache hits: %d", metrics.Hits)
		log.Printf("    Cache misses: %d", metrics.Misses)
		log.Printf("    Hit rate: %.2f%%", metrics.HitRate)
		log.Printf("    Error count: %d", metrics.Errors)
		log.Printf("    Average latency: %.2f ms", metrics.AvgLatency)
		
		// Performance assessment
		if metrics.AvgLatency <= 1.0 {
			log.Printf("    ✅ Excellent performance (avg latency ≤ 1ms)")
		} else if metrics.AvgLatency <= 5.0 {
			log.Printf("    ✅ Good performance (avg latency ≤ 5ms)")
		} else if metrics.AvgLatency <= 10.0 {
			log.Printf("    ⚠️  Acceptable performance (avg latency ≤ 10ms)")
		} else {
			log.Printf("    ❌ Performance needs improvement (avg latency > 10ms)")
		}
		
		if metrics.HitRate >= 80.0 {
			log.Printf("    ✅ Excellent hit rate (≥ 80%%)")
		} else if metrics.HitRate >= 60.0 {
			log.Printf("    ✅ Good hit rate (≥ 60%%)")
		} else if metrics.HitRate >= 40.0 {
			log.Printf("    ⚠️  Moderate hit rate (≥ 40%%)")
		} else {
			log.Printf("    ❌ Low hit rate (< 40%%) - consider cache strategy review")
		}
	}

	return nil
}

// Helper functions

func generateTestData(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}

func calculateAverageLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	
	return total / time.Duration(len(latencies))
}