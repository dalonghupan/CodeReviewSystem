// Package data job-scheduler 任务定义数据访问
// 任务元数据硬编码（K8s CronJob + 内部任务管理器，不引入额外定时中间件）
// LLD §3.6 任务清单
package data

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "cr-system/api/crsystem/v1"
)

// JobStore 任务定义存储器（内存 + DB 回写状态）
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*JobDefinition
}

// NewJobStore 初始化默认任务定义
// 任务清单对应 LLD §3.6 及 job_scheduler.proto JobType 枚举
func NewJobStore() *JobStore {
	store := &JobStore{
		jobs: make(map[string]*JobDefinition),
	}
	store.initDefaults()
	return store
}

func (s *JobStore) initDefaults() {
	now := time.Now()
	defs := []*JobDefinition{
		{
			JobID:       "job-repo-sync",
			JobType:     v1.JobType_JOB_TYPE_REPO_INCREMENTAL_SYNC.String(),
			Name:        "仓库增量同步",
			Description: "每6小时增量同步仓库MR变更（调用 git-adapter 拉取最新MR）",
			CronExpr:    "0 */6 * * *",
			Enabled:     true,
			LastStatus:  "unknown",
		},
		{
			JobID:       "job-data-reconciliation",
			JobType:     v1.JobType_JOB_TYPE_DATA_RECONCILIATION.String(),
			Name:        "数据对账",
			Description: "每日01:00 PG单据与MQ消息对账，修复不一致数据",
			CronExpr:    "0 1 * * *",
			Enabled:     true,
			LastStatus:  "unknown",
		},
		{
			JobID:       "job-cache-cleanup",
			JobType:     v1.JobType_JOB_TYPE_CACHE_CLEANUP.String(),
			Name:        "缓存清理",
			Description: "每日02:00 Redis过期缓存清理（diff缓存/repo信息缓存/权限缓存）",
			CronExpr:    "0 2 * * *",
			Enabled:     true,
			LastStatus:  "unknown",
		},
		{
			JobID:       "job-token-refresh",
			JobType:     v1.JobType_JOB_TYPE_TOKEN_REFRESH.String(),
			Name:        "Token刷新",
			Description: "每日03:00 Git授权Token自动轮换刷新，发送 token_refresh_topic",
			CronExpr:    "0 3 * * *",
			Enabled:     true,
			LastStatus:  "unknown",
		},
		{
			JobID:       "job-monthly-report",
			JobType:     v1.JobType_JOB_TYPE_MONTHLY_REPORT.String(),
			Name:        "月报生成",
			Description: "每月首日全租户质量月报生成（调用 quality-stat 报表功能）",
			CronExpr:    "0 0 1 * *",
			Enabled:     true,
			LastStatus:  "unknown",
		},
		{
			JobID:       "job-keycloak-sync",
			JobType:     v1.JobType_JOB_TYPE_KEYCLOAK_USER_SYNC.String(),
			Name:        "Keycloak账号同步",
			Description: "每日凌晨 Keycloak 账号同步（调用 iam-service 同步接口）",
			CronExpr:    "0 4 * * *",
			Enabled:     true,
			LastStatus:  "unknown",
		},
	}
	for _, j := range defs {
		j.LastRunAt = nil
		// 下一执行时间：从当前时间推算
		next := j.NextRunAfter(now)
		j.NextRunAt = &next
		s.jobs[j.JobID] = j
	}
}

// NextRunAfter 根据 cron 表达式推算下次执行时间（简化版）
// 正式环境应使用 robfig/cron 库解析
func (j *JobDefinition) NextRunAfter(from time.Time) time.Time {
	if !j.Enabled {
		return time.Time{}
	}
	// 简化实现：按常见间隔推算
	switch j.JobType {
	case v1.JobType_JOB_TYPE_REPO_INCREMENTAL_SYNC.String():
		return from.Truncate(6 * time.Hour).Add(6 * time.Hour)
	case v1.JobType_JOB_TYPE_DATA_RECONCILIATION.String():
		next := time.Date(from.Year(), from.Month(), from.Day(), 1, 0, 0, 0, from.Location())
		if !next.After(from) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	case v1.JobType_JOB_TYPE_CACHE_CLEANUP.String():
		next := time.Date(from.Year(), from.Month(), from.Day(), 2, 0, 0, 0, from.Location())
		if !next.After(from) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	case v1.JobType_JOB_TYPE_TOKEN_REFRESH.String():
		next := time.Date(from.Year(), from.Month(), from.Day(), 3, 0, 0, 0, from.Location())
		if !next.After(from) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	case v1.JobType_JOB_TYPE_MONTHLY_REPORT.String():
		next := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location())
		if !next.After(from) {
			next = next.AddDate(0, 1, 0)
		}
		return next
	case v1.JobType_JOB_TYPE_KEYCLOAK_USER_SYNC.String():
		next := time.Date(from.Year(), from.Month(), from.Day(), 4, 0, 0, 0, from.Location())
		if !next.After(from) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	}
	return from.Add(24 * time.Hour)
}

// List 获取所有任务定义
func (s *JobStore) List(jobType string, enabledOnly bool) []*JobDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*JobDefinition, 0, len(s.jobs))
	for _, j := range s.jobs {
		if jobType != "" && j.JobType != jobType {
			continue
		}
		if enabledOnly && !j.Enabled {
			continue
		}
		result = append(result, j)
	}
	return result
}

// Get 获取单个任务定义
func (s *JobStore) Get(jobID string) (*JobDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	j, ok := s.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job %s not found", jobID)
	}
	return j, nil
}

// SetEnabled 启用/停用任务
func (s *JobStore) SetEnabled(jobID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	j.Enabled = enabled
	return nil
}

// UpdateRunState 更新任务运行状态（执行后回写）
func (s *JobStore) UpdateRunState(jobID, status string, runAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	j, ok := s.jobs[jobID]
	if !ok {
		return
	}
	j.LastRunAt = &runAt
	j.LastStatus = status
	next := j.NextRunAfter(runAt)
	j.NextRunAt = &next
}

// ==================== 执行记录持久化 ====================

// InsertExecutionLog 插入执行记录
func (d *Data) InsertExecutionLog(ctx context.Context, log *JobExecutionLog) error {
	query := `INSERT INTO job_execution_log
		(id, job_name, job_type, status, start_time, end_time, duration_ms, result_summary, error_message, created_at)
		VALUES (:id, :job_name, :job_type, :status, :start_time, :end_time, :duration_ms, :result_summary, :error_message, :created_at)`
	_, err := d.writeDB.NamedExecContext(ctx, query, log)
	return err
}

// UpdateExecutionLog 更新执行记录（结束时间、状态、结果）
func (d *Data) UpdateExecutionLog(ctx context.Context, id, status string, endTime time.Time, durationMs int64, resultSummary, errorMessage string) error {
	query := `UPDATE job_execution_log SET
		status = $1, end_time = $2, duration_ms = $3, result_summary = $4, error_message = $5
		WHERE id = $6`
	_, err := d.writeDB.ExecContext(ctx, query, status, endTime, durationMs, resultSummary, errorMessage, id)
	return err
}

// ListExecutionLogs 分页查询执行记录
func (d *Data) ListExecutionLogs(ctx context.Context, jobID, status string, page, pageSize uint32) ([]JobExecutionLog, uint32, error) {
	// 解析 job_name 从 job_id
	jobName := jobID

	var total uint32
	countQuery := `SELECT COUNT(*) FROM job_execution_log WHERE job_name = $1`
	args := []interface{}{jobName}
	argIdx := 2

	if status != "" {
		countQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if err := d.readDB.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, job_name, job_type, status, start_time, end_time, duration_ms, result_summary, error_message, created_at
		FROM job_execution_log WHERE job_name = $1`
	queryArgs := []interface{}{jobName}
	qIdx := 2
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", qIdx)
		queryArgs = append(queryArgs, status)
		qIdx++
	}
	query += " ORDER BY start_time DESC LIMIT $%d OFFSET $%d"
	query = fmt.Sprintf(query, qIdx, qIdx+1)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)

	var items []JobExecutionLog
	if err := d.readDB.SelectContext(ctx, &items, query, queryArgs...); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
