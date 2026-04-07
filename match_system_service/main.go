package main

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/joho/godotenv"
	"kol_ads_marketing/match_system_service/biz/handlers/im"
	"kol_ads_marketing/match_system_service/biz/router"
	"kol_ads_marketing/match_system_service/dal/cache"
	"kol_ads_marketing/match_system_service/dal/db"
	"kol_ads_marketing/match_system_service/dal/es"
	"kol_ads_marketing/match_system_service/pkg/mq"
	"kol_ads_marketing/match_system_service/pkg/nacos"
	"kol_ads_marketing/match_system_service/pkg/utils"
	"kol_ads_marketing/match_system_service/pkg/utils/logger"
	user_rpc "kol_ads_marketing/match_system_service/rpc/user_center"
	"kol_ads_marketing/match_system_service/service/im_service"
	"kol_ads_marketing/match_system_service/service/scheduler"
	"log"
	"os"
	"strconv"
	"strings"
)

// @title 匹配系统微服务 API (Match System Service)
// @version 1.0
// @description 匹配系统接口文档，提供红人筛选、需求解析、受众重合度匹配等核心能力。
// @host localhost:8082
// @BasePath /
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// 1. 优先加载 .env
	if err := godotenv.Load(".env"); err != nil {
		hlog.Warnf("未找到 .env 文件或读取失败，将使用系统环境变量")
	}
	// 辅助函数：快速获取带默认值的字符串环境变量
	getEnv := func(key, fallback string) string {
		if value, exists := os.LookupEnv(key); exists {
			return value
		}
		return fallback
	}

	logger.InitLogger(&logger.LogConfig{
		Level:    getEnv("LOG_LEVEL", "debug"),
		Format:   getEnv("LOG_FORMAT", "console"),
		FilePath: "",
	})

	workerIDStr := os.Getenv("SNOWFLAKE_WORKER_ID")
	workerID := int64(1)
	if workerIDStr != "" {
		if id, err := strconv.ParseInt(workerIDStr, 10, 64); err == nil {
			workerID = id
		}
	}

	if err := utils.InitSnowflake(workerID); err != nil {
		log.Fatalf("核心组件 Snowflake 初始化失败: %v", err)
	}

	// 2. 执行 DAL 初始化
	db.Init()
	es.Init()
	cache.Init()
	user_rpc.Init()

	imPersistSvc := im_service.NewIMPersistenceService(db.DB)
	// 4. 【依赖注入】将 Service 注入到 Handler 层
	im.InitIMHTTPHandler(imPersistSvc)

	kafkaBrokersStr := getEnv("KAFKA_BOOTSTRAP_SERVERS", "127.0.0.1:9092")
	kafkaTopic := getEnv("KAFKA_MS_IM_TOPIC", "im_chat_messages")
	kafkaPersistGroupID := getEnv("KAFKA_MS_IM_PERSIST_GROUP", "im_persistence")
	// 支持多个 Broker 用逗号分隔 (如 127.0.0.1:9092,127.0.0.1:9093)
	kafkaBrokers := strings.Split(kafkaBrokersStr, ",")
	// 2. 初始化 Kafka 生产者
	mq.InitKafkaProducer(kafkaBrokers, kafkaTopic)
	defer mq.CloseProducer()
	// 3. 启动 Kafka 消费者 (异步挂载)
	mq.StartKafkaConsumer(kafkaBrokers, kafkaTopic)
	defer mq.CloseConsumer()
	mq.StartIMMessageConsumer(context.Background(), kafkaBrokers, kafkaTopic, kafkaPersistGroupID, imPersistSvc)

	// 4. 启动定时任务调度器 (Cron)
	cronScheduler := scheduler.InitScheduler(db.DB, cache.RDB)
	defer cronScheduler.Stop()

	// 5. 微服务引擎构建
	serverPort := getEnv("MATCH_SYSTEM_PORT", "8082")
	h := server.Default(
		server.WithHostPorts("0.0.0.0:"+serverPort),
		server.WithMaxRequestBodySize(20*1024*1024),
	)

	// 6. Nacos 注册发现逻辑
	nacosPort, _ := strconv.ParseUint(getEnv("NACOS_PORT", "8848"), 10, 64)
	svcPort, _ := strconv.ParseUint(serverPort, 10, 64)
	nacosConfig := &nacos.NacosConfig{
		Host:        getEnv("NACOS_HOST", "127.0.0.1"),
		Port:        nacosPort,
		NamespaceId: getEnv("NACOS_NAMESPACE", "public"),
		// 这里改为匹配服务的默认名称
		ServiceName: getEnv("MATCH_SYSTEM_NAME", "match-system-service"),
		Ip:          getEnv("MATCH_SYSTEM_PORT", "127.0.0.1"),
		ServicePort: svcPort,
		Weight:      1.0,
		GroupName:   getEnv("NACOS_GROUP", "DEFAULT_GROUP"),
		ClusterName: getEnv("NACOS_CLUSTER", "DEFAULT"),
	}

	deregisterSvc, err := nacos.InitNacos(nacosConfig)
	if err != nil {
		hlog.Fatalf("❌ Nacos 初始化失败: %v", err)
	}

	// 绑定 Hertz 关闭钩子，确保优雅注销
	h.OnShutdown = append(h.OnShutdown, func(ctx context.Context) {
		deregisterSvc()
	})

	// 4. 路由注册
	router.RegisterRoutes(h)

	// 5. 启动服务
	hlog.Infof("🚀 Match System 微服务正在启动，监听端口: %s...", serverPort)
	h.Spin()
}
