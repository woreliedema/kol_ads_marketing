package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/segmentio/kafka-go"
	"kol_ads_marketing/match_system_service/biz/model"
)

var chatMessageWriter *kafka.Writer

// InitKafkaProducer 初始化 Kafka 生产者连接池
// brokers 传入类似 []string{"x.x.x.x:9092"}
func InitKafkaProducer(brokers []string, topic string) {
	chatMessageWriter = &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,         // 专用于实时聊天消息的 Topic
		Balancer: &kafka.Hash{}, // 根据 Key Hash 路由到指定 Partition，保证同一对话的消息顺序
		// 异步生产，提高高并发下的吞吐量
		Async: true,
	}
	hlog.Infof("Kafka Producer 已初始化，连接 Brokers: %v, 目标 Topic: %s", brokers, topic)
}

// ProduceChatMessage 将 WebSocket 接收到的聊天消息打入 Kafka
func ProduceChatMessage(ctx context.Context, msg *model.KafkaChatMsg) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 使用 ReceiverID 作为 Key，它可以保证发给同一个人的消息始终落在同一个 Kafka Partition 中，从而保证消息的严格时序。
	keyStr := fmt.Sprintf("%d", msg.ReceiverID)

	kafkaMsg := kafka.Message{
		Key:   []byte(keyStr),
		Value: msgBytes,
		Time:  time.Now(),
	}

	// 发送消息到 Kafka
	err = chatMessageWriter.WriteMessages(ctx, kafkaMsg)
	if err != nil {
		hlog.CtxErrorf(ctx, "投递消息到 Kafka 失败: %v", err)
		return err
	}

	return nil
}

// CloseProducer 在服务退出时优雅关闭
func CloseProducer() {
	if chatMessageWriter != nil {
		_ = chatMessageWriter.Close()
	}
}
