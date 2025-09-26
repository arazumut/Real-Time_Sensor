package consumer

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/sensor-consumer-service/pkg/logger"
)

// MockMetricsRecorder for testing
type MockMetricsRecorder struct {
	mock.Mock
}

func (m *MockMetricsRecorder) IncrementMessagesConsumed(count int) {
	m.Called(count)
}

func (m *MockMetricsRecorder) IncrementMessagesProcessed(count int) {
	m.Called(count)
}

func (m *MockMetricsRecorder) IncrementMessagesFailed(count int) {
	m.Called(count)
}

func (m *MockMetricsRecorder) SetKafkaLag(lag int64) {
	m.Called(lag)
}

func (m *MockMetricsRecorder) RecordKafkaCommitLatency(duration time.Duration) {
	m.Called(duration)
}

func (m *MockMetricsRecorder) SetKafkaPartitionCount(count int) {
	m.Called(count)
}

func (m *MockMetricsRecorder) SetKafkaOffset(partition string, offset int64) {
	m.Called(partition, offset)
}

func (m *MockMetricsRecorder) SetProcessingRate(rate float64) {
	m.Called(rate)
}

// MockLogger for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields ...zap.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) With(fields ...zap.Field) logger.Logger {
	return m
}

func (m *MockLogger) Sync() error {
	return nil
}

func TestSensorMessage_Parsing(t *testing.T) {
	// Test sensor message JSON parsing
	jsonData := `{
		"device_id": "temp_001",
		"timestamp": "2023-09-26T10:00:00Z",
		"temperature": 25.5,
		"humidity": 60.0,
		"pressure": 1013.25,
		"location": {
			"latitude": 41.0,
			"longitude": 29.0,
			"region": "north"
		},
		"status": "active"
	}`

	var sensorMsg SensorMessage
	err := json.Unmarshal([]byte(jsonData), &sensorMsg)

	require.NoError(t, err)
	assert.Equal(t, "temp_001", sensorMsg.DeviceID)
	assert.Equal(t, 25.5, sensorMsg.Temperature)
	assert.Equal(t, 60.0, sensorMsg.Humidity)
	assert.Equal(t, 1013.25, sensorMsg.Pressure)
	assert.Equal(t, "active", sensorMsg.Status)
	assert.Equal(t, 41.0, sensorMsg.Location.Latitude)
	assert.Equal(t, 29.0, sensorMsg.Location.Longitude)
	assert.Equal(t, "north", sensorMsg.Location.Region)
}

func TestSensorMessage_AlertThreshold(t *testing.T) {
	tests := []struct {
		name        string
		temperature float64
		threshold   float64
		expectAlert bool
	}{
		{
			name:        "Normal temperature",
			temperature: 25.0,
			threshold:   90.0,
			expectAlert: false,
		},
		{
			name:        "High temperature",
			temperature: 95.0,
			threshold:   90.0,
			expectAlert: true,
		},
		{
			name:        "Threshold boundary",
			temperature: 90.0,
			threshold:   90.0,
			expectAlert: false,
		},
		{
			name:        "Very high temperature",
			temperature: 120.0,
			threshold:   90.0,
			expectAlert: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alertRequired := tt.temperature > tt.threshold
			assert.Equal(t, tt.expectAlert, alertRequired)
		})
	}
}

func TestProcessedMessage_Creation(t *testing.T) {
	// Create a mock Kafka message
	kafkaMsg := &sarama.ConsumerMessage{
		Topic:     "sensor-data",
		Partition: 0,
		Offset:    12345,
		Key:       []byte("temp_001"),
		Value: []byte(`{
			"device_id": "temp_001",
			"timestamp": "2023-09-26T10:00:00Z",
			"temperature": 95.0,
			"humidity": 60.0,
			"pressure": 1013.25,
			"location": {
				"latitude": 41.0,
				"longitude": 29.0,
				"region": "north"
			},
			"status": "active"
		}`),
		Headers:   []*sarama.RecordHeader{},
		Timestamp: time.Now(),
	}

	// Parse the message
	var sensorData SensorMessage
	err := json.Unmarshal(kafkaMsg.Value, &sensorData)
	require.NoError(t, err)

	// Create processed message
	processed := &ProcessedMessage{
		Original:   kafkaMsg,
		SensorData: &sensorData,
	}

	// Verify processing
	assert.NotNil(t, processed.Original)
	assert.NotNil(t, processed.SensorData)
	assert.Equal(t, "temp_001", processed.SensorData.DeviceID)
	assert.Equal(t, 95.0, processed.SensorData.Temperature)

	// Check if alert should be triggered
	alertThreshold := 90.0
	processed.AlertRequired = sensorData.Temperature > alertThreshold
	assert.True(t, processed.AlertRequired)
}

func TestConsumerGroupHandler_MessageProcessing(t *testing.T) {
	// This test would normally use a mock consumer group session
	// For now, we'll test the basic handler structure

	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	handler := &consumerGroupHandler{
		logger: mockLogger,
	}

	// Test setup
	err := handler.Setup(nil)
	assert.NoError(t, err)

	// Test cleanup
	err = handler.Cleanup(nil)
	assert.NoError(t, err)
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: Config{
				Brokers:        []string{"localhost:9092"},
				Topic:          "sensor-data",
				GroupID:        "test-group",
				WorkerCount:    5,
				AlertThreshold: 90.0,
			},
			wantErr: false,
		},
		{
			name: "Empty brokers",
			config: Config{
				Brokers:        []string{},
				Topic:          "sensor-data",
				GroupID:        "test-group",
				WorkerCount:    5,
				AlertThreshold: 90.0,
			},
			wantErr: true,
		},
		{
			name: "Empty topic",
			config: Config{
				Brokers:        []string{"localhost:9092"},
				Topic:          "",
				GroupID:        "test-group",
				WorkerCount:    5,
				AlertThreshold: 90.0,
			},
			wantErr: true,
		},
		{
			name: "Invalid worker count",
			config: Config{
				Brokers:        []string{"localhost:9092"},
				Topic:          "sensor-data",
				GroupID:        "test-group",
				WorkerCount:    0,
				AlertThreshold: 90.0,
			},
			wantErr: true,
		},
		{
			name: "Invalid alert threshold",
			config: Config{
				Brokers:        []string{"localhost:9092"},
				Topic:          "sensor-data",
				GroupID:        "test-group",
				WorkerCount:    5,
				AlertThreshold: -10.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConsumerConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function for config validation (would be in config package)
func validateConsumerConfig(config Config) error {
	if len(config.Brokers) == 0 {
		return fmt.Errorf("brokers cannot be empty")
	}
	if config.Topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}
	if config.GroupID == "" {
		return fmt.Errorf("group ID cannot be empty")
	}
	if config.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}
	if config.AlertThreshold <= 0 {
		return fmt.Errorf("alert threshold must be positive")
	}
	return nil
}

// Benchmark tests
func BenchmarkSensorMessage_Unmarshal(b *testing.B) {
	jsonData := []byte(`{
		"device_id": "temp_001",
		"timestamp": "2023-09-26T10:00:00Z",
		"temperature": 25.5,
		"humidity": 60.0,
		"pressure": 1013.25,
		"location": {
			"latitude": 41.0,
			"longitude": 29.0,
			"region": "north"
		},
		"status": "active"
	}`)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var sensorMsg SensorMessage
			json.Unmarshal(jsonData, &sensorMsg)
		}
	})
}

func BenchmarkProcessedMessage_Creation(b *testing.B) {
	sensorData := &SensorMessage{
		DeviceID:    "temp_001",
		Timestamp:   time.Now(),
		Temperature: 25.5,
		Humidity:    60.0,
		Pressure:    1013.25,
		Location: Location{
			Latitude:  41.0,
			Longitude: 29.0,
			Region:    "north",
		},
		Status: "active",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processed := &ProcessedMessage{
			SensorData: sensorData,
		}

		// Simulate processing
		processed.AlertRequired = sensorData.Temperature > 90.0
		_ = processed
	}
}
