# CR-System 开发环境（Docker Compose）

## 前置条件

- Docker Engine 24+
- Docker Compose v2

## 快速启动

```bash
# 1. 进入 docker 目录
cd deploy/docker

# 2. 启动所有服务
docker compose up -d

# 3. 查看日志
docker compose logs -f
```

## 服务访问地址

### 中间件

| 服务 | 地址 | 说明 |
|------|------|------|
| PostgreSQL | `localhost:5432` | 数据库 `cr_system`，用户 `cr_system` |
| Redis | `localhost:6379` | 无密码 |
| RocketMQ | `localhost:9876` | Namesrv |
| MinIO | `localhost:9000` | API；Console: `localhost:9001`，用户 `minioadmin` |
| Jaeger | `localhost:16686` | 链路追踪 UI |
| Keycloak | `localhost:8080` | Admin: `admin`/`admin`，Realm: `cr-system` |

### 微服务

| 服务 | HTTP | gRPC | 说明 |
|------|------|------|------|
| iam-service | `8001` | `9001` | 身份权限 |
| git-adapter | `8002` | `9002` | Git 适配 |
| cr-core | `8003` | `9003` | 评审核心 |
| message-push | `8004` | `9004` | 消息推送 |
| quality-stat | `8005` | `9005` | 质量统计 |
| job-scheduler | `8006` | `9006` | 定时任务 |
| Web | `3000` | - | 前端页面 |

## 测试账号

Keycloak 预置账号（Realm: `cr-system`）：

| 用户名 | 密码 | 角色 |
|--------|------|------|
| `admin` | `admin123` | 管理员 |
| `developer` | `dev123` | 开发者 |
| `reviewer` | `review123` | 评审人 |

## 常用命令

```bash
# 启动
docker compose up -d

# 停止（保留数据）
docker compose stop

# 停止并删除容器
docker compose down

# 完全清理（删除数据卷）
docker compose down -v

# 查看日志
docker compose logs -f

# 查看特定服务日志
docker compose logs -f iam-service

# 重启单个服务
docker compose restart cr-core

# 重新构建并启动
docker compose up -d --build cr-core
```

## 配置说明

- 环境变量在 `.env` 文件中，可修改密码等敏感信息
- 各微服务配置文件位于项目 `configs/` 目录
- Keycloak Realm 配置在 `keycloak/cr-system-realm.json`
- 数据库初始化 SQL 在 `init-sql/001_init.sql`
- RocketMQ Broker 配置在 `rocketmq/broker.conf`
