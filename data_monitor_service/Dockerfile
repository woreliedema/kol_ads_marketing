# 第一阶段：构建阶段 (Builder)
# 使用与你 WSL 环境一致的 Go 1.25.0 基础镜像
FROM golang:1.25-alpine AS builder

# 设置工作目录
WORKDIR /app

# 配置 Go 环境变量 (开启 Go Modules，配置国内镜像源加速下载)
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

# 先拷贝依赖声明文件，利用 Docker 缓存机制，只要依赖没变就不会重新下载
COPY go.mod go.sum ./
RUN go mod download

# 拷贝项目所有代码
COPY . .

# 🚀 核心编译指令：
# CGO_ENABLED=0: 禁用 CGO，确保生成的静态二进制文件能在 Alpine 这种轻量镜像中直接运行
# -ldflags="-s -w": 剥离调试信息，极大程度压缩二进制包体积
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o data_monitor_api ./main.go

# 第二阶段：运行阶段 (Runner)
# 使用极简的 alpine 镜像，最终镜像大小通常在 20MB 左右
FROM alpine:latest

# 🔴 关键步骤：设置时区和安装根证书
# 数据监控服务涉及大量 ClickHouse 时序数据查询，时区的一致性直接决定了图表展示的准确性
RUN apk --no-cache add ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

# 设置工作目录
WORKDIR /app

# 从构建阶段把编译好的二进制文件拿过来
COPY --from=builder /app/data_monitor_api .

# 暴露端口 (对应代码中的默认端口 8083)
EXPOSE 8083

# 启动微服务
CMD ["./data_monitor_api"]