# ============================================================
# CR-System Go 微服务 Dockerfile（多阶段构建）
# 用法: docker build --build-arg SERVICE=iam-service -t cr-system/iam-service .
# ============================================================

# ---- 构建阶段 ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# 缓存 go module 下载
COPY go.mod go.sum ./
RUN go mod download

# 复制全部源码（单 module 结构需要全量 proto/pkg）
COPY . .

ARG SERVICE
RUN test -n "$SERVICE" && echo "Building $SERVICE..."

# 静态编译，避免 C 库依赖
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" \
    -o /out/service \
    ./app/${SERVICE}/cmd/main.go

# ---- 运行阶段 ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# 复制编译产物
COPY --from=builder /out/service .
# 复制配置文件（运行时通过 -conf 指定）
COPY --from=builder /src/configs ./configs

EXPOSE 8000 9000

# 默认配置路径（docker-compose 中按服务覆盖 CMD）
CMD ["./service", "-conf", "configs/service.yaml"]
