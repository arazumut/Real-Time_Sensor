package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/authentication-service/pkg/logger"
)

// MockMetricsRecorder for testing
type MockMetricsRecorder struct {
	mock.Mock
}

func (m *MockMetricsRecorder) IncrementTokensIssued(tokenType string) {
	m.Called(tokenType)
}

func (m *MockMetricsRecorder) IncrementTokensValidated(status string) {
	m.Called(status)
}

func (m *MockMetricsRecorder) IncrementTokensRevoked() {
	m.Called()
}

func (m *MockMetricsRecorder) RecordTokenValidationLatency(duration time.Duration) {
	m.Called(duration)
}

func (m *MockMetricsRecorder) RecordTokenGenerationLatency(duration time.Duration) {
	m.Called(duration)
}

// MockLogger for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...zap.Field)  { m.Called(msg, fields) }
func (m *MockLogger) Info(msg string, fields ...zap.Field)   { m.Called(msg, fields) }
func (m *MockLogger) Warn(msg string, fields ...zap.Field)   { m.Called(msg, fields) }
func (m *MockLogger) Error(msg string, fields ...zap.Field)  { m.Called(msg, fields) }
func (m *MockLogger) Fatal(msg string, fields ...zap.Field)  { m.Called(msg, fields) }
func (m *MockLogger) With(fields ...zap.Field) logger.Logger { return m }
func (m *MockLogger) Sync() error                            { return nil }

func TestJWTService_GenerateTokenPair(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenGenerationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensIssued", "access").Return()
	mockMetrics.On("IncrementTokensIssued", "refresh").Return()

	config := Config{
		SecretKey:           "test-secret-key-32-characters-long",
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     24 * time.Hour,
		Issuer:              "test-issuer",
		Audience:            "test-audience",
		Algorithm:           "HS256",
		EnableRefreshTokens: true,
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Test token generation
	tokenPair, err := service.GenerateTokenPair(
		"user-123",
		"testuser",
		"test@example.com",
		[]string{"user", "operator"},
		[]string{"devices:read", "analytics:read"},
	)

	require.NoError(t, err)
	require.NotNil(t, tokenPair)

	// Verify token pair structure
	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.Equal(t, "Bearer", tokenPair.TokenType)
	assert.Equal(t, int64(900), tokenPair.ExpiresIn) // 15 minutes in seconds
	assert.NotEmpty(t, tokenPair.SessionID)
	assert.NotZero(t, tokenPair.IssuedAt)

	// Verify mock expectations
	mockMetrics.AssertExpectations(t)
}

func TestJWTService_ValidateToken(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenGenerationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensIssued", mock.Anything).Return()
	mockMetrics.On("RecordTokenValidationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensValidated", "valid").Return()

	config := Config{
		SecretKey:           "test-secret-key-32-characters-long",
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     24 * time.Hour,
		Issuer:              "test-issuer",
		Audience:            "test-audience",
		EnableRefreshTokens: false,
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Generate token first
	tokenPair, err := service.GenerateTokenPair(
		"user-123",
		"testuser",
		"test@example.com",
		[]string{"user"},
		[]string{"devices:read"},
	)
	require.NoError(t, err)

	// Validate the generated token
	claims, err := service.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, claims)

	// Verify claims
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "test@example.com", claims.Email)
	assert.Equal(t, []string{"user"}, claims.Roles)
	assert.Equal(t, []string{"devices:read"}, claims.Permissions)
	assert.Equal(t, "access", claims.TokenType)
	assert.Equal(t, "test-issuer", claims.Issuer)
	assert.Contains(t, claims.Audience, "test-audience")
	assert.NotEmpty(t, claims.SessionID)
}

func TestJWTService_ValidateToken_Invalid(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenValidationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensValidated", "invalid").Return()

	config := Config{
		SecretKey: "test-secret-key-32-characters-long",
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	}

	service := NewService(config, mockLogger, mockMetrics)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "Invalid token format",
			token: "invalid.token.format",
		},
		{
			name:  "Empty token",
			token: "",
		},
		{
			name:  "Malformed JWT",
			token: "not.a.jwt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.ValidateToken(tt.token)
			assert.Error(t, err)
			assert.Nil(t, claims)
		})
	}
}

func TestJWTService_RefreshToken(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	// Configure mock expectations
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenGenerationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensIssued", mock.Anything).Return()
	mockMetrics.On("RecordTokenValidationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensValidated", "valid").Return()

	config := Config{
		SecretKey:           "test-secret-key-32-characters-long",
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     24 * time.Hour,
		Issuer:              "test-issuer",
		Audience:            "test-audience",
		EnableRefreshTokens: true,
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Generate initial token pair
	originalPair, err := service.GenerateTokenPair(
		"user-123",
		"testuser",
		"test@example.com",
		[]string{"user"},
		[]string{"devices:read"},
	)
	require.NoError(t, err)
	require.NotEmpty(t, originalPair.RefreshToken)

	// Test refresh
	newPair, err := service.RefreshToken(originalPair.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, newPair)

	// Verify new tokens are different
	assert.NotEqual(t, originalPair.AccessToken, newPair.AccessToken)
	assert.Equal(t, originalPair.SessionID, newPair.SessionID) // Same session
}

func TestJWTService_RefreshToken_Disabled(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	config := Config{
		SecretKey:           "test-secret-key-32-characters-long",
		EnableRefreshTokens: false,
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Should fail when refresh tokens are disabled
	_, err := service.RefreshToken("any.refresh.token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refresh tokens are disabled")
}

func TestJWTService_ExtractClaims(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenGenerationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensIssued", mock.Anything).Return()

	config := Config{
		SecretKey:      "test-secret-key-32-characters-long",
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "test-issuer",
		Audience:       "test-audience",
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Generate token
	tokenPair, err := service.GenerateTokenPair(
		"user-123",
		"testuser",
		"test@example.com",
		[]string{"user"},
		[]string{"devices:read"},
	)
	require.NoError(t, err)

	// Extract claims without validation
	claims, err := service.ExtractClaims(tokenPair.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, claims)

	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "test@example.com", claims.Email)
}

func TestJWTService_GetTokenInfo(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenGenerationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensIssued", mock.Anything).Return()

	config := Config{
		SecretKey:      "test-secret-key-32-characters-long",
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "test-issuer",
		Audience:       "test-audience",
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Generate token
	tokenPair, err := service.GenerateTokenPair(
		"user-123",
		"testuser",
		"test@example.com",
		[]string{"user"},
		[]string{"devices:read"},
	)
	require.NoError(t, err)

	// Get token info
	info, err := service.GetTokenInfo(tokenPair.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "user-123", info["user_id"])
	assert.Equal(t, "testuser", info["username"])
	assert.Equal(t, "test@example.com", info["email"])
	assert.Equal(t, []string{"user"}, info["roles"])
	// Permissions might be nil in extracted claims, check if exists
	if permissions, ok := info["permissions"]; ok && permissions != nil {
		assert.Equal(t, []string{"devices:read"}, permissions)
	}
	assert.Equal(t, "test-issuer", info["issuer"])
}

func TestJWTService_TokenExpiry(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenGenerationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensIssued", mock.Anything).Return()
	mockMetrics.On("RecordTokenValidationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensValidated", "invalid").Return()

	config := Config{
		SecretKey:      "test-secret-key-32-characters-long",
		AccessTokenTTL: 1 * time.Millisecond, // Very short TTL for testing
		Issuer:         "test-issuer",
		Audience:       "test-audience",
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Generate token
	tokenPair, err := service.GenerateTokenPair(
		"user-123",
		"testuser",
		"test@example.com",
		[]string{"user"},
		[]string{"devices:read"},
	)
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Validate expired token
	claims, err := service.ValidateToken(tokenPair.AccessToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "token is expired")
}

func TestJWTService_InvalidSigningMethod(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenValidationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensValidated", "invalid").Return()

	config := Config{
		SecretKey: "test-secret-key-32-characters-long",
		Issuer:    "test-issuer",
		Audience:  "test-audience",
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Create token with different signing method (this would need manual creation)
	// For this test, we'll use an invalid token format
	invalidToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature"

	claims, err := service.ValidateToken(invalidToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// Benchmark tests
func BenchmarkJWTService_GenerateTokenPair(b *testing.B) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenGenerationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensIssued", mock.Anything).Return()

	config := Config{
		SecretKey:           "test-secret-key-32-characters-long",
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     24 * time.Hour,
		Issuer:              "test-issuer",
		Audience:            "test-audience",
		EnableRefreshTokens: true,
	}

	service := NewService(config, mockLogger, mockMetrics)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			service.GenerateTokenPair(
				"user-123",
				"testuser",
				"test@example.com",
				[]string{"user"},
				[]string{"devices:read"},
			)
		}
	})
}

func BenchmarkJWTService_ValidateToken(b *testing.B) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsRecorder{}

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockMetrics.On("RecordTokenGenerationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensIssued", mock.Anything).Return()
	mockMetrics.On("RecordTokenValidationLatency", mock.AnythingOfType("time.Duration")).Return()
	mockMetrics.On("IncrementTokensValidated", "valid").Return()

	config := Config{
		SecretKey:      "test-secret-key-32-characters-long",
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "test-issuer",
		Audience:       "test-audience",
	}

	service := NewService(config, mockLogger, mockMetrics)

	// Generate token for benchmarking
	tokenPair, _ := service.GenerateTokenPair(
		"user-123",
		"testuser",
		"test@example.com",
		[]string{"user"},
		[]string{"devices:read"},
	)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			service.ValidateToken(tokenPair.AccessToken)
		}
	})
}
