package model

// KOLOverviewDTO 前端翻牌器与基础数据模型
type KOLOverviewDTO struct {
	Mid            uint64         `json:"mid"`
	StatDate       string         `json:"stat_date"`
	VideoCount30d  uint32         `json:"video_count_30d"`
	AvgView30d     float32        `json:"avg_view_30d"`
	MedianView30d  float32        `json:"median_view_30d"`
	EngagementRate float32        `json:"engagement_rate"` // 衍生计算指标
	StabilityRatio float32        `json:"stability_ratio"` // 衍生计算指标
	Radar          *RadarScoreDTO `json:"radar"`           // 雷达图数据挂载点
}

// RadarScoreDTO 雷达图归一化分数 (0-100)
type RadarScoreDTO struct {
	SpreadScore   float32 `json:"spread_score"`   // 传播力(基于均播)
	PlantingScore float32 `json:"planting_score"` // 种草力(基于收藏率)
	LoyaltyScore  float32 `json:"loyalty_score"`  // 铁粉度(基于投币率)
	ActiveScore   float32 `json:"active_score"`   // 活跃度(基于发稿频率)
	InteractScore float32 `json:"interact_score"` // 互动性(基于整体互动率)
	StableScore   float32 `json:"stable_score"`   // 稳定性(基于中位数/均值的比值)
}

// ChartSeries 适配 ECharts 的多条折线数据列
type ChartSeries struct {
	Name   string    `json:"name"`   // 图例名称 (如: "平均播放量")
	Type   string    `json:"type"`   // 图表类型 (如: "line", "bar")
	Smooth bool      `json:"smooth"` // 是否平滑曲线
	Data   []float32 `json:"data"`   // 对应 30/90/180/365 的数据数组
}

// TrendChartDTO 给前端的最终折线图大闭环结构
type TrendChartDTO struct {
	Mid        uint64        `json:"mid"`
	Categories []string      `json:"categories"` // X轴坐标: ["近30天", "近90天", "近180天", "近一年"]
	Series     []ChartSeries `json:"series"`     // Y轴数据列
}

// AdVideoItem 近期商单视频明细 (用于前端渲染数据表格)
type AdVideoItem struct {
	Bvid          string   `json:"bvid" gorm:"column:bvid"`
	Title         string   `json:"title" gorm:"column:title"`
	BrandName     string   `json:"brand_name" gorm:"column:brand_name"`
	ProductName   string   `json:"product_name" gorm:"column:product_name"`
	SellingPoints []string `json:"selling_points" gorm:"column:selling_points;type:Array(String)"` // 自动解析 ClickHouse 的 Array(String)
	PubdateTime   string   `json:"pubdate" gorm:"column:pubdate_fmt"`
}

// BrandCount 合作品牌分布 (用于渲染饼图)
type BrandCount struct {
	BrandName string `json:"brand_name"`
	Count     int    `json:"count"`
}

// PointCount 卖点词频统计 (用于渲染词云 Word Cloud)
type PointCount struct {
	Point string `json:"point"`
	Count int    `json:"count"`
}

// AdsAnalysisDTO 商单分析大盘终态返回体
type AdsAnalysisDTO struct {
	Mid               uint64        `json:"mid"`
	TotalAds30d       int           `json:"total_ads_30d"`      // 近30天商单总数
	BrandDistribution []BrandCount  `json:"brand_distribution"` // 品牌偏好
	TopSellingPoints  []PointCount  `json:"top_selling_points"` // 核心带货卖点
	RecentVideos      []AdVideoItem `json:"recent_videos"`      // 视频列表
}
