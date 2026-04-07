package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kol_ads_marketing/user_center/app/core"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// 获取 Python 服务的基础地址 (优先读环境变量，兜底本地 8000)
func getPythonBaseURL() string {
	serviceName := os.Getenv("DATA_COLLECTION_NAME")

	// 1. 尝试走 Nacos 动态服务发现
	if serviceName != "" {
		groupName := os.Getenv("NACOS_GROUP")
		if groupName == "" {
			groupName = "DEFAULT_GROUP"
		}
		clusterName := os.Getenv("NACOS_CLUSTER")
		if clusterName == "" {
			clusterName = "DEFAULT"
		}

		ip, port, err := core.GetHealthyInstance(serviceName, groupName, clusterName)
		if err == nil && ip != "" && port != 0 {
			// Nacos 命中！动态组装 URL
			return fmt.Sprintf("http://%s:%d", ip, port)
		}

		// 如果 Nacos 没找到服务（比如 Python 端还没启动），打印警告并触发降级
		hlog.Warnf("[RPC] Nacos 服务发现失败 [%s]，触发降级机制: %v", serviceName, err)
	}

	// 2. 优雅降级：读取固定环境变量配置
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
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
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
	// Python 返回格式为 {"code": 200, "data": {"uid": "xxx"}, "msg": "ok"})
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

// RegisterCrawlerTarget 异步、并发调用 Python 服务注册爬虫任务
func RegisterCrawlerTarget(ctx context.Context, platform, targetID string) {
	var platformType int
	var resourceTypes []string

	// 根据传入的平台名称，匹配对应的 platform_type (整数) 和 需要触发的任务列表
	switch platform {
	case "bilibili":
		platformType = 3
		// B站绑定需要同时触发三个抓取任务(爬取用户基本信息、爬取用户关系信息、爬取用户发布视频内容，爬取用户发布视频内容任务会自动触发爬取视频基础信息和视频评论任务)
		resourceTypes = []string{"scrape_and_store_user_info", "scrape_and_store_user_relation", "scrape_and_store_user_videos"}
	case "douyin":
		platformType = 1
		// 假设抖音也需要这两个任务 (根据实际 数据采集服务 中的任务类型进行调整,当前还未开发douyin和tiktok模块，先占位)
		resourceTypes = []string{"scrape_and_store_user_info", "scrape_and_store_user_relation"}
	case "tiktok":
		platformType = 2
		resourceTypes = []string{"scrape_and_store_user_info"}
	default:
		hlog.CtxWarnf(ctx, "[Async RPC] 未知的平台类型: %s，无法派发任务\n", platform)
		return
	}

	baseURL := fmt.Sprintf("%s/inner/target/register", getPythonBaseURL())

	go func(bgCtx context.Context) {
		client := &http.Client{Timeout: 5 * time.Second}
		var wg sync.WaitGroup

		// 💡 核心设计 2：并发扇出 (Fan-out)
		// 之前是串行发送 HTTP 请求，现在针对 3 个任务我们同时开 3 个 Goroutine 发送
		for _, resType := range resourceTypes {
			wg.Add(1)

			// 必须将 resType 作为参数传入闭包，避免经典 for 循环变量捕获问题
			go func(taskType string) {
				defer wg.Done()

				params := url.Values{}
				params.Add("uid", targetID)
				params.Add("resource_type", taskType)
				params.Add("target_id", targetID)
				params.Add("platform_type", fmt.Sprintf("%d", platformType))

				requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

				// 使用背景 ctx 进行控制
				req, err := http.NewRequestWithContext(bgCtx, "POST", requestURL, nil)
				if err != nil {
					hlog.CtxErrorf(bgCtx, "[Async RPC] 构建请求失败 (task: %s): %v", taskType, err)
					return
				}

				req.Header.Set("X-Internal-Secret", os.Getenv("INTERNAL_SECRET_KEY"))

				resp, err := client.Do(req)
				if err != nil {
					hlog.CtxErrorf(bgCtx, "[Async RPC] 触发调度失败, target: %s, task: %s, err: %v", targetID, taskType, err)
					return
				}

				// 💡 核心设计 3：彻底解决 defer 的闭包泄露问题
				// 此时 defer 是在单独的 goroutine 闭包中，因此请求一结束立刻关闭 Body，零泄露风险。
				defer func(Body io.ReadCloser) {
					_ = Body.Close()
				}(resp.Body)

				// 非 200 状态码告警
				if resp.StatusCode != http.StatusOK {
					hlog.CtxWarnf(bgCtx, "[Async RPC] 派发任务异常状态码: %d -> Task: %s", resp.StatusCode, taskType)
					return
				}

				hlog.CtxInfof(bgCtx, "[Async RPC] 成功派发 -> 平台: %d, Task: %s, UID: %s", platformType, taskType, targetID)
			}(resType)
		}

		// 等待这几个并发请求全部发完后，退出主控协程
		wg.Wait()
		hlog.CtxInfof(bgCtx, "[Async RPC] UID: %s 的所有 (%d 个) 爬虫任务并行派发完成", targetID, len(resourceTypes))

	}(context.Background())
}

// CheckProfileFreshness 同步探测鲜活数据 (L1 Redis / L2 CK)
// 如果 Python 判定数据鲜活，返回 true 及核心数据；如果判定过期或不存在，返回 false。
func CheckProfileFreshness(ctx context.Context, platform, uid string) (bool, map[string]interface{}, error) {
	// 拼接探测路由 (根据你的架构设计：GET /inner/data/profile/{platform}/{uid})
	targetURL := fmt.Sprintf("%s/inner/data/profile/%s/%s", getPythonBaseURL(), platform, uid)

	req, _ := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
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
