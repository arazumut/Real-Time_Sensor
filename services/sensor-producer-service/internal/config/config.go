package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the sensor producer service
type Config struct {
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Producer ProducerConfig `mapstructure:"producer"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Logger   LoggerConfig   `mapstructure:"logger"`
}

// KafkaConfig contains Kafka-specific configuration
type KafkaConfig struct {
	Brokers         []string      `mapstructure:"brokers"`
	Topic           string        `mapstructure:"topic"`
	RetryMax        int           `mapstructure:"retry_max"`
	RequiredAcks    int           `mapstructure:"required_acks"`
	Timeout         time.Duration `mapstructure:"timeout"`
	CompressionType string        `mapstructure:"compression_type"`
	FlushFrequency  time.Duration `mapstructure:"flush_frequency"`
	FlushMessages   int           `mapstructure:"flush_messages"`
	FlushBytes      int           `mapstructure:"flush_bytes"`
	MaxMessageBytes int           `mapstructure:"max_message_bytes"`
}

// ProducerConfig contains producer-specific configuration
type ProducerConfig struct {
	DeviceCount       int           `mapstructure:"device_count"`
	BatchSize         int           `mapstructure:"batch_size"`
	ProduceInterval   time.Duration `mapstructure:"produce_interval"`
	WorkerCount       int           `mapstructure:"worker_count"`
	BufferSize        int           `mapstructure:"buffer_size"`
	EnableBenchmark   bool          `mapstructure:"enable_benchmark"`
	BenchmarkInterval time.Duration `mapstructure:"benchmark_interval"`
	GracefulTimeout   time.Duration `mapstructure:"graceful_timeout"`
}

// MetricsConfig contains metrics-specific configuration
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
		// Config file is optional, continue with defaults and env vars
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
	viper.SetDefault("kafka.retry_max", 3)
	viper.SetDefault("kafka.required_acks", 1)
	viper.SetDefault("kafka.timeout", "30s")
	viper.SetDefault("kafka.compression_type", "snappy")
	viper.SetDefault("kafka.flush_frequency", "100ms")
	viper.SetDefault("kafka.flush_messages", 1000)
	viper.SetDefault("kafka.flush_bytes", 1048576) // 1MB
	viper.SetDefault("kafka.max_message_bytes", 1000000)

	// Producer defaults
	viper.SetDefault("producer.device_count", 10000)
	viper.SetDefault("producer.batch_size", 1000)
	viper.SetDefault("producer.produce_interval", "1s")
	viper.SetDefault("producer.worker_count", 10)
	viper.SetDefault("producer.buffer_size", 10000)
	viper.SetDefault("producer.enable_benchmark", true)
	viper.SetDefault("producer.benchmark_interval", "10s")
	viper.SetDefault("producer.graceful_timeout", "30s")

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

	if config.Producer.DeviceCount <= 0 {
		return fmt.Errorf("device count must be positive")
	}

	if config.Producer.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}

	if config.Producer.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}

	if config.Metrics.Port <= 0 || config.Metrics.Port > 65535 {
		return fmt.Errorf("metrics port must be between 1 and 65535")
	}

	return nil
}
