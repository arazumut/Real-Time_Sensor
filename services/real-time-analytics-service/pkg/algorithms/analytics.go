package algorithms

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// DataPoint represents a single sensor data point for analytics
type DataPoint struct {
	DeviceID    string    `json:"device_id"`
	Timestamp   time.Time `json:"timestamp"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Pressure    float64   `json:"pressure"`
	LocationLat float64   `json:"location_lat"`
	LocationLng float64   `json:"location_lng"`
	Region      string    `json:"region"`
	Status      string    `json:"status"`
}

// TrendResult represents trend analysis results
type TrendResult struct {
	DeviceID     string    `json:"device_id"`
	Region       string    `json:"region"`
	MetricType   string    `json:"metric_type"`
	Direction    string    `json:"direction"` // "increasing", "decreasing", "stable"
	Strength     float64   `json:"strength"`  // 0.0 to 1.0
	Slope        float64   `json:"slope"`     // Rate of change
	RSquared     float64   `json:"r_squared"` // Correlation coefficient
	StartValue   float64   `json:"start_value"`
	EndValue     float64   `json:"end_value"`
	CalculatedAt time.Time `json:"calculated_at"`
}

// DeltaResult represents delta analysis results
type DeltaResult struct {
	DeviceID      string    `json:"device_id"`
	Region        string    `json:"region"`
	MetricType    string    `json:"metric_type"`
	CurrentValue  float64   `json:"current_value"`
	PreviousValue float64   `json:"previous_value"`
	Delta         float64   `json:"delta"`
	DeltaPercent  float64   `json:"delta_percent"`
	IsSignificant bool      `json:"is_significant"`
	CalculatedAt  time.Time `json:"calculated_at"`
}

// RegionalResult represents regional analysis results
type RegionalResult struct {
	Region       string    `json:"region"`
	MetricType   string    `json:"metric_type"`
	DeviceCount  int       `json:"device_count"`
	Average      float64   `json:"average"`
	Minimum      float64   `json:"minimum"`
	Maximum      float64   `json:"maximum"`
	StandardDev  float64   `json:"standard_deviation"`
	Median       float64   `json:"median"`
	CalculatedAt time.Time `json:"calculated_at"`
}

// AnomalyResult represents anomaly detection results
type AnomalyResult struct {
	DeviceID      string    `json:"device_id"`
	Region        string    `json:"region"`
	MetricType    string    `json:"metric_type"`
	CurrentValue  float64   `json:"current_value"`
	ExpectedValue float64   `json:"expected_value"`
	Deviation     float64   `json:"deviation"`
	Severity      string    `json:"severity"`     // "low", "medium", "high", "critical"
	AnomalyType   string    `json:"anomaly_type"` // "spike", "drop", "drift"
	DetectedAt    time.Time `json:"detected_at"`
}

// TimeSeriesWindow manages a sliding window of time-series data
type TimeSeriesWindow struct {
	data       map[string][]*DataPoint // deviceID -> data points
	windowSize time.Duration
	mutex      sync.RWMutex
	maxPoints  int
}

// NewTimeSeriesWindow creates a new time series window
func NewTimeSeriesWindow(windowSize time.Duration, maxPoints int) *TimeSeriesWindow {
	return &TimeSeriesWindow{
		data:       make(map[string][]*DataPoint),
		windowSize: windowSize,
		maxPoints:  maxPoints,
	}
}

// AddDataPoint adds a new data point to the window
func (w *TimeSeriesWindow) AddDataPoint(point *DataPoint) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	deviceID := point.DeviceID

	// Initialize device data if not exists
	if _, exists := w.data[deviceID]; !exists {
		w.data[deviceID] = make([]*DataPoint, 0, w.maxPoints)
	}

	// Add new point
	w.data[deviceID] = append(w.data[deviceID], point)

	// Remove old points outside window
	cutoffTime := time.Now().Add(-w.windowSize)
	w.data[deviceID] = w.removeOldPoints(w.data[deviceID], cutoffTime)

	// Limit memory usage
	if len(w.data[deviceID]) > w.maxPoints {
		// Keep only the most recent points
		start := len(w.data[deviceID]) - w.maxPoints
		w.data[deviceID] = w.data[deviceID][start:]
	}
}

// removeOldPoints removes data points older than cutoff time
func (w *TimeSeriesWindow) removeOldPoints(points []*DataPoint, cutoffTime time.Time) []*DataPoint {
	var result []*DataPoint
	for _, point := range points {
		if point.Timestamp.After(cutoffTime) {
			result = append(result, point)
		}
	}
	return result
}

// GetDeviceData returns data points for a specific device
func (w *TimeSeriesWindow) GetDeviceData(deviceID string) []*DataPoint {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if points, exists := w.data[deviceID]; exists {
		// Return a copy to avoid race conditions
		result := make([]*DataPoint, len(points))
		copy(result, points)
		return result
	}

	return nil
}

// GetAllDevices returns all device IDs in the window
func (w *TimeSeriesWindow) GetAllDevices() []string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	devices := make([]string, 0, len(w.data))
	for deviceID := range w.data {
		if len(w.data[deviceID]) > 0 {
			devices = append(devices, deviceID)
		}
	}

	return devices
}

// GetDataByRegion returns all data points for a specific region
func (w *TimeSeriesWindow) GetDataByRegion(region string) []*DataPoint {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	var result []*DataPoint
	for _, points := range w.data {
		for _, point := range points {
			if point.Region == region {
				result = append(result, point)
			}
		}
	}

	return result
}

// GetStats returns window statistics
func (w *TimeSeriesWindow) GetStats() map[string]interface{} {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	totalPoints := 0
	deviceCount := 0

	for _, points := range w.data {
		if len(points) > 0 {
			deviceCount++
			totalPoints += len(points)
		}
	}

	return map[string]interface{}{
		"device_count":   deviceCount,
		"total_points":   totalPoints,
		"window_minutes": w.windowSize.Minutes(),
		"max_points":     w.maxPoints,
	}
}

// CalculateTrend calculates trend for a specific metric
func CalculateTrend(points []*DataPoint, metricType string) (*TrendResult, error) {
	if len(points) < 2 {
		return nil, fmt.Errorf("insufficient data points for trend calculation")
	}

	// Sort points by timestamp
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	// Extract values based on metric type
	var values []float64
	var timestamps []float64

	for _, point := range points {
		var value float64
		switch metricType {
		case "temperature":
			value = point.Temperature
		case "humidity":
			value = point.Humidity
		case "pressure":
			value = point.Pressure
		default:
			return nil, fmt.Errorf("unsupported metric type: %s", metricType)
		}

		values = append(values, value)
		timestamps = append(timestamps, float64(point.Timestamp.Unix()))
	}

	// Calculate linear regression
	slope, _, rSquared := linearRegression(timestamps, values)

	// Determine trend direction and strength
	var direction string
	strength := math.Abs(slope) / (math.Abs(slope) + 1) // Normalize to 0-1

	if math.Abs(slope) < 0.01 {
		direction = "stable"
	} else if slope > 0 {
		direction = "increasing"
	} else {
		direction = "decreasing"
	}

	return &TrendResult{
		DeviceID:     points[0].DeviceID,
		Region:       points[0].Region,
		MetricType:   metricType,
		Direction:    direction,
		Strength:     strength,
		Slope:        slope,
		RSquared:     rSquared,
		StartValue:   values[0],
		EndValue:     values[len(values)-1],
		CalculatedAt: time.Now(),
	}, nil
}

// CalculateDelta calculates delta between current and previous values
func CalculateDelta(current, previous *DataPoint, metricType string, threshold float64) (*DeltaResult, error) {
	if current == nil || previous == nil {
		return nil, fmt.Errorf("invalid data points for delta calculation")
	}

	var currentValue, previousValue float64

	switch metricType {
	case "temperature":
		currentValue = current.Temperature
		previousValue = previous.Temperature
	case "humidity":
		currentValue = current.Humidity
		previousValue = previous.Humidity
	case "pressure":
		currentValue = current.Pressure
		previousValue = previous.Pressure
	default:
		return nil, fmt.Errorf("unsupported metric type: %s", metricType)
	}

	delta := currentValue - previousValue
	deltaPercent := 0.0
	if previousValue != 0 {
		deltaPercent = (delta / previousValue) * 100
	}

	isSignificant := math.Abs(delta) >= threshold

	return &DeltaResult{
		DeviceID:      current.DeviceID,
		Region:        current.Region,
		MetricType:    metricType,
		CurrentValue:  currentValue,
		PreviousValue: previousValue,
		Delta:         delta,
		DeltaPercent:  deltaPercent,
		IsSignificant: isSignificant,
		CalculatedAt:  time.Now(),
	}, nil
}

// CalculateRegionalAnalysis calculates regional statistics
func CalculateRegionalAnalysis(points []*DataPoint, region, metricType string) (*RegionalResult, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("no data points for regional analysis")
	}

	// Filter points by region and extract values
	var values []float64
	deviceCount := make(map[string]bool)

	for _, point := range points {
		if point.Region != region {
			continue
		}

		var value float64
		switch metricType {
		case "temperature":
			value = point.Temperature
		case "humidity":
			value = point.Humidity
		case "pressure":
			value = point.Pressure
		default:
			return nil, fmt.Errorf("unsupported metric type: %s", metricType)
		}

		values = append(values, value)
		deviceCount[point.DeviceID] = true
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no data points found for region %s", region)
	}

	// Calculate statistics
	average := calculateMean(values)
	minimum := calculateMin(values)
	maximum := calculateMax(values)
	standardDev := calculateStandardDeviation(values, average)
	median := calculateMedian(values)

	return &RegionalResult{
		Region:       region,
		MetricType:   metricType,
		DeviceCount:  len(deviceCount),
		Average:      average,
		Minimum:      minimum,
		Maximum:      maximum,
		StandardDev:  standardDev,
		Median:       median,
		CalculatedAt: time.Now(),
	}, nil
}

// DetectAnomalies detects anomalies using statistical methods
func DetectAnomalies(points []*DataPoint, metricType string) ([]*AnomalyResult, error) {
	if len(points) < 10 {
		return nil, fmt.Errorf("insufficient data points for anomaly detection")
	}

	// Extract values
	var values []float64
	for _, point := range points {
		var value float64
		switch metricType {
		case "temperature":
			value = point.Temperature
		case "humidity":
			value = point.Humidity
		case "pressure":
			value = point.Pressure
		default:
			return nil, fmt.Errorf("unsupported metric type: %s", metricType)
		}
		values = append(values, value)
	}

	// Calculate statistical thresholds
	mean := calculateMean(values)
	stdDev := calculateStandardDeviation(values, mean)

	var anomalies []*AnomalyResult

	// Check each point for anomalies
	for _, point := range points {
		var currentValue float64
		switch metricType {
		case "temperature":
			currentValue = point.Temperature
		case "humidity":
			currentValue = point.Humidity
		case "pressure":
			currentValue = point.Pressure
		}

		// Calculate z-score
		zScore := math.Abs(currentValue-mean) / stdDev

		// Determine if it's an anomaly
		var severity string
		var anomalyType string

		if zScore > 3.0 {
			severity = "critical"
		} else if zScore > 2.5 {
			severity = "high"
		} else if zScore > 2.0 {
			severity = "medium"
		} else if zScore > 1.5 {
			severity = "low"
		} else {
			continue // Not an anomaly
		}

		// Determine anomaly type
		if currentValue > mean+2*stdDev {
			anomalyType = "spike"
		} else if currentValue < mean-2*stdDev {
			anomalyType = "drop"
		} else {
			anomalyType = "drift"
		}

		anomaly := &AnomalyResult{
			DeviceID:      point.DeviceID,
			Region:        point.Region,
			MetricType:    metricType,
			CurrentValue:  currentValue,
			ExpectedValue: mean,
			Deviation:     zScore,
			Severity:      severity,
			AnomalyType:   anomalyType,
			DetectedAt:    time.Now(),
		}

		anomalies = append(anomalies, anomaly)
	}

	return anomalies, nil
}

// linearRegression calculates linear regression for trend analysis
func linearRegression(x, y []float64) (slope, intercept, rSquared float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0, 0
	}

	// Calculate means
	meanX := calculateMean(x)
	meanY := calculateMean(y)

	// Calculate slope and intercept
	var numerator, denominatorX, denominatorY float64

	for i := 0; i < len(x); i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY

		numerator += dx * dy
		denominatorX += dx * dx
		denominatorY += dy * dy
	}

	if denominatorX == 0 {
		return 0, meanY, 0
	}

	slope = numerator / denominatorX
	intercept = meanY - slope*meanX

	// Calculate R-squared
	if denominatorY == 0 {
		rSquared = 1.0
	} else {
		rSquared = (numerator * numerator) / (denominatorX * denominatorY)
	}

	return slope, intercept, rSquared
}

// Statistical helper functions
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateMin(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func calculateMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func calculateStandardDeviation(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values) - 1)

	return math.Sqrt(variance)
}

func calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Sort values
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		// Even number of values
		return (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		// Odd number of values
		return sorted[n/2]
	}
}

// MovingAverage calculates moving average for trend smoothing
func MovingAverage(values []float64, windowSize int) []float64 {
	if len(values) < windowSize {
		return values
	}

	result := make([]float64, len(values)-windowSize+1)

	for i := 0; i <= len(values)-windowSize; i++ {
		sum := 0.0
		for j := i; j < i+windowSize; j++ {
			sum += values[j]
		}
		result[i] = sum / float64(windowSize)
	}

	return result
}

// ExponentialMovingAverage calculates exponential moving average
func ExponentialMovingAverage(values []float64, alpha float64) []float64 {
	if len(values) == 0 {
		return values
	}

	result := make([]float64, len(values))
	result[0] = values[0]

	for i := 1; i < len(values); i++ {
		result[i] = alpha*values[i] + (1-alpha)*result[i-1]
	}

	return result
}

// CalculateCorrelation calculates correlation between two metrics
func CalculateCorrelation(values1, values2 []float64) (float64, error) {
	if len(values1) != len(values2) || len(values1) < 2 {
		return 0, fmt.Errorf("invalid input for correlation calculation")
	}

	mean1 := calculateMean(values1)
	mean2 := calculateMean(values2)

	var numerator, denom1, denom2 float64

	for i := 0; i < len(values1); i++ {
		diff1 := values1[i] - mean1
		diff2 := values2[i] - mean2

		numerator += diff1 * diff2
		denom1 += diff1 * diff1
		denom2 += diff2 * diff2
	}

	if denom1 == 0 || denom2 == 0 {
		return 0, nil
	}

	correlation := numerator / math.Sqrt(denom1*denom2)
	return correlation, nil
}
