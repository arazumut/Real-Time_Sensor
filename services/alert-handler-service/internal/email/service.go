package email

import (
	"context"
	"fmt"
	"html/template"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/alert-handler-service/pkg/logger"
)

// EmailRequest represents an email to be sent
type EmailRequest struct {
	To          []string               `json:"to"`
	Subject     string                 `json:"subject"`
	Body        string                 `json:"body"`
	TemplateID  string                 `json:"template_id,omitempty"`
	TemplateData map[string]interface{} `json:"template_data,omitempty"`
	Priority    string                 `json:"priority"` // "low", "normal", "high", "critical"
	AlertID     string                 `json:"alert_id"`
	DeviceID    string                 `json:"device_id"`
	CreatedAt   time.Time              `json:"created_at"`
}

// EmailResponse represents the result of an email send operation
type EmailResponse struct {
	Success   bool      `json:"success"`
	MessageID string    `json:"message_id"`
	Error     string    `json:"error,omitempty"`
	SentAt    time.Time `json:"sent_at"`
	Retries   int       `json:"retries"`
}

// Service handles email operations with dummy implementation for development
type Service struct {
	config    Config
	logger    logger.Logger
	metrics   MetricsRecorder
	templates map[string]*template.Template
	
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	mutex     sync.RWMutex
	
	// Email queue for async processing
	emailQueue chan *EmailRequest
	
	// Performance tracking
	sentCount   int64
	failedCount int64
	retryCount  int64
	startTime   time.Time
}

// Config holds email service configuration
type Config struct {
	SMTPHost     string
	SMTPPort     int
	Username     string
	Password     string
	FromEmail    string
	FromName     string
	EnableTLS    bool
	EnableSSL    bool
	Timeout      time.Duration
	RetryCount   int
	RetryDelay   time.Duration
	TemplateDir  string
	EnableDummy  bool
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementEmailsSent()
	IncrementEmailsFailed()
	RecordEmailSendLatency(duration time.Duration)
	SetEmailQueueSize(size int)
	IncrementEmailRetry()
}

// NewService creates a new email service
func NewService(config Config, logger logger.Logger, metrics MetricsRecorder) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())

	service := &Service{
		config:     config,
		logger:     logger,
		metrics:    metrics,
		templates:  make(map[string]*template.Template),
		ctx:        ctx,
		cancel:     cancel,
		emailQueue: make(chan *EmailRequest, 1000),
		startTime:  time.Now(),
	}

	// Load email templates
	if err := service.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load email templates: %w", err)
	}

	logger.Info("Email service created successfully",
		zap.String("smtp_host", config.SMTPHost),
		zap.Int("smtp_port", config.SMTPPort),
		zap.String("from_email", config.FromEmail),
		zap.Bool("dummy_mode", config.EnableDummy),
	)

	return service, nil
}

// Start begins the email service
func (s *Service) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("email service is already running")
	}

	s.logger.Info("Starting email service")

	// Start email processor workers
	for i := 0; i < 3; i++ { // 3 email workers
		s.wg.Add(1)
		go s.emailWorker(i)
	}

	s.running = true
	s.logger.Info("Email service started successfully")

	return nil
}

// Stop gracefully stops the email service
func (s *Service) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("email service is not running")
	}

	s.logger.Info("Stopping email service...")

	// Cancel context
	s.cancel()

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Email workers stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timeout waiting for email workers to stop")
	}

	// Close email queue
	close(s.emailQueue)

	s.running = false
	s.logger.Info("Email service stopped")

	return nil
}

// SendEmail queues an email for sending
func (s *Service) SendEmail(request *EmailRequest) error {
	if !s.IsRunning() {
		return fmt.Errorf("email service is not running")
	}

	request.CreatedAt = time.Now()

	select {
	case s.emailQueue <- request:
		s.metrics.SetEmailQueueSize(len(s.emailQueue))
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("email service context cancelled")
	default:
		s.logger.Warn("Email queue full, dropping email",
			zap.String("alert_id", request.AlertID),
			zap.String("device_id", request.DeviceID),
		)
		s.metrics.IncrementEmailsFailed()
		return fmt.Errorf("email queue full")
	}
}

// SendAlertEmail sends a temperature alert email
func (s *Service) SendAlertEmail(alertID, deviceID string, temperature, threshold float64, region string) error {
	subject := fmt.Sprintf("🚨 TWINUP Alert: High Temperature Detected - Device %s", deviceID)
	
	body := s.generateAlertEmailBody(alertID, deviceID, temperature, threshold, region)

	request := &EmailRequest{
		To:       []string{"admin@twinup.com", "alerts@twinup.com"},
		Subject:  subject,
		Body:     body,
		Priority: "high",
		AlertID:  alertID,
		DeviceID: deviceID,
	}

	return s.SendEmail(request)
}

// emailWorker processes emails from the queue
func (s *Service) emailWorker(workerID int) {
	defer s.wg.Done()

	s.logger.Debug("Email worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("Email worker stopping", zap.Int("worker_id", workerID))
			return

		case request := <-s.emailQueue:
			if request == nil {
				continue
			}

			s.metrics.SetEmailQueueSize(len(s.emailQueue))
			queueLatency := time.Since(request.CreatedAt)
			
			response := s.processEmail(request)
			
			if response.Success {
				s.sentCount++
				s.metrics.IncrementEmailsSent()
				s.logger.Info("Email sent successfully",
					zap.String("alert_id", request.AlertID),
					zap.String("device_id", request.DeviceID),
					zap.String("message_id", response.MessageID),
					zap.Duration("queue_latency", queueLatency),
					zap.Int("retries", response.Retries),
				)
			} else {
				s.failedCount++
				s.metrics.IncrementEmailsFailed()
				s.logger.Error("Email failed to send",
					zap.String("alert_id", request.AlertID),
					zap.String("device_id", request.DeviceID),
					zap.String("error", response.Error),
					zap.Int("retries", response.Retries),
				)
			}
		}
	}
}

// processEmail processes a single email with retry logic
func (s *Service) processEmail(request *EmailRequest) *EmailResponse {
	startTime := time.Now()
	
	var lastError error
	
	// Retry logic
	for attempt := 0; attempt <= s.config.RetryCount; attempt++ {
		if attempt > 0 {
			s.metrics.IncrementEmailRetry()
			s.retryCount++
			
			// Wait before retry
			select {
			case <-time.After(s.config.RetryDelay):
			case <-s.ctx.Done():
				return &EmailResponse{
					Success: false,
					Error:   "context cancelled during retry",
					SentAt:  time.Now(),
					Retries: attempt,
				}
			}
		}

		response, err := s.sendEmailOnce(request)
		if err == nil {
			// Success
			latency := time.Since(startTime)
			s.metrics.RecordEmailSendLatency(latency)
			
			response.Retries = attempt
			return response
		}

		lastError = err
		s.logger.Debug("Email send attempt failed",
			zap.Error(err),
			zap.String("alert_id", request.AlertID),
			zap.Int("attempt", attempt+1),
		)
	}

	// All retries failed
	return &EmailResponse{
		Success: false,
		Error:   lastError.Error(),
		SentAt:  time.Now(),
		Retries: s.config.RetryCount,
	}
}

// sendEmailOnce sends a single email (dummy implementation for development)
func (s *Service) sendEmailOnce(request *EmailRequest) (*EmailResponse, error) {
	if s.config.EnableDummy {
		// Dummy implementation - just log the email
		s.logger.Info("📧 DUMMY EMAIL SENT",
			zap.String("alert_id", request.AlertID),
			zap.String("device_id", request.DeviceID),
			zap.Strings("to", request.To),
			zap.String("subject", request.Subject),
			zap.String("priority", request.Priority),
			zap.Int("body_length", len(request.Body)),
		)

		// Simulate email sending delay
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

		// Simulate occasional failures (5% failure rate)
		if rand.Float64() < 0.05 {
			return nil, fmt.Errorf("dummy SMTP connection failed")
		}

		return &EmailResponse{
			Success:   true,
			MessageID: fmt.Sprintf("dummy_%d", time.Now().UnixNano()),
			SentAt:    time.Now(),
		}, nil
	}

	// Real SMTP implementation would go here
	// For now, return dummy success
	return &EmailResponse{
		Success:   true,
		MessageID: fmt.Sprintf("real_%d", time.Now().UnixNano()),
		SentAt:    time.Now(),
	}, nil
}

// generateAlertEmailBody generates email body for temperature alerts
func (s *Service) generateAlertEmailBody(alertID, deviceID string, temperature, threshold float64, region string) string {
	return fmt.Sprintf(`
🚨 TWINUP IoT Alert System 🚨

Alert Details:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🆔 Alert ID: %s
📟 Device ID: %s
🌡️  Current Temperature: %.2f°C
⚠️  Threshold: %.2f°C
🗺️  Region: %s
🕐 Detected At: %s

Severity: HIGH TEMPERATURE ALERT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Recommended Actions:
• Check device cooling system
• Verify sensor calibration  
• Inspect surrounding environment
• Consider maintenance if persistent

This is an automated alert from TWINUP Sensor System.
Please do not reply to this email.

Best regards,
TWINUP Alert System
	`, alertID, deviceID, temperature, threshold, region, time.Now().Format("2006-01-02 15:04:05"))
}

// loadTemplates loads email templates from directory
func (s *Service) loadTemplates() error {
	// For now, we'll use hardcoded templates
	// In production, this would load from template files
	
	alertTemplate := `
Subject: 🚨 TWINUP Alert: {{.AlertType}} - Device {{.DeviceID}}

Alert Details:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🆔 Alert ID: {{.AlertID}}
📟 Device ID: {{.DeviceID}}
🌡️  Temperature: {{.Temperature}}°C
⚠️  Threshold: {{.Threshold}}°C
🗺️  Region: {{.Region}}
🕐 Time: {{.Timestamp}}

Severity: {{.Severity}}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Message: {{.Message}}

Best regards,
TWINUP Alert System
	`

	tmpl, err := template.New("alert").Parse(alertTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse alert template: %w", err)
	}

	s.templates["alert"] = tmpl

	s.logger.Info("Email templates loaded successfully",
		zap.Int("template_count", len(s.templates)),
	)

	return nil
}

// GetStats returns email service statistics
func (s *Service) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uptime := time.Since(s.startTime)
	sendRate := float64(s.sentCount) / uptime.Seconds()
	successRate := float64(s.sentCount) / float64(s.sentCount+s.failedCount) * 100

	if s.sentCount+s.failedCount == 0 {
		successRate = 100.0
	}

	return map[string]interface{}{
		"sent_count":       s.sentCount,
		"failed_count":     s.failedCount,
		"retry_count":      s.retryCount,
		"send_rate":        sendRate,
		"success_rate_pct": successRate,
		"queue_size":       len(s.emailQueue),
		"is_running":       s.running,
		"uptime_seconds":   uptime.Seconds(),
		"dummy_mode":       s.config.EnableDummy,
	}
}

// HealthCheck checks email service health
func (s *Service) HealthCheck() error {
	if !s.IsRunning() {
		return fmt.Errorf("email service is not running")
	}

	// In dummy mode, always healthy
	if s.config.EnableDummy {
		return nil
	}

	// In real mode, test SMTP connection
	// For now, return success
	return nil
}

// IsRunning returns whether the service is currently running
func (s *Service) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	return s.running
}

