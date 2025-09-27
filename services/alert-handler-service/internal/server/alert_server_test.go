package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/alert-handler-service/pkg/logger"
)

// MockMetricsRecorder for testing
type MockMetricsRecorder struct {
	mock.Mock
}

func (m *MockMetricsRecorder) IncrementAlertsReceived()                  { m.Called() }
func (m *MockMetricsRecorder) IncrementAlertsProcessed()                 { m.Called() }
func (m *MockMetricsRecorder) IncrementAlertsFailed()                    { m.Called() }
func (m *MockMetricsRecorder) IncrementAlertsFiltered()                  { m.Called() }
func (m *MockMetricsRecorder) IncrementAlertsByType(alertType string)    { m.Called(alertType) }
func (m *MockMetricsRecorder) IncrementAlertsBySeverity(severity string) { m.Called(severity) }
func (m *MockMetricsRecorder) IncrementAlertsByDevice(deviceID string)   { m.Called(deviceID) }
func (m *MockMetricsRecorder) IncrementAlertsByRegion(region string)     { m.Called(region) }
func (m *MockMetricsRecorder) RecordAlertProcessingLatency(duration time.Duration) {
	m.Called(duration)
}
func (m *MockMetricsRecorder) SetAlertQueueSize(size int)                     { m.Called(size) }
func (m *MockMetricsRecorder) RecordAlertQueueLatency(duration time.Duration) { m.Called(duration) }
func (m *MockMetricsRecorder) IncrementRateLimitHits()                        { m.Called() }
func (m *MockMetricsRecorder) IncrementRateLimitBypass()                      { m.Called() }
func (m *MockMetricsRecorder) SetCooldownActive(count int)                    { m.Called(count) }
func (m *MockMetricsRecorder) IncrementDuplicateAlerts()                      { m.Called() }
func (m *MockMetricsRecorder) SetDeduplicationHitRate(rate float64)           { m.Called(rate) }
func (m *MockMetricsRecorder) IncrementGRPCRequests(method, status string)    { m.Called(method, status) }
func (m *MockMetricsRecorder) RecordGRPCLatency(method string, duration time.Duration) {
	m.Called(method, duration)
}
func (m *MockMetricsRecorder) IncrementGRPCErrors(method, errorType string) {
	m.Called(method, errorType)
}
func (m *MockMetricsRecorder) SetProcessingRate(rate float64) { m.Called(rate) }

// MockLogger for testing
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

func TestAlertRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request *AlertRequest
		wantErr bool
	}{
		{
			name: "Valid request",
			request: &AlertRequest{
				DeviceID:    "temp_001",
				Temperature: 95.0,
				Threshold:   90.0,
				Message:     "High temperature detected",
				Severity:    "HIGH",
				Timestamp:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "Empty device ID",
			request: &AlertRequest{
				DeviceID:    "",
				Temperature: 95.0,
				Threshold:   90.0,
			},
			wantErr: true,
		},
		{
			name: "Invalid temperature",
			request: &AlertRequest{
				DeviceID:    "temp_001",
				Temperature: 200.0, // Too high
				Threshold:   90.0,
			},
			wantErr: true,
		},
		{
			name: "Invalid threshold",
			request: &AlertRequest{
				DeviceID:    "temp_001",
				Temperature: 95.0,
				Threshold:   -10.0, // Negative
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{}
			err := server.validateAlertRequest(tt.request)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDetermineAlertType(t *testing.T) {
	config := Config{
		TemperatureThreshold: 90.0,
		HumidityThreshold:    95.0,
		PressureThreshold:    1050.0,
	}

	server := &Server{config: config}

	tests := []struct {
		name     string
		request  *AlertRequest
		expected string
	}{
		{
			name: "Temperature high",
			request: &AlertRequest{
				Temperature: 95.0,
				Humidity:    60.0,
				Pressure:    1013.25,
			},
			expected: "TEMPERATURE_HIGH",
		},
		{
			name: "Humidity high",
			request: &AlertRequest{
				Temperature: 25.0,
				Humidity:    98.0,
				Pressure:    1013.25,
			},
			expected: "HUMIDITY_HIGH",
		},
		{
			name: "Pressure high",
			request: &AlertRequest{
				Temperature: 25.0,
				Humidity:    60.0,
				Pressure:    1055.0,
			},
			expected: "PRESSURE_HIGH",
		},
		{
			name: "Normal values",
			request: &AlertRequest{
				Temperature: 25.0,
				Humidity:    60.0,
				Pressure:    1013.25,
			},
			expected: "GENERAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.determineAlertType(tt.request)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineSeverity(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name        string
		temperature float64
		threshold   float64
		expected    string
	}{
		{
			name:        "Critical severity",
			temperature: 115.0,
			threshold:   90.0,
			expected:    "CRITICAL", // diff = 25
		},
		{
			name:        "High severity",
			temperature: 105.0,
			threshold:   90.0,
			expected:    "HIGH", // diff = 15
		},
		{
			name:        "Medium severity",
			temperature: 97.0,
			threshold:   90.0,
			expected:    "MEDIUM", // diff = 7
		},
		{
			name:        "Low severity",
			temperature: 92.0,
			threshold:   90.0,
			expected:    "LOW", // diff = 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &AlertRequest{
				Temperature: tt.temperature,
				Threshold:   tt.threshold,
			}

			result := server.determineSeverity(request)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldSendEmail(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name      string
		alertType string
		severity  string
		expected  bool
	}{
		{
			name:      "Critical alert",
			alertType: "TEMPERATURE_HIGH",
			severity:  "CRITICAL",
			expected:  true,
		},
		{
			name:      "High temperature alert",
			alertType: "TEMPERATURE_HIGH",
			severity:  "HIGH",
			expected:  true,
		},
		{
			name:      "Medium temperature alert",
			alertType: "TEMPERATURE_HIGH",
			severity:  "MEDIUM",
			expected:  true,
		},
		{
			name:      "Low temperature alert",
			alertType: "TEMPERATURE_HIGH",
			severity:  "LOW",
			expected:  false,
		},
		{
			name:      "Non-temperature alert",
			alertType: "HUMIDITY_HIGH",
			severity:  "HIGH",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &AlertRequest{}
			result := server.shouldSendEmail(request, tt.alertType, tt.severity)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRateLimiting(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockMetrics.On("SetCooldownActive", mock.AnythingOfType("int")).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		CooldownPeriod: 1 * time.Minute,
	}

	server := NewServer(config, mockLogger, mockMetrics, nil, nil)

	deviceID := "test_device"

	// Initially not rate limited
	assert.False(t, server.isRateLimited(deviceID))

	// Update cooldown
	server.updateCooldown(deviceID)

	// Should now be rate limited
	assert.True(t, server.isRateLimited(deviceID))

	// Test with past cooldown time (manually set for testing)
	server.rateLimitMutex.Lock()
	server.deviceCooldowns[deviceID] = time.Now().Add(-2 * time.Minute)
	server.rateLimitMutex.Unlock()
	assert.False(t, server.isRateLimited(deviceID))
}

func TestDeduplication(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		DeduplicationWindow: 5 * time.Minute,
	}

	server := NewServer(config, mockLogger, mockMetrics, nil, nil)

	request := &AlertRequest{
		DeviceID:    "test_device",
		Temperature: 95.0,
		Threshold:   90.0,
	}

	// Initially not a duplicate
	assert.False(t, server.isDuplicate(request))

	// Update deduplication cache
	server.updateDeduplicationCache(request)

	// Should now be a duplicate
	assert.True(t, server.isDuplicate(request))

	// Test with different temperature (should not be duplicate)
	request2 := &AlertRequest{
		DeviceID:    "test_device",
		Temperature: 96.0, // Different temperature
		Threshold:   90.0,
	}
	assert.False(t, server.isDuplicate(request2))
}

func TestSendAlert_Success(t *testing.T) {
	// Setup mocks
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations
	mockMetrics.On("IncrementAlertsReceived").Return()
	mockMetrics.On("IncrementGRPCRequests", "SendAlert", "received").Return()
	mockMetrics.On("IncrementGRPCRequests", "SendAlert", "success").Return()
	mockMetrics.On("RecordGRPCLatency", "SendAlert", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("SetAlertQueueSize", mock.AnythingOfType("int")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		TemperatureThreshold: 90.0,
		QueueSize:            10,
		EnableRateLimiting:   false,
		EnableDeduplication:  false,
	}

	server := NewServer(config, mockLogger, mockMetrics, nil, nil)

	// Create valid alert request
	request := &AlertRequest{
		DeviceID:    "temp_001",
		Temperature: 95.0,
		Threshold:   90.0,
		Message:     "High temperature detected",
		Severity:    "HIGH",
		Timestamp:   time.Now(),
	}

	// Test SendAlert
	response, err := server.SendAlert(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.AlertID)
	assert.Contains(t, response.Message, "queued")

	// Verify mock expectations
	mockMetrics.AssertExpectations(t)
}

func TestSendAlert_RateLimited(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations
	mockMetrics.On("IncrementAlertsReceived").Return()
	mockMetrics.On("IncrementGRPCRequests", "SendAlert", "received").Return()
	mockMetrics.On("IncrementAlertsFiltered").Return()
	mockMetrics.On("IncrementRateLimitHits").Return()
	mockMetrics.On("SetCooldownActive", mock.AnythingOfType("int")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything, mock.Anything).Return()

	config := Config{
		TemperatureThreshold: 90.0,
		CooldownPeriod:       1 * time.Minute,
		EnableRateLimiting:   true,
		EnableDeduplication:  false,
	}

	server := NewServer(config, mockLogger, mockMetrics, nil, nil)

	request := &AlertRequest{
		DeviceID:    "temp_001",
		Temperature: 95.0,
		Threshold:   90.0,
		Timestamp:   time.Now(),
	}

	// Set device in cooldown
	server.updateCooldown("temp_001")

	// Test SendAlert - should be rate limited
	response, err := server.SendAlert(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "rate limited")
}

func TestSendAlert_Duplicate(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations
	mockMetrics.On("IncrementAlertsReceived").Return()
	mockMetrics.On("IncrementGRPCRequests", "SendAlert", "received").Return()
	mockMetrics.On("IncrementAlertsFiltered").Return()
	mockMetrics.On("IncrementDuplicateAlerts").Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		TemperatureThreshold: 90.0,
		EnableRateLimiting:   false,
		EnableDeduplication:  true,
		DeduplicationWindow:  5 * time.Minute,
	}

	server := NewServer(config, mockLogger, mockMetrics, nil, nil)

	request := &AlertRequest{
		DeviceID:    "temp_001",
		Temperature: 95.0,
		Threshold:   90.0,
		Timestamp:   time.Now(),
	}

	// Add to deduplication cache
	server.updateDeduplicationCache(request)

	// Test SendAlert - should be filtered as duplicate
	response, err := server.SendAlert(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "Duplicate")
}

func TestHealthCheck(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	server := NewServer(Config{}, mockLogger, mockMetrics, nil, nil)
	server.running = true

	response, err := server.HealthCheck(context.Background(), &HealthCheckRequest{})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.NotZero(t, response.Timestamp)
	// Note: Health check might fail due to nil emailService, but that's expected in unit test
}

func TestAlertTypeAndSeverityDetermination(t *testing.T) {
	config := Config{
		TemperatureThreshold: 90.0,
		HumidityThreshold:    95.0,
		PressureThreshold:    1050.0,
	}

	server := &Server{config: config}

	t.Run("Multiple alert conditions", func(t *testing.T) {
		// Request with multiple threshold breaches
		request := &AlertRequest{
			Temperature: 100.0, // Above temp threshold
			Humidity:    98.0,  // Above humidity threshold
			Pressure:    1013.25,
			Threshold:   90.0,
		}

		alertType := server.determineAlertType(request)
		severity := server.determineSeverity(request)

		// Should prioritize temperature
		assert.Equal(t, "TEMPERATURE_HIGH", alertType)
		assert.Equal(t, "HIGH", severity) // 100-90 = 10°C difference
	})

	t.Run("Edge case thresholds", func(t *testing.T) {
		request := &AlertRequest{
			Temperature: 90.0, // Exactly at threshold
			Threshold:   90.0,
		}

		alertType := server.determineAlertType(request)
		assert.Equal(t, "GENERAL", alertType) // Not above threshold
	})
}

func TestConcurrentAlertProcessing(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations for concurrent calls
	mockMetrics.On("IncrementAlertsReceived").Return()
	mockMetrics.On("IncrementGRPCRequests", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordGRPCLatency", mock.Anything, mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("SetAlertQueueSize", mock.AnythingOfType("int")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		TemperatureThreshold: 90.0,
		QueueSize:            100,
		EnableRateLimiting:   false,
		EnableDeduplication:  false,
	}

	server := NewServer(config, mockLogger, mockMetrics, nil, nil)

	// Test concurrent alert requests
	const numRequests = 50
	responses := make(chan *AlertResponse, numRequests)
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			request := &AlertRequest{
				DeviceID:    fmt.Sprintf("device_%03d", index),
				Temperature: 95.0,
				Threshold:   90.0,
				Timestamp:   time.Now(),
			}

			response, err := server.SendAlert(context.Background(), request)
			responses <- response
			errors <- err
		}(i)
	}

	// Collect all responses
	successCount := 0
	for i := 0; i < numRequests; i++ {
		err := <-errors
		response := <-responses

		assert.NoError(t, err)
		if response != nil && response.Success {
			successCount++
		}
	}

	// All requests should succeed
	assert.Equal(t, numRequests, successCount)
}

// Benchmark tests
func BenchmarkSendAlert(b *testing.B) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mocks to avoid overhead
	mockMetrics.On("IncrementAlertsReceived").Return()
	mockMetrics.On("IncrementGRPCRequests", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordGRPCLatency", mock.Anything, mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("SetAlertQueueSize", mock.AnythingOfType("int")).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		TemperatureThreshold: 90.0,
		QueueSize:            10000,
		EnableRateLimiting:   false,
		EnableDeduplication:  false,
	}

	server := NewServer(config, mockLogger, mockMetrics, nil, nil)

	request := &AlertRequest{
		DeviceID:    "bench_device",
		Temperature: 95.0,
		Threshold:   90.0,
		Timestamp:   time.Now(),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			server.SendAlert(context.Background(), request)
		}
	})
}

func BenchmarkDetermineAlertType(b *testing.B) {
	config := Config{
		TemperatureThreshold: 90.0,
		HumidityThreshold:    95.0,
		PressureThreshold:    1050.0,
	}

	server := &Server{config: config}

	request := &AlertRequest{
		Temperature: 95.0,
		Humidity:    60.0,
		Pressure:    1013.25,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.determineAlertType(request)
	}
}

func BenchmarkDetermineSeverity(b *testing.B) {
	server := &Server{}

	request := &AlertRequest{
		Temperature: 105.0,
		Threshold:   90.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.determineSeverity(request)
	}
}
