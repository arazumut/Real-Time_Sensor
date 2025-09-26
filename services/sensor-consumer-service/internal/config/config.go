package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the sensor consumer service
type Config struct {
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	ClickHouse ClickHouseConfig `mapstructure:"clickhouse"`
	Redis      RedisConfig      `mapstructure:"redis"`
	GRPC       GRPCConfig       `mapstructure:"grpc"`
	Consumer   ConsumerConfig   `mapstructure:"consumer"`
	Metrics    MetricsConfig    `mapstructure:"metrics"`
	Logger     LoggerConfig     `mapstructure:"logger"`
}

// KafkaConfig contains Kafka consumer configuration
type KafkaConfig struct {
	Brokers           []string      `mapstructure:"brokers"`
	Topic             string        `mapstructure:"topic"`
	GroupID           string        `mapstructure:"group_id"`
	AutoCommit        bool          `mapstructure:"auto_commit"`
	CommitInterval    time.Duration `mapstructure:"commit_interval"`
	SessionTimeout    time.Duration `mapstructure:"session_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	FetchMin          int32         `mapstructure:"fetch_min"`
	FetchDefault      int32         `mapstructure:"fetch_default"`
	FetchMax          int32         `mapstructure:"fetch_max"`
	RetryBackoff      time.Duration `mapstructure:"retry_backoff"`
}

// ClickHouseConfig contains ClickHouse database configuration
type ClickHouseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Database        string        `mapstructure:"database"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	BatchSize       int           `mapstructure:"batch_size"`
	BatchTimeout    time.Duration `mapstructure:"batch_timeout"`
	AsyncInsert     bool          `mapstructure:"async_insert"`
}

// RedisConfig contains Redis cache configuration
type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	Database     int           `mapstructure:"database"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	TTL          time.Duration `mapstructure:"ttl"`
	PipelineSize int           `mapstructure:"pipeline_size"`
}

// GRPCConfig contains gRPC client configuration
type GRPCConfig struct {
	AlertServiceAddr string        `mapstructure:"alert_service_addr"`
	ConnTimeout      time.Duration `mapstructure:"conn_timeout"`
	RequestTimeout   time.Duration `mapstructure:"request_timeout"`
	MaxRetries       int           `mapstructure:"max_retries"`
	RetryBackoff     time.Duration `mapstructure:"retry_backoff"`
	KeepAlive        time.Duration `mapstructure:"keep_alive"`
	KeepAliveTimeout time.Duration `mapstructure:"keep_alive_timeout"`
}

// ConsumerConfig contains consumer-specific configuration
type ConsumerConfig struct {
	WorkerCount       int           `mapstructure:"worker_count"`
	BufferSize        int           `mapstructure:"buffer_size"`
	ProcessingTimeout time.Duration `mapstructure:"processing_timeout"`
	EnableBenchmark   bool          `mapstructure:"enable_benchmark"`
	BenchmarkInterval time.Duration `mapstructure:"benchmark_interval"`
	GracefulTimeout   time.Duration `mapstructure:"graceful_timeout"`
	AlertThreshold    float64       `mapstructure:"alert_threshold"`
}

// MetricsConfig contains metrics configuration
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

// LoggerConfig contains logging configuration
type LoggerConfig struct {
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	Development bool   `mapstructure:"development"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	// Set default values
	setDefaults()

	// Set config file path
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath("../../configs")
	}

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("TWINUP")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Warning: Config file not found, using defaults and environment variables: %v\n", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topic", "sensor-data")
	viper.SetDefault("kafka.group_id", "sensor-consumer-group")
	viper.SetDefault("kafka.auto_commit", false)
	viper.SetDefault("kafka.commit_interval", "1s")
	viper.SetDefault("kafka.session_timeout", "30s")
	viper.SetDefault("kafka.heartbeat_interval", "3s")
	viper.SetDefault("kafka.fetch_min", 1)
	viper.SetDefault("kafka.fetch_default", 1048576) // 1MB
	viper.SetDefault("kafka.fetch_max", 10485760)    // 10MB
	viper.SetDefault("kafka.retry_backoff", "2s")

	// ClickHouse defaults
	viper.SetDefault("clickhouse.host", "localhost")
	viper.SetDefault("clickhouse.port", 9000)
	viper.SetDefault("clickhouse.database", "sensor_data")
	viper.SetDefault("clickhouse.username", "twinup")
	viper.SetDefault("clickhouse.password", "twinup123")
	viper.SetDefault("clickhouse.max_open_conns", 10)
	viper.SetDefault("clickhouse.max_idle_conns", 5)
	viper.SetDefault("clickhouse.conn_max_lifetime", "1h")
	viper.SetDefault("clickhouse.batch_size", 1000)
	viper.SetDefault("clickhouse.batch_timeout", "5s")
	viper.SetDefault("clickhouse.async_insert", true)

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.database", 0)
	viper.SetDefault("redis.pool_size", 20)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")
	viper.SetDefault("redis.idle_timeout", "5m")
	viper.SetDefault("redis.ttl", "5m")
	viper.SetDefault("redis.pipeline_size", 100)

	// gRPC defaults
	viper.SetDefault("grpc.alert_service_addr", "localhost:50051")
	viper.SetDefault("grpc.conn_timeout", "10s")
	viper.SetDefault("grpc.request_timeout", "5s")
	viper.SetDefault("grpc.max_retries", 3)
	viper.SetDefault("grpc.retry_backoff", "1s")
	viper.SetDefault("grpc.keep_alive", "30s")
	viper.SetDefault("grpc.keep_alive_timeout", "5s")

	// Consumer defaults
	viper.SetDefault("consumer.worker_count", 5)
	viper.SetDefault("consumer.buffer_size", 5000)
	viper.SetDefault("consumer.processing_timeout", "30s")
	viper.SetDefault("consumer.enable_benchmark", true)
	viper.SetDefault("consumer.benchmark_interval", "10s")
	viper.SetDefault("consumer.graceful_timeout", "30s")
	viper.SetDefault("consumer.alert_threshold", 90.0)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", 8080)
	viper.SetDefault("metrics.path", "/metrics")

	// Logger defaults
	viper.SetDefault("logger.level", "info")
	viper.SetDefault("logger.format", "json")
	viper.SetDefault("logger.development", false)
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if len(config.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka brokers cannot be empty")
	}

	if config.Kafka.Topic == "" {
		return fmt.Errorf("kafka topic cannot be empty")
	}

	if config.Kafka.GroupID == "" {
		return fmt.Errorf("kafka group ID cannot be empty")
	}

	if config.ClickHouse.Host == "" {
		return fmt.Errorf("clickhouse host cannot be empty")
	}

	if config.ClickHouse.Database == "" {
		return fmt.Errorf("clickhouse database cannot be empty")
	}

	if config.Redis.Host == "" {
		return fmt.Errorf("redis host cannot be empty")
	}

	if config.GRPC.AlertServiceAddr == "" {
		return fmt.Errorf("alert service address cannot be empty")
	}

	if config.Consumer.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}

	if config.Consumer.AlertThreshold <= 0 {
		return fmt.Errorf("alert threshold must be positive")
	}

	return nil
}
