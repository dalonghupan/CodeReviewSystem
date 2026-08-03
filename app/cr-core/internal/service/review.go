// Package service cr-core 业务逻辑层（LLD §3.3）
// 实现 ReviewService 全部 RPC：评审单CRUD、状态机流转、行级评论、Sonar门禁、Webhook
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "cr-system/api/crsystem/v1"
	"cr-system/app/cr-core/internal/bizadapter"
	"cr-system/app/cr-core/internal/data"
	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// ReviewService 评审核心服务
type ReviewService struct {
	v1.UnimplementedReviewServiceServer

	data  *data.Data
	sonar *bizadapter.SonarClient
	log   *log.Helper
}

// NewReviewService 构造服务
func NewReviewService(d *data.Data, sonarClient *bizadapter.SonarClient, logger log.Logger) *ReviewService {
	return &ReviewService{
		data:  d,
		sonar: sonarClient,
		log:   log.NewHelper(logger),
	}
}

// ==================== 评审单管理 ====================

// CreateReview 创建评审单（LLD §2 核心时序）
// 1. 分布式锁 lock:review_mr:{mrId} 防重复
// 2. 数据库唯一索引 idx_review_mr_unique 兜底
func (s *ReviewService) CreateReview(ctx context.Context, req *v1.CreateReviewReq) (*v1.ReviewDetail, error) {
	if req.TenantId == "" || req.RepoId == "" || req.MrId == "" || req.Title == "" {
		return nil, errcode.ErrParamInvalid
	}

	// 分布式锁（30s 超时，LLD §3.3-3）
	locked, err := s.data.AcquireReviewLock(ctx, req.MrId)
	if err != nil {
		return nil, errcode.ErrRedis.WithDetail(err.Error())
	}
	if !locked {
		return nil, errcode.ErrReviewAlreadyExists.WithDetail("该MR正在创建评审单，请勿重复操作")
	}
	defer s.data.ReleaseReviewLock(ctx, req.MrId)

	r := &data.ReviewMain{
		TenantID:    req.TenantId,
		RepoID:      req.RepoId,
		MRID:        req.MrId,
		GitPlatform: "", // 由 git-adapter 补充
		Title:       req.Title,
		Priority:    int(req.Priority),
		Status:      data.StatusPending,
	}
	if req.Deadline != nil {
		t := req.Deadline.AsTime()
		r.Deadline = &t
	}
	if req.RelatedIssue != "" {
		r.RelatedIssue = req.RelatedIssue
	}

	if err := s.data.CreateReview(ctx, r, req.ReviewerUids); err != nil {
		return nil, err
	}

	s.log.Infow("msg", "评审单创建成功", "review_id", r.ReviewID, "mr_id", req.MrId)
	return s.GetReview(ctx, &v1.GetReviewReq{ReviewId: r.ReviewID})
}

// GetReview 获取评审单详情
func (s *ReviewService) GetReview(ctx context.Context, req *v1.GetReviewReq) (*v1.ReviewDetail, error) {
	if req.ReviewId == "" {
		return nil, errcode.ErrParamInvalid
	}

	r, err := s.data.GetReview(ctx, req.ReviewId)
	if err != nil {
		return nil, err
	}

	reviewers, err := s.data.ListReviewers(ctx, req.ReviewId)
	if err != nil {
		return nil, err
	}

	return reviewToDetail(r, reviewers), nil
}

// ListReviews 评审单列表（多维筛选分页）
func (s *ReviewService) ListReviews(ctx context.Context, req *v1.ListReviewsReq) (*v1.ListReviewsResp, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}

	f := data.ListReviewsFilter{
		TenantID:   req.TenantId,
		RepoID:     req.RepoId,
		Status:     int(req.Status),
		CreatorUID: req.CreatorUid,
		Priority:   int(req.Priority),
		Keyword:    req.Keyword,
	}
	if req.ReviewerUid != "" {
		f.ReviewerUID = req.ReviewerUid
	}
	if req.StartTime != nil {
		t := req.StartTime.AsTime()
		f.StartTime = &t
	}
	if req.EndTime != nil {
		t := req.EndTime.AsTime()
		f.EndTime = &t
	}

	page, pageSize := paginationOf(req.Pagination)
	items, total, err := s.data.ListReviews(ctx, f, page, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &v1.ListReviewsResp{
		Pagination: paginationResp(page, pageSize, total),
	}
	for _, item := range items {
		// 获取每个评审单的评论/缺陷统计
		commentTotal, defectTotal, _ := s.data.CountComments(ctx, item.ReviewID)
		reviewers, _ := s.data.ListReviewers(ctx, item.ReviewID)
		resp.Items = append(resp.Items, reviewToSummary(item, reviewers, commentTotal, defectTotal))
	}
	return resp, nil
}

// UpdateReview 更新评审单（评审人、优先级、截止时间）
func (s *ReviewService) UpdateReview(ctx context.Context, req *v1.UpdateReviewReq) (*v1.ReviewDetail, error) {
	if req.ReviewId == "" {
		return nil, errcode.ErrParamInvalid
	}

	// 获取当前评审单
	r, err := s.data.GetReview(ctx, req.ReviewId)
	if err != nil {
		return nil, err
	}

	// 不允许在已归档状态更新
	if r.Status == data.StatusArchived {
		return nil, errcode.ErrReviewStatusInvalid.WithDetail("已归档评审单不可修改")
	}

	// 更新评审人（全量替换）
	if len(req.ReviewerUids) > 0 {
		if err := s.data.ReplaceReviewers(ctx, req.ReviewId, req.ReviewerUids); err != nil {
			return nil, err
		}
	}

	// 更新基础信息
	update := &data.ReviewMain{
		ReviewID:     req.ReviewId,
		Title:        r.Title,
		Priority:     int(req.Priority),
		RelatedIssue: req.RelatedIssue,
	}
	if req.Deadline != nil {
		t := req.Deadline.AsTime()
		update.Deadline = &t
	}
	if err := s.data.UpdateReview(ctx, update); err != nil {
		return nil, err
	}

	return s.GetReview(ctx, &v1.GetReviewReq{ReviewId: req.ReviewId})
}

// ==================== 状态流转（LLD §3.3 状态机）====================

// StartReview 开始评审：待评审(1) → 评审中(2)
func (s *ReviewService) StartReview(ctx context.Context, req *v1.StartReviewReq) (*v1.ReviewDetail, error) {
	if err := s.checkReviewOperator(ctx, req.ReviewId, req.OperatorUid, false); err != nil {
		return nil, err
	}
	// 校验操作人必须是评审人
	isReviewer, err := s.data.IsReviewer(ctx, req.ReviewId, req.OperatorUid)
	if err != nil {
		return nil, err
	}
	if !isReviewer {
		return nil, errcode.ErrReviewNotReviewer
	}

	if err := s.data.TransitionStatus(ctx, req.ReviewId, data.StatusPending, data.StatusInProgress,
		"start", req.OperatorUid, "", ""); err != nil {
		return nil, err
	}

	s.log.Infow("msg", "评审开始", "review_id", req.ReviewId, "operator", req.OperatorUid)
	return s.GetReview(ctx, &v1.GetReviewReq{ReviewId: req.ReviewId})
}

// RejectReview 驳回评审：评审中(2) → 驳回待修改(3)
func (s *ReviewService) RejectReview(ctx context.Context, req *v1.RejectReviewReq) (*v1.ReviewDetail, error) {
	if err := s.checkReviewOperator(ctx, req.ReviewId, req.OperatorUid, false); err != nil {
		return nil, err
	}
	isReviewer, err := s.data.IsReviewer(ctx, req.ReviewId, req.OperatorUid)
	if err != nil {
		return nil, err
	}
	if !isReviewer {
		return nil, errcode.ErrReviewNotReviewer
	}

	if err := s.data.TransitionStatus(ctx, req.ReviewId, data.StatusInProgress, data.StatusRejected,
		"reject", req.OperatorUid, req.Reason, "rejected"); err != nil {
		return nil, err
	}

	s.log.Infow("msg", "评审驳回", "review_id", req.ReviewId, "operator", req.OperatorUid, "reason", req.Reason)
	return s.GetReview(ctx, &v1.GetReviewReq{ReviewId: req.ReviewId})
}

// SubmitReReview 提交复审：驳回待修改(3) → 复审提交(4)
func (s *ReviewService) SubmitReReview(ctx context.Context, req *v1.SubmitReReviewReq) (*v1.ReviewDetail, error) {
	if err := s.checkReviewOperator(ctx, req.ReviewId, req.OperatorUid, false); err != nil {
		return nil, err
	}
	// 校验操作人必须是创建人
	r, err := s.data.GetReview(ctx, req.ReviewId)
	if err != nil {
		return nil, err
	}
	if r.CreatorUID != req.OperatorUid {
		return nil, errcode.ErrReviewNotCreator
	}

	if err := s.data.TransitionStatus(ctx, req.ReviewId, data.StatusRejected, data.StatusResubmitted,
		"resubmit", req.OperatorUid, req.Remark, ""); err != nil {
		return nil, err
	}

	s.log.Infow("msg", "提交复审", "review_id", req.ReviewId, "operator", req.OperatorUid)
	return s.GetReview(ctx, &v1.GetReviewReq{ReviewId: req.ReviewId})
}

// ApproveReview 通过评审：复审提交(4) → 复审通过(5)，含 Sonar 门禁校验
func (s *ReviewService) ApproveReview(ctx context.Context, req *v1.ApproveReviewReq) (*v1.ReviewDetail, error) {
	if err := s.checkReviewOperator(ctx, req.ReviewId, req.OperatorUid, false); err != nil {
		return nil, err
	}
	isReviewer, err := s.data.IsReviewer(ctx, req.ReviewId, req.OperatorUid)
	if err != nil {
		return nil, err
	}
	if !isReviewer {
		return nil, errcode.ErrReviewNotReviewer
	}

	// Sonar 门禁校验（如已配置）
	r, err := s.data.GetReview(ctx, req.ReviewId)
	if err != nil {
		return nil, err
	}
	if r.SonarPassFlag {
		// 已有 Sonar 记录时检查门禁
		hasSonar, _ := s.data.HasSonarResult(ctx, req.ReviewId)
		if hasSonar {
			gateRules, err := s.data.GetGateRules(ctx, r.TenantID)
			if err != nil {
				return nil, err
			}
			if len(gateRules) > 0 {
				for _, rule := range gateRules {
					var severities []string
					if err := json.Unmarshal(rule.SeverityLevels, &severities); err != nil {
						continue
					}
					passed, reason, err := s.data.CheckGatePassed(ctx, req.ReviewId, severities)
					if err != nil {
						return nil, err
					}
					if !passed {
						return nil, errcode.ErrSonarGateBlocked.WithDetail(reason)
					}
				}
			}
		}
	}

	if err := s.data.TransitionStatus(ctx, req.ReviewId, data.StatusResubmitted, data.StatusApproved,
		"approve", req.OperatorUid, req.Comment, "approved"); err != nil {
		return nil, err
	}

	s.log.Infow("msg", "复审通过", "review_id", req.ReviewId, "operator", req.OperatorUid)
	return s.GetReview(ctx, &v1.GetReviewReq{ReviewId: req.ReviewId})
}

// ArchiveReview 归档评审：复审通过(5) → 归档(6)
func (s *ReviewService) ArchiveReview(ctx context.Context, req *v1.ArchiveReviewReq) (*v1.ReviewDetail, error) {
	if err := s.data.MarkArchived(ctx, req.ReviewId, req.Trigger); err != nil {
		return nil, err
	}

	s.log.Infow("msg", "评审归档", "review_id", req.ReviewId, "trigger", req.Trigger)
	return s.GetReview(ctx, &v1.GetReviewReq{ReviewId: req.ReviewId})
}

// ==================== 代码行评论管理 ====================

// AddComment 添加代码行评论
func (s *ReviewService) AddComment(ctx context.Context, req *v1.AddCommentReq) (*v1.CommentInfo, error) {
	if req.ReviewId == "" || req.FilePath == "" || req.Content == "" {
		return nil, errcode.ErrParamInvalid
	}

	c := &data.ReviewComment{
		ReviewID:      req.ReviewId,
		FilePath:      req.FilePath,
		LineNum:       int(req.LineNum),
		CommitVersion: req.CommitVersion,
		Content:       req.Content,
		DefectLevel:   int(req.DefectLevel),
		CreateUID:     extractUID(ctx),
	}
	if err := s.data.AddComment(ctx, c); err != nil {
		return nil, err
	}

	return commentToInfo(c), nil
}

// ReplyComment 回复评论
func (s *ReviewService) ReplyComment(ctx context.Context, req *v1.ReplyCommentReq) (*v1.CommentInfo, error) {
	if req.ReviewId == "" || req.CommentId == "" || req.Content == "" {
		return nil, errcode.ErrParamInvalid
	}

	// 获取父评论
	parent, err := s.data.GetComment(ctx, req.CommentId)
	if err != nil {
		return nil, err
	}

	reply := &data.ReviewComment{
		ReviewID:      parent.ReviewID,
		FilePath:      parent.FilePath,
		LineNum:       parent.LineNum,
		CommitVersion: parent.CommitVersion,
		Content:       req.Content,
		CreateUID:     extractUID(ctx),
	}
	if err := s.data.ReplyComment(ctx, req.CommentId, reply); err != nil {
		return nil, err
	}

	return commentToInfo(reply), nil
}

// GetComments 获取评论（按文件分组）
func (s *ReviewService) GetComments(ctx context.Context, req *v1.GetCommentsReq) (*v1.GetCommentsResp, error) {
	if req.ReviewId == "" {
		return nil, errcode.ErrParamInvalid
	}

	items, err := s.data.GetComments(ctx, req.ReviewId, req.FilePath, req.CommitVersion, req.IncludeIsolated)
	if err != nil {
		return nil, err
	}

	// 按文件分组
	fileGroups := make(map[string]*v1.CommentGroup)
	var fileOrder []string
	for _, item := range items {
		if _, ok := fileGroups[item.FilePath]; !ok {
			fileGroups[item.FilePath] = &v1.CommentGroup{FilePath: item.FilePath}
			fileOrder = append(fileOrder, item.FilePath)
		}
		info := commentToInfo(item)

		// 顶层评论直接加入分组，回复挂到父评论
		if item.ReplyParentID == nil || *item.ReplyParentID == "" {
			fileGroups[item.FilePath].Comments = append(fileGroups[item.FilePath].Comments, info)
		} else {
			// 找到父评论并添加回复
			parentID := *item.ReplyParentID
			for _, c := range fileGroups[item.FilePath].Comments {
				if c.CommentId == parentID {
					c.Replies = append(c.Replies, info)
					break
				}
			}
		}
	}

	resp := &v1.GetCommentsResp{
		TotalCount:  uint32(len(items)),
		DefectCount: countDefects(items),
	}
	for _, fp := range fileOrder {
		resp.FileGroups = append(resp.FileGroups, fileGroups[fp])
	}
	return resp, nil
}

// UpdateComment 更新评论
func (s *ReviewService) UpdateComment(ctx context.Context, req *v1.UpdateCommentReq) (*v1.CommentInfo, error) {
	if req.ReviewId == "" || req.CommentId == "" {
		return nil, errcode.ErrParamInvalid
	}

	c, err := s.data.GetComment(ctx, req.CommentId)
	if err != nil {
		return nil, err
	}

	// 仅本人可编辑
	uid := extractUID(ctx)
	if c.CreateUID != uid {
		return nil, errcode.ErrCommentNoPermission
	}

	// 已有回复不可编辑
	hasReplies, err := s.data.HasReplies(ctx, req.CommentId)
	if err != nil {
		return nil, err
	}
	if hasReplies {
		return nil, errcode.ErrCommentAlreadyReplied
	}

	if err := s.data.UpdateComment(ctx, req.CommentId, req.Content, int(req.DefectLevel)); err != nil {
		return nil, err
	}

	return s.getCommentInfo(ctx, req.CommentId)
}

// DeleteComment 删除评论
func (s *ReviewService) DeleteComment(ctx context.Context, req *v1.DeleteCommentReq) (*v1.OperateResult, error) {
	if req.ReviewId == "" || req.CommentId == "" {
		return nil, errcode.ErrParamInvalid
	}

	c, err := s.data.GetComment(ctx, req.CommentId)
	if err != nil {
		return nil, err
	}

	// 仅本人可删除
	uid := extractUID(ctx)
	if c.CreateUID != uid {
		return nil, errcode.ErrCommentNoPermission
	}

	// 已有回复不可删除
	hasReplies, err := s.data.HasReplies(ctx, req.CommentId)
	if err != nil {
		return nil, err
	}
	if hasReplies {
		return nil, errcode.ErrCommentAlreadyReplied
	}

	if err := s.data.DeleteComment(ctx, req.CommentId); err != nil {
		return nil, err
	}

	return &v1.OperateResult{Success: true, Message: "评论已删除"}, nil
}

// ==================== SonarQube 质量门禁 ====================

// GetSonarResult 获取 Sonar 扫描结果
func (s *ReviewService) GetSonarResult(ctx context.Context, req *v1.GetSonarResultReq) (*v1.SonarResult, error) {
	if req.ReviewId == "" {
		return nil, errcode.ErrParamInvalid
	}

	r, err := s.data.GetSonarResult(ctx, req.ReviewId)
	if err != nil {
		return nil, err
	}

	return sonarResultToProto(r), nil
}

// CheckSonarGate 检查 Sonar 门禁是否通过
func (s *ReviewService) CheckSonarGate(ctx context.Context, req *v1.CheckSonarGateReq) (*v1.CheckSonarGateResp, error) {
	if req.ReviewId == "" {
		return nil, errcode.ErrParamInvalid
	}

	r, err := s.data.GetReview(ctx, req.ReviewId)
	if err != nil {
		return nil, err
	}

	sonar, err := s.data.GetSonarResult(ctx, req.ReviewId)
	if err != nil {
		return &v1.CheckSonarGateResp{Passed: false, Reason: "Sonar扫描数据暂不可用"}, nil
	}

	// 获取租户门禁规则
	rules, err := s.data.GetGateRules(ctx, r.TenantID)
	if err != nil {
		return nil, err
	}

	// 解析 issues
	if len(sonar.IssuesJSON) > 0 {
		var issues []v1.SonarIssue
		if err := json.Unmarshal(sonar.IssuesJSON, &issues); err == nil && len(issues) > 0 {
			// 构建阻断级别集合
			blockedSet := make(map[string]bool)
			for _, rule := range rules {
				var sevs []string
				if err := json.Unmarshal(rule.SeverityLevels, &sevs); err == nil {
					for _, s := range sevs {
						blockedSet[s] = true
					}
				}
			}

			// 默认阻断 BLOCKER + CRITICAL
			if len(blockedSet) == 0 {
				blockedSet["BLOCKER"] = true
				blockedSet["CRITICAL"] = true
			}

			var blockingIssues []*v1.SonarIssue
			for _, issue := range issues {
				if blockedSet[issue.Severity] && issue.Status != "CLOSED" && issue.Status != "RESOLVED" {
					blockingIssues = append(blockingIssues, &issue)
				}
			}

			if len(blockingIssues) > 0 {
				reasons := fmt.Sprintf("存在 %d 个未修复的阻断性问题", len(blockingIssues))
				return &v1.CheckSonarGateResp{
					Passed:         false,
					Reason:         reasons,
					BlockingIssues: blockingIssues,
				}, nil
			}
		}
	}

	// 质量门全局状态
	if sonar.QualityGateStatus == "ERROR" {
		return &v1.CheckSonarGateResp{Passed: false, Reason: "Sonar质量门状态为ERROR"}, nil
	}

	return &v1.CheckSonarGateResp{Passed: true, Reason: "门禁通过"}, nil
}

// ==================== Webhook ====================

// HandleWebhook 接收 Git 平台 Webhook 回调（MR合并/关闭）
func (s *ReviewService) HandleWebhook(ctx context.Context, req *v1.WebhookPayload) (*v1.OperateResult, error) {
	if req.RepoId == "" || req.MrId == "" {
		return nil, errcode.ErrParamInvalid
	}

	switch req.EventType {
	case "mr_merged":
		// MR 合并 → 归档评审单
		review, err := s.data.GetReviewByMR(ctx, req.RepoId, req.MrId)
		if err != nil {
			return nil, err
		}
		if review.Status == data.StatusApproved {
			if err := s.data.MarkArchived(ctx, review.ReviewID, "webhook:mr_merged"); err != nil {
				return nil, err
			}
		}
		return &v1.OperateResult{Success: true, Message: "MR合并事件处理完成"}, nil

	case "mr_closed":
		// MR 关闭 → 取消进行中的评审单（归档）
		review, err := s.data.GetReviewByMR(ctx, req.RepoId, req.MrId)
		if err != nil {
			return nil, err
		}
		if review.Status != data.StatusArchived {
			if err := s.data.MarkArchived(ctx, review.ReviewID, "webhook:mr_closed"); err != nil {
				return nil, err
			}
		}
		return &v1.OperateResult{Success: true, Message: "MR关闭事件处理完成"}, nil

	case "push":
		// Push 事件 → 更新 commit 版本、隔离旧评论
		if req.CommitHash != "" {
			review, err := s.data.GetReviewByMR(ctx, req.RepoId, req.MrId)
			if err != nil {
				return nil, err
			}
			// 隔离所有旧版本的评论
			if err := s.data.IsolateOldComments(ctx, review.ReviewID, req.CommitHash); err != nil {
				return nil, err
			}
		}
		return &v1.OperateResult{Success: true, Message: "Push事件处理完成"}, nil

	default:
		s.log.Warnw("msg", "未知Webhook事件类型", "event_type", req.EventType)
		return &v1.OperateResult{Success: true, Message: "事件类型已忽略"}, nil
	}
}

// ==================== 辅助方法 ====================

// checkReviewOperator 校验评审单存在且状态允许操作
func (s *ReviewService) checkReviewOperator(ctx context.Context, reviewID, operatorUID string, requireCreator bool) error {
	if reviewID == "" || operatorUID == "" {
		return errcode.ErrParamInvalid
	}
	r, err := s.data.GetReview(ctx, reviewID)
	if err != nil {
		return err
	}
	if r.Status == data.StatusArchived {
		return errcode.ErrReviewStatusInvalid.WithDetail("评审单已归档")
	}
	return nil
}

// extractUID 从上下文中提取用户ID（由 JWT 中间件注入，LLD §9-2）
func extractUID(ctx context.Context) string {
	if uid, ok := ctx.Value("user_id").(string); ok {
		return uid
	}
	return ""
}

// getCommentInfo 获取单条评论的 proto 对象
func (s *ReviewService) getCommentInfo(ctx context.Context, commentID string) (*v1.CommentInfo, error) {
	c, err := s.data.GetComment(ctx, commentID)
	if err != nil {
		return nil, err
	}
	return commentToInfo(c), nil
}

// countDefects 统计缺陷评论数
func countDefects(items []*data.ReviewComment) uint32 {
	var count uint32
	for _, item := range items {
		if item.DefectLevel > 0 {
			count++
		}
	}
	return count
}

// ==================== 转换辅助 ====================

func reviewToDetail(r *data.ReviewMain, reviewers []*data.ReviewReviewer) *v1.ReviewDetail {
	d := &v1.ReviewDetail{
		ReviewId:        r.ReviewID,
		TenantId:        r.TenantID,
		RepoId:          r.RepoID,
		MrId:            r.MRID,
		Title:           r.Title,
		Description:     r.Description,
		CreatorUid:      r.CreatorUID,
		Status:          v1.ReviewStatus(r.Status),
		Priority:        v1.ReviewPriority(r.Priority),
		SourceBranch:    r.SourceBranch,
		TargetBranch:    r.TargetBranch,
		CommitCount:     uint32(r.CommitCount),
		ChangedFileCount: uint32(r.ChangedFileCount),
		RelatedIssue:    r.RelatedIssue,
		SonarPass:       r.SonarPassFlag,
		CreatedAt:       timestamppb.New(r.CreatedAt),
		UpdatedAt:       timestamppb.New(r.UpdatedAt),
	}
	if r.Deadline != nil {
		d.Deadline = timestamppb.New(*r.Deadline)
	}
	if r.ArchiveTime != nil {
		d.ArchiveTime = timestamppb.New(*r.ArchiveTime)
	}
	for _, rev := range reviewers {
		info := &v1.ReviewerInfo{
			Uid:          rev.ReviewerUID,
			ReviewStatus: rev.ReviewStatus,
		}
		if rev.ReviewedAt != nil {
			info.ReviewedAt = timestamppb.New(*rev.ReviewedAt)
		}
		d.Reviewers = append(d.Reviewers, info)
	}
	return d
}

func reviewToSummary(r *data.ReviewMain, reviewers []*data.ReviewReviewer, commentCount, defectCount uint32) *v1.ReviewSummary {
	s := &v1.ReviewSummary{
		ReviewId:     r.ReviewID,
		TenantId:     r.TenantID,
		RepoId:       r.RepoID,
		MrId:         r.MRID,
		Title:        r.Title,
		CreatorUid:   r.CreatorUID,
		Status:       v1.ReviewStatus(r.Status),
		Priority:     v1.ReviewPriority(r.Priority),
		CommentCount: commentCount,
		DefectCount:  defectCount,
		CreatedAt:    timestamppb.New(r.CreatedAt),
		UpdatedAt:    timestamppb.New(r.UpdatedAt),
	}
	if r.Deadline != nil {
		s.Deadline = timestamppb.New(*r.Deadline)
	}
	for _, rev := range reviewers {
		s.ReviewerNames = append(s.ReviewerNames, rev.ReviewerUID)
	}
	return s
}

func commentToInfo(c *data.ReviewComment) *v1.CommentInfo {
	info := &v1.CommentInfo{
		CommentId:     c.CommentID,
		ReviewId:      c.ReviewID,
		FilePath:      c.FilePath,
		LineNum:       uint32(c.LineNum),
		CommitVersion: c.CommitVersion,
		Content:       c.Content,
		DefectLevel:   v1.DefectLevel(c.DefectLevel),
		CreateUid:     c.CreateUID,
		IsIsolated:    c.IsIsolated,
		CreatedAt:     timestamppb.New(c.CreatedAt),
		UpdatedAt:     timestamppb.New(c.UpdatedAt),
	}
	if c.ReplyParentID != nil {
		info.ReplyParentId = *c.ReplyParentID
	}
	return info
}

func sonarResultToProto(r *data.SonarResult) *v1.SonarResult {
	result := &v1.SonarResult{
		ReviewId:         r.ReviewID,
		ProjectKey:       r.ProjectKey,
		QualityGateStatus: r.QualityGateStatus,
		ScannedAt:        timestamppb.New(r.ScannedAt),
	}
	// 解析 issues
	if len(r.IssuesJSON) > 0 {
		var issues []*v1.SonarIssue
		if err := json.Unmarshal(r.IssuesJSON, &issues); err == nil {
			result.Issues = issues
		}
	}
	// 解析 metrics
	if len(r.MetricsJSON) > 0 {
		var metrics v1.SonarMetrics
		if err := json.Unmarshal(r.MetricsJSON, &metrics); err == nil {
			result.Metrics = &metrics
		}
	}
	return result
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
var _ v1.ReviewServiceServer = (*ReviewService)(nil)
