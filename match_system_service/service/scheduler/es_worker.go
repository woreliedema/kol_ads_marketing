package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"

	"kol_ads_marketing/match_system_service/dal/db"
	"kol_ads_marketing/match_system_service/dal/es"
)

// RunKOLBulkSyncToES 执行红人宽表到 ES 的批量同步任务
func RunKOLBulkSyncToES(mysqlClient *gorm.DB) {
	// 1. 从宽表捞取待同步数据 (每次限 500 条，防止内存溢出)
	var pendingKols []db.MatchKolWideIndex
	err := mysqlClient.Where("sync_status = ?", 0).Limit(500).Find(&pendingKols).Error
	if err != nil {
		hlog.Errorf("[ES Sync] 查询待同步 KOL 数据失败: %v", err)
		return
	}

	if len(pendingKols) == 0 {
		return // 没有需要同步的数据，直接返回
	}

	hlog.Infof("[ES Sync] 扫描到 %d 条待同步 KOL 数据，开始构建 Bulk 请求...", len(pendingKols))

	// 2. 构建 NDJSON 格式的 Bulk 负载 (Payload)
	var buf bytes.Buffer
	var successPKs []uint64 // 记录 MySQL 中的主键 ID，用于事后更新状态

	for _, kol := range pendingKols {
		// 调用你之前写的 Builder 将 MySQL 模型转为 ES 文档模型
		doc, err := es.BuildESKolDoc(&kol)
		if err != nil {
			hlog.Errorf("[ES Sync] KOL %d 数据转换(Pack)失败: %v", kol.KOLUserID, err)
			// 可选：在这里将这条数据的 sync_status 更新为 2 (失败)，防止死循环卡死
			mysqlClient.Model(&kol).Update("sync_status", 2)
			continue
		}

		docJSON, _ := json.Marshal(doc)

		// 拼装 NDJSON: 第一行是动作与元数据 (Action and Meta-data)
		// 注意这里使用的是我们在 mapping.go 中定义的常量 es.KolIndexName
		metaLine := fmt.Sprintf(`{"index":{"_index":"%s","_id":"%d"}}`+"\n", es.KolIndexName, kol.KOLUserID)

		buf.WriteString(metaLine)
		buf.Write(docJSON)
		buf.WriteString("\n") // 极客重点：每条数据结尾必须有换行符

		successPKs = append(successPKs, kol.ID)
	}

	if buf.Len() == 0 {
		return
	}

	// 3. 通过 go-elasticsearch/v8 发送 Bulk 请求
	ctx := context.Background()
	res, err := es.ESClient.Bulk(
		bytes.NewReader(buf.Bytes()),
		es.ESClient.Bulk.WithContext(ctx),
	)
	if err != nil {
		hlog.Errorf("[ES Sync] 发送 ES Bulk 请求失败: %v", err)
		return
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		hlog.Errorf("[ES Sync] ES Bulk 返回异常状态码: %s", res.String())
		// 生产环境中，最好读取 res.Body 里的具体报错原因进行排查
		bodyBytes, _ := io.ReadAll(res.Body)
		hlog.Errorf("[ES Sync] 异常详情: %s", string(bodyBytes))
		return
	}

	// 4. 解析 ES 响应，确认成功后更新 MySQL 状态
	// (MVP 阶段，只要 HTTP 状态不是 Error，我们就简单认为全批次成功)
	if len(successPKs) > 0 {
		updateErr := mysqlClient.Model(&db.MatchKolWideIndex{}).
			Where("id IN ?", successPKs).
			Update("sync_status", 1).Error

		if updateErr != nil {
			hlog.Errorf("[ES Sync] ES写入成功，但回写 MySQL 同步状态失败: %v", updateErr)
		} else {
			hlog.Infof("✅ [ES Sync] 成功将 %d 条 KOL 数据推入 Elasticsearch!", len(successPKs))
		}
	}
}

// RunBrandBulkSyncToES 执行品牌方宽表到 ES 的批量同步任务
func RunBrandBulkSyncToES(mysqlClient *gorm.DB) {
	// 1. 查询待同步的品牌方数据
	var pendingBrands []db.MatchBrandWideIndex
	err := mysqlClient.Where("sync_status = ?", 0).Limit(500).Find(&pendingBrands).Error
	if err != nil {
		hlog.Errorf("[ES Sync] 查询待同步 Brand 数据失败: %v", err)
		return
	}

	if len(pendingBrands) == 0 {
		return
	}

	hlog.Infof("[ES Sync] 扫描到 %d 条待同步 Brand 数据，开始构建 Bulk 请求...", len(pendingBrands))

	var buf bytes.Buffer
	var successPKs []uint64

	for _, brand := range pendingBrands {
		// 2. 转换为 ES 文档模型
		// 注意：BuildESBrandDoc 在设计时没有返回 error，因为它不涉及复杂的 JSON 反序列化
		doc := es.BuildESBrandDoc(&brand)

		docJSON, _ := json.Marshal(doc)

		// 3. 拼装 NDJSON，使用 BrandIndexName 常量
		metaLine := fmt.Sprintf(`{"index":{"_index":"%s","_id":"%d"}}`+"\n", es.BrandIndexName, brand.BrandUserID)

		buf.WriteString(metaLine)
		buf.Write(docJSON)
		buf.WriteString("\n")

		successPKs = append(successPKs, brand.ID)
	}

	if buf.Len() == 0 {
		return
	}

	// 4. 发送 Bulk 请求
	ctx := context.Background()
	res, err := es.ESClient.Bulk(
		bytes.NewReader(buf.Bytes()),
		es.ESClient.Bulk.WithContext(ctx),
	)
	if err != nil {
		hlog.Errorf("[ES Sync] 发送 Brand ES Bulk 请求失败: %v", err)
		return
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.IsError() {
		hlog.Errorf("[ES Sync] Brand ES Bulk 返回异常状态码: %s", res.String())
		return
	}

	// 5. 回写 MySQL 状态
	if len(successPKs) > 0 {
		updateErr := mysqlClient.Model(&db.MatchBrandWideIndex{}).
			Where("id IN ?", successPKs).
			Update("sync_status", 1).Error

		if updateErr != nil {
			hlog.Errorf("[ES Sync] Brand ES写入成功，但回写 MySQL 状态失败: %v", updateErr)
		} else {
			hlog.Infof("✅ [ES Sync] 成功将 %d 条 Brand 数据推入 Elasticsearch!", len(successPKs))
		}
	}
}
