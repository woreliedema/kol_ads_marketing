package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"kol_ads_marketing/user_center/app/api"
	//"kol_ads_marketing/user_center/app/api/handlers"
	//"kol_ads_marketing/user_center/app/api/middleware"
	//"kol_ads_marketing/user_center/app/api/response"
	"kol_ads_marketing/user_center/app/core"
	"kol_ads_marketing/user_center/app/db"
	"kol_ads_marketing/user_center/app/utils/logger"
	_ "kol_ads_marketing/user_center/docs"

	//"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	//"github.com/hertz-contrib/swagger"
	"github.com/joho/godotenv"
	//swaggerFiles "github.com/swaggo/files"
)

// @title KOL 营销平台用户中心 API
// @version 1.0
// @description User Center Microservice API Docs.
// @host localhost:8081
// @BasePath /
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
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
	api.RegisterRoutes(h)

	// 5. 启动服务
	hlog.Infof("🚀 User Center 微服务正在启动，监听端口: %s...", serverPort)
	h.Spin()
}
