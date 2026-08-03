package data

import "time"

// Notification 站内信通知记录（对应 DDL notification 表）
type Notification struct {
	NotificationID string    `db:"notification_id"`
	UserID         string    `db:"user_id"`
	TenantID       string    `db:"tenant_id"`
	EventType      string    `db:"event_type"` // review_assigned / review_rejected / review_resubmitted / review_completed / review_timeout
	Title          string    `db:"title"`
	Content        string    `db:"content"`
	RelatedID      string    `db:"related_id"` // 关联业务ID（review_id等）
	Channel        string    `db:"channel"`    // site / wechat / email
	IsRead         bool      `db:"is_read"`
	ReadAt         *time.Time `db:"read_at"`
	CreatedAt      time.Time `db:"created_at"`
}

// NotificationPref 用户通知偏好（对应 DDL notification_pref 表）
type NotificationPref struct {
	UserID         string    `db:"user_id"`
	TenantID       string    `db:"tenant_id"`
	EnableSite     bool      `db:"enable_site"`
	EnableWechat   bool      `db:"enable_wechat"`
	EnableEmail    bool      `db:"enable_email"`
	DisabledEvents []byte    `db:"disabled_events"` // JSONB: ["review_timeout","review_completed"]
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// TenantChannel 租户通知渠道配置（对应 DDL tenant_notification_channel 表）
type TenantChannel struct {
	ID              string    `db:"id"`
	TenantID        string    `db:"tenant_id"`
	SiteEnabled     bool      `db:"site_enabled"`
	WechatEnabled   bool      `db:"wechat_enabled"`
	WechatWebhookURL string   `db:"wechat_webhook_url"`
	EmailEnabled    bool      `db:"email_enabled"`
	SMTPHost        string    `db:"smtp_host"`
	SMTPPort        int       `db:"smtp_port"`
	SMTPUsername    string    `db:"smtp_username"`
	SMTPPassword    string    `db:"smtp_password"` // AES 加密存储
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}
