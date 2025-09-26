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

	"github.com/twinup/sensor-system/services/real-time-analytics-service/internal/analytics"
	"github.com/twinup/sensor-system/services/real-time-analytics-service/internal/config"
	"github.com/twinup/sensor-system/services/real-time-analytics-service/internal/metrics"
	"github.com/twinup/sensor-system/services/real-time-analytics-service/internal/mqtt"
	"github.com/twinup/sensor-system/services/real-time-analytics-service/pkg/logger"
)

var (
	configFile string
	version    = "1.0.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

// main is the entry point of the real-time analytics service
func main() {
	rootCmd := &cobra.Command{
		Use:   "real-time-analytics",
		Short: "TWINUP Real-Time Analytics Service - Advanced sensor data analytics with MQTT publishing",
		Long: `
TWINUP Real-Time Analytics Service

A high-performance real-time analytics engine that processes sensor data streams
and publishes intelligent insights via MQTT with QoS 1 reliability.

Features:
- Real-time trend analysis with linear regression
- Delta calculation and significant change detection
- Regional analysis and comparative statistics
- Anomaly detection using statistical methods
- MQTT publishing with QoS 1 for reliable delivery
- Comprehensive Prometheus metrics and monitoring
- Memory-efficient sliding window data management

Analytics Types:
- Trend Analysis: Direction, strength, and correlation
- Delta Analysis: Change detection and significance
- Regional Analysis: Cross-device regional statistics
- Anomaly Detection: Statistical outlier identification

Example:
  real-time-analytics --config config.yaml
  real-time-analytics --config /etc/twinup/analytics.yaml
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
			fmt.Printf("TWINUP Real-Time Analytics Service\n")
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

	log.Info("Starting TWINUP Real-Time Analytics Service",
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

	// Initialize MQTT publisher
	mqttConfig := mqtt.Config{
		Broker:               cfg.MQTT.Broker,
		ClientID:             cfg.MQTT.ClientID,
		Username:             cfg.MQTT.Username,
		Password:             cfg.MQTT.Password,
		QOS:                  cfg.MQTT.QOS,
		Retained:             cfg.MQTT.Retained,
		ConnectTimeout:       cfg.MQTT.ConnectTimeout,
		WriteTimeout:         cfg.MQTT.WriteTimeout,
		KeepAlive:            cfg.MQTT.KeepAlive,
		PingTimeout:          cfg.MQTT.PingTimeout,
		ConnectRetryInterval: cfg.MQTT.ConnectRetryInterval,
		AutoReconnect:        cfg.MQTT.AutoReconnect,
		MaxReconnectInterval: cfg.MQTT.MaxReconnectInterval,
		TopicPrefix:          cfg.MQTT.TopicPrefix,
	}

	mqttPublisher, err := mqtt.NewPublisher(mqttConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create MQTT publisher: %w", err)
	}

	// Initialize analytics engine
	analyticsConfig := analytics.Config{
		Brokers:           cfg.Kafka.Brokers,
		Topic:             cfg.Kafka.Topic,
		GroupID:           cfg.Kafka.GroupID,
		AutoCommit:        cfg.Kafka.AutoCommit,
		CommitInterval:    cfg.Kafka.CommitInterval,
		SessionTimeout:    cfg.Kafka.SessionTimeout,
		HeartbeatInterval: cfg.Kafka.HeartbeatInterval,
		WorkerCount:       cfg.Analytics.WorkerCount,
		BufferSize:        cfg.Analytics.BufferSize,
		WindowSize:        cfg.Analytics.WindowSize,
		TrendWindow:       cfg.Analytics.TrendWindow,
		DeltaThreshold:    cfg.Analytics.DeltaThreshold,
		RegionalAnalysis:  cfg.Analytics.RegionalAnalysis,
		AnomalyDetection:  cfg.Analytics.AnomalyDetection,
		PublishInterval:   cfg.Analytics.PublishInterval,
		MaxMemoryWindow:   cfg.Analytics.MaxMemoryWindow,
	}

	analyticsEngine, err := analytics.NewEngine(analyticsConfig, mqttPublisher, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create analytics engine: %w", err)
	}

	// Start MQTT publisher
	if err := mqttPublisher.Start(); err != nil {
		return fmt.Errorf("failed to start MQTT publisher: %w", err)
	}

	// Start analytics engine
	if err := analyticsEngine.Start(); err != nil {
		return fmt.Errorf("failed to start analytics engine: %w", err)
	}

	// Start benchmark logging if enabled
	if cfg.Analytics.EnableBenchmark {
		go startBenchmarkLogger(log, analyticsEngine, mqttPublisher, cfg.Analytics.BenchmarkInterval)
	}

	log.Info("All services started successfully")

	// Wait for shutdown signal
	return waitForShutdown(log, analyticsEngine, mqttPublisher, metricsServer, cfg.Analytics.GracefulTimeout)
}

// startBenchmarkLogger periodically logs performance statistics
func startBenchmarkLogger(
	log logger.Logger,
	engine *analytics.Engine,
	publisher *mqtt.Publisher,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			engineStats := engine.GetStats()
			publisherStats := publisher.GetStats()

			log.Info("Performance Benchmark",
				zap.Any("analytics_engine", engineStats),
				zap.Any("mqtt_publisher", publisherStats),
			)
		}
	}
}

// waitForShutdown waits for shutdown signal and gracefully stops all services
func waitForShutdown(
	log logger.Logger,
	engine *analytics.Engine,
	publisher *mqtt.Publisher,
	metricsServer *http.Server,
	timeout time.Duration,
) error {
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

	// Stop analytics engine
	go func() {
		log.Info("Stopping analytics engine...")
		shutdownErrors <- engine.Stop()
	}()

	// Stop MQTT publisher
	go func() {
		log.Info("Stopping MQTT publisher...")
		shutdownErrors <- publisher.Stop()
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
	engineStats := engine.GetStats()
	publisherStats := publisher.GetStats()

	log.Info("Final Statistics",
		zap.Any("analytics_engine", engineStats),
		zap.Any("mqtt_publisher", publisherStats),
	)

	if len(errors) > 0 {
		log.Error("Shutdown completed with errors", zap.Errors("errors", errors))
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	log.Info("Graceful shutdown completed successfully")
	return nil
}
