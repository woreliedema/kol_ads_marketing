import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { registerApi, getPublicKeyApi } from '../../api/user_center/auth';
import { encryptPassword } from '../../utils/crypto';

export default function Register() {
    const [searchParams] = useSearchParams();
    const roleParam = searchParams.get('role');
    const navigate = useNavigate();

    if (!roleParam || (roleParam !== '1' && roleParam !== '2')) {
        navigate('/welcome', { replace: true });
        return null;
    }

    const role = parseInt(roleParam, 10);
    const isKol = role === 1;

    // 表单状态统一化
    const [username, setUsername] = useState('');
    const [phone, setPhone] = useState('');
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');

    const [isRegistering, setIsRegistering] = useState(false);
    const [errorMsg, setErrorMsg] = useState('');

    // 动态主题配置
    const theme = {
        textColor: isKol ? 'text-cyan-400' : 'text-purple-400',
        title: isKol ? 'KOL注册' : '品牌方注册',
        subtitle: isKol ? 'KOL个人信息注册' : '品牌方/代理商信息注册',
        usernameLabel: isKol ? '用户名称' : '品牌方/代理商名称',
        phoneLabel: isKol ? '绑定手机号' : '品牌方绑定手机号',
        emailLabel: isKol ? '绑定邮箱' : '品牌方绑定邮箱',
        gradient: isKol ? 'from-cyan-400 to-blue-500' : 'from-purple-400 to-pink-500',
        focusRing: isKol ? 'focus:border-cyan-400 focus:ring-cyan-400' : 'focus:border-purple-400 focus:ring-purple-400',
        buttonStyle: isKol
            ? 'text-cyan-400 border-cyan-500/50 hover:border-cyan-400 hover:shadow-[0_0_15px_rgba(34,211,238,0.3)]'
            : 'text-purple-400 border-purple-500/50 hover:border-purple-400 hover:shadow-[0_0_15px_rgba(168,85,247,0.3)]',
        glow: isKol ? 'bg-cyan-900/20' : 'bg-purple-900/20',
    };

    const handleRegister =async (e: React.FormEvent) => {
        e.preventDefault();
        setErrorMsg('');

        if (password !== confirmPassword) {
            setErrorMsg('[ERR] 两次输入的密码不一致');
            return;
        }

        setIsRegistering(true);

        try {
            // 动态获取安全锁 (RSA 公钥)
            const pkRes: any = await getPublicKeyApi();
            if (pkRes.code !== 0) {
                setErrorMsg('[SECURE_ERR] 安全环境初始化失败，请刷新重试');
                return;
            }
            const publicKey = pkRes.data.public_key;

            // RSA 加密
            const encryptedPassword = encryptPassword(password, publicKey);
            if (!encryptedPassword) {
                setErrorMsg('[SECURE_ERR] 加密引擎异常');
                return;
            }
            // 发起网络请求
            const payload = {
                email: email,
                password: encryptedPassword,
                phone: phone,
                role: role,
                username: username
            };
            const res: any = await registerApi(payload);


            if (res.code === 0) {
                // 打印后端返回的 "注册成功，请登录"
                console.log('[SYS] 注册成功响应:', res.data?.message);
                // 注册成功后，跳转回对应角色的登录页
                navigate(`/login?role=${role}`);
            } else {
                setErrorMsg(res.msg || 'Registration failed');
            }
        } catch (err: any) {
            console.error('[SYS_NET] 注册请求异常:', err);
            // 处理后端返回的 409 冲突错误等
            const errorDetail = err.message || err.msg || err.error || '[SYS_ERR] 无法连接到服务器矩阵';
            setErrorMsg(errorDetail);
        } finally {
            setIsRegistering(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center relative overflow-hidden bg-slate-950 py-12">
            <div className={`fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[800px] h-[800px] blur-[150px] rounded-full pointer-events-none ${theme.glow}`}></div>

            <div className="relative z-10 w-full max-w-lg p-8 bg-slate-900/80 backdrop-blur-xl border border-slate-700 shadow-2xl rounded-lg">

                {/* 简易的 × 关闭按钮 */}
                <button
                    onClick={() => navigate('/welcome')}
                    className="absolute top-4 right-4 text-slate-500 hover:text-slate-300 text-2xl leading-none transition-colors"
                    title="取消注册"
                >
                    &times;
                </button>

                <div className="mb-8 mt-2 border-b border-slate-800 pb-4">
                    <h1 className={`text-2xl font-bold font-mono text-transparent bg-clip-text bg-gradient-to-r ${theme.gradient} tracking-tight`}>
                        {theme.title}
                    </h1>
                    <p className="text-slate-400 text-xs font-mono mt-2 opacity-80">
                        {theme.subtitle}
                    </p>
                </div>

                {errorMsg && (
                    <div className="mb-6 p-3 border border-red-500/50 bg-red-950/30 text-red-400 font-mono text-xs rounded animate-pulse">
                        {errorMsg}
                    </div>
                )}

                <form onSubmit={handleRegister} className="space-y-5">

                    <div>
                        <label className={`block ${theme.textColor} text-xs font-mono mb-2`}>{theme.usernameLabel}</label>
                        <input type="text" required value={username} onChange={(e) => setUsername(e.target.value)}
                               className={`w-full bg-slate-950/50 text-slate-200 border border-slate-700 ${theme.focusRing} rounded px-4 py-2 outline-none transition-all font-mono text-sm`}
                        />
                    </div>

                    <div>
                        <label className={`block ${theme.textColor} text-xs font-mono mb-2`}>{theme.phoneLabel}</label>
                        <input type="tel" required value={phone} onChange={(e) => setPhone(e.target.value)}
                               className={`w-full bg-slate-950/50 text-slate-200 border border-slate-700 ${theme.focusRing} rounded px-4 py-2 outline-none transition-all font-mono text-sm`}
                        />
                    </div>

                    <div>
                        <label className={`block ${theme.textColor} text-xs font-mono mb-2`}>{theme.emailLabel}</label>
                        <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)}
                               className={`w-full bg-slate-950/50 text-slate-200 border border-slate-700 ${theme.focusRing} rounded px-4 py-2 outline-none transition-all font-mono text-sm`}
                        />
                    </div>

                    <div>
                        <label className={`block ${theme.textColor} text-xs font-mono mb-2`}>登录密码</label>
                        <input type="password" required value={password} onChange={(e) => setPassword(e.target.value)}
                               className={`w-full bg-slate-950/50 text-slate-200 border border-slate-700 ${theme.focusRing} rounded px-4 py-2 outline-none transition-all font-mono text-sm`}
                        />
                    </div>

                    <div>
                        <label className={`block ${theme.textColor} text-xs font-mono mb-2`}>再次输入密码</label>
                        <input type="password" required value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)}
                               className={`w-full bg-slate-950/50 text-slate-200 border border-slate-700 ${theme.focusRing} rounded px-4 py-2 outline-none transition-all font-mono text-sm`}
                        />
                    </div>

                    <button
                        type="submit" disabled={isRegistering}
                        className={`w-full relative group overflow-hidden rounded border px-4 py-3 font-mono text-sm transition-all active:scale-[0.98] mt-6 ${
                            isRegistering ? 'border-slate-600 text-slate-400 bg-slate-800 cursor-not-allowed' : `bg-slate-900 ${theme.buttonStyle}`
                        }`}
                    >
            <span className="relative z-10 font-bold tracking-widest">
              {isRegistering ? '正在注册...' : '注册'}
            </span>
                    </button>
                </form>

                <div className="mt-8 text-center text-slate-500 font-mono text-xs border-t border-slate-800 pt-4">
          <span
              onClick={() => navigate(`/login?role=${role}`)}
              className={`${theme.textColor} cursor-pointer hover:text-slate-200 hover:underline transition-colors`}
          >
            已有账号？返回登录
          </span>
                </div>
            </div>
        </div>
    );
}