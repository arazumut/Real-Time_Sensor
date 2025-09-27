package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the API gateway service
type Config struct {
	HTTP       HTTPConfig       `mapstructure:"http"`
	Redis      RedisConfig      `mapstructure:"redis"`
	ClickHouse ClickHouseConfig `mapstructure:"clickhouse"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Gateway    GatewayConfig    `mapstructure:"gateway"`
	Metrics    MetricsConfig    `mapstructure:"metrics"`
	Logger     LoggerConfig     `mapstructure:"logger"`
}

// HTTPConfig contains HTTP server configuration
type HTTPConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	MaxHeaderBytes  int           `mapstructure:"max_header_bytes"`
	EnableCORS      bool          `mapstructure:"enable_cors"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies"`
	RateLimitRPS    int           `mapstructure:"rate_limit_rps"`
	EnableRateLimit bool          `mapstructure:"enable_rate_limit"`
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
	CacheTTL     time.Duration `mapstructure:"cache_ttl"`
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
	QueryTimeout    time.Duration `mapstructure:"query_timeout"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	ServiceURL    string        `mapstructure:"service_url"`
	JWTSecretKey  string        `mapstructure:"jwt_secret_key"`
	TokenCacheTTL time.Duration `mapstructure:"token_cache_ttl"`
	EnableAuth    bool          `mapstructure:"enable_auth"`
	SkipAuthPaths []string      `mapstructure:"skip_auth_paths"`
}

// GatewayConfig contains gateway-specific configuration
type GatewayConfig struct {
	EnableCaching         bool          `mapstructure:"enable_caching"`
	CacheDefaultTTL       time.Duration `mapstructure:"cache_default_ttl"`
	FallbackToClickHouse  bool          `mapstructure:"fallback_to_clickhouse"`
	MaxConcurrentRequests int           `mapstructure:"max_concurrent_requests"`
	RequestTimeout        time.Duration `mapstructure:"request_timeout"`
	EnableCompression     bool          `mapstructure:"enable_compression"`
	MaxResponseSize       int           `mapstructure:"max_response_size"`
	EnablePagination      bool          `mapstructure:"enable_pagination"`
	DefaultPageSize       int           `mapstructure:"default_page_size"`
	MaxPageSize           int           `mapstructure:"max_page_size"`
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
	// HTTP defaults
	viper.SetDefault("http.host", "0.0.0.0")
	viper.SetDefault("http.port", 8081)
	viper.SetDefault("http.read_timeout", "30s")
	viper.SetDefault("http.write_timeout", "30s")
	viper.SetDefault("http.idle_timeout", "120s")
	viper.SetDefault("http.max_header_bytes", 1048576) // 1MB
	viper.SetDefault("http.enable_cors", true)
	viper.SetDefault("http.trusted_proxies", []string{})
	viper.SetDefault("http.rate_limit_rps", 1000)
	viper.SetDefault("http.enable_rate_limit", true)

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.database", 0) // Same as consumer service
	viper.SetDefault("redis.pool_size", 20)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")
	viper.SetDefault("redis.idle_timeout", "5m")
	viper.SetDefault("redis.cache_ttl", "5m")

	// ClickHouse defaults
	viper.SetDefault("clickhouse.host", "localhost")
	viper.SetDefault("clickhouse.port", 9000)
	viper.SetDefault("clickhouse.database", "sensor_data")
	viper.SetDefault("clickhouse.username", "twinup")
	viper.SetDefault("clickhouse.password", "twinup123")
	viper.SetDefault("clickhouse.max_open_conns", 5)
	viper.SetDefault("clickhouse.max_idle_conns", 2)
	viper.SetDefault("clickhouse.conn_max_lifetime", "1h")
	viper.SetDefault("clickhouse.query_timeout", "30s")

	// Auth defaults
	viper.SetDefault("auth.service_url", "http://localhost:8080")
	viper.SetDefault("auth.jwt_secret_key", "twinup-super-secret-jwt-key-2023")
	viper.SetDefault("auth.token_cache_ttl", "15m")
	viper.SetDefault("auth.enable_auth", true)
	viper.SetDefault("auth.skip_auth_paths", []string{"/health", "/api/v1/health", "/metrics"})

	// Gateway defaults
	viper.SetDefault("gateway.enable_caching", true)
	viper.SetDefault("gateway.cache_default_ttl", "5m")
	viper.SetDefault("gateway.fallback_to_clickhouse", true)
	viper.SetDefault("gateway.max_concurrent_requests", 1000)
	viper.SetDefault("gateway.request_timeout", "30s")
	viper.SetDefault("gateway.enable_compression", true)
	viper.SetDefault("gateway.max_response_size", 10485760) // 10MB
	viper.SetDefault("gateway.enable_pagination", true)
	viper.SetDefault("gateway.default_page_size", 100)
	viper.SetDefault("gateway.max_page_size", 1000)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.port", 8081)
	viper.SetDefault("metrics.path", "/metrics")

	// Logger defaults
	viper.SetDefault("logger.level", "info")
	viper.SetDefault("logger.format", "json")
	viper.SetDefault("logger.development", false)
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.HTTP.Host == "" {
		return fmt.Errorf("HTTP host cannot be empty")
	}

	if config.HTTP.Port <= 0 || config.HTTP.Port > 65535 {
		return fmt.Errorf("HTTP port must be between 1 and 65535")
	}

	if config.Redis.Host == "" {
		return fmt.Errorf("Redis host cannot be empty")
	}

	if config.ClickHouse.Host == "" {
		return fmt.Errorf("ClickHouse host cannot be empty")
	}

	if config.ClickHouse.Database == "" {
		return fmt.Errorf("ClickHouse database cannot be empty")
	}

	if config.Auth.JWTSecretKey == "" {
		return fmt.Errorf("JWT secret key cannot be empty")
	}

	if config.Gateway.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("max concurrent requests must be positive")
	}

	if config.Gateway.DefaultPageSize <= 0 {
		return fmt.Errorf("default page size must be positive")
	}

	if config.Gateway.MaxPageSize <= 0 {
		return fmt.Errorf("max page size must be positive")
	}

	if config.Metrics.Port <= 0 || config.Metrics.Port > 65535 {
		return fmt.Errorf("metrics port must be between 1 and 65535")
	}

	return nil
}
