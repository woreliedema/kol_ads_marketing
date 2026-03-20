package cache

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func Init() {
	// 1. 获取 Redis 配置
	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")
	password := os.Getenv("REDIS_PASSWORD")
	dbStr := os.Getenv("REDIS_DB")

	if host == "" || port == "" {
		log.Fatal("致命错误: Redis 环境变量缺失 (REDIS_HOST / REDIS_PORT)")
	}

	dbIndex, _ := strconv.Atoi(dbStr)
	addr := fmt.Sprintf("%s:%s", host, port)

	// 2. 建立连接池
	RDB = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       dbIndex,
		PoolSize: 100, // 最大连接池大小
	})

	// 3. 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RDB.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Redis 连接失败: %v", err)
	}

	log.Println("✅ Redis 客户端初始化成功")
}
