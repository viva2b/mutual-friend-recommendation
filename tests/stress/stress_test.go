package stress

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mutual-friend/pkg/cache"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/redis"
)

// StressTester 스트레스 테스트 구조체
type StressTester struct {
	cacheService cache.Cache
	ctx          context.Context
	metrics      *StressMetrics
}

// StressMetrics 스트레스 테스트 메트릭
type StressMetrics struct {
	mu                    sync.RWMutex
	
	// 부하 테스트 메트릭
	TotalRequests         int64
	SuccessfulRequests    int64
	FailedRequests        int64
	TimeoutRequests       int64
	
	// 응답 시간 메트릭
	ResponseTimes         []time.Duration
	MinResponseTime       time.Duration
	MaxResponseTime       time.Duration
	AvgResponseTime       time.Duration
	
	// 처리량 메트릭
	RequestsPerSecond     float64
	PeakThroughput        float64
	SustainedThroughput   float64
	
	// 리소스 사용량 메트릭
	MemoryUsage           []uint64
	CPUUsage              []float64
	GoroutineCount        []int
	ConnectionCount       []int
	
	// 에러 패턴 메트릭
	ConnectionErrors      int64
	TimeoutErrors         int64
	MemoryErrors          int64
	ThrottlingErrors      int64
	
	// 안정성 메트릭
	SystemStability       float64  // 0-100%
	ErrorRate             float64  // 0-100%
	RecoveryTime          time.Duration
	
	// 확장성 메트릭
	ScalabilityIndex      float64  // 성능 대비 리소스 효율성
	BottleneckComponents  []string
	
	// 장애 내성 메트릭
	FailureRecoveryTime   time.Duration
	DataLossEvents        int64
	ServiceDegradation    float64
}

func (sm *StressMetrics) RecordRequest(success bool, responseTime time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	atomic.AddInt64(&sm.TotalRequests, 1)
	
	if success {
		atomic.AddInt64(&sm.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&sm.FailedRequests, 1)
	}
	
	sm.ResponseTimes = append(sm.ResponseTimes, responseTime)
	
	if sm.MinResponseTime == 0 || responseTime < sm.MinResponseTime {
		sm.MinResponseTime = responseTime
	}
	if responseTime > sm.MaxResponseTime {
		sm.MaxResponseTime = responseTime
	}
}

func (sm *StressMetrics) RecordResourceUsage() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	sm.MemoryUsage = append(sm.MemoryUsage, memStats.Alloc)
	sm.GoroutineCount = append(sm.GoroutineCount, runtime.NumGoroutine())
}

func (sm *StressMetrics) CalculateMetrics() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// 평균 응답 시간 계산
	if len(sm.ResponseTimes) > 0 {
		var total time.Duration
		for _, rt := range sm.ResponseTimes {
			total += rt
		}
		sm.AvgResponseTime = total / time.Duration(len(sm.ResponseTimes))
	}
	
	// 에러율 계산
	if sm.TotalRequests > 0 {
		sm.ErrorRate = float64(sm.FailedRequests) / float64(sm.TotalRequests) * 100
	}
	
	// 시스템 안정성 계산 (에러율 기반)
	sm.SystemStability = 100.0 - sm.ErrorRate
}

// NewStressTester 스트레스 테스터 생성
func NewStressTester(t *testing.T) *StressTester {
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
	
	return &StressTester{
		cacheService: cacheService,
		ctx:          context.Background(),
		metrics:      &StressMetrics{},
	}
}

// TestLoadTesting 부하 테스트
func TestLoadTesting(t *testing.T) {
	tester := NewStressTester(t)
	defer tester.cacheService.Close()
	
	t.Run("Gradual Load Increase", func(t *testing.T) {
		tester.testGradualLoadIncrease(t)
	})
	
	t.Run("Sustained High Load", func(t *testing.T) {
		tester.testSustainedHighLoad(t)
	})
	
	t.Run("Spike Load Testing", func(t *testing.T) {
		tester.testSpikeLoad(t)
	})
	
	t.Run("Memory Pressure Testing", func(t *testing.T) {
		tester.testMemoryPressure(t)
	})
}

// testGradualLoadIncrease 점진적 부하 증가 테스트
func (st *StressTester) testGradualLoadIncrease(t *testing.T) {
	t.Log("🔍 Testing Gradual Load Increase")
	
	// 부하 단계별 설정
	loadStages := []struct {
		name        string
		concurrency int
		duration    time.Duration
		rps         int // requests per second target
	}{
		{"Warm-up", 10, 30 * time.Second, 100},
		{"Light Load", 50, 60 * time.Second, 500},
		{"Medium Load", 100, 60 * time.Second, 1000},
		{"Heavy Load", 200, 60 * time.Second, 2000},
		{"Peak Load", 500, 60 * time.Second, 5000},
	}
	
	for _, stage := range loadStages {
		t.Run(stage.name, func(t *testing.T) {
			st.runLoadStage(t, stage.name, stage.concurrency, stage.duration, stage.rps)
		})
	}
	
	// 전체 결과 분석
	st.analyzeLoadTestResults(t)
}

// runLoadStage 부하 단계 실행
func (st *StressTester) runLoadStage(t *testing.T, stageName string, concurrency int, duration time.Duration, targetRPS int) {
	t.Logf("Running %s: %d concurrent workers, %v duration, target %d RPS", 
		stageName, concurrency, duration, targetRPS)
	
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(st.ctx, duration)
	defer cancel()
	
	// 리소스 모니터링 시작
	monitoringDone := make(chan bool)
	go st.monitorResources(monitoringDone)
	
	start := time.Now()
	
	// 워커 고루틴 시작
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			st.worker(ctx, workerID, targetRPS/concurrency)
		}(i)
	}
	
	wg.Wait()
	close(monitoringDone)
	actualDuration := time.Since(start)
	
	// 단계별 결과 계산
	st.metrics.CalculateMetrics()
	
	t.Logf("=== %s Results ===", stageName)
	t.Logf("Duration: %v", actualDuration)
	t.Logf("Total Requests: %d", st.metrics.TotalRequests)
	t.Logf("Successful: %d", st.metrics.SuccessfulRequests)
	t.Logf("Failed: %d", st.metrics.FailedRequests)
	t.Logf("Error Rate: %.2f%%", st.metrics.ErrorRate)
	t.Logf("Avg Response Time: %v", st.metrics.AvgResponseTime)
	t.Logf("Min/Max Response Time: %v/%v", st.metrics.MinResponseTime, st.metrics.MaxResponseTime)
	
	actualRPS := float64(st.metrics.TotalRequests) / actualDuration.Seconds()
	t.Logf("Actual RPS: %.2f", actualRPS)
	
	// 성능 임계값 검증
	if st.metrics.ErrorRate > 5.0 {
		t.Logf("⚠️  High error rate in %s: %.2f%%", stageName, st.metrics.ErrorRate)
	}
	if st.metrics.AvgResponseTime > 100*time.Millisecond {
		t.Logf("⚠️  High response time in %s: %v", stageName, st.metrics.AvgResponseTime)
	}
}

// worker 워커 고루틴
func (st *StressTester) worker(ctx context.Context, workerID int, targetRPS int) {
	requestInterval := time.Second / time.Duration(targetRPS)
	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			st.executeRequest(workerID)
		}
	}
}

// executeRequest 요청 실행
func (st *StressTester) executeRequest(workerID int) {
	start := time.Now()
	
	key := fmt.Sprintf("stress_test_worker_%d_req_%d", workerID, time.Now().UnixNano())
	value := fmt.Sprintf("test_value_%d", time.Now().UnixNano())
	
	// 50% SET, 50% GET 작업
	var err error
	if time.Now().UnixNano()%2 == 0 {
		// SET 작업
		err = st.cacheService.Set(st.ctx, key, value, time.Minute)
	} else {
		// GET 작업 (랜덤 키)
		randomKey := fmt.Sprintf("stress_test_worker_%d_req_%d", 
			workerID, time.Now().UnixNano()-int64(workerID*1000))
		_, err = st.cacheService.Get(st.ctx, randomKey)
	}
	
	responseTime := time.Since(start)
	success := err == nil
	
	st.metrics.RecordRequest(success, responseTime)
	
	if err != nil {
		// 에러 타입별 분류
		atomic.AddInt64(&st.metrics.ConnectionErrors, 1)
	}
}

// monitorResources 리소스 모니터링
func (st *StressTester) monitorResources(done chan bool) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			st.metrics.RecordResourceUsage()
		}
	}
}

// analyzeLoadTestResults 부하 테스트 결과 분석
func (st *StressTester) analyzeLoadTestResults(t *testing.T) {
	t.Log("📊 Load Test Results Analysis")
	
	st.metrics.CalculateMetrics()
	
	t.Logf("=== Overall Performance Summary ===")
	t.Logf("Total Requests: %d", st.metrics.TotalRequests)
	t.Logf("Success Rate: %.2f%%", 100.0-st.metrics.ErrorRate)
	t.Logf("Average Response Time: %v", st.metrics.AvgResponseTime)
	t.Logf("Response Time Range: %v - %v", st.metrics.MinResponseTime, st.metrics.MaxResponseTime)
	
	// 리소스 사용량 분석
	if len(st.metrics.MemoryUsage) > 0 {
		maxMemory := uint64(0)
		for _, mem := range st.metrics.MemoryUsage {
			if mem > maxMemory {
				maxMemory = mem
			}
		}
		t.Logf("Peak Memory Usage: %d MB", maxMemory/(1024*1024))
	}
	
	if len(st.metrics.GoroutineCount) > 0 {
		maxGoroutines := 0
		for _, count := range st.metrics.GoroutineCount {
			if count > maxGoroutines {
				maxGoroutines = count
			}
		}
		t.Logf("Peak Goroutine Count: %d", maxGoroutines)
	}
	
	// 성능 평가
	if st.metrics.ErrorRate < 1.0 {
		t.Logf("✅ Excellent error rate: %.2f%%", st.metrics.ErrorRate)
	} else if st.metrics.ErrorRate < 5.0 {
		t.Logf("🟡 Acceptable error rate: %.2f%%", st.metrics.ErrorRate)
	} else {
		t.Logf("🔴 High error rate: %.2f%%", st.metrics.ErrorRate)
	}
	
	if st.metrics.AvgResponseTime < 50*time.Millisecond {
		t.Logf("✅ Excellent response time: %v", st.metrics.AvgResponseTime)
	} else if st.metrics.AvgResponseTime < 200*time.Millisecond {
		t.Logf("🟡 Acceptable response time: %v", st.metrics.AvgResponseTime)
	} else {
		t.Logf("🔴 High response time: %v", st.metrics.AvgResponseTime)
	}
}

// testSustainedHighLoad 지속적 고부하 테스트
func (st *StressTester) testSustainedHighLoad(t *testing.T) {
	t.Log("🔍 Testing Sustained High Load")
	
	const (
		concurrency = 1000
		duration    = 5 * time.Minute
		targetRPS   = 10000
	)
	
	t.Logf("Running sustained load: %d workers, %v duration, target %d RPS", 
		concurrency, duration, targetRPS)
	
	// 초기 메트릭 초기화
	st.metrics = &StressMetrics{}
	
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(st.ctx, duration)
	defer cancel()
	
	// 메모리 사용량 모니터링
	monitoringDone := make(chan bool)
	go st.monitorSustainedLoad(monitoringDone)
	
	start := time.Now()
	
	// 고부하 워커들 시작
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			st.sustainedLoadWorker(ctx, workerID, targetRPS/concurrency)
		}(i)
	}
	
	wg.Wait()
	close(monitoringDone)
	actualDuration := time.Since(start)
	
	st.metrics.CalculateMetrics()
	
	t.Logf("=== Sustained High Load Results ===")
	t.Logf("Actual Duration: %v", actualDuration)
	t.Logf("Total Requests: %d", st.metrics.TotalRequests)
	t.Logf("Requests/Second: %.2f", float64(st.metrics.TotalRequests)/actualDuration.Seconds())
	t.Logf("Error Rate: %.2f%%", st.metrics.ErrorRate)
	t.Logf("System Stability: %.2f%%", st.metrics.SystemStability)
	t.Logf("Average Response Time: %v", st.metrics.AvgResponseTime)
	
	// 지속성 검증
	assert.Less(t, st.metrics.ErrorRate, 2.0, "Error rate too high for sustained load")
	assert.Less(t, st.metrics.AvgResponseTime, 500*time.Millisecond, "Response time degraded under sustained load")
	assert.Greater(t, st.metrics.SystemStability, 95.0, "System not stable under sustained load")
}

// sustainedLoadWorker 지속적 부하 워커
func (st *StressTester) sustainedLoadWorker(ctx context.Context, workerID int, targetRPS int) {
	requestInterval := time.Second / time.Duration(targetRPS)
	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()
	
	requestCount := 0
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			
			// 다양한 작업 패턴
			var err error
			switch requestCount % 4 {
			case 0: // SET
				key := fmt.Sprintf("sustained_set_%d_%d", workerID, requestCount)
				value := fmt.Sprintf("value_%d_%d", workerID, time.Now().UnixNano())
				err = st.cacheService.Set(st.ctx, key, value, time.Hour)
				
			case 1: // GET (existing)
				key := fmt.Sprintf("sustained_set_%d_%d", workerID, requestCount-1)
				_, err = st.cacheService.Get(st.ctx, key)
				
			case 2: // GET (non-existing)
				key := fmt.Sprintf("non_existing_%d_%d", workerID, requestCount)
				_, err = st.cacheService.Get(st.ctx, key)
				
			case 3: // DELETE
				key := fmt.Sprintf("sustained_set_%d_%d", workerID, requestCount-3)
				_, err = st.cacheService.Delete(st.ctx, key)
			}
			
			responseTime := time.Since(start)
			st.metrics.RecordRequest(err == nil, responseTime)
			
			requestCount++
			
			// 메모리 압박 감지
			if requestCount%1000 == 0 {
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)
				if memStats.Alloc > 1024*1024*1024 { // 1GB 초과
					atomic.AddInt64(&st.metrics.MemoryErrors, 1)
				}
			}
		}
	}
}

// monitorSustainedLoad 지속적 부하 모니터링
func (st *StressTester) monitorSustainedLoad(done chan bool) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			st.metrics.RecordResourceUsage()
			
			// 처리량 계산
			currentRPS := float64(atomic.LoadInt64(&st.metrics.TotalRequests)) / time.Since(time.Now()).Seconds()
			if currentRPS > st.metrics.PeakThroughput {
				st.metrics.PeakThroughput = currentRPS
			}
		}
	}
}

// testSpikeLoad 스파이크 부하 테스트
func (st *StressTester) testSpikeLoad(t *testing.T) {
	t.Log("🔍 Testing Spike Load")
	
	// 스파이크 패턴: 낮은 부하 → 급격한 증가 → 높은 부하 → 급격한 감소
	phases := []struct {
		name        string
		concurrency int
		duration    time.Duration
	}{
		{"Baseline", 50, 30 * time.Second},
		{"Spike Up", 2000, 60 * time.Second},
		{"Peak Load", 2000, 120 * time.Second},
		{"Spike Down", 50, 30 * time.Second},
	}
	
	st.metrics = &StressMetrics{}
	
	for _, phase := range phases {
		t.Run(phase.name, func(t *testing.T) {
			st.runSpikePhase(t, phase.name, phase.concurrency, phase.duration)
		})
	}
	
	// 스파이크 복구 분석
	st.analyzeSpikeRecovery(t)
}

// runSpikePhase 스파이크 단계 실행
func (st *StressTester) runSpikePhase(t *testing.T, phaseName string, concurrency int, duration time.Duration) {
	t.Logf("Running %s phase: %d workers for %v", phaseName, concurrency, duration)
	
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(st.ctx, duration)
	defer cancel()
	
	phaseStart := time.Now()
	
	// 스파이크 워커들
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					start := time.Now()
					
					key := fmt.Sprintf("spike_%s_%d_%d", phaseName, workerID, time.Now().UnixNano())
					value := fmt.Sprintf("spike_value_%d", time.Now().UnixNano())
					
					err := st.cacheService.Set(st.ctx, key, value, time.Minute)
					responseTime := time.Since(start)
					
					st.metrics.RecordRequest(err == nil, responseTime)
					
					// 스파이크 중 에러 패턴 추적
					if err != nil {
						if responseTime > 10*time.Second {
							atomic.AddInt64(&st.metrics.TimeoutErrors, 1)
						} else {
							atomic.AddInt64(&st.metrics.ConnectionErrors, 1)
						}
					}
					
					// 스파이크 중 더 공격적인 요청
					if phaseName == "Spike Up" || phaseName == "Peak Load" {
						time.Sleep(time.Microsecond * 100)
					} else {
						time.Sleep(time.Millisecond * 10)
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	phaseDuration := time.Since(phaseStart)
	
	t.Logf("%s phase completed in %v", phaseName, phaseDuration)
}

// analyzeSpikeRecovery 스파이크 복구 분석
func (st *StressTester) analyzeSpikeRecovery(t *testing.T) {
	t.Log("📊 Spike Load Recovery Analysis")
	
	st.metrics.CalculateMetrics()
	
	t.Logf("=== Spike Load Test Results ===")
	t.Logf("Total Requests: %d", st.metrics.TotalRequests)
	t.Logf("Overall Error Rate: %.2f%%", st.metrics.ErrorRate)
	t.Logf("Timeout Errors: %d", st.metrics.TimeoutErrors)
	t.Logf("Connection Errors: %d", st.metrics.ConnectionErrors)
	t.Logf("Peak Response Time: %v", st.metrics.MaxResponseTime)
	
	// 복구 능력 평가
	if st.metrics.ErrorRate < 10.0 {
		t.Logf("✅ Good spike load handling")
	} else if st.metrics.ErrorRate < 25.0 {
		t.Logf("🟡 Moderate spike load handling")
	} else {
		t.Logf("🔴 Poor spike load handling")
	}
	
	// 복구 시간 추정 (마지막 단계의 응답 시간 기반)
	if len(st.metrics.ResponseTimes) > 100 {
		lastResponses := st.metrics.ResponseTimes[len(st.metrics.ResponseTimes)-100:]
		var totalRecoveryTime time.Duration
		for _, rt := range lastResponses {
			totalRecoveryTime += rt
		}
		avgRecoveryTime := totalRecoveryTime / time.Duration(len(lastResponses))
		
		t.Logf("Recovery Response Time: %v", avgRecoveryTime)
		st.metrics.RecoveryTime = avgRecoveryTime
	}
}

// testMemoryPressure 메모리 압박 테스트
func (st *StressTester) testMemoryPressure(t *testing.T) {
	t.Log("🔍 Testing Memory Pressure")
	
	const (
		largeValueSize = 1024 * 1024 // 1MB per value
		numLargeValues = 100
		concurrency    = 50
	)
	
	st.metrics = &StressMetrics{}
	
	// 대용량 데이터 생성
	largeValue := make([]byte, largeValueSize)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}
	largeValueStr := string(largeValue)
	
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(st.ctx, 2*time.Minute)
	defer cancel()
	
	// 메모리 모니터링 시작
	monitoringDone := make(chan bool)
	go st.monitorMemoryPressure(monitoringDone)
	
	start := time.Now()
	
	// 메모리 압박 워커들
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < numLargeValues/concurrency; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					reqStart := time.Now()
					
					key := fmt.Sprintf("memory_pressure_%d_%d", workerID, j)
					err := st.cacheService.Set(st.ctx, key, largeValueStr, time.Hour)
					
					responseTime := time.Since(reqStart)
					st.metrics.RecordRequest(err == nil, responseTime)
					
					if err != nil {
						atomic.AddInt64(&st.metrics.MemoryErrors, 1)
					}
					
					// 메모리 압박 상황에서의 읽기 테스트
					_, err = st.cacheService.Get(st.ctx, key)
					if err != nil {
						atomic.AddInt64(&st.metrics.MemoryErrors, 1)
					}
					
					time.Sleep(time.Millisecond * 100)
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(monitoringDone)
	duration := time.Since(start)
	
	st.metrics.CalculateMetrics()
	
	t.Logf("=== Memory Pressure Test Results ===")
	t.Logf("Duration: %v", duration)
	t.Logf("Large Values Stored: %d", numLargeValues)
	t.Logf("Total Data Size: %d MB", (numLargeValues*largeValueSize)/(1024*1024))
	t.Logf("Memory Errors: %d", st.metrics.MemoryErrors)
	t.Logf("Error Rate: %.2f%%", st.metrics.ErrorRate)
	t.Logf("Average Response Time: %v", st.metrics.AvgResponseTime)
	
	// 메모리 압박 테스트 검증
	if st.metrics.MemoryErrors == 0 {
		t.Logf("✅ No memory-related errors")
	} else if st.metrics.MemoryErrors < 10 {
		t.Logf("🟡 Some memory pressure detected: %d errors", st.metrics.MemoryErrors)
	} else {
		t.Logf("🔴 Significant memory pressure: %d errors", st.metrics.MemoryErrors)
	}
	
	assert.Less(t, st.metrics.ErrorRate, 15.0, "Too many errors under memory pressure")
}

// monitorMemoryPressure 메모리 압박 모니터링
func (st *StressTester) monitorMemoryPressure(done chan bool) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			
			st.metrics.mu.Lock()
			st.metrics.MemoryUsage = append(st.metrics.MemoryUsage, memStats.Alloc)
			st.metrics.GoroutineCount = append(st.metrics.GoroutineCount, runtime.NumGoroutine())
			st.metrics.mu.Unlock()
			
			// 메모리 사용량이 임계치를 넘으면 경고
			if memStats.Alloc > 2*1024*1024*1024 { // 2GB
				atomic.AddInt64(&st.metrics.MemoryErrors, 1)
			}
		}
	}
}

// TestFailureRecovery 장애 복구 테스트
func TestFailureRecovery(t *testing.T) {
	tester := NewStressTester(t)
	defer tester.cacheService.Close()
	
	t.Run("Connection Failure Recovery", func(t *testing.T) {
		tester.testConnectionFailureRecovery(t)
	})
	
	t.Run("Timeout Recovery", func(t *testing.T) {
		tester.testTimeoutRecovery(t)
	})
	
	t.Run("Resource Exhaustion Recovery", func(t *testing.T) {
		tester.testResourceExhaustionRecovery(t)
	})
}

// testConnectionFailureRecovery 연결 장애 복구 테스트
func (st *StressTester) testConnectionFailureRecovery(t *testing.T) {
	t.Log("🔍 Testing Connection Failure Recovery")
	
	const (
		normalOps    = 1000
		concurrency  = 100
	)
	
	st.metrics = &StressMetrics{}
	
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(st.ctx, 2*time.Minute)
	defer cancel()
	
	failureStart := time.Now().Add(30 * time.Second) // 30초 후 장애 시뮬레이션
	recoveryTime := time.Time{}
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < normalOps/concurrency; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					start := time.Now()
					
					// 장애 시뮬레이션 (특정 시간대에 강제 지연)
					if start.After(failureStart) && start.Before(failureStart.Add(20*time.Second)) {
						if recoveryTime.IsZero() {
							recoveryTime = start.Add(20 * time.Second)
						}
						// 장애 상황 시뮬레이션 (연결 지연)
						time.Sleep(time.Millisecond * 500)
					}
					
					key := fmt.Sprintf("recovery_test_%d_%d", workerID, j)
					value := fmt.Sprintf("value_%d", time.Now().UnixNano())
					
					err := st.cacheService.Set(st.ctx, key, value, time.Minute)
					responseTime := time.Since(start)
					
					st.metrics.RecordRequest(err == nil, responseTime)
					
					if err != nil {
						atomic.AddInt64(&st.metrics.ConnectionErrors, 1)
					}
					
					time.Sleep(time.Millisecond * 10)
				}
			}
		}(i)
	}
	
	wg.Wait()
	st.metrics.CalculateMetrics()
	
	// 복구 시간 측정
	if !recoveryTime.IsZero() {
		st.metrics.FailureRecoveryTime = time.Since(recoveryTime)
	}
	
	t.Logf("=== Connection Failure Recovery Results ===")
	t.Logf("Total Requests: %d", st.metrics.TotalRequests)
	t.Logf("Connection Errors: %d", st.metrics.ConnectionErrors)
	t.Logf("Error Rate: %.2f%%", st.metrics.ErrorRate)
	t.Logf("Recovery Time: %v", st.metrics.FailureRecoveryTime)
	t.Logf("System Stability: %.2f%%", st.metrics.SystemStability)
	
	// 복구 능력 검증
	if st.metrics.ErrorRate < 20.0 {
		t.Logf("✅ Good failure recovery")
	} else {
		t.Logf("🔴 Poor failure recovery")
	}
	
	assert.Less(t, st.metrics.ErrorRate, 30.0, "Poor connection failure recovery")
}

// testTimeoutRecovery 타임아웃 복구 테스트
func (st *StressTester) testTimeoutRecovery(t *testing.T) {
	t.Log("🔍 Testing Timeout Recovery")
	
	// 짧은 타임아웃으로 설정하여 타임아웃 상황 유도
	// 실제 구현에서는 Redis 클라이언트 타임아웃 설정을 조정해야 함
	
	const (
		operations   = 500
		concurrency  = 50
	)
	
	st.metrics = &StressMetrics{}
	
	var wg sync.WaitGroup
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < operations/concurrency; j++ {
				start := time.Now()
				
				// 큰 데이터로 타임아웃 유도
				largeData := make([]byte, 1024*1024) // 1MB
				key := fmt.Sprintf("timeout_test_%d_%d", workerID, j)
				
				err := st.cacheService.Set(st.ctx, key, string(largeData), time.Minute)
				responseTime := time.Since(start)
				
				st.metrics.RecordRequest(err == nil, responseTime)
				
				if err != nil {
					if responseTime > 5*time.Second {
						atomic.AddInt64(&st.metrics.TimeoutErrors, 1)
					}
				}
				
				time.Sleep(time.Millisecond * 20)
			}
		}(i)
	}
	
	wg.Wait()
	st.metrics.CalculateMetrics()
	
	t.Logf("=== Timeout Recovery Results ===")
	t.Logf("Total Requests: %d", st.metrics.TotalRequests)
	t.Logf("Timeout Errors: %d", st.metrics.TimeoutErrors)
	t.Logf("Error Rate: %.2f%%", st.metrics.ErrorRate)
	t.Logf("Max Response Time: %v", st.metrics.MaxResponseTime)
	
	if st.metrics.TimeoutErrors < 50 {
		t.Logf("✅ Good timeout handling")
	} else {
		t.Logf("🔴 Many timeout errors: %d", st.metrics.TimeoutErrors)
	}
}

// testResourceExhaustionRecovery 리소스 고갈 복구 테스트
func (st *StressTester) testResourceExhaustionRecovery(t *testing.T) {
	t.Log("🔍 Testing Resource Exhaustion Recovery")
	
	// 의도적으로 많은 고루틴과 메모리 사용
	const (
		extremeConcurrency = 5000
		duration          = 30 * time.Second
	)
	
	st.metrics = &StressMetrics{}
	
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(st.ctx, duration)
	defer cancel()
	
	start := time.Now()
	
	// 극한 동시성으로 리소스 고갈 유도
	for i := 0; i < extremeConcurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// 메모리 사용량 증가
			buffer := make([]byte, 1024*100) // 100KB per goroutine
			
			for j := 0; j < 10; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					reqStart := time.Now()
					
					key := fmt.Sprintf("exhaustion_%d_%d", workerID, j)
					value := fmt.Sprintf("%s_%d", string(buffer[:100]), j)
					
					err := st.cacheService.Set(st.ctx, key, value, time.Minute)
					responseTime := time.Since(reqStart)
					
					st.metrics.RecordRequest(err == nil, responseTime)
					
					time.Sleep(time.Millisecond * 100)
				}
			}
		}(i)
	}
	
	wg.Wait()
	actualDuration := time.Since(start)
	
	st.metrics.CalculateMetrics()
	
	t.Logf("=== Resource Exhaustion Recovery Results ===")
	t.Logf("Extreme Concurrency: %d goroutines", extremeConcurrency)
	t.Logf("Duration: %v", actualDuration)
	t.Logf("Total Requests: %d", st.metrics.TotalRequests)
	t.Logf("Error Rate: %.2f%%", st.metrics.ErrorRate)
	t.Logf("Average Response Time: %v", st.metrics.AvgResponseTime)
	
	// 시스템이 완전히 중단되지 않았는지 확인
	if st.metrics.TotalRequests > 0 {
		t.Logf("✅ System remained responsive under extreme load")
	} else {
		t.Logf("🔴 System became unresponsive")
	}
	
	assert.Greater(t, st.metrics.TotalRequests, int64(0), "System became completely unresponsive")
}

// TestStressSummary 스트레스 테스트 종합 보고서
func TestStressSummary(t *testing.T) {
	tester := NewStressTester(t)
	defer tester.cacheService.Close()
	
	t.Log("📊 Stress Test Summary Report")
	
	// 종합 메트릭 계산
	tester.metrics.CalculateMetrics()
	
	t.Logf("=== Overall Stress Test Performance ===")
	t.Logf("Peak Throughput: %.2f RPS", tester.metrics.PeakThroughput)
	t.Logf("Sustained Throughput: %.2f RPS", tester.metrics.SustainedThroughput)
	t.Logf("System Stability: %.2f%%", tester.metrics.SystemStability)
	t.Logf("Recovery Time: %v", tester.metrics.RecoveryTime)
	
	t.Logf("=== Error Analysis ===")
	t.Logf("Connection Errors: %d", tester.metrics.ConnectionErrors)
	t.Logf("Timeout Errors: %d", tester.metrics.TimeoutErrors)
	t.Logf("Memory Errors: %d", tester.metrics.MemoryErrors)
	t.Logf("Throttling Errors: %d", tester.metrics.ThrottlingErrors)
	
	// 확장성 분석
	if len(tester.metrics.MemoryUsage) > 0 && len(tester.metrics.ResponseTimes) > 0 {
		// 간단한 확장성 지수 계산
		avgMemory := uint64(0)
		for _, mem := range tester.metrics.MemoryUsage {
			avgMemory += mem
		}
		avgMemory /= uint64(len(tester.metrics.MemoryUsage))
		
		avgResponseTime := tester.metrics.AvgResponseTime.Nanoseconds()
		
		// 메모리 효율성 (낮을수록 좋음)
		memoryEfficiency := float64(avgMemory) / float64(tester.metrics.TotalRequests)
		
		// 응답 시간 효율성 (낮을수록 좋음)
		timeEfficiency := float64(avgResponseTime) / float64(tester.metrics.TotalRequests)
		
		// 종합 확장성 지수 (높을수록 좋음)
		tester.metrics.ScalabilityIndex = 100.0 / (memoryEfficiency + timeEfficiency) * 1000000
		
		t.Logf("=== Scalability Analysis ===")
		t.Logf("Scalability Index: %.2f", tester.metrics.ScalabilityIndex)
		t.Logf("Memory Efficiency: %.2f bytes/request", memoryEfficiency)
		t.Logf("Time Efficiency: %.2f ns/request", timeEfficiency)
	}
	
	// 병목 지점 식별
	bottlenecks := []string{}
	if tester.metrics.MemoryErrors > 0 {
		bottlenecks = append(bottlenecks, "Memory")
	}
	if tester.metrics.ConnectionErrors > tester.metrics.TimeoutErrors {
		bottlenecks = append(bottlenecks, "Connection Pool")
	}
	if tester.metrics.TimeoutErrors > 0 {
		bottlenecks = append(bottlenecks, "Network/Processing")
	}
	
	tester.metrics.BottleneckComponents = bottlenecks
	
	t.Logf("=== Identified Bottlenecks ===")
	if len(bottlenecks) == 0 {
		t.Logf("✅ No significant bottlenecks identified")
	} else {
		for _, bottleneck := range bottlenecks {
			t.Logf("🔴 Bottleneck: %s", bottleneck)
		}
	}
	
	// 전체 평가
	if tester.metrics.SystemStability > 95.0 && tester.metrics.ScalabilityIndex > 50.0 {
		t.Logf("✅ Excellent stress test performance")
	} else if tester.metrics.SystemStability > 90.0 && tester.metrics.ScalabilityIndex > 25.0 {
		t.Logf("🟡 Good stress test performance")
	} else {
		t.Logf("🔴 Poor stress test performance - optimization needed")
	}
	
	// 개선 권고사항
	t.Logf("=== Improvement Recommendations ===")
	if tester.metrics.MemoryErrors > 0 {
		t.Logf("🔴 Implement memory management and garbage collection optimization")
	}
	if tester.metrics.ConnectionErrors > 50 {
		t.Logf("🔴 Increase connection pool size and implement connection retry logic")
	}
	if tester.metrics.TimeoutErrors > 50 {
		t.Logf("🟡 Optimize processing logic and consider async patterns")
	}
	if tester.metrics.RecoveryTime > 30*time.Second {
		t.Logf("🟡 Implement faster failure detection and recovery mechanisms")
	}
}