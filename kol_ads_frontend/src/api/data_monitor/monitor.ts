import api from '../request';

// --- 数据契约定义 ---
export interface RadarScore {
  spread_score: number;
  planting_score: number;
  loyalty_score: number;
  active_score: number;
  interact_score: number;
  stable_score: number;
}

export interface KOLOverview {
  mid: number;
  stat_date: string;
  video_count_30d: number;
  avg_view_30d: number;
  median_view_30d: number;
  engagement_rate: number;
  stability_ratio: number;
  radar: RadarScore;
}

export interface ChartSeries {
  name: string;
  type: string;
  smooth: boolean;
  data: number[];
}

export interface TrendChart {
  mid: number;
  categories: string[];
  series: ChartSeries[];
}

// --- 新增：商单分析数据契约 ---
export interface BrandCount {
  brand_name: string;
  count: number;
}

export interface PointCount {
  point: string;
  count: number;
}

export interface AdVideoItem {
  bvid: string;
  title: string;
  brand_name: string;
  product_name: string;
  selling_points: string[];
  pubdate: string;
}

export interface AdsAnalysisDTO {
  mid: number;
  total_ads_30d: number;
  brand_distribution: BrandCount[];
  top_selling_points: PointCount[];
  recent_videos: AdVideoItem[];
}

// --- API 调用 ---
export const getDashboardOverviewApi = () => {
  return api.get('/monitor/dashboard/overview');
};

export const getDashboardTrendApi = () => {
  return api.get('/monitor/dashboard/trend');
};

// 获取商单分析大盘
export const getDashboardAdsAnalysisApi = () => {
  return api.get('/monitor/dashboard/ads_analysis');
};