// Package mq 定义全系统 RocketMQ 消息体结构
// 所有 MQ 消息统一使用 JSON 序列化
// 对应 LLD §4 RocketMQ消息详细设计
package mq

import "time"

// ==================== 基础消息头 ====================

// MessageHeader 所有MQ消息公共头部字段
type MessageHeader struct {
	TraceID    string `json:"trace_id"`    // 全链路追踪ID（LLD §1.3：APISIX生成，贯穿全链路）
	MessageID  string `json:"message_id"`  // 消息唯一ID（用于幂等校验）
	Timestamp  int64  `json:"timestamp"`   // 消息产生时间戳（Unix毫秒）
	ProduceSvc string `json:"produce_svc"` // 生产服务名
}

// ==================== review_code_fetch_topic ====================
// 事务消息：创建评审单成功 → 通知 git-adapter 拉取代码Diff

// CodeFetchMessage 代码拉取消息体
// Topic: review_code_fetch_topic
// 生产：cr-core（创建评审单事务内）
// 消费：git-adapter
type CodeFetchMessage struct {
	MessageHeader
	ReviewID    string `json:"review_id"`    // 评审单ID
	TenantID    string `json:"tenant_id"`    // 租户ID
	RepoID      string `json:"repo_id"`      // 仓库ID
	MRID        string `json:"mr_id"`        // 平台MR编号
	Platform    string `json:"platform"`     // gitlab / gitee / github
	AuthID      string `json:"auth_id"`      // Git授权记录ID（git-adapter用于选择Token）
	CommitHash  string `json:"commit_hash"`  // 当前最新commit hash
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

// ==================== review_notice_topic ====================
// 普通消息 + 延迟消息：评审状态变更通知推送

// NoticeEventType 通知事件类型
type NoticeEventType string

const (
	NoticeReviewAssigned   NoticeEventType = "review_assigned"    // 新建评审指派本人
	NoticeReviewRejected   NoticeEventType = "review_rejected"    // 评审驳回
	NoticeReviewResubmitted NoticeEventType = "review_resubmitted" // 提交复审
	NoticeReviewCompleted  NoticeEventType = "review_completed"   // 评审完成归档
	NoticeReviewTimeout    NoticeEventType = "review_timeout"     // 评审超时预警（延迟消息触发）
	NoticeReviewDeadline   NoticeEventType = "review_deadline"    // 评审到期提醒（延迟消息，截止前1天）
)

// ReviewNoticeMessage 评审通知消息体
// Topic: review_notice_topic
// 生产：cr-core
// 消费：message-push
type ReviewNoticeMessage struct {
	MessageHeader
	EventType   NoticeEventType `json:"event_type"`   // 事件类型
	TenantID    string          `json:"tenant_id"`
	ReviewID    string          `json:"review_id"`
	ReviewTitle string          `json:"review_title"`
	RepoName    string          `json:"repo_name"`
	MRID        string          `json:"mr_id"`
	OperatorUID string          `json:"operator_uid"`  // 触发操作的人
	OperatorName string         `json:"operator_name"`
	TargetUIDs  []string        `json:"target_uids"`   // 通知目标用户UID列表
	Remark      string          `json:"remark"`        // 附加备注（驳回原因等）
	Deadline    *time.Time      `json:"deadline"`      // 评审截止时间（用于超时计算）
}

// ==================== review_finish_topic ====================
// 普通消息：评审归档完结，触发缺陷归集与统计

// ReviewFinishMessage 评审完结消息体
// Topic: review_finish_topic
// 生产：cr-core（评审归档时）
// 消费：quality-stat
type ReviewFinishMessage struct {
	MessageHeader
	ReviewID        string `json:"review_id"`
	TenantID        string `json:"tenant_id"`
	RepoID          string `json:"repo_id"`
	MRID            string `json:"mr_id"`
	CreatorUID      string `json:"creator_uid"`
	ModuleName      string `json:"module_name"`      // 代码模块名（用于模块缺陷统计）
	TotalComments   int    `json:"total_comments"`    // 总评论数
	DefectComments  int    `json:"defect_comments"`   // 标记为缺陷的评论数
	DefectIDs       []string `json:"defect_ids"`      // 缺陷ID列表（已归集）
	ArchiveTime     int64  `json:"archive_time"`      // 归档时间戳
	CreatedTime     int64  `json:"created_time"`      // 创建时间戳（用于计算评审耗时）
}

// ==================== token_refresh_topic ====================
// 定时触发消息：Git授权Token自动刷新

// TokenRefreshMessage Token刷新消息体
// Topic: token_refresh_topic
// 生产：job-scheduler（定时任务）
// 消费：git-adapter
type TokenRefreshMessage struct {
	MessageHeader
	AuthID     string `json:"auth_id"`      // 授权记录ID
	TenantID   string `json:"tenant_id"`
	Platform   string `json:"platform"`
	ExpireTime int64  `json:"expire_time"`  // 当前Token过期时间戳（用于判断是否需刷新）
}

// ==================== 死信队列消息 ====================

// DeadLetterMessage 死信消息封装（重试超过MaxRetryCount后自动进入）
// 运维后台可查询并重推处理
type DeadLetterMessage struct {
	OriginalTopic   string `json:"original_topic"`    // 原始Topic
	OriginalBody    []byte `json:"original_body"`     // 原始消息体JSON
	RetryCount      int    `json:"retry_count"`       // 已重试次数
	LastErrorMsg    string `json:"last_error_msg"`    // 最后一次失败原因
	DeadLetterTime  int64  `json:"dead_letter_time"`  // 进入死信队列时间
}
