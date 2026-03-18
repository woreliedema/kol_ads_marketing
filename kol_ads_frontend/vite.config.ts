import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
    plugins: [react()],
    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src'),
            '@components': path.resolve(__dirname, './src/components'),
            '@layouts': path.resolve(__dirname, './src/layouts'),
            '@pages': path.resolve(__dirname, './src/pages'),
            '@services': path.resolve(__dirname, './src/service'),
            '@utils': path.resolve(__dirname, './src/utils'),
            '@store': path.resolve(__dirname, './src/store'),
            '@hooks': path.resolve(__dirname, './src/hooks'),
        },
    },
    server: {
        port: 3000,
        open: true,
        // 代理配置：将前端的 API 请求路由到后端的不同微服务
        proxy: {
            '/api/v1/auth': {
                target: 'http://localhost:8081', // 指向 Go 写的 user_center 服务
                changeOrigin: true,
                // rewrite: (path) => path.replace(/^\/api\/v1\/user/, '/api/v1/user'),
            },
            '/api/v1/user': {
                target: 'http://localhost:8081',
                changeOrigin: true,
            },
            '/api/v1/crawler': {
                target: 'http://localhost:8000', // 指向 Python 写的 data_collection_service
                changeOrigin: true,
                // rewrite: (path) => path.replace(/^\/api\/v1\/crawler/, '/api/v1/crawler'),
            },
            '/uploads': {
                target: 'http://localhost:8081', // 转发给 Go 的 h.Static
                changeOrigin: true,
            },
            // 未来如果有独立网关，直接指向网关即可
        },
    },
    build: {
        target: 'es2015',
        cssCodeSplit: true,
        rollupOptions: {
            output: {
                // 分包策略：将第三方库单独打包，利用浏览器缓存机制提升加载性能
                manualChunks: {
                    vendor: ['react', 'react-dom', 'react-router-dom'],
                    ui: ['antd', '@ant-design/icons'], // 假设使用 Ant Design
                    chart: ['echarts'], // KOL 数据分析图表
                },
            },
        },
    },
});