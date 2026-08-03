package data

import (
	"context"
	"database/sql"
	"errors"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// ==================== 报表管理（DDL quality_report 表）====================

// InsertReport 创建报表记录
func (d *Data) InsertReport(ctx context.Context, r *QualityReport) error {
	if r.ReportID == "" {
		r.ReportID = util.NewUUID()
	}
	const q = `INSERT INTO quality_report (report_id, tenant_id, report_type, format, title, creator_uid,
		stat_month, start_time, end_time, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	if _, err := d.writeDB.ExecContext(ctx, q, r.ReportID, r.TenantID, r.ReportType, r.Format, r.Title,
		r.CreatorUID, r.StatMonth, r.StartTime, r.EndTime, r.Status); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetReport 查询报表
func (d *Data) GetReport(ctx context.Context, reportID string) (*QualityReport, error) {
	var r QualityReport
	const q = `SELECT report_id, tenant_id, report_type, format, title, file_key, file_url, file_size,
		status, error_message, creator_uid, stat_month, start_time, end_time, created_at, updated_at
		FROM quality_report WHERE report_id = $1`
	if err := d.readDB.GetContext(ctx, &r, q, reportID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrReportNotFound
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &r, nil
}

// ListReports 报表分页列表
func (d *Data) ListReports(ctx context.Context, tenantID, reportType string, page, pageSize uint32) ([]*QualityReport, uint32, error) {
	page, pageSize = util.NormalizePage(page, pageSize)

	filter := `FROM quality_report WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	idx := 2
	if reportType != "" {
		filter += ` AND report_type = $` + itoa(idx)
		args = append(args, reportType)
		idx++
	}

	var total uint32
	if err := d.readDB.GetContext(ctx, &total, `SELECT COUNT(*) `+filter, args...); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}

	var items []*QualityReport
	cols := `SELECT report_id, tenant_id, report_type, format, title, file_key, file_url, file_size,
		status, error_message, creator_uid, stat_month, start_time, end_time, created_at, updated_at `
	q := cols + filter + ` ORDER BY created_at DESC LIMIT $` + itoa(idx) + ` OFFSET $` + itoa(idx+1)
	args = append(args, pageSize, util.PageOffset(page, pageSize))
	if err := d.readDB.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, total, nil
}

// UpdateReportStatus 更新报表状态和文件信息
func (d *Data) UpdateReportStatus(ctx context.Context, reportID, status, fileKey, fileURL string, fileSize int64, errMsg string) error {
	const q = `UPDATE quality_report SET status = $2, file_key = $3, file_url = $4, file_size = $5,
		error_message = $6, updated_at = NOW() WHERE report_id = $1`
	if _, err := d.writeDB.ExecContext(ctx, q, reportID, status, fileKey, fileURL, fileSize, errMsg); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}
