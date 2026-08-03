package data

import (
	"context"
	"encoding/json"

	"cr-system/pkg/errcode"
)

// ==================== 通知偏好管理（DDL notification_pref 表）====================

// GetPrefs 查询用户通知偏好
func (d *Data) GetPrefs(ctx context.Context, userID, tenantID string) (*NotificationPref, error) {
	var p NotificationPref
	const q = `SELECT user_id, tenant_id, enable_site, enable_wechat, enable_email, disabled_events, created_at, updated_at
		FROM notification_pref WHERE user_id = $1 AND tenant_id = $2`
	if err := d.readDB.GetContext(ctx, &p, q, userID, tenantID); err != nil {
		return nil, errcode.ErrNotFound.WithDetail("通知偏好未设置，使用默认值")
	}
	return &p, nil
}

// UpsertPrefs 创建或更新用户通知偏好
func (d *Data) UpsertPrefs(ctx context.Context, p *NotificationPref) error {
	const q = `INSERT INTO notification_pref (user_id, tenant_id, enable_site, enable_wechat, enable_email, disabled_events)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, tenant_id) DO UPDATE SET
			enable_site = EXCLUDED.enable_site,
			enable_wechat = EXCLUDED.enable_wechat,
			enable_email = EXCLUDED.enable_email,
			disabled_events = EXCLUDED.disabled_events,
			updated_at = NOW()`
	if _, err := d.writeDB.ExecContext(ctx, q, p.UserID, p.TenantID, p.EnableSite, p.EnableWechat,
		p.EnableEmail, p.DisabledEvents); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// IsEventEnabled 检查用户是否启用了某事件类型的通知
func (d *Data) IsEventEnabled(ctx context.Context, userID, tenantID, eventType string) (bool, error) {
	prefs, err := d.GetPrefs(ctx, userID, tenantID)
	if err != nil {
		// 偏好未设置时默认全部启用
		return true, nil
	}
	var disabled []string
	if len(prefs.DisabledEvents) > 0 {
		if err := json.Unmarshal(prefs.DisabledEvents, &disabled); err != nil {
			return true, nil
		}
	}
	for _, e := range disabled {
		if e == eventType {
			return false, nil
		}
	}
	return true, nil
}

// ==================== 租户渠道管理（DDL tenant_notification_channel 表）====================

// GetChannelConfig 查询租户通知渠道配置
func (d *Data) GetChannelConfig(ctx context.Context, tenantID string) (*TenantChannel, error) {
	var c TenantChannel
	const q = `SELECT id, tenant_id, site_enabled, wechat_enabled, wechat_webhook_url,
		email_enabled, smtp_host, smtp_port, smtp_username, smtp_password, created_at, updated_at
		FROM tenant_notification_channel WHERE tenant_id = $1`
	if err := d.readDB.GetContext(ctx, &c, q, tenantID); err != nil {
		return nil, errcode.ErrNotFound.WithDetail("租户通知渠道未配置")
	}
	return &c, nil
}

// UpsertChannelConfig 创建或更新租户渠道配置
// 敏感字段（SMTP 密码等）需调用方已加密
func (d *Data) UpsertChannelConfig(ctx context.Context, c *TenantChannel) error {
	const q = `INSERT INTO tenant_notification_channel (tenant_id, site_enabled, wechat_enabled, wechat_webhook_url,
		email_enabled, smtp_host, smtp_port, smtp_username, smtp_password)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id) DO UPDATE SET
			site_enabled = EXCLUDED.site_enabled,
			wechat_enabled = EXCLUDED.wechat_enabled,
			wechat_webhook_url = EXCLUDED.wechat_webhook_url,
			email_enabled = EXCLUDED.email_enabled,
			smtp_host = EXCLUDED.smtp_host,
			smtp_port = EXCLUDED.smtp_port,
			smtp_username = EXCLUDED.smtp_username,
			smtp_password = EXCLUDED.smtp_password,
			updated_at = NOW()`
	if _, err := d.writeDB.ExecContext(ctx, q, c.TenantID, c.SiteEnabled, c.WechatEnabled,
		c.WechatWebhookURL, c.EmailEnabled, c.SMTPHost, c.SMTPPort, c.SMTPUsername, c.SMTPPassword); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetEnabledChannels 获取租户已启用的渠道列表
// 返回 site_enabled/wechat_enabled/email_enabled 的布尔值
func (d *Data) GetEnabledChannels(ctx context.Context, tenantID string) (site, wechat, email bool, webhookURL string, err error) {
	cfg, err := d.GetChannelConfig(ctx, tenantID)
	if err != nil {
		// 未配置时默认仅站内信启用
		return true, false, false, "", nil
	}
	return cfg.SiteEnabled, cfg.WechatEnabled, cfg.EmailEnabled, cfg.WechatWebhookURL, nil
}
