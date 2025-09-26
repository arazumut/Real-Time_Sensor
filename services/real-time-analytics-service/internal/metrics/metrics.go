package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the real-time analytics service
type Metrics struct {
	// Message processing metrics
	MessagesConsumedTotal  prometheus.Counter
	AnalysisProcessedTotal prometheus.Counter
	AnalysisFailedTotal    prometheus.Counter

	// Analytics metrics
	AnalysisLatency      prometheus.Histogram
	TrendCalculationTime prometheus.Histogram
	DeltaCalculationTime prometheus.Histogram
	RegionalAnalysisTime prometheus.Histogram
	AnomalyDetectionTime prometheus.Histogram

	// MQTT metrics
	MQTTMessagesPublished prometheus.Counter
	MQTTMessagesFailed    prometheus.Counter
	MQTTPublishLatency    prometheus.Histogram
	MQTTConnectionStatus  prometheus.Gauge
	MQTTReconnectCount    prometheus.Counter

	// Data window metrics
	WindowSizeGauge    prometheus.Gauge
	ActiveDevicesGauge prometheus.Gauge
	DataPointsInWindow prometheus.Gauge
	WindowMemoryUsage  prometheus.Gauge

	// Trend analysis metrics
	TrendDirectionGauge *prometheus.GaugeVec
	TrendStrengthGauge  *prometheus.GaugeVec
	DeltaValueGauge     *prometheus.GaugeVec
	AnomalyCountTotal   prometheus.Counter

	// Regional analysis metrics
	RegionalAverageGauge *prometheus.GaugeVec
	RegionalDeviceCount  *prometheus.GaugeVec
	RegionalAnomalies    *prometheus.CounterVec

	// System metrics
	MemoryUsageGauge prometheus.Gauge
	CPUUsageGauge    prometheus.Gauge
	GoroutinesGauge  prometheus.Gauge
	ProcessingRate   prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	metrics := &Metrics{
		MessagesConsumedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "analytics_messages_consumed_total",
			Help: "Total number of messages consumed for analytics",
		}),

		AnalysisProcessedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "analysis_processed_total",
			Help: "Total number of analytics processed",
		}),

		AnalysisFailedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "analysis_failed_total",
			Help: "Total number of failed analytics",
		}),

		AnalysisLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analysis_latency_ms",
			Help:    "Analytics processing latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		TrendCalculationTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytics_trend_calculation_ms",
			Help:    "Trend calculation time in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}),

		DeltaCalculationTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytics_delta_calculation_ms",
			Help:    "Delta calculation time in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}),

		RegionalAnalysisTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytics_regional_analysis_ms",
			Help:    "Regional analysis time in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		AnomalyDetectionTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytics_anomaly_detection_ms",
			Help:    "Anomaly detection time in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}),

		MQTTMessagesPublished: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mqtt_analytics_messages_sent_total",
			Help: "Total number of analytics messages published to MQTT",
		}),

		MQTTMessagesFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mqtt_analytics_messages_failed_total",
			Help: "Total number of failed MQTT analytics messages",
		}),

		MQTTPublishLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "mqtt_publish_latency_ms",
			Help:    "MQTT publish latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		MQTTConnectionStatus: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mqtt_connection_status",
			Help: "MQTT connection status (1=connected, 0=disconnected)",
		}),

		MQTTReconnectCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mqtt_reconnect_total",
			Help: "Total number of MQTT reconnections",
		}),

		WindowSizeGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_window_size_minutes",
			Help: "Current analytics window size in minutes",
		}),

		ActiveDevicesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_active_devices",
			Help: "Number of devices being analyzed",
		}),

		DataPointsInWindow: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_data_points_in_window",
			Help: "Number of data points in current analysis window",
		}),

		WindowMemoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_window_memory_bytes",
			Help: "Memory usage of analytics window in bytes",
		}),

		TrendDirectionGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "analytics_trend_direction",
			Help: "Trend direction by device (1=increasing, 0=stable, -1=decreasing)",
		}, []string{"device_id", "region"}),

		TrendStrengthGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "analytics_trend_strength",
			Help: "Trend strength by device (0-1 scale)",
		}, []string{"device_id", "region"}),

		DeltaValueGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "analytics_delta_value",
			Help: "Delta value by device",
		}, []string{"device_id", "metric_type"}),

		AnomalyCountTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "analytics_anomalies_detected_total",
			Help: "Total number of anomalies detected",
		}),

		RegionalAverageGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "analytics_regional_average",
			Help: "Regional average values",
		}, []string{"region", "metric_type"}),

		RegionalDeviceCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "analytics_regional_device_count",
			Help: "Number of devices per region",
		}, []string{"region"}),

		RegionalAnomalies: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_regional_anomalies_total",
			Help: "Total anomalies detected per region",
		}, []string{"region", "anomaly_type"}),

		MemoryUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),

		CPUUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_cpu_usage_percent",
			Help: "CPU usage percentage",
		}),

		GoroutinesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_goroutines",
			Help: "Number of goroutines",
		}),

		ProcessingRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_processing_rate_per_second",
			Help: "Analytics processing rate per second",
		}),
	}

	// Register all metrics
	prometheus.MustRegister(
		metrics.MessagesConsumedTotal,
		metrics.AnalysisProcessedTotal,
		metrics.AnalysisFailedTotal,
		metrics.AnalysisLatency,
		metrics.TrendCalculationTime,
		metrics.DeltaCalculationTime,
		metrics.RegionalAnalysisTime,
		metrics.AnomalyDetectionTime,
		metrics.MQTTMessagesPublished,
		metrics.MQTTMessagesFailed,
		metrics.MQTTPublishLatency,
		metrics.MQTTConnectionStatus,
		metrics.MQTTReconnectCount,
		metrics.WindowSizeGauge,
		metrics.ActiveDevicesGauge,
		metrics.DataPointsInWindow,
		metrics.WindowMemoryUsage,
		metrics.TrendDirectionGauge,
		metrics.TrendStrengthGauge,
		metrics.DeltaValueGauge,
		metrics.AnomalyCountTotal,
		metrics.RegionalAverageGauge,
		metrics.RegionalDeviceCount,
		metrics.RegionalAnomalies,
		metrics.MemoryUsageGauge,
		metrics.CPUUsageGauge,
		metrics.GoroutinesGauge,
		metrics.ProcessingRate,
	)

	return metrics
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(port int, path string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return server
}

// Analytics Metrics Methods
func (m *Metrics) IncrementMessagesConsumed(count int) {
	m.MessagesConsumedTotal.Add(float64(count))
}

func (m *Metrics) IncrementAnalysisProcessed(count int) {
	m.AnalysisProcessedTotal.Add(float64(count))
}

func (m *Metrics) IncrementAnalysisFailed(count int) {
	m.AnalysisFailedTotal.Add(float64(count))
}

func (m *Metrics) RecordAnalysisLatency(duration time.Duration) {
	m.AnalysisLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordTrendCalculationTime(duration time.Duration) {
	m.TrendCalculationTime.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordDeltaCalculationTime(duration time.Duration) {
	m.DeltaCalculationTime.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordRegionalAnalysisTime(duration time.Duration) {
	m.RegionalAnalysisTime.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordAnomalyDetectionTime(duration time.Duration) {
	m.AnomalyDetectionTime.Observe(float64(duration.Nanoseconds()) / 1e6)
}

// MQTT Metrics Methods
func (m *Metrics) IncrementMQTTPublished(count int) {
	m.MQTTMessagesPublished.Add(float64(count))
}

func (m *Metrics) IncrementMQTTFailed(count int) {
	m.MQTTMessagesFailed.Add(float64(count))
}

func (m *Metrics) RecordMQTTLatency(duration time.Duration) {
	m.MQTTPublishLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetMQTTConnectionStatus(connected bool) {
	if connected {
		m.MQTTConnectionStatus.Set(1)
	} else {
		m.MQTTConnectionStatus.Set(0)
	}
}

func (m *Metrics) IncrementMQTTReconnect() {
	m.MQTTReconnectCount.Inc()
}

// Window Metrics Methods
func (m *Metrics) SetWindowSize(minutes float64) {
	m.WindowSizeGauge.Set(minutes)
}

func (m *Metrics) SetActiveDevices(count int) {
	m.ActiveDevicesGauge.Set(float64(count))
}

func (m *Metrics) SetDataPointsInWindow(count int) {
	m.DataPointsInWindow.Set(float64(count))
}

func (m *Metrics) SetWindowMemoryUsage(bytes int64) {
	m.WindowMemoryUsage.Set(float64(bytes))
}

// Trend Metrics Methods
func (m *Metrics) SetTrendDirection(deviceID, region string, direction float64) {
	m.TrendDirectionGauge.WithLabelValues(deviceID, region).Set(direction)
}

func (m *Metrics) SetTrendStrength(deviceID, region string, strength float64) {
	m.TrendStrengthGauge.WithLabelValues(deviceID, region).Set(strength)
}

func (m *Metrics) SetDeltaValue(deviceID, metricType string, delta float64) {
	m.DeltaValueGauge.WithLabelValues(deviceID, metricType).Set(delta)
}

func (m *Metrics) IncrementAnomalies() {
	m.AnomalyCountTotal.Inc()
}

// Regional Metrics Methods
func (m *Metrics) SetRegionalAverage(region, metricType string, average float64) {
	m.RegionalAverageGauge.WithLabelValues(region, metricType).Set(average)
}

func (m *Metrics) SetRegionalDeviceCount(region string, count int) {
	m.RegionalDeviceCount.WithLabelValues(region).Set(float64(count))
}

func (m *Metrics) IncrementRegionalAnomalies(region, anomalyType string) {
	m.RegionalAnomalies.WithLabelValues(region, anomalyType).Inc()
}

// System Metrics Methods
func (m *Metrics) UpdateSystemMetrics(memoryBytes, cpuPercent float64, goroutines int) {
	m.MemoryUsageGauge.Set(memoryBytes)
	m.CPUUsageGauge.Set(cpuPercent)
	m.GoroutinesGauge.Set(float64(goroutines))
}

func (m *Metrics) SetProcessingRate(rate float64) {
	m.ProcessingRate.Set(rate)
}
