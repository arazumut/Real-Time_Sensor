package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/sensor-producer-service/internal/config"
	"github.com/twinup/sensor-system/services/sensor-producer-service/internal/metrics"
	"github.com/twinup/sensor-system/services/sensor-producer-service/internal/simulator"
	"github.com/twinup/sensor-system/services/sensor-producer-service/pkg/kafka"
	"github.com/twinup/sensor-system/services/sensor-producer-service/pkg/logger"
)

var (
	configFile string
	version    = "1.0.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

// main is the entry point of the sensor producer service
func main() {
	rootCmd := &cobra.Command{
		Use:   "sensor-producer",
		Short: "TWINUP Sensor Producer Service - High-performance IoT device simulator",
		Long: `
TWINUP Sensor Producer Service

A high-performance IoT device simulator that generates realistic sensor data
from 10,000+ virtual devices and sends them to Kafka with optimal throughput.

Features:
- Concurrent device simulation with realistic data patterns
- High-throughput Kafka producer with batching
- Comprehensive Prometheus metrics
- Graceful shutdown and error handling
- Configurable via YAML files and environment variables

Example:
  sensor-producer --config config.yaml
  sensor-producer --config /etc/twinup/producer.yaml
		`,
		RunE: run,
	}

	// Add flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Config file path")

	// Add version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("TWINUP Sensor Producer Service\n")
			fmt.Printf("Version: %s\n", version)
			fmt.Printf("Build Time: %s\n", buildTime)
			fmt.Printf("Git Commit: %s\n", gitCommit)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// run is the main application logic
func run(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	log, err := logger.NewLogger(cfg.Logger.Level, cfg.Logger.Format, cfg.Logger.Development)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer log.Sync()

	log.Info("Starting TWINUP Sensor Producer Service",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("git_commit", gitCommit),
	)

	// Initialize metrics
	metricsCollector := metrics.NewMetrics()

	// Start metrics server if enabled
	var metricsServer *http.Server
	if cfg.Metrics.Enabled {
		metricsServer = metrics.StartMetricsServer(cfg.Metrics.Port, cfg.Metrics.Path)
		go func() {
			log.Info("Starting metrics server",
				zap.Int("port", cfg.Metrics.Port),
				zap.String("path", cfg.Metrics.Path),
			)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("Metrics server error", zap.Error(err))
			}
		}()
	}

	// Initialize Kafka producer
	producerConfig := kafka.ProducerConfig{
		Brokers:         cfg.Kafka.Brokers,
		Topic:           cfg.Kafka.Topic,
		RetryMax:        cfg.Kafka.RetryMax,
		RequiredAcks:    cfg.Kafka.RequiredAcks,
		Timeout:         cfg.Kafka.Timeout,
		CompressionType: cfg.Kafka.CompressionType,
		FlushFrequency:  cfg.Kafka.FlushFrequency,
		FlushMessages:   cfg.Kafka.FlushMessages,
		FlushBytes:      cfg.Kafka.FlushBytes,
		MaxMessageBytes: cfg.Kafka.MaxMessageBytes,
	}

	producer, err := kafka.NewProducer(producerConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	// Initialize device simulator
	simulatorConfig := simulator.SimulatorConfig{
		DeviceCount: cfg.Producer.DeviceCount,
		BatchSize:   cfg.Producer.BatchSize,
		Interval:    cfg.Producer.ProduceInterval,
		WorkerCount: cfg.Producer.WorkerCount,
		BufferSize:  cfg.Producer.BufferSize,
	}

	sim := simulator.NewSimulator(simulatorConfig, log, metricsCollector)

	// Start simulator
	if err := sim.Start(); err != nil {
		return fmt.Errorf("failed to start simulator: %w", err)
	}

	// Start producer
	if err := producer.Start(sim.GetReadings()); err != nil {
		return fmt.Errorf("failed to start producer: %w", err)
	}

	// Start benchmark logging if enabled
	if cfg.Producer.EnableBenchmark {
		go startBenchmarkLogger(log, sim, producer, cfg.Producer.BenchmarkInterval)
	}

	log.Info("All services started successfully")

	// Wait for shutdown signal
	return waitForShutdown(log, sim, producer, metricsServer, cfg.Producer.GracefulTimeout)
}

// startBenchmarkLogger periodically logs performance statistics
func startBenchmarkLogger(log logger.Logger, sim *simulator.Simulator, producer *kafka.Producer, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			simStats := sim.GetStats()
			prodStats := producer.GetStats()

			log.Info("Performance Benchmark",
				zap.Any("simulator", simStats),
				zap.Any("producer", prodStats),
			)
		}
	}
}

// waitForShutdown waits for shutdown signal and gracefully stops all services
func waitForShutdown(log logger.Logger, sim *simulator.Simulator, producer *kafka.Producer, metricsServer *http.Server, timeout time.Duration) error {
	// Create channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	log.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Shutdown services gracefully
	shutdownErrors := make(chan error, 3)

	// Stop simulator
	go func() {
		log.Info("Stopping simulator...")
		shutdownErrors <- sim.Stop()
	}()

	// Stop producer
	go func() {
		log.Info("Stopping Kafka producer...")
		shutdownErrors <- producer.Stop()
	}()

	// Stop metrics server
	go func() {
		if metricsServer != nil {
			log.Info("Stopping metrics server...")
			shutdownErrors <- metricsServer.Shutdown(ctx)
		} else {
			shutdownErrors <- nil
		}
	}()

	// Wait for all services to stop or timeout
	var errors []error
	for i := 0; i < 3; i++ {
		select {
		case err := <-shutdownErrors:
			if err != nil {
				errors = append(errors, err)
			}
		case <-ctx.Done():
			errors = append(errors, fmt.Errorf("shutdown timeout exceeded"))
		}
	}

	// Log final statistics
	simStats := sim.GetStats()
	prodStats := producer.GetStats()

	log.Info("Final Statistics",
		zap.Any("simulator", simStats),
		zap.Any("producer", prodStats),
	)

	if len(errors) > 0 {
		log.Error("Shutdown completed with errors", zap.Errors("errors", errors))
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	log.Info("Graceful shutdown completed successfully")
	return nil
}
