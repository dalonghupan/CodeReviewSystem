package data

import (
	"context"
	"fmt"

	"cr-system/pkg/errcode"
)

// ==================== 质量统计指标（DDL quality_monthly_stat 表）====================

// QualityMetrics 综合质量指标
type QualityMetrics struct {
	ReviewCompletionRate float64 // 评审完成率(%)
	DefectFixTimelyRate  float64 // 缺陷修复及时率(%)
	TotalReviews         uint32
	CompletedReviews     uint32
	TotalDefects         uint32
	FixedDefects         uint32
	AvgFixHours          float64
	FatalDefects         uint32
	CriticalDefects      uint32
	MajorDefects         uint32
	MinorDefects         uint32
}

// UserStatItem 用户统计项
type UserStatItem struct {
	UserID         string
	UserName       string
	ReviewCount    uint32
	CompletionRate float64
	DefectCount    uint32
	FixedCount     uint32
	AvgReviewHours float64
}

// ModuleStatItem 模块统计项
type ModuleStatItem struct {
	ModuleName   string
	DefectCount  uint32
	Percentage   float64
	MaxSeverity  int // 最高缺陷等级
}

// TechDebtStats 技术债务统计
type TechDebtStats struct {
	TotalUnfixed    uint32
	FatalUnfixed    uint32
	CriticalUnfixed uint32
	DebtScore       float64
	TopModules      []ModuleStatItem
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Date  string
	Count uint32
}

// GetQualityMetrics 获取综合质量指标
func (d *Data) GetQualityMetrics(ctx context.Context, tenantID, projectID, period string, startTime, endTime interface{}) (*QualityMetrics, error) {
	m := &QualityMetrics{}

	// 总评审数和完成数
	reviewQ := `SELECT COUNT(*) AS total, COALESCE(SUM(CASE WHEN status >= 5 THEN 1 ELSE 0 END), 0) AS completed
		FROM review_main WHERE tenant_id = $1`
	reviewArgs := []interface{}{tenantID}
	idx := 2
	if projectID != "" {
		reviewQ += ` AND repo_id = $` + itoa(idx)
		reviewArgs = append(reviewArgs, projectID)
		idx++
	}
	if startTime != nil {
		reviewQ += ` AND created_at >= $` + itoa(idx)
		reviewArgs = append(reviewArgs, startTime)
		idx++
	}
	if endTime != nil {
		reviewQ += ` AND created_at <= $` + itoa(idx)
		reviewArgs = append(reviewArgs, endTime)
		idx++
	}
	type reviewCount struct {
		Total     uint32 `db:"total"`
		Completed uint32 `db:"completed"`
	}
	var rc reviewCount
	if err := d.readDB.GetContext(ctx, &rc, reviewQ, reviewArgs...); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	m.TotalReviews = rc.Total
	m.CompletedReviews = rc.Completed
	if rc.Total > 0 {
		m.ReviewCompletionRate = float64(rc.Completed) / float64(rc.Total) * 100
	}

	// 缺陷统计
	defectQ := `SELECT COUNT(*) AS total, COALESCE(SUM(CASE WHEN is_fixed THEN 1 ELSE 0 END), 0) AS fixed,
		COALESCE(AVG(CASE WHEN is_fixed AND fix_time IS NOT NULL THEN
			EXTRACT(EPOCH FROM (fix_time - created_at)) / 3600.0 END), 0) AS avg_fix_hours,
		COALESCE(SUM(CASE WHEN defect_level = 1 THEN 1 ELSE 0 END), 0) AS fatal,
		COALESCE(SUM(CASE WHEN defect_level = 2 THEN 1 ELSE 0 END), 0) AS critical,
		COALESCE(SUM(CASE WHEN defect_level = 3 THEN 1 ELSE 0 END), 0) AS major,
		COALESCE(SUM(CASE WHEN defect_level = 4 THEN 1 ELSE 0 END), 0) AS minor
		FROM defect_record WHERE tenant_id = $1`
	defectArgs := []interface{}{tenantID}
	if projectID != "" {
		defectQ += ` AND review_id IN (SELECT review_id FROM review_main WHERE repo_id = $` + itoa(2) + `)`
	}
	type defectCount struct {
		Total        uint32  `db:"total"`
		Fixed        uint32  `db:"fixed"`
		AvgFixHours  float64 `db:"avg_fix_hours"`
		Fatal        uint32  `db:"fatal"`
		Critical     uint32  `db:"critical"`
		Major        uint32  `db:"major"`
		Minor        uint32  `db:"minor"`
	}
	var dc defectCount
	if err := d.readDB.GetContext(ctx, &dc, defectQ, defectArgs...); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	m.TotalDefects = dc.Total
	m.FixedDefects = dc.Fixed
	m.AvgFixHours = dc.AvgFixHours
	m.FatalDefects = dc.Fatal
	m.CriticalDefects = dc.Critical
	m.MajorDefects = dc.Major
	m.MinorDefects = dc.Minor
	if dc.Total > 0 {
		m.DefectFixTimelyRate = float64(dc.Fixed) / float64(dc.Total) * 100
	}

	return m, nil
}

// GetUserStats 获取用户统计
func (d *Data) GetUserStats(ctx context.Context, tenantID, userID, period string, startTime, endTime interface{}) ([]UserStatItem, error) {
	q := `SELECT creator_uid AS user_id, COUNT(*) AS review_count,
		COALESCE(AVG(CASE WHEN status >= 5 THEN 1.0 ELSE 0.0 END), 0) * 100 AS completion_rate
		FROM review_main WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	idx := 2
	if userID != "" {
		q += ` AND creator_uid = $` + itoa(idx)
		args = append(args, userID)
		idx++
	}
	if startTime != nil {
		q += ` AND created_at >= $` + itoa(idx)
		args = append(args, startTime)
		idx++
	}
	if endTime != nil {
		q += ` AND created_at <= $` + itoa(idx)
		args = append(args, endTime)
		idx++
	}
	q += ` GROUP BY creator_uid ORDER BY review_count DESC`

	var items []UserStatItem
	if err := d.readDB.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// GetModuleDefectStats 获取模块缺陷分布
func (d *Data) GetModuleDefectStats(ctx context.Context, tenantID, projectID, period string, startTime, endTime interface{}) ([]ModuleStatItem, error) {
	q := `SELECT module_name, COUNT(*) AS defect_count,
		MAX(defect_level) AS max_severity
		FROM defect_record WHERE tenant_id = $1 AND module_name != ''`
	args := []interface{}{tenantID}
	idx := 2
	if projectID != "" {
		q += ` AND review_id IN (SELECT review_id FROM review_main WHERE repo_id = $` + itoa(idx) + `)`
		args = append(args, projectID)
		idx++
	}
	if startTime != nil {
		q += ` AND created_at >= $` + itoa(idx)
		args = append(args, startTime)
		idx++
	}
	if endTime != nil {
		q += ` AND created_at <= $` + itoa(idx)
		args = append(args, endTime)
		idx++
	}
	q += ` GROUP BY module_name ORDER BY defect_count DESC`

	var items []ModuleStatItem
	if err := d.readDB.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}

	// 计算百分比
	var total uint32
	for _, item := range items {
		total += item.DefectCount
	}
	if total > 0 {
		for i := range items {
			items[i].Percentage = float64(items[i].DefectCount) / float64(total) * 100
		}
	}
	return items, nil
}

// GetTechDebtStats 获取技术债务统计
func (d *Data) GetTechDebtStats(ctx context.Context, tenantID, projectID string) (*TechDebtStats, error) {
	stats := &TechDebtStats{}

	q := `SELECT COUNT(*) AS total,
		COALESCE(SUM(CASE WHEN defect_level = 1 AND NOT is_fixed THEN 1 ELSE 0 END), 0) AS fatal,
		COALESCE(SUM(CASE WHEN defect_level = 2 AND NOT is_fixed THEN 1 ELSE 0 END), 0) AS critical
		FROM defect_record WHERE tenant_id = $1 AND NOT is_fixed`
	args := []interface{}{tenantID}
	if projectID != "" {
		q += ` AND review_id IN (SELECT review_id FROM review_main WHERE repo_id = $2)`
		args = append(args, projectID)
	}
	type debtCount struct {
		Total    uint32 `db:"total"`
		Fatal    uint32 `db:"fatal"`
		Critical uint32 `db:"critical"`
	}
	var dc debtCount
	if err := d.readDB.GetContext(ctx, &dc, q, args...); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	stats.TotalUnfixed = dc.Total
	stats.FatalUnfixed = dc.Fatal
	stats.CriticalUnfixed = dc.Critical
	stats.DebtScore = float64(dc.Fatal)*10 + float64(dc.Critical)*5 + float64(dc.Total-dc.Fatal-dc.Critical)*1

	// TOP5 模块
	modQ := `SELECT module_name, COUNT(*) AS defect_count, MAX(defect_level) AS max_severity
		FROM defect_record WHERE tenant_id = $1 AND NOT is_fixed AND module_name != ''
		GROUP BY module_name ORDER BY defect_count DESC LIMIT 5`
	var topModules []ModuleStatItem
	if err := d.readDB.SelectContext(ctx, &topModules, modQ, tenantID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	stats.TopModules = topModules

	return stats, nil
}

// GetDashboardTrends 获取趋势数据
func (d *Data) GetDashboardTrends(ctx context.Context, tenantID, period string, startTime, endTime interface{}) (reviewTrend, defectTrend []TrendPoint, err error) {
	dateTrunc := "day"
	if period == "week" {
		dateTrunc = "week"
	} else if period == "month" {
		dateTrunc = "month"
	}

	reviewQ := fmt.Sprintf(`SELECT DATE_TRUNC('%s', created_at)::DATE AS date, COUNT(*) AS count
		FROM review_main WHERE tenant_id = $1`, dateTrunc)
	reviewArgs := []interface{}{tenantID}
	if startTime != nil {
		reviewQ += ` AND created_at >= $2`
		reviewArgs = append(reviewArgs, startTime)
	}
	if endTime != nil {
		reviewQ += ` AND created_at <= $3`
		reviewArgs = append(reviewArgs, endTime)
	}
	reviewQ += ` GROUP BY 1 ORDER BY 1`
	if err := d.readDB.SelectContext(ctx, &reviewTrend, reviewQ, reviewArgs...); err != nil {
		return nil, nil, errcode.ErrDatabase.WithDetail(err.Error())
	}

	defectQ := fmt.Sprintf(`SELECT DATE_TRUNC('%s', created_at)::DATE AS date, COUNT(*) AS count
		FROM defect_record WHERE tenant_id = $1`, dateTrunc)
	defectArgs := []interface{}{tenantID}
	if startTime != nil {
		defectQ += ` AND created_at >= $2`
		defectArgs = append(defectArgs, startTime)
	}
	if endTime != nil {
		defectQ += ` AND created_at <= $3`
		defectArgs = append(defectArgs, endTime)
	}
	defectQ += ` GROUP BY 1 ORDER BY 1`
	if err := d.readDB.SelectContext(ctx, &defectTrend, defectQ, defectArgs...); err != nil {
		return nil, nil, errcode.ErrDatabase.WithDetail(err.Error())
	}

	return reviewTrend, defectTrend, nil
}

// UpsertMonthlyStat 更新/插入月度统计
func (d *Data) UpsertMonthlyStat(ctx context.Context, s *QualityMonthlyStat) error {
	const q = `INSERT INTO quality_monthly_stat (tenant_id, stat_month, user_id, module_name,
		total_reviews, completed_reviews, avg_review_hours,
		total_defects, fixed_defects, fatal_defects, critical_defects, major_defects, minor_defects, avg_fix_hours,
		completion_rate, fix_timely_rate, tech_debt_score)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (tenant_id, stat_month, COALESCE(user_id, ''), COALESCE(module_name, '')) DO UPDATE SET
			total_reviews = EXCLUDED.total_reviews,
			completed_reviews = EXCLUDED.completed_reviews,
			avg_review_hours = EXCLUDED.avg_review_hours,
			total_defects = EXCLUDED.total_defects,
			fixed_defects = EXCLUDED.fixed_defects,
			fatal_defects = EXCLUDED.fatal_defects,
			critical_defects = EXCLUDED.critical_defects,
			major_defects = EXCLUDED.major_defects,
			minor_defects = EXCLUDED.minor_defects,
			avg_fix_hours = EXCLUDED.avg_fix_hours,
			completion_rate = EXCLUDED.completion_rate,
			fix_timely_rate = EXCLUDED.fix_timely_rate,
			tech_debt_score = EXCLUDED.tech_debt_score,
			updated_at = NOW()`
	if _, err := d.writeDB.ExecContext(ctx, q, s.TenantID, s.StatMonth, nullIfEmpty(s.UserID),
		nullIfEmpty(s.ModuleName), s.TotalReviews, s.CompletedReviews, s.AvgReviewHours,
		s.TotalDefects, s.FixedDefects, s.FatalDefects, s.CriticalDefects, s.MajorDefects, s.MinorDefects, s.AvgFixHours,
		s.CompletionRate, s.FixTimelyRate, s.TechDebtScore); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// nullIfEmpty 返回 nil 如果字符串为空（用于 NULL 数据库字段）
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
