// src/api/request.ts
import axios, { InternalAxiosRequestConfig, AxiosResponse, AxiosError } from 'axios';

// 1. 初始化 Axios 实例
const api = axios.create({
    // 这里的 /api/v1 会被 vite.config.ts 中的 proxy 拦截，并转发给 Go Hertz 后端 (localhost:8080)
    baseURL: '/api/v1',
    timeout: 15000, // 15秒超时设定
    headers: {
        'Content-Type': 'application/json',
    },
});

// 2. 请求拦截器 (Outbound Firewall)
api.interceptors.request.use(
    (config: InternalAxiosRequestConfig) => {
        // 从本地存储或状态管理中获取 Token
        const token = localStorage.getItem('access_token');

        // 如果存在 Token，则注入到 Authorization 头部 (Bearer 规范)
        if (token) {
            config.headers.Authorization = `Bearer ${token}`;
        }

        // 💡 极客终端日志：打印出站请求
        console.log(`[SYS_NET] 📤 Outbound transmission to: ${config.url}`);

        return config;
    },
    (error: AxiosError) => {
        console.error('[SYS_NET] ❌ Outbound sequence failed:', error);
        return Promise.reject(error);
    }
);

// 3. 响应拦截器 (Inbound Firewall)
api.interceptors.response.use(
    (response: AxiosResponse) => {
        // 💡 极客终端日志：打印入站响应
        console.log(`[SYS_NET] 📥 Data packet received from: ${response.config.url}`);

        // 自动剥离 HTTP 外壳，直接返回后端的 JSON 数据 (对应 utils.H 里的内容)
        return response.data;
    },
    (error: AxiosError) => {
        // 处理 HTTP 状态码错误
        // 【防御墙一】: 根本没有收到服务器响应 (断网、Go服务器没开、CORS跨域)
        if (!error.response) {
            console.error('[SYS_NET] 🚫 Connection severed. Target server unreachable.');
            return Promise.reject({ msg: '无法连接到远程服务器，请检查网络矩阵' });
        }
        // 【防御墙二】: 收到了服务器响应，此时 TypeScript 能够100%推断 status 必然是 number 类型
        const status = error.response?.status;
        const errorData = error.response?.data as any;

        console.error(`[SYS_NET] ⚠️ Connection intercepted. Status: ${status}`);

        if (status === 401) {
            // 401 未授权：Token 过期或伪造
            console.warn('[SYS_NET] Unauthorized access. Wiping credentials...');
            localStorage.removeItem('access_token');
            // 强制刷新并踢回网关页
            window.location.href = '/welcome';
        } else if (status === 403) {
            console.error('[SYS_NET] Access Denied. Insufficient clearance.');
        } else if (status === 409) {
            console.error(`[SYS_NET] Entity collision: ${errorData?.msg || 'Data already exists.'}`);
        } else if (status >= 500) {
            console.error('[SYS_NET] Remote server matrix failure.');
        }

        // 将后端的错误信息向下抛出，让具体的业务组件可以 catch 到并显示在 UI 上
        return Promise.reject(errorData || error);
    }
);

export default api;