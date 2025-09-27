package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the API gateway service
type Metrics struct {
	// HTTP request metrics
	HTTPRequestsTotal  *prometheus.CounterVec
	HTTPRequestLatency *prometheus.HistogramVec
	HTTPResponseSize   prometheus.Histogram
	HTTPErrorsTotal    *prometheus.CounterVec

	// Cache metrics
	CacheHitRate           prometheus.Gauge
	CacheRequestsTotal     *prometheus.CounterVec
	CacheLatency           *prometheus.HistogramVec
	CacheConnectionsActive prometheus.Gauge
	CacheErrorsTotal       prometheus.Counter

	// Database metrics
	DBRequestsTotal     *prometheus.CounterVec
	DBLatency           *prometheus.HistogramVec
	DBConnectionsActive prometheus.Gauge
	DBErrorsTotal       prometheus.Counter
	DBFallbackTotal     prometheus.Counter

	// Authentication metrics
	AuthRequestsTotal *prometheus.CounterVec
	AuthLatency       prometheus.Histogram
	AuthCacheHitRate  prometheus.Gauge
	AuthErrorsTotal   *prometheus.CounterVec

	// Device metrics
	DeviceRequestsTotal *prometheus.CounterVec
	DeviceResponseTime  prometheus.Histogram
	ActiveDevicesGauge  prometheus.Gauge
	DeviceErrorsTotal   *prometheus.CounterVec

	// Analytics metrics
	AnalyticsRequestsTotal *prometheus.CounterVec
	AnalyticsLatency       prometheus.Histogram
	AnalyticsErrorsTotal   prometheus.Counter

	// Rate limiting metrics
	RateLimitHitsTotal   *prometheus.CounterVec
	RateLimitBypassTotal prometheus.Counter

	// Gateway metrics
	ConcurrentRequestsGauge  prometheus.Gauge
	RequestQueueSize         prometheus.Gauge
	ResponseCompressionRatio prometheus.Histogram

	// System metrics
	MemoryUsageGauge prometheus.Gauge
	CPUUsageGauge    prometheus.Gauge
	GoroutinesGauge  prometheus.Gauge
	ProcessingRate   prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	metrics := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "endpoint", "status"}),

		HTTPRequestLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_http_request_latency_ms",
			Help:    "HTTP request latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}, []string{"method", "endpoint"}),

		HTTPResponseSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 2, 15),
		}),

		HTTPErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_http_errors_total",
			Help: "Total number of HTTP errors",
		}, []string{"method", "endpoint", "error_type"}),

		CacheHitRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_cache_hit_rate",
			Help: "Cache hit rate percentage",
		}),

		CacheRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_cache_requests_total",
			Help: "Total number of cache requests",
		}, []string{"operation", "result"}),

		CacheLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_cache_latency_ms",
			Help:    "Cache operation latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}, []string{"operation"}),

		CacheConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_cache_connections_active",
			Help: "Number of active cache connections",
		}),

		CacheErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_cache_errors_total",
			Help: "Total number of cache errors",
		}),

		DBRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_db_requests_total",
			Help: "Total number of database requests",
		}, []string{"operation", "result"}),

		DBLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_db_latency_ms",
			Help:    "Database operation latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}, []string{"operation"}),

		DBConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_db_connections_active",
			Help: "Number of active database connections",
		}),

		DBErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_db_errors_total",
			Help: "Total number of database errors",
		}),

		DBFallbackTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_db_fallback_total",
			Help: "Total number of database fallback operations",
		}),

		AuthRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_auth_requests_total",
			Help: "Total number of authentication requests",
		}, []string{"result"}),

		AuthLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_auth_latency_ms",
			Help:    "Authentication latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		AuthCacheHitRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_auth_cache_hit_rate",
			Help: "Authentication cache hit rate percentage",
		}),

		AuthErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_auth_errors_total",
			Help: "Total number of authentication errors",
		}, []string{"error_type"}),

		DeviceRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_device_requests_total",
			Help: "Total number of device-related requests",
		}, []string{"device_id", "operation"}),

		DeviceResponseTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_device_response_time_ms",
			Help:    "Device request response time in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		ActiveDevicesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_active_devices",
			Help: "Number of active devices being served",
		}),

		DeviceErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_device_errors_total",
			Help: "Total number of device-related errors",
		}, []string{"device_id", "error_type"}),

		AnalyticsRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_analytics_requests_total",
			Help: "Total number of analytics requests",
		}, []string{"analytics_type"}),

		AnalyticsLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_analytics_latency_ms",
			Help:    "Analytics request latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		AnalyticsErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_analytics_errors_total",
			Help: "Total number of analytics errors",
		}),

		RateLimitHitsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		}, []string{"endpoint", "client_ip"}),

		RateLimitBypassTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_rate_limit_bypass_total",
			Help: "Total number of rate limit bypasses",
		}),

		ConcurrentRequestsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_concurrent_requests",
			Help: "Number of concurrent requests being processed",
		}),

		RequestQueueSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_request_queue_size",
			Help: "Current request queue size",
		}),

		ResponseCompressionRatio: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "gateway_response_compression_ratio",
			Help:    "Response compression ratio",
			Buckets: prometheus.LinearBuckets(0.1, 0.1, 10),
		}),

		MemoryUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),

		CPUUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_cpu_usage_percent",
			Help: "CPU usage percentage",
		}),

		GoroutinesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_goroutines",
			Help: "Number of goroutines",
		}),

		ProcessingRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gateway_processing_rate_per_second",
			Help: "Request processing rate per second",
		}),
	}

	// Register all metrics
	prometheus.MustRegister(
		metrics.HTTPRequestsTotal,
		metrics.HTTPRequestLatency,
		metrics.HTTPResponseSize,
		metrics.HTTPErrorsTotal,
		metrics.CacheHitRate,
		metrics.CacheRequestsTotal,
		metrics.CacheLatency,
		metrics.CacheConnectionsActive,
		metrics.CacheErrorsTotal,
		metrics.DBRequestsTotal,
		metrics.DBLatency,
		metrics.DBConnectionsActive,
		metrics.DBErrorsTotal,
		metrics.DBFallbackTotal,
		metrics.AuthRequestsTotal,
		metrics.AuthLatency,
		metrics.AuthCacheHitRate,
		metrics.AuthErrorsTotal,
		metrics.DeviceRequestsTotal,
		metrics.DeviceResponseTime,
		metrics.ActiveDevicesGauge,
		metrics.DeviceErrorsTotal,
		metrics.AnalyticsRequestsTotal,
		metrics.AnalyticsLatency,
		metrics.AnalyticsErrorsTotal,
		metrics.RateLimitHitsTotal,
		metrics.RateLimitBypassTotal,
		metrics.ConcurrentRequestsGauge,
		metrics.RequestQueueSize,
		metrics.ResponseCompressionRatio,
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

// HTTP Metrics Methods
func (m *Metrics) IncrementHTTPRequests(method, endpoint, status string) {
	m.HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
}

func (m *Metrics) RecordHTTPLatency(method, endpoint string, duration time.Duration) {
	m.HTTPRequestLatency.WithLabelValues(method, endpoint).Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordHTTPResponseSize(size int) {
	m.HTTPResponseSize.Observe(float64(size))
}

func (m *Metrics) IncrementHTTPErrors(method, endpoint, errorType string) {
	m.HTTPErrorsTotal.WithLabelValues(method, endpoint, errorType).Inc()
}

// Cache Metrics Methods
func (m *Metrics) SetCacheHitRate(rate float64) {
	m.CacheHitRate.Set(rate)
}

func (m *Metrics) IncrementCacheRequests(operation, result string) {
	m.CacheRequestsTotal.WithLabelValues(operation, result).Inc()
}

func (m *Metrics) RecordCacheLatency(operation string, duration time.Duration) {
	m.CacheLatency.WithLabelValues(operation).Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetCacheConnectionsActive(count int) {
	m.CacheConnectionsActive.Set(float64(count))
}

func (m *Metrics) IncrementCacheErrors() {
	m.CacheErrorsTotal.Inc()
}

// Database Metrics Methods
func (m *Metrics) IncrementDBRequests(operation, result string) {
	m.DBRequestsTotal.WithLabelValues(operation, result).Inc()
}

func (m *Metrics) RecordDBLatency(operation string, duration time.Duration) {
	m.DBLatency.WithLabelValues(operation).Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetDBConnectionsActive(count int) {
	m.DBConnectionsActive.Set(float64(count))
}

func (m *Metrics) IncrementDBErrors() {
	m.DBErrorsTotal.Inc()
}

func (m *Metrics) IncrementDBFallback() {
	m.DBFallbackTotal.Inc()
}

// Authentication Metrics Methods
func (m *Metrics) IncrementAuthRequests(result string) {
	m.AuthRequestsTotal.WithLabelValues(result).Inc()
}

func (m *Metrics) RecordAuthLatency(duration time.Duration) {
	m.AuthLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetAuthCacheHitRate(rate float64) {
	m.AuthCacheHitRate.Set(rate)
}

func (m *Metrics) IncrementAuthErrors(errorType string) {
	m.AuthErrorsTotal.WithLabelValues(errorType).Inc()
}

// Device Metrics Methods
func (m *Metrics) IncrementDeviceRequests(deviceID, operation string) {
	m.DeviceRequestsTotal.WithLabelValues(deviceID, operation).Inc()
}

func (m *Metrics) RecordDeviceResponseTime(duration time.Duration) {
	m.DeviceResponseTime.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetActiveDevices(count int) {
	m.ActiveDevicesGauge.Set(float64(count))
}

func (m *Metrics) IncrementDeviceErrors(deviceID, errorType string) {
	m.DeviceErrorsTotal.WithLabelValues(deviceID, errorType).Inc()
}

// Analytics Metrics Methods
func (m *Metrics) IncrementAnalyticsRequests(analyticsType string) {
	m.AnalyticsRequestsTotal.WithLabelValues(analyticsType).Inc()
}

func (m *Metrics) RecordAnalyticsLatency(duration time.Duration) {
	m.AnalyticsLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) IncrementAnalyticsErrors() {
	m.AnalyticsErrorsTotal.Inc()
}

// Rate Limiting Metrics Methods
func (m *Metrics) IncrementRateLimitHits(endpoint, clientIP string) {
	m.RateLimitHitsTotal.WithLabelValues(endpoint, clientIP).Inc()
}

func (m *Metrics) IncrementRateLimitBypass() {
	m.RateLimitBypassTotal.Inc()
}

// Gateway Metrics Methods
func (m *Metrics) SetConcurrentRequests(count int) {
	m.ConcurrentRequestsGauge.Set(float64(count))
}

func (m *Metrics) SetRequestQueueSize(size int) {
	m.RequestQueueSize.Set(float64(size))
}

func (m *Metrics) RecordCompressionRatio(ratio float64) {
	m.ResponseCompressionRatio.Observe(ratio)
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
