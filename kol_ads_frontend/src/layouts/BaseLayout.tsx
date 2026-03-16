// src/components/Layout/BaseLayout.tsx
import { Outlet, useNavigate, useLocation } from 'react-router-dom';

export default function BaseLayout() {
    const navigate = useNavigate();
    const location = useLocation();

    // 极客风导航菜单配置
    const navItems = [
        {
            path: '/dashboard',
            label: 'CORE_MATRIX',
            zhLabel: '核心数据大盘',
            icon: '[#]',
        },
        {
            path: '/profile',
            label: 'ENTITY_PROFILE',
            zhLabel: '实体档案控制台',
            icon: '[&]',
        },
        // 为未来预留的扩展接口
        {
            path: '/campaigns',
            label: 'CAMPAIGN_NODES',
            zhLabel: '商业任务节点',
            icon: '[*]',
            disabled: true, // 尚未开发的模块打上禁用标记
        },
    ];

    // 终结当前会话 (登出)
    const handleLogout = () => {
        if (window.confirm('WARNING: 是否退出登录当前账号? ')) {
            localStorage.removeItem('access_token');
            navigate('/welcome');
        }
    };

    return (
        <div className="flex h-screen w-full bg-slate-950 overflow-hidden font-mono selection:bg-cyan-500/30">

            {/* --- 左侧全息侧边栏 (Sidebar) --- */}
            <aside className="w-64 flex-shrink-0 border-r border-slate-800 bg-slate-900/50 flex flex-col relative z-20 backdrop-blur-md">

                {/* 系统标识 / Logo 区 */}
                <div className="h-16 flex items-center px-6 border-b border-slate-800">
                    <div className="flex items-center gap-2">
                        <div className="w-3 h-3 bg-cyan-400 rounded-sm animate-pulse shadow-[0_0_10px_rgba(34,211,238,0.6)]"></div>
                        <span className="text-slate-100 font-bold tracking-widest text-lg">KOL_MATRIX</span>
                    </div>
                </div>

                {/* 导航链路映射 */}
                <nav className="flex-1 py-6 px-4 space-y-2 overflow-y-auto">
                    <div className="text-slate-600 text-[10px] mb-4 pl-2 tracking-widest">$&gt; NAV_SYSTEM</div>

                    {navItems.map((item) => {
                        const isActive = location.pathname.startsWith(item.path);

                        if (item.disabled) {
                            return (
                                <div key={item.path} className="flex flex-col px-4 py-3 rounded-lg border border-transparent text-slate-600 opacity-50 cursor-not-allowed">
                                    <div className="flex items-center gap-3">
                                        <span className="text-xs">{item.icon}</span>
                                        <span className="text-sm font-bold">{item.label}</span>
                                    </div>
                                    <span className="text-[10px] mt-1 pl-7">{item.zhLabel} (LOCKED)</span>
                                </div>
                            );
                        }

                        return (
                            <button
                                key={item.path}
                                onClick={() => navigate(item.path)}
                                className={`w-full flex flex-col px-4 py-3 rounded-lg border transition-all duration-300 text-left group cursor-pointer active:scale-[0.98]
                  ${isActive
                                    ? 'bg-cyan-950/30 border-cyan-500/50 text-cyan-400 shadow-[inset_0_0_20px_rgba(34,211,238,0.1)]'
                                    : 'border-transparent text-slate-400 hover:bg-slate-800/50 hover:border-slate-700 hover:text-slate-200'}
                `}
                            >
                                <div className="flex items-center gap-3">
                                    <span className={`text-xs ${isActive ? 'text-cyan-400' : 'text-slate-500 group-hover:text-slate-300'}`}>{item.icon}</span>
                                    <span className="text-sm font-bold tracking-wide">{item.label}</span>
                                </div>
                                <span className={`text-[10px] mt-1 pl-7 ${isActive ? 'text-cyan-600' : 'text-slate-500'}`}>
                  {item.zhLabel}
                </span>
                            </button>
                        );
                    })}
                </nav>

                {/* 底部雷达状态与登出 */}
                <div className="p-4 border-t border-slate-800 bg-slate-900/80">
                    <div className="flex items-center justify-between text-[10px] text-slate-500 mb-4">
                        <span>在线状态: </span>
                        <span className="text-green-500 animate-pulse">在线</span>
                    </div>
                    <button
                        onClick={handleLogout}
                        className="w-full py-2 border border-red-900/50 text-red-500/70 hover:text-red-400 hover:border-red-500 hover:bg-red-950/30 text-xs rounded transition-all duration-300 flex items-center justify-center gap-2 cursor-pointer"
                    >
                        <span>退出登录</span>
                    </button>
                </div>
            </aside>

            {/* --- 右侧主显示区 (Main Workspace) --- */}
            <main className="flex-1 flex flex-col relative overflow-hidden bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-slate-900 via-slate-950 to-black">

                {/* 顶部战术导轨 (Header) */}
                <header className="h-16 flex-shrink-0 border-b border-slate-800 bg-slate-900/30 backdrop-blur-sm flex items-center justify-between px-6 relative z-10">
                    {/* 动态面包屑寻址 */}
                    <div className="flex items-center gap-2 text-xs text-slate-500">
                        <span>ROOT</span>
                        <span>/</span>
                        <span className="text-cyan-400 animate-pulse">
              {location.pathname.substring(1).toUpperCase() || 'UNKNOWN_SECTOR'}
            </span>
                    </div>

                    {/* 模拟网络波形动画 */}
                    <div className="flex items-end gap-1 h-4">
                        <div className="w-1 bg-cyan-500/50 animate-[bounce_1s_infinite] h-2"></div>
                        <div className="w-1 bg-cyan-500/50 animate-[bounce_1.2s_infinite] h-4"></div>
                        <div className="w-1 bg-cyan-500/50 animate-[bounce_0.8s_infinite] h-3"></div>
                        <div className="w-1 bg-cyan-500/50 animate-[bounce_1.5s_infinite] h-1"></div>
                    </div>
                </header>

                {/* 核心内容渲染器 (Outlet 将会把子路由组件渲染在这里) */}
                <div className="flex-1 overflow-y-auto relative z-0 custom-scrollbar">
                    {/* 给主内容区加一点底部的内边距，防止内容贴底 */}
                    <div className="pb-12">
                        <Outlet />
                    </div>
                </div>
            </main>

        </div>
    );
}