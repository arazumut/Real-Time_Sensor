package grpc

import (
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/twinup/sensor-system/services/alert-handler-service/internal/server"
	"github.com/twinup/sensor-system/services/alert-handler-service/pkg/logger"
)

// Server wraps the gRPC server with configuration and lifecycle management
type Server struct {
	grpcServer  *grpc.Server
	alertServer *server.Server
	listener    net.Listener
	config      Config
	logger      logger.Logger
	running     bool
}

// Config holds gRPC server configuration
type Config struct {
	Host                 string
	Port                 int
	MaxRecvMsgSize       int
	MaxSendMsgSize       int
	ConnectionTimeout    time.Duration
	MaxConcurrentStreams uint32
	KeepAliveTime        time.Duration
	KeepAliveTimeout     time.Duration
	MaxConnectionIdle    time.Duration
	MaxConnectionAge     time.Duration
	EnableReflection     bool
}

// NewServer creates a new gRPC server
func NewServer(config Config, alertServer *server.Server, logger logger.Logger) (*Server, error) {
	// Create listener
	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	// Configure gRPC server options
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(config.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(config.MaxConcurrentStreams),
		grpc.ConnectionTimeout(config.ConnectionTimeout),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    config.KeepAliveTime,
			Timeout: config.KeepAliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(opts...)

	// Register alert service (simplified registration for now)
	// In real implementation, this would use generated proto code
	// pb.RegisterAlertServiceServer(grpcServer, alertServer)

	// Enable reflection for development
	if config.EnableReflection {
		reflection.Register(grpcServer)
	}

	server := &Server{
		grpcServer:  grpcServer,
		alertServer: alertServer,
		listener:    listener,
		config:      config,
		logger:      logger,
	}

	logger.Info("gRPC server created successfully",
		zap.String("address", address),
		zap.Int("max_recv_msg_size", config.MaxRecvMsgSize),
		zap.Int("max_send_msg_size", config.MaxSendMsgSize),
		zap.Bool("reflection_enabled", config.EnableReflection),
	)

	return server, nil
}

// Start starts the gRPC server
func (s *Server) Start() error {
	if s.running {
		return fmt.Errorf("gRPC server is already running")
	}

	s.logger.Info("Starting gRPC server",
		zap.String("address", s.listener.Addr().String()),
	)

	// Start alert server
	if err := s.alertServer.Start(); err != nil {
		return fmt.Errorf("failed to start alert server: %w", err)
	}

	s.running = true

	// Start serving (blocking)
	go func() {
		if err := s.grpcServer.Serve(s.listener); err != nil {
			s.logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	s.logger.Info("gRPC server started successfully")
	return nil
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() error {
	if !s.running {
		return fmt.Errorf("gRPC server is not running")
	}

	s.logger.Info("Stopping gRPC server...")

	// Stop alert server
	if err := s.alertServer.Stop(); err != nil {
		s.logger.Error("Error stopping alert server", zap.Error(err))
	}

	// Graceful stop with timeout
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("gRPC server stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("gRPC server graceful stop timeout, forcing stop")
		s.grpcServer.Stop()
	}

	s.running = false
	return nil
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	alertStats := s.alertServer.GetStats()

	return map[string]interface{}{
		"grpc_address": s.listener.Addr().String(),
		"is_running":   s.running,
		"alert_server": alertStats,
	}
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	return s.running
}

// GetAddress returns the server address
func (s *Server) GetAddress() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}
