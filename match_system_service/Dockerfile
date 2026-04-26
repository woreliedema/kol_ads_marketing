# 第一阶段：构建阶段 (Builder)
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 配置 Go 环境变量加速下载
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

# 利用缓存机制下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制代码并进行纯静态编译
COPY . .
# 禁用 CGO，构建 Linux 静态二进制文件，并剥离调试信息压缩体积
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o match_system_api ./main.go

# 第二阶段：运行阶段 (Runner)
FROM alpine:latest

# 🔴 核心依赖安装：
# tzdata 对于匹配系统至关重要，因为你的 main.go 里有 cronScheduler
# 如果没有时区数据，Cron 的定时调度（比如每天凌晨 2 点同步 ES 数据）将会完全错乱
RUN apk --no-cache add ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 将编译好的二进制文件拷贝过来
COPY --from=builder /app/match_system_api .

# 暴露匹配服务的 8082 端口
EXPOSE 8082

# 启动微服务
CMD ["./match_system_api"]