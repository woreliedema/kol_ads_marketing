// src/pages/Auth/Gateway.tsx
import { useNavigate } from 'react-router-dom';

export default function Gateway() {
    const navigate = useNavigate();

    const selectRole = (role: 1 | 2) => {
        // 携带角色参数跳转到登录页
        navigate(`/login?role=${role}`);
    };

    return (
        <div className="min-h-screen flex flex-col items-center justify-center relative overflow-hidden bg-slate-950">
            {/* 极客风背景网格 */}
            <div className="absolute inset-0 bg-[linear-gradient(to_right,#4f4f4f2e_1px,transparent_1px),linear-gradient(to_bottom,#4f4f4f2e_1px,transparent_1px)] bg-[size:14px_24px] [mask-image:radial-gradient(ellipse_80%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>

            <div className="relative z-10 text-center mb-12">
                <h1 className="text-4xl font-bold font-mono text-transparent bg-clip-text bg-gradient-to-r from-slate-100 to-slate-500 tracking-tighter mb-4">
                    SELECT_YOUR_ENTITY
                </h1>
                <p className="text-slate-400 font-mono text-sm">
                    Please identify your connection protocol to enter the grid.
                </p>
            </div>

            <div className="relative z-10 flex flex-col md:flex-row gap-8 w-full max-w-4xl px-8">
                {/* KOL / 创作者通道 */}
                <div
                    onClick={() => selectRole(1)}
                    className="flex-1 group cursor-pointer relative p-1 rounded-xl bg-gradient-to-b from-cyan-500/50 to-transparent hover:from-cyan-400 transition-all duration-500 hover:scale-105 hover:shadow-[0_0_40px_-10px_rgba(34,211,238,0.5)]"
                >
                    <div className="h-full w-full bg-slate-900/90 backdrop-blur-sm rounded-lg p-8 border border-cyan-900/50 group-hover:border-cyan-500/50 flex flex-col items-center justify-center text-center transition-all">
                        <div className="w-16 h-16 rounded-full bg-cyan-950 border border-cyan-500 flex items-center justify-center mb-6 group-hover:shadow-[0_0_20px_rgba(34,211,238,0.4)] transition-all">
                            <span className="text-cyan-400 font-mono text-2xl">01</span>
                        </div>
                        <h2 className="text-2xl font-bold text-cyan-50 font-mono mb-2">CREATOR / 红人</h2>
                        <p className="text-slate-400 text-sm font-mono h-16">
                            Connect your UGC platforms. Analyze audience. Monetize your influence.
                        </p>
                        <div className="mt-6 px-6 py-2 border border-cyan-500/30 text-cyan-400 font-mono text-sm rounded group-hover:bg-cyan-500/10 transition-colors">
                            我是KOL
                        </div>
                    </div>
                </div>

                {/* Brand / 品牌方通道 */}
                <div
                    onClick={() => selectRole(2)}
                    className="flex-1 group cursor-pointer relative p-1 rounded-xl bg-gradient-to-b from-purple-500/50 to-transparent hover:from-purple-400 transition-all duration-500 hover:scale-105 hover:shadow-[0_0_40px_-10px_rgba(168,85,247,0.5)]"
                >
                    <div className="h-full w-full bg-slate-900/90 backdrop-blur-sm rounded-lg p-8 border border-purple-900/50 group-hover:border-purple-500/50 flex flex-col items-center justify-center text-center transition-all">
                        <div className="w-16 h-16 rounded-full bg-purple-950 border border-purple-500 flex items-center justify-center mb-6 group-hover:shadow-[0_0_20px_rgba(168,85,247,0.4)] transition-all">
                            <span className="text-purple-400 font-mono text-2xl">02</span>
                        </div>
                        <h2 className="text-2xl font-bold text-purple-50 font-mono mb-2">BRAND / 品牌主</h2>
                        <p className="text-slate-400 text-sm font-mono h-16">
                            Discover top creators. Launch campaigns. Track ROI with data matrix.
                        </p>
                        <div className="mt-6 px-6 py-2 border border-purple-500/30 text-purple-400 font-mono text-sm rounded group-hover:bg-purple-500/10 transition-colors">
                            我是品牌方/代理商
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}