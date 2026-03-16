import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { ConfigProvider } from 'antd'; // 以 Ant Design 为例
import zhCN from 'antd/locale/zh_CN'; // 全局中文配置
// 假设使用 React Query 进行服务端状态管理，非常适合频繁的数据查询
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import App from './App';
import './index.css'; // 全局样式，包含 TailwindCSS 等

// 初始化 React Query 客户端，配置默认的缓存和重试策略
const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            refetchOnWindowFocus: false, // 失去焦点后不重新请求
            retry: 1, // 失败后重试一次
            staleTime: 5 * 60 * 1000, // 5分钟内数据视为新鲜，不发新请求，减轻后端压力
        },
    },
});

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
    // 严格模式：帮助在开发阶段发现潜在的副作用和生命周期问题
    <React.StrictMode>
        <QueryClientProvider client={queryClient}>
            <ConfigProvider locale={zhCN} theme={{ token: { colorPrimary: '#1677ff' } }}>
                <BrowserRouter>
                    <App />
                </BrowserRouter>
            </ConfigProvider>
        </QueryClientProvider>
    </React.StrictMode>,
);