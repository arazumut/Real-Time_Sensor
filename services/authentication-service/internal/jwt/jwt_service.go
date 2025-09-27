package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/authentication-service/pkg/logger"
)

// Claims represents JWT claims structure
type Claims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	SessionID   string   `json:"session_id"`
	TokenType   string   `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh token pair
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
	SessionID    string    `json:"session_id"`
}

// Service handles JWT operations
type Service struct {
	config  Config
	logger  logger.Logger
	metrics MetricsRecorder
}

// Config holds JWT service configuration
type Config struct {
	SecretKey             string
	AccessTokenTTL        time.Duration
	RefreshTokenTTL       time.Duration
	Issuer                string
	Audience              string
	Algorithm             string
	EnableRefreshTokens   bool
	MaxConcurrentSessions int
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementTokensIssued(tokenType string)
	IncrementTokensValidated(status string)
	IncrementTokensRevoked()
	RecordTokenValidationLatency(duration time.Duration)
	RecordTokenGenerationLatency(duration time.Duration)
}

// NewService creates a new JWT service
func NewService(config Config, logger logger.Logger, metrics MetricsRecorder) *Service {
	service := &Service{
		config:  config,
		logger:  logger,
		metrics: metrics,
	}

	logger.Info("JWT service created successfully",
		zap.String("issuer", config.Issuer),
		zap.String("audience", config.Audience),
		zap.Duration("access_ttl", config.AccessTokenTTL),
		zap.Duration("refresh_ttl", config.RefreshTokenTTL),
		zap.Bool("refresh_enabled", config.EnableRefreshTokens),
	)

	return service
}

// GenerateTokenPair generates access and refresh token pair
func (s *Service) GenerateTokenPair(userID, username, email string, roles, permissions []string) (*TokenPair, error) {
	startTime := time.Now()

	sessionID := uuid.New().String()
	now := time.Now()

	// Generate access token
	accessClaims := &Claims{
		UserID:      userID,
		Username:    username,
		Email:       email,
		Roles:       roles,
		Permissions: permissions,
		SessionID:   sessionID,
		TokenType:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    s.config.Issuer,
			Audience:  jwt.ClaimStrings{s.config.Audience},
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.AccessTokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	accessToken, err := s.generateToken(accessClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	tokenPair := &TokenPair{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.config.AccessTokenTTL.Seconds()),
		IssuedAt:    now,
		SessionID:   sessionID,
	}

	// Generate refresh token if enabled
	if s.config.EnableRefreshTokens {
		refreshClaims := &Claims{
			UserID:    userID,
			Username:  username,
			Email:     email,
			SessionID: sessionID,
			TokenType: "refresh",
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        uuid.New().String(),
				Issuer:    s.config.Issuer,
				Audience:  jwt.ClaimStrings{s.config.Audience},
				Subject:   userID,
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(s.config.RefreshTokenTTL)),
				NotBefore: jwt.NewNumericDate(now),
			},
		}

		refreshToken, err := s.generateToken(refreshClaims)
		if err != nil {
			return nil, fmt.Errorf("failed to generate refresh token: %w", err)
		}

		tokenPair.RefreshToken = refreshToken
	}

	// Record metrics
	generationTime := time.Since(startTime)
	s.metrics.RecordTokenGenerationLatency(generationTime)
	s.metrics.IncrementTokensIssued("access")
	if s.config.EnableRefreshTokens {
		s.metrics.IncrementTokensIssued("refresh")
	}

	s.logger.Debug("Token pair generated successfully",
		zap.String("user_id", userID),
		zap.String("session_id", sessionID),
		zap.Strings("roles", roles),
		zap.Duration("generation_time", generationTime),
	)

	return tokenPair, nil
}

// ValidateToken validates and parses a JWT token
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	startTime := time.Now()

	// Parse token with claims
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.SecretKey), nil
	})

	validationTime := time.Since(startTime)
	s.metrics.RecordTokenValidationLatency(validationTime)

	if err != nil {
		s.metrics.IncrementTokensValidated("invalid")
		s.logger.Debug("Token validation failed",
			zap.Error(err),
			zap.Duration("validation_time", validationTime),
		)
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		s.metrics.IncrementTokensValidated("invalid")
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate token type
	if claims.TokenType == "" {
		claims.TokenType = "access" // Default for backward compatibility
	}

	// Validate issuer and audience
	if claims.Issuer != s.config.Issuer {
		s.metrics.IncrementTokensValidated("invalid_issuer")
		return nil, fmt.Errorf("invalid token issuer")
	}

	// Validate audience
	validAudience := false
	for _, aud := range claims.Audience {
		if aud == s.config.Audience {
			validAudience = true
			break
		}
	}
	if !validAudience {
		s.metrics.IncrementTokensValidated("invalid_audience")
		return nil, fmt.Errorf("invalid token audience")
	}

	s.metrics.IncrementTokensValidated("valid")
	s.logger.Debug("Token validated successfully",
		zap.String("user_id", claims.UserID),
		zap.String("session_id", claims.SessionID),
		zap.String("token_type", claims.TokenType),
		zap.Duration("validation_time", validationTime),
	)

	return claims, nil
}

// RefreshToken generates a new access token using a refresh token
func (s *Service) RefreshToken(refreshTokenString string) (*TokenPair, error) {
	if !s.config.EnableRefreshTokens {
		return nil, fmt.Errorf("refresh tokens are disabled")
	}

	// Validate refresh token
	claims, err := s.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Verify it's a refresh token
	if claims.TokenType != "refresh" {
		return nil, fmt.Errorf("token is not a refresh token")
	}

	// Generate new token pair (keeping same session ID)
	tokenPair, err := s.GenerateTokenPair(
		claims.UserID,
		claims.Username,
		claims.Email,
		claims.Roles,
		claims.Permissions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new tokens: %w", err)
	}

	// Use the same session ID
	tokenPair.SessionID = claims.SessionID

	s.logger.Info("Token refreshed successfully",
		zap.String("user_id", claims.UserID),
		zap.String("session_id", claims.SessionID),
	)

	return tokenPair, nil
}

// ExtractClaims extracts claims from token without validation (for expired tokens)
func (s *Service) ExtractClaims(tokenString string) (*Claims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// generateToken generates a JWT token with the given claims
func (s *Service) generateToken(claims *Claims) (string, error) {
	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token with secret
	tokenString, err := token.SignedString([]byte(s.config.SecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateTokenType validates that the token is of the expected type
func (s *Service) ValidateTokenType(claims *Claims, expectedType string) error {
	if claims.TokenType != expectedType {
		return fmt.Errorf("expected %s token, got %s", expectedType, claims.TokenType)
	}
	return nil
}

// GetTokenInfo returns information about a token without validating signature
func (s *Service) GetTokenInfo(tokenString string) (map[string]interface{}, error) {
	claims, err := s.ExtractClaims(tokenString)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"user_id":    claims.UserID,
		"username":   claims.Username,
		"email":      claims.Email,
		"roles":      claims.Roles,
		"session_id": claims.SessionID,
		"token_type": claims.TokenType,
		"issued_at":  claims.IssuedAt.Time,
		"expires_at": claims.ExpiresAt.Time,
		"issuer":     claims.Issuer,
		"audience":   claims.Audience,
	}, nil
}
