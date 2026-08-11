// Package conf iam-service 配置结构定义
// 对应 configs/iam-service.yaml；密钥类字段从 K8s Secret 环境变量注入（LLD §9-1）
package conf

// Bootstrap 服务启动配置（与 kratos config 加载的 yaml 根结构对应）
type Bootstrap struct {
	Server   Server   `yaml:"server"`
	Data     Data     `yaml:"data"`
	Auth     Auth     `yaml:"auth"`
	Keycloak Keycloak `yaml:"keycloak"`
	Trace    Trace    `yaml:"trace"`
	Logger   Logger   `yaml:"logger"`
}

// Server 服务监听配置
type Server struct {
	HTTP Endpoint `yaml:"http"`
	GRPC Endpoint `yaml:"grpc"`
}

// Endpoint 单个协议端点
type Endpoint struct {
	Network string `yaml:"network"` // tcp
	Addr    string `yaml:"addr"`    // 0.0.0.0:8000
	Timeout string `yaml:"timeout"` // 时长字符串，如 5s（由启动时 ParseDuration 解析）
}

// Data 数据源配置
type Data struct {
	Postgres Postgres `yaml:"postgres"`
	Redis    Redis    `yaml:"redis"`
}

// Postgres PostgreSQL 配置（主从读写分离：写主库，读从库，HLD §4.1）
type Postgres struct {
	DSN             string `yaml:"dsn"`               // 主库（写）DSN，从 Secret 注入
	ReadDSN         string `yaml:"read_dsn"`          // 从库（读）DSN，空则读写均走主库
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"` // 时长字符串，如 30m
}

// Redis Redis 配置（权限缓存 user:perm:{uid}:{tenantId} TTL=2h，LLD §6）
type Redis struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"` // 从 Secret 注入
	DB           int    `yaml:"db"`
	DialTimeout  string `yaml:"dial_timeout"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

// Auth JWT 鉴权配置（对应 middleware.AuthConfig）
type Auth struct {
	JWKSURL   string   `yaml:"jwks_url"`
	Issuer    string   `yaml:"issuer"`
	Audience  []string `yaml:"audience"`
	Whitelist []string `yaml:"whitelist"` // 免鉴权 gRPC full method / HTTP path
}

// Keycloak Keycloak 管理端配置（账号同步，LLD §3.1）
type Keycloak struct {
	BaseURL      string `yaml:"base_url"`      // 如 https://keycloak:8080
	Realm        string `yaml:"realm"`         // cr-system
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"` // 从 Secret 注入
	WebClientID  string `yaml:"web_client_id"` // 前端 public client（password grant 登录用），如 cr-system-web
	Timeout      string `yaml:"timeout"`       // 时长字符串，如 10s
}

// Trace 链路追踪配置
type Trace struct {
	Endpoint    string  `yaml:"endpoint"`     // jaeger:4317
	SampleRatio float64 `yaml:"sample_ratio"`
	Insecure    bool    `yaml:"insecure"`
}

// Logger 日志配置
type Logger struct {
	Level       string `yaml:"level"`
	Development bool   `yaml:"development"`
}
