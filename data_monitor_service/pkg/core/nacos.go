package core

import (
	"fmt"
	"strconv"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"kol_ads_marketing/data_monitor_service/pkg/utils"
)

// 全局 NamingClient，用于注册、发现和注销服务
var namingClient naming_client.INamingClient

// InitNacos 无参初始化 Nacos 客户端并注册当前服务
// 返回注销函数 (DeregisterFunc)，供 main.go 在服务关闭时调用
func InitNacos() func() {
	// 内部读取环境变量
	host := utils.GetEnv("NACOS_HOST", "127.0.0.1")
	port := uint64(utils.GetEnvInt("NACOS_PORT", 8848))
	namespaceId := utils.GetEnv("NACOS_NAMESPACE", "public")
	serviceName := utils.GetEnv("MONITOR_SERVICE_NAME", "data-monitor-service")
	ip := utils.GetEnv("MONITOR_SERVICE_IP", "127.0.0.1")

	// 获取本微服务监听的端口
	serverPortStr := utils.GetEnv("MONITOR_SERVICE_PORT", "8083")
	svcPort, _ := strconv.ParseUint(serverPortStr, 10, 64)

	groupName := utils.GetEnv("NACOS_GROUP", "DEFAULT_GROUP")
	clusterName := utils.GetEnv("NACOS_CLUSTER", "DEFAULT")

	// 1. 配置 Nacos 服务端信息
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(host, port, constant.WithContextPath("/nacos")),
	}

	// 2. 配置 Nacos 客户端信息
	clientConfig := *constant.NewClientConfig(
		constant.WithNamespaceId(namespaceId),
		constant.WithTimeoutMs(5000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("/tmp/nacos/dashboard_log"),
		constant.WithCacheDir("/tmp/nacos/dashboard_cache"),
		constant.WithLogLevel("error"),
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
		hlog.Fatalf("❌ 创建 Nacos 客户端失败: %v", err)
	}

	// 4. 注册当前服务实例
	success, err := namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        svcPort,
		ServiceName: serviceName,
		Weight:      1.0,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true, // 临时实例
		GroupName:   groupName,
		ClusterName: clusterName,
	})

	if err != nil || !success {
		hlog.Fatalf("❌ 注册服务到 Nacos 失败: %v", err)
	}

	hlog.Infof("✅ 服务 [%s] 成功注册到 Nacos [地址: %s:%d]", serviceName, host, port)

	// 5. 构造并返回优雅注销 (Deregister) 的闭包函数
	deregisterFunc := func() {
		hlog.Infof("⏳ 正在从 Nacos 注销服务 [%s]...", serviceName)
		_, err := namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
			Ip:          ip,
			Port:        svcPort,
			ServiceName: serviceName,
			GroupName:   groupName,
			Cluster:     clusterName,
			Ephemeral:   true,
		})
		if err != nil {
			hlog.Errorf("❌ 从 Nacos 注销服务失败: %v", err)
		} else {
			hlog.Infof("✅ 服务 [%s] 已从 Nacos 成功注销", serviceName)
		}
	}

	return deregisterFunc
}

// GetHealthyInstance 供其他包发现健康的微服务节点 (保持不变)
func GetHealthyInstance(serviceName, groupName, clusterName string) (string, uint64, error) {
	if namingClient == nil {
		return "", 0, fmt.Errorf("nacos naming client 未初始化")
	}
	instance, err := namingClient.SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam{
		ServiceName: serviceName,
		GroupName:   groupName,
		Clusters:    []string{clusterName},
	})
	if err != nil {
		return "", 0, fmt.Errorf("获取服务 [%s] 实例失败: %w", serviceName, err)
	}
	return instance.Ip, instance.Port, nil
}
