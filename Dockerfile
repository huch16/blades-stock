# ==============================================================================
# Blades Stock — Cloud-Native Web/SaaS Dockerfile
# 多阶段极轻量构建（纯 Go 静态编译，零 CGO 依赖，内置前端静态资源与中文字体）
# ==============================================================================

# ------------------------------------------------------------------------------
# 阶段 1：编译前端 Vue 3 静态资源
# ------------------------------------------------------------------------------
FROM node:20-alpine AS frontend-builder
WORKDIR /build/frontend

# 提升 Node 内存上限至 6GB（宿主机有 12GB 物理内存，解除 V8 默认 2GB 导致的 OOM）
ENV NODE_OPTIONS="--max-old-space-size=6144"

# 先拷贝 package.json 利用 Docker 缓存层
COPY frontend/package*.json ./
RUN npm install

# 拷贝前端源码并进行生产环境打包
COPY frontend/ ./
ENV VITE_MODE=web
RUN npm run build

# ------------------------------------------------------------------------------
# 阶段 2：编译纯 Go 服务端二进制 (cmd/server)
# ------------------------------------------------------------------------------
FROM golang:1.26-alpine AS backend-builder
WORKDIR /build

# 安装基础构建工具与 Git
RUN apk add --no-cache git build-base

# 启用 Go 工具链自动模式
ENV GOTOOLCHAIN=auto

# 缓存 Go 依赖
COPY go.mod go.sum ./
RUN go mod download

# 拷贝后端与服务源码
COPY backend/ ./backend/
COPY cmd/ ./cmd/

# 静态编译纯 Go Web 服务端程序（禁用 CGO，移除调试符号，极致压缩体积）
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -ldflags="-s -w" -o /blades-stock-server ./cmd/server

# ------------------------------------------------------------------------------
# 阶段 3：超轻量生产运行时镜像
# ------------------------------------------------------------------------------
FROM alpine:3.20
WORKDIR /app

# 安装必要的运行时支撑库：HTTPS 证书、时区数据、curl（健康检查）、中文字体（防止图表乱码）
RUN apk add --no-cache ca-certificates tzdata curl font-noto-cjk

# 设置统一时区为亚洲/上海标准时间 (CST, UTC+8)
ENV TZ=Asia/Shanghai
ENV BLADES_DATA_DIR=/app/data
ENV BLADES_LOG_DIR=/app/logs
ENV PORT=8080
ENV HOST=0.0.0.0

# 从前置阶段拷贝编译产物
COPY --from=backend-builder /blades-stock-server /app/blades-stock-server
COPY --from=frontend-builder /build/frontend/dist /app/frontend/dist

# 挂载数据持久化目录与日志目录
VOLUME ["/app/data", "/app/logs"]

# 暴露 Web 端口
EXPOSE 8080

# 容器健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://127.0.0.1:8080/health || exit 1

# 启动服务端
ENTRYPOINT ["/app/blades-stock-server"]
CMD ["--data-dir=/app/data", "--dist-dir=/app/frontend/dist"]
