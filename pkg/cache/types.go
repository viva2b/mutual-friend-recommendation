package cache

import (
	"context"
	"fmt"
	"time"
)

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string) (string, error)
	
	// Set stores a value in cache with TTL
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	
	// Delete removes one or more keys from cache
	Delete(ctx context.Context, keys ...string) (int64, error)
	
	// Exists checks if keys exist in cache
	Exists(ctx context.Context, keys ...string) (int64, error)
	
	// GetTTL returns the remaining time to live for a key
	GetTTL(ctx context.Context, key string) (time.Duration, error)
	
	// SetTTL sets a TTL on an existing key
	SetTTL(ctx context.Context, key string, ttl time.Duration) (bool, error)
	
	// Flush clears all cache entries
	Flush(ctx context.Context) error
	
	// IsHealthy checks if cache is available
	IsHealthy() bool
	
	// Close closes the cache connection
	Close() error
}

// CacheMetrics holds cache performance metrics
type CacheMetrics struct {
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	HitRate     float64 `json:"hit_rate"`
	Operations  int64   `json:"operations"`
	Errors      int64   `json:"errors"`
	AvgLatency  float64 `json:"avg_latency_ms"`
	LastUpdated time.Time `json:"last_updated"`
}

// CacheKey represents different types of cache keys used in the system
type CacheKey string

const (
	// Friend-related cache keys
	CacheKeyUserFriends       CacheKey = "friends:user:%s"           // friends:user:{userID}
	CacheKeyMutualFriends     CacheKey = "mutual:friends:%s:%s"      // mutual:friends:{userA}:{userB}
	CacheKeyFriendCount       CacheKey = "count:friends:%s"          // count:friends:{userID}
	CacheKeyFriendship        CacheKey = "friendship:%s:%s"          // friendship:{userA}:{userB}
	
	// Recommendation cache keys
	CacheKeyRecommendations   CacheKey = "recommendations:%s"        // recommendations:{userID}
	CacheKeyRecommendationScore CacheKey = "rec:score:%s:%s"         // rec:score:{userID}:{targetID}
	
	// User profile cache keys
	CacheKeyUserProfile       CacheKey = "profile:user:%s"           // profile:user:{userID}
	CacheKeyUserExists        CacheKey = "exists:user:%s"            // exists:user:{userID}
	
	// Search cache keys
	CacheKeySearchResults     CacheKey = "search:results:%s"         // search:results:{queryHash}
	CacheKeyUserSearch        CacheKey = "search:user:%s"            // search:user:{query}
	
	// Analytics cache keys
	CacheKeyDailyMetrics      CacheKey = "metrics:daily:%s"          // metrics:daily:{date}
	CacheKeyUserActivity      CacheKey = "activity:user:%s"          // activity:user:{userID}
)

// FormatCacheKey formats a cache key with parameters
func (ck CacheKey) Format(params ...interface{}) string {
	return fmt.Sprintf(string(ck), params...)
}

// TTL constants for different data types
const (
	// Short-lived cache (frequently changing data)
	TTLShort = 5 * time.Minute
	
	// Medium-lived cache (moderately changing data)
	TTLMedium = 30 * time.Minute
	
	// Long-lived cache (rarely changing data)
	TTLLong = 2 * time.Hour
	
	// Very long-lived cache (static data)
	TTLVeryLong = 24 * time.Hour
	
	// Default TTL
	TTLDefault = TTLMedium
)

// DefaultTTLMap defines default TTLs for different cache key types
var DefaultTTLMap = map[CacheKey]time.Duration{
	CacheKeyUserFriends:       TTLMedium,     // 30 minutes
	CacheKeyMutualFriends:     TTLLong,       // 2 hours
	CacheKeyFriendCount:       TTLShort,      // 5 minutes
	CacheKeyFriendship:        TTLLong,       // 2 hours
	CacheKeyRecommendations:   TTLVeryLong,   // 24 hours
	CacheKeyRecommendationScore: TTLLong,     // 2 hours
	CacheKeyUserProfile:       TTLLong,       // 2 hours
	CacheKeyUserExists:        TTLVeryLong,   // 24 hours
	CacheKeySearchResults:     TTLShort,      // 5 minutes
	CacheKeyUserSearch:        TTLMedium,     // 30 minutes
	CacheKeyDailyMetrics:      TTLVeryLong,   // 24 hours
	CacheKeyUserActivity:      TTLShort,      // 5 minutes
}

// GetDefaultTTL returns the default TTL for a cache key type
func GetDefaultTTL(keyType CacheKey) time.Duration {
	if ttl, exists := DefaultTTLMap[keyType]; exists {
		return ttl
	}
	return TTLDefault
}