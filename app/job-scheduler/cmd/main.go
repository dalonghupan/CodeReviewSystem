// job-scheduler 定时任务服务入口（HLD §2.5 / LLD §3.6）
package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2"
	kconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"gopkg.in/yaml.v3"

	v1 "cr-system/api/crsystem/v1"
	"cr-system/app/job-scheduler/internal/bizadapter"
	"cr-system/app/job-scheduler/internal/conf"
	"cr-system/app/job-scheduler/internal/data"
	"cr-system/app/job-scheduler/internal/service"
	"cr-system/pkg/logger"
	"cr-system/pkg/middleware"
	"cr-system/pkg/mq"
	"cr-system/pkg/trace"
	"cr-system/pkg/util"
)

const serviceName = "job-scheduler"
const serviceVersion = "0.1.0"

var configPath = flag.String("conf", "configs/job-scheduler.yaml", "配置文件路径")

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

	// 4. 任务定义存储（内存）
	jobStore := data.NewJobStore()

	// 5. MQ 生产者（用于发送 Token 刷新等消息）
	var mqProducer *mq.Producer
	if len(bc.MQ.NameServers) > 0 {
		mqProducer, err = mq.NewProducer(mq.ProducerConfig{
			NameServers: bc.MQ.NameServers,
			GroupName:   bc.MQ.ConsumerGroup,
		}, serviceName)
		if err != nil {
			log.NewHelper(appLogger).Warnw("msg", "初始化MQ生产者失败，Token刷新功能将不可用", "error", err.Error())
			mqProducer = nil
		}
	}

	// 6. 任务执行器
	executor := bizadapter.NewJobExecutor(dataLayer, dataLayer.RDB(), mqProducer, appLogger)

	// 7. 业务服务
	svc := service.NewJobSchedulerService(jobStore, executor, dataLayer, appLogger)

	// 8. 中间件链
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
	v1.RegisterJobSchedulerServiceServer(grpcSrv, svc)

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
	v1.RegisterJobSchedulerServiceHTTPServer(httpSrv, svc)

	// 9. 启动
	app := kratos.New(
		kratos.Name(serviceName),
		kratos.Version(serviceVersion),
		kratos.Logger(appLogger),
		kratos.Server(grpcSrv, httpSrv),
	)

	defer func() {
		_ = shutdownTrace(context.Background())
		cleanupData()
		if mqProducer != nil {
			_ = mqProducer.Shutdown()
		}
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}

// expandEnv 解析配置中的 ${ENV_VAR} 占位符（K8s Secret 注入，LLD §9-1）
func expandEnv(bc *conf.Bootstrap) {
	raw, err := yaml.Marshal(bc)
	if err != nil {
		return
	}
	expanded := os.ExpandEnv(string(raw))
	_ = yaml.Unmarshal([]byte(expanded), bc)
}
