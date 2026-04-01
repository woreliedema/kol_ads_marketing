// src/pages/Match/index.tsx
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { filterKolsApi, filterBrandsApi } from '../../api/match_engine/match.ts';
import {getUserInfoApi, getTagTreeApi, TagNode} from '../../api/user_center/user.ts';

export default function MatchMatrix() {
    const navigate = useNavigate();

    const [role, setRole] = useState<number | null>(null);
    const [loading, setLoading] = useState(true);
    const [searching, setSearching] = useState(false);
    // 级联标签筛选器状态
    const [tagTree, setTagTree] = useState<TagNode[]>([]); // 👈 定义 tagTree 和 setTagTree
    const [selectedTags, setSelectedTags] = useState<string[]>([]);

    // 数据集
    const [list, setList] = useState<any[]>([]);
    const [total, setTotal] = useState(0);

    // 分页状态
    const [page, setPage] = useState(1);
    const size = 12; // 每页 12 个卡片，适合 grid 布局


    // 筛选器状态 (红人搜品牌)
    // const [brandIndustry, setBrandIndustry] = useState('');
    const [brandVerified, setBrandVerified] = useState<string>(''); // '' | '1' | '0'

    // 筛选器状态 (品牌搜红人)
    // const [kolTag, setKolTag] = useState('');
    const [kolPriceMin, setKolPriceMin] = useState<string>('');
    const [kolPriceMax, setKolPriceMax] = useState<string>('');

    // 1. 初始化鉴权与角色探测
    useEffect(() => {
        const initRole = async () => {
            try {
                const res: any = await getUserInfoApi();
                if (res.code === 0 || res.code === 200) {
                    setRole(res.data.base_info.role); // 1-KOL, 2-Brand
                }
            } catch (e) {
                console.error("角色探测失败");
            } finally {
                setLoading(false);
            }
        };
        initRole();
    }, []);

    // 2. 核心检索引擎
    const triggerSearch = async (currentPage = 1) => {
        if (!role) return;
        setSearching(true);
        try {
            const tagsPayload = selectedTags.length > 0 ? selectedTags.join(',') : undefined;

            if (role === 1) { // KOL 搜索品牌
                const res: any = await filterBrandsApi({
                    field_tag: tagsPayload,
                    is_verified: brandVerified ? Number(brandVerified) : undefined,
                    page: currentPage,
                    size
                });
                if (res.code === 0) {
                    setList(res.data.list || []);
                    setTotal(res.data.total || 0);
                }
            } else { // 品牌搜索 KOL
                const res: any = await filterKolsApi({
                    field_tag: tagsPayload,
                    price_min: kolPriceMin ? Number(kolPriceMin) : undefined,
                    price_max: kolPriceMax ? Number(kolPriceMax) : undefined,
                    page: currentPage,
                    size
                });
                if (res.code === 0) {
                    setList(res.data.list || []);
                    setTotal(res.data.total || 0);
                }
            }
        } catch (e) {
            console.error("匹配引擎异常");
        } finally {
            setSearching(false);
        }
    };

    // 阶梯式数据格式化引擎 (粉丝量智能降维)
    const formatFollowers = (count: number) => {
        if (!count || isNaN(count)) return '0';
        if (count < 1000) return count.toString(); // 0 ~ 999: 原样输出
        if (count < 10000) return (count / 1000).toFixed(1) + ' k'; // 1000 ~ 9999: 1.x k
        return (count / 10000).toFixed(1) + ' w'; // >= 10000: x.x w
    };

    // 跨路由唤起 IM 终端
    const handleStartChat = (item: any) => {
        // 💡 终极装甲：不依赖外部的 isKol 状态，直接让数据自己说话！
        // 只要卡片里有 brand_user_id 就取它，没有就取 kol_user_id
        const targetUserId = item.brand_user_id || item.kol_user_id;

        // 名字优先展示品牌公司名，没有再降级到账号用户名
        const targetUserName = item.company_name || item.username || 'UNKNOWN_TARGET';

        // 头像同理，哪边有值取哪边
        const targetAvatar = item.avatar_url || item.kol_avatar_url || '/default-avatar.png';

        // 组装标准跨端通讯档案
        const targetUser = {
            target_user_id: String(targetUserId), // 绝对安全的 String 强转
            target_user_name: targetUserName,
            target_avatar: targetAvatar
        };

        console.log('[MATCH_SYS] 准备跃迁，目标对象提取成功:', targetUser);

        // 带着真实数据跃迁到通讯终端
        navigate('/im', { state: { targetUser } });
    };

    // 角色加载完毕或页码变化时自动搜索
    useEffect(() => {
        if (role === null) return;

        // 💡 极客交叉逻辑：红人(role=1)去查品牌标签(target=1)，品牌(role=2)去查红人标签(target=2)
        const crossTargetType = role === 1 ? 1 : 2;

        const fetchTree = async () => {
            try {
                // 🚀 极客调用：显式传入对方的 targetType！
                const res: any = await getTagTreeApi(crossTargetType);
                if (res.code === 0 || res.code === 200) {
                    setTagTree(res.data.tree || []);
                }
            } catch (e) { console.error("标签字典加载失败"); }
        };
        fetchTree();
    }, [role]);

    const toggleTag = (tagName: string) => {
        setSelectedTags(prev => {
            if (prev.includes(tagName)) return prev.filter(t => t !== tagName); // 取消选择
            if (prev.length >= 6) {
                alert('[SYS_WARN] 检索条件过于宽泛：最多只能同时选择 6 个领域进行匹配！');
                return prev;
            }
            return [...prev, tagName]; // 添加选择
        });
    };

    // 重置并从第一页搜索
    const handleFilter = () => {
        setPage(1);
        triggerSearch(1);
    };

    if (loading) return <div className="p-10 font-mono text-cyan-500 animate-pulse">$&gt; INITIALIZING_MATCH_ENGINE...</div>;
    if (!role) return null;

    const isKol = role === 1;
    const theme = isKol ? 'cyan' : 'purple';

    return (
        <div className="p-6 md:p-10 max-w-7xl mx-auto text-slate-300 min-h-[calc(100vh-64px)]">

            {/* 头部标题 */}
            <div className="mb-8 border-b border-slate-800 pb-4">
                <h1 className={`text-2xl font-mono text-${theme}-400 font-bold flex items-center gap-3`}>
                    <span className="animate-pulse">●</span>
                    {isKol ? 'BUSINESS_OPPORTUNITIES (探索品牌方)' : 'CREATOR_MATRIX (探索内容红人)'}
                </h1>
                <p className="text-slate-500 text-xs font-mono mt-2">
                    {isKol ? '基于行业图谱匹配潜在的商业合作伙伴。' : '基于标签、报价与粉丝画像精准锁定优质创作者。'}
                </p>
            </div>

            {/* 战术筛选器面板 */}
            <div className="bg-slate-900/50 border border-slate-800 rounded-lg p-5 mb-8 shadow-lg">

                {/* 🚀 第一层：级联标签筛选区 (双端通用) */}
                <div className="flex flex-col gap-4 mb-6 border-b border-slate-800/50 pb-6 w-full">
                    <div className="flex items-start">
                        <span className="text-slate-500 text-xs font-mono w-24 shrink-0 pt-2">$&gt; CATEGORY</span>

                        <div className="flex-1 flex flex-wrap gap-2 items-center">
                            <button
                                onClick={() => setSelectedTags([])}
                                className={`px-3 py-1.5 text-xs font-mono rounded transition-colors ${selectedTags.length === 0 ? `bg-${theme}-500/20 text-${theme}-400 border border-${theme}-500/50` : 'text-slate-400 hover:text-slate-200'}`}
                            >
                                全部领域
                            </button>

                            {/* 动态渲染字典树：一级标签展现，Hover拉出二级标签 */}
                            {tagTree.map(parent => (
                                <div key={parent.id} className="relative group z-20">
                                    <button className={`px-3 py-1.5 text-xs font-mono text-slate-400 hover:text-${theme}-400 flex items-center gap-1 transition-colors`}>
                                        {parent.name} <span className="text-[8px] opacity-50">▼</span>
                                    </button>

                                    {/* 二级标签下拉悬浮窗 */}
                                    {parent.children && parent.children.length > 0 && (
                                        <div className="absolute left-0 top-full pt-2 w-[320px] opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 ease-in-out">
                                            <div className={`flex flex-wrap bg-slate-950 border border-${theme}-900/50 rounded shadow-[0_10px_30px_rgba(0,0,0,0.8)] p-3 gap-2`}>
                                                {parent.children.map(child => {
                                                    const isSelected = selectedTags.includes(child.name);
                                                    return (
                                                        <button
                                                            key={child.id}
                                                            onClick={() => toggleTag(child.name)}
                                                            className={`px-2 py-1 text-[10px] font-mono rounded border transition-all ${isSelected ? `bg-${theme}-900/50 border-${theme}-500 text-${theme}-300` : 'bg-slate-900 border-slate-700 text-slate-400 hover:border-slate-500 hover:text-slate-200'}`}
                                                        >
                                                            {isSelected ? '■' : '□'} {child.name}
                                                        </button>
                                                    );
                                                })}
                                            </div>
                                        </div>
                                    )}
                                </div>
                            ))}
                        </div>
                    </div>

                    {/* 已选标签回显面板 */}
                    {selectedTags.length > 0 && (
                        <div className="flex items-center">
                            <span className="text-slate-500 text-xs font-mono w-24 shrink-0">$&gt; SELECTED</span>
                            <div className="flex flex-wrap gap-2">
                                {selectedTags.map(tag => (
                                    <span key={tag} className={`bg-${theme}-950/40 border border-${theme}-800 text-${theme}-300 px-2 py-1 text-[10px] font-mono rounded flex items-center gap-1`}>
                    {tag}
                                        <button onClick={() => toggleTag(tag)} className="hover:text-red-400 ml-1 font-bold">&times;</button>
                  </span>
                                ))}
                            </div>
                        </div>
                    )}
                </div>

                {/* 🚀 第二层：角色专属属性过滤区 (认证状态 / 价格区间) */}
                <div className="flex flex-wrap gap-4 items-end">
                    {isKol ? (
                        <div className="w-48">
                            <label className="block text-slate-500 text-[10px] font-mono mb-1">$&gt; VERIFICATION_STATUS</label>
                            <select value={brandVerified} onChange={e => setBrandVerified(e.target.value)} className="w-full bg-slate-950 border border-slate-700 focus:border-cyan-500 rounded px-3 py-2 text-xs font-mono outline-none appearance-none cursor-pointer">
                                <option value="">ALL (不限)</option>
                                <option value="1">VERIFIED (已认证品牌)</option>
                                <option value="0">UNVERIFIED (未认证实体)</option>
                            </select>
                        </div>
                    ) : (
                        <>
                            <div className="w-32">
                                <label className="block text-slate-500 text-[10px] font-mono mb-1">$&gt; MIN_QUOTE (¥)</label>
                                <input type="number" placeholder="最低报价" value={kolPriceMin} onChange={e => setKolPriceMin(e.target.value)} className="w-full bg-slate-950 border border-slate-700 focus:border-purple-500 rounded px-3 py-2 text-xs font-mono outline-none" />
                            </div>
                            <div className="w-32">
                                <label className="block text-slate-500 text-[10px] font-mono mb-1">$&gt; MAX_QUOTE (¥)</label>
                                <input type="number" placeholder="最高报价" value={kolPriceMax} onChange={e => setKolPriceMax(e.target.value)} className="w-full bg-slate-950 border border-slate-700 focus:border-purple-500 rounded px-3 py-2 text-xs font-mono outline-none" />
                            </div>
                        </>
                    )}

                    {/* 执行搜索按钮 */}
                    <button
                        onClick={handleFilter}
                        disabled={searching}
                        className={`px-8 py-2 bg-${theme}-950/30 border border-${theme}-600 text-${theme}-400 text-xs font-bold font-mono rounded hover:bg-${theme}-900/50 hover:shadow-[0_0_15px_currentColor] transition-all cursor-pointer h-[34px]`}
                    >
                        {searching ? 'SCANNING...' : '[ EXECUTE_QUERY ]'}
                    </button>
                </div>
            </div>

            {/* 数据卡片矩阵 */}
            {searching ? (
                <div className="py-20 flex justify-center">
                    <div className={`w-12 h-12 border-4 border-${theme}-900 border-t-${theme}-400 rounded-full animate-spin`}></div>
                </div>
            ) : list.length === 0 ? (
                <div className="py-20 text-center border border-dashed border-slate-800 rounded-lg bg-slate-900/20">
                    <p className="text-slate-500 font-mono text-sm">NO_ENTITIES_FOUND_IN_THIS_SECTOR</p>
                </div>
            ) : (
                <div className="flex flex-col gap-4">
                    {list.map((item, idx) => {
                        // 💡 极客防御：安全解析标签 (兼容品牌方的 JSON 字符串和红人方的数组)
                        let parsedTags: string[] = [];
                        try {
                            parsedTags = Array.isArray(item.tags) ? item.tags : (typeof item.tags === 'string' ? JSON.parse(item.tags || '[]') : []);
                        } catch (e) {}
                        // 如果解析失败或者为空，但存在旧版的 industry，则降级使用 industry
                        if (parsedTags.length === 0 && item.industry) parsedTags = [item.industry];

                        return (
                            <div
                                key={idx}
                                className={`bg-slate-900/40 border rounded-lg p-5 transition-all duration-300 hover:shadow-xl hover:-translate-y-1 flex flex-col md:flex-row items-start md:items-center gap-6 ${isKol ? 'border-cyan-900/30 hover:border-cyan-600' : 'border-purple-900/30 hover:border-purple-600'}`}
                            >
                                {/* 1. 核心身份区 (左侧) */}
                                <div className="flex items-center gap-4 w-full md:w-[30%] shrink-0">
                                    <div className="relative shrink-0">
                                        <img
                                            src={isKol ? (item.avatar_url || '/default-avatar.png') : (item.kol_avatar_url || '/default-avatar.png')}
                                            alt="avatar"
                                            className={`w-14 h-14 rounded-full border-2 object-cover bg-slate-800 ${isKol ? 'border-cyan-800' : 'border-purple-800'}`}
                                        />
                                        {/* 状态绿点指示器 */}
                                        {(item.is_verified === 1 || item.status === 1) && (
                                            <span className="absolute bottom-0 right-0 w-3.5 h-3.5 rounded-full border-2 border-slate-900 bg-green-500"></span>
                                        )}
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <h3 className="text-lg font-bold text-slate-200 truncate" title={isKol ? item.company_name : item.username}>
                                            {isKol ? item.company_name : item.username}
                                        </h3>
                                        {/* 🚀 物理抹除真实 ID，展示高端业务状态 */}
                                        <p className="text-slate-500 text-[10px] font-mono mt-1">
                                            STATUS: {isKol ? (item.is_verified === 1 ? 'V_CERTIFIED' : 'UNVERIFIED') : (item.status === 1 ? 'ACTIVE' : 'IDLE')}
                                        </p>
                                    </div>
                                </div>

                                {/* 2. 商业指标区 (中部 - 仅品牌方看红人时有具体指标) */}
                                <div className="flex gap-8 w-full md:w-[25%] shrink-0">
                                    {!isKol && (
                                        <>
                                            <div>
                                                <p className="text-slate-500 text-[10px] font-mono mb-1">$&gt; FANS</p>
                                                <p className="text-purple-400 font-bold font-mono text-lg">
                                                    {formatFollowers(item.total_followers)}
                                                </p>
                                            </div>
                                            <div>
                                                <p className="text-slate-500 text-[10px] font-mono mb-1">$&gt; BASE_QUOTE</p>
                                                <p className="text-green-400 font-bold font-mono text-lg">
                                                    ¥ {item.base_quote}
                                                </p>
                                            </div>
                                        </>
                                    )}
                                </div>

                                {/* 3. 战术标签与操作区 (右侧) */}
                                <div className="flex-1 flex flex-col md:flex-row items-start md:items-center justify-between w-full gap-4 min-w-0">

                                    {/* 标签与平台矩阵 */}
                                    <div className="flex flex-wrap items-center gap-1.5 flex-1 min-w-0">
                                        {parsedTags.slice(0, 4).map((tag: string, i: number) => (
                                            <span key={i} className={`text-[10px] border px-2 py-1 rounded whitespace-nowrap ${isKol ? 'text-cyan-400 border-cyan-900 bg-cyan-950/30' : 'text-purple-400 border-purple-900 bg-purple-950/30'}`}>
                                    #{tag}
                                </span>
                                        ))}

                                        {/* 红人的 UGC 平台小图标 (保留你之前的精妙设计) */}
                                        {!isKol && item.ugc_platforms && item.ugc_platforms[0] !== "" && item.ugc_platforms.map((plat: string, i: number) => (
                                            <span key={`plat-${i}`} className="w-5 h-5 flex items-center justify-center rounded bg-slate-800 text-slate-300 text-[10px] font-bold ml-1 shadow-sm" title={plat}>
                                    {plat.substring(0,1).toUpperCase()}
                                </span>
                                        ))}
                                    </div>

                                    {/* 发起沟通按钮 (自动适配双端主题色) */}
                                    <button
                                        onClick={() => handleStartChat(item)}
                                        className={`shrink-0 px-6 py-2.5 border text-xs font-bold font-mono rounded transition-all cursor-pointer flex items-center gap-2 ${isKol ? 'bg-cyan-950/30 border-cyan-800 text-cyan-400 hover:bg-cyan-900/50 hover:shadow-[0_0_15px_rgba(34,211,238,0.3)]' : 'bg-purple-950/30 border-purple-800 text-purple-400 hover:bg-purple-900/50 hover:shadow-[0_0_15px_rgba(168,85,247,0.3)]'}`}
                                    >
                                        <span>[+] INITIATE_COMMS</span>
                                    </button>
                                </div>

                            </div>
                        );
                    })}
                </div>
            )}

            {/* 极客风分页器 */}
            {total > 0 && (
                <div className="mt-10 flex items-center justify-center gap-4 font-mono text-sm">
                    <button
                        disabled={page === 1}
                        onClick={() => setPage(p => p - 1)}
                        className="text-slate-500 hover:text-cyan-400 disabled:opacity-30 disabled:hover:text-slate-500 cursor-pointer"
                    >&lt; PREV</button>

                    <span className="text-slate-300 bg-slate-900 px-4 py-1 rounded border border-slate-700 shadow-[inset_0_0_10px_rgba(0,0,0,0.5)]">
            PAGE {page} / {Math.ceil(total / size)}
          </span>

                    <button
                        disabled={page >= Math.ceil(total / size)}
                        onClick={() => setPage(p => p + 1)}
                        className="text-slate-500 hover:text-cyan-400 disabled:opacity-30 disabled:hover:text-slate-500 cursor-pointer"
                    >NEXT &gt;</button>
                </div>
            )}

        </div>
    );
}