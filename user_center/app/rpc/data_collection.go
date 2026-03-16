package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// 获取 Python 服务的基础地址 (优先读环境变量，兜底本地 8000)
func getPythonBaseURL() string {
	ip := os.Getenv("DATA_COLLECTION_IP")
	port := os.Getenv("DATA_COLLECTION_PORT")
	if ip == "" {
		ip = "127.0.0.1"
		port = "8000"
	}
	return fmt.Sprintf("http://%s:%s", ip, port)
}

// ParseProfileURL 同步调用 Python 服务解析主页链接提取 UID
func ParseProfileURL(ctx context.Context, platform, spaceURL string) (string, error) {
	// 拼接目标 URL (务必对 spaceURL 进行 QueryEscape 编码，防止特殊字符截断 URL)
	targetURL := fmt.Sprintf("%s/inner/tools/parse_profile_url?platform=%s&url=%s",
		getPythonBaseURL(), platform, url.QueryEscape(spaceURL))
	//client := &http.Client{Timeout: 3 * time.Second} // 3秒超时，快失败
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		hlog.CtxErrorf(ctx, "[RPC] 创建请求失败: %v", err)
		return "", errors.New("内部服务构建请求失败")
	}
	// 核心防御：携带内部通信秘钥去敲 Python 的门
	req.Header.Set("X-Internal-Secret", os.Getenv("INTERNAL_SECRET_KEY"))
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		hlog.CtxErrorf(ctx, "[RPC] 请求采集服务异常: %v", err)
		return "", errors.New("内部服务通信失败")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			hlog.CtxWarnf(ctx, "[RPC] 关闭 Response Body 失败: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("不支持的平台或链接格式错误")
	}

	// 解析 Python 返回的 JSON
	// (假设 Python 返回格式为 {"code": 200, "data": {"uid": "xxx"}, "msg": "ok"})
	// 如果你的 Python 返回字段名不一样，请修改下面结构体里的 json 标签！
	var result struct {
		Code   int    `json:"code"`
		Router string `json:"router"`
		Data   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			UID     string `json:"uid"` // 提取这个字段！
		} `json:"data"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		hlog.CtxErrorf(ctx, "[RPC] 解析采集服务响应失败: %v, Body: %s", err, string(bodyBytes))
		return "", errors.New("解析采集服务响应失败")
	}

	if result.Code != 200 || result.Data.Code != 0 || result.Data.UID == "" {
		hlog.CtxWarnf(ctx, "[RPC] 采集服务未能成功提取UID: %s", string(bodyBytes))
		return "", errors.New("未能从该链接中提取到有效 UID，请检查链接是否正确")
	}

	return result.Data.UID, nil
}

// RegisterCrawlerTarget 异步调用 Python 服务注册爬虫任务
func RegisterCrawlerTarget(platform, targetID string) {
	var platformType int
	var resourceTypes []string

	// 根据传入的平台名称，匹配对应的 platform_type (整数) 和 需要触发的任务列表
	switch platform {
	case "bilibili":
		platformType = 3
		// B站绑定需要同时触发两个抓取任务
		resourceTypes = []string{"scrape_and_store_user_info", "scrape_and_store_user_relation"}
	case "douyin":
		platformType = 1
		// 假设抖音也需要这两个任务 (根据实际 数据采集服务 中的任务类型进行调整,当前还未开发douyin和tiktok模块，先占位)
		resourceTypes = []string{"scrape_and_store_user_info", "scrape_and_store_user_relation"}
	case "tiktok":
		platformType = 2
		resourceTypes = []string{"scrape_and_store_user_info"}
	default:
		fmt.Printf("[Async RPC] 未知的平台类型: %s，无法派发任务\n", platform)
		return
	}

	baseURL := fmt.Sprintf("%s/inner/target/register", getPythonBaseURL())

	// 复用同一个 HTTP Client (5秒超时足够了)
	client := &http.Client{Timeout: 5 * time.Second}

	// 2. 遍历该平台需要的所有任务类型，分别向 Python 发起 POST 请求
	for _, resType := range resourceTypes {
		// 🚀 核心看点：基于 Swagger 截图，这里必须构建 Query Params，而不是 JSON
		params := url.Values{}
		params.Add("uid", targetID)          // 用户的平台 UID
		params.Add("resource_type", resType) // 任务类型
		params.Add("target_id", targetID)    // 目标ID (同 UID)
		params.Add("platform_type", fmt.Sprintf("%d", platformType))
		// interval_minutes 截图显示有默认值，我们这里就不传了，让 Python 用默认的

		// 拼接出最终的请求地址: http://127.0.0.1:8000/inner/target/register?uid=xxx&resource_type=yyy...
		requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

		// 创建 POST 请求 (注意最后传入的 body 是 nil，因为参数全在 URL 里)
		req, err := http.NewRequest("POST", requestURL, nil)
		if err != nil {
			fmt.Printf("[Async RPC] 构建爬虫注册请求失败 (task: %s): %v\n", resType, err)
			continue
		}

		// 挂上咱们最核心的内部通信秘钥防线
		req.Header.Set("X-Internal-Secret", os.Getenv("INTERNAL_SECRET_KEY"))

		// 发起请求
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("[Async RPC] 触发爬虫调度失败, target: %s, task: %s, err: %v\n", targetID, resType, err)
			continue // 失败了就继续下一个任务，不要阻断
		}

		// 必须在循环里显式关闭 Body，防止连接池泄漏
		defer func() {
			_ = resp.Body.Close()
		}()

		fmt.Printf("[Async RPC] 成功派发爬虫任务 -> 平台: %d, Task: %s, UID: %s (Status: %d)\n",
			platformType, resType, targetID, resp.StatusCode)
	}
}

// CheckProfileFreshness 同步探测鲜活数据 (L1 Redis / L2 CK)
// 如果 Python 判定数据鲜活，返回 true 及核心数据；如果判定过期或不存在，返回 false。
func CheckProfileFreshness(ctx context.Context, platform, uid string) (bool, map[string]interface{}, error) {
	// 拼接探测路由 (根据你的架构设计：GET /inner/data/profile/{platform}/{uid})
	targetURL := fmt.Sprintf("%s/inner/data/profile/%s/%s", getPythonBaseURL(), platform, uid)

	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("X-Internal-Secret", os.Getenv("INTERNAL_SECRET_KEY"))

	client := &http.Client{Timeout: 3 * time.Second} // 3秒超时快失败
	resp, err := client.Do(req)
	if err != nil {
		hlog.CtxErrorf(ctx, "[RPC] 探测鲜活数据请求异常: %v", err)
		return false, nil, errors.New("内部服务通信失败")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			hlog.CtxWarnf(ctx, "[RPC] 关闭 Response Body 失败: %v", closeErr)
		}
	}()

	// 1. 如果 Python 返回 404，说明数据缺失或已过期，需要触发异步爬虫
	if resp.StatusCode == http.StatusNotFound {
		return false, nil, nil
	}

	// 如果返回非 200 且非 404，属于未知异常
	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("探测接口异常状态码: %d", resp.StatusCode)
	}

	// 2. 如果返回 200，说明命中热/温数据，解析返回的具体业务字段
	var result struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"` // 假设包含 nickname, followers_count 等
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return false, nil, errors.New("解析探测数据失败")
	}

	return true, result.Data, nil
}
