package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	jwtv5 "github.com/golang-jwt/jwt/v5"

	"cr-system/pkg/errcode"
)

// AuthConfig JWT 鉴权配置（Keycloak OIDC，HLD §9-2）
type AuthConfig struct {
	JWKSURL     string   // Keycloak JWKS 地址，如 https://keycloak/realms/cr-system/protocol/openid-connect/certs
	Issuer      string   // 期望的 iss，如 https://keycloak/realms/cr-system
	Audience    []string // 期望的 aud（客户端ID），空则不校验
	Whitelist   []string // 免鉴权路径（gRPC full method 或 HTTP path）
}

// Claims 业务自定义 JWT Claims（与 Keycloak Mapper 配置对应）
type Claims struct {
	jwtv5.RegisteredClaims
	TenantID string   `json:"tenant_id"` // 租户ID（Keycloak用户属性映射）
	Roles    []string `json:"roles"`     // 角色列表（Realm roles 映射）
}

// JWTAuth 构建 JWT 鉴权中间件
// 网关在 APISIX 已做第一道 JWT 校验，此处为后端服务的第二层校验（HLD §9-3 权限双层校验）
func JWTAuth(cfg AuthConfig) (middleware.Middleware, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{cfg.JWKSURL})
	if err != nil {
		return nil, err
	}

	whitelist := make(map[string]struct{}, len(cfg.Whitelist))
	for _, p := range cfg.Whitelist {
		whitelist[p] = struct{}{}
	}

	jwtOpts := []jwtv5.ParserOption{
		jwtv5.WithValidMethods([]string{"RS256"}),
		jwtv5.WithIssuer(cfg.Issuer),
		jwtv5.WithExpirationRequired(),
	}
	if len(cfg.Audience) > 0 {
		jwtOpts = append(jwtOpts, jwtv5.WithAudience(cfg.Audience...))
	} else {
		jwtOpts = append(jwtOpts, jwtv5.WithoutClaimsValidation())
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 白名单路径跳过（健康检查、Webhook回调等）
			if tr, ok := transport.FromServerContext(ctx); ok {
				if _, skip := whitelist[tr.Operation()]; skip {
					return handler(ctx, req)
				}
			}

			token, err := extractToken(ctx)
			if err != nil {
				return nil, err
			}

			var claims Claims
			if _, err := jwtv5.ParseWithClaims(token, &claims, kf.Keyfunc, jwtOpts...); err != nil {
				return nil, errcode.ErrUnauthorized.WithDetail("令牌校验失败")
			}

			ctx = WithUserID(ctx, claims.Subject)
			ctx = WithTenantID(ctx, claims.TenantID)
			ctx = WithUserRoles(ctx, claims.Roles)

			return handler(ctx, req)
		}
	}, nil
}

// extractToken 从 gRPC metadata / HTTP header 提取 Bearer Token
func extractToken(ctx context.Context) (string, error) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", errcode.ErrUnauthorized
	}
	auth := tr.RequestHeader().Get("Authorization")
	if auth == "" {
		return "", errcode.ErrUnauthorized.WithDetail("缺少Authorization头")
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errcode.ErrUnauthorized.WithDetail("令牌格式错误")
	}
	return parts[1], nil
}
