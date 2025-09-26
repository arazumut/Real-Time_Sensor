package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/sensor-consumer-service/pkg/logger"
)

// DeviceSnapshot represents cached device data
type DeviceSnapshot struct {
	DeviceID    string    `json:"device_id"`
	LastUpdate  time.Time `json:"last_update"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Pressure    float64   `json:"pressure"`
	LocationLat float64   `json:"location_lat"`
	LocationLng float64   `json:"location_lng"`
	Status      string    `json:"status"`
}

// Client handles Redis cache operations with pipeline optimization
type Client struct {
	client        *redis.Client
	config        Config
	logger        logger.Logger
	metrics       MetricsRecorder
	pipelineOps   []PipelineOperation
	pipelineMutex sync.Mutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	running       bool
	mutex         sync.RWMutex

	// Performance tracking
	hitCount   int64
	missCount  int64
	errorCount int64
	totalOps   int64
}

// Config holds Redis client configuration
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
	TTL          time.Duration
	PipelineSize int
}

// PipelineOperation represents a Redis operation for pipelining
type PipelineOperation struct {
	Operation string
	Key       string
	Value     interface{}
	TTL       time.Duration
	Callback  func(interface{}, error)
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	SetCacheHitRate(rate float64)
	RecordCacheSetLatency(duration time.Duration)
	RecordCacheGetLatency(duration time.Duration)
	SetCacheConnectionsActive(count int)
	IncrementCacheErrors()
	RecordCachePipelineSize(size int)
}

// NewClient creates a new Redis client with optimized settings
func NewClient(config Config, logger logger.Logger, metrics MetricsRecorder) (*Client, error) {
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

	client := &Client{
		client:      rdb,
		config:      config,
		logger:      logger,
		metrics:     metrics,
		pipelineOps: make([]PipelineOperation, 0, config.PipelineSize),
		ctx:         ctx,
		cancel:      cancel,
	}

	logger.Info("Redis client created successfully",
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.Int("database", config.Database),
		zap.Int("pool_size", config.PoolSize),
		zap.Duration("ttl", config.TTL),
	)

	return client, nil
}

// Start begins the pipeline processing
func (c *Client) Start() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		return fmt.Errorf("Redis client is already running")
	}

	c.logger.Info("Starting Redis client")

	// Start pipeline processor
	c.wg.Add(1)
	go c.pipelineProcessor()

	c.running = true
	c.metrics.SetCacheConnectionsActive(1)

	c.logger.Info("Redis client started successfully")
	return nil
}

// Stop gracefully stops the client
func (c *Client) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		return fmt.Errorf("Redis client is not running")
	}

	c.logger.Info("Stopping Redis client...")

	// Cancel context
	c.cancel()

	// Wait for pipeline processor to finish
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("Pipeline processor stopped gracefully")
	case <-time.After(30 * time.Second):
		c.logger.Warn("Timeout waiting for pipeline processor to stop")
	}

	// Flush remaining pipeline operations
	if err := c.flushPipeline(); err != nil {
		c.logger.Error("Error flushing final pipeline", zap.Error(err))
	}

	// Close connection
	if err := c.client.Close(); err != nil {
		c.logger.Error("Error closing Redis connection", zap.Error(err))
	}

	c.running = false
	c.metrics.SetCacheConnectionsActive(0)

	c.logger.Info("Redis client stopped")
	return nil
}

// SetDeviceSnapshot caches device snapshot with TTL
func (c *Client) SetDeviceSnapshot(snapshot *DeviceSnapshot) error {
	key := fmt.Sprintf("device:%s:snapshot", snapshot.DeviceID)

	// Serialize to JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	startTime := time.Now()

	// Use pipeline for better performance
	c.addToPipeline(PipelineOperation{
		Operation: "SET",
		Key:       key,
		Value:     data,
		TTL:       c.config.TTL,
		Callback: func(result interface{}, err error) {
			latency := time.Since(startTime)
			c.metrics.RecordCacheSetLatency(latency)

			if err != nil {
				c.metrics.IncrementCacheErrors()
				c.errorCount++
				c.logger.Error("Failed to set device snapshot",
					zap.Error(err),
					zap.String("device_id", snapshot.DeviceID),
				)
			} else {
				c.logger.Debug("Device snapshot cached",
					zap.String("device_id", snapshot.DeviceID),
					zap.Duration("latency", latency),
				)
			}
		},
	})

	return nil
}

// GetDeviceSnapshot retrieves device snapshot from cache
func (c *Client) GetDeviceSnapshot(deviceID string) (*DeviceSnapshot, error) {
	key := fmt.Sprintf("device:%s:snapshot", deviceID)

	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.client.Get(ctx, key).Result()

	latency := time.Since(startTime)
	c.metrics.RecordCacheGetLatency(latency)
	c.totalOps++

	if err == redis.Nil {
		// Cache miss
		c.missCount++
		c.updateHitRate()
		return nil, fmt.Errorf("device snapshot not found in cache")
	}

	if err != nil {
		c.metrics.IncrementCacheErrors()
		c.errorCount++
		return nil, fmt.Errorf("failed to get device snapshot: %w", err)
	}

	// Cache hit
	c.hitCount++
	c.updateHitRate()

	var snapshot DeviceSnapshot
	if err := json.Unmarshal([]byte(result), &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	c.logger.Debug("Device snapshot retrieved from cache",
		zap.String("device_id", deviceID),
		zap.Duration("latency", latency),
	)

	return &snapshot, nil
}

// SetMultipleSnapshots sets multiple device snapshots using pipeline
func (c *Client) SetMultipleSnapshots(snapshots []*DeviceSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	startTime := time.Now()

	// Add all operations to pipeline
	for _, snapshot := range snapshots {
		key := fmt.Sprintf("device:%s:snapshot", snapshot.DeviceID)

		data, err := json.Marshal(snapshot)
		if err != nil {
			c.logger.Error("Failed to marshal snapshot",
				zap.Error(err),
				zap.String("device_id", snapshot.DeviceID),
			)
			continue
		}

		c.addToPipeline(PipelineOperation{
			Operation: "SET",
			Key:       key,
			Value:     data,
			TTL:       c.config.TTL,
		})
	}

	// Force flush pipeline
	if err := c.flushPipeline(); err != nil {
		return fmt.Errorf("failed to flush pipeline: %w", err)
	}

	latency := time.Since(startTime)
	c.logger.Debug("Multiple snapshots cached",
		zap.Int("count", len(snapshots)),
		zap.Duration("latency", latency),
	)

	return nil
}

// addToPipeline adds an operation to the pipeline buffer
func (c *Client) addToPipeline(op PipelineOperation) {
	c.pipelineMutex.Lock()
	defer c.pipelineMutex.Unlock()

	c.pipelineOps = append(c.pipelineOps, op)

	// Flush if pipeline is full
	if len(c.pipelineOps) >= c.config.PipelineSize {
		go func() {
			if err := c.flushPipeline(); err != nil {
				c.logger.Error("Failed to flush pipeline", zap.Error(err))
			}
		}()
	}
}

// pipelineProcessor periodically flushes pipeline operations
func (c *Client) pipelineProcessor() {
	defer c.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond) // Flush every 100ms
	defer ticker.Stop()

	c.logger.Debug("Pipeline processor started",
		zap.Int("pipeline_size", c.config.PipelineSize),
	)

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Pipeline processor stopping")
			return

		case <-ticker.C:
			c.pipelineMutex.Lock()
			if len(c.pipelineOps) > 0 {
				if err := c.flushPipeline(); err != nil {
					c.logger.Error("Failed to flush pipeline", zap.Error(err))
				}
			}
			c.pipelineMutex.Unlock()
		}
	}
}

// flushPipeline executes all pending pipeline operations
func (c *Client) flushPipeline() error {
	if len(c.pipelineOps) == 0 {
		return nil
	}

	// Create copy of operations
	ops := make([]PipelineOperation, len(c.pipelineOps))
	copy(ops, c.pipelineOps)

	// Clear pipeline buffer
	c.pipelineOps = c.pipelineOps[:0]

	// Record pipeline size
	c.metrics.RecordCachePipelineSize(len(ops))

	// Execute pipeline
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipe := c.client.Pipeline()

	// Add all operations to pipeline
	for _, op := range ops {
		switch op.Operation {
		case "SET":
			pipe.Set(ctx, op.Key, op.Value, op.TTL)
		case "GET":
			pipe.Get(ctx, op.Key)
		case "DEL":
			pipe.Del(ctx, op.Key)
		}
	}

	// Execute pipeline
	results, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		c.metrics.IncrementCacheErrors()
		c.errorCount++
		return fmt.Errorf("failed to execute pipeline: %w", err)
	}

	// Process results and call callbacks
	for i, result := range results {
		if i < len(ops) && ops[i].Callback != nil {
			ops[i].Callback(result, result.Err())
		}
	}

	c.logger.Debug("Pipeline executed successfully",
		zap.Int("operations", len(ops)),
	)

	return nil
}

// updateHitRate calculates and updates cache hit rate
func (c *Client) updateHitRate() {
	if c.totalOps > 0 {
		hitRate := float64(c.hitCount) / float64(c.totalOps) * 100
		c.metrics.SetCacheHitRate(hitRate)
	}
}

// GetStats returns client statistics
func (c *Client) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.pipelineMutex.Lock()
	pipelineSize := len(c.pipelineOps)
	c.pipelineMutex.Unlock()

	hitRate := float64(0)
	if c.totalOps > 0 {
		hitRate = float64(c.hitCount) / float64(c.totalOps) * 100
	}

	return map[string]interface{}{
		"hit_count":         c.hitCount,
		"miss_count":        c.missCount,
		"error_count":       c.errorCount,
		"total_ops":         c.totalOps,
		"hit_rate_pct":      hitRate,
		"pipeline_size":     pipelineSize,
		"is_running":        c.running,
		"connection_active": c.client != nil,
	}
}

// HealthCheck checks the Redis connection health
func (c *Client) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis health check failed: %w", err)
	}

	return nil
}

// FlushAll flushes all cached data (use with caution)
func (c *Client) FlushAll() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.client.FlushAll(ctx).Err(); err != nil {
		c.metrics.IncrementCacheErrors()
		return fmt.Errorf("failed to flush all cache: %w", err)
	}

	c.logger.Warn("All cache data flushed")
	return nil
}

// GetDeviceKeys returns all device keys matching pattern
func (c *Client) GetDeviceKeys(pattern string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		c.metrics.IncrementCacheErrors()
		return nil, fmt.Errorf("failed to get device keys: %w", err)
	}

	return keys, nil
}

// DeleteDeviceSnapshot removes device snapshot from cache
func (c *Client) DeleteDeviceSnapshot(deviceID string) error {
	key := fmt.Sprintf("device:%s:snapshot", deviceID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.metrics.IncrementCacheErrors()
		return fmt.Errorf("failed to delete device snapshot: %w", err)
	}

	c.logger.Debug("Device snapshot deleted from cache",
		zap.String("device_id", deviceID),
	)

	return nil
}

// IsRunning returns whether the client is currently running
func (c *Client) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.running
}
