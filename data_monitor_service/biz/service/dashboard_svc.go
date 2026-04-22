package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"kol_ads_marketing/data_monitor_service/biz/dal/db"
	"kol_ads_marketing/data_monitor_service/biz/model"
	"kol_ads_marketing/data_monitor_service/biz/rpc"
	"kol_ads_marketing/data_monitor_service/pkg/utils"
	"time"
)

// CKVideoStats 对应你的表结构，用于 GORM 接收
type CKVideoStats struct {
	Mid           uint64    `gorm:"column:mid"`
	StatDate      time.Time `gorm:"column:stat_date"`
	VideoCount30d uint32    `gorm:"column:video_count_30d"`
	// 播放量指标
	View30dAvg  float32 `gorm:"column:view_30d_avg"`
	View30dMed  float32 `gorm:"column:view_30d_med"`
	View90dAvg  float32 `gorm:"column:view_90d_avg"`
	View90dMed  float32 `gorm:"column:view_90d_med"`
	View180dAvg float32 `gorm:"column:view_180d_avg"`
	View180dMed float32 `gorm:"column:view_180d_med"`
	View365dAvg float32 `gorm:"column:view_365d_avg"`
	View365dMed float32 `gorm:"column:view_365d_med"`
	// 点赞指标
	Like30dAvg  float32 `gorm:"column:like_30d_avg"`
	Like30dMed  float32 `gorm:"column:like_30d_med"`
	Like90dAvg  float32 `gorm:"column:like_90d_avg"`
	Like90dMed  float32 `gorm:"column:like_90d_med"`
	Like180dAvg float32 `gorm:"column:like_180d_avg"`
	Like180dMed float32 `gorm:"column:like_180d_med"`
	Like365dAvg float32 `gorm:"column:like_365d_avg"`
	Like365dMed float32 `gorm:"column:like_365d_med"`
	// 投币指标
	Coin30dAvg  float32 `gorm:"column:coin_30d_avg"`
	Coin30dMed  float32 `gorm:"column:coin_30d_med"`
	Coin90dAvg  float32 `gorm:"column:coin_90d_avg"`
	Coin90dMed  float32 `gorm:"column:coin_90d_med"`
	Coin180dAvg float32 `gorm:"column:coin_180d_avg"`
	Coin180dMed float32 `gorm:"column:coin_180d_med"`
	Coin365dAvg float32 `gorm:"column:coin_365d_avg"`
	Coin365dMed float32 `gorm:"column:coin_365d_med"`
	// 收藏指标
	Fav30dAvg  float32 `gorm:"column:fav_30d_avg"`
	Fav30dMed  float32 `gorm:"column:fav_30d_med"`
	Fav90dAvg  float32 `gorm:"column:fav_90d_avg"`
	Fav90dMed  float32 `gorm:"column:fav_90d_med"`
	Fav180dAvg float32 `gorm:"column:fav_180d_avg"`
	Fav180dMed float32 `gorm:"column:fav_180d_med"`
	Fav365dAvg float32 `gorm:"column:fav_365d_avg"`
	Fav365dMed float32 `gorm:"column:fav_365d_med"`
	// 分享指标
	Share30dAvg  float32 `gorm:"column:share_30d_avg"`
	Share30dMed  float32 `gorm:"column:share_30d_med"`
	Share90dAvg  float32 `gorm:"column:share_90d_avg"`
	Share90dMed  float32 `gorm:"column:share_90d_med"`
	Share180dAvg float32 `gorm:"column:share_180d_avg"`
	Share180dMed float32 `gorm:"column:share_180d_med"`
	Share365dAvg float32 `gorm:"column:share_365d_avg"`
	Share365dMed float32 `gorm:"column:share_365d_med"`
	// 回复指标
	Reply30dAvg  float32 `gorm:"column:reply_30d_avg"`
	Reply30dMed  float32 `gorm:"column:reply_30d_med"`
	Reply90dAvg  float32 `gorm:"column:reply_90d_avg"`
	Reply90dMed  float32 `gorm:"column:reply_90d_med"`
	Reply180dAvg float32 `gorm:"column:reply_180d_avg"`
	Reply180dMed float32 `gorm:"column:reply_180d_med"`
	Reply365dAvg float32 `gorm:"column:reply_365d_avg"`
	Reply365dMed float32 `gorm:"column:reply_365d_med"`
}

func (CKVideoStats) TableName() string {
	return "dws.bilibili_kol_video_stats_summary"
}

// requestGroup 用于防御缓存击穿 (Thundering Herd Problem)
var requestGroup singleflight.Group

const DashboardCacheTTL = 24 * time.Hour

// GetDashboardTrendService 新增获取折线图、时序图数据
func GetDashboardTrendService(ctx context.Context, role int, userID uint64) (*model.TrendChartDTO, error) {
	var mid uint64
	var err error

	// 角色鉴权与 RPC (与 Overview 保持一致)
	if role == 1 {
		mid, err = rpc.GetUserPlatformUid(ctx, userID, "bilibili")
		if err != nil {
			return nil, fmt.Errorf("身份校验失败: %v", err)
		}
	} else {
		return nil, fmt.Errorf("当前阶段该功能仅对 KOL 红人开放")
	}

	// 借用防击穿机制查询缓存与 DB (与前面代码类似，略去 singleflight 样板代码，直接展示核心组装逻辑)
	var stats CKVideoStats
	dbErr := db.DB.WithContext(ctx).
		Where("mid = ?", mid).
		Order("stat_date DESC").
		Limit(1).
		First(&stats).Error

	if dbErr != nil {
		return nil, fmt.Errorf("获取底层趋势数据失败: %w", dbErr)
	}

	// 3. 【核心组装】构建 ECharts 友好的数据结构
	dto := &model.TrendChartDTO{
		Mid:        mid,
		Categories: []string{"近30天", "近90天", "近180天", "近一年"},
		Series: []model.ChartSeries{
			{
				Name:   "平均播放量",
				Type:   "line",
				Smooth: true,
				Data:   []float32{stats.View30dAvg, stats.View90dAvg, stats.View180dAvg, stats.View365dAvg},
			},
			{
				Name:   "播放量中位数 (保底流量)",
				Type:   "line",
				Smooth: true,
				Data:   []float32{stats.View30dMed, stats.View90dMed, stats.View180dMed, stats.View365dMed},
			},
			{
				Name:   "平均点赞数",
				Type:   "line",
				Smooth: true,
				Data:   []float32{stats.Like30dAvg, stats.Like90dAvg, stats.Like180dAvg, stats.Like365dAvg},
			},
		},
	}

	return dto, nil
}

// GetDashboardOverviewService 处理获取大盘数据的综合业务逻辑 (编排层)
func GetDashboardOverviewService(ctx context.Context, role int, userID uint64) (*model.KOLOverviewDTO, error) {
	var platformId uint64
	var err error

	// 1. 基于角色 (RBAC) 的智能参数路由与 RPC 鉴权
	if role == 1 {
		// 【红人方】：强制通过内部 RPC 调用用户中心获取自身的 platform_id
		platformId, err = rpc.GetUserPlatformUid(ctx, userID, "bilibili")
		if err != nil {
			// 将 RPC 内部抛出的详细错误（如"未绑定B站账号"）向上抛出
			return nil, fmt.Errorf("身份校验失败: %v", err)
		}
	} else {
		// 未知角色安全拦截
		return nil, fmt.Errorf("当前阶段该功能仅对 KOL 红人开放")
	}

	// 2. 调用原有的防击穿并发查询逻辑 (从 Redis/ClickHouse 获取核心数据)
	// GetKOLOverviewWithCache 是我们上个阶段写好的方法
	data, err := GetKOLOverviewWithCache(ctx, platformId)
	if err != nil {
		return nil, fmt.Errorf("获取底层大盘数据失败: %w", err)
	}

	return data, nil
}

// 核心功能 1: 带单飞机制的缓存层查询

// GetKOLOverviewWithCache 是供 API 层调用的入口
func GetKOLOverviewWithCache(ctx context.Context, mid uint64) (*model.KOLOverviewDTO, error) {
	// 动态构造 Redis Key，这里加上日期前缀可以避免旧数据污染
	todayStr := time.Now().Format("20060102")
	redisKey := fmt.Sprintf("kol:dashboard:overview:%d:%s", mid, todayStr)
	// 1. 尝试从 Redis 缓存获取
	cachedData, err := db.RDB.Get(ctx, redisKey).Result()
	if err == nil {
		// 缓存命中 (Cache Hit)
		var dto model.KOLOverviewDTO
		_ = json.Unmarshal([]byte(cachedData), &dto)
		return &dto, nil
	} else if err != goredis.Nil {
		// Redis 发生异常，降级并打印日志，继续去查底层数据库
		hlog.CtxWarnf(ctx, "Redis 查询异常降级: %v", err)
	}
	// 2. 缓存未命中 (Cache Miss)，使用 singleflight 合并并发请求
	// 注意传入的 key 是 redisKey，确保同一时刻同一 KOL 的并发请求被拦截
	result, err, _ := requestGroup.Do(redisKey, func() (interface{}, error) {
		// 2.1 穿透到底层 ClickHouse 查询
		var stats CKVideoStats
		dbErr := db.DB.WithContext(ctx).
			Where("mid = ?", mid).
			Order("stat_date DESC").
			Limit(1).
			First(&stats).Error

		if dbErr != nil {
			return nil, fmt.Errorf("底层大盘数据查询失败: %w", dbErr)
		}
		// 2.2 数据计算与组装
		var stability, engagement, favRate, coinRate float32
		if stats.View30dAvg > 0 {
			stability = stats.View30dMed / stats.View30dAvg
			// 计算各项相对比率
			engagement = (stats.Like30dAvg + stats.Coin30dAvg + stats.Fav30dAvg + stats.Share30dAvg + stats.Reply30dAvg) / stats.View30dAvg
			favRate = stats.Fav30dAvg / stats.View30dAvg
			coinRate = stats.Coin30dAvg / stats.View30dAvg
		}
		// 2.3 【高能预警】计算雷达图归一化商业分数
		// 这里的 Benchmark (基准线) 是数据科学家根据平台大盘盘感设置的“满分线”，你可以后期做成可配置的
		radar := &model.RadarScoreDTO{
			// 传播力：绝对值对数归一化 (50万均播视为 100 分满分)
			SpreadScore: utils.LogNormalize(stats.View30dAvg, 500000.0),
			// 种草力：收藏率线性归一化 (收藏率达到 5% 视为 100 分满分，B站硬核干货极品水平)
			PlantingScore: utils.LinearNormalize(favRate, 0.05),
			// 铁粉度：投币率线性归一化 (投币率达到 3% 视为 100 分)
			LoyaltyScore: utils.LinearNormalize(coinRate, 0.03),
			// 活跃度：月更数量线性归一化 (月更 15 条视频视为 100 分)
			ActiveScore: utils.LinearNormalize(float32(stats.VideoCount30d), 15.0),
			// 互动性：综合互动率线性归一化 (互动率达到 12% 视为 100 分)
			InteractScore: utils.LinearNormalize(engagement, 0.12),
			// 稳定性：中位数/均值的比值 (达到 0.85 视为 100 分，说明极少拉跨)
			StableScore: utils.LinearNormalize(stability, 0.85),
		}

		dto := &model.KOLOverviewDTO{
			Mid:            stats.Mid,
			StatDate:       stats.StatDate.Format("2006-01-02"),
			VideoCount30d:  stats.VideoCount30d,
			AvgView30d:     stats.View30dAvg,
			MedianView30d:  stats.View30dMed,
			EngagementRate: engagement,
			StabilityRatio: stability,
			Radar:          radar,
		}

		// 2.3 异步回写 Redis (不阻塞当前请求返回)
		go func(data *model.KOLOverviewDTO) {
			bgCtx := context.Background() // 使用新的上下文，防止原请求结束被取消
			jsonData, _ := json.Marshal(data)
			// 为了防止雪崩，可以对 TTL 加一个小的随机抖动 (Jitter)
			// 这里保持极简，直接设为 24小时
			_ = db.RDB.Set(bgCtx, redisKey, jsonData, DashboardCacheTTL).Err()
		}(dto)

		return dto, nil
	})

	if err != nil {
		return nil, err
	}

	// 强转 singleflight 的 interface{} 结果并返回
	return result.(*model.KOLOverviewDTO), nil
}

// 核心功能 2: 头部 KOL 数据预热机制

// PreWarmTopKOLDashboard 供定时任务 (Cron) 每天凌晨调用
//func PreWarmTopKOLDashboard(ctx context.Context) error {
//	hlog.CtxInfof(ctx, "🔥🔥🔥 开始执行 KOL 数据大盘缓存预热...")
//	startTime := time.Now()
//
//	// 1. 从 ClickHouse 中找出最具商业价值的 Top 1000 KOL (比如按近30天均播量降序)
//	var topMids []uint64
//	err := db.DB.WithContext(ctx).
//		Table("dws.bilibili_kol_video_stats_summary").
//		Select("mid").
//		Order("view_30d_avg DESC").
//		Limit(1000).
//		Pluck("mid", &topMids).Error
//
//	if err != nil {
//		hlog.CtxErrorf(ctx, "预热失败，无法获取 Top KOL 名单: %v", err)
//		return err
//	}
//
//	hlog.CtxInfof(ctx, "获取到 %d 个头部 KOL，准备注入缓存", len(topMids))
//
//	// 2. 遍历执行缓存拉取
//	// 注意：由于 GetKOLOverviewWithCache 内部已经封装了【如果 Redis 没有就去查 CK 并回写 Redis】的逻辑，
//	// 我们在这里只需要直接调用它即可。为了防止瞬间打满 CK 连接池，我们在预热时可以加一个微小的限速休眠。
//	successCount := 0
//	for _, mid := range topMids {
//		// 为了强制覆盖刷新，如果在凌晨跑批前 Redis 还有旧数据，
//		// 这里本应该先 DEL，但因为我们在 Get 中加了日期前缀，旧数据自然就不会命中了。
//
//		_, err := GetKOLOverviewWithCache(ctx, mid)
//		if err != nil {
//			hlog.CtxWarnf(ctx, "KOL [%d] 预热失败: %v", mid, err)
//			continue
//		}
//		successCount++
//
//		// 保护底层 OLAP，每查 10 个稍微喘口气
//		if successCount%10 == 0 {
//			time.Sleep(50 * time.Millisecond)
//		}
//	}
//
//	hlog.CtxInfof(ctx, "✅ 缓存预热完成! 成功数: %d/%d, 耗时: %v", successCount, len(topMids), time.Since(startTime))
//	return nil
//}

// GetDashboardAdsAnalysisService 获取近30天商单AI分析大盘
func GetDashboardAdsAnalysisService(ctx context.Context, role int, userID uint64) (*model.AdsAnalysisDTO, error) {
	var mid uint64
	var err error
	// 1. 角色鉴权 (复用之前的内部调用)
	if role == 1 {
		mid, err = rpc.GetUserPlatformUid(ctx, userID, "bilibili")
		if err != nil {
			return nil, fmt.Errorf("身份校验失败: %v", err)
		}
	} else {
		return nil, fmt.Errorf("当前阶段该功能仅对 KOL 红人开放")
	}
	dto := &model.AdsAnalysisDTO{
		Mid:               mid,
		RecentVideos:      make([]model.AdVideoItem, 0),
		BrandDistribution: make([]model.BrandCount, 0),
		TopSellingPoints:  make([]model.PointCount, 0),
	}
	// 核心查询 1: 获取近30天商单视频列表 (按时间倒序)
	videoSql := `
		SELECT bvid, title, brand_name, product_name, selling_points, toString(toDateTime(pubdate, 'Asia/Shanghai')) as pubdate_fmt
		FROM dwd.bilibili_video_ads_analysis
		WHERE mid = ? AND pubdate >= now() - INTERVAL 30 DAY
		ORDER BY pubdate DESC
		LIMIT 20
	`
	// GORM 在处理 ClickHouse Array 时，直接扫描到 []string 中即可
	if err := db.DB.WithContext(ctx).Raw(videoSql, mid).Scan(&dto.RecentVideos).Error; err != nil {
		return nil, fmt.Errorf("查询商单视频明细失败: %v", err)
	}
	dto.TotalAds30d = len(dto.RecentVideos)
	// 如果没有商单，直接返回空面板，无需后续计算
	if dto.TotalAds30d == 0 {
		return dto, nil
	}
	// 核心查询 2: 聚合计算合作品牌分布 (Group By)
	brandSql := `
		SELECT brand_name, count(1) as count
		FROM dwd.bilibili_video_ads_analysis
		WHERE mid = ? AND pubdate >= now() - INTERVAL 30 DAY
		GROUP BY brand_name
		ORDER BY count DESC
		LIMIT 10
	`
	if err := db.DB.WithContext(ctx).Raw(brandSql, mid).Scan(&dto.BrandDistribution).Error; err != nil {
		return nil, fmt.Errorf("聚合品牌分布失败: %v", err)
	}
	// 核心查询 3: 【高阶操作】利用 ARRAY JOIN 炸裂数组，计算卖点词云
	// 解释：ARRAY JOIN 会把一条包含 ['遮瑕', '持久'] 的记录拆成两条独立记录，用于极速词频统计
	pointSql := `
		SELECT point, count(1) as count
		FROM dwd.bilibili_video_ads_analysis
		ARRAY JOIN selling_points AS point
		WHERE mid = ? AND pubdate >= now() - INTERVAL 30 DAY
		GROUP BY point
		ORDER BY count DESC
		LIMIT 15
	`
	if err := db.DB.WithContext(ctx).Raw(pointSql, mid).Scan(&dto.TopSellingPoints).Error; err != nil {
		return nil, fmt.Errorf("聚合卖点词频失败: %v", err)
	}
	return dto, nil
}
