package es

import (
	"bytes"
	"context"
	"log"
)

const (
	KolIndexName   = "kol_profiles"
	BrandIndexName = "brand_profiles"
)

// kolMapping 针对 ESKolDocument 设计的倒排索引与向量检索图纸
const kolMapping = `
{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1
  },
  "mappings": {
    "properties": {
      "kol_user_id": { "type": "long" },
      "username": { 
        "type": "text",
        "fields": {
          "keyword": { "type": "keyword", "ignore_above": 256 }
        }
      },
      "status": { "type": "byte" },
      "kol_avatar_url": { 
        "type": "keyword", 
        "index": false 
      },
      "total_followers": { "type": "integer" },
      "tags": { "type": "keyword" },
      "ugc_platforms": { "type": "keyword" },
      "ugc_accounts_detail": { 
        "type": "object",
        "enabled": false 
      },
      "audience_embedding": { 
        "type": "dense_vector", 
        "dims": 128, 
        "index": true, 
        "similarity": "cosine" 
      }
    }
  }
}
`

const brandMapping = `
{
  "mappings": {
    "properties": {
      "brand_user_id": { "type": "long" },
      "status": { "type": "byte" },
      "username": { "type": "keyword" },
      "company_name": { 
        "type": "text",
        "fields": {
          "keyword": { "type": "keyword", "ignore_above": 256 }
        }
      },
      "brand_avatar_url": { "type": "keyword", "index": false },
      "industry": { "type": "keyword" },
      "is_verified": { "type": "byte" }
    }
  }
}
`

// InitIndices 检查并初始化 Elasticsearch 索引
func InitIndices() {
	ctx := context.Background()

	// 统一管理需要初始化的索引与对应的 Mapping
	indicesToInit := map[string]string{
		KolIndexName:   kolMapping,
		BrandIndexName: brandMapping,
	}

	for indexName, mappingJSON := range indicesToInit {
		createIndexIfNotExists(ctx, indexName, mappingJSON)
	}
	log.Println("✅ Elasticsearch 所有底层基础索引检查与初始化完毕！")
}

// createIndexIfNotExists 抽取出的公共基础函数：检查并创建单个索引
func createIndexIfNotExists(ctx context.Context, indexName string, mapping string) {
	// 1. 检查索引是否存在
	res, err := ESClient.Indices.Exists(
		[]string{indexName},
		ESClient.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		log.Fatalf("❌ 检查 ES 索引 [%s] 失败: %v", indexName, err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	// 200 表示存在，直接跳过
	if res.StatusCode == 200 {
		log.Printf("ℹ️ Elasticsearch 索引 [%s] 已存在，跳过创建", indexName)
		return
	} else if res.StatusCode != 404 {
		log.Fatalf("❌ 检查 ES 索引 [%s] 状态异常: %s", indexName, res.Status())
	}

	// 2. 404 表示不存在，执行创建
	log.Printf("⏳ 正在创建 Elasticsearch 索引 [%s] 并写入 Mapping...", indexName)

	createRes, err := ESClient.Indices.Create(
		indexName,
		ESClient.Indices.Create.WithBody(bytes.NewReader([]byte(mapping))),
		ESClient.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		log.Fatalf("❌ 创建 ES 索引 [%s] 请求发送失败: %v", indexName, err)
	}
	defer func() {
		_ = createRes.Body.Close()
	}()

	if createRes.IsError() {
		log.Fatalf("❌ 创建 ES 索引 [%s] 失败: %s", indexName, createRes.String())
	}

	log.Printf("✅ Elasticsearch 索引 [%s] 及 Mapping 初始化成功！", indexName)
}
