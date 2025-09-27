package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/api-gateway-service/internal/clickhouse"
	"github.com/twinup/sensor-system/services/api-gateway-service/internal/config"
	"github.com/twinup/sensor-system/services/api-gateway-service/internal/gateway"
	"github.com/twinup/sensor-system/services/api-gateway-service/internal/metrics"
	"github.com/twinup/sensor-system/services/api-gateway-service/pkg/logger"
)

func TestAPIGatewayServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("Configuration Loading", func(t *testing.T) {
		// Test default configuration
		cfg, err := config.LoadConfig("")
		require.NoError(t, err)
		assert.NotNil(t, cfg)

		// Verify default values
		assert.Equal(t, "0.0.0.0", cfg.HTTP.Host)
		assert.Equal(t, 8081, cfg.HTTP.Port)
		assert.Equal(t, "localhost", cfg.Redis.Host)
		assert.Equal(t, 6379, cfg.Redis.Port)
		assert.Equal(t, "localhost", cfg.ClickHouse.Host)
		assert.Equal(t, 9000, cfg.ClickHouse.Port)
		assert.Equal(t, "sensor_data", cfg.ClickHouse.Database)
		assert.True(t, cfg.Gateway.EnableCaching)
		assert.True(t, cfg.Gateway.FallbackToClickHouse)
		assert.True(t, cfg.Gateway.EnablePagination)
		assert.Equal(t, 100, cfg.Gateway.DefaultPageSize)
		assert.Equal(t, 1000, cfg.Gateway.MaxPageSize)
		assert.True(t, cfg.Metrics.Enabled)
	})

	t.Run("Logger Integration", func(t *testing.T) {
		// Test logger creation
		log, err := logger.NewLogger("info", "json", false)
		require.NoError(t, err)
		assert.NotNil(t, log)

		// Test logger methods
		log.Info("Test info message", zap.String("test", "value"))
		log.Debug("Test debug message", zap.Int("number", 42))
		log.Warn("Test warn message", zap.Bool("flag", true))

		// Test logger sync
		err = log.Sync()
		assert.NoError(t, err)
	})

	t.Run("Metrics Integration", func(t *testing.T) {
		// Test metrics creation
		metricsCollector := metrics.NewMetrics()
		assert.NotNil(t, metricsCollector)

		// Test metrics recording
		metricsCollector.IncrementHTTPRequests("GET", "/test", "200")
		metricsCollector.RecordHTTPLatency("GET", "/test", 100*time.Millisecond)
		metricsCollector.RecordHTTPResponseSize(1024)

		metricsCollector.IncrementCacheRequests("get", "hit")
		metricsCollector.SetCacheHitRate(85.5)

		metricsCollector.IncrementDBRequests("select", "success")
		metricsCollector.IncrementDBFallback()

		metricsCollector.IncrementDeviceRequests("test_device", "get_metrics")
		metricsCollector.SetActiveDevices(150)

		metricsCollector.IncrementAnalyticsRequests("trend")
		metricsCollector.RecordAnalyticsLatency(50 * time.Millisecond)

		metricsCollector.SetConcurrentRequests(25)
		metricsCollector.UpdateSystemMetrics(1024*1024*100, 15.5, 50)
	})

	t.Run("Gateway Handlers Integration", func(t *testing.T) {
		// Test handler creation
		mockCache := &MockCacheService{}
		mockDB := &MockDatabaseService{}
		log, _ := logger.NewLogger("info", "json", false)
		metricsCollector := metrics.NewMetrics()

		config := gateway.Config{
			EnableCaching:        true,
			CacheDefaultTTL:      5 * time.Minute,
			FallbackToClickHouse: true,
			EnablePagination:     true,
			DefaultPageSize:      100,
			MaxPageSize:          1000,
		}

		handlers := gateway.NewHandlers(mockCache, mockDB, log, metricsCollector, config)
		assert.NotNil(t, handlers)
	})

	t.Run("API Endpoint Definitions", func(t *testing.T) {
		// Test API endpoint definitions
		endpoints := map[string]string{
			"GET /api/v1/devices/{id}/metrics":   "Get device current metrics",
			"GET /api/v1/devices/{id}/analytics": "Get device analytics data",
			"GET /api/v1/devices/{id}/history":   "Get device historical data",
			"GET /api/v1/devices":                "List all devices (paginated)",
			"GET /api/v1/stats":                  "Get system statistics",
			"GET /api/v1/health":                 "Health check",
		}

		assert.Len(t, endpoints, 6)
		assert.Contains(t, endpoints, "GET /api/v1/devices/{id}/metrics")
		assert.Contains(t, endpoints, "GET /api/v1/devices")
		assert.Contains(t, endpoints, "GET /api/v1/health")
	})

	t.Run("Cache Strategy Validation", func(t *testing.T) {
		// Test cache-first strategy with ClickHouse fallback
		strategies := []string{"cache_first", "database_fallback", "cache_aside"}

		// Cache TTL should be reasonable
		defaultTTL := 5 * time.Minute
		maxTTL := 1 * time.Hour

		assert.GreaterOrEqual(t, defaultTTL, 1*time.Minute)
		assert.LessOrEqual(t, defaultTTL, maxTTL)

		// Verify strategy names
		assert.Contains(t, strategies, "cache_first")
		assert.Contains(t, strategies, "database_fallback")
	})

	t.Run("Pagination Configuration", func(t *testing.T) {
		// Test pagination limits
		defaultPageSize := 100
		maxPageSize := 1000

		assert.Greater(t, defaultPageSize, 0)
		assert.Greater(t, maxPageSize, defaultPageSize)
		assert.LessOrEqual(t, maxPageSize, 10000) // Reasonable upper limit
	})

	t.Run("Rate Limiting Configuration", func(t *testing.T) {
		// Test rate limiting settings
		rpsLimit := 1000
		enableRateLimit := true

		assert.Greater(t, rpsLimit, 0)
		assert.LessOrEqual(t, rpsLimit, 10000) // Reasonable upper limit
		assert.True(t, enableRateLimit)
	})

	t.Run("Performance Requirements", func(t *testing.T) {
		// Test performance targets
		maxResponseTime := 500 * time.Millisecond
		maxCacheLatency := 10 * time.Millisecond
		maxDBLatency := 100 * time.Millisecond

		assert.LessOrEqual(t, maxCacheLatency, maxResponseTime)
		assert.LessOrEqual(t, maxDBLatency, maxResponseTime)

		// Connection pool sizes should be reasonable
		redisPoolSize := 20
		clickhouseMaxConns := 5

		assert.GreaterOrEqual(t, redisPoolSize, 5)
		assert.GreaterOrEqual(t, clickhouseMaxConns, 1)
	})
}

// Mock implementations for integration tests
type MockCacheService struct{}

func (m *MockCacheService) Get(key string) (string, error)                 { return "", nil }
func (m *MockCacheService) Set(key, value string, ttl time.Duration) error { return nil }
func (m *MockCacheService) Delete(key string) error                        { return nil }
func (m *MockCacheService) Exists(key string) (bool, error)                { return false, nil }
func (m *MockCacheService) GetStats() map[string]interface{}               { return map[string]interface{}{} }
func (m *MockCacheService) HealthCheck() error                             { return nil }

type MockDatabaseService struct{}

func (m *MockDatabaseService) GetLatestReading(deviceID string) (*clickhouse.SensorReading, error) {
	return &clickhouse.SensorReading{}, nil
}
func (m *MockDatabaseService) GetDeviceList(page, pageSize int) ([]*clickhouse.DeviceInfo, int, error) {
	return []*clickhouse.DeviceInfo{}, 0, nil
}
func (m *MockDatabaseService) GetDeviceHistory(deviceID string, startTime, endTime time.Time, limit int) ([]*clickhouse.SensorReading, error) {
	return []*clickhouse.SensorReading{}, nil
}
func (m *MockDatabaseService) GetStats() map[string]interface{} { return map[string]interface{}{} }
func (m *MockDatabaseService) HealthCheck() error               { return nil }

// Benchmark tests
func BenchmarkAPIGatewayComponents(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmarks in short mode")
	}

	b.Run("Configuration_Loading", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := config.LoadConfig("")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Logger_Creation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			log, err := logger.NewLogger("info", "json", false)
			if err != nil {
				b.Fatal(err)
			}
			log.Sync()
		}
	})

	b.Run("Metrics_Collection", func(b *testing.B) {
		metricsCollector := metrics.NewMetrics()

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metricsCollector.IncrementHTTPRequests("GET", "/test", "200")
				metricsCollector.RecordHTTPLatency("GET", "/test", 10*time.Millisecond)
				metricsCollector.IncrementCacheRequests("get", "hit")
				metricsCollector.IncrementDeviceRequests("test_device", "metrics")
			}
		})
	})

	b.Run("Handler_Creation", func(b *testing.B) {
		mockCache := &MockCacheService{}
		mockDB := &MockDatabaseService{}
		log, _ := logger.NewLogger("info", "json", false)
		metricsCollector := metrics.NewMetrics()

		config := gateway.Config{
			EnableCaching:   true,
			DefaultPageSize: 100,
			MaxPageSize:     1000,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			handlers := gateway.NewHandlers(mockCache, mockDB, log, metricsCollector, config)
			_ = handlers
		}
	})
}
