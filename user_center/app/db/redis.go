package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig Redis 配置边界结构体
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int // Redis 数据库编号，默认为 0
}

// RDB 全局 Redis 客户端实例
var RDB *redis.Client

// InitRedis 初始化 Redis 连接池
func InitRedis(cfg *RedisConfig) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// 初始化客户端配置
	RDB = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     100, // 最大连接池大小，应对高并发 Token 校验
		MinIdleConns: 10,  // 最小空闲连接数
	})

	// 防御机制：启动时发送 Ping 测试连通性 (Fail-Fast)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RDB.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("Redis Ping 失败: %w", err)
	}

	return nil
}
