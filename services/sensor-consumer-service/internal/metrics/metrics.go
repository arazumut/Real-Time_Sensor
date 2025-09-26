package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the sensor consumer service
type Metrics struct {
	// Message consumption metrics
	MessagesConsumedTotal  prometheus.Counter
	MessagesProcessedTotal prometheus.Counter
	MessagesFailedTotal    prometheus.Counter

	// Kafka consumer metrics
	KafkaLag            prometheus.Gauge
	KafkaCommitLatency  prometheus.Histogram
	KafkaPartitionCount prometheus.Gauge
	KafkaOffsetCurrent  *prometheus.GaugeVec

	// Database metrics
	DBInsertLatency     prometheus.Histogram
	DBInsertBatchSize   prometheus.Histogram
	DBConnectionsActive prometheus.Gauge
	DBErrorsTotal       prometheus.Counter
	DBRecordsInserted   prometheus.Counter

	// Redis cache metrics
	CacheHitRate           prometheus.Gauge
	CacheSetLatency        prometheus.Histogram
	CacheGetLatency        prometheus.Histogram
	CacheConnectionsActive prometheus.Gauge
	CacheErrorsTotal       prometheus.Counter
	CachePipelineSize      prometheus.Histogram

	// gRPC client metrics
	GRPCRequestsTotal     *prometheus.CounterVec
	GRPCRequestLatency    *prometheus.HistogramVec
	GRPCConnectionsActive prometheus.Gauge
	GRPCErrorsTotal       *prometheus.CounterVec

	// Alert metrics
	AlertsSentTotal   *prometheus.CounterVec
	AlertsFailedTotal *prometheus.CounterVec
	AlertLatency      prometheus.Histogram

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
			Name: "sensor_consumer_messages_consumed_total",
			Help: "Total number of messages consumed from Kafka",
		}),

		MessagesProcessedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_consumer_messages_processed_total",
			Help: "Total number of messages successfully processed",
		}),

		MessagesFailedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_consumer_messages_failed_total",
			Help: "Total number of messages that failed to process",
		}),

		KafkaLag: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_kafka_lag",
			Help: "Current Kafka consumer lag",
		}),

		KafkaCommitLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_consumer_kafka_commit_latency_ms",
			Help:    "Kafka commit latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		KafkaPartitionCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_kafka_partitions",
			Help: "Number of Kafka partitions assigned",
		}),

		KafkaOffsetCurrent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sensor_consumer_kafka_offset_current",
			Help: "Current Kafka offset by partition",
		}, []string{"partition"}),

		DBInsertLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_consumer_db_insert_latency_ms",
			Help:    "ClickHouse insert latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		DBInsertBatchSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_consumer_db_batch_size",
			Help:    "Size of database insert batches",
			Buckets: prometheus.LinearBuckets(100, 100, 20),
		}),

		DBConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_db_connections_active",
			Help: "Number of active database connections",
		}),

		DBErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_consumer_db_errors_total",
			Help: "Total number of database errors",
		}),

		DBRecordsInserted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_consumer_db_records_inserted_total",
			Help: "Total number of records inserted into database",
		}),

		CacheHitRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_cache_hit_rate",
			Help: "Cache hit rate percentage",
		}),

		CacheSetLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_consumer_cache_set_latency_ms",
			Help:    "Redis SET operation latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}),

		CacheGetLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_consumer_cache_get_latency_ms",
			Help:    "Redis GET operation latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}),

		CacheConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_cache_connections_active",
			Help: "Number of active Redis connections",
		}),

		CacheErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_consumer_cache_errors_total",
			Help: "Total number of cache errors",
		}),

		CachePipelineSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_consumer_cache_pipeline_size",
			Help:    "Size of Redis pipeline operations",
			Buckets: prometheus.LinearBuckets(10, 10, 20),
		}),

		GRPCRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sensor_consumer_grpc_requests_total",
			Help: "Total number of gRPC requests",
		}, []string{"service", "method", "status"}),

		GRPCRequestLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sensor_consumer_grpc_request_latency_ms",
			Help:    "gRPC request latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}, []string{"service", "method"}),

		GRPCConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_grpc_connections_active",
			Help: "Number of active gRPC connections",
		}),

		GRPCErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sensor_consumer_grpc_errors_total",
			Help: "Total number of gRPC errors",
		}, []string{"service", "method", "error_type"}),

		AlertsSentTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sensor_consumer_alerts_sent_total",
			Help: "Total number of alerts sent",
		}, []string{"device_id", "alert_type"}),

		AlertsFailedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sensor_consumer_alerts_failed_total",
			Help: "Total number of failed alerts",
		}, []string{"device_id", "alert_type", "error_type"}),

		AlertLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_consumer_alert_latency_ms",
			Help:    "Alert processing latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		MemoryUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),

		CPUUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_cpu_usage_percent",
			Help: "CPU usage percentage",
		}),

		GoroutinesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_goroutines",
			Help: "Number of goroutines",
		}),

		ProcessingRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_consumer_processing_rate_per_second",
			Help: "Message processing rate per second",
		}),
	}

	// Register all metrics
	prometheus.MustRegister(
		metrics.MessagesConsumedTotal,
		metrics.MessagesProcessedTotal,
		metrics.MessagesFailedTotal,
		metrics.KafkaLag,
		metrics.KafkaCommitLatency,
		metrics.KafkaPartitionCount,
		metrics.KafkaOffsetCurrent,
		metrics.DBInsertLatency,
		metrics.DBInsertBatchSize,
		metrics.DBConnectionsActive,
		metrics.DBErrorsTotal,
		metrics.DBRecordsInserted,
		metrics.CacheHitRate,
		metrics.CacheSetLatency,
		metrics.CacheGetLatency,
		metrics.CacheConnectionsActive,
		metrics.CacheErrorsTotal,
		metrics.CachePipelineSize,
		metrics.GRPCRequestsTotal,
		metrics.GRPCRequestLatency,
		metrics.GRPCConnectionsActive,
		metrics.GRPCErrorsTotal,
		metrics.AlertsSentTotal,
		metrics.AlertsFailedTotal,
		metrics.AlertLatency,
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

// Kafka Metrics Methods
func (m *Metrics) IncrementMessagesConsumed(count int) {
	m.MessagesConsumedTotal.Add(float64(count))
}

func (m *Metrics) IncrementMessagesProcessed(count int) {
	m.MessagesProcessedTotal.Add(float64(count))
}

func (m *Metrics) IncrementMessagesFailed(count int) {
	m.MessagesFailedTotal.Add(float64(count))
}

func (m *Metrics) SetKafkaLag(lag int64) {
	m.KafkaLag.Set(float64(lag))
}

func (m *Metrics) RecordKafkaCommitLatency(duration time.Duration) {
	m.KafkaCommitLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetKafkaPartitionCount(count int) {
	m.KafkaPartitionCount.Set(float64(count))
}

func (m *Metrics) SetKafkaOffset(partition string, offset int64) {
	m.KafkaOffsetCurrent.WithLabelValues(partition).Set(float64(offset))
}

// Database Metrics Methods
func (m *Metrics) RecordDBInsertLatency(duration time.Duration) {
	m.DBInsertLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordDBBatchSize(size int) {
	m.DBInsertBatchSize.Observe(float64(size))
}

func (m *Metrics) SetDBConnectionsActive(count int) {
	m.DBConnectionsActive.Set(float64(count))
}

func (m *Metrics) IncrementDBErrors() {
	m.DBErrorsTotal.Inc()
}

func (m *Metrics) IncrementDBRecordsInserted(count int) {
	m.DBRecordsInserted.Add(float64(count))
}

// Cache Metrics Methods
func (m *Metrics) SetCacheHitRate(rate float64) {
	m.CacheHitRate.Set(rate)
}

func (m *Metrics) RecordCacheSetLatency(duration time.Duration) {
	m.CacheSetLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordCacheGetLatency(duration time.Duration) {
	m.CacheGetLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetCacheConnectionsActive(count int) {
	m.CacheConnectionsActive.Set(float64(count))
}

func (m *Metrics) IncrementCacheErrors() {
	m.CacheErrorsTotal.Inc()
}

func (m *Metrics) RecordCachePipelineSize(size int) {
	m.CachePipelineSize.Observe(float64(size))
}

// gRPC Metrics Methods
func (m *Metrics) IncrementGRPCRequests(service, method, status string) {
	m.GRPCRequestsTotal.WithLabelValues(service, method, status).Inc()
}

func (m *Metrics) RecordGRPCLatency(service, method string, duration time.Duration) {
	m.GRPCRequestLatency.WithLabelValues(service, method).Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetGRPCConnectionsActive(count int) {
	m.GRPCConnectionsActive.Set(float64(count))
}

func (m *Metrics) IncrementGRPCErrors(service, method, errorType string) {
	m.GRPCErrorsTotal.WithLabelValues(service, method, errorType).Inc()
}

// Alert Metrics Methods
func (m *Metrics) IncrementAlertsSent(deviceID, alertType string) {
	m.AlertsSentTotal.WithLabelValues(deviceID, alertType).Inc()
}

func (m *Metrics) IncrementAlertsFailed(deviceID, alertType, errorType string) {
	m.AlertsFailedTotal.WithLabelValues(deviceID, alertType, errorType).Inc()
}

func (m *Metrics) RecordAlertLatency(duration time.Duration) {
	m.AlertLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
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
