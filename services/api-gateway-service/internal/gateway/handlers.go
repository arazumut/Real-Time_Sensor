package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/api-gateway-service/internal/clickhouse"
	"github.com/twinup/sensor-system/services/api-gateway-service/pkg/logger"
)

// DeviceMetrics represents device metrics response
type DeviceMetrics struct {
	DeviceID     string    `json:"device_id"`
	LastUpdate   time.Time `json:"last_update"`
	Temperature  float64   `json:"temperature"`
	Humidity     float64   `json:"humidity"`
	Pressure     float64   `json:"pressure"`
	LocationLat  float64   `json:"location_lat"`
	LocationLng  float64   `json:"location_lng"`
	Status       string    `json:"status"`
	Source       string    `json:"source"` // "cache" or "database"
	ResponseTime string    `json:"response_time"`
}

// DeviceList represents list of devices response
type DeviceList struct {
	Devices    []*DeviceInfo `json:"devices"`
	TotalCount int           `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	HasMore    bool          `json:"has_more"`
}

// DeviceInfo represents basic device information
type DeviceInfo struct {
	DeviceID    string    `json:"device_id"`
	LastSeen    time.Time `json:"last_seen"`
	Status      string    `json:"status"`
	Region      string    `json:"region"`
	Temperature float64   `json:"temperature"`
}

// AnalyticsData represents analytics response
type AnalyticsData struct {
	DeviceID      string                 `json:"device_id"`
	AnalyticsType string                 `json:"analytics_type"`
	Data          map[string]interface{} `json:"data"`
	GeneratedAt   time.Time              `json:"generated_at"`
	Source        string                 `json:"source"`
}

// CacheService interface for cache operations
type CacheService interface {
	Get(key string) (string, error)
	Set(key, value string, ttl time.Duration) error
	Delete(key string) error
	Exists(key string) (bool, error)
	GetStats() map[string]interface{}
	HealthCheck() error
}

// DatabaseService interface for database operations
type DatabaseService interface {
	GetLatestReading(deviceID string) (*clickhouse.SensorReading, error)
	GetDeviceList(page, pageSize int) ([]*clickhouse.DeviceInfo, int, error)
	GetDeviceHistory(deviceID string, startTime, endTime time.Time, limit int) ([]*clickhouse.SensorReading, error)
	GetStats() map[string]interface{}
	HealthCheck() error
}

// Handlers contains all HTTP handlers for the API gateway
type Handlers struct {
	cacheService CacheService
	dbService    DatabaseService
	logger       logger.Logger
	metrics      MetricsRecorder
	config       Config
}

// Config holds gateway configuration
type Config struct {
	EnableCaching         bool
	CacheDefaultTTL       time.Duration
	FallbackToClickHouse  bool
	MaxConcurrentRequests int
	RequestTimeout        time.Duration
	EnablePagination      bool
	DefaultPageSize       int
	MaxPageSize           int
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementHTTPRequests(method, endpoint, status string)
	RecordHTTPLatency(method, endpoint string, duration time.Duration)
	RecordHTTPResponseSize(size int)
	IncrementHTTPErrors(method, endpoint, errorType string)
	IncrementCacheRequests(operation, result string)
	RecordCacheLatency(operation string, duration time.Duration)
	IncrementCacheErrors()
	IncrementDBRequests(operation, result string)
	RecordDBLatency(operation string, duration time.Duration)
	IncrementDBErrors()
	IncrementDBFallback()
	IncrementDeviceRequests(deviceID, operation string)
	RecordDeviceResponseTime(duration time.Duration)
	IncrementDeviceErrors(deviceID, errorType string)
	IncrementAnalyticsRequests(analyticsType string)
	RecordAnalyticsLatency(duration time.Duration)
	IncrementAnalyticsErrors()
	SetConcurrentRequests(count int)
	SetActiveDevices(count int)
}

// NewHandlers creates new API gateway handlers
func NewHandlers(
	cacheService CacheService,
	dbService DatabaseService,
	logger logger.Logger,
	metrics MetricsRecorder,
	config Config,
) *Handlers {
	return &Handlers{
		cacheService: cacheService,
		dbService:    dbService,
		logger:       logger,
		metrics:      metrics,
		config:       config,
	}
}

// GetDeviceMetrics handles GET /api/v1/devices/{id}/metrics
func (h *Handlers) GetDeviceMetrics(c *gin.Context) {
	startTime := time.Now()
	deviceID := c.Param("id")

	if deviceID == "" {
		h.metrics.IncrementHTTPErrors("GET", "/devices/:id/metrics", "missing_device_id")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_device_id",
			"message": "Device ID is required",
		})
		return
	}

	h.logger.Debug("Getting device metrics",
		zap.String("device_id", deviceID),
		zap.String("client_ip", c.ClientIP()),
	)

	// Record device request
	h.metrics.IncrementDeviceRequests(deviceID, "get_metrics")

	// Try cache first
	var metrics *DeviceMetrics
	var err error
	var source string

	if h.config.EnableCaching {
		metrics, err = h.getDeviceMetricsFromCache(deviceID)
		if err == nil {
			source = "cache"
			h.metrics.IncrementCacheRequests("get", "hit")
		} else {
			h.metrics.IncrementCacheRequests("get", "miss")
		}
	}

	// Fallback to database if cache miss or caching disabled
	if metrics == nil && h.config.FallbackToClickHouse {
		h.metrics.IncrementDBFallback()
		metrics, err = h.getDeviceMetricsFromDB(deviceID)
		if err == nil {
			source = "database"
			h.metrics.IncrementDBRequests("get_device_metrics", "success")

			// Cache the result for future requests
			if h.config.EnableCaching {
				go h.cacheDeviceMetrics(deviceID, metrics)
			}
		} else {
			h.metrics.IncrementDBRequests("get_device_metrics", "error")
		}
	}

	// Handle errors
	if metrics == nil {
		h.metrics.IncrementDeviceErrors(deviceID, "not_found")
		h.logger.Warn("Device metrics not found",
			zap.String("device_id", deviceID),
			zap.Error(err),
		)

		c.JSON(http.StatusNotFound, gin.H{
			"error":   "device_not_found",
			"message": fmt.Sprintf("Device %s not found", deviceID),
		})
		return
	}

	// Set response metadata
	metrics.Source = source
	metrics.ResponseTime = time.Since(startTime).String()

	// Record metrics
	responseTime := time.Since(startTime)
	h.metrics.RecordDeviceResponseTime(responseTime)

	h.logger.Info("Device metrics retrieved successfully",
		zap.String("device_id", deviceID),
		zap.String("source", source),
		zap.Duration("response_time", responseTime),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// GetDeviceList handles GET /api/v1/devices
func (h *Handlers) GetDeviceList(c *gin.Context) {
	startTime := time.Now()

	// Parse pagination parameters
	page, pageSize := h.parsePaginationParams(c)

	h.logger.Debug("Getting device list",
		zap.Int("page", page),
		zap.Int("page_size", pageSize),
	)

	// Get devices from database (cache would be too complex for list operations)
	devices, totalCount, err := h.getDeviceListFromDB(page, pageSize)
	if err != nil {
		h.metrics.IncrementDBErrors()
		h.logger.Error("Failed to get device list",
			zap.Error(err),
			zap.Int("page", page),
			zap.Int("page_size", pageSize),
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to retrieve device list",
		})
		return
	}

	// Calculate pagination metadata
	hasMore := (page * pageSize) < totalCount

	deviceList := &DeviceList{
		Devices:    devices,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		HasMore:    hasMore,
	}

	responseTime := time.Since(startTime)
	h.logger.Info("Device list retrieved successfully",
		zap.Int("device_count", len(devices)),
		zap.Int("total_count", totalCount),
		zap.Int("page", page),
		zap.Duration("response_time", responseTime),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    deviceList,
	})
}

// GetDeviceAnalytics handles GET /api/v1/devices/{id}/analytics
func (h *Handlers) GetDeviceAnalytics(c *gin.Context) {
	startTime := time.Now()
	deviceID := c.Param("id")

	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_device_id",
			"message": "Device ID is required",
		})
		return
	}

	analyticsType := c.DefaultQuery("type", "all")
	h.metrics.IncrementAnalyticsRequests(analyticsType)

	h.logger.Debug("Getting device analytics",
		zap.String("device_id", deviceID),
		zap.String("analytics_type", analyticsType),
	)

	// Get analytics data (would integrate with analytics service in real implementation)
	analytics := h.generateMockAnalytics(deviceID, analyticsType)

	responseTime := time.Since(startTime)
	h.metrics.RecordAnalyticsLatency(responseTime)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    analytics,
	})
}

// HealthCheck handles GET /api/v1/health
func (h *Handlers) HealthCheck(c *gin.Context) {
	// Check cache health
	cacheHealthy := true
	if err := h.cacheService.HealthCheck(); err != nil {
		cacheHealthy = false
	}

	// Check database health
	dbHealthy := true
	if err := h.dbService.HealthCheck(); err != nil {
		dbHealthy = false
	}

	overall := cacheHealthy && dbHealthy
	status := "healthy"
	httpStatus := http.StatusOK

	if !overall {
		status = "degraded"
		if !cacheHealthy && !dbHealthy {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}
	}

	c.JSON(httpStatus, gin.H{
		"status":        status,
		"timestamp":     time.Now(),
		"cache_healthy": cacheHealthy,
		"db_healthy":    dbHealthy,
		"version":       "1.0.0",
	})
}

// getDeviceMetricsFromCache retrieves device metrics from Redis cache
func (h *Handlers) getDeviceMetricsFromCache(deviceID string) (*DeviceMetrics, error) {
	startTime := time.Now()

	key := fmt.Sprintf("device:%s:snapshot", deviceID)
	data, err := h.cacheService.Get(key)

	h.metrics.RecordCacheLatency("get", time.Since(startTime))

	if err != nil {
		return nil, err
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal([]byte(data), &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	// Convert to DeviceMetrics
	metrics := &DeviceMetrics{
		DeviceID:    deviceID,
		Temperature: snapshot["temperature"].(float64),
		Humidity:    snapshot["humidity"].(float64),
		Pressure:    snapshot["pressure"].(float64),
		LocationLat: snapshot["location_lat"].(float64),
		LocationLng: snapshot["location_lng"].(float64),
		Status:      snapshot["status"].(string),
	}

	// Parse last_update time
	if lastUpdateStr, ok := snapshot["last_update"].(string); ok {
		if lastUpdate, err := time.Parse(time.RFC3339, lastUpdateStr); err == nil {
			metrics.LastUpdate = lastUpdate
		}
	}

	return metrics, nil
}

// getDeviceMetricsFromDB retrieves device metrics from ClickHouse
func (h *Handlers) getDeviceMetricsFromDB(deviceID string) (*DeviceMetrics, error) {
	startTime := time.Now()

	reading, err := h.dbService.GetLatestReading(deviceID)

	h.metrics.RecordDBLatency("get_latest_reading", time.Since(startTime))

	if err != nil {
		return nil, err
	}

	metrics := &DeviceMetrics{
		DeviceID:    reading.DeviceID,
		LastUpdate:  reading.Timestamp,
		Temperature: reading.Temperature,
		Humidity:    reading.Humidity,
		Pressure:    reading.Pressure,
		LocationLat: reading.LocationLat,
		LocationLng: reading.LocationLng,
		Status:      reading.Status,
	}

	return metrics, nil
}

// cacheDeviceMetrics caches device metrics for future requests
func (h *Handlers) cacheDeviceMetrics(deviceID string, metrics *DeviceMetrics) {
	key := fmt.Sprintf("device:%s:snapshot", deviceID)

	// Convert to cache format
	snapshot := map[string]interface{}{
		"device_id":    metrics.DeviceID,
		"last_update":  metrics.LastUpdate.Format(time.RFC3339),
		"temperature":  metrics.Temperature,
		"humidity":     metrics.Humidity,
		"pressure":     metrics.Pressure,
		"location_lat": metrics.LocationLat,
		"location_lng": metrics.LocationLng,
		"status":       metrics.Status,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		h.logger.Error("Failed to marshal cache data",
			zap.Error(err),
			zap.String("device_id", deviceID),
		)
		return
	}

	if err := h.cacheService.Set(key, string(data), h.config.CacheDefaultTTL); err != nil {
		h.logger.Error("Failed to cache device metrics",
			zap.Error(err),
			zap.String("device_id", deviceID),
		)
		h.metrics.IncrementCacheErrors()
	}
}

// getDeviceListFromDB retrieves device list from ClickHouse
func (h *Handlers) getDeviceListFromDB(page, pageSize int) ([]*DeviceInfo, int, error) {
	startTime := time.Now()

	dbDevices, totalCount, err := h.dbService.GetDeviceList(page, pageSize)

	h.metrics.RecordDBLatency("get_device_list", time.Since(startTime))

	if err != nil {
		return nil, 0, err
	}

	// Convert ClickHouse DeviceInfo to Gateway DeviceInfo
	var devices []*DeviceInfo
	for _, dbDevice := range dbDevices {
		device := &DeviceInfo{
			DeviceID:    dbDevice.DeviceID,
			LastSeen:    dbDevice.LastSeen,
			Status:      dbDevice.Status,
			Region:      dbDevice.Region,
			Temperature: dbDevice.Temperature,
		}
		devices = append(devices, device)
	}

	return devices, totalCount, nil
}

// generateMockAnalytics generates mock analytics data for demonstration
func (h *Handlers) generateMockAnalytics(deviceID, analyticsType string) *AnalyticsData {
	// Mock analytics data - in real implementation, this would come from analytics service
	data := map[string]interface{}{
		"trend": map[string]interface{}{
			"direction": "increasing",
			"strength":  0.75,
			"slope":     0.5,
		},
		"delta": map[string]interface{}{
			"current_value":  25.5,
			"previous_value": 24.0,
			"delta":          1.5,
			"delta_percent":  6.25,
		},
		"regional": map[string]interface{}{
			"region":          "north",
			"device_count":    150,
			"avg_temperature": 24.8,
			"min_temperature": 18.2,
			"max_temperature": 31.5,
		},
	}

	selectedData := data
	if analyticsType != "all" {
		if typeData, exists := data[analyticsType]; exists {
			selectedData = map[string]interface{}{
				analyticsType: typeData,
			}
		}
	}

	return &AnalyticsData{
		DeviceID:      deviceID,
		AnalyticsType: analyticsType,
		Data:          selectedData,
		GeneratedAt:   time.Now(),
		Source:        "analytics_service",
	}
}

// parsePaginationParams parses pagination parameters from request
func (h *Handlers) parsePaginationParams(c *gin.Context) (page, pageSize int) {
	// Parse page parameter
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	// Parse page_size parameter
	pageSizeStr := c.DefaultQuery("page_size", fmt.Sprintf("%d", h.config.DefaultPageSize))
	pageSize, err = strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = h.config.DefaultPageSize
	}

	// Enforce max page size
	if pageSize > h.config.MaxPageSize {
		pageSize = h.config.MaxPageSize
	}

	return page, pageSize
}

// GetSystemStats handles GET /api/v1/stats
func (h *Handlers) GetSystemStats(c *gin.Context) {
	startTime := time.Now()

	// Get system statistics
	stats := map[string]interface{}{
		"cache":    h.cacheService.GetStats(),
		"database": h.dbService.GetStats(),
		"gateway": map[string]interface{}{
			"uptime":    time.Since(startTime).String(),
			"version":   "1.0.0",
			"timestamp": time.Now(),
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetDeviceHistory handles GET /api/v1/devices/{id}/history
func (h *Handlers) GetDeviceHistory(c *gin.Context) {
	startTime := time.Now()
	deviceID := c.Param("id")

	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_device_id",
			"message": "Device ID is required",
		})
		return
	}

	// Parse time range parameters
	startTimeStr := c.DefaultQuery("start_time", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	endTimeStr := c.DefaultQuery("end_time", time.Now().Format(time.RFC3339))
	limitStr := c.DefaultQuery("limit", "1000")

	startTimeParam, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_start_time",
			"message": "Start time must be in RFC3339 format",
		})
		return
	}

	endTimeParam, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_end_time",
			"message": "End time must be in RFC3339 format",
		})
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 1000
	}

	// Get historical data from database
	history, err := h.dbService.GetDeviceHistory(deviceID, startTimeParam, endTimeParam, limit)
	if err != nil {
		h.metrics.IncrementDBErrors()
		h.logger.Error("Failed to get device history",
			zap.Error(err),
			zap.String("device_id", deviceID),
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to retrieve device history",
		})
		return
	}

	responseTime := time.Since(startTime)
	h.metrics.RecordDeviceResponseTime(responseTime)

	h.logger.Info("Device history retrieved successfully",
		zap.String("device_id", deviceID),
		zap.Int("record_count", len(history)),
		zap.Duration("response_time", responseTime),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"device_id":    deviceID,
			"start_time":   startTimeParam,
			"end_time":     endTimeParam,
			"record_count": len(history),
			"readings":     history,
		},
	})
}
