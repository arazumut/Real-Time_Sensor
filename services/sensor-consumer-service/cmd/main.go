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

	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/clickhouse"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/config"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/consumer"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/grpc"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/metrics"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/internal/redis"
	"github.com/twinup/sensor-system/services/sensor-consumer-service/pkg/logger"
)
denemedenme

var (
	configFile string
	version    = "1.0.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

// main is the entry point of the sensor consumer service
func main() {
	rootCmd := &cobra.Command{
		Use:   "sensor-consumer",
		Short: "TWINUP Sensor Consumer Service - High-performance Kafka consumer with ClickHouse and Redis",
		Long: `
TWINUP Sensor Consumer Service

A high-performance Kafka consumer that processes sensor data and stores it in
ClickHouse for analytics while maintaining a Redis cache for fast access.

Features:
- High-throughput Kafka consumer with manual offset management
- Batch processing for ClickHouse with async inserts
- Redis caching with pipeline optimization
- gRPC client for alert service integration
- Comprehensive Prometheus metrics and monitoring
- Graceful shutdown and error handling

Example:
  sensor-consumer --config config.yaml
  sensor-consumer --config /etc/twinup/consumer.yaml
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
			fmt.Printf("TWINUP Sensor Consumer Service\n")
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

	log.Info("Starting TWINUP Sensor Consumer Service",
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

	// Initialize ClickHouse client
	clickhouseConfig := clickhouse.Config{
		Host:            cfg.ClickHouse.Host,
		Port:            cfg.ClickHouse.Port,
		Database:        cfg.ClickHouse.Database,
		Username:        cfg.ClickHouse.Username,
		Password:        cfg.ClickHouse.Password,
		MaxOpenConns:    cfg.ClickHouse.MaxOpenConns,
		MaxIdleConns:    cfg.ClickHouse.MaxIdleConns,
		ConnMaxLifetime: cfg.ClickHouse.ConnMaxLifetime,
		BatchSize:       cfg.ClickHouse.BatchSize,
		BatchTimeout:    cfg.ClickHouse.BatchTimeout,
		AsyncInsert:     cfg.ClickHouse.AsyncInsert,
	}

	clickhouseClient, err := clickhouse.NewClient(clickhouseConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create ClickHouse client: %w", err)
	}

	// Initialize Redis client
	redisConfig := redis.Config{
		Host:         cfg.Redis.Host,
		Port:         cfg.Redis.Port,
		Password:     cfg.Redis.Password,
		Database:     cfg.Redis.Database,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		MaxRetries:   cfg.Redis.MaxRetries,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
		IdleTimeout:  cfg.Redis.IdleTimeout,
		TTL:          cfg.Redis.TTL,
		PipelineSize: cfg.Redis.PipelineSize,
	}

	redisClient, err := redis.NewClient(redisConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create Redis client: %w", err)
	}

	// Initialize gRPC alert client
	grpcConfig := grpc.Config{
		AlertServiceAddr: cfg.GRPC.AlertServiceAddr,
		ConnTimeout:      cfg.GRPC.ConnTimeout,
		RequestTimeout:   cfg.GRPC.RequestTimeout,
		MaxRetries:       cfg.GRPC.MaxRetries,
		RetryBackoff:     cfg.GRPC.RetryBackoff,
		KeepAlive:        cfg.GRPC.KeepAlive,
		KeepAliveTimeout: cfg.GRPC.KeepAliveTimeout,
	}

	alertClient, err := grpc.NewAlertClient(grpcConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create gRPC alert client: %w", err)
	}

	// Initialize Kafka consumer
	consumerConfig := consumer.Config{
		Brokers:           cfg.Kafka.Brokers,
		Topic:             cfg.Kafka.Topic,
		GroupID:           cfg.Kafka.GroupID,
		AutoCommit:        cfg.Kafka.AutoCommit,
		CommitInterval:    cfg.Kafka.CommitInterval,
		SessionTimeout:    cfg.Kafka.SessionTimeout,
		HeartbeatInterval: cfg.Kafka.HeartbeatInterval,
		FetchMin:          cfg.Kafka.FetchMin,
		FetchDefault:      cfg.Kafka.FetchDefault,
		FetchMax:          cfg.Kafka.FetchMax,
		RetryBackoff:      cfg.Kafka.RetryBackoff,
		WorkerCount:       cfg.Consumer.WorkerCount,
		BufferSize:        cfg.Consumer.BufferSize,
		ProcessingTimeout: cfg.Consumer.ProcessingTimeout,
		AlertThreshold:    cfg.Consumer.AlertThreshold,
	}

	kafkaConsumer, err := consumer.NewConsumer(
		consumerConfig,
		log,
		metricsCollector,
		clickhouseClient,
		redisClient,
		alertClient,
	)
	if err != nil {
		return fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	// Start all clients
	if err := clickhouseClient.Start(); err != nil {
		return fmt.Errorf("failed to start ClickHouse client: %w", err)
	}

	if err := redisClient.Start(); err != nil {
		return fmt.Errorf("failed to start Redis client: %w", err)
	}

	if err := alertClient.Start(); err != nil {
		return fmt.Errorf("failed to start gRPC alert client: %w", err)
	}

	// Start Kafka consumer
	if err := kafkaConsumer.Start(); err != nil {
		return fmt.Errorf("failed to start Kafka consumer: %w", err)
	}

	// Start benchmark logging if enabled
	if cfg.Consumer.EnableBenchmark {
		go startBenchmarkLogger(log, kafkaConsumer, clickhouseClient, redisClient, alertClient, cfg.Consumer.BenchmarkInterval)
	}

	log.Info("All services started successfully")

	// Wait for shutdown signal
	return waitForShutdown(log, kafkaConsumer, clickhouseClient, redisClient, alertClient, metricsServer, cfg.Consumer.GracefulTimeout)
}

// startBenchmarkLogger periodically logs performance statistics
func startBenchmarkLogger(
	log logger.Logger,
	kafkaConsumer *consumer.Consumer,
	clickhouseClient *clickhouse.Client,
	redisClient *redis.Client,
	alertClient *grpc.AlertClient,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			consumerStats := kafkaConsumer.GetStats()
			clickhouseStats := clickhouseClient.GetStats()
			redisStats := redisClient.GetStats()
			alertStats := alertClient.GetStats()

			log.Info("Performance Benchmark",
				zap.Any("kafka_consumer", consumerStats),
				zap.Any("clickhouse", clickhouseStats),
				zap.Any("redis_cache", redisStats),
				zap.Any("grpc_alert", alertStats),
			)
		}
	}
}

// waitForShutdown waits for shutdown signal and gracefully stops all services
func waitForShutdown(
	log logger.Logger,
	kafkaConsumer *consumer.Consumer,
	clickhouseClient *clickhouse.Client,
	redisClient *redis.Client,
	alertClient *grpc.AlertClient,
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
	shutdownErrors := make(chan error, 5)

	// Stop Kafka consumer
	go func() {
		log.Info("Stopping Kafka consumer...")
		shutdownErrors <- kafkaConsumer.Stop()
	}()

	// Stop ClickHouse client
	go func() {
		log.Info("Stopping ClickHouse client...")
		shutdownErrors <- clickhouseClient.Stop()
	}()

	// Stop Redis client
	go func() {
		log.Info("Stopping Redis client...")
		shutdownErrors <- redisClient.Stop()
	}()

	// Stop gRPC client
	go func() {
		log.Info("Stopping gRPC alert client...")
		shutdownErrors <- alertClient.Stop()
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
	for i := 0; i < 5; i++ {
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
	consumerStats := kafkaConsumer.GetStats()
	clickhouseStats := clickhouseClient.GetStats()
	redisStats := redisClient.GetStats()
	alertStats := alertClient.GetStats()

	log.Info("Final Statistics",
		zap.Any("kafka_consumer", consumerStats),
		zap.Any("clickhouse", clickhouseStats),
		zap.Any("redis_cache", redisStats),
		zap.Any("grpc_alert", alertStats),
	)

	if len(errors) > 0 {
		log.Error("Shutdown completed with errors", zap.Errors("errors", errors))
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	log.Info("Graceful shutdown completed successfully")
	return nil
}
