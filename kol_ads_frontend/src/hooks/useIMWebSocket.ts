// src/hooks/useIMWebSocket.ts
import { useEffect, useRef, useState, useCallback } from 'react';
import { WsCmdType, WsMsgReq, WsMsgPush, WsReadReceiptPush } from '../api/match_engine/im_protocol';

export const useIMWebSocket = () => {
    const wsRef = useRef<WebSocket | null>(null);
    const heartbeatTimerRef = useRef<NodeJS.Timeout | null>(null);
    const reconnectTimerRef = useRef<NodeJS.Timeout | null>(null);

    // 引擎状态
    const [isConnected, setIsConnected] = useState(false);
    // 收到的最新一条消息，供外部组件监听
    const [incomingMessage, setIncomingMessage] = useState<WsMsgPush | null>(null);
    // 专门用来存放收到的已读回执信令
    const [readReceipt, setReadReceipt] = useState<WsReadReceiptPush | null>(null);

    const connect = useCallback(() => {
        const token = localStorage.getItem('access_token');
        if (!token) return;

        // 根据当前环境自动拼装 WS 地址 (走 Vite 代理)
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/im_service`;

        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
            console.log('[IM_ENGINE] TCP 连接已建立，发送鉴权信令...');
            // 🚀 1. 建立连接后，立即在 5 秒内发送鉴权包
            ws.send(JSON.stringify({
                cmd_type: WsCmdType.AUTH,
                data: { token }
            }));
        };

        ws.onmessage = (event) => {
            try {
                const packet = JSON.parse(event.data);
                const { cmd_type, data } = packet;

                switch (cmd_type) {
                    case WsCmdType.AUTH_ACK:
                        if (data.status === 'success') {
                            console.log('[IM_ENGINE] 鉴权通过，长连接激活！');
                            setIsConnected(true);
                            startHeartbeat(); // 鉴权成功后启动心跳
                        } else {
                            console.error('[IM_ENGINE] 鉴权被拒:', data.error);
                            ws.close();
                        }
                        break;

                    case WsCmdType.MESSAGE:
                        // 2. 收到别人发来的聊天消息
                        console.log('[IM_ENGINE] 截获加密通讯封包:', data);
                        setIncomingMessage(data as WsMsgPush);
                        break;

                    // 拦截已读信令
                    case WsCmdType.READ_RECEIPT:
                        console.log('[IM_ENGINE] 截获对方已读信令:', data);
                        setReadReceipt(data as WsReadReceiptPush);
                        break;

                    case WsCmdType.MSG_ACK:
                        // 收到服务器的消息送达回执 (用于消除 UI 上的发送中 loading 状态)
                        break;

                    case WsCmdType.ERROR:
                        console.warn('[IM_ENGINE] 远端服务器异常:', data.error);
                        break;
                }
            } catch (e) {
                console.error('[IM_ENGINE] 无法解析的数据包', event.data);
            }
        };

        ws.onclose = () => {
            console.log('[IM_ENGINE] 链接已断开，进入静默重连序列...');
            setIsConnected(false);
            stopHeartbeat();
            // 3秒后自动重连
            reconnectTimerRef.current = setTimeout(connect, 3000);
        };

        ws.onerror = (err) => {
            console.error('[IM_ENGINE] 发生底层波动:', err);
        };
    }, []);

    // 维持生命的心跳发生器 (每 20 秒发送一次)
    const startHeartbeat = () => {
        stopHeartbeat();
        heartbeatTimerRef.current = setInterval(() => {
            if (wsRef.current?.readyState === WebSocket.OPEN) {
                wsRef.current.send(JSON.stringify({ cmd_type: WsCmdType.HEARTBEAT, data: {} }));
            }
        }, 20000);
    };

    const stopHeartbeat = () => {
        if (heartbeatTimerRef.current) clearInterval(heartbeatTimerRef.current);
    };

    // 暴露给组件的发送方法
    const transmitMessage = useCallback((req: WsMsgReq) => {
        if (wsRef.current?.readyState === WebSocket.OPEN) {
            wsRef.current.send(JSON.stringify({
                cmd_type: WsCmdType.MESSAGE,
                data: req
            }));
            return true;
        }
        return false;
    }, []);

    // 生命周期：组件挂载时连接，卸载时销毁
    useEffect(() => {
        connect();
        return () => {
            if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
            stopHeartbeat();
            if (wsRef.current) {
                wsRef.current.onclose = null; // 阻止卸载时触发重连
                wsRef.current.close();
            }
        };
    }, [connect]);

    return { isConnected, transmitMessage, incomingMessage, readReceipt };
};