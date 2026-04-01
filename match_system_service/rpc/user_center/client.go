package user_center

import (
	"context"
	"encoding/json"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"net/url"
	"os"
	"time"
)

// 1. 将原本在 im_service 的 DTO 结构体全部迁移到这里
type RemoteBaseInfo struct {
	Username string `json:"username"`
	Role     int    `json:"role"`
}

type RemoteProfile struct {
	RealName  string `json:"real_name"`
	AvatarURL string `json:"avatar_url"`
}

type RemoteUGCAccount struct {
	Platform string `json:"platform"`
	Nickname string `json:"nickname"`
}

type BaseUserInfo struct {
	BaseInfo    RemoteBaseInfo     `json:"base_info"`
	Profile     RemoteProfile      `json:"profile"`
	UGCAccounts []RemoteUGCAccount `json:"ugc_accounts"`
}

// GetDisplayName 智能提取展示名称
func (u BaseUserInfo) GetDisplayName() string {
	if len(u.UGCAccounts) > 0 && u.UGCAccounts[0].Nickname != "" {
		return u.UGCAccounts[0].Nickname
	}
	if u.Profile.RealName != "" {
		return u.Profile.RealName
	}
	if u.BaseInfo.Username != "" {
		return u.BaseInfo.Username
	}
	return "未知用户"
}

// GetDisplayAvatar 智能提取显示头像
func (u BaseUserInfo) GetDisplayAvatar() string {
	if u.Profile.AvatarURL != "" {
		return u.Profile.AvatarURL
	}
	return ""
}

// ---------------- RPC 核心配置与客户端 ----------------

var (
	// 全局复用的 Hertz HTTP 客户端（自带高性能连接池）
	rpcClient *client.Client

	// 内存缓存的微服务配置，彻底杜绝高频 syscall
	baseURL    string
	internalSK string
)

// Init 初始化 UserCenter RPC 客户端 (系统启动时调用一次即可)
func Init() {
	var err error
	// 1. 初始化单例客户端
	rpcClient, err = client.NewClient(
		client.WithDialTimeout(3*time.Second),        // TCP 拨号建立连接的超时时间
		client.WithMaxConnWaitTimeout(3*time.Second), // 从连接池获取空闲连接的超时时间
	)
	if err != nil {
		hlog.Fatalf("[RPC UserCenter] 致命错误: 无法初始化 HTTP 客户端: %v", err)
	}

	// 2. 将环境变量载入内存，仅执行一次
	baseIP := os.Getenv("USER_CENTER_IP")
	basePORT := os.Getenv("USER_CENTER_PORT")
	if baseIP == "" || basePORT == "" {
		baseURL = "http://127.0.0.1:8081" // 开发环境默认降级
		hlog.Infof("[RPC UserCenter] 未检测到完整的 IP/PORT 环境变量，使用默认降级地址")
	} else {
		baseURL = "http://" + baseIP + ":" + basePORT
	}

	internalSK = os.Getenv("INTERNAL_SECRET_KEY")
	if internalSK == "" {
		hlog.Warnf("[RPC UserCenter] 警告: 未配置 INTERNAL_SECRET_KEY，内部通信可能受阻")
	}

	hlog.Infof("[RPC UserCenter] 初始化完成，TargetBaseURL: %s", baseURL)
}

// ---------------- 外部调用接口 ----------------

// BatchGetUserInfo 跨微服务批量获取用户信息
func BatchGetUserInfo(ctx context.Context, uids []uint64) map[uint64]BaseUserInfo {
	result := make(map[uint64]BaseUserInfo)
	if len(uids) == 0 {
		return result
	}

	// 1. UID 去重（过滤重复请求）
	uniqueUIDs := make(map[uint64]struct{})
	var cleanUIDs []uint64
	for _, uid := range uids {
		if _, exists := uniqueUIDs[uid]; !exists {
			uniqueUIDs[uid] = struct{}{}
			cleanUIDs = append(cleanUIDs, uid)
		}
	}

	// 2. 安全的 URL 拼接 (利用 Go 1.19+ 的 url.JoinPath 自动处理多余的斜杠 '/')
	targetURL, err := url.JoinPath(baseURL, "/api/internal/v1/users/batch_info")
	if err != nil {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] URL 拼接异常: %v", err)
		return result
	}

	// 3. 构造请求报文
	reqBody := map[string]interface{}{"uids": cleanUIDs}
	bodyBytes, _ := json.Marshal(reqBody)

	req := &protocol.Request{}
	res := &protocol.Response{}

	req.SetRequestURI(targetURL)
	req.SetMethod(consts.MethodPost)
	req.Header.SetContentTypeBytes([]byte("application/json"))
	req.Header.Set("X-Internal-Secret", internalSK) // 直接使用内存变量
	req.SetBody(bodyBytes)

	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 4. 执行复用连接的 RPC 调用
	if err := rpcClient.Do(timeoutCtx, req, res); err != nil {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] 批量接口调用失败 (网络层面): %v", err)
		return result
	}

	// 5. 反序列化
	type StandardResponse struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Users map[uint64]BaseUserInfo `json:"users"`
		} `json:"data"`
	}

	rawBody := string(res.Body())
	statusCode := res.StatusCode()
	if statusCode != 200 {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] 异常状态码: %d, 原始响应体: %s", statusCode, rawBody)
	}

	var resp StandardResponse
	if err := json.Unmarshal(res.Body(), &resp); err != nil {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] 响应解析失败: %v", err)
		return result
	}

	if resp.Code != 0 {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] 业务异常 Code: %d, Msg: %s", resp.Code, resp.Message)
		return result
	}

	// 6. 成功返回
	for uid, info := range resp.Data.Users {
		result[uid] = info
	}

	return result
}
