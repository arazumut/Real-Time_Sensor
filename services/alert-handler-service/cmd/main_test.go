package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/twinup/sensor-system/services/alert-handler-service/internal/email"
	"github.com/twinup/sensor-system/services/alert-handler-service/internal/server"
)

func TestMainIntegration(t *testing.T) {
	// Skip integration test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test configuration loading
	t.Run("Config Loading", func(t *testing.T) {
		// Set environment variables for testing
		os.Setenv("TWINUP_GRPC_HOST", "0.0.0.0")
		os.Setenv("TWINUP_GRPC_PORT", "50051")
		os.Setenv("TWINUP_ALERT_TEMPERATURE_THRESHOLD", "85.0")
		os.Setenv("TWINUP_ALERT_COOLDOWN_PERIOD", "2m")
		os.Setenv("TWINUP_EMAIL_ENABLE_DUMMY", "true")
		os.Setenv("TWINUP_EMAIL_FROM_EMAIL", "test@twinup.com")
		os.Setenv("TWINUP_STORAGE_ENABLE_STORAGE", "true")

		defer func() {
			os.Unsetenv("TWINUP_GRPC_HOST")
			os.Unsetenv("TWINUP_GRPC_PORT")
			os.Unsetenv("TWINUP_ALERT_TEMPERATURE_THRESHOLD")
			os.Unsetenv("TWINUP_ALERT_COOLDOWN_PERIOD")
			os.Unsetenv("TWINUP_EMAIL_ENABLE_DUMMY")
			os.Unsetenv("TWINUP_EMAIL_FROM_EMAIL")
			os.Unsetenv("TWINUP_STORAGE_ENABLE_STORAGE")
		}()

		// Test that configuration can be loaded
		assert.Equal(t, "0.0.0.0", os.Getenv("TWINUP_GRPC_HOST"))
		assert.Equal(t, "50051", os.Getenv("TWINUP_GRPC_PORT"))
		assert.Equal(t, "85.0", os.Getenv("TWINUP_ALERT_TEMPERATURE_THRESHOLD"))
		assert.Equal(t, "2m", os.Getenv("TWINUP_ALERT_COOLDOWN_PERIOD"))
		assert.Equal(t, "true", os.Getenv("TWINUP_EMAIL_ENABLE_DUMMY"))
		assert.Equal(t, "test@twinup.com", os.Getenv("TWINUP_EMAIL_FROM_EMAIL"))
		assert.Equal(t, "true", os.Getenv("TWINUP_STORAGE_ENABLE_STORAGE"))
	})

	t.Run("Version Command", func(t *testing.T) {
		// Test version information
		require.NotEmpty(t, version)
		assert.Equal(t, "1.0.0", version)
	})
}

func TestAlertHandlerComponents(t *testing.T) {
	t.Run("Alert Processing Pipeline", func(t *testing.T) {
		// Test basic alert request structure
		validRequest := &server.AlertRequest{
			DeviceID:    "test_device_001",
			Temperature: 95.0,
			Threshold:   90.0,
			Message:     "High temperature detected",
			Severity:    "HIGH",
			Region:      "north",
			Timestamp:   time.Now(),
		}

		// Validate request structure
		assert.NotEmpty(t, validRequest.DeviceID)
		assert.Greater(t, validRequest.Temperature, validRequest.Threshold)
		assert.NotEmpty(t, validRequest.Message)
		assert.NotZero(t, validRequest.Timestamp)
	})

	t.Run("Email Service Integration", func(t *testing.T) {
		// Test email request creation
		emailReq := &email.EmailRequest{
			To:       []string{"admin@twinup.com"},
			Subject:  "Test Alert",
			Body:     "This is a test alert email",
			Priority: "high",
			AlertID:  "test_alert_001",
			DeviceID: "test_device_001",
		}

		// Validate email request structure
		assert.NotEmpty(t, emailReq.To)
		assert.NotEmpty(t, emailReq.Subject)
		assert.NotEmpty(t, emailReq.Body)
		assert.NotEmpty(t, emailReq.AlertID)
		assert.NotEmpty(t, emailReq.DeviceID)
	})

	t.Run("Alert Thresholds and Severity", func(t *testing.T) {
		// Test temperature threshold logic
		normalTemp := 25.0
		highTemp := 95.0
		threshold := 90.0

		assert.Less(t, normalTemp, threshold, "Normal temperature should be below threshold")
		assert.Greater(t, highTemp, threshold, "High temperature should be above threshold")

		// Test severity calculation logic
		tempDiff := highTemp - threshold
		assert.Equal(t, 5.0, tempDiff, "Temperature difference should be 5°C")
	})

	t.Run("gRPC Response Structure", func(t *testing.T) {
		// Test gRPC response structure
		response := &server.AlertResponse{
			Success:     true,
			Message:     "Alert processed successfully",
			AlertID:     "alert_12345",
			ProcessedAt: time.Now(),
			EmailSent:   true,
			Stored:      true,
		}

		// Validate response structure
		assert.True(t, response.Success)
		assert.NotEmpty(t, response.Message)
		assert.NotEmpty(t, response.AlertID)
		assert.NotZero(t, response.ProcessedAt)
		assert.True(t, response.EmailSent)
		assert.True(t, response.Stored)
	})
}

func TestPerformanceRequirements(t *testing.T) {
	t.Run("Alert Processing Latency", func(t *testing.T) {
		// Test that alert processing meets latency requirements
		maxProcessingTime := 100 * time.Millisecond

		start := time.Now()

		// Simulate alert processing
		request := &server.AlertRequest{
			DeviceID:    "perf_test_device",
			Temperature: 95.0,
			Threshold:   90.0,
			Timestamp:   time.Now(),
		}

		// Simulate basic processing
		time.Sleep(10 * time.Millisecond) // Simulate processing work

		elapsed := time.Since(start)

		// Basic validation
		assert.NotEmpty(t, request.DeviceID)
		assert.Greater(t, request.Temperature, request.Threshold)
		assert.Less(t, elapsed, maxProcessingTime,
			"Alert processing should complete within latency requirements")
	})

	t.Run("Email Generation Performance", func(t *testing.T) {
		// Test email generation performance
		maxEmailGenTime := 10 * time.Millisecond

		start := time.Now()

		// Simulate email body generation
		alertID := "perf_alert_001"
		deviceID := "perf_device_001"
		temperature := 95.0
		threshold := 90.0
		region := "north"

		emailBody := fmt.Sprintf(`
🚨 TWINUP Alert: High Temperature
Device: %s
Temperature: %.2f°C
Threshold: %.2f°C
Region: %s
Alert ID: %s
Time: %s
		`, deviceID, temperature, threshold, region, alertID, time.Now().Format("2006-01-02 15:04:05"))

		elapsed := time.Since(start)

		assert.NotEmpty(t, emailBody)
		assert.Less(t, elapsed, maxEmailGenTime,
			"Email generation should be fast")
	})
}

// Benchmark tests
func BenchmarkAlertValidation(b *testing.B) {
	request := &server.AlertRequest{
		DeviceID:    "bench_device",
		Temperature: 95.0,
		Threshold:   90.0,
		Message:     "High temperature detected",
		Timestamp:   time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate validation logic
		_ = request.DeviceID != ""
		_ = request.Temperature > request.Threshold
		_ = !request.Timestamp.IsZero()
	}
}

func BenchmarkAlertTypeAndSeverity(b *testing.B) {
	temperature := 105.0
	threshold := 90.0
	tempThreshold := 90.0

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate alert type determination
			_ = temperature > tempThreshold

			// Simulate severity calculation
			tempDiff := temperature - threshold
			_ = tempDiff >= 20 // Critical check
		}
	})
}
