package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Log        LogConfig        `mapstructure:"log"`
	GRPC       GRPCConfig       `mapstructure:"grpc"`
	DynamoDB   DynamoDBConfig   `mapstructure:"dynamodb"`
	RabbitMQ   RabbitMQConfig   `mapstructure:"rabbitmq"`
	Elasticsearch ElasticsearchConfig `mapstructure:"elasticsearch"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Cache      CacheConfig      `mapstructure:"cache"`
	Batch      BatchConfig      `mapstructure:"batch"`
	Recommendation RecommendationConfig `mapstructure:"recommendation"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
	Port        int    `mapstructure:"port"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type GRPCConfig struct {
	Port       int  `mapstructure:"port"`
	Reflection bool `mapstructure:"reflection"`
}

type DynamoDBConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	Region    string `mapstructure:"region"`
	TableName string `mapstructure:"table_name"`
}

type RabbitMQConfig struct {
	Host       string           `mapstructure:"host"`
	Port       int              `mapstructure:"port"`
	Username   string           `mapstructure:"username"`
	Password   string           `mapstructure:"password"`
	Vhost      string           `mapstructure:"vhost"`
	RetryDelay time.Duration    `mapstructure:"retry_delay"`
	Exchange   ExchangeConfig   `mapstructure:"exchange"`
	Queue      QueueConfig      `mapstructure:"queue"`
}

type ExchangeConfig struct {
	Name    string `mapstructure:"name"`
	Type    string `mapstructure:"type"`
	Durable bool   `mapstructure:"durable"`
}

type QueueConfig struct {
	Name       string `mapstructure:"name"`
	Durable    bool   `mapstructure:"durable"`
	RoutingKey string `mapstructure:"routing_key"`
}

type ElasticsearchConfig struct {
	URL           string            `mapstructure:"url"`
	Indices       IndicesConfig     `mapstructure:"indices"`
	BulkSize      int              `mapstructure:"bulk_size"`
	FlushInterval string           `mapstructure:"flush_interval"`
}

type IndicesConfig struct {
	Users string `mapstructure:"users"`
}

type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type CacheConfig struct {
	DefaultTTL        string `mapstructure:"default_ttl"`
	FriendListTTL     string `mapstructure:"friend_list_ttl"`
	RecommendationTTL string `mapstructure:"recommendation_ttl"`
}

type BatchConfig struct {
	Size        int    `mapstructure:"size"`
	Timeout     string `mapstructure:"timeout"`
	WorkerCount int    `mapstructure:"worker_count"`
}

type RecommendationConfig struct {
	MaxResults       int `mapstructure:"max_results"`
	MinMutualFriends int `mapstructure:"min_mutual_friends"`
	CalculationDepth int `mapstructure:"calculation_depth"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setDefaults()

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func setDefaults() {
	viper.SetDefault("app.name", "Mutual Friend System")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.port", 8080)

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output", "stdout")

	viper.SetDefault("grpc.port", 9090)
	viper.SetDefault("grpc.reflection", true)

	viper.SetDefault("dynamodb.endpoint", "http://localhost:8000")
	viper.SetDefault("dynamodb.region", "us-east-1")
	viper.SetDefault("dynamodb.table_name", "MutualFriendSystem")

	viper.SetDefault("rabbitmq.host", "localhost")
	viper.SetDefault("rabbitmq.port", 5672)
	viper.SetDefault("rabbitmq.username", "admin")
	viper.SetDefault("rabbitmq.password", "admin123")
	viper.SetDefault("rabbitmq.vhost", "/")
	viper.SetDefault("rabbitmq.retry_delay", "2s")
	viper.SetDefault("rabbitmq.exchange.name", "friend-events")
	viper.SetDefault("rabbitmq.exchange.type", "topic")
	viper.SetDefault("rabbitmq.exchange.durable", true)
	viper.SetDefault("rabbitmq.queue.name", "friend-events-queue")
	viper.SetDefault("rabbitmq.queue.durable", true)
	viper.SetDefault("rabbitmq.queue.routing_key", "friend.*")

	viper.SetDefault("elasticsearch.url", "http://localhost:9200")
	viper.SetDefault("elasticsearch.indices.users", "users")
	viper.SetDefault("elasticsearch.bulk_size", 100)
	viper.SetDefault("elasticsearch.flush_interval", "5s")

	viper.SetDefault("redis.url", "redis://localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	viper.SetDefault("cache.default_ttl", "3600s")
	viper.SetDefault("cache.friend_list_ttl", "1800s")
	viper.SetDefault("cache.recommendation_ttl", "7200s")

	viper.SetDefault("batch.size", 1000)
	viper.SetDefault("batch.timeout", "2s")
	viper.SetDefault("batch.worker_count", 5)

	viper.SetDefault("recommendation.max_results", 50)
	viper.SetDefault("recommendation.min_mutual_friends", 1)
	viper.SetDefault("recommendation.calculation_depth", 2)
}

// GetEnv gets environment variable with default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}