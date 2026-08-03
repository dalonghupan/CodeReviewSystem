package data

import "time"

// 表结构模型（与 sql/003_review.sql DDL 字段一一对应）

// 评审单状态常量（DDL status 注释 / proto ReviewStatus 同值）
const (
	StatusPending     = 1 // 待评审
	StatusInProgress  = 2 // 评审中
	StatusRejected    = 3 // 驳回待修改
	StatusResubmitted = 4 // 复审提交
	StatusApproved    = 5 // 复审通过
	StatusArchived    = 6 // 归档
)

// ReviewMain 评审主单表
type ReviewMain struct {
	ReviewID         string     `db:"review_id"`
	TenantID         string     `db:"tenant_id"`
	RepoID           string     `db:"repo_id"`
	MRID             string     `db:"mr_id"`
	GitPlatform      string     `db:"git_platform"`
	Title            string     `db:"title"`
	Description      string     `db:"description"`
	CreatorUID       string     `db:"creator_uid"`
	Priority         int        `db:"priority"`
	Status           int        `db:"status"`
	SourceBranch     string     `db:"source_branch"`
	TargetBranch     string     `db:"target_branch"`
	CommitCount      int        `db:"commit_count"`
	ChangedFileCount int        `db:"changed_file_count"`
	RelatedIssue     string     `db:"related_issue"`
	SonarPassFlag    bool       `db:"sonar_pass_flag"`
	Deadline         *time.Time `db:"deadline"`
	ArchiveTime      *time.Time `db:"archive_time"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

// ReviewReviewer 评审人关联表
type ReviewReviewer struct {
	ID            string     `db:"id"`
	ReviewID      string     `db:"review_id"`
	ReviewerUID   string     `db:"reviewer_uid"`
	ReviewStatus  string     `db:"review_status"` // pending / approved / rejected
	ReviewComment string     `db:"review_comment"`
	ReviewedAt    *time.Time `db:"reviewed_at"`
	CreatedAt     time.Time  `db:"created_at"`
}

// ReviewComment 代码评论（PostgreSQL 声明式分区，按月自动路由，DDL §3）
type ReviewComment struct {
	CommentID     string    `db:"comment_id"`
	ReviewID      string    `db:"review_id"`
	FilePath      string    `db:"file_path"`
	LineNum       int       `db:"line_num"`
	CommitVersion string    `db:"commit_version"`
	Content       string    `db:"content"`
	DefectLevel   int       `db:"defect_level"` // 0无 1致命 2严重 3一般 4建议
	ReplyParentID *string   `db:"reply_parent_id"`
	CreateUID     string    `db:"create_uid"`
	IsIsolated    bool      `db:"is_isolated"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// ReReviewLog 状态流转日志（不可篡改，SRS F02-06）
type ReReviewLog struct {
	ID          string    `db:"id"`
	ReviewID    string    `db:"review_id"`
	Action      string    `db:"action"` // start / reject / resubmit / approve / archive
	FromStatus  int       `db:"from_status"`
	ToStatus    int       `db:"to_status"`
	OperatorUID string    `db:"operator_uid"`
	Remark      string    `db:"remark"`
	CreatedAt   time.Time `db:"created_at"`
}

// SonarResult Sonar 扫描关联表（issues/metrics 存 JSONB，LLD §4.3-1）
type SonarResult struct {
	ID                string    `db:"id"`
	ReviewID          string    `db:"review_id"`
	ProjectKey        string    `db:"project_key"`
	QualityGateStatus string    `db:"quality_gate_status"`
	IssuesJSON        []byte    `db:"issues_json"`
	MetricsJSON       []byte    `db:"metrics_json"`
	ScannedAt         time.Time `db:"scanned_at"`
	CreatedAt         time.Time `db:"created_at"`
}

// SonarGateRule 租户自定义门禁规则
type SonarGateRule struct {
	ID             string    `db:"id"`
	TenantID       string    `db:"tenant_id"`
	RuleName       string    `db:"rule_name"`
	SeverityLevels []byte    `db:"severity_levels"` // JSONB: ["BLOCKER","CRITICAL"]
	IsEnabled      bool      `db:"is_enabled"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}
