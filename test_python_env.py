import sys
import os

print("Python版本:", sys.version)
print("Python可执行文件路径:", sys.executable)
print("当前工作目录:", os.getcwd())
print("系统平台:", sys.platform)

# 测试WSL环境特征
try:
    import subprocess
    result = subprocess.run(["wsl", "-d", "Ubuntu-22.04", "--", "echo", "WSL环境测试成功"], 
                           capture_output=True, text=True)
    print("WSL测试输出:", result.stdout.strip())
except Exception as e:
    print("WSL测试失败:", e)
