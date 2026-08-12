package data

import (
	"context"
	"database/sql"
	"errors"

	"github.com/redis/go-redis/v9"

	"cr-system/app/message-push/internal/sse"
	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// ==================== 站内信管理（DDL notification 表）====================

// InsertNotification 插入一条站内信记录
func (d *Data) InsertNotification(ctx context.Context, n *Notification) error {
	if n.NotificationID == "" {
		n.NotificationID = util.NewUUID()
	}
	const q = `INSERT INTO notification (notification_id, user_id, tenant_id, event_type, title, content,
		related_id, channel) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := d.writeDB.ExecContext(ctx, q, n.NotificationID, n.UserID, n.TenantID,
		n.EventType, n.Title, n.Content, n.RelatedID, n.Channel); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// PublishNotificationEvent 发布实时通知事件到 SSE 广播频道（Redis Pub/Sub）
// 推送失败不影响站内信主流程（调用方记日志即可，用户仍可在通知列表看到）
func (d *Data) PublishNotificationEvent(ctx context.Context, payload []byte) error {
	if err := d.rdb.Publish(ctx, sse.Channel, payload).Err(); err != nil {
		return errcode.ErrRedis.WithDetail(err.Error())
	}
	return nil
}

// RDB 暴露 Redis 客户端（SSE Hub 订阅频道用）
func (d *Data) RDB() *redis.Client {
	return d.rdb
}

// FindUserIDByUsername 按租户+用户名解析系统用户ID（SSE token 身份映射用）
// 注意：sys_user 属 iam 域表，此处只读——开发期同库直查；
// 正式方案应由 iam-service 提供 gRPC 查询或 token 直接注入 user_id claim
func (d *Data) FindUserIDByUsername(ctx context.Context, tenantID, username string) (string, error) {
	var userID string
	const q = `SELECT user_id FROM sys_user WHERE tenant_id = $1 AND username = $2 AND is_active = TRUE`
	if err := d.readDB.GetContext(ctx, &userID, q, tenantID, username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errcode.ErrNotFound.WithDetail("用户不存在或已禁用")
		}
		return "", errcode.ErrDatabase.WithDetail(err.Error())
	}
	return userID, nil
}

// BatchInsertNotifications 批量插入站内信（同一事件多用户）
func (d *Data) BatchInsertNotifications(ctx context.Context, items []*Notification) error {
	for _, n := range items {
		if err := d.InsertNotification(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

// ListNotifications 分页查询站内信
// unreadOnly: 仅未读；eventType: 按事件类型筛选
func (d *Data) ListNotifications(ctx context.Context, userID string, unreadOnly bool, eventType string, page, pageSize uint32) ([]*Notification, uint32, error) {
	page, pageSize = util.NormalizePage(page, pageSize)

	filter := `FROM notification WHERE user_id = $1`
	args := []interface{}{userID}
	argIdx := 2

	if unreadOnly {
		filter += ` AND is_read = FALSE`
	}
	if eventType != "" {
		filter += ` AND event_type = $` + itoa(argIdx)
		args = append(args, eventType)
		argIdx++
	}

	var total uint32
	if err := d.readDB.GetContext(ctx, &total, `SELECT COUNT(*) `+filter, args...); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}

	var items []*Notification
	q := `SELECT notification_id, user_id, tenant_id, event_type, title, content, related_id, channel,
		is_read, read_at, created_at ` + filter + ` ORDER BY created_at DESC LIMIT $` + itoa(argIdx) +
		` OFFSET $` + itoa(argIdx+1)
	args = append(args, pageSize, util.PageOffset(page, pageSize))
	if err := d.readDB.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, total, nil
}

// MarkAsRead 标记单条站内信为已读
func (d *Data) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	const q = `UPDATE notification SET is_read = TRUE, read_at = NOW()
		WHERE notification_id = $1 AND user_id = $2 AND is_read = FALSE`
	res, err := d.writeDB.ExecContext(ctx, q, notificationID, userID)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrNotFound.WithDetail("通知不存在或已读")
	}
	return nil
}

// BatchMarkAsRead 批量标记已读（空列表则标记全部已读）
func (d *Data) BatchMarkAsRead(ctx context.Context, userID string, ids []string) error {
	if len(ids) > 0 {
		placeholders, idArgs := buildIN(ids, 2)
		q := `UPDATE notification SET is_read = TRUE, read_at = NOW() WHERE user_id = $1 AND notification_id IN (` + placeholders + `) AND is_read = FALSE`
		args := append([]interface{}{userID}, idArgs...)
		if _, err := d.writeDB.ExecContext(ctx, q, args...); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	} else {
		const q = `UPDATE notification SET is_read = TRUE, read_at = NOW() WHERE user_id = $1 AND is_read = FALSE`
		if _, err := d.writeDB.ExecContext(ctx, q, userID); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	}
	return nil
}

// GetUnreadCount 获取未读数（含按事件类型分组）
func (d *Data) GetUnreadCount(ctx context.Context, userID string) (total uint32, byType map[string]uint32, err error) {
	const totalQ = `SELECT COUNT(*) FROM notification WHERE user_id = $1 AND is_read = FALSE`
	if err := d.readDB.GetContext(ctx, &total, totalQ, userID); err != nil {
		return 0, nil, errcode.ErrDatabase.WithDetail(err.Error())
	}

	type countByType struct {
		EventType string `db:"event_type"`
		Count     uint32 `db:"cnt"`
	}
	var rows []countByType
	const groupQ = `SELECT event_type, COUNT(*) AS cnt FROM notification WHERE user_id = $1 AND is_read = FALSE GROUP BY event_type`
	if err := d.readDB.SelectContext(ctx, &rows, groupQ, userID); err != nil {
		return 0, nil, errcode.ErrDatabase.WithDetail(err.Error())
	}

	byType = make(map[string]uint32, len(rows))
	for _, r := range rows {
		byType[r.EventType] = r.Count
	}
	return total, byType, nil
}

// GetNotification 查询单条站内信
func (d *Data) GetNotification(ctx context.Context, notificationID string) (*Notification, error) {
	var n Notification
	const q = `SELECT notification_id, user_id, tenant_id, event_type, title, content, related_id,
		channel, is_read, read_at, created_at FROM notification WHERE notification_id = $1`
	if err := d.readDB.GetContext(ctx, &n, q, notificationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &n, nil
}

// itoa 简易整数转字符串（避免依赖 strconv）
func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// buildIN 构建带 IN 子句的查询参数
// 返回 (placeholders, args) 用于拼接到 SQL 中
// 示例: buildIN([]string{"a","b"}) → ("$2,$3", ["a","b"])
func buildIN(ids []string, startIdx int) (string, []interface{}) {
	placeholders := ""
	args := make([]interface{}, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "$" + itoa(startIdx+i)
		args = append(args, id)
	}
	return placeholders, args
}
