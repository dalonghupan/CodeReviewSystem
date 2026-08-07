// git-adapter 仓库适配服务入口（HLD §2.1 / LLD §3.2）
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"strings"
	"time"
	"os"
	"github.com/go-kratos/kratos/v2"
	kconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"gopkg.in/yaml.v3"

	v1 "cr-system/api/crsystem/v1"
	"cr-system/app/git-adapter/internal/bizadapter"
	"cr-system/app/git-adapter/internal/conf"
	"cr-system/app/git-adapter/internal/consumer"
	"cr-system/app/git-adapter/internal/data"
	"cr-system/app/git-adapter/internal/service"
	"cr-system/pkg/logger"
	"cr-system/pkg/middleware"
	"cr-system/pkg/mq"
	"cr-system/pkg/trace"
	"cr-system/pkg/util"
)

const serviceName = "git-adapter"
const serviceVersion = "0.1.0"

var configPath = flag.String("conf", "configs/git-adapter.yaml", "配置文件路径")

func main() {
	flag.Parse()

	// 1. 加载配置
	c := kconfig.New(kconfig.WithSource(file.NewSource(*configPath)))
	if err := c.Load(); err != nil {
		panic("加载配置失败: " + err.Error())
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic("解析配置失败: " + err.Error())
	}
	expandEnv(&bc)

	// 2. 日志与链路追踪
	appLogger := logger.New(logger.Config{
		ServiceName: serviceName,
		Level:       bc.Logger.Level,
		Development: bc.Logger.Development,
	})
	shutdownTrace, err := trace.InitProvider(trace.Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Endpoint:       bc.Trace.Endpoint,
		SampleRatio:    bc.Trace.SampleRatio,
		Insecure:       bc.Trace.Insecure,
	})
	if err != nil {
		panic("初始化链路追踪失败: " + err.Error())
	}

	// 3. 数据层
	dataLayer, cleanupData, err := data.NewData(&bc.Data, appLogger)
	if err != nil {
		panic("初始化数据层失败: " + err.Error())
	}

	// 4. 平台适配器注册表（HLD §8-1 适配层模式：新增平台仅扩展适配器）
	registry := bizadapter.NewRegistry(
		bizadapter.NewGitLabProvider(bc.Git.GitLab),
		bizadapter.NewGiteeProvider(bc.Git.Gitee),
		bizadapter.NewGitHubProvider(bc.Git.GitHub),
	)

	// 5. Token 加密密钥（hex 编码的 32 字节，从 Secret 注入，LLD §9-4）
	aesKey, err := hex.DecodeString(strings.TrimSpace(bc.Security.TokenAESKey))
	if err != nil || len(aesKey) != 32 {
		panic("TOKEN_AES_KEY 必须为 hex 编码的 32 字节密钥")
	}

	// 6. 业务服务
	svc := service.NewGitAdapterService(dataLayer, registry, aesKey, bc.Git.RateLimit.MaxCallsPerHour, appLogger)

	// 7. MQ 消费者（代码拉取 + Token刷新，LLD §4）
	codeFetchConsumer, err := mq.NewConsumer(mq.ConsumerConfig{
		NameServers: bc.MQ.NameServers,
		GroupName:   mq.GroupGitAdapterCodeFetch,
	}, appLogger)
	if err != nil {
		panic("初始化代码拉取消费者失败: " + err.Error())
	}
	fetchHandler := consumer.NewCodeFetchConsumer(svc, appLogger)
	if err := codeFetchConsumer.Subscribe(mq.TopicReviewCodeFetch, fetchHandler.Handle); err != nil {
		panic("订阅代码拉取Topic失败: " + err.Error())
	}

	tokenConsumer, err := mq.NewConsumer(mq.ConsumerConfig{
		NameServers: bc.MQ.NameServers,
		GroupName:   mq.GroupGitAdapterToken,
	}, appLogger)
	if err != nil {
		panic("初始化Token刷新消费者失败: " + err.Error())
	}
	tokenHandler := consumer.NewTokenRefreshConsumer(svc, appLogger)
	if err := tokenConsumer.Subscribe(mq.TopicTokenRefresh, tokenHandler.Handle); err != nil {
		panic("订阅Token刷新Topic失败: " + err.Error())
	}

	// 8. 传输层（git-adapter 多数接口为服务间/网关内部调用，鉴权由 APISIX+iam 完成，
	//    Webhook 与 OAuth 回调为平台来源不走 JWT，故本服务不强制 JWT 中间件）
	grpcSrv := kgrpc.NewServer(
		kgrpc.Address(bc.Server.GRPC.Addr),
		kgrpc.Timeout(util.ParseDurationOr(bc.Server.GRPC.Timeout, 5*time.Second)),
		kgrpc.Middleware(
			recovery.Recovery(),
			tracing.Server(),
			middleware.ErrorHandler(),
			middleware.ServerLogging(appLogger),
			validate.Validator(),
		),
	)
	v1.RegisterGitAdapterServiceServer(grpcSrv, svc)

	httpSrv := khttp.NewServer(
		khttp.Address(bc.Server.HTTP.Addr),
		khttp.Timeout(util.ParseDurationOr(bc.Server.HTTP.Timeout, 5*time.Second)),
		khttp.Middleware(
			recovery.Recovery(),
			tracing.Server(),
			middleware.ErrorHandler(),
			middleware.ServerLogging(appLogger),
			validate.Validator(),
		),
	)
	v1.RegisterGitAdapterServiceHTTPServer(httpSrv, svc)

	// 9. 启动
	app := kratos.New(
		kratos.Name(serviceName),
		kratos.Version(serviceVersion),
		kratos.Logger(appLogger),
		kratos.Server(grpcSrv, httpSrv),
		kratos.BeforeStop(func(ctx context.Context) error {
			_ = codeFetchConsumer.Shutdown()
			_ = tokenConsumer.Shutdown()
			return nil
		}),
	)

	if err := codeFetchConsumer.Start(); err != nil {
		panic("启动代码拉取消费者失败: " + err.Error())
	}
	if err := tokenConsumer.Start(); err != nil {
		panic("启动Token刷新消费者失败: " + err.Error())
	}

	defer func() {
		_ = shutdownTrace(context.Background())
		cleanupData()
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}

// expandEnv 解析配置中的 ${ENV_VAR} 占位符（K8s Secret 注入，LLD §9-1）
func expandEnv(bc *conf.Bootstrap) {
	raw, err := os.ReadFile(*configPath)
	if err != nil {
		return
	}
	expanded := os.ExpandEnv(string(raw))
	_ = yaml.Unmarshal([]byte(expanded), bc)
}
	expanded := util.ExpandEnv(string(raw))
	_ = yaml.Unmarshal([]byte(expanded), bc)
}
