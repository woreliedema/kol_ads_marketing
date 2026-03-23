package es

import (
	"log"
	"os"

	"github.com/elastic/go-elasticsearch/v8"
)

var ESClient *elasticsearch.Client

func Init() {
	// 1. 获取 ES 地址
	esURL := os.Getenv("ES_URL")
	if esURL == "" {
		esURL = "http://127.0.0.1:9200" // 默认退避策略
	}

	cfg := elasticsearch.Config{
		Addresses: []string{esURL},
	}

	var err error
	ESClient, err = elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Elasticsearch 创建客户端失败: %v", err)
	}

	// 2. Fail-fast 机制：发送 Ping 请求，确保 ES 服务真的可用
	res, err := ESClient.Info()
	if err != nil {
		log.Fatalf("Elasticsearch 无法连接 (Ping失败): %v", err)
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		log.Fatalf("Elasticsearch 返回错误状态: %s", res.String())
	}

	log.Println("✅ Elasticsearch 8.0 客户端初始化成功")

	InitIndices()
}
