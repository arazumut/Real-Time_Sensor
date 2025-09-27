package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the authentication service
type Config struct {
	HTTP    HTTPConfig    `mapstructure:"http"`
	JWT     JWTConfig     `mapstructure:"jwt"`
	Redis   RedisConfig   `mapstructure:"redis"`
	Auth    AuthConfig    `mapstructure:"auth"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	Logger  LoggerConfig  `mapstructure:"logger"`
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

// JWTConfig contains JWT-specific configuration
type JWTConfig struct {
	SecretKey             string        `mapstructure:"secret_key"`
	AccessTokenTTL        time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL       time.Duration `mapstructure:"refresh_token_ttl"`
	Issuer                string        `mapstructure:"issuer"`
	Audience              string        `mapstructure:"audience"`
	Algorithm             string        `mapstructure:"algorithm"`
	EnableRefreshTokens   bool          `mapstructure:"enable_refresh_tokens"`
	MaxConcurrentSessions int           `mapstructure:"max_concurrent_sessions"`
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
}

// AuthConfig contains authentication-specific configuration
type AuthConfig struct {
	EnableRegistration       bool          `mapstructure:"enable_registration"`
	RequireEmailVerification bool          `mapstructure:"require_email_verification"`
	PasswordMinLength        int           `mapstructure:"password_min_length"`
	PasswordRequireSpecial   bool          `mapstructure:"password_require_special"`
	MaxLoginAttempts         int           `mapstructure:"max_login_attempts"`
	LoginAttemptWindow       time.Duration `mapstructure:"login_attempt_window"`
	SessionTimeout           time.Duration `mapstructure:"session_timeout"`
	EnableMFA                bool          `mapstructure:"enable_mfa"`
	DefaultRole              string        `mapstructure:"default_role"`
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
	viper.SetDefault("http.port", 8080)
	viper.SetDefault("http.read_timeout", "30s")
	viper.SetDefault("http.write_timeout", "30s")
	viper.SetDefault("http.idle_timeout", "120s")
	viper.SetDefault("http.max_header_bytes", 1048576) // 1MB
	viper.SetDefault("http.enable_cors", true)
	viper.SetDefault("http.trusted_proxies", []string{})
	viper.SetDefault("http.rate_limit_rps", 100)
	viper.SetDefault("http.enable_rate_limit", true)

	// JWT defaults
	viper.SetDefault("jwt.secret_key", "twinup-super-secret-jwt-key-2023")
	viper.SetDefault("jwt.access_token_ttl", "15m")
	viper.SetDefault("jwt.refresh_token_ttl", "24h")
	viper.SetDefault("jwt.issuer", "twinup-auth-service")
	viper.SetDefault("jwt.audience", "twinup-users")
	viper.SetDefault("jwt.algorithm", "HS256")
	viper.SetDefault("jwt.enable_refresh_tokens", true)
	viper.SetDefault("jwt.max_concurrent_sessions", 5)

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.database", 1) // Different DB than consumer service
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 3)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")
	viper.SetDefault("redis.idle_timeout", "5m")

	// Auth defaults
	viper.SetDefault("auth.enable_registration", false) // Disabled for demo
	viper.SetDefault("auth.require_email_verification", false)
	viper.SetDefault("auth.password_min_length", 8)
	viper.SetDefault("auth.password_require_special", true)
	viper.SetDefault("auth.max_login_attempts", 5)
	viper.SetDefault("auth.login_attempt_window", "15m")
	viper.SetDefault("auth.session_timeout", "24h")
	viper.SetDefault("auth.enable_mfa", false)
	viper.SetDefault("auth.default_role", "user")

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
	if config.HTTP.Host == "" {
		return fmt.Errorf("HTTP host cannot be empty")
	}

	if config.HTTP.Port <= 0 || config.HTTP.Port > 65535 {
		return fmt.Errorf("HTTP port must be between 1 and 65535")
	}

	if config.JWT.SecretKey == "" {
		return fmt.Errorf("JWT secret key cannot be empty")
	}

	if len(config.JWT.SecretKey) < 32 {
		return fmt.Errorf("JWT secret key must be at least 32 characters")
	}

	if config.JWT.AccessTokenTTL <= 0 {
		return fmt.Errorf("access token TTL must be positive")
	}

	if config.JWT.RefreshTokenTTL <= 0 {
		return fmt.Errorf("refresh token TTL must be positive")
	}

	if config.Redis.Host == "" {
		return fmt.Errorf("Redis host cannot be empty")
	}

	if config.Auth.PasswordMinLength < 4 {
		return fmt.Errorf("password minimum length must be at least 4")
	}

	if config.Auth.MaxLoginAttempts <= 0 {
		return fmt.Errorf("max login attempts must be positive")
	}

	if config.Metrics.Port <= 0 || config.Metrics.Port > 65535 {
		return fmt.Errorf("metrics port must be between 1 and 65535")
	}

	return nil
}
