package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"mutual-friend/internal/domain"
	"mutual-friend/internal/service"
	"mutual-friend/pkg/cache"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/redis"
)

// IntegrationTestSuite 통합 테스트 스위트
type IntegrationTestSuite struct {
	suite.Suite
	friendService *service.FriendService
	userService   *service.UserService
	eventService  *service.EventService
	cacheService  cache.Cache
	
	// 테스트 인프라
	config *config.Config
	ctx    context.Context
	
	// 성능 메트릭 수집
	metrics *TestMetrics
}

// TestMetrics 테스트 성능 메트릭
type TestMetrics struct {
	mu                sync.RWMutex
	ResponseTimes     []time.Duration
	CacheHits         int64
	CacheMisses       int64
	DatabaseCalls     int64
	ErrorCount        int64
	ConcurrencyErrors int64
	ConsistencyErrors int64
}

func (tm *TestMetrics) RecordResponseTime(duration time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.ResponseTimes = append(tm.ResponseTimes, duration)
}

func (tm *TestMetrics) IncrementCacheHit() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.CacheHits++
}

func (tm *TestMetrics) IncrementCacheMiss() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.CacheMisses++
}

func (tm *TestMetrics) IncrementDatabaseCall() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.DatabaseCalls++
}

func (tm *TestMetrics) IncrementError() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.ErrorCount++
}

func (tm *TestMetrics) IncrementConcurrencyError() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.ConcurrencyErrors++
}

func (tm *TestMetrics) IncrementConsistencyError() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.ConsistencyErrors++
}

func (tm *TestMetrics) GetCacheHitRate() float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	total := tm.CacheHits + tm.CacheMisses
	if total == 0 {
		return 0
	}
	return float64(tm.CacheHits) / float64(total) * 100
}

func (tm *TestMetrics) GetAverageResponseTime() time.Duration {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	if len(tm.ResponseTimes) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, rt := range tm.ResponseTimes {
		total += rt
	}
	return total / time.Duration(len(tm.ResponseTimes))
}

// SetupSuite 테스트 환경 초기화
func (suite *IntegrationTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.metrics = &TestMetrics{}
	
	// 설정 로드
	cfg, err := config.LoadConfig("../../configs/config.yaml")
	require.NoError(suite.T(), err)
	suite.config = cfg
	
	// Redis 클라이언트 초기화
	redisClient, err := redis.NewClient(&redis.Config{
		Address:      cfg.Redis.Address,
		Password:     cfg.Redis.Password,
		Database:     cfg.Redis.Database,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		MaxRetries:   cfg.Redis.MaxRetries,
		DialTimeout:  time.Duration(cfg.Redis.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.Redis.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Redis.WriteTimeout) * time.Second,
	})
	require.NoError(suite.T(), err)
	
	// 캐시 서비스 초기화
	suite.cacheService, err = cache.NewService(redisClient)
	require.NoError(suite.T(), err)
	
	// 테스트용 서비스 초기화 (실제 구현은 나중에)
	// suite.friendService = service.NewFriendService(...)
	// suite.userService = service.NewUserService(...)
	// suite.eventService = service.NewEventService(...)
}

// TearDownSuite 테스트 환경 정리
func (suite *IntegrationTestSuite) TearDownSuite() {
	if suite.cacheService != nil {
		suite.cacheService.Close()
	}
}

// SetupTest 각 테스트 전 초기화
func (suite *IntegrationTestSuite) SetupTest() {
	// 캐시 플러시
	err := suite.cacheService.Flush(suite.ctx)
	require.NoError(suite.T(), err)
	
	// 메트릭 초기화
	suite.metrics = &TestMetrics{}
}

// TestPerformanceBottleneckAnalysis 성능 병목 분석 테스트
func (suite *IntegrationTestSuite) TestPerformanceBottleneckAnalysis() {
	suite.T().Run("Redis Cache Performance", func(t *testing.T) {
		suite.testRedisCachePerformance(t)
	})
	
	suite.T().Run("DynamoDB Query Performance", func(t *testing.T) {
		suite.testDynamoDBQueryPerformance(t)
	})
	
	suite.T().Run("gRPC Layer Performance", func(t *testing.T) {
		suite.testGRPCLayerPerformance(t)
	})
	
	suite.T().Run("End-to-End Performance", func(t *testing.T) {
		suite.testEndToEndPerformance(t)
	})
}

// testRedisCachePerformance Redis 캐시 성능 테스트
func (suite *IntegrationTestSuite) testRedisCachePerformance(t *testing.T) {
	t.Log("🔍 Redis Cache Performance Analysis")
	
	// 다양한 데이터 크기로 테스트
	dataSizes := []struct {
		name string
		size int
		data map[string]interface{}
	}{
		{
			name: "Small Data (100B)",
			size: 100,
			data: map[string]interface{}{
				"user_id": "test_user_001",
				"friends": []string{"friend_001", "friend_002"},
			},
		},
		{
			name: "Medium Data (1KB)",
			size: 1024,
			data: generateTestData(1024),
		},
		{
			name: "Large Data (10KB)",
			size: 10240,
			data: generateTestData(10240),
		},
	}
	
	for _, ds := range dataSizes {
		t.Run(ds.name, func(t *testing.T) {
			key := fmt.Sprintf("perf_test_%s", ds.name)
			
			// SET 성능 측정
			start := time.Now()
			err := suite.cacheService.Set(suite.ctx, key, ds.data, 5*time.Minute)
			setDuration := time.Since(start)
			
			assert.NoError(t, err)
			t.Logf("SET %s: %v", ds.name, setDuration)
			
			// GET 성능 측정
			start = time.Now()
			result, err := suite.cacheService.Get(suite.ctx, key)
			getDuration := time.Since(start)
			
			assert.NoError(t, err)
			assert.NotEmpty(t, result)
			t.Logf("GET %s: %v", ds.name, getDuration)
			
			// 성능 임계값 검증
			assert.Less(t, setDuration, 5*time.Millisecond, "SET operation too slow")
			assert.Less(t, getDuration, 2*time.Millisecond, "GET operation too slow")
			
			suite.metrics.RecordResponseTime(setDuration)
			suite.metrics.RecordResponseTime(getDuration)
		})
	}
}

// testDynamoDBQueryPerformance DynamoDB 쿼리 성능 테스트
func (suite *IntegrationTestSuite) testDynamoDBQueryPerformance(t *testing.T) {
	t.Log("🔍 DynamoDB Query Performance Analysis")
	
	// 실제 DynamoDB 쿼리 성능 테스트는 서비스 레이어가 구현된 후 진행
	// 현재는 구조적 분석을 위한 로그 출력
	
	queryPatterns := []struct {
		name        string
		complexity  string
		expectedMax time.Duration
	}{
		{"Single Item Get", "Simple", 50 * time.Millisecond},
		{"Friends List Query", "Medium", 100 * time.Millisecond},
		{"Mutual Friends Calculation", "Complex", 300 * time.Millisecond},
		{"Batch Operations", "High", 500 * time.Millisecond},
	}
	
	for _, pattern := range queryPatterns {
		t.Logf("Query Pattern: %s (Complexity: %s, Expected Max: %v)", 
			pattern.name, pattern.complexity, pattern.expectedMax)
		
		// TODO: 실제 쿼리 실행 및 성능 측정
		// 현재는 테스트 구조만 정의
	}
}

// testGRPCLayerPerformance gRPC 레이어 성능 테스트
func (suite *IntegrationTestSuite) testGRPCLayerPerformance(t *testing.T) {
	t.Log("🔍 gRPC Layer Performance Analysis")
	
	// gRPC 클라이언트-서버 성능 테스트
	// 직렬화/역직렬화 성능
	// 네트워크 오버헤드 측정
	
	t.Log("gRPC Performance Test - Structure Defined")
	// TODO: 실제 gRPC 서버 연동 테스트
}

// testEndToEndPerformance 종단간 성능 테스트
func (suite *IntegrationTestSuite) testEndToEndPerformance(t *testing.T) {
	t.Log("🔍 End-to-End Performance Analysis")
	
	// 친구 추가 → 캐시 무효화 → 재조회 전체 플로우 성능 측정
	scenarios := []struct {
		name           string
		operations     int
		concurrency    int
		expectedMaxAvg time.Duration
	}{
		{"Light Load", 100, 5, 50 * time.Millisecond},
		{"Medium Load", 500, 20, 100 * time.Millisecond},
		{"Heavy Load", 1000, 50, 200 * time.Millisecond},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Running %s: %d operations with %d concurrent workers", 
				scenario.name, scenario.operations, scenario.concurrency)
			
			// TODO: 실제 부하 테스트 실행
			// 현재는 테스트 시나리오 정의
		})
	}
}

// TestConcurrencyIssues 동시성 문제 테스트
func (suite *IntegrationTestSuite) TestConcurrencyIssues() {
	suite.T().Run("Race Condition Detection", func(t *testing.T) {
		suite.testRaceConditions(t)
	})
	
	suite.T().Run("Cache Invalidation Consistency", func(t *testing.T) {
		suite.testCacheInvalidationConsistency(t)
	})
	
	suite.T().Run("Event Ordering Issues", func(t *testing.T) {
		suite.testEventOrderingIssues(t)
	})
	
	suite.T().Run("Thundering Herd Problem", func(t *testing.T) {
		suite.testThunderingHerdProblem(t)
	})
}

// testRaceConditions 레이스 컨디션 테스트
func (suite *IntegrationTestSuite) testRaceConditions(t *testing.T) {
	t.Log("🔍 Race Condition Detection Test")
	
	const numGoroutines = 100
	const userID = "race_test_user"
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	var mu sync.Mutex
	
	// 동시에 같은 키에 대해 캐시 작업 수행
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			
			key := fmt.Sprintf("friends:user:%s", userID)
			value := fmt.Sprintf("data_from_goroutine_%d", i)
			
			// 캐시에 데이터 저장
			err := suite.cacheService.Set(suite.ctx, key, value, time.Minute)
			
			mu.Lock()
			if err != nil {
				errorCount++
				suite.metrics.IncrementConcurrencyError()
			} else {
				successCount++
			}
			mu.Unlock()
			
			// 즉시 읽기 시도
			_, err = suite.cacheService.Get(suite.ctx, key)
			if err != nil {
				mu.Lock()
				errorCount++
				suite.metrics.IncrementConcurrencyError()
				mu.Unlock()
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Race Condition Test Results:")
	t.Logf("  Success: %d", successCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Error Rate: %.2f%%", float64(errorCount)/float64(numGoroutines*2)*100)
	
	// 에러율이 5% 이하여야 함
	assert.Less(t, errorCount, int64(numGoroutines/10), "Too many concurrency errors")
}

// testCacheInvalidationConsistency 캐시 무효화 일관성 테스트
func (suite *IntegrationTestSuite) testCacheInvalidationConsistency(t *testing.T) {
	t.Log("🔍 Cache Invalidation Consistency Test")
	
	userID := "consistency_test_user"
	friendID := "consistency_test_friend"
	
	// 1. 초기 데이터 캐싱
	cacheKey := fmt.Sprintf("friends:user:%s", userID)
	initialData := []string{friendID}
	
	err := suite.cacheService.Set(suite.ctx, cacheKey, initialData, time.Hour)
	require.NoError(t, err)
	
	// 2. 동시에 캐시 무효화와 새 데이터 삽입
	const numWorkers = 50
	var wg sync.WaitGroup
	var invalidationErrors int64
	var dataInconsistencies int64
	var mu sync.Mutex
	
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// 무효화 시도
			_, err := suite.cacheService.Delete(suite.ctx, cacheKey)
			if err != nil {
				mu.Lock()
				invalidationErrors++
				mu.Unlock()
				return
			}
			
			// 새 데이터 즉시 삽입
			newData := []string{fmt.Sprintf("friend_%d", workerID)}
			err = suite.cacheService.Set(suite.ctx, cacheKey, newData, time.Hour)
			if err != nil {
				mu.Lock()
				invalidationErrors++
				mu.Unlock()
				return
			}
			
			// 데이터 검증
			result, err := suite.cacheService.Get(suite.ctx, cacheKey)
			if err != nil {
				mu.Lock()
				invalidationErrors++
				mu.Unlock()
				return
			}
			
			var retrievedData []string
			err = json.Unmarshal([]byte(result), &retrievedData)
			if err != nil || len(retrievedData) == 0 {
				mu.Lock()
				dataInconsistencies++
				suite.metrics.IncrementConsistencyError()
				mu.Unlock()
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Cache Invalidation Consistency Results:")
	t.Logf("  Invalidation Errors: %d", invalidationErrors)
	t.Logf("  Data Inconsistencies: %d", dataInconsistencies)
	t.Logf("  Total Workers: %d", numWorkers)
	
	// 데이터 일관성 검증
	assert.Less(t, dataInconsistencies, int64(numWorkers/5), "Too many data inconsistencies")
}

// testEventOrderingIssues 이벤트 순서 문제 테스트
func (suite *IntegrationTestSuite) testEventOrderingIssues(t *testing.T) {
	t.Log("🔍 Event Ordering Issues Test")
	
	// 이벤트 시퀀스 시뮬레이션
	userID := "ordering_test_user"
	events := []struct {
		eventType string
		friendID  string
		timestamp time.Time
	}{
		{"FRIEND_ADDED", "friend_001", time.Now()},
		{"FRIEND_ADDED", "friend_002", time.Now().Add(1 * time.Millisecond)},
		{"FRIEND_REMOVED", "friend_001", time.Now().Add(2 * time.Millisecond)},
		{"FRIEND_ADDED", "friend_003", time.Now().Add(3 * time.Millisecond)},
	}
	
	// 순서가 섞인 이벤트 처리 시뮬레이션
	const numWorkers = 10
	var wg sync.WaitGroup
	var orderingErrors int64
	var mu sync.Mutex
	
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// 역순으로 이벤트 처리 시뮬레이션
			for j := len(events) - 1; j >= 0; j-- {
				event := events[j]
				
				// 캐시 키 생성
				cacheKey := fmt.Sprintf("events:%s:%d", userID, workerID)
				
				// 이벤트 순서 검증을 위한 데이터 저장
				eventData := map[string]interface{}{
					"event_type": event.eventType,
					"friend_id":  event.friendID,
					"timestamp":  event.timestamp.UnixNano(),
					"worker_id":  workerID,
				}
				
				err := suite.cacheService.Set(suite.ctx, cacheKey, eventData, time.Minute)
				if err != nil {
					mu.Lock()
					orderingErrors++
					mu.Unlock()
				}
				
				// 짧은 지연으로 경쟁 조건 유도
				time.Sleep(time.Microsecond * 100)
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Event Ordering Test Results:")
	t.Logf("  Ordering Errors: %d", orderingErrors)
	t.Logf("  Expected Events: %d", numWorkers*len(events))
	
	// 이벤트 순서 오류 검증
	assert.Less(t, orderingErrors, int64(numWorkers), "Too many event ordering errors")
}

// testThunderingHerdProblem 캐시 스탬피드 문제 테스트
func (suite *IntegrationTestSuite) testThunderingHerdProblem(t *testing.T) {
	t.Log("🔍 Thundering Herd Problem Test")
	
	const numClients = 100
	const popularKey = "popular_user_friends"
	
	// 인기 있는 키의 TTL을 매우 짧게 설정
	err := suite.cacheService.Set(suite.ctx, popularKey, "initial_data", 100*time.Millisecond)
	require.NoError(t, err)
	
	// TTL 만료 대기
	time.Sleep(150 * time.Millisecond)
	
	var wg sync.WaitGroup
	var cacheMisses int64
	var dbCalls int64
	var mu sync.Mutex
	
	start := time.Now()
	
	// 동시에 만료된 키에 대한 요청
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			// 캐시 조회 시도
			result, err := suite.cacheService.Get(suite.ctx, popularKey)
			
			if err != nil || result == "" {
				mu.Lock()
				cacheMisses++
				mu.Unlock()
				
				// 캐시 미스 시 "데이터베이스" 조회 시뮬레이션
				expensiveData := fmt.Sprintf("expensive_data_computed_by_client_%d", clientID)
				
				// 데이터베이스 호출 시뮬레이션 (지연 시간)
				time.Sleep(50 * time.Millisecond)
				
				mu.Lock()
				dbCalls++
				suite.metrics.IncrementDatabaseCall()
				mu.Unlock()
				
				// 캐시에 저장
				suite.cacheService.Set(suite.ctx, popularKey, expensiveData, time.Minute)
			} else {
				mu.Lock()
				suite.metrics.IncrementCacheHit()
				mu.Unlock()
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	t.Logf("Thundering Herd Test Results:")
	t.Logf("  Total Clients: %d", numClients)
	t.Logf("  Cache Misses: %d", cacheMisses)
	t.Logf("  Database Calls: %d", dbCalls)
	t.Logf("  Duration: %v", duration)
	t.Logf("  DB Call Rate: %.2f%%", float64(dbCalls)/float64(numClients)*100)
	
	// Thundering Herd 문제 검증
	// 이상적으로는 DB 호출이 1번만 발생해야 하지만, 현실적으로는 여러 번 발생 가능
	assert.Less(t, dbCalls, int64(numClients/2), "Too many database calls - thundering herd problem detected")
}

// TestDataConsistency 데이터 일관성 테스트
func (suite *IntegrationTestSuite) TestDataConsistency() {
	suite.T().Run("DynamoDB-Redis Consistency", func(t *testing.T) {
		suite.testDynamoDBRedisConsistency(t)
	})
	
	suite.T().Run("Cache-Database Sync", func(t *testing.T) {
		suite.testCacheDatabaseSync(t)
	})
	
	suite.T().Run("Event-Driven Consistency", func(t *testing.T) {
		suite.testEventDrivenConsistency(t)
	})
}

// testDynamoDBRedisConsistency DynamoDB-Redis 일관성 테스트
func (suite *IntegrationTestSuite) testDynamoDBRedisConsistency(t *testing.T) {
	t.Log("🔍 DynamoDB-Redis Consistency Test")
	
	// 실제 구현 시 DynamoDB와 Redis 간의 데이터 일관성 검증
	// 현재는 테스트 구조 정의
	
	testCases := []struct {
		name          string
		operation     string
		expectedState string
	}{
		{"Friend Addition", "ADD_FRIEND", "CONSISTENT"},
		{"Friend Removal", "REMOVE_FRIEND", "CONSISTENT"},
		{"Cache Invalidation", "INVALIDATE_CACHE", "EVENTUALLY_CONSISTENT"},
	}
	
	for _, tc := range testCases {
		t.Logf("Testing %s: %s -> %s", tc.name, tc.operation, tc.expectedState)
	}
}

// testCacheDatabaseSync 캐시-데이터베이스 동기화 테스트
func (suite *IntegrationTestSuite) testCacheDatabaseSync(t *testing.T) {
	t.Log("🔍 Cache-Database Sync Test")
	
	// Write-through, Write-behind 패턴 검증
	// 데이터 손실 및 불일치 시나리오 테스트
}

// testEventDrivenConsistency 이벤트 기반 일관성 테스트
func (suite *IntegrationTestSuite) testEventDrivenConsistency(t *testing.T) {
	t.Log("🔍 Event-Driven Consistency Test")
	
	// 이벤트 발행과 처리 간의 일관성 검증
	// 메시지 중복, 순서 변경, 손실 시나리오
}

// TestStructuralProblems 구조적 문제 분석 테스트
func (suite *IntegrationTestSuite) TestStructuralProblems() {
	suite.T().Run("Architecture Bottlenecks", func(t *testing.T) {
		suite.testArchitectureBottlenecks(t)
	})
	
	suite.T().Run("Scalability Limits", func(t *testing.T) {
		suite.testScalabilityLimits(t)
	})
	
	suite.T().Run("Single Points of Failure", func(t *testing.T) {
		suite.testSinglePointsOfFailure(t)
	})
}

// testArchitectureBottlenecks 아키텍처 병목 테스트
func (suite *IntegrationTestSuite) testArchitectureBottlenecks(t *testing.T) {
	t.Log("🔍 Architecture Bottlenecks Analysis")
	
	// 각 레이어별 병목 지점 식별
	layers := []string{
		"gRPC API Layer",
		"Service Layer", 
		"Cache Layer",
		"Database Layer",
		"Event Layer",
	}
	
	for _, layer := range layers {
		t.Logf("Analyzing bottlenecks in %s", layer)
		// TODO: 실제 병목 분석 로직
	}
}

// testScalabilityLimits 확장성 한계 테스트
func (suite *IntegrationTestSuite) testScalabilityLimits(t *testing.T) {
	t.Log("🔍 Scalability Limits Analysis")
	
	// 시스템 확장성 한계점 식별
	scaleFactors := []int{100, 1000, 10000, 100000}
	
	for _, factor := range scaleFactors {
		t.Logf("Testing scale factor: %d users", factor)
		// TODO: 확장성 테스트 로직
	}
}

// testSinglePointsOfFailure 단일 장애점 테스트
func (suite *IntegrationTestSuite) testSinglePointsOfFailure(t *testing.T) {
	t.Log("🔍 Single Points of Failure Analysis")
	
	// 시스템의 단일 장애점 식별
	components := []string{
		"Redis Cache",
		"DynamoDB",
		"RabbitMQ",
		"Elasticsearch",
		"gRPC Server",
	}
	
	for _, component := range components {
		t.Logf("Analyzing SPOF in %s", component)
		// TODO: 장애 시뮬레이션 및 복구 테스트
	}
}

// TestSummaryReport 테스트 결과 종합 분석
func (suite *IntegrationTestSuite) TestSummaryReport() {
	t := suite.T()
	t.Log("📊 Integration Test Summary Report")
	
	// 메트릭 종합
	t.Logf("=== Performance Metrics ===")
	t.Logf("Average Response Time: %v", suite.metrics.GetAverageResponseTime())
	t.Logf("Cache Hit Rate: %.2f%%", suite.metrics.GetCacheHitRate())
	t.Logf("Total Database Calls: %d", suite.metrics.DatabaseCalls)
	
	t.Logf("=== Error Analysis ===")
	t.Logf("Total Errors: %d", suite.metrics.ErrorCount)
	t.Logf("Concurrency Errors: %d", suite.metrics.ConcurrencyErrors)
	t.Logf("Consistency Errors: %d", suite.metrics.ConsistencyErrors)
	
	// 문제점 요약
	t.Logf("=== Identified Issues ===")
	if suite.metrics.ConcurrencyErrors > 0 {
		t.Logf("⚠️  Concurrency issues detected: %d errors", suite.metrics.ConcurrencyErrors)
	}
	if suite.metrics.ConsistencyErrors > 0 {
		t.Logf("⚠️  Data consistency issues detected: %d errors", suite.metrics.ConsistencyErrors)
	}
	if suite.metrics.GetCacheHitRate() < 90.0 {
		t.Logf("⚠️  Low cache hit rate: %.2f%% (expected >90%%)", suite.metrics.GetCacheHitRate())
	}
	
	// 개선 권고사항
	t.Logf("=== Improvement Recommendations ===")
	t.Logf("1. Implement distributed locking for cache consistency")
	t.Logf("2. Add circuit breaker pattern for resilience") 
	t.Logf("3. Optimize cache TTL strategies")
	t.Logf("4. Implement proper event ordering mechanisms")
	t.Logf("5. Add comprehensive monitoring and alerting")
}

// generateTestData 테스트 데이터 생성 헬퍼
func generateTestData(targetSize int) map[string]interface{} {
	data := map[string]interface{}{
		"user_id": "test_user",
		"friends": make([]map[string]interface{}, 0),
	}
	
	// 목표 크기에 맞춰 데이터 생성
	friendCount := targetSize / 100 // 대략적인 크기 조절
	for i := 0; i < friendCount; i++ {
		friend := map[string]interface{}{
			"id":           fmt.Sprintf("friend_%d", i),
			"name":         fmt.Sprintf("Friend Name %d", i),
			"status":       "active",
			"created_at":   time.Now().Unix(),
			"mutual_count": i % 50,
		}
		data["friends"] = append(data["friends"].([]map[string]interface{}), friend)
	}
	
	return data
}

// TestIntegrationSuite 테스트 스위트 실행
func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}