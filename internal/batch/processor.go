package batch

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mutual-friend/internal/cache"
	"mutual-friend/internal/domain"
	"mutual-friend/internal/repository"
	"mutual-friend/pkg/events"
)

// ProcessorConfig contains configuration for the batch processor
type ProcessorConfig struct {
	BatchSize         int           // Maximum events in a batch (default: 1000)
	BatchTimeout      time.Duration // Maximum time to wait for batch (default: 2s)
	WorkerPoolSize    int           // Number of worker goroutines (default: 20)
	EventChannelSize  int           // Size of event channel buffer (default: 10000)
	MaxRetries        int           // Maximum retry attempts for failed batches
	RetryDelay        time.Duration // Delay between retries
}

// DefaultProcessorConfig returns default configuration
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		BatchSize:        1000,
		BatchTimeout:     2 * time.Second,
		WorkerPoolSize:   20,
		EventChannelSize: 10000,
		MaxRetries:       3,
		RetryDelay:       1 * time.Second,
	}
}

// BatchProcessor handles event batching and processing using goroutine pipelines
type BatchProcessor struct {
	config           ProcessorConfig
	eventChan        chan *events.FriendEvent
	batchChan        chan []*events.FriendEvent
	friendRepo       *repository.FriendRepository
	recommendationSvc *RecommendationService
	cacheService     cache.Service
	workerWg         sync.WaitGroup
	shutdownChan     chan struct{}
	metrics          *ProcessorMetrics
	logger           *log.Logger
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(
	config ProcessorConfig,
	friendRepo *repository.FriendRepository,
	cacheService cache.Service,
	logger *log.Logger,
) *BatchProcessor {
	if config.BatchSize == 0 {
		config = DefaultProcessorConfig()
	}

	return &BatchProcessor{
		config:           config,
		eventChan:        make(chan *events.FriendEvent, config.EventChannelSize),
		batchChan:        make(chan []*events.FriendEvent, config.WorkerPoolSize),
		friendRepo:       friendRepo,
		recommendationSvc: NewRecommendationService(friendRepo),
		cacheService:     cacheService,
		shutdownChan:     make(chan struct{}),
		metrics:          NewProcessorMetrics(),
		logger:           logger,
	}
}

// Start begins the batch processing pipeline
func (bp *BatchProcessor) Start(ctx context.Context) error {
	bp.logger.Printf("Starting batch processor with config: %+v", bp.config)

	// Start batch collector goroutine
	go bp.batchCollector(ctx)

	// Start worker pool
	for i := 0; i < bp.config.WorkerPoolSize; i++ {
		bp.workerWg.Add(1)
		go bp.worker(ctx, i)
	}

	// Start metrics reporter
	go bp.reportMetrics(ctx)

	bp.logger.Println("Batch processor started successfully")
	return nil
}

// ProcessEvent adds an event to the processing pipeline
func (bp *BatchProcessor) ProcessEvent(event *events.FriendEvent) error {
	select {
	case bp.eventChan <- event:
		bp.metrics.IncrementEventsIngested()
		return nil
	default:
		bp.metrics.IncrementEventsDropped()
		return fmt.Errorf("event channel full, dropping event")
	}
}

// batchCollector collects events and creates batches based on size or timeout
func (bp *BatchProcessor) batchCollector(ctx context.Context) {
	ticker := time.NewTicker(bp.config.BatchTimeout)
	defer ticker.Stop()

	batch := make([]*events.FriendEvent, 0, bp.config.BatchSize)
	
	for {
		select {
		case event := <-bp.eventChan:
			batch = append(batch, event)
			
			// Check if batch is full
			if len(batch) >= bp.config.BatchSize {
				bp.sendBatch(batch)
				batch = make([]*events.FriendEvent, 0, bp.config.BatchSize)
				ticker.Reset(bp.config.BatchTimeout)
			}
			
		case <-ticker.C:
			// Timeout reached, send whatever we have
			if len(batch) > 0 {
				bp.sendBatch(batch)
				batch = make([]*events.FriendEvent, 0, bp.config.BatchSize)
			}
			
		case <-ctx.Done():
			// Graceful shutdown - process remaining events
			if len(batch) > 0 {
				bp.sendBatch(batch)
			}
			close(bp.batchChan)
			return
			
		case <-bp.shutdownChan:
			// Immediate shutdown
			close(bp.batchChan)
			return
		}
	}
}

// sendBatch sends a batch to the processing channel
func (bp *BatchProcessor) sendBatch(batch []*events.FriendEvent) {
	batchCopy := make([]*events.FriendEvent, len(batch))
	copy(batchCopy, batch)
	
	select {
	case bp.batchChan <- batchCopy:
		bp.metrics.IncrementBatchesCreated(len(batchCopy))
		bp.logger.Printf("Created batch with %d events", len(batchCopy))
	default:
		bp.metrics.IncrementBatchesDropped()
		bp.logger.Printf("WARNING: Batch channel full, dropping batch of %d events", len(batchCopy))
	}
}

// worker processes batches from the batch channel
func (bp *BatchProcessor) worker(ctx context.Context, workerID int) {
	defer bp.workerWg.Done()
	
	bp.logger.Printf("Worker %d started", workerID)
	
	for batch := range bp.batchChan {
		startTime := time.Now()
		
		err := bp.processBatchWithRetry(ctx, batch, workerID)
		if err != nil {
			bp.metrics.IncrementProcessingErrors()
			bp.logger.Printf("Worker %d: Failed to process batch after retries: %v", workerID, err)
		} else {
			bp.metrics.RecordBatchProcessingTime(time.Since(startTime))
			bp.logger.Printf("Worker %d: Successfully processed batch of %d events in %v", 
				workerID, len(batch), time.Since(startTime))
		}
	}
	
	bp.logger.Printf("Worker %d stopped", workerID)
}

// processBatchWithRetry processes a batch with retry logic
func (bp *BatchProcessor) processBatchWithRetry(ctx context.Context, batch []*events.FriendEvent, workerID int) error {
	var lastErr error
	
	for attempt := 0; attempt <= bp.config.MaxRetries; attempt++ {
		if attempt > 0 {
			bp.metrics.IncrementRetryAttempts()
			time.Sleep(bp.config.RetryDelay)
			bp.logger.Printf("Worker %d: Retry attempt %d for batch", workerID, attempt)
		}
		
		err := bp.processBatch(ctx, batch, workerID)
		if err == nil {
			return nil
		}
		
		lastErr = err
		bp.logger.Printf("Worker %d: Batch processing failed (attempt %d): %v", workerID, attempt+1, err)
	}
	
	return fmt.Errorf("batch processing failed after %d attempts: %w", bp.config.MaxRetries+1, lastErr)
}

// processBatch processes a single batch of events
func (bp *BatchProcessor) processBatch(ctx context.Context, batch []*events.FriendEvent, workerID int) error {
	// Group events by user for efficient processing
	userEvents := bp.groupEventsByUser(batch)
	
	// Process each user's events
	for userID, events := range userEvents {
		if err := bp.processUserEvents(ctx, userID, events, workerID); err != nil {
			// In the real implementation mentioned in the portfolio,
			// a single error here would cause the entire batch to be retried.
			// This is one of the limitations that was identified.
			return fmt.Errorf("failed to process events for user %s: %w", userID, err)
		}
	}
	
	bp.metrics.IncrementBatchesCompleted()
	return nil
}

// groupEventsByUser groups events by affected users (including friends)
func (bp *BatchProcessor) groupEventsByUser(batch []*events.FriendEvent) map[string][]*events.FriendEvent {
	userEvents := make(map[string][]*events.FriendEvent)
	
	for _, event := range batch {
		// Each friend event affects both users
		userEvents[event.UserID] = append(userEvents[event.UserID], event)
		userEvents[event.FriendID] = append(userEvents[event.FriendID], event)
	}
	
	return userEvents
}

// processUserEvents processes all events for a specific user
func (bp *BatchProcessor) processUserEvents(ctx context.Context, userID string, events []*events.FriendEvent, workerID int) error {
	bp.logger.Printf("Worker %d: Processing %d events for user %s", workerID, len(events), userID)
	
	// Calculate 2-hop recommendations for the user
	recommendations, err := bp.recommendationSvc.Calculate2HopRecommendations(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to calculate recommendations: %w", err)
	}
	
	// Save recommendations to DynamoDB
	if err := bp.recommendationSvc.SaveRecommendations(ctx, userID, recommendations); err != nil {
		return fmt.Errorf("failed to save recommendations: %w", err)
	}
	
	// Update cache
	if err := bp.updateCache(ctx, userID, recommendations); err != nil {
		// Cache errors are non-fatal but should be logged
		bp.logger.Printf("Worker %d: Failed to update cache for user %s: %v", workerID, userID, err)
	}
	
	bp.metrics.IncrementUsersProcessed()
	return nil
}

// updateCache updates the cache with new recommendations
func (bp *BatchProcessor) updateCache(ctx context.Context, userID string, recommendations []domain.Recommendation) error {
	// Invalidate old recommendations
	if err := bp.cacheService.Delete(ctx, fmt.Sprintf("recommendations:%s", userID)); err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}
	
	// Cache new recommendations with TTL
	if len(recommendations) > 0 {
		ttl := 1 * time.Hour
		if err := bp.cacheService.Set(ctx, fmt.Sprintf("recommendations:%s", userID), recommendations, ttl); err != nil {
			return fmt.Errorf("failed to cache recommendations: %w", err)
		}
	}
	
	return nil
}

// Stop gracefully shuts down the batch processor
func (bp *BatchProcessor) Stop(ctx context.Context) error {
	bp.logger.Println("Stopping batch processor...")
	
	// Stop accepting new events
	close(bp.eventChan)
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		bp.workerWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		bp.logger.Println("Batch processor stopped gracefully")
		return nil
	case <-ctx.Done():
		// Force shutdown
		close(bp.shutdownChan)
		return fmt.Errorf("batch processor shutdown timeout")
	}
}

// GetMetrics returns current processor metrics
func (bp *BatchProcessor) GetMetrics() ProcessorMetricsSnapshot {
	return bp.metrics.GetSnapshot()
}

// reportMetrics periodically logs processor metrics
func (bp *BatchProcessor) reportMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			metrics := bp.metrics.GetSnapshot()
			bp.logger.Printf("Processor Metrics: %+v", metrics)
			
			// Check for potential issues
			if metrics.EventChannelUtilization > 0.8 {
				bp.logger.Printf("WARNING: Event channel utilization high: %.2f%%", 
					metrics.EventChannelUtilization*100)
			}
			
			if metrics.GoroutineCount > bp.config.WorkerPoolSize*2 {
				bp.logger.Printf("WARNING: Goroutine count higher than expected: %d", 
					metrics.GoroutineCount)
			}
			
		case <-ctx.Done():
			return
		}
	}
}