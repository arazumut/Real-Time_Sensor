package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/alert-handler-service/pkg/logger"
)

// AlertRecord represents a stored alert record
type AlertRecord struct {
	AlertID     string            `json:"alert_id"`
	DeviceID    string            `json:"device_id"`
	AlertType   string            `json:"alert_type"`
	Severity    string            `json:"severity"`
	Message     string            `json:"message"`
	Temperature float64           `json:"temperature"`
	Threshold   float64           `json:"threshold"`
	Region      string            `json:"region"`
	Timestamp   time.Time         `json:"timestamp"`
	ProcessedAt time.Time         `json:"processed_at"`
	EmailSent   bool              `json:"email_sent"`
	Metadata    map[string]string `json:"metadata"`
}

// AlertStorage provides in-memory storage for alert records
type AlertStorage struct {
	alerts          map[string]*AlertRecord // alertID -> AlertRecord
	deviceIndex     map[string][]*AlertRecord // deviceID -> AlertRecords
	config          Config
	logger          logger.Logger
	metrics         MetricsRecorder
	
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mutex           sync.RWMutex
	running         bool
	
	// Performance tracking
	storedCount     int64
	cleanupCount    int64
	startTime       time.Time
}

// Config holds storage configuration
type Config struct {
	EnableStorage   bool
	RetentionPeriod time.Duration
	CleanupInterval time.Duration
	MaxStoredAlerts int
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementStoredAlerts()
	IncrementStorageErrors()
	IncrementStorageCleanup()
	SetStorageSize(size int)
}

// NewAlertStorage creates a new alert storage
func NewAlertStorage(config Config, logger logger.Logger, metrics MetricsRecorder) *AlertStorage {
	ctx, cancel := context.WithCancel(context.Background())

	storage := &AlertStorage{
		alerts:      make(map[string]*AlertRecord),
		deviceIndex: make(map[string][]*AlertRecord),
		config:      config,
		logger:      logger,
		metrics:     metrics,
		ctx:         ctx,
		cancel:      cancel,
		startTime:   time.Now(),
	}

	logger.Info("Alert storage created successfully",
		zap.Bool("enabled", config.EnableStorage),
		zap.Duration("retention_period", config.RetentionPeriod),
		zap.Int("max_stored_alerts", config.MaxStoredAlerts),
	)

	return storage
}

// Start begins the storage service
func (s *AlertStorage) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("alert storage is already running")
	}

	if !s.config.EnableStorage {
		s.logger.Info("Alert storage disabled")
		return nil
	}

	s.logger.Info("Starting alert storage")

	// Start cleanup worker
	s.wg.Add(1)
	go s.cleanupWorker()

	s.running = true
	s.logger.Info("Alert storage started successfully")

	return nil
}

// Stop gracefully stops the storage service
func (s *AlertStorage) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("alert storage is not running")
	}

	s.logger.Info("Stopping alert storage...")

	// Cancel context
	s.cancel()

	// Wait for cleanup worker to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Storage cleanup worker stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timeout waiting for storage cleanup worker to stop")
	}

	s.running = false
	s.logger.Info("Alert storage stopped")

	return nil
}

// StoreAlert stores an alert record
func (s *AlertStorage) StoreAlert(alert *AlertRecord) error {
	if !s.config.EnableStorage {
		return nil // Storage disabled
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check storage limits
	if len(s.alerts) >= s.config.MaxStoredAlerts {
		// Remove oldest alerts to make space
		s.removeOldestAlerts(s.config.MaxStoredAlerts / 10) // Remove 10% of capacity
	}

	// Store alert
	s.alerts[alert.AlertID] = alert

	// Update device index
	if _, exists := s.deviceIndex[alert.DeviceID]; !exists {
		s.deviceIndex[alert.DeviceID] = make([]*AlertRecord, 0)
	}
	s.deviceIndex[alert.DeviceID] = append(s.deviceIndex[alert.DeviceID], alert)

	// Update metrics
	s.storedCount++
	s.metrics.IncrementStoredAlerts()
	s.metrics.SetStorageSize(len(s.alerts))

	s.logger.Debug("Alert stored successfully",
		zap.String("alert_id", alert.AlertID),
		zap.String("device_id", alert.DeviceID),
		zap.String("alert_type", alert.AlertType),
		zap.Int("total_stored", len(s.alerts)),
	)

	return nil
}

// GetAlertHistory retrieves alert history for a device
func (s *AlertStorage) GetAlertHistory(deviceID string, startTime, endTime time.Time, limit int) ([]*AlertRecord, error) {
	if !s.config.EnableStorage {
		return []*AlertRecord{}, nil
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	deviceAlerts, exists := s.deviceIndex[deviceID]
	if !exists {
		return []*AlertRecord{}, nil
	}

	// Filter by time range
	var filteredAlerts []*AlertRecord
	for _, alert := range deviceAlerts {
		if alert.Timestamp.After(startTime) && alert.Timestamp.Before(endTime) {
			filteredAlerts = append(filteredAlerts, alert)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(filteredAlerts, func(i, j int) bool {
		return filteredAlerts[i].Timestamp.After(filteredAlerts[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(filteredAlerts) > limit {
		filteredAlerts = filteredAlerts[:limit]
	}

	s.logger.Debug("Alert history retrieved",
		zap.String("device_id", deviceID),
		zap.Int("total_alerts", len(filteredAlerts)),
		zap.Time("start_time", startTime),
		zap.Time("end_time", endTime),
	)

	return filteredAlerts, nil
}

// GetAlert retrieves a specific alert by ID
func (s *AlertStorage) GetAlert(alertID string) (*AlertRecord, error) {
	if !s.config.EnableStorage {
		return nil, fmt.Errorf("storage disabled")
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	alert, exists := s.alerts[alertID]
	if !exists {
		return nil, fmt.Errorf("alert not found: %s", alertID)
	}

	return alert, nil
}

// cleanupWorker periodically cleans up old alerts
func (s *AlertStorage) cleanupWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	s.logger.Debug("Storage cleanup worker started",
		zap.Duration("interval", s.config.CleanupInterval),
	)

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("Storage cleanup worker stopping")
			return

		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup removes old alerts based on retention policy
func (s *AlertStorage) performCleanup() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.alerts) == 0 {
		return
	}

	cutoffTime := time.Now().Add(-s.config.RetentionPeriod)
	var removedCount int

	// Remove expired alerts
	for alertID, alert := range s.alerts {
		if alert.Timestamp.Before(cutoffTime) {
			// Remove from main storage
			delete(s.alerts, alertID)

			// Remove from device index
			s.removeFromDeviceIndex(alert.DeviceID, alertID)
			removedCount++
		}
	}

	if removedCount > 0 {
		s.cleanupCount++
		s.metrics.IncrementStorageCleanup()
		s.metrics.SetStorageSize(len(s.alerts))

		s.logger.Info("Storage cleanup completed",
			zap.Int("removed_alerts", removedCount),
			zap.Int("remaining_alerts", len(s.alerts)),
			zap.Time("cutoff_time", cutoffTime),
		)
	}
}

// removeOldestAlerts removes the oldest alerts to free space
func (s *AlertStorage) removeOldestAlerts(count int) {
	if count <= 0 || len(s.alerts) == 0 {
		return
	}

	// Convert to slice for sorting
	alerts := make([]*AlertRecord, 0, len(s.alerts))
	for _, alert := range s.alerts {
		alerts = append(alerts, alert)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Timestamp.Before(alerts[j].Timestamp)
	})

	// Remove oldest alerts
	removeCount := count
	if removeCount > len(alerts) {
		removeCount = len(alerts)
	}

	for i := 0; i < removeCount; i++ {
		alert := alerts[i]
		delete(s.alerts, alert.AlertID)
		s.removeFromDeviceIndex(alert.DeviceID, alert.AlertID)
	}

	s.logger.Debug("Removed oldest alerts to free space",
		zap.Int("removed_count", removeCount),
		zap.Int("remaining_count", len(s.alerts)),
	)
}

// removeFromDeviceIndex removes an alert from the device index
func (s *AlertStorage) removeFromDeviceIndex(deviceID, alertID string) {
	if deviceAlerts, exists := s.deviceIndex[deviceID]; exists {
		for i, alert := range deviceAlerts {
			if alert.AlertID == alertID {
				// Remove from slice
				s.deviceIndex[deviceID] = append(deviceAlerts[:i], deviceAlerts[i+1:]...)
				break
			}
		}

		// Remove device entry if no alerts remain
		if len(s.deviceIndex[deviceID]) == 0 {
			delete(s.deviceIndex, deviceID)
		}
	}
}

// GetStats returns storage statistics
func (s *AlertStorage) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uptime := time.Since(s.startTime)

	return map[string]interface{}{
		"stored_count":       s.storedCount,
		"cleanup_count":      s.cleanupCount,
		"current_size":       len(s.alerts),
		"device_count":       len(s.deviceIndex),
		"uptime_seconds":     uptime.Seconds(),
		"is_running":         s.running,
		"storage_enabled":    s.config.EnableStorage,
	}
}

// IsRunning returns whether the storage is currently running
func (s *AlertStorage) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.running
}
