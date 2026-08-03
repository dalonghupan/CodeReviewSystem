package data

import (
	"context"
	"database/sql"
	"errors"

	"cr-system/pkg/errcode"
	"cr-system/pkg/util"
)

// ==================== 评论管理（LLD §3.3 / LLD §5.2）====================

// AddComment 添加代码行评论（文件路径+行号+版本三元组绑定）
// 按月分表 comment_yyyyMM（PG声明式分区自动路由）
func (d *Data) AddComment(ctx context.Context, c *ReviewComment) error {
	c.CommentID = util.NewUUID()
	const q = `INSERT INTO review_comment (comment_id, review_id, file_path, line_num, commit_version,
		content, defect_level, create_uid)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := d.writeDB.ExecContext(ctx, q, c.CommentID, c.ReviewID, c.FilePath, c.LineNum,
		c.CommitVersion, c.Content, c.DefectLevel, c.CreateUID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// ReplyComment 回复评论（多层回复：reply_parent_id 指向父评论）
func (d *Data) ReplyComment(ctx context.Context, parentID string, reply *ReviewComment) error {
	reply.CommentID = util.NewUUID()
	reply.ReplyParentID = &parentID

	const q = `INSERT INTO review_comment (comment_id, review_id, file_path, line_num, commit_version,
		content, defect_level, reply_parent_id, create_uid)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := d.writeDB.ExecContext(ctx, q, reply.CommentID, reply.ReviewID, reply.FilePath,
		reply.LineNum, reply.CommitVersion, reply.Content, reply.DefectLevel, parentID, reply.CreateUID); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// GetComments 获取评审单全部评论（按文件分组）
// filePath/commitVersion 可选筛选；includeIsolated 控制是否包含已隔离评论
func (d *Data) GetComments(ctx context.Context, reviewID, filePath, commitVersion string, includeIsolated bool) ([]*ReviewComment, error) {
	q := `SELECT comment_id, review_id, file_path, line_num, commit_version, content, defect_level,
		reply_parent_id, create_uid, is_isolated, created_at, updated_at
		FROM review_comment WHERE review_id = $1`
	args := []interface{}{reviewID}
	argIdx := 2

	if filePath != "" {
		q += ` AND file_path = $` + itoa(argIdx)
		args = append(args, filePath)
		argIdx++
	}
	if commitVersion != "" {
		q += ` AND commit_version = $` + itoa(argIdx)
		args = append(args, commitVersion)
		argIdx++
	}
	if !includeIsolated {
		q += ` AND is_isolated = FALSE`
	}

	q += ` ORDER BY file_path, created_at ASC`

	var items []*ReviewComment
	if err := d.readDB.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return items, nil
}

// GetComment 查询单条评论
func (d *Data) GetComment(ctx context.Context, commentID string) (*ReviewComment, error) {
	var c ReviewComment
	const q = `SELECT comment_id, review_id, file_path, line_num, commit_version, content, defect_level,
		reply_parent_id, create_uid, is_isolated, created_at, updated_at
		FROM review_comment WHERE comment_id = $1`
	if err := d.readDB.GetContext(ctx, &c, q, commentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errcode.ErrCommentNotFound
		}
		return nil, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return &c, nil
}

// HasReplies 检查评论是否有回复（用于编辑/删除权限校验）
func (d *Data) HasReplies(ctx context.Context, commentID string) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS(SELECT 1 FROM review_comment WHERE reply_parent_id = $1)`
	if err := d.readDB.GetContext(ctx, &exists, q, commentID); err != nil {
		return false, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return exists, nil
}

// UpdateComment 更新评论内容（仅限本人且未被回复前）
func (d *Data) UpdateComment(ctx context.Context, commentID, content string, defectLevel int) error {
	const q = `UPDATE review_comment SET content = $2, defect_level = $3, updated_at = NOW()
		WHERE comment_id = $1`
	res, err := d.writeDB.ExecContext(ctx, q, commentID, content, defectLevel)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrCommentNotFound
	}
	return nil
}

// DeleteComment 删除评论（物理删除，调用前需校验权限和是否有回复）
func (d *Data) DeleteComment(ctx context.Context, commentID string) error {
	const q = `DELETE FROM review_comment WHERE comment_id = $1`
	res, err := d.writeDB.ExecContext(ctx, q, commentID)
	if err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errcode.ErrCommentNotFound
	}
	return nil
}

// IsolateOldComments 隔离旧版本评论（版本迭代后，SRS F02-03）
func (d *Data) IsolateOldComments(ctx context.Context, reviewID, oldCommitVersion string) error {
	const q = `UPDATE review_comment SET is_isolated = TRUE
		WHERE review_id = $1 AND commit_version = $2 AND is_isolated = FALSE`
	if _, err := d.writeDB.ExecContext(ctx, q, reviewID, oldCommitVersion); err != nil {
		return errcode.ErrDatabase.WithDetail(err.Error())
	}
	return nil
}

// CountComments 统计评审单评论和缺陷数
func (d *Data) CountComments(ctx context.Context, reviewID string) (total, defects uint32, err error) {
	const q = `SELECT COUNT(*), COALESCE(SUM(CASE WHEN defect_level > 0 THEN 1 ELSE 0 END), 0)
		FROM review_comment WHERE review_id = $1 AND reply_parent_id IS NULL`
	if err := d.readDB.GetContext(ctx, &struct{ total, defects *uint32 }{&total, &defects}, q, reviewID); err != nil {
		return 0, 0, errcode.ErrDatabase.WithDetail(err.Error())
	}
	return
}

// itoa 简易整数转字符串（避免依赖 strconv 在 SQL 拼接场景使用）
func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0' + i/10)) + string(rune('0' + i%10))
}
