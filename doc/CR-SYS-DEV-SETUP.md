# CR-System 本地开发环境部署指南

> 版本：1.0 | 最后更新：2026-08-06

---

## 目录

1. [项目概述](#1-项目概述)
2. [前置条件](#2-前置条件)
3. [获取代码](#3-获取代码)
4. [方案一：Docker Compose 一键部署（推荐）](#4-方案一docker-compose-一键部署推荐)
5. [方案二：本地原生开发（无 Docker）](#5-方案二本地原生开发无-docker)
6. [验证部署](#6-验证部署)
7. [常用操作](#7-常用操作)
8. [故障排查](#8-故障排查)
9. [开发规范速查](#9-开发规范速查)

---

## 1. 项目概述

CR-System 是一个企业级代码评审与质量管理系统，采用微服务架构：

### 后端微服务（Go + Kratos）

| 服务 | 端口（HTTP） | 端口（gRPC） | 职责 |
|------|-------------|-------------|------|
| **iam-service** | 8001 | 9001 | 身份认证、租户管理、RBAC 权限 |
| **git-adapter** | 8002 | 9002 | Git 平台 OAuth、MR 同步、Diff 解析 |
| **cr-core** | 8003 | 9003 | 评审单生命周期、行级评论、Sonar 门禁 |
| **message-push** | 8004 | 9004 | 站内信、企业微信、邮件推送 |
| **quality-stat** | 8005 | 9005 | 缺陷台账、指标计算、报表生成 |
| **job-scheduler** | 8006 | 9006 | 定时任务：同步/对账/缓存清理/月报 |

### 前端（Next.js 16 + Shadcn/ui + TanStack Query）

| 模块 | 端口 | 说明 |
|------|------|------|
| **web** | 3000 | 管理控制台 |

### 中间件依赖

| 中间件 | 用途 |
|--------|------|
| PostgreSQL 17 | 主数据库 |
| Redis 7.2 | 缓存 + 分布式锁 |
| RocketMQ 5 | 异步消息（评审完成、通知推送等） |
| MinIO | 报表文件存储 |
| Keycloak 26 | 统一认证 + OIDC/JWT |
| Jaeger | 链路追踪 |

### 技术栈

- **后端语言**：Go 1.25
- **后端框架**：Kratos v2.9.2
- **前端框架**：Next.js 16 + React 19
- **UI 组件**：Shadcn/ui + Base UI
- **状态管理**：TanStack Query v5 + TanStack Table
- **数据访问**：sqlx + pgx (PostgreSQL)、go-redis
- **消息队列**：RocketMQ (rocketmq-client-go)
- **认证**：Keycloak + JWT (keyfunc/v3)
- **可观测**：OpenTelemetry + Jaeger + Zap

---

## 2. 前置条件

### 硬件要求

| 项目 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 4 核 | 8 核 |
| 内存 | 8 GB | 16 GB |
| 磁盘 | 20 GB 可用 | 50 GB SSD |

### 软件要求

| 软件 | 版本要求 | 验证命令 |
|------|---------|---------|
| Docker | 24+ | `docker --version` |
| Docker Compose | v2 | `docker compose version` |
| Go | 1.25+ | `go version` |
| Node.js | 22+ | `node --version` |
| npm | 10+ | `npm --version` |
| Git | 2.30+ | `git --version` |

### 端口占用

确保以下端口未被占用：

```
5432  (PostgreSQL)
6379  (Redis)
9876  (RocketMQ Namesrv)
10909 (RocketMQ Broker v1)
10911 (RocketMQ Broker v2)
9000  (MinIO API)
9001  (MinIO Console)
8080  (Keycloak)
4317  (Jaeger gRPC)
16686 (Jaeger UI)
8001-8006  (微服务 HTTP)
9001-9006  (微服务 gRPC)
3000  (Web 前端)
```

---

## 3. 获取代码

```bash
# 克隆仓库
git clone <repository-url> cr-system
cd cr-system

# 查看分支结构
git branch -a

# 主要分支说明
#   main        — 主干分支（稳定发布版本）
#   develop     — 集成分支（日常开发目标分支）
#   feature/*   — 功能开发分支
```

---

## 4. 方案一：Docker Compose 一键部署（推荐）

### 4.1 启动所有服务

```bash
# 进入 docker 目录
cd deploy/docker

# 首次启动（构建镜像 + 启动所有容器）
docker compose up -d

# 查看启动日志
docker compose logs -f

# 等待所有服务健康检查通过（约 2-3 分钟）
docker compose ps
```

### 4.2 验证服务状态

所有服务就绪后，应看到所有容器状态为 `healthy` 或 `running`：

```
NAME                 SERVICE              STATUS          PORTS
cr-postgres          postgres             healthy         0.0.0.0:5432->5432/tcp
cr-redis             redis                healthy         0.0.0.0:6379->6379/tcp
cr-rocketmq-namesrv  rocketmq-namesrv     healthy         0.0.0.0:9876->9876/tcp
cr-rocketmq-broker   rocketmq-broker      healthy         0.0.0.0:10909,10911->...
cr-minio             minio                healthy         0.0.0.0:9000-9001->...
cr-keycloak          keycloak             running         0.0.0.0:8080->8080/tcp
cr-jaeger            jaeger               running         0.0.0.0:4317,16686->...
cr-iam-service       iam-service          running         0.0.0.0:8001,9001->...
cr-git-adapter       git-adapter          running         0.0.0.0:8002,9002->...
cr-cr-core           cr-core              running         0.0.0.0:8003,9003->...
cr-message-push      message-push         running         0.0.0.0:8004,9004->...
cr-quality-stat      quality-stat         running         0.0.0.0:8005,9005->...
cr-job-scheduler     job-scheduler        running         0.0.0.0:8006,9006->...
cr-web               web                  running         0.0.0.0:3000->3000/tcp
```

### 4.3 服务访问地址

| 服务 | 地址 | 说明 |
|------|------|------|
| **Web 前端** | http://localhost:3000 | 管理控制台 |
| **Keycloak 管理** | http://localhost:8080 | Admin: `admin` / `admin` |
| **Jaeger UI** | http://localhost:16686 | 链路追踪查询 |
| **MinIO Console** | http://localhost:9001 | 用户: `minioadmin` / `minioadmin` |
| **MinIO API** | http://localhost:9000 | S3 兼容对象存储 |

### 4.4 预置测试账号

Keycloak Realm `cr-system` 预置了三个测试用户：

| 用户名 | 密码 | 角色 | 说明 |
|--------|------|------|------|
| `admin` | `admin123` | 管理员 | 系统管理权限 |
| `developer` | `dev123` | 开发者 | 提交代码、发起评审 |
| `reviewer` | `review123` | 评审人 | 评审代码、驳回/通过 |

### 4.5 数据库初始化

首次启动 PostgreSQL 容器时，`init-sql/001_init.sql` 会自动执行，完成建库建表：

- 5 个业务模块的 DDL（IAM / Git / Review / Quality / System）
- UUID 扩展、索引、外键约束
- 无需手动执行 SQL

### 4.6 重新构建单个服务

修改代码后，重新构建并重启指定服务：

```bash
# 重新构建并启动 cr-core
docker compose up -d --build cr-core

# 查看日志
docker compose logs -f cr-core
```

---

## 5. 方案二：本地原生开发（无 Docker）

> 适用于没有 Docker 环境的场景，或需要对单个服务进行断点调试。

### 5.1 启动中间件

#### PostgreSQL 17

```bash
# macOS (Homebrew)
brew install postgresql@17
brew services start postgresql@17
createdb cr_system

# 或使用 Docker 仅启动数据库
docker run -d --name cr-pg \
  -e POSTGRES_USER=cr_system \
  -e POSTGRES_PASSWORD=cr_dev_password \
  -e POSTGRES_DB=cr_system \
  -p 5432:5432 \
  postgres:17-alpine

# 执行初始化 SQL
psql -U cr_system -d cr_system -f sql/001_iam.sql
psql -U cr_system -d cr_system -f sql/002_git.sql
psql -U cr_system -d cr_system -f sql/003_review.sql
psql -U cr_system -d cr_system -f sql/004_quality.sql
psql -U cr_system -d cr_system -f sql/005_system.sql
```

#### Redis 7.2

```bash
brew install redis@7.2
brew services start redis

# 或 Docker
docker run -d --name cr-redis -p 6379:6379 redis:7.2-alpine
```

#### RocketMQ 5

```bash
# Docker（推荐）
docker run -d --name cr-rocketmq-namesrv -p 9876:9876 apache/rocketmq:5.3.1 sh mqnamesrv
docker run -d --name cr-rocketmq-broker -p 10909:10909 -p 10911:10911 \
  -e NAMESRV_ADDR=localhost:9876 \
  apache/rocketmq:5.3.1 sh mqbroker
```

#### MinIO

```bash
docker run -d --name cr-minio \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -p 9000:9000 -p 9001:9001 \
  minio/minio server /data --console-address ":9001"

# 创建 bucket
docker run --rm --entrypoint /bin/sh minio/mc -c "
  mc alias set local http://host.docker.internal:9000 minioadmin minioadmin
  mc mb local/cr-system-reports --ignore-existing
"
```

#### Keycloak

```bash
docker run -d --name cr-keycloak \
  -e KC_BOOTSTRAP_ADMIN_USERNAME=admin \
  -e KC_BOOTSTRAP_ADMIN_PASSWORD=admin \
  -p 8080:8080 \
  -v $(pwd)/deploy/docker/keycloak:/opt/keycloak/data/import \
  quay.io/keycloak/keycloak:26.0 start-dev --import-realm
```

### 5.2 配置环境变量

```bash
# 复制环境变量模板
cp deploy/docker/.env .env.local

# 根据本地环境修改配置（如端口、密码等）
# 关键变量说明：
#   POSTGRES_DSN=postgres://cr_system:cr_dev_password@localhost:5432/cr_system?sslmode=disable
#   REDIS_ADDR=localhost:6379
#   ROCKETMQ_NAMESRV=localhost:9876
#   MINIO_ENDPOINT=http://localhost:9000
#   KEYCLOAK_BASE_URL=http://localhost:8080
#   OTEL_ENDPOINT=localhost:4317
```

### 5.3 启动后端微服务

每个微服务独立启动，命令相同：

```bash
# 启动 iam-service
cd app/iam-service/cmd
POSTGRES_DSN="postgres://..." \
REDIS_ADDR="localhost:6379" \
KEYCLOAK_BASE_URL="http://localhost:8080" \
go run main.go -conf ../../../configs/iam-service.yaml

# 启动 git-adapter（新终端）
cd app/git-adapter/cmd
POSTGRES_DSN="postgres://..." \
REDIS_ADDR="localhost:6379" \
go run main.go -conf ../../../configs/git-adapter.yaml

# 启动 cr-core（需 iam-service + git-adapter 就绪）
cd app/cr-core/cmd
POSTGRES_DSN="postgres://..." \
REDIS_ADDR="localhost:6379" \
ROCKETMQ_NAMESRV="localhost:9876" \
go run main.go -conf ../../../configs/cr-core.yaml

# 启动 message-push
cd app/message-push/cmd
POSTGRES_DSN="postgres://..." \
REDIS_ADDR="localhost:6379" \
ROCKETMQ_NAMESRV="localhost:9876" \
go run main.go -conf ../../../configs/message-push.yaml

# 启动 quality-stat（需 MinIO 就绪）
cd app/quality-stat/cmd
POSTGRES_DSN="postgres://..." \
REDIS_ADDR="localhost:6379" \
ROCKETMQ_NAMESRV="localhost:9876" \
MINIO_ENDPOINT="http://localhost:9000" \
go run main.go -conf ../../../configs/quality-stat.yaml

# 启动 job-scheduler
cd app/job-scheduler/cmd
POSTGRES_DSN="postgres://..." \
REDIS_ADDR="localhost:6379" \
ROCKETMQ_NAMESRV="localhost:9876" \
go run main.go -conf ../../../configs/job-scheduler.yaml
```

> **注意**：配置文件中的 `${ENV_VAR}` 占位符由 main.go 的 `os.ExpandEnv` 解析，必须通过环境变量注入。

### 5.4 启动前端

```bash
# 安装依赖
cd web
npm install

# 开发模式启动（热更新）
NEXT_PUBLIC_API_BASE=http://localhost:8001 npm run dev

# 或生产构建
npm run build
npm run start
```

---

## 6. 验证部署

### 6.1 健康检查

```bash
# 验证 iam-service
curl http://localhost:8001/health

# 验证 git-adapter
curl http://localhost:8002/health

# 验证 cr-core
curl http://localhost:8003/health

# 验证 message-push
curl http://localhost:8004/health

# 验证 quality-stat
curl http://localhost:8005/health

# 验证 job-scheduler
curl http://localhost:8006/health
```

### 6.2 获取 Keycloak Token

```bash
# 使用预置测试账号获取 Token
curl -s -X POST http://localhost:8080/realms/cr-system/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=cr-system-web" \
  -d "username=admin" \
  -d "password=admin123" \
  -d "grant_type=password" | jq -r '.access_token'
```

### 6.3 验证 API

```bash
# 设置 Token
TOKEN="<上一步获取的 Token>"

# 创建评审单
curl -X POST http://localhost:8003/api/v1/reviews \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "default",
    "repo_id": "repo-1",
    "mr_id": "1",
    "title": "测试评审"
  }'

# 获取评审列表
curl http://localhost:8003/api/v1/reviews \
  -H "Authorization: Bearer $TOKEN"
```

### 6.4 访问前端

打开浏览器访问 http://localhost:3000 ，使用预置账号登录：

| 用户名 | 密码 |
|--------|------|
| `admin` | `admin123` |
| `developer` | `dev123` |
| `reviewer` | `review123` |

---

## 7. 常用操作

### 7.1 Docker Compose 管理

```bash
# 启动
docker compose up -d

# 停止（保留数据）
docker compose stop

# 停止并删除容器
docker compose down

# 完全清理（删除数据卷，数据库数据会丢失）
docker compose down -v

# 查看日志
docker compose logs -f

# 查看特定服务日志
docker compose logs -f cr-core

# 重启单个服务
docker compose restart cr-core

# 重新构建并启动
docker compose up -d --build cr-core

# 查看容器状态
docker compose ps
```

### 7.2 数据库操作

```bash
# 进入 PostgreSQL
docker compose exec postgres psql -U cr_system -d cr_system

# 查看表
\dt

# 查看表结构
\d review

# 查询示例
SELECT review_id, title, status FROM review LIMIT 10;
```

### 7.3 查看 Jaeger 链路追踪

1. 打开 http://localhost:16686
2. 在 Service 下拉框选择 `cr-core`（或其他服务）
3. 点击 Find Traces 查看请求链路

### 7.4 查看 RocketMQ 消息

```bash
# 进入 RocketMQ 容器
docker compose exec rocketmq-namesrv sh

# 查看集群状态
mqadmin clusterList -n localhost:9876

# 查看消费组状态
mqadmin consumerProgress -n localhost:9876 -g GID_cr_core_producer
```

---

## 8. 故障排查

### 8.1 容器启动失败

```bash
# 查看具体错误日志
docker compose logs <service-name>

# 常见问题：
#   - 端口被占用 → 修改 .env 或停止占用程序
#   - 镜像拉取失败 → 检查网络，配置镜像加速器
#   - Keycloak 启动慢 → 首次启动需初始化数据库，约 30-60 秒
```

### 8.2 PostgreSQL 连接失败

```bash
# 检查数据库是否就绪
docker compose exec postgres pg_isready -U cr_system

# 进入数据库检查
docker compose exec postgres psql -U cr_system -d cr_system -c "SELECT 1"

# 常见问题：
#   - DSN 中密码错误 → 检查 .env 中的 POSTGRES_PASSWORD
#   - 数据库未初始化 → 检查 init-sql/ 目录是否存在
```

### 8.3 Keycloak 认证失败

```bash
# 检查 Keycloak 是否就绪
curl http://localhost:8080/realms/cr-system/.well-known/openid-configuration

# 常见问题：
#   - Realm 未导入 → 检查 keycloak/cr-system-realm.json 是否存在
#   - Token 获取失败 → 确认 client_id 和密码正确
#   - 需要重新导入 Realm → docker compose down -v && docker compose up -d
```

### 8.4 RocketMQ 消息发送失败

```bash
# 检查 Namesrv 是否就绪
docker compose exec rocketmq-namesrv nc -z localhost 9876

# 检查 Broker 是否已注册
docker compose exec rocketmq-namesrv sh -c "mqadmin clusterList -n localhost:9876"

# 常见问题：
#   - Broker 未连上 Namesrv → 检查 broker.conf 中的 namesrvAddr
#   - 磁盘空间不足 → RocketMQ 默认需要 4GB+ 可用空间
```

### 8.5 微服务 gRPC 调用失败

```bash
# 检查服务是否启动
curl http://localhost:8001/health

# 检查依赖服务是否就绪（cr-core 依赖 iam-service 和 git-adapter）
# 检查日志
docker compose logs cr-core

# 常见问题：
#   - 依赖服务未就绪 → 确认依赖的 gRPC 端口可访问
#   - JWT 鉴权失败 → 确认 Keycloak 已启动且 Realm 已导入
#   - 配置中的服务名解析失败 → Docker 内部用服务名，本地运行用 localhost
```

---

## 9. 开发规范速查

### 9.1 Git 分支策略

```
main → develop → feature/*（Git Flow）
```

- 新功能从 `develop` 创建 `feature/<功能名>` 分支
- 完成后 `--no-ff` 合回 `develop`
- `main` 仅用于稳定发布版本

### 9.2 代码结构

```
cr-system/
├── api/              # 生成的 protobuf Go 代码
├── app/              # 微服务源码
│   ├── iam-service/
│   ├── git-adapter/
│   ├── cr-core/
│   ├── message-push/
│   ├── quality-stat/
│   └── job-scheduler/
├── configs/          # 服务配置文件（${ENV_VAR} 占位符）
├── deploy/           # 部署配置
│   └── docker/       # Docker Compose 开发环境
├── doc/              # 设计文档
├── pkg/              # 公共库（logger/trace/middleware/util/mq）
├── proto/            # protobuf 定义
├── sql/              # DDL 建表语句
├── scripts/          # 工具脚本
└── web/              # 前端源码（Next.js）
```

### 9.3 关键设计约束

| 项目 | 说明 |
|------|------|
| 状态机 | 待评审 → 评审中 → 驳回待修改 → 复审提交 → 复审通过 → 归档 |
| 分布式锁 | `lock:review_mr:{mrId}` 防重复建单，30s 超时 |
| 缓存 Key | `diff:{mrId}:{hash}`(7d)、`repo:info:{repoId}`(12h)、`user:perm:{uid}:{tenantId}`(2h) |
| MQ | 事务消息绑定本地事务；延迟消息等级14≈1天；重试3次进死信 |
| 评论分表 | `comment_yyyyMM` 按月自动路由 |
| 错误码 | 系统10xxx / 业务20xxx / 第三方30xxx，重试 1s/3s/5s 指数退避 |

### 9.4 重新生成 protobuf

```bash
# 确保 buf 和相关插件已安装
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
go install github.com/envoyproxy/protoc-gen-validate@latest

# 生成代码
export PATH="$PATH:$(go env GOPATH)/bin"
bash scripts/gen_proto.sh
```

---

> **文档维护**：本指南应与 `deploy/docker/` 下的配置文件保持同步。更新中间件版本或新增服务时，请同步更新本文档和 `TASKS.md`。
