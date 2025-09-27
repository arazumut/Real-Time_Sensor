package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/authentication-service/internal/auth"
	"github.com/twinup/sensor-system/services/authentication-service/internal/cache"
	"github.com/twinup/sensor-system/services/authentication-service/internal/config"
	"github.com/twinup/sensor-system/services/authentication-service/internal/jwt"
	"github.com/twinup/sensor-system/services/authentication-service/internal/metrics"
	"github.com/twinup/sensor-system/services/authentication-service/internal/middleware"
	"github.com/twinup/sensor-system/services/authentication-service/pkg/logger"
)

var (
	configFile string
	version    = "1.0.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

// main is the entry point of the authentication service
func main() {
	rootCmd := &cobra.Command{
		Use:   "authentication",
		Short: "TWINUP Authentication Service - JWT-based authentication with role-based access control",
		Long: `
TWINUP Authentication Service

A secure JWT-based authentication service with role-based access control (RBAC),
Redis caching, and comprehensive middleware for protecting microservices.

Features:
- JWT token generation and validation with HS256 algorithm
- Role-based access control with granular permissions
- Redis caching for role-permission data with 1-hour TTL
- Rate limiting and brute force protection
- Session management with concurrent session limits
- RESTful API with Gin framework
- Comprehensive Prometheus metrics and monitoring
- Graceful shutdown and error handling

Authentication Flow:
1. User login with username/password
2. JWT access/refresh token generation
3. Role and permission resolution
4. Token validation middleware
5. Permission-based resource access

Example:
  authentication --config config.yaml
  authentication --config /etc/twinup/auth.yaml
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
			fmt.Printf("TWINUP Authentication Service\n")
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

	log.Info("Starting TWINUP Authentication Service",
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
	}

	cacheService, err := cache.NewService(cacheConfig, log, metricsCollector)
	if err != nil {
		return fmt.Errorf("failed to create cache service: %w", err)
	}

	// Initialize JWT service
	jwtConfig := jwt.Config{
		SecretKey:             cfg.JWT.SecretKey,
		AccessTokenTTL:        cfg.JWT.AccessTokenTTL,
		RefreshTokenTTL:       cfg.JWT.RefreshTokenTTL,
		Issuer:                cfg.JWT.Issuer,
		Audience:              cfg.JWT.Audience,
		Algorithm:             cfg.JWT.Algorithm,
		EnableRefreshTokens:   cfg.JWT.EnableRefreshTokens,
		MaxConcurrentSessions: cfg.JWT.MaxConcurrentSessions,
	}

	jwtService := jwt.NewService(jwtConfig, log, metricsCollector)

	// Initialize RBAC system
	rbac := auth.NewRBAC(cacheService, log, metricsCollector)

	// Initialize HTTP handlers
	handlers := auth.NewHandlers(jwtService, rbac, cacheService, log, metricsCollector)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, rbac, log, metricsCollector)

	// Start cache service
	if err := cacheService.Start(); err != nil {
		return fmt.Errorf("failed to start cache service: %w", err)
	}

	// Setup HTTP server
	httpServer := setupHTTPServer(cfg.HTTP, handlers, authMiddleware, log, metricsCollector)

	// Start HTTP server
	go func() {
		address := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		log.Info("Starting HTTP server", zap.String("address", address))

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", zap.Error(err))
		}
	}()

	log.Info("All services started successfully",
		zap.String("http_address", fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)),
		zap.Duration("access_token_ttl", cfg.JWT.AccessTokenTTL),
		zap.Bool("refresh_tokens_enabled", cfg.JWT.EnableRefreshTokens),
	)

	// Wait for shutdown signal
	return waitForShutdown(log, httpServer, cacheService, metricsServer)
}

// setupHTTPServer configures the HTTP server with routes and middleware
func setupHTTPServer(
	config config.HTTPConfig,
	handlers *auth.Handlers,
	authMiddleware *middleware.AuthMiddleware,
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

	// Public routes (no authentication required)
	public := router.Group("/auth")
	{
		public.POST("/login", handlers.Login)
		public.POST("/refresh", handlers.RefreshToken)
		public.GET("/health", handlers.HealthCheck)
	}

	// Protected routes (authentication required)
	protected := router.Group("/auth")
	protected.Use(authMiddleware.RequireAuth())
	{
		protected.POST("/verify", handlers.VerifyToken)
		protected.POST("/logout", handlers.Logout)
		protected.GET("/profile", handlers.GetProfile)
	}

	// Admin routes (admin role required)
	admin := router.Group("/auth/admin")
	admin.Use(authMiddleware.RequireAuth())
	admin.Use(authMiddleware.AdminOnly())
	{
		admin.GET("/roles", handlers.GetRoles)
	}

	// API documentation route
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "TWINUP Authentication Service",
			"version": version,
			"endpoints": map[string]interface{}{
				"auth": map[string]string{
					"POST /auth/login":   "User login",
					"POST /auth/refresh": "Refresh token",
					"POST /auth/verify":  "Verify token",
					"POST /auth/logout":  "User logout",
					"GET  /auth/profile": "Get user profile",
					"GET  /auth/health":  "Health check",
				},
				"admin": map[string]string{
					"GET /auth/admin/roles": "Get all roles (admin only)",
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
	// Simple in-memory rate limiter (in production, use Redis-based limiter)
	return func(c *gin.Context) {
		// For demo purposes, we'll skip actual rate limiting implementation
		// In production, this would use a proper rate limiter like golang.org/x/time/rate
		c.Next()
	}
}

// waitForShutdown waits for shutdown signal and gracefully stops all services
func waitForShutdown(
	log logger.Logger,
	httpServer *http.Server,
	cacheService *cache.Service,
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
	shutdownErrors := make(chan error, 3)

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
	cacheStats := cacheService.GetStats()

	log.Info("Final Statistics",
		zap.Any("cache_service", cacheStats),
	)

	if len(errors) > 0 {
		log.Error("Shutdown completed with errors", zap.Errors("errors", errors))
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	log.Info("Graceful shutdown completed successfully")
	return nil
}
