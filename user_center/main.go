package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"kol_ads_marketing/user_center/app/api/handlers"
	"kol_ads_marketing/user_center/app/api/middleware"
	"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/core"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/utils/logger"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/joho/godotenv"
)

func main() {
	// 0. 加载环境变量 (Load Environment Variables)
	// 注意：因为我们在项目根目录运行，所以这里的路径直接写 ".env" 即可。
	// 如果在 Docker 中运行，我们通常通过 K8s ConfigMap 注入环境，这时候找不到 .env 也不应报错断开。
	if err := godotenv.Load(".env"); err != nil {
		hlog.Warnf("未找到本地 .env 文件，将尝试使用系统环境变量...")
	}

	// 辅助函数：快速获取带默认值的字符串环境变量
	getEnv := func(key, fallback string) string {
		if value, exists := os.LookupEnv(key); exists {
			return value
		}
		return fallback
	}

	// 辅助函数：快速获取带默认值的整型环境变量
	getEnvInt := func(key string, fallback int) int {
		if value, exists := os.LookupEnv(key); exists {
			if intVal, err := strconv.Atoi(value); err == nil {
				return intVal
			}
		}
		return fallback
	}

	// 1. 基础设施初始化
	// 1.1 初始化日志
	logger.InitLogger(&logger.LogConfig{
		Level:    getEnv("LOG_LEVEL", "debug"),
		Format:   getEnv("LOG_FORMAT", "console"),
		FilePath: "",
	})

	// 1.2 初始化数据库
	dbConfig := &db.MySQLConfig{
		Host:         getEnv("MYSQL_HOST", "127.0.0.1"),
		Port:         getEnvInt("MYSQL_PORT", 3306),
		User:         getEnv("MYSQL_USER", "root"),
		Password:     getEnv("MYSQL_PASSWORD", ""), // 密码必须从环境读取
		DBName:       getEnv("MYSQL_DBNAME", "kol_user_center"),
		Charset:      "utf8mb4",
		MaxIdleConns: 10,
		MaxOpenConns: 100,
		MaxLifetime:  time.Hour,
		Debug:        getEnv("LOG_LEVEL", "debug") == "debug", // 如果是 debug 模式，自动开启 SQL 打印
	}

	if err := db.InitMySQL(dbConfig); err != nil {
		hlog.Fatalf("❌ 数据库初始化失败，微服务拒绝启动: %v", err)
	}

	// 1.3 初始化 Redis 连接池
	redisConfig := &db.RedisConfig{
		Host:     getEnv("REDIS_HOST", "127.0.0.1"),
		Port:     getEnvInt("REDIS_PORT", 6379),
		Password: getEnv("REDIS_PASSWORD", ""), // 默认无密码
		DB:       getEnvInt("REDIS_DB", 0),
	}

	if err := db.InitRedis(redisConfig); err != nil {
		hlog.Fatalf("❌ Redis 初始化失败，微服务拒绝启动: %v", err)
	}
	hlog.Infof("✅ Redis 底层连接池初始化成功！[Addr: %s:%d]", redisConfig.Host, redisConfig.Port)

	// 2. 微服务引擎构建
	serverPort := getEnv("USER_CENTER_PORT", "8081")
	h := server.Default(
		server.WithHostPorts("0.0.0.0:"+serverPort),
		server.WithMaxRequestBodySize(20*1024*1024),
	)

	//  3. 接入 Nacos 服务注册中心
	// 将字符串端口转为 uint64
	nacosPort, _ := strconv.ParseUint(getEnv("NACOS_PORT", "8848"), 10, 64)
	svcPort, _ := strconv.ParseUint(serverPort, 10, 64)

	nacosConfig := &core.NacosConfig{
		Host:        getEnv("NACOS_HOST", "127.0.0.1"),
		Port:        nacosPort,
		NamespaceId: getEnv("NACOS_NAMESPACE", "public"),
		ServiceName: getEnv("SERVICE_NAME", "user-center-service"),
		Ip:          getEnv("SERVICE_IP", "127.0.0.1"),
		ServicePort: svcPort,
		Weight:      1.0,
		GroupName:   getEnv("NACOS_GROUP", "DEFAULT_GROUP"),
		ClusterName: getEnv("NACOS_CLUSTER", "DEFAULT"),
	}

	// 启动 Nacos 注册，并获取注销函数
	deregisterSvc, err := core.InitNacos(nacosConfig)
	if err != nil {
		hlog.Fatalf("❌ Nacos 初始化失败: %v", err)
	}

	// 利用 Hertz 的钩子函数 (Hook)，在微服务被关闭 (Ctrl+C / 容器停止) 时，自动注销 Nacos 实例
	h.OnShutdown = append(h.OnShutdown, func(ctx context.Context) {
		deregisterSvc()
	})

	// 4. 路由注册
	h.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		hlog.CtxInfof(c, "收到来自 %s 的 Ping 请求", ctx.ClientIP())
		response.Success(ctx, map[string]interface{}{
			"service":   "kol_user_center",
			"status":    "running",
			"timestamp": time.Now().Unix(),
		})
	})

	// 5. 认证相关路由组
	authGroup := h.Group("/api/v1/auth")
	{
		authGroup.POST("/register", handlers.Register)
		authGroup.POST("/login", handlers.Login)
	}

	protectedGroup := h.Group("/api/v1/user", middleware.AuthMiddleware())
	{
		// 测试接口：获取当前登录用户的简要信息
		protectedGroup.GET("/me", func(c context.Context, ctx *app.RequestContext) {
			// 直接从上下文中取出中间件塞进来的 user_id 和 role
			userID, _ := ctx.Get("user_id")
			role, _ := ctx.Get("role")

			response.Success(ctx, map[string]interface{}{
				"message": "恭喜！你成功通过了防线！",
				"user_id": userID,
				"role":    role,
			})
		})
	}

	// 6. 启动服务
	hlog.Infof("🚀 User Center 微服务正在启动，监听端口: %s...", serverPort)
	h.Spin()
}
