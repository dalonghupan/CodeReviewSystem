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
