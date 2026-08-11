# ============================================================
# CR-System Web Dockerfile（多阶段构建）
# ============================================================

# ---- 构建阶段 ----
FROM node:22-alpine AS builder

WORKDIR /app

# NEXT_PUBLIC_* 变量在构建期内联进前端产物（运行时 env 不生效），由 compose build args 注入
ARG NEXT_PUBLIC_API_BASE=http://localhost:8000
ENV NEXT_PUBLIC_API_BASE=$NEXT_PUBLIC_API_BASE

# 缓存 npm 依赖
COPY web/package.json web/package-lock.json ./
RUN npm ci

# 复制源码并构建
COPY web/ .
RUN npm run build

# ---- 运行阶段 ----
FROM node:22-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production

COPY --from=builder /app/package.json /app/package-lock.json ./
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/next.config.ts ./
COPY --from=builder /app/tsconfig.json ./

EXPOSE 3000

CMD ["npm", "run", "start"]
