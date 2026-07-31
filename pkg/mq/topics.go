// Package mq 定义全系统 RocketMQ Topic 常量与 Consumer Group 命名规范
// 对应 LLD §4 RocketMQ消息详细设计
package mq

// Topic 名称常量（LLD §4 消息主题定义）
const (
	// TopicReviewCodeFetch 创建评审单后触发代码拉取（事务消息）
	// 生产者：cr-core（创建评审单事务内）
	// 消费者：git-adapter（拉取MR变更文件、解析Diff）
	TopicReviewCodeFetch = "review_code_fetch_topic"

	// TopicReviewNotice 评审通知推送（普通消息 + 延迟消息）
	// 生产者：cr-core（评审状态变更时）
	// 消费者：message-push（分发站内信/企业微信/邮件）
	TopicReviewNotice = "review_notice_topic"

	// TopicReviewFinish 评审归档完结事件（普通消息）
	// 生产者：cr-core（评审单归档时）
	// 消费者：quality-stat（归集缺陷数据、更新统计）
	TopicReviewFinish = "review_finish_topic"

	// TopicTokenRefresh Git授权Token刷新提醒（定时触发消息）
	// 生产者：job-scheduler（定时任务触发）
	// 消费者：git-adapter（执行Token刷新逻辑）
	TopicTokenRefresh = "token_refresh_topic"
)

// Consumer Group 命名规范
const (
	GroupGitAdapterCodeFetch = "GID_git_adapter_code_fetch"
	GroupMessagePushNotice   = "GID_message_push_notice"
	GroupQualityStatFinish   = "GID_quality_stat_finish"
	GroupGitAdapterToken     = "GID_git_adapter_token_refresh"
)

// 延迟消息等级（RocketMQ延迟等级 → 实际时间映射）
// 用于评审到期提醒（SRS F04-03：截止前1天发送提醒）
const (
	DelayLevel1Day    = 14 // RocketMQ延迟等级14 ≈ 1天
	DelayLevel1Hour   = 9  // RocketMQ延迟等级9 ≈ 1小时（超时催办周期）
)

// 消息重试配置
const (
	MaxRetryCount = 3 // 最大重试次数，超过后转入死信队列（LLD §4 补充规则）
)
