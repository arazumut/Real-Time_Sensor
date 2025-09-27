package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the alert handler service
type Metrics struct {
	// Alert processing metrics
	AlertsReceivedTotal    prometheus.Counter
	AlertsProcessedTotal   prometheus.Counter
	AlertsFailedTotal      prometheus.Counter
	AlertsFilteredTotal    prometheus.Counter
	
	// Alert type metrics
	AlertsByTypeTotal      *prometheus.CounterVec
	AlertsBySeverityTotal  *prometheus.CounterVec
	AlertsByDeviceTotal    *prometheus.CounterVec
	AlertsByRegionTotal    *prometheus.CounterVec
	
	// Processing metrics
	AlertProcessingLatency prometheus.Histogram
	AlertQueueSize         prometheus.Gauge
	AlertQueueLatency      prometheus.Histogram
	
	// Email metrics
	EmailsSentTotal        prometheus.Counter
	EmailsFailedTotal      prometheus.Counter
	EmailSendLatency       prometheus.Histogram
	EmailQueueSize         prometheus.Gauge
	EmailRetryCount        prometheus.Counter
	
	// Rate limiting metrics
	RateLimitHitsTotal     prometheus.Counter
	RateLimitBypassTotal   prometheus.Counter
	CooldownActiveGauge    prometheus.Gauge
	
	// Deduplication metrics
	DuplicateAlertsTotal   prometheus.Counter
	DeduplicationHitRate   prometheus.Gauge
	
	// gRPC server metrics
	GRPCRequestsTotal      *prometheus.CounterVec
	GRPCRequestLatency     *prometheus.HistogramVec
	GRPCConnectionsActive  prometheus.Gauge
	GRPCErrorsTotal        *prometheus.CounterVec
	
	// Storage metrics
	StoredAlertsTotal      prometheus.Counter
	StorageErrorsTotal     prometheus.Counter
	StorageCleanupTotal    prometheus.Counter
	StorageSize            prometheus.Gauge
	
	// System metrics
	MemoryUsageGauge       prometheus.Gauge
	CPUUsageGauge          prometheus.Gauge
	GoroutinesGauge        prometheus.Gauge
	ProcessingRate         prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	metrics := &Metrics{
		AlertsReceivedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_alerts_received_total",
			Help: "Total number of alerts received",
		}),
		
		AlertsProcessedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_alerts_processed_total",
			Help: "Total number of alerts successfully processed",
		}),
		
		AlertsFailedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_alerts_failed_total",
			Help: "Total number of alerts that failed to process",
		}),
		
		AlertsFilteredTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_alerts_filtered_total",
			Help: "Total number of alerts filtered out (rate limiting, duplicates)",
		}),
		
		AlertsByTypeTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "alert_handler_alerts_by_type_total",
			Help: "Total alerts by alert type",
		}, []string{"alert_type"}),
		
		AlertsBySeverityTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "alert_handler_alerts_by_severity_total",
			Help: "Total alerts by severity level",
		}, []string{"severity"}),
		
		AlertsByDeviceTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "alert_handler_alerts_by_device_total",
			Help: "Total alerts by device",
		}, []string{"device_id"}),
		
		AlertsByRegionTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "alert_handler_alerts_by_region_total",
			Help: "Total alerts by region",
		}, []string{"region"}),
		
		AlertProcessingLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "alert_handler_processing_latency_ms",
			Help:    "Alert processing latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),
		
		AlertQueueSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_queue_size",
			Help: "Current alert queue size",
		}),
		
		AlertQueueLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "alert_handler_queue_latency_ms",
			Help:    "Time alerts spend in queue before processing",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),
		
		EmailsSentTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_emails_sent_total",
			Help: "Total number of emails sent",
		}),
		
		EmailsFailedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_emails_failed_total",
			Help: "Total number of emails that failed to send",
		}),
		
		EmailSendLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "alert_handler_email_send_latency_ms",
			Help:    "Email send latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 15),
		}),
		
		EmailQueueSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_email_queue_size",
			Help: "Current email queue size",
		}),
		
		EmailRetryCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_email_retry_total",
			Help: "Total number of email retry attempts",
		}),
		
		RateLimitHitsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		}),
		
		RateLimitBypassTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_rate_limit_bypass_total",
			Help: "Total number of rate limit bypasses",
		}),
		
		CooldownActiveGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_cooldown_active",
			Help: "Number of devices currently in cooldown",
		}),
		
		DuplicateAlertsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_duplicate_alerts_total",
			Help: "Total number of duplicate alerts filtered",
		}),
		
		DeduplicationHitRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_deduplication_hit_rate",
			Help: "Deduplication hit rate percentage",
		}),
		
		GRPCRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "alert_handler_grpc_requests_total",
			Help: "Total number of gRPC requests",
		}, []string{"method", "status"}),
		
		GRPCRequestLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "alert_handler_grpc_request_latency_ms",
			Help:    "gRPC request latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}, []string{"method"}),
		
		GRPCConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_grpc_connections_active",
			Help: "Number of active gRPC connections",
		}),
		
		GRPCErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "alert_handler_grpc_errors_total",
			Help: "Total number of gRPC errors",
		}, []string{"method", "error_type"}),
		
		StoredAlertsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_stored_alerts_total",
			Help: "Total number of alerts stored",
		}),
		
		StorageErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_storage_errors_total",
			Help: "Total number of storage errors",
		}),
		
		StorageCleanupTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "alert_handler_storage_cleanup_total",
			Help: "Total number of storage cleanup operations",
		}),
		
		StorageSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_storage_size",
			Help: "Current number of stored alerts",
		}),
		
		MemoryUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),
		
		CPUUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_cpu_usage_percent",
			Help: "CPU usage percentage",
		}),
		
		GoroutinesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_goroutines",
			Help: "Number of goroutines",
		}),
		
		ProcessingRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alert_handler_processing_rate_per_second",
			Help: "Alert processing rate per second",
		}),
	}

	// Register all metrics
	prometheus.MustRegister(
		metrics.AlertsReceivedTotal,
		metrics.AlertsProcessedTotal,
		metrics.AlertsFailedTotal,
		metrics.AlertsFilteredTotal,
		metrics.AlertsByTypeTotal,
		metrics.AlertsBySeverityTotal,
		metrics.AlertsByDeviceTotal,
		metrics.AlertsByRegionTotal,
		metrics.AlertProcessingLatency,
		metrics.AlertQueueSize,
		metrics.AlertQueueLatency,
		metrics.EmailsSentTotal,
		metrics.EmailsFailedTotal,
		metrics.EmailSendLatency,
		metrics.EmailQueueSize,
		metrics.EmailRetryCount,
		metrics.RateLimitHitsTotal,
		metrics.RateLimitBypassTotal,
		metrics.CooldownActiveGauge,
		metrics.DuplicateAlertsTotal,
		metrics.DeduplicationHitRate,
		metrics.GRPCRequestsTotal,
		metrics.GRPCRequestLatency,
		metrics.GRPCConnectionsActive,
		metrics.GRPCErrorsTotal,
		metrics.StoredAlertsTotal,
		metrics.StorageErrorsTotal,
		metrics.StorageCleanupTotal,
		metrics.StorageSize,
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

// Alert Processing Metrics Methods
func (m *Metrics) IncrementAlertsReceived() {
	m.AlertsReceivedTotal.Inc()
}

func (m *Metrics) IncrementAlertsProcessed() {
	m.AlertsProcessedTotal.Inc()
}

func (m *Metrics) IncrementAlertsFailed() {
	m.AlertsFailedTotal.Inc()
}

func (m *Metrics) IncrementAlertsFiltered() {
	m.AlertsFilteredTotal.Inc()
}

func (m *Metrics) IncrementAlertsByType(alertType string) {
	m.AlertsByTypeTotal.WithLabelValues(alertType).Inc()
}

func (m *Metrics) IncrementAlertsBySeverity(severity string) {
	m.AlertsBySeverityTotal.WithLabelValues(severity).Inc()
}

func (m *Metrics) IncrementAlertsByDevice(deviceID string) {
	m.AlertsByDeviceTotal.WithLabelValues(deviceID).Inc()
}

func (m *Metrics) IncrementAlertsByRegion(region string) {
	m.AlertsByRegionTotal.WithLabelValues(region).Inc()
}

func (m *Metrics) RecordAlertProcessingLatency(duration time.Duration) {
	m.AlertProcessingLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetAlertQueueSize(size int) {
	m.AlertQueueSize.Set(float64(size))
}

func (m *Metrics) RecordAlertQueueLatency(duration time.Duration) {
	m.AlertQueueLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

// Email Metrics Methods
func (m *Metrics) IncrementEmailsSent() {
	m.EmailsSentTotal.Inc()
}

func (m *Metrics) IncrementEmailsFailed() {
	m.EmailsFailedTotal.Inc()
}

func (m *Metrics) RecordEmailSendLatency(duration time.Duration) {
	m.EmailSendLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetEmailQueueSize(size int) {
	m.EmailQueueSize.Set(float64(size))
}

func (m *Metrics) IncrementEmailRetry() {
	m.EmailRetryCount.Inc()
}

// Rate Limiting Metrics Methods
func (m *Metrics) IncrementRateLimitHits() {
	m.RateLimitHitsTotal.Inc()
}

func (m *Metrics) IncrementRateLimitBypass() {
	m.RateLimitBypassTotal.Inc()
}

func (m *Metrics) SetCooldownActive(count int) {
	m.CooldownActiveGauge.Set(float64(count))
}

// Deduplication Metrics Methods
func (m *Metrics) IncrementDuplicateAlerts() {
	m.DuplicateAlertsTotal.Inc()
}

func (m *Metrics) SetDeduplicationHitRate(rate float64) {
	m.DeduplicationHitRate.Set(rate)
}

// gRPC Metrics Methods
func (m *Metrics) IncrementGRPCRequests(method, status string) {
	m.GRPCRequestsTotal.WithLabelValues(method, status).Inc()
}

func (m *Metrics) RecordGRPCLatency(method string, duration time.Duration) {
	m.GRPCRequestLatency.WithLabelValues(method).Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetGRPCConnectionsActive(count int) {
	m.GRPCConnectionsActive.Set(float64(count))
}

func (m *Metrics) IncrementGRPCErrors(method, errorType string) {
	m.GRPCErrorsTotal.WithLabelValues(method, errorType).Inc()
}

// Storage Metrics Methods
func (m *Metrics) IncrementStoredAlerts() {
	m.StoredAlertsTotal.Inc()
}

func (m *Metrics) IncrementStorageErrors() {
	m.StorageErrorsTotal.Inc()
}

func (m *Metrics) IncrementStorageCleanup() {
	m.StorageCleanupTotal.Inc()
}

func (m *Metrics) SetStorageSize(size int) {
	m.StorageSize.Set(float64(size))
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
