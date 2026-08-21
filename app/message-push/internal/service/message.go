// Package service message-push 业务逻辑层（LLD §3.4）
// 实现 MessageService 全部 RPC：站内信管理、通知偏好、租户渠道配置
package service

import (
	"context"
	"encoding/json"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "cr-system/api/crsystem/v1"
	"cr-system/app/message-push/internal/data"
	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// MessageService 消息推送服务
type MessageService struct {
	v1.UnimplementedMessageServiceServer

	data *data.Data
	log  *log.Helper
}

// NewMessageService 构造服务
func NewMessageService(d *data.Data, logger log.Logger) *MessageService {
	return &MessageService{
		data: d,
		log:  log.NewHelper(logger),
	}
}

// ==================== 站内信管理 ====================

// ListNotifications 获取用户站内信列表
func (s *MessageService) ListNotifications(ctx context.Context, req *v1.ListNotificationsReq) (*v1.ListNotificationsResp, error) {
	if req.UserId == "" {
		return nil, errcode.ErrParamInvalid
	}

	page, pageSize := paginationOf(req.Pagination)
	items, total, err := s.data.ListNotifications(ctx, req.UserId, req.UnreadOnly, req.EventType, page, pageSize)
	if err != nil {
		return nil, err
	}

	// 获取未读总数
	unreadTotal, _, err := s.data.GetUnreadCount(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	resp := &v1.ListNotificationsResp{
		Pagination:  paginationResp(page, pageSize, total),
		UnreadTotal: unreadTotal,
	}
	for _, n := range items {
		resp.Items = append(resp.Items, notificationToProto(n))
	}
	return resp, nil
}

// MarkAsRead 标记站内信为已读
func (s *MessageService) MarkAsRead(ctx context.Context, req *v1.MarkAsReadReq) (*v1.OperateResult, error) {
	if req.UserId == "" || req.NotificationId == "" {
		return nil, errcode.ErrParamInvalid
	}
	if err := s.data.MarkAsRead(ctx, req.UserId, req.NotificationId); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "已标记为已读"}, nil
}

// BatchMarkAsRead 批量标记已读
func (s *MessageService) BatchMarkAsRead(ctx context.Context, req *v1.BatchMarkAsReadReq) (*v1.OperateResult, error) {
	if req.UserId == "" {
		return nil, errcode.ErrParamInvalid
	}
	if err := s.data.BatchMarkAsRead(ctx, req.UserId, req.NotificationIds); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "批量标记已读完成"}, nil
}

// GetUnreadCount 获取未读消息数量
func (s *MessageService) GetUnreadCount(ctx context.Context, req *v1.GetUnreadCountReq) (*v1.UnreadCountResp, error) {
	if req.UserId == "" {
		return nil, errcode.ErrParamInvalid
	}

	total, byType, err := s.data.GetUnreadCount(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	return &v1.UnreadCountResp{
		Total:  total,
		ByType: byType,
	}, nil
}

// ==================== 通知偏好设置 ====================

// GetNotificationPrefs 获取用户通知偏好
func (s *MessageService) GetNotificationPrefs(ctx context.Context, req *v1.GetPrefsReq) (*v1.NotificationPrefs, error) {
	if req.UserId == "" {
		return nil, errcode.ErrParamInvalid
	}

	pref, err := s.data.GetPrefs(ctx, req.UserId, extractTenantID(ctx))
	if err != nil {
		// 未设置偏好时返回默认值
		return &v1.NotificationPrefs{
			UserId:       req.UserId,
			EnableSite:   true,
			EnableWechat: true,
			EnableEmail:  true,
		}, nil
	}

	return prefToProto(pref), nil
}

// UpdateNotificationPrefs 更新用户通知偏好
func (s *MessageService) UpdateNotificationPrefs(ctx context.Context, req *v1.UpdatePrefsReq) (*v1.OperateResult, error) {
	if req.UserId == "" {
		return nil, errcode.ErrParamInvalid
	}

	disabledEvents, err := json.Marshal(req.DisabledEvents)
	if err != nil {
		return nil, errcode.ErrParamInvalid.WithDetail("disabled_events 格式错误")
	}

	p := &data.NotificationPref{
		UserID:         req.UserId,
		TenantID:       extractTenantID(ctx),
		EnableSite:     req.EnableSite,
		EnableWechat:   req.EnableWechat,
		EnableEmail:    req.EnableEmail,
		DisabledEvents: disabledEvents,
	}
	if err := s.data.UpsertPrefs(ctx, p); err != nil {
		return nil, err
	}

	return &v1.OperateResult{Success: true, Message: "通知偏好更新成功"}, nil
}

// ==================== 租户通知渠道管理 ====================

// GetChannelConfig 获取租户通知渠道配置
func (s *MessageService) GetChannelConfig(ctx context.Context, req *v1.GetChannelConfigReq) (*v1.ChannelConfig, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}

	cfg, err := s.data.GetChannelConfig(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	return channelToProto(cfg), nil
}

// UpdateChannelConfig 更新租户通知渠道配置
func (s *MessageService) UpdateChannelConfig(ctx context.Context, req *v1.UpdateChannelConfigReq) (*v1.OperateResult, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}

	c := &data.TenantChannel{
		TenantID:         req.TenantId,
		SiteEnabled:      req.SiteEnabled,
		WechatEnabled:    req.WechatEnabled,
		WechatWebhookURL: req.WechatWebhookUrl,
		EmailEnabled:     req.EmailEnabled,
		SMTPHost:         req.SmtpHost,
		SMTPPort:         int(req.SmtpPort),
	}

	if err := s.data.UpsertChannelConfig(ctx, c); err != nil {
		return nil, err
	}

	return &v1.OperateResult{Success: true, Message: "渠道配置更新成功"}, nil
}

// ==================== 辅助 ====================

// extractTenantID 从上下文中提取租户ID（由 JWT 中间件注入）
func extractTenantID(ctx context.Context) string {
	if tid, ok := ctx.Value("tenant_id").(string); ok {
		return tid
	}
	return ""
}

// ==================== 转换辅助 ====================

func notificationToProto(n *data.Notification) *v1.NotificationInfo {
	info := &v1.NotificationInfo{
		NotificationId: n.NotificationID,
		UserId:         n.UserID,
		EventType:      n.EventType,
		Title:          n.Title,
		Content:        n.Content,
		RelatedId:      n.RelatedID,
		Channel:        n.Channel,
		IsRead:         n.IsRead,
		CreatedAt:      timestamppb.New(n.CreatedAt),
	}
	if n.ReadAt != nil {
		info.ReadAt = timestamppb.New(*n.ReadAt)
	}
	return info
}

func prefToProto(p *data.NotificationPref) *v1.NotificationPrefs {
	prefs := &v1.NotificationPrefs{
		UserId:       p.UserID,
		EnableSite:   p.EnableSite,
		EnableWechat: p.EnableWechat,
		EnableEmail:  p.EnableEmail,
		UpdatedAt:    timestamppb.New(p.UpdatedAt),
	}
	if len(p.DisabledEvents) > 0 {
		var events []string
		if err := json.Unmarshal(p.DisabledEvents, &events); err == nil {
			prefs.DisabledEvents = events
		}
	}
	return prefs
}

func channelToProto(c *data.TenantChannel) *v1.ChannelConfig {
	return &v1.ChannelConfig{
		TenantId:         c.TenantID,
		SiteEnabled:      c.SiteEnabled,
		WechatEnabled:    c.WechatEnabled,
		WechatWebhookUrl: c.WechatWebhookURL,
		EmailEnabled:     c.EmailEnabled,
		SmtpHost:         c.SMTPHost,
		SmtpPort:         uint32(c.SMTPPort),
		UpdatedAt:        timestamppb.New(c.UpdatedAt),
	}
}

func paginationOf(p *v1.Pagination) (uint32, uint32) {
	if p == nil {
		return util.NormalizePage(0, 0)
	}
	return util.NormalizePage(p.Page, p.PageSize)
}

func paginationResp(page, pageSize, total uint32) *v1.PaginationResp {
	return &v1.PaginationResp{
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: util.TotalPages(total, pageSize),
	}
}

// Ensure interface compliance
var _ v1.MessageServiceServer = (*MessageService)(nil)
