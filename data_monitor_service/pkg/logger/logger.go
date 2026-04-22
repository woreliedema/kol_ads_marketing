package logger

import (
	"os"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"kol_ads_marketing/data_monitor_service/pkg/utils"
)

// InitLogger 无参全局日志初始化函数
func InitLogger() {
	levelStr := utils.GetEnv("LOG_LEVEL", "debug")
	formatStr := utils.GetEnv("LOG_FORMAT", "console")
	filePath := utils.GetEnv("LOG_FILE_PATH", "") // 如果为空则只输出到终端

	// 1. 设置日志级别
	var level hlog.Level
	switch levelStr {
	case "debug":
		level = hlog.LevelDebug
	case "info":
		level = hlog.LevelInfo
	case "warn":
		level = hlog.LevelWarn
	case "error":
		level = hlog.LevelError
	case "fatal":
		level = hlog.LevelFatal
	default:
		level = hlog.LevelInfo
	}
	hlog.SetLevel(level)

	// 2. 配置日志输出目标
	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			hlog.Fatalf("❌ 无法打开日志文件: %v", err)
		}
		hlog.SetOutput(f)
	} else {
		// 开发环境下输出到标准输出
		hlog.SetOutput(os.Stdout)
	}

	hlog.Infof("✅ Logger 初始化完成 [Level: %s, Format: %s]", levelStr, formatStr)
}
