package auth

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/twinup/sensor-system/services/authentication-service/pkg/logger"
)

// Permission represents a single permission
type Permission struct {
	Resource string `json:"resource"` // "devices", "analytics", "alerts", "users"
	Action   string `json:"action"`   // "read", "write", "delete", "admin"
}

// Role represents a user role with permissions
type Role struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	IsActive    bool         `json:"is_active"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// User represents a user with roles
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Roles        []string  `json:"roles"`
	IsActive     bool      `json:"is_active"`
	LastLogin    time.Time `json:"last_login"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	IsActive  bool      `json:"is_active"`
}

// RBAC handles role-based access control
type RBAC struct {
	roles   map[string]*Role
	users   map[string]*User
	cache   CacheService
	logger  logger.Logger
	metrics RBACMetricsRecorder
	mutex   sync.RWMutex
}

// CacheService interface for caching role-permission data
type CacheService interface {
	Get(key string) (string, error)
	Set(key string, value string, ttl time.Duration) error
	Delete(key string) error
}

// RBACMetricsRecorder interface for RBAC metrics collection
type RBACMetricsRecorder interface {
	IncrementRoleAssignments(role string)
	IncrementPermissionChecks(resource, action, result string)
	IncrementPermissionDenied(resource, action, role string)
	SetRoleCacheHitRate(rate float64)
}

// NewRBAC creates a new RBAC instance
func NewRBAC(cache CacheService, logger logger.Logger, metrics RBACMetricsRecorder) *RBAC {
	rbac := &RBAC{
		roles:   make(map[string]*Role),
		users:   make(map[string]*User),
		cache:   cache,
		logger:  logger,
		metrics: metrics,
	}

	// Initialize default roles
	rbac.initializeDefaultRoles()

	// Initialize demo users
	rbac.initializeDemoUsers()

	logger.Info("RBAC system initialized successfully",
		zap.Int("role_count", len(rbac.roles)),
		zap.Int("user_count", len(rbac.users)),
	)

	return rbac
}

// initializeDefaultRoles creates default system roles
func (r *RBAC) initializeDefaultRoles() {
	now := time.Now()

	// Admin role - full access
	adminRole := &Role{
		Name:        "admin",
		Description: "Full system administrator access",
		Permissions: []Permission{
			{Resource: "devices", Action: "admin"},
			{Resource: "analytics", Action: "admin"},
			{Resource: "alerts", Action: "admin"},
			{Resource: "users", Action: "admin"},
			{Resource: "system", Action: "admin"},
		},
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// User role - read access to devices and analytics
	userRole := &Role{
		Name:        "user",
		Description: "Standard user with read access",
		Permissions: []Permission{
			{Resource: "devices", Action: "read"},
			{Resource: "analytics", Action: "read"},
			{Resource: "alerts", Action: "read"},
		},
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Operator role - read/write access to devices and alerts
	operatorRole := &Role{
		Name:        "operator",
		Description: "Device operator with read/write access",
		Permissions: []Permission{
			{Resource: "devices", Action: "read"},
			{Resource: "devices", Action: "write"},
			{Resource: "analytics", Action: "read"},
			{Resource: "alerts", Action: "read"},
			{Resource: "alerts", Action: "write"},
		},
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Readonly role - read-only access
	readonlyRole := &Role{
		Name:        "readonly",
		Description: "Read-only access to all resources",
		Permissions: []Permission{
			{Resource: "devices", Action: "read"},
			{Resource: "analytics", Action: "read"},
			{Resource: "alerts", Action: "read"},
		},
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.roles["admin"] = adminRole
	r.roles["user"] = userRole
	r.roles["operator"] = operatorRole
	r.roles["readonly"] = readonlyRole
}

// initializeDemoUsers creates demo users for testing
func (r *RBAC) initializeDemoUsers() {
	now := time.Now()

	// Admin user
	adminUser := &User{
		ID:           "admin-001",
		Username:     "admin",
		Email:        "admin@twinup.com",
		PasswordHash: "$2a$10$dummy.hash.for.demo.purposes.only", // Dummy hash
		Roles:        []string{"admin"},
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Regular user
	regularUser := &User{
		ID:           "user-001",
		Username:     "user",
		Email:        "user@twinup.com",
		PasswordHash: "$2a$10$dummy.hash.for.demo.purposes.only", // Dummy hash
		Roles:        []string{"user"},
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Operator user
	operatorUser := &User{
		ID:           "operator-001",
		Username:     "operator",
		Email:        "operator@twinup.com",
		PasswordHash: "$2a$10$dummy.hash.for.demo.purposes.only", // Dummy hash
		Roles:        []string{"operator"},
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Readonly user
	readonlyUser := &User{
		ID:           "readonly-001",
		Username:     "readonly",
		Email:        "readonly@twinup.com",
		PasswordHash: "$2a$10$dummy.hash.for.demo.purposes.only", // Dummy hash
		Roles:        []string{"readonly"},
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	r.users["admin"] = adminUser
	r.users["user"] = regularUser
	r.users["operator"] = operatorUser
	r.users["readonly"] = readonlyUser
}

// GetUser retrieves a user by username
func (r *RBAC) GetUser(username string) (*User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	user, exists := r.users[username]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user is inactive: %s", username)
	}

	return user, nil
}

// GetUserRoles retrieves roles for a user with caching
func (r *RBAC) GetUserRoles(userID string) ([]string, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("user_roles:%s", userID)
	if cached, err := r.cache.Get(cacheKey); err == nil {
		var roles []string
		if err := json.Unmarshal([]byte(cached), &roles); err == nil {
			r.metrics.SetRoleCacheHitRate(100.0) // Cache hit
			return roles, nil
		}
	}

	// Cache miss - get from memory
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var user *User
	for _, u := range r.users {
		if u.ID == userID {
			user = u
			break
		}
	}

	if user == nil {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	// Cache the result
	rolesJSON, _ := json.Marshal(user.Roles)
	r.cache.Set(cacheKey, string(rolesJSON), 1*time.Hour)
	r.metrics.SetRoleCacheHitRate(0.0) // Cache miss

	return user.Roles, nil
}

// GetUserPermissions retrieves all permissions for a user
func (r *RBAC) GetUserPermissions(userID string) ([]Permission, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("user_permissions:%s", userID)
	if cached, err := r.cache.Get(cacheKey); err == nil {
		var permissions []Permission
		if err := json.Unmarshal([]byte(cached), &permissions); err == nil {
			return permissions, nil
		}
	}

	// Get user roles
	userRoles, err := r.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}

	// Collect all permissions from roles
	var allPermissions []Permission
	permissionMap := make(map[string]bool) // For deduplication

	r.mutex.RLock()
	for _, roleName := range userRoles {
		if role, exists := r.roles[roleName]; exists && role.IsActive {
			for _, permission := range role.Permissions {
				key := fmt.Sprintf("%s:%s", permission.Resource, permission.Action)
				if !permissionMap[key] {
					allPermissions = append(allPermissions, permission)
					permissionMap[key] = true
				}
			}
		}
	}
	r.mutex.RUnlock()

	// Cache the result
	permissionsJSON, _ := json.Marshal(allPermissions)
	r.cache.Set(cacheKey, string(permissionsJSON), 1*time.Hour)

	return allPermissions, nil
}

// HasPermission checks if a user has a specific permission
func (r *RBAC) HasPermission(userID, resource, action string) bool {
	permissions, err := r.GetUserPermissions(userID)
	if err != nil {
		r.metrics.IncrementPermissionChecks(resource, action, "error")
		r.logger.Error("Failed to get user permissions",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return false
	}

	// Check for specific permission
	for _, permission := range permissions {
		if permission.Resource == resource {
			// Check for exact match or admin permission
			if permission.Action == action || permission.Action == "admin" {
				r.metrics.IncrementPermissionChecks(resource, action, "granted")
				return true
			}
		}
	}

	// Check for global admin permission
	for _, permission := range permissions {
		if permission.Resource == "system" && permission.Action == "admin" {
			r.metrics.IncrementPermissionChecks(resource, action, "granted")
			return true
		}
	}

	r.metrics.IncrementPermissionChecks(resource, action, "denied")
	return false
}

// AssignRole assigns a role to a user
func (r *RBAC) AssignRole(userID, roleName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Verify role exists
	if _, exists := r.roles[roleName]; !exists {
		return fmt.Errorf("role not found: %s", roleName)
	}

	// Find user
	var user *User
	for _, u := range r.users {
		if u.ID == userID {
			user = u
			break
		}
	}

	if user == nil {
		return fmt.Errorf("user not found: %s", userID)
	}

	// Check if role already assigned
	for _, existingRole := range user.Roles {
		if existingRole == roleName {
			return nil // Already assigned
		}
	}

	// Assign role
	user.Roles = append(user.Roles, roleName)
	user.UpdatedAt = time.Now()

	// Invalidate cache
	r.invalidateUserCache(userID)

	r.metrics.IncrementRoleAssignments(roleName)
	r.logger.Info("Role assigned to user",
		zap.String("user_id", userID),
		zap.String("role", roleName),
	)

	return nil
}

// RemoveRole removes a role from a user
func (r *RBAC) RemoveRole(userID, roleName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Find user
	var user *User
	for _, u := range r.users {
		if u.ID == userID {
			user = u
			break
		}
	}

	if user == nil {
		return fmt.Errorf("user not found: %s", userID)
	}

	// Remove role
	var newRoles []string
	found := false
	for _, existingRole := range user.Roles {
		if existingRole != roleName {
			newRoles = append(newRoles, existingRole)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("role not assigned to user: %s", roleName)
	}

	user.Roles = newRoles
	user.UpdatedAt = time.Now()

	// Invalidate cache
	r.invalidateUserCache(userID)

	r.logger.Info("Role removed from user",
		zap.String("user_id", userID),
		zap.String("role", roleName),
	)

	return nil
}

// GetRole retrieves a role by name
func (r *RBAC) GetRole(roleName string) (*Role, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	role, exists := r.roles[roleName]
	if !exists {
		return nil, fmt.Errorf("role not found: %s", roleName)
	}

	if !role.IsActive {
		return nil, fmt.Errorf("role is inactive: %s", roleName)
	}

	return role, nil
}

// GetAllRoles retrieves all active roles
func (r *RBAC) GetAllRoles() []*Role {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var activeRoles []*Role
	for _, role := range r.roles {
		if role.IsActive {
			activeRoles = append(activeRoles, role)
		}
	}

	return activeRoles
}

// ValidateCredentials validates user credentials (dummy implementation)
func (r *RBAC) ValidateCredentials(username, password string) (*User, error) {
	user, err := r.GetUser(username)
	if err != nil {
		return nil, err
	}

	// Dummy password validation for demo
	// In production, this would use bcrypt or similar
	if password == "password" || password == "admin123" || password == username {
		user.LastLogin = time.Now()
		r.logger.Info("User credentials validated",
			zap.String("username", username),
			zap.String("user_id", user.ID),
		)
		return user, nil
	}

	return nil, fmt.Errorf("invalid credentials")
}

// invalidateUserCache invalidates cached data for a user
func (r *RBAC) invalidateUserCache(userID string) {
	r.cache.Delete(fmt.Sprintf("user_roles:%s", userID))
	r.cache.Delete(fmt.Sprintf("user_permissions:%s", userID))
}

// GetStats returns RBAC statistics
func (r *RBAC) GetStats() map[string]interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	activeUsers := 0
	activeRoles := 0

	for _, user := range r.users {
		if user.IsActive {
			activeUsers++
		}
	}

	for _, role := range r.roles {
		if role.IsActive {
			activeRoles++
		}
	}

	return map[string]interface{}{
		"total_users":  len(r.users),
		"active_users": activeUsers,
		"total_roles":  len(r.roles),
		"active_roles": activeRoles,
	}
}

// PermissionString converts permission to string format
func (p Permission) String() string {
	return fmt.Sprintf("%s:%s", p.Resource, p.Action)
}

// HasRole checks if a user has a specific role
func (r *RBAC) HasRole(userID, roleName string) bool {
	userRoles, err := r.GetUserRoles(userID)
	if err != nil {
		return false
	}

	for _, role := range userRoles {
		if role == roleName {
			return true
		}
	}

	return false
}

// IsAdmin checks if a user has admin role
func (r *RBAC) IsAdmin(userID string) bool {
	return r.HasRole(userID, "admin")
}
