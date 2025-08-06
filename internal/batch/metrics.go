package batch

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ProcessorMetrics tracks metrics for the batch processor
type ProcessorMetrics struct {
	// Counters
	eventsIngested   atomic.Uint64
	eventsDropped    atomic.Uint64
	batchesCreated   atomic.Uint64
	batchesCompleted atomic.Uint64
	batchesDropped   atomic.Uint64
	usersProcessed   atomic.Uint64
	processingErrors atomic.Uint64
	retryAttempts    atomic.Uint64
	
	// Timing metrics
	batchProcessingTimes sync.Map // map[time.Time]time.Duration
	
	// Resource metrics
	lastGoroutineCount int
	lastMemoryUsage    uint64
}

// ProcessorMetricsSnapshot represents a point-in-time snapshot of metrics
type ProcessorMetricsSnapshot struct {
	EventsIngested          uint64
	EventsDropped           uint64
	BatchesCreated          uint64
	BatchesCompleted        uint64
	BatchesDropped          uint64
	UsersProcessed          uint64
	ProcessingErrors        uint64
	RetryAttempts           uint64
	AvgBatchProcessingTime  time.Duration
	GoroutineCount          int
	MemoryUsageMB           float64
	EventChannelUtilization float64
	
	// Calculated rates
	EventsPerSecond   float64
	BatchesPerSecond  float64
	ErrorRate         float64
	RetryRate         float64
}

// NewProcessorMetrics creates a new metrics tracker
func NewProcessorMetrics() *ProcessorMetrics {
	return &ProcessorMetrics{}
}

// IncrementEventsIngested increments the events ingested counter
func (m *ProcessorMetrics) IncrementEventsIngested() {
	m.eventsIngested.Add(1)
}

// IncrementEventsDropped increments the events dropped counter
func (m *ProcessorMetrics) IncrementEventsDropped() {
	m.eventsDropped.Add(1)
}

// IncrementBatchesCreated increments the batches created counter
func (m *ProcessorMetrics) IncrementBatchesCreated(batchSize int) {
	m.batchesCreated.Add(1)
}

// IncrementBatchesCompleted increments the batches completed counter
func (m *ProcessorMetrics) IncrementBatchesCompleted() {
	m.batchesCompleted.Add(1)
}

// IncrementBatchesDropped increments the batches dropped counter
func (m *ProcessorMetrics) IncrementBatchesDropped() {
	m.batchesDropped.Add(1)
}

// IncrementUsersProcessed increments the users processed counter
func (m *ProcessorMetrics) IncrementUsersProcessed() {
	m.usersProcessed.Add(1)
}

// IncrementProcessingErrors increments the processing errors counter
func (m *ProcessorMetrics) IncrementProcessingErrors() {
	m.processingErrors.Add(1)
}

// IncrementRetryAttempts increments the retry attempts counter
func (m *ProcessorMetrics) IncrementRetryAttempts() {
	m.retryAttempts.Add(1)
}

// RecordBatchProcessingTime records the time taken to process a batch
func (m *ProcessorMetrics) RecordBatchProcessingTime(duration time.Duration) {
	m.batchProcessingTimes.Store(time.Now(), duration)
	
	// Clean up old entries (keep last 100)
	count := 0
	m.batchProcessingTimes.Range(func(key, value interface{}) bool {
		count++
		if count > 100 {
			m.batchProcessingTimes.Delete(key)
		}
		return true
	})
}

// GetSnapshot returns a snapshot of current metrics
func (m *ProcessorMetrics) GetSnapshot() ProcessorMetricsSnapshot {
	// Update resource metrics
	m.lastGoroutineCount = runtime.NumGoroutine()
	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.lastMemoryUsage = memStats.Alloc
	
	// Calculate average batch processing time
	var totalDuration time.Duration
	var count int
	m.batchProcessingTimes.Range(func(key, value interface{}) bool {
		if duration, ok := value.(time.Duration); ok {
			totalDuration += duration
			count++
		}
		return true
	})
	
	avgDuration := time.Duration(0)
	if count > 0 {
		avgDuration = totalDuration / time.Duration(count)
	}
	
	// Calculate rates
	eventsIngested := m.eventsIngested.Load()
	batchesCompleted := m.batchesCompleted.Load()
	processingErrors := m.processingErrors.Load()
	retryAttempts := m.retryAttempts.Load()
	
	errorRate := float64(0)
	if batchesCompleted > 0 {
		errorRate = float64(processingErrors) / float64(batchesCompleted)
	}
	
	retryRate := float64(0)
	if batchesCompleted > 0 {
		retryRate = float64(retryAttempts) / float64(batchesCompleted)
	}
	
	return ProcessorMetricsSnapshot{
		EventsIngested:         eventsIngested,
		EventsDropped:          m.eventsDropped.Load(),
		BatchesCreated:         m.batchesCreated.Load(),
		BatchesCompleted:       batchesCompleted,
		BatchesDropped:         m.batchesDropped.Load(),
		UsersProcessed:         m.usersProcessed.Load(),
		ProcessingErrors:       processingErrors,
		RetryAttempts:          retryAttempts,
		AvgBatchProcessingTime: avgDuration,
		GoroutineCount:         m.lastGoroutineCount,
		MemoryUsageMB:          float64(m.lastMemoryUsage) / 1024 / 1024,
		ErrorRate:              errorRate,
		RetryRate:              retryRate,
	}
}

// LimitationAnalysis represents analysis of known limitations
type LimitationAnalysis struct {
	NackCascadeIssues    NackCascadeAnalysis
	GoroutineOverhead    GoroutineOverheadAnalysis
	MemoryUsageAnalysis  MemoryUsageAnalysis
	ThroughputAnalysis   ThroughputAnalysis
}

// NackCascadeAnalysis analyzes the nack cascade problem
type NackCascadeAnalysis struct {
	BatchFailureRate     float64
	CascadeFailures      int
	ReprocessingOverhead time.Duration
	Recommendation       string
}

// GoroutineOverheadAnalysis analyzes goroutine overhead
type GoroutineOverheadAnalysis struct {
	GoroutineCount   int
	MemoryUsage      uint64
	ContextSwitching float64
	ScalingFactor    float64
	Recommendation   string
}

// MemoryUsageAnalysis analyzes memory usage patterns
type MemoryUsageAnalysis struct {
	HeapAlloc       uint64
	HeapInuse       uint64
	StackInuse      uint64
	GCPauseTime     time.Duration
	MemoryLeakRisk  bool
	Recommendation  string
}

// ThroughputAnalysis analyzes throughput limitations
type ThroughputAnalysis struct {
	CurrentThroughput    float64
	MaxThroughput        float64
	BottleneckComponent  string
	ScalabilityLimit     int
	Recommendation       string
}

// AnalyzeLimitations performs analysis of known limitations
func (m *ProcessorMetrics) AnalyzeLimitations() LimitationAnalysis {
	snapshot := m.GetSnapshot()
	
	return LimitationAnalysis{
		NackCascadeIssues: m.analyzeNackCascade(snapshot),
		GoroutineOverhead: m.analyzeGoroutineOverhead(snapshot),
		MemoryUsageAnalysis: m.analyzeMemoryUsage(),
		ThroughputAnalysis: m.analyzeThroughput(snapshot),
	}
}

// analyzeNackCascade analyzes the nack cascade problem
func (m *ProcessorMetrics) analyzeNackCascade(snapshot ProcessorMetricsSnapshot) NackCascadeAnalysis {
	// Calculate cascade impact
	cascadeFailures := int(snapshot.RetryAttempts)
	reprocessingOverhead := time.Duration(float64(snapshot.AvgBatchProcessingTime) * snapshot.RetryRate)
	
	recommendation := "Consider implementing message-level acknowledgment instead of batch-level"
	if snapshot.ErrorRate > 0.1 {
		recommendation = "CRITICAL: High error rate causing excessive batch retries. " + recommendation
	}
	
	return NackCascadeAnalysis{
		BatchFailureRate:     snapshot.ErrorRate,
		CascadeFailures:      cascadeFailures,
		ReprocessingOverhead: reprocessingOverhead,
		Recommendation:       recommendation,
	}
}

// analyzeGoroutineOverhead analyzes goroutine overhead
func (m *ProcessorMetrics) analyzeGoroutineOverhead(snapshot ProcessorMetricsSnapshot) GoroutineOverheadAnalysis {
	// Calculate scaling factor (goroutines per event)
	scalingFactor := float64(snapshot.GoroutineCount) / float64(snapshot.EventsIngested+1)
	
	recommendation := "Goroutine count within acceptable range"
	if snapshot.GoroutineCount > 1000 {
		recommendation = "WARNING: High goroutine count detected. Consider using worker pool pattern"
	}
	if scalingFactor > 0.01 {
		recommendation = "CRITICAL: Goroutine scaling issue detected. " + recommendation
	}
	
	return GoroutineOverheadAnalysis{
		GoroutineCount:   snapshot.GoroutineCount,
		MemoryUsage:      uint64(snapshot.MemoryUsageMB * 1024 * 1024),
		ContextSwitching: float64(snapshot.GoroutineCount) * 0.001, // Approximate overhead
		ScalingFactor:    scalingFactor,
		Recommendation:   recommendation,
	}
}

// analyzeMemoryUsage analyzes memory usage patterns
func (m *ProcessorMetrics) analyzeMemoryUsage() MemoryUsageAnalysis {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Simple leak detection based on heap growth
	memoryLeakRisk := memStats.HeapAlloc > 500*1024*1024 // 500MB threshold
	
	recommendation := "Memory usage normal"
	if memoryLeakRisk {
		recommendation = "WARNING: Potential memory leak detected. Check for unreleased resources"
	}
	
	return MemoryUsageAnalysis{
		HeapAlloc:      memStats.HeapAlloc,
		HeapInuse:      memStats.HeapInuse,
		StackInuse:     memStats.StackInuse,
		GCPauseTime:    time.Duration(memStats.PauseTotalNs),
		MemoryLeakRisk: memoryLeakRisk,
		Recommendation: recommendation,
	}
}

// analyzeThroughput analyzes throughput limitations
func (m *ProcessorMetrics) analyzeThroughput(snapshot ProcessorMetricsSnapshot) ThroughputAnalysis {
	// Estimate current throughput (events per second)
	// This is simplified - in reality would track over time windows
	currentThroughput := float64(snapshot.EventsIngested) / 60.0 // Assume 60 second window
	
	// Theoretical max based on batch size and processing time
	batchesPerSecond := 1.0 / snapshot.AvgBatchProcessingTime.Seconds()
	maxThroughput := batchesPerSecond * 1000 // Assuming 1000 events per batch
	
	bottleneck := "None identified"
	if snapshot.EventChannelUtilization > 0.8 {
		bottleneck = "Event channel capacity"
	} else if snapshot.ErrorRate > 0.05 {
		bottleneck = "Processing errors causing retries"
	} else if snapshot.GoroutineCount > 500 {
		bottleneck = "Goroutine scheduling overhead"
	}
	
	recommendation := "System operating within capacity"
	if currentThroughput > maxThroughput*0.8 {
		recommendation = "WARNING: Approaching throughput limit. Consider scaling horizontally"
	}
	
	return ThroughputAnalysis{
		CurrentThroughput:   currentThroughput,
		MaxThroughput:       maxThroughput,
		BottleneckComponent: bottleneck,
		ScalabilityLimit:    int(maxThroughput),
		Recommendation:      recommendation,
	}
}