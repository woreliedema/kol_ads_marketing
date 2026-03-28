import { useEffect, useState, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
    getUserInfoApi,
    resetPasswordApi,
    updateKolProfileApi,
    updateBrandProfileApi,
    uploadBrandLicenseApi,
    deleteBrandLicenseApi,
    bindUgcAccountApi,
    getUgcBindResultApi,
    uploadAvatarApi,
    getTagTreeApi,
    updateUserTagsApi,
    UserInfoResponse,
    TagNode
} from '../../api/user_center/user.ts';

export default function Profile() {
    const navigate = useNavigate();
    const [data, setData] = useState<UserInfoResponse | null>(null);
    const [loading, setLoading] = useState(true);
    const [errorMsg, setErrorMsg] = useState('');

    // 弹窗控制状态
    const [isPwdModalOpen, setIsPwdModalOpen] = useState(false);
    const [isProfileModalOpen, setIsProfileModalOpen] = useState(false);
    const [submitting, setSubmitting] = useState(false);

    // 密码表单状态
    const [oldPassword, setOldPassword] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');

    // 资料表单状态 (聚合了KOL和Brand的字段)
    // const [editAvatar, setEditAvatar] = useState('');
    const [editName, setEditName] = useState(''); // KOL用real_name, Brand用company_name
    const [editQuote, setEditQuote] = useState<number | ''>('');

    // UGC 绑定表单状态
    const [isUgcModalOpen, setIsUgcModalOpen] = useState(false);
    const [ugcPlatform, setUgcPlatform] = useState('bilibili'); // 默认平台
    const [ugcPlatformSpaceUrl, setUgcPlatformSpaceUrl] = useState('');

    // 头像上传相关的状态和 Ref
    const fileInputRef = useRef<HTMLInputElement>(null);
    const [isUploadingAvatar, setIsUploadingAvatar] = useState(false);

    // 营业执照上传相关的状态和 Ref
    const licenseInputRef = useRef<HTMLInputElement>(null);
    const [isUploadingLicense, setIsUploadingLicense] = useState(false);

    // 营业执照交互相关的状态
    const [isPreviewModalOpen, setIsPreviewModalOpen] = useState(false); // 控制大图预览
    const [isDeleteLicenseModalOpen, setIsDeleteLicenseModalOpen] = useState(false); // 控制删除确认弹窗
    const [licenseDeletePassword, setLicenseDeletePassword] = useState(''); // 绑定的密码输入
    const [isDeletingLicense, setIsDeletingLicense] = useState(false); // 删除 Loading 状态

    // 标签矩阵专属状态
    const [isTagModalOpen, setIsTagModalOpen] = useState(false);
    const [tagTree, setTagTree] = useState<TagNode[]>([]); // 树形字典
    const [selectedTags, setSelectedTags] = useState<string[]>([]); // 用户已选的标签名
    const [isFirstLoginIntercept, setIsFirstLoginIntercept] = useState(false); // 是否是注册后首次拦截

    // 处理头像选择与上传逻辑
    const handleAvatarChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) return;

        // 1. 前端极速防御：拦截体积和格式 (和后端逻辑严格对齐)
        if (file.size > 5 * 1024 * 1024) {
            alert('[SYS_ERR] 图片载荷过大，不能超过 5MB');
            e.target.value = ''; // 清空选择
            return;
        }
        const validTypes = ['image/jpeg', 'image/jpg', 'image/png'];
        if (!validTypes.includes(file.type)) {
            alert('[SYS_ERR] 非法的数据协议，仅支持 JPG/PNG 格式');
            e.target.value = '';
            return;
        }

        // 2. 组装 FormData
        const formData = new FormData();
        formData.append('avatar', file);

        // 3. 发送上传指令
        setIsUploadingAvatar(true);
        try {
            const res: any = await uploadAvatarApi(formData);
            if (res.code === 0 || res.code === 200) {
                // 后端返回了 avatar_url，我们直接刷新整个大盘数据即可
                fetchUserData();
            } else {
                // 如果触发了 7 天冷却锁，这里会弹出后端的报错：修改太频繁啦！还需等待...
                alert(res.message || res.msg || '头像上传失败');
            }
        } catch (err: any) {
            alert(err.message || err.msg || '网络链接断开');
        } finally {
            setIsUploadingAvatar(false);
            e.target.value = ''; // 无论成功失败，重置 input
        }
    };


    // 处理营业执照上传
    const handleLicenseChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) return;

        // 1. 极速防御：拦截 10MB 体积
        if (file.size > 10 * 1024 * 1024) {
            alert('[SYS_ERR] 资质文件体积过大，不能超过 10MB');
            e.target.value = '';
            return;
        }
        const validTypes = ['image/jpeg', 'image/jpg', 'image/png'];
        if (!validTypes.includes(file.type)) {
            alert('[SYS_ERR] 非法协议，仅支持 JPG/PNG 格式图片');
            e.target.value = '';
            return;
        }

        // 2. 组装载荷 (字段名必须与后端 FormFile 提取的 "license" 一致)
        const formData = new FormData();
        formData.append('license', file);

        // 3. 建立传输加密隧道
        setIsUploadingLicense(true);
        try {
            const res: any = await uploadBrandLicenseApi(formData);
            if (res.code === 0 || res.code === 200) {
                // 成功后重新拉取大盘数据，刷新凭证 URL
                fetchUserData();
            } else {
                alert(res.message || res.msg || '资质文件部署失败');
            }
        } catch (err: any) {
            alert(err.message || err.msg || '连接中继站失败');
        } finally {
            setIsUploadingLicense(false);
            e.target.value = '';
        }
    };

    // 处理营业执照销毁
    const handleDeleteLicenseSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsDeletingLicense(true);
        try {
            const res: any = await deleteBrandLicenseApi({ password: licenseDeletePassword });
            if (res.code === 0 || res.code === 200) {
                setIsDeleteLicenseModalOpen(false);
                setLicenseDeletePassword(''); // 清空密码
                fetchUserData(); // 刷新大盘数据，执照会恢复成未上传状态
            } else {
                alert(res.message || res.msg || '销毁失败');
            }
        } catch (err: any) {
            alert(err.message || err.msg || '网络异常');
        } finally {
            setIsDeletingLicense(false);
        }
    };



    const handleUgcSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setSubmitting(true);
        try {
            const payload = {
                platform: ugcPlatform,
                platform_space_url: ugcPlatformSpaceUrl
            };
            const res: any = await bindUgcAccountApi(payload);

            if (res.code === 200 || res.code === 202 || res.code === 0) {
                setIsUgcModalOpen(false);
                // 清空输入框，防止下次打开残留
                setUgcPlatform('bilibili');
                setUgcPlatformSpaceUrl('');
                alert(res.message);
                // 重新拉取大盘数据，你刚绑定的账号会立刻出现在列表中！
                fetchUserData();
            } else {
                alert(res.message || '节点绑定失败');
            }
        } catch (err: any) {
            alert(err.message || err.msg || '网络异常');
        } finally {
            setSubmitting(false);
        }
    };

    const fetchUserData = async (silent = false) => {
        try {
            if (!silent) setLoading(true);
            const res: any = await getUserInfoApi();
            if (res.code === 0 || res.code === 200) {
                setData(res.data);
            } else {
                if (!silent) setErrorMsg(res.message || '数据提取失败');
            }
        } catch (err: any) {
            if (!silent) setErrorMsg(err.message || err.msg || '[SYS_ERR] 无法连接到服务器矩阵');
        } finally {
            if (!silent) setLoading(false);
        }
    };

    useEffect(() => {
        fetchUserData();
    }, []);
    useEffect(() => {
        // 如果还没加载出数据，或者该用户没有绑定任何平台，则雷达休眠
        if (!data || !data.ugc_accounts || data.ugc_accounts.length === 0) return;
        // 提取当前用户所有的平台名称 (例如 ['bilibili', 'douyin'])
        const platforms = data.ugc_accounts.map(acc => acc.platform);

        console.log('[SYS] 心跳雷达启动，开始自动同步以下平台的鲜活数据:', platforms);
        const timer = setInterval(() => {
            platforms.forEach(async (platform) => {
                try {
                    const res: any = await getUgcBindResultApi(platform);
                    if (res.code === 0 || res.code === 200) {
                        const syncedData = res.data;
                        // 热更新：通过 setState 的函数式更新，精准修改 data 中对应平台的数据
                        // 这样做的好处是绝对不会触发 loading 白屏，数据会像魔术一样在页面上自己变化
                        setData(prevData => {
                            if (!prevData) return prevData;
                            return {
                                ...prevData,
                                ugc_accounts: prevData.ugc_accounts.map(acc => {
                                    if (acc.platform === platform) {
                                        return {
                                            ...acc,
                                            auth_status: syncedData.status, // 后端的 status 映射回前端的 auth_status
                                            nickname: syncedData.nickname,
                                            fans_count: syncedData.fans_count,
                                            bound_at: syncedData.bound_at,
                                        };
                                    }
                                    return acc;
                                })
                            };
                });
            }
        } catch (error) {
                    // 忽略单次网络波动，等待下一次心跳
                }
            });
        }, 1800000);
        return () => clearInterval(timer);
    }, [data?.ugc_accounts?.length]);

    // 探测用户的标签是否为空，如果为空，主动弹出标签选择矩阵 (场景一)
    useEffect(() => {
        if (!data) return;

        let currentTags: string[] = [];
        if (data.profile.tags) {
            try { currentTags = JSON.parse(data.profile.tags); } catch(e) {}
        }

        if (currentTags.length === 0 && !sessionStorage.getItem('tag_intercepted')) {
            setIsFirstLoginIntercept(true);
            setIsTagModalOpen(true);
            sessionStorage.setItem('tag_intercepted', 'true');
        }
    }, [data]);

    // 当标签弹窗打开时，去后端请求字典树
    useEffect(() => {
        if (isTagModalOpen && tagTree.length === 0) {
            const fetchTree = async () => {
                try {
                    const res: any = await getTagTreeApi();
                    if (res.code === 0 || res.code === 200) {
                        setTagTree(res.data.tree || []);
                    }
                } catch (e) { console.error("标签字典库加载失败", e); }
            };
            fetchTree();
        }
    }, [isTagModalOpen]);

    if (loading && !data) return <div className="p-10 font-mono text-cyan-500 animate-pulse">$&gt; Scanning Database...</div>;
    if (errorMsg && !data) return <div className="p-10 font-mono text-red-500">[ERR_CRITICAL]: {errorMsg}</div>;
    if (!data) return null;

    // 这里的解构现在完美生效了
    const { base_info, profile, ugc_accounts } = data;
    const isKol = base_info.role === 1;
    const themeColor = isKol ? 'cyan' : 'purple';

    // 日期格式化现在也被使用了
    const formatDate = (dateString: string) => dateString.substring(0, 10);

    let parsedTags: string[] = [];
    if (profile.tags) {
        try { parsedTags = JSON.parse(profile.tags); } catch (e) {}
    }

    // --- 交互处理逻辑 ---

    // 处理标签的点选/反选
    const toggleTag = (tagName: string) => {
        setSelectedTags(prev => {
            if (prev.includes(tagName)) {
                return prev.filter(t => t !== tagName); // 取消选择
            } else {
                if (prev.length >= 6) {
                    alert('[SYS_WARN] 载荷超限：最多只能配置 6 个领域节点！');
                    return prev;
                }
                return [...prev, tagName]; // 增加选择
            }
        });
    };

    // 提交标签更新 (场景一 & 场景二)
    const handleTagSubmit = async () => {
        setSubmitting(true);
        try {
            const res: any = await updateUserTagsApi(selectedTags);
            if (res.code === 0 || res.code === 200) {
                setIsTagModalOpen(false);
                fetchUserData(); // 刷新大盘
            } else {
                alert(res.message || '更新失败');
            }
        } catch (err: any) { alert(err.message || '网络异常'); }
        finally { setSubmitting(false); }
    };

    // 主动点击修改标签 (场景二)
    const openTagModal = () => {
        setIsFirstLoginIntercept(false);
        // 回显已有的标签
        let currentTags: string[] = [];
        if (data?.profile.tags) {
            try { currentTags = JSON.parse(data.profile.tags); } catch(e) {}
        }
        setSelectedTags(currentTags);
        setIsTagModalOpen(true);
    };

    const openProfileModal = () => {
        // setEditAvatar(profile.avatar_url || '');
        if (isKol) {
            setEditName(profile.real_name || '');
            setEditQuote(profile.base_quote || '');
        } else {
            setEditName(profile.company_name || '');
        }
        setIsProfileModalOpen(true);
    };

    const handlePwdSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (newPassword !== confirmPassword) {
            alert('[SYS_ERR] 两次输入的新密码不一致，请重新核对。');
            return; // 直接 return，不触发后端网络请求
        }
        setSubmitting(true);
        try {
            const res: any = await resetPasswordApi({ old_password: oldPassword, new_password: newPassword });
            if (res.code === 0) {
                alert('密码修改成功，安全协议要求重新进行身份验证。');
                localStorage.removeItem('access_token');
                navigate('/login?role=' + base_info.role);
            } else {
                alert(res.message || '密码修改失败');
            }
        } catch (err: any) {
            alert(err.message || err.msg || '网络异常');
        } finally {
            setSubmitting(false);
        }
    };

    const handleProfileSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setSubmitting(true);
        try {
            let res: any;
            if (isKol) {
                res = await updateKolProfileApi({
                    real_name: editName,
                    base_quote: Number(editQuote) || 0
                });
            } else {
                res = await updateBrandProfileApi({
                    company_name: editName
                });
            }

            if (res.code === 0 || res.code === 200) {
                setIsProfileModalOpen(false);
                fetchUserData(); // 成功后刷新背景大盘数据
            } else {
                alert(res.message || '更新失败');
            }
        } catch (err: any) {
            alert(err.message || err.msg || '网络异常');
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <div className="p-6 md:p-10 max-w-6xl mx-auto min-h-[calc(100vh-64px)] text-slate-300 relative">

            {/* 1. 顶部身份牌 */}
            <div className="mb-8 border-b border-slate-800 pb-6 flex items-end justify-between">
                <div className="flex items-center gap-6">
                    {/*  隐藏的文件选择器  */}
                    <input
                        type="file"
                        accept=".jpg,.jpeg,.png"
                        ref={fileInputRef}
                        onChange={handleAvatarChange}
                        className="hidden"
                    />

                    {/*  炫酷的交互式头像容器  */}
                    <div
                        onClick={() => !isUploadingAvatar && fileInputRef.current?.click()}
                        className={`relative w-20 h-20 rounded-lg bg-slate-900 border border-${themeColor}-500/50 flex items-center justify-center shadow-[0_0_20px_-5px_rgba(var(--tw-colors-${themeColor}-500),0.4)] group overflow-hidden cursor-pointer`}
                    >
                        {/* 上传时的 Loading 蒙层 */}
                        {isUploadingAvatar ? (
                            <div className="absolute inset-0 z-10 bg-black/70 flex items-center justify-center backdrop-blur-sm">
                                <span className={`text-${themeColor}-400 text-[10px] font-mono animate-pulse tracking-widest`}>UPLOADING</span>
                            </div>
                        ) : (
                            <div className="absolute inset-0 z-10 bg-black/60 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity duration-300">
                                <span className={`text-${themeColor}-400 text-xs font-mono tracking-widest`}>EDIT()</span>
                            </div>
                        )}

                        {/* 实际渲染的头像或占位符 */}
                        {profile.avatar_url && profile.avatar_url !== "string" ? (
                            <img src={profile.avatar_url} alt="Avatar" className="w-full h-full rounded-lg object-cover" />
                        ) : (
                            <span className={`text-${themeColor}-400 text-3xl font-mono`}>{base_info.username.charAt(0)}</span>
                        )}
                    </div> {/* 👈 极客提示：刚才就是这里少了一个结束标签 */}

                    {/* 用户名与状态标签 */}
                    <div>
                        <h1 className="text-3xl font-bold font-mono text-slate-100 tracking-tight">{base_info.username}</h1>
                        <div className="flex gap-3 mt-2">
              <span className={`px-2 py-0.5 border border-${themeColor}-500/50 text-${themeColor}-400 text-xs font-mono rounded bg-${themeColor}-950/30`}>
                角色: {isKol ? '红人' : '品牌方/代理商'}
              </span>
                            <span className={`px-2 py-0.5 border border-slate-600 text-slate-400 text-xs font-mono rounded bg-slate-800/50`}>
                账号状态: {base_info.status === 1 ? '正常' : '封禁中'}
              </span>
                        </div>
                    </div>

                </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">

                {/* 左侧：基础通讯录与操作区 */}
                <div className="lg:col-span-1 space-y-6">
                    {/* 基础通讯录 (formatDate 被使用) */}
                    <div className="bg-slate-900/40 border border-slate-800 rounded-lg p-5">
                        <h3 className="text-slate-500 font-mono text-xs mb-4 border-b border-slate-800 pb-2">基础信息</h3>
                        <div className="space-y-3 font-mono text-sm">
                            {/*<div><span className="text-slate-500">$&gt; ID_NUMBER:</span> <span className="text-slate-300 ml-2">{base_info.id}</span></div>*/}
                            <div><span className="text-slate-500">联系方式</span> <span className="text-slate-300 ml-2">{base_info.phone || 'N/A'}</span></div>
                            <div><span className="text-slate-500">联系邮箱</span> <span className="text-slate-300 ml-2 break-all">{base_info.email || 'N/A'}</span></div>
                            <div><span className="text-slate-500">入驻日期</span> <span className="text-slate-300 ml-2">{formatDate(base_info.created_at)}</span></div>
                        </div>
                    </div>

                    <div className="bg-slate-900/40 border border-slate-800 rounded-lg p-5 space-y-3">
                        <h3 className="text-slate-500 font-mono text-xs mb-4 border-b border-slate-800 pb-2">个人信息修改</h3>
                        <button onClick={openProfileModal} className={`w-full py-2.5 border border-slate-700 text-slate-400 font-mono text-xs rounded text-left px-4 cursor-pointer transition-all duration-300 hover:border-${themeColor}-400 hover:text-${themeColor}-400 hover:bg-${themeColor}-950/20 hover:shadow-[0_0_15px_rgba(0,0,0,0.3)] active:scale-[0.98] group flex justify-between items-center`}>
                            <span>拓展资料修改</span>
                            <span className={`opacity-0 group-hover:opacity-100 text-${themeColor}-400 animate-pulse`}>_</span>
                        </button>

                        <button onClick={() => setIsPwdModalOpen(true)} className={`w-full py-2.5 border border-slate-700 text-slate-400 font-mono text-xs rounded text-left px-4 cursor-pointer transition-all duration-300 hover:border-red-500/50 hover:text-red-400 hover:bg-red-950/20 hover:shadow-[0_0_15px_rgba(0,0,0,0.3)] active:scale-[0.98] group flex justify-between items-center`}>
                            <span>密码重置</span>
                            <span className="opacity-0 group-hover:opacity-100 text-red-500 animate-pulse">_</span>
                        </button>
                    </div>
                </div>

                {/* 右侧：扩展矩阵与UGC账号 */}
                <div className="lg:col-span-2 space-y-6">

                    <div className={`bg-slate-900/40 border border-${themeColor}-900/30 rounded-lg p-5 relative overflow-hidden`}>
                        <div className={`absolute top-0 left-0 w-1 h-full bg-${themeColor}-600/50`}></div>
                        <h3 className={`text-${themeColor}-500 font-mono text-xs mb-4 border-b border-slate-800 pb-2`}>
                            拓展资料({isKol ? '红人' : '品牌方/代理商'})
                        </h3>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                            {isKol ? (
                                <>
                                    <div><span className="block text-slate-500 text-xs font-mono mb-1">真实姓名</span><span className="text-slate-200 font-mono text-sm">{profile.real_name || 'UNDEFINED'}</span></div>
                                    <div><span className="block text-slate-500 text-xs font-mono mb-1">信用积分</span><span className="text-cyan-400 font-mono text-sm">{profile.credit_score} PT</span></div>
                                    <div><span className="block text-slate-500 text-xs font-mono mb-1">基础报价</span><span className="text-green-400 font-mono text-sm">¥ {profile.base_quote}</span></div>
                                    <div className="md:col-span-2 mt-2 pt-4 border-t border-slate-800/50">
                                        <div className="flex justify-between items-center mb-2">
                                            <span className="block text-slate-500 text-xs font-mono">领域标签</span>
                                            {/* 场景二：红人主动修改入口 */}
                                            <button onClick={openTagModal} className={`text-[10px] text-${themeColor}-400 hover:text-${themeColor}-300 hover:underline font-mono cursor-pointer`}>
                                                [ 修改标签 ]
                                            </button>
                                        </div>
                                        <div className="flex flex-wrap gap-2">
                                            {parsedTags.length > 0 ? parsedTags.map((tag, idx) => (
                                                <span key={idx} className="px-2 py-1 bg-cyan-950/50 border border-cyan-800 text-cyan-300 text-xs font-mono rounded shadow-[0_0_8px_rgba(34,211,238,0.1)]">
                                                    {tag}
                                                </span>
                                            )) : <span className="text-slate-600 text-sm">暂未添加标签</span>}
                                        </div>
                                    </div>
                                </>
                            ) : (
                                <>
                                    <div className="md:col-span-2"><span className="block text-slate-500 text-xs font-mono mb-1">公司名称</span><span className="text-slate-200 font-mono text-sm">{profile.company_name || 'UNDEFINED'}</span></div>
                                    <div className="md:col-span-2 mt-2 pt-4 border-t border-slate-800/50">
                                        <div className="flex justify-between items-center mb-2">
                                            <span className="block text-slate-500 text-xs font-mono">所属领域</span>
                                            {/* 场景二：品牌方主动修改入口 */}
                                            <button onClick={openTagModal} className={`text-[10px] text-${themeColor}-400 hover:text-${themeColor}-300 hover:underline font-mono cursor-pointer`}>
                                                [ 修改领域标签 ]
                                            </button>
                                        </div>
                                        <div className="flex flex-wrap gap-2">
                                            {parsedTags.length > 0 ? parsedTags.map((tag, idx) => (
                                                <span key={idx} className="px-2 py-1 bg-purple-950/50 border border-purple-800 text-purple-300 text-xs font-mono rounded shadow-[0_0_8px_rgba(168,85,247,0.1)]">
                                                    {tag}
                                                </span>
                                            )) : <span className="text-slate-600 text-sm">暂未添加领域</span>}
                                        </div>
                                    </div>
                                    {/*  品牌方机密资质展示与上传区  */}
                                    <div className="md:col-span-2 mt-2 pt-4 border-t border-slate-800/50">
                                        <span className="block text-slate-500 text-xs font-mono mb-3">营业执照</span>

                                        {/* 隐藏的执照选择器 (仅用于首次上传) */}
                                        <input type="file" accept=".jpg,.jpeg,.png" ref={licenseInputRef} onChange={handleLicenseChange} className="hidden" />

                                        <div className="flex items-end gap-4">
                                            {profile.license_url ? (
                                                // 🔒 状态 A：已上传 (模糊化 + 点击放大 + 右上角删除 X)
                                                <div
                                                    className="relative group w-32 h-24 rounded border border-purple-500/50 overflow-hidden bg-slate-900 shadow-[0_0_15px_rgba(168,85,247,0.2)]"
                                                >
                                                    {/* 模糊化的图片 (添加 blur-md 和亮度调低) */}
                                                    <img
                                                        src={profile.license_url}
                                                        alt="Secure License"
                                                        className="w-full h-full object-cover blur-md brightness-50 cursor-zoom-in transition-all duration-300 group-hover:brightness-75"
                                                        onClick={() => setIsPreviewModalOpen(true)} // 点击看大图
                                                    />

                                                    {/* 中间的机密标识字样 (穿透点击) */}
                              {/*                      <div className="absolute inset-0 pointer-events-none flex items-center justify-center">*/}
                              {/*<span className="text-purple-400/80 text-[10px] font-mono font-bold tracking-widest border border-purple-400/50 px-2 py-1 rounded bg-slate-950/50 backdrop-blur-sm">*/}
                              {/*</span>*/}
                              {/*                      </div>*/}

                                                    {/* 右上角的危险删除按钮 X */}
                                                    <button
                                                        onClick={(e) => {
                                                            e.stopPropagation(); // 阻止冒泡，防止触发看大图
                                                            setIsDeleteLicenseModalOpen(true);
                                                        }}
                                                        className="absolute top-1 right-1 w-6 h-6 bg-red-500/80 hover:bg-red-500 text-white rounded flex items-center justify-center opacity-0 group-hover:opacity-100 transition-all duration-300 z-20 shadow-[0_0_10px_rgba(239,68,68,0.5)] cursor-pointer"
                                                        title="销毁机密文件"
                                                    >
                                                        &times;
                                                    </button>
                                                </div>
                                            ) : (
                                                // 🔓 状态 B：未上传 (保持原样)
                                                <div
                                                    onClick={() => !isUploadingLicense && licenseInputRef.current?.click()}
                                                    className={`w-32 h-24 rounded border border-dashed flex flex-col items-center justify-center transition-all cursor-pointer group ${
                                                        isUploadingLicense
                                                            ? 'border-purple-500 bg-purple-900/20'
                                                            : 'border-slate-600 hover:border-purple-500 hover:bg-purple-950/30'
                                                    }`}
                                                >
                                                    {isUploadingLicense ? (
                                                        <span className="text-purple-400 text-[10px] font-mono animate-pulse">UPLOADING...</span>
                                                    ) : (
                                                        <>
                                                            <span className="text-xl leading-none text-slate-500 group-hover:text-purple-400 transition-colors">+</span>
                                                            <span className="text-[10px] mt-2 font-mono text-slate-500 group-hover:text-purple-400 transition-colors">UPLOAD_FILE</span>
                                                        </>
                                                    )}
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                </>
                            )}
                        </div>
                    </div>

                    {/* UGC账号展示区 (仅对红人开放) */}
                    {isKol && (
                        <div className="bg-slate-900/40 border border-slate-800 rounded-lg p-5">
                            <div className="flex justify-between items-center mb-4 border-b border-slate-800 pb-2">
                                <h3 className="text-slate-500 font-mono text-xs">已认证/绑定平台</h3>
                                <button
                                    onClick={() => setIsUgcModalOpen(true)}
                                    className={`px-3 py-1.5 border border-${themeColor}-500/40 text-${themeColor}-400 font-mono text-xs rounded bg-${themeColor}-950/10 hover:bg-${themeColor}-900/30 hover:border-${themeColor}-400 hover:text-${themeColor}-200 hover:shadow-[0_0_12px_currentColor] transition-all duration-300 cursor-pointer active:scale-95 flex items-center gap-1.5 group`}
                                >
                                    <span className="text-lg leading-none group-hover:animate-pulse">+</span>
                                    <span className="tracking-wide">绑定新平台账号</span>
                                </button>
                            </div>
                            {ugc_accounts.length === 0 ? (
                                <div className="text-center py-6 border border-dashed border-slate-700 rounded bg-slate-900/20">
                                    <p className="text-slate-500 font-mono text-xs">NO_NODES_DETECTED</p>
                                </div>
                            ) : (
                                <div className="space-y-3">
                                    {ugc_accounts.map((acc) => (
                                        <div key={acc.id} className="flex items-center justify-between p-3 border border-slate-800 rounded bg-slate-950/50 hover:border-slate-600 transition-colors">
                                            <div className="flex items-center gap-4">
                                                <div className={`w-8 h-8 rounded flex items-center justify-center font-bold text-xs ${
                                                    acc.auth_status === 0
                                                        ? 'bg-slate-900 text-slate-500 border border-slate-700 animate-pulse'
                                                        : 'bg-cyan-950/50 text-cyan-400 border border-cyan-800'
                                                }`}>
                                                    {acc.platform.substring(0,2).toUpperCase()}
                                                </div>
                                                {/* 动态渲染账号信息 */}
                                                <div>
                                                    {acc.auth_status === 0 ? (
                                                        <>
                                                            <div className="text-yellow-500 font-mono text-sm flex items-center gap-2">
                                                                <span>SYNCING_DATA_FROM_TARGET...</span>
                                                                <span className="w-2 h-2 rounded-full bg-yellow-500 animate-ping"></span>
                                                            </div>
                                                            <div className="text-slate-500 font-mono text-xs">等待数据抓取返回</div>
                                                        </>
                                                    ) : (
                                                        <>
                                                            <div className="text-slate-200 font-mono text-sm">{acc.nickname || 'UNKNOWN'}</div>
                                                            <div className="text-slate-500 font-mono text-xs">UID: {acc.platform_uid} | 平台: {acc.platform}</div>
                                                        </>
                                                    )}
                                                </div>
                                            </div>
                                            <div className="text-right">
                                                {acc.auth_status === 0 ? (
                                                    <div className="text-slate-600 font-mono text-[10px] animate-pulse">ESTABLISHING_LINK...</div>
                                                ) : (
                                                    <>
                                                        <div className="text-cyan-400 font-mono text-sm">粉丝数: {acc.fans_count.toLocaleString()}</div>
                                                        <div className="text-slate-600 font-mono text-[10px]">绑定日期: {formatDate(acc.bound_at)}</div>
                                                    </>
                                                )}
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>

            {/* --- 全息密码修改弹窗 --- */}
            {isPwdModalOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
                    <div className="bg-slate-950 border border-slate-700 shadow-2xl rounded-lg w-full max-w-md p-6 relative">
                        {/* 优化：关闭时清空所有输入框状态 */}
                        <button
                            onClick={() => {
                                setIsPwdModalOpen(false);
                                setOldPassword('');
                                setNewPassword('');
                                setConfirmPassword('');
                            }}
                            className="absolute top-4 right-4 text-slate-500 hover:text-red-400 font-mono text-xl"
                        >
                            &times;
                        </button>
                        <h2 className="text-lg font-mono text-slate-200 border-b border-slate-800 pb-2 mb-6">密码修改</h2>
                        <form onSubmit={handlePwdSubmit} className="space-y-4">
                            <div>
                                <label className="block text-slate-500 text-xs font-mono mb-2">旧密码</label>
                                <input type="password" required value={oldPassword} onChange={e => setOldPassword(e.target.value)} className="w-full bg-slate-900 border border-slate-700 focus:border-cyan-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none" />
                            </div>
                            <div>
                                <label className="block text-slate-500 text-xs font-mono mb-2">新密码</label>
                                <input type="password" required value={newPassword} onChange={e => setNewPassword(e.target.value)} className="w-full bg-slate-900 border border-slate-700 focus:border-red-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none" />
                            </div>
                            <div>
                                <label className="block text-slate-500 text-xs font-mono mb-2">确认新密码</label>
                                <input type="password" required value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} className="w-full bg-slate-900 border border-slate-700 focus:border-red-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none" />
                            </div>
                            <button disabled={submitting} type="submit" className="w-full mt-6 py-2 bg-slate-800 border border-slate-600 text-slate-300 hover:text-cyan-400 hover:border-cyan-500 font-mono rounded transition-colors">
                                {submitting ? '重置中...' : '提交'}
                            </button>
                        </form>
                    </div>
                </div>
            )}

            {/* --- 全息资料修改弹窗 --- */}
            {isProfileModalOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
                    <div className={`bg-slate-950 border border-${themeColor}-900 shadow-2xl shadow-${themeColor}-900/20 rounded-lg w-full max-w-md p-6 relative`}>
                        <button onClick={() => setIsProfileModalOpen(false)} className="absolute top-4 right-4 text-slate-500 hover:text-red-400 font-mono text-xl">&times;</button>
                        <h2 className={`text-lg font-mono text-${themeColor}-400 border-b border-slate-800 pb-2 mb-6`}>拓展资料修改</h2>
                        <form onSubmit={handleProfileSubmit} className="space-y-4">
                            {/*<div>*/}
                            {/*    <label className="block text-slate-500 text-xs font-mono mb-2">avatar_url (Image Link)</label>*/}
                            {/*    <input type="text" value={editAvatar} onChange={e => setEditAvatar(e.target.value)} className={`w-full bg-slate-900 border border-slate-700 focus:border-${themeColor}-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none`} />*/}
                            {/*</div>*/}

                            {isKol ? (
                                <>
                                    <div>
                                        <label className="block text-slate-500 text-xs font-mono mb-2">真实姓名</label>
                                        <input type="text" value={editName} onChange={e => setEditName(e.target.value)} className="w-full bg-slate-900 border border-slate-700 focus:border-cyan-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none" />
                                    </div>
                                    <div>
                                        <label className="block text-slate-500 text-xs font-mono mb-2">基础报价 (人民币¥)</label>
                                        <input type="number" value={editQuote} onChange={e => setEditQuote(e.target.value === '' ? '' : Number(e.target.value))} className="w-full bg-slate-900 border border-slate-700 focus:border-cyan-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none" />
                                    </div>
                                    {/*<div>*/}
                                    {/*    <label className="block text-slate-500 text-xs font-mono mb-2">领域标签 (用逗号分隔)</label>*/}
                                    {/*    <input type="text" placeholder="e.g. 游戏, 主播, 美食" value={editTags} onChange={e => setEditTags(e.target.value)} className="w-full bg-slate-900 border border-slate-700 focus:border-cyan-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none" />*/}
                                    {/*</div>*/}
                                </>
                            ) : (
                                <>
                                    <div>
                                        <label className="block text-slate-500 text-xs font-mono mb-2">公司名称</label>
                                        <input type="text" value={editName} onChange={e => setEditName(e.target.value)} className="w-full bg-slate-900 border border-slate-700 focus:border-purple-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none" />
                                    </div>
                                    {/*<div>*/}
                                    {/*    <label className="block text-slate-500 text-xs font-mono mb-2">所属行业</label>*/}
                                    {/*    <input type="text" value={editIndustry} onChange={e => setEditIndustry(e.target.value)} className="w-full bg-slate-900 border border-slate-700 focus:border-purple-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none" />*/}
                                    {/*</div>*/}
                                </>
                            )}

                            <button disabled={submitting} type="submit" className={`w-full mt-6 py-2 bg-slate-800 border border-slate-600 text-slate-300 hover:text-${themeColor}-400 hover:border-${themeColor}-500 font-mono rounded transition-colors`}>
                                {submitting ? '提交中...' : '提交'}
                            </button>
                        </form>
                    </div>
                </div>
            )}

            {/* --- 全息 UGC 节点绑定弹窗 --- */}
            {isUgcModalOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
                    <div className={`bg-slate-950 border border-cyan-900 shadow-2xl shadow-cyan-900/20 rounded-lg w-full max-w-md p-6 relative`}>
                        <button
                            onClick={() => {
                                setIsUgcModalOpen(false);
                                setUgcPlatform('bilibili');
                                setUgcPlatformSpaceUrl('');
                            }}
                            className="absolute top-4 right-4 text-slate-500 hover:text-red-400 font-mono text-xl"
                        >
                            &times;
                        </button>
                        <h2 className="text-lg font-mono text-cyan-400 border-b border-slate-800 pb-2 mb-6">绑定三方平台</h2>

                        <form onSubmit={handleUgcSubmit} className="space-y-4">
                            <div>
                                <label className="block text-slate-500 text-xs font-mono mb-2">平台</label>
                                <select
                                    value={ugcPlatform}
                                    onChange={e => setUgcPlatform(e.target.value)}
                                    className="w-full bg-slate-900 border border-slate-700 focus:border-cyan-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none appearance-none"
                                >
                                    <option value="bilibili">Bilibili (哔哩哔哩)</option>
                                    <option value="douyin">Douyin (抖音)</option>
                                    <option value="tiktok">TikTok</option>
                                </select>
                            </div>

                            {/* 2. 个人主页链接输入 */}
                            <div>
                                <label className="block text-slate-500 text-xs font-mono mb-2">platform_space_url (个人主页链接)</label>
                                <input
                                    type="url"  /* 极客细节：使用 url 类型，移动端会自动调出带 .com 的键盘 */
                                    required
                                    placeholder="e.g. https://space.bilibili.com/xxx"
                                    value={ugcPlatformSpaceUrl}
                                    onChange={e => setUgcPlatformSpaceUrl(e.target.value)}
                                    className="w-full bg-slate-900 border border-slate-700 focus:border-cyan-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none"
                                />
                            </div>

                            <button
                                disabled={submitting}
                                type="submit"
                                className="w-full mt-6 py-2 bg-slate-800 border border-slate-600 text-slate-300 hover:text-cyan-400 hover:border-cyan-500 font-mono rounded transition-colors"
                            >
                                {submitting ? '提交中...' : '提交'}
                            </button>
                        </form>
                    </div>
                </div>
            )}

            {/* --- 全息大图预览弹窗 (Preview Modal) --- */}
            {isPreviewModalOpen && profile.license_url && (
                <div
                    className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 backdrop-blur-md cursor-zoom-out p-4 md:p-10"
                    onClick={() => setIsPreviewModalOpen(false)} // 点击背景关闭
                >
                    <div className="relative max-w-full max-h-full">
                        <span className="absolute -top-10 right-0 text-slate-400 font-mono text-sm">[ CLICK_ANYWHERE_TO_CLOSE ]</span>
                        <img
                            src={profile.license_url}
                            alt="Full Resolution License"
                            className="max-w-full max-h-[85vh] object-contain border border-purple-500/30 shadow-[0_0_50px_rgba(168,85,247,0.2)] rounded-lg"
                        />
                    </div>
                </div>
            )}

            {/* --- 危险级销毁确认弹窗 (Delete Confirm Modal) --- */}
            {isDeleteLicenseModalOpen && (
                <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/80 backdrop-blur-sm">
                    <div className="bg-slate-950 border border-red-900 shadow-2xl shadow-red-900/20 rounded-lg w-full max-w-md p-6 relative">
                        <button
                            onClick={() => {
                                setIsDeleteLicenseModalOpen(false);
                                setLicenseDeletePassword('');
                            }}
                            className="absolute top-4 right-4 text-slate-500 hover:text-red-400 font-mono text-xl"
                        >
                            &times;
                        </button>
                        <h2 className="text-lg font-mono text-red-500 border-b border-slate-800 pb-2 mb-4">
                            [CRITICAL] DESTROY_SECURE_ASSET
                        </h2>
                        <p className="text-slate-400 text-xs font-mono mb-6 leading-relaxed">
                            警告：您即将从服务器物理擦除企业机密资质。此操作不可逆。为确保实体身份安全，请输入当前连接的登录密码以覆盖保护协议。
                        </p>

                        <form onSubmit={handleDeleteLicenseSubmit} className="space-y-4">
                            <div>
                                <label className="block text-slate-500 text-xs font-mono mb-2">AUTH_PASSWORD</label>
                                <input
                                    type="password"
                                    required
                                    value={licenseDeletePassword}
                                    onChange={e => setLicenseDeletePassword(e.target.value)}
                                    className="w-full bg-slate-900 border border-slate-700 focus:border-red-500 rounded px-3 py-2 text-slate-200 font-mono text-sm outline-none"
                                    placeholder="Enter login password..."
                                />
                            </div>

                            <div className="flex gap-4 mt-6">
                                <button
                                    type="button"
                                    onClick={() => setIsDeleteLicenseModalOpen(false)}
                                    className="flex-1 py-2 bg-slate-800 border border-slate-700 text-slate-400 hover:text-slate-200 font-mono rounded transition-colors cursor-pointer"
                                >
                                    [ ABORT ]
                                </button>
                                <button
                                    disabled={isDeletingLicense}
                                    type="submit"
                                    className={`flex-1 py-2 bg-red-950/30 border border-red-900 text-red-500 font-mono rounded font-bold transition-all duration-300 cursor-pointer 
                    ${isDeletingLicense ? 'opacity-50 cursor-not-allowed' : 'hover:bg-red-900/50 hover:text-red-400 hover:shadow-[0_0_15px_rgba(239,68,68,0.4)] active:scale-95'}`}
                                >
                                    {isDeletingLicense ? 'PURGING...' : '[ CONFIRM_PURGE ]'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* 🚀 独立领域标签选择弹窗 (Tag Matrix) */}
            {isTagModalOpen && (
                <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/80 backdrop-blur-md p-4">
                    <div className={`bg-slate-950 border border-${themeColor}-900 shadow-2xl shadow-${themeColor}-900/20 rounded-lg w-full max-w-2xl max-h-[85vh] flex flex-col relative overflow-hidden`}>

                        {/* Header */}
                        <div className="p-5 border-b border-slate-800 flex justify-between items-center bg-slate-900/50 shrink-0">
                            <div>
                                <h2 className={`text-lg font-mono text-${themeColor}-400`}>$&gt; ./configure_domain_nodes.sh</h2>
                                <p className="text-slate-500 text-xs mt-1">请选择您的核心领域标签 (已选择 {selectedTags.length}/6)</p>
                            </div>
                            {!isFirstLoginIntercept && (
                                <button onClick={() => setIsTagModalOpen(false)} className="text-slate-500 hover:text-red-400 font-mono text-xl">&times;</button>
                            )}
                        </div>

                        {/* Body: 标签树渲染 */}
                        <div className="flex-1 overflow-y-auto p-6 space-y-6 custom-scrollbar">
                            {tagTree.length === 0 ? (
                                <div className={`text-${themeColor}-500 font-mono text-center py-10 animate-pulse`}>LOADING_DICTIONARY...</div>
                            ) : (
                                tagTree.map(parent => (
                                    <div key={parent.id} className="space-y-3">
                                        <h3 className="text-slate-400 font-mono text-sm border-b border-slate-800/50 pb-1">## {parent.name}</h3>
                                        <div className="flex flex-wrap gap-2.5">
                                            {parent.children?.map(child => {
                                                const isSelected = selectedTags.includes(child.name);
                                                return (
                                                    <button
                                                        key={child.id}
                                                        onClick={() => toggleTag(child.name)}
                                                        className={`px-3 py-1.5 text-xs font-mono rounded border transition-all duration-300 cursor-pointer active:scale-95
                              ${isSelected
                                                            ? `bg-${themeColor}-900/40 border-${themeColor}-400 text-${themeColor}-300 shadow-[0_0_10px_rgba(var(--tw-colors-${themeColor}-500),0.3)]`
                                                            : 'bg-slate-900 border-slate-700 text-slate-400 hover:border-slate-500 hover:bg-slate-800'}
                            `}
                                                    >
                                                        {isSelected ? '■' : '□'} {child.name}
                                                    </button>
                                                );
                                            })}
                                        </div>
                                    </div>
                                ))
                            )}
                        </div>

                        {/* Footer */}
                        <div className="p-5 border-t border-slate-800 bg-slate-900/80 flex gap-4 shrink-0">
                            {isFirstLoginIntercept && (
                                <button
                                    onClick={() => setIsTagModalOpen(false)}
                                    className="px-6 py-2 bg-slate-800 border border-slate-700 text-slate-400 hover:text-slate-200 font-mono rounded transition-colors cursor-pointer"
                                >
                                    [ SKIP_FOR_NOW ]
                                </button>
                            )}
                            <button
                                onClick={handleTagSubmit}
                                disabled={submitting || selectedTags.length === 0}
                                className={`flex-1 py-2 bg-slate-900 border border-slate-600 text-slate-300 font-mono rounded font-bold tracking-widest transition-all duration-300 cursor-pointer 
                  ${(submitting || selectedTags.length === 0) ? 'opacity-50 cursor-not-allowed' : `hover:border-${themeColor}-400 hover:text-${themeColor}-400 hover:bg-${themeColor}-950/30 hover:shadow-[0_0_20px_currentColor] active:scale-[0.98]`}`}
                            >
                                {submitting ? 'COMMITTING...' : '[ WRITE_TO_MATRIX ]'}
                            </button>
                        </div>

                    </div>
                </div>
            )}

        </div>
    );
}