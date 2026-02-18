# 1. 使用 Python 3.10 作为基础镜像
FROM python:3.10-slim

# 2. 设置容器内的工作目录
WORKDIR /app

# 3. 设置环境变量
# 防止 Python 生成 .pyc 文件
ENV PYTHONDONTWRITEBYTECODE=1
# 让 Python 日志直接输出到终端，方便 Docker 查看
ENV PYTHONUNBUFFERED=1
# 将 /app 加入 PYTHONPATH，确保能正确导入 data_collection_service 模块
ENV PYTHONPATH=/app

# 4. 复制依赖文件并安装
COPY requirements.txt .


# 5. 复制整个项目代码到容器中
COPY . .

# 6. 暴露端口
EXPOSE 8000

# 7. 启动命令
CMD ["python", "-m", "uvicorn", "data_collection_service.app.main:app", "--host", "0.0.0.0", "--port", "8000"]