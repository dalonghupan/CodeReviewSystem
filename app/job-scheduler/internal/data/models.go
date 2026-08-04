// Package data job-scheduler 数据模型（LLD §3.6）
package data

import "time"

// JobDefinition 定时任务定义（硬编码 + DB 持久化）
// 任务元数据由服务启动时加载，执行记录写入 job_execution_log
type JobDefinition struct {
	JobID       string `db:"job_id"`
	JobType     string `db:"job_type"`
	Name        string `db:"name"`
	Description string `db:"description"`
	CronExpr    string `db:"cron_expr"`
	Enabled     bool   `db:"enabled"`

	// 运行时状态（不持久化）
	LastRunAt  *time.Time
	NextRunAt  *time.Time
	LastStatus string
}

// JobExecutionLog 定时任务执行记录（对应 SQL: job_execution_log）
type JobExecutionLog struct {
	ID             string     `db:"id"`
	JobName        string     `db:"job_name"`
	JobType        string     `db:"job_type"`
	Status         string     `db:"status"` // running / success / failed
	StartTime      time.Time  `db:"start_time"`
	EndTime        *time.Time `db:"end_time"`
	DurationMs     *int64     `db:"duration_ms"`
	ResultSummary  string     `db:"result_summary"`
	ErrorMessage   string     `db:"error_message"`
	CreatedAt      time.Time  `db:"created_at"`
}

// MQReconciliation MQ消息对账记录（对应 SQL: mq_reconciliation）
type MQReconciliation struct {
	ID            string     `db:"id"`
	Topic         string     `db:"topic"`
	MessageID     string     `db:"message_id"`
	BusinessID    string     `db:"business_id"`
	BusinessType  string     `db:"business_type"`
	ProduceTime   time.Time  `db:"produce_time"`
	ConsumeTime   *time.Time `db:"consume_time"`
	Status        string     `db:"status"` // pending / consumed / reconciled / failed
	RetryCount    int        `db:"retry_count"`
	ReconcileNote string     `db:"reconcile_note"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}
