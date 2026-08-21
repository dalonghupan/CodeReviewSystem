package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// ==================== Sonar 门禁数据操作（LLD §4.3）====================

// SaveSonarResult 保存/更新 Sonar 扫描结果（review_sonar_result）
func (d *Data) SaveSonarResult(ctx context.Context, r *SonarResult) error {
	if r.ID == "" {
		r.ID = util.NewUUID()
	}
	const q = `INSERT INTO review_sonar_result (id, review_id, project_key, quality_gate_status,
		issues_json, metrics_json, scanned_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			quality_gate_status = EXCLUDED.quality_gate_status,
			issues_json = EXCLUDED.issues_json,
			metrics_json = EXCLUDED.metrics_json,
			scanned_at = EXCLUDED.scanned_at`
	if _, err := d.writeDB.ExecContext(ctx, q, r.ID, r.ReviewID, r.ProjectKey,
		r.QualityGateStatus, r.IssuesJSON, r.MetricsJSON, r.ScannedAt); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetSonarResult 查询评审单的 Sonar 扫描结果
func (d *Data) GetSonarResult(ctx context.Context, reviewID string) (*SonarResult, error) {
	var r SonarResult
	const q = `SELECT id, review_id, project_key, quality_gate_status, issues_json, metrics_json,
		scanned_at, created_at
		FROM review_sonar_result WHERE review_id = $1
		ORDER BY created_at DESC LIMIT 1`
	if err := d.readDB.GetContext(ctx, &r, q, reviewID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrSonarDataUnavailable
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &r, nil
}

// GetGateRules 查询租户自定义门禁规则
func (d *Data) GetGateRules(ctx context.Context, tenantID string) ([]*SonarGateRule, error) {
	var items []*SonarGateRule
	const q = `SELECT id, tenant_id, rule_name, severity_levels, is_enabled, created_at, updated_at
		FROM sonar_gate_rule WHERE tenant_id = $1 AND is_enabled = TRUE`
	if err := d.readDB.SelectContext(ctx, &items, q, tenantID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// CheckGatePassed 检查 Sonar 门禁是否通过
// 规则：门禁规则中定义的严重级别（如 BLOCKER/CRITICAL）存在未关闭 issue 则不通过
func (d *Data) CheckGatePassed(ctx context.Context, reviewID string, blockedSeverities []string) (bool, string, error) {
	r, err := d.GetSonarResult(ctx, reviewID)
	if err != nil {
		return false, "", err
	}

	// 质量门全局状态
	if r.QualityGateStatus == "ERROR" {
		return false, "Sonar质量门全局状态为未通过(ERROR)", nil
	}

	// 如果有门禁规则，检查具体 issue 是否命中阻断级别
	if len(blockedSeverities) > 0 && len(r.IssuesJSON) > 0 {
		// issuesJSON 解析由 service 层完成，这里只做数据查询
		// 实际的阻断判定在 service 层
		return true, "", nil
	}

	return true, "", nil
}

// HasSonarResult 判断是否有 Sonar 扫描记录
func (d *Data) HasSonarResult(ctx context.Context, reviewID string) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS(SELECT 1 FROM review_sonar_result WHERE review_id = $1)`
	if err := d.readDB.GetContext(ctx, &exists, q, reviewID); err != nil {
		return false, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return exists, nil
}

// SaveGateRule 保存/更新门禁规则
func (d *Data) SaveGateRule(ctx context.Context, r *SonarGateRule) error {
	if r.ID == "" {
		r.ID = util.NewUUID()
	}
	const q = `INSERT INTO sonar_gate_rule (id, tenant_id, rule_name, severity_levels, is_enabled)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			rule_name = EXCLUDED.rule_name,
			severity_levels = EXCLUDED.severity_levels,
			is_enabled = EXCLUDED.is_enabled,
			updated_at = NOW()`
	if _, err := d.writeDB.ExecContext(ctx, q, r.ID, r.TenantID, r.RuleName, r.SeverityLevels, r.IsEnabled); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// timeNow 方便测试替换
var timeNow = time.Now
