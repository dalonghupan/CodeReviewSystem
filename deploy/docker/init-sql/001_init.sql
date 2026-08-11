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

-- 系统级占位租户（全零UUID）：预设角色模板的归属，满足外键约束；非业务租户，禁用状态不可登录
INSERT INTO tenant (tenant_id, name, description, is_active) VALUES
    ('00000000-0000-0000-0000-000000000000', 'system_reserved', '系统级角色模板归属占位租户，非业务租户', FALSE)
ON CONFLICT (tenant_id) DO NOTHING;

-- 插入系统预设角色（tenant_id 为全局空UUID表示系统级角色模板）
INSERT INTO sys_role (role_id, tenant_id, name, display_name, description, is_system) VALUES
    ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000000', 'super_admin',    '超级管理员', '拥有系统全部权限，管理租户、用户、配置', TRUE),
    ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000000', 'review_leader',  '评审组长',   '管理评审流程、查看所有评审单、配置门禁规则', TRUE),
    ('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000000', 'developer',      '普通开发人员', '创建评审单、提交代码评论、提交复审',     TRUE),
    ('00000000-0000-0000-0000-000000000004', '00000000-0000-0000-0000-000000000000', 'guest',          '只读访客',   '仅查看评审详情和统计数据，无操作权限',     TRUE);

-- ============================================================
-- 开发环境预置数据：默认租户 + 测试用户（与 Keycloak realm 三个账号对应）
-- 登录页租户号填：00000000-0000-0000-0000-000000000001
-- ============================================================
INSERT INTO tenant (tenant_id, name, description, contact_email) VALUES
    ('00000000-0000-0000-0000-000000000001', '默认租户', '开发环境预置租户', 'admin@cr-system.local')
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO sys_user (user_id, tenant_id, username, display_name, email) VALUES
    ('00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000001', 'admin',     '系统管理员', 'admin@cr-system.local'),
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000001', 'developer', '开发者',     'developer@cr-system.local'),
    ('00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000001', 'reviewer',  '评审人',     'reviewer@cr-system.local')
ON CONFLICT DO NOTHING;

INSERT INTO sys_user_role (user_id, role_id, tenant_id) VALUES
    ('00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001'),  -- admin → super_admin
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001'),  -- developer → developer
    ('00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001')   -- reviewer → review_leader
ON CONFLICT DO NOTHING;

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
-- ============================================================
-- CR-System DDL: 评审核心业务模块
-- 对应服务：cr-core（HLD §2.2 / LLD §3.3）
-- 数据库：PostgreSQL 17
-- ============================================================

-- ============================================================
-- 1. 评审主单表（LLD §5.1）
-- ============================================================
CREATE TABLE review_main (
    review_id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    repo_id             UUID NOT NULL,                                -- 关联仓库ID
    mr_id               VARCHAR(64) NOT NULL,                         -- 平台侧MR编号
    git_platform        VARCHAR(16) NOT NULL,                         -- gitlab / gitee / github
    title               VARCHAR(512) NOT NULL,                        -- 评审标题
    description         TEXT DEFAULT '',                              -- 评审描述
    creator_uid         UUID NOT NULL,                                -- 创建人UID
    priority            SMALLINT NOT NULL DEFAULT 2,                  -- 1低 2普通 3高 4紧急
    status              SMALLINT NOT NULL DEFAULT 1,                  -- 1待评审 2评审中 3驳回 4复审 5通过 6归档
    source_branch       VARCHAR(128),                                 -- 源分支
    target_branch       VARCHAR(128),                                 -- 目标分支
    commit_count        INT DEFAULT 0,                                -- 提交数量
    changed_file_count  INT DEFAULT 0,                                -- 变更文件数
    related_issue       VARCHAR(256) DEFAULT '',                      -- 关联业务工单号（SRS F02-01）
    sonar_pass_flag     BOOLEAN DEFAULT FALSE,                        -- Sonar门禁是否通过（SRS F06-02）
    deadline            TIMESTAMPTZ,                                  -- 评审截止时间
    archive_time        TIMESTAMPTZ,                                  -- 归档时间
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE review_main IS '评审主单表，一条记录对应一次MR评审（LLD §5.1）';
COMMENT ON COLUMN review_main.status IS '1=待评审 2=评审中 3=驳回待修改 4=提交复审 5=复审通过 6=归档关闭';
COMMENT ON COLUMN review_main.priority IS '1=低 2=普通 3=高 4=紧急';

CREATE INDEX idx_review_tenant ON review_main(tenant_id);
CREATE INDEX idx_review_repo ON review_main(repo_id);
CREATE INDEX idx_review_mr ON review_main(mr_id);
CREATE UNIQUE INDEX idx_review_mr_unique ON review_main(repo_id, mr_id, status) WHERE status != 6;
CREATE INDEX idx_review_creator ON review_main(creator_uid);
CREATE INDEX idx_review_status ON review_main(tenant_id, status);
CREATE INDEX idx_review_deadline ON review_main(deadline);
CREATE INDEX idx_review_created ON review_main(created_at);

-- ============================================================
-- 2. 评审人关联表（支持多人评审）
-- ============================================================
CREATE TABLE review_reviewer (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    review_id           UUID NOT NULL REFERENCES review_main(review_id) ON DELETE CASCADE,
    reviewer_uid        UUID NOT NULL,                                -- 评审人UID
    review_status       VARCHAR(16) NOT NULL DEFAULT 'pending',       -- pending / approved / rejected
    review_comment      TEXT DEFAULT '',                              -- 评审结论评语
    reviewed_at         TIMESTAMPTZ,                                  -- 评审完成时间
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE review_reviewer IS '评审人关联表，支持指定多位评审人';

CREATE INDEX idx_reviewer_review ON review_reviewer(review_id);
CREATE INDEX idx_reviewer_user ON review_reviewer(reviewer_uid);
CREATE UNIQUE INDEX idx_reviewer_unique ON review_reviewer(review_id, reviewer_uid);

-- ============================================================
-- 3. 代码评论分表（LLD §5.2 / LLD §3.3：comment_yyyyMM 按月分表）
-- 使用 PostgreSQL 声明式分区实现自动按月分表
-- ============================================================
CREATE TABLE review_comment (
    comment_id          UUID NOT NULL DEFAULT uuid_generate_v4(),
    review_id           UUID NOT NULL,
    file_path           VARCHAR(512) NOT NULL,                        -- 文件路径（三元组之一）
    line_num            INT NOT NULL,                                 -- 代码行号（三元组之二）
    commit_version      VARCHAR(64) NOT NULL,                         -- commit hash（三元组之三）
    content             TEXT NOT NULL,                                -- 评论内容（Markdown）
    defect_level        SMALLINT NOT NULL DEFAULT 0,                  -- 0无 1致命 2严重 3一般 4建议
    reply_parent_id     UUID,                                         -- 回复父评论ID（NULL表示顶层评论）
    create_uid          UUID NOT NULL,
    is_isolated         BOOLEAN NOT NULL DEFAULT FALSE,               -- 是否已被隔离（版本迭代后旧评论）
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (comment_id, created_at)
) PARTITION BY RANGE (created_at);

COMMENT ON TABLE review_comment IS '代码行评论表，按月分区（LLD §3.3 评论分表策略 comment_yyyyMM）';
COMMENT ON COLUMN review_comment.defect_level IS '0=非缺陷 1=致命P0 2=严重P1 3=一般P2 4=建议P3（SRS F02-03）';
COMMENT ON COLUMN review_comment.is_isolated IS '代码版本迭代后旧版本评论自动隔离，防止行号错乱（SRS F02-03）';

-- 创建初始分区（当前月和下个月），后续由定时任务自动创建
CREATE TABLE review_comment_202607 PARTITION OF review_comment
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE review_comment_202608 PARTITION OF review_comment
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');

CREATE INDEX idx_comment_review ON review_comment(review_id);
CREATE INDEX idx_comment_file ON review_comment(review_id, file_path);
CREATE INDEX idx_comment_parent ON review_comment(reply_parent_id) WHERE reply_parent_id IS NOT NULL;
CREATE INDEX idx_comment_creator ON review_comment(create_uid);
CREATE INDEX idx_comment_defect ON review_comment(defect_level) WHERE defect_level > 0;

-- ============================================================
-- 4. 复审记录表（SRS F02-05：驳回后必须手动发起复审）
-- ============================================================
CREATE TABLE review_re_review_log (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    review_id           UUID NOT NULL REFERENCES review_main(review_id),
    action              VARCHAR(32) NOT NULL,                         -- start / reject / resubmit / approve / archive
    from_status         SMALLINT NOT NULL,                            -- 操作前状态
    to_status           SMALLINT NOT NULL,                            -- 操作后状态
    operator_uid        UUID NOT NULL,                                -- 操作人
    remark              TEXT DEFAULT '',                               -- 操作备注（驳回原因/复审说明等）
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE review_re_review_log IS '评审状态流转日志，全程记录不可篡改（SRS F02-06）';

CREATE INDEX idx_re_review_log_review ON review_re_review_log(review_id);
CREATE INDEX idx_re_review_log_time ON review_re_review_log(created_at);

-- ============================================================
-- 5. Sonar扫描关联表（SRS F06系列）
-- ============================================================
CREATE TABLE review_sonar_result (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    review_id           UUID NOT NULL REFERENCES review_main(review_id),
    project_key         VARCHAR(256),                                 -- SonarQube项目Key
    quality_gate_status VARCHAR(16),                                  -- OK / ERROR
    issues_json         JSONB DEFAULT '[]',                           -- 问题列表JSON（LLD §4.3 JSONB策略）
    metrics_json        JSONB DEFAULT '{}',                           -- 指标数据JSON（覆盖率/重复率/BUG数等）
    scanned_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE review_sonar_result IS 'SonarQube扫描结果关联表（SRS F06-01）';
COMMENT ON COLUMN review_sonar_result.issues_json IS 'JSONB存储问题列表，包含severity/type/filePath/lineNum/message';
COMMENT ON COLUMN review_sonar_result.metrics_json IS 'JSONB存储指标：coverage/duplications/bugs/vulnerabilities/code_smells';

CREATE INDEX idx_sonar_review ON review_sonar_result(review_id);

-- ============================================================
-- 6. 自定义门禁规则表（SRS F06-02：租户可自定义阻断规则）
-- ============================================================
CREATE TABLE sonar_gate_rule (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    rule_name           VARCHAR(128) NOT NULL,                        -- 规则名称
    severity_levels     JSONB NOT NULL DEFAULT '["BLOCKER","CRITICAL"]', -- 阻断的严重级别列表
    is_enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE sonar_gate_rule IS 'SonarQube门禁阻断规则，致命/严重级别漏洞未修复时禁止放行（SRS F06-02）';

CREATE INDEX idx_gate_rule_tenant ON sonar_gate_rule(tenant_id);
-- ============================================================
-- CR-System DDL: 质量统计与缺陷台账模块
-- 对应服务：quality-stat（HLD §2.3 / LLD §3.5）
-- 数据库：PostgreSQL 17
-- ============================================================

-- ============================================================
-- 1. 缺陷台账表（LLD §5.4 / SRS F05-01）
-- ============================================================
CREATE TABLE defect_record (
    defect_id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    review_id           UUID NOT NULL,                                -- 关联评审单ID
    comment_id          UUID NOT NULL,                                -- 关联评论ID
    tenant_id           UUID NOT NULL,
    file_path           VARCHAR(512),                                 -- 缺陷所在文件
    line_num            INT,                                          -- 缺陷所在行号
    defect_level        SMALLINT NOT NULL,                            -- 1致命 2严重 3一般 4建议
    content             TEXT NOT NULL,                                -- 缺陷描述内容
    module_name         VARCHAR(128) DEFAULT '',                      -- 所属代码模块
    creator_uid         UUID NOT NULL,                                -- 缺陷标记人
    is_fixed            BOOLEAN NOT NULL DEFAULT FALSE,               -- 是否已修复
    fixed_by_uid        UUID,                                         -- 修复人
    fix_time            TIMESTAMPTZ,                                  -- 修复时间
    stat_month          VARCHAR(7) NOT NULL,                          -- 统计归属月份 yyyy-MM
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE defect_record IS '缺陷台账表，自动从评论中归集标记为缺陷的记录（SRS F05-01 / LLD §5.4）';
COMMENT ON COLUMN defect_record.defect_level IS '1=致命P0 2=严重P1 3=一般P2 4=建议P3';
COMMENT ON COLUMN defect_record.stat_month IS '统计归属月份，格式yyyy-MM，用于月度统计';

CREATE INDEX idx_defect_tenant ON defect_record(tenant_id);
CREATE INDEX idx_defect_review ON defect_record(review_id);
CREATE INDEX idx_defect_level ON defect_record(tenant_id, defect_level);
CREATE INDEX idx_defect_fixed ON defect_record(tenant_id, is_fixed);
CREATE INDEX idx_defect_month ON defect_record(stat_month);
CREATE INDEX idx_defect_module ON defect_record(tenant_id, module_name);
CREATE INDEX idx_defect_created ON defect_record(created_at);

-- ============================================================
-- 2. 月度统计数据表（LLD §3.5：每日凌晨定时计算）
-- ============================================================
CREATE TABLE quality_monthly_stat (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    stat_month          VARCHAR(7) NOT NULL,                          -- 统计月份 yyyy-MM
    user_id             UUID,                                         -- 用户维度（NULL表示租户整体）
    module_name         VARCHAR(128) DEFAULT '',                      -- 模块维度（空表示整体）

    -- 评审指标
    total_reviews       INT NOT NULL DEFAULT 0,                       -- 评审单总数
    completed_reviews   INT NOT NULL DEFAULT 0,                       -- 已完成评审数
    avg_review_hours    DECIMAL(8,2) DEFAULT 0,                       -- 平均评审耗时(小时)

    -- 缺陷指标
    total_defects       INT NOT NULL DEFAULT 0,                       -- 缺陷总数
    fixed_defects       INT NOT NULL DEFAULT 0,                       -- 已修复数
    fatal_defects       INT NOT NULL DEFAULT 0,                       -- 致命P0数
    critical_defects    INT NOT NULL DEFAULT 0,                       -- 严重P1数
    major_defects       INT NOT NULL DEFAULT 0,                       -- 一般P2数
    minor_defects       INT NOT NULL DEFAULT 0,                       -- 建议P3数
    avg_fix_hours       DECIMAL(8,2) DEFAULT 0,                       -- 平均修复时长(小时)

    -- 计算指标
    completion_rate     DECIMAL(5,2) DEFAULT 0,                       -- 评审完成率(%)
    fix_timely_rate     DECIMAL(5,2) DEFAULT 0,                       -- 缺陷修复及时率(%)
    tech_debt_score     DECIMAL(8,2) DEFAULT 0,                       -- 技术债务分值

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE quality_monthly_stat IS '月度质量统计表，定时任务每日凌晨计算（LLD §3.5）';

CREATE UNIQUE INDEX idx_monthly_stat_unique ON quality_monthly_stat(tenant_id, stat_month, user_id, module_name);
CREATE INDEX idx_monthly_stat_tenant ON quality_monthly_stat(tenant_id);
CREATE INDEX idx_monthly_stat_month ON quality_monthly_stat(stat_month);
CREATE INDEX idx_monthly_stat_user ON quality_monthly_stat(user_id) WHERE user_id IS NOT NULL;

-- ============================================================
-- 3. 报表记录表（SRS F05-03：报表存储至MinIO）
-- ============================================================
CREATE TABLE quality_report (
    report_id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    report_type         VARCHAR(16) NOT NULL,                         -- daily / weekly / monthly
    format              VARCHAR(8) NOT NULL,                          -- excel / pdf
    title               VARCHAR(256) NOT NULL,                        -- 报表标题
    file_key            VARCHAR(512),                                 -- MinIO对象Key
    file_url            VARCHAR(1024),                                -- MinIO文件访问URL
    file_size           BIGINT DEFAULT 0,                             -- 文件大小(bytes)
    status              VARCHAR(16) NOT NULL DEFAULT 'generating',    -- generating / ready / failed
    error_message       TEXT DEFAULT '',                              -- 生成失败原因
    creator_uid         UUID NOT NULL,
    stat_month          VARCHAR(7),                                   -- 统计月份（月报）
    start_time          TIMESTAMPTZ,                                  -- 数据时间范围起
    end_time            TIMESTAMPTZ,                                  -- 数据时间范围止
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE quality_report IS '质量报表记录表，文件存储至MinIO对应租户桶（SRS F05-03）';

CREATE INDEX idx_report_tenant ON quality_report(tenant_id);
CREATE INDEX idx_report_type ON quality_report(tenant_id, report_type);
CREATE INDEX idx_report_created ON quality_report(created_at);
-- ============================================================
-- CR-System DDL: 系统支撑模块
-- 对应服务：message-push / job-scheduler / 全局
-- 数据库：PostgreSQL 17
-- ============================================================

-- ============================================================
-- 1. 用户消息偏好配置表（SRS F04-04：用户可关闭部分通知类型）
-- ============================================================
CREATE TABLE notification_pref (
    user_id             UUID NOT NULL,
    tenant_id           UUID NOT NULL,
    enable_site         BOOLEAN NOT NULL DEFAULT TRUE,                -- 站内信开关
    enable_wechat       BOOLEAN NOT NULL DEFAULT TRUE,                -- 企业微信开关
    enable_email        BOOLEAN NOT NULL DEFAULT TRUE,                -- 邮件开关
    disabled_events     JSONB NOT NULL DEFAULT '[]',                  -- 关闭的通知事件类型列表
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, tenant_id)
);

COMMENT ON TABLE notification_pref IS '用户通知偏好配置（SRS F04-04）';
COMMENT ON COLUMN notification_pref.disabled_events IS 'JSON数组，如 ["review_timeout","review_completed"]';

CREATE INDEX idx_notif_pref_tenant ON notification_pref(tenant_id);

-- ============================================================
-- 2. 站内信通知记录表
-- ============================================================
CREATE TABLE notification (
    notification_id     UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id             UUID NOT NULL,                                -- 接收人UID
    tenant_id           UUID NOT NULL,
    event_type          VARCHAR(32) NOT NULL,                         -- 事件类型
    title               VARCHAR(256) NOT NULL,                        -- 通知标题
    content             TEXT DEFAULT '',                              -- 通知正文
    related_id          VARCHAR(128) DEFAULT '',                      -- 关联业务ID（review_id等）
    channel             VARCHAR(16) NOT NULL DEFAULT 'site',          -- site / wechat / email
    is_read             BOOLEAN NOT NULL DEFAULT FALSE,
    read_at             TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE notification IS '站内信通知记录表（SRS F04系列）';
COMMENT ON COLUMN notification.event_type IS '事件类型: review_assigned / review_rejected / review_resubmitted / review_completed / review_timeout';

CREATE INDEX idx_notif_user ON notification(user_id, is_read);
CREATE INDEX idx_notif_tenant ON notification(tenant_id);
CREATE INDEX idx_notif_event ON notification(event_type);
CREATE INDEX idx_notif_created ON notification(created_at);
CREATE INDEX idx_notif_user_unread ON notification(user_id, created_at) WHERE is_read = FALSE;

-- ============================================================
-- 3. 租户通知渠道配置表（SRS F04-01：租户管理员可开关渠道）
-- ============================================================
CREATE TABLE tenant_notification_channel (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID NOT NULL,
    site_enabled        BOOLEAN NOT NULL DEFAULT TRUE,
    wechat_enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    wechat_webhook_url  VARCHAR(512) DEFAULT '',                      -- 企业微信机器人Webhook
    email_enabled       BOOLEAN NOT NULL DEFAULT FALSE,
    smtp_host           VARCHAR(256) DEFAULT '',
    smtp_port           INT DEFAULT 0,
    smtp_username       VARCHAR(128) DEFAULT '',
    smtp_password       TEXT DEFAULT '',                              -- AES加密存储
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE tenant_notification_channel IS '租户通知渠道配置（SRS F04-01）';

CREATE UNIQUE INDEX idx_tenant_channel ON tenant_notification_channel(tenant_id);

-- ============================================================
-- 4. 系统操作日志表（SRS N09：关键操作日志不可篡改，留存≥6个月）
-- ============================================================
CREATE TABLE system_operation_log (
    log_id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id           UUID,
    user_id             UUID,                                         -- 操作人
    user_name           VARCHAR(128),                                 -- 操作人姓名（冗余，防止用户删除后丢失）
    operation           VARCHAR(64) NOT NULL,                         -- 操作类型
    resource_type       VARCHAR(64) NOT NULL,                         -- 资源类型
    resource_id         VARCHAR(128),                                 -- 资源ID
    detail              JSONB DEFAULT '{}',                           -- 操作详情（前后数据变更）
    ip_address          VARCHAR(64),                                  -- 操作人IP
    trace_id            VARCHAR(64),                                  -- 全链路TraceID（LLD §1.3）
    user_agent          VARCHAR(512) DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE system_operation_log IS '系统操作日志，不可篡改，最低留存6个月（SRS N09 / LLD §9）';
COMMENT ON COLUMN system_operation_log.detail IS 'JSONB记录操作前后数据变更，用于审计追溯';

CREATE INDEX idx_oplog_tenant ON system_operation_log(tenant_id);
CREATE INDEX idx_oplog_user ON system_operation_log(user_id);
CREATE INDEX idx_oplog_resource ON system_operation_log(resource_type, resource_id);
CREATE INDEX idx_oplog_trace ON system_operation_log(trace_id);
CREATE INDEX idx_oplog_created ON system_operation_log(created_at);

-- ============================================================
-- 5. 定时任务执行记录表（LLD §3.6 job-scheduler）
-- ============================================================
CREATE TABLE job_execution_log (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_name            VARCHAR(128) NOT NULL,                        -- 任务名称
    job_type            VARCHAR(32) NOT NULL,                         -- sync / cleanup / stats / refresh / reconcile
    status              VARCHAR(16) NOT NULL DEFAULT 'running',       -- running / success / failed
    start_time          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_time            TIMESTAMPTZ,
    duration_ms         BIGINT,                                       -- 执行耗时(毫秒)
    result_summary      TEXT DEFAULT '',                              -- 执行结果摘要
    error_message       TEXT DEFAULT '',                              -- 失败错误信息
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE job_execution_log IS '定时任务执行记录（LLD §3.6）';

CREATE INDEX idx_job_log_name ON job_execution_log(job_name);
CREATE INDEX idx_job_log_status ON job_execution_log(status);
CREATE INDEX idx_job_log_time ON job_execution_log(start_time);

-- ============================================================
-- 6. MQ消息对账表（LLD §3.6：定时对账修复数据不一致）
-- ============================================================
CREATE TABLE mq_reconciliation (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    topic               VARCHAR(128) NOT NULL,                        -- MQ Topic名称
    message_id          VARCHAR(128) NOT NULL,                        -- 消息ID
    business_id         VARCHAR(128) NOT NULL,                        -- 业务ID（review_id等）
    business_type       VARCHAR(64) NOT NULL,                         -- 业务类型
    produce_time        TIMESTAMPTZ NOT NULL,                         -- 消息发送时间
    consume_time        TIMESTAMPTZ,                                  -- 消息消费时间
    status              VARCHAR(16) NOT NULL DEFAULT 'pending',       -- pending / consumed / reconciled / failed
    retry_count         INT NOT NULL DEFAULT 0,                       -- 重试次数
    reconcile_note      TEXT DEFAULT '',                              -- 对账备注
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE mq_reconciliation IS 'MQ消息对账表，定时任务对账修复消息不一致（LLD §3.6）';

CREATE INDEX idx_mq_recon_topic ON mq_reconciliation(topic);
CREATE INDEX idx_mq_recon_business ON mq_reconciliation(business_type, business_id);
CREATE INDEX idx_mq_recon_status ON mq_reconciliation(status);
CREATE INDEX idx_mq_recon_time ON mq_reconciliation(created_at);
