// Package bizadapter message-push 第三方通知适配层
// 企业微信机器人 Webhook 客户端（LLD §3.4-2）
package bizadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cr-system/pkg/errcode"
)

// WeChatClient 企业微信机器人 Webhook 客户端
type WeChatClient struct {
	httpClient *http.Client
}

// weChatMessage 企业微信机器人消息体
type weChatMessage struct {
	MsgType  string          `json:"msgtype"`
	Markdown *weChatMarkdown `json:"markdown,omitempty"`
	Text     *weChatText     `json:"text,omitempty"`
}

type weChatMarkdown struct {
	Content string `json:"content"`
}

type weChatText struct {
	Content string `json:"content"`
}

// NewWeChatClient 创建企业微信客户端
func NewWeChatClient(timeout time.Duration) *WeChatClient {
	return &WeChatClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// SendMarkdown 发送 Markdown 消息
func (c *WeChatClient) SendMarkdown(ctx context.Context, webhookURL, content string) error {
	msg := weChatMessage{
		MsgType: "markdown",
		Markdown: &weChatMarkdown{
			Content: content,
		},
	}
	return c.doPost(ctx, webhookURL, msg)
}

// SendText 发送文本消息
func (c *WeChatClient) SendText(ctx context.Context, webhookURL, content string) error {
	msg := weChatMessage{
		MsgType: "text",
		Text: &weChatText{
			Content: content,
		},
	}
	return c.doPost(ctx, webhookURL, msg)
}

// doPost 发送 POST 请求（支持1次重试应对 429）
func (c *WeChatClient) doPost(ctx context.Context, webhookURL string, msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return errcode.ErrWechatPushFailed.WithDetail("序列化消息失败: " + err.Error())
	}

	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
		if err != nil {
			return errcode.ErrWechatPushFailed.WithDetail("创建请求失败: " + err.Error())
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == 0 {
				continue
			}
			return errcode.ErrWechatPushFailed.WithDetail("请求失败: " + err.Error())
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests {
			continue // 429 重试
		}
		if resp.StatusCode != http.StatusOK {
			return errcode.ErrWechatPushFailed.WithDetail(
				fmt.Sprintf("企业微信返回状态码 %d: %s", resp.StatusCode, string(respBody)))
		}

		// 解析响应判断业务是否成功
		var result struct {
			Errcode int    `json:"errcode"`
			Errmsg  string `json:"errmsg"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil // 企业微信偶尔返回非 JSON 但仍算成功
		}
		if result.Errcode != 0 {
			return errcode.ErrWechatPushFailed.WithDetail(
				fmt.Sprintf("企业微信业务错误 code=%d: %s", result.Errcode, result.Errmsg))
		}
		return nil
	}

	return errcode.ErrWechatPushFailed.WithDetail("发送失败（已重试1次）")
}

// BuildReviewNoticeMarkdown 构建评审通知的 Markdown 内容
func BuildReviewNoticeMarkdown(title, reviewID, repoName, mrID, eventType, operatorName, remark string) string {
	content := fmt.Sprintf("## %s\n\n", title)
	content += fmt.Sprintf("> **评审单**：[%s](https://cr-system/reviews/%s)\n", reviewID[:8], reviewID)
	content += fmt.Sprintf("> **仓库**：%s\n", repoName)
	content += fmt.Sprintf("> **MR**：%s\n", mrID)

	eventLabels := map[string]string{
		"review_assigned":    "指派评审",
		"review_rejected":    "驳回",
		"review_resubmitted": "提交复审",
		"review_completed":   "评审通过",
		"review_timeout":     "超时提醒",
	}
	label := eventLabels[eventType]
	if label == "" {
		label = eventType
	}
	content += fmt.Sprintf("> **事件**：%s\n", label)

	if operatorName != "" {
		content += fmt.Sprintf("> **操作人**：%s\n", operatorName)
	}
	if remark != "" {
		content += fmt.Sprintf("\n---\n> **备注**：%s\n", remark)
	}
	return content
}
