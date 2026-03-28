import api from '../request';

// 红人检索参数
export interface KolFilterParams {
    field_tag?: string;
    fan_level?: string; // e.g., '10000,100000'
    price_min?: number;
    price_max?: number;
    page?: number;
    size?: number;
}

// 品牌方检索参数
export interface BrandFilterParams {
    field_tag?: string;
    is_verified?: number;
    page?: number;
    size?: number;
}

// 品牌方调用：筛选红人
export const filterKolsApi = (params: KolFilterParams) => {
    return api.get('/match/kol/filter', { params });
};

// 红人调用：筛选品牌方
export const filterBrandsApi = (params: BrandFilterParams) => {
    return api.get('/match/brand/filter', { params });
};