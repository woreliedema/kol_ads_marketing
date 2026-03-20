package logger

import (
	"os"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// LogConfig 日志配置边界结构体
type LogConfig struct {
	Level    string // 日志级别：debug, info, warn, error, fatal
	Format   string // 输出格式：console, json
	FilePath string // 日志文件路径 (如果为空则只输出到终端)
}

// InitLogger 全局日志初始化函数
func InitLogger(cfg *LogConfig) {
	// 1. 设置日志级别
	var level hlog.Level
	switch cfg.Level {
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

	// 2. 动态设置日志格式
	// (在未来需要 JSON 格式对接到 ELK 时，可以引入 hertz-zap)

	// 3. 配置日志输出目标
	if cfg.FilePath != "" {
		f, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			hlog.Fatalf("无法打开日志文件: %v", err)
		}
		hlog.SetOutput(f)
	} else {
		hlog.SetOutput(os.Stdout)
	}

	hlog.Infof("✅ Logger 初始化完成 [Level: %s, Format: %s]", cfg.Level, cfg.Format)
}
