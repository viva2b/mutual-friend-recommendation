package consistency

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mutual-friend/internal/domain"
	"mutual-friend/pkg/cache"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/redis"
)

// ConsistencyTester 일관성 테스트 구조체
type ConsistencyTester struct {
	cacheService cache.Cache
	ctx          context.Context
	metrics      *ConsistencyMetrics
}

// ConsistencyMetrics 일관성 관련 메트릭
type ConsistencyMetrics struct {
	mu                        sync.RWMutex
	
	// 데이터 일관성 메트릭
	StaleReads               int64
	InconsistentWrites       int64
	MissingUpdates           int64
	DuplicateData            int64
	
	// 캐시-DB 일관성
	CacheDBMismatches        int64
	CacheStaleData           int64
	CacheInvalidationLag     time.Duration
	
	// 트랜잭션 일관성
	IsolationViolations      int64
	AtomicityViolations      int64
	DurabilityViolations     int64
	
	// 이벤트 일관성
	EventOrderViolations     int64
	EventLoss                int64
	DuplicateEvents          int64
	
	// 시간 기반 일관성
	ClockSkewIssues          int64
	TimestampInconsistencies int64
	CausalOrderViolations    int64
	
	// 복구 메트릭
	AutoRecoveryAttempts     int64
	ManualRecoveryRequired   int64
	DataLossIncidents        int64
}

func (cm *ConsistencyMetrics) IncrementStaleRead() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.StaleReads++
}

func (cm *ConsistencyMetrics) IncrementInconsistentWrite() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.InconsistentWrites++
}

func (cm *ConsistencyMetrics) IncrementCacheDBMismatch() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.CacheDBMismatches++
}

func (cm *ConsistencyMetrics) IncrementEventOrderViolation() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.EventOrderViolations++
}

func (cm *ConsistencyMetrics) AddCacheInvalidationLag(lag time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.CacheInvalidationLag += lag
}

// ConsistencySnapshot 일관성 검증을 위한 스냅샷
type ConsistencySnapshot struct {
	Timestamp   time.Time
	CacheState  map[string]interface{}
	DBState     map[string]interface{}
	EventQueue  []Event
	Operations  []Operation
}

// Event 이벤트 구조체
type Event struct {
	ID        string
	Type      string
	UserID    string
	FriendID  string
	Timestamp time.Time
	Sequence  int64
}

// Operation 작업 구조체
type Operation struct {
	ID        string
	Type      string
	Target    string
	Before    interface{}
	After     interface{}
	Timestamp time.Time
	Success   bool
}

// NewConsistencyTester 일관성 테스터 생성
func NewConsistencyTester(t *testing.T) *ConsistencyTester {
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
	
	return &ConsistencyTester{
		cacheService: cacheService,
		ctx:          context.Background(),
		metrics:      &ConsistencyMetrics{},
	}
}

// TestDataConsistency 데이터 일관성 테스트
func TestDataConsistency(t *testing.T) {
	tester := NewConsistencyTester(t)
	defer tester.cacheService.Close()
	
	t.Run("Cache-Database Consistency", func(t *testing.T) {
		tester.testCacheDatabaseConsistency(t)
	})
	
	t.Run("Read-Write Consistency", func(t *testing.T) {
		tester.testReadWriteConsistency(t)
	})
	
	t.Run("Eventual Consistency", func(t *testing.T) {
		tester.testEventualConsistency(t)
	})
	
	t.Run("Strong Consistency", func(t *testing.T) {
		tester.testStrongConsistency(t)
	})
}

// testCacheDatabaseConsistency 캐시-데이터베이스 일관성 테스트
func (ct *ConsistencyTester) testCacheDatabaseConsistency(t *testing.T) {
	t.Log("🔍 Testing Cache-Database Consistency")
	
	const (
		numOperations = 100
		numCheckers   = 5
		testPrefix    = "consistency_test"
	)
	
	var wg sync.WaitGroup
	var operations []Operation
	var mu sync.Mutex
	
	// 일관성 검증을 위한 시뮬레이션된 DB 상태
	simulatedDB := make(map[string]interface{})
	var dbMu sync.RWMutex
	
	start := time.Now()
	
	// 데이터 조작 작업들
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(opID int) {
			defer wg.Done()
			
			key := fmt.Sprintf("%s_%d", testPrefix, opID%10) // 키 충돌 유도
			value := fmt.Sprintf("value_%d_%d", opID, time.Now().UnixNano())
			
			operation := Operation{
				ID:        fmt.Sprintf("op_%d", opID),
				Type:      "WRITE",
				Target:    key,
				Timestamp: time.Now(),
			}
			
			// 캐시에 쓰기
			cacheStart := time.Now()
			err := ct.cacheService.Set(ct.ctx, key, value, time.Hour)
			cacheTime := time.Since(cacheStart)
			
			if err != nil {
				operation.Success = false
			} else {
				operation.Success = true
				operation.After = value
				
				// 시뮬레이션된 DB 업데이트 (지연 포함)
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(5)+1))
				
				dbMu.Lock()
				simulatedDB[key] = value
				dbMu.Unlock()
			}
			
			mu.Lock()
			operations = append(operations, operation)
			mu.Unlock()
			
			// 캐시 지연이 있으면 비일관성 가능성
			if cacheTime > 50*time.Millisecond {
				ct.metrics.AddCacheInvalidationLag(cacheTime)
			}
		}(i)
	}
	
	// 일관성 검증자들
	for c := 0; c < numCheckers; c++ {
		wg.Add(1)
		go func(checkerID int) {
			defer wg.Done()
			
			for i := 0; i < 20; i++ {
				key := fmt.Sprintf("%s_%d", testPrefix, i%10)
				
				// 캐시에서 읽기
				cacheValue, cacheErr := ct.cacheService.Get(ct.ctx, key)
				
				// 시뮬레이션된 DB에서 읽기
				dbMu.RLock()
				dbValue, dbExists := simulatedDB[key]
				dbMu.RUnlock()
				
				// 일관성 검증
				if cacheErr == nil && dbExists {
					if cacheValue != dbValue {
						ct.metrics.IncrementCacheDBMismatch()
						t.Logf("Mismatch detected - Key: %s, Cache: %s, DB: %v", 
							key, cacheValue, dbValue)
					}
				} else if cacheErr == nil && !dbExists {
					ct.metrics.IncrementStaleRead()
					t.Logf("Stale read detected - Key: %s exists in cache but not in DB", key)
				} else if cacheErr != nil && dbExists {
					ct.metrics.IncrementCacheDBMismatch()
					t.Logf("Cache miss but DB has data - Key: %s", key)
				}
				
				time.Sleep(time.Millisecond * 10)
			}
		}(c)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	t.Logf("=== Cache-Database Consistency Results ===")
	t.Logf("Total Operations: %d", len(operations))
	t.Logf("Cache-DB Mismatches: %d", ct.metrics.CacheDBMismatches)
	t.Logf("Stale Reads: %d", ct.metrics.StaleReads)
	t.Logf("Average Cache Invalidation Lag: %v", ct.metrics.CacheInvalidationLag/time.Duration(numOperations))
	t.Logf("Duration: %v", duration)
	
	// 일관성 검증
	successfulOps := 0
	for _, op := range operations {
		if op.Success {
			successfulOps++
		}
	}
	
	consistencyRate := float64(successfulOps-int(ct.metrics.CacheDBMismatches)) / float64(successfulOps) * 100
	t.Logf("Consistency Rate: %.2f%%", consistencyRate)
	
	// 일관성 임계값 검증
	assert.Greater(t, consistencyRate, 90.0, "Consistency rate too low")
	assert.Less(t, ct.metrics.CacheDBMismatches, int64(numOperations/10), "Too many cache-DB mismatches")
}

// testReadWriteConsistency 읽기-쓰기 일관성 테스트
func (ct *ConsistencyTester) testReadWriteConsistency(t *testing.T) {
	t.Log("🔍 Testing Read-Write Consistency")
	
	const (
		numWriters = 10
		numReaders = 20
		testKey    = "rw_consistency_test"
	)
	
	var wg sync.WaitGroup
	var writes []string
	var reads []string
	var mu sync.Mutex
	
	// 초기값 설정
	initialValue := "initial_rw_value"
	err := ct.cacheService.Set(ct.ctx, testKey, initialValue, time.Hour)
	require.NoError(t, err)
	
	start := time.Now()
	
	// 쓰기 작업들
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			
			for i := 0; i < 5; i++ {
				value := fmt.Sprintf("writer_%d_value_%d", writerID, i)
				writeTime := time.Now()
				
				err := ct.cacheService.Set(ct.ctx, testKey, value, time.Hour)
				if err == nil {
					mu.Lock()
					writes = append(writes, fmt.Sprintf("%s@%d", value, writeTime.UnixNano()))
					mu.Unlock()
				}
				
				time.Sleep(time.Millisecond * 5)
			}
		}(w)
	}
	
	// 읽기 작업들
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			
			for i := 0; i < 10; i++ {
				readTime := time.Now()
				value, err := ct.cacheService.Get(ct.ctx, testKey)
				
				if err == nil {
					mu.Lock()
					reads = append(reads, fmt.Sprintf("%s@%d", value, readTime.UnixNano()))
					mu.Unlock()
				}
				
				time.Sleep(time.Millisecond * 3)
			}
		}(r)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// 읽기-쓰기 일관성 분석
	inconsistencies := ct.analyzeReadWriteConsistency(writes, reads, initialValue)
	
	t.Logf("=== Read-Write Consistency Results ===")
	t.Logf("Total Writes: %d", len(writes))
	t.Logf("Total Reads: %d", len(reads))
	t.Logf("Inconsistencies Detected: %d", inconsistencies)
	t.Logf("Duration: %v", duration)
	
	if inconsistencies > 0 {
		t.Logf("🔴 Read-write consistency violations: %d", inconsistencies)
	}
	
	// 일관성 검증
	assert.Less(t, inconsistencies, len(reads)/10, "Too many read-write inconsistencies")
}

// analyzeReadWriteConsistency 읽기-쓰기 일관성 분석
func (ct *ConsistencyTester) analyzeReadWriteConsistency(writes, reads []string, initialValue string) int {
	inconsistencies := 0
	
	// 타임스탬프별로 쓰기 작업 정렬
	writeMap := make(map[int64]string)
	for _, write := range writes {
		parts := parseTimestampedValue(write)
		if len(parts) == 2 {
			writeMap[parts[1]] = parts[0]
		}
	}
	
	// 각 읽기에 대해 일관성 검증
	for _, read := range reads {
		parts := parseTimestampedValue(read)
		if len(parts) != 2 {
			continue
		}
		
		readValue, readTime := parts[0], parts[1]
		
		// 읽기 시점 이후에 쓰인 값을 읽었다면 일관성 위반
		validValue := ct.findValidValueAtTime(writeMap, readTime, initialValue)
		
		if readValue != validValue {
			inconsistencies++
			ct.metrics.IncrementInconsistentWrite()
		}
	}
	
	return inconsistencies
}

// parseTimestampedValue 타임스탬프가 포함된 값 파싱
func parseTimestampedValue(timestampedValue string) []interface{} {
	var value string
	var timestamp int64
	
	n, err := fmt.Sscanf(timestampedValue, "%s@%d", &value, &timestamp)
	if n == 2 && err == nil {
		return []interface{}{value, timestamp}
	}
	
	return nil
}

// findValidValueAtTime 특정 시점에서 유효한 값 찾기
func (ct *ConsistencyTester) findValidValueAtTime(writeMap map[int64]string, readTime int64, initialValue string) string {
	validValue := initialValue
	validTime := int64(0)
	
	for writeTime, writeValue := range writeMap {
		if writeTime <= readTime && writeTime > validTime {
			validValue = writeValue
			validTime = writeTime
		}
	}
	
	return validValue
}

// testEventualConsistency 최종 일관성 테스트
func (ct *ConsistencyTester) testEventualConsistency(t *testing.T) {
	t.Log("🔍 Testing Eventual Consistency")
	
	const (
		numNodes       = 5
		operationsPerNode = 20
		testKey        = "eventual_consistency_test"
	)
	
	var wg sync.WaitGroup
	var allOperations []Operation
	var mu sync.Mutex
	
	// 각 노드에서 독립적인 작업 수행
	for node := 0; node < numNodes; node++ {
		wg.Add(1)
		go func(nodeID int) {
			defer wg.Done()
			
			for op := 0; op < operationsPerNode; op++ {
				operation := Operation{
					ID:        fmt.Sprintf("node_%d_op_%d", nodeID, op),
					Type:      "UPDATE",
					Target:    testKey,
					Timestamp: time.Now(),
				}
				
				value := fmt.Sprintf("node_%d_value_%d", nodeID, op)
				err := ct.cacheService.Set(ct.ctx, testKey, value, time.Hour)
				
				operation.Success = err == nil
				operation.After = value
				
				mu.Lock()
				allOperations = append(allOperations, operation)
				mu.Unlock()
				
				// 노드 간 지연 시뮬레이션
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(10)+1))
			}
		}(node)
	}
	
	wg.Wait()
	
	// 최종 일관성 수렴 대기
	convergenceTimeout := 5 * time.Second
	converged := ct.waitForEventualConsistency(testKey, convergenceTimeout)
	
	// 최종 값 확인
	finalValue, err := ct.cacheService.Get(ct.ctx, testKey)
	require.NoError(t, err)
	
	t.Logf("=== Eventual Consistency Results ===")
	t.Logf("Total Operations: %d", len(allOperations))
	t.Logf("Converged: %t", converged)
	t.Logf("Final Value: %s", finalValue)
	
	// 최종 값이 실제 작업 중 하나인지 확인
	isValidFinalValue := false
	for _, op := range allOperations {
		if op.Success && op.After == finalValue {
			isValidFinalValue = true
			break
		}
	}
	
	t.Logf("Final Value is Valid: %t", isValidFinalValue)
	assert.True(t, isValidFinalValue, "Final value is not from any performed operation")
	
	if !converged {
		t.Logf("⚠️  System did not converge within timeout")
	}
}

// waitForEventualConsistency 최종 일관성 수렴 대기
func (ct *ConsistencyTester) waitForEventualConsistency(key string, timeout time.Duration) bool {
	start := time.Now()
	lastValue := ""
	stableCount := 0
	requiredStableReads := 10
	
	for time.Since(start) < timeout {
		value, err := ct.cacheService.Get(ct.ctx, key)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		
		if value == lastValue {
			stableCount++
			if stableCount >= requiredStableReads {
				return true
			}
		} else {
			stableCount = 0
			lastValue = value
		}
		
		time.Sleep(100 * time.Millisecond)
	}
	
	return false
}

// testStrongConsistency 강한 일관성 테스트
func (ct *ConsistencyTester) testStrongConsistency(t *testing.T) {
	t.Log("🔍 Testing Strong Consistency")
	
	const (
		numClients = 20
		testKey    = "strong_consistency_test"
	)
	
	var wg sync.WaitGroup
	var readResults []string
	var mu sync.Mutex
	
	// 강한 일관성을 위한 순차적 쓰기
	sequentialValues := make([]string, 10)
	for i := 0; i < 10; i++ {
		value := fmt.Sprintf("sequential_value_%d", i)
		sequentialValues[i] = value
		
		err := ct.cacheService.Set(ct.ctx, testKey, value, time.Hour)
		require.NoError(t, err)
		
		time.Sleep(50 * time.Millisecond) // 강한 일관성을 위한 대기
	}
	
	// 동시 읽기로 강한 일관성 검증
	finalValue := sequentialValues[len(sequentialValues)-1]
	
	for c := 0; c < numClients; c++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			value, err := ct.cacheService.Get(ct.ctx, testKey)
			if err == nil {
				mu.Lock()
				readResults = append(readResults, value)
				mu.Unlock()
			}
		}(c)
	}
	
	wg.Wait()
	
	// 강한 일관성 검증
	consistentReads := 0
	for _, readValue := range readResults {
		if readValue == finalValue {
			consistentReads++
		}
	}
	
	t.Logf("=== Strong Consistency Results ===")
	t.Logf("Total Reads: %d", len(readResults))
	t.Logf("Consistent Reads: %d", consistentReads)
	t.Logf("Consistency Rate: %.2f%%", float64(consistentReads)/float64(len(readResults))*100)
	
	// 강한 일관성은 100% 일관성을 요구
	assert.Equal(t, len(readResults), consistentReads, "Strong consistency violation")
}

// TestEventConsistency 이벤트 일관성 테스트
func TestEventConsistency(t *testing.T) {
	tester := NewConsistencyTester(t)
	defer tester.cacheService.Close()
	
	t.Run("Event Ordering", func(t *testing.T) {
		tester.testEventOrdering(t)
	})
	
	t.Run("Event Deduplication", func(t *testing.T) {
		tester.testEventDeduplication(t)
	})
	
	t.Run("Event Replay Consistency", func(t *testing.T) {
		tester.testEventReplayConsistency(t)
	})
}

// testEventOrdering 이벤트 순서 테스트
func (ct *ConsistencyTester) testEventOrdering(t *testing.T) {
	t.Log("🔍 Testing Event Ordering Consistency")
	
	const (
		numEvents = 100
		testKey   = "event_ordering_test"
	)
	
	var events []Event
	var processedEvents []Event
	var mu sync.Mutex
	
	// 순차적 이벤트 생성
	for i := 0; i < numEvents; i++ {
		event := Event{
			ID:        fmt.Sprintf("event_%03d", i),
			Type:      "UPDATE",
			UserID:    "test_user",
			FriendID:  fmt.Sprintf("friend_%d", i%10),
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
			Sequence:  int64(i),
		}
		events = append(events, event)
	}
	
	// 이벤트를 랜덤 순서로 처리 (실제 분산 시스템에서 발생 가능)
	shuffledEvents := make([]Event, len(events))
	copy(shuffledEvents, events)
	rand.Shuffle(len(shuffledEvents), func(i, j int) {
		shuffledEvents[i], shuffledEvents[j] = shuffledEvents[j], shuffledEvents[i]
	})
	
	var wg sync.WaitGroup
	
	// 이벤트 병렬 처리
	for _, event := range shuffledEvents {
		wg.Add(1)
		go func(e Event) {
			defer wg.Done()
			
			// 이벤트 처리 시뮬레이션
			key := fmt.Sprintf("%s_%s", testKey, e.UserID)
			value := fmt.Sprintf("event_%s_processed", e.ID)
			
			err := ct.cacheService.Set(ct.ctx, key, value, time.Hour)
			if err == nil {
				mu.Lock()
				processedEvents = append(processedEvents, e)
				mu.Unlock()
			}
			
			// 처리 지연 시뮬레이션
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(5)+1))
		}(event)
	}
	
	wg.Wait()
	
	// 이벤트 순서 검증
	orderViolations := ct.analyzeEventOrdering(events, processedEvents)
	
	t.Logf("=== Event Ordering Results ===")
	t.Logf("Total Events: %d", len(events))
	t.Logf("Processed Events: %d", len(processedEvents))
	t.Logf("Order Violations: %d", orderViolations)
	
	if orderViolations > 0 {
		t.Logf("🔴 Event ordering violations detected: %d", orderViolations)
		ct.metrics.EventOrderViolations += int64(orderViolations)
	}
	
	// 순서 위반이 전체의 10% 이하여야 함
	assert.Less(t, orderViolations, len(events)/10, "Too many event ordering violations")
}

// analyzeEventOrdering 이벤트 순서 분석
func (ct *ConsistencyTester) analyzeEventOrdering(originalEvents, processedEvents []Event) int {
	// 처리된 이벤트를 시퀀스 번호로 정렬
	eventsBySequence := make(map[int64]Event)
	for _, event := range processedEvents {
		eventsBySequence[event.Sequence] = event
	}
	
	violations := 0
	lastSequence := int64(-1)
	
	// 순차적으로 처리되었는지 확인
	for i := 0; i < len(originalEvents); i++ {
		expectedSeq := int64(i)
		if event, exists := eventsBySequence[expectedSeq]; exists {
			if event.Sequence < lastSequence {
				violations++
			}
			lastSequence = event.Sequence
		}
	}
	
	return violations
}

// testEventDeduplication 이벤트 중복 제거 테스트
func (ct *ConsistencyTester) testEventDeduplication(t *testing.T) {
	t.Log("🔍 Testing Event Deduplication")
	
	const (
		uniqueEvents = 50
		duplicateCount = 3
		testKey = "dedup_test"
	)
	
	var allEvents []Event
	var processedValues []string
	var mu sync.Mutex
	
	// 중복 이벤트 생성
	for i := 0; i < uniqueEvents; i++ {
		baseEvent := Event{
			ID:        fmt.Sprintf("unique_event_%d", i),
			Type:      "ADD_FRIEND",
			UserID:    "test_user",
			FriendID:  fmt.Sprintf("friend_%d", i),
			Timestamp: time.Now(),
			Sequence:  int64(i),
		}
		
		// 원본 + 중복본들
		for d := 0; d < duplicateCount; d++ {
			duplicateEvent := baseEvent
			duplicateEvent.ID = fmt.Sprintf("%s_dup_%d", baseEvent.ID, d)
			allEvents = append(allEvents, duplicateEvent)
		}
	}
	
	// 중복 이벤트 랜덤 처리
	rand.Shuffle(len(allEvents), func(i, j int) {
		allEvents[i], allEvents[j] = allEvents[j], allEvents[i]
	})
	
	var wg sync.WaitGroup
	
	for _, event := range allEvents {
		wg.Add(1)
		go func(e Event) {
			defer wg.Done()
			
			// 중복 제거 시뮬레이션 (간단한 키 기반)
			dedupKey := fmt.Sprintf("%s_dedup_%s_%s", testKey, e.UserID, e.FriendID)
			
			// 중복 확인
			existing, err := ct.cacheService.Get(ct.ctx, dedupKey)
			if err != nil || existing == "" {
				// 새로운 이벤트 처리
				value := fmt.Sprintf("processed_%s", e.FriendID)
				ct.cacheService.Set(ct.ctx, dedupKey, value, time.Hour)
				
				mu.Lock()
				processedValues = append(processedValues, value)
				mu.Unlock()
			}
			// 중복 이벤트는 무시
		}(event)
	}
	
	wg.Wait()
	
	// 중복 제거 효과 검증
	uniqueProcessedValues := make(map[string]bool)
	for _, value := range processedValues {
		uniqueProcessedValues[value] = true
	}
	
	t.Logf("=== Event Deduplication Results ===")
	t.Logf("Total Events: %d", len(allEvents))
	t.Logf("Unique Events: %d", uniqueEvents)
	t.Logf("Processed Values: %d", len(processedValues))
	t.Logf("Unique Processed Values: %d", len(uniqueProcessedValues))
	
	duplicatesProcessed := len(processedValues) - len(uniqueProcessedValues)
	t.Logf("Duplicates Processed: %d", duplicatesProcessed)
	
	// 중복 제거 효율성 검증
	assert.LessOrEqual(t, len(uniqueProcessedValues), uniqueEvents, 
		"More unique values processed than unique events")
	
	if duplicatesProcessed > 0 {
		t.Logf("🟡 Duplicate events processed: %d", duplicatesProcessed)
		ct.metrics.DuplicateEvents += int64(duplicatesProcessed)
	}
}

// testEventReplayConsistency 이벤트 재생 일관성 테스트
func (ct *ConsistencyTester) testEventReplayConsistency(t *testing.T) {
	t.Log("🔍 Testing Event Replay Consistency")
	
	const testKey = "replay_test"
	
	// 원본 이벤트 시퀀스
	originalEvents := []Event{
		{ID: "1", Type: "SET", UserID: "user1", Timestamp: time.Now(), Sequence: 1},
		{ID: "2", Type: "UPDATE", UserID: "user1", Timestamp: time.Now().Add(time.Second), Sequence: 2},
		{ID: "3", Type: "DELETE", UserID: "user1", Timestamp: time.Now().Add(2*time.Second), Sequence: 3},
		{ID: "4", Type: "SET", UserID: "user1", Timestamp: time.Now().Add(3*time.Second), Sequence: 4},
	}
	
	// 첫 번째 실행: 정상 순서로 이벤트 처리
	firstRunState := ct.processEventSequence(originalEvents, "first_run")
	
	// 캐시 초기화
	ct.cacheService.Flush(ct.ctx)
	
	// 두 번째 실행: 같은 이벤트 재생
	secondRunState := ct.processEventSequence(originalEvents, "second_run")
	
	// 세 번째 실행: 부분 재생 (장애 복구 시뮬레이션)
	ct.cacheService.Flush(ct.ctx)
	partialEvents := originalEvents[1:] // 첫 번째 이벤트 누락
	thirdRunState := ct.processEventSequence(partialEvents, "third_run")
	
	t.Logf("=== Event Replay Consistency Results ===")
	t.Logf("First Run Final State: %s", firstRunState)
	t.Logf("Second Run Final State: %s", secondRunState)
	t.Logf("Third Run Final State: %s", thirdRunState)
	
	// 재생 일관성 검증
	if firstRunState == secondRunState {
		t.Logf("✅ Replay consistency maintained")
	} else {
		t.Logf("🔴 Replay consistency violation")
		ct.metrics.IncrementEventOrderViolation()
	}
	
	// 부분 재생 검증
	if firstRunState != thirdRunState {
		t.Logf("🟡 Partial replay produced different result (expected)")
	}
	
	assert.Equal(t, firstRunState, secondRunState, "Event replay inconsistency")
}

// processEventSequence 이벤트 시퀀스 처리
func (ct *ConsistencyTester) processEventSequence(events []Event, runID string) string {
	key := fmt.Sprintf("replay_test_%s", runID)
	
	for _, event := range events {
		switch event.Type {
		case "SET":
			value := fmt.Sprintf("value_from_event_%s", event.ID)
			ct.cacheService.Set(ct.ctx, key, value, time.Hour)
		case "UPDATE":
			existing, err := ct.cacheService.Get(ct.ctx, key)
			if err == nil {
				updated := fmt.Sprintf("%s_updated_by_%s", existing, event.ID)
				ct.cacheService.Set(ct.ctx, key, updated, time.Hour)
			}
		case "DELETE":
			ct.cacheService.Delete(ct.ctx, key)
		}
		
		time.Sleep(10 * time.Millisecond) // 이벤트 간 지연
	}
	
	// 최종 상태 반환
	finalState, err := ct.cacheService.Get(ct.ctx, key)
	if err != nil {
		return "DELETED"
	}
	return finalState
}

// TestTimestampConsistency 타임스탬프 일관성 테스트
func TestTimestampConsistency(t *testing.T) {
	tester := NewConsistencyTester(t)
	defer tester.cacheService.Close()
	
	t.Run("Clock Skew Detection", func(t *testing.T) {
		tester.testClockSkewDetection(t)
	})
	
	t.Run("Causal Order Consistency", func(t *testing.T) {
		tester.testCausalOrderConsistency(t)
	})
}

// testClockSkewDetection 시계 편차 감지 테스트
func (ct *ConsistencyTester) testClockSkewDetection(t *testing.T) {
	t.Log("🔍 Testing Clock Skew Detection")
	
	const (
		numNodes = 5
		testKey  = "clock_skew_test"
	)
	
	var timestamps []time.Time
	var mu sync.Mutex
	
	var wg sync.WaitGroup
	
	// 각 노드에서 시뮬레이션된 시계 편차로 작업
	for node := 0; node < numNodes; node++ {
		wg.Add(1)
		go func(nodeID int) {
			defer wg.Done()
			
			// 시계 편차 시뮬레이션 (-100ms ~ +100ms)
			skew := time.Duration((nodeID-2)*50) * time.Millisecond
			nodeTime := time.Now().Add(skew)
			
			value := fmt.Sprintf("node_%d_time_%d", nodeID, nodeTime.UnixNano())
			err := ct.cacheService.Set(ct.ctx, testKey, value, time.Hour)
			
			if err == nil {
				mu.Lock()
				timestamps = append(timestamps, nodeTime)
				mu.Unlock()
			}
		}(node)
	}
	
	wg.Wait()
	
	// 시계 편차 분석
	if len(timestamps) > 1 {
		var minTime, maxTime time.Time
		minTime = timestamps[0]
		maxTime = timestamps[0]
		
		for _, ts := range timestamps {
			if ts.Before(minTime) {
				minTime = ts
			}
			if ts.After(maxTime) {
				maxTime = ts
			}
		}
		
		skewRange := maxTime.Sub(minTime)
		
		t.Logf("=== Clock Skew Detection Results ===")
		t.Logf("Timestamps Collected: %d", len(timestamps))
		t.Logf("Skew Range: %v", skewRange)
		
		if skewRange > 200*time.Millisecond {
			t.Logf("🔴 Significant clock skew detected: %v", skewRange)
			ct.metrics.ClockSkewIssues++
		}
		
		assert.Less(t, skewRange, 500*time.Millisecond, "Clock skew too large")
	}
}

// testCausalOrderConsistency 인과 순서 일관성 테스트
func (ct *ConsistencyTester) testCausalOrderConsistency(t *testing.T) {
	t.Log("🔍 Testing Causal Order Consistency")
	
	const testKey = "causal_order_test"
	
	// 인과관계 체인: A → B → C
	var wg sync.WaitGroup
	
	// 이벤트 A
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		err := ct.cacheService.Set(ct.ctx, testKey, "event_A", time.Hour)
		if err != nil {
			t.Logf("Event A failed: %v", err)
		}
	}()
	
	wg.Wait()
	time.Sleep(50 * time.Millisecond) // 인과관계 보장
	
	// 이벤트 B (A에 의존)
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		value, err := ct.cacheService.Get(ct.ctx, testKey)
		if err == nil && value == "event_A" {
			ct.cacheService.Set(ct.ctx, testKey, "event_B_after_A", time.Hour)
		} else {
			ct.metrics.CausalOrderViolations++
			t.Logf("Causal order violation: B executed without A")
		}
	}()
	
	wg.Wait()
	time.Sleep(50 * time.Millisecond)
	
	// 이벤트 C (B에 의존)
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		value, err := ct.cacheService.Get(ct.ctx, testKey)
		if err == nil && value == "event_B_after_A" {
			ct.cacheService.Set(ct.ctx, testKey, "event_C_after_B", time.Hour)
		} else {
			ct.metrics.CausalOrderViolations++
			t.Logf("Causal order violation: C executed without B")
		}
	}()
	
	wg.Wait()
	
	// 최종 인과관계 검증
	finalValue, err := ct.cacheService.Get(ct.ctx, testKey)
	require.NoError(t, err)
	
	t.Logf("=== Causal Order Consistency Results ===")
	t.Logf("Final Value: %s", finalValue)
	t.Logf("Causal Order Violations: %d", ct.metrics.CausalOrderViolations)
	
	expectedFinalValue := "event_C_after_B"
	if finalValue == expectedFinalValue {
		t.Logf("✅ Causal order maintained")
	} else {
		t.Logf("🔴 Causal order violation detected")
	}
	
	assert.Equal(t, expectedFinalValue, finalValue, "Causal order consistency violation")
}

// TestConsistencySummary 일관성 테스트 종합 보고서
func TestConsistencySummary(t *testing.T) {
	tester := NewConsistencyTester(t)
	defer tester.cacheService.Close()
	
	t.Log("📊 Consistency Test Summary Report")
	
	// 전체 메트릭 요약
	t.Logf("=== Data Consistency Issues ===")
	t.Logf("Stale Reads: %d", tester.metrics.StaleReads)
	t.Logf("Inconsistent Writes: %d", tester.metrics.InconsistentWrites)
	t.Logf("Missing Updates: %d", tester.metrics.MissingUpdates)
	t.Logf("Cache-DB Mismatches: %d", tester.metrics.CacheDBMismatches)
	
	t.Logf("=== Transaction Consistency ===")
	t.Logf("Isolation Violations: %d", tester.metrics.IsolationViolations)
	t.Logf("Atomicity Violations: %d", tester.metrics.AtomicityViolations)
	t.Logf("Durability Violations: %d", tester.metrics.DurabilityViolations)
	
	t.Logf("=== Event Consistency ===")
	t.Logf("Event Order Violations: %d", tester.metrics.EventOrderViolations)
	t.Logf("Event Loss: %d", tester.metrics.EventLoss)
	t.Logf("Duplicate Events: %d", tester.metrics.DuplicateEvents)
	
	t.Logf("=== Temporal Consistency ===")
	t.Logf("Clock Skew Issues: %d", tester.metrics.ClockSkewIssues)
	t.Logf("Timestamp Inconsistencies: %d", tester.metrics.TimestampInconsistencies)
	t.Logf("Causal Order Violations: %d", tester.metrics.CausalOrderViolations)
	
	// 심각도 분석
	criticalIssues := tester.metrics.AtomicityViolations + tester.metrics.DurabilityViolations + tester.metrics.DataLossIncidents
	majorIssues := tester.metrics.IsolationViolations + tester.metrics.EventOrderViolations + tester.metrics.CausalOrderViolations
	minorIssues := tester.metrics.StaleReads + tester.metrics.CacheDBMismatches + tester.metrics.DuplicateEvents
	
	t.Logf("=== Consistency Issue Severity ===")
	t.Logf("Critical Issues: %d", criticalIssues)
	t.Logf("Major Issues: %d", majorIssues)
	t.Logf("Minor Issues: %d", minorIssues)
	
	// 개선 권고사항
	t.Logf("=== Consistency Improvement Recommendations ===")
	if criticalIssues > 0 {
		t.Logf("🔴 CRITICAL: Implement stronger consistency guarantees")
		t.Logf("🔴 CRITICAL: Add transaction support and rollback mechanisms")
	}
	if majorIssues > 0 {
		t.Logf("🟡 MAJOR: Implement proper isolation levels")
		t.Logf("🟡 MAJOR: Add event ordering and causality tracking")
	}
	if minorIssues > 0 {
		t.Logf("🟢 MINOR: Optimize cache invalidation patterns")
		t.Logf("🟢 MINOR: Implement better duplicate detection")
	}
	
	// 전체 일관성 점수
	totalIssues := criticalIssues + majorIssues + minorIssues
	consistencyScore := 100.0
	
	if totalIssues > 0 {
		// 가중치 적용하여 점수 계산
		weightedScore := float64(criticalIssues)*10 + float64(majorIssues)*3 + float64(minorIssues)*1
		consistencyScore = math.Max(0, 100.0-weightedScore)
	}
	
	t.Logf("=== Overall Consistency Score ===")
	t.Logf("Consistency Score: %.1f/100", consistencyScore)
	
	if consistencyScore >= 90 {
		t.Logf("✅ Excellent consistency")
	} else if consistencyScore >= 70 {
		t.Logf("🟡 Good consistency with room for improvement")
	} else {
		t.Logf("🔴 Poor consistency - immediate attention required")
	}
}