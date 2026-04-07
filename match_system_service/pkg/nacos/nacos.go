// kol_ads_marketing/match_system_service/pkg/nacos/nacos.go
package nacos

import (
	"fmt"
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

// NamingClient 全局实例，用于注册、发现和跨服务调用
var NamingClient naming_client.INamingClient

// InitNacos 初始化 Nacos 客户端并注册当前服务
// 返回注销函数 (DeregisterFunc)，供 main.go 在服务关闭时调用
func InitNacos(cfg *NacosConfig) (func(), error) {
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(cfg.Host, cfg.Port, constant.WithContextPath("/nacos")),
	}

	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId(cfg.NamespaceId),
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("/tmp/nacos/match_log"),     // 改一下日志路径，防止多服务本地冲突
		constant.WithCacheDir("/tmp/nacos/match_cache"), // 同上
		constant.WithLogLevel("error"),
	)

	var err error
	NamingClient, err = clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("创建 Nacos 客户端失败: %w", err)
	}

	success, err := NamingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          cfg.Ip,
		Port:        cfg.ServicePort,
		ServiceName: cfg.ServiceName,
		Weight:      cfg.Weight,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		GroupName:   cfg.GroupName,
		ClusterName: cfg.ClusterName,
	})
	if err != nil || !success {
		return nil, fmt.Errorf("注册服务到 Nacos 失败: %w", err)
	}
	hlog.Infof("✅ 服务 [%s] 成功注册到 Nacos [地址: %s:%d]", cfg.ServiceName, cfg.Host, cfg.Port)

	deregisterFunc := func() {
		hlog.Infof("⏳ 正在从 Nacos 注销服务 [%s]...", cfg.ServiceName)
		_, err := NamingClient.DeregisterInstance(vo.DeregisterInstanceParam{
			Ip:          cfg.Ip,
			Port:        cfg.ServicePort,
			ServiceName: cfg.ServiceName,
			GroupName:   cfg.GroupName,
			Cluster:     cfg.ClusterName,
			Ephemeral:   true,
		})
		if err != nil {
			hlog.Errorf("❌ 从 Nacos 注销服务失败: %v", err)
		} else {
			hlog.Infof("✅ 服务 [%s] 已从 Nacos 成功注销", cfg.ServiceName)
		}
	}
	return deregisterFunc, nil
}
