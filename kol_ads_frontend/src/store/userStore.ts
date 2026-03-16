// src/store/userStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface UserState {
    token: string | null;
    role: number | null; // 1-红人, 2-品牌方, 99-管理员
    username: string | null;
    setAuth: (token: string, role: number, username: string) => void;
    logout: () => void;
}

export const useUserStore = create<UserState>()(
    persist(
        (set) => ({
            token: null,
            role: null,
            username: null,
            setAuth: (token, role, username) => set({ token, role, username }),
            logout: () => set({ token: null, role: null, username: null }),
        }),
        {
            name: 'kol-auth-storage', // 存储在 localStorage 中的 key
        }
    )
);