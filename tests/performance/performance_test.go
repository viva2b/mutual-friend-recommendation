package performance

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mutual-friend/pkg/cache"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/redis"
)

// PerformanceBenchmark 성능 벤치마크 구조체
type PerformanceBenchmark struct {
	cacheService cache.Cache
	ctx          context.Context
	metrics      *PerformanceMetrics
}

// PerformanceMetrics 상세 성능 메트릭
type PerformanceMetrics struct {
	mu               sync.RWMutex
	
	// 응답 시간 메트릭
	ResponseTimes    []time.Duration
	P50Latency       time.Duration
	P95Latency       time.Duration
	P99Latency       time.Duration
	MaxLatency       time.Duration
	MinLatency       time.Duration
	
	// 처리량 메트릭
	TotalOperations  int64
	SuccessfulOps    int64
	FailedOps        int64
	OpsPerSecond     float64
	
	// 리소스 사용량
	MemoryBefore     runtime.MemStats
	MemoryAfter      runtime.MemStats
	GoroutinesBefore int
	GoroutinesAfter  int
	
	// 캐시 메트릭
	CacheHits        int64
	CacheMisses      int64
	CacheErrors      int64
	
	// 네트워크 메트릭
	NetworkCalls     int64
	NetworkLatency   []time.Duration
	
	// 동시성 메트릭
	ConcurrentUsers  int
	ContentionCount  int64
	DeadlockCount    int64
}

func (pm *PerformanceMetrics) AddResponseTime(duration time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.ResponseTimes = append(pm.ResponseTimes, duration)
}

func (pm *PerformanceMetrics) CalculatePercentiles() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if len(pm.ResponseTimes) == 0 {
		return
	}
	
	// Sort response times
	times := make([]time.Duration, len(pm.ResponseTimes))
	copy(times, pm.ResponseTimes)
	
	// Simple bubble sort for small datasets
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[i] > times[j] {
				times[i], times[j] = times[j], times[i]
			}
		}
	}
	
	// Calculate percentiles
	pm.MinLatency = times[0]
	pm.MaxLatency = times[len(times)-1]
	pm.P50Latency = times[len(times)/2]
	pm.P95Latency = times[int(float64(len(times))*0.95)]
	pm.P99Latency = times[int(float64(len(times))*0.99)]
}

func (pm *PerformanceMetrics) GetThroughput(duration time.Duration) float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return float64(pm.TotalOperations) / duration.Seconds()
}

// NewPerformanceBenchmark 성능 벤치마크 생성
func NewPerformanceBenchmark(t *testing.T) *PerformanceBenchmark {
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
	
	return &PerformanceBenchmark{
		cacheService: cacheService,
		ctx:          context.Background(),
		metrics:      &PerformanceMetrics{},
	}
}

// BenchmarkRedisOperations Redis 기본 연산 벤치마크
func BenchmarkRedisOperations(b *testing.B) {
	benchmark := NewPerformanceBenchmark(&testing.T{})
	defer benchmark.cacheService.Close()
	
	// 메모리 상태 기록
	runtime.GC()
	runtime.ReadMemStats(&benchmark.metrics.MemoryBefore)
	benchmark.metrics.GoroutinesBefore = runtime.NumGoroutine()
	
	b.ResetTimer()
	start := time.Now()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("benchmark_key_%d", i)
			value := fmt.Sprintf("benchmark_value_%d", i)
			
			// SET 연산
			setStart := time.Now()
			err := benchmark.cacheService.Set(benchmark.ctx, key, value, time.Minute)
			setDuration := time.Since(setStart)
			
			benchmark.metrics.AddResponseTime(setDuration)
			benchmark.metrics.TotalOperations++
			
			if err != nil {
				benchmark.metrics.FailedOps++
				benchmark.metrics.CacheErrors++
			} else {
				benchmark.metrics.SuccessfulOps++
			}
			
			// GET 연산
			getStart := time.Now()
			_, err = benchmark.cacheService.Get(benchmark.ctx, key)
			getDuration := time.Since(getStart)
			
			benchmark.metrics.AddResponseTime(getDuration)
			benchmark.metrics.TotalOperations++
			
			if err != nil {
				benchmark.metrics.FailedOps++
				benchmark.metrics.CacheMisses++
			} else {
				benchmark.metrics.SuccessfulOps++
				benchmark.metrics.CacheHits++
			}
			
			i++
		}
	})
	
	duration := time.Since(start)
	
	// 최종 메모리 상태 기록
	runtime.GC()
	runtime.ReadMemStats(&benchmark.metrics.MemoryAfter)
	benchmark.metrics.GoroutinesAfter = runtime.NumGoroutine()
	
	// 성능 메트릭 계산
	benchmark.metrics.CalculatePercentiles()
	benchmark.metrics.OpsPerSecond = benchmark.metrics.GetThroughput(duration)
	
	// 결과 출력
	benchmark.PrintBenchmarkResults(b, duration)
}

// BenchmarkConcurrentAccess 동시 접근 벤치마크
func BenchmarkConcurrentAccess(b *testing.B) {
	benchmark := NewPerformanceBenchmark(&testing.T{})
	defer benchmark.cacheService.Close()
	
	concurrencyLevels := []int{1, 10, 50, 100, 200}
	
	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			benchmark.runConcurrencyBenchmark(b, concurrency)
		})
	}
}

func (pb *PerformanceBenchmark) runConcurrencyBenchmark(b *testing.B, concurrency int) {
	pb.metrics = &PerformanceMetrics{ConcurrentUsers: concurrency}
	
	// 공통 키로 경합 상황 생성
	commonKey := "contention_test_key"
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	b.ResetTimer()
	start := time.Now()
	
	// 동시성 워커 실행
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < b.N/concurrency; j++ {
				// 경합 상황 시뮬레이션
				opStart := time.Now()
				
				// 읽기 시도
				_, err := pb.cacheService.Get(pb.ctx, commonKey)
				if err != nil {
					// 캐시 미스 시 쓰기 시도 (경합 발생)
					value := fmt.Sprintf("worker_%d_iteration_%d", workerID, j)
					err = pb.cacheService.Set(pb.ctx, commonKey, value, time.Minute)
					
					if err != nil {
						mu.Lock()
						pb.metrics.ContentionCount++
						mu.Unlock()
					}
				}
				
				opDuration := time.Since(opStart)
				
				mu.Lock()
				pb.metrics.AddResponseTime(opDuration)
				pb.metrics.TotalOperations++
				if err != nil {
					pb.metrics.FailedOps++
				} else {
					pb.metrics.SuccessfulOps++
				}
				mu.Unlock()
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	pb.metrics.CalculatePercentiles()
	pb.metrics.OpsPerSecond = pb.metrics.GetThroughput(duration)
	
	// 동시성 결과 출력
	pb.PrintConcurrencyResults(b, concurrency, duration)
}

// BenchmarkDataSizes 데이터 크기별 벤치마크
func BenchmarkDataSizes(b *testing.B) {
	benchmark := NewPerformanceBenchmark(&testing.T{})
	defer benchmark.cacheService.Close()
	
	dataSizes := []struct {
		name string
		size int
	}{
		{"100B", 100},
		{"1KB", 1024},
		{"10KB", 10240},
		{"100KB", 102400},
		{"1MB", 1048576},
	}
	
	for _, ds := range dataSizes {
		b.Run(ds.name, func(b *testing.B) {
			benchmark.runDataSizeBenchmark(b, ds.size)
		})
	}
}

func (pb *PerformanceBenchmark) runDataSizeBenchmark(b *testing.B, dataSize int) {
	pb.metrics = &PerformanceMetrics{}
	
	// 테스트 데이터 생성
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	dataString := string(data)
	
	b.ResetTimer()
	start := time.Now()
	
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("size_test_%d", i)
		
		// SET 연산 측정
		setStart := time.Now()
		err := pb.cacheService.Set(pb.ctx, key, dataString, time.Minute)
		setDuration := time.Since(setStart)
		
		pb.metrics.AddResponseTime(setDuration)
		pb.metrics.TotalOperations++
		
		if err != nil {
			pb.metrics.FailedOps++
			continue
		}
		pb.metrics.SuccessfulOps++
		
		// GET 연산 측정
		getStart := time.Now()
		_, err = pb.cacheService.Get(pb.ctx, key)
		getDuration := time.Since(getStart)
		
		pb.metrics.AddResponseTime(getDuration)
		pb.metrics.TotalOperations++
		
		if err != nil {
			pb.metrics.FailedOps++
		} else {
			pb.metrics.SuccessfulOps++
		}
	}
	
	duration := time.Since(start)
	
	pb.metrics.CalculatePercentiles()
	pb.metrics.OpsPerSecond = pb.metrics.GetThroughput(duration)
	
	// 데이터 크기별 결과 출력
	pb.PrintDataSizeResults(b, dataSize, duration)
}

// BenchmarkMemoryUsage 메모리 사용량 벤치마크
func BenchmarkMemoryUsage(b *testing.B) {
	benchmark := NewPerformanceBenchmark(&testing.T{})
	defer benchmark.cacheService.Close()
	
	entryCount := []int{1000, 10000, 50000, 100000}
	entrySize := 1024 // 1KB per entry
	
	for _, count := range entryCount {
		b.Run(fmt.Sprintf("Entries_%d", count), func(b *testing.B) {
			benchmark.runMemoryUsageBenchmark(b, count, entrySize)
		})
	}
}

func (pb *PerformanceBenchmark) runMemoryUsageBenchmark(b *testing.B, entryCount, entrySize int) {
	// 메모리 상태 초기화
	runtime.GC()
	runtime.ReadMemStats(&pb.metrics.MemoryBefore)
	
	// 테스트 데이터 생성
	data := make([]byte, entrySize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	dataString := string(data)
	
	b.ResetTimer()
	start := time.Now()
	
	// 대량 데이터 삽입
	for i := 0; i < entryCount; i++ {
		key := fmt.Sprintf("memory_test_%d", i)
		err := pb.cacheService.Set(pb.ctx, key, dataString, time.Hour)
		
		if err != nil {
			pb.metrics.FailedOps++
		} else {
			pb.metrics.SuccessfulOps++
		}
		pb.metrics.TotalOperations++
	}
	
	duration := time.Since(start)
	
	// 메모리 사용량 측정
	runtime.GC()
	runtime.ReadMemStats(&pb.metrics.MemoryAfter)
	
	pb.metrics.OpsPerSecond = pb.metrics.GetThroughput(duration)
	
	// 메모리 사용량 결과 출력
	pb.PrintMemoryUsageResults(b, entryCount, entrySize, duration)
}

// PrintBenchmarkResults 벤치마크 결과 출력
func (pb *PerformanceBenchmark) PrintBenchmarkResults(b *testing.B, duration time.Duration) {
	b.Logf("=== Performance Benchmark Results ===")
	b.Logf("Total Operations: %d", pb.metrics.TotalOperations)
	b.Logf("Successful Operations: %d", pb.metrics.SuccessfulOps)
	b.Logf("Failed Operations: %d", pb.metrics.FailedOps)
	b.Logf("Success Rate: %.2f%%", float64(pb.metrics.SuccessfulOps)/float64(pb.metrics.TotalOperations)*100)
	
	b.Logf("=== Latency Metrics ===")
	b.Logf("Min Latency: %v", pb.metrics.MinLatency)
	b.Logf("P50 Latency: %v", pb.metrics.P50Latency)
	b.Logf("P95 Latency: %v", pb.metrics.P95Latency)
	b.Logf("P99 Latency: %v", pb.metrics.P99Latency)
	b.Logf("Max Latency: %v", pb.metrics.MaxLatency)
	
	b.Logf("=== Throughput Metrics ===")
	b.Logf("Operations/Second: %.2f", pb.metrics.OpsPerSecond)
	b.Logf("Total Duration: %v", duration)
	
	b.Logf("=== Cache Metrics ===")
	b.Logf("Cache Hits: %d", pb.metrics.CacheHits)
	b.Logf("Cache Misses: %d", pb.metrics.CacheMisses)
	b.Logf("Cache Errors: %d", pb.metrics.CacheErrors)
	if pb.metrics.CacheHits+pb.metrics.CacheMisses > 0 {
		hitRate := float64(pb.metrics.CacheHits) / float64(pb.metrics.CacheHits+pb.metrics.CacheMisses) * 100
		b.Logf("Hit Rate: %.2f%%", hitRate)
	}
	
	b.Logf("=== Resource Usage ===")
	memDiff := pb.metrics.MemoryAfter.Alloc - pb.metrics.MemoryBefore.Alloc
	b.Logf("Memory Usage Delta: %d bytes", memDiff)
	goroutineDiff := pb.metrics.GoroutinesAfter - pb.metrics.GoroutinesBefore
	b.Logf("Goroutine Count Delta: %d", goroutineDiff)
}

// PrintConcurrencyResults 동시성 결과 출력
func (pb *PerformanceBenchmark) PrintConcurrencyResults(b *testing.B, concurrency int, duration time.Duration) {
	b.Logf("=== Concurrency Benchmark Results (Level: %d) ===", concurrency)
	b.Logf("Total Operations: %d", pb.metrics.TotalOperations)
	b.Logf("Operations/Second: %.2f", pb.metrics.OpsPerSecond)
	b.Logf("Contention Count: %d", pb.metrics.ContentionCount)
	b.Logf("Contention Rate: %.2f%%", float64(pb.metrics.ContentionCount)/float64(pb.metrics.TotalOperations)*100)
	
	b.Logf("=== Latency Under Concurrency ===")
	b.Logf("P50 Latency: %v", pb.metrics.P50Latency)
	b.Logf("P95 Latency: %v", pb.metrics.P95Latency)
	b.Logf("P99 Latency: %v", pb.metrics.P99Latency)
}

// PrintDataSizeResults 데이터 크기별 결과 출력
func (pb *PerformanceBenchmark) PrintDataSizeResults(b *testing.B, dataSize int, duration time.Duration) {
	b.Logf("=== Data Size Benchmark Results (%d bytes) ===", dataSize)
	b.Logf("Operations/Second: %.2f", pb.metrics.OpsPerSecond)
	b.Logf("Throughput (bytes/sec): %.2f", pb.metrics.OpsPerSecond*float64(dataSize))
	
	b.Logf("=== Size-Specific Latency ===")
	b.Logf("P50 Latency: %v", pb.metrics.P50Latency)
	b.Logf("P95 Latency: %v", pb.metrics.P95Latency)
	b.Logf("Latency per Byte: %.2f ns/byte", float64(pb.metrics.P50Latency.Nanoseconds())/float64(dataSize))
}

// PrintMemoryUsageResults 메모리 사용량 결과 출력
func (pb *PerformanceBenchmark) PrintMemoryUsageResults(b *testing.B, entryCount, entrySize int, duration time.Duration) {
	memDiff := pb.metrics.MemoryAfter.Alloc - pb.metrics.MemoryBefore.Alloc
	expectedMemory := uint64(entryCount * entrySize)
	overhead := float64(memDiff) / float64(expectedMemory)
	
	b.Logf("=== Memory Usage Benchmark Results ===")
	b.Logf("Entry Count: %d", entryCount)
	b.Logf("Entry Size: %d bytes", entrySize)
	b.Logf("Expected Memory: %d bytes", expectedMemory)
	b.Logf("Actual Memory Delta: %d bytes", memDiff)
	b.Logf("Memory Overhead: %.2fx", overhead)
	b.Logf("Memory per Entry: %.2f bytes", float64(memDiff)/float64(entryCount))
}

// TestPerformanceRegression 성능 회귀 테스트
func TestPerformanceRegression(t *testing.T) {
	benchmark := NewPerformanceBenchmark(t)
	defer benchmark.cacheService.Close()
	
	// 성능 기준선 정의
	performanceBaseline := struct {
		maxLatencyP95        time.Duration
		minOpsPerSecond     float64
		maxMemoryOverhead   float64
		maxErrorRate        float64
	}{
		maxLatencyP95:       10 * time.Millisecond,
		minOpsPerSecond:    1000.0,
		maxMemoryOverhead:  2.0,
		maxErrorRate:       1.0, // 1%
	}
	
	// 표준 워크로드 실행
	const testOperations = 10000
	start := time.Now()
	
	for i := 0; i < testOperations; i++ {
		key := fmt.Sprintf("regression_test_%d", i)
		value := fmt.Sprintf("test_value_%d", i)
		
		opStart := time.Now()
		err := benchmark.cacheService.Set(benchmark.ctx, key, value, time.Minute)
		opDuration := time.Since(opStart)
		
		benchmark.metrics.AddResponseTime(opDuration)
		benchmark.metrics.TotalOperations++
		
		if err != nil {
			benchmark.metrics.FailedOps++
		} else {
			benchmark.metrics.SuccessfulOps++
		}
		
		// 읽기 테스트
		opStart = time.Now()
		_, err = benchmark.cacheService.Get(benchmark.ctx, key)
		opDuration = time.Since(opStart)
		
		benchmark.metrics.AddResponseTime(opDuration)
		benchmark.metrics.TotalOperations++
		
		if err != nil {
			benchmark.metrics.FailedOps++
		} else {
			benchmark.metrics.SuccessfulOps++
		}
	}
	
	duration := time.Since(start)
	
	// 메트릭 계산
	benchmark.metrics.CalculatePercentiles()
	benchmark.metrics.OpsPerSecond = benchmark.metrics.GetThroughput(duration)
	errorRate := float64(benchmark.metrics.FailedOps) / float64(benchmark.metrics.TotalOperations) * 100
	
	// 성능 기준 검증
	t.Logf("=== Performance Regression Test Results ===")
	t.Logf("P95 Latency: %v (baseline: %v)", benchmark.metrics.P95Latency, performanceBaseline.maxLatencyP95)
	t.Logf("Ops/Second: %.2f (baseline: %.2f)", benchmark.metrics.OpsPerSecond, performanceBaseline.minOpsPerSecond)
	t.Logf("Error Rate: %.2f%% (baseline: %.2f%%)", errorRate, performanceBaseline.maxErrorRate)
	
	// 회귀 검증
	assert.Less(t, benchmark.metrics.P95Latency, performanceBaseline.maxLatencyP95, 
		"P95 latency regression detected")
	assert.Greater(t, benchmark.metrics.OpsPerSecond, performanceBaseline.minOpsPerSecond, 
		"Throughput regression detected")
	assert.Less(t, errorRate, performanceBaseline.maxErrorRate, 
		"Error rate regression detected")
	
	// 성능 개선 권고사항
	if benchmark.metrics.P95Latency > performanceBaseline.maxLatencyP95/2 {
		t.Logf("⚠️  Warning: Latency approaching threshold")
	}
	if benchmark.metrics.OpsPerSecond < performanceBaseline.minOpsPerSecond*1.5 {
		t.Logf("⚠️  Warning: Throughput below optimal range")
	}
}

// TestBottleneckIdentification 병목 지점 식별 테스트
func TestBottleneckIdentification(t *testing.T) {
	benchmark := NewPerformanceBenchmark(t)
	defer benchmark.cacheService.Close()
	
	t.Log("🔍 Bottleneck Identification Analysis")
	
	// 다양한 시나리오에서 병목 지점 식별
	scenarios := []struct {
		name        string
		operations  int
		concurrency int
		dataSize    int
	}{
		{"Light Load", 1000, 1, 100},
		{"Medium Load", 5000, 10, 1024},
		{"Heavy Load", 10000, 50, 10240},
		{"Extreme Load", 20000, 100, 102400},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			benchmark.identifyBottlenecks(t, scenario.operations, scenario.concurrency, scenario.dataSize)
		})
	}
}

func (pb *PerformanceBenchmark) identifyBottlenecks(t *testing.T, operations, concurrency, dataSize int) {
	pb.metrics = &PerformanceMetrics{ConcurrentUsers: concurrency}
	
	// 테스트 데이터 생성
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	dataString := string(data)
	
	// 병목 지점별 측정
	var (
		serializationTime   time.Duration
		networkTime        time.Duration
		processingTime     time.Duration
	)
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	start := time.Now()
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < operations/concurrency; j++ {
				key := fmt.Sprintf("bottleneck_%d_%d", workerID, j)
				
				// 직렬화 시간 측정 (근사치)
				serStart := time.Now()
				_ = len(dataString) // 직렬화 시뮬레이션
				serDuration := time.Since(serStart)
				
				// 네트워크 + 처리 시간 측정
				netStart := time.Now()
				err := pb.cacheService.Set(pb.ctx, key, dataString, time.Minute)
				netDuration := time.Since(netStart)
				
				// 처리 시간 = 전체 시간 - 직렬화 시간
				procDuration := netDuration - serDuration
				
				mu.Lock()
				serializationTime += serDuration
				networkTime += netDuration
				processingTime += procDuration
				
				pb.metrics.AddResponseTime(netDuration)
				pb.metrics.TotalOperations++
				
				if err != nil {
					pb.metrics.FailedOps++
				} else {
					pb.metrics.SuccessfulOps++
				}
				mu.Unlock()
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	// 병목 분석
	avgSerialization := serializationTime / time.Duration(operations)
	avgNetwork := networkTime / time.Duration(operations)
	avgProcessing := processingTime / time.Duration(operations)
	
	pb.metrics.CalculatePercentiles()
	pb.metrics.OpsPerSecond = pb.metrics.GetThroughput(duration)
	
	t.Logf("=== Bottleneck Analysis Results ===")
	t.Logf("Operations: %d, Concurrency: %d, Data Size: %d bytes", operations, concurrency, dataSize)
	t.Logf("Average Serialization Time: %v", avgSerialization)
	t.Logf("Average Network Time: %v", avgNetwork)
	t.Logf("Average Processing Time: %v", avgProcessing)
	t.Logf("Operations/Second: %.2f", pb.metrics.OpsPerSecond)
	
	// 병목 지점 식별
	totalTime := avgSerialization + avgNetwork + avgProcessing
	serializationPct := float64(avgSerialization) / float64(totalTime) * 100
	networkPct := float64(avgNetwork) / float64(totalTime) * 100
	processingPct := float64(avgProcessing) / float64(totalTime) * 100
	
	t.Logf("=== Bottleneck Distribution ===")
	t.Logf("Serialization: %.1f%%", serializationPct)
	t.Logf("Network: %.1f%%", networkPct)
	t.Logf("Processing: %.1f%%", processingPct)
	
	// 병목 지점 권고사항
	if serializationPct > 30 {
		t.Logf("🔴 Serialization bottleneck detected - consider data compression")
	}
	if networkPct > 50 {
		t.Logf("🔴 Network bottleneck detected - consider connection pooling")
	}
	if processingPct > 40 {
		t.Logf("🔴 Processing bottleneck detected - consider Redis optimization")
	}
}