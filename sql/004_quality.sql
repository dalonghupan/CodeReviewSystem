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
