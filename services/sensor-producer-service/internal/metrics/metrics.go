package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the sensor producer service
type Metrics struct {
	// Message production metrics
	MessagesProducedTotal prometheus.Counter
	MessagesSentTotal     prometheus.Counter
	MessagesFailedTotal   prometheus.Counter

	// Performance metrics
	ProducerLatency    prometheus.Histogram
	BatchSizeHistogram prometheus.Histogram

	// Device simulation metrics
	ActiveDevicesGauge  prometheus.Gauge
	SimulationRateGauge prometheus.Gauge

	// Kafka metrics
	KafkaConnectionsGauge prometheus.Gauge
	KafkaErrorsTotal      prometheus.Counter

	// System metrics
	MemoryUsageGauge prometheus.Gauge
	CPUUsageGauge    prometheus.Gauge
	GoroutinesGauge  prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	metrics := &Metrics{
		MessagesProducedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_producer_messages_produced_total",
			Help: "Total number of sensor messages produced",
		}),

		MessagesSentTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_producer_messages_sent_total",
			Help: "Total number of sensor messages successfully sent to Kafka",
		}),

		MessagesFailedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_producer_messages_failed_total",
			Help: "Total number of sensor messages that failed to send",
		}),

		ProducerLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_producer_latency_ms",
			Help:    "Latency of message production in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15), // 1ms to ~32s
		}),

		BatchSizeHistogram: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "sensor_producer_batch_size",
			Help:    "Size of message batches sent to Kafka",
			Buckets: prometheus.LinearBuckets(100, 100, 20), // 100 to 2000
		}),

		ActiveDevicesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_producer_active_devices",
			Help: "Number of active simulated devices",
		}),

		SimulationRateGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_producer_simulation_rate_per_second",
			Help: "Rate of sensor data simulation per second",
		}),

		KafkaConnectionsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_producer_kafka_connections",
			Help: "Number of active Kafka connections",
		}),

		KafkaErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "sensor_producer_kafka_errors_total",
			Help: "Total number of Kafka errors",
		}),

		MemoryUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_producer_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),

		CPUUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_producer_cpu_usage_percent",
			Help: "CPU usage percentage",
		}),

		GoroutinesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "sensor_producer_goroutines",
			Help: "Number of goroutines",
		}),
	}

	// Register all metrics
	prometheus.MustRegister(
		metrics.MessagesProducedTotal,
		metrics.MessagesSentTotal,
		metrics.MessagesFailedTotal,
		metrics.ProducerLatency,
		metrics.BatchSizeHistogram,
		metrics.ActiveDevicesGauge,
		metrics.SimulationRateGauge,
		metrics.KafkaConnectionsGauge,
		metrics.KafkaErrorsTotal,
		metrics.MemoryUsageGauge,
		metrics.CPUUsageGauge,
		metrics.GoroutinesGauge,
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

// RecordProducerLatency records the latency of message production
func (m *Metrics) RecordProducerLatency(duration time.Duration) {
	m.ProducerLatency.Observe(float64(duration.Nanoseconds()) / 1e6) // Convert to milliseconds
}

// RecordBatchSize records the size of a message batch
func (m *Metrics) RecordBatchSize(size int) {
	m.BatchSizeHistogram.Observe(float64(size))
}

// IncrementMessagesProduced increments the total messages produced counter
func (m *Metrics) IncrementMessagesProduced(count int) {
	m.MessagesProducedTotal.Add(float64(count))
}

// IncrementMessagesSent increments the total messages sent counter
func (m *Metrics) IncrementMessagesSent(count int) {
	m.MessagesSentTotal.Add(float64(count))
}

// IncrementMessagesFailed increments the total messages failed counter
func (m *Metrics) IncrementMessagesFailed(count int) {
	m.MessagesFailedTotal.Add(float64(count))
}

// SetActiveDevices sets the number of active devices
func (m *Metrics) SetActiveDevices(count int) {
	m.ActiveDevicesGauge.Set(float64(count))
}

// SetSimulationRate sets the simulation rate per second
func (m *Metrics) SetSimulationRate(rate float64) {
	m.SimulationRateGauge.Set(rate)
}

// SetKafkaConnections sets the number of Kafka connections
func (m *Metrics) SetKafkaConnections(count int) {
	m.KafkaConnectionsGauge.Set(float64(count))
}

// IncrementKafkaErrors increments the Kafka errors counter
func (m *Metrics) IncrementKafkaErrors() {
	m.KafkaErrorsTotal.Inc()
}

// UpdateSystemMetrics updates system-related metrics
func (m *Metrics) UpdateSystemMetrics(memoryBytes, cpuPercent float64, goroutines int) {
	m.MemoryUsageGauge.Set(memoryBytes)
	m.CPUUsageGauge.Set(cpuPercent)
	m.GoroutinesGauge.Set(float64(goroutines))
}
