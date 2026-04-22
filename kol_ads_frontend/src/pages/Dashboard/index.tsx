// src/pages/Dashboard/index.tsx
import { useEffect, useState } from 'react';
import ReactECharts from 'echarts-for-react';
import {
    getDashboardOverviewApi,
    getDashboardTrendApi,
    getDashboardAdsAnalysisApi,
    KOLOverview,
    TrendChart,
    AdsAnalysisDTO
} from '../../api/data_monitor/monitor';

export default function Dashboard() {
    const [loading, setLoading] = useState(true);
    const [overview, setOverview] = useState<KOLOverview | null>(null);
    const [trend, setTrend] = useState<TrendChart | null>(null);
    const [errorMsg, setErrorMsg] = useState('');
    const [adsData, setAdsData] = useState<AdsAnalysisDTO | null>(null);



    useEffect(() => {
        const fetchDashboardData = async () => {
            try {
                // 🚀 并发请求：同时拉取大盘概览和趋势图数据
                const [overviewRes, trendRes, adsRes]: any = await Promise.all([
                    getDashboardOverviewApi(),
                    getDashboardTrendApi(),
                    getDashboardAdsAnalysisApi()
                ]);

                const errorDetails: string[] = [];

                if (overviewRes.code !== 0 && overviewRes.code !== 200) {
                    errorDetails.push(`大盘概览(${overviewRes.code}): ${overviewRes.message || overviewRes.msg}`);
                }
                if (trendRes.code !== 0 && trendRes.code !== 200) {
                    errorDetails.push(`趋势分析(${trendRes.code}): ${trendRes.message || trendRes.msg}`);
                }
                if (adsRes.code !== 0 && adsRes.code !== 200) {
                    errorDetails.push(`商单洞察(${adsRes.code}): ${adsRes.message || adsRes.msg}`);
                }

                // 判断是否有拦截到的错误
                if (errorDetails.length === 0) {
                    // 全部成功，渲染视图
                    setOverview(overviewRes.data);
                    setTrend(trendRes.data);
                    setAdsData(adsRes.data);
                } else {
                    // 💥 爆炸现场还原：把具体哪个接口报什么错直接打印在屏幕和控制台上
                    const combinedError = errorDetails.join(' | ');
                    console.error('[MONITOR_SYS] 发现异常数据包:', errorDetails);
                    setErrorMsg(`API_ERR >> ${combinedError}`);
                }

            } catch (e: any) {
                // 捕获真正的 HTTP 500/404 等网络层崩溃
                console.error('[MONITOR_SYS] 网络层或网关崩溃', e);
                setErrorMsg(`NET_ERR >> ${e.message || '无法连接到监控核心引擎'}`);
            } finally {
                setLoading(false);
            }
        };
        fetchDashboardData();
    }, []);

    // 🚀 新增 ECharts 引擎：品牌分布环形图
    const getBrandPieOption = () => {
        if (!adsData || adsData.brand_distribution.length === 0) return {};
        return {
            tooltip: { trigger: 'item', backgroundColor: '#0f172a', borderColor: '#334155', textStyle: { color: '#e2e8f0', fontFamily: 'monospace' } },
            legend: { top: '5%', left: 'center', textStyle: { color: '#94a3b8', fontFamily: 'monospace' } },
            color: ['#22d3ee', '#818cf8', '#c084fc', '#f472b6', '#fb7185', '#e879f9'], // 极客渐变色系
            series: [{
                name: '合作品牌',
                type: 'pie',
                radius: ['40%', '70%'], // 环形图设计
                avoidLabelOverlap: false,
                itemStyle: { borderRadius: 5, borderColor: '#0f172a', borderWidth: 2 },
                label: { show: false, position: 'center' },
                emphasis: {
                    label: { show: true, fontSize: 16, fontWeight: 'bold', color: '#fff' }
                },
                labelLine: { show: false },
                data: adsData.brand_distribution.map(b => ({ name: b.brand_name, value: b.count }))
            }]
        };
    };

    // 智能数字降维格式化器 (如 150000 -> 15.0 W)
    const formatNumber = (num: number) => {
        if (!num) return '0';
        if (num >= 10000) return (num / 10000).toFixed(1) + ' W';
        if (num >= 1000) return (num / 1000).toFixed(1) + ' k';
        return num.toFixed(0);
    };

    // 格式化百分比
    const formatPercent = (num: number) => {
        if (!num) return '0.0%';
        return (num * 100).toFixed(1) + '%';
    };

    // 🚀 ECharts 核心图表引擎配置：商业雷达图
    const getRadarOption = () => {
        if (!overview?.radar) return {};
        const { radar } = overview;
        return {
            tooltip: { trigger: 'item', backgroundColor: '#0f172a', borderColor: '#334155', textStyle: { color: '#e2e8f0', fontFamily: 'monospace' } },
            radar: {
                indicator: [
                    { name: '传播力 (Spread)', max: 100 },
                    { name: '种草力 (Planting)', max: 100 },
                    { name: '铁粉度 (Loyalty)', max: 100 },
                    { name: '活跃度 (Active)', max: 100 },
                    { name: '互动性 (Interact)', max: 100 },
                    { name: '稳定性 (Stable)', max: 100 }
                ],
                shape: 'polygon',
                splitNumber: 5,
                axisName: { color: '#94a3b8', fontFamily: 'monospace', fontSize: 10 },
                splitLine: { lineStyle: { color: ['#1e293b', '#334155'] } },
                splitArea: { show: false },
                axisLine: { lineStyle: { color: '#334155' } }
            },
            series: [{
                name: 'KOL 六维能力值',
                type: 'radar',
                data: [{
                    value: [
                        radar.spread_score, radar.planting_score, radar.loyalty_score,
                        radar.active_score, radar.interact_score, radar.stable_score
                    ],
                    name: 'KOL Score',
                    itemStyle: { color: '#22d3ee' }, // Cyan-400
                    areaStyle: { color: 'rgba(34, 211, 238, 0.2)' },
                    lineStyle: { width: 2 }
                }]
            }]
        };
    };

    // 🚀 ECharts 核心图表引擎配置：生命周期趋势折线图
    const getTrendOption = () => {
        if (!trend) return {};
        return {
            tooltip: { trigger: 'axis', backgroundColor: '#0f172a', borderColor: '#334155', textStyle: { color: '#e2e8f0', fontFamily: 'monospace' } },
            legend: { data: trend.series.map(s => s.name), textStyle: { color: '#94a3b8', fontFamily: 'monospace' }, bottom: 0 },
            grid: { left: '3%', right: '4%', bottom: '15%', top: '10%', containLabel: true },
            xAxis: { type: 'category', boundaryGap: false, data: trend.categories, axisLabel: { color: '#94a3b8', fontFamily: 'monospace' }, axisLine: { lineStyle: { color: '#334155' } } },
            yAxis: { type: 'value', axisLabel: { color: '#94a3b8', fontFamily: 'monospace' }, splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } } },
            color: ['#22d3ee', '#a855f7', '#10b981'], // Cyan, Purple, Emerald
            series: trend.series.map(s => ({
                ...s,
                symbolSize: 8,
                lineStyle: { width: 3 }
            }))
        };
    };

    if (loading) return <div className="p-10 font-mono text-cyan-500 animate-pulse">$&gt; DECRYPTING_DASHBOARD_MATRIX...</div>;

    if (errorMsg) return (
        <div className="p-10">
            <div className="bg-red-950/30 border border-red-900 text-red-400 p-5 rounded font-mono">
                [SYS_ERR] {errorMsg}
            </div>
        </div>
    );

    return (
        <div className="p-6 max-w-7xl mx-auto space-y-6">
            {/* 顶栏控制台 */}
            <div className="flex justify-between items-end border-b border-slate-800 pb-4">
                <div>
                    <h1 className="text-2xl font-bold text-slate-100 font-mono tracking-wider">数据监控大屏</h1>
                    <p className="text-slate-500 font-mono text-xs mt-1">数据更新时间: {overview?.stat_date} | 平台id: {overview?.mid}</p>
                </div>
            </div>

            {/* 第 1 层：北极星指标翻牌器 (Hero Metrics) */}
            {overview && (
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    {/* 均播指标 (爆发力) */}
                    <div className="bg-slate-900/60 border border-slate-800 rounded-lg p-5 hover:border-cyan-700 transition-colors">
                        <div className="text-slate-500 font-mono text-xs mb-2">近30天播放量 (爆发力)</div>
                        <div className="text-3xl font-bold text-cyan-400 font-mono">{formatNumber(overview.avg_view_30d)}</div>
                        <div className="text-slate-600 font-mono text-[10px] mt-2">近30天视频产出: {overview.video_count_30d} 条</div>
                    </div>
                    {/* 中位数指标 (保底能力) */}
                    <div className="bg-slate-900/60 border border-slate-800 rounded-lg p-5 hover:border-purple-700 transition-colors">
                        <div className="text-slate-500 font-mono text-xs mb-2">近30天播放量中位数 (保底力)</div>
                        <div className="text-3xl font-bold text-purple-400 font-mono">{formatNumber(overview.median_view_30d)}</div>
                    </div>
                    {/* 互动率 */}
                    <div className="bg-slate-900/60 border border-slate-800 rounded-lg p-5 hover:border-emerald-700 transition-colors">
                        <div className="text-slate-500 font-mono text-xs mb-2">近30天互动率</div>
                        <div className="text-3xl font-bold text-emerald-400 font-mono">{formatPercent(overview.engagement_rate)}</div>
                    </div>
                    {/* 稳定性 */}
                    <div className="bg-slate-900/60 border border-slate-800 rounded-lg p-5 hover:border-amber-700 transition-colors">
                        <div className="text-slate-500 font-mono text-xs mb-2">近30天抗波段</div>
                        <div className="text-3xl font-bold text-amber-400 font-mono">{formatPercent(overview.stability_ratio)}</div>
                        <div className="text-slate-600 font-mono text-[10px] mt-2">中位数与均播的比值</div>
                    </div>
                </div>
            )}

            {/* 第 2 层与第 3 层：图表矩阵 */}
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">

                {/* 左侧：商业六维雷达图 */}
                <div className="lg:col-span-1 bg-slate-900/60 border border-slate-800 rounded-lg p-5 flex flex-col">
                    <h2 className="text-slate-400 font-mono text-sm border-b border-slate-800 pb-2 mb-4">商业能力雷达</h2>
                    <div className="flex-1 min-h-[300px]">
                        <ReactECharts
                            option={getRadarOption()}
                            style={{ height: '100%', width: '100%' }}
                            opts={{ renderer: 'svg' }}
                        />
                    </div>
                </div>

                {/* 右侧：生命周期趋势折线图 */}
                <div className="lg:col-span-2 bg-slate-900/60 border border-slate-800 rounded-lg p-5 flex flex-col">
                    <h2 className="text-slate-400 font-mono text-sm border-b border-slate-800 pb-2 mb-4">时间序列趋势分析</h2>
                    <div className="flex-1 min-h-[300px]">
                        <ReactECharts
                            option={getTrendOption()}
                            style={{ height: '100%', width: '100%' }}
                        />
                    </div>
                </div>

            </div>

            {/* 第 4 层：商单转化与品牌资产 (AI Commercial Insights) */}
            {adsData && adsData.total_ads_30d > 0 && (
                <div className="space-y-6 mt-8 pt-8 border-t border-slate-800 border-dashed">

                    {/* 标题栏 */}
                    <div className="flex items-center gap-4">
                        <h2 className="text-xl font-bold text-slate-100 font-mono tracking-wider">商业洞察(商单分析)</h2>
                        <span className="text-cyan-400 bg-cyan-950/30 border border-cyan-800 px-2 py-0.5 rounded text-xs font-mono">
                        近30天商单视频数: {adsData.total_ads_30d}条
                    </span>
                    </div>

                    {/* 🚀 严格划分界限 A：上半部分（双图表，双列网格） */}
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        {/* 左侧：品牌偏好分布图 */}
                        <div className="bg-slate-900/60 border border-slate-800 rounded-lg p-5">
                            <h3 className="text-slate-400 font-mono text-sm border-b border-slate-800 pb-2 mb-4">品牌偏好分布</h3>
                            <div className="h-[250px]">
                                <ReactECharts option={getBrandPieOption()} style={{ height: '100%', width: '100%' }} />
                            </div>
                        </div>

                        {/* 右侧：卖点词云热力阵列 */}
                        <div className="bg-slate-900/60 border border-slate-800 rounded-lg p-5">
                            <h3 className="text-slate-400 font-mono text-sm border-b border-slate-800 pb-2 mb-4">产品卖点热力图</h3>
                            <div className="flex flex-wrap gap-2.5 items-center justify-center h-[250px] overflow-y-auto p-2">
                                {adsData.top_selling_points.map((point, idx) => {
                                    const maxCount = adsData.top_selling_points[0]?.count || 1;
                                    const ratio = point.count / maxCount;
                                    const fontSize = 11 + (ratio * 5);
                                    const opacity = 0.5 + (ratio * 0.5);

                                    return (
                                        <span
                                            key={idx}
                                            style={{ fontSize: `${fontSize}px`, opacity }}
                                            className="font-bold text-cyan-400 hover:text-cyan-200 transition-colors cursor-default max-w-full truncate px-1"
                                            title={`提及次数: ${point.count} | 完整内容: ${point.point}`}
                                        >
                                        #{point.point}
                                    </span>
                                    );
                                })}
                            </div>
                        </div>
                    </div>
                    {/* 🛑 结束网格区域！下面的表格千万不能放进上面那个 div 里！ */}


                    {/* 🚀 严格划分界限 B：下半部分（数据表，强制占满 100% 宽度） */}
                    <div className="bg-slate-900/60 border border-slate-800 rounded-lg p-5 overflow-x-auto w-full block">
                        <h3 className="text-slate-400 font-mono text-sm border-b border-slate-800 pb-2 mb-4">近期商单明细</h3>
                        <table className="w-full text-left text-sm font-mono text-slate-300">
                            <thead className="text-xs text-slate-500 bg-slate-950/50 uppercase">
                            <tr>
                                <th className="px-4 py-3 rounded-tl-lg whitespace-nowrap">视频发布日期</th>
                                <th className="px-4 py-3 whitespace-nowrap">视频标题</th>
                                <th className="px-4 py-3 whitespace-nowrap">品牌方名称</th>
                                <th className="px-4 py-3 whitespace-nowrap">产品内容</th>
                                <th className="px-4 py-3 rounded-tr-lg">卖点标签</th>
                            </tr>
                            </thead>
                            <tbody>
                            {adsData.recent_videos.map((vid, idx) => (
                                <tr key={idx} className="border-b border-slate-800/50 hover:bg-slate-800/30 transition-colors">
                                    <td className="px-4 py-3 whitespace-nowrap text-xs text-slate-500">
                                        {vid.pubdate ? vid.pubdate.split(' ')[0] : 'N/A'}
                                    </td>
                                    <td className="px-4 py-3 max-w-[250px] truncate text-slate-200" title={vid.title}>{vid.title}</td>
                                    <td className="px-4 py-3 text-cyan-400 whitespace-nowrap">{vid.brand_name}</td>
                                    <td className="px-4 py-3 text-purple-400 min-w-[120px]">{vid.product_name}</td>
                                    <td className="px-4 py-3">
                                        <div className="flex flex-wrap gap-1">

                                            {/* 🚀 探针逻辑：如果数组完全为空，显示明确的提示 */}
                                            {(!vid.selling_points || vid.selling_points.length === 0) ? (
                                                <span className="text-[10px] text-slate-600/50 italic border border-dashed border-slate-700/50 px-1.5 py-0.5 rounded">
                                                [DB_NULL: 底表未提取或解析失败]
                                            </span>
                                            ) : (
                                                /* 如果有数据，正常渲染 */
                                                <>
                                                    {vid.selling_points.slice(0, 3).map((p, i) => (
                                                        <span key={i} className="text-[10px] bg-slate-800 border border-slate-700 px-1.5 py-0.5 rounded text-slate-400 max-w-[150px] truncate" title={p}>
                                                        {p}
                                                    </span>
                                                    ))}
                                                    {vid.selling_points.length > 3 && <span className="text-[10px] text-slate-600">...</span>}
                                                </>
                                            )}

                                        </div>
                                    </td>
                                </tr>
                            ))}
                            </tbody>
                        </table>
                    </div>

                </div>
            )}
        </div>
    );
}