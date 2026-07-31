// Package conf git-adapter 配置结构定义
// 对应 configs/git-adapter.yaml；密钥类字段从 K8s Secret 环境变量注入（LLD §9-1）
package conf

// Bootstrap 服务启动配置
type Bootstrap struct {
	Server   Server              `yaml:"server"`
	Data     Data                `yaml:"data"`
	MQ       MQ                  `yaml:"mq"`
	Git      Git                 `yaml:"git"`
	Security Security            `yaml:"security"`
	Trace    Trace               `yaml:"trace"`
	Logger   Logger              `yaml:"logger"`
}

// Server 服务监听配置
type Server struct {
	HTTP Endpoint `yaml:"http"`
	GRPC Endpoint `yaml:"grpc"`
}

// Endpoint 单个协议端点
type Endpoint struct {
	Network string `yaml:"network"`
	Addr    string `yaml:"addr"`
	Timeout string `yaml:"timeout"`
}

// Data 数据源配置
type Data struct {
	Postgres Postgres `yaml:"postgres"`
	Redis    Redis    `yaml:"redis"`
}

// Postgres PostgreSQL 配置（读写分离，HLD §4.1）
type Postgres struct {
	DSN             string `yaml:"dsn"`
	ReadDSN         string `yaml:"read_dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

// Redis Redis 配置（Diff缓存 diff:{mrId}:{commitHash}、限流计数 git_limit:{platform}:{ip}，LLD §6）
type Redis struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	DialTimeout  string `yaml:"dial_timeout"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

// MQ RocketMQ 配置（消费 review_code_fetch_topic / token_refresh_topic，LLD §4）
type MQ struct {
	NameServers        []string `yaml:"name_servers"`
	ProducerGroup      string   `yaml:"producer_group"`
}

// Git 三方平台 OAuth 与限流配置（LLD §3.2）
type Git struct {
	GitLab   PlatformOAuth `yaml:"gitlab"`
	Gitee    PlatformOAuth `yaml:"gitee"`
	GitHub   PlatformOAuth `yaml:"github"`
	RateLimit RateLimit   `yaml:"rate_limit"`
}

// PlatformOAuth 单平台 OAuth2.1 配置
type PlatformOAuth struct {
	BaseURL      string `yaml:"base_url"`       // 平台API地址（私有化GitLab填自建域名）
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`  // 从 Secret 注入
	RedirectURI  string `yaml:"redirect_uri"`   // OAuth回调地址
}

// RateLimit Git接口限流配置（防平台封禁，LLD §3.2-2）
type RateLimit struct {
	MaxCallsPerHour int `yaml:"max_calls_per_hour"` // 单IP单平台每小时最大调用数
}

// Security 安全配置
type Security struct {
	TokenAESKey string `yaml:"token_aes_key"` // Token加密密钥（32字节base64/hex，从 Secret 注入，LLD §9-4）
}

// Trace 链路追踪配置
type Trace struct {
	Endpoint    string  `yaml:"endpoint"`
	SampleRatio float64 `yaml:"sample_ratio"`
	Insecure    bool    `yaml:"insecure"`
}

// Logger 日志配置
type Logger struct {
	Level       string `yaml:"level"`
	Development bool   `yaml:"development"`
}
