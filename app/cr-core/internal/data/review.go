package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// CreateReview 创建评审单 + 评审人关联（事务原子性，LLD §1.3-3）
// 唯一索引 idx_review_mr_unique 兜底防重复（与分布式锁双保险）
func (d *Data) CreateReview(ctx context.Context, r *ReviewMain, reviewerUIDs []string) error {
	r.ReviewID = util.NewUUID()
	tx, err := d.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	defer func() { _ = tx.Rollback() }()

	const q = `INSERT INTO review_main (review_id, tenant_id, repo_id, mr_id, git_platform,
		title, description, creator_uid, priority, status, source_branch, target_branch,
		commit_count, changed_file_count, related_issue, deadline)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`
	if _, err := tx.ExecContext(ctx, q, r.ReviewID, r.TenantID, r.RepoID, r.MRID, r.GitPlatform,
		r.Title, r.Description, r.CreatorUID, r.Priority, r.Status, r.SourceBranch, r.TargetBranch,
		r.CommitCount, r.ChangedFileCount, r.RelatedIssue, r.Deadline); err != nil {
		if isUniqueViolation(err) {
			return errcode.ErrReviewAlreadyExists
		}
		return errcode.ErrDatabase.WithDetail(err.Error())
	}

	for _, uid := range reviewerUIDs {
		const rq = `INSERT INTO review_reviewer (review_id, reviewer_uid)
			VALUES ($1, $2) ON CONFLICT (review_id, reviewer_uid) DO NOTHING`
		if _, err := tx.ExecContext(ctx, rq, r.ReviewID, uid); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetReview 查询评审单详情
func (d *Data) GetReview(ctx context.Context, reviewID string) (*ReviewMain, error) {
	var r ReviewMain
	const q = `SELECT review_id, tenant_id, repo_id, mr_id, git_platform, title, description,
		creator_uid, priority, status, source_branch, target_branch, commit_count,
		changed_file_count, related_issue, sonar_pass_flag, deadline, archive_time,
		created_at, updated_at
		FROM review_main WHERE review_id = $1`
	if err := d.readDB.GetContext(ctx, &r, q, reviewID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrReviewNotFound
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &r, nil
}

// GetReviewByMR 按仓库+MR编号查询评审单（Webhook回调定位）
func (d *Data) GetReviewByMR(ctx context.Context, repoID, mrID string) (*ReviewMain, error) {
	var r ReviewMain
	const q = `SELECT review_id, tenant_id, repo_id, mr_id, git_platform, title, description,
		creator_uid, priority, status, source_branch, target_branch, commit_count,
		changed_file_count, related_issue, sonar_pass_flag, deadline, archive_time,
		created_at, updated_at
		FROM review_main WHERE repo_id = $1 AND mr_id = $2 AND status != $3
		ORDER BY created_at DESC LIMIT 1`
	if err := d.readDB.GetContext(ctx, &r, q, repoID, mrID, StatusArchived); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrReviewNotFound
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &r, nil
}

// ListReviewsFilter 评审单列表筛选条件
type ListReviewsFilter struct {
	TenantID    string
	RepoID      string
	Status      int // 0 = 全部
	CreatorUID  string
	ReviewerUID string
	Priority    int // 0 = 全部
	Keyword     string
	StartTime   *time.Time
	EndTime     *time.Time
}

// ListReviews 多维筛选分页查询
func (d *Data) ListReviews(ctx context.Context, f ListReviewsFilter, page, pageSize uint32) ([]*ReviewMain, uint32, error) {
	page, pageSize = util.NormalizePage(page, pageSize)

	const filter = `FROM review_main r
		WHERE r.tenant_id = $1
		  AND ($2 = '' OR r.repo_id = $2)
		  AND ($3 = 0 OR r.status = $3)
		  AND ($4 = '' OR r.creator_uid = $4)
		  AND ($5 = 0 OR r.priority = $5)
		  AND ($6 = '' OR r.title ILIKE '%' || $6 || '%')
		  AND ($7::timestamptz IS NULL OR r.created_at >= $7)
		  AND ($8::timestamptz IS NULL OR r.created_at <= $8)
		  AND ($9 = '' OR EXISTS (
				SELECT 1 FROM review_reviewer rr WHERE rr.review_id = r.review_id AND rr.reviewer_uid = $9))`

	args := []interface{}{f.TenantID, f.RepoID, f.Status, f.CreatorUID, f.Priority, f.Keyword,
		f.StartTime, f.EndTime, f.ReviewerUID}

	var total uint32
	if err := d.readDB.GetContext(ctx, &total, `SELECT COUNT(*) `+filter, args...); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}

	var items []*ReviewMain
	const q = `SELECT r.review_id, r.tenant_id, r.repo_id, r.mr_id, r.git_platform, r.title,
		r.description, r.creator_uid, r.priority, r.status, r.source_branch, r.target_branch,
		r.commit_count, r.changed_file_count, r.related_issue, r.sonar_pass_flag, r.deadline,
		r.archive_time, r.created_at, r.updated_at ` + filter + `
		ORDER BY r.created_at DESC LIMIT $10 OFFSET $11`
	args = append(args, pageSize, util.PageOffset(page, pageSize))
	if err := d.readDB.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, total, nil
}

// UpdateReview 更新评审单基础信息
func (d *Data) UpdateReview(ctx context.Context, r *ReviewMain) error {
	const q = `UPDATE review_main SET title = $2, priority = $3, deadline = $4,
		related_issue = $5, updated_at = NOW()
		WHERE review_id = $1`
	res, err := d.writeDB.ExecContext(ctx, q, r.ReviewID, r.Title, r.Priority, r.Deadline, r.RelatedIssue)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrReviewNotFound
	}
	return nil
}

// ReplaceReviewers 全量替换评审人（事务：先删后插）
func (d *Data) ReplaceReviewers(ctx context.Context, reviewID string, reviewerUIDs []string) error {
	tx, err := d.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM review_reviewer WHERE review_id = $1`, reviewID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	for _, uid := range reviewerUIDs {
		const q = `INSERT INTO review_reviewer (review_id, reviewer_uid) VALUES ($1, $2)`
		if _, err := tx.ExecContext(ctx, q, reviewID, uid); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	}
	if err := tx.Commit(); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// ListReviewers 查询评审人列表
func (d *Data) ListReviewers(ctx context.Context, reviewID string) ([]*ReviewReviewer, error) {
	var items []*ReviewReviewer
	const q = `SELECT id, review_id, reviewer_uid, review_status, review_comment, reviewed_at, created_at
		FROM review_reviewer WHERE review_id = $1 ORDER BY created_at ASC`
	if err := d.readDB.SelectContext(ctx, &items, q, reviewID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// IsReviewer 判断是否指定评审单的评审人
func (d *Data) IsReviewer(ctx context.Context, reviewID, uid string) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS(SELECT 1 FROM review_reviewer WHERE review_id = $1 AND reviewer_uid = $2)`
	if err := d.readDB.GetContext(ctx, &exists, q, reviewID, uid); err != nil {
		return false, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return exists, nil
}

// TransitionStatus 状态流转（CAS 乐观锁：仅当当前状态匹配才更新，防并发错乱）
// 同时写流转日志 + 更新评审人结论，全程单事务（LLD §1.3-3 核心状态变更原子性）
func (d *Data) TransitionStatus(ctx context.Context, reviewID string, fromStatus, toStatus int,
	action, operatorUID, remark string, reviewerConclusion string) error {
	tx, err := d.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	defer func() { _ = tx.Rollback() }()

	// CAS 更新状态
	const updateQ = `UPDATE review_main SET status = $3, updated_at = NOW()
		WHERE review_id = $1 AND status = $2`
	res, err := tx.ExecContext(ctx, updateQ, reviewID, fromStatus, toStatus)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrReviewStatusInvalid
	}

	// 流转日志（不可篡改，SRS F02-06）
	const logQ = `INSERT INTO review_re_review_log (review_id, action, from_status, to_status, operator_uid, remark)
		VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := tx.ExecContext(ctx, logQ, reviewID, action, fromStatus, toStatus, operatorUID, remark); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}

	// 评审人结论（reject/approve 时更新）
	if reviewerConclusion != "" {
		const reviewerQ = `UPDATE review_reviewer SET review_status = $3, review_comment = $4, reviewed_at = NOW()
			WHERE review_id = $1 AND reviewer_uid = $2`
		if _, err := tx.ExecContext(ctx, reviewerQ, reviewID, operatorUID, reviewerConclusion, remark); err != nil {
			return errcode.ErrDatabase.WithDetail(err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// MarkArchived 归档（复审通过 → 归档，记录归档时间）
func (d *Data) MarkArchived(ctx context.Context, reviewID, operatorUID string) error {
	tx, err := d.writeDB.BeginTxx(ctx, nil)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	defer func() { _ = tx.Rollback() }()

	const updateQ = `UPDATE review_main SET status = $2, archive_time = NOW(), updated_at = NOW()
		WHERE review_id = $1 AND status = $3`
	res, err := tx.ExecContext(ctx, updateQ, reviewID, StatusArchived, StatusApproved)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrReviewStatusInvalid.WithDetail("仅复审通过状态可归档")
	}

	const logQ = `INSERT INTO review_re_review_log (review_id, action, from_status, to_status, operator_uid, remark)
		VALUES ($1, 'archive', $2, $3, $4, 'MR合并归档')`
	if _, err := tx.ExecContext(ctx, logQ, reviewID, StatusApproved, StatusArchived, operatorUID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}

	if err := tx.Commit(); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// UpdateSonarPassFlag 更新 Sonar 门禁通过标记
func (d *Data) UpdateSonarPassFlag(ctx context.Context, reviewID string, passed bool) error {
	const q = `UPDATE review_main SET sonar_pass_flag = $2, updated_at = NOW() WHERE review_id = $1`
	if _, err := d.writeDB.ExecContext(ctx, q, reviewID, passed); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// ListTransitionLogs 查询流转日志（审计追溯）
func (d *Data) ListTransitionLogs(ctx context.Context, reviewID string) ([]*ReReviewLog, error) {
	var items []*ReReviewLog
	const q = `SELECT id, review_id, action, from_status, to_status, operator_uid, remark, created_at
		FROM review_re_review_log WHERE review_id = $1 ORDER BY created_at ASC`
	if err := d.readDB.SelectContext(ctx, &items, q, reviewID); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// isUniqueViolation 判断是否为 PG 唯一约束冲突（SQLSTATE 23505）
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
