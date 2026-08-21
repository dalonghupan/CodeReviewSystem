package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// ==================== 缺陷台账管理（DDL defect_record 表）====================

// InsertDefect 插入一条缺陷记录
func (d *Data) InsertDefect(ctx context.Context, rec *DefectRecord) error {
	if rec.DefectID == "" {
		rec.DefectID = util.NewUUID()
	}
	const q = `INSERT INTO defect_record (defect_id, review_id, comment_id, tenant_id, file_path, line_num,
		defect_level, content, module_name, creator_uid, stat_month)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	if _, err := d.writeDB.ExecContext(ctx, q, rec.DefectID, rec.ReviewID, rec.CommentID, rec.TenantID,
		rec.FilePath, rec.LineNum, rec.DefectLevel, rec.Content, rec.ModuleName, rec.CreatorUID, rec.StatMonth); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// BatchInsertDefects 批量插入缺陷记录
func (d *Data) BatchInsertDefects(ctx context.Context, items []*DefectRecord) error {
	for _, item := range items {
		if err := d.InsertDefect(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// GetDefect 查询单条缺陷
func (d *Data) GetDefect(ctx context.Context, defectID string) (*DefectRecord, error) {
	var r DefectRecord
	const q = `SELECT defect_id, review_id, comment_id, tenant_id, file_path, line_num, defect_level,
		content, module_name, creator_uid, is_fixed, fixed_by_uid, fix_time, stat_month, created_at, updated_at
		FROM defect_record WHERE defect_id = $1`
	if err := d.readDB.GetContext(ctx, &r, q, defectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrDefectNotFound
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &r, nil
}

// ListDefectsFilter 缺陷列表筛选条件
type ListDefectsFilter struct {
	TenantID    string
	ReviewID    string
	DefectLevel int  // 0 = 全部
	IsFixed     *bool
	ModuleName  string
	StatMonth   string
	StartTime   *time.Time
	EndTime     *time.Time
}

// ListDefects 多维筛选分页查询
func (d *Data) ListDefects(ctx context.Context, f ListDefectsFilter, page, pageSize uint32) ([]*DefectRecord, uint32, error) {
	page, pageSize = util.NormalizePage(page, pageSize)

	filter := `FROM defect_record WHERE tenant_id = $1`
	args := []interface{}{f.TenantID}
	idx := 2

	if f.ReviewID != "" {
		filter += ` AND review_id = $` + itoa(idx)
		args = append(args, f.ReviewID)
		idx++
	}
	if f.DefectLevel > 0 {
		filter += ` AND defect_level = $` + itoa(idx)
		args = append(args, f.DefectLevel)
		idx++
	}
	if f.IsFixed != nil {
		filter += ` AND is_fixed = $` + itoa(idx)
		args = append(args, *f.IsFixed)
		idx++
	}
	if f.ModuleName != "" {
		filter += ` AND module_name = $` + itoa(idx)
		args = append(args, f.ModuleName)
		idx++
	}
	if f.StatMonth != "" {
		filter += ` AND stat_month = $` + itoa(idx)
		args = append(args, f.StatMonth)
		idx++
	}
	if f.StartTime != nil {
		filter += ` AND created_at >= $` + itoa(idx)
		args = append(args, *f.StartTime)
		idx++
	}
	if f.EndTime != nil {
		filter += ` AND created_at <= $` + itoa(idx)
		args = append(args, *f.EndTime)
		idx++
	}

	var total uint32
	if err := d.readDB.GetContext(ctx, &total, `SELECT COUNT(*) `+filter, args...); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}

	var items []*DefectRecord
	cols := `SELECT defect_id, review_id, comment_id, tenant_id, file_path, line_num, defect_level,
		content, module_name, creator_uid, is_fixed, fixed_by_uid, fix_time, stat_month, created_at, updated_at `
	q := cols + filter + ` ORDER BY created_at DESC LIMIT $` + itoa(idx) + ` OFFSET $` + itoa(idx+1)
	args = append(args, pageSize, util.PageOffset(page, pageSize))
	if err := d.readDB.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, total, nil
}

// UpdateDefectStatus 更新缺陷修复状态
func (d *Data) UpdateDefectStatus(ctx context.Context, defectID string, isFixed bool, fixedByUID string) error {
	if isFixed {
		const q = `UPDATE defect_record SET is_fixed = TRUE, fixed_by_uid = $2, fix_time = NOW(), updated_at = NOW()
			WHERE defect_id = $1`
		if _, err := d.writeDB.ExecContext(ctx, q, defectID, fixedByUID); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	} else {
		const q = `UPDATE defect_record SET is_fixed = FALSE, fixed_by_uid = NULL, fix_time = NULL, updated_at = NOW()
			WHERE defect_id = $1`
		if _, err := d.writeDB.ExecContext(ctx, q, defectID); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	}
	return nil
}
