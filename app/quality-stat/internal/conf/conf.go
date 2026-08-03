// Package conf quality-stat 配置结构定义
package conf

// Bootstrap 服务启动配置
type Bootstrap struct {
	Server Server  `yaml:"server"`
	Data   Data    `yaml:"data"`
	MQ     MQ      `yaml:"mq"`
	MinIO  MinIO   `yaml:"minio"`
	Trace  Trace   `yaml:"trace"`
	Logger Logger  `yaml:"logger"`
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

// Postgres PostgreSQL 配置
type Postgres struct {
	DSN             string `yaml:"dsn"`
	ReadDSN         string `yaml:"read_dsn"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
}

// Redis Redis 配置
type Redis struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	DialTimeout  string `yaml:"dial_timeout"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

// MQ RocketMQ 配置
type MQ struct {
	NameServers   []string `yaml:"name_servers"`
	ConsumerGroup string   `yaml:"consumer_group"`
}

// MinIO 对象存储配置
type MinIO struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
	UseSSL    bool   `yaml:"use_ssl"`
	Timeout   string `yaml:"timeout"`
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
