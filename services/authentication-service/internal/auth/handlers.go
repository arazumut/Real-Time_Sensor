package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/authentication-service/internal/cache"
	"github.com/twinup/sensor-system/services/authentication-service/internal/jwt"
	"github.com/twinup/sensor-system/services/authentication-service/pkg/logger"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresIn    int64     `json:"expires_in,omitempty"`
	User         *UserInfo `json:"user,omitempty"`
	SessionID    string    `json:"session_id,omitempty"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// VerifyResponse represents a token verification response
type VerifyResponse struct {
	Valid     bool      `json:"valid"`
	User      *UserInfo `json:"user,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
}

// UserInfo represents user information in responses
type UserInfo struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	LastLogin   time.Time `json:"last_login,omitempty"`
}

// LogoutRequest represents a logout request
type LogoutRequest struct {
	SessionID string `json:"session_id,omitempty"`
}

// Handlers contains all HTTP handlers for authentication
type Handlers struct {
	jwtService   *jwt.Service
	rbac         *RBAC
	cacheService *cache.Service
	logger       logger.Logger
	metrics      MetricsRecorder
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementLoginAttempts(method, userAgent string)
	IncrementLoginSuccess()
	IncrementLoginFailed(reason string)
	IncrementLogout()
	IncrementTokensIssued(tokenType string)
	IncrementTokensValidated(status string)
	IncrementTokensRevoked()
	IncrementFailedLoginAttempts(clientIP, username string)
	SetActiveSessions(count int)
	RecordSessionDuration(duration time.Duration)
}

// NewHandlers creates new authentication handlers
func NewHandlers(
	jwtService *jwt.Service,
	rbac *RBAC,
	cacheService *cache.Service,
	logger logger.Logger,
	metrics MetricsRecorder,
) *Handlers {
	return &Handlers{
		jwtService:   jwtService,
		rbac:         rbac,
		cacheService: cacheService,
		logger:       logger,
		metrics:      metrics,
	}
}

// Login handles user login
func (h *Handlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	// Record login attempt
	h.metrics.IncrementLoginAttempts("password", userAgent)

	h.logger.Info("Login attempt",
		zap.String("username", req.Username),
		zap.String("client_ip", clientIP),
		zap.String("user_agent", userAgent),
	)

	// Validate credentials
	user, err := h.rbac.ValidateCredentials(req.Username, req.Password)
	if err != nil {
		h.metrics.IncrementLoginFailed("invalid_credentials")
		h.metrics.IncrementFailedLoginAttempts(clientIP, req.Username)

		h.logger.Warn("Login failed",
			zap.String("username", req.Username),
			zap.String("client_ip", clientIP),
			zap.Error(err),
		)

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_credentials",
			"message": "Invalid username or password",
		})
		return
	}

	// Get user permissions
	permissions, err := h.rbac.GetUserPermissions(user.ID)
	if err != nil {
		h.logger.Error("Failed to get user permissions",
			zap.Error(err),
			zap.String("user_id", user.ID),
		)
		permissions = []Permission{} // Empty permissions on error
	}

	// Convert permissions to strings
	var permissionStrings []string
	for _, perm := range permissions {
		permissionStrings = append(permissionStrings, perm.String())
	}

	// Generate JWT tokens
	tokenPair, err := h.jwtService.GenerateTokenPair(
		user.ID,
		user.Username,
		user.Email,
		user.Roles,
		permissionStrings,
	)
	if err != nil {
		h.logger.Error("Failed to generate tokens",
			zap.Error(err),
			zap.String("user_id", user.ID),
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "token_generation_failed",
			"message": "Failed to generate authentication tokens",
		})
		return
	}

	// Store session in cache
	sessionData := map[string]interface{}{
		"user_id":    user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"roles":      user.Roles,
		"ip_address": clientIP,
		"user_agent": userAgent,
		"login_time": time.Now(),
	}

	if err := h.cacheService.SetUserSession(tokenPair.SessionID, user.ID, sessionData, 24*time.Hour); err != nil {
		h.logger.Error("Failed to store session",
			zap.Error(err),
			zap.String("session_id", tokenPair.SessionID),
		)
	}

	// Update metrics
	h.metrics.IncrementLoginSuccess()

	// Prepare response
	userInfo := &UserInfo{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Roles:       user.Roles,
		Permissions: permissionStrings,
		LastLogin:   user.LastLogin,
	}

	response := &LoginResponse{
		Success:      true,
		Message:      "Login successful",
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		TokenType:    tokenPair.TokenType,
		ExpiresIn:    tokenPair.ExpiresIn,
		User:         userInfo,
		SessionID:    tokenPair.SessionID,
	}

	h.logger.Info("Login successful",
		zap.String("user_id", user.ID),
		zap.String("username", user.Username),
		zap.Strings("roles", user.Roles),
		zap.String("session_id", tokenPair.SessionID),
	)

	c.JSON(http.StatusOK, response)
}

// RefreshToken handles token refresh
func (h *Handlers) RefreshToken(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
		return
	}

	// Refresh token
	tokenPair, err := h.jwtService.RefreshToken(req.RefreshToken)
	if err != nil {
		h.logger.Debug("Token refresh failed", zap.Error(err))

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "invalid_refresh_token",
			"message": "Invalid or expired refresh token",
		})
		return
	}

	h.logger.Info("Token refreshed successfully",
		zap.String("session_id", tokenPair.SessionID),
	)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Token refreshed successfully",
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"token_type":    tokenPair.TokenType,
		"expires_in":    tokenPair.ExpiresIn,
		"session_id":    tokenPair.SessionID,
	})
}

// VerifyToken handles token verification
func (h *Handlers) VerifyToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_authorization",
			"message": "Authorization header is required",
		})
		return
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_authorization",
			"message": "Authorization header must be 'Bearer <token>'",
		})
		return
	}

	claims, err := h.jwtService.ValidateToken(tokenParts[1])
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
		})
		return
	}

	// Get user permissions
	permissions, _ := h.rbac.GetUserPermissions(claims.UserID)
	var permissionStrings []string
	for _, perm := range permissions {
		permissionStrings = append(permissionStrings, perm.String())
	}

	userInfo := &UserInfo{
		ID:          claims.UserID,
		Username:    claims.Username,
		Email:       claims.Email,
		Roles:       claims.Roles,
		Permissions: permissionStrings,
	}

	response := &VerifyResponse{
		Valid:     true,
		User:      userInfo,
		ExpiresAt: claims.ExpiresAt.Time,
		SessionID: claims.SessionID,
	}

	c.JSON(http.StatusOK, response)
}

// Logout handles user logout
func (h *Handlers) Logout(c *gin.Context) {
	sessionID, exists := c.Get("session_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "no_session",
			"message": "No active session found",
		})
		return
	}

	sessionIDStr := sessionID.(string)

	// Get token claims for revocation
	claims, exists := c.Get("token_claims")
	if exists {
		if jwtClaims, ok := claims.(*jwt.Claims); ok {
			// Add token to revoked list
			h.cacheService.SetRevokedToken(jwtClaims.ID, jwtClaims.ExpiresAt.Time)
			h.metrics.IncrementTokensRevoked()
		}
	}

	// Remove session from cache
	if err := h.cacheService.DeleteUserSession(sessionIDStr); err != nil {
		h.logger.Error("Failed to delete session",
			zap.Error(err),
			zap.String("session_id", sessionIDStr),
		)
	}

	h.metrics.IncrementLogout()

	userID, _ := c.Get("user_id")
	h.logger.Info("User logged out",
		zap.String("user_id", userID.(string)),
		zap.String("session_id", sessionIDStr),
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logout successful",
	})
}

// GetProfile returns current user profile
func (h *Handlers) GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	email, _ := c.Get("email")
	roles, _ := c.Get("roles")
	permissions, _ := c.Get("permissions")

	userInfo := &UserInfo{
		ID:          userID.(string),
		Username:    username.(string),
		Email:       email.(string),
		Roles:       roles.([]string),
		Permissions: permissions.([]string),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    userInfo,
	})
}

// GetRoles returns all available roles (admin only)
func (h *Handlers) GetRoles(c *gin.Context) {
	roles := h.rbac.GetAllRoles()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"roles":   roles,
		"count":   len(roles),
	})
}

// HealthCheck returns service health status
func (h *Handlers) HealthCheck(c *gin.Context) {
	// Check cache health
	cacheHealthy := true
	if err := h.cacheService.HealthCheck(); err != nil {
		cacheHealthy = false
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !cacheHealthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":        status,
		"timestamp":     time.Now(),
		"cache_healthy": cacheHealthy,
		"version":       "1.0.0",
	})
}
