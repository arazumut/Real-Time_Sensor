package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/real-time-analytics-service/pkg/logger"
)

// AnalyticsMessage represents an analytics result message for MQTT
type AnalyticsMessage struct {
	DeviceID    string                 `json:"device_id"`
	Region      string                 `json:"region"`
	MessageType string                 `json:"message_type"` // "trend", "delta", "regional", "anomaly"
	Timestamp   time.Time              `json:"timestamp"`
	Data        map[string]interface{} `json:"data"`
	Metadata    map[string]string      `json:"metadata"`
}

// Publisher handles MQTT message publishing with QoS 1
type Publisher struct {
	client  mqtt.Client
	config  Config
	logger  logger.Logger
	metrics MetricsRecorder
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	mutex   sync.RWMutex

	// Message queue for reliable delivery
	messageQueue chan *AnalyticsMessage

	// Performance tracking
	publishedCount int64
	failedCount    int64
	reconnectCount int64
	startTime      time.Time
}

// Config holds MQTT publisher configuration
type Config struct {
	Broker               string
	ClientID             string
	Username             string
	Password             string
	QOS                  byte
	Retained             bool
	ConnectTimeout       time.Duration
	WriteTimeout         time.Duration
	KeepAlive            time.Duration
	PingTimeout          time.Duration
	ConnectRetryInterval time.Duration
	AutoReconnect        bool
	MaxReconnectInterval time.Duration
	TopicPrefix          string
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementMQTTPublished(count int)
	IncrementMQTTFailed(count int)
	RecordMQTTLatency(duration time.Duration)
	SetMQTTConnectionStatus(connected bool)
	IncrementMQTTReconnect()
}

// NewPublisher creates a new MQTT publisher
func NewPublisher(config Config, logger logger.Logger, metrics MetricsRecorder) (*Publisher, error) {
	// Configure MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.Broker)
	opts.SetClientID(config.ClientID)
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetKeepAlive(config.KeepAlive)
	opts.SetPingTimeout(config.PingTimeout)
	opts.SetConnectTimeout(config.ConnectTimeout)
	opts.SetWriteTimeout(config.WriteTimeout)
	opts.SetAutoReconnect(config.AutoReconnect)
	opts.SetMaxReconnectInterval(config.MaxReconnectInterval)
	opts.SetConnectRetryInterval(config.ConnectRetryInterval)

	ctx, cancel := context.WithCancel(context.Background())

	publisher := &Publisher{
		config:       config,
		logger:       logger,
		metrics:      metrics,
		ctx:          ctx,
		cancel:       cancel,
		messageQueue: make(chan *AnalyticsMessage, 1000),
		startTime:    time.Now(),
	}

	// Set connection callbacks
	opts.SetConnectionLostHandler(publisher.onConnectionLost)
	opts.SetReconnectingHandler(publisher.onReconnecting)
	opts.SetOnConnectHandler(publisher.onConnect)

	// Create MQTT client
	client := mqtt.NewClient(opts)
	publisher.client = client

	logger.Info("MQTT publisher created successfully",
		zap.String("broker", config.Broker),
		zap.String("client_id", config.ClientID),
		zap.Uint8("qos", config.QOS),
		zap.String("topic_prefix", config.TopicPrefix),
	)

	return publisher, nil
}

// Start connects to MQTT broker and begins publishing
func (p *Publisher) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return fmt.Errorf("MQTT publisher is already running")
	}

	p.logger.Info("Starting MQTT publisher")

	// Connect to MQTT broker
	if token := p.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	// Start message publisher worker
	p.wg.Add(1)
	go p.messagePublisher()

	p.running = true
	p.metrics.SetMQTTConnectionStatus(true)

	p.logger.Info("MQTT publisher started successfully")
	return nil
}

// Stop gracefully stops the publisher
func (p *Publisher) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return fmt.Errorf("MQTT publisher is not running")
	}

	p.logger.Info("Stopping MQTT publisher...")

	// Cancel context
	p.cancel()

	// Wait for message publisher to finish
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Message publisher stopped gracefully")
	case <-time.After(30 * time.Second):
		p.logger.Warn("Timeout waiting for message publisher to stop")
	}

	// Disconnect from MQTT broker
	p.client.Disconnect(250) // 250ms timeout

	// Close message queue
	close(p.messageQueue)

	p.running = false
	p.metrics.SetMQTTConnectionStatus(false)

	p.logger.Info("MQTT publisher stopped")
	return nil
}

// PublishAnalytics publishes analytics results to MQTT
func (p *Publisher) PublishAnalytics(message *AnalyticsMessage) error {
	if !p.IsRunning() {
		return fmt.Errorf("publisher is not running")
	}

	select {
	case p.messageQueue <- message:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("publisher context cancelled")
	default:
		p.logger.Warn("Message queue full, dropping analytics message",
			zap.String("device_id", message.DeviceID),
			zap.String("message_type", message.MessageType),
		)
		p.metrics.IncrementMQTTFailed(1)
		return fmt.Errorf("message queue full")
	}
}

// messagePublisher main publishing loop
func (p *Publisher) messagePublisher() {
	defer p.wg.Done()

	p.logger.Debug("MQTT message publisher started")

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("MQTT message publisher stopping")
			return

		case message := <-p.messageQueue:
			if message == nil {
				continue
			}

			if err := p.publishMessage(message); err != nil {
				p.logger.Error("Failed to publish MQTT message",
					zap.Error(err),
					zap.String("device_id", message.DeviceID),
					zap.String("message_type", message.MessageType),
				)
				p.failedCount++
				p.metrics.IncrementMQTTFailed(1)
			} else {
				p.publishedCount++
				p.metrics.IncrementMQTTPublished(1)
			}
		}
	}
}

// publishMessage publishes a single message to MQTT
func (p *Publisher) publishMessage(message *AnalyticsMessage) error {
	startTime := time.Now()

	// Serialize message to JSON
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Generate topic based on message type and device
	topic := p.generateTopic(message)

	// Publish with QoS 1 for reliable delivery
	token := p.client.Publish(topic, p.config.QOS, p.config.Retained, payload)

	// Wait for publish confirmation
	if !token.WaitTimeout(p.config.WriteTimeout) {
		return fmt.Errorf("publish timeout")
	}

	if token.Error() != nil {
		return fmt.Errorf("publish error: %w", token.Error())
	}

	// Record metrics
	latency := time.Since(startTime)
	p.metrics.RecordMQTTLatency(latency)

	p.logger.Debug("MQTT message published successfully",
		zap.String("topic", topic),
		zap.String("device_id", message.DeviceID),
		zap.String("message_type", message.MessageType),
		zap.Duration("latency", latency),
		zap.Int("payload_size", len(payload)),
	)

	return nil
}

// generateTopic generates MQTT topic based on message content
func (p *Publisher) generateTopic(message *AnalyticsMessage) string {
	switch message.MessageType {
	case "trend":
		return fmt.Sprintf("%s/device/%s/trend", p.config.TopicPrefix, message.DeviceID)
	case "delta":
		return fmt.Sprintf("%s/device/%s/delta", p.config.TopicPrefix, message.DeviceID)
	case "regional":
		return fmt.Sprintf("%s/region/%s/summary", p.config.TopicPrefix, message.Region)
	case "anomaly":
		return fmt.Sprintf("%s/device/%s/anomaly", p.config.TopicPrefix, message.DeviceID)
	default:
		return fmt.Sprintf("%s/device/%s/general", p.config.TopicPrefix, message.DeviceID)
	}
}

// Connection event handlers
func (p *Publisher) onConnect(client mqtt.Client) {
	p.logger.Info("MQTT client connected successfully")
	p.metrics.SetMQTTConnectionStatus(true)
}

func (p *Publisher) onConnectionLost(client mqtt.Client, err error) {
	p.logger.Warn("MQTT connection lost", zap.Error(err))
	p.metrics.SetMQTTConnectionStatus(false)
}

func (p *Publisher) onReconnecting(client mqtt.Client, opts *mqtt.ClientOptions) {
	p.logger.Info("MQTT client reconnecting...",
		zap.String("broker", opts.Servers[0].String()),
	)
	p.reconnectCount++
	p.metrics.IncrementMQTTReconnect()
}

// HealthCheck checks MQTT connection health
func (p *Publisher) HealthCheck() error {
	if !p.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	// Send a test ping
	testTopic := fmt.Sprintf("%s/health/ping", p.config.TopicPrefix)
	testPayload := fmt.Sprintf(`{"timestamp":"%s","service":"analytics"}`, time.Now().Format(time.RFC3339))

	token := p.client.Publish(testTopic, 0, false, testPayload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("MQTT health check timeout")
	}

	if token.Error() != nil {
		return fmt.Errorf("MQTT health check failed: %w", token.Error())
	}

	return nil
}

// GetStats returns publisher statistics
func (p *Publisher) GetStats() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	uptime := time.Since(p.startTime)
	publishRate := float64(p.publishedCount) / uptime.Seconds()
	successRate := float64(p.publishedCount) / float64(p.publishedCount+p.failedCount) * 100

	return map[string]interface{}{
		"published_count":  p.publishedCount,
		"failed_count":     p.failedCount,
		"reconnect_count":  p.reconnectCount,
		"publish_rate":     publishRate,
		"success_rate_pct": successRate,
		"is_connected":     p.client.IsConnected(),
		"is_running":       p.running,
		"uptime_seconds":   uptime.Seconds(),
	}
}

// IsRunning returns whether the publisher is currently running
func (p *Publisher) IsRunning() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return p.running
}

// IsConnected returns whether MQTT client is connected
func (p *Publisher) IsConnected() bool {
	return p.client.IsConnected()
}
