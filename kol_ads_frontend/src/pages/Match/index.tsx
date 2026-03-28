// src/pages/Match/index.tsx
import { useEffect, useState } from 'react';
import { filterKolsApi, filterBrandsApi } from '../../api/match_engine/match.ts';
import {getUserInfoApi, getTagTreeApi, TagNode} from '../../api/user_center/user.ts';

export default function MatchMatrix() {
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
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
                    {list.map((item, idx) => (
                        <div key={idx} className="bg-slate-900/60 border border-slate-800 hover:border-slate-600 rounded-lg overflow-hidden group transition-all duration-300 hover:shadow-xl hover:-translate-y-1">

                            {/* 红人看品牌方的卡片 */}
                            {isKol ? (
                                <div className="p-5">
                                    <div className="flex items-start justify-between mb-4">
                                        <img src={item.avatar_url || '/default-avatar.png'} alt="logo" className="w-12 h-12 rounded bg-slate-800 object-cover" />
                                        {item.is_verified === 1 && <span className="text-xs text-green-400 border border-green-800 bg-green-950/30 px-2 py-0.5 rounded font-mono">V_CERTIFIED</span>}
                                    </div>
                                    <h3 className="text-lg font-bold text-slate-200 truncate">{item.company_name}</h3>
                                    <p className="text-slate-500 text-xs font-mono mt-1">ID: {item.brand_user_id} | @{item.username}</p>
                                    <div className="mt-4 pt-4 border-t border-slate-800">
                    <span className="text-cyan-400 text-xs border border-cyan-900 bg-cyan-950/30 px-2 py-1 rounded">
                      INDUSTRY: {item.industry || 'UNKNOWN'}
                    </span>
                                    </div>
                                </div>
                            ) : (

                                // 品牌方看红人的卡片
                                <div className="p-5">
                                    <div className="flex items-center gap-4 mb-4">
                                        <img src={item.kol_avatar_url || '/default-avatar.png'} alt="avatar" className="w-12 h-12 rounded-full border border-purple-500/30 object-cover" />
                                        <div>
                                            <h3 className="text-base font-bold text-slate-200 truncate">{item.username}</h3>
                                            <p className="text-slate-500 text-[10px] font-mono">ID: {item.kol_user_id} | STATUS: {item.status === 1 ? 'OK' : 'ERR'}</p>
                                        </div>
                                    </div>
                                    <div className="grid grid-cols-2 gap-2 mb-4">
                                        <div className="bg-slate-950 p-2 rounded border border-slate-800">
                                            <div className="text-slate-500 text-[10px] font-mono">FANS</div>
                                            <div className="text-purple-400 font-mono font-bold">{(item.total_followers / 10000).toFixed(1)} W</div>
                                        </div>
                                        <div className="bg-slate-950 p-2 rounded border border-slate-800">
                                            <div className="text-slate-500 text-[10px] font-mono">BASE_QUOTE</div>
                                            <div className="text-green-400 font-mono font-bold">¥ {item.base_quote}</div>
                                        </div>
                                    </div>
                                    <div className="flex flex-wrap gap-1 mb-3">
                                        {item.tags?.slice(0,3).map((tag: string, i: number) => (
                                            <span key={i} className="text-[10px] text-slate-400 border border-slate-700 px-1.5 py-0.5 rounded bg-slate-900">#{tag}</span>
                                        ))}
                                    </div>
                                    <div className="pt-3 border-t border-slate-800 flex gap-2">
                                        {item.ugc_platforms?.map((plat: string, i: number) => (
                                            <span key={i} className="w-5 h-5 flex items-center justify-center rounded bg-slate-800 text-slate-300 text-[10px] font-bold" title={plat}>
                        {plat.substring(0,1).toUpperCase()}
                      </span>
                                        ))}
                                    </div>
                                </div>
                            )}
                        </div>
                    ))}
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