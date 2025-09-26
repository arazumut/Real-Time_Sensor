package clickhouse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/sensor-consumer-service/pkg/logger"
)

// SensorReading represents a sensor reading for database storage
type SensorReading struct {
	DeviceID    string    `json:"device_id"`
	Timestamp   time.Time `json:"timestamp"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Pressure    float64   `json:"pressure"`
	LocationLat float64   `json:"location_lat"`
	LocationLng float64   `json:"location_lng"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// Client handles ClickHouse database operations with high performance
type Client struct {
	conn        driver.Conn
	config      Config
	logger      logger.Logger
	metrics     MetricsRecorder
	batchBuffer []*SensorReading
	batchMutex  sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
	mutex       sync.RWMutex

	// Performance tracking
	insertCount   int64
	errorCount    int64
	lastFlushTime time.Time
}

// Config holds ClickHouse client configuration
type Config struct {
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	BatchSize       int
	BatchTimeout    time.Duration
	AsyncInsert     bool
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	RecordDBInsertLatency(duration time.Duration)
	RecordDBBatchSize(size int)
	SetDBConnectionsActive(count int)
	IncrementDBErrors()
	IncrementDBRecordsInserted(count int)
}

// NewClient creates a new ClickHouse client with optimized settings
func NewClient(config Config, logger logger.Logger, metrics MetricsRecorder) (*Client, error) {
	// Build connection options
	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:      30 * time.Second,
		MaxOpenConns:     config.MaxOpenConns,
		MaxIdleConns:     config.MaxIdleConns,
		ConnMaxLifetime:  config.ConnMaxLifetime,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	}

	// Enable async inserts for better performance
	if config.AsyncInsert {
		options.Settings["async_insert"] = 1
		options.Settings["wait_for_async_insert"] = 0
	}

	// Create connection
	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Test connection
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		conn:          conn,
		config:        config,
		logger:        logger,
		metrics:       metrics,
		batchBuffer:   make([]*SensorReading, 0, config.BatchSize),
		ctx:           ctx,
		cancel:        cancel,
		lastFlushTime: time.Now(),
	}

	logger.Info("ClickHouse client created successfully",
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.String("database", config.Database),
		zap.Int("batch_size", config.BatchSize),
		zap.Bool("async_insert", config.AsyncInsert),
	)

	return client, nil
}

// Start begins the batch processing
func (c *Client) Start() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		return fmt.Errorf("ClickHouse client is already running")
	}

	c.logger.Info("Starting ClickHouse client")

	// Start batch flusher
	c.wg.Add(1)
	go c.batchFlusher()

	c.running = true
	c.metrics.SetDBConnectionsActive(1)

	c.logger.Info("ClickHouse client started successfully")
	return nil
}

// Stop gracefully stops the client
func (c *Client) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		return fmt.Errorf("ClickHouse client is not running")
	}

	c.logger.Info("Stopping ClickHouse client...")

	// Cancel context
	c.cancel()

	// Wait for batch flusher to finish
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("Batch flusher stopped gracefully")
	case <-time.After(30 * time.Second):
		c.logger.Warn("Timeout waiting for batch flusher to stop")
	}

	// Flush remaining data
	if err := c.flushBatch(); err != nil {
		c.logger.Error("Error flushing final batch", zap.Error(err))
	}

	// Close connection
	if err := c.conn.Close(); err != nil {
		c.logger.Error("Error closing ClickHouse connection", zap.Error(err))
	}

	c.running = false
	c.metrics.SetDBConnectionsActive(0)

	c.logger.Info("ClickHouse client stopped")
	return nil
}

// InsertReading adds a sensor reading to the batch buffer
func (c *Client) InsertReading(reading *SensorReading) error {
	c.batchMutex.Lock()
	defer c.batchMutex.Unlock()

	// Add to batch buffer
	c.batchBuffer = append(c.batchBuffer, reading)

	// Flush if batch is full
	if len(c.batchBuffer) >= c.config.BatchSize {
		return c.flushBatch()
	}

	return nil
}

// InsertBatch inserts multiple sensor readings efficiently
func (c *Client) InsertBatch(readings []*SensorReading) error {
	if len(readings) == 0 {
		return nil
	}

	startTime := time.Now()

	// Prepare batch insert
	batch, err := c.conn.PrepareBatch(context.Background(), `
		INSERT INTO sensor_data.sensor_readings (
			device_id, timestamp, temperature, humidity, pressure,
			location_lat, location_lng, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		c.metrics.IncrementDBErrors()
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	// Add all readings to batch
	for _, reading := range readings {
		err := batch.Append(
			reading.DeviceID,
			reading.Timestamp,
			reading.Temperature,
			reading.Humidity,
			reading.Pressure,
			reading.LocationLat,
			reading.LocationLng,
			reading.Status,
			reading.CreatedAt,
		)
		if err != nil {
			c.metrics.IncrementDBErrors()
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}

	// Execute batch
	if err := batch.Send(); err != nil {
		c.metrics.IncrementDBErrors()
		c.errorCount++
		return fmt.Errorf("failed to send batch: %w", err)
	}

	// Record metrics
	latency := time.Since(startTime)
	c.metrics.RecordDBInsertLatency(latency)
	c.metrics.RecordDBBatchSize(len(readings))
	c.metrics.IncrementDBRecordsInserted(len(readings))

	c.insertCount += int64(len(readings))

	c.logger.Debug("Batch inserted successfully",
		zap.Int("batch_size", len(readings)),
		zap.Duration("latency", latency),
		zap.Int64("total_inserted", c.insertCount),
	)

	return nil
}

// batchFlusher periodically flushes the batch buffer
func (c *Client) batchFlusher() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.BatchTimeout)
	defer ticker.Stop()

	c.logger.Debug("Batch flusher started",
		zap.Duration("timeout", c.config.BatchTimeout),
		zap.Int("batch_size", c.config.BatchSize),
	)

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Batch flusher stopping")
			return

		case <-ticker.C:
			c.batchMutex.Lock()
			if len(c.batchBuffer) > 0 {
				if err := c.flushBatch(); err != nil {
					c.logger.Error("Failed to flush batch", zap.Error(err))
				}
			}
			c.batchMutex.Unlock()
		}
	}
}

// flushBatch flushes the current batch buffer (must be called with mutex held)
func (c *Client) flushBatch() error {
	if len(c.batchBuffer) == 0 {
		return nil
	}

	// Create copy of buffer
	batch := make([]*SensorReading, len(c.batchBuffer))
	copy(batch, c.batchBuffer)

	// Clear buffer
	c.batchBuffer = c.batchBuffer[:0]
	c.lastFlushTime = time.Now()

	// Insert batch (release mutex during insert)
	c.batchMutex.Unlock()
	err := c.InsertBatch(batch)
	c.batchMutex.Lock()

	return err
}

// GetStats returns client statistics
func (c *Client) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.batchMutex.Lock()
	bufferSize := len(c.batchBuffer)
	c.batchMutex.Unlock()

	return map[string]interface{}{
		"insert_count":      c.insertCount,
		"error_count":       c.errorCount,
		"buffer_size":       bufferSize,
		"last_flush_time":   c.lastFlushTime,
		"is_running":        c.running,
		"connection_active": c.conn != nil,
	}
}

// HealthCheck checks the database connection health
func (c *Client) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.conn.Ping(ctx); err != nil {
		return fmt.Errorf("ClickHouse health check failed: %w", err)
	}

	return nil
}

// QueryDeviceSnapshot queries the latest snapshot for a device
func (c *Client) QueryDeviceSnapshot(deviceID string) (*SensorReading, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var reading SensorReading

	query := `
		SELECT 
			device_id, timestamp, temperature, humidity, pressure,
			location_lat, location_lng, status, created_at
		FROM sensor_data.sensor_readings 
		WHERE device_id = ? 
		ORDER BY timestamp DESC 
		LIMIT 1
	`

	row := c.conn.QueryRow(ctx, query, deviceID)
	err := row.Scan(
		&reading.DeviceID,
		&reading.Timestamp,
		&reading.Temperature,
		&reading.Humidity,
		&reading.Pressure,
		&reading.LocationLat,
		&reading.LocationLng,
		&reading.Status,
		&reading.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query device snapshot: %w", err)
	}

	return &reading, nil
}

// QueryDeviceHistory queries historical data for a device
func (c *Client) QueryDeviceHistory(deviceID string, startTime, endTime time.Time, limit int) ([]*SensorReading, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := `
		SELECT 
			device_id, timestamp, temperature, humidity, pressure,
			location_lat, location_lng, status, created_at
		FROM sensor_data.sensor_readings 
		WHERE device_id = ? 
		AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := c.conn.Query(ctx, query, deviceID, startTime, endTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query device history: %w", err)
	}
	defer rows.Close()

	var readings []*SensorReading
	for rows.Next() {
		var reading SensorReading
		err := rows.Scan(
			&reading.DeviceID,
			&reading.Timestamp,
			&reading.Temperature,
			&reading.Humidity,
			&reading.Pressure,
			&reading.LocationLat,
			&reading.LocationLng,
			&reading.Status,
			&reading.CreatedAt,
		)
		if err != nil {
			c.logger.Error("Failed to scan row", zap.Error(err))
			continue
		}
		readings = append(readings, &reading)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return readings, nil
}

// GetAggregatedData returns aggregated sensor data for analytics
func (c *Client) GetAggregatedData(deviceID string, interval time.Duration) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now().Add(-interval)

	query := `
		SELECT 
			count() as reading_count,
			avg(temperature) as avg_temperature,
			min(temperature) as min_temperature,
			max(temperature) as max_temperature,
			avg(humidity) as avg_humidity,
			avg(pressure) as avg_pressure,
			max(timestamp) as last_reading
		FROM sensor_data.sensor_readings 
		WHERE device_id = ? 
		AND timestamp >= ?
	`

	var result struct {
		ReadingCount   uint64    `ch:"reading_count"`
		AvgTemperature float64   `ch:"avg_temperature"`
		MinTemperature float64   `ch:"min_temperature"`
		MaxTemperature float64   `ch:"max_temperature"`
		AvgHumidity    float64   `ch:"avg_humidity"`
		AvgPressure    float64   `ch:"avg_pressure"`
		LastReading    time.Time `ch:"last_reading"`
	}

	row := c.conn.QueryRow(ctx, query, deviceID, startTime)
	if err := row.ScanStruct(&result); err != nil {
		return nil, fmt.Errorf("failed to scan aggregated data: %w", err)
	}

	return map[string]interface{}{
		"device_id":        deviceID,
		"reading_count":    result.ReadingCount,
		"avg_temperature":  result.AvgTemperature,
		"min_temperature":  result.MinTemperature,
		"max_temperature":  result.MaxTemperature,
		"avg_humidity":     result.AvgHumidity,
		"avg_pressure":     result.AvgPressure,
		"last_reading":     result.LastReading,
		"interval_minutes": interval.Minutes(),
	}, nil
}

// IsRunning returns whether the client is currently running
func (c *Client) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.running
}
