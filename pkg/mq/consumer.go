package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/go-kratos/kratos/v2/log"
)

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
	NameServers []string // RocketMQ NameServer 地址列表
	GroupName   string   // 消费组（命名见 topics.go GroupXxx 常量）
}

// MessageHandler 消息处理函数
// 返回 nil 表示消费成功；返回 error 触发重试（最多 MaxRetryCount 次后进死信队列）
type MessageHandler func(ctx context.Context, body []byte) error

// Consumer RocketMQ 消费者封装（集群消费模式，广播不适用于本系统多副本部署）
type Consumer struct {
	c      rocketmq.PushConsumer
	logger *log.Helper
}

// NewConsumer 创建消费者
func NewConsumer(cfg ConsumerConfig, logger log.Logger) (*Consumer, error) {
	c, err := rocketmq.NewPushConsumer(
		consumer.WithNsResolver(primitive.NewPassthroughResolver(resolveNameServers(cfg.NameServers))),
		consumer.WithGroupName(cfg.GroupName),
		consumer.WithConsumeFromWhere(consumer.ConsumeFromLastOffset),
		// 消息重试：最多3次，失败转死信队列（LLD §4 补充规则）
		consumer.WithMaxReconsumeTimes(MaxRetryCount),
	)
	if err != nil {
		return nil, fmt.Errorf("创建消费者失败: %w", err)
	}
	return &Consumer{c: c, logger: log.NewHelper(logger)}, nil
}

// Subscribe 订阅 Topic 并注册处理器
// selector 传 ExpressionType TAG 过滤（如 review_notice_topic 按 eventType 过滤）
func (c *Consumer) Subscribe(topic string, handler MessageHandler) error {
	err := c.c.Subscribe(topic, consumer.MessageSelector{},
		func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
			for _, msg := range msgs {
				if err := handler(ctx, msg.Body); err != nil {
					c.logger.Errorw(
						"msg", "消息消费失败，将触发重试",
						"topic", topic,
						"msg_id", msg.MsgId,
						"reconsume_times", msg.ReconsumeTimes,
						"error", err.Error(),
					)
					return consumer.ConsumeRetryLater, nil
				}
			}
			return consumer.ConsumeSuccess, nil
		})
	if err != nil {
		return fmt.Errorf("订阅Topic失败 topic=%s: %w", topic, err)
	}
	return nil
}

// Start 启动消费（订阅全部注册后调用）
func (c *Consumer) Start() error {
	return c.c.Start()
}

// Shutdown 优雅关闭
func (c *Consumer) Shutdown() error {
	return c.c.Shutdown()
}

// Unmarshal 反序列化消息体为业务结构体（handler 内使用）
func Unmarshal(body []byte, v interface{}) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("消息反序列化失败: %w", err)
	}
	return nil
}

// nowMillis 当前 Unix 毫秒时间戳
func nowMillis() int64 {
	return time.Now().UnixMilli()
}
