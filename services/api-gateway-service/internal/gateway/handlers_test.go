package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/api-gateway-service/internal/clickhouse"
	"github.com/twinup/sensor-system/services/api-gateway-service/pkg/logger"
)

// Mock implementations for testing
type MockCacheService struct {
	mock.Mock
}

func (m *MockCacheService) Get(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}

func (m *MockCacheService) Set(key, value string, ttl time.Duration) error {
	args := m.Called(key, value, ttl)
	return args.Error(0)
}

func (m *MockCacheService) Delete(key string) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *MockCacheService) Exists(key string) (bool, error) {
	args := m.Called(key)
	return args.Bool(0), args.Error(1)
}

func (m *MockCacheService) GetStats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func (m *MockCacheService) HealthCheck() error {
	args := m.Called()
	return args.Error(0)
}

type MockClickHouseService struct {
	mock.Mock
}

func (m *MockClickHouseService) GetLatestReading(deviceID string) (*clickhouse.SensorReading, error) {
	args := m.Called(deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clickhouse.SensorReading), args.Error(1)
}

func (m *MockClickHouseService) GetDeviceList(page, pageSize int) ([]*clickhouse.DeviceInfo, int, error) {
	args := m.Called(page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*clickhouse.DeviceInfo), args.Int(1), args.Error(2)
}

func (m *MockClickHouseService) GetDeviceHistory(deviceID string, startTime, endTime time.Time, limit int) ([]*clickhouse.SensorReading, error) {
	args := m.Called(deviceID, startTime, endTime, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*clickhouse.SensorReading), args.Error(1)
}

func (m *MockClickHouseService) GetStats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func (m *MockClickHouseService) HealthCheck() error {
	args := m.Called()
	return args.Error(0)
}

type MockMetricsRecorder struct {
	mock.Mock
}

func (m *MockMetricsRecorder) IncrementHTTPRequests(method, endpoint, status string) {
	m.Called(method, endpoint, status)
}
func (m *MockMetricsRecorder) RecordHTTPLatency(method, endpoint string, duration time.Duration) {
	m.Called(method, endpoint, duration)
}
func (m *MockMetricsRecorder) RecordHTTPResponseSize(size int) { m.Called(size) }
func (m *MockMetricsRecorder) IncrementHTTPErrors(method, endpoint, errorType string) {
	m.Called(method, endpoint, errorType)
}
func (m *MockMetricsRecorder) IncrementCacheRequests(operation, result string) {
	m.Called(operation, result)
}
func (m *MockMetricsRecorder) RecordCacheLatency(operation string, duration time.Duration) {
	m.Called(operation, duration)
}
func (m *MockMetricsRecorder) IncrementCacheErrors() { m.Called() }
func (m *MockMetricsRecorder) IncrementDBRequests(operation, result string) {
	m.Called(operation, result)
}
func (m *MockMetricsRecorder) RecordDBLatency(operation string, duration time.Duration) {
	m.Called(operation, duration)
}
func (m *MockMetricsRecorder) IncrementDBErrors()   { m.Called() }
func (m *MockMetricsRecorder) IncrementDBFallback() { m.Called() }
func (m *MockMetricsRecorder) IncrementDeviceRequests(deviceID, operation string) {
	m.Called(deviceID, operation)
}
func (m *MockMetricsRecorder) RecordDeviceResponseTime(duration time.Duration) { m.Called(duration) }
func (m *MockMetricsRecorder) IncrementDeviceErrors(deviceID, errorType string) {
	m.Called(deviceID, errorType)
}
func (m *MockMetricsRecorder) IncrementAnalyticsRequests(analyticsType string) {
	m.Called(analyticsType)
}
func (m *MockMetricsRecorder) RecordAnalyticsLatency(duration time.Duration) { m.Called(duration) }
func (m *MockMetricsRecorder) IncrementAnalyticsErrors()                     { m.Called() }
func (m *MockMetricsRecorder) SetConcurrentRequests(count int)               { m.Called(count) }
func (m *MockMetricsRecorder) SetActiveDevices(count int)                    { m.Called(count) }

type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...zap.Field)  { m.Called(msg, fields) }
func (m *MockLogger) Info(msg string, fields ...zap.Field)   { m.Called(msg, fields) }
func (m *MockLogger) Warn(msg string, fields ...zap.Field)   { m.Called(msg, fields) }
func (m *MockLogger) Error(msg string, fields ...zap.Field)  { m.Called(msg, fields) }
func (m *MockLogger) Fatal(msg string, fields ...zap.Field)  { m.Called(msg, fields) }
func (m *MockLogger) With(fields ...zap.Field) logger.Logger { return m }
func (m *MockLogger) Sync() error                            { return nil }

func TestGetDeviceMetrics_CacheHit(t *testing.T) {
	// Setup mocks
	mockCache := &MockCacheService{}
	mockDB := &MockClickHouseService{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations
	cacheData := `{
		"device_id": "test_device_001",
		"last_update": "2023-09-26T10:00:00Z",
		"temperature": 25.5,
		"humidity": 60.0,
		"pressure": 1013.25,
		"location_lat": 41.0,
		"location_lng": 29.0,
		"status": "active"
	}`

	mockCache.On("Get", "device:test_device_001:snapshot").Return(cacheData, nil)
	mockMetrics.On("IncrementDeviceRequests", "test_device_001", "get_metrics").Return()
	mockMetrics.On("IncrementCacheRequests", "get", "hit").Return()
	mockMetrics.On("RecordCacheLatency", "get", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("RecordDeviceResponseTime", mock.AnythingOfType("time.Duration")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		EnableCaching: true,
	}

	handlers := NewHandlers(mockCache, mockDB, mockLogger, mockMetrics, config)

	// Setup Gin test context
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/devices/:id/metrics", handlers.GetDeviceMetrics)

	// Create test request
	req, _ := http.NewRequest("GET", "/devices/test_device_001/metrics", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.NotNil(t, response["data"])

	data := response["data"].(map[string]interface{})
	assert.Equal(t, "test_device_001", data["device_id"])
	assert.Equal(t, "cache", data["source"])

	// Verify mock expectations
	mockCache.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGetDeviceMetrics_CacheMissDBFallback(t *testing.T) {
	// Setup mocks
	mockCache := &MockCacheService{}
	mockDB := &MockClickHouseService{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations - cache miss, DB hit
	mockCache.On("Get", "device:test_device_002:snapshot").Return("", fmt.Errorf("not found"))

	dbReading := &clickhouse.SensorReading{
		DeviceID:    "test_device_002",
		Timestamp:   time.Now(),
		Temperature: 28.5,
		Humidity:    65.0,
		Pressure:    1015.0,
		LocationLat: 42.0,
		LocationLng: 30.0,
		Status:      "active",
		CreatedAt:   time.Now(),
	}

	mockDB.On("GetLatestReading", "test_device_002").Return(dbReading, nil)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil).Maybe()

	mockMetrics.On("IncrementDeviceRequests", "test_device_002", "get_metrics").Return()
	mockMetrics.On("IncrementCacheRequests", "get", "miss").Return()
	mockMetrics.On("RecordCacheLatency", "get", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementDBFallback").Return()
	mockMetrics.On("IncrementDBRequests", "get_device_metrics", "success").Return()
	mockMetrics.On("RecordDBLatency", "get_latest_reading", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("RecordDeviceResponseTime", mock.AnythingOfType("time.Duration")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	config := Config{
		EnableCaching:        true,
		FallbackToClickHouse: true,
	}

	handlers := NewHandlers(mockCache, mockDB, mockLogger, mockMetrics, config)

	// Setup Gin test context
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/devices/:id/metrics", handlers.GetDeviceMetrics)

	// Create test request
	req, _ := http.NewRequest("GET", "/devices/test_device_002/metrics", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.NotNil(t, response["data"])

	data := response["data"].(map[string]interface{})
	assert.Equal(t, "test_device_002", data["device_id"])
	assert.Equal(t, "database", data["source"])

	// Verify mock expectations
	mockCache.AssertExpectations(t)
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGetDeviceMetrics_NotFound(t *testing.T) {
	// Setup mocks
	mockCache := &MockCacheService{}
	mockDB := &MockClickHouseService{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations - both cache and DB miss
	mockCache.On("Get", "device:nonexistent:snapshot").Return("", fmt.Errorf("not found"))
	mockDB.On("GetLatestReading", "nonexistent").Return(nil, fmt.Errorf("device not found"))

	mockMetrics.On("IncrementDeviceRequests", "nonexistent", "get_metrics").Return()
	mockMetrics.On("IncrementCacheRequests", "get", "miss").Return()
	mockMetrics.On("RecordCacheLatency", "get", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementDBFallback").Return()
	mockMetrics.On("IncrementDBRequests", "get_device_metrics", "error").Return()
	mockMetrics.On("IncrementDeviceErrors", "nonexistent", "not_found").Return()
	mockMetrics.On("RecordDBLatency", "get_latest_reading", mock.AnythingOfType("time.Duration")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	config := Config{
		EnableCaching:        true,
		FallbackToClickHouse: true,
	}

	handlers := NewHandlers(mockCache, mockDB, mockLogger, mockMetrics, config)

	// Setup Gin test context
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/devices/:id/metrics", handlers.GetDeviceMetrics)

	// Create test request
	req, _ := http.NewRequest("GET", "/devices/nonexistent/metrics", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "device_not_found", response["error"])
	assert.Contains(t, response["message"], "nonexistent")
}

func TestGetDeviceList(t *testing.T) {
	// Setup mocks
	mockCache := &MockCacheService{}
	mockDB := &MockClickHouseService{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations
	dbDevices := []*clickhouse.DeviceInfo{
		{
			DeviceID:    "device_001",
			LastSeen:    time.Now(),
			Status:      "active",
			Region:      "north",
			Temperature: 25.0,
		},
		{
			DeviceID:    "device_002",
			LastSeen:    time.Now().Add(-time.Hour),
			Status:      "active",
			Region:      "south",
			Temperature: 30.0,
		},
	}

	mockDB.On("GetDeviceList", 1, 100).Return(dbDevices, 2, nil)
	mockMetrics.On("RecordDBLatency", "get_device_list", mock.AnythingOfType("time.Duration")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		EnablePagination: true,
		DefaultPageSize:  100,
		MaxPageSize:      1000,
	}

	handlers := NewHandlers(mockCache, mockDB, mockLogger, mockMetrics, config)

	// Setup Gin test context
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/devices", handlers.GetDeviceList)

	// Create test request
	req, _ := http.NewRequest("GET", "/devices", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.NotNil(t, response["data"])

	data := response["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total_count"])
	assert.Equal(t, float64(1), data["page"])
	assert.Equal(t, float64(100), data["page_size"])
	assert.False(t, data["has_more"].(bool))

	// Verify mock expectations
	mockDB.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestGetDeviceAnalytics(t *testing.T) {
	// Setup mocks
	mockCache := &MockCacheService{}
	mockDB := &MockClickHouseService{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockMetrics.On("IncrementAnalyticsRequests", "trend").Return()
	mockMetrics.On("RecordAnalyticsLatency", mock.AnythingOfType("time.Duration")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	config := Config{}
	handlers := NewHandlers(mockCache, mockDB, mockLogger, mockMetrics, config)

	// Setup Gin test context
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/devices/:id/analytics", handlers.GetDeviceAnalytics)

	// Create test request
	req, _ := http.NewRequest("GET", "/devices/test_device/analytics?type=trend", nil)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.NotNil(t, response["data"])

	data := response["data"].(map[string]interface{})
	assert.Equal(t, "test_device", data["device_id"])
	assert.Equal(t, "trend", data["analytics_type"])
	assert.NotNil(t, data["data"])

	// Verify mock expectations
	mockMetrics.AssertExpectations(t)
}

func TestHealthCheck(t *testing.T) {
	// Setup mocks
	mockCache := &MockCacheService{}
	mockDB := &MockClickHouseService{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Test healthy case
	t.Run("All services healthy", func(t *testing.T) {
		mockCache.On("HealthCheck").Return(nil)
		mockDB.On("HealthCheck").Return(nil)

		handlers := NewHandlers(mockCache, mockDB, mockLogger, mockMetrics, Config{})

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/health", handlers.HealthCheck)

		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response["status"])
		assert.True(t, response["cache_healthy"].(bool))
		assert.True(t, response["db_healthy"].(bool))
	})

	// Test degraded case
	t.Run("Cache unhealthy", func(t *testing.T) {
		mockCache2 := &MockCacheService{}
		mockDB2 := &MockClickHouseService{}

		mockCache2.On("HealthCheck").Return(fmt.Errorf("cache error"))
		mockDB2.On("HealthCheck").Return(nil)

		handlers := NewHandlers(mockCache2, mockDB2, mockLogger, mockMetrics, Config{})

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/health", handlers.HealthCheck)

		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // Still 200 but degraded

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "degraded", response["status"])
		assert.False(t, response["cache_healthy"].(bool))
		assert.True(t, response["db_healthy"].(bool))
	})
}

func TestPaginationParams(t *testing.T) {
	config := Config{
		DefaultPageSize: 50,
		MaxPageSize:     200,
	}

	handlers := &Handlers{config: config}

	tests := []struct {
		name             string
		queryParams      string
		expectedPage     int
		expectedPageSize int
	}{
		{
			name:             "Default parameters",
			queryParams:      "",
			expectedPage:     1,
			expectedPageSize: 50,
		},
		{
			name:             "Custom page",
			queryParams:      "?page=3",
			expectedPage:     3,
			expectedPageSize: 50,
		},
		{
			name:             "Custom page size",
			queryParams:      "?page_size=25",
			expectedPage:     1,
			expectedPageSize: 25,
		},
		{
			name:             "Page size exceeds max",
			queryParams:      "?page_size=500",
			expectedPage:     1,
			expectedPageSize: 200, // Should be capped at max
		},
		{
			name:             "Invalid parameters",
			queryParams:      "?page=invalid&page_size=invalid",
			expectedPage:     1,
			expectedPageSize: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock Gin context
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			req, _ := http.NewRequest("GET", "/test"+tt.queryParams, nil)
			c.Request = req

			page, pageSize := handlers.parsePaginationParams(c)

			assert.Equal(t, tt.expectedPage, page)
			assert.Equal(t, tt.expectedPageSize, pageSize)
		})
	}
}

// Benchmark tests
func BenchmarkGetDeviceMetrics_CacheHit(b *testing.B) {
	// Setup lightweight mocks
	mockCache := &MockCacheService{}
	mockDB := &MockClickHouseService{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	cacheData := `{"device_id":"bench_device","temperature":25.0}`
	mockCache.On("Get", mock.Anything).Return(cacheData, nil)
	mockMetrics.On("IncrementDeviceRequests", mock.Anything, mock.Anything).Return()
	mockMetrics.On("IncrementCacheRequests", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordCacheLatency", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordDeviceResponseTime", mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{EnableCaching: true}
	handlers := NewHandlers(mockCache, mockDB, mockLogger, mockMetrics, config)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/devices/:id/metrics", handlers.GetDeviceMetrics)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("GET", "/devices/bench_device/metrics", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}
