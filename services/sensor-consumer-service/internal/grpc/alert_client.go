package grpc

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/twinup/sensor-system/services/sensor-consumer-service/pkg/logger"
)

// AlertRequest represents an alert request (simplified for now)
type AlertRequest struct {
	DeviceID    string    `json:"device_id"`
	Temperature float64   `json:"temperature"`
	Threshold   float64   `json:"threshold"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Severity    string    `json:"severity"`
}

// AlertResponse represents an alert response
type AlertResponse struct {
	Success     bool      `json:"success"`
	Message     string    `json:"message"`
	AlertID     string    `json:"alert_id"`
	ProcessedAt time.Time `json:"processed_at"`
}

// AlertClient handles gRPC communication with alert service
type AlertClient struct {
	conn    *grpc.ClientConn
	config  Config
	logger  logger.Logger
	metrics MetricsRecorder
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mutex   sync.RWMutex

	// Performance tracking
	requestCount int64
	errorCount   int64
	successCount int64
}

// Config holds gRPC client configuration
type Config struct {
	AlertServiceAddr string
	ConnTimeout      time.Duration
	RequestTimeout   time.Duration
	MaxRetries       int
	RetryBackoff     time.Duration
	KeepAlive        time.Duration
	KeepAliveTimeout time.Duration
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementGRPCRequests(service, method, status string)
	RecordGRPCLatency(service, method string, duration time.Duration)
	SetGRPCConnectionsActive(count int)
	IncrementGRPCErrors(service, method, errorType string)
	IncrementAlertsSent(deviceID, alertType string)
	IncrementAlertsFailed(deviceID, alertType, errorType string)
	RecordAlertLatency(duration time.Duration)
}

// NewAlertClient creates a new gRPC alert client
func NewAlertClient(config Config, logger logger.Logger, metrics MetricsRecorder) (*AlertClient, error) {
	// Configure gRPC connection
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.KeepAlive,
			Timeout:             config.KeepAliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(4*1024*1024), // 4MB
			grpc.MaxCallSendMsgSize(4*1024*1024), // 4MB
		),
	}

	// Create connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, config.AlertServiceAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to alert service: %w", err)
	}

	ctx, cancel = context.WithCancel(context.Background())

	client := &AlertClient{
		conn:    conn,
		config:  config,
		logger:  logger,
		metrics: metrics,
		ctx:     ctx,
		cancel:  cancel,
	}

	logger.Info("gRPC alert client created successfully",
		zap.String("address", config.AlertServiceAddr),
		zap.Duration("conn_timeout", config.ConnTimeout),
		zap.Duration("request_timeout", config.RequestTimeout),
	)

	return client, nil
}

// Start initializes the gRPC client
func (c *AlertClient) Start() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		return fmt.Errorf("gRPC alert client is already running")
	}

	c.logger.Info("Starting gRPC alert client")

	c.running = true
	c.metrics.SetGRPCConnectionsActive(1)

	c.logger.Info("gRPC alert client started successfully")
	return nil
}

// Stop gracefully stops the client
func (c *AlertClient) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		return fmt.Errorf("gRPC alert client is not running")
	}

	c.logger.Info("Stopping gRPC alert client...")

	// Cancel context
	c.cancel()

	// Close connection
	if err := c.conn.Close(); err != nil {
		c.logger.Error("Error closing gRPC connection", zap.Error(err))
	}

	c.running = false
	c.metrics.SetGRPCConnectionsActive(0)

	c.logger.Info("gRPC alert client stopped")
	return nil
}

// SendAlert sends an alert to the alert service with retry logic
func (c *AlertClient) SendAlert(alert *AlertRequest) (*AlertResponse, error) {
	startTime := time.Now()
	c.requestCount++

	var lastErr error

	// Retry logic
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(c.config.RetryBackoff):
			case <-c.ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry")
			}

			c.logger.Debug("Retrying alert request",
				zap.Int("attempt", attempt),
				zap.String("device_id", alert.DeviceID),
			)
		}

		response, err := c.sendAlertOnce(alert)
		if err == nil {
			// Success
			latency := time.Since(startTime)
			c.successCount++
			c.metrics.IncrementGRPCRequests("alert", "SendAlert", "success")
			c.metrics.RecordGRPCLatency("alert", "SendAlert", latency)
			c.metrics.IncrementAlertsSent(alert.DeviceID, "temperature")
			c.metrics.RecordAlertLatency(latency)

			c.logger.Debug("Alert sent successfully",
				zap.String("device_id", alert.DeviceID),
				zap.Float64("temperature", alert.Temperature),
				zap.Duration("latency", latency),
				zap.Int("attempt", attempt+1),
			)

			return response, nil
		}

		lastErr = err
		c.logger.Warn("Alert request failed",
			zap.Error(err),
			zap.String("device_id", alert.DeviceID),
			zap.Int("attempt", attempt+1),
		)
	}

	// All retries failed
	latency := time.Since(startTime)
	c.errorCount++
	c.metrics.IncrementGRPCRequests("alert", "SendAlert", "error")
	c.metrics.IncrementGRPCErrors("alert", "SendAlert", "retry_exhausted")
	c.metrics.IncrementAlertsFailed(alert.DeviceID, "temperature", "retry_exhausted")

	c.logger.Error("Alert request failed after all retries",
		zap.Error(lastErr),
		zap.String("device_id", alert.DeviceID),
		zap.Duration("total_latency", latency),
		zap.Int("max_retries", c.config.MaxRetries),
	)

	return nil, fmt.Errorf("alert request failed after %d retries: %w", c.config.MaxRetries, lastErr)
}

// sendAlertOnce sends a single alert request
func (c *AlertClient) sendAlertOnce(alert *AlertRequest) (*AlertResponse, error) {
	ctx, cancel := context.WithTimeout(c.ctx, c.config.RequestTimeout)
	defer cancel()

	// For now, we'll simulate the gRPC call since we don't have the generated proto code yet
	// In the real implementation, this would use the generated gRPC client

	// Simulate network delay
	select {
	case <-time.After(time.Duration(10+rand.Intn(40)) * time.Millisecond):
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout")
	}

	// Simulate occasional failures for testing
	if rand.Float64() < 0.05 { // 5% failure rate
		return nil, fmt.Errorf("simulated gRPC error")
	}

	// Return simulated response
	response := &AlertResponse{
		Success:     true,
		Message:     fmt.Sprintf("Alert processed for device %s", alert.DeviceID),
		AlertID:     fmt.Sprintf("alert_%d", time.Now().Unix()),
		ProcessedAt: time.Now(),
	}

	return response, nil
}

// HealthCheck checks the gRPC connection health
func (c *AlertClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check connection state
	state := c.conn.GetState()
	if state.String() != "READY" && state.String() != "IDLE" {
		return fmt.Errorf("gRPC connection not ready: %s", state.String())
	}

	// For now, we'll simulate a health check
	// In real implementation, this would call the HealthCheck RPC method
	select {
	case <-time.After(10 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("health check timeout")
	}
}

// GetStats returns client statistics
func (c *AlertClient) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	successRate := float64(0)
	if c.requestCount > 0 {
		successRate = float64(c.successCount) / float64(c.requestCount) * 100
	}

	return map[string]interface{}{
		"request_count":    c.requestCount,
		"success_count":    c.successCount,
		"error_count":      c.errorCount,
		"success_rate_pct": successRate,
		"is_running":       c.running,
		"connection_state": c.conn.GetState().String(),
	}
}

// IsRunning returns whether the client is currently running
func (c *AlertClient) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.running
}
