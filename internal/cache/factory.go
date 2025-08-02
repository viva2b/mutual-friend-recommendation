package cache

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"mutual-friend/pkg/cache"
	"mutual-friend/pkg/config"
	"mutual-friend/pkg/redis"
)

// Factory creates cache-related services
type Factory struct {
	config *config.Config
}

// NewFactory creates a new cache factory
func NewFactory(config *config.Config) *Factory {
	return &Factory{
		config: config,
	}
}

// CreateRedisClient creates a Redis client from configuration
func (f *Factory) CreateRedisClient() (*redis.Client, error) {
	// Parse Redis URL or use individual config fields
	redisConfig := f.parseRedisConfig()
	
	client, err := redis.NewClient(redisConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}

	return client, nil
}

// CreateCacheService creates a cache service with Redis backend
func (f *Factory) CreateCacheService() (cache.Cache, error) {
	redisClient, err := f.CreateRedisClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client for cache: %w", err)
	}

	service := NewService(redisClient)
	return service, nil
}

// parseRedisConfig converts config.RedisConfig to redis.Config
func (f *Factory) parseRedisConfig() *redis.Config {
	cfg := redis.DefaultConfig()
	
	// Parse URL if provided, otherwise use individual fields
	if f.config.Redis.URL != "" {
		if host, port, password, db := parseRedisURL(f.config.Redis.URL); host != "" {
			cfg.Host = host
			cfg.Port = port
			cfg.Password = password
			cfg.DB = db
		}
	} else {
		// Fallback to default localhost if no URL
		cfg.Host = "localhost"
		cfg.Port = 6379
	}

	// Override with specific config fields if they exist
	if f.config.Redis.Password != "" {
		cfg.Password = f.config.Redis.Password
	}
	if f.config.Redis.DB != 0 {
		cfg.DB = f.config.Redis.DB
	}
	if f.config.Redis.PoolSize != 0 {
		cfg.PoolSize = f.config.Redis.PoolSize
	}

	return cfg
}

// parseRedisURL parses a Redis URL like redis://localhost:6379/0 or redis://:password@localhost:6379/1
func parseRedisURL(url string) (host string, port int, password string, db int) {
	// Default values
	host = "localhost"
	port = 6379
	password = ""
	db = 0

	// Remove redis:// prefix
	if strings.HasPrefix(url, "redis://") {
		url = strings.TrimPrefix(url, "redis://")
	}

	// Parse password if present (format: :password@host:port/db)
	if strings.Contains(url, "@") {
		parts := strings.Split(url, "@")
		if len(parts) == 2 {
			authPart := parts[0]
			url = parts[1]
			
			if strings.HasPrefix(authPart, ":") {
				password = strings.TrimPrefix(authPart, ":")
			}
		}
	}

	// Parse database if present (format: host:port/db)
	if strings.Contains(url, "/") {
		parts := strings.Split(url, "/")
		if len(parts) == 2 {
			url = parts[0]
			if dbNum, err := strconv.Atoi(parts[1]); err == nil {
				db = dbNum
			}
		}
	}

	// Parse host and port (format: host:port)
	if strings.Contains(url, ":") {
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			host = parts[0]
			if portNum, err := strconv.Atoi(parts[1]); err == nil {
				port = portNum
			}
		}
	} else if url != "" {
		host = url
	}

	return host, port, password, db
}

// GetCacheTTLs returns cache TTL configurations as time.Duration
func (f *Factory) GetCacheTTLs() map[cache.CacheKey]time.Duration {
	ttls := make(map[cache.CacheKey]time.Duration)
	
	// Parse default TTL
	if defaultTTL, err := time.ParseDuration(f.config.Cache.DefaultTTL); err == nil {
		ttls[cache.CacheKey("default")] = defaultTTL
	}
	
	// Parse friend list TTL
	if friendListTTL, err := time.ParseDuration(f.config.Cache.FriendListTTL); err == nil {
		ttls[cache.CacheKeyUserFriends] = friendListTTL
		ttls[cache.CacheKeyFriendCount] = friendListTTL
		ttls[cache.CacheKeyMutualFriends] = friendListTTL
	}
	
	// Parse recommendation TTL
	if recTTL, err := time.ParseDuration(f.config.Cache.RecommendationTTL); err == nil {
		ttls[cache.CacheKeyRecommendations] = recTTL
		ttls[cache.CacheKeyRecommendationScore] = recTTL
	}
	
	return ttls
}

// UpdateDefaultTTLs updates the default TTL map with configuration values
func (f *Factory) UpdateDefaultTTLs() {
	ttls := f.GetCacheTTLs()
	
	for key, ttl := range ttls {
		if key != cache.CacheKey("default") {
			cache.DefaultTTLMap[key] = ttl
		}
	}
}

// CreateCacheManager creates a cache manager with metrics and health checking
func (f *Factory) CreateCacheManager() (*Manager, error) {
	cacheService, err := f.CreateCacheService()
	if err != nil {
		return nil, fmt.Errorf("failed to create cache service: %w", err)
	}

	manager := NewManager(cacheService)
	
	// Update TTL configurations
	f.UpdateDefaultTTLs()
	
	return manager, nil
}

// Manager provides high-level cache management functionality
type Manager struct {
	cache   cache.Cache
	metrics *cache.CacheMetrics
}

// NewManager creates a new cache manager
func NewManager(cacheService cache.Cache) *Manager {
	return &Manager{
		cache:   cacheService,
		metrics: &cache.CacheMetrics{},
	}
}

// GetCache returns the underlying cache service
func (m *Manager) GetCache() cache.Cache {
	return m.cache
}

// GetMetrics returns cache metrics if the service supports it
func (m *Manager) GetMetrics() *cache.CacheMetrics {
	if service, ok := m.cache.(*Service); ok {
		return service.GetMetrics()
	}
	return m.metrics
}

// HealthCheck performs a health check on the cache
func (m *Manager) HealthCheck() error {
	if !m.cache.IsHealthy() {
		return fmt.Errorf("cache is not healthy")
	}
	return nil
}

// Close closes the cache connection
func (m *Manager) Close() error {
	return m.cache.Close()
}