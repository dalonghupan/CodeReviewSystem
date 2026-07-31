package data

import "time"

// 表结构模型（与 sql/001_iam.sql DDL 字段一一对应）

// Tenant 租户信息表
type Tenant struct {
	TenantID     string    `db:"tenant_id"`
	Name         string    `db:"name"`
	Description  string    `db:"description"`
	ContactEmail string    `db:"contact_email"`
	IsActive     bool      `db:"is_active"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// User 系统用户表（与 Keycloak 账号一一对应）
type User struct {
	UserID      string     `db:"user_id"`
	TenantID    string     `db:"tenant_id"`
	Username    string     `db:"username"` // Keycloak sub
	DisplayName string     `db:"display_name"`
	Email       string     `db:"email"`
	Phone       string     `db:"phone"`
	AvatarURL   string     `db:"avatar_url"`
	IsActive    bool       `db:"is_active"`
	LastLoginAt *time.Time `db:"last_login_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// Role 角色表
type Role struct {
	RoleID      string    `db:"role_id"`
	TenantID    string    `db:"tenant_id"`
	Name        string    `db:"name"`
	DisplayName string    `db:"display_name"`
	Description string    `db:"description"`
	IsSystem    bool      `db:"is_system"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Permission 权限资源表
type Permission struct {
	PermissionID string    `db:"permission_id"`
	ResourceType string    `db:"resource_type"`
	Action       string    `db:"action"`
	DisplayName  string    `db:"display_name"`
	Description  string    `db:"description"`
	CreatedAt    time.Time `db:"created_at"`
}

// KeycloakMapping Keycloak账号映射表
type KeycloakMapping struct {
	MappingID        string    `db:"mapping_id"`
	UserID           string    `db:"user_id"`
	KeycloakSub      string    `db:"keycloak_sub"`
	KeycloakUsername string    `db:"keycloak_username"`
	KeycloakRealm    string    `db:"keycloak_realm"`
	LastSyncedAt     time.Time `db:"last_synced_at"`
	CreatedAt        time.Time `db:"created_at"`
}
