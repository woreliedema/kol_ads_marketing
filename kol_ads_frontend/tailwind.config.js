/** @type {import('tailwindcss').Config} */
export default {
    // 1. 核心导航图：告诉 Tailwind 去哪里扫描 class 类名
    content: [
        "./index.html",
        "./src/**/*.{js,ts,jsx,tsx}",
    ],
    // 2. 主题扩展：可以在这里定义我们的极客专属色盘、字体等
    theme: {
        extend: {
            colors: {
                // 自定义一些赛博朋克风的背景色，方便以后调用 (如 bg-geek-dark)
                geek: {
                    dark: '#0A0A0F',
                    panel: '#13131A',
                    neon: '#22D3EE',
                }
            },
            fontFamily: {
                // 强制代码区块使用等宽字体
                mono: ['"Fira Code"', '"JetBrains Mono"', 'monospace'],
            }
        },
    },
    plugins: [],
}