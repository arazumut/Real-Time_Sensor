package simulator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDevice(t *testing.T) {
	tests := []struct {
		name       string
		deviceID   string
		deviceType DeviceType
		location   Location
	}{
		{
			name:       "Temperature sensor",
			deviceID:   "temp_001",
			deviceType: TemperatureSensor,
			location:   Location{Latitude: 41.0, Longitude: 29.0, Region: "north"},
		},
		{
			name:       "Weather station",
			deviceID:   "weather_001",
			deviceType: WeatherStation,
			location:   Location{Latitude: 36.0, Longitude: 30.0, Region: "south"},
		},
		{
			name:       "Industrial sensor",
			deviceID:   "industrial_001",
			deviceType: IndustrialSensor,
			location:   Location{Latitude: 38.0, Longitude: 32.0, Region: "central"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := NewDevice(tt.deviceID, tt.deviceType, tt.location)

			require.NotNil(t, device)
			assert.Equal(t, tt.deviceID, device.ID)
			assert.Equal(t, tt.deviceType, device.Type)
			assert.Equal(t, tt.location, device.Location)
			assert.Equal(t, "active", device.Status)
			assert.True(t, device.BaseTemperature > 0)
			assert.True(t, device.BaseHumidity >= 0)
			assert.True(t, device.BasePressure > 0)
			assert.True(t, device.VariationFactor >= 0)
		})
	}
}

func TestDevice_GenerateReading(t *testing.T) {
	device := NewDevice("test_001", TemperatureSensor, Location{
		Latitude:  41.0,
		Longitude: 29.0,
		Region:    "north",
	})

	reading := device.GenerateReading()

	require.NotNil(t, reading)
	assert.Equal(t, "test_001", reading.DeviceID)
	assert.True(t, reading.Timestamp.After(time.Now().Add(-time.Second)))
	assert.True(t, reading.Temperature >= -50 && reading.Temperature <= 60)
	assert.True(t, reading.Humidity >= 0 && reading.Humidity <= 100)
	assert.True(t, reading.Pressure >= 950 && reading.Pressure <= 1050)
	assert.Equal(t, device.Location, reading.Location)
	assert.NotEmpty(t, reading.Status)
}

func TestDevice_GenerateReading_Consistency(t *testing.T) {
	device := NewDevice("test_002", WeatherStation, Location{
		Latitude:  38.0,
		Longitude: 32.0,
		Region:    "central",
	})

	// Generate multiple readings and check consistency
	readings := make([]*SensorReading, 100)
	for i := 0; i < 100; i++ {
		readings[i] = device.GenerateReading()
		time.Sleep(time.Millisecond) // Small delay to ensure different timestamps
	}

	// Check that all readings are from the same device
	for _, reading := range readings {
		assert.Equal(t, "test_002", reading.DeviceID)
		assert.Equal(t, device.Location, reading.Location)
	}

	// Check that readings have realistic variations
	var tempSum, humSum, pressSum float64
	for _, reading := range readings {
		tempSum += reading.Temperature
		humSum += reading.Humidity
		pressSum += reading.Pressure
	}

	avgTemp := tempSum / 100
	avgHum := humSum / 100
	avgPress := pressSum / 100

	// Average should be close to base values (within reasonable range)
	assert.InDelta(t, device.BaseTemperature, avgTemp, 10.0)
	assert.InDelta(t, device.BaseHumidity, avgHum, 15.0)
	assert.InDelta(t, device.BasePressure, avgPress, 10.0)
}

func TestDevice_ErrorSimulation(t *testing.T) {
	device := NewDevice("test_003", TemperatureSensor, Location{
		Latitude:  41.0,
		Longitude: 29.0,
		Region:    "north",
	})

	// Force high error rate for testing
	device.ErrorRate = 0.5 // 50% error rate

	errorCount := 0
	totalReadings := 100

	for i := 0; i < totalReadings; i++ {
		reading := device.GenerateReading()
		if reading.Status == "error" {
			errorCount++
			assert.Equal(t, -999.0, reading.Temperature)
			assert.Equal(t, -999.0, reading.Humidity)
			assert.Equal(t, -999.0, reading.Pressure)
		}
	}

	// Should have some errors with 50% error rate
	assert.True(t, errorCount > 0, "Expected some error readings with high error rate")
}

func TestDevice_ThreadSafety(t *testing.T) {
	device := NewDevice("test_004", WeatherStation, Location{
		Latitude:  38.0,
		Longitude: 32.0,
		Region:    "central",
	})

	// Run concurrent reading generation
	const numGoroutines = 10
	const readingsPerGoroutine = 100

	readings := make(chan *SensorReading, numGoroutines*readingsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < readingsPerGoroutine; j++ {
				reading := device.GenerateReading()
				readings <- reading
			}
		}()
	}

	// Collect all readings
	collectedReadings := make([]*SensorReading, 0, numGoroutines*readingsPerGoroutine)
	timeout := time.After(5 * time.Second)

	for len(collectedReadings) < numGoroutines*readingsPerGoroutine {
		select {
		case reading := <-readings:
			collectedReadings = append(collectedReadings, reading)
		case <-timeout:
			t.Fatal("Timeout waiting for readings")
		}
	}

	// Verify all readings are valid
	assert.Len(t, collectedReadings, numGoroutines*readingsPerGoroutine)
	for _, reading := range collectedReadings {
		assert.Equal(t, "test_004", reading.DeviceID)
		assert.NotZero(t, reading.Timestamp)
	}

	// Verify device stats
	stats := device.GetStats()
	assert.Equal(t, "test_004", stats["device_id"])
	assert.Equal(t, WeatherStation, stats["type"])
	// Reading count might be slightly different due to concurrent access
	readingCount := stats["reading_count"].(int64)
	assert.True(t, readingCount >= int64(numGoroutines*readingsPerGoroutine-10) && readingCount <= int64(numGoroutines*readingsPerGoroutine))
}

func TestGenerateDeviceID(t *testing.T) {
	tests := []struct {
		index      int
		deviceType DeviceType
		expected   string
	}{
		{1, TemperatureSensor, "temperature_sensor_000001"},
		{42, WeatherStation, "weather_station_000042"},
		{9999, IndustrialSensor, "industrial_sensor_009999"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := GenerateDeviceID(tt.index, tt.deviceType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateRandomLocation(t *testing.T) {
	regions := []string{"north", "south", "east", "west", "central"}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			location := GenerateRandomLocation(region)

			assert.Equal(t, region, location.Region)
			assert.True(t, location.Latitude >= 35.0 && location.Latitude <= 45.0)
			assert.True(t, location.Longitude >= 25.0 && location.Longitude <= 40.0)
		})
	}
}

func TestDevice_GetStats(t *testing.T) {
	device := NewDevice("test_005", HumiditySensor, Location{
		Latitude:  40.0,
		Longitude: 30.0,
		Region:    "central",
	})

	// Generate some readings
	for i := 0; i < 5; i++ {
		device.GenerateReading()
		time.Sleep(time.Millisecond)
	}

	stats := device.GetStats()

	assert.Equal(t, "test_005", stats["device_id"])
	assert.Equal(t, HumiditySensor, stats["type"])
	assert.Equal(t, int64(5), stats["reading_count"])
	assert.NotZero(t, stats["last_reading"])
}

func TestDevice_IsHealthy(t *testing.T) {
	device := NewDevice("test_006", PressureSensor, Location{
		Latitude:  39.0,
		Longitude: 35.0,
		Region:    "central",
	})

	// Initially should be healthy
	assert.True(t, device.IsHealthy())

	// Set to maintenance - should still be healthy
	device.SetStatus("maintenance")
	assert.True(t, device.IsHealthy())

	// Set to error - should not be healthy
	device.SetStatus("error")
	assert.False(t, device.IsHealthy())

	// Set back to active - should be healthy
	device.SetStatus("active")
	assert.True(t, device.IsHealthy())
}

// Benchmark tests
func BenchmarkDevice_GenerateReading(b *testing.B) {
	device := NewDevice("bench_001", TemperatureSensor, Location{
		Latitude:  41.0,
		Longitude: 29.0,
		Region:    "north",
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			device.GenerateReading()
		}
	})
}

func BenchmarkGenerateDeviceID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateDeviceID(i, TemperatureSensor)
	}
}

func BenchmarkGenerateRandomLocation(b *testing.B) {
	regions := []string{"north", "south", "east", "west", "central"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		region := regions[i%len(regions)]
		GenerateRandomLocation(region)
	}
}
