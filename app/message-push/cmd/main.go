// message-push 消息推送服务入口（HLD §2.4 / LLD §3.4）
package main

import (
	"context"
	"flag"
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
	"cr-system/app/message-push/internal/bizadapter"
	"cr-system/app/message-push/internal/conf"
	"cr-system/app/message-push/internal/consumer"
	"cr-system/app/message-push/internal/data"
	"cr-system/app/message-push/internal/service"
	"cr-system/pkg/logger"
	"cr-system/pkg/middleware"
	"cr-system/pkg/mq"
	"cr-system/pkg/trace"
	"cr-system/pkg/util"
)

const serviceName = "message-push"
const serviceVersion = "0.1.0"

var configPath = flag.String("conf", "configs/message-push.yaml", "配置文件路径")

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

	// 4. 第三方适配器
	wechatClient := bizadapter.NewWeChatClient(
		util.ParseDurationOr(bc.WeChat.Timeout, 5*time.Second),
	)
	emailClient := bizadapter.NewEmailClient(
		util.ParseDurationOr(bc.Email.Timeout, 10*time.Second),
	)

	// 5. 业务服务
	svc := service.NewMessageService(dataLayer, appLogger)

	// 6. MQ 消费者
	noticeConsumer := consumer.NewNoticeConsumer(dataLayer, wechatClient, emailClient, appLogger)
	mqConsumer, err := mq.NewConsumer(mq.ConsumerConfig{
		NameServers: bc.MQ.NameServers,
		GroupName:   bc.MQ.ConsumerGroup,
	}, appLogger)
	if err != nil {
		panic("初始化MQ消费者失败: " + err.Error())
	}
	if err := mqConsumer.Subscribe(mq.TopicReviewNotice, noticeConsumer.Handle); err != nil {
		panic("订阅review_notice_topic失败: " + err.Error())
	}
	if err := mqConsumer.Start(); err != nil {
		panic("启动MQ消费者失败: " + err.Error())
	}

	// 7. 中间件链
	authMW, err := middleware.JWTAuth(middleware.AuthConfig{
		JWKSURL:   "",
		Whitelist: nil,
	})
	if err != nil {
		panic("初始化JWT鉴权失败: " + err.Error())
	}

	grpcSrv := kgrpc.NewServer(
		kgrpc.Address(bc.Server.GRPC.Addr),
		kgrpc.Timeout(util.ParseDurationOr(bc.Server.GRPC.Timeout, 5*time.Second)),
		kgrpc.Middleware(
			recovery.Recovery(),
			tracing.Server(),
			middleware.ErrorHandler(),
			middleware.ServerLogging(appLogger),
			authMW,
			validate.Validator(),
		),
	)
	v1.RegisterMessageServiceServer(grpcSrv, svc)

	httpSrv := khttp.NewServer(
		khttp.Address(bc.Server.HTTP.Addr),
		khttp.Timeout(util.ParseDurationOr(bc.Server.HTTP.Timeout, 5*time.Second)),
		khttp.Middleware(
			recovery.Recovery(),
			tracing.Server(),
			middleware.ErrorHandler(),
			middleware.ServerLogging(appLogger),
			authMW,
			validate.Validator(),
		),
	)
	v1.RegisterMessageServiceHTTPServer(httpSrv, svc)

	// 8. 启动
	app := kratos.New(
		kratos.Name(serviceName),
		kratos.Version(serviceVersion),
		kratos.Logger(appLogger),
		kratos.Server(grpcSrv, httpSrv),
	)

	defer func() {
		_ = shutdownTrace(context.Background())
		cleanupData()
		_ = mqConsumer.Shutdown()
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
