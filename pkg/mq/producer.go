package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"

	"cr-system/pkg/trace"
	"cr-system/pkg/util"
)

// ProducerConfig 生产者配置
type ProducerConfig struct {
	NameServers []string // RocketMQ NameServer 地址列表
	GroupName   string   // 生产者组
	Retry       int      // 发送失败重试次数（默认2）
}

// Producer RocketMQ 生产者封装
// 统一：JSON序列化、自动注入 MessageHeader（TraceID/MessageID/时间戳/来源服务）
type Producer struct {
	p           rocketmq.Producer
	tp          rocketmq.TransactionProducer
	serviceName string
}

// NewProducer 创建普通消息生产者
func NewProducer(cfg ProducerConfig, serviceName string) (*Producer, error) {
	retry := cfg.Retry
	if retry <= 0 {
		retry = 2
	}
	p, err := rocketmq.NewProducer(
		producer.WithNsResolver(primitive.NewPassthroughResolver(resolveNameServers(cfg.NameServers))),
		producer.WithGroupName(cfg.GroupName),
		producer.WithRetry(retry),
	)
	if err != nil {
		return nil, fmt.Errorf("创建生产者失败: %w", err)
	}
	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("启动生产者失败: %w", err)
	}
	return &Producer{p: p, serviceName: serviceName}, nil
}

// NewTransactionProducer 创建事务消息生产者（LLD §4：创建评审单使用事务消息）
// listener 由业务方实现：ExecuteLocalTransaction 执行本地事务（如单据入库），
// CheckLocalTransaction 供 Broker 回查事务状态
func NewTransactionProducer(cfg ProducerConfig, serviceName string, listener primitive.TransactionListener) (*Producer, error) {
	tp, err := rocketmq.NewTransactionProducer(
		listener,
		producer.WithNsResolver(primitive.NewPassthroughResolver(resolveNameServers(cfg.NameServers))),
		producer.WithGroupName(cfg.GroupName),
	)
	if err != nil {
		return nil, fmt.Errorf("创建事务生产者失败: %w", err)
	}
	if err := tp.Start(); err != nil {
		return nil, fmt.Errorf("启动事务生产者失败: %w", err)
	}
	return &Producer{tp: tp, serviceName: serviceName}, nil
}

// buildMessage 构造消息体：JSON序列化 + 注入公共头
func (p *Producer) buildMessage(ctx context.Context, topic string, body interface{}) (*primitive.Message, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("消息序列化失败: %w", err)
	}
	msg := primitive.NewMessage(topic, data)
	msg.WithProperty("trace_id", trace.TraceIDFromContext(ctx))
	msg.WithProperty("produce_svc", p.serviceName)
	return msg, nil
}

// NewHeader 生成消息公共头（业务构造消息体时使用）
func NewHeader(ctx context.Context, serviceName string) MessageHeader {
	return MessageHeader{
		TraceID:    trace.TraceIDFromContext(ctx),
		MessageID:  util.NewMessageID(),
		Timestamp:  nowMillis(),
		ProduceSvc: serviceName,
	}
}

// SendJSON 发送普通消息（同步）
func (p *Producer) SendJSON(ctx context.Context, topic string, body interface{}) error {
	msg, err := p.buildMessage(ctx, topic, body)
	if err != nil {
		return err
	}
	if _, err := p.p.SendSync(ctx, msg); err != nil {
		return fmt.Errorf("消息发送失败 topic=%s: %w", topic, err)
	}
	return nil
}

// SendDelayJSON 发送延迟消息（LLD §4：评审到期提醒，延迟等级见 topics.go）
func (p *Producer) SendDelayJSON(ctx context.Context, topic string, body interface{}, delayLevel int) error {
	msg, err := p.buildMessage(ctx, topic, body)
	if err != nil {
		return err
	}
	msg.WithDelayTimeLevel(delayLevel)
	if _, err := p.p.SendSync(ctx, msg); err != nil {
		return fmt.Errorf("延迟消息发送失败 topic=%s level=%d: %w", topic, delayLevel, err)
	}
	return nil
}

// SendTransactionJSON 发送事务消息
// 消息正式投递与否由 TransactionListener 中本地事务执行结果决定：
// 本地事务（如单据入库）所需数据请直接放入消息体，Listener 从 msg.Body 解析
func (p *Producer) SendTransactionJSON(ctx context.Context, topic string, body interface{}) error {
	if p.tp == nil {
		return fmt.Errorf("当前生产者非事务生产者，无法发送事务消息")
	}
	msg, err := p.buildMessage(ctx, topic, body)
	if err != nil {
		return err
	}
	if _, err := p.tp.SendMessageInTransaction(ctx, msg); err != nil {
		return fmt.Errorf("事务消息发送失败 topic=%s: %w", topic, err)
	}
	return nil
}

// Shutdown 优雅关闭
func (p *Producer) Shutdown() error {
	if p.p != nil {
		return p.p.Shutdown()
	}
	if p.tp != nil {
		return p.tp.Shutdown()
	}
	return nil
}
