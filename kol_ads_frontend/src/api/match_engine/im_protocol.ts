// 信令枚举
export const WsCmdType = {
    AUTH: 100,
    AUTH_ACK: 101,
    HEARTBEAT: 200,
    MESSAGE: 300,
    MSG_ACK: 301,
    READ_RECEIPT: 302,
    ERROR: 400,
};

// 已读回执推送载荷
export interface WsReadReceiptPush {
    session_id: string;
    reader_id: string;
    read_time: number;
}

// 发送消息载荷
export interface WsMsgReq {
    receiver_id: string;
    msg_type: number;
    content: string;
}

// 接收消息载荷 (后端推送过来的)
export interface WsMsgPush {
    msg_id: string;
    sender_id: string;
    msg_type: number;
    content: string;
    send_time: number;
}