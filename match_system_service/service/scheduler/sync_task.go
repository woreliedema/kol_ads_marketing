package scheduler

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"kol_ads_marketing/match_system_service/dal/db"
	"kol_ads_marketing/match_system_service/pkg/constants"
)

// InitScheduler 初始化并启动所有后台定时任务
// 建议在 main.go 中调用此方法，并传入已初始化的 mysql 和 redis 客户端
func InitScheduler(mysqlClient *gorm.DB, redisClient *redis.Client) *cron.Cron {
	// 开启秒级支持 (默认是分级，MVP 阶段为了方便本地调试，开启秒级)
	c := cron.New(cron.WithSeconds())

	// 1. 注册品牌方宽表同步任务 (例如：每 1 分钟执行一次)
	_, err := c.AddFunc("0 */1 * * * *", func() {
		runBrandSyncJob(mysqlClient, redisClient)
	})
	if err != nil {
		hlog.Fatalf("注册品牌同步任务失败: %v", err)
	}

	// 2. 注册红人宽表同步任务 (例如：每 1 分钟执行一次)
	_, err = c.AddFunc("0 */1 * * * *", func() {
		runKOLSyncJob(mysqlClient, redisClient)
	})
	if err != nil {
		hlog.Fatalf("注册红人同步任务失败: %v", err)
	}

	// 3. 注册 KOL 2 ES 批量推送 Worker(每分钟1次)
	_, err = c.AddFunc("0 */1 * * * *", func() {
		RunKOLBulkSyncToES(mysqlClient)
	})
	if err != nil {
		hlog.Fatalf("注册 KOL 2 ES 同步任务失败: %v", err)
	}
	// 4. 注册 Brand 2 ES 批量推送 Worker(每分钟1次)
	_, err = c.AddFunc("0 */1 * * * *", func() {
		RunBrandBulkSyncToES(mysqlClient)
	})

	if err != nil {
		hlog.Fatalf("注册 Brand 2 ES 同步任务失败: %v", err)
	}

	// 启动调度器 (非阻塞，会在后台启动 Goroutine 运行)
	c.Start()
	hlog.Info("后台定时调度器启动成功...")

	return c
}

// runBrandSyncJob 品牌方同步核心逻辑
func runBrandSyncJob(mysqlClient *gorm.DB, redisClient *redis.Client) {
	ctx := context.Background()
	hlog.Info("开始执行 [品牌方] 宽表增量同步任务...")

	// 1. 从 Redis 读取上次同步时间
	val, err := redisClient.Get(ctx, constants.RedisKeySyncBrandWide).Result()
	var lastSyncTime time.Time

	if err == redis.Nil || val == "" {
		// Redis 中没有记录，说明是首次运行，从 1970 年开始拉取全量数据
		lastSyncTime = time.Unix(0, 0)
	} else if err != nil {
		hlog.Errorf("读取 Redis 品牌同步时间失败: %v", err)
		return
	} else {
		// 解析 RFC3339 格式的时间字符串
		lastSyncTime, err = time.Parse(time.RFC3339, val)
		if err != nil {
			hlog.Errorf("解析品牌同步时间格式失败: %v", err)
			lastSyncTime = time.Unix(0, 0)
		}
	}

	// 2. 记录当前时间作为下一轮的锚点 (在执行 SQL 前获取当前时间，防止 SQL 执行耗时导致漏数据)
	currentSyncTime := time.Now()

	// 3. 调用 DAL 层执行原生 SQL
	err = db.SyncBrandToWideIndex(mysqlClient, lastSyncTime)
	if err != nil {
		hlog.Errorf("品牌宽表同步 SQL 执行失败: %v", err)
		return
	}

	// 4. 同步成功，更新 Redis
	err = redisClient.Set(ctx, constants.RedisKeySyncBrandWide, currentSyncTime.Format(time.RFC3339), 0).Err()
	if err != nil {
		hlog.Errorf("更新 Redis 品牌同步时间失败: %v", err)
	} else {
		hlog.Infof("[品牌方] 宽表同步完成，最后同步时间已更新为: %v", currentSyncTime.Format(time.RFC3339))
	}
}

// runKOLSyncJob 红人同步核心逻辑
func runKOLSyncJob(mysqlClient *gorm.DB, redisClient *redis.Client) {
	ctx := context.Background()
	hlog.Info("开始执行 [红人] 宽表增量同步任务...")

	// 1. 获取上次同步时间
	val, err := redisClient.Get(ctx, constants.RedisKeySyncKOLWide).Result()
	var lastSyncTime time.Time

	if err == redis.Nil || val == "" {
		lastSyncTime = time.Unix(0, 0)
	} else if err == nil {
		lastSyncTime, _ = time.Parse(time.RFC3339, val)
	} else {
		hlog.Errorf("读取 Redis 红人同步时间失败: %v", err)
		return
	}

	currentSyncTime := time.Now()

	// 2. 执行 DAL 层的聚合 SQL
	err = db.SyncKOLToWideIndex(mysqlClient, lastSyncTime)
	if err != nil {
		hlog.Errorf("红人宽表同步 SQL 执行失败: %v", err)
		return
	}

	// 3. 成功后更新 Redis
	redisClient.Set(ctx, constants.RedisKeySyncKOLWide, currentSyncTime.Format(time.RFC3339), 0)
	hlog.Infof("[红人] 宽表同步完成，最后同步时间已更新为: %v", currentSyncTime.Format(time.RFC3339))
}
