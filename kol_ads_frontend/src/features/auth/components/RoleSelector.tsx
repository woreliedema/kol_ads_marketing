// src/features/auth/components/RoleSelector.tsx
import React from 'react';

interface RoleSelectorProps {
    currentRole: 1 | 2; // 1代表红人，2代表品牌方，对应你的 sys_users 表
    onChange: (role: 1 | 2) => void;
}

const RoleSelector: React.FC<RoleSelectorProps> = ({ currentRole, onChange }) => {
    return (
        <div className="flex space-x-4">
            {/* 红人选择按钮 */}
            <button
                type="button"
                onClick={() => onChange(1)}
                className={`flex-1 py-3 px-4 border rounded font-mono text-sm transition-all duration-300 ${
                    currentRole === 1
                        ? 'border-cyan-500 bg-cyan-500/10 text-cyan-400 shadow-[0_0_15px_rgba(34,211,238,0.2)]'
                        : 'border-slate-700 bg-slate-900 text-slate-500 hover:border-slate-500 hover:text-slate-300'
                }`}
            >
                <span className="block text-xs mb-1 opacity-70">ROLE:</span>
                [1] CREATOR / 红人
            </button>

            {/* 品牌方选择按钮 */}
            <button
                type="button"
                onClick={() => onChange(2)}
                className={`flex-1 py-3 px-4 border rounded font-mono text-sm transition-all duration-300 ${
                    currentRole === 2
                        ? 'border-purple-500 bg-purple-500/10 text-purple-400 shadow-[0_0_15px_rgba(168,85,247,0.2)]'
                        : 'border-slate-700 bg-slate-900 text-slate-500 hover:border-slate-500 hover:text-slate-300'
                }`}
            >
                <span className="block text-xs mb-1 opacity-70">ROLE:</span>
                [2] BRAND / 品牌方
            </button>
        </div>
    );
};

export default RoleSelector;