package redis

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client represents a Redis client
type Client struct {
	rdb     *redis.Client
	config  *Config
	mu      sync.RWMutex
	closed  bool
}

// Config holds Redis configuration
type Config struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
	// Connection timeout
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// Pool settings
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
}

// DefaultConfig returns a default Redis configuration
func DefaultConfig() *Config {
	return &Config{
		Host:            "localhost",
		Port:            6379,
		Password:        "", // No password
		DB:              0,  // Default DB
		PoolSize:        10,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	}
}

// NewClient creates a new Redis client
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	client := &Client{
		config: config,
	}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return client, nil
}

// connect establishes connection to Redis
func (c *Client) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create Redis client options
	opts := &redis.Options{
		Addr:            fmt.Sprintf("%s:%d", c.config.Host, c.config.Port),
		Password:        c.config.Password,
		DB:              c.config.DB,
		PoolSize:        c.config.PoolSize,
		DialTimeout:     c.config.DialTimeout,
		ReadTimeout:     c.config.ReadTimeout,
		WriteTimeout:    c.config.WriteTimeout,
		MaxRetries:      c.config.MaxRetries,
		MinRetryBackoff: c.config.MinRetryBackoff,
		MaxRetryBackoff: c.config.MaxRetryBackoff,
	}

	c.rdb = redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to ping Redis: %w", err)
	}

	c.closed = false
	log.Printf("Successfully connected to Redis at %s:%d", c.config.Host, c.config.Port)
	return nil
}

// Get retrieves a value by key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return "", fmt.Errorf("client is closed")
	}

	result, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key %s not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}

	return result, nil
}

// Set stores a value with optional TTL
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	err := c.rdb.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return nil
}

// SetNX sets a value only if the key does not exist
func (c *Client) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false, fmt.Errorf("client is closed")
	}

	result, err := c.rdb.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to setnx key %s: %w", key, err)
	}

	return result, nil
}

// Delete removes one or more keys
func (c *Client) Delete(ctx context.Context, keys ...string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, fmt.Errorf("client is closed")
	}

	result, err := c.rdb.Del(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to delete keys: %w", err)
	}

	return result, nil
}

// Exists checks if keys exist
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, fmt.Errorf("client is closed")
	}

	result, err := c.rdb.Exists(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to check existence of keys: %w", err)
	}

	return result, nil
}

// Expire sets a TTL on an existing key
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false, fmt.Errorf("client is closed")
	}

	result, err := c.rdb.Expire(ctx, key, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set expiration on key %s: %w", key, err)
	}

	return result, nil
}

// TTL returns the remaining time to live for a key
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, fmt.Errorf("client is closed")
	}

	result, err := c.rdb.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", key, err)
	}

	return result, nil
}

// Increment increments a counter
func (c *Client) Increment(ctx context.Context, key string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, fmt.Errorf("client is closed")
	}

	result, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}

	return result, nil
}

// IncrementBy increments a counter by a specific amount
func (c *Client) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, fmt.Errorf("client is closed")
	}

	result, err := c.rdb.IncrBy(ctx, key, value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s by %d: %w", key, value, err)
	}

	return result, nil
}

// MGet retrieves multiple values
func (c *Client) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	result, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to mget keys: %w", err)
	}

	return result, nil
}

// MSet sets multiple key-value pairs
func (c *Client) MSet(ctx context.Context, pairs ...interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	err := c.rdb.MSet(ctx, pairs...).Err()
	if err != nil {
		return fmt.Errorf("failed to mset: %w", err)
	}

	return nil
}

// Pipeline creates a new pipeline for batch operations
func (c *Client) Pipeline() redis.Pipeliner {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.rdb.Pipeline()
}

// IsConnected checks if the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.rdb == nil {
		return false
	}

	// Test connection with a quick ping
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := c.rdb.Ping(ctx).Result()
	return err == nil
}

// Reconnect attempts to reconnect to Redis
func (c *Client) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing connection
	if c.rdb != nil {
		if err := c.rdb.Close(); err != nil {
			log.Printf("Error closing existing Redis connection: %v", err)
		}
	}

	// Reset state
	c.closed = true

	// Reconnect
	return c.connect()
}

// Close closes the Redis connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	if c.rdb != nil {
		if err := c.rdb.Close(); err != nil {
			log.Printf("Error closing Redis connection: %v", err)
			return err
		}
	}

	log.Printf("Redis connection closed")
	return nil
}

// FlushDB removes all keys from the current database
func (c *Client) FlushDB(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	err := c.rdb.FlushDB(ctx).Err()
	if err != nil {
		return fmt.Errorf("failed to flush database: %w", err)
	}

	return nil
}

// GetRedisClient returns the underlying redis client for advanced operations
func (c *Client) GetRedisClient() *redis.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.rdb
}