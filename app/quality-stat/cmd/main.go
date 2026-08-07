// quality-stat 质量统计服务入口（HLD §2.3 / LLD §3.5）
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
	"cr-system/app/quality-stat/internal/bizadapter"
	"cr-system/app/quality-stat/internal/conf"
	"cr-system/app/quality-stat/internal/consumer"
	"cr-system/app/quality-stat/internal/data"
	"cr-system/app/quality-stat/internal/service"
	"cr-system/pkg/logger"
	"cr-system/pkg/middleware"
	"cr-system/pkg/mq"
	"cr-system/pkg/trace"
	"cr-system/pkg/util"
)

const serviceName = "quality-stat"
const serviceVersion = "0.1.0"

var configPath = flag.String("conf", "configs/quality-stat.yaml", "配置文件路径")

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

	// 4. MinIO 客户端
	minioClient, err := bizadapter.NewMinioClient(bizadapter.MinioConfig{
		Endpoint:  bc.MinIO.Endpoint,
		AccessKey: bc.MinIO.AccessKey,
		SecretKey: bc.MinIO.SecretKey,
		Bucket:    bc.MinIO.Bucket,
		UseSSL:    bc.MinIO.UseSSL,
		Timeout:   util.ParseDurationOr(bc.MinIO.Timeout, 30*time.Second),
	})
	if err != nil {
		panic("初始化MinIO客户端失败: " + err.Error())
	}

	// 5. 业务服务
	svc := service.NewQualityStatService(dataLayer, minioClient, appLogger)

	// 6. MQ 消费者
	finishConsumer := consumer.NewFinishConsumer(dataLayer, appLogger)
	mqConsumer, err := mq.NewConsumer(mq.ConsumerConfig{
		NameServers: bc.MQ.NameServers,
		GroupName:   bc.MQ.ConsumerGroup,
	}, appLogger)
	if err != nil {
		panic("初始化MQ消费者失败: " + err.Error())
	}
	if err := mqConsumer.Subscribe(mq.TopicReviewFinish, finishConsumer.Handle); err != nil {
		panic("订阅review_finish_topic失败: " + err.Error())
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
	v1.RegisterQualityStatServiceServer(grpcSrv, svc)

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
	v1.RegisterQualityStatServiceHTTPServer(httpSrv, svc)

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
