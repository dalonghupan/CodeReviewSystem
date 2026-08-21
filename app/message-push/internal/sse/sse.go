// Package sse 实时消息推送（Server-Sent Events）
// 链路：MQ 消费 → 站内信入库 → Redis Pub/Sub 广播 → Hub 按用户分发到在线连接
// 对应前端 web/src/api/services/notification.ts 的 SSEManager
package sse

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// Channel Redis Pub/Sub 频道名（多实例部署时跨实例广播，单实例同样走此通道）
const Channel = "sse:notification"

// reconnectDelay 订阅中断后的重试间隔
const reconnectDelay = 5 * time.Second

// Event 推送给前端的事件载荷
// 字段与 web/src/api/types/notification.ts 的 SSEEventData 一一对应
type Event struct {
	NotificationID string `json:"notification_id"`
	UserID         string `json:"user_id"`
	EventType      string `json:"event_type"` // review_assigned / review_rejected / ... / connection_established
	Title          string `json:"title"`
	Content        string `json:"content"`
	RelatedID      string `json:"related_id"`
	CreatedAt      string `json:"created_at"`
}

// client 一条在线 SSE 连接
type client struct {
	userID string
	ch     chan *Event
}

// Hub 在线连接注册表：userID -> 该用户的全部连接
// MQ 消费者发布的 Redis 消息经 Run 订阅后按 userID 本地分发
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*client]struct{}
	log     *log.Helper
}

// NewHub 构造 Hub
func NewHub(logger log.Logger) *Hub {
	return &Hub{
		clients: make(map[string]map[*client]struct{}),
		log:     log.NewHelper(logger),
	}
}

// register 注册连接，返回的事件通道由 handler 消费
func (h *Hub) register(userID string) *client {
	c := &client{userID: userID, ch: make(chan *Event, 16)} // 缓冲 16 条，写阻塞时兜底
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*client]struct{})
	}
	h.clients[userID][c] = struct{}{}
	h.log.Infow("msg", "SSE连接已注册", "user_id", userID)
	return c
}

// unregister 注销连接
func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.clients[c.userID]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.clients, c.userID)
		}
	}
	h.log.Infow("msg", "SSE连接已注销", "user_id", c.userID)
}

// dispatch 本地分发给目标用户的所有在线连接（非阻塞，channel 满则丢弃并告警）
func (h *Hub) dispatch(evt *Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[evt.UserID] {
		select {
		case c.ch <- evt:
		default:
			h.log.Warnw("msg", "SSE事件通道已满，丢弃事件", "user_id", evt.UserID, "event_type", evt.EventType)
		}
	}
}

// Run 订阅 Redis 频道并分发消息（goroutine 运行，ctx 取消后退出）
// 订阅失败（如 Redis 短暂抖动）按固定间隔重试，避免一次抖动导致推送永久中断
func (h *Hub) Run(ctx context.Context, rdb *redis.Client) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := h.subscribeLoop(ctx, rdb); err != nil && ctx.Err() == nil {
			h.log.Warnw("msg", "SSE Redis订阅中断，5s后重试", "error", err.Error())
			timer := time.NewTimer(reconnectDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (h *Hub) subscribeLoop(ctx context.Context, rdb *redis.Client) error {
	sub := rdb.Subscribe(ctx, Channel)
	defer sub.Close()

	msgCh := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m, ok := <-msgCh:
			if !ok {
				return nil // 通道关闭（Redis 断连），由外层重试
			}
			var evt Event
			if err := json.Unmarshal([]byte(m.Payload), &evt); err != nil {
				h.log.Warnw("msg", "SSE事件解析失败，丢弃", "payload", m.Payload)
				continue
			}
			if evt.UserID == "" {
				continue
			}
			h.dispatch(&evt)
		}
	}
}
