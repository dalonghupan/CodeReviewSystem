package data

import "time"

// 表结构模型（与 sql/002_git.sql DDL 字段一一对应）

// GitAuth Git平台OAuth授权绑定表
// AccessToken/RefreshToken 均为 AES 加密密文（LLD §9-4）
type GitAuth struct {
	AuthID           string     `db:"auth_id"`
	TenantID         string     `db:"tenant_id"`
	Platform         string     `db:"platform"` // gitlab / gitee / github
	PlatformUserID   string     `db:"platform_user_id"`
	PlatformUsername string     `db:"platform_username"`
	PlatformAvatar   string     `db:"platform_avatar"`
	AccessToken      string     `db:"access_token"`  // AES加密密文
	RefreshToken     string     `db:"refresh_token"` // AES加密密文
	TokenExpireTime  time.Time  `db:"token_expire_time"`
	SyncStatus       string     `db:"sync_status"` // idle / syncing / synced / error
	LastSyncedAt     *time.Time `db:"last_synced_at"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

// GitRepository 已绑定仓库表
type GitRepository struct {
	RepoID         string     `db:"repo_id"`
	TenantID       string     `db:"tenant_id"`
	AuthID         string     `db:"auth_id"`
	Platform       string     `db:"platform"`
	PlatformRepoID string     `db:"platform_repo_id"`
	FullName       string     `db:"full_name"`
	Description    string     `db:"description"`
	DefaultBranch  string     `db:"default_branch"`
	CloneURL       string     `db:"clone_url"`
	WebURL         string     `db:"web_url"`
	IsPrivate      bool       `db:"is_private"`
	IsBound        bool       `db:"is_bound"`
	SyncStatus     string     `db:"sync_status"`
	LastSyncedAt   *time.Time `db:"last_synced_at"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// RepoBlacklist 仓库黑名单
type RepoBlacklist struct {
	ID          string    `db:"id"`
	TenantID    string    `db:"tenant_id"`
	RepoPattern string    `db:"repo_pattern"`
	Reason      string    `db:"reason"`
	CreatedBy   *string   `db:"created_by"`
	CreatedAt   time.Time `db:"created_at"`
}

// SensitiveFileConfig 敏感文件后缀配置
type SensitiveFileConfig struct {
	ID            string    `db:"id"`
	TenantID      string    `db:"tenant_id"`
	FileExtension string    `db:"file_extension"`
	Description   string    `db:"description"`
	CreatedBy     *string   `db:"created_by"`
	CreatedAt     time.Time `db:"created_at"`
}
