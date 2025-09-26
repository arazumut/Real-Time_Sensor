package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the real-time analytics service
type Config struct {
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	MQTT      MQTTConfig      `mapstructure:"mqtt"`
	Analytics AnalyticsConfig `mapstructure:"analytics"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	Logger    LoggerConfig    `mapstructure:"logger"`
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

// MQTTConfig contains MQTT publisher configuration
type MQTTConfig struct {
	Broker               string        `mapstructure:"broker"`
	ClientID             string        `mapstructure:"client_id"`
	Username             string        `mapstructure:"username"`
	Password             string        `mapstructure:"password"`
	QOS                  byte          `mapstructure:"qos"`
	Retained             bool          `mapstructure:"retained"`
	ConnectTimeout       time.Duration `mapstructure:"connect_timeout"`
	WriteTimeout         time.Duration `mapstructure:"write_timeout"`
	KeepAlive            time.Duration `mapstructure:"keep_alive"`
	PingTimeout          time.Duration `mapstructure:"ping_timeout"`
	ConnectRetryInterval time.Duration `mapstructure:"connect_retry_interval"`
	AutoReconnect        bool          `mapstructure:"auto_reconnect"`
	MaxReconnectInterval time.Duration `mapstructure:"max_reconnect_interval"`
	TopicPrefix          string        `mapstructure:"topic_prefix"`
}

// AnalyticsConfig contains analytics-specific configuration
type AnalyticsConfig struct {
	WorkerCount       int           `mapstructure:"worker_count"`
	BufferSize        int           `mapstructure:"buffer_size"`
	WindowSize        time.Duration `mapstructure:"window_size"`
	TrendWindow       time.Duration `mapstructure:"trend_window"`
	DeltaThreshold    float64       `mapstructure:"delta_threshold"`
	RegionalAnalysis  bool          `mapstructure:"regional_analysis"`
	AnomalyDetection  bool          `mapstructure:"anomaly_detection"`
	PublishInterval   time.Duration `mapstructure:"publish_interval"`
	EnableBenchmark   bool          `mapstructure:"enable_benchmark"`
	BenchmarkInterval time.Duration `mapstructure:"benchmark_interval"`
	GracefulTimeout   time.Duration `mapstructure:"graceful_timeout"`
	MaxMemoryWindow   int           `mapstructure:"max_memory_window"`
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
	viper.SetDefault("kafka.group_id", "analytics-consumer-group")
	viper.SetDefault("kafka.auto_commit", true)
	viper.SetDefault("kafka.commit_interval", "1s")
	viper.SetDefault("kafka.session_timeout", "30s")
	viper.SetDefault("kafka.heartbeat_interval", "3s")
	viper.SetDefault("kafka.fetch_min", 1)
	viper.SetDefault("kafka.fetch_default", 1048576) // 1MB
	viper.SetDefault("kafka.fetch_max", 10485760)    // 10MB
	viper.SetDefault("kafka.retry_backoff", "2s")

	// MQTT defaults
	viper.SetDefault("mqtt.broker", "tcp://localhost:1883")
	viper.SetDefault("mqtt.client_id", "twinup-analytics")
	viper.SetDefault("mqtt.username", "")
	viper.SetDefault("mqtt.password", "")
	viper.SetDefault("mqtt.qos", 1)
	viper.SetDefault("mqtt.retained", false)
	viper.SetDefault("mqtt.connect_timeout", "30s")
	viper.SetDefault("mqtt.write_timeout", "30s")
	viper.SetDefault("mqtt.keep_alive", "60s")
	viper.SetDefault("mqtt.ping_timeout", "10s")
	viper.SetDefault("mqtt.connect_retry_interval", "10s")
	viper.SetDefault("mqtt.auto_reconnect", true)
	viper.SetDefault("mqtt.max_reconnect_interval", "10m")
	viper.SetDefault("mqtt.topic_prefix", "analytics")

	// Analytics defaults
	viper.SetDefault("analytics.worker_count", 3)
	viper.SetDefault("analytics.buffer_size", 1000)
	viper.SetDefault("analytics.window_size", "5m")
	viper.SetDefault("analytics.trend_window", "15m")
	viper.SetDefault("analytics.delta_threshold", 2.0)
	viper.SetDefault("analytics.regional_analysis", true)
	viper.SetDefault("analytics.anomaly_detection", true)
	viper.SetDefault("analytics.publish_interval", "30s")
	viper.SetDefault("analytics.enable_benchmark", true)
	viper.SetDefault("analytics.benchmark_interval", "10s")
	viper.SetDefault("analytics.graceful_timeout", "30s")
	viper.SetDefault("analytics.max_memory_window", 10000)

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

	if config.MQTT.Broker == "" {
		return fmt.Errorf("MQTT broker cannot be empty")
	}

	if config.MQTT.ClientID == "" {
		return fmt.Errorf("MQTT client ID cannot be empty")
	}

	if config.Analytics.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}

	if config.Analytics.WindowSize <= 0 {
		return fmt.Errorf("window size must be positive")
	}

	if config.Analytics.TrendWindow <= 0 {
		return fmt.Errorf("trend window must be positive")
	}

	if config.Metrics.Port <= 0 || config.Metrics.Port > 65535 {
		return fmt.Errorf("metrics port must be between 1 and 65535")
	}

	return nil
}
