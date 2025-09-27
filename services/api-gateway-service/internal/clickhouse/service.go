package clickhouse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/api-gateway-service/pkg/logger"
)

// SensorReading represents a sensor reading from database
type SensorReading struct {
	DeviceID    string    `json:"device_id" ch:"device_id"`
	Timestamp   time.Time `json:"timestamp" ch:"timestamp"`
	Temperature float64   `json:"temperature" ch:"temperature"`
	Humidity    float64   `json:"humidity" ch:"humidity"`
	Pressure    float64   `json:"pressure" ch:"pressure"`
	LocationLat float64   `json:"location_lat" ch:"location_lat"`
	LocationLng float64   `json:"location_lng" ch:"location_lng"`
	Status      string    `json:"status" ch:"status"`
	CreatedAt   time.Time `json:"created_at" ch:"created_at"`
}

// DeviceInfo represents basic device information
type DeviceInfo struct {
	DeviceID    string    `json:"device_id" ch:"device_id"`
	LastSeen    time.Time `json:"last_seen" ch:"last_seen"`
	Status      string    `json:"status" ch:"status"`
	Region      string    `json:"region" ch:"region"`
	Temperature float64   `json:"temperature" ch:"temperature"`
}

// Service handles ClickHouse database operations for the API gateway
type Service struct {
	conn    driver.Conn
	config  Config
	logger  logger.Logger
	metrics MetricsRecorder

	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mutex   sync.RWMutex

	// Performance tracking
	queryCount int64
	errorCount int64
	startTime  time.Time
}

// Config holds ClickHouse service configuration
type Config struct {
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	QueryTimeout    time.Duration
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementDBRequests(operation, result string)
	RecordDBLatency(operation string, duration time.Duration)
	SetDBConnectionsActive(count int)
	IncrementDBErrors()
}

// NewService creates a new ClickHouse service
func NewService(config Config, logger logger.Logger, metrics MetricsRecorder) (*Service, error) {
	// Build connection options
	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": int(config.QueryTimeout.Seconds()),
		},
		DialTimeout:      30 * time.Second,
		MaxOpenConns:     config.MaxOpenConns,
		MaxIdleConns:     config.MaxIdleConns,
		ConnMaxLifetime:  config.ConnMaxLifetime,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
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

	service := &Service{
		conn:      conn,
		config:    config,
		logger:    logger,
		metrics:   metrics,
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	logger.Info("ClickHouse service created successfully",
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.String("database", config.Database),
	)

	return service, nil
}

// Start begins the ClickHouse service
func (s *Service) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("ClickHouse service is already running")
	}

	s.logger.Info("Starting ClickHouse service")

	s.running = true
	s.metrics.SetDBConnectionsActive(1)

	s.logger.Info("ClickHouse service started successfully")
	return nil
}

// Stop gracefully stops the service
func (s *Service) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("ClickHouse service is not running")
	}

	s.logger.Info("Stopping ClickHouse service...")

	// Cancel context
	s.cancel()

	// Close connection
	if err := s.conn.Close(); err != nil {
		s.logger.Error("Error closing ClickHouse connection", zap.Error(err))
	}

	s.running = false
	s.metrics.SetDBConnectionsActive(0)

	s.logger.Info("ClickHouse service stopped")
	return nil
}

// GetLatestReading retrieves the latest reading for a device
func (s *Service) GetLatestReading(deviceID string) (*SensorReading, error) {
	ctx, cancel := context.WithTimeout(s.ctx, s.config.QueryTimeout)
	defer cancel()

	query := `
		SELECT 
			device_id, timestamp, temperature, humidity, pressure,
			location_lat, location_lng, status, created_at
		FROM sensor_data.sensor_readings 
		WHERE device_id = ? 
		ORDER BY timestamp DESC 
		LIMIT 1
	`

	var reading SensorReading
	row := s.conn.QueryRow(ctx, query, deviceID)
	err := row.ScanStruct(&reading)

	s.queryCount++

	if err != nil {
		s.errorCount++
		return nil, fmt.Errorf("failed to query latest reading: %w", err)
	}

	s.logger.Debug("Latest reading retrieved",
		zap.String("device_id", deviceID),
		zap.Time("timestamp", reading.Timestamp),
	)

	return &reading, nil
}

// GetDeviceList retrieves paginated list of devices
func (s *Service) GetDeviceList(page, pageSize int) ([]*DeviceInfo, int, error) {
	ctx, cancel := context.WithTimeout(s.ctx, s.config.QueryTimeout)
	defer cancel()

	offset := (page - 1) * pageSize

	// Get total count
	countQuery := `
		SELECT count(DISTINCT device_id) 
		FROM sensor_data.sensor_readings
	`

	var totalCount uint64
	row := s.conn.QueryRow(ctx, countQuery)
	if err := row.Scan(&totalCount); err != nil {
		s.errorCount++
		return nil, 0, fmt.Errorf("failed to get device count: %w", err)
	}

	// Get device list with latest data
	listQuery := `
		SELECT 
			device_id,
			max(timestamp) as last_seen,
			argMax(status, timestamp) as status,
			'unknown' as region,
			argMax(temperature, timestamp) as temperature
		FROM sensor_data.sensor_readings 
		GROUP BY device_id
		ORDER BY last_seen DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.conn.Query(ctx, listQuery, pageSize, offset)
	if err != nil {
		s.errorCount++
		return nil, 0, fmt.Errorf("failed to query device list: %w", err)
	}
	defer rows.Close()

	var devices []*DeviceInfo
	for rows.Next() {
		var device DeviceInfo
		err := rows.ScanStruct(&device)
		if err != nil {
			s.logger.Error("Failed to scan device row", zap.Error(err))
			continue
		}
		devices = append(devices, &device)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating device rows: %w", err)
	}

	s.queryCount++

	s.logger.Debug("Device list retrieved",
		zap.Int("device_count", len(devices)),
		zap.Int("total_count", int(totalCount)),
		zap.Int("page", page),
		zap.Int("page_size", pageSize),
	)

	return devices, int(totalCount), nil
}

// GetDeviceHistory retrieves historical data for a device
func (s *Service) GetDeviceHistory(deviceID string, startTime, endTime time.Time, limit int) ([]*SensorReading, error) {
	ctx, cancel := context.WithTimeout(s.ctx, s.config.QueryTimeout)
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

	rows, err := s.conn.Query(ctx, query, deviceID, startTime, endTime, limit)
	if err != nil {
		s.errorCount++
		return nil, fmt.Errorf("failed to query device history: %w", err)
	}
	defer rows.Close()

	var readings []*SensorReading
	for rows.Next() {
		var reading SensorReading
		err := rows.ScanStruct(&reading)
		if err != nil {
			s.logger.Error("Failed to scan reading row", zap.Error(err))
			continue
		}
		readings = append(readings, &reading)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reading rows: %w", err)
	}

	s.queryCount++

	s.logger.Debug("Device history retrieved",
		zap.String("device_id", deviceID),
		zap.Int("record_count", len(readings)),
		zap.Time("start_time", startTime),
		zap.Time("end_time", endTime),
	)

	return readings, nil
}

// HealthCheck checks the database connection health
func (s *Service) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.conn.Ping(ctx); err != nil {
		return fmt.Errorf("ClickHouse health check failed: %w", err)
	}

	return nil
}

// GetStats returns service statistics
func (s *Service) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uptime := time.Since(s.startTime)
	queryRate := float64(s.queryCount) / uptime.Seconds()

	return map[string]interface{}{
		"query_count":       s.queryCount,
		"error_count":       s.errorCount,
		"query_rate":        queryRate,
		"uptime_seconds":    uptime.Seconds(),
		"is_running":        s.running,
		"connection_active": s.conn != nil,
	}
}

// IsRunning returns whether the service is currently running
func (s *Service) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.running
}
