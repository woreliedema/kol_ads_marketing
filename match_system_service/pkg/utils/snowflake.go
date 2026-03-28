package utils

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/snowflake"
)

// 全局雪花算法节点实例
var node *snowflake.Node

// InitSnowflake 初始化雪花算法节点 (必须在 main.go 或 router 初始化前调用)
// 极客提示：workerID (机器码) 必须在 0 ~ 1023 之间。
// 在微服务(K8s/Docker)部署时，绝不能写死！建议从环境变量获取，或通过 Redis INCR 动态分配。
func InitSnowflake(workerID int64) error {
	if workerID < 0 || workerID > 1023 {
		return fmt.Errorf("WorkerID 必须在 0 到 1023 之间, 当前传入: %d", workerID)
	}

	// 【架构优化】：自定义 Epoch (纪元时间)
	// 默认的 Twitter 纪元是 2010 年。雪花算法的 41 位时间戳可用 69 年。
	// 为了让我们的系统能支撑到 2090 年以后，我们将纪元设置为项目启动时间（例如 2024-01-01）
	// 注意：一旦设定并投入生产，此时间【绝对不能更改】，否则会导致 ID 重复！
	st, err := time.Parse("2006-01-02", "2024-01-01")
	if err != nil {
		return err
	}
	snowflake.Epoch = st.UnixNano() / 1000000 // 转换为毫秒

	// 实例化 Node
	node, err = snowflake.NewNode(workerID)
	if err != nil {
		return fmt.Errorf("初始化雪花算法节点失败: %w", err)
	}

	log.Printf("🔥 极客系统提示：全局 Snowflake ID 生成器初始化成功, WorkerID: %d, Epoch: %s", workerID, "2024-01-01")
	return nil
}

// GenerateSnowflakeID 生成全局唯一的 int64 ID (并发安全)
func GenerateSnowflakeID() int64 {
	if node == nil {
		// 防御性编程：对于底层核心工具，如果上游忘记初始化，应当尽早暴露问题（Fail-Fast）
		// 在高并发 IM 聊天中，若静默返回 0 或伪随机数，会导致严重的数据库唯一索引冲突或死锁。
		panic("Snowflake node 未初始化！请确保在应用启动时调用了 utils.InitSnowflake()")
	}

	// node.Generate() 底层自带互斥锁(Mutex)，并利用了 12 bit 序列号(每毫秒可生成 4096 个 ID)
	return node.Generate().Int64()
}
