import api from '../request';

// 1. 基础信息定义
export interface BaseInfo {
    id: number;
    username: string;
    phone: string;
    email: string;
    role: number; // 1: KOL, 2: Brand
    status: number;
    created_at: string;
}

// 2. UGC 账号定义
export interface UgcAccount {
    id: number;
    platform: string;
    platform_uid: string;
    nickname: string;
    fans_count: number;
    bound_at: string;
    auth_status: number;// 新增：0=待同步爬取，1=同步成功
}

// 3. 动态配置扩展资料 (使用 any 兼容两者的字段，或者分开写)
export interface UserProfileData {
    // 公共字段
    id: number;
    user_id: number;
    // KOL 专属字段
    real_name?: string;
    avatar_url?: string;
    tags?: string; // 注意：后端返回的是字符串化的 JSON 数组，如 "[\"游戏\", \"主播\"]"
    base_quote?: number;
    credit_score?: number;
    // 品牌方专属字段
    company_name?: string;
    industry?: string;
    license_url?: string;
    is_verified?: boolean;
}

// 红人三方平台接口绑定请求体
export interface UGCBindPayload {
    platform: string;
    platform_space_url: string;
}


// 4. 聚合返回体
export interface UserInfoResponse {
    base_info: BaseInfo;
    profile: UserProfileData;
    ugc_accounts: UgcAccount[];
}

export interface KOLProfilePayload {
    real_name: string;
    base_quote: number;
}

export interface BrandProfilePayload {
    company_name: string;
}

// 新增：标签系统专属数据结构
export interface TagNode {
    id: number;
    name: string;
    children?: TagNode[];
}

// 新增：获取专属标签树
export const getTagTreeApi = (targetType?: number) => {
    return api.get('/user/tags/tree',{
        params: { target_type: targetType }
    });
};

// 新增：更新用户领域标签
export const updateUserTagsApi = (tags: string[]) => {
    return api.put('/user/profile/tags', { tags });
};

// 获取账号详细信息
export const getUserInfoApi = () => {
    return api.get('/user/info');
};

// 修改密码
export const resetPasswordApi = (data: any) => {
    return api.post('/user/password/reset', data);
};

// 修改 KOL 扩展资料
export const updateKolProfileApi = (data: KOLProfilePayload) => {
    return api.put('/user/kol/profile', data);
};

// 修改品牌方扩展资料
export const updateBrandProfileApi = (data: BrandProfilePayload) => {
    return api.put('/user/brand/profile', data);
};

// 品牌方绑定三方平台账号
export const bindUgcAccountApi = (data: UGCBindPayload) => {
    return api.post('/user/ugc/bind', data);
};

// 精准查询单个 UGC 平台的绑定/同步状态 (GET)
export const getUgcBindResultApi = (platform: string) => {
    return api.get(`/user/ugc/bind/result?platform=${platform}`);
};


// 5. 上传/修改头像 (POST)
// 注意：上传文件必须使用 FormData 格式，并且让 Axios 自动处理 Boundary
export const uploadAvatarApi = (formData: FormData) => {
    return api.post('/user/avatar/upload', formData, {
        headers: {
            'Content-Type': 'multipart/form-data',
        },
    });
};

// 6. 品牌方上传营业执照 (POST)
// 字段名必须是 'license'
export const uploadBrandLicenseApi = (formData: FormData) => {
    return api.post('/user/brand/license/upload', formData, {
        headers: {
            'Content-Type': 'multipart/form-data',
        },
    });
};

// 7. 品牌方删除营业执照 (DELETE)
export const deleteBrandLicenseApi = (data: { password: string }) => {
    // axios 的 delete 方法传递 body 比较特殊，需要放在 data 属性里
    return api.delete('/user/brand/license', { data });
};