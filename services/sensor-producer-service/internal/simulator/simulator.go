package simulator

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/twinup/sensor-system/services/sensor-producer-service/pkg/logger"
	"go.uber.org/zap"
)

// Simulator manages multiple device simulations
type Simulator struct {
	devices     []*Device
	deviceCount int
	batchSize   int
	interval    time.Duration
	workerCount int
	bufferSize  int
	logger      logger.Logger
	metrics     MetricsRecorder
	readings    chan *SensorReading
	batches     chan []*SensorReading
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
	mutex       sync.RWMutex

	// Benchmark data
	startTime     time.Time
	totalReadings int64
	lastBenchmark time.Time
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementMessagesProduced(count int)
	SetActiveDevices(count int)
	SetSimulationRate(rate float64)
	UpdateSystemMetrics(memoryBytes, cpuPercent float64, goroutines int)
	RecordBatchSize(size int)
}

// SimulatorConfig holds configuration for the simulator
type SimulatorConfig struct {
	DeviceCount int
	BatchSize   int
	Interval    time.Duration
	WorkerCount int
	BufferSize  int
}

// NewSimulator creates a new device simulator
func NewSimulator(config SimulatorConfig, logger logger.Logger, metrics MetricsRecorder) *Simulator {
	ctx, cancel := context.WithCancel(context.Background())

	return &Simulator{
		deviceCount: config.DeviceCount,
		batchSize:   config.BatchSize,
		interval:    config.Interval,
		workerCount: config.WorkerCount,
		bufferSize:  config.BufferSize,
		logger:      logger,
		metrics:     metrics,
		readings:    make(chan *SensorReading, config.BufferSize),
		batches:     make(chan []*SensorReading, config.BufferSize/config.BatchSize+1),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins the device simulation
func (s *Simulator) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("simulator is already running")
	}

	s.logger.Info("Starting device simulator",
		zap.Int("device_count", s.deviceCount),
		zap.Int("batch_size", s.batchSize),
		zap.Duration("interval", s.interval),
		zap.Int("worker_count", s.workerCount),
	)

	// Initialize devices
	if err := s.initializeDevices(); err != nil {
		return fmt.Errorf("failed to initialize devices: %w", err)
	}

	// Start benchmark tracking
	s.startTime = time.Now()
	s.lastBenchmark = time.Now()

	// Start worker goroutines
	s.startWorkers()

	// Start batch processor
	s.startBatchProcessor()

	// Start metrics updater
	s.startMetricsUpdater()

	s.running = true
	s.logger.Info("Device simulator started successfully")

	return nil
}

// Stop gracefully stops the simulator
func (s *Simulator) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("simulator is not running")
	}

	s.logger.Info("Stopping device simulator...")

	// Cancel context to stop all goroutines
	s.cancel()

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		s.logger.Info("All workers stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timeout waiting for workers to stop")
	}

	// Close channels
	close(s.readings)
	close(s.batches)

	s.running = false
	s.logger.Info("Device simulator stopped")

	return nil
}

// GetReadings returns the readings channel for external consumption
func (s *Simulator) GetReadings() <-chan []*SensorReading {
	return s.batches
}

// GetStats returns simulator statistics
func (s *Simulator) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uptime := time.Since(s.startTime)
	readingsPerSecond := float64(s.totalReadings) / uptime.Seconds()

	activeDevices := 0
	errorDevices := 0
	maintenanceDevices := 0

	for _, device := range s.devices {
		switch device.Status {
		case "active":
			activeDevices++
		case "error":
			errorDevices++
		case "maintenance":
			maintenanceDevices++
		}
	}

	return map[string]interface{}{
		"total_devices":       len(s.devices),
		"active_devices":      activeDevices,
		"error_devices":       errorDevices,
		"maintenance_devices": maintenanceDevices,
		"total_readings":      s.totalReadings,
		"readings_per_second": readingsPerSecond,
		"uptime_seconds":      uptime.Seconds(),
		"is_running":          s.running,
	}
}

// initializeDevices creates and initializes all simulated devices
func (s *Simulator) initializeDevices() error {
	s.devices = make([]*Device, s.deviceCount)

	regions := []string{"north", "south", "east", "west", "central"}
	deviceTypes := []DeviceType{
		TemperatureSensor,
		HumiditySensor,
		PressureSensor,
		WeatherStation,
		IndustrialSensor,
	}

	s.logger.Info("Initializing devices", logger.Int("count", s.deviceCount))

	for i := 0; i < s.deviceCount; i++ {
		// Distribute device types evenly
		deviceType := deviceTypes[i%len(deviceTypes)]
		region := regions[i%len(regions)]

		// Generate device ID and location
		deviceID := GenerateDeviceID(i+1, deviceType)
		location := GenerateRandomLocation(region)

		// Create device
		device := NewDevice(deviceID, deviceType, location)
		s.devices[i] = device

		if i%1000 == 0 {
			s.logger.Debug("Initialized devices",
				zap.Int("count", i+1),
				zap.Int("total", s.deviceCount),
			)
		}
	}

	s.metrics.SetActiveDevices(len(s.devices))
	s.logger.Info("All devices initialized successfully")

	return nil
}

// startWorkers starts the worker goroutines for generating readings
func (s *Simulator) startWorkers() {
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	s.logger.Info("Started worker goroutines", zap.Int("count", s.workerCount))
}

// worker generates sensor readings from assigned devices
func (s *Simulator) worker(workerID int) {
	defer s.wg.Done()

	// Calculate device range for this worker
	devicesPerWorker := s.deviceCount / s.workerCount
	startIdx := workerID * devicesPerWorker
	endIdx := startIdx + devicesPerWorker

	// Last worker handles remaining devices
	if workerID == s.workerCount-1 {
		endIdx = s.deviceCount
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Debug("Worker started",
		zap.Int("worker_id", workerID),
		zap.Int("start_device", startIdx),
		zap.Int("end_device", endIdx-1),
		zap.Int("device_count", endIdx-startIdx),
	)

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("Worker stopping", zap.Int("worker_id", workerID))
			return

		case <-ticker.C:
			// Generate readings for assigned devices
			for i := startIdx; i < endIdx; i++ {
				if i >= len(s.devices) {
					break
				}

				reading := s.devices[i].GenerateReading()

				select {
				case s.readings <- reading:
					// Reading sent successfully
				case <-s.ctx.Done():
					return
				default:
					// Channel is full, skip this reading
					s.logger.Warn("Readings channel full, skipping reading",
						zap.String("device_id", reading.DeviceID),
					)
				}
			}
		}
	}
}

// startBatchProcessor starts the batch processor goroutine
func (s *Simulator) startBatchProcessor() {
	s.wg.Add(1)
	go s.batchProcessor()
}

// batchProcessor collects individual readings into batches
func (s *Simulator) batchProcessor() {
	defer s.wg.Done()

	batch := make([]*SensorReading, 0, s.batchSize)
	ticker := time.NewTicker(time.Second) // Force batch every second
	defer ticker.Stop()

	s.logger.Debug("Batch processor started", zap.Int("batch_size", s.batchSize))

	for {
		select {
		case <-s.ctx.Done():
			// Send remaining batch before stopping
			if len(batch) > 0 {
				s.sendBatch(batch)
			}
			s.logger.Debug("Batch processor stopping")
			return

		case reading := <-s.readings:
			batch = append(batch, reading)
			s.totalReadings++

			// Send batch when it's full
			if len(batch) >= s.batchSize {
				s.sendBatch(batch)
				batch = make([]*SensorReading, 0, s.batchSize)
			}

		case <-ticker.C:
			// Send partial batch periodically to avoid delays
			if len(batch) > 0 {
				s.sendBatch(batch)
				batch = make([]*SensorReading, 0, s.batchSize)
			}
		}
	}
}

// sendBatch sends a batch of readings to the output channel
func (s *Simulator) sendBatch(batch []*SensorReading) {
	if len(batch) == 0 {
		return
	}

	// Create a copy of the batch
	batchCopy := make([]*SensorReading, len(batch))
	copy(batchCopy, batch)

	select {
	case s.batches <- batchCopy:
		s.metrics.IncrementMessagesProduced(len(batchCopy))
		s.metrics.RecordBatchSize(len(batchCopy))

	case <-s.ctx.Done():
		return

	default:
		s.logger.Warn("Batch channel full, dropping batch",
			zap.Int("batch_size", len(batchCopy)),
		)
	}
}

// startMetricsUpdater starts the metrics updater goroutine
func (s *Simulator) startMetricsUpdater() {
	s.wg.Add(1)
	go s.metricsUpdater()
}

// metricsUpdater periodically updates system and simulation metrics
func (s *Simulator) metricsUpdater() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var m runtime.MemStats

	for {
		select {
		case <-s.ctx.Done():
			return

		case <-ticker.C:
			// Update system metrics
			runtime.ReadMemStats(&m)
			goroutines := runtime.NumGoroutine()

			s.metrics.UpdateSystemMetrics(
				float64(m.Alloc),
				0.0, // CPU usage would need additional library
				goroutines,
			)

			// Update simulation rate
			now := time.Now()
			duration := now.Sub(s.lastBenchmark)
			if duration > 0 {
				rate := float64(s.totalReadings) / time.Since(s.startTime).Seconds()
				s.metrics.SetSimulationRate(rate)
			}

			// Count active devices
			activeCount := 0
			for _, device := range s.devices {
				if device.IsHealthy() {
					activeCount++
				}
			}
			s.metrics.SetActiveDevices(activeCount)

			s.logger.Debug("Updated metrics",
				zap.Int64("total_readings", s.totalReadings),
				zap.Int("active_devices", activeCount),
				zap.Int("goroutines", goroutines),
				zap.Float64("memory_mb", float64(m.Alloc)/1024/1024),
			)
		}
	}
}

// IsRunning returns whether the simulator is currently running
func (s *Simulator) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.running
}
