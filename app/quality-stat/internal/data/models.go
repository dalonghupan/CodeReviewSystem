package data

import "time"

// DefectRecord 缺陷台账（对应 DDL defect_record 表）
type DefectRecord struct {
	DefectID    string     `db:"defect_id"`
	ReviewID    string     `db:"review_id"`
	CommentID   string     `db:"comment_id"`
	TenantID    string     `db:"tenant_id"`
	FilePath    string     `db:"file_path"`
	LineNum     int        `db:"line_num"`
	DefectLevel int        `db:"defect_level"` // 1致命 2严重 3一般 4建议
	Content     string     `db:"content"`
	ModuleName  string     `db:"module_name"`
	CreatorUID  string     `db:"creator_uid"`
	IsFixed     bool       `db:"is_fixed"`
	FixedByUID  string     `db:"fixed_by_uid"`
	FixTime     *time.Time `db:"fix_time"`
	StatMonth   string     `db:"stat_month"` // yyyy-MM
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// QualityMonthlyStat 月度质量统计（对应 DDL quality_monthly_stat 表）
type QualityMonthlyStat struct {
	ID                 string  `db:"id"`
	TenantID           string  `db:"tenant_id"`
	StatMonth          string  `db:"stat_month"`
	UserID             string  `db:"user_id"`    // 空表示租户整体
	ModuleName         string  `db:"module_name"` // 空表示整体

	TotalReviews       int     `db:"total_reviews"`
	CompletedReviews   int     `db:"completed_reviews"`
	AvgReviewHours     float64 `db:"avg_review_hours"`

	TotalDefects       int     `db:"total_defects"`
	FixedDefects       int     `db:"fixed_defects"`
	FatalDefects       int     `db:"fatal_defects"`
	CriticalDefects    int     `db:"critical_defects"`
	MajorDefects       int     `db:"major_defects"`
	MinorDefects       int     `db:"minor_defects"`
	AvgFixHours        float64 `db:"avg_fix_hours"`

	CompletionRate     float64 `db:"completion_rate"`
	FixTimelyRate      float64 `db:"fix_timely_rate"`
	TechDebtScore      float64 `db:"tech_debt_score"`

	CreatedAt          time.Time `db:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"`
}

// QualityReport 报表记录（对应 DDL quality_report 表）
type QualityReport struct {
	ReportID     string     `db:"report_id"`
	TenantID     string     `db:"tenant_id"`
	ReportType   string     `db:"report_type"`  // daily / weekly / monthly
	Format       string     `db:"format"`       // excel / pdf
	Title        string     `db:"title"`
	FileKey      string     `db:"file_key"`
	FileURL      string     `db:"file_url"`
	FileSize     int64      `db:"file_size"`
	Status       string     `db:"status"`       // generating / ready / failed
	ErrorMessage string     `db:"error_message"`
	CreatorUID   string     `db:"creator_uid"`
	StatMonth    string     `db:"stat_month"`
	StartTime    *time.Time `db:"start_time"`
	EndTime      *time.Time `db:"end_time"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

// DefectLevel 缺陷等级常量（匹配 DDL 注释）
const (
	DefectFatal    = 1 // 致命P0
	DefectCritical = 2 // 严重P1
	DefectMajor    = 3 // 一般P2
	DefectMinor    = 4 // 建议P3
)
