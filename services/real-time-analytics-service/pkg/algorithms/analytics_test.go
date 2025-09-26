package algorithms

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeSeriesWindow_AddDataPoint(t *testing.T) {
	window := NewTimeSeriesWindow(5*time.Minute, 1000)

	// Add test data points
	now := time.Now()
	points := []*DataPoint{
		{
			DeviceID:    "test_001",
			Timestamp:   now,
			Temperature: 25.0,
			Region:      "north",
		},
		{
			DeviceID:    "test_001",
			Timestamp:   now.Add(time.Minute),
			Temperature: 26.0,
			Region:      "north",
		},
		{
			DeviceID:    "test_002",
			Timestamp:   now,
			Temperature: 30.0,
			Region:      "south",
		},
	}

	for _, point := range points {
		window.AddDataPoint(point)
	}

	// Verify data retrieval
	device1Data := window.GetDeviceData("test_001")
	assert.Len(t, device1Data, 2)
	assert.Equal(t, "test_001", device1Data[0].DeviceID)

	device2Data := window.GetDeviceData("test_002")
	assert.Len(t, device2Data, 1)
	assert.Equal(t, "test_002", device2Data[0].DeviceID)

	// Test regional data
	northData := window.GetDataByRegion("north")
	assert.Len(t, northData, 2)

	southData := window.GetDataByRegion("south")
	assert.Len(t, southData, 1)
}

func TestTimeSeriesWindow_WindowExpiry(t *testing.T) {
	window := NewTimeSeriesWindow(1*time.Second, 1000)

	now := time.Now()

	// Add old data point
	oldPoint := &DataPoint{
		DeviceID:    "test_001",
		Timestamp:   now.Add(-2 * time.Second), // Outside window
		Temperature: 25.0,
		Region:      "north",
	}
	window.AddDataPoint(oldPoint)

	// Add current data point
	currentPoint := &DataPoint{
		DeviceID:    "test_001",
		Timestamp:   now,
		Temperature: 26.0,
		Region:      "north",
	}
	window.AddDataPoint(currentPoint)

	// Wait for window expiry simulation
	time.Sleep(10 * time.Millisecond)

	// Add another current point to trigger cleanup
	newPoint := &DataPoint{
		DeviceID:    "test_001",
		Timestamp:   time.Now(),
		Temperature: 27.0,
		Region:      "north",
	}
	window.AddDataPoint(newPoint)

	// Should have only recent points
	deviceData := window.GetDeviceData("test_001")
	assert.GreaterOrEqual(t, len(deviceData), 1)

	// All remaining points should be recent
	for _, point := range deviceData {
		assert.True(t, point.Timestamp.After(now.Add(-2*time.Second)))
	}
}

func TestCalculateTrend(t *testing.T) {
	// Create test data with increasing trend
	now := time.Now()
	points := []*DataPoint{
		{DeviceID: "test_001", Timestamp: now, Temperature: 20.0, Region: "north"},
		{DeviceID: "test_001", Timestamp: now.Add(time.Minute), Temperature: 22.0, Region: "north"},
		{DeviceID: "test_001", Timestamp: now.Add(2 * time.Minute), Temperature: 24.0, Region: "north"},
		{DeviceID: "test_001", Timestamp: now.Add(3 * time.Minute), Temperature: 26.0, Region: "north"},
		{DeviceID: "test_001", Timestamp: now.Add(4 * time.Minute), Temperature: 28.0, Region: "north"},
	}

	trend, err := CalculateTrend(points, "temperature")
	require.NoError(t, err)
	require.NotNil(t, trend)

	assert.Equal(t, "test_001", trend.DeviceID)
	assert.Equal(t, "north", trend.Region)
	assert.Equal(t, "temperature", trend.MetricType)
	assert.Equal(t, "increasing", trend.Direction)
	assert.Greater(t, trend.Strength, 0.0)
	assert.Greater(t, trend.Slope, 0.0)
	assert.Equal(t, 20.0, trend.StartValue)
	assert.Equal(t, 28.0, trend.EndValue)
}

func TestCalculateTrend_StableTrend(t *testing.T) {
	// Create test data with stable trend
	now := time.Now()
	points := []*DataPoint{
		{DeviceID: "test_002", Timestamp: now, Temperature: 25.0, Region: "central"},
		{DeviceID: "test_002", Timestamp: now.Add(time.Minute), Temperature: 25.1, Region: "central"},
		{DeviceID: "test_002", Timestamp: now.Add(2 * time.Minute), Temperature: 24.9, Region: "central"},
		{DeviceID: "test_002", Timestamp: now.Add(3 * time.Minute), Temperature: 25.0, Region: "central"},
	}

	trend, err := CalculateTrend(points, "temperature")
	require.NoError(t, err)
	require.NotNil(t, trend)

	assert.Equal(t, "stable", trend.Direction)
	assert.Less(t, math.Abs(trend.Slope), 0.01)
}

func TestCalculateDelta(t *testing.T) {
	current := &DataPoint{
		DeviceID:    "test_001",
		Timestamp:   time.Now(),
		Temperature: 28.0,
		Region:      "north",
	}

	previous := &DataPoint{
		DeviceID:    "test_001",
		Timestamp:   time.Now().Add(-time.Minute),
		Temperature: 25.0,
		Region:      "north",
	}

	delta, err := CalculateDelta(current, previous, "temperature", 2.0)
	require.NoError(t, err)
	require.NotNil(t, delta)

	assert.Equal(t, "test_001", delta.DeviceID)
	assert.Equal(t, "temperature", delta.MetricType)
	assert.Equal(t, 28.0, delta.CurrentValue)
	assert.Equal(t, 25.0, delta.PreviousValue)
	assert.Equal(t, 3.0, delta.Delta)
	assert.Equal(t, 12.0, delta.DeltaPercent) // (3/25)*100
	assert.True(t, delta.IsSignificant)       // 3.0 > 2.0 threshold
}

func TestCalculateRegionalAnalysis(t *testing.T) {
	// Create test data for regional analysis
	points := []*DataPoint{
		{DeviceID: "dev_001", Temperature: 20.0, Region: "north"},
		{DeviceID: "dev_002", Temperature: 22.0, Region: "north"},
		{DeviceID: "dev_003", Temperature: 24.0, Region: "north"},
		{DeviceID: "dev_004", Temperature: 26.0, Region: "north"},
		{DeviceID: "dev_005", Temperature: 28.0, Region: "north"},
		{DeviceID: "dev_006", Temperature: 35.0, Region: "south"}, // Different region
	}

	regional, err := CalculateRegionalAnalysis(points, "north", "temperature")
	require.NoError(t, err)
	require.NotNil(t, regional)

	assert.Equal(t, "north", regional.Region)
	assert.Equal(t, "temperature", regional.MetricType)
	assert.Equal(t, 5, regional.DeviceCount) // Only north region devices
	assert.Equal(t, 24.0, regional.Average)  // (20+22+24+26+28)/5
	assert.Equal(t, 20.0, regional.Minimum)
	assert.Equal(t, 28.0, regional.Maximum)
	assert.Greater(t, regional.StandardDev, 0.0)
}

func TestDetectAnomalies(t *testing.T) {
	// Create test data with anomalies
	points := []*DataPoint{
		{DeviceID: "test_001", Temperature: 20.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 21.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 22.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 23.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 24.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 25.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 26.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 27.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 28.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 29.0, Region: "north"},
		{DeviceID: "test_001", Temperature: 50.0, Region: "north"}, // Anomaly!
	}

	anomalies, err := DetectAnomalies(points, "temperature")
	require.NoError(t, err)

	// Should detect the 50.0 temperature as an anomaly
	assert.Greater(t, len(anomalies), 0)

	// Check the anomaly
	anomaly := anomalies[0]
	assert.Equal(t, "test_001", anomaly.DeviceID)
	assert.Equal(t, "temperature", anomaly.MetricType)
	assert.Equal(t, 50.0, anomaly.CurrentValue)
	assert.NotEmpty(t, anomaly.Severity)
	assert.NotEmpty(t, anomaly.AnomalyType)
}

func TestLinearRegression(t *testing.T) {
	// Test perfect linear relationship: y = 2x + 1
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{3, 5, 7, 9, 11}

	slope, intercept, rSquared := linearRegression(x, y)

	assert.InDelta(t, 2.0, slope, 0.001)
	assert.InDelta(t, 1.0, intercept, 0.001)
	assert.InDelta(t, 1.0, rSquared, 0.001) // Perfect correlation
}

func TestStatisticalFunctions(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	t.Run("Mean", func(t *testing.T) {
		mean := calculateMean(values)
		assert.Equal(t, 5.5, mean)
	})

	t.Run("Min", func(t *testing.T) {
		min := calculateMin(values)
		assert.Equal(t, 1.0, min)
	})

	t.Run("Max", func(t *testing.T) {
		max := calculateMax(values)
		assert.Equal(t, 10.0, max)
	})

	t.Run("Median", func(t *testing.T) {
		median := calculateMedian(values)
		assert.Equal(t, 5.5, median) // (5+6)/2
	})

	t.Run("Standard Deviation", func(t *testing.T) {
		mean := calculateMean(values)
		stdDev := calculateStandardDeviation(values, mean)
		assert.InDelta(t, 3.027, stdDev, 0.01) // Approximately sqrt(55/6)
	})
}

func TestMovingAverage(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	windowSize := 3

	result := MovingAverage(values, windowSize)
	expected := []float64{2, 3, 4, 5, 6, 7, 8, 9} // 3-point moving averages

	require.Len(t, result, len(expected))
	for i, v := range expected {
		assert.Equal(t, v, result[i])
	}
}

func TestExponentialMovingAverage(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	alpha := 0.5

	result := ExponentialMovingAverage(values, alpha)

	require.Len(t, result, len(values))
	assert.Equal(t, 1.0, result[0])  // First value unchanged
	assert.Equal(t, 1.5, result[1])  // 0.5*2 + 0.5*1
	assert.Equal(t, 2.25, result[2]) // 0.5*3 + 0.5*1.5
}

func TestCalculateCorrelation(t *testing.T) {
	// Perfect positive correlation
	values1 := []float64{1, 2, 3, 4, 5}
	values2 := []float64{2, 4, 6, 8, 10}

	correlation, err := CalculateCorrelation(values1, values2)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, correlation, 0.001)

	// Perfect negative correlation
	values3 := []float64{5, 4, 3, 2, 1}
	correlation2, err := CalculateCorrelation(values1, values3)
	require.NoError(t, err)
	assert.InDelta(t, -1.0, correlation2, 0.001)

	// No correlation
	values4 := []float64{1, 5, 2, 4, 3}
	correlation3, err := CalculateCorrelation(values1, values4)
	require.NoError(t, err)
	assert.Less(t, math.Abs(correlation3), 0.5) // Should be close to 0
}

func TestTimeSeriesWindow_MemoryManagement(t *testing.T) {
	// Create window with small limits for testing
	window := NewTimeSeriesWindow(1*time.Hour, 10) // Max 10 points

	now := time.Now()

	// Add more points than the limit
	for i := 0; i < 15; i++ {
		point := &DataPoint{
			DeviceID:    "test_001",
			Timestamp:   now.Add(time.Duration(i) * time.Minute),
			Temperature: float64(20 + i),
			Region:      "north",
		}
		window.AddDataPoint(point)
	}

	// Should have only the most recent 10 points
	deviceData := window.GetDeviceData("test_001")
	assert.LessOrEqual(t, len(deviceData), 10)

	// Last point should be the most recent
	if len(deviceData) > 0 {
		lastPoint := deviceData[len(deviceData)-1]
		assert.Equal(t, 34.0, lastPoint.Temperature) // 20 + 14
	}
}

func TestTimeSeriesWindow_MultipleDevices(t *testing.T) {
	window := NewTimeSeriesWindow(5*time.Minute, 1000)

	now := time.Now()

	// Add data for multiple devices
	devices := []string{"dev_001", "dev_002", "dev_003"}
	for _, deviceID := range devices {
		for i := 0; i < 5; i++ {
			point := &DataPoint{
				DeviceID:    deviceID,
				Timestamp:   now.Add(time.Duration(i) * time.Minute),
				Temperature: float64(20 + i),
				Region:      "north",
			}
			window.AddDataPoint(point)
		}
	}

	// Check all devices are present
	allDevices := window.GetAllDevices()
	assert.Len(t, allDevices, 3)

	for _, deviceID := range devices {
		assert.Contains(t, allDevices, deviceID)

		deviceData := window.GetDeviceData(deviceID)
		assert.Len(t, deviceData, 5)
	}

	// Check stats
	stats := window.GetStats()
	assert.Equal(t, 3, stats["device_count"])
	assert.Equal(t, 15, stats["total_points"]) // 3 devices * 5 points each
}

func TestCalculateTrend_InsufficientData(t *testing.T) {
	// Test with insufficient data points
	points := []*DataPoint{
		{DeviceID: "test_001", Temperature: 25.0, Region: "north"},
	}

	trend, err := CalculateTrend(points, "temperature")
	assert.Error(t, err)
	assert.Nil(t, trend)
	assert.Contains(t, err.Error(), "insufficient data points")
}

func TestCalculateDelta_InvalidInput(t *testing.T) {
	current := &DataPoint{DeviceID: "test_001", Temperature: 25.0}

	// Test with nil previous
	delta, err := CalculateDelta(current, nil, "temperature", 1.0)
	assert.Error(t, err)
	assert.Nil(t, delta)

	// Test with nil current
	previous := &DataPoint{DeviceID: "test_001", Temperature: 20.0}
	delta, err = CalculateDelta(nil, previous, "temperature", 1.0)
	assert.Error(t, err)
	assert.Nil(t, delta)
}

func TestCalculateRegionalAnalysis_EmptyData(t *testing.T) {
	// Test with empty data
	points := []*DataPoint{}

	regional, err := CalculateRegionalAnalysis(points, "north", "temperature")
	assert.Error(t, err)
	assert.Nil(t, regional)
	assert.Contains(t, err.Error(), "no data points")
}

func TestDetectAnomalies_InsufficientData(t *testing.T) {
	// Test with insufficient data points
	points := []*DataPoint{
		{DeviceID: "test_001", Temperature: 25.0},
		{DeviceID: "test_001", Temperature: 26.0},
	}

	anomalies, err := DetectAnomalies(points, "temperature")
	assert.Error(t, err)
	assert.Nil(t, anomalies)
	assert.Contains(t, err.Error(), "insufficient data points")
}

// Benchmark tests
func BenchmarkTimeSeriesWindow_AddDataPoint(b *testing.B) {
	window := NewTimeSeriesWindow(5*time.Minute, 10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		deviceID := 0
		for pb.Next() {
			point := &DataPoint{
				DeviceID:    fmt.Sprintf("bench_%d", deviceID%1000),
				Timestamp:   time.Now(),
				Temperature: 25.0,
				Region:      "north",
			}
			window.AddDataPoint(point)
			deviceID++
		}
	})
}

func BenchmarkCalculateTrend(b *testing.B) {
	// Prepare test data
	now := time.Now()
	points := make([]*DataPoint, 100)
	for i := 0; i < 100; i++ {
		points[i] = &DataPoint{
			DeviceID:    "bench_001",
			Timestamp:   now.Add(time.Duration(i) * time.Second),
			Temperature: float64(20) + float64(i)*0.1,
			Region:      "north",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateTrend(points, "temperature")
	}
}

func BenchmarkCalculateDelta(b *testing.B) {
	current := &DataPoint{
		DeviceID:    "bench_001",
		Temperature: 28.0,
		Region:      "north",
	}

	previous := &DataPoint{
		DeviceID:    "bench_001",
		Temperature: 25.0,
		Region:      "north",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateDelta(current, previous, "temperature", 2.0)
	}
}

func BenchmarkDetectAnomalies(b *testing.B) {
	// Prepare test data
	points := make([]*DataPoint, 100)
	for i := 0; i < 100; i++ {
		temp := 25.0
		if i == 50 { // Add an anomaly
			temp = 100.0
		}
		points[i] = &DataPoint{
			DeviceID:    "bench_001",
			Temperature: temp,
			Region:      "north",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectAnomalies(points, "temperature")
	}
}
