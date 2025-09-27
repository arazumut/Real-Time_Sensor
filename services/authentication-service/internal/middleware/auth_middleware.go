package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/authentication-service/internal/auth"
	"github.com/twinup/sensor-system/services/authentication-service/internal/jwt"
	"github.com/twinup/sensor-system/services/authentication-service/pkg/logger"
)

// AuthMiddleware handles JWT authentication and authorization
type AuthMiddleware struct {
	jwtService *jwt.Service
	rbac       *auth.RBAC
	logger     logger.Logger
	metrics    MetricsRecorder
}

// MetricsRecorder interface for metrics collection
type MetricsRecorder interface {
	IncrementTokensValidated(status string)
	IncrementPermissionChecks(resource, action, result string)
	IncrementPermissionDenied(resource, action, role string)
	RecordTokenValidationLatency(duration time.Duration)
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(jwtService *jwt.Service, rbac *auth.RBAC, logger logger.Logger, metrics MetricsRecorder) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		rbac:       rbac,
		logger:     logger,
		metrics:    metrics,
	}
}

// RequireAuth middleware that requires valid JWT authentication
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.logger.Debug("Missing Authorization header",
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method),
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "missing_authorization",
				"message": "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Check Bearer token format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			m.logger.Debug("Invalid Authorization header format",
				zap.String("header", authHeader),
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_authorization",
				"message": "Authorization header must be 'Bearer <token>'",
			})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Validate token
		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			m.logger.Debug("Token validation failed",
				zap.Error(err),
				zap.String("path", c.Request.URL.Path),
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Verify token type
		if claims.TokenType != "access" {
			m.logger.Debug("Invalid token type",
				zap.String("token_type", claims.TokenType),
				zap.String("expected", "access"),
			)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token_type",
				"message": "Access token required",
			})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("roles", claims.Roles)
		c.Set("permissions", claims.Permissions)
		c.Set("session_id", claims.SessionID)
		c.Set("token_claims", claims)

		// Record metrics
		validationTime := time.Since(startTime)
		m.metrics.RecordTokenValidationLatency(validationTime)

		m.logger.Debug("Authentication successful",
			zap.String("user_id", claims.UserID),
			zap.String("username", claims.Username),
			zap.Strings("roles", claims.Roles),
			zap.Duration("validation_time", validationTime),
		)

		c.Next()
	}
}

// RequireRole middleware that requires specific role
func (m *AuthMiddleware) RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user roles from context
		rolesInterface, exists := c.Get("roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "no_roles",
				"message": "User roles not found",
			})
			c.Abort()
			return
		}

		roles, ok := rolesInterface.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "invalid_roles",
				"message": "Invalid roles format",
			})
			c.Abort()
			return
		}

		// Check if user has required role
		hasRole := false
		for _, role := range roles {
			if role == requiredRole || role == "admin" { // Admin has access to everything
				hasRole = true
				break
			}
		}

		if !hasRole {
			userID, _ := c.Get("user_id")
			m.logger.Warn("Role access denied",
				zap.String("user_id", userID.(string)),
				zap.String("required_role", requiredRole),
				zap.Strings("user_roles", roles),
				zap.String("path", c.Request.URL.Path),
			)

			c.JSON(http.StatusForbidden, gin.H{
				"error":   "insufficient_role",
				"message": fmt.Sprintf("Role '%s' required", requiredRole),
			})
			c.Abort()
			return
		}

		m.logger.Debug("Role check passed",
			zap.String("required_role", requiredRole),
			zap.Strings("user_roles", roles),
		)

		c.Next()
	}
}

// RequirePermission middleware that requires specific permission
func (m *AuthMiddleware) RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "no_user",
				"message": "User ID not found",
			})
			c.Abort()
			return
		}

		userIDStr := userID.(string)

		// Check permission
		hasPermission := m.rbac.HasPermission(userIDStr, resource, action)
		if !hasPermission {
			roles, _ := c.Get("roles")
			m.metrics.IncrementPermissionDenied(resource, action, fmt.Sprintf("%v", roles))

			m.logger.Warn("Permission denied",
				zap.String("user_id", userIDStr),
				zap.String("resource", resource),
				zap.String("action", action),
				zap.String("path", c.Request.URL.Path),
			)

			c.JSON(http.StatusForbidden, gin.H{
				"error":   "insufficient_permission",
				"message": fmt.Sprintf("Permission '%s:%s' required", resource, action),
			})
			c.Abort()
			return
		}

		m.logger.Debug("Permission check passed",
			zap.String("user_id", userIDStr),
			zap.String("resource", resource),
			zap.String("action", action),
		)

		c.Next()
	}
}

// OptionalAuth middleware that optionally authenticates but doesn't require it
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No authentication provided, continue without user context
			c.Next()
			return
		}

		// Try to authenticate
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) == 2 && tokenParts[0] == "Bearer" {
			claims, err := m.jwtService.ValidateToken(tokenParts[1])
			if err == nil && claims.TokenType == "access" {
				// Set user context if valid
				c.Set("user_id", claims.UserID)
				c.Set("username", claims.Username)
				c.Set("email", claims.Email)
				c.Set("roles", claims.Roles)
				c.Set("permissions", claims.Permissions)
				c.Set("session_id", claims.SessionID)
				c.Set("authenticated", true)
			}
		}

		c.Next()
	}
}

// AdminOnly middleware that requires admin role
func (m *AuthMiddleware) AdminOnly() gin.HandlerFunc {
	return m.RequireRole("admin")
}

// UserOrAdmin middleware that requires user or admin role
func (m *AuthMiddleware) UserOrAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		rolesInterface, exists := c.Get("roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "no_roles",
				"message": "User roles not found",
			})
			c.Abort()
			return
		}

		roles, ok := rolesInterface.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "invalid_roles",
				"message": "Invalid roles format",
			})
			c.Abort()
			return
		}

		// Check for user, operator, or admin role
		hasValidRole := false
		for _, role := range roles {
			if role == "user" || role == "operator" || role == "admin" {
				hasValidRole = true
				break
			}
		}

		if !hasValidRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "insufficient_role",
				"message": "User, operator, or admin role required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserFromContext extracts user information from Gin context
func GetUserFromContext(c *gin.Context) (map[string]interface{}, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return nil, fmt.Errorf("user not authenticated")
	}

	username, _ := c.Get("username")
	email, _ := c.Get("email")
	roles, _ := c.Get("roles")
	permissions, _ := c.Get("permissions")
	sessionID, _ := c.Get("session_id")

	return map[string]interface{}{
		"user_id":     userID,
		"username":    username,
		"email":       email,
		"roles":       roles,
		"permissions": permissions,
		"session_id":  sessionID,
	}, nil
}

// IsAuthenticated checks if the current request is authenticated
func IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("user_id")
	return exists
}

// HasRole checks if the current user has a specific role
func HasRole(c *gin.Context, roleName string) bool {
	rolesInterface, exists := c.Get("roles")
	if !exists {
		return false
	}

	roles, ok := rolesInterface.([]string)
	if !ok {
		return false
	}

	for _, role := range roles {
		if role == roleName || role == "admin" {
			return true
		}
	}

	return false
}
