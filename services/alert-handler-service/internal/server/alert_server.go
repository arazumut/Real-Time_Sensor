package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/alert-handler-service/internal/email"
	"github.com/twinup/sensor-system/services/alert-handler-service/internal/storage"
	"github.com/twinup/sensor-system/services/alert-handler-service/pkg/logger"
)

// AlertRequest represents an incoming alert request (simplified proto)
type AlertRequest struct {
	DeviceID    string    `json:"device_id"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Pressure    float64   `json:"pressure"`
	Threshold   float64   `json:"threshold"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"`
	Region      string    `json:"region"`
	Timestamp   time.Time `json:"timestamp"`
	Metadata    map[string]string `json:"metadata"`
}

// AlertResponse represents an alert response
type AlertResponse struct {
	Success     bool      `json:"success"`
	Message     string    `json:"message"`
	AlertID     string    `json:"alert_id"`
	ProcessedAt time.Time `json:"processed_at"`
	EmailSent   bool      `json:"email_sent"`
	Stored      bool      `json:"stored"`
}

// HealthCheckRequest represents a health check request
type HealthCheckRequest struct{}

// HealthCheckResponse represents a health check response
type HealthCheckResponse struct {
	Healthy   bool      `json:"healthy"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
}

// AlertHistoryRequest represents an alert history request
type AlertHistoryRequest struct {
	DeviceID  string    `json:"device_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Limit     int       `json:"limit"`
	Severity  string    `json:"min_severity"`
}

// AlertHistoryResponse represents an alert history response
type AlertHistoryResponse struct {
	Alerts     []*storage.AlertRecord `json:"alerts"`
	TotalCount int                    `json:"total_count"`
}

// Server implements the gRPC alert service
type Server struct {
	config       Config
	logger       logger.Logger
	metrics      MetricsRecorder
	emailService *email.Service
	storage      *storage.AlertStorage
	
	// Rate limiting and deduplication
	deviceCooldowns   map[string]time.Time
	recentAlerts      map[string]time.Time
	rateLimitMutex    sync.RWMutex
	
	// Processing queue
	alertQueue        chan *AlertProcessingTask
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	running           bool
	mutex             sync.RWMutex
	
	// Performance tracking
	processedCount    int64
	filteredCount     int64
	errorCount        int64
	startTime         time.Time
}

// AlertProcessingTask represents an alert processing task
type AlertProcessingTask struct {
	Request     *AlertRequest
	AlertID     string
	ReceivedAt  time.Time
	ProcessedAt time.Time
}

// Config holds server configuration
type Config struct {
	TemperatureThreshold  float64
	HumidityThreshold     float64
	PressureThreshold     float64
	CooldownPeriod        time.Duration
	MaxAlertsPerDevice    int
	MaxAlertsPerMinute    int
	EnableRateLimiting    bool
	EnableDeduplication   bool
	DeduplicationWindow   time.Duration
	WorkerCount           int
	QueueSize             int
	ProcessingTimeout     time.Duration
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementAlertsReceived()
	IncrementAlertsProcessed()
	IncrementAlertsFailed()
	IncrementAlertsFiltered()
	IncrementAlertsByType(alertType string)
	IncrementAlertsBySeverity(severity string)
	IncrementAlertsByDevice(deviceID string)
	IncrementAlertsByRegion(region string)
	RecordAlertProcessingLatency(duration time.Duration)
	SetAlertQueueSize(size int)
	RecordAlertQueueLatency(duration time.Duration)
	IncrementRateLimitHits()
	IncrementRateLimitBypass()
	SetCooldownActive(count int)
	IncrementDuplicateAlerts()
	SetDeduplicationHitRate(rate float64)
	IncrementGRPCRequests(method, status string)
	RecordGRPCLatency(method string, duration time.Duration)
	IncrementGRPCErrors(method, errorType string)
	SetProcessingRate(rate float64)
}

// NewServer creates a new alert server
func NewServer(
	config Config,
	logger logger.Logger,
	metrics MetricsRecorder,
	emailService *email.Service,
	storage *storage.AlertStorage,
) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	server := &Server{
		config:          config,
		logger:          logger,
		metrics:         metrics,
		emailService:    emailService,
		storage:         storage,
		deviceCooldowns: make(map[string]time.Time),
		recentAlerts:    make(map[string]time.Time),
		alertQueue:      make(chan *AlertProcessingTask, config.QueueSize),
		ctx:             ctx,
		cancel:          cancel,
		startTime:       time.Now(),
	}

	logger.Info("Alert server created successfully",
		zap.Float64("temperature_threshold", config.TemperatureThreshold),
		zap.Duration("cooldown_period", config.CooldownPeriod),
		zap.Bool("rate_limiting", config.EnableRateLimiting),
		zap.Bool("deduplication", config.EnableDeduplication),
		zap.Int("worker_count", config.WorkerCount),
	)

	return server
}

// Start begins the alert processing
func (s *Server) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("alert server is already running")
	}

	s.logger.Info("Starting alert server")

	// Start alert processing workers
	for i := 0; i < s.config.WorkerCount; i++ {
		s.wg.Add(1)
		go s.alertWorker(i)
	}

	// Start cleanup worker
	s.wg.Add(1)
	go s.cleanupWorker()

	s.running = true
	s.logger.Info("Alert server started successfully")

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("alert server is not running")
	}

	s.logger.Info("Stopping alert server...")

	// Cancel context
	s.cancel()

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Alert workers stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timeout waiting for alert workers to stop")
	}

	// Close alert queue
	close(s.alertQueue)

	s.running = false
	s.logger.Info("Alert server stopped")

	return nil
}

// SendAlert processes an alert request (main gRPC method)
func (s *Server) SendAlert(ctx context.Context, req *AlertRequest) (*AlertResponse, error) {
	startTime := time.Now()
	
	// Generate alert ID
	alertID := uuid.New().String()
	
	// Update metrics
	s.metrics.IncrementAlertsReceived()
	s.metrics.IncrementGRPCRequests("SendAlert", "received")

	s.logger.Debug("Alert received",
		zap.String("alert_id", alertID),
		zap.String("device_id", req.DeviceID),
		zap.Float64("temperature", req.Temperature),
		zap.Float64("threshold", req.Threshold),
		zap.String("severity", req.Severity),
	)

	// Validate request
	if err := s.validateAlertRequest(req); err != nil {
		s.metrics.IncrementGRPCErrors("SendAlert", "validation_error")
		return &AlertResponse{
			Success:     false,
			Message:     fmt.Sprintf("Invalid request: %v", err),
			AlertID:     alertID,
			ProcessedAt: time.Now(),
		}, nil
	}

	// Check rate limiting
	if s.config.EnableRateLimiting && s.isRateLimited(req.DeviceID) {
		s.metrics.IncrementAlertsFiltered()
		s.metrics.IncrementRateLimitHits()
		
		s.logger.Warn("Alert rate limited",
			zap.String("alert_id", alertID),
			zap.String("device_id", req.DeviceID),
		)

		return &AlertResponse{
			Success:     false,
			Message:     "Alert rate limited - device in cooldown",
			AlertID:     alertID,
			ProcessedAt: time.Now(),
		}, nil
	}

	// Check deduplication
	if s.config.EnableDeduplication && s.isDuplicate(req) {
		s.metrics.IncrementAlertsFiltered()
		s.metrics.IncrementDuplicateAlerts()
		
		s.logger.Debug("Duplicate alert filtered",
			zap.String("alert_id", alertID),
			zap.String("device_id", req.DeviceID),
		)

		return &AlertResponse{
			Success:     false,
			Message:     "Duplicate alert filtered",
			AlertID:     alertID,
			ProcessedAt: time.Now(),
		}, nil
	}

	// Queue alert for processing
	task := &AlertProcessingTask{
		Request:    req,
		AlertID:    alertID,
		ReceivedAt: startTime,
	}

	select {
	case s.alertQueue <- task:
		s.metrics.SetAlertQueueSize(len(s.alertQueue))
		
		// Record gRPC latency
		grpcLatency := time.Since(startTime)
		s.metrics.RecordGRPCLatency("SendAlert", grpcLatency)
		s.metrics.IncrementGRPCRequests("SendAlert", "success")

		return &AlertResponse{
			Success:     true,
			Message:     "Alert queued for processing",
			AlertID:     alertID,
			ProcessedAt: time.Now(),
		}, nil

	default:
		s.metrics.IncrementAlertsFailed()
		s.metrics.IncrementGRPCErrors("SendAlert", "queue_full")
		
		return &AlertResponse{
			Success:     false,
			Message:     "Alert queue full",
			AlertID:     alertID,
			ProcessedAt: time.Now(),
		}, nil
	}
}

// HealthCheck implements health check gRPC method
func (s *Server) HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	uptime := time.Since(s.startTime)
	
	healthy := s.IsRunning()
	if s.emailService != nil {
		healthy = healthy && s.emailService.IsRunning()
	}
	
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}

	return &HealthCheckResponse{
		Healthy:   healthy,
		Status:    status,
		Timestamp: time.Now(),
		Uptime:    uptime.String(),
	}, nil
}

// GetAlertHistory implements alert history gRPC method
func (s *Server) GetAlertHistory(ctx context.Context, req *AlertHistoryRequest) (*AlertHistoryResponse, error) {
	if s.storage == nil {
		return &AlertHistoryResponse{
			Alerts:     []*storage.AlertRecord{},
			TotalCount: 0,
		}, nil
	}

	alerts, err := s.storage.GetAlertHistory(req.DeviceID, req.StartTime, req.EndTime, req.Limit)
	if err != nil {
		s.logger.Error("Failed to get alert history",
			zap.Error(err),
			zap.String("device_id", req.DeviceID),
		)
		return nil, fmt.Errorf("failed to get alert history: %w", err)
	}

	return &AlertHistoryResponse{
		Alerts:     alerts,
		TotalCount: len(alerts),
	}, nil
}

// alertWorker processes alerts from the queue
func (s *Server) alertWorker(workerID int) {
	defer s.wg.Done()

	s.logger.Debug("Alert worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("Alert worker stopping", zap.Int("worker_id", workerID))
			return

		case task := <-s.alertQueue:
			if task == nil {
				continue
			}

			s.processAlert(task)
		}
	}
}

// processAlert processes a single alert
func (s *Server) processAlert(task *AlertProcessingTask) {
	startTime := time.Now()
	task.ProcessedAt = startTime
	
	queueLatency := startTime.Sub(task.ReceivedAt)
	s.metrics.RecordAlertQueueLatency(queueLatency)
	s.metrics.SetAlertQueueSize(len(s.alertQueue))

	req := task.Request
	alertID := task.AlertID

	s.logger.Info("Processing alert",
		zap.String("alert_id", alertID),
		zap.String("device_id", req.DeviceID),
		zap.Float64("temperature", req.Temperature),
		zap.Duration("queue_latency", queueLatency),
	)

	// Determine alert type and severity
	alertType := s.determineAlertType(req)
	severity := s.determineSeverity(req)

	// Update metrics
	s.metrics.IncrementAlertsByType(alertType)
	s.metrics.IncrementAlertsBySeverity(severity)
	s.metrics.IncrementAlertsByDevice(req.DeviceID)
	if req.Region != "" {
		s.metrics.IncrementAlertsByRegion(req.Region)
	}

	// Send email notification
	emailSent := false
	if s.shouldSendEmail(req, alertType, severity) {
		if err := s.sendAlertEmail(alertID, req, alertType, severity); err != nil {
			s.logger.Error("Failed to send alert email",
				zap.Error(err),
				zap.String("alert_id", alertID),
				zap.String("device_id", req.DeviceID),
			)
		} else {
			emailSent = true
		}
	}

	// Store alert if storage is enabled
	stored := false
	if s.storage != nil {
		alertRecord := &storage.AlertRecord{
			AlertID:     alertID,
			DeviceID:    req.DeviceID,
			AlertType:   alertType,
			Severity:    severity,
			Message:     req.Message,
			Temperature: req.Temperature,
			Threshold:   req.Threshold,
			Region:      req.Region,
			Timestamp:   req.Timestamp,
			ProcessedAt: startTime,
			EmailSent:   emailSent,
			Metadata:    req.Metadata,
		}

		if err := s.storage.StoreAlert(alertRecord); err != nil {
			s.logger.Error("Failed to store alert",
				zap.Error(err),
				zap.String("alert_id", alertID),
			)
		} else {
			stored = true
		}
	}

	// Update cooldown
	s.updateCooldown(req.DeviceID)

	// Update deduplication cache
	s.updateDeduplicationCache(req)

	// Record processing metrics
	processingLatency := time.Since(startTime)
	s.processedCount++
	s.metrics.IncrementAlertsProcessed()
	s.metrics.RecordAlertProcessingLatency(processingLatency)

	s.logger.Info("Alert processed successfully",
		zap.String("alert_id", alertID),
		zap.String("device_id", req.DeviceID),
		zap.String("alert_type", alertType),
		zap.String("severity", severity),
		zap.Bool("email_sent", emailSent),
		zap.Bool("stored", stored),
		zap.Duration("processing_latency", processingLatency),
		zap.Duration("total_latency", time.Since(task.ReceivedAt)),
	)
}

// validateAlertRequest validates an incoming alert request
func (s *Server) validateAlertRequest(req *AlertRequest) error {
	if req.DeviceID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}

	if req.Temperature < -50 || req.Temperature > 150 {
		return fmt.Errorf("temperature value out of valid range")
	}

	if req.Threshold <= 0 {
		return fmt.Errorf("threshold must be positive")
	}

	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}

	return nil
}

// determineAlertType determines the type of alert based on the request
func (s *Server) determineAlertType(req *AlertRequest) string {
	if req.Temperature > s.config.TemperatureThreshold {
		return "TEMPERATURE_HIGH"
	}
	if req.Humidity > s.config.HumidityThreshold {
		return "HUMIDITY_HIGH"
	}
	if req.Pressure > s.config.PressureThreshold {
		return "PRESSURE_HIGH"
	}
	return "GENERAL"
}

// determineSeverity determines the severity of the alert
func (s *Server) determineSeverity(req *AlertRequest) string {
	tempDiff := req.Temperature - req.Threshold
	
	if tempDiff >= 20 {
		return "CRITICAL"
	} else if tempDiff >= 10 {
		return "HIGH"
	} else if tempDiff >= 5 {
		return "MEDIUM"
	} else {
		return "LOW"
	}
}

// shouldSendEmail determines if an email should be sent for this alert
func (s *Server) shouldSendEmail(req *AlertRequest, alertType, severity string) bool {
	// Always send email for critical alerts
	if severity == "CRITICAL" {
		return true
	}

	// Send email for high severity temperature alerts
	if alertType == "TEMPERATURE_HIGH" && (severity == "HIGH" || severity == "MEDIUM") {
		return true
	}

	return false
}

// sendAlertEmail sends an email notification for the alert
func (s *Server) sendAlertEmail(alertID string, req *AlertRequest, alertType, severity string) error {
	return s.emailService.SendAlertEmail(
		alertID,
		req.DeviceID,
		req.Temperature,
		req.Threshold,
		req.Region,
	)
}

// isRateLimited checks if a device is currently rate limited
func (s *Server) isRateLimited(deviceID string) bool {
	s.rateLimitMutex.RLock()
	defer s.rateLimitMutex.RUnlock()

	if cooldownEnd, exists := s.deviceCooldowns[deviceID]; exists {
		if time.Now().Before(cooldownEnd) {
			return true
		}
	}

	return false
}

// updateCooldown updates the cooldown period for a device
func (s *Server) updateCooldown(deviceID string) {
	s.rateLimitMutex.Lock()
	defer s.rateLimitMutex.Unlock()

	cooldownEnd := time.Now().Add(s.config.CooldownPeriod)
	s.deviceCooldowns[deviceID] = cooldownEnd

	// Count active cooldowns
	activeCount := 0
	now := time.Now()
	for _, endTime := range s.deviceCooldowns {
		if now.Before(endTime) {
			activeCount++
		}
	}
	s.metrics.SetCooldownActive(activeCount)
}

// isDuplicate checks if an alert is a duplicate
func (s *Server) isDuplicate(req *AlertRequest) bool {
	s.rateLimitMutex.RLock()
	defer s.rateLimitMutex.RUnlock()

	// Create deduplication key
	dedupKey := fmt.Sprintf("%s:%.1f:%.1f", req.DeviceID, req.Temperature, req.Threshold)
	
	if lastSeen, exists := s.recentAlerts[dedupKey]; exists {
		if time.Since(lastSeen) < s.config.DeduplicationWindow {
			return true
		}
	}

	return false
}

// updateDeduplicationCache updates the deduplication cache
func (s *Server) updateDeduplicationCache(req *AlertRequest) {
	s.rateLimitMutex.Lock()
	defer s.rateLimitMutex.Unlock()

	dedupKey := fmt.Sprintf("%s:%.1f:%.1f", req.DeviceID, req.Temperature, req.Threshold)
	s.recentAlerts[dedupKey] = time.Now()
}

// cleanupWorker periodically cleans up expired cooldowns and deduplication entries
func (s *Server) cleanupWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	s.logger.Debug("Cleanup worker started")

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("Cleanup worker stopping")
			return

		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup cleans up expired entries
func (s *Server) performCleanup() {
	s.rateLimitMutex.Lock()
	defer s.rateLimitMutex.Unlock()

	now := time.Now()
	
	// Clean up expired cooldowns
	for deviceID, endTime := range s.deviceCooldowns {
		if now.After(endTime) {
			delete(s.deviceCooldowns, deviceID)
		}
	}

	// Clean up expired deduplication entries
	for key, lastSeen := range s.recentAlerts {
		if now.Sub(lastSeen) > s.config.DeduplicationWindow {
			delete(s.recentAlerts, key)
		}
	}

	// Update metrics
	activeCount := 0
	for _, endTime := range s.deviceCooldowns {
		if now.Before(endTime) {
			activeCount++
		}
	}
	s.metrics.SetCooldownActive(activeCount)

	s.logger.Debug("Cleanup completed",
		zap.Int("active_cooldowns", activeCount),
		zap.Int("recent_alerts", len(s.recentAlerts)),
	)
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	s.rateLimitMutex.RLock()
	activeCooldowns := len(s.deviceCooldowns)
	recentAlertsCount := len(s.recentAlerts)
	s.rateLimitMutex.RUnlock()

	uptime := time.Since(s.startTime)
	processingRate := float64(s.processedCount) / uptime.Seconds()

	return map[string]interface{}{
		"processed_count":     s.processedCount,
		"filtered_count":      s.filteredCount,
		"error_count":         s.errorCount,
		"processing_rate":     processingRate,
		"queue_size":          len(s.alertQueue),
		"active_cooldowns":    activeCooldowns,
		"recent_alerts":       recentAlertsCount,
		"uptime_seconds":      uptime.Seconds(),
		"is_running":          s.running,
	}
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.running
}
