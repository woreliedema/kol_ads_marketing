// match_system_service/pkg/mq/kafka_consumer.go
package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/google/uuid"
	"github.com/hertz-contrib/websocket"
	"github.com/segmentio/kafka-go"

	"kol_ads_marketing/match_system_service/biz/model"
	"kol_ads_marketing/match_system_service/service/im_service"
)

var chatMessageReader *kafka.Reader

// StartKafkaConsumer 启动 Kafka 消费者 (异步阻塞)
func StartKafkaConsumer(brokers []string, topic string) {
	// ⚡️ 核心架构点：每个 Pod/实例 必须拥有独立的 GroupID，实现广播消费
	nodeID := uuid.New().String()
	groupID := fmt.Sprintf("im_service-node-group-%s", nodeID)

	chatMessageReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID, // 独一无二的 GroupID
		MinBytes: 10e3,    // 10KB
		MaxBytes: 10e6,    // 10MB
	})

	hlog.Infof("Kafka Consumer 已启动，节点广播 GroupID: %s, 监听 Topic: %s", groupID, topic)

	// 开启常驻 Goroutine 消费消息
	go func() {
		for {
			ctx := context.Background()
			m, err := chatMessageReader.ReadMessage(ctx)
			if err != nil {
				hlog.CtxErrorf(ctx, "Kafka 消费者读取异常: %v", err)
				// 生产环境可以考虑加入重试或平滑退避逻辑 (Backoff)
				continue
			}

			// 解析富文本消息体
			var kafkaMsg model.KafkaChatMsg
			if err := json.Unmarshal(m.Value, &kafkaMsg); err != nil {
				hlog.CtxErrorf(ctx, "解析 Kafka 消息失败: %v", err)
				continue
			}

			// 路由分发消息
			dispatchMsgToLocalClient(kafkaMsg)
		}
	}()
}

// dispatchMsgToLocalClient 在本地连接池中寻找接收方并推送
func dispatchMsgToLocalClient(msg model.KafkaChatMsg) {
	// 1. 去本地 ClientManager 查接收方是否在线
	// (注：你需要确保你的 im_service.GlobalClientManager 实现了 GetClient 方法)
	conn, _ := im_service.GlobalClientManager.GetClient(msg.ReceiverID)
	if conn == nil {
		// 接收方不在此节点，直接 return
		// 意味着要么他在其他节点在线，要么他彻底离线了
		return
	}

	// 2. 接收方在本地！转换为向前端推送的结构体
	pushData := model.MsgPush{
		MsgID:    strconv.FormatInt(msg.MsgID, 10),
		SenderID: msg.SenderID,
		MsgType:  msg.MsgType,
		Content:  msg.Content,
		SendTime: msg.SendTime,
	}

	// 3. 封装为标准 WebSocket 信令外壳
	pushDataBytes, _ := json.Marshal(pushData)
	wsCmd := model.WsCmd{
		CmdType: model.CmdTypeMessage, // 300
		Data:    pushDataBytes,
	}
	wsCmdBytes, _ := json.Marshal(wsCmd)

	// 4. 执行物理推送
	err := conn.WriteMessage(websocket.TextMessage, wsCmdBytes)
	if err != nil {
		hlog.Errorf("向用户 [%d] 推送消息失败: %v", msg.ReceiverID, err)
		// 如果写入失败，通常意味着连接已断开，可以主动清理
		im_service.GlobalClientManager.RemoveClient(msg.ReceiverID)
	} else {
		hlog.Infof("已成功向本地用户 [%d] 推送来自 [%d] 的消息", msg.ReceiverID, msg.SenderID)
	}
}

// CloseConsumer 优雅停机
func CloseConsumer() {
	if chatMessageReader != nil {
		_ = chatMessageReader.Close()
	}
}

func StartIMMessageConsumer(ctx context.Context, brokers []string, topic, groupID string, svc *im_service.IMPersistenceService) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	go func() {
		defer func(r *kafka.Reader) {
			_ = r.Close()
		}(r)
		log.Printf("极客系统提示：Kafka IM消费者已启动, Topic: %s", topic)
		for {
			m, err := r.FetchMessage(ctx)
			if err != nil {
				log.Printf("拉取Kafka消息异常: %v", err)
				break
			}

			// 处理并落库
			err = svc.ProcessKafkaMessage(ctx, m.Value)
			if err != nil {
				hlog.CtxErrorf(ctx, "持久化消息失败 (offset: %d): %v", m.Offset, err)

				// 【极客防御】：判断是否为 JSON 解析等不可恢复的业务错误
				// 假设它属于结构体/数据污染导致的不可恢复错误，我们必须 Commit 把它跳过去！
				// tod如果是数据库短时宕机导致的 err，才需要利用延时重试，不要 Commit。
				// MVP 阶段为了防止主链路堵死，我们强制提交脏数据 offset：
				hlog.CtxWarnf(ctx, "跳过毒药消息并提交 Offset，防止死循环")
				_ = r.CommitMessages(ctx, m)
				continue
			}
			// 落库成功后提交Offset，保证At-Least-Once语义
			if err := r.CommitMessages(ctx, m); err != nil {
				log.Printf("提交Offset失败: %v", err)
			}
		}
	}()
}
