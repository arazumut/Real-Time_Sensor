package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/authentication-service/pkg/logger"
)

// Service handles Redis cache operations for authentication
type Service struct {
	client  *redis.Client
	config  Config
	logger  logger.Logger
	metrics MetricsRecorder

	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mutex   sync.RWMutex

	// Performance tracking
	hitCount   int64
	missCount  int64
	errorCount int64
	totalOps   int64
}

// Config holds Redis cache configuration
type Config struct {
	Host         string
	Port         int
	Password     string
	Database     int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	SetCacheHitRate(rate float64)
	RecordCacheSetLatency(duration time.Duration)
	RecordCacheGetLatency(duration time.Duration)
	SetCacheConnectionsActive(count int)
	IncrementCacheErrors()
}

// NewService creates a new Redis cache service
func NewService(config Config, logger logger.Logger, metrics MetricsRecorder) (*Service, error) {
	// Configure Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.Database,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	ctx, cancel = context.WithCancel(context.Background())

	service := &Service{
		client:  rdb,
		config:  config,
		logger:  logger,
		metrics: metrics,
		ctx:     ctx,
		cancel:  cancel,
	}

	logger.Info("Redis cache service created successfully",
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.Int("database", config.Database),
		zap.Int("pool_size", config.PoolSize),
	)

	return service, nil
}

// Start begins the cache service
func (s *Service) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("cache service is already running")
	}

	s.logger.Info("Starting Redis cache service")

	s.running = true
	s.metrics.SetCacheConnectionsActive(1)

	s.logger.Info("Redis cache service started successfully")
	return nil
}

// Stop gracefully stops the cache service
func (s *Service) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("cache service is not running")
	}

	s.logger.Info("Stopping Redis cache service...")

	// Cancel context
	s.cancel()

	// Close connection
	if err := s.client.Close(); err != nil {
		s.logger.Error("Error closing Redis connection", zap.Error(err))
	}

	s.running = false
	s.metrics.SetCacheConnectionsActive(0)

	s.logger.Info("Redis cache service stopped")
	return nil
}

// Get retrieves a value from cache
func (s *Service) Get(key string) (string, error) {
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	result, err := s.client.Get(ctx, key).Result()

	latency := time.Since(startTime)
	s.metrics.RecordCacheGetLatency(latency)
	s.totalOps++

	if err == redis.Nil {
		// Cache miss
		s.missCount++
		s.updateHitRate()
		return "", fmt.Errorf("key not found in cache")
	}

	if err != nil {
		s.metrics.IncrementCacheErrors()
		s.errorCount++
		return "", fmt.Errorf("failed to get from cache: %w", err)
	}

	// Cache hit
	s.hitCount++
	s.updateHitRate()

	s.logger.Debug("Cache GET successful",
		zap.String("key", key),
		zap.Duration("latency", latency),
	)

	return result, nil
}

// Set stores a value in cache with TTL
func (s *Service) Set(key, value string, ttl time.Duration) error {
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	err := s.client.Set(ctx, key, value, ttl).Err()

	latency := time.Since(startTime)
	s.metrics.RecordCacheSetLatency(latency)

	if err != nil {
		s.metrics.IncrementCacheErrors()
		s.errorCount++
		return fmt.Errorf("failed to set cache: %w", err)
	}

	s.logger.Debug("Cache SET successful",
		zap.String("key", key),
		zap.Duration("ttl", ttl),
		zap.Duration("latency", latency),
	)

	return nil
}

// Delete removes a key from cache
func (s *Service) Delete(key string) error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	err := s.client.Del(ctx, key).Err()
	if err != nil {
		s.metrics.IncrementCacheErrors()
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	s.logger.Debug("Cache DELETE successful", zap.String("key", key))
	return nil
}

// SetUserSession caches user session data
func (s *Service) SetUserSession(sessionID, userID string, sessionData map[string]interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)

	// Add user ID to session data
	sessionData["user_id"] = userID
	sessionData["cached_at"] = time.Now().Unix()

	// Serialize session data
	data, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	return s.Set(key, string(data), ttl)
}

// GetUserSession retrieves user session data
func (s *Service) GetUserSession(sessionID string) (map[string]interface{}, error) {
	key := fmt.Sprintf("session:%s", sessionID)

	data, err := s.Get(key)
	if err != nil {
		return nil, err
	}

	var sessionData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return sessionData, nil
}

// DeleteUserSession removes user session from cache
func (s *Service) DeleteUserSession(sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return s.Delete(key)
}

// SetRevokedToken adds a token to the revoked tokens list
func (s *Service) SetRevokedToken(tokenID string, expiresAt time.Time) error {
	key := fmt.Sprintf("revoked_token:%s", tokenID)
	ttl := time.Until(expiresAt)

	if ttl <= 0 {
		return nil // Token already expired
	}

	return s.Set(key, "revoked", ttl)
}

// IsTokenRevoked checks if a token is revoked
func (s *Service) IsTokenRevoked(tokenID string) bool {
	key := fmt.Sprintf("revoked_token:%s", tokenID)
	_, err := s.Get(key)
	return err == nil // If key exists, token is revoked
}

// updateHitRate calculates and updates cache hit rate
func (s *Service) updateHitRate() {
	if s.totalOps > 0 {
		hitRate := float64(s.hitCount) / float64(s.totalOps) * 100
		s.metrics.SetCacheHitRate(hitRate)
	}
}

// GetStats returns cache service statistics
func (s *Service) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	hitRate := float64(0)
	if s.totalOps > 0 {
		hitRate = float64(s.hitCount) / float64(s.totalOps) * 100
	}

	return map[string]interface{}{
		"hit_count":         s.hitCount,
		"miss_count":        s.missCount,
		"error_count":       s.errorCount,
		"total_ops":         s.totalOps,
		"hit_rate_pct":      hitRate,
		"is_running":        s.running,
		"connection_active": s.client != nil,
	}
}

// HealthCheck checks cache service health
func (s *Service) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis health check failed: %w", err)
	}

	return nil
}

// IsRunning returns whether the service is currently running
func (s *Service) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.running
}
