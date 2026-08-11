// Package bizadapter 第三方外部接口适配封装（LLD §1.2-4）
// Keycloak 身份适配层：统一封装登录、账号同步接口（HLD §8-4）
package bizadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"cr-system/app/iam-service/internal/conf"
	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// KeycloakUser Keycloak Admin API 返回的用户结构
type KeycloakUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	Enabled       bool   `json:"enabled"`
	EmailVerified bool   `json:"emailVerified"`
}

// KeycloakClient Keycloak 管理端适配器
// 所有外部调用带超时控制与指数退避重试（LLD §1.3-2 / §7.2）
type KeycloakClient struct {
	cfg        conf.Keycloak
	httpClient *http.Client

	mu          sync.RWMutex
	adminToken  string    // Admin API 访问令牌（client_credentials）
	tokenExpire time.Time // 令牌过期时间（提前30秒轮换）
}

// NewKeycloakClient 创建适配器
func NewKeycloakClient(cfg conf.Keycloak) *KeycloakClient {
	return &KeycloakClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: util.ParseDurationOr(cfg.Timeout, 10*time.Second)},
	}
}

// TokenResp 用户登录令牌响应（password grant）
type TokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// PasswordLogin 用户密码登录（Resource Owner Password Credentials）
// 走前端 public client（如 cr-system-web），签发的 JWT 受众与后端 JWTAuth 校验一致
// 用户名或密码错误返回 ErrUnauthorized；Keycloak 不可用返回 ErrKeycloakConnect
func (c *KeycloakClient) PasswordLogin(ctx context.Context, clientID, username, password string) (*TokenResp, error) {
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {clientID},
		"username":   {username},
		"password":   {password},
	}
	reqURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.cfg.BaseURL, c.cfg.Realm)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errcode.ErrKeycloakConnect.WithDetail(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
		// invalid_grant：用户名/密码错误，或账号被禁用
		return nil, errcode.ErrUnauthorized.WithDetail("用户名或密码错误")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, errcode.ErrKeycloakConnect.WithDetail(
			fmt.Sprintf("登录请求失败 HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var tokenResp TokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, errcode.ErrKeycloakConnect.WithDetail("令牌响应解析失败")
	}
	return &tokenResp, nil
}

// DisplayName 拼接显示名
func (u *KeycloakUser) DisplayName() string {
	name := strings.TrimSpace(u.FirstName + u.LastName)
	if name == "" {
		return u.Username
	}
	return name
}

// ListUsers 拉取 Realm 下全部用户（分页聚合，单页100条）
func (c *KeycloakClient) ListUsers(ctx context.Context) ([]*KeycloakUser, error) {
	const pageSize = 100
	var all []*KeycloakUser
	for first := 0; ; first += pageSize {
		users, err := c.listUsersPage(ctx, first, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, users...)
		if len(users) < pageSize {
			break
		}
	}
	return all, nil
}

func (c *KeycloakClient) listUsersPage(ctx context.Context, first, max int) ([]*KeycloakUser, error) {
	var users []*KeycloakUser
	err := util.RetryWithBackoff(ctx, util.DefaultRetryDelays, func(ctx context.Context) error {
		token, err := c.getAdminToken(ctx)
		if err != nil {
			return err
		}
		reqURL := fmt.Sprintf("%s/admin/realms/%s/users?first=%d&max=%d",
			c.cfg.BaseURL, c.cfg.Realm, first, max)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return errcode.ErrKeycloakConnect.WithDetail(err.Error())
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			c.invalidateToken() // 令牌失效，清除后重试会重新获取
			return errcode.ErrKeycloakConnect.WithDetail("管理令牌已失效")
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			return errcode.ErrKeycloakSyncFailed.WithDetail(
				fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)))
		}
		return json.NewDecoder(resp.Body).Decode(&users)
	})
	return users, err
}

// getAdminToken 获取 Admin API 令牌（client_credentials，带本地缓存与提前轮换）
func (c *KeycloakClient) getAdminToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.adminToken != "" && time.Now().Before(c.tokenExpire) {
		defer c.mu.RUnlock()
		return c.adminToken, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	// 双重检查
	if c.adminToken != "" && time.Now().Before(c.tokenExpire) {
		return c.adminToken, nil
	}

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
	}
	reqURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.cfg.BaseURL, c.cfg.Realm)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", errcode.ErrKeycloakConnect.WithDetail(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errcode.ErrKeycloakConnect.WithDetail(fmt.Sprintf("获取令牌失败 HTTP %d", resp.StatusCode))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", errcode.ErrKeycloakConnect.WithDetail("令牌响应解析失败")
	}

	// 提前30秒视为过期，避免边界请求失败（LLD §3.2-3 Token提前轮换思想一致）
	c.adminToken = tokenResp.AccessToken
	c.tokenExpire = time.Now().Add(time.Duration(tokenResp.ExpiresIn-30) * time.Second)
	return c.adminToken, nil
}

// invalidateToken 清除缓存令牌
func (c *KeycloakClient) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.adminToken = ""
}
