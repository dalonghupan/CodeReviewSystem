package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-kratos/kratos/v2/log"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// heartbeatInterval SSE 心跳间隔（注释行保活，防止中间代理空闲断连）
const heartbeatInterval = 25 * time.Second

// claims SSE token 中的业务字段（与 pkg/middleware.Claims 对应，另取 preferred_username 做用户映射）
type claims struct {
	jwtv5.RegisteredClaims
	TenantID          string `json:"tenant_id"`
	PreferredUsername string `json:"preferred_username"`
}

// LookupUserIDFunc 按租户+用户名解析系统用户ID（由 main 注入 data 层实现，避免包循环依赖）
type LookupUserIDFunc func(ctx context.Context, tenantID, username string) (string, error)

// Handler SSE HTTP 端点：GET /api/v1/sse/events?token=<JWT>
// EventSource 无法设置 Authorization 头，token 经 query 参数传入，此处独立验签
type Handler struct {
	hub       *Hub
	lookupUID LookupUserIDFunc
	kf        keyfunc.Keyfunc
	issuer    string
	audience  []string
	log       *log.Helper
}

// NewHandler 构造 SSE 端点；jwksURL 为空时返回 nil（鉴权未启用，端点不注册）
func NewHandler(hub *Hub, lookup LookupUserIDFunc, jwksURL, issuer string, audience []string, logger log.Logger) (*Handler, error) {
	if jwksURL == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	kf, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("加载JWKS失败: %w", err)
	}
	return &Handler{
		hub:       hub,
		lookupUID: lookup,
		kf:        kf,
		issuer:    issuer,
		audience:  audience,
		log:       log.NewHelper(logger),
	}, nil
}

// ServeHTTP 建立 SSE 长连接
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. token 验签（query 参数）
	userID, err := h.authenticate(r)
	if err != nil {
		h.log.Warnw("msg", "SSE鉴权失败", "error", err.Error())
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "未授权，请先登录"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "当前环境不支持SSE"})
		return
	}

	// 2. SSE 响应头（X-Accel-Buffering 提示 nginx 等代理不要缓冲）
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// 3. 注册连接并发送连接建立事件
	c := h.hub.register(userID)
	defer h.hub.unregister(c)

	h.writeEvent(w, flusher, "", &Event{
		UserID:    userID,
		EventType: "connection_established",
		Title:     "实时消息通道已建立",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	// 4. 事件循环：业务事件 + 心跳
	// 注意：不能用 r.Context() 判断断开——kratos 全局 filter 给每个请求挂了
	// server.http.timeout（5s）的 ctx 超时，长连接会被提前取消。
	// 客户端断连通过写失败（Flush 返回错误）检测，最坏一个心跳周期内发现。
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case evt := <-c.ch:
			if err := h.writeEvent(w, flusher, evt.NotificationID, evt); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// authenticate 校验 query 中的 token，返回系统用户ID
func (h *Handler) authenticate(r *http.Request) (string, error) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		return "", fmt.Errorf("缺少token参数")
	}

	opts := []jwtv5.ParserOption{
		jwtv5.WithValidMethods([]string{"RS256"}),
		jwtv5.WithIssuer(h.issuer),
		jwtv5.WithExpirationRequired(),
	}
	if len(h.audience) > 0 {
		opts = append(opts, jwtv5.WithAudience(h.audience...))
	}

	var c claims
	if _, err := jwtv5.ParseWithClaims(tokenStr, &c, h.kf.Keyfunc, opts...); err != nil {
		return "", fmt.Errorf("token校验失败: %w", err)
	}
	if c.TenantID == "" || c.PreferredUsername == "" {
		return "", fmt.Errorf("token缺少tenant_id或preferred_username")
	}

	// Keycloak sub ≠ 本系统 user_id（登录即同步通道按 username 落库），此处解析映射
	userID, err := h.lookupUID(r.Context(), c.TenantID, c.PreferredUsername)
	if err != nil {
		return "", fmt.Errorf("用户映射失败: %w", err)
	}
	return userID, nil
}

// writeEvent 按 SSE 协议写入一条事件（id 用于前端断线重连的 lastEventId）
func (h *Handler) writeEvent(w http.ResponseWriter, flusher http.Flusher, id string, evt *Event) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if id != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// writeJSON 输出 JSON 错误响应
func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
