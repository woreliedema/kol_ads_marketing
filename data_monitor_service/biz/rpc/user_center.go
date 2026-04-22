package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"kol_ads_marketing/data_monitor_service/pkg/core" // 替换为你的真实路径
)

// UGCAccount 对应 user_center 返回的平台信息
type UGCAccount struct {
	Platform    string `json:"platform"`
	PlatformUid string `json:"platform_uid"`
}

// UserCenterInfoResponse 对应用户中心 /api/internal/v1/user/info 的返回体
type UserCenterInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		UgcAccounts []UGCAccount `json:"ugc_accounts"`
	} `json:"data"`
}

// ---------------- RPC 核心配置与客户端 ----------------

var (
	// 全局单例 Hertz HTTP 客户端 (底层自带连接池)
	rpcClient *client.Client

	// internalSK 内部通信密钥
	internalSK string

	// fallbackBaseURL Nacos挂掉时的容灾降级地址
	fallbackBaseURL string
)

// Init 初始化 UserCenter RPC 客户端 (系统启动时调用一次即可)
func Init() {
	var err error
	// 1. 初始化高性能客户端 (设置超时拦截)
	rpcClient, err = client.NewClient(
		client.WithDialTimeout(3*time.Second),
		client.WithMaxConnWaitTimeout(3*time.Second),
	)
	if err != nil {
		hlog.Fatalf("❌ [RPC UserCenter] 致命错误: 无法初始化 HTTP 客户端: %v", err)
	}

	// 2. 环境配置加载
	baseIP := os.Getenv("USER_CENTER_IP")
	basePORT := os.Getenv("USER_CENTER_PORT")
	if baseIP == "" || basePORT == "" {
		fallbackBaseURL = "http://127.0.0.1:8081" // 默认开发环境地址
		hlog.Infof("⚠️ [RPC UserCenter] 未检测到完整的 IP/PORT，启用默认降级地址: %s", fallbackBaseURL)
	} else {
		fallbackBaseURL = fmt.Sprintf("http://%s:%s", baseIP, basePORT)
	}

	internalSK = os.Getenv("INTERNAL_SECRET_KEY")
	if internalSK == "" {
		hlog.Warnf("⚠️ [RPC UserCenter] 警告: 未配置 INTERNAL_SECRET_KEY，跨服务鉴权可能会被拦截")
	}

	hlog.Infof("✅ [RPC UserCenter] 客户端初始化完成")
}

// getTargetURL 核心路由逻辑：Nacos 动态发现 + 失败降级
func getTargetURL(ctx context.Context, path string) (string, error) {
	// 调用我们在 core 封装好的 Nacos 寻址方法
	ip, port, err := core.GetHealthyInstance("user-center-service", "DEFAULT_GROUP", "DEFAULT")

	if err != nil {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] Nacos 寻址失败 (%v), 触发降级路由 -> %s", err, fallbackBaseURL)
		return url.JoinPath(fallbackBaseURL, path)
	}

	// 动态拼接目标服务器 URL
	dynamicBaseURL := fmt.Sprintf("http://%s:%d", ip, port)
	return url.JoinPath(dynamicBaseURL, path)
}

// ---------------- 外部调用接口 ----------------

// GetUserPlatformUid 获取当前用户在指定平台(如 bilibili)的 UID
func GetUserPlatformUid(ctx context.Context, userID uint64, platform string) (uint64, error) {
	// 1. 拼接 URL (GET 请求，参数放在 Query 里)
	apiPath := "/api/internal/v1/user/info"
	targetURL, err := getTargetURL(ctx, apiPath)
	if err != nil {
		return 0, fmt.Errorf("路由拼接失败: %v", err)
	}

	// 2. 获取 Request 和 Response 的对象池实例
	req := &protocol.Request{}
	res := &protocol.Response{}

	// 3. 构造请求头与报文
	req.SetRequestURI(targetURL)
	req.SetMethod(consts.MethodGet) // 这是 GET 请求
	// 使用 Hertz 原生的高性能 API 追加 Query 参数！
	// 这会自动处理内存分配和 URL 编码，且绝不会污染 Path
	req.URI().QueryArgs().Add("user_id", strconv.FormatUint(userID, 10))
	// 设置内部通信校验 Header，与 User Center 的 middleware.InternalAuth 对齐
	// 注意：这里的 Key 要和 user_center 要求的保持一致 (此处假设为 X-Internal-Secret)
	req.Header.Set("X-Internal-Secret", internalSK)

	// 4. 设置上下文超时 (防止下游微服务雪崩拖垮当前服务)
	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 5. 发起 RPC 调用
	if err := rpcClient.Do(timeoutCtx, req, res); err != nil {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] HTTP调用失败: %v", err)
		return 0, fmt.Errorf("内部服务请求失败")
	}

	// 6. 校验状态码并反序列化
	if res.StatusCode() != 200 {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] 异常状态码: %d, body: %s", res.StatusCode(), string(res.Body()))
		return 0, fmt.Errorf("用户中心服务异常")
	}

	var ucResp UserCenterInfoResponse
	if err := json.Unmarshal(res.Body(), &ucResp); err != nil {
		hlog.CtxErrorf(ctx, "[RPC UserCenter] JSON 解析失败: %v", err)
		return 0, fmt.Errorf("内部数据解析异常")
	}

	if ucResp.Code != 0 {
		return 0, fmt.Errorf("请求被拒绝: %s", ucResp.Message)
	}

	// 7. 遍历提取指定平台的 UID
	for _, acc := range ucResp.Data.UgcAccounts {
		if acc.Platform == platform {
			mid, err := strconv.ParseUint(acc.PlatformUid, 10, 64)
			if err != nil {
				hlog.CtxErrorf(ctx, "平台 UID 非法 [%s]: %v", acc.PlatformUid, err)
				return 0, fmt.Errorf("获取到的账号标识非法")
			}
			return mid, nil
		}
	}

	return 0, fmt.Errorf("当前账号未绑定 [%s] 平台", platform)
}
