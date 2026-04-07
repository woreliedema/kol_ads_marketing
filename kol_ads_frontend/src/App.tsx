import { Suspense, lazy } from 'react';
import type { ReactNode } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';

// 懒加载页面：提升首屏加载速度
const Gateway = lazy(() => import('@pages/Auth/Gateway'));
const BaseLayout = lazy(() => import('@layouts/BaseLayout'));
const Login = lazy(() => import('@pages/Auth/Login'));
const Register = lazy(() => import('@pages/Auth/Register'));
const Dashboard = lazy(() => import('@pages/Dashboard'));
const KOLList = lazy(() => import('@pages/KOL/List'));
const KOLAnalysis = lazy(() => import('@pages/KOL/Analysis'));
const CampaignManager = lazy(() => import('@pages/Campaign'));
const Profile = lazy(() => import('@pages/Profile'));
const Match=lazy(()=>import('@pages/Match'));
const IMTerminal = lazy(() => import('@pages/IM'));


// 简单的鉴权高阶组件拦截器示例
const RequireAuth = ({ children }: { children: ReactNode }) => {
    const token = localStorage.getItem('access_token');
    if (!token) {
        return <Navigate to="/welcome" replace />;
    }
    return <>{children}</>;
};

// 加载时的骨架屏或 Loading
const PageLoader = () => (
    <div className="flex items-center justify-center h-screen">
        <span>Loading...</span>
    </div>
);

function App() {
    return (
        <Suspense fallback={<PageLoader />}>
            <Routes>
                {/* 公共路由 */}
                <Route path="/welcome" element={<Gateway />} />
                <Route path="/login" element={<Login />} />
                <Route path="/register" element={<Register />} />


                {/* 核心业务路由（需要登录鉴权，包裹在全局 Layout 中） */}
                <Route path="/" element={ <RequireAuth> <BaseLayout /> </RequireAuth>}>
                    {/* 重定向默认到看板 */}
                    <Route index element={<Navigate to="/dashboard" replace />} />
                    <Route path="dashboard" element={<Dashboard />} />
                    {/* 挂载个人中心路由 */}
                    <Route path="profile" element={<Profile />} />
                    {/* 挂载匹配系统路由 */}
                    <Route path="match" element={<Match />} />
                    {/* 挂载IM模块路由 */}
                    <Route path="im" element={<IMTerminal />} />
                    {/* KOL 资源库 */}
                    <Route path="kol">
                        <Route index element={<KOLList />} />
                        {/* 动态路由：分析特定 KOL 的商业价值 */}
                        <Route path="analysis/:kolId" element={<KOLAnalysis />} />
                    </Route>

                    {/* 广告活动（Campaign）管理 */}
                    <Route path="campaign" element={<CampaignManager />} />

                    {/* 捕获 404 */}
                    <Route path="*" element={<div>404 Not Found</div>} />
                </Route>
            </Routes>
        </Suspense>
    );
}

export default App;