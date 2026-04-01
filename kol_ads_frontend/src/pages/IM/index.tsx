import { useEffect, useState, useRef } from 'react';
import { useLocation } from 'react-router-dom';
import { getSessionListApi, getHistoryMessagesApi, SessionItem, IMMessage,clearUnreadApi } from '../../api/match_engine/im.ts';
import { useIMWebSocket } from '../../hooks/useIMWebSocket.ts';
import { getUserInfoApi } from '../../api/user_center/user.ts';


// 前端提前计算与后端 100% 一致的全局唯一 SessionID
const generateSessionId = (uid1: string, uid2: string) => {
    const u1 = BigInt(uid1);
    const u2 = BigInt(uid2);
    return u1 < u2 ? `${u1}_${u2}` : `${u2}_${u1}`;
};

export default function IMTerminal() {
    const [myUserId, setMyUserId] = useState<string | null>(null);

    const location = useLocation();

    // 🚀 挂载全息长连接引擎
    const { isConnected, transmitMessage, incomingMessage, readReceipt } = useIMWebSocket();

    // 会话状态
    const [sessions, setSessions] = useState<SessionItem[]>([]);
    const [activeSession, setActiveSession] = useState<SessionItem | null>(null);

    // 消息状态
    const [messages, setMessages] = useState<IMMessage[]>([]);
    const [cursor, setCursor] = useState<string>("0");
    const [hasMore, setHasMore] = useState<boolean>(false);
    const [isLoadingHistory, setIsLoadingHistory] = useState(false);

    // 输入状态
    const [inputText, setInputText] = useState('');

    // 滚动锚点
    const messagesEndRef = useRef<HTMLDivElement>(null);
    // 消息防重物理护盾（记录已经上屏的 WS 消息 ID）
    const processedMsgIds = useRef<Set<string>>(new Set());

    // 👇 新增：初始化免疫护盾
    const hasInitialized = useRef(false);

    // 🚀 处理 WebSocket 推送过来的实时消息
    useEffect(() => {
        if (!incomingMessage) return;

        // 防弹衣：如果这条消息的 ID 已经被处理过了，直接物理拦截！绝对不渲染第二次！
        if (processedMsgIds.current.has(incomingMessage.msg_id)) {
            return;
        }
        // 登记这条新消息，加入已处理名册
        processedMsgIds.current.add(incomingMessage.msg_id);

        const senderId = incomingMessage.sender_id;

        // 场景 A：如果消息的发件人就是当前正在聊天的人
        if (activeSession && senderId === activeSession.target_user_id) {
            setMessages(prev => [...prev, {
                msg_id: incomingMessage.msg_id,
                session_id: activeSession.session_id,
                sender_id: senderId,
                receiver_id: myUserId!,
                content: incomingMessage.content,
                msg_type: incomingMessage.msg_type,
                send_time: incomingMessage.send_time,
                status: 0
            }]);
            setTimeout(() => messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }), 100);

            setSessions(prev => prev.map(s =>
                s.target_user_id === senderId
                    ? { ...s, latest_msg: incomingMessage.content, updated_at: incomingMessage.send_time }
                    : s
            ));
            clearUnreadApi(senderId).catch(e => console.error('[IM_SYS] 实时通讯红点压制失败', e));
        } else {
            setSessions(prev => {
                const exists = prev.some(s => s.target_user_id === senderId);
                if (exists) {
                    return prev.map(s => s.target_user_id === senderId
                        ? { ...s, unread_count: s.unread_count + 1, latest_msg: incomingMessage.content, updated_at: incomingMessage.send_time }
                        : s
                    );
                } else {
                    getSessionListApi().then((res: any) => {
                        if (res.code === 0) setSessions(res.data.sessions || []);
                    });
                    return prev;
                }
            });
        }
    }, [incomingMessage, activeSession, myUserId]);

    // 1. 初始化获取自身 ID 和 会话列表
    useEffect(() => {
        // 极客防御：如果已经初始化过，直接拦截，免疫 React 18 双重渲染
        if (hasInitialized.current) return;
        hasInitialized.current = true;
        const initData = async () => {
            try {
                const userRes: any = await getUserInfoApi();
                let currentUserId = null;
                if (userRes.code === 0) {
                    currentUserId = String(userRes.data.base_info.id);
                    setMyUserId(currentUserId);
                }

                const sessionRes: any = await getSessionListApi();
                let loadedSessions = sessionRes.data?.sessions || [];

                // 🚀 核心逻辑：嗅探是否是从匹配大厅(Match)携参跳转过来的
                const targetUserFromState = location.state?.targetUser;

                if (targetUserFromState) {
                    console.log('[IM_SYS] 检测到外部会话注入请求:', targetUserFromState);

                    // 检查该用户是否已经在左侧会话列表中
                    const existingSession = loadedSessions.find((s: SessionItem) => s.target_user_id === targetUserFromState.target_user_id);

                    let sessionToActivate: SessionItem;

                    if (existingSession) {
                        sessionToActivate = existingSession;
                    } else {
                        const realSessionId = generateSessionId(currentUserId!, targetUserFromState.target_user_id);
                        // 如果是全新联系人，在前端构建一个“虚拟幽灵会话”并推入列表顶部
                        sessionToActivate = {
                            session_id: realSessionId, // 临时虚拟 ID
                            target_user_id: targetUserFromState.target_user_id,
                            target_user_name: targetUserFromState.target_user_name,
                            target_avatar: targetUserFromState.target_avatar,
                            latest_msg: '[ System: 开始建立加密通讯通道 ]',
                            unread_count: 0,
                            updated_at: Date.now() / 1000
                        };
                        loadedSessions = [sessionToActivate, ...loadedSessions];
                    }

                    // 强制更新左侧列表并激活该会话
                    setSessions(loadedSessions);
                    setActiveSession(sessionToActivate);

                    // 立即拉取历史记录 (如果是虚拟会话，后端会返回空，这是正常的)
                    setMessages([]);
                    setCursor("0");
                    setHasMore(false);
                    fetchHistory(sessionToActivate.target_user_id, "0");

                    // 💡 极客细节：清空路由状态，防止用户刷新页面时反复触发这段逻辑
                    window.history.replaceState({}, document.title);

                } else {
                    // 如果是正常点击菜单进来的，只渲染列表，不选中任何会话
                    setSessions(loadedSessions);
                }

            } catch (e) {
                console.error('IM矩阵初始化失败', e);
            }
        };
        initData();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [location.state]);

    // 监听对方的已读回执信令
    useEffect(() => {
        if (!readReceipt) return;

        // 如果信令对应的 session_id 就是我当前打开的聊天窗口
        if (activeSession && readReceipt.session_id === activeSession.session_id) {
            // 瞬间遍历当前面板的所有消息
            setMessages(prev => prev.map(m =>
                // 如果这条消息是我发的 (ME)，且之前是未读状态 (0)，则瞬间全标为已读 (1)！
                (m.sender_id === myUserId && m.status === 0)
                    ? { ...m, status: 1 }
                    : m
            ));
        }
    }, [readReceipt, activeSession, myUserId]);

    // 2. 切换会话：重置状态并拉取首页历史记录
    const handleSelectSession = async (session: SessionItem) => {
        setActiveSession(session);
        setMessages([]);
        setCursor("0");
        setHasMore(false);

        // 乐观消除本地红点 (后端已经在 GET /history 接口里异步清除了)
        setSessions(prev => prev.map(s => s.session_id === session.session_id ? { ...s, unread_count: 0 } : s));

        // 通知后端物理清零底表红点
        if (session.unread_count > 0) {
            clearUnreadApi(session.target_user_id).catch(e => console.error('[IM_SYS] 切换会话时清空红点失败', e));
        }

        await fetchHistory(session.target_user_id, "0");
    };

    // 3. 核心游标加载器 (拉取历史记录)
    const fetchHistory = async (targetId: string, currentCursor: string) => {
        setIsLoadingHistory(true);
        try {
            const res: any = await getHistoryMessagesApi(targetId, currentCursor);
            if (res.code === 0) {
                const data = res.data;
                // 游标判断变为字符串
                setMessages(prev => currentCursor === "0" ? data.messages : [...data.messages, ...prev]);
                setCursor(data.next_cursor);
                setHasMore(data.has_more);
                if (currentCursor === "0") {
                    setTimeout(() => messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }), 100);
                }
            }
        } catch (e) {
            console.error('获取通信记录失败');
        } finally {
            setIsLoadingHistory(false);
        }
    };

    // 4. 发送消息 (占位逻辑，等待你接入 WebSocket 或 POST 接口)
    const handleSendMessage = (e: React.FormEvent) => {
        e.preventDefault();
        if (!inputText.trim() || !activeSession || !myUserId) return;

        // 乐观 UI 更新：先上屏
        const mockMsg: IMMessage = {
            msg_id: String(Date.now()), // 👈 ID 直接用 string
            session_id: activeSession.session_id,
            sender_id: myUserId,
            receiver_id: activeSession.target_user_id,
            content: inputText,
            msg_type: 1,
            send_time: Date.now(),
            status: 0
        };

        setMessages(prev => [...prev, mockMsg]);
        setInputText('');
        setTimeout(() => messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }), 100);

        transmitMessage({
            receiver_id: activeSession.target_user_id,
            msg_type: 1,
            content: mockMsg.content
        });
    };

    const formatTime = (ts: number) => new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

    return (
        <div className="p-6 max-w-7xl mx-auto h-[calc(100vh-64px)] flex gap-6 text-slate-300">

            {/* 左侧：通信频道雷达 (会话列表) */}
            <div className="w-80 bg-slate-900/50 border border-slate-800 rounded-lg flex flex-col overflow-hidden shadow-xl">
                <div className="p-4 border-b border-slate-800 bg-slate-900/80">
                    <h2 className="font-mono text-cyan-400 font-bold flex items-center gap-2">
                        <span className={`w-2 h-2 rounded-full ${isConnected ? 'bg-green-400 animate-pulse shadow-[0_0_8px_#4ade80]' : 'bg-red-500'}`}></span>
                        SECURE_CHANNELS
                    </h2>
                </div>
                <div className="flex-1 overflow-y-auto custom-scrollbar">
                    {sessions.length === 0 ? (
                        <div className="p-6 text-center text-slate-600 font-mono text-xs">NO_ACTIVE_CHANNELS</div>
                    ) : (
                        sessions.map(session => (
                            <div
                                key={session.session_id}
                                onClick={() => handleSelectSession(session)}
                                className={`p-4 border-b border-slate-800/50 flex items-center gap-3 cursor-pointer transition-colors ${activeSession?.session_id === session.session_id ? 'bg-cyan-950/30 border-l-2 border-l-cyan-400' : 'hover:bg-slate-800/50 border-l-2 border-l-transparent'}`}
                            >
                                <img src={session.target_avatar || '/default-avatar.png'} alt="avatar" className="w-10 h-10 rounded-full object-cover bg-slate-800 shrink-0" />
                                <div className="flex-1 min-w-0">
                                    <div className="flex justify-between items-center mb-1">
                                        <h3 className="font-bold text-slate-200 text-sm truncate">{session.target_user_name}</h3>
                                        {session.unread_count > 0 && (
                                            <span className="bg-red-500 text-white text-[10px] font-mono px-1.5 py-0.5 rounded-full shadow-[0_0_8px_rgba(239,68,68,0.5)]">
                        {session.unread_count}
                      </span>
                                        )}
                                    </div>
                                    <p className="text-slate-500 text-xs truncate font-mono">{session.latest_msg}</p>
                                </div>
                            </div>
                        ))
                    )}
                </div>
            </div>

            {/* 右侧：全息通信流 (聊天面板) */}
            <div className="flex-1 bg-slate-900/50 border border-slate-800 rounded-lg flex flex-col overflow-hidden shadow-xl relative">
                {activeSession ? (
                    <>
                        {/* Header */}
                        <div className="p-4 border-b border-slate-800 bg-slate-900/80 flex items-center gap-3 shadow-md z-10">
                            <span className="text-cyan-400 font-mono">$&gt; ENCRYPTED_LINK_TO:</span>
                            <span className="font-bold text-slate-200">{activeSession.target_user_name}</span>
                        </div>

                        {/* Message Stream */}
                        <div className="flex-1 overflow-y-auto p-6 space-y-6 custom-scrollbar flex flex-col">
                            {/* 游标加载器 */}
                            {hasMore && (
                                <div className="text-center pb-4">
                                    <button
                                        onClick={() => fetchHistory(activeSession.target_user_id, cursor)}
                                        disabled={isLoadingHistory}
                                        className="text-[10px] font-mono text-cyan-500/70 hover:text-cyan-400 px-3 py-1 border border-cyan-900/50 rounded bg-cyan-950/20 transition-colors"
                                    >
                                        {isLoadingHistory ? 'DECRYPTING...' : '[ FETCH_OLDER_PACKETS ]'}
                                    </button>
                                </div>
                            )}

                            {/* 消息气泡渲染 */}
                            {messages.map(msg => {
                                const isMe = msg.sender_id === myUserId;
                                return (
                                    <div key={msg.msg_id} className={`flex flex-col ${isMe ? 'items-end' : 'items-start'} mb-4 w-full`}>
                                        <div className="text-slate-600 text-[10px] font-mono mb-1 mx-1">
                                            {isMe ? 'ME' : 'TARGET'} | {formatTime(msg.send_time)}
                                        </div>
                                        {/* 🚀 修复1：将最大宽度限制 (max-w-[80%]) 移到这个外层容器上 */}
                                        <div className={`flex items-end gap-2 max-w-[80%] ${isMe ? 'flex-row-reverse' : 'flex-row'}`}>
                                            {/* 🚀 修复2：气泡本体移除 max-w-[70%]，新增三个极其关键的文字排版属性 */}
                                            {/* break-words: 允许长单词或连续字母在边界处换行 */}
                                            {/* whitespace-pre-wrap: 保留用户输入的空格和回车换行 */}
                                            {/* min-w-0: 打破 Flex 子元素的最小宽度限制，允许其向内收缩 */}
                                            <div className={`p-3 rounded-lg text-sm leading-relaxed shadow-md break-words whitespace-pre-wrap min-w-0 ${
                                                isMe
                                                    ? 'bg-cyan-900/40 border border-cyan-700/50 text-cyan-50 rounded-tr-none'
                                                    : 'bg-slate-800/80 border border-slate-700/50 text-slate-200 rounded-tl-none'
                                            }`}>
                                                {msg.content}
                                            </div>
                                            {/* 企微同款已读/未读指示器 (保持不变，但 flex-shrink-0 确保它不会被文字挤扁) */}
                                            {isMe && (
                                                <div className="flex-shrink-0 mb-1" title={msg.status === 1 ? "已读" : "未读"}>
                                                    {msg.status === 1 ? (
                                                        <svg className="w-4 h-4 text-cyan-500" fill="currentColor" viewBox="0 0 20 20">
                                                            <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                                                        </svg>
                                                    ) : (
                                                        <svg className="w-4 h-4 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <circle cx="12" cy="12" r="9" strokeWidth="2" />
                                                        </svg>
                                                    )}
                                                </div>
                                            )}

                                        </div>
                                    </div>
                                );
                            })}
                            {/* 底部锚点 */}
                            <div ref={messagesEndRef} />
                        </div>

                        {/* Input Terminal */}
                        <div className="p-4 border-t border-slate-800 bg-slate-900/80">
                            <form onSubmit={handleSendMessage} className="flex gap-4">
                                <input
                                    type="text"
                                    value={inputText}
                                    onChange={e => setInputText(e.target.value)}
                                    placeholder="Type message here..."
                                    className="flex-1 bg-slate-950 border border-slate-700 focus:border-cyan-500 rounded px-4 py-3 text-slate-200 font-mono text-sm outline-none transition-colors"
                                />
                                <button
                                    type="submit"
                                    disabled={!inputText.trim()}
                                    className="px-8 py-3 bg-cyan-950/40 border border-cyan-600 text-cyan-400 font-bold font-mono rounded hover:bg-cyan-900/60 hover:shadow-[0_0_15px_rgba(34,211,238,0.4)] transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    [ TRANSMIT ]
                                </button>
                            </form>
                        </div>
                    </>
                ) : (
                    <div className="flex-1 flex flex-col items-center justify-center text-slate-600 font-mono">
                        <div className="text-4xl mb-4 opacity-50">⎔</div>
                        <div>AWAITING_CONNECTION_ESTABLISHMENT</div>
                        <div className="text-[10px] mt-2">Select a channel from the radar to decrypt stream.</div>
                    </div>
                )}
            </div>

        </div>
    );
}