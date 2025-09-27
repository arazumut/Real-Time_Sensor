package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the authentication service
type Metrics struct {
	// Authentication metrics
	LoginAttemptsTotal *prometheus.CounterVec
	LoginSuccessTotal  prometheus.Counter
	LoginFailedTotal   *prometheus.CounterVec
	LogoutTotal        prometheus.Counter

	// JWT metrics
	TokensIssuedTotal      *prometheus.CounterVec
	TokensValidatedTotal   *prometheus.CounterVec
	TokensRevokedTotal     prometheus.Counter
	TokenValidationLatency prometheus.Histogram
	TokenGenerationLatency prometheus.Histogram

	// Session metrics
	ActiveSessionsGauge     prometheus.Gauge
	SessionDuration         prometheus.Histogram
	SessionTimeoutTotal     prometheus.Counter
	ConcurrentSessionsGauge *prometheus.GaugeVec

	// Role and permission metrics
	RoleAssignmentsTotal  *prometheus.CounterVec
	PermissionChecksTotal *prometheus.CounterVec
	PermissionDeniedTotal *prometheus.CounterVec
	RoleCacheHitRate      prometheus.Gauge

	// Cache metrics
	CacheHitRate           prometheus.Gauge
	CacheSetLatency        prometheus.Histogram
	CacheGetLatency        prometheus.Histogram
	CacheConnectionsActive prometheus.Gauge
	CacheErrorsTotal       prometheus.Counter

	// HTTP metrics
	HTTPRequestsTotal  *prometheus.CounterVec
	HTTPRequestLatency *prometheus.HistogramVec
	HTTPResponseSize   prometheus.Histogram

	// Rate limiting metrics
	RateLimitHitsTotal   *prometheus.CounterVec
	RateLimitBypassTotal prometheus.Counter

	// Security metrics
	FailedLoginAttemptsTotal *prometheus.CounterVec
	SuspiciousActivityTotal  *prometheus.CounterVec
	BruteForceAttemptsTotal  prometheus.Counter

	// System metrics
	MemoryUsageGauge prometheus.Gauge
	CPUUsageGauge    prometheus.Gauge
	GoroutinesGauge  prometheus.Gauge
	ProcessingRate   prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	metrics := &Metrics{
		LoginAttemptsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_login_attempts_total",
			Help: "Total number of login attempts",
		}, []string{"method", "user_agent"}),

		LoginSuccessTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "auth_login_success_total",
			Help: "Total number of successful logins",
		}),

		LoginFailedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_login_failed_total",
			Help: "Total number of failed login attempts",
		}, []string{"reason"}),

		LogoutTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "auth_logout_total",
			Help: "Total number of logout operations",
		}),

		TokensIssuedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_tokens_issued_total",
			Help: "Total number of JWT tokens issued",
		}, []string{"token_type"}),

		TokensValidatedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_tokens_validated_total",
			Help: "Total number of JWT tokens validated",
		}, []string{"status"}),

		TokensRevokedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "auth_tokens_revoked_total",
			Help: "Total number of JWT tokens revoked",
		}),

		TokenValidationLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "auth_token_validation_latency_ms",
			Help:    "JWT token validation latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}),

		TokenGenerationLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "auth_token_generation_latency_ms",
			Help:    "JWT token generation latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		ActiveSessionsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_active_sessions",
			Help: "Number of active user sessions",
		}),

		SessionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "auth_session_duration_minutes",
			Help:    "User session duration in minutes",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}),

		SessionTimeoutTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "auth_session_timeout_total",
			Help: "Total number of session timeouts",
		}),

		ConcurrentSessionsGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "auth_concurrent_sessions",
			Help: "Number of concurrent sessions per user",
		}, []string{"user_id"}),

		RoleAssignmentsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_role_assignments_total",
			Help: "Total number of role assignments",
		}, []string{"role"}),

		PermissionChecksTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_permission_checks_total",
			Help: "Total number of permission checks",
		}, []string{"resource", "action", "result"}),

		PermissionDeniedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_permission_denied_total",
			Help: "Total number of permission denials",
		}, []string{"resource", "action", "role"}),

		RoleCacheHitRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_role_cache_hit_rate",
			Help: "Role cache hit rate percentage",
		}),

		CacheHitRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_cache_hit_rate",
			Help: "General cache hit rate percentage",
		}),

		CacheSetLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "auth_cache_set_latency_ms",
			Help:    "Cache SET operation latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}),

		CacheGetLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "auth_cache_get_latency_ms",
			Help:    "Cache GET operation latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}),

		CacheConnectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_cache_connections_active",
			Help: "Number of active cache connections",
		}),

		CacheErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "auth_cache_errors_total",
			Help: "Total number of cache errors",
		}),

		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "endpoint", "status"}),

		HTTPRequestLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "auth_http_request_latency_ms",
			Help:    "HTTP request latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 15),
		}, []string{"method", "endpoint"}),

		HTTPResponseSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "auth_http_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 2, 15),
		}),

		RateLimitHitsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		}, []string{"endpoint", "client_ip"}),

		RateLimitBypassTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "auth_rate_limit_bypass_total",
			Help: "Total number of rate limit bypasses",
		}),

		FailedLoginAttemptsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_failed_login_attempts_total",
			Help: "Total number of failed login attempts by IP",
		}, []string{"client_ip", "username"}),

		SuspiciousActivityTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_suspicious_activity_total",
			Help: "Total number of suspicious activities detected",
		}, []string{"activity_type", "client_ip"}),

		BruteForceAttemptsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "auth_brute_force_attempts_total",
			Help: "Total number of brute force attempts detected",
		}),

		MemoryUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),

		CPUUsageGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_cpu_usage_percent",
			Help: "CPU usage percentage",
		}),

		GoroutinesGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_goroutines",
			Help: "Number of goroutines",
		}),

		ProcessingRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "auth_processing_rate_per_second",
			Help: "Authentication processing rate per second",
		}),
	}

	// Register all metrics
	prometheus.MustRegister(
		metrics.LoginAttemptsTotal,
		metrics.LoginSuccessTotal,
		metrics.LoginFailedTotal,
		metrics.LogoutTotal,
		metrics.TokensIssuedTotal,
		metrics.TokensValidatedTotal,
		metrics.TokensRevokedTotal,
		metrics.TokenValidationLatency,
		metrics.TokenGenerationLatency,
		metrics.ActiveSessionsGauge,
		metrics.SessionDuration,
		metrics.SessionTimeoutTotal,
		metrics.ConcurrentSessionsGauge,
		metrics.RoleAssignmentsTotal,
		metrics.PermissionChecksTotal,
		metrics.PermissionDeniedTotal,
		metrics.RoleCacheHitRate,
		metrics.CacheHitRate,
		metrics.CacheSetLatency,
		metrics.CacheGetLatency,
		metrics.CacheConnectionsActive,
		metrics.CacheErrorsTotal,
		metrics.HTTPRequestsTotal,
		metrics.HTTPRequestLatency,
		metrics.HTTPResponseSize,
		metrics.RateLimitHitsTotal,
		metrics.RateLimitBypassTotal,
		metrics.FailedLoginAttemptsTotal,
		metrics.SuspiciousActivityTotal,
		metrics.BruteForceAttemptsTotal,
		metrics.MemoryUsageGauge,
		metrics.CPUUsageGauge,
		metrics.GoroutinesGauge,
		metrics.ProcessingRate,
	)

	return metrics
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(port int, path string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return server
}

// Authentication Metrics Methods
func (m *Metrics) IncrementLoginAttempts(method, userAgent string) {
	m.LoginAttemptsTotal.WithLabelValues(method, userAgent).Inc()
}

func (m *Metrics) IncrementLoginSuccess() {
	m.LoginSuccessTotal.Inc()
}

func (m *Metrics) IncrementLoginFailed(reason string) {
	m.LoginFailedTotal.WithLabelValues(reason).Inc()
}

func (m *Metrics) IncrementLogout() {
	m.LogoutTotal.Inc()
}

// JWT Metrics Methods
func (m *Metrics) IncrementTokensIssued(tokenType string) {
	m.TokensIssuedTotal.WithLabelValues(tokenType).Inc()
}

func (m *Metrics) IncrementTokensValidated(status string) {
	m.TokensValidatedTotal.WithLabelValues(status).Inc()
}

func (m *Metrics) IncrementTokensRevoked() {
	m.TokensRevokedTotal.Inc()
}

func (m *Metrics) RecordTokenValidationLatency(duration time.Duration) {
	m.TokenValidationLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordTokenGenerationLatency(duration time.Duration) {
	m.TokenGenerationLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

// Session Metrics Methods
func (m *Metrics) SetActiveSessions(count int) {
	m.ActiveSessionsGauge.Set(float64(count))
}

func (m *Metrics) RecordSessionDuration(duration time.Duration) {
	m.SessionDuration.Observe(duration.Minutes())
}

func (m *Metrics) IncrementSessionTimeout() {
	m.SessionTimeoutTotal.Inc()
}

func (m *Metrics) SetConcurrentSessions(userID string, count int) {
	m.ConcurrentSessionsGauge.WithLabelValues(userID).Set(float64(count))
}

// Role and Permission Metrics Methods
func (m *Metrics) IncrementRoleAssignments(role string) {
	m.RoleAssignmentsTotal.WithLabelValues(role).Inc()
}

func (m *Metrics) IncrementPermissionChecks(resource, action, result string) {
	m.PermissionChecksTotal.WithLabelValues(resource, action, result).Inc()
}

func (m *Metrics) IncrementPermissionDenied(resource, action, role string) {
	m.PermissionDeniedTotal.WithLabelValues(resource, action, role).Inc()
}

func (m *Metrics) SetRoleCacheHitRate(rate float64) {
	m.RoleCacheHitRate.Set(rate)
}

// Cache Metrics Methods
func (m *Metrics) SetCacheHitRate(rate float64) {
	m.CacheHitRate.Set(rate)
}

func (m *Metrics) RecordCacheSetLatency(duration time.Duration) {
	m.CacheSetLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordCacheGetLatency(duration time.Duration) {
	m.CacheGetLatency.Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) SetCacheConnectionsActive(count int) {
	m.CacheConnectionsActive.Set(float64(count))
}

func (m *Metrics) IncrementCacheErrors() {
	m.CacheErrorsTotal.Inc()
}

// HTTP Metrics Methods
func (m *Metrics) IncrementHTTPRequests(method, endpoint, status string) {
	m.HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
}

func (m *Metrics) RecordHTTPLatency(method, endpoint string, duration time.Duration) {
	m.HTTPRequestLatency.WithLabelValues(method, endpoint).Observe(float64(duration.Nanoseconds()) / 1e6)
}

func (m *Metrics) RecordHTTPResponseSize(size int) {
	m.HTTPResponseSize.Observe(float64(size))
}

// Rate Limiting Metrics Methods
func (m *Metrics) IncrementRateLimitHits(endpoint, clientIP string) {
	m.RateLimitHitsTotal.WithLabelValues(endpoint, clientIP).Inc()
}

func (m *Metrics) IncrementRateLimitBypass() {
	m.RateLimitBypassTotal.Inc()
}

// Security Metrics Methods
func (m *Metrics) IncrementFailedLoginAttempts(clientIP, username string) {
	m.FailedLoginAttemptsTotal.WithLabelValues(clientIP, username).Inc()
}

func (m *Metrics) IncrementSuspiciousActivity(activityType, clientIP string) {
	m.SuspiciousActivityTotal.WithLabelValues(activityType, clientIP).Inc()
}

func (m *Metrics) IncrementBruteForceAttempts() {
	m.BruteForceAttemptsTotal.Inc()
}

// System Metrics Methods
func (m *Metrics) UpdateSystemMetrics(memoryBytes, cpuPercent float64, goroutines int) {
	m.MemoryUsageGauge.Set(memoryBytes)
	m.CPUUsageGauge.Set(cpuPercent)
	m.GoroutinesGauge.Set(float64(goroutines))
}

func (m *Metrics) SetProcessingRate(rate float64) {
	m.ProcessingRate.Set(rate)
}
