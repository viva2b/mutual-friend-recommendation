package concurrency

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mutual-friend/internal/domain"
	"mutual-friend/pkg/cache"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/redis"
)

// ConcurrencyTester 동시성 테스트 구조체
type ConcurrencyTester struct {
	cacheService cache.Cache
	ctx          context.Context
	metrics      *ConcurrencyMetrics
}

// ConcurrencyMetrics 동시성 관련 메트릭
type ConcurrencyMetrics struct {
	mu                    sync.RWMutex
	
	// 경합 관련 메트릭
	RaceConditions        int64
	DataInconsistencies   int64
	CacheConflicts        int64
	DeadlockAttempts      int64
	
	// 성능 영향 메트릭
	ContentionDelay       time.Duration
	LockWaitTime          time.Duration
	RetryAttempts         int64
	
	// 무결성 메트릭
	DataCorruptions       int64
	LostUpdates           int64
	DirtyReads            int64
	PhantomReads          int64
	
	// 리소스 경합 메트릭
	MemoryContention      int64
	ConnectionContention  int64
	ThreadContention      int64
}

func (cm *ConcurrencyMetrics) IncrementRaceCondition() {
	atomic.AddInt64(&cm.RaceConditions, 1)
}

func (cm *ConcurrencyMetrics) IncrementDataInconsistency() {
	atomic.AddInt64(&cm.DataInconsistencies, 1)
}

func (cm *ConcurrencyMetrics) IncrementCacheConflict() {
	atomic.AddInt64(&cm.CacheConflicts, 1)
}

func (cm *ConcurrencyMetrics) IncrementDeadlockAttempt() {
	atomic.AddInt64(&cm.DeadlockAttempts, 1)
}

func (cm *ConcurrencyMetrics) AddContentionDelay(duration time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.ContentionDelay += duration
}

func (cm *ConcurrencyMetrics) IncrementRetryAttempt() {
	atomic.AddInt64(&cm.RetryAttempts, 1)
}

func (cm *ConcurrencyMetrics) IncrementDataCorruption() {
	atomic.AddInt64(&cm.DataCorruptions, 1)
}

func (cm *ConcurrencyMetrics) IncrementLostUpdate() {
	atomic.AddInt64(&cm.LostUpdates, 1)
}

// NewConcurrencyTester 동시성 테스터 생성
func NewConcurrencyTester(t *testing.T) *ConcurrencyTester {
	// 설정 로드
	cfg, err := config.LoadConfig("../../configs/config.yaml")
	require.NoError(t, err)
	
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
	require.NoError(t, err)
	
	// 캐시 서비스 초기화
	cacheService, err := cache.NewService(redisClient)
	require.NoError(t, err)
	
	return &ConcurrencyTester{
		cacheService: cacheService,
		ctx:          context.Background(),
		metrics:      &ConcurrencyMetrics{},
	}
}

// TestRaceConditions 레이스 컨디션 테스트
func TestRaceConditions(t *testing.T) {
	tester := NewConcurrencyTester(t)
	defer tester.cacheService.Close()
	
	t.Run("Write-Write Race Condition", func(t *testing.T) {
		tester.testWriteWriteRaceCondition(t)
	})
	
	t.Run("Read-Write Race Condition", func(t *testing.T) {
		tester.testReadWriteRaceCondition(t)
	})
	
	t.Run("Cache Invalidation Race", func(t *testing.T) {
		tester.testCacheInvalidationRace(t)
	})
	
	t.Run("Counter Race Condition", func(t *testing.T) {
		tester.testCounterRaceCondition(t)
	})
}

// testWriteWriteRaceCondition Write-Write 레이스 컨디션 테스트
func (ct *ConcurrencyTester) testWriteWriteRaceCondition(t *testing.T) {
	t.Log("🔍 Testing Write-Write Race Conditions")
	
	const (
		numWriters = 100
		testKey    = "race_write_test"
	)
	
	var wg sync.WaitGroup
	var successfulWrites int64
	var failedWrites int64
	
	// 초기 값 설정
	err := ct.cacheService.Set(ct.ctx, testKey, "initial_value", time.Hour)
	require.NoError(t, err)
	
	start := time.Now()
	
	// 동시에 같은 키에 쓰기
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			
			value := fmt.Sprintf("writer_%d_value", writerID)
			
			writeStart := time.Now()
			err := ct.cacheService.Set(ct.ctx, testKey, value, time.Hour)
			writeTime := time.Since(writeStart)
			
			if writeTime > 10*time.Millisecond {
				ct.metrics.AddContentionDelay(writeTime)
			}
			
			if err != nil {
				atomic.AddInt64(&failedWrites, 1)
				ct.metrics.IncrementRaceCondition()
			} else {
				atomic.AddInt64(&successfulWrites, 1)
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// 최종 값 확인
	finalValue, err := ct.cacheService.Get(ct.ctx, testKey)
	require.NoError(t, err)
	
	t.Logf("=== Write-Write Race Condition Results ===")
	t.Logf("Total Writers: %d", numWriters)
	t.Logf("Successful Writes: %d", successfulWrites)
	t.Logf("Failed Writes: %d", failedWrites)
	t.Logf("Final Value: %s", finalValue)
	t.Logf("Total Duration: %v", duration)
	t.Logf("Race Conditions Detected: %d", ct.metrics.RaceConditions)
	
	// 검증
	assert.Equal(t, numWriters, int(successfulWrites+failedWrites), "Total operations mismatch")
	assert.NotEqual(t, "initial_value", finalValue, "Value should have been updated")
	
	// 경고 임계값
	if ct.metrics.RaceConditions > int64(numWriters/10) {
		t.Logf("⚠️  High race condition rate: %d/%d", ct.metrics.RaceConditions, numWriters)
	}
}

// testReadWriteRaceCondition Read-Write 레이스 컨디션 테스트
func (ct *ConcurrencyTester) testReadWriteRaceCondition(t *testing.T) {
	t.Log("🔍 Testing Read-Write Race Conditions")
	
	const (
		numReaders = 50
		numWriters = 10
		testKey    = "race_rw_test"
	)
	
	var wg sync.WaitGroup
	var readValues []string
	var writeValues []string
	var mu sync.Mutex
	
	// 초기 값 설정
	initialValue := "initial_rw_value"
	err := ct.cacheService.Set(ct.ctx, testKey, initialValue, time.Hour)
	require.NoError(t, err)
	
	start := time.Now()
	
	// 읽기 고루틴들
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			
			// 랜덤 지연으로 읽기 타이밍 분산
			time.Sleep(time.Duration(readerID%10) * time.Millisecond)
			
			value, err := ct.cacheService.Get(ct.ctx, testKey)
			if err != nil {
				ct.metrics.IncrementRaceCondition()
				return
			}
			
			mu.Lock()
			readValues = append(readValues, value)
			mu.Unlock()
		}(i)
	}
	
	// 쓰기 고루틴들
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			
			// 랜덤 지연으로 쓰기 타이밍 분산
			time.Sleep(time.Duration(writerID%5) * time.Millisecond)
			
			value := fmt.Sprintf("writer_%d_updated", writerID)
			err := ct.cacheService.Set(ct.ctx, testKey, value, time.Hour)
			if err != nil {
				ct.metrics.IncrementRaceCondition()
				return
			}
			
			mu.Lock()
			writeValues = append(writeValues, value)
			mu.Unlock()
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// 최종 값 확인
	finalValue, err := ct.cacheService.Get(ct.ctx, testKey)
	require.NoError(t, err)
	
	t.Logf("=== Read-Write Race Condition Results ===")
	t.Logf("Readers: %d, Writers: %d", numReaders, numWriters)
	t.Logf("Read Values Collected: %d", len(readValues))
	t.Logf("Write Values Collected: %d", len(writeValues))
	t.Logf("Final Value: %s", finalValue)
	t.Logf("Duration: %v", duration)
	
	// 데이터 일관성 검증
	ct.analyzeReadWriteConsistency(t, readValues, writeValues, initialValue, finalValue)
}

// analyzeReadWriteConsistency 읽기-쓰기 일관성 분석
func (ct *ConcurrencyTester) analyzeReadWriteConsistency(t *testing.T, readValues, writeValues []string, initialValue, finalValue string) {
	// 읽기 값들 분석
	readValueCounts := make(map[string]int)
	for _, value := range readValues {
		readValueCounts[value]++
	}
	
	// 쓰기 값들 분석
	writeValueCounts := make(map[string]int)
	for _, value := range writeValues {
		writeValueCounts[value]++
	}
	
	t.Logf("=== Read-Write Consistency Analysis ===")
	t.Logf("Unique Read Values: %d", len(readValueCounts))
	t.Logf("Unique Write Values: %d", len(writeValueCounts))
	
	// 비정상적인 읽기 감지
	for value, count := range readValueCounts {
		if value != initialValue && writeValueCounts[value] == 0 {
			ct.metrics.IncrementDataInconsistency()
			t.Logf("🔴 Phantom read detected: %s (count: %d)", value, count)
		}
	}
	
	// 더티 읽기 감지
	if readValueCounts[initialValue] > 0 && len(writeValues) > 0 {
		t.Logf("🟡 Potential dirty read: initial value read %d times after writes", readValueCounts[initialValue])
	}
	
	t.Logf("Data Inconsistencies: %d", ct.metrics.DataInconsistencies)
}

// testCacheInvalidationRace 캐시 무효화 경합 테스트
func (ct *ConcurrencyTester) testCacheInvalidationRace(t *testing.T) {
	t.Log("🔍 Testing Cache Invalidation Race Conditions")
	
	const (
		numInvalidators = 20
		numSetters      = 30
		testKeyPrefix   = "invalidation_race"
	)
	
	var wg sync.WaitGroup
	var invalidationConflicts int64
	var setterConflicts int64
	
	// 초기 데이터 설정
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("%s_%d", testKeyPrefix, i)
		value := fmt.Sprintf("initial_value_%d", i)
		err := ct.cacheService.Set(ct.ctx, key, value, time.Hour)
		require.NoError(t, err)
	}
	
	start := time.Now()
	
	// 무효화 고루틴들
	for i := 0; i < numInvalidators; i++ {
		wg.Add(1)
		go func(invalidatorID int) {
			defer wg.Done()
			
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("%s_%d", testKeyPrefix, j%10)
				
				invalidateStart := time.Now()
				_, err := ct.cacheService.Delete(ct.ctx, key)
				invalidateTime := time.Since(invalidateStart)
				
				if invalidateTime > 5*time.Millisecond {
					ct.metrics.AddContentionDelay(invalidateTime)
				}
				
				if err != nil {
					atomic.AddInt64(&invalidationConflicts, 1)
					ct.metrics.IncrementCacheConflict()
				}
				
				time.Sleep(time.Millisecond) // 약간의 지연
			}
		}(i)
	}
	
	// 설정 고루틴들
	for i := 0; i < numSetters; i++ {
		wg.Add(1)
		go func(setterID int) {
			defer wg.Done()
			
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("%s_%d", testKeyPrefix, j%10)
				value := fmt.Sprintf("setter_%d_value_%d", setterID, j)
				
				setStart := time.Now()
				err := ct.cacheService.Set(ct.ctx, key, value, time.Hour)
				setTime := time.Since(setStart)
				
				if setTime > 5*time.Millisecond {
					ct.metrics.AddContentionDelay(setTime)
				}
				
				if err != nil {
					atomic.AddInt64(&setterConflicts, 1)
					ct.metrics.IncrementCacheConflict()
				}
				
				time.Sleep(time.Millisecond) // 약간의 지연
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// 최종 상태 확인
	finalStates := make(map[string]string)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("%s_%d", testKeyPrefix, i)
		value, err := ct.cacheService.Get(ct.ctx, key)
		if err != nil {
			finalStates[key] = "DELETED"
		} else {
			finalStates[key] = value
		}
	}
	
	t.Logf("=== Cache Invalidation Race Results ===")
	t.Logf("Invalidators: %d, Setters: %d", numInvalidators, numSetters)
	t.Logf("Invalidation Conflicts: %d", invalidationConflicts)
	t.Logf("Setter Conflicts: %d", setterConflicts)
	t.Logf("Total Cache Conflicts: %d", ct.metrics.CacheConflicts)
	t.Logf("Duration: %v", duration)
	
	t.Logf("=== Final Cache States ===")
	deletedCount := 0
	for key, state := range finalStates {
		t.Logf("%s: %s", key, state)
		if state == "DELETED" {
			deletedCount++
		}
	}
	t.Logf("Deleted Entries: %d/10", deletedCount)
}

// testCounterRaceCondition 카운터 레이스 컨디션 테스트
func (ct *ConcurrencyTester) testCounterRaceCondition(t *testing.T) {
	t.Log("🔍 Testing Counter Race Conditions")
	
	const (
		numIncrementers = 50
		incrementCount  = 20
		testKey        = "counter_race_test"
	)
	
	// 초기 카운터 값 설정
	err := ct.cacheService.Set(ct.ctx, testKey, "0", time.Hour)
	require.NoError(t, err)
	
	var wg sync.WaitGroup
	var lostUpdates int64
	
	start := time.Now()
	
	// 동시에 카운터 증가
	for i := 0; i < numIncrementers; i++ {
		wg.Add(1)
		go func(incrementerID int) {
			defer wg.Done()
			
			for j := 0; j < incrementCount; j++ {
				// Read-Modify-Write 패턴으로 레이스 컨디션 유도
				success := ct.incrementCounter(testKey)
				if !success {
					atomic.AddInt64(&lostUpdates, 1)
					ct.metrics.IncrementLostUpdate()
				}
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// 최종 카운터 값 확인
	finalValueStr, err := ct.cacheService.Get(ct.ctx, testKey)
	require.NoError(t, err)
	
	expectedValue := numIncrementers * incrementCount
	
	t.Logf("=== Counter Race Condition Results ===")
	t.Logf("Incrementers: %d", numIncrementers)
	t.Logf("Increments per Incrementer: %d", incrementCount)
	t.Logf("Expected Final Value: %d", expectedValue)
	t.Logf("Actual Final Value: %s", finalValueStr)
	t.Logf("Lost Updates: %d", lostUpdates)
	t.Logf("Duration: %v", duration)
	
	// 실제로 손실된 업데이트 계산
	var actualValue int
	fmt.Sscanf(finalValueStr, "%d", &actualValue)
	actualLostUpdates := expectedValue - actualValue
	
	t.Logf("Calculated Lost Updates: %d", actualLostUpdates)
	
	// 검증
	assert.True(t, actualLostUpdates >= 0, "Negative lost updates detected")
	if actualLostUpdates > 0 {
		t.Logf("🔴 Lost update problem detected: %d updates lost", actualLostUpdates)
	}
}

// incrementCounter 카운터 증가 (레이스 컨디션에 취약한 구현)
func (ct *ConcurrencyTester) incrementCounter(key string) bool {
	// 현재 값 읽기
	currentStr, err := ct.cacheService.Get(ct.ctx, key)
	if err != nil {
		return false
	}
	
	// 값 파싱
	var current int
	fmt.Sscanf(currentStr, "%d", &current)
	
	// 의도적 지연으로 레이스 컨디션 유도
	time.Sleep(time.Microsecond * 10)
	
	// 증가된 값 저장
	newValue := current + 1
	newValueStr := fmt.Sprintf("%d", newValue)
	
	err = ct.cacheService.Set(ct.ctx, key, newValueStr, time.Hour)
	return err == nil
}

// TestDeadlockDetection 데드락 감지 테스트
func TestDeadlockDetection(t *testing.T) {
	tester := NewConcurrencyTester(t)
	defer tester.cacheService.Close()
	
	t.Run("Resource Ordering Deadlock", func(t *testing.T) {
		tester.testResourceOrderingDeadlock(t)
	})
	
	t.Run("Circular Wait Deadlock", func(t *testing.T) {
		tester.testCircularWaitDeadlock(t)
	})
}

// testResourceOrderingDeadlock 리소스 순서 데드락 테스트
func (ct *ConcurrencyTester) testResourceOrderingDeadlock(t *testing.T) {
	t.Log("🔍 Testing Resource Ordering Deadlocks")
	
	const (
		numWorkers = 10
		resource1  = "resource_A"
		resource2  = "resource_B"
	)
	
	var wg sync.WaitGroup
	var deadlockAttempts int64
	var completedOperations int64
	
	// 초기 리소스 설정
	err := ct.cacheService.Set(ct.ctx, resource1, "data_A", time.Hour)
	require.NoError(t, err)
	err = ct.cacheService.Set(ct.ctx, resource2, "data_B", time.Hour)
	require.NoError(t, err)
	
	start := time.Now()
	
	// 데드락 시나리오 시뮬레이션
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			timeout := time.After(5 * time.Second)
			
			if workerID%2 == 0 {
				// Worker A: resource1 -> resource2 순서
				select {
				case <-timeout:
					atomic.AddInt64(&deadlockAttempts, 1)
					ct.metrics.IncrementDeadlockAttempt()
					return
				default:
					success := ct.acquireResourcesInOrder(resource1, resource2)
					if success {
						atomic.AddInt64(&completedOperations, 1)
					}
				}
			} else {
				// Worker B: resource2 -> resource1 순서 (데드락 유발 가능)
				select {
				case <-timeout:
					atomic.AddInt64(&deadlockAttempts, 1)
					ct.metrics.IncrementDeadlockAttempt()
					return
				default:
					success := ct.acquireResourcesInOrder(resource2, resource1)
					if success {
						atomic.AddInt64(&completedOperations, 1)
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	t.Logf("=== Resource Ordering Deadlock Results ===")
	t.Logf("Workers: %d", numWorkers)
	t.Logf("Completed Operations: %d", completedOperations)
	t.Logf("Deadlock Attempts: %d", deadlockAttempts)
	t.Logf("Duration: %v", duration)
	
	// 데드락 가능성 분석
	if deadlockAttempts > 0 {
		t.Logf("🔴 Potential deadlock detected: %d timeout cases", deadlockAttempts)
	}
	
	successRate := float64(completedOperations) / float64(numWorkers) * 100
	t.Logf("Success Rate: %.2f%%", successRate)
}

// acquireResourcesInOrder 리소스를 순서대로 획득
func (ct *ConcurrencyTester) acquireResourcesInOrder(first, second string) bool {
	// 첫 번째 리소스 획득 시뮬레이션
	_, err := ct.cacheService.Get(ct.ctx, first)
	if err != nil {
		return false
	}
	
	// 작업 시뮬레이션
	time.Sleep(time.Millisecond * 10)
	
	// 두 번째 리소스 획득 시뮬레이션
	_, err = ct.cacheService.Get(ct.ctx, second)
	if err != nil {
		return false
	}
	
	// 작업 완료 시뮬레이션
	time.Sleep(time.Millisecond * 5)
	
	return true
}

// testCircularWaitDeadlock 순환 대기 데드락 테스트
func (ct *ConcurrencyTester) testCircularWaitDeadlock(t *testing.T) {
	t.Log("🔍 Testing Circular Wait Deadlocks")
	
	const (
		numResources = 5
		numWorkers   = 10
	)
	
	var wg sync.WaitGroup
	var circularWaitDetected int64
	var completedChains int64
	
	// 리소스 체인 초기화
	resources := make([]string, numResources)
	for i := 0; i < numResources; i++ {
		resources[i] = fmt.Sprintf("chain_resource_%d", i)
		err := ct.cacheService.Set(ct.ctx, resources[i], fmt.Sprintf("data_%d", i), time.Hour)
		require.NoError(t, err)
	}
	
	start := time.Now()
	
	// 순환 대기 시나리오
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			timeout := time.After(3 * time.Second)
			
			select {
			case <-timeout:
				atomic.AddInt64(&circularWaitDetected, 1)
				return
			default:
				// 리소스 체인 순차 획득
				success := ct.acquireResourceChain(resources, workerID)
				if success {
					atomic.AddInt64(&completedChains, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	t.Logf("=== Circular Wait Deadlock Results ===")
	t.Logf("Resources: %d, Workers: %d", numResources, numWorkers)
	t.Logf("Completed Chains: %d", completedChains)
	t.Logf("Circular Wait Detected: %d", circularWaitDetected)
	t.Logf("Duration: %v", duration)
	
	if circularWaitDetected > 0 {
		t.Logf("🔴 Circular wait deadlock detected: %d cases", circularWaitDetected)
	}
}

// acquireResourceChain 리소스 체인 획득
func (ct *ConcurrencyTester) acquireResourceChain(resources []string, workerID int) bool {
	// 워커별로 다른 시작점으로 순환 패턴 생성
	startIndex := workerID % len(resources)
	
	for i := 0; i < len(resources); i++ {
		resourceIndex := (startIndex + i) % len(resources)
		resource := resources[resourceIndex]
		
		_, err := ct.cacheService.Get(ct.ctx, resource)
		if err != nil {
			return false
		}
		
		// 리소스 보유 시뮬레이션
		time.Sleep(time.Millisecond * time.Duration(10+i))
	}
	
	return true
}

// TestMemoryConsistency 메모리 일관성 테스트
func TestMemoryConsistency(t *testing.T) {
	tester := NewConcurrencyTester(t)
	defer tester.cacheService.Close()
	
	t.Run("Sequential Consistency", func(t *testing.T) {
		tester.testSequentialConsistency(t)
	})
	
	t.Run("Eventual Consistency", func(t *testing.T) {
		tester.testEventualConsistency(t)
	})
	
	t.Run("Causal Consistency", func(t *testing.T) {
		tester.testCausalConsistency(t)
	})
}

// testSequentialConsistency 순차 일관성 테스트
func (ct *ConcurrencyTester) testSequentialConsistency(t *testing.T) {
	t.Log("🔍 Testing Sequential Consistency")
	
	const (
		numOperations = 100
		numReaders    = 10
		testKey       = "sequential_test"
	)
	
	var wg sync.WaitGroup
	var operations []string
	var readings [][]string
	var mu sync.Mutex
	
	// 초기값 설정
	err := ct.cacheService.Set(ct.ctx, testKey, "operation_0", time.Hour)
	require.NoError(t, err)
	
	start := time.Now()
	
	// 순차적 쓰기 작업
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= numOperations; i++ {
			value := fmt.Sprintf("operation_%d", i)
			err := ct.cacheService.Set(ct.ctx, testKey, value, time.Hour)
			if err == nil {
				mu.Lock()
				operations = append(operations, value)
				mu.Unlock()
			}
			time.Sleep(time.Millisecond) // 순차성 보장을 위한 지연
		}
	}()
	
	// 동시 읽기 작업들
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			
			var readerValues []string
			for i := 0; i < numOperations/2; i++ {
				value, err := ct.cacheService.Get(ct.ctx, testKey)
				if err == nil {
					readerValues = append(readerValues, value)
				}
				time.Sleep(time.Millisecond * 2)
			}
			
			mu.Lock()
			readings = append(readings, readerValues)
			mu.Unlock()
		}(r)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// 순차 일관성 분석
	violations := ct.analyzeSequentialConsistency(operations, readings)
	
	t.Logf("=== Sequential Consistency Results ===")
	t.Logf("Total Operations: %d", len(operations))
	t.Logf("Readers: %d", numReaders)
	t.Logf("Consistency Violations: %d", violations)
	t.Logf("Duration: %v", duration)
	
	if violations > 0 {
		t.Logf("🔴 Sequential consistency violations detected: %d", violations)
		ct.metrics.DataInconsistencies += int64(violations)
	}
}

// analyzeSequentialConsistency 순차 일관성 분석
func (ct *ConcurrencyTester) analyzeSequentialConsistency(operations []string, readings [][]string) int {
	violations := 0
	
	// 각 리더의 읽기 시퀀스 분석
	for readerID, readerValues := range readings {
		for i := 1; i < len(readerValues); i++ {
			currentVal := readerValues[i]
			prevVal := readerValues[i-1]
			
			// 이전 값보다 작은 인덱스의 값이 나타나면 순차성 위반
			currentIdx := ct.findOperationIndex(operations, currentVal)
			prevIdx := ct.findOperationIndex(operations, prevVal)
			
			if currentIdx >= 0 && prevIdx >= 0 && currentIdx < prevIdx {
				violations++
				// 자세한 로깅은 테스트에서 제어
			}
		}
	}
	
	return violations
}

// findOperationIndex 작업 인덱스 찾기
func (ct *ConcurrencyTester) findOperationIndex(operations []string, value string) int {
	for i, op := range operations {
		if op == value {
			return i
		}
	}
	return -1
}

// testEventualConsistency 최종 일관성 테스트
func (ct *ConcurrencyTester) testEventualConsistency(t *testing.T) {
	t.Log("🔍 Testing Eventual Consistency")
	
	const (
		numUpdaters    = 5
		updatesPerUser = 10
		testKey        = "eventual_test"
	)
	
	var wg sync.WaitGroup
	var finalValues []string
	var mu sync.Mutex
	
	// 동시 업데이트
	for u := 0; u < numUpdaters; u++ {
		wg.Add(1)
		go func(updaterID int) {
			defer wg.Done()
			
			for i := 0; i < updatesPerUser; i++ {
				value := fmt.Sprintf("updater_%d_value_%d", updaterID, i)
				err := ct.cacheService.Set(ct.ctx, testKey, value, time.Hour)
				if err == nil {
					mu.Lock()
					finalValues = append(finalValues, value)
					mu.Unlock()
				}
				time.Sleep(time.Millisecond * 5)
			}
		}(u)
	}
	
	wg.Wait()
	
	// 최종 일관성 확인을 위한 대기
	time.Sleep(100 * time.Millisecond)
	
	// 최종 값 확인
	finalValue, err := ct.cacheService.Get(ct.ctx, testKey)
	require.NoError(t, err)
	
	// 최종 값이 수행된 업데이트 중 하나인지 확인
	isConsistent := false
	for _, value := range finalValues {
		if value == finalValue {
			isConsistent = true
			break
		}
	}
	
	t.Logf("=== Eventual Consistency Results ===")
	t.Logf("Total Updates: %d", len(finalValues))
	t.Logf("Final Value: %s", finalValue)
	t.Logf("Is Eventually Consistent: %t", isConsistent)
	
	assert.True(t, isConsistent, "Eventual consistency violation")
}

// testCausalConsistency 인과 일관성 테스트
func (ct *ConcurrencyTester) testCausalConsistency(t *testing.T) {
	t.Log("🔍 Testing Causal Consistency")
	
	const testKey = "causal_test"
	
	// 인과관계가 있는 작업 시퀀스
	var wg sync.WaitGroup
	
	// 첫 번째 작업
	err := ct.cacheService.Set(ct.ctx, testKey, "cause_1", time.Hour)
	require.NoError(t, err)
	
	time.Sleep(10 * time.Millisecond) // 인과관계 보장을 위한 지연
	
	// 두 번째 작업 (첫 번째에 의존)
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 첫 번째 값 읽기
		value, err := ct.cacheService.Get(ct.ctx, testKey)
		if err == nil && value == "cause_1" {
			// 인과관계에 따른 업데이트
			ct.cacheService.Set(ct.ctx, testKey, "effect_1", time.Hour)
		}
	}()
	
	// 세 번째 작업 (두 번째에 의존)
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond) // 두 번째 작업 후 실행 보장
		
		value, err := ct.cacheService.Get(ct.ctx, testKey)
		if err == nil && value == "effect_1" {
			ct.cacheService.Set(ct.ctx, testKey, "effect_2", time.Hour)
		}
	}()
	
	wg.Wait()
	
	// 최종 인과관계 검증
	finalValue, err := ct.cacheService.Get(ct.ctx, testKey)
	require.NoError(t, err)
	
	t.Logf("=== Causal Consistency Results ===")
	t.Logf("Final Value: %s", finalValue)
	
	// 인과관계 검증
	expectedSequence := []string{"cause_1", "effect_1", "effect_2"}
	isValid := false
	for _, expected := range expectedSequence {
		if finalValue == expected {
			isValid = true
			break
		}
	}
	
	t.Logf("Causal Consistency Maintained: %t", isValid)
	assert.True(t, isValid, "Causal consistency violation")
}

// TestConcurrencySummary 동시성 테스트 종합 보고서
func TestConcurrencySummary(t *testing.T) {
	tester := NewConcurrencyTester(t)
	defer tester.cacheService.Close()
	
	t.Log("📊 Concurrency Test Summary Report")
	
	// 전체 메트릭 요약
	t.Logf("=== Concurrency Issues Summary ===")
	t.Logf("Race Conditions: %d", tester.metrics.RaceConditions)
	t.Logf("Data Inconsistencies: %d", tester.metrics.DataInconsistencies)
	t.Logf("Cache Conflicts: %d", tester.metrics.CacheConflicts)
	t.Logf("Deadlock Attempts: %d", tester.metrics.DeadlockAttempts)
	t.Logf("Lost Updates: %d", tester.metrics.LostUpdates)
	t.Logf("Data Corruptions: %d", tester.metrics.DataCorruptions)
	
	// 심각도별 분류
	criticalIssues := tester.metrics.DeadlockAttempts + tester.metrics.DataCorruptions
	majorIssues := tester.metrics.RaceConditions + tester.metrics.DataInconsistencies
	minorIssues := tester.metrics.CacheConflicts + tester.metrics.LostUpdates
	
	t.Logf("=== Issue Severity Analysis ===")
	t.Logf("Critical Issues: %d", criticalIssues)
	t.Logf("Major Issues: %d", majorIssues)
	t.Logf("Minor Issues: %d", minorIssues)
	
	// 개선 권고사항
	t.Logf("=== Improvement Recommendations ===")
	if tester.metrics.RaceConditions > 0 {
		t.Logf("🔴 Implement proper synchronization mechanisms")
	}
	if tester.metrics.DeadlockAttempts > 0 {
		t.Logf("🔴 Add deadlock detection and recovery")
	}
	if tester.metrics.DataInconsistencies > 0 {
		t.Logf("🟡 Review consistency models and isolation levels")
	}
	if tester.metrics.CacheConflicts > 0 {
		t.Logf("🟡 Optimize cache invalidation strategies")
	}
	
	// 전체 평가
	totalIssues := criticalIssues + majorIssues + minorIssues
	if totalIssues == 0 {
		t.Logf("✅ No significant concurrency issues detected")
	} else if totalIssues < 10 {
		t.Logf("🟡 Minor concurrency issues detected - monitoring recommended")
	} else {
		t.Logf("🔴 Significant concurrency issues detected - immediate attention required")
	}
}