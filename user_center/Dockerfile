# 第一阶段：构建阶段 (Builder)
# 使用带有完整 Go 环境的基础镜像来编译代码
FROM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /app

# 配置 Go 环境变量 (开启 Go Modules，配置国内镜像源加速下载)
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

# 先拷贝 go.mod 和 go.sum，利用 Docker 缓存机制，只要这俩文件没变，就不会重新下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 拷贝项目所有代码
COPY . .

# 🚀 核心编译指令：
# CGO_ENABLED=0: 禁用 CGO，确保编译出纯静态二进制文件，使其能在一个没有 C 标准库的极简环境里运行
# GOOS=linux: 指定编译目标系统为 Linux
# -ldflags="-s -w": 剥离调试信息，进一步极致压缩包体积
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o user_center_api ./main.go

# 第二阶段：运行阶段 (Runner)
# 使用极简的 alpine 镜像作为运行环境，最终镜像大小通常只有 10MB~20MB
FROM alpine:latest

# 设置时区和安装根证书 (发起 HTTPS 请求时必须要有证书)
RUN apk --no-cache add ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

# 设置工作目录
WORKDIR /app

# 🚀 从第一阶段 (builder) 中把编译好的二进制文件拿过来
COPY --from=builder /app/user_center_api .

# 暴露端口 (与你在 .env 里配置的端口保持一致)
EXPOSE 8081

# 启动微服务
CMD ["./user_center_api"]