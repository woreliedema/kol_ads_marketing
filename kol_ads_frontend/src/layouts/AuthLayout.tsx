import React from 'react';
import { Outlet } from 'react-router-dom';

const AuthLayout: React.FC = () => {
    return (
        <div className="min-h-screen bg-slate-950 flex items-center justify-center relative overflow-hidden">
            {/* 极客风背景：动态网格与渐变 */}
            <div className="absolute inset-0 bg-[linear-gradient(to_right,#4f4f4f2e_1px,transparent_1px),linear-gradient(to_bottom,#4f4f4f2e_1px,transparent_1px)] bg-[size:14px_24px] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)]"></div>

            {/* 光晕点缀 */}
            <div className="absolute top-0 -translate-y-12 left-1/2 -translate-x-1/2 w-[800px] h-[400px] bg-cyan-500/20 blur-[120px] rounded-full pointer-events-none"></div>

            {/* 核心内容区 */}
            <div className="relative z-10 w-full max-w-md p-8 bg-slate-900/80 backdrop-blur-md border border-slate-700 rounded-xl shadow-[0_0_40px_-10px_rgba(34,211,238,0.3)]">
                {/* 顶部 Logo 或 Terminal Header */}
                <div className="flex items-center space-x-2 mb-8">
                    <div className="w-3 h-3 rounded-full bg-red-500"></div>
                    <div className="w-3 h-3 rounded-full bg-yellow-500"></div>
                    <div className="w-3 h-3 rounded-full bg-green-500"></div>
                    <span className="ml-4 text-slate-400 font-mono text-sm border-l border-slate-600 pl-4">
             ~/kol_ads_platform/auth
          </span>
                </div>

                {/* 渲染子路由 (Login 或 Register) */}
                <Outlet />
            </div>
        </div>
    );
};

export default AuthLayout;