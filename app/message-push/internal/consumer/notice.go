// Package consumer message-push MQ 消费者
// 消费 review_notice_topic（评审事件通知 → 渠道分发：站内信/企业微信/邮件）
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"cr-system/app/message-push/internal/bizadapter"
	"cr-system/app/message-push/internal/data"
	"cr-system/app/message-push/internal/sse"
	"cr-system/pkg/mq"
)

// NoticeConsumer 评审通知消费者
// 对应 LLD §2 步骤：消费 review_notice_topic → 查询偏好+渠道 → 多渠道分发
type NoticeConsumer struct {
	data  *data.Data
	wx    *bizadapter.WeChatClient
	email *bizadapter.EmailClient
	log   *log.Helper
}

// NewNoticeConsumer 构造
func NewNoticeConsumer(d *data.Data, wx *bizadapter.WeChatClient, email *bizadapter.EmailClient, logger log.Logger) *NoticeConsumer {
	return &NoticeConsumer{data: d, wx: wx, email: email, log: log.NewHelper(logger)}
}

// Handle 处理评审通知消息（mq.MessageHandler 签名）
func (c *NoticeConsumer) Handle(ctx context.Context, body []byte) error {
	var msg mq.ReviewNoticeMessage
	if err := mq.Unmarshal(body, &msg); err != nil {
		return nil // 格式非法直接丢弃
	}

	c.log.Infow("msg", "收到评审通知", "event_type", msg.EventType,
		"review_id", msg.ReviewID, "tenant_id", msg.TenantID, "trace_id", msg.TraceID)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 查询租户渠道配置
	siteEnabled, wechatEnabled, emailEnabled, webhookURL, err := c.data.GetEnabledChannels(ctx, msg.TenantID)
	if err != nil {
		c.log.Warnw("msg", "查询租户渠道配置失败，使用默认", "tenant_id", msg.TenantID, "error", err.Error())
	}

	// 构建通知标题
	title := buildNoticeTitle(msg.EventType, msg.ReviewTitle)
	content := buildNoticeContent(msg)

	// 对每个目标用户分发
	for _, uid := range msg.TargetUIDs {
		// 检查用户偏好
		eventEnabled, err := c.data.IsEventEnabled(ctx, uid, msg.TenantID, string(msg.EventType))
		if err != nil || !eventEnabled {
			continue // 用户关闭了此类通知
		}

		// 站内信
		if siteEnabled {
			n := &data.Notification{
				UserID:    uid,
				TenantID:  msg.TenantID,
				EventType: string(msg.EventType),
				Title:     title,
				Content:   content,
				RelatedID: msg.ReviewID,
				Channel:   "site",
			}
			if err := c.data.InsertNotification(ctx, n); err != nil {
				c.log.Errorw("msg", "站内信入库失败", "user_id", uid, "error", err.Error())
			} else {
				c.publishSSE(ctx, n) // 实时推送到在线前端（失败不阻塞，通知列表仍可见）
			}
		}

		// 企业微信（按用户维度发送，但企业微信是群发到 Webhook，只发一次）
		if wechatEnabled && webhookURL != "" && uid == msg.TargetUIDs[0] {
			mdContent := bizadapter.BuildReviewNoticeMarkdown(title, msg.ReviewID,
				msg.RepoName, msg.MRID, string(msg.EventType), msg.OperatorName, msg.Remark)
			if err := c.wx.SendMarkdown(ctx, webhookURL, mdContent); err != nil {
				c.log.Errorw("msg", "企业微信推送失败", "error", err.Error())
			}
		}
	}

	// 邮件（只发一次到所有收件人）
	if emailEnabled && len(msg.TargetUIDs) > 0 {
		// 邮件发送需要租户配置 SMTP，此处由 service 层组装 EmailClient 时配置
		c.log.Infow("msg", "邮件通知待发送", "review_id", msg.ReviewID, "targets", len(msg.TargetUIDs))
		// 实际邮件发送由调用方根据租户 SMTP 配置执行
	}

	c.log.Infow("msg", "评审通知分发完成", "event_type", msg.EventType, "targets", len(msg.TargetUIDs))
	return nil
}

// publishSSE 站内信入库成功后广播实时事件（SSE 通道：Redis Pub/Sub → 在线前端）
func (c *NoticeConsumer) publishSSE(ctx context.Context, n *data.Notification) {
	payload, err := json.Marshal(sse.Event{
		NotificationID: n.NotificationID,
		UserID:         n.UserID,
		EventType:      n.EventType,
		Title:          n.Title,
		Content:        n.Content,
		RelatedID:      n.RelatedID,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	if err := c.data.PublishNotificationEvent(ctx, payload); err != nil {
		c.log.Warnw("msg", "SSE事件发布失败", "user_id", n.UserID, "error", err.Error())
	}
}

// buildNoticeTitle 构建通知标题
func buildNoticeTitle(eventType mq.NoticeEventType, reviewTitle string) string {
	prefix := ""
	switch eventType {
	case mq.NoticeReviewAssigned:
		prefix = "【评审指派】"
	case mq.NoticeReviewRejected:
		prefix = "【评审驳回】"
	case mq.NoticeReviewResubmitted:
		prefix = "【提交复审】"
	case mq.NoticeReviewCompleted:
		prefix = "【评审完成】"
	case mq.NoticeReviewTimeout:
		prefix = "【评审超时】"
	case mq.NoticeReviewDeadline:
		prefix = "【截止提醒】"
	default:
		prefix = "【通知】"
	}
	if reviewTitle != "" {
		return prefix + reviewTitle
	}
	return prefix + "代码评审通知"
}

// buildNoticeContent 构建通知内容
func buildNoticeContent(msg mq.ReviewNoticeMessage) string {
	content := fmt.Sprintf("评审单：%s\n仓库：%s\nMR：%s", msg.ReviewID, msg.RepoName, msg.MRID)
	if msg.OperatorName != "" {
		content += fmt.Sprintf("\n操作人：%s", msg.OperatorName)
	}
	if msg.Remark != "" {
		content += fmt.Sprintf("\n备注：%s", msg.Remark)
	}
	if msg.Deadline != nil {
		content += fmt.Sprintf("\n截止时间：%s", msg.Deadline.Format("2006-01-02 15:04"))
	}
	return content
}
