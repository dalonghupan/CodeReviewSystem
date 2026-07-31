-- ============================================================
-- CR-System DDL: Git仓库授权与配置模块
-- 对应服务：git-adapter（HLD §2.1 / LLD §3.2）
-- 数据库：PostgreSQL 17
-- ============================================================

-- ============================================================
-- 1. Git授权绑定表（SRS F01-01：OAuth2.1授权，Token加密存储）
-- ============================================================
CREATE TABLE git_auth (
    auth_id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,                               -- 租户ID（不建外键，跨服务隔离）
    platform            VARCHAR(16) NOT NULL,                         -- gitlab / gitee / github
    platform_user_id    VARCHAR(128),                                 -- 平台侧用户ID
    platform_username   VARCHAR(128),                                 -- 平台用户名
    platform_avatar     VARCHAR(512) DEFAULT '',
    access_token        TEXT NOT NULL,                                -- 加密存储的Access Token（AES）
    refresh_token       TEXT,                                         -- 加密存储的Refresh Token
    token_expire_time   TIMESTAMPTZ NOT NULL,                         -- Token过期时间
    sync_status         VARCHAR(16) NOT NULL DEFAULT 'idle',          -- idle / syncing / synced / error
    last_synced_at      TIMESTAMPTZ,                                  -- 最后同步时间
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE git_auth IS 'Git平台OAuth授权绑定表，Token加密存储，定时自动刷新（LLD §3.2）';
COMMENT ON COLUMN git_auth.access_token IS 'AES加密存储，禁止明文';
COMMENT ON COLUMN git_auth.sync_status IS '同步状态：idle空闲 / syncing同步中 / synced已同步 / error异常';

CREATE INDEX idx_git_auth_tenant ON git_auth(tenant_id);
CREATE INDEX idx_git_auth_platform ON git_auth(tenant_id, platform);
CREATE INDEX idx_git_auth_expire ON git_auth(token_expire_time);

-- ============================================================
-- 2. 绑定仓库表（SRS F01-02：授权后拉取仓库信息）
-- ============================================================
CREATE TABLE git_repository (
    repo_id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    auth_id             UUID NOT NULL,                                -- 关联的授权记录
    platform            VARCHAR(16) NOT NULL,
    platform_repo_id    VARCHAR(128) NOT NULL,                        -- 平台侧仓库ID
    full_name           VARCHAR(256) NOT NULL,                        -- 仓库全名 org/repo
    description         TEXT DEFAULT '',
    default_branch      VARCHAR(64) NOT NULL DEFAULT 'main',
    clone_url           VARCHAR(512),
    web_url             VARCHAR(512),
    is_private          BOOLEAN NOT NULL DEFAULT TRUE,
    is_bound            BOOLEAN NOT NULL DEFAULT TRUE,                -- 是否绑定（解绑保留记录）
    sync_status         VARCHAR(16) NOT NULL DEFAULT 'idle',
    last_synced_at      TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE git_repository IS '已绑定仓库表，纳入评审管理范围的仓库';

CREATE INDEX idx_git_repo_tenant ON git_repository(tenant_id);
CREATE INDEX idx_git_repo_platform ON git_repository(tenant_id, platform);
CREATE UNIQUE INDEX idx_git_repo_platform_id ON git_repository(platform, platform_repo_id, tenant_id);
CREATE INDEX idx_git_repo_bound ON git_repository(tenant_id, is_bound);

-- ============================================================
-- 3. 仓库黑名单表（SRS F01-04：黑名单仓库禁止创建评审单）
-- ============================================================
CREATE TABLE repo_blacklist (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    repo_pattern        VARCHAR(256) NOT NULL,                        -- 仓库名匹配模式（支持通配符 *）
    reason              TEXT DEFAULT '',                               -- 加入黑名单原因
    created_by          UUID,                                         -- 创建人
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE repo_blacklist IS '仓库黑名单，命中后禁止创建评审单（SRS F01-04）';

CREATE INDEX idx_blacklist_tenant ON repo_blacklist(tenant_id);
CREATE UNIQUE INDEX idx_blacklist_pattern ON repo_blacklist(tenant_id, repo_pattern);

-- ============================================================
-- 4. 敏感文件后缀配置表（SRS F01-04：敏感文件Diff内容屏蔽展示）
-- ============================================================
CREATE TABLE sensitive_file_config (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    file_extension      VARCHAR(32) NOT NULL,                         -- 文件后缀，如 .pem / .key / .env
    description         VARCHAR(128) DEFAULT '',                      -- 说明
    created_by          UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE sensitive_file_config IS '敏感文件后缀配置，命中后Diff内容屏蔽展示（SRS F01-04）';

CREATE INDEX idx_sensitive_tenant ON sensitive_file_config(tenant_id);
CREATE UNIQUE INDEX idx_sensitive_ext ON sensitive_file_config(tenant_id, file_extension);

-- 插入默认敏感文件后缀
INSERT INTO sensitive_file_config (tenant_id, file_extension, description) VALUES
    ('00000000-0000-0000-0000-000000000000', '.pem',    'SSL证书私钥'),
    ('00000000-0000-0000-0000-000000000000', '.key',    '密钥文件'),
    ('00000000-0000-0000-0000-000000000000', '.env',    '环境变量文件'),
    ('00000000-0000-0000-0000-000000000000', '.p12',    'PKCS12证书'),
    ('00000000-0000-0000-0000-000000000000', '.pfx',    'PFX证书'),
    ('00000000-0000-0000-0000-000000000000', '.keystore', 'Java密钥库');
