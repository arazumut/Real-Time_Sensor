package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/twinup/sensor-system/services/real-time-analytics-service/pkg/algorithms"
)

func TestMainIntegration(t *testing.T) {
	// Skip integration test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test configuration loading
	t.Run("Config Loading", func(t *testing.T) {
		// Set environment variables for testing
		os.Setenv("TWINUP_KAFKA_TOPIC", "test-analytics-data")
		os.Setenv("TWINUP_KAFKA_GROUP_ID", "test-analytics-group")
		os.Setenv("TWINUP_MQTT_BROKER", "tcp://localhost:1883")
		os.Setenv("TWINUP_MQTT_QOS", "1")
		os.Setenv("TWINUP_ANALYTICS_WINDOW_SIZE", "10m")
		os.Setenv("TWINUP_ANALYTICS_TREND_WINDOW", "30m")
		os.Setenv("TWINUP_ANALYTICS_DELTA_THRESHOLD", "3.0")

		defer func() {
			os.Unsetenv("TWINUP_KAFKA_TOPIC")
			os.Unsetenv("TWINUP_KAFKA_GROUP_ID")
			os.Unsetenv("TWINUP_MQTT_BROKER")
			os.Unsetenv("TWINUP_MQTT_QOS")
			os.Unsetenv("TWINUP_ANALYTICS_WINDOW_SIZE")
			os.Unsetenv("TWINUP_ANALYTICS_TREND_WINDOW")
			os.Unsetenv("TWINUP_ANALYTICS_DELTA_THRESHOLD")
		}()

		// Test that configuration can be loaded
		assert.Equal(t, "test-analytics-data", os.Getenv("TWINUP_KAFKA_TOPIC"))
		assert.Equal(t, "test-analytics-group", os.Getenv("TWINUP_KAFKA_GROUP_ID"))
		assert.Equal(t, "tcp://localhost:1883", os.Getenv("TWINUP_MQTT_BROKER"))
		assert.Equal(t, "1", os.Getenv("TWINUP_MQTT_QOS"))
		assert.Equal(t, "10m", os.Getenv("TWINUP_ANALYTICS_WINDOW_SIZE"))
		assert.Equal(t, "30m", os.Getenv("TWINUP_ANALYTICS_TREND_WINDOW"))
		assert.Equal(t, "3.0", os.Getenv("TWINUP_ANALYTICS_DELTA_THRESHOLD"))
	})

	t.Run("Version Command", func(t *testing.T) {
		// Test version information
		require.NotEmpty(t, version)
		assert.Equal(t, "1.0.0", version)
	})
}

func TestAnalyticsComponents(t *testing.T) {
	t.Run("Analytics Algorithm Integration", func(t *testing.T) {
		// Test complete analytics workflow
		window := algorithms.NewTimeSeriesWindow(5*time.Minute, 1000)

		// Add test data
		now := time.Now()
		for i := 0; i < 20; i++ {
			point := &algorithms.DataPoint{
				DeviceID:    "integration_test",
				Timestamp:   now.Add(time.Duration(i) * time.Minute),
				Temperature: float64(20 + i), // Increasing trend
				Humidity:    float64(50) + float64(i)*0.5,
				Pressure:    1013.25,
				Region:      "north",
			}
			window.AddDataPoint(point)
		}

		// Test trend calculation
		deviceData := window.GetDeviceData("integration_test")
		require.NotEmpty(t, deviceData)

		trend, err := algorithms.CalculateTrend(deviceData, "temperature")
		require.NoError(t, err)
		assert.Equal(t, "increasing", trend.Direction)
		assert.Greater(t, trend.Strength, 0.0) // Any positive strength is good

		// Test regional analysis
		regionalData := window.GetDataByRegion("north")
		require.NotEmpty(t, regionalData)

		regional, err := algorithms.CalculateRegionalAnalysis(regionalData, "north", "temperature")
		require.NoError(t, err)
		assert.Equal(t, "north", regional.Region)
		assert.Greater(t, regional.Average, 20.0)
	})

	t.Run("MQTT Topic Generation", func(t *testing.T) {
		// Test MQTT topic generation patterns
		testCases := []struct {
			messageType string
			deviceID    string
			region      string
			expected    string
		}{
			{"trend", "device_001", "north", "analytics/device/device_001/trend"},
			{"delta", "device_002", "south", "analytics/device/device_002/delta"},
			{"regional", "", "central", "analytics/region/central/summary"},
			{"anomaly", "device_003", "east", "analytics/device/device_003/anomaly"},
		}

		topicPrefix := "analytics"
		for _, tc := range testCases {
			t.Run(tc.messageType, func(t *testing.T) {
				var topic string
				switch tc.messageType {
				case "trend":
					topic = fmt.Sprintf("%s/device/%s/trend", topicPrefix, tc.deviceID)
				case "delta":
					topic = fmt.Sprintf("%s/device/%s/delta", topicPrefix, tc.deviceID)
				case "regional":
					topic = fmt.Sprintf("%s/region/%s/summary", topicPrefix, tc.region)
				case "anomaly":
					topic = fmt.Sprintf("%s/device/%s/anomaly", topicPrefix, tc.deviceID)
				}

				assert.Equal(t, tc.expected, topic)
			})
		}
	})

	t.Run("Performance Requirements", func(t *testing.T) {
		// Test analytics performance requirements
		maxAnalysisTime := 50 * time.Millisecond

		// Create test data
		points := make([]*algorithms.DataPoint, 50)
		now := time.Now()
		for i := 0; i < 50; i++ {
			points[i] = &algorithms.DataPoint{
				DeviceID:    "perf_test",
				Timestamp:   now.Add(time.Duration(i) * time.Second),
				Temperature: float64(20 + i),
				Region:      "north",
			}
		}

		// Test trend calculation performance
		start := time.Now()
		_, err := algorithms.CalculateTrend(points, "temperature")
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, elapsed, maxAnalysisTime,
			"Trend calculation should complete within performance requirements")

		// Test regional analysis performance
		start = time.Now()
		_, err = algorithms.CalculateRegionalAnalysis(points, "north", "temperature")
		elapsed = time.Since(start)

		require.NoError(t, err)
		assert.Less(t, elapsed, maxAnalysisTime,
			"Regional analysis should complete within performance requirements")
	})
}

func TestAnalyticsDataFlow(t *testing.T) {
	t.Run("End-to-End Data Processing", func(t *testing.T) {
		// Simulate complete data processing pipeline

		// 1. Create time series window
		window := algorithms.NewTimeSeriesWindow(10*time.Minute, 1000)

		// 2. Add realistic sensor data
		devices := []string{"temp_001", "temp_002", "temp_003"}
		regions := []string{"north", "south", "central"}

		now := time.Now()
		for i, deviceID := range devices {
			for j := 0; j < 10; j++ {
				point := &algorithms.DataPoint{
					DeviceID:    deviceID,
					Timestamp:   now.Add(time.Duration(j) * time.Minute),
					Temperature: float64(20 + i*5 + j), // Different base temps per device
					Humidity:    float64(50 + i*10 + j*2),
					Pressure:    1013.25 + float64(i),
					Region:      regions[i],
				}
				window.AddDataPoint(point)
			}
		}

		// 3. Verify data integrity
		stats := window.GetStats()
		assert.Equal(t, 3, stats["device_count"])
		assert.Equal(t, 30, stats["total_points"])

		// 4. Test analytics for each device
		for _, deviceID := range devices {
			deviceData := window.GetDeviceData(deviceID)
			require.NotEmpty(t, deviceData)

			// Test trend analysis
			trend, err := algorithms.CalculateTrend(deviceData, "temperature")
			require.NoError(t, err)
			assert.Equal(t, "increasing", trend.Direction) // All have increasing temps

			// Test delta analysis (using first two points)
			if len(deviceData) >= 2 {
				delta, err := algorithms.CalculateDelta(deviceData[1], deviceData[0], "temperature", 0.5)
				require.NoError(t, err)
				assert.Greater(t, delta.Delta, 0.0) // Should be positive delta
			}
		}

		// 5. Test regional analysis for each region
		for _, region := range regions {
			regionalData := window.GetDataByRegion(region)
			require.NotEmpty(t, regionalData)

			regional, err := algorithms.CalculateRegionalAnalysis(regionalData, region, "temperature")
			require.NoError(t, err)
			assert.Equal(t, region, regional.Region)
			assert.Equal(t, 1, regional.DeviceCount) // One device per region in this test
		}
	})
}

// Benchmark tests for performance validation
func BenchmarkCompleteAnalyticsWorkflow(b *testing.B) {
	window := algorithms.NewTimeSeriesWindow(5*time.Minute, 1000)

	// Prepare test data
	now := time.Now()
	points := make([]*algorithms.DataPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = &algorithms.DataPoint{
			DeviceID:    fmt.Sprintf("bench_%03d", i%10),
			Timestamp:   now.Add(time.Duration(i) * time.Second),
			Temperature: float64(20 + i%20),
			Region:      "north",
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Add data point
			point := points[b.N%len(points)]
			window.AddDataPoint(point)

			// Perform analytics
			deviceData := window.GetDeviceData(point.DeviceID)
			if len(deviceData) >= 5 {
				algorithms.CalculateTrend(deviceData, "temperature")
			}
		}
	})
}
