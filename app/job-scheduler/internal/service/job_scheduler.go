// Package service job-scheduler 业务逻辑层（LLD §3.6）
// 实现 JobSchedulerService 全部 RPC：任务查询、手动触发、启用/停用、执行记录
package service

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "cr-system/api/crsystem/v1"
	"cr-system/app/job-scheduler/internal/bizadapter"
	"cr-system/app/job-scheduler/internal/data"
	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// JobSchedulerService 定时任务服务
type JobSchedulerService struct {
	v1.UnimplementedJobSchedulerServiceServer

	store    *data.JobStore
	executor *bizadapter.JobExecutor
	data     *data.Data
	log      *log.Helper
}

// NewJobSchedulerService 构造服务
func NewJobSchedulerService(store *data.JobStore, executor *bizadapter.JobExecutor, d *data.Data, logger log.Logger) *JobSchedulerService {
	return &JobSchedulerService{
		store:    store,
		executor: executor,
		data:     d,
		log:      log.NewHelper(logger),
	}
}

// ==================== 任务管理 ====================

// ListJobs 获取定时任务列表
func (s *JobSchedulerService) ListJobs(ctx context.Context, req *v1.ListJobsReq) (*v1.ListJobsResp, error) {
	enabledOnly := req.EnabledOnly
	items := s.store.List(req.JobType, enabledOnly)

	resp := &v1.ListJobsResp{}
	for _, item := range items {
		resp.Items = append(resp.Items, jobToProto(item))
	}
	return resp, nil
}

// GetJob 获取任务详情
func (s *JobSchedulerService) GetJob(ctx context.Context, req *v1.GetJobReq) (*v1.JobInfo, error) {
	if req.JobId == "" {
		return nil, errcode.ErrParamInvalid
	}
	job, err := s.store.Get(req.JobId)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return jobToProto(job), nil
}

// SetJobEnabled 启用/停用任务
func (s *JobSchedulerService) SetJobEnabled(ctx context.Context, req *v1.SetJobEnabledReq) (*v1.OperateResult, error) {
	if req.JobId == "" {
		return nil, errcode.ErrParamInvalid
	}
	if err := s.store.SetEnabled(req.JobId, req.Enabled); err != nil {
		return nil, errcode.ErrNotFound
	}
	action := "已停用"
	if req.Enabled {
		action = "已启用"
	}
	return &v1.OperateResult{Success: true, Message: "任务" + action}, nil
}

// ==================== 手动触发 ====================

// TriggerJob 手动触发任务执行
func (s *JobSchedulerService) TriggerJob(ctx context.Context, req *v1.TriggerJobReq) (*v1.JobExecutionInfo, error) {
	if req.JobId == "" {
		return nil, errcode.ErrParamInvalid
	}

	job, err := s.store.Get(req.JobId)
	if err != nil {
		return nil, errcode.ErrNotFound
	}

	if !job.Enabled {
		return nil, errcode.ErrForbidden.WithDetail("任务已停用，无法触发")
	}

	// 异步执行任务
	go func(j *data.JobDefinition, param string) {
		execCtx := context.Background()
		execCtx, cancel := context.WithTimeout(execCtx, 60*time.Second)
		defer cancel()

		execID, affected, summary, execErr := s.executor.ExecuteJob(execCtx, j, param)
		status := "success"
		if execErr != nil {
			status = "failed"
			s.log.Errorw("msg", "任务执行失败", "job_id", j.JobID, "error", execErr.Error())
		}

		// 回写任务运行状态
		s.store.UpdateRunState(j.JobID, status, time.Now())

		s.log.Infow("msg", "任务执行完成", "job_id", j.JobID,
			"exec_id", execID, "status", status, "affected", affected, "summary", summary)
	}(job, req.Param)

	// 返回执行中的记录信息
	now := time.Now()
	return &v1.JobExecutionInfo{
		ExecutionId:   "",
		JobId:         job.JobID,
		JobType:       parseJobType(job.JobType),
		TriggerType:   "manual",
		Status:        "running",
		AffectedCount: 0,
		TraceId:       "",
		StartedAt:     timestamppb.New(now),
	}, nil
}

// ==================== 执行记录 ====================

// ListJobExecutions 获取任务执行记录
func (s *JobSchedulerService) ListJobExecutions(ctx context.Context, req *v1.ListJobExecutionsReq) (*v1.ListJobExecutionsResp, error) {
	if req.JobId == "" {
		return nil, errcode.ErrParamInvalid
	}
	page, pageSize := paginationOf(req.Pagination)
	items, total, err := s.data.ListExecutionLogs(ctx, req.JobId, req.Status, page, pageSize)
	if err != nil {
		s.log.Errorw("msg", "查询执行记录失败", "job_id", req.JobId, "error", err.Error())
		return nil, errcode.ErrInternal
	}

	resp := &v1.ListJobExecutionsResp{
		Pagination: paginationResp(page, pageSize, total),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, executionToProto(item))
	}
	return resp, nil
}

// ==================== 辅助转换 ====================

func jobToProto(j *data.JobDefinition) *v1.JobInfo {
	info := &v1.JobInfo{
		JobId:       j.JobID,
		JobType:     parseJobType(j.JobType),
		Name:        j.Name,
		Description: j.Description,
		CronExpr:    j.CronExpr,
		Enabled:     j.Enabled,
		LastStatus:  j.LastStatus,
	}
	if j.LastRunAt != nil {
		info.LastRunAt = timestamppb.New(*j.LastRunAt)
	}
	if j.NextRunAt != nil {
		info.NextRunAt = timestamppb.New(*j.NextRunAt)
	}
	return info
}

func executionToProto(e data.JobExecutionLog) *v1.JobExecutionInfo {
	info := &v1.JobExecutionInfo{
		ExecutionId:   e.ID,
		JobId:         e.JobName,
		JobType:       parseJobType(e.JobType),
		TriggerType:   "cron",
		Status:        e.Status,
		AffectedCount: 0,
		ErrorMessage:  e.ErrorMessage,
		StartedAt:     timestamppb.New(e.StartTime),
	}
	if e.EndTime != nil {
		info.FinishedAt = timestamppb.New(*e.EndTime)
	}
	return info
}

func parseJobType(s string) v1.JobType {
	if v, ok := v1.JobType_value[s]; ok {
		return v1.JobType(v)
	}
	return v1.JobType_JOB_TYPE_UNSPECIFIED
}

func paginationOf(p *v1.Pagination) (uint32, uint32) {
	if p == nil {
		return util.NormalizePage(0, 0)
	}
	return util.NormalizePage(p.Page, p.PageSize)
}

func paginationResp(page, pageSize, total uint32) *v1.PaginationResp {
	return &v1.PaginationResp{
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: util.TotalPages(total, pageSize),
	}
}

// Ensure interface compliance
var _ v1.JobSchedulerServiceServer = (*JobSchedulerService)(nil)
