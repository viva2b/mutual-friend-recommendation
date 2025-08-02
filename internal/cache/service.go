package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"mutual-friend/pkg/cache"
	"mutual-friend/pkg/redis"
)

// Service implements the cache.Cache interface using Redis
type Service struct {
	redisClient *redis.Client
	metrics     *cache.CacheMetrics
	mu          sync.RWMutex
}

// NewService creates a new cache service
func NewService(redisClient *redis.Client) *Service {
	return &Service{
		redisClient: redisClient,
		metrics: &cache.CacheMetrics{
			LastUpdated: time.Now(),
		},
	}
}

// Get retrieves a value from cache
func (s *Service) Get(ctx context.Context, key string) (string, error) {
	start := time.Now()
	defer s.updateLatency(start)

	value, err := s.redisClient.Get(ctx, key)
	if err != nil {
		s.incrementMiss()
		if err.Error() == fmt.Sprintf("key %s not found", key) {
			return "", fmt.Errorf("cache miss: key not found")
		}
		s.incrementError()
		return "", fmt.Errorf("cache get error: %w", err)
	}

	s.incrementHit()
	log.Printf("Cache hit for key: %s", key)
	return value, nil
}

// Set stores a value in cache with TTL
func (s *Service) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	defer s.updateLatency(start)

	// Convert value to string if it's not already
	var valueStr string
	switch v := value.(type) {
	case string:
		valueStr = v
	case []byte:
		valueStr = string(v)
	default:
		// Marshal to JSON for complex objects
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			s.incrementError()
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		valueStr = string(jsonBytes)
	}

	err := s.redisClient.Set(ctx, key, valueStr, ttl)
	if err != nil {
		s.incrementError()
		return fmt.Errorf("cache set error: %w", err)
	}

	s.incrementOperation()
	log.Printf("Cache set for key: %s (TTL: %v)", key, ttl)
	return nil
}

// Delete removes one or more keys from cache
func (s *Service) Delete(ctx context.Context, keys ...string) (int64, error) {
	start := time.Now()
	defer s.updateLatency(start)

	deleted, err := s.redisClient.Delete(ctx, keys...)
	if err != nil {
		s.incrementError()
		return 0, fmt.Errorf("cache delete error: %w", err)
	}

	s.incrementOperation()
	log.Printf("Cache deleted %d keys", deleted)
	return deleted, nil
}

// Exists checks if keys exist in cache
func (s *Service) Exists(ctx context.Context, keys ...string) (int64, error) {
	start := time.Now()
	defer s.updateLatency(start)

	exists, err := s.redisClient.Exists(ctx, keys...)
	if err != nil {
		s.incrementError()
		return 0, fmt.Errorf("cache exists error: %w", err)
	}

	s.incrementOperation()
	return exists, nil
}

// GetTTL returns the remaining time to live for a key
func (s *Service) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	start := time.Now()
	defer s.updateLatency(start)

	ttl, err := s.redisClient.TTL(ctx, key)
	if err != nil {
		s.incrementError()
		return 0, fmt.Errorf("cache TTL error: %w", err)
	}

	s.incrementOperation()
	return ttl, nil
}

// SetTTL sets a TTL on an existing key
func (s *Service) SetTTL(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	start := time.Now()
	defer s.updateLatency(start)

	success, err := s.redisClient.Expire(ctx, key, ttl)
	if err != nil {
		s.incrementError()
		return false, fmt.Errorf("cache set TTL error: %w", err)
	}

	s.incrementOperation()
	return success, nil
}

// Flush clears all cache entries
func (s *Service) Flush(ctx context.Context) error {
	start := time.Now()
	defer s.updateLatency(start)

	err := s.redisClient.FlushDB(ctx)
	if err != nil {
		s.incrementError()
		return fmt.Errorf("cache flush error: %w", err)
	}

	s.incrementOperation()
	log.Printf("Cache flushed successfully")
	return nil
}

// IsHealthy checks if cache is available
func (s *Service) IsHealthy() bool {
	return s.redisClient.IsConnected()
}

// Close closes the cache connection
func (s *Service) Close() error {
	return s.redisClient.Close()
}

// GetMetrics returns current cache metrics
func (s *Service) GetMetrics() *cache.CacheMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Calculate hit rate
	total := s.metrics.Hits + s.metrics.Misses
	if total > 0 {
		s.metrics.HitRate = float64(s.metrics.Hits) / float64(total) * 100.0
	}

	s.metrics.LastUpdated = time.Now()
	return s.metrics
}

// ResetMetrics resets all cache metrics
func (s *Service) ResetMetrics() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metrics = &cache.CacheMetrics{
		LastUpdated: time.Now(),
	}
}

// Helper methods for metrics tracking

func (s *Service) incrementHit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.Hits++
}

func (s *Service) incrementMiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.Misses++
}

func (s *Service) incrementError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.Errors++
}

func (s *Service) incrementOperation() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.Operations++
}

func (s *Service) updateLatency(start time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	latency := time.Since(start).Nanoseconds() / int64(time.Millisecond)
	
	// Calculate moving average (simple implementation)
	if s.metrics.Operations == 0 {
		s.metrics.AvgLatency = float64(latency)
	} else {
		s.metrics.AvgLatency = (s.metrics.AvgLatency + float64(latency)) / 2.0
	}
}

// High-level cache operations for application use

// GetJSON retrieves and unmarshals JSON data from cache
func (s *Service) GetJSON(ctx context.Context, key string, dest interface{}) error {
	value, err := s.Get(ctx, key)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(value), dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cached JSON: %w", err)
	}

	return nil
}

// SetJSON marshals and stores JSON data in cache
func (s *Service) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return s.Set(ctx, key, string(jsonBytes), ttl)
}

// GetOrSet retrieves a value from cache, or sets it using the provided function if not found
func (s *Service) GetOrSet(ctx context.Context, key string, ttl time.Duration, setter func() (interface{}, error)) (string, error) {
	// Try to get from cache first
	value, err := s.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	// If cache miss, call setter function
	if err.Error() == "cache miss: key not found" {
		newValue, setErr := setter()
		if setErr != nil {
			return "", fmt.Errorf("setter function failed: %w", setErr)
		}

		// Store in cache
		setErr = s.Set(ctx, key, newValue, ttl)
		if setErr != nil {
			log.Printf("Failed to cache value for key %s: %v", key, setErr)
			// Return the value even if caching failed
		}

		// Convert to string for return
		switch v := newValue.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		default:
			jsonBytes, _ := json.Marshal(newValue)
			return string(jsonBytes), nil
		}
	}

	return "", err
}

// InvalidatePattern removes all keys matching a pattern (use with caution)
func (s *Service) InvalidatePattern(ctx context.Context, pattern string) error {
	// Note: This requires Redis KEYS command which should be used carefully in production
	// For better performance, consider using Redis SCAN command
	rdb := s.redisClient.GetRedisClient()
	keys, err := rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to find keys with pattern %s: %w", pattern, err)
	}

	if len(keys) > 0 {
		_, err = s.Delete(ctx, keys...)
		if err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
		log.Printf("Invalidated %d keys matching pattern: %s", len(keys), pattern)
	}

	return nil
}