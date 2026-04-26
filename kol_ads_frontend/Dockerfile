# 第一阶段：构建阶段 (Builder)
FROM node:24-alpine AS builder

# 设置容器内的工作目录
WORKDIR /app

# 优先拷贝依赖清单，利用 Docker 缓存加速后续构建
COPY package.json package-lock.json ./

# 安装依赖 (推荐使用 npm ci，它会严格按照 package-lock.json 安装，速度更快且版本锁定)
RUN npm ci

# 拷贝前端项目的所有源码 (注意：务必确保有 .dockerignore 忽略掉本地的 node_modules)
COPY . .

# 执行 Vite 生产环境打包，产物将生成在 /app/dist 目录
RUN npm run build

# 第二阶段：运行阶段 (Runner)
# 使用极轻量的 Nginx 来托管纯静态文件
FROM nginx:alpine

# 设置时区为上海，保持全栈环境统一
RUN apk --no-cache add tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

# 清空 Nginx 的默认欢迎页面
RUN rm -rf /usr/share/nginx/html/*

# 从 builder 阶段提取打包好的 dist 目录文件
COPY --from=builder /app/dist /usr/share/nginx/html

# 🔴 核心配置：将你上一轮创建的 nginx.conf 复制进去，解决 React 路由刷新 404 问题
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 暴露容器的 80 端口
EXPOSE 80

# 以前台模式启动 Nginx
CMD ["nginx", "-g", "daemon off;"]