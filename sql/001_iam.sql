-- ============================================================
-- CR-System DDL: IAM 身份权限租户模块
-- 对应服务：iam-service（HLD §2.6 / LLD §3.1）
-- 数据库：PostgreSQL 17
-- ============================================================

-- 启用UUID扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- 1. 租户信息表
-- ============================================================
CREATE TABLE tenant (
    tenant_id       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(128) NOT NULL,                           -- 租户名称
    description     TEXT DEFAULT '',                                 -- 租户描述
    contact_email   VARCHAR(256),                                    -- 联系邮箱
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,                   -- 是否启用
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE tenant IS '租户信息表';
COMMENT ON COLUMN tenant.is_active IS '租户启用状态，禁用后该租户下所有用户无法登录';

CREATE UNIQUE INDEX idx_tenant_name ON tenant(name) WHERE is_active = TRUE;

-- ============================================================
-- 2. 用户账号表
-- ============================================================
CREATE TABLE sys_user (
    user_id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenant(tenant_id),
    username        VARCHAR(128) NOT NULL,                           -- 用户名（Keycloak sub）
    display_name    VARCHAR(128) NOT NULL DEFAULT '',                -- 显示名称
    email           VARCHAR(256),
    phone           VARCHAR(32),
    avatar_url      VARCHAR(512) DEFAULT '',
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,                   -- 是否启用（离职禁用）
    last_login_at   TIMESTAMPTZ,                                     -- 最后登录时间
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE sys_user IS '系统用户表，与Keycloak账号一一对应';

CREATE INDEX idx_sys_user_tenant ON sys_user(tenant_id);
CREATE UNIQUE INDEX idx_sys_user_username ON sys_user(tenant_id, username);
CREATE INDEX idx_sys_user_email ON sys_user(email);

-- ============================================================
-- 3. Keycloak账号映射表
-- ============================================================
CREATE TABLE keycloak_user_mapping (
    mapping_id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id             UUID NOT NULL REFERENCES sys_user(user_id),
    keycloak_sub        VARCHAR(256) NOT NULL,                       -- Keycloak Subject ID
    keycloak_username   VARCHAR(128) NOT NULL,
    keycloak_realm      VARCHAR(64) NOT NULL DEFAULT 'cr-system',
    last_synced_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),          -- 最后同步时间
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE keycloak_user_mapping IS 'Keycloak账号与系统用户映射，支持多Realm';

CREATE UNIQUE INDEX idx_kc_mapping_user ON keycloak_user_mapping(user_id);
CREATE UNIQUE INDEX idx_kc_mapping_sub ON keycloak_user_mapping(keycloak_sub, keycloak_realm);

-- ============================================================
-- 4. 角色表（SRS F03-02：预设四类角色）
-- ============================================================
CREATE TABLE sys_role (
    role_id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenant(tenant_id),
    name            VARCHAR(64) NOT NULL,                            -- 角色标识
    display_name    VARCHAR(128) NOT NULL DEFAULT '',                -- 角色显示名
    description     TEXT DEFAULT '',
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,                  -- 是否系统预设（不可删除）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE sys_role IS '角色表，预设 super_admin / review_leader / developer / guest';

CREATE INDEX idx_sys_role_tenant ON sys_role(tenant_id);
CREATE UNIQUE INDEX idx_sys_role_name ON sys_role(tenant_id, name);

-- 插入系统预设角色（tenant_id 为全局空UUID表示系统级角色模板）
INSERT INTO sys_role (role_id, tenant_id, name, display_name, description, is_system) VALUES
    ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000000', 'super_admin',    '超级管理员', '拥有系统全部权限，管理租户、用户、配置', TRUE),
    ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000000', 'review_leader',  '评审组长',   '管理评审流程、查看所有评审单、配置门禁规则', TRUE),
    ('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000000', 'developer',      '普通开发人员', '创建评审单、提交代码评论、提交复审',     TRUE),
    ('00000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000000', 'guest',          '只读访客',   '仅查看评审详情和统计数据，无操作权限',     TRUE);

-- ============================================================
-- 5. 权限资源表
-- ============================================================
CREATE TABLE sys_permission (
    permission_id   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource_type   VARCHAR(64) NOT NULL,                            -- 资源类型: tenant / repo / review / report / user / config
    action          VARCHAR(32) NOT NULL,                            -- 操作: read / write / delete / admin
    display_name    VARCHAR(128) NOT NULL DEFAULT '',
    description     TEXT DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE sys_permission IS 'RBAC权限资源定义表';

CREATE UNIQUE INDEX idx_sys_permission_res_act ON sys_permission(resource_type, action);

-- ============================================================
-- 6. 用户角色关联表
-- ============================================================
CREATE TABLE sys_user_role (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES sys_user(user_id) ON DELETE CASCADE,
    role_id         UUID NOT NULL REFERENCES sys_role(role_id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL REFERENCES tenant(tenant_id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE sys_user_role IS '用户-角色多对多关联';

CREATE UNIQUE INDEX idx_user_role_unique ON sys_user_role(user_id, role_id);
CREATE INDEX idx_user_role_tenant ON sys_user_role(tenant_id);
CREATE INDEX idx_user_role_user ON sys_user_role(user_id);

-- ============================================================
-- 7. 角色权限关联表
-- ============================================================
CREATE TABLE sys_role_permission (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    role_id         UUID NOT NULL REFERENCES sys_role(role_id) ON DELETE CASCADE,
    permission_id   UUID NOT NULL REFERENCES sys_permission(permission_id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE sys_role_permission IS '角色-权限多对多关联';

CREATE UNIQUE INDEX idx_role_perm_unique ON sys_role_permission(role_id, permission_id);
CREATE INDEX idx_role_perm_role ON sys_role_permission(role_id);
