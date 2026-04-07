import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { loginApi, getPublicKeyApi } from '../../api/user_center/auth.ts';
import { encryptPassword } from '../../utils/crypto';

export default function Login() {
    // 读取 URL 参数
    const [searchParams] = useSearchParams();
    const roleParam = searchParams.get('role');

    // 如果没有携带 role 参数（比如用户手敲 /login），我们强制将他踢回网关页
    const navigate = useNavigate();
    if (!roleParam || (roleParam !== '1' && roleParam !== '2')) {
        navigate('/welcome', { replace: true });
        return null; // 防止页面闪烁
    }

    const role = parseInt(roleParam, 10);
    const isKol = role === 1;

    // 动态主题配置
    const theme = {
        color: isKol ? 'cyan' : 'purple',
        title: isKol ? 'KOL_LOGIN' : 'BRAND_LOGIN',
        roleText: isKol ? '当前登录角色：红人/KOL' : '当前登录角色：品牌方/代理商',
        usernameLabel: isKol ? '用户名称' : '品牌方/代理商名称',
        usernamePlaceholder: isKol ? '请输入用户名称' : '请输入品牌方/代理商名称',
        contactLabel: isKol ? '用户绑定手机号或邮箱' : '品牌方绑定手机号或邮箱',
        passwordLabel: isKol ? '用户密码' : '品牌方/代理商登录密码',
        textGradient: isKol ? 'from-cyan-400 to-blue-500' : 'from-purple-400 to-pink-500',
        borderFocus: isKol ? 'focus:border-cyan-400 focus:ring-cyan-400' : 'focus:border-purple-400 focus:ring-purple-400',
        buttonBg: isKol ? 'text-cyan-400 border-cyan-500/50 hover:border-cyan-400 hover:shadow-[0_0_15px_rgba(34,211,238,0.3)]'
            : 'text-purple-400 border-purple-500/50 hover:border-purple-400 hover:shadow-[0_0_15px_rgba(168,85,247,0.3)]',
        glow: isKol ? 'bg-cyan-900/20' : 'bg-purple-900/20',
    };

    const [username, setUsername] = useState('');
    const [contact, setContact] = useState('');
    const [password, setPassword] = useState('');
    const [isAuthenticating, setIsAuthenticating] = useState(false);
    const [errorMsg, setErrorMsg] = useState('');

    const handleLogin =async (e: React.FormEvent) => {
        e.preventDefault();
        setIsAuthenticating(true);
        setErrorMsg('');

        try {
            console.log(`[SYS] Auth sequence initiated...`);
            // 动态获取安全锁 (RSA 公钥)
            const pkRes: any = await getPublicKeyApi();
            if (pkRes.code !== 0) {
                setErrorMsg('[SECURE_ERR] 安全环境初始化失败，请刷新重试');
                return;
            }
            const publicKey = pkRes.data.public_key;
            // RSA 融合时间戳高强度加密
            const encryptedPassword = encryptPassword(password, publicKey);
            if (!encryptedPassword) {
                setErrorMsg('[SECURE_ERR] 加密引擎异常');
                return;
            }
            // 发起网络请求
            const payload = {
                account: contact,        // 将前端的 contact(手机/邮箱) 映射给后端的 account
                client_type: 'pc',       // 注入终端类型
                password: encryptedPassword,
                username: username,      // 如果后端强校验 username，则一起传过去
                role: role               // 透传角色 (1: KOL, 2: Brand)
            };

            console.log(`[SYS] 正在构建出站协议数据包...`, payload);

            const res: any = await loginApi(payload);

            // 假设后端成功返回的 code 是 200
            if (res.code === 0 ||res.code === 200) {
                // 保存真正的 JWT Token
                localStorage.setItem('access_token', res.data.token);
                navigate('/dashboard');
            } else {
                setErrorMsg(res.message || res.msg || 'Authentication failed');
            }
        } catch (err: any) {
            // 捕获 Axios 响应拦截器抛出的错误
            setErrorMsg(err.message || err.msg || '[SYS_ERR] Cannot connect to server gateway.');
        } finally {
            setIsAuthenticating(false);
        }
    };


    return (
        <div className="min-h-screen flex items-center justify-center relative overflow-hidden bg-slate-950">
            <div className={`absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] blur-[120px] rounded-full pointer-events-none ${theme.glow}`}></div>

            <div className="relative z-10 w-full max-w-md p-8 bg-slate-900/80 backdrop-blur-xl border border-slate-700 shadow-2xl rounded-lg">

                {/* 简易的 × 关闭按钮 */}
                <button
                    onClick={() => navigate('/welcome')}
                    className="absolute top-4 right-4 text-slate-500 hover:text-slate-300 text-2xl leading-none transition-colors"
                    title="返回"
                >
                    &times;
                </button>

                <div className="mb-8 mt-2 border-b border-slate-800 pb-4">
                    <h1 className={`text-2xl font-bold font-mono text-transparent bg-clip-text bg-gradient-to-r ${theme.textGradient} tracking-tight`}>
                        {theme.title}
                    </h1>
                    <p className="text-slate-400 text-xs font-mono mt-2 opacity-80">
                        {theme.roleText}
                    </p>
                </div>

                {errorMsg && <div className="mb-4 text-red-400 text-xs font-mono">{errorMsg}</div>}

                <form onSubmit={handleLogin} className="space-y-5">
                    {/* 用户名称 */}
                    <div>
                        <label className={`block text-${theme.color}-400 text-xs font-mono mb-2`}>{theme.usernameLabel}</label>
                        <input
                            type="text" required value={username} onChange={(e) => setUsername(e.target.value)}
                            className={`w-full bg-slate-950/50 text-slate-200 border border-slate-700 ${theme.borderFocus} rounded px-4 py-2 outline-none transition-all font-mono text-sm`}
                            placeholder={theme.usernamePlaceholder}
                        />
                    </div>

                    {/* 绑定手机号或邮箱 */}
                    <div>
                        <label className={`block text-${theme.color}-400 text-xs font-mono mb-2`}>{theme.contactLabel}</label>
                        <input
                            type="text" required value={contact} onChange={(e) => setContact(e.target.value)}
                            className={`w-full bg-slate-950/50 text-slate-200 border border-slate-700 ${theme.borderFocus} rounded px-4 py-2 outline-none transition-all font-mono text-sm`}
                            placeholder="请输入绑定手机号或邮箱"
                        />
                    </div>

                    {/* 登录密码 */}
                    <div>
                        <label className={`block text-${theme.color}-500 text-xs font-mono mb-2`}>{theme.passwordLabel}</label>
                        <input
                            type="password" required value={password} onChange={(e) => setPassword(e.target.value)}
                            className={`w-full bg-slate-950/50 text-slate-200 border border-slate-700 ${theme.borderFocus} rounded px-4 py-2 outline-none transition-all font-mono text-sm`}
                            placeholder="请输入登录密码"
                        />
                    </div>

                    <button
                        type="submit" disabled={isAuthenticating}
                        className={`w-full relative group overflow-hidden rounded border px-4 py-3 font-mono text-sm transition-all active:scale-[0.98] mt-2 ${
                            isAuthenticating ? 'border-slate-600 text-slate-400 bg-slate-800 cursor-not-allowed' : `bg-slate-900 ${theme.buttonBg}`
                        }`}
                    >
            <span className="relative z-10 font-bold tracking-widest">
              {isAuthenticating ? '登录中...' : '登录'}
            </span>
                    </button>
                </form>

                <div className="mt-8 text-center text-slate-500 font-mono text-xs border-t border-slate-800 pt-4">
          <span
              onClick={() => navigate(`/register?role=${role}`)}
              className={`text-${theme.color}-400 cursor-pointer hover:text-slate-200 hover:underline transition-colors`}
          >
            还未注册? 注册
          </span>
                </div>
            </div>
        </div>
    );
}