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

	// 2. 动态设置日志格式 (生产环境必须用 JSON 格式，方便日志系统分析)
	// 极客提示：在实际工程中，JSON 日志可以完美适配 Kibana 或 Grafana 的日志切分规则。
	/* 注意：为了保持代码极简，这里我们依赖 hlog 默认输出。
	   如果 format="json"，可配合 Hertz 的 zap/logrus 扩展模块替换底层日志核心，
	   由于 Hertz hlog 的原生接口极简，后续可根据需要平滑挂载 hertz-zap。
	*/

	// 3. 配置日志输出目标
	if cfg.FilePath != "" {
		// 这里暂以简单的文件追加为例。
		// 在真正的生产大流量环境中，通常会结合 "gopkg.in/natefinch/lumberjack.v2" 实现日志自动按天/按大小切割 (Log Rotation)
		f, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			hlog.Fatalf("无法打开日志文件: %v", err)
		}
		hlog.SetOutput(f)
	} else {
		// 开发环境下输出到标准输出
		hlog.SetOutput(os.Stdout)
	}

	hlog.Infof("✅ Logger 初始化完成 [Level: %s, Format: %s]", cfg.Level, cfg.Format)
}

/* 使用示例 (在 Controller 或 Service 中)：
hlog.CtxInfof(ctx, "用户[%s]正在登录", req.Username)
hlog.CtxErrorf(ctx, "数据库查询失败: %v", err)
*/
