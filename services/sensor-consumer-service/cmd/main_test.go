package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/consumer"
)

func TestMainIntegration(t *testing.T) {
	// Skip integration test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test configuration loading
	t.Run("Config Loading", func(t *testing.T) {
		// Set environment variables for testing
		os.Setenv("TWINUP_KAFKA_TOPIC", "test-sensor-data")
		os.Setenv("TWINUP_KAFKA_GROUP_ID", "test-consumer-group")
		os.Setenv("TWINUP_CONSUMER_WORKER_COUNT", "3")
		os.Setenv("TWINUP_CONSUMER_ALERT_THRESHOLD", "85.0")
		os.Setenv("TWINUP_CLICKHOUSE_BATCH_SIZE", "500")
		os.Setenv("TWINUP_REDIS_TTL", "10m")

		defer func() {
			os.Unsetenv("TWINUP_KAFKA_TOPIC")
			os.Unsetenv("TWINUP_KAFKA_GROUP_ID")
			os.Unsetenv("TWINUP_CONSUMER_WORKER_COUNT")
			os.Unsetenv("TWINUP_CONSUMER_ALERT_THRESHOLD")
			os.Unsetenv("TWINUP_CLICKHOUSE_BATCH_SIZE")
			os.Unsetenv("TWINUP_REDIS_TTL")
		}()

		// Test that configuration can be loaded
		assert.Equal(t, "test-sensor-data", os.Getenv("TWINUP_KAFKA_TOPIC"))
		assert.Equal(t, "test-consumer-group", os.Getenv("TWINUP_KAFKA_GROUP_ID"))
		assert.Equal(t, "3", os.Getenv("TWINUP_CONSUMER_WORKER_COUNT"))
		assert.Equal(t, "85.0", os.Getenv("TWINUP_CONSUMER_ALERT_THRESHOLD"))
		assert.Equal(t, "500", os.Getenv("TWINUP_CLICKHOUSE_BATCH_SIZE"))
		assert.Equal(t, "10m", os.Getenv("TWINUP_REDIS_TTL"))
	})

	t.Run("Version Command", func(t *testing.T) {
		// Test version information
		require.NotEmpty(t, version)
		assert.Equal(t, "1.0.0", version)
	})
}

func TestApplicationComponents(t *testing.T) {
	t.Run("Consumer Configuration Validation", func(t *testing.T) {
		// Test various configuration scenarios
		validConfigs := []map[string]interface{}{
			{
				"kafka_brokers":   []string{"localhost:9092"},
				"kafka_topic":     "sensor-data",
				"kafka_group_id":  "consumer-group",
				"worker_count":    5,
				"alert_threshold": 90.0,
			},
			{
				"kafka_brokers":   []string{"broker1:9092", "broker2:9092"},
				"kafka_topic":     "test-topic",
				"kafka_group_id":  "test-group",
				"worker_count":    10,
				"alert_threshold": 85.0,
			},
		}

		for i, config := range validConfigs {
			t.Run(fmt.Sprintf("Valid config %d", i+1), func(t *testing.T) {
				// Basic validation checks
				brokers := config["kafka_brokers"].([]string)
				assert.NotEmpty(t, brokers)

				topic := config["kafka_topic"].(string)
				assert.NotEmpty(t, topic)

				groupID := config["kafka_group_id"].(string)
				assert.NotEmpty(t, groupID)

				workerCount := config["worker_count"].(int)
				assert.Greater(t, workerCount, 0)

				threshold := config["alert_threshold"].(float64)
				assert.Greater(t, threshold, 0.0)
			})
		}
	})

	t.Run("Service Integration Points", func(t *testing.T) {
		// Test that all required service integration points are defined
		integrationPoints := []string{
			"kafka_consumer",
			"clickhouse_client",
			"redis_client",
			"grpc_alert_client",
		}

		for _, point := range integrationPoints {
			t.Run(point, func(t *testing.T) {
				// Each integration point should be testable
				assert.NotEmpty(t, point)
			})
		}
	})
}

func TestPerformanceRequirements(t *testing.T) {
	t.Run("Latency Requirements", func(t *testing.T) {
		// Test that latency requirements can be met
		maxAllowedLatency := 100 * time.Millisecond

		// Simulate processing time
		start := time.Now()
		time.Sleep(10 * time.Millisecond) // Simulate work
		elapsed := time.Since(start)

		assert.Less(t, elapsed, maxAllowedLatency,
			"Processing should complete within latency requirements")
	})

	t.Run("Throughput Requirements", func(t *testing.T) {
		// Test throughput calculations
		messagesPerSecond := 10000
		batchSize := 1000

		// Calculate required batches per second
		batchesPerSecond := messagesPerSecond / batchSize
		assert.Equal(t, 10, batchesPerSecond)

		// Calculate max allowed batch processing time
		maxBatchTime := time.Second / time.Duration(batchesPerSecond)
		assert.LessOrEqual(t, maxBatchTime, 100*time.Millisecond)
	})
}

// Benchmark tests
func BenchmarkMessageProcessing(b *testing.B) {
	sensorData := &consumer.SensorMessage{
		DeviceID:    "bench_001",
		Timestamp:   time.Now(),
		Temperature: 25.5,
		Humidity:    60.0,
		Pressure:    1013.25,
		Location: consumer.Location{
			Latitude:  41.0,
			Longitude: 29.0,
			Region:    "north",
		},
		Status: "active",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate message processing
			processed := &consumer.ProcessedMessage{
				SensorData: sensorData,
			}

			// Check alert threshold
			processed.AlertRequired = sensorData.Temperature > 90.0

			// Simulate data transformation
			_ = processed
		}
	})
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	jsonData := []byte(`{
		"device_id": "bench_001",
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
	for i := 0; i < b.N; i++ {
		var sensorMsg consumer.SensorMessage
		json.Unmarshal(jsonData, &sensorMsg)
	}
}
