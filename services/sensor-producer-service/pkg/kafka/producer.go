package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/twinup/sensor-system/services/sensor-producer-service/internal/simulator"
	"github.com/twinup/sensor-system/services/sensor-producer-service/pkg/logger"
	"go.uber.org/zap"
)

// Producer handles Kafka message production with high performance
type Producer struct {
	producer sarama.AsyncProducer
	topic    string
	logger   logger.Logger
	metrics  ProducerMetrics
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
	mutex    sync.RWMutex

	// Performance tracking
	successCount int64
	errorCount   int64
	startTime    time.Time
}

// ProducerMetrics interface for metrics collection
type ProducerMetrics interface {
	IncrementMessagesSent(count int)
	IncrementMessagesFailed(count int)
	RecordProducerLatency(duration time.Duration)
	SetKafkaConnections(count int)
	IncrementKafkaErrors()
}

// ProducerConfig holds Kafka producer configuration
type ProducerConfig struct {
	Brokers         []string
	Topic           string
	RetryMax        int
	RequiredAcks    int
	Timeout         time.Duration
	CompressionType string
	FlushFrequency  time.Duration
	FlushMessages   int
	FlushBytes      int
	MaxMessageBytes int
}

// NewProducer creates a new Kafka producer with optimized settings
func NewProducer(config ProducerConfig, logger logger.Logger, metrics ProducerMetrics) (*Producer, error) {
	// Configure Sarama
	saramaConfig := sarama.NewConfig()

	// Producer settings for high throughput
	saramaConfig.Producer.RequiredAcks = sarama.RequiredAcks(config.RequiredAcks)
	saramaConfig.Producer.Retry.Max = config.RetryMax
	saramaConfig.Producer.Timeout = config.Timeout
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true

	// Compression for better network utilization
	switch config.CompressionType {
	case "gzip":
		saramaConfig.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		saramaConfig.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaConfig.Producer.Compression = sarama.CompressionZSTD
	default:
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	}

	// Batching settings for performance
	saramaConfig.Producer.Flush.Frequency = config.FlushFrequency
	saramaConfig.Producer.Flush.Messages = config.FlushMessages
	saramaConfig.Producer.Flush.Bytes = config.FlushBytes
	saramaConfig.Producer.MaxMessageBytes = config.MaxMessageBytes

	// Network settings
	saramaConfig.Net.DialTimeout = 30 * time.Second
	saramaConfig.Net.ReadTimeout = 30 * time.Second
	saramaConfig.Net.WriteTimeout = 30 * time.Second

	// Metadata settings
	saramaConfig.Metadata.RefreshFrequency = 10 * time.Minute
	saramaConfig.Metadata.Full = true

	// Version
	saramaConfig.Version = sarama.V3_5_0_0

	// Create async producer
	producer, err := sarama.NewAsyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Producer{
		producer:  producer,
		topic:     config.Topic,
		logger:    logger,
		metrics:   metrics,
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	logger.Info("Kafka producer created successfully",
		zap.Strings("brokers", config.Brokers),
		zap.String("topic", config.Topic),
		zap.String("compression", config.CompressionType),
	)

	return p, nil
}

// Start begins producing messages
func (p *Producer) Start(readings <-chan []*simulator.SensorReading) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return fmt.Errorf("producer is already running")
	}

	p.logger.Info("Starting Kafka producer")

	// Start success/error handlers
	p.wg.Add(2)
	go p.handleSuccesses()
	go p.handleErrors()

	// Start main producer loop
	p.wg.Add(1)
	go p.produceMessages(readings)

	p.running = true
	p.metrics.SetKafkaConnections(1)

	p.logger.Info("Kafka producer started successfully")
	return nil
}

// Stop gracefully stops the producer
func (p *Producer) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return fmt.Errorf("producer is not running")
	}

	p.logger.Info("Stopping Kafka producer...")

	// Cancel context
	p.cancel()

	// Close producer
	if err := p.producer.Close(); err != nil {
		p.logger.Error("Error closing Kafka producer", logger.Error(err))
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Kafka producer stopped gracefully")
	case <-time.After(30 * time.Second):
		p.logger.Warn("Timeout waiting for producer to stop")
	}

	p.running = false
	p.metrics.SetKafkaConnections(0)

	return nil
}

// produceMessages main loop for producing messages to Kafka
func (p *Producer) produceMessages(readings <-chan []*simulator.SensorReading) {
	defer p.wg.Done()

	p.logger.Debug("Producer message loop started")

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("Producer message loop stopping")
			return

		case batch := <-readings:
			if batch == nil {
				continue
			}

			startTime := time.Now()

			// Send each reading in the batch
			for _, reading := range batch {
				if err := p.sendReading(reading); err != nil {
					p.logger.Error("Failed to send reading",
						zap.Error(err),
						zap.String("device_id", reading.DeviceID),
					)
					continue
				}
			}

			// Record latency
			latency := time.Since(startTime)
			p.metrics.RecordProducerLatency(latency)

			p.logger.Debug("Batch sent to Kafka",
				zap.Int("batch_size", len(batch)),
				zap.Duration("latency", latency),
			)
		}
	}
}

// sendReading sends a single sensor reading to Kafka
func (p *Producer) sendReading(reading *simulator.SensorReading) error {
	// Serialize reading to JSON
	data, err := json.Marshal(reading)
	if err != nil {
		return fmt.Errorf("failed to marshal reading: %w", err)
	}

	// Create Kafka message
	message := &sarama.ProducerMessage{
		Topic:     p.topic,
		Key:       sarama.StringEncoder(reading.DeviceID), // Partition by device ID
		Value:     sarama.ByteEncoder(data),
		Timestamp: reading.Timestamp,
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("device_type"),
				Value: []byte("sensor"),
			},
			{
				Key:   []byte("version"),
				Value: []byte("1.0"),
			},
		},
	}

	// Send message asynchronously
	select {
	case p.producer.Input() <- message:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("producer context cancelled")
	default:
		return fmt.Errorf("producer input channel full")
	}
}

// handleSuccesses handles successful message deliveries
func (p *Producer) handleSuccesses() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return

		case success := <-p.producer.Successes():
			if success != nil {
				p.successCount++
				p.metrics.IncrementMessagesSent(1)

				p.logger.Debug("Message sent successfully",
					zap.String("topic", success.Topic),
					zap.Int32("partition", success.Partition),
					zap.Int64("offset", success.Offset),
				)
			}
		}
	}
}

// handleErrors handles message delivery errors
func (p *Producer) handleErrors() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return

		case err := <-p.producer.Errors():
			if err != nil {
				p.errorCount++
				p.metrics.IncrementMessagesFailed(1)
				p.metrics.IncrementKafkaErrors()

				p.logger.Error("Failed to send message to Kafka",
					zap.Error(err.Err),
					zap.String("topic", err.Msg.Topic),
					zap.Any("key", err.Msg.Key),
				)
			}
		}
	}
}

// GetStats returns producer statistics
func (p *Producer) GetStats() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	uptime := time.Since(p.startTime)
	successRate := float64(p.successCount) / float64(p.successCount+p.errorCount) * 100

	return map[string]interface{}{
		"success_count":    p.successCount,
		"error_count":      p.errorCount,
		"success_rate_pct": successRate,
		"uptime_seconds":   uptime.Seconds(),
		"is_running":       p.running,
	}
}

// IsRunning returns whether the producer is currently running
func (p *Producer) IsRunning() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.running
}
