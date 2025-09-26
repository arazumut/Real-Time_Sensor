package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/real-time-analytics-service/internal/mqtt"
	"github.com/twinup/sensor-system/services/real-time-analytics-service/pkg/algorithms"
	"github.com/twinup/sensor-system/services/real-time-analytics-service/pkg/logger"
)

// SensorData represents incoming sensor data
type SensorData struct {
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

// Engine handles real-time analytics processing
type Engine struct {
	consumerGroup sarama.ConsumerGroup
	mqttPublisher *mqtt.Publisher
	timeWindow    *algorithms.TimeSeriesWindow
	config        Config
	logger        logger.Logger
	metrics       MetricsRecorder

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	mutex   sync.RWMutex

	// Processing channels
	sensorData     chan *SensorData
	analyticsQueue chan *AnalyticsTask

	// Performance tracking
	processedCount int64
	errorCount     int64
	startTime      time.Time

	// Analytics state
	lastRegionalAnalysis map[string]time.Time
	deviceLastValues     map[string]*SensorData
	regionalMutex        sync.RWMutex
}

// AnalyticsTask represents an analytics processing task
type AnalyticsTask struct {
	Type      string      `json:"type"` // "trend", "delta", "regional", "anomaly"
	DeviceID  string      `json:"device_id,omitempty"`
	Region    string      `json:"region,omitempty"`
	Data      *SensorData `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Config holds analytics engine configuration
type Config struct {
	// Kafka configuration
	Brokers           []string
	Topic             string
	GroupID           string
	AutoCommit        bool
	CommitInterval    time.Duration
	SessionTimeout    time.Duration
	HeartbeatInterval time.Duration

	// Analytics configuration
	WorkerCount      int
	BufferSize       int
	WindowSize       time.Duration
	TrendWindow      time.Duration
	DeltaThreshold   float64
	RegionalAnalysis bool
	AnomalyDetection bool
	PublishInterval  time.Duration
	MaxMemoryWindow  int
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementMessagesConsumed(count int)
	IncrementAnalysisProcessed(count int)
	IncrementAnalysisFailed(count int)
	RecordAnalysisLatency(duration time.Duration)
	RecordTrendCalculationTime(duration time.Duration)
	RecordDeltaCalculationTime(duration time.Duration)
	RecordRegionalAnalysisTime(duration time.Duration)
	RecordAnomalyDetectionTime(duration time.Duration)
	SetActiveDevices(count int)
	SetDataPointsInWindow(count int)
	SetWindowMemoryUsage(bytes int64)
	SetTrendDirection(deviceID, region string, direction float64)
	SetTrendStrength(deviceID, region string, strength float64)
	SetDeltaValue(deviceID, metricType string, delta float64)
	IncrementAnomalies()
	SetRegionalAverage(region, metricType string, average float64)
	SetRegionalDeviceCount(region string, count int)
	IncrementRegionalAnomalies(region, anomalyType string)
	UpdateSystemMetrics(memoryBytes, cpuPercent float64, goroutines int)
	SetProcessingRate(rate float64)
}

// NewEngine creates a new analytics engine
func NewEngine(config Config, mqttPublisher *mqtt.Publisher, logger logger.Logger, metrics MetricsRecorder) (*Engine, error) {
	// Configure Kafka consumer
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Group.Session.Timeout = config.SessionTimeout
	saramaConfig.Consumer.Group.Heartbeat.Interval = config.HeartbeatInterval
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest

	if config.AutoCommit {
		saramaConfig.Consumer.Offsets.AutoCommit.Enable = true
		saramaConfig.Consumer.Offsets.AutoCommit.Interval = config.CommitInterval
	}

	saramaConfig.Version = sarama.V3_5_0_0

	// Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Create time series window
	timeWindow := algorithms.NewTimeSeriesWindow(config.WindowSize, config.MaxMemoryWindow)

	ctx, cancel := context.WithCancel(context.Background())

	engine := &Engine{
		consumerGroup:        consumerGroup,
		mqttPublisher:        mqttPublisher,
		timeWindow:           timeWindow,
		config:               config,
		logger:               logger,
		metrics:              metrics,
		ctx:                  ctx,
		cancel:               cancel,
		sensorData:           make(chan *SensorData, config.BufferSize),
		analyticsQueue:       make(chan *AnalyticsTask, config.BufferSize),
		lastRegionalAnalysis: make(map[string]time.Time),
		deviceLastValues:     make(map[string]*SensorData),
		startTime:            time.Now(),
	}

	logger.Info("Analytics engine created successfully",
		zap.Strings("brokers", config.Brokers),
		zap.String("topic", config.Topic),
		zap.String("group_id", config.GroupID),
		zap.Duration("window_size", config.WindowSize),
		zap.Int("worker_count", config.WorkerCount),
	)

	return engine, nil
}

// Start begins the analytics processing
func (e *Engine) Start() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.running {
		return fmt.Errorf("analytics engine is already running")
	}

	e.logger.Info("Starting analytics engine")

	// Start analytics workers
	for i := 0; i < e.config.WorkerCount; i++ {
		e.wg.Add(1)
		go e.analyticsWorker(i)
	}

	// Start data processor
	e.wg.Add(1)
	go e.dataProcessor()

	// Start periodic analytics
	e.wg.Add(1)
	go e.periodicAnalytics()

	// Start Kafka consumer
	e.wg.Add(1)
	go e.consumeMessages()

	// Start metrics updater
	e.wg.Add(1)
	go e.metricsUpdater()

	e.running = true
	e.logger.Info("Analytics engine started successfully")

	return nil
}

// Stop gracefully stops the analytics engine
func (e *Engine) Stop() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if !e.running {
		return fmt.Errorf("analytics engine is not running")
	}

	e.logger.Info("Stopping analytics engine...")

	// Cancel context
	e.cancel()

	// Close consumer group
	if err := e.consumerGroup.Close(); err != nil {
		e.logger.Error("Error closing consumer group", zap.Error(err))
	}

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		e.logger.Info("All workers stopped gracefully")
	case <-time.After(30 * time.Second):
		e.logger.Warn("Timeout waiting for workers to stop")
	}

	// Close channels
	close(e.sensorData)
	close(e.analyticsQueue)

	e.running = false
	e.logger.Info("Analytics engine stopped")

	return nil
}

// consumeMessages consumes messages from Kafka
func (e *Engine) consumeMessages() {
	defer e.wg.Done()

	handler := &consumerHandler{
		engine: e,
		logger: e.logger,
	}

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Debug("Kafka consumer stopping")
			return

		default:
			if err := e.consumerGroup.Consume(e.ctx, []string{e.config.Topic}, handler); err != nil {
				e.logger.Error("Error consuming messages", zap.Error(err))

				select {
				case <-time.After(5 * time.Second):
				case <-e.ctx.Done():
					return
				}
			}
		}
	}
}

// dataProcessor processes incoming sensor data
func (e *Engine) dataProcessor() {
	defer e.wg.Done()

	e.logger.Debug("Data processor started")

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Debug("Data processor stopping")
			return

		case data := <-e.sensorData:
			if data == nil {
				continue
			}

			e.processSensorData(data)
		}
	}
}

// processSensorData processes a single sensor data point
func (e *Engine) processSensorData(data *SensorData) {
	startTime := time.Now()

	// Convert to DataPoint for algorithms
	dataPoint := &algorithms.DataPoint{
		DeviceID:    data.DeviceID,
		Timestamp:   data.Timestamp,
		Temperature: data.Temperature,
		Humidity:    data.Humidity,
		Pressure:    data.Pressure,
		LocationLat: data.Location.Latitude,
		LocationLng: data.Location.Longitude,
		Region:      data.Location.Region,
		Status:      data.Status,
	}

	// Add to time window
	e.timeWindow.AddDataPoint(dataPoint)

	// Queue analytics tasks
	e.queueAnalyticsTasks(data)

	// Update metrics
	processingTime := time.Since(startTime)
	e.processedCount++
	e.metrics.IncrementAnalysisProcessed(1)
	e.metrics.RecordAnalysisLatency(processingTime)

	e.logger.Debug("Sensor data processed",
		zap.String("device_id", data.DeviceID),
		zap.String("region", data.Location.Region),
		zap.Duration("processing_time", processingTime),
	)
}

// queueAnalyticsTasks queues various analytics tasks for processing
func (e *Engine) queueAnalyticsTasks(data *SensorData) {
	// Queue delta analysis
	e.queueTask(&AnalyticsTask{
		Type:      "delta",
		DeviceID:  data.DeviceID,
		Data:      data,
		Timestamp: time.Now(),
	})

	// Queue trend analysis (less frequent)
	if e.processedCount%10 == 0 { // Every 10th message
		e.queueTask(&AnalyticsTask{
			Type:      "trend",
			DeviceID:  data.DeviceID,
			Data:      data,
			Timestamp: time.Now(),
		})
	}

	// Queue anomaly detection
	if e.config.AnomalyDetection {
		e.queueTask(&AnalyticsTask{
			Type:      "anomaly",
			DeviceID:  data.DeviceID,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
}

// queueTask queues an analytics task for processing
func (e *Engine) queueTask(task *AnalyticsTask) {
	select {
	case e.analyticsQueue <- task:
		// Task queued successfully
	default:
		e.logger.Warn("Analytics queue full, dropping task",
			zap.String("task_type", task.Type),
			zap.String("device_id", task.DeviceID),
		)
		e.metrics.IncrementAnalysisFailed(1)
	}
}

// analyticsWorker processes analytics tasks
func (e *Engine) analyticsWorker(workerID int) {
	defer e.wg.Done()

	e.logger.Debug("Analytics worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Debug("Analytics worker stopping", zap.Int("worker_id", workerID))
			return

		case task := <-e.analyticsQueue:
			if task == nil {
				continue
			}

			e.processAnalyticsTask(task)
		}
	}
}

// processAnalyticsTask processes a single analytics task
func (e *Engine) processAnalyticsTask(task *AnalyticsTask) {
	switch task.Type {
	case "trend":
		e.processTrendAnalysis(task)
	case "delta":
		e.processDeltaAnalysis(task)
	case "regional":
		e.processRegionalAnalysis(task)
	case "anomaly":
		e.processAnomalyDetection(task)
	default:
		e.logger.Warn("Unknown analytics task type", zap.String("type", task.Type))
	}
}

// processTrendAnalysis calculates and publishes trend analysis
func (e *Engine) processTrendAnalysis(task *AnalyticsTask) {
	startTime := time.Now()

	// Get device data from window
	deviceData := e.timeWindow.GetDeviceData(task.DeviceID)
	if len(deviceData) < 5 { // Need minimum points for trend
		return
	}

	// Calculate trends for each metric
	metrics := []string{"temperature", "humidity", "pressure"}

	for _, metricType := range metrics {
		trend, err := algorithms.CalculateTrend(deviceData, metricType)
		if err != nil {
			e.logger.Error("Failed to calculate trend",
				zap.Error(err),
				zap.String("device_id", task.DeviceID),
				zap.String("metric_type", metricType),
			)
			continue
		}

		// Update metrics
		direction := 0.0
		switch trend.Direction {
		case "increasing":
			direction = 1.0
		case "decreasing":
			direction = -1.0
		case "stable":
			direction = 0.0
		}

		e.metrics.SetTrendDirection(task.DeviceID, task.Data.Location.Region, direction)
		e.metrics.SetTrendStrength(task.DeviceID, task.Data.Location.Region, trend.Strength)

		// Publish to MQTT
		e.publishTrendResult(trend)
	}

	processingTime := time.Since(startTime)
	e.metrics.RecordTrendCalculationTime(processingTime)
}

// processDeltaAnalysis calculates and publishes delta analysis
func (e *Engine) processDeltaAnalysis(task *AnalyticsTask) {
	startTime := time.Now()

	// Get previous value for this device
	e.regionalMutex.RLock()
	previousData, exists := e.deviceLastValues[task.DeviceID]
	e.regionalMutex.RUnlock()

	if !exists {
		// Store current as previous for next time
		e.regionalMutex.Lock()
		e.deviceLastValues[task.DeviceID] = task.Data
		e.regionalMutex.Unlock()
		return
	}

	// Convert to DataPoint format
	currentPoint := &algorithms.DataPoint{
		DeviceID:    task.Data.DeviceID,
		Timestamp:   task.Data.Timestamp,
		Temperature: task.Data.Temperature,
		Humidity:    task.Data.Humidity,
		Pressure:    task.Data.Pressure,
		Region:      task.Data.Location.Region,
	}

	previousPoint := &algorithms.DataPoint{
		DeviceID:    previousData.DeviceID,
		Timestamp:   previousData.Timestamp,
		Temperature: previousData.Temperature,
		Humidity:    previousData.Humidity,
		Pressure:    previousData.Pressure,
		Region:      previousData.Location.Region,
	}

	// Calculate deltas for each metric
	metrics := []string{"temperature", "humidity", "pressure"}

	for _, metricType := range metrics {
		delta, err := algorithms.CalculateDelta(currentPoint, previousPoint, metricType, e.config.DeltaThreshold)
		if err != nil {
			e.logger.Error("Failed to calculate delta",
				zap.Error(err),
				zap.String("device_id", task.DeviceID),
				zap.String("metric_type", metricType),
			)
			continue
		}

		// Update metrics
		e.metrics.SetDeltaValue(task.DeviceID, metricType, delta.Delta)

		// Publish significant deltas
		if delta.IsSignificant {
			e.publishDeltaResult(delta)
		}
	}

	// Update previous value
	e.regionalMutex.Lock()
	e.deviceLastValues[task.DeviceID] = task.Data
	e.regionalMutex.Unlock()

	processingTime := time.Since(startTime)
	e.metrics.RecordDeltaCalculationTime(processingTime)
}

// processRegionalAnalysis calculates and publishes regional analysis
func (e *Engine) processRegionalAnalysis(task *AnalyticsTask) {
	startTime := time.Now()

	region := task.Region
	if region == "" && task.Data != nil {
		region = task.Data.Location.Region
	}

	// Get regional data
	regionalData := e.timeWindow.GetDataByRegion(region)
	if len(regionalData) < 5 {
		return
	}

	// Calculate regional analysis for each metric
	metrics := []string{"temperature", "humidity", "pressure"}

	for _, metricType := range metrics {
		regional, err := algorithms.CalculateRegionalAnalysis(regionalData, region, metricType)
		if err != nil {
			e.logger.Error("Failed to calculate regional analysis",
				zap.Error(err),
				zap.String("region", region),
				zap.String("metric_type", metricType),
			)
			continue
		}

		// Update metrics
		e.metrics.SetRegionalAverage(region, metricType, regional.Average)
		e.metrics.SetRegionalDeviceCount(region, regional.DeviceCount)

		// Publish to MQTT
		e.publishRegionalResult(regional)
	}

	// Update last regional analysis time
	e.regionalMutex.Lock()
	e.lastRegionalAnalysis[region] = time.Now()
	e.regionalMutex.Unlock()

	processingTime := time.Since(startTime)
	e.metrics.RecordRegionalAnalysisTime(processingTime)
}

// processAnomalyDetection detects and publishes anomalies
func (e *Engine) processAnomalyDetection(task *AnalyticsTask) {
	startTime := time.Now()

	// Get device data for anomaly detection
	deviceData := e.timeWindow.GetDeviceData(task.DeviceID)
	if len(deviceData) < 10 { // Need minimum points for anomaly detection
		return
	}

	// Detect anomalies for each metric
	metrics := []string{"temperature", "humidity", "pressure"}

	for _, metricType := range metrics {
		anomalies, err := algorithms.DetectAnomalies(deviceData, metricType)
		if err != nil {
			e.logger.Error("Failed to detect anomalies",
				zap.Error(err),
				zap.String("device_id", task.DeviceID),
				zap.String("metric_type", metricType),
			)
			continue
		}

		// Process detected anomalies
		for _, anomaly := range anomalies {
			e.metrics.IncrementAnomalies()
			e.metrics.IncrementRegionalAnomalies(anomaly.Region, anomaly.AnomalyType)

			// Publish significant anomalies
			if anomaly.Severity == "high" || anomaly.Severity == "critical" {
				e.publishAnomalyResult(anomaly)
			}

			e.logger.Info("Anomaly detected",
				zap.String("device_id", anomaly.DeviceID),
				zap.String("region", anomaly.Region),
				zap.String("metric_type", anomaly.MetricType),
				zap.String("severity", anomaly.Severity),
				zap.Float64("deviation", anomaly.Deviation),
			)
		}
	}

	processingTime := time.Since(startTime)
	e.metrics.RecordAnomalyDetectionTime(processingTime)
}

// periodicAnalytics runs periodic analytics tasks
func (e *Engine) periodicAnalytics() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.PublishInterval)
	defer ticker.Stop()

	e.logger.Debug("Periodic analytics started",
		zap.Duration("interval", e.config.PublishInterval),
	)

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Debug("Periodic analytics stopping")
			return

		case <-ticker.C:
			if e.config.RegionalAnalysis {
				e.triggerRegionalAnalysis()
			}
		}
	}
}

// triggerRegionalAnalysis triggers regional analysis for all regions
func (e *Engine) triggerRegionalAnalysis() {
	regions := []string{"north", "south", "east", "west", "central"}

	for _, region := range regions {
		e.queueTask(&AnalyticsTask{
			Type:      "regional",
			Region:    region,
			Timestamp: time.Now(),
		})
	}
}

// MQTT Publishing Methods
func (e *Engine) publishTrendResult(trend *algorithms.TrendResult) {
	message := &mqtt.AnalyticsMessage{
		DeviceID:    trend.DeviceID,
		Region:      trend.Region,
		MessageType: "trend",
		Timestamp:   trend.CalculatedAt,
		Data: map[string]interface{}{
			"metric_type": trend.MetricType,
			"direction":   trend.Direction,
			"strength":    trend.Strength,
			"slope":       trend.Slope,
			"r_squared":   trend.RSquared,
			"start_value": trend.StartValue,
			"end_value":   trend.EndValue,
		},
		Metadata: map[string]string{
			"analysis_type": "trend",
			"version":       "1.0",
		},
	}

	if err := e.mqttPublisher.PublishAnalytics(message); err != nil {
		e.logger.Error("Failed to publish trend result", zap.Error(err))
	}
}

func (e *Engine) publishDeltaResult(delta *algorithms.DeltaResult) {
	message := &mqtt.AnalyticsMessage{
		DeviceID:    delta.DeviceID,
		Region:      delta.Region,
		MessageType: "delta",
		Timestamp:   delta.CalculatedAt,
		Data: map[string]interface{}{
			"metric_type":    delta.MetricType,
			"current_value":  delta.CurrentValue,
			"previous_value": delta.PreviousValue,
			"delta":          delta.Delta,
			"delta_percent":  delta.DeltaPercent,
			"is_significant": delta.IsSignificant,
		},
		Metadata: map[string]string{
			"analysis_type": "delta",
			"version":       "1.0",
		},
	}

	if err := e.mqttPublisher.PublishAnalytics(message); err != nil {
		e.logger.Error("Failed to publish delta result", zap.Error(err))
	}
}

func (e *Engine) publishRegionalResult(regional *algorithms.RegionalResult) {
	message := &mqtt.AnalyticsMessage{
		DeviceID:    "", // Regional analysis doesn't have specific device
		Region:      regional.Region,
		MessageType: "regional",
		Timestamp:   regional.CalculatedAt,
		Data: map[string]interface{}{
			"metric_type":        regional.MetricType,
			"device_count":       regional.DeviceCount,
			"average":            regional.Average,
			"minimum":            regional.Minimum,
			"maximum":            regional.Maximum,
			"standard_deviation": regional.StandardDev,
			"median":             regional.Median,
		},
		Metadata: map[string]string{
			"analysis_type": "regional",
			"version":       "1.0",
		},
	}

	if err := e.mqttPublisher.PublishAnalytics(message); err != nil {
		e.logger.Error("Failed to publish regional result", zap.Error(err))
	}
}

func (e *Engine) publishAnomalyResult(anomaly *algorithms.AnomalyResult) {
	message := &mqtt.AnalyticsMessage{
		DeviceID:    anomaly.DeviceID,
		Region:      anomaly.Region,
		MessageType: "anomaly",
		Timestamp:   anomaly.DetectedAt,
		Data: map[string]interface{}{
			"metric_type":    anomaly.MetricType,
			"current_value":  anomaly.CurrentValue,
			"expected_value": anomaly.ExpectedValue,
			"deviation":      anomaly.Deviation,
			"severity":       anomaly.Severity,
			"anomaly_type":   anomaly.AnomalyType,
		},
		Metadata: map[string]string{
			"analysis_type": "anomaly",
			"version":       "1.0",
		},
	}

	if err := e.mqttPublisher.PublishAnalytics(message); err != nil {
		e.logger.Error("Failed to publish anomaly result", zap.Error(err))
	}
}

// metricsUpdater periodically updates system metrics
func (e *Engine) metricsUpdater() {
	defer e.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var m runtime.MemStats

	for {
		select {
		case <-e.ctx.Done():
			return

		case <-ticker.C:
			// Update system metrics
			runtime.ReadMemStats(&m)
			goroutines := runtime.NumGoroutine()

			e.metrics.UpdateSystemMetrics(
				float64(m.Alloc),
				0.0, // CPU usage would need additional library
				goroutines,
			)

			// Update window metrics
			windowStats := e.timeWindow.GetStats()
			e.metrics.SetActiveDevices(windowStats["device_count"].(int))
			e.metrics.SetDataPointsInWindow(windowStats["total_points"].(int))
			e.metrics.SetWindowMemoryUsage(int64(m.Alloc))

			// Update processing rate
			uptime := time.Since(e.startTime)
			if uptime > 0 {
				rate := float64(e.processedCount) / uptime.Seconds()
				e.metrics.SetProcessingRate(rate)
			}

			e.logger.Debug("Updated analytics metrics",
				zap.Int64("processed_count", e.processedCount),
				zap.Int("active_devices", windowStats["device_count"].(int)),
				zap.Int("data_points", windowStats["total_points"].(int)),
				zap.Int("goroutines", goroutines),
			)
		}
	}
}

// GetStats returns engine statistics
func (e *Engine) GetStats() map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	uptime := time.Since(e.startTime)
	processingRate := float64(e.processedCount) / uptime.Seconds()
	windowStats := e.timeWindow.GetStats()

	return map[string]interface{}{
		"processed_count": e.processedCount,
		"error_count":     e.errorCount,
		"processing_rate": processingRate,
		"uptime_seconds":  uptime.Seconds(),
		"is_running":      e.running,
		"window_stats":    windowStats,
	}
}

// IsRunning returns whether the engine is currently running
func (e *Engine) IsRunning() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.running
}

// consumerHandler implements sarama.ConsumerGroupHandler
type consumerHandler struct {
	engine *Engine
	logger logger.Logger
}

func (h *consumerHandler) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Analytics consumer session started")
	return nil
}

func (h *consumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info("Analytics consumer session ended")
	return nil
}

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	h.logger.Info("Started consuming partition for analytics",
		zap.String("topic", claim.Topic()),
		zap.Int32("partition", claim.Partition()),
	)

	for {
		select {
		case <-session.Context().Done():
			return nil

		case message := <-claim.Messages():
			if message == nil {
				continue
			}

			// Parse sensor data
			var sensorData SensorData
			if err := json.Unmarshal(message.Value, &sensorData); err != nil {
				h.logger.Error("Failed to unmarshal sensor data",
					zap.Error(err),
					zap.String("topic", message.Topic),
					zap.Int32("partition", message.Partition),
				)
				continue
			}

			// Update metrics
			h.engine.metrics.IncrementMessagesConsumed(1)

			// Send to processing
			select {
			case h.engine.sensorData <- &sensorData:
				// Data sent for processing
			default:
				h.logger.Warn("Sensor data channel full, dropping message",
					zap.String("device_id", sensorData.DeviceID),
				)
				h.engine.metrics.IncrementAnalysisFailed(1)
			}

			// Mark message as processed
			session.MarkMessage(message, "")
		}
	}
}
