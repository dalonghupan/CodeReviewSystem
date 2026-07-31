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
