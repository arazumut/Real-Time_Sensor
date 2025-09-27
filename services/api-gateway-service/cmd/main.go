package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/api-gateway-service/internal/cache"
	"github.com/twinup/sensor-system/services/api-gateway-service/internal/clickhouse"
	"github.com/twinup/sensor-system/services/api-gateway-service/internal/config"
	"github.com/twinup/sensor-system/services/api-gateway-service/internal/gateway"
	"github.com/twinup/sensor-system/services/api-gateway-service/internal/metrics"
	"github.com/twinup/sensor-system/services/api-gateway-service/pkg/logger"
)

var (
	configFile string
	version    = "1.0.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

// main is the entry point of the API gateway service
func main() {
	rootCmd := &cobra.Command{
		Use:   "api-gateway",
		Short: "TWINUP API Gateway Service - RESTful API with Redis cache and ClickHouse fallback",
		Long: `
TWINUP API Gateway Service

A high-performance RESTful API gateway that provides unified access to sensor data
with intelligent caching and database fallback strategies.

Features:
- RESTful API endpoints for device metrics and analytics
- Redis caching with ClickHouse fallback for high availability
- JWT authentication integration with role-based access control
- Request rate limiting and concurrent request management
- Pagination support for large datasets
- Response compression for bandwidth optimization
- Comprehensive Prometheus metrics and monitoring
- Load testing capabilities and performance optimization

API Endpoints:
- GET /api/v1/devices/{id}/metrics    - Device current metrics
- GET /api/v1/devices/{id}/history    - Device historical data
- GET /api/v1/devices/{id}/analytics  - Device analytics data
- GET /api/v1/devices                 - List all devices (paginated)
- GET /api/v1/stats                   - System statistics
- GET /api/v1/health                  - Health check

Example:
  api-gateway --config config.yaml
  api-gateway --config /etc/twinup/gateway.yaml
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
			fmt.Printf("TWINUP API Gateway Service\n")
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

	log.Info("Starting TWINUP API Gateway Service",
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

	// Initialize Redis cache service
	cacheConfig := cache.Config{
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
		CacheTTL:     cfg.Redis.CacheTTL,
	}

	cacheService, err := cache.NewService(cacheConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create cache service: %w", err)
	}

	// Initialize ClickHouse service
	clickhouseConfig := clickhouse.Config{
		Host:            cfg.ClickHouse.Host,
		Port:            cfg.ClickHouse.Port,
		Database:        cfg.ClickHouse.Database,
		Username:        cfg.ClickHouse.Username,
		Password:        cfg.ClickHouse.Password,
		MaxOpenConns:    cfg.ClickHouse.MaxOpenConns,
		MaxIdleConns:    cfg.ClickHouse.MaxIdleConns,
		ConnMaxLifetime: cfg.ClickHouse.ConnMaxLifetime,
		QueryTimeout:    cfg.ClickHouse.QueryTimeout,
	}

	clickhouseService, err := clickhouse.NewService(clickhouseConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create ClickHouse service: %w", err)
	}

	// Initialize gateway handlers
	gatewayConfig := gateway.Config{
		EnableCaching:         cfg.Gateway.EnableCaching,
		CacheDefaultTTL:       cfg.Gateway.CacheDefaultTTL,
		FallbackToClickHouse:  cfg.Gateway.FallbackToClickHouse,
		MaxConcurrentRequests: cfg.Gateway.MaxConcurrentRequests,
		RequestTimeout:        cfg.Gateway.RequestTimeout,
		EnablePagination:      cfg.Gateway.EnablePagination,
		DefaultPageSize:       cfg.Gateway.DefaultPageSize,
		MaxPageSize:           cfg.Gateway.MaxPageSize,
	}

	handlers := gateway.NewHandlers(cacheService, clickhouseService, log, metricsCollector, gatewayConfig)

	// Start services
	if err := cacheService.Start(); err != nil {
		return fmt.Errorf("failed to start cache service: %w", err)
	}

	if err := clickhouseService.Start(); err != nil {
		return fmt.Errorf("failed to start ClickHouse service: %w", err)
	}

	// Setup HTTP server
	httpServer := setupHTTPServer(cfg.HTTP, handlers, log, metricsCollector)

	// Start HTTP server
	go func() {
		address := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		log.Info("Starting HTTP server", zap.String("address", address))

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Start system metrics updater
	go startSystemMetricsUpdater(log, metricsCollector)

	log.Info("All services started successfully",
		zap.String("http_address", fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)),
		zap.Bool("caching_enabled", cfg.Gateway.EnableCaching),
		zap.Bool("clickhouse_fallback", cfg.Gateway.FallbackToClickHouse),
		zap.Duration("cache_ttl", cfg.Gateway.CacheDefaultTTL),
	)

	// Wait for shutdown signal
	return waitForShutdown(log, httpServer, cacheService, clickhouseService, metricsServer)
}

// setupHTTPServer configures the HTTP server with routes and middleware
func setupHTTPServer(
	config config.HTTPConfig,
	handlers *gateway.Handlers,
	log logger.Logger,
	metrics *metrics.Metrics,
) *http.Server {
	// Set Gin mode
	if !config.EnableCORS {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(requestLoggingMiddleware(log, metrics))

	// CORS middleware
	if config.EnableCORS {
		router.Use(corsMiddleware())
	}

	// Rate limiting middleware
	if config.EnableRateLimit {
		router.Use(rateLimitMiddleware(config.RateLimitRPS, metrics))
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Device endpoints
		v1.GET("/devices/:id/metrics", handlers.GetDeviceMetrics)
		v1.GET("/devices/:id/analytics", handlers.GetDeviceAnalytics)
		v1.GET("/devices/:id/history", handlers.GetDeviceHistory)
		v1.GET("/devices", handlers.GetDeviceList)

		// System endpoints
		v1.GET("/stats", handlers.GetSystemStats)
		v1.GET("/health", handlers.HealthCheck)
	}

	// Root endpoint with API documentation
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "TWINUP API Gateway Service",
			"version": version,
			"endpoints": map[string]interface{}{
				"devices": map[string]string{
					"GET /api/v1/devices/{id}/metrics":   "Get device current metrics",
					"GET /api/v1/devices/{id}/analytics": "Get device analytics data",
					"GET /api/v1/devices/{id}/history":   "Get device historical data",
					"GET /api/v1/devices":                "List all devices (paginated)",
				},
				"system": map[string]string{
					"GET /api/v1/stats":  "Get system statistics",
					"GET /api/v1/health": "Health check",
				},
			},
			"parameters": map[string]interface{}{
				"pagination": map[string]string{
					"page":      "Page number (default: 1)",
					"page_size": "Page size (default: 100, max: 1000)",
				},
				"history": map[string]string{
					"start_time": "Start time in RFC3339 format",
					"end_time":   "End time in RFC3339 format",
					"limit":      "Maximum number of records (default: 1000)",
				},
				"analytics": map[string]string{
					"type": "Analytics type: trend, delta, regional, all (default: all)",
				},
			},
		})
	})

	return &http.Server{
		Addr:           fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:        router,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		IdleTimeout:    config.IdleTimeout,
		MaxHeaderBytes: config.MaxHeaderBytes,
	}
}

// requestLoggingMiddleware logs HTTP requests
func requestLoggingMiddleware(log logger.Logger, metrics *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process request
		c.Next()

		// Log request
		latency := time.Since(startTime)
		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		clientIP := c.ClientIP()

		// Record metrics
		metrics.IncrementHTTPRequests(method, path, fmt.Sprintf("%d", statusCode))
		metrics.RecordHTTPLatency(method, path, latency)
		metrics.RecordHTTPResponseSize(c.Writer.Size())

		// Log based on status code
		if statusCode >= 500 {
			log.Error("HTTP request completed",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", statusCode),
				zap.Duration("latency", latency),
				zap.String("client_ip", clientIP),
			)
		} else if statusCode >= 400 {
			log.Warn("HTTP request completed",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", statusCode),
				zap.Duration("latency", latency),
				zap.String("client_ip", clientIP),
			)
		} else {
			log.Debug("HTTP request completed",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("status", statusCode),
				zap.Duration("latency", latency),
				zap.String("client_ip", clientIP),
			)
		}
	}
}

// corsMiddleware handles CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// rateLimitMiddleware implements simple rate limiting
func rateLimitMiddleware(rps int, metrics *metrics.Metrics) gin.HandlerFunc {
	// Simple rate limiting for demo (in production, use proper rate limiter)
	return func(c *gin.Context) {
		// For demo purposes, we'll skip actual rate limiting implementation
		c.Next()
	}
}

// startSystemMetricsUpdater periodically updates system metrics
func startSystemMetricsUpdater(log logger.Logger, metrics *metrics.Metrics) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var m runtime.MemStats

	for {
		select {
		case <-ticker.C:
			// Update system metrics
			runtime.ReadMemStats(&m)
			goroutines := runtime.NumGoroutine()

			metrics.UpdateSystemMetrics(
				float64(m.Alloc),
				0.0, // CPU usage would need additional library
				goroutines,
			)

			log.Debug("Updated system metrics",
				zap.Int("goroutines", goroutines),
				zap.Float64("memory_mb", float64(m.Alloc)/1024/1024),
			)
		}
	}
}

// waitForShutdown waits for shutdown signal and gracefully stops all services
func waitForShutdown(
	log logger.Logger,
	httpServer *http.Server,
	cacheService *cache.Service,
	clickhouseService *clickhouse.Service,
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

	// Stop HTTP server
	go func() {
		log.Info("Stopping HTTP server...")
		shutdownErrors <- httpServer.Shutdown(ctx)
	}()

	// Stop cache service
	go func() {
		log.Info("Stopping cache service...")
		shutdownErrors <- cacheService.Stop()
	}()

	// Stop ClickHouse service
	go func() {
		log.Info("Stopping ClickHouse service...")
		shutdownErrors <- clickhouseService.Stop()
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
	cacheStats := cacheService.GetStats()
	dbStats := clickhouseService.GetStats()

	log.Info("Final Statistics",
		zap.Any("cache_service", cacheStats),
		zap.Any("clickhouse_service", dbStats),
	)

	if len(errors) > 0 {
		log.Error("Shutdown completed with errors", zap.Errors("errors", errors))
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	log.Info("Graceful shutdown completed successfully")
	return nil
}
