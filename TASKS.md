# CR-System 任务清单（跨会话衔接用）

> 用途：记录当前进度、待办事项、已做决策与遗留问题，供下一次对话快速衔接。
> 维护规则：每次会话结束或有重要进展时更新本文件；任务完成后标记 ✅ 并注明完成日期。

当前阶段：**阶段三（编码开发）进行中** —— pkg 公共层完成，iam-service 首个服务已落地
最后更新：2026-07-31（二次更新）

---

## 一、待决策事项（阻塞项优先）

| # | 事项 | 状态 | 说明 |
|---|------|------|------|
| D1 | Go module 结构：单 module（mono-repo）vs 每服务独立 module | ✅ 已决策 2026-07-31 | **单 module**，模块名 `cr-system`，proto/pkg/app 全共享 |

## 二、阶段二收尾：设计基线补齐

| # | 任务 | 状态 | 产出位置 | 备注 |
|---|------|------|----------|------|
| P1 | 编写 `iam.proto`（CheckDataPermission、SyncUserFromKeycloak、租户/角色管理） | ✅ 已完成 | proto/crsystem/v1/iam.proto | 2026-07-30 已有（上次会话产出） |
| P2 | 编写 `git_adapter.proto`（OAuth授权、MR查询、Diff拉取、限流状态） | ✅ 已完成 | proto/crsystem/v1/git_adapter.proto | 2026-07-30 已有 |
| P3 | 编写 `review.proto`（评审单CRUD、状态流转、行级评论、Sonar门禁） | ✅ 已完成 | proto/crsystem/v1/review.proto | 2026-07-30 已有 |
| P4 | 编写 `quality.proto`（缺陷台账、指标统计、报表生成/下载） | ✅ 已完成 | proto/crsystem/v1/quality.proto | 2026-07-30 已有 |
| P5 | 编写 `message.proto`（站内信、偏好配置、渠道管理） | ✅ 已完成 | proto/crsystem/v1/message.proto | 2026-07-30 已有 |
| P6 | 编写 `job_scheduler.proto`（任务列表、手动触发、执行记录） | ✅ 已完成 2026-07-31 | proto/crsystem/v1/job_scheduler.proto | 含 JobType 枚举对齐 LLD §3.6 任务清单 |
| P7 | 配置 buf 生成脚本并生成代码 | ✅ 已完成 2026-07-31 | buf.yaml、buf.gen.yaml、scripts/gen_proto.sh → api/crsystem/v1/ | buf+4插件均经 go install 安装；生成命令 `buf generate --path proto/crsystem/v1` |

## 三、阶段三：公共层 pkg 建设（服务编码前置）

| # | 任务 | 状态 | 产出位置 | 备注 |
|---|------|------|----------|------|
| G1 | 初始化 Go module（单 module `cr-system`） | ✅ 已完成 2026-07-31 | go.mod | kratos v2.9.2 / grpc v1.83 / otel v1.35 |
| G2 | pkg/logger：zap 结构化日志封装，支持 TraceID 注入 | ✅ 已完成 2026-07-31 | pkg/logger/logger.go | 适配 kratos log.Logger，JSON输出供 Loki 采集 |
| G3 | pkg/trace：OTel/Jaeger 链路追踪初始化 | ✅ 已完成 2026-07-31 | pkg/trace/trace.go | OTLP gRPC 导出，W3C TraceContext 传播 |
| G4 | pkg/middleware：错误统一、JWT鉴权、请求日志 | ✅ 已完成 2026-07-31 | pkg/middleware/ | errors.go/auth.go/logging.go/context.go；JWT 用 keyfunc/v3 对接 Keycloak JWKS |
| G5 | pkg/util：AES-GCM、UUID、分页、退避重试、时长解析 | ✅ 已完成 2026-07-31 | pkg/util/ | 含单测 util_test.go |
| G6 | pkg/mq：RocketMQ 生产者/消费者封装 | ✅ 已完成 2026-07-31 | pkg/mq/producer.go、consumer.go | rocketmq-client-go/v2；普通/事务/延迟消息、重试3次进死信 |

## 四、阶段三：微服务编码（按依赖顺序）

> 统一分层约束（LLD §1.2）：api → service → data → bizadapter；禁止跨服务直连数据库；单测覆盖率 ≥85%。

| # | 任务 | 状态 | 依赖 |
|---|------|------|------|
| S1 | iam-service：租户/用户/角色/RBAC + Keycloak 适配 + CheckDataPermission gRPC | 🔶 主体完成 2026-07-31（编译+核心单测通过；service层单测、Keycloak联调待补） | G1-G6, P1 |
| S2 | git-adapter：OAuth 授权管理、GitLab/Gitee/GitHub 适配器、MR 同步、Diff 解析、限流 | ⬜ 未开始 | S1, P2 |
| S3 | cr-core：评审单生命周期、状态机（待评审→评审中→驳回待修改→复审提交→复审通过→归档）、行级评论（按月分表 comment_yyyyMM）、Sonar 门禁、事务消息 | ⬜ 未开始 | S1, S2, P3 |
| S4 | message-push：站内信/企业微信/SMTP 适配器、偏好过滤、消费 review_notice_topic | ⬜ 未开始 | S3, P5 |
| S5 | quality-stat：消费 review_finish_topic 归集缺陷、指标计算、Excel/PDF 报表 + MinIO 存储 | ⬜ 未开始 | S3, P4 |
| S6 | job-scheduler：仓库增量同步(6h)、数据对账(01:00)、缓存清理(02:00)、Token刷新(03:00)、月报(每月首日) | ⬜ 未开始 | S2, S5, P6 |

## 五、前端 web/（Next.js 19 + Shadcn/ui + Monaco + TanStack Query）

| # | 任务 | 状态 | 备注 |
|---|------|------|------|
| F1 | 工程脚手架：请求封装、Token 拦截、权限路由守卫 | ⬜ 未开始 | LLD §8 |
| F2 | 页面：登录、仓库管理、评审创建、Diff 评审、缺陷台账、数据大盘、个人设置 | ⬜ 未开始 | 核心逻辑覆盖率 ≥75% |
| F3 | Monaco Diff 双栏组件封装（行号定位、评论悬浮、大文件虚拟滚动） | ⬜ 未开始 | |
| F4 | SSE 实时消息订阅（评审状态变更、新评论提醒） | ⬜ 未开始 | |

## 六、运维部署 deploy/（可与三、四并行）

| # | 任务 | 状态 | 备注 |
|---|------|------|------|
| O1 | deploy/docker：各服务 Dockerfile、中间件 docker-compose（开发联调用） | ⬜ 未开始 | |
| O2 | 中间件测试环境部署：PostgreSQL17 / Redis7.2 / RocketMQ5 / ES8 / MinIO / Keycloak / APISIX | ⬜ 未开始 | PDP 要求阶段三前期就绪 |
| O3 | deploy/apisix：路由、JWT 校验、限流、gRPC 协议转换配置 | ⬜ 未开始 | |
| O4 | deploy/k8s：Deployment/Service/ConfigMap/Secret/CronJob 清单 + ArgoCD 配置 | ⬜ 未开始 | |

---

## 已完成 ✅

| 完成日期 | 事项 |
|----------|------|
| 2026-07-30 | 10 份 CMMI 文档 V1.0（doc/CR-SYS-01 ~ 10） |
| 2026-07-30 | 数据库 DDL 5 份（sql/001_iam ~ 005_system） |
| 2026-07-30 | 6 个微服务目录骨架（app/* 五层结构） |
| 2026-07-30 | pkg/errcode 统一错误码（10xxx/20xxx/30xxx + HTTP 映射） |
| 2026-07-30 | pkg/mq Topic/Group/延迟等级常量（topics.go、messages.go） |
| 2026-07-30 | third_party proto（google/api、validate） |
| 2026-07-31 | D1 决策：单 module `cr-system`；go.mod 初始化 |
| 2026-07-31 | job_scheduler.proto 补齐；buf 工具链 + 全部 pb 代码生成（api/crsystem/v1/） |
| 2026-07-31 | pkg 公共层全部完成（logger/trace/middleware/util/mq）+ 单测通过 |
| 2026-07-31 | iam-service 主体完成：conf/data(读写分离+权限缓存)/service(全部RPC)/bizadapter(Keycloak)/cmd 启动装配；configs/iam-service.yaml |

## 环境备忘（新会话必读）

- **Git 仓库已建立（2026-07-31）**：基线提交 `41a2006`（88 文件），分支模型 Git Flow：`main`（主干/基线）+ `develop`（集成分支，当前所在），后续开发走 feature 分支
- 工具链：Go 1.24.5；buf/protoc-gen-*/protoc-gen-go-http/validate 均装于 GOPATH/bin（brew 网络不通，走 go install）
- proto 重新生成：`bash scripts/gen_proto.sh`（需 PATH 含 `$(go env GOPATH)/bin`）
- 配置规范：yaml 中时长一律用字符串（"5s"）由 `util.ParseDurationOr` 解析；`${ENV_VAR}` 占位符由 main 启动时 os.ExpandEnv 展开（K8s Secret 注入）
- 数据库访问：sqlx + pgx 驱动；写走 writeDB 主库、读走 readDB 从库（未配置从库时合并）
- 下一步：S2 git-adapter（依赖 sql/002_git.sql 表结构）或先补 iam-service 单测覆盖率 |

## 关键设计约束速查（新会话必读）

- 状态机硬编码：待评审→评审中→驳回待修改→复审提交→复审通过→归档（LLD §3.3）
- 分布式锁：`lock:review_mr:{mrId}` 防同一 MR 重复建单，30s 超时
- 缓存 Key：`diff:{mrId}:{commitHash}`(7天)、`repo:info:{repoId}`(12h)、`user:perm:{uid}:{tenantId}`(2h)、`git_limit:{platform}:{ip}`(按小时)
- MQ：创建评审单用事务消息绑定本地事务；到期提醒用延迟消息（等级14≈1天）；重试3次进死信
- 评论分表：comment_yyyyMM 按月自动分表路由
- 错误码：系统级10xxx / 业务级20xxx / 第三方30xxx，第三方重试 1s/3s/5s 指数退避
- 安全：密钥只存 K8s Secret；Token AES 加密存储；禁止裸写 SQL 拼接
