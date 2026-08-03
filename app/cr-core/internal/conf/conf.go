// Package conf cr-core 配置结构定义
// 对应 configs/cr-core.yaml；密钥类字段从 K8s Secret 环境变量注入（LLD §9-1）
package conf

// Bootstrap 服务启动配置
type Bootstrap struct {
	Server     Server     `yaml:"server"`
	Data       Data       `yaml:"data"`
	MQ         MQ         `yaml:"mq"`
	Clients    Clients    `yaml:"clients"`
	Sonar      Sonar      `yaml:"sonar"`
	Trace      Trace      `yaml:"trace"`
	Logger     Logger     `yaml:"logger"`
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

// Redis Redis 配置（分布式锁 lock:review_mr:{mrId}，LLD §3.3-3）
type Redis struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	DialTimeout  string `yaml:"dial_timeout"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

// MQ RocketMQ 配置（事务消息发评审代码拉取，普通消息发通知/完结事件，LLD §4）
type MQ struct {
	NameServers   []string `yaml:"name_servers"`
	ProducerGroup string   `yaml:"producer_group"`
}

// Clients 内部 gRPC 依赖服务（LLD §1.2：禁止跨服务直连数据库，同步走 gRPC）
type Clients struct {
	IAMService  Endpoint `yaml:"iam_service"`  // 权限校验
	GitAdapter  Endpoint `yaml:"git_adapter"`  // MR基础数据查询
}

// Sonar SonarQube 配置（HLD §8-2 适配层接入）
type Sonar struct {
	BaseURL string `yaml:"base_url"` // 如 http://sonarqube:9000
	Token   string `yaml:"token"`    // 从 Secret 注入
	Timeout string `yaml:"timeout"`
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
