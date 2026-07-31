// Package consumer git-adapter MQ 消费者
// 消费 review_code_fetch_topic（拉取MR变更并预解析Diff入缓存）
// 消费 token_refresh_topic（Token 定时刷新，LLD §3.2-3）
package consumer

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"cr-system/app/git-adapter/internal/service"
	"cr-system/pkg/mq"
)

// CodeFetchConsumer 代码拉取消费者
// 对应 LLD §2 步骤7-9：消费消息 → 调Git平台拉取变更文件 → 结构化解析 → 写Redis缓存
type CodeFetchConsumer struct {
	svc *service.GitAdapterService
	log *log.Helper
}

// NewCodeFetchConsumer 构造
func NewCodeFetchConsumer(svc *service.GitAdapterService, logger log.Logger) *CodeFetchConsumer {
	return &CodeFetchConsumer{svc: svc, log: log.NewHelper(logger)}
}

// Handle 处理代码拉取消息（mq.MessageHandler 签名）
func (c *CodeFetchConsumer) Handle(ctx context.Context, body []byte) error {
	var msg mq.CodeFetchMessage
	if err := mq.Unmarshal(body, &msg); err != nil {
		return nil // 格式非法的消息重试无意义，直接确认丢弃
	}

	c.log.Infow("msg", "开始拉取MR代码", "review_id", msg.ReviewID, "repo_id", msg.RepoID,
		"mr_id", msg.MRID, "platform", msg.Platform, "trace_id", msg.TraceID)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := c.svc.FetchAndCacheDiff(ctx, msg.RepoID, msg.MRID, msg.CommitHash); err != nil {
		c.log.Errorw("msg", "代码拉取失败", "review_id", msg.ReviewID, "error", err.Error())
		return err // 触发重试（最多3次后进死信）
	}

	c.log.Infow("msg", "代码拉取完成", "review_id", msg.ReviewID, "mr_id", msg.MRID)
	return nil
}

// TokenRefreshConsumer Token 刷新消费者
type TokenRefreshConsumer struct {
	svc *service.GitAdapterService
	log *log.Helper
}

// NewTokenRefreshConsumer 构造
func NewTokenRefreshConsumer(svc *service.GitAdapterService, logger log.Logger) *TokenRefreshConsumer {
	return &TokenRefreshConsumer{svc: svc, log: log.NewHelper(logger)}
}

// Handle 处理 Token 刷新消息
func (c *TokenRefreshConsumer) Handle(ctx context.Context, body []byte) error {
	var msg mq.TokenRefreshMessage
	if err := mq.Unmarshal(body, &msg); err != nil {
		return nil
	}

	c.log.Infow("msg", "执行Token刷新", "auth_id", msg.AuthID, "platform", msg.Platform)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := c.svc.RefreshAuthTokenByID(ctx, msg.AuthID); err != nil {
		c.log.Errorw("msg", "Token刷新失败", "auth_id", msg.AuthID, "error", err.Error())
		return err
	}
	return nil
}
