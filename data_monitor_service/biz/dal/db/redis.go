package db

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/redis/go-redis/v9"
	"kol_ads_marketing/data_monitor_service/pkg/utils"
)

// RDB 全局 Redis 客户端实例
var RDB *redis.Client

// InitRedis 无参初始化，配置内聚
func InitRedis() {
	// 内部读取环境变量
	host := utils.GetEnv("REDIS_HOST", "127.0.0.1")
	port := utils.GetEnvInt("REDIS_PORT", 6379)
	password := utils.GetEnv("REDIS_PASSWORD", "")
	dbNum := utils.GetEnvInt("REDIS_DB", 0)

	addr := fmt.Sprintf("%s:%d", host, port)

	// 初始化客户端配置
	RDB = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           dbNum,
		PoolSize:     100, // 最大连接池大小，应对高并发 Token 校验
		MinIdleConns: 10,  // 最小空闲连接数
	})

	// 防御机制：启动时发送 Ping 测试连通性 (Fail-Fast)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RDB.Ping(ctx).Result()
	if err != nil {
		// 启动期发生致命错误，直接宕机，拒绝提供不健康的服务
		hlog.Fatalf("❌ Redis Ping 失败，拒绝启动微服务: %v", err)
	}

	hlog.Infof("✅ Redis 连接池初始化成功！[Addr: %s]", addr)
}
