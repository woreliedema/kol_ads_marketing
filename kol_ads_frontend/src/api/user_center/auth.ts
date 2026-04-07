import api from '../request';

// 我们可以利用 TypeScript 定义接口的入参和出参，做到极其严谨的代码提示
export interface LoginPayload {
    role: number;
    username?: string;
    account?: string; // 手机或邮箱
    password: string;
    client_type: string;
}

export interface RegisterPayload {
    role: number;
    username: string;
    phone?: string;
    email?: string;
    password: string;
}

export const getPublicKeyApi = () => {
    return api.get('/auth/public-key');
};

// 登录接口
export const loginApi = (data: LoginPayload) => {
    return api.post('/auth/login', data);
};

// 注册接口
export const registerApi = (data: RegisterPayload) => {
    return api.post('/auth/register', data);
};