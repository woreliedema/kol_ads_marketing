import api from '../request';

// --- 接口契约 ---
export interface SessionItem {
    session_id: string;
    target_user_id: string;
    target_user_name: string;
    target_avatar: string;
    latest_msg: string;
    unread_count: number;
    updated_at: number;
}

export interface IMMessage {
    msg_id: string;
    session_id: string;
    sender_id: string;
    receiver_id: string;
    content: string;
    msg_type: number; // 假设 1-文本, 2-图片等
    send_time: number;
    status: number;
    // created_at: number;
}

export interface HistoryResponse {
    messages: IMMessage[];
    next_cursor: string;
    has_more: boolean;
}

// --- API 请求 ---

// 1. 获取会话列表
export const getSessionListApi = () => {
    return api.get('/match/im/sessions');
};

// 2. 获取历史聊天记录 (游标分页)
export const getHistoryMessagesApi = (targetId: string, cursor: string = "0", limit: number = 20) => {
    return api.get(`/match/im/history?target_id=${targetId}&cursor=${cursor}&limit=${limit}`);
};
// 5. 主动清空某个会话的未读数 (Read ACK)
export const clearUnreadApi = (targetId: string) => {
    // 注意：这里使用 post，并且以 JSON 对象的形式传入 target_id
    return api.post('/match/im/read', {
        target_id: targetId
    });
};

// (预留) 3. 发送消息接口 (如果是 HTTP 降级方案，或者是后续 WebSocket 的鉴权前置)
// export const sendMessageApi = ...