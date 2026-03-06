package core

import (
	"fmt"
	//"strconv"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// NacosConfig 定义 Nacos 的配置边界
type NacosConfig struct {
	Host        string
	Port        uint64
	NamespaceId string
	ServiceName string
	Ip          string // 本服务的 IP
	ServicePort uint64 // 本服务的端口
	Weight      float64
	GroupName   string
	ClusterName string
}

// 全局 NamingClient，用于注册、发现和注销服务
var namingClient naming_client.INamingClient

// InitNacos 初始化 Nacos 客户端并注册当前服务
// 返回注销函数 (DeregisterFunc)，供 main.go 在服务关闭时调用
func InitNacos(cfg *NacosConfig) (func(), error) {
	// 1. 配置 Nacos 服务端信息
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(cfg.Host, cfg.Port, constant.WithContextPath("/nacos")),
	}

	// 2. 配置 Nacos 客户端信息
	// LogDir 和 CacheDir 指定日志和缓存的存放路径，避免写满根目录
	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId(cfg.NamespaceId),
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("/tmp/nacos/log"),
		constant.WithCacheDir("/tmp/nacos/cache"),
		constant.WithLogLevel("error"), // 避免 Nacos 底层日志刷屏
	)

	// 3. 创建 Naming 客户端
	var err error
	namingClient, err = clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("创建 Nacos 客户端失败: %w", err)
	}

	// 4. 注册当前服务实例
	success, err := namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          cfg.Ip,
		Port:        cfg.ServicePort,
		ServiceName: cfg.ServiceName,
		Weight:      cfg.Weight,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true, // 临时实例 (微服务推荐配置：服务断开后 Nacos 自动剔除)
		GroupName:   cfg.GroupName,
		ClusterName: cfg.ClusterName,
	})

	if err != nil || !success {
		return nil, fmt.Errorf("注册服务到 Nacos 失败: %w", err)
	}

	hlog.Infof("✅ 服务 [%s] 成功注册到 Nacos [地址: %s:%d]", cfg.ServiceName, cfg.Host, cfg.Port)

	// 5. 构造并返回优雅注销 (Deregister) 的闭包函数
	deregisterFunc := func() {
		hlog.Infof("⏳ 正在从 Nacos 注销服务 [%s]...", cfg.ServiceName)
		_, err := namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
			Ip:          cfg.Ip,
			Port:        cfg.ServicePort,
			ServiceName: cfg.ServiceName,
			GroupName:   cfg.GroupName,
			Cluster:     cfg.ClusterName,
			Ephemeral:   true, // 设置为临时实例，服务挂掉时 Nacos 会基于心跳超时自动剔除节点
		})
		if err != nil {
			hlog.Errorf("❌ 从 Nacos 注销服务失败: %v", err)
		} else {
			hlog.Infof("✅ 服务 [%s] 已从 Nacos 成功注销", cfg.ServiceName)
		}
	}

	return deregisterFunc, nil
}
