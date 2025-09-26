package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/clickhouse"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/grpc"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/redis"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/pkg/logger"
)

// SensorMessage represents a sensor message from Kafka
type SensorMessage struct {
	DeviceID    string    `json:"device_id"`
	Timestamp   time.Time `json:"timestamp"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Pressure    float64   `json:"pressure"`
	Location    Location  `json:"location"`
	Status      string    `json:"status"`
}

// Location represents geographical coordinates
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Region    string  `json:"region"`
}

// Consumer handles Kafka message consumption with high performance
type Consumer struct {
	consumerGroup    sarama.ConsumerGroup
	topic            string
	groupID          string
	config           Config
	logger           logger.Logger
	metrics          MetricsRecorder
	clickhouseClient *clickhouse.Client
	redisClient      *redis.Client
	alertClient      *grpc.AlertClient

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	mutex   sync.RWMutex

	// Processing channels
	messages      chan *sarama.ConsumerMessage
	processedData chan *ProcessedMessage

	// Performance tracking
	consumedCount  int64
	processedCount int64
	errorCount     int64
	startTime      time.Time
	lastCommitTime time.Time
}

// ProcessedMessage represents a processed sensor message
type ProcessedMessage struct {
	Original       *sarama.ConsumerMessage
	SensorData     *SensorMessage
	ClickHouseData *clickhouse.SensorReading
	RedisSnapshot  *redis.DeviceSnapshot
	AlertRequired  bool
	AlertRequest   *grpc.AlertRequest
	ProcessingTime time.Duration
}

// Config holds consumer configuration
type Config struct {
	Brokers           []string
	Topic             string
	GroupID           string
	AutoCommit        bool
	CommitInterval    time.Duration
	SessionTimeout    time.Duration
	HeartbeatInterval time.Duration
	FetchMin          int32
	FetchDefault      int32
	FetchMax          int32
	RetryBackoff      time.Duration
	WorkerCount       int
	BufferSize        int
	ProcessingTimeout time.Duration
	AlertThreshold    float64
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementMessagesConsumed(count int)
	IncrementMessagesProcessed(count int)
	IncrementMessagesFailed(count int)
	SetKafkaLag(lag int64)
	RecordKafkaCommitLatency(duration time.Duration)
	SetKafkaPartitionCount(count int)
	SetKafkaOffset(partition string, offset int64)
	SetProcessingRate(rate float64)
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(
	config Config,
	logger logger.Logger,
	metrics MetricsRecorder,
	clickhouseClient *clickhouse.Client,
	redisClient *redis.Client,
	alertClient *grpc.AlertClient,
) (*Consumer, error) {
	// Configure Sarama
	saramaConfig := sarama.NewConfig()

	// Consumer settings for high throughput
	saramaConfig.Consumer.Group.Session.Timeout = config.SessionTimeout
	saramaConfig.Consumer.Group.Heartbeat.Interval = config.HeartbeatInterval
	saramaConfig.Consumer.Fetch.Min = config.FetchMin
	saramaConfig.Consumer.Fetch.Default = config.FetchDefault
	saramaConfig.Consumer.Fetch.Max = config.FetchMax
	saramaConfig.Consumer.MaxWaitTime = 250 * time.Millisecond
	saramaConfig.Consumer.Return.Errors = true

	// Offset management
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest

	// Enable auto-commit or manual commit
	if config.AutoCommit {
		saramaConfig.Consumer.Offsets.AutoCommit.Enable = true
		saramaConfig.Consumer.Offsets.AutoCommit.Interval = config.CommitInterval
	} else {
		saramaConfig.Consumer.Offsets.AutoCommit.Enable = false
	}

	// Network settings
	saramaConfig.Net.DialTimeout = 30 * time.Second
	saramaConfig.Net.ReadTimeout = 30 * time.Second
	saramaConfig.Net.WriteTimeout = 30 * time.Second

	// Version
	saramaConfig.Version = sarama.V3_5_0_0

	// Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	consumer := &Consumer{
		consumerGroup:    consumerGroup,
		topic:            config.Topic,
		groupID:          config.GroupID,
		config:           config,
		logger:           logger,
		metrics:          metrics,
		clickhouseClient: clickhouseClient,
		redisClient:      redisClient,
		alertClient:      alertClient,
		ctx:              ctx,
		cancel:           cancel,
		messages:         make(chan *sarama.ConsumerMessage, config.BufferSize),
		processedData:    make(chan *ProcessedMessage, config.BufferSize),
		startTime:        time.Now(),
		lastCommitTime:   time.Now(),
	}

	logger.Info("Kafka consumer created successfully",
		zap.Strings("brokers", config.Brokers),
		zap.String("topic", config.Topic),
		zap.String("group_id", config.GroupID),
		zap.Int("worker_count", config.WorkerCount),
	)

	return consumer, nil
}

// Start begins consuming messages
func (c *Consumer) Start() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		return fmt.Errorf("consumer is already running")
	}

	c.logger.Info("Starting Kafka consumer")

	// Start worker goroutines for message processing
	for i := 0; i < c.config.WorkerCount; i++ {
		c.wg.Add(1)
		go c.messageProcessor(i)
	}

	// Start data persistence worker
	c.wg.Add(1)
	go c.dataPersister()

	// Start consumer group session
	c.wg.Add(1)
	go c.consumeMessages()

	// Start metrics updater
	c.wg.Add(1)
	go c.metricsUpdater()

	c.running = true
	c.logger.Info("Kafka consumer started successfully")

	return nil
}

// Stop gracefully stops the consumer
func (c *Consumer) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		return fmt.Errorf("consumer is not running")
	}

	c.logger.Info("Stopping Kafka consumer...")

	// Cancel context
	c.cancel()

	// Close consumer group
	if err := c.consumerGroup.Close(); err != nil {
		c.logger.Error("Error closing consumer group", zap.Error(err))
	}

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("All workers stopped gracefully")
	case <-time.After(30 * time.Second):
		c.logger.Warn("Timeout waiting for workers to stop")
	}

	// Close channels
	close(c.messages)
	close(c.processedData)

	c.running = false
	c.logger.Info("Kafka consumer stopped")

	return nil
}

// consumeMessages main consumer loop
func (c *Consumer) consumeMessages() {
	defer c.wg.Done()

	handler := &consumerGroupHandler{
		consumer: c,
		logger:   c.logger,
	}

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Consumer group session stopping")
			return

		default:
			// Start consuming
			if err := c.consumerGroup.Consume(c.ctx, []string{c.topic}, handler); err != nil {
				c.logger.Error("Error consuming messages", zap.Error(err))
				c.metrics.IncrementMessagesFailed(1)

				// Wait before retrying
				select {
				case <-time.After(c.config.RetryBackoff):
				case <-c.ctx.Done():
					return
				}
			}
		}
	}
}

// messageProcessor processes individual messages
func (c *Consumer) messageProcessor(workerID int) {
	defer c.wg.Done()

	c.logger.Debug("Message processor started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Message processor stopping", zap.Int("worker_id", workerID))
			return

		case msg := <-c.messages:
			if msg == nil {
				continue
			}

			startTime := time.Now()
			processed := c.processMessage(msg)
			processed.ProcessingTime = time.Since(startTime)

			// Send to data persister
			select {
			case c.processedData <- processed:
				c.processedCount++
				c.metrics.IncrementMessagesProcessed(1)
			case <-c.ctx.Done():
				return
			default:
				c.logger.Warn("Processed data channel full, dropping message",
					zap.String("device_id", processed.SensorData.DeviceID),
				)
				c.metrics.IncrementMessagesFailed(1)
			}
		}
	}
}

// processMessage processes a single Kafka message
func (c *Consumer) processMessage(msg *sarama.ConsumerMessage) *ProcessedMessage {
	processed := &ProcessedMessage{
		Original: msg,
	}

	// Parse sensor data
	var sensorData SensorMessage
	if err := json.Unmarshal(msg.Value, &sensorData); err != nil {
		c.logger.Error("Failed to unmarshal sensor data",
			zap.Error(err),
			zap.String("topic", msg.Topic),
			zap.Int32("partition", msg.Partition),
			zap.Int64("offset", msg.Offset),
		)
		return processed
	}

	processed.SensorData = &sensorData

	// Convert to ClickHouse format
	processed.ClickHouseData = &clickhouse.SensorReading{
		DeviceID:    sensorData.DeviceID,
		Timestamp:   sensorData.Timestamp,
		Temperature: sensorData.Temperature,
		Humidity:    sensorData.Humidity,
		Pressure:    sensorData.Pressure,
		LocationLat: sensorData.Location.Latitude,
		LocationLng: sensorData.Location.Longitude,
		Status:      sensorData.Status,
		CreatedAt:   time.Now(),
	}

	// Create Redis snapshot
	processed.RedisSnapshot = &redis.DeviceSnapshot{
		DeviceID:    sensorData.DeviceID,
		LastUpdate:  sensorData.Timestamp,
		Temperature: sensorData.Temperature,
		Humidity:    sensorData.Humidity,
		Pressure:    sensorData.Pressure,
		LocationLat: sensorData.Location.Latitude,
		LocationLng: sensorData.Location.Longitude,
		Status:      sensorData.Status,
	}

	// Check if alert is required
	if sensorData.Temperature > c.config.AlertThreshold {
		processed.AlertRequired = true
		processed.AlertRequest = &grpc.AlertRequest{
			DeviceID:    sensorData.DeviceID,
			Temperature: sensorData.Temperature,
			Threshold:   c.config.AlertThreshold,
			Message:     fmt.Sprintf("High temperature detected: %.2f°C", sensorData.Temperature),
			Timestamp:   sensorData.Timestamp,
			Severity:    "HIGH",
		}
	}

	return processed
}

// dataPersister handles data persistence to ClickHouse and Redis
func (c *Consumer) dataPersister() {
	defer c.wg.Done()

	c.logger.Debug("Data persister started")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Debug("Data persister stopping")
			return

		case processed := <-c.processedData:
			if processed == nil || processed.SensorData == nil {
				continue
			}

			// Store in ClickHouse
			if processed.ClickHouseData != nil {
				if err := c.clickhouseClient.InsertReading(processed.ClickHouseData); err != nil {
					c.logger.Error("Failed to insert to ClickHouse",
						zap.Error(err),
						zap.String("device_id", processed.SensorData.DeviceID),
					)
					c.metrics.IncrementMessagesFailed(1)
					continue
				}
			}

			// Cache in Redis
			if processed.RedisSnapshot != nil {
				if err := c.redisClient.SetDeviceSnapshot(processed.RedisSnapshot); err != nil {
					c.logger.Error("Failed to cache snapshot",
						zap.Error(err),
						zap.String("device_id", processed.SensorData.DeviceID),
					)
					// Don't fail the entire process for cache errors
				}
			}

			// Send alert if required
			if processed.AlertRequired && processed.AlertRequest != nil {
				go func(alertReq *grpc.AlertRequest) {
					if _, err := c.alertClient.SendAlert(alertReq); err != nil {
						c.logger.Error("Failed to send alert",
							zap.Error(err),
							zap.String("device_id", alertReq.DeviceID),
							zap.Float64("temperature", alertReq.Temperature),
						)
					}
				}(processed.AlertRequest)
			}

			c.logger.Debug("Message processed successfully",
				zap.String("device_id", processed.SensorData.DeviceID),
				zap.Float64("temperature", processed.SensorData.Temperature),
				zap.Duration("processing_time", processed.ProcessingTime),
				zap.Bool("alert_sent", processed.AlertRequired),
			)
		}
	}
}

// metricsUpdater periodically updates metrics
func (c *Consumer) metricsUpdater() {
	defer c.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case <-ticker.C:
			// Calculate processing rate
			uptime := time.Since(c.startTime)
			if uptime > 0 {
				rate := float64(c.processedCount) / uptime.Seconds()
				c.metrics.SetProcessingRate(rate)
			}

			c.logger.Debug("Updated consumer metrics",
				zap.Int64("consumed", c.consumedCount),
				zap.Int64("processed", c.processedCount),
				zap.Int64("errors", c.errorCount),
				zap.Duration("uptime", uptime),
			)
		}
	}
}

// GetStats returns consumer statistics
func (c *Consumer) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	uptime := time.Since(c.startTime)
	processingRate := float64(c.processedCount) / uptime.Seconds()
	successRate := float64(c.processedCount) / float64(c.consumedCount) * 100

	return map[string]interface{}{
		"consumed_count":   c.consumedCount,
		"processed_count":  c.processedCount,
		"error_count":      c.errorCount,
		"processing_rate":  processingRate,
		"success_rate_pct": successRate,
		"uptime_seconds":   uptime.Seconds(),
		"is_running":       c.running,
		"last_commit_time": c.lastCommitTime,
	}
}

// IsRunning returns whether the consumer is currently running
func (c *Consumer) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.running
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	consumer *Consumer
	logger   logger.Logger
}

// Setup is run at the beginning of a new session
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Consumer group session started")
	return nil
}

// Cleanup is run at the end of a session
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Consumer group session ended")
	return nil
}

// ConsumeClaim processes messages from a partition
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	h.logger.Info("Started consuming partition",
		zap.String("topic", claim.Topic()),
		zap.Int32("partition", claim.Partition()),
		zap.Int64("initial_offset", claim.InitialOffset()),
	)

	// Update partition metrics
	h.consumer.metrics.SetKafkaOffset(
		fmt.Sprintf("%d", claim.Partition()),
		claim.InitialOffset(),
	)

	for {
		select {
		case <-session.Context().Done():
			h.logger.Debug("Consumer claim context cancelled")
			return nil

		case message := <-claim.Messages():
			if message == nil {
				continue
			}

			// Update metrics
			h.consumer.consumedCount++
			h.consumer.metrics.IncrementMessagesConsumed(1)
			h.consumer.metrics.SetKafkaOffset(
				fmt.Sprintf("%d", message.Partition),
				message.Offset,
			)

			// Send to processing channel
			select {
			case h.consumer.messages <- message:
				// Message sent for processing
			case <-session.Context().Done():
				return nil
			default:
				// Channel full, drop message
				h.logger.Warn("Message channel full, dropping message",
					zap.String("topic", message.Topic),
					zap.Int32("partition", message.Partition),
					zap.Int64("offset", message.Offset),
				)
				h.consumer.metrics.IncrementMessagesFailed(1)
			}

			// Manual commit (if auto-commit is disabled)
			if !h.consumer.config.AutoCommit {
				session.MarkMessage(message, "")

				// Commit periodically
				if time.Since(h.consumer.lastCommitTime) >= h.consumer.config.CommitInterval {
					startTime := time.Now()
					session.Commit()
					commitLatency := time.Since(startTime)

					h.consumer.metrics.RecordKafkaCommitLatency(commitLatency)
					h.consumer.lastCommitTime = time.Now()

					h.logger.Debug("Committed offset",
						zap.Int32("partition", message.Partition),
						zap.Int64("offset", message.Offset),
						zap.Duration("commit_latency", commitLatency),
					)
				}
			}
		}
	}
}
