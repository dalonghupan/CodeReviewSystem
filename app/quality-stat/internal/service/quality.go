// Package service quality-stat 业务逻辑层（LLD §3.5）
// 实现 QualityStatService 全部 RPC：缺陷台账、质量统计、数据大盘、报表
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "cr-system/api/crsystem/v1"
	"cr-system/app/quality-stat/internal/bizadapter"
	"cr-system/app/quality-stat/internal/data"
	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// QualityStatService 质量统计服务
type QualityStatService struct {
	v1.UnimplementedQualityStatServiceServer

	data   *data.Data
	minio  *bizadapter.MinioClient
	excel  *bizadapter.ExcelRenderer
	log    *log.Helper
}

// NewQualityStatService 构造服务
func NewQualityStatService(d *data.Data, mc *bizadapter.MinioClient, logger log.Logger) *QualityStatService {
	return &QualityStatService{
		data:  d,
		minio: mc,
		excel: bizadapter.NewExcelRenderer(),
		log:   log.NewHelper(logger),
	}
}

// ==================== 缺陷台账管理 ====================

// ListDefects 获取缺陷列表
func (s *QualityStatService) ListDefects(ctx context.Context, req *v1.ListDefectsReq) (*v1.ListDefectsResp, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}

	f := data.ListDefectsFilter{
		TenantID:    req.TenantId,
		ReviewID:    req.ReviewId,
		DefectLevel: int(req.DefectLevel),
		ModuleName:  req.ModuleName,
		StatMonth:   req.StatMonth,
	}
	if req.IsFixed {
		t := true
		f.IsFixed = &t
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
	items, total, err := s.data.ListDefects(ctx, f, page, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &v1.ListDefectsResp{
		Pagination: paginationResp(page, pageSize, total),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, defectToProto(item))
	}
	return resp, nil
}

// GetDefect 获取缺陷详情
func (s *QualityStatService) GetDefect(ctx context.Context, req *v1.GetDefectReq) (*v1.DefectInfo, error) {
	if req.DefectId == "" {
		return nil, errcode.ErrParamInvalid
	}
	d, err := s.data.GetDefect(ctx, req.DefectId)
	if err != nil {
		return nil, err
	}
	return defectToProto(d), nil
}

// UpdateDefectStatus 更新缺陷修复状态
func (s *QualityStatService) UpdateDefectStatus(ctx context.Context, req *v1.UpdateDefectStatusReq) (*v1.OperateResult, error) {
	if req.DefectId == "" || req.OperatorUid == "" {
		return nil, errcode.ErrParamInvalid
	}
	if err := s.data.UpdateDefectStatus(ctx, req.DefectId, req.IsFixed, req.OperatorUid); err != nil {
		return nil, err
	}
	return &v1.OperateResult{Success: true, Message: "缺陷状态已更新"}, nil
}

// ==================== 质量统计指标 ====================

// GetQualityMetrics 获取综合质量统计指标
func (s *QualityStatService) GetQualityMetrics(ctx context.Context, req *v1.GetMetricsReq) (*v1.QualityMetrics, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}
	var startTime, endTime interface{}
	if req.StartTime != nil {
		startTime = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		endTime = req.EndTime.AsTime()
	}

	m, err := s.data.GetQualityMetrics(ctx, req.TenantId, req.ProjectId, req.Period, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return &v1.QualityMetrics{
		ReviewCompletionRate:  m.ReviewCompletionRate,
		DefectFixTimelyRate:  m.DefectFixTimelyRate,
		TotalReviews:         m.TotalReviews,
		CompletedReviews:     m.CompletedReviews,
		TotalDefects:         m.TotalDefects,
		FixedDefects:         m.FixedDefects,
		AvgFixHours:          m.AvgFixHours,
		FatalDefects:         m.FatalDefects,
		CriticalDefects:      m.CriticalDefects,
		MajorDefects:         m.MajorDefects,
		MinorDefects:         m.MinorDefects,
	}, nil
}

// GetUserStats 获取个人评审统计
func (s *QualityStatService) GetUserStats(ctx context.Context, req *v1.GetUserStatsReq) (*v1.UserStats, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}
	var startTime, endTime interface{}
	if req.StartTime != nil {
		startTime = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		endTime = req.EndTime.AsTime()
	}

	items, err := s.data.GetUserStats(ctx, req.TenantId, req.UserId, req.Period, startTime, endTime)
	if err != nil {
		return nil, err
	}

	resp := &v1.UserStats{}
	for _, item := range items {
		resp.Items = append(resp.Items, &v1.UserStatItem{
			UserId:         item.UserID,
			UserName:       item.UserName,
			ReviewCount:    item.ReviewCount,
			CompletionRate: item.CompletionRate,
			DefectCount:    item.DefectCount,
			FixedCount:     item.FixedCount,
			AvgReviewHours: item.AvgReviewHours,
		})
	}
	return resp, nil
}

// GetModuleDefectStats 获取模块缺陷分布
func (s *QualityStatService) GetModuleDefectStats(ctx context.Context, req *v1.GetModuleStatsReq) (*v1.ModuleDefectStats, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}
	var startTime, endTime interface{}
	if req.StartTime != nil {
		startTime = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		endTime = req.EndTime.AsTime()
	}

	items, err := s.data.GetModuleDefectStats(ctx, req.TenantId, req.ProjectId, req.Period, startTime, endTime)
	if err != nil {
		return nil, err
	}

	resp := &v1.ModuleDefectStats{}
	for _, item := range items {
		resp.Items = append(resp.Items, &v1.ModuleStatItem{
			ModuleName:  item.ModuleName,
			DefectCount: item.DefectCount,
			Percentage:  item.Percentage,
			MaxSeverity: v1.DefectLevel(item.MaxSeverity),
		})
	}
	return resp, nil
}

// GetTechDebtStats 获取技术债务统计
func (s *QualityStatService) GetTechDebtStats(ctx context.Context, req *v1.GetTechDebtReq) (*v1.TechDebtStats, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}

	stats, err := s.data.GetTechDebtStats(ctx, req.TenantId, req.ProjectId)
	if err != nil {
		return nil, err
	}

	resp := &v1.TechDebtStats{
		TotalUnfixed:    stats.TotalUnfixed,
		FatalUnfixed:    stats.FatalUnfixed,
		CriticalUnfixed: stats.CriticalUnfixed,
		DebtScore:       stats.DebtScore,
	}
	for _, m := range stats.TopModules {
		resp.TopModules = append(resp.TopModules, &v1.ModuleStatItem{
			ModuleName:  m.ModuleName,
			DefectCount: m.DefectCount,
			Percentage:  m.Percentage,
			MaxSeverity: v1.DefectLevel(m.MaxSeverity),
		})
	}
	return resp, nil
}

// ==================== 数据大盘 ====================

// GetDashboard 获取大盘概览数据
func (s *QualityStatService) GetDashboard(ctx context.Context, req *v1.GetDashboardReq) (*v1.DashboardData, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}
	var startTime, endTime interface{}
	if req.StartTime != nil {
		startTime = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		endTime = req.EndTime.AsTime()
	}

	// 综合指标
	metrics, err := s.data.GetQualityMetrics(ctx, req.TenantId, "", req.Period, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// 趋势数据
	reviewTrend, defectTrend, err := s.data.GetDashboardTrends(ctx, req.TenantId, req.Period, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// 模块分布
	moduleStats, err := s.data.GetModuleDefectStats(ctx, req.TenantId, "", req.Period, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// 用户统计 TOP5
	userStats, err := s.data.GetUserStats(ctx, req.TenantId, "", req.Period, startTime, endTime)
	if err != nil {
		return nil, err
	}

	resp := &v1.DashboardData{
		Metrics: &v1.QualityMetrics{
			ReviewCompletionRate:  metrics.ReviewCompletionRate,
			DefectFixTimelyRate:  metrics.DefectFixTimelyRate,
			TotalReviews:         metrics.TotalReviews,
			CompletedReviews:     metrics.CompletedReviews,
			TotalDefects:         metrics.TotalDefects,
			FixedDefects:         metrics.FixedDefects,
			AvgFixHours:          metrics.AvgFixHours,
			FatalDefects:         metrics.FatalDefects,
			CriticalDefects:      metrics.CriticalDefects,
			MajorDefects:         metrics.MajorDefects,
			MinorDefects:         metrics.MinorDefects,
		},
	}
	for _, t := range reviewTrend {
		resp.ReviewTrend = append(resp.ReviewTrend, &v1.TrendPoint{Date: t.Date, Count: t.Count})
	}
	for _, t := range defectTrend {
		resp.DefectTrend = append(resp.DefectTrend, &v1.TrendPoint{Date: t.Date, Count: t.Count})
	}
	for _, m := range moduleStats {
		resp.ModuleDistribution = append(resp.ModuleDistribution, &v1.ModuleStatItem{
			ModuleName:  m.ModuleName,
			DefectCount: m.DefectCount,
			Percentage:  m.Percentage,
			MaxSeverity: v1.DefectLevel(m.MaxSeverity),
		})
	}
	top5 := userStats
	if len(top5) > 5 {
		top5 = top5[:5]
	}
	for _, u := range top5 {
		resp.TopReviewers = append(resp.TopReviewers, &v1.UserStatItem{
			UserId:         u.UserID,
			UserName:       u.UserName,
			ReviewCount:    u.ReviewCount,
			CompletionRate: u.CompletionRate,
			DefectCount:    u.DefectCount,
			FixedCount:     u.FixedCount,
			AvgReviewHours: u.AvgReviewHours,
		})
	}
	return resp, nil
}

// ==================== 报表生成与导出 ====================

// GenerateReport 生成质量报表
func (s *QualityStatService) GenerateReport(ctx context.Context, req *v1.GenerateReportReq) (*v1.ReportInfo, error) {
	if req.TenantId == "" || req.ReportType == "" || req.Format == "" {
		return nil, errcode.ErrParamInvalid
	}

	// 创建报表记录
	report := &data.QualityReport{
		TenantID:   req.TenantId,
		ReportType: req.ReportType,
		Format:     req.Format,
		Title:      fmt.Sprintf("%s质量报表-%s-%s", req.ReportType, req.TenantId, time.Now().Format("20060102")),
		CreatorUID: extractUID(ctx),
		StatMonth:  req.StatMonth,
		Status:     "generating",
	}
	if req.StartTime != nil {
		t := req.StartTime.AsTime()
		report.StartTime = &t
	}
	if req.EndTime != nil {
		t := req.EndTime.AsTime()
		report.EndTime = &t
	}

	if err := s.data.InsertReport(ctx, report); err != nil {
		return nil, err
	}

	// 异步生成报表（goroutine 中执行）
	go func(r *data.QualityReport) {
		genCtx := context.Background()
		genCtx, cancel := context.WithTimeout(genCtx, 60*time.Second)
		defer cancel()

		var startTime, endTime interface{}
		if r.StartTime != nil {
			startTime = r.StartTime
		}
		if r.EndTime != nil {
			endTime = r.EndTime
		}

		// 获取统计数据
		metrics, err := s.data.GetQualityMetrics(genCtx, r.TenantID, "", r.ReportType, startTime, endTime)
		if err != nil {
			s.updateReportError(r.ReportID, "获取指标失败: "+err.Error())
			return
		}
		moduleStats, _ := s.data.GetModuleDefectStats(genCtx, r.TenantID, "", r.ReportType, startTime, endTime)
		userStats, _ := s.data.GetUserStats(genCtx, r.TenantID, "", r.ReportType, startTime, endTime)
		reviewTrend, defectTrend, _ := s.data.GetDashboardTrends(genCtx, r.TenantID, r.ReportType, startTime, endTime)

		// 生成 Excel
		excelData, err := s.excel.RenderMonthlyReport(metrics, moduleStats, userStats, reviewTrend, defectTrend)
		if err != nil {
			s.updateReportError(r.ReportID, "生成Excel失败: "+err.Error())
			return
		}

		// 上传 MinIO
		objectKey := fmt.Sprintf("reports/%s/%s/%s.xlsx", r.ReportType, r.StatMonth, r.ReportID)
		contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		fileURL, err := s.minio.UploadFile(genCtx, objectKey, contentType, excelData)
		if err != nil {
			s.updateReportError(r.ReportID, "上传MinIO失败: "+err.Error())
			return
		}

		// 更新报表状态
		if err := s.data.UpdateReportStatus(genCtx, r.ReportID, "ready", objectKey, fileURL, int64(len(excelData)), ""); err != nil {
			s.log.Errorw("msg", "更新报表状态失败", "report_id", r.ReportID, "error", err.Error())
		}
	}(report)

	return &v1.ReportInfo{
		ReportId:   report.ReportID,
		TenantId:   report.TenantID,
		ReportType: report.ReportType,
		Format:     report.Format,
		Title:      report.Title,
		Status:     "generating",
		CreatorUid: report.CreatorUID,
		CreatedAt:  timestamppb.New(report.CreatedAt),
	}, nil
}

// ListReports 获取报表列表
func (s *QualityStatService) ListReports(ctx context.Context, req *v1.ListReportsReq) (*v1.ListReportsResp, error) {
	if req.TenantId == "" {
		return nil, errcode.ErrParamInvalid
	}
	page, pageSize := paginationOf(req.Pagination)
	items, total, err := s.data.ListReports(ctx, req.TenantId, req.ReportType, page, pageSize)
	if err != nil {
		return nil, err
	}
	resp := &v1.ListReportsResp{
		Pagination: paginationResp(page, pageSize, total),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, reportToProto(item))
	}
	return resp, nil
}

// GetReportDownload 获取报表下载链接
func (s *QualityStatService) GetReportDownload(ctx context.Context, req *v1.GetReportDownloadReq) (*v1.ReportDownloadResp, error) {
	if req.ReportId == "" {
		return nil, errcode.ErrParamInvalid
	}
	r, err := s.data.GetReport(ctx, req.ReportId)
	if err != nil {
		return nil, err
	}
	if r.Status != "ready" {
		return nil, errcode.ErrReportGenerating
	}

	downloadURL, expireAt, err := s.minio.GetDownloadURL(ctx, r.FileKey)
	if err != nil {
		return nil, err
	}

	return &v1.ReportDownloadResp{
		DownloadUrl: downloadURL,
		ExpireAt:    timestamppb.New(expireAt),
	}, nil
}

// ==================== 辅助 ====================

func (s *QualityStatService) updateReportError(reportID, errMsg string) {
	s.log.Errorw("msg", "报表生成失败", "report_id", reportID, "error", errMsg)
	_ = s.data.UpdateReportStatus(context.Background(), reportID, "failed", "", "", 0, errMsg)
}

func extractUID(ctx context.Context) string {
	if uid, ok := ctx.Value("user_id").(string); ok {
		return uid
	}
	return ""
}

// ==================== 转换辅助 ====================

func defectToProto(d *data.DefectRecord) *v1.DefectInfo {
	info := &v1.DefectInfo{
		DefectId:    d.DefectID,
		ReviewId:    d.ReviewID,
		CommentId:   d.CommentID,
		FilePath:    d.FilePath,
		LineNum:     uint32(d.LineNum),
		DefectLevel: v1.DefectLevel(d.DefectLevel),
		Content:     d.Content,
		ModuleName:  d.ModuleName,
		CreatorUid:  d.CreatorUID,
		IsFixed:     d.IsFixed,
		FixedByUid:  d.FixedByUID,
		StatMonth:   d.StatMonth,
		CreatedAt:   timestamppb.New(d.CreatedAt),
	}
	if d.FixTime != nil {
		info.FixTime = timestamppb.New(*d.FixTime)
	}
	return info
}

func reportToProto(r *data.QualityReport) *v1.ReportInfo {
	info := &v1.ReportInfo{
		ReportId:   r.ReportID,
		TenantId:   r.TenantID,
		ReportType: r.ReportType,
		Format:     r.Format,
		Title:      r.Title,
		FileUrl:    r.FileURL,
		Status:     r.Status,
		CreatorUid: r.CreatorUID,
		CreatedAt:  timestamppb.New(r.CreatedAt),
	}
	return info
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
var _ v1.QualityStatServiceServer = (*QualityStatService)(nil)
