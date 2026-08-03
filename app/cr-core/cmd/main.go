// cr-core 评审核心服务入口（HLD §2.2 / LLD §3.3）
package main

import (
	"context"
	"flag"
	"os"
	"time"

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
	"cr-system/app/cr-core/internal/bizadapter"
	"cr-system/app/cr-core/internal/conf"
	"cr-system/app/cr-core/internal/data"
	"cr-system/app/cr-core/internal/service"
	"cr-system/pkg/logger"
	"cr-system/pkg/middleware"
	"cr-system/pkg/trace"
	"cr-system/pkg/util"
)

const serviceName = "cr-core"
const serviceVersion = "0.1.0"

var configPath = flag.String("conf", "configs/cr-core.yaml", "配置文件路径")

func main() {
	flag.Parse()

	// 1. 加载配置（kratos config + yaml，环境变量占位符由部署层注入）
	c := kconfig.New(kconfig.WithSource(file.NewSource(*configPath)))
	if err := c.Load(); err != nil {
		panic("加载配置失败: " + err.Error())
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic("解析配置失败: " + err.Error())
	}
	expandEnv(&bc)

	// 2. 日志与链路追踪（LLD §10）
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

	// 3. 数据层与第三方适配
	dataLayer, cleanupData, err := data.NewData(&bc.Data, appLogger)
	if err != nil {
		panic("初始化数据层失败: " + err.Error())
	}
	sonarClient := bizadapter.NewSonarClient(
		bc.Sonar.BaseURL,
		bc.Sonar.Token,
		util.ParseDurationOr(bc.Sonar.Timeout, 10*time.Second),
	)

	// 4. 业务服务
	svc := service.NewReviewService(dataLayer, sonarClient, appLogger)

	// 5. 中间件链：recovery → tracing → 错误统一 → 请求日志 → JWT鉴权 → 参数校验
	authMW, err := middleware.JWTAuth(middleware.AuthConfig{
		JWKSURL:   "",   // 当前版本 Keycloak 未就绪，跳过鉴权
		Whitelist: nil,  // 全部放行
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
	v1.RegisterReviewServiceServer(grpcSrv, svc)

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
	v1.RegisterReviewServiceHTTPServer(httpSrv, svc)

	// 6. 启动
	app := kratos.New(
		kratos.Name(serviceName),
		kratos.Version(serviceVersion),
		kratos.Logger(appLogger),
		kratos.Server(grpcSrv, httpSrv),
	)

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
	raw, err := yaml.Marshal(bc)
	if err != nil {
		return
	}
	expanded := os.ExpandEnv(string(raw))
	_ = yaml.Unmarshal([]byte(expanded), bc)
}
