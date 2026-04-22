package main

import (
	"context"

	"kol_ads_marketing/data_monitor_service/biz/dal/db"
	"kol_ads_marketing/data_monitor_service/biz/router"
	"kol_ads_marketing/data_monitor_service/biz/rpc"
	"kol_ads_marketing/data_monitor_service/pkg/core"
	"kol_ads_marketing/data_monitor_service/pkg/logger"
	"kol_ads_marketing/data_monitor_service/pkg/utils"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/joho/godotenv"
)

// @title KOL 营销平台 - 数据监控大脑 API
// @version 1.0
// @description Data Monitor Microservice API Docs. 负责提供 KOL 核心数据大盘与 AI 洞察分析。
// @host localhost:8083
// @BasePath /api/v1/monitor
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// 0. 加载本地 .env 文件（如果在 Docker/K8s 生产环境中没找到 .env 也不影响，会自动读系统变量）
	_ = godotenv.Load(".env")

	// 1. 日志初始化
	logger.InitLogger()

	// 2. 数据底层初始化 (ClickHouse, Redis)
	db.InitClickHouseClient()
	db.InitRedis()
	// 2.5 初始化内部通信RPC服务
	rpc.Init()

	// 3. 构建 Hertz 微服务引擎 (非常关键的一步，之前的代码漏掉了)
	serverPort := utils.GetEnv("MONITOR_SERVICE_PORT", "8083") // 从环境变量动态读取端口
	h := server.Default(server.WithHostPorts("0.0.0.0:" + serverPort))

	// 4. Nacos 服务注册
	// InitNacos 内部会去读配置并把当前服务注册上去，同时返回一个注销函数
	deregisterSvc := core.InitNacos()

	// 5. 优雅关机 (Graceful Shutdown) Hook
	// 将 Nacos 的注销函数挂载到 Hertz 的关闭事件上。
	// 极客提示：这步极为重要！如果不做，重启服务时 Nacos 上会残留“幽灵节点”，导致网关负载均衡路由到死节点上报错。
	h.OnShutdown = append(h.OnShutdown, func(ctx context.Context) {
		deregisterSvc()
		// 此处后续还可以追加 db.CloseClickHouse() 等底层资源的释放逻辑
	})

	// 6. 注册业务路由 (下一步的里程碑)
	router.RegisterRoutes(h)

	// 7. 启动服务
	hlog.Infof("🚀 Data Monitor 微服务启动成功，监听端口: %s", serverPort)
	h.Spin() // 阻塞主协程，开始监听处理 HTTP 请求
}
