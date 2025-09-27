package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the alert handler service
type Config struct {
	GRPC    GRPCConfig    `mapstructure:"grpc"`
	Email   EmailConfig   `mapstructure:"email"`
	Alert   AlertConfig   `mapstructure:"alert"`
	Storage StorageConfig `mapstructure:"storage"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	Logger  LoggerConfig  `mapstructure:"logger"`
}

// GRPCConfig contains gRPC server configuration
type GRPCConfig struct {
	Host                 string        `mapstructure:"host"`
	Port                 int           `mapstructure:"port"`
	MaxRecvMsgSize       int           `mapstructure:"max_recv_msg_size"`
	MaxSendMsgSize       int           `mapstructure:"max_send_msg_size"`
	ConnectionTimeout    time.Duration `mapstructure:"connection_timeout"`
	MaxConcurrentStreams uint32        `mapstructure:"max_concurrent_streams"`
	KeepAliveTime        time.Duration `mapstructure:"keep_alive_time"`
	KeepAliveTimeout     time.Duration `mapstructure:"keep_alive_timeout"`
	MaxConnectionIdle    time.Duration `mapstructure:"max_connection_idle"`
	MaxConnectionAge     time.Duration `mapstructure:"max_connection_age"`
	EnableReflection     bool          `mapstructure:"enable_reflection"`
}

// EmailConfig contains email service configuration
type EmailConfig struct {
	SMTPHost     string        `mapstructure:"smtp_host"`
	SMTPPort     int           `mapstructure:"smtp_port"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	FromEmail    string        `mapstructure:"from_email"`
	FromName     string        `mapstructure:"from_name"`
	EnableTLS    bool          `mapstructure:"enable_tls"`
	EnableSSL    bool          `mapstructure:"enable_ssl"`
	Timeout      time.Duration `mapstructure:"timeout"`
	RetryCount   int           `mapstructure:"retry_count"`
	RetryDelay   time.Duration `mapstructure:"retry_delay"`
	TemplateDir  string        `mapstructure:"template_dir"`
	EnableDummy  bool          `mapstructure:"enable_dummy"`
}

// AlertConfig contains alert processing configuration
type AlertConfig struct {
	TemperatureThreshold   float64       `mapstructure:"temperature_threshold"`
	HumidityThreshold      float64       `mapstructure:"humidity_threshold"`
	PressureThreshold      float64       `mapstructure:"pressure_threshold"`
	CooldownPeriod         time.Duration `mapstructure:"cooldown_period"`
	MaxAlertsPerDevice     int           `mapstructure:"max_alerts_per_device"`
	MaxAlertsPerMinute     int           `mapstructure:"max_alerts_per_minute"`
	EnableRateLimiting     bool          `mapstructure:"enable_rate_limiting"`
	EnableDeduplication    bool          `mapstructure:"enable_deduplication"`
	DeduplicationWindow    time.Duration `mapstructure:"deduplication_window"`
	WorkerCount            int           `mapstructure:"worker_count"`
	QueueSize              int           `mapstructure:"queue_size"`
	ProcessingTimeout      time.Duration `mapstructure:"processing_timeout"`
}

// StorageConfig contains alert storage configuration
type StorageConfig struct {
	EnableStorage    bool          `mapstructure:"enable_storage"`
	RetentionPeriod  time.Duration `mapstructure:"retention_period"`
	CleanupInterval  time.Duration `mapstructure:"cleanup_interval"`
	MaxStoredAlerts  int           `mapstructure:"max_stored_alerts"`
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
	// gRPC defaults
	viper.SetDefault("grpc.host", "0.0.0.0")
	viper.SetDefault("grpc.port", 50051)
	viper.SetDefault("grpc.max_recv_msg_size", 4194304)  // 4MB
	viper.SetDefault("grpc.max_send_msg_size", 4194304)  // 4MB
	viper.SetDefault("grpc.connection_timeout", "30s")
	viper.SetDefault("grpc.max_concurrent_streams", 1000)
	viper.SetDefault("grpc.keep_alive_time", "30s")
	viper.SetDefault("grpc.keep_alive_timeout", "5s")
	viper.SetDefault("grpc.max_connection_idle", "15m")
	viper.SetDefault("grpc.max_connection_age", "30m")
	viper.SetDefault("grpc.enable_reflection", true)

	// Email defaults
	viper.SetDefault("email.smtp_host", "smtp.gmail.com")
	viper.SetDefault("email.smtp_port", 587)
	viper.SetDefault("email.username", "")
	viper.SetDefault("email.password", "")
	viper.SetDefault("email.from_email", "noreply@twinup.com")
	viper.SetDefault("email.from_name", "TWINUP Alert System")
	viper.SetDefault("email.enable_tls", true)
	viper.SetDefault("email.enable_ssl", false)
	viper.SetDefault("email.timeout", "30s")
	viper.SetDefault("email.retry_count", 3)
	viper.SetDefault("email.retry_delay", "5s")
	viper.SetDefault("email.template_dir", "./templates")
	viper.SetDefault("email.enable_dummy", true) // Dummy mode for development

	// Alert defaults
	viper.SetDefault("alert.temperature_threshold", 90.0)
	viper.SetDefault("alert.humidity_threshold", 95.0)
	viper.SetDefault("alert.pressure_threshold", 1050.0)
	viper.SetDefault("alert.cooldown_period", "1m")
	viper.SetDefault("alert.max_alerts_per_device", 10)
	viper.SetDefault("alert.max_alerts_per_minute", 100)
	viper.SetDefault("alert.enable_rate_limiting", true)
	viper.SetDefault("alert.enable_deduplication", true)
	viper.SetDefault("alert.deduplication_window", "5m")
	viper.SetDefault("alert.worker_count", 3)
	viper.SetDefault("alert.queue_size", 1000)
	viper.SetDefault("alert.processing_timeout", "30s")

	// Storage defaults
	viper.SetDefault("storage.enable_storage", true)
	viper.SetDefault("storage.retention_period", "30d")
	viper.SetDefault("storage.cleanup_interval", "1h")
	viper.SetDefault("storage.max_stored_alerts", 100000)

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
	if config.GRPC.Host == "" {
		return fmt.Errorf("gRPC host cannot be empty")
	}

	if config.GRPC.Port <= 0 || config.GRPC.Port > 65535 {
		return fmt.Errorf("gRPC port must be between 1 and 65535")
	}

	if config.Alert.TemperatureThreshold <= 0 {
		return fmt.Errorf("temperature threshold must be positive")
	}

	if config.Alert.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}

	if config.Alert.QueueSize <= 0 {
		return fmt.Errorf("queue size must be positive")
	}

	if config.Email.SMTPPort <= 0 || config.Email.SMTPPort > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}

	if config.Metrics.Port <= 0 || config.Metrics.Port > 65535 {
		return fmt.Errorf("metrics port must be between 1 and 65535")
	}

	return nil
}
