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

	"github.com/twinup/sensor-system/services/alert-handler-service/internal/config"
	"github.com/twinup/sensor-system/services/alert-handler-service/internal/email"
	"github.com/twinup/sensor-system/services/alert-handler-service/internal/metrics"
	"github.com/twinup/sensor-system/services/alert-handler-service/internal/server"
	"github.com/twinup/sensor-system/services/alert-handler-service/internal/storage"
	"github.com/twinup/sensor-system/services/alert-handler-service/pkg/grpc"
	"github.com/twinup/sensor-system/services/alert-handler-service/pkg/logger"
)

var (
	configFile string
	version    = "1.0.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

// main is the entry point of the alert handler service
func main() {
	rootCmd := &cobra.Command{
		Use:   "alert-handler",
		Short: "TWINUP Alert Handler Service - gRPC-based alert processing with email notifications",
		Long: `
TWINUP Alert Handler Service

A high-performance gRPC server that processes temperature alerts and sends
email notifications with intelligent rate limiting and deduplication.

Features:
- gRPC server with high concurrency support
- Temperature threshold alert processing (>90°C)
- Dummy email service with retry logic and templates
- Rate limiting and cooldown periods per device
- Alert deduplication to prevent spam
- In-memory alert storage with retention policies
- Comprehensive Prometheus metrics and monitoring
- Graceful shutdown and error handling

Alert Types:
- Temperature High: Above configured threshold
- Humidity High: Above humidity limits
- Pressure High: Above pressure limits
- Device Offline: Connection lost alerts

Example:
  alert-handler --config config.yaml
  alert-handler --config /etc/twinup/alert.yaml
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
			fmt.Printf("TWINUP Alert Handler Service\n")
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

	log.Info("Starting TWINUP Alert Handler Service",
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

	// Initialize email service
	emailConfig := email.Config{
		SMTPHost:    cfg.Email.SMTPHost,
		SMTPPort:    cfg.Email.SMTPPort,
		Username:    cfg.Email.Username,
		Password:    cfg.Email.Password,
		FromEmail:   cfg.Email.FromEmail,
		FromName:    cfg.Email.FromName,
		EnableTLS:   cfg.Email.EnableTLS,
		EnableSSL:   cfg.Email.EnableSSL,
		Timeout:     cfg.Email.Timeout,
		RetryCount:  cfg.Email.RetryCount,
		RetryDelay:  cfg.Email.RetryDelay,
		TemplateDir: cfg.Email.TemplateDir,
		EnableDummy: cfg.Email.EnableDummy,
	}

	emailService, err := email.NewService(emailConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create email service: %w", err)
	}

	// Initialize alert storage
	storageConfig := storage.Config{
		EnableStorage:   cfg.Storage.EnableStorage,
		RetentionPeriod: cfg.Storage.RetentionPeriod,
		CleanupInterval: cfg.Storage.CleanupInterval,
		MaxStoredAlerts: cfg.Storage.MaxStoredAlerts,
	}

	alertStorage := storage.NewAlertStorage(storageConfig, log, metricsCollector)

	// Initialize alert server
	serverConfig := server.Config{
		TemperatureThreshold: cfg.Alert.TemperatureThreshold,
		HumidityThreshold:    cfg.Alert.HumidityThreshold,
		PressureThreshold:    cfg.Alert.PressureThreshold,
		CooldownPeriod:       cfg.Alert.CooldownPeriod,
		MaxAlertsPerDevice:   cfg.Alert.MaxAlertsPerDevice,
		MaxAlertsPerMinute:   cfg.Alert.MaxAlertsPerMinute,
		EnableRateLimiting:   cfg.Alert.EnableRateLimiting,
		EnableDeduplication:  cfg.Alert.EnableDeduplication,
		DeduplicationWindow:  cfg.Alert.DeduplicationWindow,
		WorkerCount:          cfg.Alert.WorkerCount,
		QueueSize:            cfg.Alert.QueueSize,
		ProcessingTimeout:    cfg.Alert.ProcessingTimeout,
	}

	alertServer := server.NewServer(serverConfig, log, metricsCollector, emailService, alertStorage)

	// Initialize gRPC server
	grpcConfig := grpc.Config{
		Host:                 cfg.GRPC.Host,
		Port:                 cfg.GRPC.Port,
		MaxRecvMsgSize:       cfg.GRPC.MaxRecvMsgSize,
		MaxSendMsgSize:       cfg.GRPC.MaxSendMsgSize,
		ConnectionTimeout:    cfg.GRPC.ConnectionTimeout,
		MaxConcurrentStreams: cfg.GRPC.MaxConcurrentStreams,
		KeepAliveTime:        cfg.GRPC.KeepAliveTime,
		KeepAliveTimeout:     cfg.GRPC.KeepAliveTimeout,
		MaxConnectionIdle:    cfg.GRPC.MaxConnectionIdle,
		MaxConnectionAge:     cfg.GRPC.MaxConnectionAge,
		EnableReflection:     cfg.GRPC.EnableReflection,
	}

	grpcServer, err := grpc.NewServer(grpcConfig, alertServer, log)
	if err != nil {
		return fmt.Errorf("failed to create gRPC server: %w", err)
	}

	// Start email service
	if err := emailService.Start(); err != nil {
		return fmt.Errorf("failed to start email service: %w", err)
	}

	// Start alert storage
	if err := alertStorage.Start(); err != nil {
		return fmt.Errorf("failed to start alert storage: %w", err)
	}

	// Start gRPC server
	if err := grpcServer.Start(); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Start benchmark logging
	go startBenchmarkLogger(log, alertServer, emailService, alertStorage, grpcServer)

	log.Info("All services started successfully",
		zap.String("grpc_address", grpcServer.GetAddress()),
		zap.Float64("temperature_threshold", cfg.Alert.TemperatureThreshold),
		zap.Bool("email_dummy_mode", cfg.Email.EnableDummy),
	)

	// Wait for shutdown signal
	return waitForShutdown(log, grpcServer, emailService, alertStorage, metricsServer)
}

// startBenchmarkLogger periodically logs performance statistics
func startBenchmarkLogger(
	log logger.Logger,
	alertServer *server.Server,
	emailService *email.Service,
	alertStorage *storage.AlertStorage,
	grpcServer *grpc.Server,
) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			alertStats := alertServer.GetStats()
			emailStats := emailService.GetStats()
			storageStats := alertStorage.GetStats()
			grpcStats := grpcServer.GetStats()

			log.Info("Performance Benchmark",
				zap.Any("alert_server", alertStats),
				zap.Any("email_service", emailStats),
				zap.Any("alert_storage", storageStats),
				zap.Any("grpc_server", grpcStats),
			)
		}
	}
}

// waitForShutdown waits for shutdown signal and gracefully stops all services
func waitForShutdown(
	log logger.Logger,
	grpcServer *grpc.Server,
	emailService *email.Service,
	alertStorage *storage.AlertStorage,
	metricsServer *http.Server,
) error {
	// Create channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	log.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown services gracefully
	shutdownErrors := make(chan error, 4)

	// Stop gRPC server
	go func() {
		log.Info("Stopping gRPC server...")
		shutdownErrors <- grpcServer.Stop()
	}()

	// Stop email service
	go func() {
		log.Info("Stopping email service...")
		shutdownErrors <- emailService.Stop()
	}()

	// Stop alert storage
	go func() {
		log.Info("Stopping alert storage...")
		shutdownErrors <- alertStorage.Stop()
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
	for i := 0; i < 4; i++ {
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
	alertStats := grpcServer.GetStats()
	emailStats := emailService.GetStats()
	storageStats := alertStorage.GetStats()

	log.Info("Final Statistics",
		zap.Any("alert_server", alertStats),
		zap.Any("email_service", emailStats),
		zap.Any("alert_storage", storageStats),
	)

	if len(errors) > 0 {
		log.Error("Shutdown completed with errors", zap.Errors("errors", errors))
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	log.Info("Graceful shutdown completed successfully")
	return nil
}
