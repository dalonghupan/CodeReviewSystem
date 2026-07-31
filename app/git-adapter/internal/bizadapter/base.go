package bizadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// httpClient 平台 API 调用基座
// 统一：超时控制、指数退避重试（LLD §1.3-2 / §7.2）、错误码归一
type httpClient struct {
	client *http.Client
}

func newHTTPClient(timeout time.Duration) *httpClient {
	return &httpClient{client: &http.Client{Timeout: timeout}}
}

// doJSON 执行请求并解析 JSON 响应
// authHeader 为空表示匿名请求（OAuth 换 token 场景）
// 限流响应（429/403 rate limit）返回 ErrGitRateLimited 触发重试
func (c *httpClient) doJSON(ctx context.Context, method, url, authHeader string, body io.Reader, out interface{}) error {
	return util.RetryWithBackoff(ctx, util.DefaultRetryDelays, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return errcode.ErrGitAPIError.WithDetail(err.Error())
		}
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return errcode.ErrGitAPIError.WithDetail("网络错误: " + err.Error())
		}
		defer resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			return errcode.ErrGitTokenExpired
		case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
			if isRateLimitBody(resp.Body) {
				return errcode.ErrGitRateLimited // 触发退避重试
			}
			return errcode.ErrGitAPIError.WithDetail(fmt.Sprintf("HTTP %d", resp.StatusCode))
		case resp.StatusCode == http.StatusNotFound:
			return errcode.ErrGitMRNotFound
		case resp.StatusCode >= 400:
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			return errcode.ErrGitAPIError.WithDetail(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(b)))
		}

		if out == nil {
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return errcode.ErrGitAPIError.WithDetail("响应解析失败: " + err.Error())
		}
		return nil
	})
}

// isRateLimitBody 判断 403 是否为限流（GitHub 特有：普通权限不足也是 403）
func isRateLimitBody(body io.Reader) bool {
	b, _ := io.ReadAll(io.LimitReader(body, 1024))
	s := string(b)
	return strings.Contains(s, "rate limit") || strings.Contains(s, "API rate limit")
}

// postForm 构造 form 请求体
func postForm(values map[string]string) io.Reader {
	parts := make([]string, 0, len(values))
	for k, v := range values {
		parts = append(parts, k+"="+v)
	}
	return strings.NewReader(strings.Join(parts, "&"))
}
